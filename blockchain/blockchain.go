package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"os"
	"sync"
	"time"

	"tripcodechain_go/currency"
	"tripcodechain_go/utils"
)

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
	Address     string
	Creator     string
	Code        []*ContractOperation
	State       ContractState
	Balance     *currency.Balance
	CreatedAt   time.Time
	LastUpdated time.Time
}

/**
 * GetCurrencyManager returns the CurrencyManager associated with this blockchain.
 *
 * Returns:
 *   - *currency.CurrencyManager: The CurrencyManager instance
 */
func (bc *Blockchain) GetCurrencyManager() *currency.CurrencyManager {
	return bc.currencyManager
}

// NewContractManager creates a new instance of ContractManager with database support
func NewContractManager(currencyManager *currency.CurrencyManager, db *BlockchainDB) *ContractManager {
	cm := &ContractManager{
		contracts:       make(map[string]*Contract),
		currencyManager: currencyManager,
		db:              db,
	}

	// Load existing contracts from database
	contracts, err := db.GetAllContracts()
	if err != nil {
		utils.LogError("Failed to load contracts from database: %v", err)
	} else {
		// Add contracts to in-memory cache
		for _, contract := range contracts {
			cm.contracts[contract.Address] = contract
		}
		utils.LogInfo("Loaded %d contracts from database", len(contracts))
	}

	return cm
}

// Initialize system with data directory
func InitializeBlockchain(dataDir string, blockType BlockType) (*Blockchain, *ContractManager, *currency.CurrencyManager, error) {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create data directory: %v", err)
	}

	// Initialize blockchain
	blockchain, err := NewBlockchain(blockType, dataDir)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to initialize blockchain: %v", err)
	}

	// Initialize currency manager
	currencyManager := currency.NewCurrencyManager()

	// Initialize native token
	currencyManager = InitNativeToken(blockchain, "TCC", 100000000) // 100M initial supply

	// Initialize contract manager
	contractManager := NewContractManager(currencyManager, blockchain.db)

	return blockchain, contractManager, currencyManager, nil
}

/**
 * SetConsensus sets the consensus mechanism for this blockchain.
 *
 * Parameters:
 *   - consensus: The consensus implementation to use
 */
func (bc *Blockchain) SetConsensus(consensus any) {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()

	bc.consensus = consensus
	utils.LogInfo("Consenso establecido para la cadena de bloques tipo: %s", bc.blockType)

	// Registrar evento de seguridad
	utils.LogSecurityEvent("consensus_set", map[string]interface{}{
		"blockchain_type": bc.blockType,
		"consensus_type":  fmt.Sprintf("%T", consensus),
		"blocks_count":    len(bc.blocks),
	})
}

/**
 * GetConsensus returns the consensus mechanism for this blockchain.
 *
 * Returns:
 *   - interface{}: The consensus implementation being used
 */
func (bc *Blockchain) GetConsensus() any {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	return bc.consensus
}

/**
 * NewSystemContract creates a new system contract.
 *
 * Parameters:
 *   - name: Name of the contract
 *
 * Returns:
 *   - A pointer to the newly created system contract
 */
func NewSystemContract(name string) *SystemContract {
	// Generate a unique address based on name and timestamp
	timestamp := time.Now().UnixNano()
	addressInput := name + string(rune(timestamp))
	hash := sha256.Sum256([]byte(addressInput))
	address := hex.EncodeToString(hash[:])

	return &SystemContract{
		Name:      name,
		Address:   address,
		State:     make(map[string]string),
		CreatedAt: time.Now(),
	}
}

