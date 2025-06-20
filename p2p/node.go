// p2p/node.go
package p2p

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"

	// discoveryUtil "github.com/libp2p/go-libp2p/p2p/discovery/util" // Unused
	"tripcodechain_go/pkg/validation" // Changed for DPoS types
	"tripcodechain_go/security"
	"tripcodechain_go/utils"

	"github.com/multiformats/go-multiaddr"
	// "tripcodechain_go/consensus" // Removed if DPoS was the only reason, or keep if other consensus types used
)

const ValidatorServiceTag = "tripcodechain-validator"
const GossipSubValidatorTopic = "/tripcodechain/validators/gossip/1.0.0"
const GossipInterval = 30 * time.Second
const ValidatorRecheckInterval = 5 * time.Minute
const partitionCheckInterval = 2 * time.Minute
const partitionDetectionThreshold = 0.3
const minPeersForPartitionCheck = 2
const partitionDebounceDuration = 5 * time.Minute

var lastPartitionTime time.Time

type GossipEventType string

const (
	GossipEventAddValidator    GossipEventType = "ADD_VALIDATOR"
	GossipEventRemoveValidator GossipEventType = "REMOVE_VALIDATOR"
	GossipEventFullSync        GossipEventType = "FULL_SYNC"
)

type NodeInfo struct {
	Address  string `json:"address"`
	LastSeen int64  `json:"lastSeen"`
}

type PeerReputation struct {
	Address                string        `json:"address"`
	SuccessfulConnections  uint64        `json:"successfulConnections"`
	FailedConnections      uint64        `json:"failedConnections"`
	SuccessfulHeartbeats   uint64        `json:"successfulHeartbeats"`
	FailedHeartbeats       uint64        `json:"failedHeartbeats"`
	TotalLatency           time.Duration `json:"totalLatency"`
	LatencyObservations    uint64        `json:"latencyObservations"`
	AverageLatency         time.Duration `json:"averageLatency"`
	LastSeenOnline         time.Time     `json:"lastSeenOnline"`
	ReputationScore        float64       `json:"reputationScore"`
	IsTemporarilyPenalized bool          `json:"isTemporarilyPenalized"`
	PenaltyEndTime         time.Time     `json:"penaltyEndTime"`
	LastUpdated            time.Time     `json:"lastUpdated"`
}

type HeartbeatPayload struct {
	NodeID          string       `json:"nodeId"`
	Libp2pPeerID    string       `json:"libp2pPeerId"`
	Timestamp       string       `json:"timestamp"`
	KnownValidators []NodeStatus `json:"knownValidators"`
}

type Node struct {
	ID         string
	NodeType   string
	Port       int
	knownNodes map[string]NodeStatus
	Signer     security.Signer
	CryptoID   string
	mutex      sync.RWMutex

	discoveredPeersWithStatus []*NodeStatus
	discoveredPeersMutex      sync.Mutex

	Libp2pHost       host.Host
	KadDHT           *dht.IpfsDHT
	p2pCtx           context.Context
	p2pCancel        context.CancelFunc
	routingDiscovery *routing.RoutingDiscovery
	PubSubService    *pubsub.PubSub
	gossipTopic      *pubsub.Topic
	gossipSub        *pubsub.Subscription
	DPoS             *validation.DPoS // Changed DPoS field type

	peerReputations map[string]*PeerReputation
	reputationMutex sync.RWMutex

	peerValidatorViews       map[string][]NodeStatus
	partitionCheckMutex      sync.RWMutex
	isPotentiallyPartitioned bool
	defaultBootstrapPeers    []string

	ipScanRanges          []string
	ipScannerEnabled      bool
	maxScanAttemptsPerRun int
	targetPeerHttpPort    int
	metricsTicker         *time.Ticker // For periodic metrics updates
}

type ValidatorGossipMessage struct {
	SenderNodeID string          `json:"senderNodeId"`
	EventType    GossipEventType `json:"eventType"`
	Validators   []NodeStatus    `json:"validators"`
}

type NodeRegistrationRequest struct {
	Address  string `json:"address"`
	NodeType string `json:"nodeType"`
}

type NodeStatus struct {
	Address  string `json:"address"`
	NodeType string `json:"nodeType"`
}

func NewNode(port int, dpos *validation.DPoS, initialBootstrapPeers []string, scannerEnabled bool, scanRanges []string, targetPortForScan int, dataDir string, passphrase string) *Node { // Changed dpos parameter type
	nodeID := fmt.Sprintf("localhost:%d", port)

	localSigner, err := security.NewLocalSigner(dataDir, passphrase)
	if err != nil {
		utils.LogError("FATAL: Failed to initialize node signer: %v", err)
		// In a real app, you might os.Exit(1) or ensure this error is handled gracefully upstream.
		// For now, returning nil will likely cause a panic if not checked by the caller.
		return nil
	}
	utils.LogInfo("Node signer initialized successfully.")

	node := &Node{
		ID:                    nodeID,
		NodeType:              "unknown",
		Port:                  port,
		knownNodes:            make(map[string]NodeStatus),
		Signer:                localSigner,
		CryptoID:              localSigner.Address(),
		mutex:                 sync.RWMutex{},
		DPoS:                  dpos,
		peerReputations:       make(map[string]*PeerReputation),
		peerValidatorViews:    make(map[string][]NodeStatus),
		defaultBootstrapPeers: initialBootstrapPeers,
		ipScannerEnabled:      scannerEnabled,
		ipScanRanges:          scanRanges,
		maxScanAttemptsPerRun: 50,
		targetPeerHttpPort:    targetPortForScan,
		metricsTicker:         time.NewTicker(30 * time.Second), // Initialize metrics ticker
	}
	utils.LogInfo("Node cryptographic ID: %s", node.CryptoID)

	p2pCtx, p2pCancel := context.WithCancel(context.Background())
	node.p2pCtx = p2pCtx
	node.p2pCancel = p2pCancel

	listenAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port+1000))
	h, err := libp2p.New(libp2p.ListenAddrs(listenAddr))
	if err != nil {
		utils.LogError("Failed to create libp2p host: %v", err)
		return nil
	}
	node.Libp2pHost = h
	utils.LogInfo("LibP2P Host created: %s, listening on: %v", h.ID(), h.Addrs())

	ps, err := pubsub.NewGossipSub(node.p2pCtx, node.Libp2pHost)
	if err != nil {
		utils.LogError("Failed to create LibP2P PubSub service: %v", err)
		h.Close()
		return nil
	}
	node.PubSubService = ps
	utils.LogInfo("LibP2P PubSub service created.")

	kDHT, err := dht.New(node.p2pCtx, node.Libp2pHost, dht.Mode(dht.ModeServer))
	if err != nil {
		utils.LogError("Failed to create Kademlia DHT: %v", err)
		return nil
	}
	node.KadDHT = kDHT
	node.routingDiscovery = routing.NewRoutingDiscovery(node.KadDHT)

	return node
}

