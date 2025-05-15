package blockchain

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"

	"tripcodechain_go/currency"
	"tripcodechain_go/utils"
)

// Database keys prefixes for better organization
const (
	blockKeyPrefix       = "block_"       // Prefix for block storage
	blockHashKeyPrefix   = "blockhash_"   // Prefix for accessing blocks by hash
	blockIndexKeyPrefix  = "blockindex_"  // Prefix for accessing blocks by index
	blockHeightKey       = "height"       // Key for the current blockchain height
	contractKeyPrefix    = "contract_"    // Prefix for contract storage
	stateKeyPrefix       = "state_"       // Prefix for blockchain state variables
	systemContractPrefix = "syscontract_" // Prefix for system contract registration
)

// BlockchainDB handles the persistence layer of the blockchain
type BlockchainDB struct {
	db        *leveldb.DB
	batchLock sync.Mutex
	path      string
}

// NewBlockchainDB creates a new database connection
func NewBlockchainDB(dataDir string) (*BlockchainDB, error) {
	// Ensure data directory exists
	dbPath := filepath.Join(dataDir, "blockchain")

	// Configure database options
	options := &opt.Options{
		BlockCacheCapacity:  32 * 1024 * 1024, // 32MB block cache
		WriteBuffer:         16 * 1024 * 1024, // 16MB write buffer
		CompactionTableSize: 2 * 1024 * 1024,  // 2MB compaction table size
	}

	// Open LevelDB database
	db, err := leveldb.OpenFile(dbPath, options)
	if err != nil {
		return nil, fmt.Errorf("failed to open blockchain database: %v", err)
	}

	utils.LogInfo("Blockchain database initialized at: %s", dbPath)

	return &BlockchainDB{
		db:   db,
		path: dbPath,
	}, nil
}

// Close closes the database connection
func (bdb *BlockchainDB) Close() error {
	if bdb.db != nil {
		return bdb.db.Close()
	}
	return nil
}

// SaveBlock stores a block in the database
func (bdb *BlockchainDB) SaveBlock(block *Block) error {
	// Serialize the block to JSON
	blockData, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf("failed to marshal block: %v", err)
	}

	// Start a batch transaction
	batch := new(leveldb.Batch)

	// Store block by index
	indexKey := fmt.Sprintf("%s%d", blockIndexKeyPrefix, block.Index)
	batch.Put([]byte(indexKey), blockData)

	// Store block by hash
	hashKey := fmt.Sprintf("%s%s", blockHashKeyPrefix, block.Hash)
	batch.Put([]byte(hashKey), blockData)

	// Update current blockchain height if this is the latest block
	currentHeight, err := bdb.GetBlockchainHeight()
	if err != nil || block.Index > currentHeight {
		batch.Put([]byte(blockHeightKey), []byte(fmt.Sprintf("%d", block.Index)))
	}

	// Execute the batch
	bdb.batchLock.Lock()
	defer bdb.batchLock.Unlock()

	if err := bdb.db.Write(batch, nil); err != nil {
		return fmt.Errorf("failed to save block to database: %v", err)
	}

	utils.LogInfo("Block %d saved to database with hash %s", block.Index, block.Hash)
	return nil
}

// GetBlockByIndex retrieves a block by its index
func (bdb *BlockchainDB) GetBlockByIndex(index uint64) (*Block, error) {
	indexKey := fmt.Sprintf("%s%d", blockIndexKeyPrefix, index)

	data, err := bdb.db.Get([]byte(indexKey), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, fmt.Errorf("block with index %d not found", index)
		}
		return nil, fmt.Errorf("failed to retrieve block: %v", err)
	}

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %v", err)
	}

	return &block, nil
}

// GetBlockByHash retrieves a block by its hash
func (bdb *BlockchainDB) GetBlockByHash(hash string) (*Block, error) {
	hashKey := fmt.Sprintf("%s%s", blockHashKeyPrefix, hash)

	data, err := bdb.db.Get([]byte(hashKey), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, fmt.Errorf("block with hash %s not found", hash)
		}
		return nil, fmt.Errorf("failed to retrieve block: %v", err)
	}

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %v", err)
	}

	return &block, nil
}

// GetBlockchainHeight returns the current blockchain height
func (bdb *BlockchainDB) GetBlockchainHeight() (uint64, error) {
	data, err := bdb.db.Get([]byte(blockHeightKey), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return 0, nil // No blocks yet, height is 0
		}
		return 0, fmt.Errorf("failed to retrieve blockchain height: %v", err)
	}

	var height uint64
	_, err = fmt.Sscanf(string(data), "%d", &height)
	if err != nil {
		return 0, fmt.Errorf("failed to parse blockchain height: %v", err)
	}

	return height, nil
}

