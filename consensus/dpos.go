// consensus/dpos.go
package consensus

import (
	"fmt"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"slices"
	"tripcodechain_go/blockchain"
	"tripcodechain_go/currency"
	"tripcodechain_go/utils"
)

// DelegateInfo stores information about a delegate in the DPoS system
type DelegateInfo struct {
	NodeID          string  `json:"nodeId"`
	Stake           float64 `json:"stake"`
	Votes           int     `json:"votes"`
	BlocksCreated   int     `json:"blocksCreated"`
	IsActive        bool    `json:"isActive"`
	LastActive      string  `json:"lastActive"`
	Reliability     float64 `json:"reliability"`     // Percentage of blocks produced on time
	RewardAccrued   float64 `json:"rewardAccrued"`   // Total rewards earned
	VoterCount      int     `json:"voterCount"`      // Number of unique voters
	MissedBlocks    int     `json:"missedBlocks"`    // Number of blocks missed when scheduled
	CreationTime    string  `json:"creationTime"`    // When the delegate was registered
	RepresentativID string  `json:"representativId"` // Optional real-world identity reference
}

// DPoS implements the Delegated Proof of Stake consensus algorithm
type DPoS struct {
	NodeID           string                   // ID of the current node
	Delegates        map[string]*DelegateInfo // Map of delegate IDs to their info
	ActiveDelegates  []string                 // List of active delegates for current round
	CurrentProducer  string                   // Current block producer
	RoundLength      int                      // Number of blocks in a round
	BlockTime        int                      // Time between blocks in seconds
	CurrentRound     int                      // Current round number
	CurrentSlot      int                      // Current slot in the round
	LastBlockTime    time.Time                // Time of last block
	StakeByNodeID    map[string]float64       // Map of node IDs to their stake
	VotesByNodeID    map[string]string        // Map of voter IDs to delegate IDs they voted for
	RewardPool       float64                  // Total available rewards in the pool
	mutex            sync.RWMutex             // Mutex for thread safety
	initialized      bool                     // Whether the DPoS has been initialized
	currencyManager  *currency.CurrencyManager
	validators       map[string]*big.Int  // Validadores y su stake
	bannedValidators map[string]time.Time // Map of banned validators and their ban expiration time
}

// ValidatorInfo represents information about a validator
type ValidatorInfo struct {
	Address string `json:"address"`
	Stake   string `json:"stake"`
}

// NewDPoS creates a new DPoS consensus instance
func NewDPoS(currency *currency.CurrencyManager) *DPoS {
	return &DPoS{
		Delegates:        make(map[string]*DelegateInfo),
		ActiveDelegates:  make([]string, 0),
		RoundLength:      21,                       // 21 delegates per round - estándar de DPoS
		BlockTime:        3,                        // 3 seconds between blocks
		StakeByNodeID:    make(map[string]float64), // Initialize stake map
		VotesByNodeID:    make(map[string]string),  // Initialize votes map
		RewardPool:       1000.0,                   // Initial reward pool
		validators:       make(map[string]*big.Int),
		bannedValidators: make(map[string]time.Time),
	}
}

func (d *DPoS) AddValidator(address string) {
	stake := d.currencyManager.GetStake(address).Int
	minimumStakeFloat := big.NewFloat(currency.MinimumStake)
	minimumStake := new(big.Int)
	minimumStakeFloat.Int(minimumStake)
	if stake.Cmp(minimumStake) >= 0 {
		d.validators[address] = stake
	}
}

func (d *DPoS) selectBlockProducer() string {
	// El validador con mayor stake produce el bloque
	var selected string
	maxStake := big.NewInt(0)

	for addr, stake := range d.validators {
		if stake.Cmp(maxStake) > 0 {
			maxStake = stake
			selected = addr
		}
	}

	return selected
}