func (n *Node) AddNodeStatus(status NodeStatus) {
	n.mutex.Lock()
	originalKnown := false
	originalNodeType := ""
	if existing, ok := n.knownNodes[status.Address]; ok {
		originalKnown = true
		originalNodeType = existing.NodeType
	}

	if status.Address == n.ID {
		n.mutex.Unlock()
		return
	}
	isNewAddition := false
	wasPreviouslyNotValidator := false
	isNowValidator := status.NodeType == "validator"
	existingStatus, exists := n.knownNodes[status.Address]
	if exists {
		if existingStatus.NodeType != status.NodeType {
			utils.LogInfo("Updating node status for %s: NodeType changed from %s to %s", status.Address, existingStatus.NodeType, status.NodeType)
			n.knownNodes[status.Address] = status
			if isNowValidator && existingStatus.NodeType != "validator" {
				wasPreviouslyNotValidator = true
			}
		} else {
			n.mutex.Unlock()
			return
		}
	} else {
		utils.LogInfo("Adding new node with status: %s (NodeType: %s)", status.Address, status.NodeType)
		n.knownNodes[status.Address] = status
		isNewAddition = true
	}
	n.mutex.Unlock()
	if isNowValidator && (isNewAddition || wasPreviouslyNotValidator) {
		n.publishValidatorEvent(GossipEventAddValidator, status)
		// This is a good place to increment ValidatorsAdded if the source is known
		// However, AddNodeStatus is generic. Source-specific metric increment should be done by caller.
	}
	// If node type changed from validator to something else, or was removed (handled in recheck/gossip remove)
	if originalKnown && originalNodeType == "validator" && !isNowValidator {
		ValidatorsRemoved.WithLabelValues("type_change").Inc()
	}
}

func (n *Node) GetKnownNodeStatuses() []NodeStatus {
	n.mutex.RLock()
	defer n.mutex.RUnlock()
	statuses := make([]NodeStatus, 0, len(n.knownNodes))
	for _, status := range n.knownNodes {
		statuses = append(statuses, status)
	}
	return statuses
}

func (n *Node) GetKnownNodeStatus(address string) (NodeStatus, bool) {
	n.mutex.RLock()
	defer n.mutex.RUnlock()
	status, exists := n.knownNodes[address]
	return status, exists
}

func (n *Node) StartHeartbeatSender() {
	if n.Libp2pHost == nil {
		utils.LogInfo("WARN: LibP2P Host not initialized, cannot include Libp2pPeerID in heartbeats. Skipping heartbeat sending.")
		return
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	defer utils.LogInfo("Exiting StartHeartbeatSender goroutine.")
	for {
		select {
		case <-n.p2pCtx.Done():
			utils.LogInfo("Stopping heartbeat sender due to context cancellation.")
			return
		case <-ticker.C:
			allKnownStatuses := n.GetKnownNodeStatuses()
			validatorStatuses := []NodeStatus{}
			for _, status := range allKnownStatuses {
				if status.NodeType == "validator" {
					validatorStatuses = append(validatorStatuses, status)
				}
			}
			payload := HeartbeatPayload{
				NodeID: n.ID, Libp2pPeerID: n.Libp2pHost.ID().String(),
				Timestamp: time.Now().UTC().Format(time.RFC3339), KnownValidators: validatorStatuses,
			}
			for _, peerNodeStatus := range allKnownStatuses {
				if peerNodeStatus.Address == n.ID {
					continue
				}
				rep := n.GetOrInitializeReputation(peerNodeStatus.Address)
				n.reputationMutex.RLock()
				isPenalized := rep.IsTemporarilyPenalized
				penaltyEndTime := rep.PenaltyEndTime
				currentRepScore := rep.ReputationScore
				n.reputationMutex.RUnlock()
				if isPenalized && time.Now().Before(penaltyEndTime) {
					utils.LogDebug("Skipping heartbeat to %s due to active penalty (score: %.2f). Penalty ends at %s.", peerNodeStatus.Address, currentRepScore, penaltyEndTime.Format(time.RFC3339))
					continue
				}
				go n.sendSingleHeartbeat(peerNodeStatus.Address, payload)
			}
		}
	}
}

func (n *Node) sendSingleHeartbeat(targetHttpAddress string, payload HeartbeatPayload) {
	HeartbeatsSent.Inc() // Metric
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		utils.LogError("Failed to marshal heartbeat payload for %s: %v", targetHttpAddress, err)
		return
	}
	url := fmt.Sprintf("http://%s/heartbeat", targetHttpAddress)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		utils.LogError("Error creating heartbeat request for %s: %v", targetHttpAddress, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	startTime := time.Now()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		utils.LogInfo("WARN: Failed to send heartbeat to %s: %v", targetHttpAddress, err)
		n.RecordHeartbeatResponse(targetHttpAddress, false, 0)
		HeartbeatFailures.WithLabelValues(targetHttpAddress).Inc() // Metric
		return
	}
	defer resp.Body.Close()
	latency := time.Since(startTime)
	if resp.StatusCode != http.StatusOK {
		utils.LogInfo("WARN: Heartbeat to %s responded with status %s", targetHttpAddress, resp.Status)
		n.RecordHeartbeatResponse(targetHttpAddress, false, latency)
		HeartbeatFailures.WithLabelValues(targetHttpAddress).Inc() // Metric
	} else {
		utils.LogDebug("Heartbeat successfully sent to %s (latency: %v)", targetHttpAddress, latency)
		n.RecordHeartbeatResponse(targetHttpAddress, true, latency)
	}
}

