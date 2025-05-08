// contracts/contracts.go
package contracts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"sync"
	"time"

	"tripcodechain_go/currency"
	"tripcodechain_go/utils"
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

// Operation codes
const (
	OpAdd        = "ADD"
	OpSub        = "SUB"
	OpMul        = "MUL"
	OpDiv        = "DIV"
	OpTransfer   = "TRANSFER"
	OpStore      = "STORE"
	OpLoad       = "LOAD"
	OpIf         = "IF"
	OpCompare    = "COMPARE"
	OpReturn     = "RETURN"
	OpCall       = "CALL"
	OpEmitEvent  = "EMIT_EVENT"
	OpRequire    = "REQUIRE"
	OpRevert     = "REVERT"
	OpNativeCall = "NATIVE_CALL"
)

// Gas costs for operations
const (
	GasCostBase         = 10   // Base cost for any operation
	GasCostStore        = 100  // Cost for storing a value
	GasCostLoad         = 20   // Cost for loading a value
	GasCostTransfer     = 500  // Cost for transferring funds
	GasCostCall         = 200  // Cost for calling another method
	GasCostEmitEvent    = 50   // Cost for emitting an event
	GasCostNativeCall   = 1000 // Cost for calling a native function
	GasCostCompute      = 5    // Cost per arithmetic operation
	GasCostContractCall = 1000 // Base cost for calling another contract
)

// NewContractManager creates a new contract manager
func NewContractManager(currencyMgr *currency.CurrencyManager) *ContractManager {
	return &ContractManager{
		contracts:      make(map[string]*Contract),
		currencyMgr:    currencyMgr,
		globalState:    make(map[string]ContractState),
		eventListeners: make(map[string][]EventListener),
	}
}

// GenerateContractAddress generates a unique address for a new contract
func GenerateContractAddress(creator string, nonce uint64, timestamp string) string {
	// Combine creator, nonce, and timestamp
	data := fmt.Sprintf("%s:%d:%s", creator, nonce, timestamp)

	// Create a hash
	hash := sha256.Sum256([]byte(data))

	// Return hex string with "contract-" prefix
	return "contract-" + hex.EncodeToString(hash[:])[:40]
}

// GetContract returns a contract by address
func (cm *ContractManager) GetContract(address string) (*Contract, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	contract, exists := cm.contracts[address]
	if !exists {
		return nil, fmt.Errorf("contract not found: %s", address)
	}

	return contract, nil
}

// CreateContract creates a new contract
func (cm *ContractManager) CreateContract(creator string, code []*ContractOperation, initialState ContractState, value *currency.Balance) (*Contract, error) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Get creator account
	creatorAccount := cm.currencyMgr.GetAccount(creator)

	// Validate balance if value is provided
	if value != nil && value.Int.Cmp(big.NewInt(0)) > 0 {
		if creatorAccount.Balance.Int.Cmp(value.Int) < 0 {
			return nil, fmt.Errorf("insufficient balance for contract creation")
		}
	}

	// Generate contract address
	now := time.Now().UTC()
	timestamp := now.Format(time.RFC3339)
	address := GenerateContractAddress(creator, creatorAccount.Nonce, timestamp)

	// Convert code to JSON to create hash
	codeBytes, err := json.Marshal(code)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal contract code: %v", err)
	}

	// Create code hash
	codeHash := sha256.Sum256(codeBytes)

	// Create contract
	contract := &Contract{
		Address:      address,
		Creator:      creator,
		Code:         code,
		State:        make(ContractState),
		Balance:      &currency.Balance{Int: big.NewInt(0)},
		CreatedAt:    timestamp,
		LastExecuted: timestamp,
		CodeHash:     hex.EncodeToString(codeHash[:]),
		Metadata:     make(map[string]interface{}),
	}

	// Initialize state if provided
	if initialState != nil {
		for k, v := range initialState {
			contract.State[k] = v
		}
	}

	// Transfer value if provided
	if value != nil && value.Int.Cmp(big.NewInt(0)) > 0 {
		err := cm.currencyMgr.TransferFunds(creator, address, value)
		if err != nil {
			return nil, fmt.Errorf("failed to transfer funds to contract: %v", err)
		}
		contract.Balance = value
	}

	// Store contract
	cm.contracts[address] = contract

	// Increment creator's nonce
	creatorAccount.Nonce++

	utils.LogInfo("Contract created: %s by %s", address, creator)

	return contract, nil
}