// GetAllBlocks retrieves all blocks in the blockchain
func (bdb *BlockchainDB) GetAllBlocks() ([]*Block, error) {
	blocks := make([]*Block, 0)

	height, err := bdb.GetBlockchainHeight()
	if err != nil {
		return nil, err
	}

	// Retrieve blocks from index 0 to height
	for i := uint64(0); i <= height; i++ {
		block, err := bdb.GetBlockByIndex(i)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}

	return blocks, nil
}

// SaveContract stores a contract in the database
func (bdb *BlockchainDB) SaveContract(contract *Contract) error {
	// Serialize the contract to JSON
	contractData, err := json.Marshal(contract)
	if err != nil {
		return fmt.Errorf("failed to marshal contract: %v", err)
	}

	// Generate key for contract storage
	contractKey := fmt.Sprintf("%s%s", contractKeyPrefix, contract.Address)

	// Save the contract
	if err := bdb.db.Put([]byte(contractKey), contractData, nil); err != nil {
		return fmt.Errorf("failed to save contract to database: %v", err)
	}

	utils.LogInfo("Contract saved to database with address %s", contract.Address)
	return nil
}

// GetContract retrieves a contract by its address
func (bdb *BlockchainDB) GetContract(address string) (*Contract, error) {
	contractKey := fmt.Sprintf("%s%s", contractKeyPrefix, address)

	data, err := bdb.db.Get([]byte(contractKey), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, fmt.Errorf("contract with address %s not found", address)
		}
		return nil, fmt.Errorf("failed to retrieve contract: %v", err)
	}

	var contract Contract
	if err := json.Unmarshal(data, &contract); err != nil {
		return nil, fmt.Errorf("failed to unmarshal contract: %v", err)
	}

	return &contract, nil
}

// GetAllContracts retrieves all contracts in the blockchain
func (bdb *BlockchainDB) GetAllContracts() ([]*Contract, error) {
	contracts := make([]*Contract, 0)

	// Create iterator for contract prefix
	iter := bdb.db.NewIterator(util.BytesPrefix([]byte(contractKeyPrefix)), nil)
	defer iter.Release()

	// Iterate through all contracts
	for iter.Next() {
		var contract Contract
		if err := json.Unmarshal(iter.Value(), &contract); err != nil {
			return nil, fmt.Errorf("failed to unmarshal contract: %v", err)
		}

		if contract.Balance == nil {
			contract.Balance = currency.NewBalance(0)
		}

		contracts = append(contracts, &contract)
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("error iterating contracts: %v", err)
	}

	return contracts, nil
}

// RegisterSystemContract saves a system contract mapping
func (bdb *BlockchainDB) RegisterSystemContract(name, address string) error {
	key := fmt.Sprintf("%s%s", systemContractPrefix, name)

	if err := bdb.db.Put([]byte(key), []byte(address), nil); err != nil {
		return fmt.Errorf("failed to register system contract: %v", err)
	}

	return nil
}

// GetSystemContractAddress returns the address of a system contract by its name
func (bdb *BlockchainDB) GetSystemContractAddress(name string) (string, error) {
	key := fmt.Sprintf("%s%s", systemContractPrefix, name)

	data, err := bdb.db.Get([]byte(key), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return "", fmt.Errorf("system contract %s not registered", name)
		}
		return "", fmt.Errorf("failed to retrieve system contract: %v", err)
	}

	return string(data), nil
}

// SaveState saves a key-value state in the blockchain
func (bdb *BlockchainDB) SaveState(key string, value any) error {
	// Serialize the value to JSON
	valueData, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal state value: %v", err)
	}

	stateKey := fmt.Sprintf("%s%s", stateKeyPrefix, key)

	if err := bdb.db.Put([]byte(stateKey), valueData, nil); err != nil {
		return fmt.Errorf("failed to save state: %v", err)
	}

	return nil
}

// GetState retrieves a state value by its key
func (bdb *BlockchainDB) GetState(key string) ([]byte, error) {
	stateKey := fmt.Sprintf("%s%s", stateKeyPrefix, key)

	data, err := bdb.db.Get([]byte(stateKey), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, nil // No value found
		}
		return nil, fmt.Errorf("failed to retrieve state: %v", err)
	}

	return data, nil
}

// DeleteState removes a state entry
func (bdb *BlockchainDB) DeleteState(key string) error {
	stateKey := fmt.Sprintf("%s%s", stateKeyPrefix, key)

	if err := bdb.db.Delete([]byte(stateKey), nil); err != nil {
		return fmt.Errorf("failed to delete state: %v", err)
	}

	return nil
}