func (n *Node) DiscoverNodes() {
	if n.Libp2pHost == nil || n.KadDHT == nil || n.routingDiscovery == nil {
		utils.LogError("LibP2P components not initialized, cannot discover nodes via DHT.")
		return
	}
	peerChan, err := n.FindValidatorsDHT()
	if err != nil {
		utils.LogError("Failed to start DHT validator discovery: %v", err)
		return
	}
	utils.LogInfo("Started DHT validator discovery. Waiting for peers...")
	processedInThisCycle := make(map[peer.ID]bool)
	discoveryTimeout := 60 * time.Second
	ctx, cancel := context.WithTimeout(n.p2pCtx, discoveryTimeout)
	defer cancel()
	for {
		select {
		case peerInfo, ok := <-peerChan:
			if !ok {
				utils.LogInfo("DHT peer channel closed.")
				return
			}
			if peerInfo.ID == n.Libp2pHost.ID() {
				continue
			}
			if processedInThisCycle[peerInfo.ID] {
				continue
			}
			processedInThisCycle[peerInfo.ID] = true
			DHTPeersDiscovered.WithLabelValues("any").Inc() // Metric for any peer
			utils.LogInfo("Discovered potential validator via DHT: %s, Addrs: %v", peerInfo.ID, peerInfo.Addrs)
			var httpAddr string
			for _, addr := range peerInfo.Addrs {
				protoTCP, errTCP := addr.ValueForProtocol(multiaddr.P_TCP)
				if errTCP == nil {
					ipProto, errIP := addr.ValueForProtocol(multiaddr.P_IP4)
					if errIP == nil {
						tcpPort, convErr := strconv.Atoi(protoTCP)
						if convErr == nil && tcpPort > 1000 {
							httpPort := tcpPort - 1000
							httpAddr = fmt.Sprintf("%s:%d", ipProto, httpPort)
							utils.LogDebug("Derived HTTP address %s for peer %s (LibP2P TCP port: %d)", httpAddr, peerInfo.ID, tcpPort)
							break
						}
					}
				}
			}
			if httpAddr == "" {
				utils.LogInfo("Could not determine HTTP address for DHT peer: %s from addrs: %v", peerInfo.ID, peerInfo.Addrs)
				continue
			}
			nodeStatus, err := n.getNodeStatusFromHttp(httpAddr)
			if err != nil {
				utils.LogInfo("WARN: Failed to get NodeStatus from %s (peer %s): %v", httpAddr, peerInfo.ID, err)
				continue
			}
			if nodeStatus.NodeType == "validator" {
				DHTPeersDiscovered.WithLabelValues("validator").Inc() // Metric
				if n.DPoS == nil {
					utils.LogError("DPoS service is nil in Node, cannot verify validator %s", nodeStatus.Address)
				} else {
					eligible, errV := validation.VerifyValidatorEligibility(n.DPoS, nodeStatus.Address)
					if errV != nil {
						utils.LogError("Error verifying eligibility for discovered validator %s: %v", nodeStatus.Address, errV)
						ValidatorVerifications.WithLabelValues("dht", "error").Inc() // Metric
					} else if eligible {
						ValidatorVerifications.WithLabelValues("dht", "eligible").Inc() // Metric
						ValidatorsAdded.WithLabelValues("dht").Inc()                    // Metric
						utils.LogInfo("Confirmed eligible validator %s (%s) via DHT and HTTP status check.", nodeStatus.Address, peerInfo.ID)
						n.AddNodeStatus(*nodeStatus)
					} else {
						ValidatorVerifications.WithLabelValues("dht", "ineligible").Inc() // Metric
						utils.LogInfo("Discovered validator %s (%s) is not eligible.", nodeStatus.Address, peerInfo.ID)
					}
				}
			} else {
				DHTPeersDiscovered.WithLabelValues(nodeStatus.NodeType).Inc()
			} // Metric for other types
		case <-ctx.Done():
			utils.LogInfo("DHT discovery cycle timed out after %v.", discoveryTimeout)
			return
		case <-n.p2pCtx.Done():
			utils.LogInfo("DHT discovery stopped due to p2p context cancellation.")
			return
		}
	}
}

func (n *Node) getNodeStatusFromHttp(httpPeerAddress string) (*NodeStatus, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	url := fmt.Sprintf("http://%s/node-status", httpPeerAddress)
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error getting node status from %s: %w", httpPeerAddress, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting node status from %s: status %d", httpPeerAddress, resp.StatusCode)
	}
	var status NodeStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("error decoding node status from %s: %w", httpPeerAddress, err)
	}
	return &status, nil
}

func (n *Node) getNodePeers(nodeAddress string) ([]*NodeStatus, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s/nodes", nodeAddress))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var peers []*NodeStatus
	if err := json.NewDecoder(resp.Body).Decode(&peers); err != nil {
		return nil, err
	}
	return peers, nil
}

func (n *Node) RegisterWithNode(seedNodeAddress string, libp2pBootstrapPeers []string) { /* ... unchanged ... */
}

func (n *Node) attemptHttpRegistration(seedNodeAddress string, requestBodyBytes []byte, attempt int) bool { /* ... unchanged ... */
	return true
}
func (n *Node) AddNode(address string)                      { /* ... unchanged ... */ }
func (n *Node) GetKnownNodes() []string                     { /* ... unchanged ... */ return nil }
func (n *Node) GetDiscoveredPeersWithStatus() []*NodeStatus { /* ... unchanged ... */ return nil }

func (n *Node) StopLibp2pServices() {
	if n.metricsTicker != nil {
		n.metricsTicker.Stop()
	} // Stop metrics ticker
	if n.p2pCancel != nil {
		n.p2pCancel()
		utils.LogInfo("p2p context cancelled.")
	}
	if n.gossipSub != nil {
		n.gossipSub.Cancel()
		utils.LogInfo("Cancelled gossip subscription.")
	}
	if n.gossipTopic != nil {
		if err := n.gossipTopic.Close(); err != nil {
			utils.LogError("Error closing gossip topic: %v", err)
		} else {
			utils.LogInfo("Closed gossip topic.")
		}
	}
	if n.KadDHT != nil {
		if err := n.KadDHT.Close(); err != nil {
			utils.LogError("Error closing Kademlia DHT: %v", err)
		} else {
			utils.LogInfo("Closed Kademlia DHT.")
		}
	}
	if n.Libp2pHost != nil {
		if err := n.Libp2pHost.Close(); err != nil {
			utils.LogError("Error closing LibP2P host: %v", err)
		} else {
			utils.LogInfo("Closed LibP2P host.")
		}
	}
	utils.LogInfo("LibP2P services stopped for node %s", n.ID)
}

func (n *Node) BootstrapDHT(bootstrapPeerAddrs []string) error { /* ... unchanged, uses p2pCtx ... */
	return nil
}
func (n *Node) AdvertiseAsValidator() { /* ... unchanged, uses p2pCtx ... */ }
func (n *Node) FindValidatorsDHT() (<-chan peer.AddrInfo, error) { /* ... unchanged, uses p2pCtx ... */
	return nil, nil
}

