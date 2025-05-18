package blockchain

import (
	"sync"
	"time"
	"tripcodechain_go/currency"
)

// MempoolItem interface for items that can be stored in the mempool
type MempoolItem interface {
	GetID() string
	GetTimestamp() string
}

/**
 * Blockchain represents the chain of blocks.
 * It includes metadata like block type and mining difficulty,
 * as well as concurrency protection using a mutex.
 */
type Blockchain struct {
	blocks          []*Block     // Ordered list of blocks in the chain
	blockType       BlockType    // Type of blocks this blockchain accepts
	difficulty      int          // Mining difficulty (number of leading zeros required)
	mutex           sync.RWMutex // Mutex to ensure thread-safe access to the blockchain
	consensus       any          // Interface to the consensus mechanism
	db              *BlockchainDB
	currencyManager *currency.CurrencyManager // Reference to the CurrencyManager
}

// Códigos de operación para el lenguaje del contrato
const (
	OpStore       = "STORE"
	OpRequire     = "REQUIRE"
	OpCompare     = "COMPARE"
	OpCaller      = "CALLER"
	OpArgs        = "ARGS"
	OpEmitEvent   = "EMIT_EVENT"
	OpTransfer    = "TRANSFER"
	OpNativeCall  = "NATIVE_CALL"
	OpResult      = "RESULT"
	OpTimestamp   = "TIMESTAMP"
	OpBlockNumber = "BLOCK_NUMBER"
)

/**
 * SystemContract represents a contract deployed on the blockchain.
 * These contracts handle system-level functionality like rewards,
 * validator management, and blockchain governance.
 */
type SystemContract struct {
	Name      string            // Name of the contract
	Address   string            // Unique identifier for the contract
	State     map[string]string // Contract's state variables
	CreatedAt time.Time         // Time when contract was deployed
}

// Balance representa un saldo de moneda en la blockchain
// ContractManager gestiona la creación y ejecución de contratos
type ContractManager struct {
	contracts       map[string]*Contract
	currencyManager *currency.CurrencyManager
	db              *BlockchainDB
	mutex           sync.Mutex
}

// ContractOperation representa una operación en el código del contrato
type ContractOperation struct {
	OpCode string
	Args   []any
}

// ContractState representa el estado de un contrato
type ContractState map[string]string

// Contract representa un contrato inteligente en la blockchain
type Contract struct {
	Address     string               `json:"address"`
	Creator     string               `json:"creator"`
	Code        []*ContractOperation `json:"code"`
	State       ContractState        `json:"state"`
	Balance     *currency.Balance    `json:"balance"`
	CreatedAt   time.Time            `json:"created_at"`
	LastUpdated time.Time            `json:"last_updated"`
}
