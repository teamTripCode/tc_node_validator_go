package validation

import (
	"fmt"
	"math/big"
	"sync"
	"time"

	"errors"
	"slices"
	"sort"
	"tripcodechain_go/blockchain"
	"tripcodechain_go/currency"
	"tripcodechain_go/utils"
	"tripcodechain_go/pkg/consensus_types" // Changed for consensus_types.Message and consensus_types.ConsensusType
)

// --- Copied from existing pkg/validation/dpos_types.go ---

// Validator defines the structure for a validator in the DPoS system.
type Validator struct {
	Address          string
	Stake            uint64
	IsActive         bool
	ReliabilityScore float64
	TotalVotes       uint64
}

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
	validators       map[string]*big.Int  // Validadores y su stake - This seems like an older field, distinct from Validators map. Retaining for now.
	bannedValidators map[string]time.Time // Map of banned validators and their ban expiration time
	Validators       map[string]Validator // Map of registered validators using the new Validator struct
}

// ValidatorInfo represents information about a validator for external queries or simpler representations.
type ValidatorInfo struct {
	Address string `json:"address"`
	Stake   string `json:"stake"` // Stake is string here, might need to align with Validator.Stake (uint64) if used interchangeably
}

// --- Moved from consensus/dpos.go ---

// NewDPoS creates a new DPoS consensus instance
func NewDPoS(currencyManager *currency.CurrencyManager) *DPoS { // Returns *validation.DPoS (implicitly, as it's in this package)
	return &DPoS{
		Delegates:        make(map[string]*DelegateInfo),
		Validators:       make(map[string]Validator),
		ActiveDelegates:  make([]string, 0),
		RoundLength:      blockchain.GetNumberOfDelegates(),
		BlockTime:        3,
		StakeByNodeID:    make(map[string]float64),
		VotesByNodeID:    make(map[string]string),
		RewardPool:       1000.0,
		currencyManager:  currencyManager,
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

func (d *DPoS) SlashDelegate(address string, severity string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	delegate, exists := d.Delegates[address]
	if !exists {
		return fmt.Errorf("delegate %s not found for slashing", address)
	}

	penaltyPercentage := 0.1
	switch severity {
	case "minor":
		penaltyPercentage = 0.1
		delegate.Reliability *= 0.95
	case "severe":
		penaltyPercentage = 0.3
		delegate.Reliability *= 0.80
	case "critical":
		penaltyPercentage = 0.5
		delegate.Reliability = 0
	}

	stakeBalance := d.currencyManager.GetStake(address)
	penaltyAmount := new(big.Float).Mul(
		new(big.Float).SetInt(stakeBalance.Int),
		big.NewFloat(penaltyPercentage),
	)
	penaltyInt, _ := penaltyAmount.Int(nil)
	penalty := currency.NewBalanceFromBigInt(penaltyInt)

	utils.LogInfo("Placeholder: BurnTokens would be called here for delegate %s, amount %s", address, penalty.TripCoinString())

	penaltyFloat, _ := penaltyAmount.Float64()
	delegate.Stake -= penaltyFloat
	delegate.MissedBlocks += 1

	utils.LogSecurityEvent("delegate_slashed", map[string]interface{}{
		"delegate":        address,
		"severity":        severity,
		"penalty_amount":  penalty.TripCoinString(),
		"new_reliability": delegate.Reliability,
		"remaining_stake": delegate.Stake,
	})

	if delegate.Reliability < 50 {
		banDuration := time.Hour * 24
		utils.LogSecurityEvent("delegate_banned", map[string]interface{}{
			"delegate": address,
			"duration": banDuration.String(),
			"reason":   "low reliability after slashing",
		})
		d.bannedValidators[address] = time.Now().Add(banDuration)
		delete(d.validators, address)
		for i, id := range d.ActiveDelegates {
			if id == address {
				d.ActiveDelegates = append(d.ActiveDelegates[:i], d.ActiveDelegates[i+1:]...)
				break
			}
		}
		time.AfterFunc(banDuration, func() {
			d.mutex.Lock()
			defer d.mutex.Unlock()
			delete(d.bannedValidators, address)
			utils.LogSecurityEvent("delegate_reinstated", map[string]interface{}{"delegate": address})
		})
	}
	d.UpdateActiveDelegates()
	return nil
}

func (d *DPoS) GetValidatorInfo(address string) (Validator, bool) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	validator, exists := d.Validators[address]
	return validator, exists
}

func DistributeBlockReward(dpos *DPoS, blockCreatorAddress string) error {
	errUpdate := currency.UpdateBalance(blockCreatorAddress, 0)
	if errUpdate != nil {
		utils.LogError("Placeholder: Error during conceptual block reward distribution for %s: %v", blockCreatorAddress, errUpdate)
	}
	utils.LogInfo("Validator %s conceptually rewarded %d TripCoins for creating a block.", blockCreatorAddress, blockchain.BlockRewardAmount)
	return nil
}

func UpdateValidatorReliability(dpos *DPoS, validatorAddress string, wasSuccessfulAttempt bool) error {
	dpos.mutex.Lock()
	defer dpos.mutex.Unlock()
	validator, exists := dpos.Validators[validatorAddress]
	if !exists {
		return errors.New("validator not found for reliability update")
	}
	if wasSuccessfulAttempt {
		validator.ReliabilityScore += blockchain.ReliabilityChangeRate
		if validator.ReliabilityScore > 100.0 {
			validator.ReliabilityScore = 100.0
		}
		utils.LogInfo("Increased reliability score for validator %s to %.2f", validatorAddress, validator.ReliabilityScore)
	} else {
		validator.ReliabilityScore -= blockchain.ReliabilityChangeRate
		if validator.ReliabilityScore < 0.0 {
			validator.ReliabilityScore = 0.0
		}
		utils.LogInfo("Decreased reliability score for validator %s to %.2f", validatorAddress, validator.ReliabilityScore)
	}
	if validator.ReliabilityScore < blockchain.ReliabilityThreshold {
		if validator.IsActive {
			validator.IsActive = false
			utils.LogInfo("Validator %s marked INACTIVE due to low reliability score: %.2f", validatorAddress, validator.ReliabilityScore)
		}
	}
	dpos.Validators[validatorAddress] = validator
	return nil
}

func (d *DPoS) Initialize(nodeID string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	if d.initialized {
		return nil
	}
	d.NodeID = nodeID
	d.LastBlockTime = time.Now()
	d.UpdateActiveDelegates()
	d.initialized = true
	utils.LogInfo("DPoS consensus initialized. Node ID: %s", d.NodeID)
	utils.LogInfo("Active delegates: %v", d.ActiveDelegates)
	go d.runBlockProductionSchedule()
	return nil
}

func (d *DPoS) runBlockProductionSchedule() {
	ticker := time.NewTicker(time.Duration(d.BlockTime) * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		d.updateSchedule()
	}
}

func (d *DPoS) updateSchedule() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	elapsed := time.Since(d.LastBlockTime).Seconds()
	slotsElapsed := int(elapsed) / d.BlockTime

	if slotsElapsed > 0 {
		d.CurrentSlot = (d.CurrentSlot + slotsElapsed) % d.RoundLength
		d.LastBlockTime = time.Now()
		if d.CurrentSlot == 0 {
			d.CurrentRound++
			d.UpdateActiveDelegates()
			d.distributeRewards()
		}
		if len(d.ActiveDelegates) == 0 || len(d.ActiveDelegates) < d.RoundLength {
			utils.LogInfo("Waiting for sufficient delegates to join. Currently have %d, need %d.", len(d.ActiveDelegates), d.RoundLength)
			return
		}
		d.CurrentProducer = d.ActiveDelegates[d.CurrentSlot]
		utils.LogInfo("DPoS schedule updated. Round: %d, Slot: %d, Producer: %s", d.CurrentRound, d.CurrentSlot, d.CurrentProducer)
		if d.CurrentProducer == d.NodeID {
			utils.LogInfo("This node is the current block producer!")
			if delegate, exists := d.Delegates[d.NodeID]; exists {
				delegate.BlocksCreated++
				delegate.LastActive = time.Now().UTC().Format(time.RFC3339)
			}
		}
	}
}