func (n *Node) GetOrInitializeReputation(address string) *PeerReputation {
	n.reputationMutex.Lock()
	defer n.reputationMutex.Unlock()
	if rep, exists := n.peerReputations[address]; exists {
		return rep
	}
	newRep := &PeerReputation{Address: address, ReputationScore: 75.0, LastUpdated: time.Now()}
	n.peerReputations[address] = newRep
	return newRep
}

func (n *Node) RecordConnectionAttempt(address string, successful bool) {
	rep := n.GetOrInitializeReputation(address)
	n.reputationMutex.Lock()
	defer n.reputationMutex.Unlock()
	if successful {
		rep.SuccessfulConnections++
		rep.LastSeenOnline = time.Now()
	} else {
		rep.FailedConnections++
	}
	rep.LastUpdated = time.Now()
	n.calculateReputationScoreInternal(rep)
}

func (n *Node) RecordHeartbeatResponse(address string, successful bool, latency time.Duration) {
	rep := n.GetOrInitializeReputation(address)
	n.reputationMutex.Lock()
	defer n.reputationMutex.Unlock()
	if successful {
		rep.SuccessfulHeartbeats++
		rep.LastSeenOnline = time.Now()
		if latency > 0 {
			rep.TotalLatency += latency
			rep.LatencyObservations++
			if rep.LatencyObservations > 0 {
				rep.AverageLatency = rep.TotalLatency / time.Duration(rep.LatencyObservations)
			}
		}
	} else {
		rep.FailedHeartbeats++
	}
	rep.LastUpdated = time.Now()
	n.calculateReputationScoreInternal(rep)
}

