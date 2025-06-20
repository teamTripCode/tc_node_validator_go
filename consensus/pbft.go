package consensus

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"slices"
	"tripcodechain_go/blockchain"
	consensus_events "tripcodechain_go/pkg/consensus_events" // Added for PBFT logging
	"tripcodechain_go/pkg/consensus_types" // Added for Message and ConsensusType
	"tripcodechain_go/utils"
)

// PBFTState represents the state of a block in the PBFT process
type PBFTState struct {
	Block           *blockchain.Block `json:"block"`
	PrePrepared     bool              `json:"prePrepared"`
	PrepareCount    int               `json:"prepareCount"`
	CommitCount     int               `json:"commitCount"`
	Prepared        bool              `json:"prepared"`
	Committed       bool              `json:"committed"`
	PreparedBy      map[string]bool   `json:"preparedBy"`
	CommittedBy     map[string]bool   `json:"committedBy"`
	ViewChangeCount int               `json:"viewChangeCount"`
}

// PBFT implements the Practical Byzantine Fault Tolerance consensus algorithm
type PBFT struct {
	NodeID         string                 // ID of the current node
	Primary        string                 // ID of the primary node
	Validators     []string               // List of validator nodes
	BlockStates    map[string]*PBFTState  // Map of block hash to state
	View           int                    // Current view number
	SequenceNumber int                    // Sequence number for PBFT messages
	mutex          sync.RWMutex           // Mutex for thread safety
	viewChanges    map[string]map[int]int // Map of node ID to view number to count
	Broadcaster    consensus_events.ConsensusBroadcaster
	Logger         consensus_events.ConsensusLogger
}

// NewPBFT creates a new PBFT consensus instance
func NewPBFT(broadcaster consensus_events.ConsensusBroadcaster, logger consensus_events.ConsensusLogger) *PBFT {
	return &PBFT{
		BlockStates: make(map[string]*PBFTState),
		View:        0,
		viewChanges: make(map[string]map[int]int),
		Broadcaster: broadcaster,
		Logger:      logger,
	}
}

// Initialize sets up the PBFT consensus algorithm
func (p *PBFT) Initialize(nodeID string) error {
	p.NodeID = nodeID
	p.Validators = []string{"localhost:3000", "localhost:3001", "localhost:3002", "localhost:3003"}
	p.Primary = p.Validators[p.View%len(p.Validators)]
	utils.LogInfo("PBFT consensus initialized. Node ID: %s, Primary: %s", p.NodeID, p.Primary)
	return nil
}

// ValidateBlock validates a block according to PBFT rules
func (p *PBFT) ValidateBlock(block *blockchain.Block) bool {
	// Basic block validation
	return block.Hash == block.CalculateHash()
}

// GetBlockState returns the PBFT state for a block, creating it if it doesn't exist
func (p *PBFT) GetBlockState(block *blockchain.Block) *PBFTState {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	blockHash := block.Hash
	if state, exists := p.BlockStates[blockHash]; exists {
		return state
	}

	// Create new state
	state := &PBFTState{
		Block:       block,
		PreparedBy:  make(map[string]bool),
		CommittedBy: make(map[string]bool),
	}
	p.BlockStates[blockHash] = state
	return state
}

// ProcessConsensusMessage processes an incoming PBFT consensus message
func (p *PBFT) ProcessConsensusMessage(message *consensus_types.Message) error {
	var block blockchain.Block
	blockData, err := json.Marshal(message.Data)
	if err != nil {
		return fmt.Errorf("error marshaling block data: %v", err)
	}

	if err := json.Unmarshal(blockData, &block); err != nil {
		return fmt.Errorf("error unmarshaling block: %v", err)
	}

	switch message.Type {
	case "PrePrepare":
		return p.handlePrePrepare(&block, message.NodeID)
	case "Prepare":
		return p.handlePrepare(&block, message.NodeID)
	case "Commit":
		return p.handleCommit(&block, message.NodeID)
	case "ViewChange":
		return p.handleViewChange(message)
	default:
		return fmt.Errorf("unknown message type: %s", message.Type)
	}
}

