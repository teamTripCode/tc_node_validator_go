// p2p/node.go
package p2p

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"tripcodechain_go/utils"
)

// NodeInfo represents information about a peer node in the network.
type NodeInfo struct {
	Address  string `json:"address"`
	LastSeen int64  `json:"lastSeen"`
	// IsActive bool   `json:"isActive"` // Will be determined dynamically or by lastSeen
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
func NewNode(port int) *Node {
	nodeID := fmt.Sprintf("localhost:%d", port)
	return &Node{
		ID:         nodeID,
		NodeType:   "unknown", // Default NodeType, will be set by caller or registration
		Port:       port,
		knownNodes: make(map[string]NodeStatus), // Initialize map
		mutex:      sync.Mutex{},
		// discoveredPeersWithStatus is initially nil, discoveredPeersMutex is zero value
	}
}

// AddNodeStatus adds or updates a node's status in the knownNodes map.
func (n *Node) AddNodeStatus(status NodeStatus) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	if status.Address == n.ID { // Do not add self
		return
	}

	existingStatus, exists := n.knownNodes[status.Address]
	if exists {
		// Update if different, especially NodeType
		if existingStatus.NodeType != status.NodeType {
			utils.LogInfo("Updating node status for %s: NodeType changed from %s to %s", status.Address, existingStatus.NodeType, status.NodeType)
			n.knownNodes[status.Address] = status
		} else {
			utils.LogDebug("Node status for %s already up-to-date (NodeType: %s).", status.Address, status.NodeType)
		}
	} else {
		utils.LogInfo("Adding new node with status: %s (NodeType: %s)", status.Address, status.NodeType)
		n.knownNodes[status.Address] = status
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

// StartHeartbeat sends regular status updates
func (n *Node) StartHeartbeat() {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			status := map[string]any{
				"nodeId":     n.ID,
				"timestamp":  time.Now().UTC().Format(time.RFC3339),
				"knownNodes": n.GetKnownNodes(),
			}
			utils.LogDebug("Heartbeat: %v", status)

			// Try to ping other nodes
			for _, node := range n.GetKnownNodes() {
				if node != n.ID {
					n.PingNode(node)
				}
			}
		}
	}
}

// PingNode sends a simple ping to another node
func (n *Node) PingNode(node string) {
	url := fmt.Sprintf("http://%s/ping", node)
	client := &http.Client{Timeout: 2 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		utils.LogDebug("Node %s seems to be offline: %v", node, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		utils.LogDebug("Node %s is online", node)
	}
}

// DiscoverNodes attempts to connect to known nodes
func (n *Node) DiscoverNodes() {
	knownNodes := n.GetKnownNodes()

	for _, node := range knownNodes {
		if node == n.ID {
			continue
		}

		// getNodePeers returns []*NodeStatus. We should use AddNodeStatus.
		peersStatusList, err := n.getNodePeers(node)
		if err != nil {
			utils.LogError("Error getting peers from %s: %v", node, err)
			continue
		}

		for _, peerStatus := range peersStatusList {
			if peerStatus.Address != n.ID { // Don't add self
				// AddNodeStatus handles logging and deciding if it's new or an update
				n.AddNodeStatus(*peerStatus)
			}
		}
	}
}

// En getNodePeers
func (n *Node) getNodePeers(node string) ([]*NodeStatus, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s/nodes", node))
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

func (n *Node) RegisterWithNode(seedNodeAddress string) {
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

	for attempt := 1; attempt <= maxRetries; attempt++ {
		utils.LogInfo("Attempt %d/%d to register with seed node %s", attempt, maxRetries, seedNodeAddress)

		success := n.attemptRegistration(seedNodeAddress, requestBodyBytes, attempt)
		if success {
			utils.LogInfo("Successfully registered and fetched peers from %s after %d attempt(s).", seedNodeAddress, attempt)
			return
		}

		// If not the last attempt, wait with exponential backoff
		if attempt < maxRetries {
			backoffDuration := baseBackoff * time.Duration(1<<(attempt-1))
			utils.LogInfo("Attempt %d failed. Retrying in %v...", attempt, backoffDuration)
			time.Sleep(backoffDuration)
		}
	}

	utils.LogError("All %d attempts to register with and fetch peers from %s failed.", maxRetries, seedNodeAddress)
}

func (n *Node) attemptRegistration(seedNodeAddress string, requestBodyBytes []byte, attempt int) bool {
	// 1. Try to register
	regURL := fmt.Sprintf("http://%s/register", seedNodeAddress)
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