func InitNativeToken(chain *Blockchain, symbol string, initialSupply int) *currency.CurrencyManager {
	// Create a new CurrencyManager using the exported constructor
	cm := currency.NewCurrencyManager()

	// Add defensive error checking
	if cm == nil {
		utils.LogError("Failed to create currency manager")
		return nil
	}

	// Check if Genesis address exists, create if needed
	if !cm.AccountExists(currency.GenesisAddress) {
		utils.LogInfo("Creating Genesis account: %s", currency.GenesisAddress)
		cm.CreateAccount(currency.GenesisAddress)
	}

	// Convert the initial supply to base units
	supplyInQuark := new(big.Int).Mul(
		big.NewInt(int64(initialSupply)),
		big.NewInt(currency.TripCoin), // TripCoin = 1e18
	)

	// Get current total supply
	currentSupply := cm.GetTotalSupply()

	// Only burn if there's something to burn
	if currentSupply != nil && currentSupply.Cmp(big.NewInt(0)) > 0 {
		// Try to burn existing tokens from genesis account
		if err := cm.BurnTokens(currency.GenesisAddress, currentSupply); err != nil {
			utils.LogError("Error burning tokens: %v", err)
			// Continue with existing supply - don't fail the entire process
		}
	}

	// Now mint the new amount to genesis
	if err := cm.MintTokens(currency.GenesisAddress, currency.NewBalanceFromBigInt(supplyInQuark)); err != nil {
		utils.LogError("Error minting tokens: %v", err)
		// Still return the currency manager, even if minting failed
	}

	// Calculate reserved amount (5%)
	reservedPercentage := 0.05
	reservedAmount := new(big.Int).Div(
		new(big.Int).Mul(supplyInQuark, big.NewInt(int64(reservedPercentage*100))),
		big.NewInt(100),
	)

	// Create a special account for reserved funds if needed
	reservedAccount := "RESERVED_FUNDS_ACCOUNT"
	if !cm.AccountExists(reservedAccount) {
		cm.CreateAccount(reservedAccount)
	}

	// Transfer reserved amount from genesis to reserved account
	if err := cm.TransferFunds(
		currency.GenesisAddress,
		reservedAccount,
		currency.NewBalanceFromBigInt(reservedAmount),
	); err != nil {
		utils.LogError("Error transferring reserved funds: %v", err)
	}

	utils.LogInfo("Native token %s initialized:", symbol)
	utils.LogInfo("- Total supply: %s", cm.GetTotalSupply().TripCoinString())
	utils.LogInfo("- Genesis account balance: %s", cm.GetBalance(currency.GenesisAddress).TripCoinString())
	utils.LogInfo("- Reserved funds: %s", cm.GetBalance(reservedAccount).TripCoinString())

	// Set the currency manager in the blockchain
	chain.currencyManager = cm

	return cm
}

// Fix for CreateContract method to avoid nil pointer exceptions
func (cm *ContractManager) CreateContract(creator string, code []*ContractOperation, initialState ContractState, initialBalance *currency.Balance) (*Contract, error) {
	// Guard against nil currency manager
	if cm.currencyManager == nil {
		return nil, errors.New("currency manager is not initialized")
	}

	// Guard against nil initialBalance
	if initialBalance == nil {
		initialBalance = currency.NewBalance(0)
	}

	// Check if creator has sufficient funds (logic similar to original implementation)
	if initialBalance.Cmp(big.NewInt(0)) > 0 {
		creatorBalance := cm.currencyManager.GetBalance(creator)
		if creatorBalance == nil {
			return nil, fmt.Errorf("creator account %s not found", creator)
		}

		if creatorBalance.Cmp(initialBalance.Int) < 0 {
			return nil, errors.New("insufficient funds to create contract")
		}

		// Transfer funds to the contract
		err := cm.currencyManager.TransferFunds(creator, "contract_creation", initialBalance)
		if err != nil {
			return nil, err
		}
	}

	// Generate contract address
	contractAddress := generateContractAddress(creator, code, time.Now().UnixNano())

	// Create the contract
	contract := &Contract{
		Address:     contractAddress,
		Creator:     creator,
		Code:        code,
		State:       initialState,
		Balance:     initialBalance,
		CreatedAt:   time.Now(),
		LastUpdated: time.Now(),
	}

	// Save contract to database first to ensure persistence
	if cm.db == nil {
		return nil, errors.New("database connection is not initialized")
	}

	if err := cm.db.SaveContract(contract); err != nil {
		return nil, fmt.Errorf("failed to save contract to database: %v", err)
	}

	// Register contract in memory cache
	cm.contracts[contractAddress] = contract

	utils.LogInfo("Contract created at address %s by %s", contractAddress, creator)
	return contract, nil
}

// generateContractAddress genera una dirección única para un contrato
func generateContractAddress(creator string, _ []*ContractOperation, timestamp int64) string {
	// En una implementación real, esto usaría hash criptográficos
	return fmt.Sprintf("contract_%s_%d", creator[:8], timestamp)
}

// OpBalanceOf crea una operación para consultar el balance de una dirección
func OpBalanceOf(address any) string {
	return fmt.Sprintf("BALANCE_OF(%v)", address)
}

// Añadir métodos a la estructura Blockchain para soportar contratos

