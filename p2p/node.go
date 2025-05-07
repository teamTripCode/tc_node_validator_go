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

// Node represents a node in the blockchain network
type Node struct {
	ID         string
	Port       int
	KnownNodes []string
	mutex      sync.Mutex
}

// NewNode creates a new node instance
func NewNode(port int) *Node {
	nodeID := fmt.Sprintf("localhost:%d", port)
	return &Node{
		ID:         nodeID,
		Port:       port,
		KnownNodes: []string{"localhost:3000", "localhost:3001"},
		mutex:      sync.Mutex{},
	}
}

// StartHeartbeat sends regular status updates
func (n *Node) StartHeartbeat() {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			status := map[string]interface{}{
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
	for _, node := range n.GetKnownNodes() {
		if node != n.ID {
			utils.LogInfo("Attempting to connect to node %s", node)
			resp, err := http.Get(fmt.Sprintf("http://%s/nodes", node))
			if err != nil {
				utils.LogError("Could not connect to node %s: %v", node, err)
				continue
			}
			defer resp.Body.Close()

			var nodes []string
			if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
				utils.LogError("Error decoding nodes from %s: %v", node, err)
				continue
			}

			// Register the new node
			n.RegisterWithNode(node)

			// Add new nodes to our known nodes
			n.mutex.Lock()
			for _, newNode := range nodes {
				if newNode != n.ID && !utils.Contains(n.KnownNodes, newNode) {
					n.KnownNodes = append(n.KnownNodes, newNode)
					utils.LogInfo("Added new node to known nodes: %s", newNode)
				}
			}
			n.mutex.Unlock()
			utils.LogInfo("Successfully connected to network via node %s", node)
		}
	}
}

// RegisterWithNode registers the current node with another node
func (n *Node) RegisterWithNode(node string) {
	nodeData, _ := json.Marshal(n.ID)
	utils.LogDebug("Registering with node %s", node)

	resp, err := http.Post(fmt.Sprintf("http://%s/register", node), "application/json", bytes.NewBuffer(nodeData))
	if err != nil {
		utils.LogError("Failed to register with node %s: %v", node, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		utils.LogInfo("Successfully registered with node %s", node)
	} else {
		utils.LogError("Failed to register with node %s: HTTP %d", node, resp.StatusCode)
	}
}

// AddNode adds a new node to the known nodes list
func (n *Node) AddNode(node string) bool {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	if !utils.Contains(n.KnownNodes, node) {
		n.KnownNodes = append(n.KnownNodes, node)
		utils.LogInfo("New node added: %s", node)
		return true
	}

	utils.LogDebug("Node already known: %s", node)
	return false
}

// GetKnownNodes returns a copy of the known nodes list
func (n *Node) GetKnownNodes() []string {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	// Return a copy to avoid concurrency issues
	nodesCopy := make([]string, len(n.KnownNodes))
	copy(nodesCopy, n.KnownNodes)
	return nodesCopy
}
