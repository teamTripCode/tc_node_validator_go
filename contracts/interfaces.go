package contracts

import (
	"sync"
	"tripcodechain_go/currency"
)

// ContractState represents the state of a contract
type ContractState map[string]any

// ContractOperation defines an operation that can be executed in a contract
type ContractOperation struct {
	OpCode       string        `json:"opCode"`       // Operation code (ADD, SUB, TRANSFER, etc.)
	Args         []interface{} `json:"args"`         // Arguments for the operation
	RequiresAuth bool          `json:"requiresAuth"` // Whether this operation requires authentication
}

// Contract represents a smart contract in the TripCoin network
type Contract struct {
	Address      string                 `json:"address"`      // Contract address
	Creator      string                 `json:"creator"`      // Address of the contract creator
	Code         []*ContractOperation   `json:"code"`         // Contract code (list of operations)
	State        ContractState          `json:"state"`        // Contract state
	Balance      *currency.Balance      `json:"balance"`      // Contract balance in TCC
	CreatedAt    string                 `json:"createdAt"`    // Contract creation timestamp
	LastExecuted string                 `json:"lastExecuted"` // Last execution timestamp
	CodeHash     string                 `json:"codeHash"`     // Hash of the contract code
	Metadata     map[string]interface{} `json:"metadata"`     // Additional contract metadata
	mutex        sync.RWMutex           // Mutex for thread-safe operations
}

// ContractInvocation represents a call to a contract
type ContractInvocation struct {
	ContractAddress string        `json:"contractAddress"` // Contract address
	Method          string        `json:"method"`          // Method name to call
	Args            []interface{} `json:"args"`            // Method arguments
	Caller          string        `json:"caller"`          // Address of the caller
	Value           string        `json:"value"`           // TCC value to send with the call
	GasLimit        uint64        `json:"gasLimit"`        // Maximum gas allowed
	GasPrice        string        `json:"gasPrice"`        // Gas price in wei
	Nonce           uint64        `json:"nonce"`           // Caller's nonce
	Timestamp       string        `json:"timestamp"`       // Invocation timestamp
	Signature       string        `json:"signature"`       // Signature of the invocation
}

// ContractExecution represents a result of contract execution
type ContractExecution struct {
	Success      bool                   `json:"success"`      // Whether execution was successful
	GasUsed      uint64                 `json:"gasUsed"`      // Amount of gas used
	ReturnValue  interface{}            `json:"returnValue"`  // Return value of the execution
	ErrorMessage string                 `json:"errorMessage"` // Error message if execution failed
	StateUpdates map[string]interface{} `json:"stateUpdates"` // State changes made
	Events       []*ContractEvent       `json:"events"`       // Events emitted
	Logs         []string               `json:"logs"`         // Execution logs
}

// ContractEvent represents an event emitted by a contract
type ContractEvent struct {
	ContractAddress string                 `json:"contractAddress"` // Contract address
	EventName       string                 `json:"eventName"`       // Event name
	Data            map[string]interface{} `json:"data"`            // Event data
	BlockNumber     uint64                 `json:"blockNumber"`     // Block number when event was emitted
	Timestamp       string                 `json:"timestamp"`       // Event timestamp
}

// ContractManager manages smart contracts in the TripCoin network
type ContractManager struct {
	contracts      map[string]*Contract       // Map of contract addresses to contracts
	currencyMgr    *currency.CurrencyManager  // Reference to currency manager
	globalState    map[string]ContractState   // Global state of contracts
	eventListeners map[string][]EventListener // Map of event names to listeners
	mutex          sync.RWMutex               // Mutex for thread-safe operations
}

// EventListener represents a callback function for contract events
type EventListener func(event *ContractEvent)

// ContractMethod defines a standard interface for contract methods
type ContractMethod struct {
	Name        string                 `json:"name"`        // Method name
	Description string                 `json:"description"` // Method description
	Args        []string               `json:"args"`        // Argument names
	Operations  []*ContractOperation   `json:"operations"`  // Operations to execute
	Metadata    map[string]interface{} `json:"metadata"`    // Additional method metadata
	IsPublic    bool                   `json:"isPublic"`    // Whether the method is publicly callable
}

// ContractTemplate represents a predefined contract template
type ContractTemplate struct {
	Name        string                    `json:"name"`        // Template name
	Description string                    `json:"description"` // Template description
	Methods     map[string]ContractMethod `json:"methods"`     // Methods provided by the template
	InitState   ContractState             `json:"initState"`   // Initial state
	Metadata    map[string]interface{}    `json:"metadata"`    // Additional template metadata
}
