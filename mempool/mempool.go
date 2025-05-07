// mempool/mempool.go
package mempool

import (
	"sync"

	"tripcodechain_go/blockchain"
)

// Mempool manages pending items (transactions or critical processes)
type Mempool struct {
	items map[string]blockchain.MempoolItem
	mutex sync.RWMutex
}

// NewMempool creates a new mempool
func NewMempool() *Mempool {
	return &Mempool{
		items: make(map[string]blockchain.MempoolItem),
	}
}

// AddItem adds an item to the mempool
func (mp *Mempool) AddItem(item blockchain.MempoolItem) {
	mp.mutex.Lock()
	defer mp.mutex.Unlock()

	mp.items[item.GetID()] = item
}

// GetItem retrieves an item from the mempool
func (mp *Mempool) GetItem(id string) (blockchain.MempoolItem, bool) {
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()

	item, exists := mp.items[id]
	return item, exists
}

// RemoveItem removes an item from the mempool
func (mp *Mempool) RemoveItem(id string) {
	mp.mutex.Lock()
	defer mp.mutex.Unlock()

	delete(mp.items, id)
}

// RemoveProcessedItems removes items from the mempool after they've been processed
func (mp *Mempool) RemoveProcessedItems(items []blockchain.MempoolItem) {
	mp.mutex.Lock()
	defer mp.mutex.Unlock()

	for _, item := range items {
		delete(mp.items, item.GetID())
	}
}

// GetPendingItems returns a list of pending items that can be processed
func (mp *Mempool) GetPendingItems() []blockchain.MempoolItem {
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()

	// Limit the number of items returned to prevent blocks from being too large
	// This is a simple implementation - in a real world scenario, you'd want to
	// prioritize items based on gas price, timestamp, etc.
	maxItems := 100
	result := make([]blockchain.MempoolItem, 0, maxItems)

	// Get all items
	for _, item := range mp.items {
		result = append(result, item)
		if len(result) >= maxItems {
			break
		}
	}

	return result
}

// GetAllItems returns all items in mempool (for API/debug purposes)
func (mp *Mempool) GetAllItems() []blockchain.MempoolItem {
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()

	result := make([]blockchain.MempoolItem, 0, len(mp.items))
	for _, item := range mp.items {
		result = append(result, item)
	}

	return result
}

// GetSize returns the number of items in the mempool
func (mp *Mempool) GetSize() int {
	mp.mutex.RLock()
	defer mp.mutex.RUnlock()

	return len(mp.items)
}

// Clear removes all items from the mempool
func (mp *Mempool) Clear() {
	mp.mutex.Lock()
	defer mp.mutex.Unlock()

	mp.items = make(map[string]blockchain.MempoolItem)
}
