package p2p

import (
	"context"         // Added for Shutdown method
	"encoding/base64" // Added for signBlock
	"encoding/json"
	"fmt"

	// "log" // Removed as unused
	"net/http"
	"time"

	"tripcodechain_go/blockchain"
	// "tripcodechain_go/llm" // Removed as LocalLLMClient is replaced by LocalLLMProcessor interface
	"tripcodechain_go/mempool"
	"tripcodechain_go/pkg/validation" // Changed for DPoS types
	"tripcodechain_go/utils"

	// "tripcodechain_go/consensus" // Removed if DPoS was the only reason

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp" // Added for metrics
)

// Server represents the HTTP server for the blockchain node
type Server struct {
	Router          *mux.Router
	Node            *Node
	TxChain         *blockchain.Blockchain
	CriticalChain   *blockchain.Blockchain
	TxMempool       *mempool.Mempool
	CriticalMempool *mempool.Mempool
	NodeMgr         *NodeManager         // Added NodeManager
	LLMService      MCPResponseProcessor // Changed to interface
	localLLM        LocalLLMProcessor    // Changed to interface
	DPoS            *validation.DPoS     // Changed DPoS field type
	httpServer      *http.Server         // Added for graceful shutdown
}

// NewServer creates a new server instance
func NewServer(node *Node, txChain *blockchain.Blockchain, criticalChain *blockchain.Blockchain,
	txMempool *mempool.Mempool, criticalMempool *mempool.Mempool,
	llmService MCPResponseProcessor, localLLMClient LocalLLMProcessor, dpos *validation.DPoS) *Server { // Changed dpos parameter type

	server := &Server{
		Router:          mux.NewRouter(),
		NodeMgr:         NewNodeManager(), // Initialize NodeManager
		Node:            node,
		TxChain:         txChain,
		CriticalChain:   criticalChain,
		TxMempool:       txMempool,
		CriticalMempool: criticalMempool,
		LLMService:      llmService,     // LLMService (MCPResponseProcessor) is set here
		localLLM:        localLLMClient, // Initialize localLLM
		DPoS:            dpos,           // Store DPoS instance
	}

	// server.setupRoutes() // setupRoutes will be called from main after LLMAPIHandler is created
	return server
}

// SetupRoutes configures the API routes
func (s *Server) SetupRoutes() { // llmAPIHandler parameter removed
	// Node management endpoints
	// s.Router.HandleFunc("/nodes", s.GetNodesHandler).Methods("GET") // Original, may conflict or be replaced
	// s.Router.HandleFunc("/register", s.RegisterNodeHandler).Methods("POST") // Original, may conflict or be replaced
	s.Router.HandleFunc("/ping", s.PingHandler).Methods("GET")

	// New Node Discovery Endpoints
	// Note: The task asks for http.HandleFunc, but gorilla/mux uses Router.HandleFunc.
	// The new handlers are designed to be compatible with http.HandlerFunc, which mux.Router also accepts.
	s.Router.HandleFunc("/nodes", RegisterNodeHandler(s.NodeMgr)).Methods("POST")         // New: For registering a node
	s.Router.HandleFunc("/nodes/active", GetActiveNodesHandler(s.NodeMgr)).Methods("GET") // New: For getting active nodes

	// Retaining original /nodes GET and /register POST for now, but commented out to avoid conflict.
	// These should be reviewed: if the new /nodes POST and /nodes/active GET replace their functionality,
	// then the original s.GetNodesHandler and s.RegisterNodeHandler (now OriginalRegisterNodeHandler)
	// might need to be aliased, removed, or adapted if they serve a different purpose.
	// For now, the new handlers are added as per spec. The original /nodes GET is different from /nodes/active GET.
	// The original /register POST is different from the new /nodes POST.
	// To avoid direct conflict on the same path+method, the original ones might need path changes if kept.
	// For now, I'll assume the new /nodes (POST) and /nodes/active (GET) are the primary ones as per task.
	// The original /nodes (GET) was (s *Server) GetNodesHandler.
	// The original /register (POST) was (s *Server) RegisterNodeHandler - renamed to OriginalRegisterNodeHandler.

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

	// MCP routes
	s.Router.HandleFunc("/mcp/query", s.HandleMCPQuery).Methods("POST")
	s.Router.HandleFunc("/mcp/response", s.HandleMCPResponse).Methods("POST")

	// Heartbeat endpoint
	s.Router.HandleFunc("/heartbeat", s.HeartbeatHandler).Methods("POST")

	// Metrics endpoint
	s.Router.Handle("/metrics", promhttp.Handler())
	utils.LogInfo("Exposing /metrics endpoint for Prometheus.")

	// LLM API routes are now registered in main.go
}

// GetNodeID returns the ID of the node associated with this server.
// This makes Server satisfy a part of llm.P2PBroadcaster interface.
func (s *Server) GetNodeID() string {
	if s.Node == nil {
		return "" // Or handle error appropriately
	}
	return s.Node.ID
}

// GetNodeMgr returns the NodeManager of this server.
// This makes Server satisfy a part of llm.P2PBroadcaster interface.
func (s *Server) GetNodeMgr() *NodeManager {
	return s.NodeMgr
}

