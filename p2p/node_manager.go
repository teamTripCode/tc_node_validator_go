package p2p

import (
	"log"
	"net/http"
	"time"
)

// NodeManager is an interface or struct that manages known nodes
type NodeManagerInterface interface {
	AddNode(node string) bool
	checkSingleAttempt(address string) bool
}

// NodeManager manages nodes in the P2P network.
type NodeManager struct {
	logger *log.Logger
	// add other fields as needed
}

func (nm *NodeManager) CheckNodeStatus(address string) bool {
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		if nm.checkSingleAttempt(address) {
			return true
		}
		time.Sleep(1 * time.Second)
	}
	return false
}

func (nm *NodeManager) checkSingleAttempt(address string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + address + "/ping")

	if err != nil {
		nm.logger.Printf("Intento fallido para nodo %s: %v", address, err)
		return false
	}

	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