func (d *DPoS) distributeRewards() {
	roundReward := 10.0 * float64(d.RoundLength)
	for _, delegateID := range d.ActiveDelegates {
		if delegate, exists := d.Delegates[delegateID]; exists {
			reward := roundReward * (delegate.Reliability / 100.0) / float64(d.RoundLength)
			delegate.RewardAccrued += reward
			utils.LogInfo("Delegate %s received reward: %.2f tokens", delegateID, reward)
		}
	}
}

func (d *DPoS) UpdateActiveDelegates() {
	type delegateScore struct {
		id    string
		score float64
	}
	var scores []delegateScore
	for id, info := range d.Delegates {
		score := float64(info.Votes)*10.0 + info.Stake + (info.Reliability * 0.5)
		scores = append(scores, delegateScore{id, score})
	}
	for i := range scores {
		for j := range len(scores) - i - 1 {
			if scores[j].score < scores[j+1].score {
				scores[j], scores[j+1] = scores[j+1], scores[j]
			}
		}
	}
	for _, delegate := range d.Delegates {
		delegate.IsActive = false
	}
	numActive := min(len(scores), d.RoundLength)
	d.ActiveDelegates = make([]string, numActive)
	for i := range numActive {
		d.ActiveDelegates[i] = scores[i].id
		d.Delegates[scores[i].id].IsActive = true
		d.Delegates[scores[i].id].LastActive = time.Now().UTC().Format(time.RFC3339)
	}
	utils.LogInfo("Updated active delegates. Total: %d", len(d.ActiveDelegates))
}