// ExecuteContract executes a contract invocation
func (cm *ContractManager) ExecuteContract(invocation *ContractInvocation) (*ContractExecution, error) {
	// Validate invocation
	if invocation.ContractAddress == "" {
		return nil, fmt.Errorf("contract address is required")
	}

	if invocation.Method == "" {
		return nil, fmt.Errorf("method name is required")
	}

	// Get contract
	contract, err := cm.GetContract(invocation.ContractAddress)
	if err != nil {
		return nil, err
	}

	// Validate gas limit
	if invocation.GasLimit == 0 {
		invocation.GasLimit = currency.DefaultGasLimit
	}

	// Create execution context
	execution := &ContractExecution{
		Success:      false,
		GasUsed:      0,
		StateUpdates: make(map[string]interface{}),
		Events:       make([]*ContractEvent, 0),
		Logs:         make([]string, 0),
	}

	// Get value to transfer if provided
	var value *currency.Balance
	if invocation.Value != "" {
		value, err = currency.NewBalanceFromString(invocation.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid value: %v", err)
		}

		// Transfer value to contract
		if value.Int.Cmp(big.NewInt(0)) > 0 {
			err = cm.currencyMgr.TransferFunds(invocation.Caller, invocation.ContractAddress, value)
			if err != nil {
				return nil, fmt.Errorf("failed to transfer value to contract: %v", err)
			}

			// Update contract balance
			contract.mutex.Lock()
			contract.Balance = contract.Balance.Add(value)
			contract.mutex.Unlock()
		}
	}

	// Find method in contract
	found := false
	methodOperations := make([]*ContractOperation, 0)

	// Look for method in contract code
	// This is a simplified implementation - in a real system,
	// contracts would have a more structured organization of methods
	for _, op := range contract.Code {
		if op.OpCode == "METHOD" && len(op.Args) >= 1 {
			if methodName, ok := op.Args[0].(string); ok && methodName == invocation.Method {
				found = true

				// Find all operations for this method until next METHOD or end
				for _, methodOp := range contract.Code {
					if methodOp.OpCode == "METHOD" {
						if found && methodOp.OpCode != "METHOD" {
							methodOperations = append(methodOperations, methodOp)
						}
					}
				}
				break
			}
		}
	}

	if !found {
		return nil, fmt.Errorf("method not found: %s", invocation.Method)
	}

	// Execute method operations
	result, gasUsed, err := cm.executeOperations(contract, methodOperations, invocation, execution)

	// Update execution results
	execution.GasUsed = gasUsed
	execution.Success = (err == nil)

	if err != nil {
		execution.ErrorMessage = err.Error()
	} else {
		execution.ReturnValue = result
	}

	// Update contract last executed timestamp
	contract.mutex.Lock()
	contract.LastExecuted = time.Now().UTC().Format(time.RFC3339)
	contract.mutex.Unlock()

	// Process gas fees
	gasPrice, err := currency.NewBalanceFromString(invocation.GasPrice)
	if err != nil {
		gasPrice = &currency.Balance{Int: big.NewInt(currency.DefaultGasPrice)}
	}

	totalFee := cm.currencyMgr.CalculateTransactionFee(gasUsed, gasPrice)
	utils.LogInfo("Contract execution gas used: %d, fee: %s", gasUsed, totalFee.TripCoinString())

	// Transfer gas fee from caller
	err = cm.currencyMgr.TransferFunds(invocation.Caller, "SYSTEM_FEES", totalFee)
	if err != nil {
		utils.LogError("Failed to collect gas fee: %v", err)
	}

	return execution, nil
}

// executeOperations executes a sequence of contract operations
func (cm *ContractManager) executeOperations(contract *Contract, operations []*ContractOperation, invocation *ContractInvocation, execution *ContractExecution) (any, uint64, error) {
	var result interface{}
	var gasUsed uint64 = 0

	// Create a local state that will be committed if execution succeeds
	localState := make(ContractState)
	for k, v := range contract.State {
		localState[k] = v
	}

	// Execute operations sequentially
	for _, op := range operations {
		// Check gas limit
		if gasUsed >= invocation.GasLimit {
			return nil, gasUsed, fmt.Errorf("out of gas")
		}

		// Execute operation based on OpCode
		opResult, opGasUsed, err := cm.executeOperation(op, contract, localState, invocation, execution)
		if err != nil {
			return nil, gasUsed, err
		}

		gasUsed += opGasUsed

		// Handle return statements
		if op.OpCode == OpReturn {
			result = opResult
			break
		}
	}

	// Commit state changes if execution was successful
	contract.mutex.Lock()
	for k, v := range localState {
		contract.State[k] = v
		execution.StateUpdates[k] = v
	}
	contract.mutex.Unlock()

	return result, gasUsed, nil
}

