package p2p

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"tripcodechain_go/blockchain"
	"tripcodechain_go/utils"
)

type BlockMessage struct {
	Block     *blockchain.Block
	Signature string
	NodeID    string
}

func (s *Server) BroadcastNewBlock(block *blockchain.Block, chainType string) {
	message := BlockMessage{
		Block:  block,
		NodeID: s.Node.ID,
	}

	// Firmar el bloque
	message.Signature = s.signBlock(block.Hash, s.Node.PrivateKey)

	blockData, _ := json.Marshal(block)

	for _, node := range s.Node.GetKnownNodes() {
		if node != s.Node.ID {
			go func(n string) {
				url := fmt.Sprintf("http://%s/block/%s", node, chainType)
				req, _ := http.NewRequest("POST", url, bytes.NewBuffer(blockData))
				req.Header.Set("X-Node-ID", s.Node.ID)
				req.Header.Set("X-Signature", message.Signature)

				client := &http.Client{Timeout: 5 * time.Second}
				resp, err := client.Do(req)
				if err == nil && resp.StatusCode == http.StatusOK {
					utils.LogInfo("Block accepted by %s", n)
				}
			}(node)
		}
	}
}

// BroadcastTransaction sends a transaction to all known nodes
func (s *Server) BroadcastTransaction(tx *blockchain.Transaction) {
	txData, _ := json.Marshal(tx)

	for _, node := range s.Node.GetKnownNodes() {
		if node != s.Node.ID {
			utils.LogDebug("Broadcasting transaction %s to %s", tx.ProcessID, node)
			_, err := http.Post(fmt.Sprintf("http://%s/tx", node), "application/json", bytes.NewBuffer(txData))
			if err != nil {
				utils.LogError("Error broadcasting transaction to %s: %v", node, err)
			} else {
				utils.LogDebug("Transaction successfully broadcast to %s", node)
			}
		}
	}
}

// BroadcastCriticalProcess sends a critical process to all known nodes
func (s *Server) BroadcastCriticalProcess(cp *blockchain.CriticalProcess) {
	cpData, _ := json.Marshal(cp)

	for _, node := range s.Node.GetKnownNodes() {
		if node != s.Node.ID {
			utils.LogDebug("Broadcasting critical process %s to %s", cp.ProcessID, node)
			_, err := http.Post(fmt.Sprintf("http://%s/critical", node), "application/json", bytes.NewBuffer(cpData))
			if err != nil {
				utils.LogError("Error broadcasting critical process to %s: %v", node, err)
			} else {
				utils.LogDebug("Critical process successfully broadcast to %s", node)
			}
		}
	}
}

// calculateTotalFees calculates the total fees for a set of transactions
func (s *Server) calculateTotalFees(transactions []*blockchain.Transaction) float64 {
	var totalFees float64
	for _, tx := range transactions {
		totalFees += float64(tx.GasLimit) * 0.0001 // Simple fee calculation
	}
	return totalFees
}

func (s *Server) SyncChains() {
	utils.LogInfo("Starting blockchain synchronization...")

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		s.syncChain(s.TxChain, "tx")
	}()

	go func() {
		defer wg.Done()
		s.syncChain(s.CriticalChain, "critical")
	}()

	wg.Wait()
	utils.LogInfo("Blockchain synchronization completed")
}

func (s *Server) syncChain(chain *blockchain.Blockchain, chainType string) {
	var bestChain []*blockchain.Block
	maxLength := chain.GetLength()

	for _, node := range s.Node.GetKnownNodes() {
		if node == s.Node.ID {
			continue
		}

		url := fmt.Sprintf("http://%s/chain/%s", node, chainType)
		resp, err := http.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		var remoteChain []*blockchain.Block
		if err := json.NewDecoder(resp.Body).Decode(&remoteChain); err != nil {
			continue
		}

		isValid, err := chain.IsValidChain(remoteChain)
		if len(remoteChain) > maxLength && err == nil && isValid {
			maxLength = len(remoteChain)
			bestChain = remoteChain
		}
	}

	if bestChain != nil {
		utils.LogInfo("Replacing %s chain with new chain (length: %d)", chainType, maxLength)
		chain.ReplaceChain(bestChain)
	}
}

// signMessage is a placeholder for signing an arbitrary message ID.
// In a real scenario, this would involve cryptographic signing of the message content or its hash.
func (s *Server) signMessage(messageID string) (string, string) {
	// Placeholder: Using node's ID and a dummy signature based on the messageID.
	// This should be replaced with actual cryptographic signing using s.Node.PrivateKey
	// For example:
	// signaturePayload := []byte(messageID) // Or a hash of the message content
	// signature, err := crypto.Sign(signaturePayload, s.Node.PrivateKey)
	// if err != nil {
	//     utils.LogError("Failed to sign message %s: %v", messageID, err)
	//     return s.Node.ID, "" // Return empty signature on error
	// }
	// return s.Node.ID, base64.StdEncoding.EncodeToString(signature)
	return s.Node.ID, "dummy-signature-for-message-" + messageID
}