func (n *Node) calculateReputationScoreInternal(rep *PeerReputation) {
	var score float64 = 50.0
	totalConnections := rep.SuccessfulConnections + rep.FailedConnections
	if totalConnections > 0 {
		score += (float64(rep.SuccessfulConnections) / float64(totalConnections)) * 30.0
	} else {
		score += 15.0
	}
	totalHeartbeats := rep.SuccessfulHeartbeats + rep.FailedHeartbeats
	if totalHeartbeats > 0 {
		score += (float64(rep.SuccessfulHeartbeats) / float64(totalHeartbeats)) * 40.0
	} else {
		score += 20.0
	}
	if rep.AverageLatency > 0 {
		if rep.AverageLatency > time.Second {
			score -= 20.0
		} else if rep.AverageLatency > 200*time.Millisecond {
			score -= (float64(rep.AverageLatency-200*time.Millisecond) / float64(time.Second-200*time.Millisecond)) * 20.0
		}
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	rep.ReputationScore = score
	const minInteractionsForPenalization = 10
	totalObservedHeartbeats := rep.SuccessfulHeartbeats + rep.FailedHeartbeats
	if rep.IsTemporarilyPenalized {
		if time.Now().After(rep.PenaltyEndTime) {
			if rep.ReputationScore > 40 {
				utils.LogInfo("Peer %s reputation improved (%.2f) and penalty time passed. Lifting penalty.", rep.Address, rep.ReputationScore)
				rep.IsTemporarilyPenalized = false
			} else {
				utils.LogInfo("Peer %s penalty time passed, but score (%.2f) still too low for rehabilitation. Penalty extended.", rep.Address, rep.ReputationScore)
				rep.PenaltyEndTime = time.Now().Add(10 * time.Minute)
			}
		} else if rep.ReputationScore > 60 {
			utils.LogInfo("Peer %s reputation significantly improved (%.2f) before penalty time fully passed. Lifting penalty early.", rep.Address, rep.ReputationScore)
			rep.IsTemporarilyPenalized = false
		}
	} else {
		if rep.ReputationScore < 20 && totalObservedHeartbeats >= minInteractionsForPenalization {
			utils.LogInfo("Peer %s has low reputation (%.2f) after %d heartbeats. Temporarily penalizing for 10m.", rep.Address, rep.ReputationScore, totalObservedHeartbeats)
			rep.IsTemporarilyPenalized = true
			rep.PenaltyEndTime = time.Now().Add(10 * time.Minute)
		}
	}
	// ReputationScoreHistogram.Observe(rep.ReputationScore) // Metric - done by periodic updater
	utils.LogDebug("Calculated reputation for %s: Score %.2f, Penalized: %t", rep.Address, rep.ReputationScore, rep.IsTemporarilyPenalized)
}

func (n *Node) UpdatePeerLastSeen(address string) {
	rep := n.GetOrInitializeReputation(address)
	n.reputationMutex.Lock()
	defer n.reputationMutex.Unlock()
	rep.LastSeenOnline = time.Now()
	rep.LastUpdated = time.Now()
}

func (n *Node) GetPeerReputationScore(address string) (float64, bool) {
	n.reputationMutex.RLock()
	defer n.reputationMutex.RUnlock()
	if rep, exists := n.peerReputations[address]; exists {
		return rep.ReputationScore, true
	}
	return 0, false
}

// GetDefaultBootstrapPeers returns the initial list of bootstrap peers.
func (n *Node) GetDefaultBootstrapPeers() []string {
	// Consider if a lock is needed if this can be modified post-initialization
	// For now, assuming it's set at init and read-only afterwards.
	return n.defaultBootstrapPeers
}

// P2pCtx returns the node's P2P context.
func (n *Node) P2pCtx() context.Context {
	return n.p2pCtx
}

func (n *Node) StartValidatorMonitoring() {
	if n.DPoS == nil {
		utils.LogInfo("WARN: DPoS instance not available in Node, cannot start validator monitoring.")
		return
	}
	utils.LogInfo("Starting periodic monitoring of known validators.")
	go n.periodicallyRecheckValidators()
}

func (n *Node) periodicallyRecheckValidators() {
	ticker := time.NewTicker(ValidatorRecheckInterval)
	defer ticker.Stop()
	defer utils.LogInfo("Exiting periodicallyRecheckValidators goroutine.")
	for {
		select {
		case <-n.p2pCtx.Done():
			utils.LogInfo("Stopping periodic validator re-check due to context cancellation.")
			return
		case <-ticker.C:
			utils.LogInfo("Performing periodic re-check of known validators.")
			n.recheckKnownValidators()
		}
	}
}

func (n *Node) recheckKnownValidators() {
	if n.DPoS == nil {
		utils.LogError("DPoS instance not available in recheckKnownValidators.")
		return
	}
	currentKnownStatuses := n.GetKnownNodeStatuses()
	validatorsToRemoveDetails := []NodeStatus{}
	for _, status := range currentKnownStatuses {
		if status.NodeType == "validator" {
			eligible, err := validation.VerifyValidatorEligibility(n.DPoS, status.Address)
			outcome := "eligible"
			if err != nil {
				utils.LogError("Error re-checking eligibility for validator %s: %v. Assuming ineligible for now.", status.Address, err)
				eligible = false
				outcome = "error"
			} else if !eligible {
				outcome = "ineligible"
			}
			ValidatorVerifications.WithLabelValues("recheck", outcome).Inc() // Metric
			if !eligible {
				utils.LogInfo("Previously known validator %s (%s) is no longer eligible. Marking for removal.", status.Address, status.NodeType)
				validatorsToRemoveDetails = append(validatorsToRemoveDetails, status)
			} else {
				utils.LogDebug("Validator %s remains eligible.", status.Address)
			}
		}
	}
	if len(validatorsToRemoveDetails) > 0 {
		n.mutex.Lock()
		removedCount := 0
		for _, statusToRemove := range validatorsToRemoveDetails {
			if _, exists := n.knownNodes[statusToRemove.Address]; exists {
				utils.LogInfo("Removing validator %s from known nodes due to ineligibility.", statusToRemove.Address)
				delete(n.knownNodes, statusToRemove.Address)
				removedCount++
				ValidatorsRemoved.WithLabelValues("ineligible_recheck").Inc() // Metric
			}
		}
		n.mutex.Unlock()
		for _, removedStatus := range validatorsToRemoveDetails {
			n.publishValidatorEvent(GossipEventRemoveValidator, removedStatus)
		}
		if removedCount > 0 {
			utils.LogInfo("Completed removal of %d ineligible validators.", removedCount)
		}
	} else {
		utils.LogInfo("All known validators passed re-check.")
	}
}

func (n *Node) publishValidatorEvent(eventType GossipEventType, validator NodeStatus) {
	if n.gossipTopic == nil || n.PubSubService == nil {
		utils.LogInfo("WARN: Cannot publish validator event: PubSub or topic not initialized.")
		return
	}
	if eventType != GossipEventAddValidator && eventType != GossipEventRemoveValidator {
		utils.LogInfo("WARN: Invalid event type for single validator event: %s", eventType)
		return
	}
	gossipMsg := ValidatorGossipMessage{SenderNodeID: n.ID, EventType: eventType, Validators: []NodeStatus{validator}}
	msgBytes, err := json.Marshal(gossipMsg)
	if err != nil {
		utils.LogError("Failed to marshal validator event message (%s for %s): %v", eventType, validator.Address, err)
		return
	}
	utils.LogInfo("Publishing validator event: %s for %s", eventType, validator.Address)
	if err := n.gossipTopic.Publish(n.p2pCtx, msgBytes); err != nil {
		utils.LogError("Failed to publish validator event message (%s for %s): %v", eventType, validator.Address, err)
	} else {
		GossipMessagesSent.WithLabelValues(string(eventType)).Inc() // Metric
	}
}

func (n *Node) StartGossip() error {
	if n.PubSubService == nil {
		return fmt.Errorf("PubSubService not initialized")
	}
	topic, err := n.PubSubService.Join(GossipSubValidatorTopic)
	if err != nil {
		utils.LogError("Failed to join gossip topic %s: %v", GossipSubValidatorTopic, err)
		return err
	}
	n.gossipTopic = topic
	sub, err := topic.Subscribe()
	if err != nil {
		utils.LogError("Failed to subscribe to gossip topic %s: %v", GossipSubValidatorTopic, err)
		return err
	}
	n.gossipSub = sub
	utils.LogInfo("Successfully joined and subscribed to gossip topic: %s", GossipSubValidatorTopic)
	go n.processGossipMessages()
	go n.periodicallyPublishValidators()
	return nil
}

func (n *Node) processGossipMessages() {
	if n.gossipSub == nil {
		utils.LogError("Gossip subscription is nil, cannot process messages.")
		return
	}
	defer utils.LogInfo("Exiting processGossipMessages goroutine.")
	for {
		select {
		case <-n.p2pCtx.Done():
			utils.LogInfo("Stopping gossip message processing due to context cancellation.")
			return
		default:
			ctx, cancel := context.WithTimeout(n.p2pCtx, 5*time.Second)
			msg, err := n.gossipSub.Next(ctx)
			cancel()
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					if n.p2pCtx.Err() != nil {
						utils.LogInfo("Gossip subscription context done, exiting message processing loop.")
						return
					}
					continue
				}
				utils.LogError("Error receiving gossip message: %v", err)
				if n.p2pCtx.Err() != nil {
					return
				}
				continue
			}
			if msg.ReceivedFrom == n.Libp2pHost.ID() {
				continue
			}
			var gossipMsg ValidatorGossipMessage
			if err := json.Unmarshal(msg.Data, &gossipMsg); err != nil {
				utils.LogInfo("WARN: Failed to unmarshal gossip message from %s: %v", msg.ReceivedFrom.String(), err)
				continue
			}

			GossipMessagesReceived.WithLabelValues(string(gossipMsg.EventType)).Inc() // Metric

			switch gossipMsg.EventType {
			case GossipEventAddValidator:
				if len(gossipMsg.Validators) == 1 {
					validatorNodeStatus := gossipMsg.Validators[0]
					if validatorNodeStatus.Address == n.ID {
						continue
					}
					if validatorNodeStatus.NodeType != "validator" {
						utils.LogInfo("WARN: Received ADD_VALIDATOR event for non-validator type %s from %s. Skipping.", validatorNodeStatus.NodeType, gossipMsg.SenderNodeID)
						continue
					}
					utils.LogInfo("Received ADD_VALIDATOR event for %s from %s.", validatorNodeStatus.Address, gossipMsg.SenderNodeID)
					if n.DPoS == nil {
						utils.LogError("DPoS service is nil in Node, cannot verify ADD_VALIDATOR for %s", validatorNodeStatus.Address)
						continue
					}

					eligible, errV := validation.VerifyValidatorEligibility(n.DPoS, validatorNodeStatus.Address)
					outcome := "eligible"
					if errV != nil {
						utils.LogError("Error verifying eligibility for ADD_VALIDATOR %s: %v", validatorNodeStatus.Address, errV)
						outcome = "error"
					} else if !eligible {
						outcome = "ineligible"
					}
					ValidatorVerifications.WithLabelValues("gossip_add", outcome).Inc() // Metric

					if eligible {
						ValidatorsAdded.WithLabelValues("gossip_add").Inc() // Metric
						n.AddNodeStatus(validatorNodeStatus)
					} else {
						utils.LogInfo("ADD_VALIDATOR event for %s is not eligible.", validatorNodeStatus.Address)
					}
				} else {
					utils.LogInfo("WARN: ADD_VALIDATOR event from %s has incorrect validator count: %d", gossipMsg.SenderNodeID, len(gossipMsg.Validators))
				}
			case GossipEventRemoveValidator:
				if len(gossipMsg.Validators) == 1 {
					validatorNodeStatus := gossipMsg.Validators[0]
					if validatorNodeStatus.Address == n.ID {
						continue
					}
					utils.LogInfo("Received REMOVE_VALIDATOR event for %s from %s.", validatorNodeStatus.Address, gossipMsg.SenderNodeID)
					n.mutex.Lock()
					if knownStatus, exists := n.knownNodes[validatorNodeStatus.Address]; exists {
						if knownStatus.NodeType == "validator" {
							delete(n.knownNodes, validatorNodeStatus.Address)
							ValidatorsRemoved.WithLabelValues("gossip_remove").Inc() // Metric
							utils.LogInfo("Removed validator %s based on REMOVE_VALIDATOR event from %s.", validatorNodeStatus.Address, gossipMsg.SenderNodeID)
						} else {
							utils.LogInfo("WARN: Received REMOVE_VALIDATOR for %s, but it's not typed as validator locally (%s). No action taken.", validatorNodeStatus.Address, knownStatus.NodeType)
						}
					} else {
						utils.LogDebug("Received REMOVE_VALIDATOR for %s, but it's not in knownNodes. No action taken.", validatorNodeStatus.Address)
					}
					n.mutex.Unlock()
				} else {
					utils.LogInfo("WARN: REMOVE_VALIDATOR event from %s has incorrect validator count: %d", gossipMsg.SenderNodeID, len(gossipMsg.Validators))
				}
			case GossipEventFullSync:
				utils.LogInfo("Received FULL_SYNC event from %s with %d validators.", gossipMsg.SenderNodeID, len(gossipMsg.Validators))
				if gossipMsg.SenderNodeID != "" && gossipMsg.SenderNodeID != n.ID {
					n.StorePeerValidatorView(gossipMsg.SenderNodeID, gossipMsg.Validators)
				}
				for _, validatorNodeStatus := range gossipMsg.Validators {
					if validatorNodeStatus.Address == n.ID {
						continue
					}
					if validatorNodeStatus.NodeType != "validator" {
						continue
					}
					if n.DPoS == nil {
						utils.LogError("DPoS service is nil in Node, cannot verify validator %s from FULL_SYNC", validatorNodeStatus.Address)
						continue
					}

					eligible, errV := validation.VerifyValidatorEligibility(n.DPoS, validatorNodeStatus.Address)
					outcome := "eligible"
					if errV != nil {
						utils.LogError("Error verifying eligibility for validator %s from FULL_SYNC: %v", validatorNodeStatus.Address, errV)
						outcome = "error"
					} else if !eligible {
						outcome = "ineligible"
					}
					ValidatorVerifications.WithLabelValues("gossip_full_sync", outcome).Inc() // Metric

					if eligible {
						// Check if it's a truly new addition before calling ValidatorsAdded
						_, exists := n.GetKnownNodeStatus(validatorNodeStatus.Address)
						if !exists {
							ValidatorsAdded.WithLabelValues("gossip_full_sync").Inc()
						} // Metric
						n.AddNodeStatus(validatorNodeStatus)
					} else {
						utils.LogInfo("Validator %s from FULL_SYNC is not eligible.", validatorNodeStatus.Address)
					}
				}
			default:
				utils.LogInfo("WARN: Received gossip message with unknown event type: %s from %s", gossipMsg.EventType, gossipMsg.SenderNodeID)
			}
		}
	}
}