func (d *DPoS) ValidateBlock(block *blockchain.Block) bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	isActiveDelegate := slices.Contains(d.ActiveDelegates, block.Validator)
	if !isActiveDelegate {
		utils.LogError("Block validator %s is not an active delegate", block.Validator)
		return false
	}
	if block.Hash != block.CalculateHash() {
		utils.LogError("Block hash is invalid for block %d by %s", block.Index, block.Validator)
		return false
	}
	for i, tx := range block.Transactions {
		if err := tx.Validate(); err != nil {
			utils.LogError("Transaction %d in block %d (validator: %s) failed validation: %v", i, block.Index, block.Validator, err)
			return false
		}
	}
	if len(block.Transactions) > 0 {
		utils.LogDebug("All %d transactions in block %d validated successfully.", len(block.Transactions), block.Index)
	}
	return true
}

func (d *DPoS) ProcessConsensusMessage(message *consensus_types.Message) error { // Use consensus_types.Message
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

func (d *DPoS) handleVote(message *consensus_types.Message) error { // Use consensus_types.Message
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
	delegate, exists := d.Delegates[delegateID]
	if !exists {
		return fmt.Errorf("delegate %s does not exist", delegateID)
	}
	previousDelegate, voted := d.VotesByNodeID[voterID]
	if voted {
		d.Delegates[previousDelegate].Votes--
		d.Delegates[previousDelegate].VoterCount--
	}
	d.VotesByNodeID[voterID] = delegateID
	delegate.Votes++
	delegate.VoterCount++
	utils.LogInfo("Vote recorded from %s for delegate %s", voterID, delegateID)
	return nil
}

func (d *DPoS) handleNewDelegate(message *consensus_types.Message) error { // Use consensus_types.Message
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
	if stakeFloat < 100.0 {
		return fmt.Errorf("insufficient stake: minimum 100 required, got %.2f", stakeFloat)
	}
	d.Delegates[nodeID] = &DelegateInfo{
		NodeID:        nodeID,
		Stake:         stakeFloat,
		Votes:         0,
		BlocksCreated: 0,
		IsActive:      false,
		LastActive:    time.Now().UTC().Format(time.RFC3339),
		Reliability:   100.0,
		RewardAccrued: 0.0,
		VoterCount:    0,
		MissedBlocks:  0,
		CreationTime:  time.Now().UTC().Format(time.RFC3339),
	}
	d.StakeByNodeID[nodeID] = stakeFloat
	utils.LogInfo("New delegate registered: %s with stake %.2f", nodeID, stakeFloat)
	return nil
}

func (d *DPoS) handleDelegateUpdate(message *consensus_types.Message) error { // Use consensus_types.Message
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
	delegate, exists := d.Delegates[nodeID]
	if !exists {
		return fmt.Errorf("delegate %s does not exist", nodeID)
	}
	if stakeFloat, ok := updateData["stake"].(float64); ok {
		if stakeFloat < 100.0 {
			return fmt.Errorf("insufficient stake: minimum 100 required, got %.2f", stakeFloat)
		}
		delegate.Stake = stakeFloat
		d.StakeByNodeID[nodeID] = stakeFloat
	}
	if repID, ok := updateData["representativId"].(string); ok {
		delegate.RepresentativID = repID
	}
	utils.LogInfo("Delegate updated: %s", nodeID)
	return nil
}

func (d *DPoS) RegisterVote(voterID, delegateID string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	delegate, exists := d.Delegates[delegateID]
	if !exists {
		return fmt.Errorf("delegate %s does not exist", delegateID)
	}
	previousDelegate, voted := d.VotesByNodeID[voterID]
	if voted {
		d.Delegates[previousDelegate].Votes--
		d.Delegates[previousDelegate].VoterCount--
	}
	d.VotesByNodeID[voterID] = delegateID
	delegate.Votes++
	delegate.VoterCount++
	return nil
}

func (d *DPoS) RegisterDelegate(nodeID string, stake float64) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	if stake < 100.0 {
		return fmt.Errorf("insufficient stake: minimum 100 required, got %.2f", stake)
	}
	d.Delegates[nodeID] = &DelegateInfo{
		NodeID:        nodeID,
		Stake:         stake,
		Votes:         0,
		BlocksCreated: 0,
		IsActive:      false,
		LastActive:    time.Now().UTC().Format(time.RFC3339),
		Reliability:   100.0,
		RewardAccrued: 0.0,
		VoterCount:    0,
		MissedBlocks:  0,
		CreationTime:  time.Now().UTC().Format(time.RFC3339),
	}
	d.StakeByNodeID[nodeID] = stake
	return nil
}

