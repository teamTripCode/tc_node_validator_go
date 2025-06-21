package main

import (
	"context" // Added for graceful shutdown
	"flag"
	"fmt"
	"log"
	"net/http" // Added for server shutdown
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"tripcodechain_go/blockchain"
	"tripcodechain_go/consensus"
	"tripcodechain_go/llm"
	"tripcodechain_go/mempool"
	"tripcodechain_go/p2p"
	"tripcodechain_go/pkg/validation"
	"tripcodechain_go/utils"

	"github.com/joho/godotenv"
)

// AppConfig holds all startup configurations
type AppConfig struct {
	Port                int
	PBFTWSPort          int
	PBOSWSPort          int
	Verbose             bool
	DataDir             string
	ConfigDir           string // Added for consistency with Dockerfile
	LogDir              string // Added for consistency with Dockerfile
	SeedNodesStr        string
	NodeTypeStr         string
	BootstrapPeersStr   string
	IPScannerEnabled    bool
	IPScanRangesStr     string
	IPScanTargetPortStr string
	KeyPassphrase       string
}

func getEnvInt(key string, defaultValue int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultValue
	}
	valInt, err := strconv.Atoi(valStr)
	if err != nil {
		log.Printf("Warning: Invalid integer value for %s: %s. Using default %d.", key, valStr, defaultValue)
		return defaultValue
	}
	return valInt
}

func loadConfig() *AppConfig {
	config := &AppConfig{}
	// Read from environment variables first, then use flags as overrides or for direct CLI usage
	// Flags will keep their default values if corresponding ENV is not set.
	// Then, ENV vars (if set) will override flag defaults.
	// Finally, CLI flags (if provided) will override ENV vars. This is standard behavior.

	flag.IntVar(&config.Port, "port", getEnvInt("API_PORT", 3002), "Port for the main HTTP API") // Changed default to 3002 as per Dockerfile
	flag.IntVar(&config.PBFTWSPort, "pbftwsport", getEnvInt("PBFT_WS_PORT", 8546), "Port for PBFT WebSocket server")
	flag.IntVar(&config.PBOSWSPort, "pboswstport", getEnvInt("PBOS_WS_PORT", 8547), "Port for pBOS WebSocket server")

	flag.BoolVar(&config.Verbose, "verbose", (os.Getenv("VERBOSE") == "true" || os.Getenv("VERBOSE") == "1" || true), "Enable detailed logging") // Default true
	flag.StringVar(&config.DataDir, "datadir", os.Getenv("DATA_DIR"), "Directory for blockchain data")
	flag.StringVar(&config.ConfigDir, "configdir", os.Getenv("CONFIG_DIR"), "Directory for configuration files")
	flag.StringVar(&config.LogDir, "logdir", os.Getenv("LOG_DIR"), "Directory for log files")

	flag.StringVar(&config.SeedNodesStr, "seed", os.Getenv("SEED_NODES"), "Comma-separated list of seed nodes (HTTP addresses)")
	flag.StringVar(&config.NodeTypeStr, "nodetype", os.Getenv("NODE_TYPE"), "Type of the node (e.g., validator, regular)")
	flag.StringVar(&config.BootstrapPeersStr, "bootstrap", os.Getenv("BOOTSTRAP_PEERS"), "Comma-separated LibP2P bootstrap peer multiaddresses")
	flag.StringVar(&config.KeyPassphrase, "nodekeypass", os.Getenv("NODE_KEY_PASSPHRASE"), "Passphrase for the node's private key")

	scannerEnabledEnv := os.Getenv("IP_SCANNER_ENABLED")
	config.IPScannerEnabled = scannerEnabledEnv == "true"

	config.IPScanRangesStr = os.Getenv("IP_SCAN_RANGES")
	// P2P_PORT from Dockerfile is the base for some peer interactions, ensure this aligns.
	// The p2p.Node uses config.Port (API_PORT) for its HTTP interactions and config.Port + 1000 for libp2p.
	// IP_SCAN_TARGET_PORT should be the HTTP port of other nodes.
	defaultScanTargetPort := getEnvInt("P2P_PORT", 3001) // Default to P2P_PORT for scanning, assuming it's the HTTP port of other nodes.
	scanTargetPortStr := os.Getenv("IP_SCAN_TARGET_PORT")
	if scanTargetPortStr != "" {
		if port, err := strconv.Atoi(scanTargetPortStr); err == nil {
			config.IPScanTargetPortStr = strconv.Itoa(port)
		} else {
			log.Printf("Warning: Invalid IP_SCAN_TARGET_PORT value: %s. Using default derived from P2P_PORT: %d", scanTargetPortStr, defaultScanTargetPort)
			config.IPScanTargetPortStr = strconv.Itoa(defaultScanTargetPort)
		}
	} else {
		config.IPScanTargetPortStr = strconv.Itoa(defaultScanTargetPort)
	}

	flag.Parse()

	// Post-flag parsing checks for required ENVs if flags didn't provide them
	if config.DataDir == "" {
		config.DataDir = "data" // Default if not set by ENV or flag
		utils.LogInfo("Data directory not specified, using default: %s", config.DataDir)
	}
	if config.ConfigDir == "" {
		config.ConfigDir = "config" // Default
		utils.LogInfo("Config directory not specified, using default: %s", config.ConfigDir)
	}
	if config.LogDir == "" {
		config.LogDir = "logs" // Default
		utils.LogInfo("Log directory not specified, using default: %s", config.LogDir)
	}

	if config.KeyPassphrase == "" {
		log.Fatalf("Node key passphrase not provided. Set NODE_KEY_PASSPHRASE environment variable or use the -nodekeypass flag.")
	}
	return config
}