func (n *Node) periodicallyPublishValidators() {
	if n.gossipTopic == nil {
		utils.LogError("Gossip topic is nil, cannot publish.")
		return
	}
	ticker := time.NewTicker(GossipInterval)
	defer ticker.Stop()
	defer utils.LogInfo("Exiting periodicallyPublishValidators goroutine.")
	for {
		select {
		case <-n.p2pCtx.Done():
			utils.LogInfo("Stopping periodic validator publishing due to context cancellation.")
			return
		case <-ticker.C:
			allKnownStatuses := n.GetKnownNodeStatuses()
			validatorStatuses := []NodeStatus{}
			for _, status := range allKnownStatuses {
				if status.NodeType == "validator" {
					validatorStatuses = append(validatorStatuses, status)
				}
			}
			if len(validatorStatuses) == 0 {
				continue
			}
			gossipMsg := ValidatorGossipMessage{SenderNodeID: n.ID, EventType: GossipEventFullSync, Validators: validatorStatuses}
			msgBytes, err := json.Marshal(gossipMsg)
			if err != nil {
				utils.LogError("Failed to marshal validator gossip message: %v", err)
				continue
			}
			utils.LogInfo("Publishing FULL_SYNC with %d validators via gossip.", len(validatorStatuses))
			if err := n.gossipTopic.Publish(n.p2pCtx, msgBytes); err != nil {
				utils.LogError("Failed to publish FULL_SYNC validator gossip message: %v", err)
			} else {
				GossipMessagesSent.WithLabelValues(string(GossipEventFullSync)).Inc() // Metric
			}
		}
	}
}

