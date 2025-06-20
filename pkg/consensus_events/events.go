package consensus_events

import "time"

// ConsensusBroadcaster defines the interface for broadcasting consensus messages.
type ConsensusBroadcaster interface {
	BroadcastPBFTMessage(data []byte) error
}

// ConsensusLogger defines the interface for logging PBFT specific events.
type ConsensusLogger interface {
	LogPBFTConsensusReached(nodeID, blockHash string, duration time.Duration)
	LogPBFTViewChange(nodeID, view string, duration time.Duration)
}