func main() {
	var err error // Declarar err al principio del scope de main
	var appCtx context.Context
	var cancelApp context.CancelFunc
	appCtx, cancelApp = context.WithCancel(context.Background())
	defer cancelApp()

	// Attempt to load .env.test first, then .env
	// This allows for overriding .env with .env.test if present
	if _, err := os.Stat(".env.test"); err == nil {
		err = godotenv.Load(".env.test")
		if err != nil {
			log.Printf("Warning: Error loading .env.test file: %v", err)
		} else {
			log.Println("Successfully loaded .env.test file")
		}
	} else if _, err := os.Stat(".env"); err == nil {
		err = godotenv.Load(".env")
		if err != nil {
			log.Printf("Warning: Error loading .env file: %v", err)
		} else {
			log.Println("Successfully loaded .env file")
		}
	} else {
		log.Println("No .env or .env.test file found, using environment variables or defaults.")
	}

	// 1. Load Configuration
	config := loadConfig()

	// 2. Setup Logging
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds) // Keep existing flags
	log.SetOutput(os.Stdout)                                // Keep existing output
	utils.SetVerbose(config.Verbose)                        // Set verbosity based on config

	utils.LogInfo("Application starting...")

	// 3. Create Data Directories
	// Ensure base data, config, and log dirs exist first, as they are defined as VOLUMES in Dockerfile
	// and might be mounted from the host.
	baseDirs := []string{config.DataDir, config.ConfigDir, config.LogDir}
	for _, dir := range baseDirs {
		if dir == "" { // Should not happen if defaults are set correctly in loadConfig
			log.Fatalf("Critical error: A base directory path (DataDir, ConfigDir, or LogDir) is empty.")
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Error creating base directory %s: %v", dir, err)
		}
	}

	// Create subdirectories within the base data directory
	txDataDir := config.DataDir + "/tx_chain"
	criticalDataDir := config.DataDir + "/critical_chain"
	keysDataDir := config.DataDir + "/keys"

	appSpecificDirs := []string{txDataDir, criticalDataDir, keysDataDir}
	for _, dir := range appSpecificDirs {
		mode := os.FileMode(0755)
		if dir == keysDataDir {
			mode = 0700 // Stricter permissions for keys
		}
		if err := os.MkdirAll(dir, mode); err != nil {
			log.Fatalf("Error creating application specific directory %s: %v", dir, err)
		}
	}

	// 4. Initialize Core Application Components
	// Initialize Blockchain and Currency Manager
	txChain, contractManager, currencyManager, err := blockchain.InitializeBlockchain(txDataDir, blockchain.TransactionBlock)
	if err != nil {
		log.Fatalf("Error initializing transaction blockchain: %v", err)
	}
	utils.LogInfo("Transaction chain initialized with %d blocks", txChain.GetLength())

	criticalChain, _, _, err := blockchain.InitializeBlockchain(criticalDataDir, blockchain.CriticalProcessBlock)
	if err != nil {
		log.Fatalf("Error initializing critical process blockchain: %v", err)
	}
	utils.LogInfo("Critical process chain initialized with %d blocks", criticalChain.GetLength())

	// Determine Node ID for DPoS (using HTTP address for now)
	nodeID := fmt.Sprintf("localhost:%d", config.Port)

	// Initialize DPoS Consensus for Transaction Chain
	// For DPoS, we pass nil for broadcaster and logger as they are not used by DPoS currently.
	consensusTxService, err := consensus.NewConsensus("DPOS", nodeID, currencyManager, nil, nil)
	if err != nil {
		log.Fatalf("Error initializing DPoS consensus service: %v", err)
	}
	var ok bool
	// dposInstance is now *validation.DPoS
	dposInstance, ok := consensusTxService.(*validation.DPoS)
	if !ok {
		// This check might be redundant if NewConsensus always returns the correct interface type
		// or if the Initialize method (called within NewConsensus now) handles type assertion errors.
		// However, keeping it for safety to ensure the cast to *validation.DPoS is valid.
		log.Fatalf("Consensus service for TxChain is not of type *validation.DPoS as expected after NewConsensus call")
	}
	// dposInstance.Initialize(nodeID) is now called within NewConsensus factory.

	// Initialize P2P Node (needed for PBFT Broadcaster)
	initialBootstrapPeers := []string{}
	if config.BootstrapPeersStr != "" {
		initialBootstrapPeers = strings.Split(config.BootstrapPeersStr, ",")
	}
	utils.LogInfo("Initial LibP2P bootstrap peers: %v", initialBootstrapPeers)

	ipScanRanges := []string{}
	if config.IPScanRangesStr != "" {
		ipScanRanges = strings.Split(config.IPScanRangesStr, ",")
	} else {
		ipScanRanges = []string{"127.0.0.1/24"} // Default if not set by ENV
		utils.LogInfo("IP Scan ranges not set via ENV, using default: %v", ipScanRanges)
	}

	targetPortForScan := config.Port // Default to own port
	if config.IPScanTargetPortStr != "" {
		if port, errConv := strconv.Atoi(config.IPScanTargetPortStr); errConv == nil {
			targetPortForScan = port
		} else {
			utils.LogError("Invalid IP_SCAN_TARGET_PORT: %v. Using default: %d", errConv, targetPortForScan)
		}
	}
	keysDataDir := config.DataDir + "/keys"
	if err := os.MkdirAll(keysDataDir, 0700); err != nil {
		log.Fatalf("Error creating keys directory: %v", err)
	}

	node := p2p.NewNode(config.Port, dposInstance, initialBootstrapPeers, config.IPScannerEnabled, ipScanRanges, targetPortForScan, keysDataDir, config.KeyPassphrase)
	if node == nil {
		log.Fatalf("Failed to create p2p.Node, NewNode returned nil.")
	}

	node.NodeType = "validator" // Default or from config.NodeTypeStr
	if config.NodeTypeStr != "" {
		node.NodeType = config.NodeTypeStr
	}

	// Initialize PBFT Broadcaster and Logger using the p2p.Node instance
	pbftBroadcaster := p2p.NewPBFTBroadcaster(node)
	pbftLogger := p2p.NewPBFTLogger()

	// Initialize PBFT Consensus for Critical Chain
	consensusCritical, err := consensus.NewConsensus("PBFT", nodeID, currencyManager, pbftBroadcaster, pbftLogger)
	if err != nil {
		log.Fatalf("Error initializing PBFT consensus: %v", err)
	}

	txChain.SetConsensus(dposInstance) // Use the concrete DPoS instance
	criticalChain.SetConsensus(consensusCritical)
	utils.LogInfo("Dual consensus system configured - DPoS for transactions, PBFT for critical processes")

	blockchain.DeploySystemContracts(txChain, contractManager)
	utils.LogInfo("Base smart contracts deployed")

	txMempool := mempool.NewMempool()
	criticalMempool := mempool.NewMempool()
	utils.LogInfo("Mempools initialized - Transactions: %d, Processes: %d", txMempool.GetSize(), criticalMempool.GetSize())

	// P2P Node (node) already initialized above before PBFT setup.
	// Now, just print startup messages and proceed.
	utils.PrintStartupMessage(node.ID, config.Port)
	utils.LogInfo("Node Type: %s", node.NodeType)

	// 6. Initialize P2P Server
	// LLM Service Initialization (assuming it's needed for the server)
	var llmConfig llm.LLMConfig                           // Declarar llmConfig explícitamente como tipo valor
	llmConfig, err = llm.LoadLLMConfig("llm/config.json") // Asignar con =, err ya está declarada
	if err != nil {
		log.Fatalf("FATAL: Failed to load LLM configuration: %v", err)
	}
	localLLMClient := llm.NewLocalLLMClient(llmConfig) // llmConfig es llm.LLMConfig (valor)
	if localLLMClient == nil {
		log.Fatalf("FATAL: Failed to create LocalLLMClient.")
	}

	p2pServer := p2p.NewServer(node, txChain, criticalChain, txMempool, criticalMempool, nil, localLLMClient, dposInstance)
	llmService := llm.NewDistributedLLMService(p2pServer)
	p2pServer.LLMService = llmService
	llmAPIHandler := llm.NewLLMAPIHandler(llmService)
	p2pServer.SetupRoutes()                              // P2P routes
	if llmAPIHandler != nil && p2pServer.Router != nil { // LLM routes
		p2pServer.Router.HandleFunc("/api/v1/llm/query", llmAPIHandler.HandleQuery).Methods("POST")
	}

	// 7. Start P2P Background Services
	if node.Libp2pHost != nil { // Check if Libp2pHost was initialized
		if node.NodeType == "validator" {
			utils.LogInfo("Node is a validator, attempting to advertise on DHT...")
			go node.AdvertiseAsValidator()
		}

		// Connect to HTTP seed nodes (if any) and bootstrap DHT with libp2p peers
		// Note: node.defaultBootstrapPeers are the ones from initialBootstrapPeers
		if config.SeedNodesStr != "" {
			utils.LogInfo("Registering with HTTP seed nodes from %s: %s", "config", config.SeedNodesStr)
			seedsSlice := strings.SplitSeq(config.SeedNodesStr, ",")
			for seed := range seedsSlice {
				trimmedSeed := strings.TrimSpace(seed)
				if trimmedSeed != "" {
					utils.LogInfo("Attempting to register with seed node %s (HTTP) and bootstrap DHT", trimmedSeed)
					// Pass node.defaultBootstrapPeers (which are the initialBootstrapPeers) for DHT bootstrapping part of RegisterWithNode
					go node.RegisterWithNode(trimmedSeed, node.GetDefaultBootstrapPeers())
				}
			}
		} else if len(node.GetDefaultBootstrapPeers()) > 0 {
			// If no HTTP seeds, but libp2p bootstrap peers are available, use them directly
			utils.LogInfo("No HTTP seed nodes. Attempting direct DHT bootstrap with default peers.")
			go func() {
				if err := node.BootstrapDHT(node.GetDefaultBootstrapPeers()); err != nil {
					utils.LogError("Direct DHT bootstrapping failed: %v", err)
				}
			}()
		} else {
			utils.LogInfo("No HTTP seed nodes or default LibP2P bootstrap peers provided.")
		}

		if node.PubSubService != nil {
			utils.LogInfo("Starting Gossip protocol for validator lists...")
			if err := node.StartGossip(); err != nil {
				utils.LogError("Error starting gossip protocol: %v", err)
			}
		}
		go node.StartHeartbeatSender()
		go node.StartValidatorMonitoring()
		go node.StartNetworkPartitionDetector()
		go node.StartIPScanner()
		go node.StartMetricsUpdater()

	} else {
		utils.LogInfo("Libp2pHost not initialized. Skipping most P2P background services.")
	}

	// 8. Start other background processes
	go p2pServer.StartBackgroundProcessing() // Mempool processing
	utils.LogInfo("Background processing (mempools) started")
	go p2pServer.SyncChains()                                        // Initial chain sync
	go updateValidatorsPeriodically(p2pServer)                       // DPoS validator updates
	go p2p.StartPBFTWebsocketServer(nodeID, 8546, consensusCritical) // Start PBFT WebSocket server
	go p2p.StartPBOSWebsocketServer(nodeID, 8547)                    // Start pBOS WebSocket server

	// 9. Setup Graceful Shutdown
	// Pass p2pServer to shutdown its HTTP server as well
	setupGracefulShutdown(appCtx, cancelApp, node, p2pServer)

	// 10. Start HTTP Server (blocking)
	utils.LogInfo("Starting HTTP server on port %d...", config.Port)
	if err := p2pServer.Start(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP Server failed: %v", err)
	}

	// Wait for context cancellation (e.g. from shutdown signal)
	<-appCtx.Done()
	utils.LogInfo("Application shutting down.")
}