// RegisterSystemContract registers a contract as a system contract
func (bc *Blockchain) RegisterSystemContract(name string, address string) {
	utils.LogInfo("Registering system contract: %s at address %s", name, address)

	// Save contract registration to database
	if err := bc.db.RegisterSystemContract(name, address); err != nil {
		utils.LogError("Failed to register system contract in database: %v", err)
		return
	}

	// Add a block to the chain to record this event (optional)
	newBlock := bc.CreateBlock()
	newBlock.Data = fmt.Sprintf("RegisterSystemContract:%s:%s", name, address)
	newBlock.ForgeBlock("system_registration")
	bc.AddBlock(newBlock)
}

// GetSystemContract retrieves a system contract by name
func (bc *Blockchain) GetSystemContract(name string) (string, error) {
	address, err := bc.db.GetSystemContractAddress(name)
	if err != nil {
		return "", err
	}
	return address, nil
}

// Fix for the DeploySystemContracts function to properly handle errors
func DeploySystemContracts(chain *Blockchain, contractManager *ContractManager) {
	// Check if required components are initialized
	if chain == nil {
		utils.LogError("Blockchain is nil in DeploySystemContracts")
		return
	}

	if contractManager == nil {
		utils.LogError("ContractManager is nil in DeploySystemContracts")
		return
	}

	// Check if system contracts are already deployed
	_, err := chain.GetSystemContract("governance")
	if err == nil {
		utils.LogInfo("System contracts already deployed, skipping deployment")
		return
	}

	// Make sure the currency manager is set in both blockchain and contract manager
	currencyManager := chain.GetCurrencyManager()
	if currencyManager == nil {
		utils.LogError("CurrencyManager is nil in DeploySystemContracts")
		return
	}

	// Ensure contract manager has the same currency manager
	contractManager.currencyManager = currencyManager

	// 1. Governance Contract
	deployGovernanceContract(contractManager, chain)

	// 2. Validator Registry
	deployValidatorRegistry(contractManager, chain)

	// 3. Rewards System - this was failing
	deployRewardsSystem(contractManager, chain)

	utils.LogInfo("Base contract system deployed")
}

func deployGovernanceContract(cm *ContractManager, chain *Blockchain) {
	// Código del contrato de gobernanza
	governanceCode := []*ContractOperation{
		{
			OpCode: "METHOD",
			Args:   []interface{}{"propose", "Crear nueva propuesta", true},
		},
		{
			OpCode: OpRequire,
			Args:   []interface{}{"caller == creator", OpCompare, OpCaller, "$creator"},
		},
		{
			OpCode: OpStore,
			Args:   []interface{}{"proposals.$id", 0},
		},
		{
			OpCode: OpEmitEvent,
			Args:   []interface{}{"ProposalCreated", map[string]interface{}{"id": 0}},
		},
	}

	// Estado inicial del contrato
	initialState := ContractState{
		"voting_delay":      "172800", // 2 días en bloques
		"voting_period":     "259200", // 3 días en bloques
		"proposal_count":    "0",
		"quorum_percentage": "4", // 4% del total de TCC
	}

	// Desplegar contrato
	contract, err := cm.CreateContract(
		"GENESIS_ACCOUNT",
		governanceCode,
		initialState,
		currency.NewBalance(0),
	)

	if err != nil {
		utils.LogError("Error desplegando contrato de gobernanza: %v", err)
	}

	chain.RegisterSystemContract("governance", contract.Address)
	utils.LogInfo("Contrato de Gobernanza desplegado: %s", contract.Address)
}

func deployValidatorRegistry(cm *ContractManager, chain *Blockchain) {
	// Código del registro de validadores
	validatorCode := []*ContractOperation{
		{
			OpCode: "METHOD",
			Args:   []interface{}{"register", "Registrar nuevo validador", true},
		},
		{
			OpCode: OpRequire,
			Args:   []interface{}{"balance >= 100 TCC", OpCompare, OpBalanceOf(OpCaller), "100000000000000000000"},
		},
		{
			OpCode: OpStore,
			Args: []interface{}{"validators.$caller", map[string]interface{}{
				"stake":      "ARGS[0]",
				"status":     "active",
				"registered": OpTimestamp,
			}},
		},
		{
			OpCode: OpEmitEvent,
			Args:   []interface{}{"ValidatorRegistered", map[string]interface{}{"address": OpCaller}},
		},
	}

	contract, err := cm.CreateContract(
		"GENESIS_ACCOUNT",
		validatorCode,
		ContractState{"min_stake": "100000000000000000000"}, // 100 TCC en wei
		currency.NewBalance(0),
	)

	if err != nil {
		utils.LogError("Error desplegando registro de validadores: %v", err)
	}

	chain.RegisterSystemContract("validators", contract.Address)
	utils.LogInfo("Registro de Validadores desplegado: %s", contract.Address)
}

