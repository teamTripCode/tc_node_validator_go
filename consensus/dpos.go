// consensus/dpos.go
package consensus

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"tripcodechain_go/blockchain"
	"tripcodechain_go/utils"
)

// DelegateInfo stores information about a delegate in the DPoS system
type DelegateInfo struct {
	NodeID        string  `json:"nodeId"`
	Stake         float64 `json:"stake"`
	Votes         int     `json:"votes"`
	BlocksCreated int     `json:"blocksCreated"`
	IsActive      bool    `json:"isActive"`
	LastActive    string  `json:"lastActive"`
}

// DPoS implements the Delegated Proof of Stake consensus algorithm
type DPoS struct {
	NodeID          string                   // ID of the current node
	Delegates       map[string]*DelegateInfo // Map of delegate IDs to their info
	ActiveDelegates []string                 // List of active delegates for current round
	CurrentProducer string                   // Current block producer
	RoundLength     int                      // Number of blocks in a round
	BlockTime       int                      // Time between blocks in seconds
	CurrentRound    int                      // Current round number
	CurrentSlot     int                      // Current slot in the round
	LastBlockTime   time.Time                // Time of last block
	StakeByNodeID   map[string]float64       // Map of node IDs to their stake
	VotesByNodeID   map[string]string        // Map of voter IDs to delegate IDs they voted for
	mutex           sync.RWMutex             // Mutex for thread safety
	initialized     bool                     // Whether the DPoS has been initialized
}

// NewDPoS creates a new DPoS consensus instance
func NewDPoS() *DPoS {
	return &DPoS{
		Delegates:       make(map[string]*DelegateInfo),
		ActiveDelegates: make([]string, 0),
		RoundLength:     4,                        // 4 delegates per round
		BlockTime:       3,                        // 3 seconds between blocks
		StakeByNodeID:   make(map[string]float64), // Initialize stake map
		VotesByNodeID:   make(map[string]string),  // Initialize votes map
	}
}

// Initialize sets up the DPoS consensus algorithm
func (d *DPoS) Initialize(nodeID string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.initialized {
		return nil
	}

	d.NodeID = nodeID
	d.LastBlockTime = time.Now()

	// For demo purposes, we'll create some delegates
	delegatesList := []string{"localhost:3000", "localhost:3001", "localhost:3002", "localhost:3003"}

	// Initialize delegates with random stake
	for _, id := range delegatesList {
		stake := 100.0 + float64(rand.Intn(900))
		d.Delegates[id] = &DelegateInfo{
			NodeID:        id,
			Stake:         stake,
			Votes:         0,
			BlocksCreated: 0,
			IsActive:      false,
			LastActive:    time.Now().UTC().Format(time.RFC3339),
		}
		d.StakeByNodeID[id] = stake
	}

	// Set up initial active delegates
	d.UpdateActiveDelegates()
	d.initialized = true

	utils.LogInfo("DPoS consensus initialized. Node ID: %s", d.NodeID)
	utils.LogInfo("Active delegates: %v", d.ActiveDelegates)

	// Start the block production schedule
	go d.runBlockProductionSchedule()

	return nil
}

// runBlockProductionSchedule runs the block production schedule
func (d *DPoS) runBlockProductionSchedule() {
	ticker := time.NewTicker(time.Duration(d.BlockTime) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		d.updateSchedule()
	}
}

// updateSchedule updates the block production schedule
func (d *DPoS) updateSchedule() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Update current slot
	elapsed := time.Since(d.LastBlockTime).Seconds()
	slotsElapsed := int(elapsed) / d.BlockTime

	if slotsElapsed > 0 {
		d.CurrentSlot = (d.CurrentSlot + slotsElapsed) % d.RoundLength
		d.LastBlockTime = time.Now()

		// If we've completed a round, update active delegates
		if d.CurrentSlot == 0 {
			d.CurrentRound++
			d.UpdateActiveDelegates()
		}

		// Update current producer
		d.CurrentProducer = d.ActiveDelegates[d.CurrentSlot]

		utils.LogInfo("DPoS schedule updated. Round: %d, Slot: %d, Producer: %s",
			d.CurrentRound, d.CurrentSlot, d.CurrentProducer)

		// If this node is the current producer, it should produce a block
		if d.CurrentProducer == d.NodeID {
			utils.LogInfo("This node is the current block producer!")
			// In a real implementation, this would trigger block production
		}
	}
}

// UpdateActiveDelegates selects active delegates based on votes and stake
func (d *DPoS) UpdateActiveDelegates() {
	// Sort delegates by votes and stake
	type delegateScore struct {
		id    string
		score float64
	}

	var scores []delegateScore
	for id, info := range d.Delegates {
		// Score is a combination of votes and stake
		score := float64(info.Votes)*10.0 + info.Stake
		scores = append(scores, delegateScore{id, score})
	}

	// Sort by score (in a real implementation, this would be more sophisticated)
	// This is a simple bubble sort for demonstration
	for i := 0; i < len(scores); i++ {
		for j := 0; j < len(scores)-i-1; j++ {
			if scores[j].score < scores[j+1].score {
				scores[j], scores[j+1] = scores[j+1], scores[j]
			}
		}
	}

	// Select top delegates
	d.ActiveDelegates = make([]string, d.RoundLength)
	for i := 0; i < d.RoundLength && i < len(scores); i++ {
		d.ActiveDelegates[i] = scores[i].id
		d.Delegates[scores[i].id].IsActive = true
		d.Delegates[scores[i].id].LastActive = time.Now().UTC().Format(time.RFC3339)
	}
}