// executeOperation executes a single contract operation
func (cm *ContractManager) executeOperation(op *ContractOperation, contract *Contract, state ContractState, invocation *ContractInvocation, execution *ContractExecution) (any, uint64, error) {
	var gasUsed uint64 = GasCostBase

	// Log operation execution
	execution.Logs = append(execution.Logs, fmt.Sprintf("Executing %s operation", op.OpCode))

	// Check authorization if required
	if op.RequiresAuth && invocation.Caller != contract.Creator {
		return nil, gasUsed, fmt.Errorf("operation requires authorization")
	}

	// Execute operation based on OpCode
	switch op.OpCode {
	case OpStore:
		// STORE operation: store a value in contract state
		if len(op.Args) < 2 {
			return nil, gasUsed, fmt.Errorf("STORE requires key and value arguments")
		}

		key, ok := op.Args[0].(string)
		if !ok {
			return nil, gasUsed, fmt.Errorf("STORE key must be a string")
		}

		value := op.Args[1]
		state[key] = value

		gasUsed += GasCostStore
		return value, gasUsed, nil

	case OpLoad:
		// LOAD operation: load a value from contract state
		if len(op.Args) < 1 {
			return nil, gasUsed, fmt.Errorf("LOAD requires key argument")
		}

		key, ok := op.Args[0].(string)
		if !ok {
			return nil, gasUsed, fmt.Errorf("LOAD key must be a string")
		}

		value, exists := state[key]
		if !exists {
			return nil, gasUsed, fmt.Errorf("key not found: %s", key)
		}

		gasUsed += GasCostLoad
		return value, gasUsed, nil

	case OpTransfer:
		// TRANSFER operation: transfer funds from contract
		if len(op.Args) < 2 {
			return nil, gasUsed, fmt.Errorf("TRANSFER requires to and amount arguments")
		}

		to, ok := op.Args[0].(string)
		if !ok {
			return nil, gasUsed, fmt.Errorf("TRANSFER to address must be a string")
		}

		amountStr, ok := op.Args[1].(string)
		if !ok {
			return nil, gasUsed, fmt.Errorf("TRANSFER amount must be a string")
		}

		amount, err := currency.NewBalanceFromString(amountStr)
		if err != nil {
			return nil, gasUsed, fmt.Errorf("invalid amount: %v", err)
		}

		// Check contract balance
		if contract.Balance.Int.Cmp(amount.Int) < 0 {
			return nil, gasUsed, fmt.Errorf("insufficient contract balance")
		}

		// Transfer funds
		err = cm.currencyMgr.TransferFunds(contract.Address, to, amount)
		if err != nil {
			return nil, gasUsed, fmt.Errorf("transfer failed: %v", err)
		}

		// Update contract balance
		contract.mutex.Lock()
		contract.Balance = contract.Balance.Sub(amount)
		contract.mutex.Unlock()

		gasUsed += GasCostTransfer
		return true, gasUsed, nil

	case OpEmitEvent:
		// EMIT_EVENT operation: emit a contract event
		if len(op.Args) < 2 {
			return nil, gasUsed, fmt.Errorf("EMIT_EVENT requires name and data arguments")
		}

		eventName, ok := op.Args[0].(string)
		if !ok {
			return nil, gasUsed, fmt.Errorf("event name must be a string")
		}

		eventData, ok := op.Args[1].(map[string]interface{})
		if !ok {
			// Try to convert to map if possible
			eventData = make(map[string]interface{})
			eventData["value"] = op.Args[1]
		}

		event := &ContractEvent{
			ContractAddress: contract.Address,
			EventName:       eventName,
			Data:            eventData,
			BlockNumber:     0, // Will be set when added to block
			Timestamp:       time.Now().UTC().Format(time.RFC3339),
		}

		// Add event to execution results
		execution.Events = append(execution.Events, event)

		// Notify event listeners
		cm.notifyEventListeners(event)

		gasUsed += GasCostEmitEvent
		return event, gasUsed, nil

	case OpAdd:
		// ADD operation: add two numeric values
		if len(op.Args) < 2 {
			return nil, gasUsed, fmt.Errorf("ADD requires two arguments")
		}

		// Try to handle different numeric types
		switch a := op.Args[0].(type) {
		case float64:
			if b, ok := op.Args[1].(float64); ok {
				gasUsed += GasCostCompute
				return a + b, gasUsed, nil
			}
		case int:
			if b, ok := op.Args[1].(int); ok {
				gasUsed += GasCostCompute
				return a + b, gasUsed, nil
			}
		case int64:
			if b, ok := op.Args[1].(int64); ok {
				gasUsed += GasCostCompute
				return a + b, gasUsed, nil
			}
		case string:
			if b, ok := op.Args[1].(string); ok {
				// Try to parse as big.Int
				aBig, success := new(big.Int).SetString(a, 10)
				if !success {
					return nil, gasUsed, fmt.Errorf("invalid number format: %s", a)
				}

				bBig, success := new(big.Int).SetString(b, 10)
				if !success {
					return nil, gasUsed, fmt.Errorf("invalid number format: %s", b)
				}

				result := new(big.Int).Add(aBig, bBig)
				gasUsed += GasCostCompute
				return result.String(), gasUsed, nil
			}
		}

		return nil, gasUsed, fmt.Errorf("ADD requires numeric arguments")

	case OpSub:
		// SUB operation: subtract second value from first
		if len(op.Args) < 2 {
			return nil, gasUsed, fmt.Errorf("SUB requires two arguments")
		}

		// Try to handle different numeric types
		switch a := op.Args[0].(type) {
		case float64:
			if b, ok := op.Args[1].(float64); ok {
				gasUsed += GasCostCompute
				return a - b, gasUsed, nil
			}
		case int:
			if b, ok := op.Args[1].(int); ok {
				gasUsed += GasCostCompute
				return a - b, gasUsed, nil
			}
		case int64:
			if b, ok := op.Args[1].(int64); ok {
				gasUsed += GasCostCompute
				return a - b, gasUsed, nil
			}
		case string:
			if b, ok := op.Args[1].(string); ok {
				// Try to parse as big.Int
				aBig, success := new(big.Int).SetString(a, 10)
				if !success {
					return nil, gasUsed, fmt.Errorf("invalid number format: %s", a)
				}

				bBig, success := new(big.Int).SetString(b, 10)
				if !success {
					return nil, gasUsed, fmt.Errorf("invalid number format: %s", b)
				}

				result := new(big.Int).Sub(aBig, bBig)
				gasUsed += GasCostCompute
				return result.String(), gasUsed, nil
			}
		}

		return nil, gasUsed, fmt.Errorf("SUB requires numeric arguments")

	case OpMul:
		// MUL operation: multiply two values
		if len(op.Args) < 2 {
			return nil, gasUsed, fmt.Errorf("MUL requires two arguments")
		}

		// Try to handle different numeric types
		switch a := op.Args[0].(type) {
		case float64:
			if b, ok := op.Args[1].(float64); ok {
				gasUsed += GasCostCompute
				return a * b, gasUsed, nil
			}
		case int:
			if b, ok := op.Args[1].(int); ok {
				gasUsed += GasCostCompute
				return a * b, gasUsed, nil
			}
		case int64:
			if b, ok := op.Args[1].(int64); ok {
				gasUsed += GasCostCompute
				return a * b, gasUsed, nil
			}
		case string:
			if b, ok := op.Args[1].(string); ok {
				// Try to parse as big.Int
				aBig, success := new(big.Int).SetString(a, 10)
				if !success {
					return nil, gasUsed, fmt.Errorf("invalid number format: %s", a)
				}

				bBig, success := new(big.Int).SetString(b, 10)
				if !success {
					return nil, gasUsed, fmt.Errorf("invalid number format: %s", b)
				}

				result := new(big.Int).Mul(aBig, bBig)
				gasUsed += GasCostCompute
				return result.String(), gasUsed, nil
			}
		}

		return nil, gasUsed, fmt.Errorf("MUL requires numeric arguments")

	case OpDiv:
		// DIV operation: divide first value by second
		if len(op.Args) < 2 {
			return nil, gasUsed, fmt.Errorf("DIV requires two arguments")
		}

		// Try to handle different numeric types
		switch a := op.Args[0].(type) {
		case float64:
			if b, ok := op.Args[1].(float64); ok {
				if b == 0 {
					return nil, gasUsed, fmt.Errorf("division by zero")
				}
				gasUsed += GasCostCompute
				return a / b, gasUsed, nil
			}
		case int:
			if b, ok := op.Args[1].(int); ok {
				if b == 0 {
					return nil, gasUsed, fmt.Errorf("division by zero")
				}
				gasUsed += GasCostCompute
				return a / b, gasUsed, nil
			}
		case int64:
			if b, ok := op.Args[1].(int64); ok {
				if b == 0 {
					return nil, gasUsed, fmt.Errorf("division by zero")
				}
				gasUsed += GasCostCompute
				return a / b, gasUsed, nil
			}
		case string:
			if b, ok := op.Args[1].(string); ok {
				// Try to parse as big.Int
				aBig, success := new(big.Int).SetString(a, 10)
				if !success {
					return nil, gasUsed, fmt.Errorf("invalid number format: %s", a)
				}

				bBig, success := new(big.Int).SetString(b, 10)
				if !success {
					return nil, gasUsed, fmt.Errorf("invalid number format: %s", b)
				}

				if bBig.Cmp(big.NewInt(0)) == 0 {
					return nil, gasUsed, fmt.Errorf("division by zero")
				}

				result := new(big.Int).Div(aBig, bBig)
				gasUsed += GasCostCompute
				return result.String(), gasUsed, nil
			}
		}

		return nil, gasUsed, fmt.Errorf("DIV requires numeric arguments")

	case OpCompare:
		// COMPARE operation: compare two values
		if len(op.Args) < 3 {
			return nil, gasUsed, fmt.Errorf("COMPARE requires value1, value2, and operator arguments")
		}

		operator, ok := op.Args[2].(string)
		if !ok {
			return nil, gasUsed, fmt.Errorf("COMPARE operator must be a string")
		}

		// Handle equality comparison for any type
		if operator == "==" || operator == "equals" {
			// For simple types, do direct comparison
			if op.Args[0] == op.Args[1] {
				gasUsed += GasCostCompute
				return true, gasUsed, nil
			}

			// Try string comparison
			str1, isStr1 := op.Args[0].(string)
			str2, isStr2 := op.Args[1].(string)
			if isStr1 && isStr2 {
				gasUsed += GasCostCompute
				return str1 == str2, gasUsed, nil
			}

			// Try numeric comparison
			num1, isNum1 := op.Args[0].(float64)
			num2, isNum2 := op.Args[1].(float64)
			if isNum1 && isNum2 {
				gasUsed += GasCostCompute
				return num1 == num2, gasUsed, nil
			}

			// Fallback
			gasUsed += GasCostCompute
			return false, gasUsed, nil
		}

		// Numeric comparisons
		num1, ok1 := getNumericValue(op.Args[0])
		num2, ok2 := getNumericValue(op.Args[1])

		if !ok1 || !ok2 {
			return nil, gasUsed, fmt.Errorf("COMPARE requires numeric values for operator %s", operator)
		}

		gasUsed += GasCostCompute

		switch operator {
		case "<":
			return num1 < num2, gasUsed, nil
		case "<=":
			return num1 <= num2, gasUsed, nil
		case ">":
			return num1 > num2, gasUsed, nil
		case ">=":
			return num1 >= num2, gasUsed, nil
		case "!=":
			return num1 != num2, gasUsed, nil
		default:
			return nil, gasUsed, fmt.Errorf("unknown comparison operator: %s", operator)
		}

	case OpRequire:
		// REQUIRE operation: check a condition and revert if false
		if len(op.Args) < 1 {
			return nil, gasUsed, fmt.Errorf("REQUIRE requires at least one argument")
		}

		condition, ok := op.Args[0].(bool)
		if !ok {
			// Try to evaluate as a boolean
			if strCond, isStr := op.Args[0].(string); isStr {
				condition = strCond == "true"
			} else {
				numVal, isNum := getNumericValue(op.Args[0])
				if isNum {
					condition = numVal != 0
				} else {
					return nil, gasUsed, fmt.Errorf("REQUIRE condition must evaluate to a boolean")
				}
			}
		}

		// If condition is false, return error with message if provided
		if !condition {
			errMsg := "requirement failed"
			if len(op.Args) > 1 {
				if msgStr, ok := op.Args[1].(string); ok {
					errMsg = msgStr
				}
			}
			return nil, gasUsed, fmt.Errorf("%s", errMsg)
		}

		return true, gasUsed, nil

	case OpRevert:
		// REVERT operation: abort execution with message
		errMsg := "execution reverted"
		if len(op.Args) > 0 {
			if msgStr, ok := op.Args[0].(string); ok {
				errMsg = msgStr
			}
		}
		return nil, gasUsed, fmt.Errorf("%s", errMsg)

	case OpIf:
		// IF operation: conditional execution
		if len(op.Args) < 2 {
			return nil, gasUsed, fmt.Errorf("IF requires condition and operations arguments")
		}

		// Check condition
		condition, ok := op.Args[0].(bool)
		if !ok {
			// Try to evaluate as a boolean
			if strCond, isStr := op.Args[0].(string); isStr {
				condition = strCond == "true"
			} else {
				numVal, isNum := getNumericValue(op.Args[0])
				if isNum {
					condition = numVal != 0
				} else {
					return nil, gasUsed, fmt.Errorf("IF condition must evaluate to a boolean")
				}
			}
		}

		// Execute operations if condition is true
		if condition {
			operations, ok := op.Args[1].([]*ContractOperation)
			if !ok {
				return nil, gasUsed, fmt.Errorf("IF operations must be an array of operations")
			}

			// Execute operations in the IF block
			result, opGasUsed, err := cm.executeOperations(contract, operations, invocation, execution)
			gasUsed += opGasUsed
			if err != nil {
				return nil, gasUsed, err
			}
			return result, gasUsed, nil
		} else if len(op.Args) > 2 {
			// Execute ELSE block if exists
			operations, ok := op.Args[2].([]*ContractOperation)
			if !ok {
				return nil, gasUsed, fmt.Errorf("ELSE operations must be an array of operations")
			}

			// Execute operations in the ELSE block
			result, opGasUsed, err := cm.executeOperations(contract, operations, invocation, execution)
			gasUsed += opGasUsed
			if err != nil {
				return nil, gasUsed, err
			}
			return result, gasUsed, nil
		}

		return nil, gasUsed, nil

	case OpCall:
		// CALL operation: call another method in this contract
		if len(op.Args) < 1 {
			return nil, gasUsed, fmt.Errorf("CALL requires method name argument")
		}

		methodName, ok := op.Args[0].(string)
		if !ok {
			return nil, gasUsed, fmt.Errorf("method name must be a string")
		}

		// Find method in contract
		found := false
		methodOperations := make([]*ContractOperation, 0)

		// Look for method in contract code
		for i, contractOp := range contract.Code {
			if contractOp.OpCode == "METHOD" && len(contractOp.Args) >= 1 {
				if name, ok := contractOp.Args[0].(string); ok && name == methodName {
					found = true

					// Add operations for this method until next METHOD or end
					for j := i + 1; j < len(contract.Code); j++ {
						if contract.Code[j].OpCode == "METHOD" {
							break
						}
						methodOperations = append(methodOperations, contract.Code[j])
					}
					break
				}
			}
		}

		if !found {
			return nil, gasUsed, fmt.Errorf("method not found: %s", methodName)
		}

		// Prepare arguments for the call
		callArgs := make([]interface{}, 0)
		if len(op.Args) > 1 {
			callArgs = op.Args[1:]
		}

		// Execute method with gas limit check
		remainingGas := invocation.GasLimit - gasUsed
		if remainingGas <= 0 {
			return nil, gasUsed, fmt.Errorf("out of gas")
		}

		// Create a nested invocation
		nestedInvocation := &ContractInvocation{
			ContractAddress: contract.Address,
			Method:          methodName,
			Args:            callArgs,
			Caller:          invocation.Caller,
			GasLimit:        remainingGas,
			GasPrice:        invocation.GasPrice,
			Timestamp:       time.Now().UTC().Format(time.RFC3339),
		}

		// Execute operations
		result, opGasUsed, err := cm.executeOperations(contract, methodOperations, nestedInvocation, execution)
		gasUsed += opGasUsed + GasCostCall

		if err != nil {
			return nil, gasUsed, fmt.Errorf("method call to %s failed: %v", methodName, err)
		}

		return result, gasUsed, nil

	case OpNativeCall:
		// NATIVE_CALL operation: call native system functions
		if len(op.Args) < 1 {
			return nil, gasUsed, fmt.Errorf("NATIVE_CALL requires function name argument")
		}

		funcName, ok := op.Args[0].(string)
		if !ok {
			return nil, gasUsed, fmt.Errorf("function name must be a string")
		}

		// Execute native function based on name
		switch funcName {
		case "timestamp":
			// Get current timestamp
			return time.Now().UTC().Format(time.RFC3339), gasUsed + GasCostNativeCall, nil

		case "random":
			// Generate a pseudo-random number (not secure for cryptographic purposes)
			if len(op.Args) < 2 {
				return nil, gasUsed, fmt.Errorf("random requires max argument")
			}

			maxVal, ok := getNumericValue(op.Args[1])
			if !ok {
				return nil, gasUsed, fmt.Errorf("max value must be numeric")
			}

			if maxVal <= 0 {
				return nil, gasUsed, fmt.Errorf("max value must be positive")
			}

			// Use a combination of current time, invocation, and contract address as seed
			seed := time.Now().UnixNano() ^ int64(contract.Address[0])
			r := utils.NewSeededRand(seed)
			randomValue := r.Float64() * maxVal

			return randomValue, gasUsed + GasCostNativeCall, nil

		case "sha256":
			// Calculate SHA256 hash
			if len(op.Args) < 2 {
				return nil, gasUsed, fmt.Errorf("sha256 requires data argument")
			}

			var data []byte
			if str, ok := op.Args[1].(string); ok {
				data = []byte(str)
			} else {
				// Convert to JSON string if not a string
				jsonData, err := json.Marshal(op.Args[1])
				if err != nil {
					return nil, gasUsed, fmt.Errorf("failed to serialize data: %v", err)
				}
				data = jsonData
			}

			hash := sha256.Sum256(data)
			return hex.EncodeToString(hash[:]), gasUsed + GasCostNativeCall, nil

		case "caller":
			// Get caller address
			return invocation.Caller, gasUsed + GasCostNativeCall, nil

		case "contract_address":
			// Get contract address
			return contract.Address, gasUsed + GasCostNativeCall, nil

		case "balance":
			// Get account balance
			address := contract.Address
			if len(op.Args) > 1 {
				if addrStr, ok := op.Args[1].(string); ok {
					address = addrStr
				}
			}

			balance := cm.currencyMgr.GetBalance(address)
			return balance.String(), gasUsed + GasCostNativeCall, nil

		default:
			return nil, gasUsed, fmt.Errorf("unknown native function: %s", funcName)
		}

	case OpReturn:
		// RETURN operation: return a value from execution
		if len(op.Args) < 1 {
			return nil, gasUsed, nil
		}
		return op.Args[0], gasUsed, nil

	default:
		return nil, gasUsed, fmt.Errorf("unknown operation: %s", op.OpCode)
	}
}

