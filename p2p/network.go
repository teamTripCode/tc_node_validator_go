// p2p/network.go
package p2p

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"tripcodechain_go/blockchain"
	"tripcodechain_go/utils"
)

// ProcessTxMempool creates new blocks from pending transactions
func (s *Server) ProcessTxMempool() {
	pendingTxs := s.TxMempool.GetPendingItems()
	if len(pendingTxs) == 0 {
		utils.LogDebug("No pending transactions to process")
		return
	}

	utils.LogInfo("Processing %d pending transactions", len(pendingTxs))

	// Create a slice of transactions from the pending items
	transactions := make([]*blockchain.Transaction, 0)
	for _, item := range pendingTxs {
		tx, ok := item.(*blockchain.Transaction)
		if ok {
			transactions = append(transactions, tx)
		}
	}

	// Create a new block
	block := s.TxChain.CreateBlock()
	block.Transactions = transactions
	block.TotalFees = s.calculateTotalFees(transactions)

	// Mine the block and add it to the chain
	startTime := time.Now()
	block.ForgeBlock(s.Node.ID)
	endTime := time.Now()

	utils.LogInfo("Block mined in %v seconds", endTime.Sub(startTime).Seconds())
	s.TxChain.AddBlock(block)
	utils.LogInfo("Added new block #%d with %d transactions", block.Index, len(transactions))

	// Remove processed transactions from the mempool
	s.TxMempool.RemoveProcessedItems(pendingTxs)

	// Broadcast the new block to other nodes
	s.BroadcastNewBlock(block, "tx")
}

// ProcessCriticalMempool creates new blocks from pending critical processes
func (s *Server) ProcessCriticalMempool() {
	pendingCritical := s.CriticalMempool.GetPendingItems()
	if len(pendingCritical) == 0 {
		utils.LogDebug("No pending critical processes to process")
		return
	}

	utils.LogInfo("Processing %d pending critical processes", len(pendingCritical))

	// Create a slice of critical processes from the pending items
	criticalProcesses := make([]*blockchain.CriticalProcess, 0)
	for _, item := range pendingCritical {
		cp, ok := item.(*blockchain.CriticalProcess)
		if ok {
			criticalProcesses = append(criticalProcesses, cp)
		}
	}

	// Create a new block
	block := s.CriticalChain.CreateBlock()
	block.CriticalProcesses = criticalProcesses

	// Mine the block and add it to the chain
	startTime := time.Now()
	block.ForgeBlock(s.Node.ID)
	endTime := time.Now()

	utils.LogInfo("Block mined in %v seconds", endTime.Sub(startTime).Seconds())
	s.CriticalChain.AddBlock(block)
	utils.LogInfo("Added new block #%d with %d critical processes", block.Index, len(criticalProcesses))

	// Remove processed critical processes from the mempool
	s.CriticalMempool.RemoveProcessedItems(pendingCritical)

	// Broadcast the new block to other nodes
	s.BroadcastNewBlock(block, "critical")
}

// BroadcastNewBlock sends a new block to all known nodes
func (s *Server) BroadcastNewBlock(block *blockchain.Block, chainType string) {
	blockData, _ := json.Marshal(block)

	for _, node := range s.Node.GetKnownNodes() {
		if node != s.Node.ID {
			url := fmt.Sprintf("http://%s/block/%s", node, chainType)
			utils.LogDebug("Broadcasting new block to %s", node)

			_, err := http.Post(url, "application/json", bytes.NewBuffer(blockData))
			if err != nil {
				utils.LogError("Error broadcasting block to %s: %v", node, err)
			} else {
				utils.LogInfo("Successfully broadcast block to %s", node)
			}
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
