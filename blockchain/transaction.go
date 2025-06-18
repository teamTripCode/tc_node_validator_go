package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv" // Added for FlexTimestamp
	"strings" // Added for FlexTimestamp
	"time"    // Added for NewTransaction helper
)

// FlexTimestamp is a type that can unmarshal an int64 or a string representing an int64 from JSON.
type FlexTimestamp int64

// UnmarshalJSON implements the json.Unmarshaler interface.
func (ft *FlexTimestamp) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as an int64 directly.
	var i int64
	if err := json.Unmarshal(data, &i); err == nil {
		*ft = FlexTimestamp(i)
		return nil
	}

	// If direct unmarshalling fails, try to unmarshal as a string and convert.
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("timestamp is not a valid string or integer: %v", err)
	}

	// Remove quotes if present (json.Unmarshal to string sometimes keeps them)
	s = strings.Trim(s, "\"")

	parsedInt, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse timestamp string to int64: %v", err)
	}
	*ft = FlexTimestamp(parsedInt)
	return nil
}

// MarshalJSON implements the json.Marshaler interface to ensure it's always marshalled as an int64.
func (ft FlexTimestamp) MarshalJSON() ([]byte, error) {
	return json.Marshal(int64(ft))
}

// Transaction types
const (
	TxTransfer       = "TRANSFER"
	TxLogMCPActivity = "LOG_MCP_ACTIVITY"
	// Add other transaction types here
)

// TransferData defines the structure of transaction data for TxTransfer
type TransferData struct {
	Amount    float64 `json:"amount"`
	Sender    string  `json:"sender"`
	Recipient string  `json:"recipient"`
}

// LogMCPActivityData defines the structure for TxLogMCPActivity
type LogMCPActivityData struct {
	QueryID                string   `json:"queryId"`
	RequestHash            string   `json:"requestHash"`            // Hash of the initial LLM request
	AggregatedResponseHash string   `json:"aggregatedResponseHash"` // Hash of the aggregated LLM response
	OriginNodeID           string   `json:"originNodeId"`           // Node that initiated the LLM query
	McpNodeIDs             []string `json:"mcpNodeIds"`             // IDs of MCP nodes that participated
	Timestamp              int64    `json:"timestamp"`              // Unix timestamp
}

// NestedObject defines the structure for nested objects that can represent complex data structures
type NestedObject map[string]any

// CriticalProcess represents a critical process included in a block
type CriticalProcess struct {
	ProcessID             string       `json:"processId"`
	HashData              string       `json:"hashData"`
	OriginalDataStructure NestedObject `json:"originalDataStructure"`
	Description           string       `json:"description"`
	Timestamp             string       `json:"timestamp"` // Consider int64 for Unix timestamp for consistency
	Signature             string       `json:"signature"`
}

// Transaction represents a transaction included in a block
type Transaction struct {
	ProcessID   string          `json:"processId,omitempty"` // Could be deprecated if Hash becomes the primary ID
	Hash        string          `json:"hash"`                // Hash of the transaction (primary identifier)
	Type        string          `json:"type"`                // Type of the transaction (e.g., TxTransfer, TxLogMCPActivity)
	Description string          `json:"description,omitempty"`
	Data        json.RawMessage `json:"data"`                // Holds specific data based on Type
	Timestamp   FlexTimestamp   `json:"timestamp"`           // Unix timestamp
	Signature   string          `json:"signature,omitempty"` // Signature of the transaction hash
	GasLimit    uint64          `json:"gasLimit"`
	// Nonce      uint64          `json:"nonce"` // Often included for replay protection
}

// CalculateHash calculates the SHA256 hash of the transaction.
// It serializes parts of the transaction (excluding Hash and Signature)
// and then hashes the serialized data.
func (t *Transaction) CalculateHash() (string, error) {
	// Define what parts of the transaction contribute to its hash
	hashablePart := struct {
		Type        string          `json:"type"`
		Description string          `json:"description,omitempty"`
		Data        json.RawMessage `json:"data"`
		Timestamp   FlexTimestamp   `json:"timestamp"` // Changed to FlexTimestamp
		GasLimit    uint64          `json:"gasLimit"`
		ProcessID   string          `json:"processId,omitempty"` // Include if still relevant for identity before hashing
		// Add Nonce here if it's part of the hashable content
	}{
		Type:        t.Type,
		Description: t.Description,
		Data:        t.Data,
		Timestamp:   t.Timestamp,
		GasLimit:    t.GasLimit,
		ProcessID:   t.ProcessID,
	}

	serializedTx, err := json.Marshal(hashablePart)
	if err != nil {
		return "", fmt.Errorf("failed to serialize transaction for hashing: %w", err)
	}
	hashBytes := sha256.Sum256(serializedTx)
	return hex.EncodeToString(hashBytes[:]), nil
}