// ValidateBlock validates a block according to DPoS rules
func (d *DPoS) ValidateBlock(block *blockchain.Block) bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	// Check that the block is produced by an active delegate
	isActiveDelegate := false
	for _, delegateID := range d.ActiveDelegates {
		if delegateID == block.Validator {
			isActiveDelegate = true
			break
		}
	}

	if !isActiveDelegate {
		utils.LogError("Block validator %s is not an active delegate", block.Validator)
		return false
	}

	// Validate block hash
	if block.Hash != block.CalculateHash() {
		utils.LogError("Block hash is invalid")
		return false
	}

	return true
}

// ProcessConsensusMessage processes an incoming DPoS consensus message
func (d *DPoS) ProcessConsensusMessage(message *Message) error {
	switch message.Type {
	case "Vote":
		return d.handleVote(message)
	case "NewDelegate":
		return d.handleNewDelegate(message)
	default:
		return fmt.Errorf("unknown message type: %s", message.Type)
	}
}

// handleVote processes a vote message
func (d *DPoS) handleVote(message *Message) error {
	voteData, ok := message.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid vote data format")
	}

	voterID, ok := voteData["voterId"].(string)
	if !ok {
		return fmt.Errorf("invalid voter ID format")
	}

	delegateID, ok := voteData["delegateId"].(string)
	if !ok {
		return fmt.Errorf("invalid delegate ID format")
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Check if this delegate exists
	delegate, exists := d.Delegates[delegateID]
	if !exists {
		return fmt.Errorf("delegate %s does not exist", delegateID)
	}

	// Check if voter has already voted
	previousDelegate, voted := d.VotesByNodeID[voterID]
	if voted {
		// Remove previous vote
		d.Delegates[previousDelegate].Votes--
	}

	// Record new vote
	d.VotesByNodeID[voterID] = delegateID
	delegate.Votes++

	utils.LogInfo("Vote recorded from %s for delegate %s", voterID, delegateID)
	return nil
}

// handleNewDelegate processes a new delegate registration
func (d *DPoS) handleNewDelegate(message *Message) error {
	delegateData, ok := message.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid delegate data format")
	}

	nodeID, ok := delegateData["nodeId"].(string)
	if !ok {
		return fmt.Errorf("invalid node ID format")
	}

	stakeFloat, ok := delegateData["stake"].(float64)
	if !ok {
		return fmt.Errorf("invalid stake format")
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Register new delegate
	d.Delegates[nodeID] = &DelegateInfo{
		NodeID:        nodeID,
		Stake:         stakeFloat,
		Votes:         0,
		BlocksCreated: 0,
		IsActive:      false,
		LastActive:    time.Now().UTC().Format(time.RFC3339),
	}

	d.StakeByNodeID[nodeID] = stakeFloat

	utils.LogInfo("New delegate registered: %s with stake %.2f", nodeID, stakeFloat)
	return nil
}

// RegisterVote registers a vote for a delegate
func (d *DPoS) RegisterVote(voterID, delegateID string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Check if this delegate exists
	delegate, exists := d.Delegates[delegateID]
	if !exists {
		return fmt.Errorf("delegate %s does not exist", delegateID)
	}

	// Check if voter has already voted
	previousDelegate, voted := d.VotesByNodeID[voterID]
	if voted {
		// Remove previous vote
		d.Delegates[previousDelegate].Votes--
	}

	// Record new vote
	d.VotesByNodeID[voterID] = delegateID
	delegate.Votes++

	return nil
}

// RegisterDelegate registers a new delegate
func (d *DPoS) RegisterDelegate(nodeID string, stake float64) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Register new delegate
	d.Delegates[nodeID] = &DelegateInfo{
		NodeID:        nodeID,
		Stake:         stake,
		Votes:         0,
		BlocksCreated: 0,
		IsActive:      false,
		LastActive:    time.Now().UTC().Format(time.RFC3339),
	}

	d.StakeByNodeID[nodeID] = stake

	return nil
}

// BroadcastPrepare is a no-op for DPoS (PBFT interface compatibility)
func (d *DPoS) BroadcastPrepare(block *blockchain.Block) error {
	// DPoS doesn't use prepare messages (PBFT interface compatibility)
	return nil
}

// BroadcastCommit is a no-op for DPoS (PBFT interface compatibility)
func (d *DPoS) BroadcastCommit(block *blockchain.Block) error {
	// DPoS doesn't use commit messages (PBFT interface compatibility)
	return nil
}

// IsValidator checks if the node is an active delegate
func (d *DPoS) IsValidator() bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	for _, delegateID := range d.ActiveDelegates {
		if delegateID == d.NodeID {
			return true
		}
	}
	return false
}

// GetValidators returns the current set of active delegates
func (d *DPoS) GetValidators() []string {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.ActiveDelegates
}

// GetType returns the type of consensus algorithm
func (d *DPoS) GetType() ConsensusType {
	return DPOS
}

// GetCurrentProducer returns the current block producer
func (d *DPoS) GetCurrentProducer() string {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.CurrentProducer
}

// GetDelegateInfo returns information about a delegate
func (d *DPoS) GetDelegateInfo(nodeID string) (*DelegateInfo, bool) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	delegate, exists := d.Delegates[nodeID]
	return delegate, exists
}

// GetAllDelegates returns information about all delegates
func (d *DPoS) GetAllDelegates() map[string]*DelegateInfo {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	// Create a copy to avoid concurrency issues
	delegates := make(map[string]*DelegateInfo)
	for id, delegate := range d.Delegates {
		delegates[id] = &DelegateInfo{
			NodeID:        delegate.NodeID,
			Stake:         delegate.Stake,
			Votes:         delegate.Votes,
			BlocksCreated: delegate.BlocksCreated,
			IsActive:      delegate.IsActive,
			LastActive:    delegate.LastActive,
		}
	}

	return delegates
}