func (d *DPoS) SlashValidator(address string, severity string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// 1. Verificar que el validador existe
	delegate, exists := d.Delegates[address]
	if !exists {
		return fmt.Errorf("validator %s not found", address)
	}

	// 2. Determinar porcentaje de penalización según severidad
	penaltyPercentage := 0.1 // 10% por defecto
	switch severity {
	case "minor":
		penaltyPercentage = 0.1
		delegate.Reliability *= 0.95 // Reducir 5% de confiabilidad
	case "severe":
		penaltyPercentage = 0.3
		delegate.Reliability *= 0.80 // Reducir 20% de confiabilidad
	case "critical":
		penaltyPercentage = 0.5
		delegate.Reliability = 0 // Confiabilidad a cero
	}

	// 3. Calcular cantidad a quemar
	stake := d.currencyManager.GetStake(address)
	penaltyAmount := new(big.Float).Mul(
		new(big.Float).SetInt(stake.Int),
		big.NewFloat(penaltyPercentage),
	)

	penaltyInt, _ := penaltyAmount.Int(nil)
	penalty := currency.NewBalanceFromBigInt(penaltyInt)

	// 4. Aplicar penalización
	if err := d.currencyManager.BurnTokens(address, penalty); err != nil {
		return err
	}

	// 5. Actualizar stake y reputación
	penaltyFloat, _ := penaltyAmount.Float64()
	delegate.Stake -= penaltyFloat
	delegate.MissedBlocks += 1

	// 6. Registrar evento de seguridad
	utils.LogSecurityEvent("validator_slashed", map[string]interface{}{
		"validator":       address,
		"severity":        severity,
		"penalty_amount":  penalty.TripCoinString(),
		"new_reliability": delegate.Reliability,
		"remaining_stake": delegate.Stake,
	})

	// 7. Ban temporal por baja confiabilidad
	if delegate.Reliability < 50 {
		banDuration := time.Hour * 24
		utils.LogSecurityEvent("validator_banned", map[string]interface{}{
			"validator": address,
			"duration":  banDuration.String(),
			"reason":    "low reliability after slashing",
		})

		// Agregar a lista de baneados
		d.bannedValidators[address] = time.Now().Add(banDuration)

		// Eliminar de validadores activos
		delete(d.validators, address)
		for i, id := range d.ActiveDelegates {
			if id == address {
				d.ActiveDelegates = append(d.ActiveDelegates[:i], d.ActiveDelegates[i+1:]...)
				break
			}
		}

		// Programar rehabilitación automática
		time.AfterFunc(banDuration, func() {
			d.mutex.Lock()
			defer d.mutex.Unlock()
			delete(d.bannedValidators, address)
			d.validators[address] = stake.Sub(penalty).Int
			utils.LogSecurityEvent("validator_reinstated", map[string]interface{}{
				"validator": address,
			})
		})
	}

	// 8. Actualizar listas de delegados
	d.UpdateActiveDelegates()

	return nil
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

	// Creamos delegados para simulación
	// En producción, estos se registrarían dinámicamente
	delegatesList := []string{
		"localhost:3000", "localhost:3001", "localhost:3002", "localhost:3003",
		"localhost:3004", "localhost:3005", "localhost:3006", "localhost:3007",
		"localhost:3008", "localhost:3009", "localhost:3010", "localhost:3011",
		"localhost:3012", "localhost:3013", "localhost:3014", "localhost:3015",
		"localhost:3016", "localhost:3017", "localhost:3018", "localhost:3019",
		"localhost:3020", "localhost:3021", "localhost:3022", "localhost:3023",
	}

	// Initialize delegates with random stake
	now := time.Now().UTC().Format(time.RFC3339)
	for _, id := range delegatesList {
		stake := 100.0 + float64(rand.Intn(900))
		reliability := 95.0 + float64(rand.Intn(5)) // 95-100% reliability

		d.Delegates[id] = &DelegateInfo{
			NodeID:        id,
			Stake:         stake,
			Votes:         0,
			BlocksCreated: 0,
			IsActive:      false,
			LastActive:    now,
			Reliability:   reliability,
			RewardAccrued: 0.0,
			MissedBlocks:  0,
			CreationTime:  now,
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
			// Distribute rewards at the end of each round
			d.distributeRewards()
		}

		// Update current producer
		d.CurrentProducer = d.ActiveDelegates[d.CurrentSlot]

		utils.LogInfo("DPoS schedule updated. Round: %d, Slot: %d, Producer: %s",
			d.CurrentRound, d.CurrentSlot, d.CurrentProducer)

		// If this node is the current producer, it should produce a block
		if d.CurrentProducer == d.NodeID {
			utils.LogInfo("This node is the current block producer!")
			// In a real implementation, this would trigger block production

			// Record that this delegate produced a block
			if delegate, exists := d.Delegates[d.NodeID]; exists {
				delegate.BlocksCreated++
				delegate.LastActive = time.Now().UTC().Format(time.RFC3339)
			}
		}
	}
}