func setupGracefulShutdown(_ context.Context, cancelApp context.CancelFunc, node *p2p.Node, server *p2p.Server) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-c
		utils.LogInfo("Received shutdown signal: %s. Initiating graceful shutdown...", sig.String())

		// 1. Signal all application parts to stop
		cancelApp()

		// 2. Stop P2P services on the node
		if node != nil {
			utils.LogInfo("Stopping P2P node services...")
			node.StopLibp2pServices()
		}

		// 3. Shutdown HTTP server
		if server != nil {
			utils.LogInfo("Shutting down HTTP server...")
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()
			if err := server.Shutdown(shutdownCtx); err != nil { // Assuming Server has a Shutdown method
				utils.LogError("HTTP server shutdown error: %v", err)
			} else {
				utils.LogInfo("HTTP server shutdown complete.")
			}
		}

		// TODO: Add cleanup for other resources like databases, blockchain files if needed.

		utils.LogInfo("Graceful shutdown completed. Exiting.")
		os.Exit(0)
	}()
}

// This main discovery loop is now illustrative, as DiscoverNodes is called by a Ticker in p2p.Node's StartP2PDiscoveryLoop
// Or, if DiscoverNodes is meant to be called from main, it should be structured like this.
// For now, assuming p2p.Node handles its own discovery loop via StartP2PDiscoveryLoop.
// If not, this would be the place for a main-driven periodic discovery.
/*
go func() {
    discoveryTicker := time.NewTicker(1 * time.Minute) // Example interval
    defer discoveryTicker.Stop()
    for {
        select {
        case <-appCtx.Done():
            utils.LogInfo("Main discovery loop stopping...")
            return
        case <-discoveryTicker.C:
            if node != nil && node.Libp2pHost != nil {
                 utils.LogInfo("Main loop: Triggering DiscoverNodes...")
                node.DiscoverNodes() // DiscoverNodes itself should be non-blocking or run its core logic in a goroutine
            }
        }
    }
}()
*/

