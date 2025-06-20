// p2p/node.go
package p2p

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"errors"
	"github.com/libp2p/go-libp2p/core/peer"
	"strconv"

	"github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	discoveryUtil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/multiformats/go-multiaddr"
	"tripcodechain_go/utils"
	"tripcodechain_go/consensus" // Added for DPoS
)

const ValidatorServiceTag = "tripcodechain-validator"
const GossipSubValidatorTopic = "/tripcodechain/validators/gossip/1.0.0"
const GossipInterval = 30 * time.Second // For periodic FULL_SYNC
const ValidatorRecheckInterval = 5 * time.Minute

type GossipEventType string

const (
	GossipEventAddValidator    GossipEventType = "ADD_VALIDATOR"
	GossipEventRemoveValidator GossipEventType = "REMOVE_VALIDATOR"
	GossipEventFullSync        GossipEventType = "FULL_SYNC"
)

// NodeInfo represents information about a peer node in the network.
type NodeInfo struct {
	Address  string `json:"address"`
	LastSeen int64  `json:"lastSeen"`
	// IsActive bool   `json:"isActive"` // Will be determined dynamically or by lastSeen
}

// PeerReputation stores reputation metrics for a peer.
type PeerReputation struct {
	Address               string        `json:"address"` // Peer's HTTP address
	SuccessfulConnections uint64        `json:"successfulConnections"`
	FailedConnections     uint64        `json:"failedConnections"`
	SuccessfulHeartbeats  uint64        `json:"successfulHeartbeats"`
	FailedHeartbeats      uint64        `json:"failedHeartbeats"`
	TotalLatency          time.Duration `json:"totalLatency"`
	LatencyObservations   uint64        `json:"latencyObservations"`
	AverageLatency        time.Duration `json:"averageLatency"`
	LastSeenOnline        time.Time     `json:"lastSeenOnline"`
	ReputationScore       float64       `json:"reputationScore"` // e.g., 0-100
	IsTemporarilyPenalized bool          `json:"isTemporarilyPenalized"`
	PenaltyEndTime        time.Time     `json:"penaltyEndTime"`
	LastUpdated           time.Time     `json:"lastUpdated"`
}

// HeartbeatPayload defines the structure for heartbeat messages.
type HeartbeatPayload struct {
	NodeID          string       `json:"nodeId"`      // HTTP address of the sender
	Libp2pPeerID    string       `json:"libp2pPeerId"`// LibP2P Peer ID of the sender
	Timestamp       string       `json:"timestamp"`
	KnownValidators []NodeStatus `json:"knownValidators"`
}

// Node represents a node in the blockchain network
type Node struct {
	ID         string
	NodeType   string
	Port       int
	knownNodes map[string]NodeStatus // Changed from []string to map[string]NodeStatus
	PrivateKey string
	mutex      sync.Mutex // Protects knownNodes

	discoveredPeersWithStatus []*NodeStatus // For results from specific getNodePeers calls
	discoveredPeersMutex      sync.Mutex    // Protects discoveredPeersWithStatus

	Libp2pHost host.Host
	KadDHT     *dht.IpfsDHT
	p2pCtx     context.Context // Renamed from dhtCtx
	p2pCancel  context.CancelFunc // Renamed from dhtCancel
	routingDiscovery *routing.RoutingDiscovery
	PubSubService *pubsub.PubSub
	gossipTopic   *pubsub.Topic
	gossipSub     *pubsub.Subscription
	DPoS          *consensus.DPoS // Added DPoS field

	peerReputations map[string]*PeerReputation
	reputationMutex sync.RWMutex
}

// ValidatorGossipMessage defines the structure for gossip messages.
type ValidatorGossipMessage struct {
	SenderNodeID string            `json:"senderNodeId"` // LibP2P Peer ID of the sender
	EventType    GossipEventType   `json:"eventType"`
	Validators   []NodeStatus      `json:"validators"` // Single for ADD/REMOVE, multiple for FULL_SYNC
}

type NodeRegistrationRequest struct {
	Address  string `json:"address"`  // Dirección del nodo (ej: "localhost:3001")
	NodeType string `json:"nodeType"` // Tipo de nodo (ej: "validator", "regular", "api", "seed")
}

type NodeStatus struct {
	Address  string `json:"address"`
	NodeType string `json:"nodeType"`
}

