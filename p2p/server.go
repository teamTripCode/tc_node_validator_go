package p2p

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"tripcodechain_go/blockchain"
	"tripcodechain_go/mempool"
	"tripcodechain_go/utils"

	"github.com/gorilla/mux"
)

// Server represents the HTTP server for the blockchain node
type Server struct {
	Router          *mux.Router
	Node            *Node
	TxChain         *blockchain.Blockchain
	CriticalChain   *blockchain.Blockchain
	TxMempool       *mempool.Mempool
	CriticalMempool *mempool.Mempool
}

// NewServer creates a new server instance
func NewServer(node *Node, txChain *blockchain.Blockchain, criticalChain *blockchain.Blockchain,
	txMempool *mempool.Mempool, criticalMempool *mempool.Mempool) *Server {

	server := &Server{
		Router:          mux.NewRouter(),
		Node:            node,
		TxChain:         txChain,
		CriticalChain:   criticalChain,
		TxMempool:       txMempool,
		CriticalMempool: criticalMempool,
	}

	server.setupRoutes()
	return server
}

// setupRoutes configures the API routes
func (s *Server) setupRoutes() {
	// Node management endpoints
	s.Router.HandleFunc("/nodes", s.GetNodesHandler).Methods("GET")
	s.Router.HandleFunc("/register", s.RegisterNodeHandler).Methods("POST")
	s.Router.HandleFunc("/ping", s.PingHandler).Methods("GET")

	// Transaction endpoints
	s.Router.HandleFunc("/tx", s.TransactionHandler).Methods("POST")
	s.Router.HandleFunc("/tx/batch", s.BatchTransactionHandler).Methods("POST")

	// Critical process endpoints
	s.Router.HandleFunc("/critical", s.CriticalProcessHandler).Methods("POST")

	// Blockchain status endpoints
	s.Router.HandleFunc("/status/tx", s.TxBlockchainStatusHandler).Methods("GET")
	s.Router.HandleFunc("/status/critical", s.CriticalBlockchainStatusHandler).Methods("GET")

	// Blockchain sync endpoints
	s.Router.HandleFunc("/chain/tx", s.TxChainHandler).Methods("GET")
	s.Router.HandleFunc("/chain/critical", s.CriticalChainHandler).Methods("GET")

	// Mempool status endpoints
	s.Router.HandleFunc("/mempool/tx", s.TxMempoolHandler).Methods("GET")
	s.Router.HandleFunc("/mempool/critical", s.CriticalMempoolHandler).Methods("GET")

	// Block addition endpoints (for P2P synchronization)
	s.Router.HandleFunc("/block/tx", s.AddTxBlockHandler).Methods("POST")
	s.Router.HandleFunc("/block/critical", s.AddCriticalBlockHandler).Methods("POST")
}

// Start starts the HTTP server
func (s *Server) Start() {
	utils.LogInfo("Server starting on port %d", s.Node.Port)

	// Set timeouts for the server
	srv := &http.Server{
		Handler:      s.Router,
		Addr:         fmt.Sprintf(":%d", s.Node.Port),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

// StartBackgroundProcessing starts the mempool processing jobs
func (s *Server) StartBackgroundProcessing() {
	go s.ProcessMempools()
}

// ProcessMempools runs background jobs that process pending items in mempools
func (s *Server) ProcessMempools() {
	tickerTx := time.NewTicker(10 * time.Second)
	tickerCritical := time.NewTicker(15 * time.Second)

	for {
		select {
		case <-tickerTx.C:
			utils.LogDebug("Checking transaction mempool for processing")
			s.ProcessTxMempool()
		case <-tickerCritical.C:
			utils.LogDebug("Checking critical process mempool for processing")
			s.ProcessCriticalMempool()
		}
	}
}
