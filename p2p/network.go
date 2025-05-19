// p2p/network.go
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