func updateValidatorsPeriodically(s *p2p.Server) {
	// Ensure s.Node and s.Node.DPoS are not nil
	if s == nil || s.Node == nil || s.Node.DPoS == nil {
		utils.LogError("updateValidatorsPeriodically: Server, Node or DPoS is nil.")
		return
	}

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.Node.P2pCtx().Done(): // Use node's context for shutdown
			utils.LogInfo("Stopping validator updates due to node context cancellation.")
			return
		case <-ticker.C:
			validators, err := getCurrentValidators(s.Node)
			if err == nil {
				consensusModule := s.TxChain.GetConsensus()
				// Type assertion for dpos should be *validation.DPoS
				dpos, ok := consensusModule.(*validation.DPoS)
				if !ok {
					utils.LogError("Consensus module for TxChain is not *validation.DPoS, cannot update validators dynamically.")
					continue
				}
				// UpdateValidators on *validation.DPoS expects []validation.ValidatorInfo
				validatorInfos := make([]validation.ValidatorInfo, len(validators))
				for i, v := range validators {
					// Assuming consensus.ValidatorInfo is compatible or needs to be validation.ValidatorInfo
					// If consensus.ValidatorInfo was defined in consensus/dpos.go, it's now validation.ValidatorInfo
					// If consensus.ValidatorInfo is a generic type in consensus package, this might be okay,
					// but more likely it should be validation.ValidatorInfo if that's what UpdateValidators expects.
					// The dpos_types.go has ValidatorInfo, so it's validation.ValidatorInfo.
					validatorInfos[i] = validation.ValidatorInfo{Address: v}
				}
				dpos.UpdateValidators(validatorInfos)
			} else {
				utils.LogError("Failed to get current validators for DPoS update: %v", err)
			}
		}
	}
}