func (n *Node) StorePeerValidatorView(peerHttpAddress string, validators []NodeStatus) { /* ... unchanged ... */
}
func (n *Node) checkForNetworkPartition() {
	localValidatorsMap := make(map[string]struct{})
	n.mutex.RLock()
	for addr, status := range n.knownNodes {
		if status.NodeType == "validator" {
			localValidatorsMap[addr] = struct{}{}
		}
	}
	n.mutex.RUnlock()
	n.partitionCheckMutex.RLock()
	if len(n.peerValidatorViews) < minPeersForPartitionCheck && len(localValidatorsMap) > 0 {
		n.partitionCheckMutex.RUnlock()
		return
	}
	partitionSuspicions := 0
	validPeerViewsChecked := 0
	for peerAddr, peerValList := range n.peerValidatorViews {
		validPeerViewsChecked++
		peerValidatorsMap := make(map[string]struct{})
		for _, status := range peerValList {
			peerValidatorsMap[status.Address] = struct{}{}
		}
		if len(localValidatorsMap) == 0 && len(peerValidatorsMap) == 0 {
			continue
		}
		if (len(localValidatorsMap) == 0 && len(peerValidatorsMap) > 0) || (len(localValidatorsMap) > 0 && len(peerValidatorsMap) == 0) {
			utils.LogInfo("Partition Check: Major discrepancy with %s. Local: %d, Peer: %d.", peerAddr, len(localValidatorsMap), len(peerValidatorsMap))
			partitionSuspicions++
			continue
		}
		intersectionSize := 0
		for valAddr := range localValidatorsMap {
			if _, exists := peerValidatorsMap[valAddr]; exists {
				intersectionSize++
			}
		}
		unionSize := len(localValidatorsMap) + len(peerValidatorsMap) - intersectionSize
		if unionSize == 0 {
			continue
		}
		jaccardIndex := float64(intersectionSize) / float64(unionSize)
		utils.LogDebug("Partition Check with %s: Local Vals: %d, Peer Vals: %d, Jaccard: %.2f", peerAddr, len(localValidatorsMap), len(peerValidatorsMap), jaccardIndex)
		if jaccardIndex < partitionDetectionThreshold {
			utils.LogInfo("Low validator set overlap with peer %s (Jaccard: %.2f). Possible partition symptom.", peerAddr, jaccardIndex)
			partitionSuspicions++
		}
	}
	n.partitionCheckMutex.RUnlock()
	n.partitionCheckMutex.Lock()
	defer n.partitionCheckMutex.Unlock()
	if validPeerViewsChecked > 0 && (float64(partitionSuspicions)/float64(validPeerViewsChecked) >= 0.5) {
		if !n.isPotentiallyPartitioned {
			utils.LogInfo("WARN: NETWORK PARTITION DETECTED: Significant validator discrepancies with multiple peers (%d out of %d peers checked).", partitionSuspicions, validPeerViewsChecked)
			NetworkPartitionsDetected.Inc() // Metric
			InPartitionStateGauge.Set(1.0)  // Metric
			n.isPotentiallyPartitioned = true
			lastPartitionTime = time.Now()
			go n.triggerReconnectionActions()
		} else if time.Since(lastPartitionTime) > partitionDebounceDuration {
			utils.LogInfo("WARN: NETWORK PARTITION PERSISTS: Still detected after debounce period. Re-triggering reconnection strategy.")
			NetworkPartitionsDetected.Inc() // Metric for re-trigger/persistence
			lastPartitionTime = time.Now()
			go n.triggerReconnectionActions()
		}
	} else if n.isPotentiallyPartitioned && partitionSuspicions == 0 && validPeerViewsChecked >= minPeersForPartitionCheck {
		utils.LogInfo("NETWORK PARTITION RESOLVED: Validator set discrepancies no longer detected with sufficient peer views.")
		InPartitionStateGauge.Set(0.0) // Metric
		n.isPotentiallyPartitioned = false
	} else if n.isPotentiallyPartitioned && validPeerViewsChecked < minPeersForPartitionCheck && len(localValidatorsMap) > 0 {
		utils.LogInfo("NETWORK PARTITION status maintained: Not enough peer views to confirm resolution, but was previously partitioned.")
	}
}

func (n *Node) triggerReconnectionActions() {
	ReconnectionAttempts.Inc() // Metric
	utils.LogInfo("Reconnection Strategy: Attempting to bootstrap DHT with default peers.")
	if err := n.BootstrapDHT(n.defaultBootstrapPeers); err != nil {
		utils.LogError("Reconnection Strategy: DHT bootstrapping failed: %v", err)
	} else {
		utils.LogInfo("Reconnection Strategy: DHT bootstrapping successful or attempt completed.")
	}
}

func (n *Node) StartNetworkPartitionDetector() {
	if len(n.defaultBootstrapPeers) == 0 {
		utils.LogInfo("WARN: NetworkPartitionDetector: No default bootstrap peers configured. Reconnection strategy will be limited.")
	}
	utils.LogInfo("Starting network partition detector.")
	go func() {
		time.Sleep(partitionCheckInterval / 2)
		ticker := time.NewTicker(partitionCheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-n.p2pCtx.Done():
				utils.LogInfo("Stopping network partition detector.")
				return
			case <-ticker.C:
				n.checkForNetworkPartition()
			}
		}
	}()
}

func (n *Node) StartIPScanner() {
	if !n.ipScannerEnabled {
		utils.LogInfo("IP Scanner is disabled.")
		return
	}
	if len(n.ipScanRanges) == 0 {
		utils.LogInfo("IP Scanner: No IP ranges configured to scan.")
		return
	}
	if n.targetPeerHttpPort == 0 {
		utils.LogInfo("WARN: IP Scanner: Target peer HTTP port is not set. Cannot scan.")
		return
	}
	n.mutex.RLock()
	knownPeersCount := len(n.knownNodes)
	n.mutex.RUnlock()
	const minPeersToSkipScan = 3
	if knownPeersCount >= minPeersToSkipScan {
		utils.LogInfo("IP Scanner: Sufficient peers known (%d). Skipping IP scan.", knownPeersCount)
		return
	}
	utils.LogInfo("IP Scanner: Low peer count (%d). Starting IP scan cycle.", knownPeersCount)
	go n.runIPScanCycle()
}

