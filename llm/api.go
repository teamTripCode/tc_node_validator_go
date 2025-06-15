package llm

import (
	"encoding/json"
	"net/http"
	"time"

	"tripcodechain_go/utils"
	// Assuming p2p.MCPResponse is accessible for encoding responses.
	// If not, a DTO might be needed here.
)

const defaultQueryTimeout = 10 * time.Second // Example timeout

// LLMAPIHandler handles API requests for the DistributedLLMService.
type LLMAPIHandler struct {
	service *DistributedLLMService
}

// NewLLMAPIHandler creates a new LLMAPIHandler.
func NewLLMAPIHandler(service *DistributedLLMService) *LLMAPIHandler {
	if service == nil {
		// This should ideally not happen if initialization is done correctly.
		// Consider logging a fatal error or returning an error.
		utils.LogError("LLMAPIHandler: DistributedLLMService is nil during initialization.")
	}
	return &LLMAPIHandler{
		service: service,
	}
}

// QueryRequestPayload defines the expected structure for the client's query request.
type QueryRequestPayload struct {
	Prompt     string                 `json:"prompt"`
	Model      string                 `json:"model,omitempty"`      // Optional: specify a model
	Parameters map[string]interface{} `json:"parameters,omitempty"` // Optional: other parameters
	// The actual payload sent via MCPQuery will be this entire struct marshalled,
	// or just a part of it, depending on design. For now, let's assume the core
	// part is the requestPayload []byte for the service.Query method.
	// Here, we define what the API client sends to this handler.
}

// HandleQuery handles incoming queries from clients.
func (h *LLMAPIHandler) HandleQuery(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		utils.LogError("LLMAPIHandler: Service not initialized.")
		http.Error(w, "LLM service is not available", http.StatusInternalServerError)
		return
	}

	var requestPayload QueryRequestPayload // This is what the API client sends
	if err := json.NewDecoder(r.Body).Decode(&requestPayload); err != nil {
		utils.LogError("LLMAPIHandler: Error decoding request body: %v", err)
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// The service.Query method expects []byte. We need to decide what part of
	// QueryRequestPayload to pass or if the whole thing should be marshalled.
	// For now, let's marshal the received payload (or just its core part) to []byte.
	// This marshalled data will be the Payload in MCPQuery.
	mcpPayloadBytes, err := json.Marshal(requestPayload)
	if err != nil {
		utils.LogError("LLMAPIHandler: Error marshalling request payload for MCP: %v", err)
		http.Error(w, "Internal server error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	utils.LogInfo("LLMAPIHandler: Received query: %s", requestPayload.Prompt)

	// Call the DistributedLLMService's Query method
	// defaultTimeout can be a constant or configurable
	responses, err := h.service.Query(mcpPayloadBytes, defaultQueryTimeout)
	if err != nil {
		utils.LogError("LLMAPIHandler: Error from service.Query: %v", err)
		http.Error(w, "Error processing query: "+err.Error(), http.StatusInternalServerError)
		return
	}

	utils.LogInfo("LLMAPIHandler: Query processed. Responses collected: %d", len(responses))

	// Encode the collected responses and send back to the client
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(responses); err != nil {
		utils.LogError("LLMAPIHandler: Error encoding responses: %v", err)
		// http.Error is not ideal here as headers might have been written
		// Best effort to inform client if possible, or just log.
	}
}
