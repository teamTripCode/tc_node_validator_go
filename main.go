// main.go
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tripcodechain_go/blockchain"
	"tripcodechain_go/consensus"
	"tripcodechain_go/contracts"
	"tripcodechain_go/currency"
	"tripcodechain_go/mempool"
	"tripcodechain_go/p2p"
	"tripcodechain_go/utils"
)

func main() {
	// Configure logging
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.SetOutput(os.Stdout)

	const (
		DPOS = "DPOS"
	)

	// Parse command line flags
	portFlag := flag.Int("port", 3000, "Port to listen on")
	verboseFlag := flag.Bool("verbose", true, "Enable verbose logging")
	flag.Parse()

	// Set verbose logging mode
	utils.SetVerbose(*verboseFlag)

	// Create a new node
	node := p2p.NewNode(*portFlag)
	utils.PrintStartupMessage(node.ID, *portFlag)

	// Initialize blockchain for transactions
	txChain := blockchain.NewBlockchain(blockchain.TransactionBlock)
	utils.LogInfo("Transaction blockchain initialized with %d blocks", txChain.GetLength())

	currencyManager := currency.InitNativeToken(txChain, "TC", 1000000) // 1 millón de unidades iniciales
	utils.LogInfo("Native currency TripCoin initialized")

	consensus, err := consensus.NewConsensus(DPOS, node.ID, currencyManager)
	if err != nil {
		log.Fatal("Error inicializando consenso:", err)
	}
	utils.LogInfo("Consenso %s inicializado", consensus.GetType())

	// Desplegar contratos base del sistema
	contracts.DeploySystemContracts(txChain)
	utils.LogInfo("System contracts deployed")

	// Initialize blockchain for critical processes
	criticalChain := blockchain.NewBlockchain(blockchain.CriticalProcessBlock)
	utils.LogInfo("Critical process blockchain initialized with %d blocks", criticalChain.GetLength())

	// Initialize mempools
	txMempool := mempool.NewMempool()
	criticalMempool := mempool.NewMempool()
	utils.LogInfo("Mempools initialized")

	// Create and start server
	server := p2p.NewServer(node, txChain, criticalChain, txMempool, criticalMempool)

	// Start background processing for mempools
	server.StartBackgroundProcessing()
	utils.LogInfo("Background processing started")

	// Start node heartbeat
	go node.StartHeartbeat()
	utils.LogInfo("Node heartbeat started")

	// Discover other nodes in the network
	go func() {
		// Wait a bit to let the server start
		time.Sleep(2 * time.Second)
		node.DiscoverNodes()
	}()

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		utils.LogInfo("Shutting down node...")
		os.Exit(0)
	}()

	// Start the server (this blocks until the server exits)
	server.Start()
}
