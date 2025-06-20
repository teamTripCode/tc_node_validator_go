package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"tripcodechain_go/blockchain"
	"tripcodechain_go/consensus"
	"tripcodechain_go/llm"
	"tripcodechain_go/mempool"
	"tripcodechain_go/p2p"
	"tripcodechain_go/utils"
)

func main() {
	// Set up logging
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.SetOutput(os.Stdout)

	// Define command line flags
	seedNodesFlag := flag.String("seed", "", "Comma-separated list of seed nodes (fallback if SEED_NODES env var is not set)")
	//	externalIP := flag.String("ip", "0.0.0.0", "External IP address for node")

	// Parse command line flags
	portFlag := flag.Int("port", 3001, "Port to listen on")
	verboseFlag := flag.Bool("verbose", true, "Enable detailed logging")
	dataDirFlag := flag.String("datadir", "data", "Directory for blockchain data")
	flag.Parse()

	// Set verbose mode
	utils.SetVerbose(*verboseFlag)

	// Create new node
	node := p2p.NewNode(*portFlag)
	node.NodeType = "validator"
	utils.PrintStartupMessage(node.ID, *portFlag)

	// Determine seed nodes: prioritize environment variable, then flag
	var seedNodesStr string
	var seedNodesSource string

	envSeedNodes := os.Getenv("SEED_NODES")
	if envSeedNodes != "" {
		seedNodesStr = envSeedNodes
		seedNodesSource = "environment variable SEED_NODES"
	} else {
		seedNodesStr = *seedNodesFlag
		seedNodesSource = "command-line flag -seed"
	}

	if seedNodesStr != "" {
		utils.LogInfo("Loading seed nodes from %s: %s", seedNodesSource, seedNodesStr)
		seeds := strings.SplitSeq(seedNodesStr, ",")
		for seed := range seeds {
			trimmedSeed := strings.TrimSpace(seed)
			if trimmedSeed != "" {
				node.AddNode(trimmedSeed) // AddNode should handle if it's already known or self
				go node.RegisterWithNode(trimmedSeed)
			}
		}
	} else {
		utils.LogInfo("No seed nodes provided via environment variable or command-line flag.")
	}

	// Create data directories
	txDataDir := *dataDirFlag + "/tx_chain"
	criticalDataDir := *dataDirFlag + "/critical_chain"

	if err := os.MkdirAll(txDataDir, 0755); err != nil {
		log.Fatal("Error creating transaction chain directory:", err)
	}
	if err := os.MkdirAll(criticalDataDir, 0755); err != nil {
		log.Fatal("Error creating critical chain directory:", err)
	}

	// Initialize blockchain systems - only once per chain type
	txChain, contractManager, currencyManager, err := blockchain.InitializeBlockchain(txDataDir, blockchain.TransactionBlock)
	if err != nil {
		log.Fatal("Error initializing transaction blockchain:", err)
	}
	utils.LogInfo("Transaction chain initialized with %d blocks", txChain.GetLength())

	// Initialize critical process blockchain
	criticalChain, _, _, err := blockchain.InitializeBlockchain(criticalDataDir, blockchain.CriticalProcessBlock)
	if err != nil {
		log.Fatal("Error initializing critical process blockchain:", err)
	}
	utils.LogInfo("Critical process chain initialized with %d blocks", criticalChain.GetLength())

	// Configure dual consensus systems
	consensusTx, err := consensus.NewConsensus("DPOS", node.ID, currencyManager)
	if err != nil {
		log.Fatal("Error initializing DPoS consensus:", err)
	}

	consensusCritical, err := consensus.NewConsensus("PBFT", node.ID, currencyManager)
	if err != nil {
		log.Fatal("Error initializing PBFT consensus:", err)
	}

	// Assign consensus mechanisms to corresponding chains
	txChain.SetConsensus(consensusTx)
	criticalChain.SetConsensus(consensusCritical)
	utils.LogInfo("Dual consensus system configured - DPoS for transactions, PBFT for critical processes")

	// Deploy system contracts
	blockchain.DeploySystemContracts(txChain, contractManager)
	utils.LogInfo("Base smart contracts deployed")

	// Initialize mempools
	txMempool := mempool.NewMempool()
	criticalMempool := mempool.NewMempool()
	utils.LogInfo("Mempools initialized - Transactions: %d, Processes: %d",
		txMempool.GetSize(), criticalMempool.GetSize())

	// Load LLM Configuration
	llmConfig, err := llm.LoadLLMConfig("llm/config.json") // Or pass "" to use default path in LoadLLMConfig
	if err != nil {
		log.Fatalf("FATAL: Failed to load LLM configuration: %v", err)
	}

	// Create Local LLM Client
	localLLMClient := llm.NewLocalLLMClient(llmConfig)
	if localLLMClient == nil {
		// NewLocalLLMClient might not return nil based on current implementation,
		// but good practice to check if it could.
		// If NewLocalLLMClient logs and returns a non-nil client even on error,
		// this check might be redundant or need adjustment based on NewLocalLLMClient's behavior.
		log.Fatalf("FATAL: Failed to create LocalLLMClient.")
	}

	// --- LLM Service and P2P Server Initialization ---
	// 1. Create P2P server instance.
	// The MCPResponseProcessor (llmService) is set to nil initially.
	// localLLMClient is passed as the LocalLLMProcessor.
	p2pServer := p2p.NewServer(node, txChain, criticalChain, txMempool, criticalMempool, nil, localLLMClient)

	// 2. Create DistributedLLMService. p2pServer implements llm.P2PBroadcaster.
	llmService := llm.NewDistributedLLMService(p2pServer)

	// 3. Set the LLMService (which is an MCPResponseProcessor) on the p2pServer.
	p2pServer.LLMService = llmService

	// 4. Create LLMAPIHandler
	llmAPIHandler := llm.NewLLMAPIHandler(llmService)

	// 5. Setup P2P routes
	p2pServer.SetupRoutes()

	// 6. Register LLM API routes directly using the p2pServer's router
	if llmAPIHandler != nil && p2pServer.Router != nil {
		p2pServer.Router.HandleFunc("/api/v1/llm/query", llmAPIHandler.HandleQuery).Methods("POST")
		utils.LogInfo("LLM API route /api/v1/llm/query registered via main.")
	} else {
		if llmAPIHandler == nil {
			utils.LogError("LLMAPIHandler is nil, LLM routes not registered.")
		}
		if p2pServer.Router == nil { // Should not happen if NewServer initializes Router
			utils.LogError("P2P Server Router is nil, LLM routes not registered.")
		}
	}
	// --- End LLM Service and P2P Server Initialization ---

	// Start background processes
	p2pServer.StartBackgroundProcessing()
	utils.LogInfo("Background processing started")

	// Sincronización inicial
	p2pServer.SyncChains()

	// Actualizar validadores desde contrato
	go updateValidatorsPeriodically(p2pServer)

	// Start node monitoring
	go node.StartHeartbeat()
	utils.LogInfo("Node heartbeat started")

	// Discover other nodes
	go func() {
		time.Sleep(2 * time.Second)
		node.DiscoverNodes()
		utils.LogInfo("Node discovery initiated...")
	}()

	// Handle graceful shutdown
	setupGracefulShutdown()

	// Start server (blocking)
	utils.LogInfo("Starting server on port %d...", *portFlag)
	p2pServer.Start()
}

func setupGracefulShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		utils.LogInfo("Shutting down node...")
		// Add resource cleanup here
		os.Exit(0)
	}()
}

func updateValidatorsPeriodically(s *p2p.Server) {
	ticker := time.NewTicker(5 * time.Minute) // Consider making this configurable or shorter for faster updates
	defer ticker.Stop()
	for range ticker.C {
		// Pass s.Node instead of s.TxChain
		validators, err := getCurrentValidators(s.Node)
		if err == nil {
			// Ensure Consensus is DPoS type if not already guaranteed by txChain setup
			consensusModule := s.TxChain.GetConsensus()
			dpos, ok := consensusModule.(*consensus.DPoS)
			if !ok {
				utils.LogError("Consensus module for TxChain is not DPoS, cannot update validators dynamically.")
				continue
			}
			validatorInfos := make([]consensus.ValidatorInfo, len(validators))
			for i, v := range validators {
				validatorInfos[i] = consensus.ValidatorInfo{Address: v}
			}
			dpos.UpdateValidators(validatorInfos)
		} else {
			utils.LogError("Failed to get current validators: %v", err)
		}
	}
}

// getCurrentValidators retrieves validator node addresses from the discovered peers.
func getCurrentValidators(node *p2p.Node) ([]string, error) {
	if node == nil {
		return []string{}, fmt.Errorf("p2p.Node instance is nil")
	}

	// Use GetKnownNodeStatuses which returns []p2p.NodeStatus (values, not pointers)
	// This provides a more comprehensive list of all known peers.
	knownStatuses := node.GetKnownNodeStatuses()
	if len(knownStatuses) == 0 {
		utils.LogInfo("No known node statuses available to filter for validators.")
		return []string{}, nil
	}

	var validatorAddresses []string
	// Iterate over []p2p.NodeStatus. 'peer' is a p2p.NodeStatus value.
	for _, peerStatus := range knownStatuses {
		if peerStatus.NodeType == "validator" {
			validatorAddresses = append(validatorAddresses, peerStatus.Address)
		}
	}

	if len(validatorAddresses) == 0 {
		utils.LogInfo("No nodes with NodeType 'validator' found among known node statuses.")
	} else {
		utils.LogInfo("Found %d validators: %v", len(validatorAddresses), validatorAddresses)
	}

	return validatorAddresses, nil
}
