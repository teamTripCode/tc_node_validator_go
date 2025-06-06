package p2p

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"tripcodechain_go/blockchain"
	"tripcodechain_go/utils"
)

type SeedNode struct {
	nodeManager NodeManagerInterface
}

// GetNodesHandler returns the list of known nodes
func (s *Server) GetNodesHandler(w http.ResponseWriter, r *http.Request) {
	utils.LogDebug("Received request for nodes list from %s", r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.Node.GetKnownNodes())
}

// RegisterNodeHandler handles node registration requests
func (s *Server) RegisterNodeHandler(w http.ResponseWriter, r *http.Request) {
	var node string
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		http.Error(w, "Invalid node format", http.StatusBadRequest)
		utils.LogError("Invalid node registration format: %v", err)
		return
	}

	// Validate node address format
	if !isValidNodeAddress(node) {
		http.Error(w, "Invalid node address format", http.StatusBadRequest)
		utils.LogError("Invalid node address: %s", node)
		return
	}

	// Check if node is responsive before adding
	if isPingable(node) {
		if s.Node.AddNode(node) {
			utils.LogInfo("New node registered: %s", node)
			w.WriteHeader(http.StatusCreated)
		} else {
			utils.LogInfo("Node already registered: %s", node)
			w.WriteHeader(http.StatusOK)
		}

		// Immediately sync with the new node
		go s.syncWithNode(node)
	} else {
		http.Error(w, "Node not reachable", http.StatusBadRequest)
		utils.LogError("Failed to register unreachable node: %s", node)
	}
}

func isPingable(nodeAddr string) bool {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s/ping", nodeAddr))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// isValidNodeAddress validates the node address format
func isValidNodeAddress(addr string) bool {
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		return false
	}

	// Check that we have a valid hostname/IP
	if len(parts[0]) == 0 {
		return false
	}

	// Check that port is a valid number
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}

	// Check port range
	return port > 0 && port <= 65535
}

// verifyNodeSignature verifies a block signature from a node
func (s *Server) verifyNodeSignature(blockHash, signature, nodeID string) bool {
	// In a real implementation, you would verify using the node's public key
	// This is a simplified placeholder implementation
	expectedSig := fmt.Sprintf("signed(%s, %s)", blockHash, s.Node.PrivateKey)
	return signature == expectedSig
}

// signBlock generates a digital signature for a block hash using HMAC-SHA256
// In production, you should use proper asymmetric cryptography
func (s *Server) signBlock(hash string, privateKey string) string {
	h := hmac.New(sha256.New, []byte(privateKey))
	h.Write([]byte(hash))
	return hex.EncodeToString(h.Sum(nil))
}

// syncWithNode synchronizes chains with a specific node
func (s *Server) syncWithNode(node string) {
	utils.LogInfo("Syncing chains with node %s", node)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		s.syncChainWithNode(s.TxChain, "tx", node)
	}()

	go func() {
		defer wg.Done()
		s.syncChainWithNode(s.CriticalChain, "critical", node)
	}()

	wg.Wait()
	utils.LogInfo("Completed sync with node %s", node)
}

// syncChainWithNode synchronizes a specific chain with a specific node
func (s *Server) syncChainWithNode(chain *blockchain.Blockchain, chainType string, node string) {
	url := fmt.Sprintf("http://%s/chain/%s", node, chainType)
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		utils.LogError("Failed to sync %s chain with %s: %v", chainType, node, err)
		return
	}
	defer resp.Body.Close()

	var remoteChain []*blockchain.Block
	if err := json.NewDecoder(resp.Body).Decode(&remoteChain); err != nil {
		utils.LogError("Failed to decode %s chain from %s: %v", chainType, node, err)
		return
	}

	// Validate and potentially adopt the chain
	isValid, err := chain.IsValidChain(remoteChain)
	if err != nil {
		utils.LogError("Invalid %s chain from %s: %v", chainType, node, err)
		return
	}

	if isValid && len(remoteChain) > chain.GetLength() {
		utils.LogInfo("Adopting longer %s chain from %s (%d blocks)",
			chainType, node, len(remoteChain))
		chain.ReplaceChain(remoteChain)
	}
}

// optimizedBroadcastBlock broadcasts a block to other nodes with retry logic and better error handling
func (s *Server) optimizedBroadcastBlock(block *blockchain.Block, chainType string) {
	message := BlockMessage{
		Block:  block,
		NodeID: s.Node.ID,
	}

	// Sign the block
	message.Signature = s.signBlock(block.Hash, s.Node.PrivateKey)

	blockData, err := json.Marshal(block)
	if err != nil {
		utils.LogError("Failed to marshal block for broadcast: %v", err)
		return
	}

	knownNodes := s.Node.GetKnownNodes()

	// Use a wait group to track all broadcast operations
	var wg sync.WaitGroup

	for _, node := range knownNodes {
		if node != s.Node.ID {
			wg.Add(1)
			go func(nodeAddr string) {
				defer wg.Done()
				s.broadcastBlockToNode(nodeAddr, blockData, chainType, message.Signature)
			}(node)
		}
	}

	// Wait for all broadcast operations to complete
	wg.Wait()
	utils.LogInfo("Block broadcast complete for block #%d", block.Index)
}