// distributeRewards distributes rewards to active delegates based on their performance
func (d *DPoS) distributeRewards() {
	// Calculate total rewards to distribute this round
	roundReward := 10.0 * float64(d.RoundLength) // Example: 10 tokens per block

	// Distribute rewards to active delegates
	for _, delegateID := range d.ActiveDelegates {
		if delegate, exists := d.Delegates[delegateID]; exists {
			// Calculate reward based on reliability and blocks created
			reward := roundReward * (delegate.Reliability / 100.0) / float64(d.RoundLength)
			delegate.RewardAccrued += reward

			utils.LogInfo("Delegate %s received reward: %.2f tokens", delegateID, reward)
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
		// Fórmula: Votos * 10 + Stake + (Reliability * 0.5)
		score := float64(info.Votes)*10.0 + info.Stake + (info.Reliability * 0.5)
		scores = append(scores, delegateScore{id, score})
	}

	// Sort by score (bubble sort for demonstration)
	for i := range scores {
		for j := range len(scores) - i - 1 {
			if scores[j].score < scores[j+1].score {
				scores[j], scores[j+1] = scores[j+1], scores[j]
			}
		}
	}

	// Reset active status
	for _, delegate := range d.Delegates {
		delegate.IsActive = false
	}

	// Select top delegates
	numActive := min(len(scores), d.RoundLength)

	d.ActiveDelegates = make([]string, numActive)
	for i := range numActive {
		d.ActiveDelegates[i] = scores[i].id
		d.Delegates[scores[i].id].IsActive = true
		d.Delegates[scores[i].id].LastActive = time.Now().UTC().Format(time.RFC3339)
	}

	utils.LogInfo("Updated active delegates. Total: %d", len(d.ActiveDelegates))
}

// ValidateBlock validates a block according to DPoS rules
func (d *DPoS) ValidateBlock(block *blockchain.Block) bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	// Check that the block is produced by an active delegate
	isActiveDelegate := slices.Contains(d.ActiveDelegates, block.Validator)

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
	case "DelegateUpdate":
		return d.handleDelegateUpdate(message)
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
		d.Delegates[previousDelegate].VoterCount--
	}

	// Record new vote
	d.VotesByNodeID[voterID] = delegateID
	delegate.Votes++
	delegate.VoterCount++

	utils.LogInfo("Vote recorded from %s for delegate %s", voterID, delegateID)
	return nil
}

// handleNewDelegate processes a new delegate registration
func (d *DPoS) handleNewDelegate(message *Message) error {
	delegateData, ok := message.Data.(map[string]any)
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

	// Check minimum stake requirement
	if stakeFloat < 100.0 {
		return fmt.Errorf("insufficient stake: minimum 100 required, got %.2f", stakeFloat)
	}

	// Register new delegate
	d.Delegates[nodeID] = &DelegateInfo{
		NodeID:        nodeID,
		Stake:         stakeFloat,
		Votes:         0,
		BlocksCreated: 0,
		IsActive:      false,
		LastActive:    time.Now().UTC().Format(time.RFC3339),
		Reliability:   100.0, // Start with perfect reliability
		RewardAccrued: 0.0,
		VoterCount:    0,
		MissedBlocks:  0,
		CreationTime:  time.Now().UTC().Format(time.RFC3339),
	}

	d.StakeByNodeID[nodeID] = stakeFloat

	utils.LogInfo("New delegate registered: %s with stake %.2f", nodeID, stakeFloat)
	return nil
}

// handleDelegateUpdate processes updates to delegate information
func (d *DPoS) handleDelegateUpdate(message *Message) error {
	updateData, ok := message.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid update data format")
	}

	nodeID, ok := updateData["nodeId"].(string)
	if !ok {
		return fmt.Errorf("invalid node ID format")
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Check if delegate exists
	delegate, exists := d.Delegates[nodeID]
	if !exists {
		return fmt.Errorf("delegate %s does not exist", nodeID)
	}

	// Update stake if provided
	if stakeFloat, ok := updateData["stake"].(float64); ok {
		if stakeFloat < 100.0 {
			return fmt.Errorf("insufficient stake: minimum 100 required, got %.2f", stakeFloat)
		}
		delegate.Stake = stakeFloat
		d.StakeByNodeID[nodeID] = stakeFloat
	}

	// Update representative ID if provided
	if repID, ok := updateData["representativId"].(string); ok {
		delegate.RepresentativID = repID
	}

	utils.LogInfo("Delegate updated: %s", nodeID)
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
		d.Delegates[previousDelegate].VoterCount--
	}

	// Record new vote
	d.VotesByNodeID[voterID] = delegateID
	delegate.Votes++
	delegate.VoterCount++

	return nil
}

// RegisterDelegate registers a new delegate
func (d *DPoS) RegisterDelegate(nodeID string, stake float64) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Check minimum stake requirement
	if stake < 100.0 {
		return fmt.Errorf("insufficient stake: minimum 100 required, got %.2f", stake)
	}

	// Register new delegate
	d.Delegates[nodeID] = &DelegateInfo{
		NodeID:        nodeID,
		Stake:         stake,
		Votes:         0,
		BlocksCreated: 0,
		IsActive:      false,
		LastActive:    time.Now().UTC().Format(time.RFC3339),
		Reliability:   100.0, // Start with perfect reliability
		RewardAccrued: 0.0,
		VoterCount:    0,
		MissedBlocks:  0,
		CreationTime:  time.Now().UTC().Format(time.RFC3339),
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

	return slices.Contains(d.ActiveDelegates, d.NodeID)
}

