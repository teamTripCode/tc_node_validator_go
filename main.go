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

	// Initialize DPoS (before NewNode and NewServer)
	// consensusTx is already initialized as a DPoS instance later, let's use that or create one dedicated for p2p needs.
	// For now, let's assume consensusTx (which is *DPoS) will be used.
	// It's initialized later, so we need to adjust the order or pass a placeholder.
	// For now, we will initialize DPoS here and ensure it's the same one used by txChain.

	// Create a DPoS instance for p2p components.
	// Note: `currencyManager` is initialized later. This is a temporary DPoS for p2p.Node.
	// The DPoS instance for the actual consensus mechanism (consensusTx) is created later.
	// This highlights a potential need to refactor DPoS initialization or how it's accessed.
	// For this step, we'll create a temporary dposP2P.
	// A better approach might be to initialize all main components first, then pass them around.
	dposP2P := consensus.NewDPoS(nil) // Passing nil for currencyManager initially for p2p.Node
	// It's crucial that p2p.Node.DPoS and p2p.Server.DPoS point to the *actual* DPoS instance
	// used by the consensus mechanism. This will be adjusted when consensusTx is created.

	// Create new node
	node := p2p.NewNode(*portFlag, dposP2P) // Pass DPoS to NewNode
	if node == nil {
		log.Fatalf("Failed to create p2p.Node, NewNode returned nil.")
	}
	node.NodeType = "validator" // Example: set node type
	utils.PrintStartupMessage(node.ID, *portFlag)

	// If this node is a validator, start advertising itself on the DHT.
	// This should be done after the LibP2P host and DHT are initialized in NewNode.
	if node.NodeType == "validator" {
		if node.Libp2pHost != nil { // Check if Libp2pHost was initialized
			utils.LogInfo("Node is a validator, attempting to advertise on DHT...")
			go node.AdvertiseAsValidator()
		} else {
			utils.LogInfo("Node is a validator, but Libp2pHost not initialized. Skipping DHT advertisement.")
		}
	}

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
		seedsSlice := strings.Split(seedNodesStr, ",") // Use strings.Split
		var defaultLibp2pBootstrapPeers []string      // Define empty slice for now
		// Example: defaultLibp2pBootstrapPeers = []string{"/ip4/127.0.0.1/tcp/11001/p2p/QmAnotherPeer"}

		for _, seed := range seedsSlice { // Corrected loop
			trimmedSeed := strings.TrimSpace(seed)
			if trimmedSeed != "" {
				// The old AddNode and RegisterWithNode might be redundant if DHT is primary
				// For now, keeping existing behavior and adding DHT bootstrap call.
				// node.AddNode(trimmedSeed) // This adds to the HTTP knownNodes
				utils.LogInfo("Attempting to register with seed node %s (HTTP) and bootstrap DHT", trimmedSeed)
				go node.RegisterWithNode(trimmedSeed, defaultLibp2pBootstrapPeers)
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
	// Use the DPoS instance created for p2p.Node for consensusTx as well, or ensure they are the same.
	// The current `NewConsensus` returns a `Consensus` interface. We need the concrete DPoS type.
	var dposInstance *consensus.DPoS
	consensusTxService, err := consensus.NewConsensus("DPOS", node.ID, currencyManager)
	if err != nil {
		log.Fatal("Error initializing DPoS consensus service:", err)
	}
	var ok bool
	dposInstance, ok = consensusTxService.(*consensus.DPoS)
	if !ok {
		log.Fatal("Consensus service for TxChain is not of type DPoS")
	}
	// Now, ensure the node and server use this fully initialized DPoS instance.
	node.DPoS = dposInstance      // Update node's DPoS to the fully initialized one
	dposP2P = dposInstance        // Ensure dposP2P also points to this, if used by server separately

	consensusCritical, err := consensus.NewConsensus("PBFT", node.ID, currencyManager)
	if err != nil {
		log.Fatal("Error initializing PBFT consensus:", err)
	}

	// Assign consensus mechanisms to corresponding chains
	txChain.SetConsensus(consensusTxService) // Corrected variable name
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
	// Pass the dposInstance (which is dposP2P, now updated to the actual consensus DPoS) to NewServer
	p2pServer := p2p.NewServer(node, txChain, criticalChain, txMempool, criticalMempool, nil, localLLMClient, dposInstance)

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

	// Start node monitoring (now StartHeartbeatSender)
	go node.StartHeartbeatSender()
	utils.LogInfo("Node heartbeat sender started")

	// Discover other nodes
	go func() {
		time.Sleep(2 * time.Second) // Initial delay before first discovery
		node.DiscoverNodes()
		utils.LogInfo("Node discovery initiated...")
	}()

	// Start Gossip Protocol
	if node.PubSubService != nil {
		utils.LogInfo("Starting Gossip protocol for validator lists...")
		if err := node.StartGossip(); err != nil {
			utils.LogError("Error starting gossip protocol: %v", err)
			// Decide if this is fatal or if the node can continue without gossip
		}
	} else {
		utils.LogInfo("PubSubService not available, skipping StartGossip.")
	}

	// Start Validator Monitoring
	// Ensure DPoS is properly initialized and available in node.DPoS before calling this
	if node.DPoS != nil {
		utils.LogInfo("Starting validator monitoring service...")
		node.StartValidatorMonitoring()
	} else {
		utils.LogInfo("DPoS not available in Node, skipping StartValidatorMonitoring.")
	}

	// Handle graceful shutdown
	setupGracefulShutdown(node) // Pass node to graceful shutdown

	// Start server (blocking)
	utils.LogInfo("Starting server on port %d...", *portFlag)
	p2pServer.Start()
}

func setupGracefulShutdown(node *p2p.Node) { // Accept node
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		utils.LogInfo("Shutting down node...")
		if node != nil {
			node.StopLibp2pServices() // Call the renamed stop function
		}
		// Add other resource cleanup here if necessary
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
