package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
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
	blocks     []*Block     // Ordered list of blocks in the chain
	blockType  BlockType    // Type of blocks this blockchain accepts
	difficulty int          // Mining difficulty (number of leading zeros required)
	mutex      sync.RWMutex // Mutex to ensure thread-safe access to the blockchain
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
}

// ContractOperation representa una operación en el código del contrato
type ContractOperation struct {
	OpCode string
	Args   []interface{}
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

// NewContractManager crea una nueva instancia de ContractManager
func NewContractManager(currencyManager *currency.CurrencyManager) *ContractManager {
	return &ContractManager{
		contracts:       make(map[string]*Contract),
		currencyManager: currencyManager,
	}
}

// CreateContract crea un nuevo contrato en la blockchain
// CreateContract crea un nuevo contrato en la blockchain
func (cm *ContractManager) CreateContract(creator string, code []*ContractOperation, initialState ContractState, initialBalance *currency.Balance) (*Contract, error) {
	// Verificar que el creador tiene suficientes fondos
	if initialBalance.Cmp(big.NewInt(0)) > 0 {
		// Determine which version of GetBalance to use based on how it's implemented in CurrencyManager

		// Option 1: If GetBalance returns only a balance (no error)
		creatorBalance := cm.currencyManager.GetBalance(creator)

		// Compare creator's balance with the initial balance required
		if creatorBalance.Cmp(initialBalance.Int) < 0 {
			return nil, errors.New("fondos insuficientes para crear contrato")
		}

		// Transferir fondos al contrato
		err := cm.currencyManager.TransferFunds(creator, "contract_creation", initialBalance)
		if err != nil {
			return nil, err
		}
	}

	// Generar dirección del contrato
	contractAddress := generateContractAddress(creator, code, time.Now().UnixNano())

	// Crear el contrato
	contract := &Contract{
		Address:     contractAddress,
		Creator:     creator,
		Code:        code,
		State:       initialState,
		Balance:     initialBalance,
		CreatedAt:   time.Now(),
		LastUpdated: time.Now(),
	}

	// Registrar el contrato
	cm.contracts[contractAddress] = contract

	return contract, nil
}

// GetContract devuelve un contrato por su dirección
func (cm *ContractManager) GetContract(address string) (*Contract, error) {
	contract, exists := cm.contracts[address]
	if !exists {
		return nil, errors.New("contrato no encontrado")
	}
	return contract, nil
}

// generateContractAddress genera una dirección única para un contrato
func generateContractAddress(creator string, code []*ContractOperation, timestamp int64) string {
	// En una implementación real, esto usaría hash criptográficos
	return fmt.Sprintf("contract_%s_%d", creator[:8], timestamp)
}

// OpBalanceOf crea una operación para consultar el balance de una dirección
func OpBalanceOf(address interface{}) string {
	return fmt.Sprintf("BALANCE_OF(%v)", address)
}

// Añadir métodos a la estructura Blockchain para soportar contratos

// GetCurrencyManager devuelve el gestor de moneda de la blockchain
func (bc *Blockchain) GetCurrencyManager() *currency.CurrencyManager {
	// En una implementación real, esto estaría almacenado como campo en la estructura Blockchain
	// Para este ejemplo, creamos uno nuevo
	return currency.NewCurrencyManager()
}

// RegisterSystemContract registra un contrato como contrato del sistema
func (bc *Blockchain) RegisterSystemContract(name string, address string) {
	// En una implementación real, esto almacenaría el contrato en una estructura de datos persistente
	utils.LogInfo("Registrando contrato del sistema: %s en dirección %s", name, address)

	// Añadir un bloque a la cadena para registrar este evento
	newBlock := bc.CreateBlock()
	newBlock.Data = fmt.Sprintf("RegisterSystemContract:%s:%s", name, address)
	newBlock.ForgeBlock("system_registration")
	bc.AddBlock(newBlock)
}

// DeploySystemContracts despliega todos los contratos del sistema
func DeploySystemContracts(chain *Blockchain) {
	// Obtener el CurrencyManager de la blockchain
	currencyManager := chain.GetCurrencyManager()

	// Crear ContractManager
	contractManager := NewContractManager(currencyManager)

	// 1. Contrato de Gobernanza
	deployGovernanceContract(contractManager, chain)

	// 2. Registro de Validadores
	deployValidatorRegistry(contractManager, chain)

	// 3. Sistema de Recompensas
	deployRewardsSystem(contractManager, chain)

	utils.LogInfo("Sistema de contratos base desplegado")
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

func deployRewardsSystem(cm *ContractManager, chain *Blockchain) {
	// Código del sistema de recompensas
	rewardsCode := []*ContractOperation{
		{
			OpCode: "METHOD",
			Args:   []interface{}{"distribute", "Distribuir recompensas", true},
		},
		{
			OpCode: OpNativeCall,
			Args:   []interface{}{"get_block_producer"},
		},
		{
			OpCode: OpTransfer,
			Args:   []interface{}{OpResult, "2000000000000000000"}, // 2 TCC
		},
		{
			OpCode: OpEmitEvent,
			Args: []interface{}{"RewardsDistributed", map[string]interface{}{
				"block":     OpBlockNumber,
				"validator": OpResult,
				"amount":    "2000000000000000000",
			}},
		},
	}

	// Crear el contrato con balance inicial
	initialBalance, _ := new(big.Int).SetString("100000000000000000000", 10) // 100 TCC
	contract, err := cm.CreateContract(
		"GENESIS_ACCOUNT",
		rewardsCode,
		ContractState{"reward_per_block": "2000000000000000000"},
		currency.NewBalanceFromBigInt(initialBalance),
	)

	if err != nil {
		utils.LogError("Error desplegando sistema de recompensas: %v", err)
	}

	chain.RegisterSystemContract("rewards", contract.Address)
	utils.LogInfo("Sistema de Recompensas desplegado: %s", contract.Address)
}

/**
 * NewBlockchain initializes a new blockchain with a genesis block.
 *
 * Parameters:
 *   - blockType: The type of blocks that will be added to this chain
 *
 * Returns:
 *   - A pointer to the newly created blockchain
 */
func NewBlockchain(blockType BlockType) *Blockchain {
	blockchain := &Blockchain{
		blocks:     make([]*Block, 0),
		blockType:  blockType,
		difficulty: 2, // Initial difficulty for mining
	}

	// Create genesis block
	genesisBlock := NewBlock(0, "0", blockType)
	genesisBlock.ForgeBlock("genesis")
	blockchain.blocks = append(blockchain.blocks, genesisBlock)

	return blockchain
}

/**
 * CreateBlock generates a new block based on the latest block in the chain.
 *
 * Returns:
 *   - A pointer to the newly created block
 */
func (bc *Blockchain) CreateBlock() *Block {
	bc.mutex.RLock()
	lastBlock := bc.blocks[len(bc.blocks)-1]
	bc.mutex.RUnlock()

	newBlock := NewBlock(lastBlock.Index+1, lastBlock.Hash, bc.blockType)
	return newBlock
}

/**
 * AddBlock adds a validated block to the blockchain.
 *
 * Parameters:
 *   - block: The block to add to the chain
 *
 * Returns:
 *   - bool: True if the block was added successfully, false otherwise
 */
func (bc *Blockchain) AddBlock(block *Block) bool {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()

	// Validate the block before adding it
	if !bc.isValidBlock(block) {
		return false
	}

	bc.blocks = append(bc.blocks, block)
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

/**
 * ReplaceChain replaces the current chain with a new one if it's longer and valid.
 *
 * Parameters:
 *   - newBlocks: Slice of blocks representing the candidate chain
 *
 * Returns:
 *   - bool: True if the chain was replaced, false otherwise
 */
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

	// Replace the chain
	bc.blocks = newBlocks
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