// NewNode creates a new node instance
func NewNode(port int, dpos *consensus.DPoS) *Node { // Added dpos parameter
	nodeID := fmt.Sprintf("localhost:%d", port)
	node := &Node{
		ID:         nodeID,
		NodeType:   "unknown", // Default NodeType, will be set by caller or registration
		Port:       port,
		knownNodes: make(map[string]NodeStatus), // Initialize map
		mutex:      sync.Mutex{},
		// discoveredPeersWithStatus is initially nil, discoveredPeersMutex is zero value
		DPoS:            dpos, // Store DPoS instance
		peerReputations: make(map[string]*PeerReputation),
	}

	p2pCtx, p2pCancel := context.WithCancel(context.Background()) // Renamed
	node.p2pCtx = p2pCtx
	node.p2pCancel = p2pCancel

	listenAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port+1000)) // Use a different port for libp2p
	h, err := libp2p.New(libp2p.ListenAddrs(listenAddr))
	if err != nil {
		utils.LogError("Failed to create libp2p host: %v", err)
		// Decide on error handling: return nil, err or panic
		return nil // Or handle more gracefully
	}
	node.Libp2pHost = h
	utils.LogInfo("LibP2P Host created: %s, listening on: %v", h.ID(), h.Addrs())

	// Initialize PubSub Service
	ps, err := pubsub.NewGossipSub(node.p2pCtx, node.Libp2pHost)
	if err != nil {
		utils.LogError("Failed to create LibP2P PubSub service: %v", err)
		h.Close() // Close the host if PubSub fails
		return nil
	}
	node.PubSubService = ps
	utils.LogInfo("LibP2P PubSub service created.")

	kDHT, err := dht.New(node.p2pCtx, node.Libp2pHost, dht.Mode(dht.ModeServer)) // Initially start as server mode
	if err != nil {
		utils.LogError("Failed to create Kademlia DHT: %v", err)
		// h.Close() // Clean up host
		// return nil, err // Or handle more gracefully
		return nil
	}
	node.KadDHT = kDHT
	node.routingDiscovery = routing.NewRoutingDiscovery(node.KadDHT)

	return node
}

// AddNodeStatus adds or updates a node's status in the knownNodes map.
// It now also publishes an ADD_VALIDATOR event if a new validator is added or an existing node becomes a validator.
func (n *Node) AddNodeStatus(status NodeStatus) {
	n.mutex.Lock()

	if status.Address == n.ID { // Do not add self
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
			utils.LogDebug("Node status for %s already up-to-date (NodeType: %s).", status.Address, status.NodeType)
			// No change, no event to publish. Unlock and return.
			n.mutex.Unlock()
			return
		}
	} else {
		utils.LogInfo("Adding new node with status: %s (NodeType: %s)", status.Address, status.NodeType)
		n.knownNodes[status.Address] = status
		isNewAddition = true
	}
	n.mutex.Unlock() // Unlock before potentially publishing

	// Publish event if a new validator was effectively added or promoted
	if isNowValidator && (isNewAddition || wasPreviouslyNotValidator) {
		// This status comes from an external source (DHT, Gossip) and should have been verified for eligibility
		// *before* calling AddNodeStatus.
		n.publishValidatorEvent(GossipEventAddValidator, status)
	}
}

// GetKnownNodeStatuses returns a slice of NodeStatus objects from the knownNodes map.
func (n *Node) GetKnownNodeStatuses() []NodeStatus {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	statuses := make([]NodeStatus, 0, len(n.knownNodes))
	for _, status := range n.knownNodes {
		statuses = append(statuses, status)
	}
	return statuses
}

// GetKnownNodeStatus retrieves a single NodeStatus by address.
func (n *Node) GetKnownNodeStatus(address string) (NodeStatus, bool) {
	n.mutex.Lock() // Changed to full lock as it's a short operation
	defer n.mutex.Unlock()
	status, exists := n.knownNodes[address]
	return status, exists
}

// StartHeartbeatSender is responsible for periodically sending heartbeats.
func (n *Node) StartHeartbeatSender() {
	if n.Libp2pHost == nil {
		utils.LogInfo("WARN: LibP2P Host not initialized, cannot include Libp2pPeerID in heartbeats. Skipping heartbeat sending.")
		return
	}
	ticker := time.NewTicker(15 * time.Second) // Heartbeat interval, e.g., 15s
	defer ticker.Stop()
	defer utils.LogInfo("Exiting StartHeartbeatSender goroutine.")

	for {
		select {
		case <-n.p2pCtx.Done():
			utils.LogInfo("Stopping heartbeat sender due to context cancellation.")
			return
		case <-ticker.C:
			allKnownStatuses := n.GetKnownNodeStatuses() // This is thread-safe
			validatorStatuses := []NodeStatus{}
			for _, status := range allKnownStatuses {
				if status.NodeType == "validator" {
					validatorStatuses = append(validatorStatuses, status)
				}
			}

			payload := HeartbeatPayload{
				NodeID:          n.ID, // HTTP Address
				Libp2pPeerID:    n.Libp2pHost.ID().String(),
				Timestamp:       time.Now().UTC().Format(time.RFC3339),
				KnownValidators: validatorStatuses,
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
					utils.LogDebug("Skipping heartbeat to %s due to active penalty (score: %.2f). Penalty ends at %s.",
						peerNodeStatus.Address, currentRepScore, penaltyEndTime.Format(time.RFC3339))
					continue
				}
				go n.sendSingleHeartbeat(peerNodeStatus.Address, payload)
			}
		}
	}
}