// broadcastBlockToNode sends a block to a specific node with retry logic
func (s *Server) broadcastBlockToNode(node string, blockData []byte, chainType string, signature string) {
	maxRetries := 3
	retryDelay := 2 * time.Second

	url := fmt.Sprintf("http://%s/block/%s", node, chainType)

	for attempt := 0; attempt < maxRetries; attempt++ {
		req, err := http.NewRequest("POST", url, strings.NewReader(string(blockData)))
		if err != nil {
			utils.LogError("Failed to create request for %s: %v", node, err)
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Node-ID", s.Node.ID)
		req.Header.Set("X-Signature", signature)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)

		if err == nil {
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
				utils.LogInfo("Block accepted by %s", node)
				return
			}

			// If we got a response but not success, read the error
			if resp.StatusCode != http.StatusCreated {
				body, _ := io.ReadAll(resp.Body)
				utils.LogError("Block rejected by %s: %s (HTTP %d)",
					node, string(body), resp.StatusCode)
			}
		} else {
			utils.LogError("Block delivery attempt %d to %s failed: %v",
				attempt+1, node, err)
		}

		// Wait before retrying
		if attempt < maxRetries-1 {
			time.Sleep(retryDelay)
			// Increase delay for next attempt
			retryDelay *= 2
		}
	}

	utils.LogError("Failed to deliver block to %s after %d attempts", node, maxRetries)
}

// ProcessTxMempool creates new blocks from pending transactions with improved batching
func (s *Server) ProcessTxMempool() {
	pendingTxs := s.TxMempool.GetPendingItems()
	if len(pendingTxs) == 0 {
		utils.LogDebug("No pending transactions to process")
		return
	}

	// Define maximum batch size
	const maxBatchSize = 100

	// Process transactions in batches if there are too many
	for i := 0; i < len(pendingTxs); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(pendingTxs) {
			end = len(pendingTxs)
		}

		batch := pendingTxs[i:end]
		utils.LogInfo("Processing batch of %d transactions (batch %d/%d)",
			len(batch), (i/maxBatchSize)+1, (len(pendingTxs)+maxBatchSize-1)/maxBatchSize)

		// Convert batch to []interface{} for processTxBatch
		batchInterfaces := make([]interface{}, len(batch))
		for j, item := range batch {
			batchInterfaces[j] = item
		}
		s.processTxBatch(batchInterfaces)
	}
}

// processTxBatch processes a batch of transactions
func (s *Server) processTxBatch(pendingTxs []interface{}) {
	// Create a slice of transactions from the pending items
	transactions := make([]*blockchain.Transaction, 0, len(pendingTxs))
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

	if err := s.TxChain.AddBlock(block); err != nil {
		utils.LogError("Failed to add block to chain: %v", err)
		return
	}

	utils.LogInfo("Added new block #%d with %d transactions", block.Index, len(transactions))

	// Remove processed transactions from the mempool
	mempoolItems := make([]blockchain.MempoolItem, len(pendingTxs))
	for i, item := range pendingTxs {
		mempoolItems[i] = item.(blockchain.MempoolItem)
	}
	s.TxMempool.RemoveProcessedItems(mempoolItems)

	// Broadcast the new block to other nodes
	s.optimizedBroadcastBlock(block, "tx")
}

// ProcessCriticalMempool creates new blocks from pending critical processes with improved batching
func (s *Server) ProcessCriticalMempool() {
	pendingCritical := s.CriticalMempool.GetPendingItems()
	if len(pendingCritical) == 0 {
		utils.LogDebug("No pending critical processes to process")
		return
	}

	// Define maximum batch size
	const maxBatchSize = 50

	// Process critical processes in batches if there are too many
	for i := 0; i < len(pendingCritical); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(pendingCritical) {
			end = len(pendingCritical)
		}

		batch := pendingCritical[i:end]
		utils.LogInfo("Processing batch of %d critical processes (batch %d/%d)",
			len(batch), (i/maxBatchSize)+1, (len(pendingCritical)+maxBatchSize-1)/maxBatchSize)

		// Convert batch to []interface{} for processCriticalBatch
		batchInterfaces := make([]interface{}, len(batch))
		for j, item := range batch {
			batchInterfaces[j] = item
		}
		s.processCriticalBatch(batchInterfaces)
	}
}

// processCriticalBatch processes a batch of critical processes
func (s *Server) processCriticalBatch(pendingCritical []interface{}) {
	// Create a slice of critical processes from the pending items
	criticalProcesses := make([]*blockchain.CriticalProcess, 0, len(pendingCritical))
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

	if err := s.CriticalChain.AddBlock(block); err != nil {
		utils.LogError("Failed to add critical block to chain: %v", err)
		return
	}

	utils.LogInfo("Added new block #%d with %d critical processes",
		block.Index, len(criticalProcesses))

	// Remove processed critical processes from the mempool
	mempoolItems := make([]blockchain.MempoolItem, len(pendingCritical))
	for i, item := range pendingCritical {
		mempoolItems[i] = item.(blockchain.MempoolItem)
	}
	s.CriticalMempool.RemoveProcessedItems(mempoolItems)

	// Broadcast the new block to other nodes
	s.optimizedBroadcastBlock(block, "critical")
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

	if err := s.TxChain.AddBlock(&block); err == nil {
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

	if err := s.CriticalChain.AddBlock(&block); err == nil {
		utils.LogInfo("Added new critical process block #%d from network", block.Index)
		w.WriteHeader(http.StatusCreated)
	} else {
		utils.LogError("Failed to add critical process block #%d from network", block.Index)
		http.Error(w, "Invalid block", http.StatusBadRequest)
	}
}