// Helper function to get numeric value from various types
func getNumericValue(val any) (float64, bool) {
	switch v := val.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case string:
		// Try to parse as number
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

// AddEventListener adds a listener for a specific event
func (cm *ContractManager) AddEventListener(eventName string, listener EventListener) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if _, exists := cm.eventListeners[eventName]; !exists {
		cm.eventListeners[eventName] = make([]EventListener, 0)
	}

	cm.eventListeners[eventName] = append(cm.eventListeners[eventName], listener)
	utils.LogInfo("Event listener added for %s", eventName)
}

// notifyEventListeners notifies all listeners of an event
func (cm *ContractManager) notifyEventListeners(event *ContractEvent) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	// Notify generic listeners (for all events)
	if listeners, exists := cm.eventListeners["*"]; exists {
		for _, listener := range listeners {
			go listener(event)
		}
	}

	// Notify specific event listeners
	if listeners, exists := cm.eventListeners[event.EventName]; exists {
		for _, listener := range listeners {
			go listener(event)
		}
	}
}

// UpdateContractCode updates the code of an existing contract
func (cm *ContractManager) UpdateContractCode(contractAddress string, newCode []*ContractOperation, caller string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Get contract
	contract, exists := cm.contracts[contractAddress]
	if !exists {
		return fmt.Errorf("contract not found: %s", contractAddress)
	}

	// Check authorization
	if contract.Creator != caller {
		return fmt.Errorf("only contract creator can update code")
	}

	// Convert code to JSON to create hash
	codeBytes, err := json.Marshal(newCode)
	if err != nil {
		return fmt.Errorf("failed to marshal contract code: %v", err)
	}

	// Create code hash
	codeHash := sha256.Sum256(codeBytes)

	// Update contract code
	contract.Code = newCode
	contract.CodeHash = hex.EncodeToString(codeHash[:])
	contract.LastExecuted = time.Now().UTC().Format(time.RFC3339)

	utils.LogInfo("Contract code updated: %s by %s", contractAddress, caller)

	return nil
}