// sendSingleHeartbeat sends a heartbeat payload to a target HTTP address.
func (n *Node) sendSingleHeartbeat(targetHttpAddress string, payload HeartbeatPayload) {
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

	startTime := time.Now() // Record start time for latency calculation
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)

	if err != nil {
		utils.LogInfo("WARN: Failed to send heartbeat to %s: %v", targetHttpAddress, err)
		n.RecordHeartbeatResponse(targetHttpAddress, false, 0) // Record failure (no latency)
		return
	}
	defer resp.Body.Close()

	latency := time.Since(startTime) // Calculate latency

	if resp.StatusCode != http.StatusOK {
		utils.LogInfo("WARN: Heartbeat to %s responded with status %s", targetHttpAddress, resp.Status)
		n.RecordHeartbeatResponse(targetHttpAddress, false, latency) // Record failure (with measured latency)
	} else {
		utils.LogDebug("Heartbeat successfully sent to %s (latency: %v)", targetHttpAddress, latency)
		n.RecordHeartbeatResponse(targetHttpAddress, true, latency)  // Record success
	}
}

// DiscoverNodes now uses DHT to find validators and then confirms their status via HTTP.
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
	discoveryTimeout := 60 * time.Second // Timeout for this discovery cycle
	ctx, cancel := context.WithTimeout(n.p2pCtx, discoveryTimeout) // Use p2pCtx
	defer cancel()

	for {
		select {
		case peerInfo, ok := <-peerChan:
			if !ok { // Channel closed
				utils.LogInfo("DHT peer channel closed.")
				return
			}

			if peerInfo.ID == n.Libp2pHost.ID() { // Skip self
				continue
			}

			if processedInThisCycle[peerInfo.ID] { // Skip already processed in this cycle
				continue
			}
			processedInThisCycle[peerInfo.ID] = true

			utils.LogInfo("Discovered potential validator via DHT: %s, Addrs: %v", peerInfo.ID, peerInfo.Addrs)

			var httpAddr string
			for _, addr := range peerInfo.Addrs {
				protoTCP, errTCP := addr.ValueForProtocol(multiaddr.P_TCP)
				if errTCP == nil {
					ipProto, errIP := addr.ValueForProtocol(multiaddr.P_IP4) // TODO: Also handle P_IP6
					if errIP == nil {
						tcpPort, convErr := strconv.Atoi(protoTCP)
						if convErr == nil && tcpPort > 1000 { // Convention: LibP2P port is HTTP port + 1000
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
				utils.LogInfo("WARN: Failed to get NodeStatus from %s (peer %s): %v", httpAddr, peerInfo.ID, err) // Using LogInfo for warnings
				continue
			}

			if nodeStatus.NodeType == "validator" {
				// Verify eligibility before adding
				if n.DPoS == nil {
					utils.LogError("DPoS service is nil in Node, cannot verify validator %s", nodeStatus.Address)
				} else {
					eligible, err := consensus.VerifyValidatorEligibility(n.DPoS, nodeStatus.Address)
					if err != nil {
						utils.LogError("Error verifying eligibility for discovered validator %s: %v", nodeStatus.Address, err)
					} else if eligible {
						utils.LogInfo("Confirmed eligible validator %s (%s) via DHT and HTTP status check.", nodeStatus.Address, peerInfo.ID)
						n.AddNodeStatus(*nodeStatus) // Add to known nodes
					} else {
						utils.LogInfo("Discovered validator %s (%s) is not eligible.", nodeStatus.Address, peerInfo.ID)
					}
				}
			} else {
				utils.LogInfo("Peer %s (%s) is not a validator (type: %s).", httpAddr, peerInfo.ID, nodeStatus.NodeType)
			}

		case <-ctx.Done(): // Discovery timeout
			utils.LogInfo("DHT discovery cycle timed out after %v.", discoveryTimeout)
			return
		case <-n.p2pCtx.Done(): // Main context cancelled
			utils.LogInfo("DHT discovery stopped due to p2p context cancellation.")
			return
		}
	}
}

// getNodeStatusFromHttp fetches the NodeStatus from a peer's HTTP /node-status endpoint.
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

// getNodePeers (old HTTP-based peer list fetching - can be deprecated or removed if DHT is primary)
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

// RegisterWithNode now also attempts to bootstrap the DHT using a provided list of LibP2P peers.
func (n *Node) RegisterWithNode(seedNodeAddress string, libp2pBootstrapPeers []string) {
	maxRetries := 5
	baseBackoff := 5 * time.Second

	regRequestBody := NodeRegistrationRequest{
		Address:  n.ID,
		NodeType: n.NodeType,
	}
	requestBodyBytes, err := json.Marshal(regRequestBody)
	if err != nil {
		utils.LogError("Error marshaling registration request: %v", err)
		return
	}

	// HTTP Registration Part
	registrationSuccessful := false
	for attempt := 1; attempt <= maxRetries; attempt++ {
		utils.LogInfo("Attempt %d/%d to register with seed node %s via HTTP", attempt, maxRetries, seedNodeAddress)
		if n.attemptHttpRegistration(seedNodeAddress, requestBodyBytes, attempt) {
			utils.LogInfo("Successfully registered and fetched initial peers from %s via HTTP after %d attempt(s).", seedNodeAddress, attempt)
			registrationSuccessful = true
			break // Exit loop on success
		}
		if attempt < maxRetries {
			backoffDuration := baseBackoff * time.Duration(1<<(attempt-1))
			utils.LogInfo("HTTP registration attempt %d failed. Retrying in %v...", attempt, backoffDuration)
			time.Sleep(backoffDuration)
		}
	}

	if !registrationSuccessful {
		utils.LogError("All %d attempts to register with %s via HTTP failed.", maxRetries, seedNodeAddress)
		// Depending on policy, we might still want to try DHT bootstrapping or return.
		// For now, we'll proceed to DHT bootstrap if LibP2P components are up.
	}

	// DHT Bootstrapping Part (always attempt if host is up, could be independent of HTTP registration outcome)
	if n.Libp2pHost != nil && n.KadDHT != nil {
		utils.LogInfo("Attempting to bootstrap DHT using %d provided peers...", len(libp2pBootstrapPeers))
		if err := n.BootstrapDHT(libp2pBootstrapPeers); err != nil {
			utils.LogInfo("WARN: DHT bootstrapping failed or partially failed: %v", err) // Using LogInfo for warnings
		} else {
			utils.LogInfo("DHT bootstrapping process completed via RegisterWithNode.")
		}
	} else {
		utils.LogInfo("WARN: LibP2P host or DHT not initialized, skipping DHT bootstrap in RegisterWithNode.") // Using LogInfo for warnings
	}
}

// attemptHttpRegistration handles the HTTP part of node registration.
func (n *Node) attemptHttpRegistration(seedNodeAddress string, requestBodyBytes []byte, attempt int) bool {
	regURL := fmt.Sprintf("http://%s/register", seedNodeAddress) // This still points to the old /register, ensure it's what you want.
	req, err := http.NewRequest("POST", regURL, bytes.NewBuffer(requestBodyBytes))
	if err != nil {
		utils.LogError("Attempt %d: Error creating registration request: %v", attempt, err)
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		utils.LogError("Attempt %d: Failed to send registration request to %s: %v", attempt, seedNodeAddress, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		utils.LogError("Attempt %d: Failed to register with node %s: HTTP %d", attempt, seedNodeAddress, resp.StatusCode)
		return false
	}

	utils.LogInfo("Attempt %d: Successfully registered with node %s (status %d)", attempt, seedNodeAddress, resp.StatusCode)

	// 2. Try to fetch peers
	utils.LogInfo("Attempt %d: Registration successful, now fetching peers from %s", attempt, seedNodeAddress)
	peersWithStatus, err := n.getNodePeers(seedNodeAddress)
	if err != nil {
		utils.LogError("Attempt %d: Error fetching peers from %s after registration: %v", attempt, seedNodeAddress, err)
		return false
	}

	// 3. Store peers and update known nodes
	n.discoveredPeersMutex.Lock()
	n.discoveredPeersWithStatus = peersWithStatus
	n.discoveredPeersMutex.Unlock()
	utils.LogInfo("Attempt %d: Successfully fetched %d peers from %s", attempt, len(peersWithStatus), seedNodeAddress)

	for _, peerStatus := range peersWithStatus {
		if peerStatus.Address != n.ID {
			n.AddNodeStatus(*peerStatus)
		}
	}

	return true
}

// AddNode adds a new node with 'unknown' type.
// Prefer AddNodeStatus if NodeType is known.
func (n *Node) AddNode(address string) {
	if address == n.ID { // Do not add self
		return
	}
	// This will log if it's new or updates (though update is less likely here unless type changes from 'unknown')
	n.AddNodeStatus(NodeStatus{Address: address, NodeType: "unknown"})
}

// GetKnownNodes returns a slice of known node addresses.
// Consider if this is still needed widely, or if GetKnownNodeStatuses is preferred.
func (n *Node) GetKnownNodes() []string {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	addresses := make([]string, 0, len(n.knownNodes))
	for addr := range n.knownNodes {
		addresses = append(addresses, addr)
	}
	return addresses
}

// GetDiscoveredPeersWithStatus returns a copy of the discovered peers with their status
// from the last call to getNodePeers in RegisterWithNode.
func (n *Node) GetDiscoveredPeersWithStatus() []*NodeStatus {
	n.discoveredPeersMutex.Lock()
	defer n.discoveredPeersMutex.Unlock()

	if n.discoveredPeersWithStatus == nil {
		return []*NodeStatus{} // Return empty slice if nil
	}

	// Return a copy to avoid external modification
	peersCopy := make([]*NodeStatus, len(n.discoveredPeersWithStatus))
	copy(peersCopy, n.discoveredPeersWithStatus)
	return peersCopy
}

// StopLibp2pServices stops all LibP2P related services including DHT and PubSub.
func (n *Node) StopLibp2pServices() {
	if n.p2pCancel != nil {
		n.p2pCancel() // This should signal all goroutines using p2pCtx to stop
		utils.LogInfo("p2p context cancelled.")
	}

	// Order might matter, try to close subscriptions/topics before the main service
	if n.gossipSub != nil {
		n.gossipSub.Cancel() // Cancel the subscription
		utils.LogInfo("Cancelled gossip subscription.")
	}
	if n.gossipTopic != nil {
		if err := n.gossipTopic.Close(); err != nil { // Close the topic
			utils.LogError("Error closing gossip topic: %v", err)
		} else {
			utils.LogInfo("Closed gossip topic.")
		}
	}
	// PubSubService itself doesn't have a direct Close() method. Its lifecycle is tied to the host & context.

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

// BootstrapDHT connects to a set of bootstrap peers to join the DHT network.
func (n *Node) BootstrapDHT(bootstrapPeerAddrs []string) error {
	if n.Libp2pHost == nil {
		return fmt.Errorf("Libp2pHost not initialized")
	}
	if n.KadDHT == nil {
		return fmt.Errorf("KadDHT not initialized")
	}

	var wg sync.WaitGroup
	successfulConnections := 0
	var firstError error
	var errLock sync.Mutex

	for _, peerAddrStr := range bootstrapPeerAddrs {
		if peerAddrStr == "" {
			continue
		}
		peerMA, err := multiaddr.NewMultiaddr(peerAddrStr)
		if err != nil {
			utils.LogInfo("WARN: Error parsing bootstrap multiaddr %s: %v", peerAddrStr, err)
			continue
		}
		peerInfo, err := peer.AddrInfoFromP2pAddr(peerMA) // Corrected function name
		if err != nil {
			utils.LogInfo("WARN: Error getting peer info from multiaddr %s: %v", peerAddrStr, err)
			continue
		}

		wg.Add(1)
		go func(pi peer.AddrInfo) {
			defer wg.Done()
			if err := n.Libp2pHost.Connect(n.p2pCtx, pi); err != nil { // Use p2pCtx
				utils.LogInfo("WARN: Failed to connect to bootstrap peer %s: %v", pi.ID.String(), err)
				errLock.Lock()
				if firstError == nil {
					firstError = err
				}
				errLock.Unlock()
			} else {
				utils.LogInfo("Successfully connected to bootstrap peer: %s", pi.ID.String())
				successfulConnections++
			}
		}(*peerInfo)
	}
	wg.Wait()

	if successfulConnections == 0 && len(bootstrapPeerAddrs) > 0 && firstError != nil {
		return fmt.Errorf("failed to connect to any bootstrap peers, first error: %w", firstError)
	}
	if successfulConnections < len(bootstrapPeerAddrs) && len(bootstrapPeerAddrs) > 0 {
		utils.LogInfo("WARN: Successfully connected to %d out of %d bootstrap peers.", successfulConnections, len(bootstrapPeerAddrs))
	}

	utils.LogInfo("Bootstrapping DHT process initiated with %d successful connections.", successfulConnections)
	// It might take some time for the DHT to be ready after connecting.
	// For now, we assume it will be ready for advertising/discovery shortly after.
	// A more robust solution might wait for dht.Ready() or similar signal if available.

	// According to libp2p docs, bootstrapping the DHT itself is also important
	if err := n.KadDHT.Bootstrap(n.p2pCtx); err != nil { // Use p2pCtx
		utils.LogError("Failed to bootstrap KadDHT: %v", err)
		return err
	}
	utils.LogInfo("KadDHT bootstrapping process completed.")
	return nil
}

// AdvertiseAsValidator starts advertising the node's presence as a validator.
func (n *Node) AdvertiseAsValidator() {
	if n.routingDiscovery == nil {
		utils.LogError("RoutingDiscovery not initialized, cannot advertise.")
		return
	}
	if n.NodeType != "validator" { // Assuming NodeType is set correctly
		utils.LogInfo("Node type is '%s', not advertising as validator.", n.NodeType)
		return
	}

	utils.LogInfo("Advertising this node (%s) as a validator with service tag: %s", n.Libp2pHost.ID().String(), ValidatorServiceTag)
	// The Advertise call will run in a loop in the background until the context is cancelled.
	discoveryUtil.Advertise(n.p2pCtx, n.routingDiscovery, ValidatorServiceTag) // Use p2pCtx
	utils.LogInfo("Successfully started advertising as validator.")
}

// FindValidatorsDHT attempts to find other validators on the DHT.
// Returns a channel of peer.AddrInfo.
func (n *Node) FindValidatorsDHT() (<-chan peer.AddrInfo, error) {
	if n.routingDiscovery == nil {
		return nil, fmt.Errorf("RoutingDiscovery not initialized, cannot find peers")
	}

	utils.LogInfo("Searching for other validators on the DHT with service tag: %s", ValidatorServiceTag)
	// FindPeers will return a channel that outputs peers as they are found.
	peerChan, err := n.routingDiscovery.FindPeers(n.p2pCtx, ValidatorServiceTag) // Use p2pCtx
	if err != nil {
		utils.LogError("Error starting peer discovery for tag %s: %v", ValidatorServiceTag, err)
		return nil, err
	}
	return peerChan, nil
}

// GetOrInitializeReputation retrieves or creates a reputation entry for a peer.
func (n *Node) GetOrInitializeReputation(address string) *PeerReputation {
	n.reputationMutex.Lock() // Full lock for potential creation and write
	defer n.reputationMutex.Unlock()

	if rep, exists := n.peerReputations[address]; exists {
		return rep
	}
	newRep := &PeerReputation{
		Address:         address,
		ReputationScore: 75.0, // Start with a neutral score
		LastUpdated:     time.Now(),
	}
	n.peerReputations[address] = newRep
	return newRep
}

// RecordConnectionAttempt records the outcome of a connection attempt.
// Note: This is currently not called from BootstrapDHT due to address type mismatch.
// It should be called from places where HTTP connections are made to peers.
func (n *Node) RecordConnectionAttempt(address string, successful bool) {
	rep := n.GetOrInitializeReputation(address)
	n.reputationMutex.Lock() // Lock for write
	defer n.reputationMutex.Unlock()

	if successful {
		rep.SuccessfulConnections++
		rep.LastSeenOnline = time.Now()
	} else {
		rep.FailedConnections++
	}
	rep.LastUpdated = time.Now()
	n.calculateReputationScoreInternal(rep) // Recalculate score
}

// RecordHeartbeatResponse records the outcome of a heartbeat and network latency.
func (n *Node) RecordHeartbeatResponse(address string, successful bool, latency time.Duration) {
	rep := n.GetOrInitializeReputation(address)
	n.reputationMutex.Lock() // Lock for write
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
	n.calculateReputationScoreInternal(rep) // Recalculate score
}

// calculateReputationScoreInternal recalculates a peer's reputation score.
// This is a simplified initial scoring logic.
func (n *Node) calculateReputationScoreInternal(rep *PeerReputation) {
	var score float64 = 50.0 // Base score

	// Connection Success Rate (contributes up to 30 points)
	totalConnections := rep.SuccessfulConnections + rep.FailedConnections
	if totalConnections > 0 {
		connectionRate := float64(rep.SuccessfulConnections) / float64(totalConnections)
		score += connectionRate * 30.0
	} else {
		score += 15.0 // Neutral if no connection data
	}

	// Heartbeat Success Rate (contributes up to 40 points)
	totalHeartbeats := rep.SuccessfulHeartbeats + rep.FailedHeartbeats
	if totalHeartbeats > 0 {
		heartbeatRate := float64(rep.SuccessfulHeartbeats) / float64(totalHeartbeats)
		score += heartbeatRate * 40.0
	} else {
		score += 20.0 // Neutral if no heartbeat data
	}

	// Latency Factor (deducts points for high latency, up to -20)
	if rep.AverageLatency > 0 {
		if rep.AverageLatency > time.Second {
			score -= 20.0
		} else if rep.AverageLatency > 200*time.Millisecond {
			penalty := (float64(rep.AverageLatency-200*time.Millisecond) / float64(time.Second-200*time.Millisecond)) * 20.0
			score -= penalty
		}
	}

	// Normalize score to be between 0 and 100
	if score < 0 { score = 0 }
	if score > 100 { score = 100 }
	rep.ReputationScore = score

	// Refined Penalization/Rehabilitation Logic
	const minInteractionsForPenalization = 10
	totalObservedHeartbeats := rep.SuccessfulHeartbeats + rep.FailedHeartbeats

	if rep.IsTemporarilyPenalized {
		if time.Now().After(rep.PenaltyEndTime) {
			// Penalty time has passed
			if rep.ReputationScore > 40 { // Threshold for rehabilitation
				utils.LogInfo("Peer %s reputation improved (%.2f) and penalty time passed. Lifting penalty.", rep.Address, rep.ReputationScore)
				rep.IsTemporarilyPenalized = false
			} else {
				utils.LogInfo("Peer %s penalty time passed, but score (%.2f) still too low for rehabilitation. Penalty extended.", rep.Address, rep.ReputationScore)
				rep.PenaltyEndTime = time.Now().Add(10 * time.Minute) // Extend penalty
			}
		} else if rep.ReputationScore > 60 { // Proactive rehabilitation if score improves significantly
			utils.LogInfo("Peer %s reputation significantly improved (%.2f) before penalty time fully passed. Lifting penalty early.", rep.Address, rep.ReputationScore)
			rep.IsTemporarilyPenalized = false
		}
	} else {
		// Not currently penalized, check if they should be
		if rep.ReputationScore < 20 && totalObservedHeartbeats >= minInteractionsForPenalization {
			utils.LogInfo("Peer %s has low reputation (%.2f) after %d heartbeats. Temporarily penalizing for 10m.", rep.Address, rep.ReputationScore, totalObservedHeartbeats)
			rep.IsTemporarilyPenalized = true
			rep.PenaltyEndTime = time.Now().Add(10 * time.Minute)
		}
	}
	utils.LogDebug("Calculated reputation for %s: Score %.2f, Penalized: %t", rep.Address, rep.ReputationScore, rep.IsTemporarilyPenalized)
}

// UpdatePeerLastSeen updates the last seen time for a peer.
func (n *Node) UpdatePeerLastSeen(address string) {
	rep := n.GetOrInitializeReputation(address)
	n.reputationMutex.Lock()
	defer n.reputationMutex.Unlock()
	rep.LastSeenOnline = time.Now()
	rep.LastUpdated = time.Now()
}

// GetPeerReputationScore retrieves the reputation score for a given peer address.
func (n *Node) GetPeerReputationScore(address string) (float64, bool) {
	n.reputationMutex.RLock()
	defer n.reputationMutex.RUnlock()
	if rep, exists := n.peerReputations[address]; exists {
		return rep.ReputationScore, true
	}
	return 0, false // Default score or indicate not found
}


// StartValidatorMonitoring starts a goroutine to periodically re-check known validators.
func (n *Node) StartValidatorMonitoring() {
	if n.DPoS == nil {
		utils.LogInfo("WARN: DPoS instance not available in Node, cannot start validator monitoring.") // LogInfo for warnings
		return
	}
	utils.LogInfo("Starting periodic monitoring of known validators.")
	go n.periodicallyRecheckValidators()
}

// periodicallyRecheckValidators is a goroutine that periodically calls recheckKnownValidators.
func (n *Node) periodicallyRecheckValidators() {
	ticker := time.NewTicker(ValidatorRecheckInterval)
	defer ticker.Stop()
	defer utils.LogInfo("Exiting periodicallyRecheckValidators goroutine.")

	for {
		select {
		case <-n.p2pCtx.Done(): // Use existing p2pCtx for cancellation
			utils.LogInfo("Stopping periodic validator re-check due to context cancellation.")
			return
		case <-ticker.C:
			utils.LogInfo("Performing periodic re-check of known validators...")
			n.recheckKnownValidators()
		}
	}
}

// recheckKnownValidators iterates through known validators and removes them if they are no longer eligible.
func (n *Node) recheckKnownValidators() {
	if n.DPoS == nil { // Double check DPoS, though StartValidatorMonitoring should prevent this.
		utils.LogError("DPoS instance not available in recheckKnownValidators.")
		return
	}
	currentKnownStatuses := n.GetKnownNodeStatuses() // Gets a copy of current statuses
	validatorsToRemoveDetails := []NodeStatus{}

	for _, status := range currentKnownStatuses {
		if status.NodeType == "validator" {
			eligible, err := consensus.VerifyValidatorEligibility(n.DPoS, status.Address)
			if err != nil {
				utils.LogError("Error re-checking eligibility for validator %s: %v. Assuming ineligible for now.", status.Address, err)
				eligible = false
			}

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
			}
		}
		n.mutex.Unlock()

		// Publish REMOVE events after releasing the lock
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


// publishValidatorEvent constructs and publishes a ValidatorGossipMessage for ADD or REMOVE events.
func (n *Node) publishValidatorEvent(eventType GossipEventType, validator NodeStatus) {
	if n.gossipTopic == nil || n.PubSubService == nil {
		utils.LogInfo("WARN: Cannot publish validator event: PubSub or topic not initialized.") // LogInfo for warnings
		return
	}
	if eventType != GossipEventAddValidator && eventType != GossipEventRemoveValidator {
		utils.LogInfo("WARN: Invalid event type for single validator event: %s", eventType) // LogInfo for warnings
		return
	}

	gossipMsg := ValidatorGossipMessage{
		SenderNodeID: n.Libp2pHost.ID().String(),
		EventType:    eventType,
		Validators:   []NodeStatus{validator},
	}
	msgBytes, err := json.Marshal(gossipMsg)
	if err != nil {
		utils.LogError("Failed to marshal validator event message (%s for %s): %v", eventType, validator.Address, err)
		return
	}

	utils.LogInfo("Publishing validator event: %s for %s", eventType, validator.Address)
	if err := n.gossipTopic.Publish(n.p2pCtx, msgBytes); err != nil {
		utils.LogError("Failed to publish validator event message (%s for %s): %v", eventType, validator.Address, err)
	}
}

// StartGossip initializes and starts the gossip protocol participation.
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
		// topic.Close() // Should we close topic if subscribe fails? Depends on desired retry logic.
		return err
	}
	n.gossipSub = sub

	utils.LogInfo("Successfully joined and subscribed to gossip topic: %s", GossipSubValidatorTopic)

	go n.processGossipMessages()
	go n.periodicallyPublishValidators()

	return nil
}

// processGossipMessages handles incoming messages from the gossip subscription.
func (n *Node) processGossipMessages() {
	if n.gossipSub == nil {
		utils.LogError("Gossip subscription is nil, cannot process messages.")
		return
	}
	defer utils.LogInfo("Exiting processGossipMessages goroutine.") // Ensure we know when it exits

	for {
		select {
		case <-n.p2pCtx.Done(): // Use existing context for cancellation
			utils.LogInfo("Stopping gossip message processing due to context cancellation.")
			return
		default:
			// Set a timeout for msg.Next to prevent indefinite blocking if context is not checked often enough
			// or if there's an issue with pubsub not respecting parent context immediately.
			ctx, cancel := context.WithTimeout(n.p2pCtx, 5*time.Second)
			msg, err := n.gossipSub.Next(ctx)
			cancel() // Release context resources promptly

			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					// Check if the main p2pCtx is done; if so, exit. Otherwise, it's just the Next timeout.
					if n.p2pCtx.Err() != nil {
						utils.LogInfo("Gossip subscription context done, exiting message processing loop.")
						return
					}
					// utils.LogDebug("Gossip sub.Next timed out or was cancelled, continuing loop.")
					continue
				}
				utils.LogError("Error receiving gossip message: %v", err)
				// If context is cancelled, loop will break on next select or here.
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

			switch gossipMsg.EventType {
			case GossipEventAddValidator:
				if len(gossipMsg.Validators) == 1 {
					validatorNodeStatus := gossipMsg.Validators[0]
					if validatorNodeStatus.Address == n.ID { continue }
					if validatorNodeStatus.NodeType != "validator" {
						utils.LogInfo("WARN: Received ADD_VALIDATOR event for non-validator type %s from %s. Skipping.", validatorNodeStatus.NodeType, gossipMsg.SenderNodeID)
						continue
					}
					utils.LogInfo("Received ADD_VALIDATOR event for %s from %s.", validatorNodeStatus.Address, gossipMsg.SenderNodeID)

					// Check if already known to avoid redundant eligibility checks if AddNodeStatus is smart enough
					// For now, always verify, AddNodeStatus will handle idempotency for storage.
					if n.DPoS == nil {
						utils.LogError("DPoS service is nil in Node, cannot verify ADD_VALIDATOR for %s", validatorNodeStatus.Address)
						continue
					}
					eligible, err := consensus.VerifyValidatorEligibility(n.DPoS, validatorNodeStatus.Address)
					if err != nil {
						utils.LogError("Error verifying eligibility for ADD_VALIDATOR %s: %v", validatorNodeStatus.Address, err)
						continue
					}
					if eligible {
						n.AddNodeStatus(validatorNodeStatus) // AddNodeStatus might trigger its own publish if it's new to this node.
					} else {
						utils.LogInfo("ADD_VALIDATOR event for %s is not eligible.", validatorNodeStatus.Address)
					}
				} else {
					utils.LogInfo("WARN: ADD_VALIDATOR event from %s has incorrect validator count: %d", gossipMsg.SenderNodeID, len(gossipMsg.Validators))
				}

			case GossipEventRemoveValidator:
				if len(gossipMsg.Validators) == 1 {
					validatorNodeStatus := gossipMsg.Validators[0]
					if validatorNodeStatus.Address == n.ID { continue }
					utils.LogInfo("Received REMOVE_VALIDATOR event for %s from %s.", validatorNodeStatus.Address, gossipMsg.SenderNodeID)
					n.mutex.Lock()
					if knownStatus, exists := n.knownNodes[validatorNodeStatus.Address]; exists {
						if knownStatus.NodeType == "validator" { // Make sure we are removing a validator
							delete(n.knownNodes, validatorNodeStatus.Address)
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
				for _, validatorNodeStatus := range gossipMsg.Validators {
					if validatorNodeStatus.Address == n.ID { continue }
					if validatorNodeStatus.NodeType != "validator" { continue }

					if n.DPoS == nil {
						utils.LogError("DPoS service is nil in Node, cannot verify validator %s from FULL_SYNC", validatorNodeStatus.Address)
						continue
					}
					eligible, err := consensus.VerifyValidatorEligibility(n.DPoS, validatorNodeStatus.Address)
					if err != nil {
						utils.LogError("Error verifying eligibility for validator %s from FULL_SYNC: %v", validatorNodeStatus.Address, err)
						continue
					}
					if eligible {
						n.AddNodeStatus(validatorNodeStatus) // AddNodeStatus handles add or update
					} else {
						utils.LogInfo("Validator %s from FULL_SYNC is not eligible.", validatorNodeStatus.Address)
						// If this node was previously known and eligible, this full sync might implicitly mean it's no longer a validator
						// or the sender has a more up-to-date (negative) eligibility view.
						// For now, we only add/update eligible ones. We don't remove based on absence in a FULL_SYNC from a single peer.
						// Removals are handled by this node's own periodic recheck or direct REMOVE_VALIDATOR events.
					}
				}
			default:
				utils.LogInfo("WARN: Received gossip message with unknown event type: %s from %s", gossipMsg.EventType, gossipMsg.SenderNodeID)
			}
		}
	}
}

// periodicallyPublishValidators publishes the node's list of known validators.
func (n *Node) periodicallyPublishValidators() {
	if n.gossipTopic == nil {
		utils.LogError("Gossip topic is nil, cannot publish.")
		return
	}
	ticker := time.NewTicker(GossipInterval)
	defer ticker.Stop()
	defer utils.LogInfo("Exiting periodicallyPublishValidators goroutine.") // Ensure we know when it exits

	for {
		select {
		case <-n.p2pCtx.Done(): // Use existing context for cancellation
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
				// utils.LogDebug("No validators to publish via gossip at this time.")
				continue
			}

			gossipMsg := ValidatorGossipMessage{
				SenderNodeID: n.Libp2pHost.ID().String(),
				EventType:    GossipEventFullSync, // Changed
				Validators:   validatorStatuses,
			}

			msgBytes, err := json.Marshal(gossipMsg)
			if err != nil {
				utils.LogError("Failed to marshal validator gossip message: %v", err)
				continue
			}

			utils.LogInfo("Publishing FULL_SYNC with %d validators via gossip.", len(validatorStatuses)) // Changed log
			if err := n.gossipTopic.Publish(n.p2pCtx, msgBytes); err != nil { // Use p2pCtx
				utils.LogError("Failed to publish FULL_SYNC validator gossip message: %v", err)
			}
		}
	}
}
