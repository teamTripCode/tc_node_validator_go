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
	KnownNodes []string
	PrivateKey string
	mutex      sync.Mutex
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
		Port:       port,
		KnownNodes: []string{},
		mutex:      sync.Mutex{},
	}
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

		peers, err := n.getNodePeers(node)
		if err != nil {
			utils.LogError("Error obteniendo peers de %s: %v", node, err)
			continue
		}

		for _, peer := range peers {
			if !utils.Contains(n.KnownNodes, peer) && peer != n.ID {
				n.AddNode(peer)
				utils.LogInfo("Nodo descubierto: %s", peer)
			}
		}
	}
}

// En getNodePeers
func (n *Node) getNodePeers(node string) ([]string, error) {
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

	// Extraer solo las direcciones
	var addresses []string
	for _, p := range peers {
		addresses = append(addresses, p.Address)
	}
	return addresses, nil
}

func (n *Node) RegisterWithNode(node string) {
	// Crea el cuerpo de la solicitud
	reqBody := NodeRegistrationRequest{
		Address:  n.ID,       // Ej: "localhost:3001"
		NodeType: n.NodeType, // Ej: "validator"
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		utils.LogError("Error marshaling request: %v", err)
		return
	}

	// Crea la solicitud HTTP
	url := fmt.Sprintf("http://%s/register", node)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		utils.LogError("Error creating request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json") // Requerido por el semilla

	// Envía la solicitud
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		utils.LogError("Failed to register with node %s: %v", node, err)
		return
	}
	defer resp.Body.Close()

	// Maneja la respuesta
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
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