// handlePrePrepare processes a pre-prepare message
func (p *PBFT) handlePrePrepare(block *blockchain.Block, nodeID string) error {
	if nodeID != p.Primary {
		return fmt.Errorf("received pre-prepare from non-primary node: %s", nodeID)
	}

	if !p.ValidateBlock(block) {
		return fmt.Errorf("invalid block in pre-prepare from %s", nodeID)
	}

	state := p.GetBlockState(block)

	p.mutex.Lock()
	defer p.mutex.Unlock()

	if state.PrePrepared {
		return nil // Already pre-prepared
	}

	state.PrePrepared = true
	utils.LogInfo("Received valid pre-prepare for block %s from primary %s", block.Hash, nodeID)

	// Send prepare message
	return p.BroadcastPrepare(block)
}

// handlePrepare processes a prepare message
func (p *PBFT) handlePrepare(block *blockchain.Block, nodeID string) error {
	state := p.GetBlockState(block)

	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Check if already prepared by this node
	if state.PreparedBy[nodeID] {
		return nil
	}

	// Record prepare vote
	state.PreparedBy[nodeID] = true
	state.PrepareCount++

	utils.LogInfo("Received prepare for block %s from node %s (%d/%d)",
		block.Hash, nodeID, state.PrepareCount, len(p.Validators))

	// Check if we have enough prepare messages (2f+1 in a system that can tolerate f faults)
	// For simplicity, we're using 2/3 of validators
	threshold := 2 * len(p.Validators) / 3
	if state.PrepareCount >= threshold && !state.Prepared {
		state.Prepared = true
		utils.LogInfo("Block %s prepared with %d votes", block.Hash, state.PrepareCount)

		// Move to commit phase
		return p.BroadcastCommit(block)
	}

	return nil
}

// handleCommit processes a commit message
func (p *PBFT) handleCommit(block *blockchain.Block, nodeID string) error {
	state := p.GetBlockState(block)

	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Check if already committed by this node
	if state.CommittedBy[nodeID] {
		return nil
	}

	// Record commit vote
	state.CommittedBy[nodeID] = true
	state.CommitCount++

	utils.LogInfo("Received commit for block %s from node %s (%d/%d)",
		block.Hash, nodeID, state.CommitCount, len(p.Validators))

	// Check if we have enough commit messages
	threshold := 2 * len(p.Validators) / 3
	if state.CommitCount >= threshold && !state.Committed {
		state.Committed = true
		utils.LogInfo("Block %s committed with %d votes", block.Hash, state.CommitCount)
		p.Logger.LogPBFTConsensusReached(p.NodeID, block.Hash, time.Duration(0)) // New log call

		// The block can now be added to the blockchain
		return nil
	}

	return nil
}

// handleViewChange processes a view change message
func (p *PBFT) handleViewChange(message *consensus_types.Message) error {
	viewData, ok := message.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid view change data format")
	}

	viewNumber, ok := viewData["view"].(float64)
	if !ok {
		return fmt.Errorf("invalid view number format")
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Initialize view change count for this node and view if not exist
	if _, exists := p.viewChanges[message.NodeID]; !exists {
		p.viewChanges[message.NodeID] = make(map[int]int)
	}

	viewInt := int(viewNumber)
	p.viewChanges[message.NodeID][viewInt]++

	// Count how many nodes agree on this view
	viewCount := 0
	for _, nodeViews := range p.viewChanges {
		if count, exists := nodeViews[viewInt]; exists {
			viewCount += count
		}
	}

	// If enough nodes agree, change the view
	threshold := 2 * len(p.Validators) / 3
	if viewCount >= threshold && viewInt > p.View {
		p.View = viewInt
		p.Primary = p.Validators[p.View%len(p.Validators)]
		utils.LogInfo("View changed to %d. New primary: %s", p.View, p.Primary)
		p.Logger.LogPBFTViewChange(p.NodeID, fmt.Sprintf("%d", p.View), time.Duration(0)) // New log call
	}

	return nil
}