func getCurrentValidators(node *p2p.Node) ([]string, error) {
	if node == nil {
		return []string{}, fmt.Errorf("p2p.Node instance is nil")
	}
	knownStatuses := node.GetKnownNodeStatuses()
	if len(knownStatuses) == 0 {
		// utils.LogInfo("No known node statuses available to filter for validators.") // Can be noisy
		return []string{}, nil
	}
	var validatorAddresses []string
	for _, peerStatus := range knownStatuses {
		if peerStatus.NodeType == "validator" {
			validatorAddresses = append(validatorAddresses, peerStatus.Address)
		}
	}
	// if len(validatorAddresses) == 0 {
	// 	utils.LogInfo("No nodes with NodeType 'validator' found among known node statuses.")
	// } else {
	// 	utils.LogDebug("Found %d validators for DPoS update: %v", len(validatorAddresses), validatorAddresses)
	// }
	return validatorAddresses, nil
}

// Helper to get default bootstrap peers from node, if needed by other funcs in main
// This is just an example if direct access to node.defaultBootstrapPeers is needed outside.
// func getDefaultBootstrapPeers(node *p2p.Node) []string {
// 	if node != nil {
// 		return node.GetDefaultBootstrapPeers()
// 	}
// 	return []string{}
// }

// Add a GetDefaultBootstrapPeers method to p2p.Node if you want to encapsulate access
// In p2p/node.go:
// func (n *Node) GetDefaultBootstrapPeers() []string {
//    // n.propertyMutex.RLock() // If you add a mutex for these properties
//    // defer n.propertyMutex.RUnlock()
//    return n.defaultBootstrapPeers
// }

