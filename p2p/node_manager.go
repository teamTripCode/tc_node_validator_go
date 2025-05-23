package p2p

import (
	"sync"
	"time"
	"tripcodechain_go/utils" // Assuming utils.LogInfo is available
	// NodeInfo is in node.go, so it will be imported if needed by other files,
	// but not directly needed for the definition of NodeManager itself here.
)

// NodeManager manages a list of known peer nodes in the network.
type NodeManager struct {
	Nodes map[string]NodeInfo // Key is node address (e.g., "ip:port")
	mu    sync.Mutex
}

// NewNodeManager creates and returns a new NodeManager instance.
func NewNodeManager() *NodeManager {
	return &NodeManager{
		Nodes: make(map[string]NodeInfo),
	}
}

// AddNode adds a new node to the NodeManager's list or updates its LastSeen timestamp if it already exists.
func (nm *NodeManager) AddNode(address string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	nm.Nodes[address] = NodeInfo{
		Address:  address,
		LastSeen: time.Now().Unix(),
	}

	utils.LogInfo("Node registered/updated: %s", address)
	return nil
}

// GetActiveNodes returns a slice of all nodes currently in the NodeManager.
// For MVP, "active" means all registered nodes. Filtering by LastSeen can be added later.
func (nm *NodeManager) GetActiveNodes() []NodeInfo {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	activeNodes := make([]NodeInfo, 0, len(nm.Nodes))
	for _, nodeInfo := range nm.Nodes {
		activeNodes = append(activeNodes, nodeInfo)
	}

	return activeNodes
}
