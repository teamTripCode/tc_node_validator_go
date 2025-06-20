package consensus

import (
	"tripcodechain_go/blockchain"
	"tripcodechain_go/pkg/consensus_types" // Added import
)

// Consensus interface defines methods that must be implemented by any consensus algorithm
type Consensus interface {
	// Initialize sets up the consensus algorithm
	Initialize(nodeID string) error

	// ValidateBlock validates a block according to the consensus rules
	ValidateBlock(block *blockchain.Block) bool

	// ProcessConsensusMessage processes an incoming consensus message
	ProcessConsensusMessage(message *consensus_types.Message) error // Updated type

	// BroadcastPrepare broadcasts a prepare message for PBFT
	BroadcastPrepare(block *blockchain.Block) error

	// BroadcastCommit broadcasts a commit message for PBFT
	BroadcastCommit(block *blockchain.Block) error

	// IsValidator checks if the node is a validator in the current round
	IsValidator() bool

	// GetValidators returns the current set of validators
	GetValidators() []string

	// GetType returns the type of consensus algorithm
	GetType() consensus_types.ConsensusType // Updated type
}