func (d *DPoS) BroadcastPrepare(block *blockchain.Block) error { return nil }
func (d *DPoS) BroadcastCommit(block *blockchain.Block) error  { return nil }

func (d *DPoS) IsValidator() bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return slices.Contains(d.ActiveDelegates, d.NodeID)
}

func (d *DPoS) GetValidators() []string {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.ActiveDelegates
}

func (d *DPoS) GetType() consensus_types.ConsensusType { // Use consensus_types.ConsensusType
	return consensus_types.ConsensusType("DPOS")
}

func (d *DPoS) GetCurrentProducer() string {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.CurrentProducer
}

func (d *DPoS) GetDelegateInfo(nodeID string) (*DelegateInfo, bool) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	delegate, exists := d.Delegates[nodeID]
	if !exists {
		return nil, false
	}
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

func (d *DPoS) GetAllDelegates() map[string]*DelegateInfo {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
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
	utils.LogInfo("Delegate %s increased stake by %.2f to %.2f", nodeID, additionalStake, delegate.Stake)
	return nil
}

func (d *DPoS) UnregisterDelegate(nodeID string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	_, exists := d.Delegates[nodeID]
	if !exists {
		return fmt.Errorf("delegate %s does not exist", nodeID)
	}
	delete(d.Delegates, nodeID)
	delete(d.StakeByNodeID, nodeID)
	for i, id := range d.ActiveDelegates {
		if id == nodeID {
			d.ActiveDelegates = append(d.ActiveDelegates[:i], d.ActiveDelegates[i+1:]...)
			break
		}
	}
	for voterID, votedFor := range d.VotesByNodeID {
		if votedFor == nodeID {
			delete(d.VotesByNodeID, voterID)
		}
	}
	utils.LogInfo("Delegate %s unregistered", nodeID)
	return nil
}

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

