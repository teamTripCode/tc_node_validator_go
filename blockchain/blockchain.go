package blockchain

import (
	"sync"
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
