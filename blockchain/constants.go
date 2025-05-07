package blockchain

// BlockType represents the type of a block in the blockchain
type BlockType string

const (
	// TransactionBlock indicates a block that contains transactions
	TransactionBlock BlockType = "TRANSACTION"
	// CriticalProcessBlock indicates a block that contains critical processes
	CriticalProcessBlock BlockType = "CRITICAL_PROCESS"
)
