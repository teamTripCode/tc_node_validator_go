package blockchain

// BlockType represents the type of a block in the blockchain
type BlockType string

const (
	// TransactionBlock indicates a block that contains transactions
	TransactionBlock BlockType = "TRANSACTION"
	// CriticalProcessBlock indicates a block that contains critical processes
	CriticalProcessBlock BlockType = "CRITICAL_PROCESS"
)

const (
	// MinValidatorStake defines the minimum stake required to become a validator.
	MinValidatorStake = 100
	// NumberOfDelegates defines the number of active delegates in the DPoS system.
	NumberOfDelegates = 21
	// ReliabilityThreshold is the minimum reliability score before a validator is marked inactive.
	ReliabilityThreshold = 50.0
	// ReliabilityChangeRate is the amount by which reliability score changes.
	ReliabilityChangeRate = 10.0
	// ValidatorFeeShare is the percentage of transaction fees given to the validator.
	ValidatorFeeShare = 0.3
	// DevFundFeeShare is the percentage of transaction fees given to the development fund.
	DevFundFeeShare = 0.4
	// BurnFeeShare is the percentage of transaction fees to be burned.
	BurnFeeShare = 0.3
	// DevFundAddress is the address for the development fund.
	DevFundAddress = "DEVELOPMENT_FUND_ADDRESS_PLACEHOLDER"
	// BlockRewardAmount is the amount of TripCoins rewarded for creating a block.
	BlockRewardAmount = 2
)
