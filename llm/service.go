package llm

import (
	"fmt"
	"sync"
	"time"

	"tripcodechain_go/p2p" // For p2p.MCPResponse
	"tripcodechain_go/utils"

	"github.com/google/uuid"
)

// QueryState holds the state of a distributed query.
type QueryState struct {
	Responses []p2p.MCPResponse
	Mutex     sync.Mutex
	DoneChan  chan struct{} // Signals that the query processing is complete (either by timeout or enough responses)
	QueryID   string
	Timeout   time.Duration
	StartTime time.Time
}

// DistributedLLMService manages distributed queries to LLMs across the network.
// It now implements p2p.MCPResponseProcessor.
type DistributedLLMService struct {
	p2pAdapter    P2PBroadcaster // Changed from *p2p.Server to P2PBroadcaster interface
	activeQueries map[string]*QueryState
	queriesMutex  sync.RWMutex
}

// NewDistributedLLMService creates a new DistributedLLMService.
// Takes P2PBroadcaster interface instead of concrete *p2p.Server and *p2p.NodeManager.
func NewDistributedLLMService(p2pAdapter P2PBroadcaster) *DistributedLLMService {
	if p2pAdapter == nil {
		// Log or handle critical error: p2pAdapter cannot be nil
		utils.LogError("FATAL: P2PBroadcaster is nil in NewDistributedLLMService. Service cannot function.")
		// Depending on desired behavior, could panic or return nil with error.
		// For now, let it proceed but it will likely fail later.
	}
	return &DistributedLLMService{
		p2pAdapter:    p2pAdapter,
		activeQueries: make(map[string]*QueryState),
	}
}

// Query sends a request to LLMs on the network and waits for responses.
func (s *DistributedLLMService) Query(requestPayload []byte, timeout time.Duration) ([]p2p.MCPResponse, error) {
	queryID := uuid.New().String()

	if s.p2pAdapter == nil {
		return nil, fmt.Errorf("P2P adapter is not initialized in DistributedLLMService")
	}

	originNodeID := s.p2pAdapter.GetNodeID() // Use P2PBroadcaster interface method

	// Create MCPQuery
	mcpQuery := p2p.MCPQuery{
		QueryID:      queryID,
		Timestamp:    time.Now().Unix(),
		OriginNodeID: originNodeID,
		Payload:      requestPayload,
		// Signature will be populated by the p2pServer's broadcast/send methods if not already set
	}

	// Sign the query (placeholder, actual signing should be more robust)
	// The p2p.BroadcastMCPQuery method already handles signing if Signature is empty.
	// We can pre-populate it here if we want this layer to explicitly own the signature creation.
	// For now, let's assume BroadcastMCPQuery handles it.
	// If specific signing logic is needed here:
	// nodeID, signatureString := s.p2pServer.SignMessage(mcpQuery.QueryID) // Assuming SignMessage exists and is appropriate
	// mcpQuery.Signature = p2p.MCPMessageSignature{NodeID: nodeID, Signature: signatureString}

	// Register the query
	queryState := &QueryState{
		Responses: make([]p2p.MCPResponse, 0),
		DoneChan:  make(chan struct{}),
		QueryID:   queryID,
		Timeout:   timeout,
		StartTime: time.Now(),
	}

	s.queriesMutex.Lock()
	s.activeQueries[queryID] = queryState
	s.queriesMutex.Unlock()

	utils.LogInfo("Service: Broadcasting MCPQuery QueryID: %s", queryID)
	s.p2pAdapter.BroadcastMCPQuery(&mcpQuery) // Use P2PBroadcaster interface method

	// Wait for responses or timeout
	select {
	case <-queryState.DoneChan:
		utils.LogInfo("Service: QueryID %s completed via DoneChan.", queryID)
	case <-time.After(timeout):
		utils.LogInfo("Service: QueryID %s timed out after %v.", queryID, timeout)
	}

	// Cleanup and collect responses
	s.queriesMutex.Lock()
	delete(s.activeQueries, queryID)
	// Consider closing DoneChan here carefully. If Query() is the sole owner of deletion and timeout, it's safer.
	// close(queryState.DoneChan) // Ensure this is safe from concurrent writes.
	s.queriesMutex.Unlock()

	queryState.Mutex.Lock()
	defer queryState.Mutex.Unlock()

	return queryState.Responses, nil
}

// ProcessIncomingResponse is called by the P2P layer when an MCPResponse is received.
func (s *DistributedLLMService) ProcessIncomingResponse(response *p2p.MCPResponse) {
	if response == nil {
		utils.LogInfo("Service: Received nil MCPResponse.")
		return
	}
	utils.LogInfo("Service: Processing incoming MCPResponse for QueryID: %s from ResponderID: %s", response.QueryID, response.ResponderNodeID)

	s.queriesMutex.RLock()
	queryState, exists := s.activeQueries[response.QueryID]
	s.queriesMutex.RUnlock()

	if !exists {
		utils.LogInfo("Service: Received MCPResponse for unknown or timed-out QueryID: %s", response.QueryID)
		return
	}

	queryState.Mutex.Lock()
	queryState.Responses = append(queryState.Responses, *response)
	numResponses := len(queryState.Responses)
	queryState.Mutex.Unlock()

	utils.LogInfo("Service: QueryID %s now has %d responses.", response.QueryID, numResponses)

	// Example completion condition: if we have received responses from a certain number of nodes
	// For now, we rely on the timeout in the Query method.
	// If we wanted to signal completion early:
	// if numResponses >= someThreshold { // someThreshold would be a configurable value
	//     // Safely try to send to DoneChan without blocking and prevent panic on closed channel
	//     select {
	//     case queryState.DoneChan <- struct{}{}:
	//         utils.LogInfo("Service: Signaled DoneChan for QueryID %s due to sufficient responses.", response.QueryID)
	//     default:
	//         // This default case prevents blocking if DoneChan is not ready (e.g., already signaled and closed, or buffer full if any)
	//         utils.LogInfo("Service: DoneChan for QueryID %s not signaled (already closed or not ready).", response.QueryID)
	//     }
	// }
}