// Fix for the deployRewardsSystem function
func deployRewardsSystem(cm *ContractManager, chain *Blockchain) {
	// Check if currency manager is properly initialized
	if cm.currencyManager == nil {
		utils.LogError("Currency manager is nil in deployRewardsSystem")
		return
	}

	// Código del sistema de recompensas
	rewardsCode := []*ContractOperation{
		{
			OpCode: "METHOD",
			Args:   []any{"distribute", "Distribuir recompensas", true},
		},
		{
			OpCode: OpNativeCall,
			Args:   []any{"get_block_producer"},
		},
		{
			OpCode: OpTransfer,
			Args:   []any{OpResult, "2000000000000000000"}, // 2 TCC
		},
		{
			OpCode: OpEmitEvent,
			Args: []any{"RewardsDistributed", map[string]any{
				"block":     OpBlockNumber,
				"validator": OpResult,
				"amount":    "2000000000000000000",
			}},
		},
	}

	// Create contract with zero initial balance first to avoid potential nil issues
	contract, err := cm.CreateContract(
		"GENESIS_ACCOUNT",
		rewardsCode,
		ContractState{"reward_per_block": "2000000000000000000"},
		currency.NewBalance(0), // Use zero balance initially
	)

	if err != nil {
		utils.LogError("Error desplegando sistema de recompensas: %v", err)
		return
	}

	// Now try to fund the contract if everything is properly set up
	if cm.currencyManager != nil {
		initialBalance, _ := new(big.Int).SetString("100000000000000000000", 10) // 100 TCC
		err = cm.currencyManager.TransferFunds(
			"GENESIS_ACCOUNT",
			contract.Address,
			currency.NewBalanceFromBigInt(initialBalance),
		)

		if err != nil {
			utils.LogError("Failed to fund rewards contract: %v", err)
		} else {
			utils.LogInfo("Funded rewards contract with 100 TCC")
		}
	}

	chain.RegisterSystemContract("rewards", contract.Address)
	utils.LogInfo("Sistema de Recompensas desplegado: %s", contract.Address)
}

// NewBlockchain initializes a new blockchain with a genesis block.
// Parameters:
//   - blockType: The type of blocks that will be added to this chain
//   - dataDir: Directory where blockchain data will be stored
//
// Returns:
//   - A pointer to the newly created blockchain
func NewBlockchain(blockType BlockType, dataDir string) (*Blockchain, error) {
	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %v", err)
	}

	// Initialize blockchain DB
	db, err := NewBlockchainDB(dataDir)
	if err != nil {
		return nil, err
	}

	blockchain := &Blockchain{
		blocks:     make([]*Block, 0),
		blockType:  blockType,
		difficulty: 2, // Initial difficulty for mining
		db:         db,
	}

	// Check if blockchain already exists
	height, err := db.GetBlockchainHeight()
	if err != nil {
		utils.LogError("Error getting blockchain height: %v", err)
	}

	if height > 0 {
		// Blockchain exists, load blocks from database
		utils.LogInfo("Loading existing blockchain with height %d", height)
		blocks, err := db.GetAllBlocks()
		if err != nil {
			return nil, fmt.Errorf("failed to load blockchain: %v", err)
		}
		blockchain.blocks = blocks
	} else {
		// Create genesis block
		utils.LogInfo("Creating new blockchain with genesis block")
		genesisBlock := NewBlock(0, "0", blockType)
		genesisBlock.ForgeBlock("genesis")
		blockchain.blocks = append(blockchain.blocks, genesisBlock)

		// Save genesis block to database
		if err := db.SaveBlock(genesisBlock); err != nil {
			return nil, fmt.Errorf("failed to save genesis block: %v", err)
		}
	}

	return blockchain, nil
}

// CreateBlock generates a new block based on the latest block in the chain.
// Returns:
//   - A pointer to the newly created block
func (bc *Blockchain) CreateBlock() *Block {
	bc.mutex.RLock()
	lastBlock := bc.blocks[len(bc.blocks)-1]
	bc.mutex.RUnlock()

	newBlock := NewBlock(lastBlock.Index+1, lastBlock.Hash, bc.blockType)
	return newBlock
}

// AddBlock adds a validated block to the blockchain.
// Parameters:
//   - block: The block to add to the chain
//
// Returns:
//   - bool: True if the block was added successfully, false otherwise
func (bc *Blockchain) AddBlock(block *Block) bool {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()

	// Validate the block before adding it
	if !bc.isValidBlock(block) {
		utils.LogError("Attempted to add invalid block %d", block.Index)
		return false
	}

	// Save to database first to ensure persistence
	if err := bc.db.SaveBlock(block); err != nil {
		utils.LogError("Failed to save block %d to database: %v", block.Index, err)
		return false
	}

	// Add to in-memory cache
	bc.blocks = append(bc.blocks, block)

	utils.LogInfo("Block %d added to blockchain with hash %s", block.Index, block.Hash)
	return true
}