// Start starts the HTTP server
func (s *Server) Start() error { // Modified to return error
	utils.LogInfo("Server starting on port %d", s.Node.Port)

	// Set timeouts for the server
	s.httpServer = &http.Server{ // Store server instance
		Handler:      s.Router,
		Addr:         fmt.Sprintf(":%d", s.Node.Port),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	utils.LogInfo("HTTP server listening on %s", s.httpServer.Addr)
	err := s.httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed { // Don't log ErrServerClosed as fatal
		utils.LogError("HTTP server ListenAndServe error: %v", err)
		return err
	}
	utils.LogInfo("HTTP server stopped.")
	return nil
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return fmt.Errorf("HTTP server not started or already shut down")
	}
	utils.LogInfo("HTTP server shutting down...")
	return s.httpServer.Shutdown(ctx)
}

// signBlock signs a block hash using the node's signer.
// It returns the base64 encoded signature or an empty string on error.
func (s *Server) signBlock(blockHash string) string {
	data := []byte(blockHash)

	if s.Node == nil || s.Node.Signer == nil {
		utils.LogError("signBlock: Node or Signer is nil")
		return ""
	}

	signatureBytes, err := s.Node.Signer.Sign(data)
	if err != nil {
		utils.LogError("signBlock: Error signing data: %v", err)
		return ""
	}

	return base64.StdEncoding.EncodeToString(signatureBytes)
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

// HandleMCPQuery maneja las consultas MCP entrantes.
func (s *Server) HandleMCPQuery(w http.ResponseWriter, r *http.Request) {
	var query MCPQuery
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		utils.LogError("Error decodificando MCPQuery: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	utils.LogInfo("MCPQuery recibido: QueryID=%s, OriginNodeID=%s", query.QueryID, query.OriginNodeID)

	// Placeholder para verificación de firma
	if query.Signature.NodeID != "" && query.Signature.Signature != "" {
		utils.LogDebug("Firma presente para MCPQuery: QueryID=%s, NodeID=%s", query.QueryID, query.Signature.NodeID)
		// Aquí iría la lógica de verificación de la firma
	} else {
		utils.LogInfo("Firma ausente para MCPQuery: QueryID=%s", query.QueryID)
	}

	// Lógica de procesamiento de la consulta (placeholder)
	if s.localLLM != nil {
		llmResponsePayload, err := s.localLLM.QueryLLM(query.Payload)
		if err != nil {
			utils.LogError("Failed to process MCPQuery %s with local LLM: %v", query.QueryID, err)
			// Still send HTTP OK because the query was received, but no MCPResponse will be sent.
		} else {
			utils.LogInfo("MCPQuery %s processed successfully by local LLM.", query.QueryID)

			// Construct MCPResponse
			mcpResponse := &MCPResponse{
				QueryID:         query.QueryID,
				Timestamp:       time.Now().Unix(),
				OriginNodeID:    query.OriginNodeID, // This should be the original querier
				ResponderNodeID: s.Node.ID,          // This node is responding
				Payload:         llmResponsePayload,
				// Signature would be calculated here if implemented
				// Signature: s.Node.SignData(mcpResponse.SignableData()) // Placeholder for signing
			}

			// For now, log the response instead of sending it back via P2P
			responseJSON, _ := json.Marshal(mcpResponse) // Error handling omitted for brevity in log
			utils.LogInfo("MCPResponse prepared for QueryID %s: %s", query.QueryID, string(responseJSON))

			// TODO: Implement actual P2P sending of MCPResponse to query.OriginNodeID
			// Example: err := s.SendMCPResponse(query.OriginNodeID, mcpResponse)
			// if err != nil {
			//    utils.LogError("Failed to send MCPResponse for QueryID %s to %s: %v", query.QueryID, query.OriginNodeID, err)
			// }
		}
	} else {
		utils.LogInfo("Local LLM client not configured, MCPQuery %s cannot be processed locally.", query.QueryID)
	}

	w.WriteHeader(http.StatusOK)
	// Respond with a simple confirmation message or the processed data
	if _, err := w.Write([]byte("MCPQuery recibido con éxito")); err != nil { //TODO: Consider if response should change based on LLM outcome
		utils.LogError("Error escribiendo respuesta para MCPQuery: %v", err)
	}
}

// HandleMCPResponse maneja las respuestas MCP entrantes.
func (s *Server) HandleMCPResponse(w http.ResponseWriter, r *http.Request) {
	var response MCPResponse
	if err := json.NewDecoder(r.Body).Decode(&response); err != nil {
		utils.LogError("Error decodificando MCPResponse: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	utils.LogInfo("MCPResponse recibido: QueryID=%s, ResponderNodeID=%s", response.QueryID, response.ResponderNodeID)

	// Placeholder para verificación de firma
	if response.Signature.NodeID != "" && response.Signature.Signature != "" {
		utils.LogDebug("Firma presente para MCPResponse: QueryID=%s, NodeID=%s", response.QueryID, response.Signature.NodeID)
		// Aquí iría la lógica de verificación de la firma
	} else {
		utils.LogInfo("Firma ausente para MCPResponse: QueryID=%s", response.QueryID)
	}

	// Lógica de procesamiento de la respuesta (placeholder)
	if s.LLMService != nil {
		s.LLMService.ProcessIncomingResponse(&response)
	} else {
		utils.LogInfo("LLMService not initialized in P2P server, cannot process MCPResponse further.")
	}

	w.WriteHeader(http.StatusOK)
	// Respond with a simple confirmation message or the processed data
	if _, err := w.Write([]byte("MCPResponse recibido con éxito y procesado por LLMService si disponible")); err != nil {
		utils.LogError("Error escribiendo respuesta para MCPResponse: %v", err)
	}
}