func (n *Node) runIPScanCycle() {
	successfulFinds := 0
	for i := 0; i < n.maxScanAttemptsPerRun; i++ {
		select {
		case <-n.p2pCtx.Done():
			utils.LogInfo("IP Scanner: Scan cycle cancelled.")
			return
		default:
		}
		IPScanAttempts.Inc() // Metric
		rangeIndex := rand.Intn(len(n.ipScanRanges))
		selectedRange := n.ipScanRanges[rangeIndex]
		randomIP, err := getRandomIPFromRange(selectedRange)
		if err != nil {
			utils.LogInfo("WARN: IP Scanner: Error generating random IP from range %s: %v", selectedRange, err)
			continue
		}
		targetHttpAddress := fmt.Sprintf("%s:%d", randomIP, n.targetPeerHttpPort)
		if targetHttpAddress == n.ID {
			continue
		}
		utils.LogDebug("IP Scanner: Attempting to scan %s", targetHttpAddress)
		nodeStatus, err := n.getNodeStatusFromHttp(targetHttpAddress)
		if err != nil {
			continue
		}
		if nodeStatus.NodeType == "validator" {
			IPScanNodesFound.WithLabelValues("validator_potential").Inc() // Metric
			utils.LogInfo("IP Scanner: Found potential validator %s (type: %s) at %s", nodeStatus.Address, nodeStatus.NodeType, targetHttpAddress)
			if n.DPoS == nil {
				utils.LogError("IP Scanner: DPoS service is nil in Node, cannot verify validator %s", nodeStatus.Address)
				continue
			}
			eligible, verifyErr := validation.VerifyValidatorEligibility(n.DPoS, nodeStatus.Address)
			outcome := "eligible"
			if verifyErr != nil {
				utils.LogError("IP Scanner: Error verifying eligibility for %s: %v", nodeStatus.Address, verifyErr)
				outcome = "error"
			} else if !eligible {
				outcome = "ineligible"
			}
			ValidatorVerifications.WithLabelValues("ip_scan", outcome).Inc() // Metric
			if eligible {
				utils.LogInfo("IP Scanner: Validator %s is eligible. Adding.", nodeStatus.Address)
				IPScanNodesFound.WithLabelValues("validator_eligible").Inc() // Metric
				ValidatorsAdded.WithLabelValues("ip_scan").Inc()             // Metric
				n.AddNodeStatus(*nodeStatus)
				successfulFinds++
				if successfulFinds >= 5 {
					utils.LogInfo("IP Scanner: Found %d validators, concluding scan cycle early.", successfulFinds)
					return
				}
			} else {
				utils.LogInfo("IP Scanner: Potential validator %s not eligible.", nodeStatus.Address)
			}
		} else {
			IPScanNodesFound.WithLabelValues(nodeStatus.NodeType).Inc() // Metric
		}
		time.Sleep(500 * time.Millisecond)
	}
	utils.LogInfo("IP Scanner: Scan cycle completed. Found %d new validators.", successfulFinds)
}

func getRandomIPFromRange(cidr string) (string, error) {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR: %w", err)
	}
	var ones, bits int
	if ip.To4() != nil {
		ones, bits = ipNet.Mask.Size()
		if bits != 32 {
			return "", fmt.Errorf("unexpected IPv4 mask size: %d for CIDR %s", bits, cidr)
		}
	} else if ip.To16() != nil {
		ones, bits = ipNet.Mask.Size()
		if bits != 128 {
			return "", fmt.Errorf("unexpected IPv6 mask size: %d for CIDR %s", bits, cidr)
		}
	} else {
		return "", fmt.Errorf("unsupported IP address type in CIDR: %s", cidr)
	}
	hostBits := bits - ones
	if hostBits < 0 {
		return "", fmt.Errorf("invalid mask in CIDR: %s", cidr)
	}
	if hostBits == 0 {
		return ip.String(), nil
	}
	if hostBits > 31 && ip.To4() != nil {
		return "", fmt.Errorf("CIDR range %s has too many host bits (%d) for random IPv4 selection", cidr, hostBits)
	}
	if hostBits > 30 {
		return "", fmt.Errorf("CIDR range %s is too large to pick a random host effectively with this method", cidr)
	}
	numHosts := int64(1) << hostBits
	if numHosts <= 0 {
		return "", fmt.Errorf("CIDR range %s results in non-positive number of hosts: %d", cidr, numHosts)
	}
	var randomOffset int64
	if numHosts <= 2 {
		randomOffset = rand.New(rand.NewSource(time.Now().UnixNano())).Int63n(numHosts)
	} else {
		if numHosts-2 <= 0 {
			return ip.Mask(ipNet.Mask).String(), nil
		}
		randomOffset = rand.New(rand.NewSource(time.Now().UnixNano())).Int63n(numHosts-2) + 1
	}
	ipAsInt := big.NewInt(0).SetBytes(ip.Mask(ipNet.Mask))
	offsetBigInt := big.NewInt(randomOffset)
	randomIPBigInt := ipAsInt.Add(ipAsInt, offsetBigInt)
	randomIPBytes := randomIPBigInt.Bytes()
	finalIP := make(net.IP, len(ip.Mask(ipNet.Mask)))
	copy(finalIP[len(finalIP)-len(randomIPBytes):], randomIPBytes)
	return finalIP.String(), nil
}

// StartMetricsUpdater starts a goroutine to periodically update Prometheus gauges.
func (n *Node) StartMetricsUpdater() {
	utils.LogInfo("Starting periodic metrics updater.")
	if n.metricsTicker == nil { // Should have been initialized in NewNode
		n.metricsTicker = time.NewTicker(30 * time.Second)
		utils.LogInfo("WARN: Metrics ticker was nil, initialized in StartMetricsUpdater.")
	}
	go func() {
		defer n.metricsTicker.Stop()
		defer utils.LogInfo("Exiting periodic metrics updater goroutine.")
		for {
			select {
			case <-n.p2pCtx.Done():
				utils.LogInfo("Stopping periodic metrics updater.")
				return
			case <-n.metricsTicker.C:
				n.updateAllPeriodicGauges()
			}
		}
	}()
}

// updateAllPeriodicGauges calculates and sets values for gauges.
func (n *Node) updateAllPeriodicGauges() {
	// Update KnownValidatorsGauge
	validatorCount := 0
	n.mutex.RLock()
	for _, status := range n.knownNodes {
		if status.NodeType == "validator" {
			validatorCount++
		}
	}
	n.mutex.RUnlock()
	KnownValidatorsGauge.Set(float64(validatorCount))

	// Update PenalizedPeersGauge and ReputationScoreHistogram
	penalizedCount := 0
	n.reputationMutex.RLock()
	for _, rep := range n.peerReputations {
		ReputationScoreHistogram.Observe(rep.ReputationScore)
		if rep.IsTemporarilyPenalized && time.Now().Before(rep.PenaltyEndTime) {
			penalizedCount++
		}
	}
	n.reputationMutex.RUnlock()
	PenalizedPeersGauge.Set(float64(penalizedCount))

	// Update InPartitionStateGauge
	n.partitionCheckMutex.RLock()
	if n.isPotentiallyPartitioned {
		InPartitionStateGauge.Set(1.0)
	} else {
		InPartitionStateGauge.Set(0.0)
	}
	n.partitionCheckMutex.RUnlock()
}