// BroadcastMCPQuery sends an MCPQuery to all known peers except itself.
func (s *Server) BroadcastMCPQuery(query *MCPQuery) {
	if query == nil {
		utils.LogInfo("BroadcastMCPQuery: query is nil, cannot proceed.")
		return
	}

	// Ensure the query has an OriginNodeID, if not, set it to this node's ID.
	if query.OriginNodeID == "" {
		query.OriginNodeID = s.Node.ID
	}

	// Populate signature if not already present.
	// The signature should ideally be created by the true originator of the query.
	if query.Signature.NodeID == "" || query.Signature.Signature == "" {
		nodeID, sig := s.signMessage(query.QueryID) // Use QueryID for context in signing
		query.Signature = MCPMessageSignature{NodeID: nodeID, Signature: sig}
	}

	queryJSON, err := json.Marshal(query)
	if err != nil {
		utils.LogError("Error marshalling MCPQuery (QueryID: %s): %v", query.QueryID, err)
		return
	}

	// Iterate over active nodes from NodeManager, excluding self
	activeNodeInfos := s.NodeMgr.GetActiveNodes() // Use the existing method
	nodesToBroadcast := make(map[string]string)
	for _, nodeInfo := range activeNodeInfos {
		if nodeInfo.Address != s.Node.ID { // Compare nodeInfo.Address with this server's Node.ID
			nodesToBroadcast[nodeInfo.Address] = nodeInfo.Address // Store address, key by address
		}
	}

	if len(nodesToBroadcast) == 0 {
		utils.LogInfo("BroadcastMCPQuery: No other active nodes to broadcast MCPQuery (QueryID: %s) to.", query.QueryID)
		return
	}

	utils.LogInfo("Broadcasting MCPQuery (QueryID: %s, Origin: %s) to %d nodes", query.QueryID, query.OriginNodeID, len(nodesToBroadcast))

	// Iterating over nodeAddress which is now both key and value from nodesToBroadcast
	for _, nodeAddress := range nodesToBroadcast {
		go func(addr string) {
			fullAddress := fmt.Sprintf("http://%s/mcp/query", addr)
			req, err := http.NewRequest("POST", fullAddress, bytes.NewBuffer(queryJSON))
			if err != nil {
				utils.LogError("Error creating request for MCPQuery to %s: %v", addr, err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			// Set headers from the sender's perspective (this node)
			req.Header.Set("X-Node-ID", s.Node.ID)
			// The signature in the header should be from the perspective of the sender of this HTTP request,
			// which might be different from query.Signature if this node is relaying.
			// For now, let's assume the signature in the query object is the one to be verified by the recipient.
			// If X-Signature is for transport layer auth, a new signature might be needed.
			// Using query.Signature.Signature for now.
			req.Header.Set("X-Signature", query.Signature.Signature)

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				utils.LogError("Error sending MCPQuery (QueryID: %s) to %s: %v", query.QueryID, addr, err)
				// s.NodeMgr.MarkNodeAsInactive(addr) // Mark as inactive on failure - method doesn't exist
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				utils.LogInfo("Failed to send MCPQuery (QueryID: %s) to %s, status: %s", query.QueryID, addr, resp.Status)
			} else {
				utils.LogDebug("MCPQuery (QueryID: %s) successfully sent to %s", query.QueryID, addr)
			}
		}(nodeAddress) // Pass nodeAddress (which is addr in the goroutine)
	}
}

// SendMCPResponse sends an MCPResponse to a specific target node address.
func (s *Server) SendMCPResponse(response *MCPResponse, targetNodeAddress string) {
	if response == nil {
		utils.LogInfo("SendMCPResponse: response is nil, cannot proceed.")
		return
	}
	if targetNodeAddress == "" {
		utils.LogInfo("SendMCPResponse: targetNodeAddress is empty for QueryID: %s", response.QueryID)
		return
	}

	// Ensure the response has a ResponderNodeID, if not, set it to this node's ID.
	if response.ResponderNodeID == "" {
		response.ResponderNodeID = s.Node.ID
	}

	// Populate signature if not already present.
	// This signature is from the responder (this node).
	if response.Signature.NodeID == "" || response.Signature.Signature == "" {
		nodeID, sig := s.signMessage(response.QueryID) // Use QueryID for context in signing
		response.Signature = MCPMessageSignature{NodeID: nodeID, Signature: sig}
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		utils.LogError("Error marshalling MCPResponse (QueryID: %s): %v", response.QueryID, err)
		return
	}

	fullAddress := fmt.Sprintf("http://%s/mcp/response", targetNodeAddress)
	req, err := http.NewRequest("POST", fullAddress, bytes.NewBuffer(responseJSON))
	if err != nil {
		utils.LogError("Error creating request for MCPResponse (QueryID: %s) to %s: %v", response.QueryID, targetNodeAddress, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	// Set headers from the sender's perspective (this node, the responder)
	req.Header.Set("X-Node-ID", s.Node.ID)
	req.Header.Set("X-Signature", response.Signature.Signature) // Signature of the MCPResponse payload

	utils.LogInfo("Sending MCPResponse (QueryID: %s, Responder: %s) to %s", response.QueryID, response.ResponderNodeID, targetNodeAddress)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		utils.LogError("Error sending MCPResponse (QueryID: %s) to %s: %v", response.QueryID, targetNodeAddress, err)
		// s.NodeMgr.MarkNodeAsInactive(targetNodeAddress) // Method doesn't exist
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		utils.LogInfo("Failed to send MCPResponse (QueryID: %s) to %s, status: %s", response.QueryID, targetNodeAddress, resp.Status)
	} else {
		utils.LogDebug("MCPResponse (QueryID: %s) successfully sent to %s", response.QueryID, targetNodeAddress)
	}
}
