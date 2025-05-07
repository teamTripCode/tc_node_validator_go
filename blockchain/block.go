package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"time"
)

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
}

/**
 * CalculateHash generates a SHA-256 hash of the block based on its contents.
 * This is used both for mining and forging the block.
 */
func (b *Block) CalculateHash() string {
	txJSON, _ := json.Marshal(b.Transactions)
	cpJSON, _ := json.Marshal(b.CriticalProcesses)
	record := strconv.FormatUint(b.Index, 10) +
		b.PreviousHash +
		b.Timestamp +
		string(txJSON) +
		string(cpJSON) +
		strconv.FormatUint(b.Nonce, 10) +
		b.Signature

	h := sha256.New()
	h.Write([]byte(record))

	return hex.EncodeToString(h.Sum(nil))
}

/**
 * MineBlock performs Proof-of-Work mining by incrementing the nonce until
 * the resulting hash starts with a number of zeros equal to the difficulty.
 *
 * Parameters:
 *   - difficulty: The number of leading zero characters required in the hash
 */
func (b *Block) MineBlock(difficulty int) {
	prefix := make([]byte, difficulty)
	for i := 0; i < difficulty; i++ {
		prefix[i] = '0'
	}

	for {
		b.Hash = b.CalculateHash()
		if b.Hash[:difficulty] == string(prefix) {
			break
		}
		b.Nonce++
	}
}

/**
 * ForgeBlock finalizes the block creation process by setting the validator
 * and updating the timestamp. It also recalculates the block hash.
 *
 * Parameters:
 *   - validator: Identifier (e.g., public key) of the validator forging the block
 */
func (b *Block) ForgeBlock(validator string) {
	b.Validator = validator
	b.Timestamp = time.Now().UTC().Format(time.RFC3339)
	b.Hash = b.CalculateHash()
}

/**
 * NewBlock initializes a new block with default values.
 * It sets the index, previous hash, type, and prepares it for mining or forging.
 *
 * Parameters:
 *   - index: Position of the block in the blockchain
 *   - previousHash: Hash of the last block in the chain
 *   - blockType: Type of block being created (e.g., transaction or critical process)
 *
 * Returns:
 *   - A pointer to the newly created block
 */
func NewBlock(index uint64, previousHash string, blockType BlockType) *Block {
	return &Block{
		Index:        index,
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		Type:         blockType,
		PreviousHash: previousHash,
		Nonce:        0,
		Hash:         "",
		Signature:    "",
		Validator:    "",
	}
}
