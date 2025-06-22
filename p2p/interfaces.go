package p2p

// LocalQueryProcessor defines the interface for local query processing.
// This interface will be implemented by the expert system's local processor,
// replacing the previous LocalLLMProcessor.
// It allows the p2p server to delegate query processing to a dedicated module.
type LocalQueryProcessor interface {
	ProcessQuery(payload []byte) ([]byte, error) // Payload is the query, returns a response or error
}
