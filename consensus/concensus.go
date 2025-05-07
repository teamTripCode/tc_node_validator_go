package consensus

import (
	"tripcodechain_go/blockchain"
)

// ConsensusType represents the type of consensus algorithm
type ConsensusType string

const (
	// PBFT represents the Practical Byzantine Fault Tolerance consensus
	PBFT ConsensusType = "PBFT"
	// DPOS represents the Delegated Proof of Stake consensus
	DPOS ConsensusType = "DPOS"
)

// Message represents a consensus message
type Message struct {
	Type      string `json:"type"`
	NodeID    string `json:"nodeId"`
	BlockHash string `json:"blockHash"`
	BlockType string `json:"blockType"`
	Data      any    `json:"data"`
	Signature string `json:"signature"`
}

// Consensus interface defines methods that must be implemented by any consensus algorithm
type Consensus interface {
	// Initialize sets up the consensus algorithm
	Initialize(nodeID string) error

	// ValidateBlock validates a block according to the consensus rules
	ValidateBlock(block *blockchain.Block) bool

	// ProcessConsensusMessage processes an incoming consensus message
	ProcessConsensusMessage(message *Message) error

	// BroadcastPrepare broadcasts a prepare message for PBFT
	BroadcastPrepare(block *blockchain.Block) error

	// BroadcastCommit broadcasts a commit message for PBFT
	BroadcastCommit(block *blockchain.Block) error

	// IsValidator checks if the node is a validator in the current round
	IsValidator() bool

	// GetValidators returns the current set of validators
	GetValidators() []string

	// GetType returns the type of consensus algorithm
	GetType() ConsensusType
}