// Validate performs basic validation on the transaction structure and type-specific validation.
func (t *Transaction) Validate() error {
	if t.Hash == "" {
		// In some models, hash might be calculated here if not present,
		// but for validation, it's usually expected to be pre-calculated and set.
		return errors.New("transaction hash is missing")
	}
	// Verify that the provided hash is correct (optional, can be computationally expensive here)
	// currentHash, err := t.CalculateHash()
	// if err != nil {
	// 	return fmt.Errorf("failed to calculate current hash for validation: %w", err)
	// }
	// if t.Hash != currentHash {
	// 	return fmt.Errorf("transaction hash mismatch: provided %s, calculated %s", t.Hash, currentHash)
	// }

	if t.Timestamp <= FlexTimestamp(0) { // Timestamps should be positive // <--- MODIFIED HERE
		return errors.New("transaction timestamp is invalid")
	}
	if t.Type == "" {
		return errors.New("transaction type is missing")
	}
	// Basic GasLimit check (e.g., non-zero, within a reasonable max if defined)
	if t.GasLimit == 0 {
		return errors.New("transaction gasLimit must be positive")
	}

	// Signature presence (actual verification is separate)
	// if t.Signature == "" {
	// 	return errors.New("transaction signature is missing")
	// }

	switch t.Type {
	case TxTransfer:
		var transferData TransferData
		if err := json.Unmarshal(t.Data, &transferData); err != nil {
			return fmt.Errorf("invalid TxTransfer data: %w", err)
		}
		if transferData.Sender == "" {
			return errors.New("TxTransfer sender is missing")
		}
		if transferData.Recipient == "" {
			return errors.New("TxTransfer recipient is missing")
		}
		if transferData.Amount <= 0 {
			return errors.New("TxTransfer amount must be positive")
		}
	case TxLogMCPActivity:
		var mcpData LogMCPActivityData
		if err := json.Unmarshal(t.Data, &mcpData); err != nil {
			return fmt.Errorf("invalid TxLogMCPActivity data: %w", err)
		}
		if mcpData.QueryID == "" {
			return errors.New("TxLogMCPActivity QueryID is missing")
		}
		if mcpData.Timestamp <= 0 { // Assuming timestamp should be positive
			return errors.New("TxLogMCPActivity Timestamp is invalid")
		}
		if mcpData.RequestHash == "" {
			return errors.New("TxLogMCPActivity RequestHash is missing")
		}
		if mcpData.AggregatedResponseHash == "" {
			return errors.New("TxLogMCPActivity AggregatedResponseHash is missing")
		}
		if mcpData.OriginNodeID == "" {
			return errors.New("TxLogMCPActivity OriginNodeID is missing")
		}
		// McpNodeIDs can be empty.
	default:
		return fmt.Errorf("unknown transaction type: '%s'", t.Type)
	}
	return nil
}

// SignWithPrivateKey (placeholder) - signs the transaction hash with a private key from a wallet/signer.
// The actual signing logic would involve a crypto library (e.g., ecdsa).
// This method should ideally take a signer interface or private key bytes.
func (t *Transaction) SignWithPrivateKey(privateKeyHex string /* Placeholder type */) error {
	// Ensure hash is calculated and set before signing
	if t.Hash == "" {
		calculatedHash, err := t.CalculateHash()
		if err != nil {
			return fmt.Errorf("could not calculate hash for signing: %w", err)
		}
		t.Hash = calculatedHash
	}
	// This is a dummy signature. Real implementation needed.
	t.Signature = fmt.Sprintf("signed(%s-by-%s)", t.Hash, privateKeyHex)
	return nil
}

// VerifySignature (placeholder) - verifies the transaction signature.
// This would take the public key corresponding to the private key used for signing.
func (t *Transaction) VerifySignature(publicKeyHex string /* Placeholder type */) (bool, error) {
	if t.Hash == "" {
		return false, errors.New("cannot verify signature of transaction with empty hash")
	}
	if t.Signature == "" {
		return false, errors.New("transaction signature is missing")
	}
	// This is a dummy verification. Real implementation needed.
	expectedSignature := fmt.Sprintf("signed(%s-by-%s)", t.Hash, publicKeyHex) // Assuming privateKey was used as publicKey for dummy
	return t.Signature == expectedSignature, nil
}

// GetID returns the primary identifier (hash) of a transaction.
func (t *Transaction) GetID() string {
	return t.Hash // ProcessID is deprecated in favor of Hash
}

// GetTimestamp returns the timestamp of a transaction as a string.
// This is to satisfy the MempoolItem interface.
func (t *Transaction) GetTimestamp() string {
	return fmt.Sprintf("%d", int64(t.Timestamp)) // <--- MODIFIED HERE
}

// GetRawTimestamp returns the raw int64 timestamp.
func (t *Transaction) GetRawTimestamp() int64 {
	return int64(t.Timestamp) // <--- MODIFIED HERE
}

// GetID returns the process ID of a critical process.
func (c *CriticalProcess) GetID() string {
	return c.ProcessID
}

// GetTimestamp returns the timestamp of a critical process.
// For CriticalProcess, Timestamp is still string. Consider unifying to int64.
func (c *CriticalProcess) GetTimestamp() string {
	return c.Timestamp
}

// NewTransaction is a helper to create a new transaction and calculate its hash.
// Specific data should be marshalled to json.RawMessage before calling.
func NewTransaction(txType string, data json.RawMessage, gasLimit uint64, description string, processID ...string) (*Transaction, error) {
	ts := time.Now().Unix()
	tx := &Transaction{
		Type:        txType,
		Data:        data,
		Timestamp:   FlexTimestamp(ts), // <--- MODIFIED HERE
		GasLimit:    gasLimit,
		Description: description,
	}
	if len(processID) > 0 {
		tx.ProcessID = processID[0] // Optional, for backward compatibility or specific uses
	}

	hash, err := tx.CalculateHash()
	if err != nil {
		return nil, err
	}
	tx.Hash = hash
	return tx, nil
}