// RegisterContractTemplate registers a new contract template
func (cm *ContractManager) RegisterContractTemplate(template *ContractTemplate) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Store template in global state
	if cm.globalState["templates"] == nil {
		cm.globalState["templates"] = make(ContractState)
	}

	templates, ok := cm.globalState["templates"]
	if !ok {
		return fmt.Errorf("templates not found or invalid type")
	}
	templates[template.Name] = template

	utils.LogInfo("Contract template registered: %s", template.Name)

	return nil
}

// DeployContractFromTemplate creates a new contract from a template
func (cm *ContractManager) DeployContractFromTemplate(
	templateName string,
	creator string,
	initialState ContractState,
	value *currency.Balance,
) (*Contract, error) {
	cm.mutex.Lock()

	// Get template
	templatesState, exists := cm.globalState["templates"]
	if !exists {
		cm.mutex.Unlock()
		return nil, fmt.Errorf("no templates registered")
	}

	templates := templatesState
	templateObj, exists := templates[templateName]
	if !exists {
		cm.mutex.Unlock()
		return nil, fmt.Errorf("template not found: %s", templateName)
	}

	template, ok := templateObj.(*ContractTemplate)
	if !ok {
		cm.mutex.Unlock()
		return nil, fmt.Errorf("invalid template format")
	}

	cm.mutex.Unlock()

	// Prepare initial state by merging template state with provided state
	mergedState := make(ContractState)

	// Copy template state
	for k, v := range template.InitState {
		mergedState[k] = v
	}

	// Override with provided state
	if initialState != nil {
		for k, v := range initialState {
			mergedState[k] = v
		}
	}

	// Convert methods to code operations
	code := make([]*ContractOperation, 0)

	// Add methods to code
	for name, method := range template.Methods {
		// Add method declaration
		code = append(code, &ContractOperation{
			OpCode: "METHOD",
			Args:   []interface{}{name, method.Description, method.IsPublic},
		})

		// Add method operations
		code = append(code, method.Operations...)
	}

	// Create contract
	return cm.CreateContract(creator, code, mergedState, value)
}