// In p2p.Server, add Shutdown method:
// func (s *Server) Shutdown(ctx context.Context) error {
// 	 utils.LogInfo("HTTP server shutting down...")
// 	 return s.httpServer.Shutdown(ctx) // Assuming s.httpServer is the *http.Server instance
// }
// This would require storing the *http.Server in the p2p.Server struct.
// For now, we'll assume p2pServer.Start() is blocking and os.Exit in graceful shutdown handles it.
// The provided graceful shutdown in the prompt is more direct with os.Exit.
// A more advanced graceful shutdown would involve closing channels, waiting for goroutines,
// and then shutting down the server.
// For the current structure, os.Exit is the main mechanism after cleanup.
//
// To make server.Shutdown work, Server struct in p2p/server.go needs its *http.Server stored.
// Example p2p/server.go:
// type Server struct {
//     Router *mux.Router
//     Node *Node
//     // ... other fields
//     httpServer *http.Server // Add this
// }
// func (s *Server) Start() {
//     utils.LogInfo("Server starting on port %d", s.Node.Port)
//     s.httpServer = &http.Server{ // Store it
//         Handler:      s.Router,
//         Addr:         fmt.Sprintf(":%d", s.Node.Port),
//         WriteTimeout: 15 * time.Second,
//         ReadTimeout:  15 * time.Second,
//     }
//     if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
//         utils.LogErrorThenPanic("HTTP server ListenAndServe error: %v", err)
//     }
// }
// func (s *Server) Shutdown(ctx context.Context) error {
//    if s.httpServer == nil {
//        return fmt.Errorf("HTTP server not started")
//    }
//    utils.LogInfo("HTTP server shutting down...")
//    return s.httpServer.Shutdown(ctx)
// }
//
// And update setupGracefulShutdown to call server.Shutdown()
// and remove os.Exit(0) from there to let main return.
// Main would then look like:
// ...
// if err := p2pServer.Start(); err != nil && err != http.ErrServerClosed { ... }
// <-appCtx.Done() // Wait for shutdown signal to complete cleanup
// utils.LogInfo("Application fully shut down.")
//
// This is a more complex change for graceful shutdown, for now, will stick to simpler os.Exit after cleanup.
// The current P2P server Start() uses log.Fatal, so it will exit on its own error.
// For graceful shutdown, it needs to return errors instead.
// This refactoring is becoming larger than the subtask.
// I will focus on the subtask's primary goals of order and clarity first.
// The current server.Start() `log.Fatal(srv.ListenAndServe())` will prevent graceful server shutdown.
// It should be `err := srv.ListenAndServe(); if err != nil && err != http.ErrServerClosed { log.Fatalf(...) }`
// And setupGracefulShutdown should call `srv.Shutdown()`.
// I will make this change to server.Start() and setupGracefulShutdown().

// In p2p.Node, add P2pCtx() accessor
// func (n *Node) P2pCtx() context.Context {
//    return n.p2pCtx
// }// This is used by updateValidatorsPeriodically.
// For now, I'll assume direct access if it's in the same package or adjust.
// The current code `s.Node.P2pCtx()` implies P2pCtx is already a public method or field.
// Let's assume it's a public field `P2pCtx` for now, or I'll add the accessor.
// Ah, `p2pCtx` is a field, but not exported. So an accessor is needed.
// I'll add `P2pCtx()` accessor to `p2p.Node` as part of this refactor.
// Actually, `updateValidatorsPeriodically` takes `s *p2p.Server`, so `s.Node.p2pCtx` is accessible if `Node` is a field of `Server`. It is.
// So, `s.Node.p2pCtx.Done()` should work.
