package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"tripcodechain_go/security" // Added for Signer interface
	// "tripcodechain_go/utils" // Will be needed if logging is added here
)

// MerkleRoot calculates the Merkle root of transactions and critical processes.
// This is a placeholder and should be implemented properly.
func (b *Block) MerkleRoot() string {
	if b.Type == TransactionBlock && len(b.Transactions) > 0 {
		// In a real implementation, you would hash each transaction and build a Merkle tree.
		// For now, just hash the JSON of the first transaction as a placeholder.
		firstTxData, _ := json.Marshal(b.Transactions[0])
		hash := sha256.Sum256(firstTxData)
		return hex.EncodeToString(hash[:])
	} else if b.Type == CriticalProcessBlock && len(b.CriticalProcesses) > 0 {
		// Placeholder for critical processes
		firstCpData, _ := json.Marshal(b.CriticalProcesses[0])
		hash := sha256.Sum256(firstCpData)
		return hex.EncodeToString(hash[:])
	}
	return ""
}

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
	Signature         string             `json:"signature"`                   // Hex-encoded Ed25519 signature of the block's content hash
	Validator         string             `json:"validator"`                   // Hex-encoded Ed25519 public key of the block signer
	TotalFees         float64            `json:"totalFees,omitempty"`         // Total fees collected by the validator
	Data              string             `json:"data,omitempty"`              // Generic data field, usage depends on block type
	MerkleRoot        string             `json:"merkleRoot,omitempty"`        // Merkle root of transactions/processes
	Difficulty        int                `json:"difficulty,omitempty"`        // Difficulty target for PoW
}

/**
 * CalculateHash generates a SHA-256 hash of the block's content, excluding the Signature.
 * This hash is what gets signed.
 */
func (b *Block) CalculateHash() string {
	// Ensure MerkleRoot is calculated if not already set (e.g. before signing)
	// This is a simplified placeholder; proper Merkle root calculation should be robust.
	if b.MerkleRoot == "" {
		b.MerkleRoot = b.MerkleRoot() // Calculate if empty
	}

	txJSON, _ := json.Marshal(b.Transactions)
	cpJSON, _ := json.Marshal(b.CriticalProcesses)

	// Fields to include in the hash (excluding Signature)
	// Order matters for consistent hashing.
	var records []string
	records = append(records, strconv.FormatUint(b.Index, 10))
	records = append(records, b.Timestamp)
	records = append(records, string(txJSON))
	records = append(records, string(cpJSON))
	records = append(records, b.PreviousHash)
	records = append(records, b.Validator) // Validator's address IS part of the signed content
	records = append(records, strconv.FormatUint(b.Nonce, 10))
	records = append(records, b.MerkleRoot)
	records = append(records, strconv.Itoa(b.Difficulty))
	records = append(records, fmt.Sprintf("%.8f", b.TotalFees)) // Consistent float formatting
	records = append(records, string(b.Type))

	// Concatenate all parts into a single string for hashing
	fullRecord := strings.Join(records, "|") // Use a delimiter

	h := sha256.New()
	h.Write([]byte(fullRecord))
	return hex.EncodeToString(h.Sum(nil))
}

// Sign calculates the content hash, signs it using the provided signer,
// and stores the hex-encoded signature. It expects b.Validator to be already set.
func (b *Block) Sign(signer security.Signer) error {
	if signer == nil {
		return errors.New("signer cannot be nil")
	}
	if b.Validator == "" {
		return errors.New("validator address not set on block before signing")
	}
	// Optional: Check if the block's validator matches the signer's address.
	// This depends on whether a block can be signed by a delegate or only the validator themselves.
	// For now, we assume the caller ensures consistency or it's not a strict requirement here.
	// if b.Validator != signer.Address() {
	// 	return fmt.Errorf("block validator address '%s' does not match signer address '%s'", b.Validator, signer.Address())
	// }

	contentHashHex := b.CalculateHash() // This hash is of content including b.Validator
	hashBytes, err := hex.DecodeString(contentHashHex)
	if err != nil {
		return fmt.Errorf("failed to decode content hash for signing: %w", err)
	}

	signatureBytes, err := signer.Sign(hashBytes)
	if err != nil {
		return fmt.Errorf("failed to sign block hash: %w", err)
	}

	b.Signature = hex.EncodeToString(signatureBytes)
	return nil
}

// VerifySignature verifies the block's Ed25519 signature.
// It recalculates the content hash and checks it against the stored signature
// using the Validator's public key.
func (b *Block) VerifySignature() (bool, error) {
	if b.Validator == "" {
		return false, errors.New("block validator (public key) is empty")
	}
	publicKeyBytes, err := hex.DecodeString(b.Validator)
	if err != nil {
		return false, fmt.Errorf("failed to decode validator public key: %w", err)
	}

	if b.Signature == "" {
		return false, errors.New("block signature is empty")
	}
	signatureBytes, err := hex.DecodeString(b.Signature)
	if err != nil {
		return false, fmt.Errorf("failed to decode block signature: %w", err)
	}

	// Recalculate the content hash. This hash must be of the same fields
	// that were included when b.Sign() called b.CalculateHash().
	// Crucially, b.Validator was already set before CalculateHash in Sign().
	expectedHashHex := b.CalculateHash()
	expectedHashBytes, err := hex.DecodeString(expectedHashHex)
	if err != nil {
		return false, fmt.Errorf("failed to decode expected content hash for verification: %w", err)
	}

	// Verify using ed25519
	isValid := ed25519.Verify(publicKeyBytes, expectedHashBytes, signatureBytes)
	return isValid, nil
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
 * ForgeBlock performs the Proof-of-Work by finding a suitable nonce.
 * It sets the MerkleRoot, Timestamp, and the final content Hash.
 * It uses the `b.Validator` field (expected to be pre-set) for hash calculation during PoW.
 */
func (b *Block) ForgeBlock() {
	if b.Validator == "" {
		// This case should ideally not happen if called correctly from handlers.
		// If it does, PoW will be based on an empty validator string, which is consistent
		// but likely not what's intended for a block that will later be signed.
		// Consider logging a warning here if this state is unexpected.
		// utils.LogInfo("WARN: ForgeBlock called with empty b.Validator. PoW hash will reflect this.")
	}
	b.Timestamp = time.Now().UTC().Format(time.RFC3339)
	b.MerkleRoot = b.MerkleRoot() // Calculate Merkle root

	b.Nonce = 0 // Reset nonce
	if b.Difficulty > 0 {
		// MineBlock uses b.CalculateHash(), which now correctly includes the pre-set b.Validator.
		b.MineBlock(b.Difficulty) // This sets b.Nonce and b.Hash
	} else {
		b.Hash = b.CalculateHash() // Calculate hash once if no PoW mining
	}
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