// InitiateViewChange starts a view change process
func (p *PBFT) InitiateViewChange() error {
	p.mutex.Lock()
	p.View++
	newView := p.View
	p.mutex.Unlock()

	viewChangeData := map[string]interface{}{
		"view":      newView,
		"nodeId":    p.NodeID,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	message := &consensus_types.Message{ // Changed to consensus_types.Message
		Type:   "ViewChange",
		NodeID: p.NodeID,
		Data:   viewChangeData,
	}

	// Hash the message data for signature
	dataBytes, _ := json.Marshal(viewChangeData)
	hash := sha256.Sum256(dataBytes)
	message.Signature = hex.EncodeToString(hash[:])

	// In a real implementation, this would be signed with the node's private key

	// Here, we would broadcast the message to all validators
	utils.LogInfo("Initiated view change to view %d", newView)
	jsonData, err := json.Marshal(message)
	if err != nil {
		utils.LogError("PBFT: Error marshalling ViewChange message: %v", err)
		return err // Or handle error appropriately
	}
	if err := p.Broadcaster.BroadcastPBFTMessage(jsonData); err != nil {
		utils.LogError("PBFT: Error broadcasting ViewChange message: %v", err)
		// Handle error appropriately, e.g., retry or log
	}

	return nil
}

// BroadcastPrepare broadcasts a prepare message for PBFT
func (p *PBFT) BroadcastPrepare(block *blockchain.Block) error {
	prepareData := map[string]interface{}{
		"blockHash": block.Hash,
		"nodeId":    p.NodeID,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	message := &consensus_types.Message{ // Changed to consensus_types.Message
		Type:      "Prepare",
		NodeID:    p.NodeID,
		BlockHash: block.Hash,
		BlockType: string(block.Type),
		Data:      block,
	}

	// Hash the message data for signature
	dataBytes, _ := json.Marshal(prepareData)
	hash := sha256.Sum256(dataBytes)
	message.Signature = hex.EncodeToString(hash[:])

	utils.LogInfo("PBFT: Broadcasting PREPARE for block %s", block.Hash)
	jsonData, err := json.Marshal(message)
	if err != nil {
		utils.LogError("PBFT: Error marshalling Prepare message: %v", err)
		return err // Or handle error appropriately
	}
	if err := p.Broadcaster.BroadcastPBFTMessage(jsonData); err != nil {
		utils.LogError("PBFT: Error broadcasting Prepare message: %v", err)
		// Handle error appropriately
	}

	return nil
}

// BroadcastCommit broadcasts a commit message for PBFT
func (p *PBFT) BroadcastCommit(block *blockchain.Block) error {
	commitData := map[string]interface{}{
		"blockHash": block.Hash,
		"nodeId":    p.NodeID,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	message := &consensus_types.Message{ // Changed to consensus_types.Message
		Type:      "Commit",
		NodeID:    p.NodeID,
		BlockHash: block.Hash,
		BlockType: string(block.Type),
		Data:      block,
	}

	// Hash the message data for signature
	dataBytes, _ := json.Marshal(commitData)
	hash := sha256.Sum256(dataBytes)
	message.Signature = hex.EncodeToString(hash[:])

	utils.LogInfo("PBFT: Broadcasting COMMIT for block %s", block.Hash)
	jsonData, err := json.Marshal(message)
	if err != nil {
		utils.LogError("PBFT: Error marshalling Commit message: %v", err)
		return err // Or handle error appropriately
	}
	if err := p.Broadcaster.BroadcastPBFTMessage(jsonData); err != nil {
		utils.LogError("PBFT: Error broadcasting Commit message: %v", err)
		// Handle error appropriately
	}

	return nil
}

// IsValidator checks if the node is a validator in the current round
func (p *PBFT) IsValidator() bool {
	return slices.Contains(p.Validators, p.NodeID)
}

// GetValidators returns the current set of validators
func (p *PBFT) GetValidators() []string {
	return p.Validators
}

// GetType returns the type of consensus algorithm
func (p *PBFT) GetType() consensus_types.ConsensusType { // Changed to consensus_types.ConsensusType
	return consensus_types.ConsensusType("PBFT")
}
