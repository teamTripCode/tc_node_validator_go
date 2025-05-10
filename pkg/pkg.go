package pkg

import "sync"

// BlockType represents the type of a block in the blockchain
type BlockType string

/**
 * Block represents a single block in the blockchain.
 * It contains metadata such as index, timestamp, and hash information,
 * along with data like transactions or critical processes.
 */
type Block struct {
	Index             uint64             `json:"index"`                       // Position of the block in the chain
	Timestamp         string             `json:"timestamp"`                   // Time when the block was created (UTC)
	Type              BlockType          `json:"type"`                        // Type of block (e.g., TransactionBlock or CriticalProcessBlock)
	Transactions      []*Transaction     `json:"transactions,omitempty"`      // List of transactions (optional)
	CriticalProcesses []*CriticalProcess `json:"criticalProcesses,omitempty"` // List of critical processes (optional)
	PreviousHash      string             `json:"previousHash"`                // Hash of the previous block in the chain
	Hash              string             `json:"hash"`                        // Current block's hash
	Nonce             uint64             `json:"nonce"`                       // Number used during mining to find a valid hash
	Signature         string             `json:"signature"`                   // Signature of the validator who forged this block
	Validator         string             `json:"validator"`                   // Public key or ID of the validator who forged the block
	TotalFees         float64            `json:"totalFees,omitempty"`         // Total fees collected by the validator
	Data              string
}

type Blockchain struct {
	blocks     []*Block     // Ordered list of blocks in the chain
	blockType  BlockType    // Type of blocks this blockchain accepts
	difficulty int          // Mining difficulty (number of leading zeros required)
	mutex      sync.RWMutex // Mutex to ensure thread-safe access to the blockchain
	consensus  interface{}  // Interface to the consensus mechanism
}

// NestedObject defines the structure for nested objects that can represent complex data structures
type NestedObject map[string]interface{}

// CriticalProcess represents a critical process included in a block
type CriticalProcess struct {
	ProcessID             string       `json:"processId"`
	HashData              string       `json:"hashData"`
	OriginalDataStructure NestedObject `json:"originalDataStructure"`
	Description           string       `json:"description"`
	Timestamp             string       `json:"timestamp"`
	Signature             string       `json:"signature"`
}

// BlockDataTransaction defines the structure of transaction data
type BlockDataTransaction struct {
	Amount    float64 `json:"amount"`
	Sender    string  `json:"sender"`
	Recipient string  `json:"recipient"`
}

// Transaction represents a transaction included in a block
type Transaction struct {
	ProcessID   string               `json:"processId"`
	Description string               `json:"description"`
	Data        BlockDataTransaction `json:"data"`
	Timestamp   string               `json:"timestamp"`
	Signature   string               `json:"signature"`
	GasLimit    uint64               `json:"gasLimit"`
}
