package validation

import (
	"errors"
	"fmt"
	"tripcodechain_go/blockchain"
	"tripcodechain_go/currency"
	"tripcodechain_go/utils"
)

// VerifyValidatorEligibility checks if a node at validatorAddress is eligible.
// It checks if the validator is registered, has minimum stake, and is active.
// For unregistered nodes, it checks if their general balance meets the minimum stake requirement.
func VerifyValidatorEligibility(dpos *DPoS, validatorAddress string) (bool, error) {
	if dpos == nil {
		return false, errors.New("DPoS instance is nil")
	}

	// 1. Check if validator is already registered and known by DPoS
	// GetValidatorInfo now returns validation.Validator
	valInfo, exists := dpos.GetValidatorInfo(validatorAddress)
	if exists {
		// valInfo is validation.Validator
		// valInfo.Stake is uint64
		// blockchain.MinValidatorStake is uint64
		if valInfo.Stake < blockchain.MinValidatorStake {
			utils.LogInfo("Registered validator %s has insufficient stake: %d < %d",
				validatorAddress, valInfo.Stake, blockchain.MinValidatorStake)
			return false, nil // Not an error, just ineligible
		}
		// valInfo.IsActive is bool
		if !valInfo.IsActive { // Assuming IsActive reflects not being banned, etc.
			utils.LogInfo("Registered validator %s is not active.", validatorAddress)
			return false, nil
		}
		utils.LogInfo("Validator %s is registered, has sufficient stake (%d), and is active. Eligible.",
			validatorAddress, valInfo.Stake)
		return true, nil
	}

	// 2. If not registered, check general balance against MinValidatorStake
	// This scenario is for nodes that *want* to become validators or are advertising as such
	// but are not yet formally in the dpos.Validators list.
	balance, err := currency.GetBalance(validatorAddress) // currency.GetBalance is from package currency
	if err != nil {
		utils.LogError("Error getting balance for potential validator %s: %v", validatorAddress, err)
		return false, fmt.Errorf("could not get balance for %s: %w", validatorAddress, err)
	}

	if balance < blockchain.MinValidatorStake {
		utils.LogInfo("Potential validator %s has insufficient balance to meet min stake: %d < %d",
			validatorAddress, balance, blockchain.MinValidatorStake)
		return false, nil // Not an error, just ineligible
	}

	// If they have enough balance, they are considered "eligible" to attempt registration.
	// The actual registration process (consensus.RegisterValidator) will formally deduct the stake.
	utils.LogInfo("Potential validator %s has sufficient balance (%d) to meet min stake. Eligible to register.",
		validatorAddress, balance)
	return true, nil
}