// GetContractState returns the state of a contract
func (cm *ContractManager) GetContractState(address string) (ContractState, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	contract, exists := cm.contracts[address]
	if !exists {
		return nil, fmt.Errorf("contract not found: %s", address)
	}

	// Create a copy of the state to prevent external modification
	stateCopy := make(ContractState)
	contract.mutex.RLock()
	for k, v := range contract.State {
		stateCopy[k] = v
	}
	contract.mutex.RUnlock()

	return stateCopy, nil
}

// GetContractEvents retrieves events emitted by a contract
func (cm *ContractManager) GetContractEvents(address string, eventName string, limit int) ([]*ContractEvent, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	// In a real implementation, events would be stored in a database or other persistent storage
	// This is a simplified implementation that returns an empty slice
	// In a full implementation, we would query the database for events

	// For now, return an empty slice
	return make([]*ContractEvent, 0), nil
}

// ExecuteReadOnlyCall executes a contract method without changing state
func (cm *ContractManager) ExecuteReadOnlyCall(address string, method string, args []interface{}, caller string) (interface{}, error) {
	// Get contract
	contract, err := cm.GetContract(address)
	if err != nil {
		return nil, err
	}

	// Create invocation with high gas limit (since we're not actually using gas)
	invocation := &ContractInvocation{
		ContractAddress: address,
		Method:          method,
		Args:            args,
		Caller:          caller,
		GasLimit:        1000000,
		GasPrice:        "0",
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	}

	// Create execution context that won't be committed
	execution := &ContractExecution{
		Success:      false,
		GasUsed:      0,
		StateUpdates: make(map[string]interface{}),
		Events:       make([]*ContractEvent, 0),
		Logs:         make([]string, 0),
	}

	// Find method in contract
	found := false
	methodOperations := make([]*ContractOperation, 0)

	// Look for method in contract code
	for i, op := range contract.Code {
		if op.OpCode == "METHOD" && len(op.Args) >= 1 {
			if methodName, ok := op.Args[0].(string); ok && methodName == method {
				found = true

				// Find all operations for this method until next METHOD or end
				for j := i + 1; j < len(contract.Code); j++ {
					if contract.Code[j].OpCode == "METHOD" {
						break
					}
					methodOperations = append(methodOperations, contract.Code[j])
				}
				break
			}
		}
	}

	if !found {
		return nil, fmt.Errorf("method not found: %s", method)
	}

	// Make a copy of the contract state to avoid modifying it
	stateCopy := make(ContractState)
	contract.mutex.RLock()
	for k, v := range contract.State {
		stateCopy[k] = v
	}
	contract.mutex.RUnlock()

	// Execute method operations
	result, _, err := cm.executeOperations(contract, methodOperations, invocation, execution)
	if err != nil {
		return nil, err
	}

	return result, nil
}
