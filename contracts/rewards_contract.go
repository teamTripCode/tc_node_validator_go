package contracts

import (
	"fmt"
	"tripcodechain_go/blockchain"
	"tripcodechain_go/pkg/validation" // Changed import
	"tripcodechain_go/utils"
	// "errors" // Not strictly needed if only wrapping errors with fmt.Errorf
)

// RewardsContract defines the structure for the rewards contract.
// It can be empty for now, as DPoS instance and other parameters will be passed to methods.
type RewardsContract struct{}

// NewRewardsContract creates a new instance of RewardsContract.
func NewRewardsContract() *RewardsContract {
	return &RewardsContract{}
}

// DistributeRewards orchestrates the distribution of both transaction fees and block rewards.
func DistributeRewards(contract *RewardsContract, dpos *validation.DPoS, blockCreatorAddress string, totalTransactionFees uint64) error { // Changed dpos type
	// Distribute transaction fees
	err := blockchain.DistributeTransactionFees(blockCreatorAddress, totalTransactionFees)
	if err != nil {
		utils.LogError("RewardsContract.DistributeRewards: Error calling blockchain.DistributeTransactionFees for %s: %v", blockCreatorAddress, err)
		return fmt.Errorf("error distributing transaction fees via contract for %s: %w", blockCreatorAddress, err)
	}

	// Distribute block reward
	// Assuming DistributeBlockReward is a function in the consensus package that takes *DPoS as its first argument.
	// Based on previous work: func DistributeBlockReward(dpos *DPoS, blockCreatorAddress string) error
	// This was a standalone function, not a method of dpos. So it should be validation.DistributeBlockReward.
	err = validation.DistributeBlockReward(dpos, blockCreatorAddress) // Changed to validation.DistributeBlockReward
	if err != nil {
		utils.LogError("RewardsContract.DistributeRewards: Error calling validation.DistributeBlockReward for %s: %v", blockCreatorAddress, err)
		return fmt.Errorf("error distributing block reward via contract for %s: %w", blockCreatorAddress, err)
	}

	utils.LogInfo("RewardsContract: Successfully processed fee distribution and block reward for %s.", blockCreatorAddress)
	return nil
}
