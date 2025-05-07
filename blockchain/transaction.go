package blockchain

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

// GetID returns the process ID of a transaction
func (t *Transaction) GetID() string {
	return t.ProcessID
}

// GetTimestamp returns the timestamp of a transaction
func (t *Transaction) GetTimestamp() string {
	return t.Timestamp
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

// GetID returns the process ID of a critical process
func (c *CriticalProcess) GetID() string {
	return c.ProcessID
}

// GetTimestamp returns the timestamp of a critical process
func (c *CriticalProcess) GetTimestamp() string {
	return c.Timestamp
}
