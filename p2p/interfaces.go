package p2p

// MCPResponseProcessor defines the interface for processing incoming MCP responses.
// This interface is typically implemented by a higher-level service (e.g., LLMService)
// and used by the P2P server to decouple message handling.
type MCPResponseProcessor interface {
	ProcessIncomingResponse(response *MCPResponse)
}

// LocalLLMProcessor defines the interface for local LLM query processing.
// This allows the p2p server to use a local LLM without directly depending on the llm package.
type LocalLLMProcessor interface {
	QueryLLM(payload []byte) ([]byte, error)
}
