// p2p/handlers.go
package p2p

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"tripcodechain_go/blockchain"
	"tripcodechain_go/utils"
)

// GetNodesHandler returns the list of known nodes
func (s *Server) GetNodesHandler(w http.ResponseWriter, r *http.Request) {
	utils.LogDebug("Received request for nodes list from %s", r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.Node.GetKnownNodes())
}

// RegisterNodeHandler adds a new node to the known nodes list
func (s *Server) RegisterNodeHandler(w http.ResponseWriter, r *http.Request) {
	var node string
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		utils.LogError("Error decoding registration request: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.Node.AddNode(node)
	w.WriteHeader(http.StatusCreated)
}

// PingHandler responds to ping requests
func (s *Server) PingHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Node %s is alive", s.Node.ID)
	utils.LogDebug("Received ping from %s", r.RemoteAddr)
}

// TransactionHandler adds a new transaction to the mempool
func (s *Server) TransactionHandler(w http.ResponseWriter, r *http.Request) {
	var tx blockchain.Transaction
	if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
		utils.LogError("Error decoding transaction: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Set timestamp if not provided
	if tx.Timestamp == "" {
		tx.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	// Add to mempool
	s.TxMempool.AddItem(&tx)
	utils.LogInfo("Transaction added to mempool: %s", tx.ProcessID)

	// Broadcast to other nodes
	go s.BroadcastTransaction(&tx)

	w.WriteHeader(http.StatusCreated)
}

// BatchTransactionHandler adds multiple transactions to the mempool
func (s *Server) BatchTransactionHandler(w http.ResponseWriter, r *http.Request) {
	var txs []*blockchain.Transaction
	if err := json.NewDecoder(r.Body).Decode(&txs); err != nil {
		utils.LogError("Error decoding batch transactions: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, tx := range txs {
		// Set timestamp if not provided
		if tx.Timestamp == "" {
			tx.Timestamp = time.Now().UTC().Format(time.RFC3339)
		}

		// Add to mempool
		s.TxMempool.AddItem(tx)
	}

	utils.LogInfo("Added %d transactions to mempool", len(txs))
	w.WriteHeader(http.StatusCreated)
}

// CriticalProcessHandler adds a new critical process to the mempool
func (s *Server) CriticalProcessHandler(w http.ResponseWriter, r *http.Request) {
	var cp blockchain.CriticalProcess
	if err := json.NewDecoder(r.Body).Decode(&cp); err != nil {
		utils.LogError("Error decoding critical process: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Set timestamp if not provided
	if cp.Timestamp == "" {
		cp.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	// Add to mempool
	s.CriticalMempool.AddItem(&cp)
	utils.LogInfo("Critical process added to mempool: %s", cp.ProcessID)

	// Broadcast to other nodes
	go s.BroadcastCriticalProcess(&cp)

	w.WriteHeader(http.StatusCreated)
}

// TxBlockchainStatusHandler returns the status of the transaction blockchain
func (s *Server) TxBlockchainStatusHandler(w http.ResponseWriter, r *http.Request) {
	s.blockchainStatusHandler(w, r, s.TxChain)
}

// CriticalBlockchainStatusHandler returns the status of the critical process blockchain
func (s *Server) CriticalBlockchainStatusHandler(w http.ResponseWriter, r *http.Request) {
	s.blockchainStatusHandler(w, r, s.CriticalChain)
}

// blockchainStatusHandler returns the status of a blockchain
func (s *Server) blockchainStatusHandler(w http.ResponseWriter, r *http.Request, chain *blockchain.Blockchain) {
	utils.LogDebug("Received request for blockchain status from %s", r.RemoteAddr)

	status := map[string]interface{}{
		"length":     chain.GetLength(),
		"blockType":  chain.GetBlockType(),
		"lastBlock":  chain.GetLastBlock(),
		"difficulty": chain.GetDifficulty(),
		"nodeId":     s.Node.ID,
		"knownNodes": s.Node.GetKnownNodes(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// TxChainHandler returns the transaction blockchain
func (s *Server) TxChainHandler(w http.ResponseWriter, r *http.Request) {
	s.getChainHandler(w, r, s.TxChain)
}

// CriticalChainHandler returns the critical process blockchain
func (s *Server) CriticalChainHandler(w http.ResponseWriter, r *http.Request) {
	s.getChainHandler(w, r, s.CriticalChain)
}

// getChainHandler returns the entire blockchain
func (s *Server) getChainHandler(w http.ResponseWriter, r *http.Request, chain *blockchain.Blockchain) {
	utils.LogDebug("Received request for blockchain from %s", r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chain.GetBlocks())
}

// TxMempoolHandler returns the transaction mempool
func (s *Server) TxMempoolHandler(w http.ResponseWriter, r *http.Request) {
	s.getMempoolHandler(w, r, s.TxMempool)
}

// CriticalMempoolHandler returns the critical process mempool
func (s *Server) CriticalMempoolHandler(w http.ResponseWriter, r *http.Request) {
	s.getMempoolHandler(w, r, s.CriticalMempool)
}

// getMempoolHandler returns the contents of a mempool
func (s *Server) getMempoolHandler(w http.ResponseWriter, r *http.Request, mempool interface{}) {
	utils.LogDebug("Received request for mempool from %s", r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mempool)
}

// AddTxBlockHandler adds a new transaction block from another node
func (s *Server) AddTxBlockHandler(w http.ResponseWriter, r *http.Request) {
	var block blockchain.Block
	if err := json.NewDecoder(r.Body).Decode(&block); err != nil {
		utils.LogError("Error decoding transaction block: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if s.TxChain.AddBlock(&block) {
		utils.LogInfo("Added new transaction block #%d from network", block.Index)
		w.WriteHeader(http.StatusCreated)
	} else {
		utils.LogError("Failed to add transaction block #%d from network", block.Index)
		http.Error(w, "Invalid block", http.StatusBadRequest)
	}
}

// AddCriticalBlockHandler adds a new critical process block from another node
func (s *Server) AddCriticalBlockHandler(w http.ResponseWriter, r *http.Request) {
	var block blockchain.Block
	if err := json.NewDecoder(r.Body).Decode(&block); err != nil {
		utils.LogError("Error decoding critical process block: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if s.CriticalChain.AddBlock(&block) {
		utils.LogInfo("Added new critical process block #%d from network", block.Index)
		w.WriteHeader(http.StatusCreated)
	} else {
		utils.LogError("Failed to add critical process block #%d from network", block.Index)
		http.Error(w, "Invalid block", http.StatusBadRequest)
	}
}
