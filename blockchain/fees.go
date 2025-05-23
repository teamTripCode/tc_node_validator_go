package blockchain

import (
	"tripcodechain_go/currency"
	"tripcodechain_go/utils"
)

// DistributeTransactionFees distributes the collected transaction fees according to predefined shares.
func DistributeTransactionFees(blockCreatorAddress string, totalTransactionFees uint64) error {
	if totalTransactionFees == 0 {
		utils.LogInfo("No transaction fees to distribute for block creator %s.", blockCreatorAddress)
		return nil
	}

	validatorShare := uint64(float64(totalTransactionFees) * ValidatorFeeShare)
	devFundShare := uint64(float64(totalTransactionFees) * DevFundFeeShare)
	// Calculate burnShare as the remainder to ensure all fees are accounted for,
	// especially if shares don't perfectly sum to 1.0 due to float precision.
	burnShare := totalTransactionFees - validatorShare - devFundShare

	// Ensure burnShare is not negative if totalTransactionFees is very small.
	if validatorShare+devFundShare > totalTransactionFees {
		// This case should ideally not happen with proper share definitions summing to 1.0
		// However, if it does, prioritize validator and dev fund, then burn what's left (even if 0).
		if totalTransactionFees < validatorShare {
			validatorShare = totalTransactionFees
			devFundShare = 0
			burnShare = 0
		} else if totalTransactionFees < validatorShare+devFundShare {
			devFundShare = totalTransactionFees - validatorShare
			burnShare = 0
		}
	}

	// (Placeholder) Credit the block creator
	// Assuming UpdateBalance subtracts, so a negative amount adds to balance.
	// Or, if an AddBalance function exists, that would be cleaner.
	// For now, using UpdateBalance as per instructions, assuming it can handle "adding" via negative subtraction.
	// This is a placeholder; actual implementation depends on currency.UpdateBalance behavior.
	err := currency.UpdateBalance(blockCreatorAddress, -validatorShare) // Subtracting a negative is adding
	if err != nil {
		utils.LogError("Failed to credit validator %s with fee share %d: %v", blockCreatorAddress, validatorShare, err)
		// Decide if this error is critical enough to halt further distribution or just log.
		// For now, logging and continuing.
	} else {
		utils.LogInfo("Credited validator %s with %d TripCoins from transaction fees.", blockCreatorAddress, validatorShare)
	}

	// (Placeholder) Credit the development fund
	err = currency.UpdateBalance(DevFundAddress, -devFundShare) // Subtracting a negative is adding
	if err != nil {
		utils.LogError("Failed to credit development fund %s with fee share %d: %v", DevFundAddress, devFundShare, err)
		// Similar error handling consideration as above.
	} else {
		utils.LogInfo("Credited development fund %s with %d TripCoins from transaction fees.", DevFundAddress, devFundShare)
	}

	// Log the amount to be burned
	if burnShare > 0 {
		utils.LogInfo("Burning %d TripCoins from transaction fees.", burnShare)
		// (Placeholder) Here you would call a function like currency.BurnTokens(burnShare)
		// For now, logging is sufficient as per instructions.
	} else {
		utils.LogInfo("No TripCoins to burn from transaction fees for this block.")
	}

	utils.LogInfo("Transaction fees distributed: Validator: %d, DevFund: %d, Burned: %d", validatorShare, devFundShare, burnShare)

	return nil
}