func (d *DPoS) UpdateValidators(validators []ValidatorInfo) { // ValidatorInfo from this package
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

func RegisterValidator(dpos *DPoS, address string, stakeAmount uint64) error {
	if stakeAmount < blockchain.MinValidatorStake {
		return errors.New("insufficient stake to register as validator")
	}
	dpos.mutex.RLock()
	_, exists := dpos.Validators[address]
	dpos.mutex.RUnlock()
	if exists {
		return errors.New("validator already registered")
	}
	balance, err := currency.GetBalance(address)
	if err != nil {
		return fmt.Errorf("failed to get balance for validator %s: %w", address, err)
	}
	if balance < stakeAmount {
		return fmt.Errorf("insufficient balance for validator %s: has %d, needs %d", address, balance, stakeAmount)
	}
	err = currency.UpdateBalance(address, stakeAmount)
	if err != nil {
		return fmt.Errorf("failed to update balance for validator %s: %w", address, err)
	}
	validator := Validator{
		Address:          address,
		Stake:            stakeAmount,
		IsActive:         true,
		ReliabilityScore: 100.0,
		TotalVotes:       0,
	}
	dpos.mutex.Lock()
	dpos.Validators[address] = validator
	dpos.mutex.Unlock()
	utils.LogInfo("Validator registered: %s with stake %d", address, stakeAmount)
	return nil
}

func VoteForValidator(dpos *DPoS, voterAddress string, validatorAddress string, voteStake uint64) error {
	dpos.mutex.RLock()
	validator, exists := dpos.Validators[validatorAddress]
	dpos.mutex.RUnlock()
	if !exists {
		return errors.New("validator not found")
	}
	balance, err := currency.GetBalance(voterAddress)
	if err != nil {
		return fmt.Errorf("failed to get balance for voter %s: %w", voterAddress, err)
	}
	if balance < voteStake {
		return errors.New("insufficient funds to vote")
	}
	validator.TotalVotes += voteStake
	dpos.mutex.Lock()
	dpos.Validators[validatorAddress] = validator
	dpos.mutex.Unlock()
	utils.LogInfo("Vote cast from %s for validator %s with vote stake %d", voterAddress, validatorAddress, voteStake)
	return nil
}

func SelectDelegates(dpos *DPoS) error {
	dpos.mutex.Lock()
	defer dpos.mutex.Unlock()
	if len(dpos.Validators) == 0 {
		utils.LogInfo("No validators available to select delegates.")
		return nil
	}
	validators := make([]Validator, 0, len(dpos.Validators))
	for _, val := range dpos.Validators {
		validators = append(validators, val)
	}
	sort.SliceStable(validators, func(i, j int) bool {
		if validators[i].TotalVotes != validators[j].TotalVotes {
			return validators[i].TotalVotes > validators[j].TotalVotes
		}
		return validators[i].Stake > validators[j].Stake
	})
	for address, val := range dpos.Validators {
		val.IsActive = false
		dpos.Validators[address] = val
	}
	numDelegates := blockchain.GetNumberOfDelegates()
	if len(validators) < numDelegates {
		numDelegates = len(validators)
	}
	activeDelegateAddresses := make([]string, 0, numDelegates)
	for i := 0; i < numDelegates; i++ {
		val := validators[i]
		val.IsActive = true
		dpos.Validators[val.Address] = val
		activeDelegateAddresses = append(activeDelegateAddresses, val.Address)
		utils.LogInfo("Validator %s selected as active delegate. Votes: %d, Stake: %d", val.Address, val.TotalVotes, val.Stake)
	}
	dpos.ActiveDelegates = activeDelegateAddresses
	utils.LogInfo("Selected %d active delegates: %v", len(activeDelegateAddresses), activeDelegateAddresses)
	utils.LogInfo("Successfully selected %d delegates.", numDelegates)
	return nil
}

func SlashValidator(dpos *DPoS, validatorAddress string, penaltyPercentage float64, reason string) error {
	dpos.mutex.Lock()
	defer dpos.mutex.Unlock()
	validator, exists := dpos.Validators[validatorAddress]
	if !exists {
		return errors.New("validator not found for slashing")
	}
	if penaltyPercentage < 0.0 || penaltyPercentage > 1.0 {
		return errors.New("invalid penalty percentage")
	}
	slashedAmount := uint64(float64(validator.Stake) * penaltyPercentage)
	if slashedAmount > validator.Stake {
		slashedAmount = validator.Stake
	}
	validator.Stake -= slashedAmount
	dpos.Validators[validatorAddress] = validator
	utils.LogInfo("Validator %s slashed by %d%% (%d tokens) for reason: %s. New stake: %d",
		validatorAddress, int(penaltyPercentage*100), slashedAmount, reason, validator.Stake)
	utils.LogInfo("Placeholder: Slashed tokens (%d) for validator %s would be burned here.", slashedAmount, validatorAddress)
	return nil
}

// min is a helper function (already present in p2p/pbft_integration.go, but good to have here if used locally and that file is not imported)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