/**
 * isValidBlock checks if a block is valid in the context of this blockchain.
 * It verifies index continuity, previous hash, and hash integrity.
 *
 * Parameters:
 *   - block: The block to validate
 *
 * Returns:
 *   - bool: True if the block is valid, false otherwise
 */
func (bc *Blockchain) isValidBlock(block *Block) bool {
	lastBlock := bc.blocks[len(bc.blocks)-1]

	// Check index continuity
	if block.Index != lastBlock.Index+1 {
		return false
	}

	// Ensure correct reference to previous block
	if block.PreviousHash != lastBlock.Hash {
		return false
	}

	// Recalculate hash to confirm integrity
	if block.Hash != block.CalculateHash() {
		return false
	}

	return true
}

/**
 * GetLength returns the number of blocks in the blockchain.
 *
 * Returns:
 *   - int: Length of the blockchain
 */
func (bc *Blockchain) GetLength() int {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	return len(bc.blocks)
}

/**
 * GetBlockType returns the type of blocks accepted by this blockchain.
 *
 * Returns:
 *   - BlockType: The type of blocks used in this chain
 */
func (bc *Blockchain) GetBlockType() BlockType {
	return bc.blockType
}

/**
 * GetLastBlock returns the most recent block in the blockchain.
 *
 * Returns:
 *   - *Block: The latest block
 */
func (bc *Blockchain) GetLastBlock() *Block {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	return bc.blocks[len(bc.blocks)-1]
}

/**
 * GetDifficulty returns the current mining difficulty.
 *
 * Returns:
 *   - int: Current difficulty level (number of leading zeros required)
 */
func (bc *Blockchain) GetDifficulty() int {
	return bc.difficulty
}

/**
 * GetBlocks returns all blocks in the blockchain.
 *
 * Returns:
 *   - []*Block: All blocks in order
 */
func (bc *Blockchain) GetBlocks() []*Block {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	return bc.blocks
}

// ReplaceChain replaces the current chain with a new one if it's longer and valid.
// Parameters:
//   - newBlocks: Slice of blocks representing the candidate chain
//
// Returns:
//   - bool: True if the chain was replaced, false otherwise
func (bc *Blockchain) ReplaceChain(newBlocks []*Block) bool {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()

	// Only replace if the new chain is longer
	if len(newBlocks) <= len(bc.blocks) {
		return false
	}

	// Validate the entire new chain
	if !bc.isValidChain(newBlocks) {
		return false
	}

	// Save all blocks to database
	for _, block := range newBlocks {
		if err := bc.db.SaveBlock(block); err != nil {
			utils.LogError("Failed to save block during chain replacement: %v", err)
			return false
		}
	}

	// Replace the in-memory chain
	bc.blocks = newBlocks
	utils.LogInfo("Blockchain replaced with new chain of length %d", len(newBlocks))
	return true
}

/**
 * isValidChain validates an entire chain of blocks starting from the genesis block.
 *
 * Parameters:
 *   - chain: Slice of blocks to validate
 *
 * Returns:
 *   - bool: True if the chain is valid, false otherwise
 */
func (bc *Blockchain) isValidChain(chain []*Block) bool {
	// Genesis block must match
	if chain[0].Hash != bc.blocks[0].Hash {
		return false
	}

	// Validate each block against its predecessor
	for i := 1; i < len(chain); i++ {
		if !bc.isBlockConsistentWithPrevious(chain[i], chain[i-1]) {
			return false
		}
	}

	return true
}

/**
 * isBlockConsistentWithPrevious ensures a block is consistent with its predecessor.
 *
 * Parameters:
 *   - block: The current block being checked
 *   - previousBlock: The block immediately before the current one
 *
 * Returns:
 *   - bool: True if the block is consistent, false otherwise
 */
func (bc *Blockchain) isBlockConsistentWithPrevious(block *Block, previousBlock *Block) bool {
	// Index should be exactly one greater
	if block.Index != previousBlock.Index+1 {
		return false
	}

	// Previous hash must match the predecessor's hash
	if block.PreviousHash != previousBlock.Hash {
		return false
	}

	// Block hash must match its calculated value
	if block.Hash != block.CalculateHash() {
		return false
	}

	return true
}