// GetValidators returns the current set of active delegates
func (d *DPoS) GetValidators() []string {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.ActiveDelegates
}

// GetType returns the type of consensus algorithm
func (d *DPoS) GetType() ConsensusType {
	return "DPOS"
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
	if !exists {
		return nil, false
	}

	// Return a copy to avoid concurrency issues
	delegateCopy := &DelegateInfo{
		NodeID:          delegate.NodeID,
		Stake:           delegate.Stake,
		Votes:           delegate.Votes,
		BlocksCreated:   delegate.BlocksCreated,
		IsActive:        delegate.IsActive,
		LastActive:      delegate.LastActive,
		Reliability:     delegate.Reliability,
		RewardAccrued:   delegate.RewardAccrued,
		VoterCount:      delegate.VoterCount,
		MissedBlocks:    delegate.MissedBlocks,
		CreationTime:    delegate.CreationTime,
		RepresentativID: delegate.RepresentativID,
	}

	return delegateCopy, true
}

// GetAllDelegates returns information about all delegates
func (d *DPoS) GetAllDelegates() map[string]*DelegateInfo {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	// Create a copy to avoid concurrency issues
	delegates := make(map[string]*DelegateInfo)
	for id, delegate := range d.Delegates {
		delegates[id] = &DelegateInfo{
			NodeID:          delegate.NodeID,
			Stake:           delegate.Stake,
			Votes:           delegate.Votes,
			BlocksCreated:   delegate.BlocksCreated,
			IsActive:        delegate.IsActive,
			LastActive:      delegate.LastActive,
			Reliability:     delegate.Reliability,
			RewardAccrued:   delegate.RewardAccrued,
			VoterCount:      delegate.VoterCount,
			MissedBlocks:    delegate.MissedBlocks,
			CreationTime:    delegate.CreationTime,
			RepresentativID: delegate.RepresentativID,
		}
	}

	return delegates
}

// IncreaseStake increases the stake of a delegate
func (d *DPoS) IncreaseStake(nodeID string, additionalStake float64) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	delegate, exists := d.Delegates[nodeID]
	if !exists {
		return fmt.Errorf("delegate %s does not exist", nodeID)
	}

	if additionalStake <= 0 {
		return fmt.Errorf("additional stake must be positive, got %.2f", additionalStake)
	}

	delegate.Stake += additionalStake
	d.StakeByNodeID[nodeID] = delegate.Stake

	utils.LogInfo("Delegate %s increased stake by %.2f to %.2f",
		nodeID, additionalStake, delegate.Stake)
	return nil
}

// UnregisterDelegate removes a delegate
func (d *DPoS) UnregisterDelegate(nodeID string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	_, exists := d.Delegates[nodeID]
	if !exists {
		return fmt.Errorf("delegate %s does not exist", nodeID)
	}

	// Remove delegate
	delete(d.Delegates, nodeID)
	delete(d.StakeByNodeID, nodeID)

	// Remove from active delegates if present
	for i, id := range d.ActiveDelegates {
		if id == nodeID {
			d.ActiveDelegates = append(d.ActiveDelegates[:i], d.ActiveDelegates[i+1:]...)
			break
		}
	}

	// Remove votes for this delegate
	for voterID, votedFor := range d.VotesByNodeID {
		if votedFor == nodeID {
			delete(d.VotesByNodeID, voterID)
		}
	}

	utils.LogInfo("Delegate %s unregistered", nodeID)
	return nil
}

// GetDelegateStats returns statistics about the delegate system
func (d *DPoS) GetDelegateStats() map[string]interface{} {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	totalStake := 0.0
	totalVotes := 0
	activeStake := 0.0

	for _, delegate := range d.Delegates {
		totalStake += delegate.Stake
		totalVotes += delegate.Votes

		if delegate.IsActive {
			activeStake += delegate.Stake
		}
	}

	return map[string]interface{}{
		"totalDelegates":  len(d.Delegates),
		"activeDelegates": len(d.ActiveDelegates),
		"totalStake":      totalStake,
		"activeStake":     activeStake,
		"totalVotes":      totalVotes,
		"currentRound":    d.CurrentRound,
		"currentSlot":     d.CurrentSlot,
		"currentProducer": d.CurrentProducer,
		"rewardPool":      d.RewardPool,
		"lastBlockTime":   d.LastBlockTime.Format(time.RFC3339),
	}
}

func (d *DPoS) UpdateValidators(validators []ValidatorInfo) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.validators = make(map[string]*big.Int)
	for _, val := range validators {
		stake, _ := new(big.Int).SetString(val.Stake, 10)
		d.validators[val.Address] = stake
	}

	d.UpdateActiveDelegates()
	utils.LogInfo("Validators updated: %d active validators", len(d.validators))
}
