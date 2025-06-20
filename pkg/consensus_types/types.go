package consensus_types

// ConsensusType represents the type of consensus algorithm
type ConsensusType string

// Message represents a consensus message
type Message struct {
	Type      string `json:"type"`
	NodeID    string `json:"nodeId"`
	BlockHash string `json:"blockHash"`
	BlockType string `json:"blockType"`
	Data      any    `json:"data"`
	Signature string `json:"signature"`
}
