package blockchain

// MempoolItem interface for items that can be stored in the mempool
type MempoolItem interface {
	GetID() string
	GetTimestamp() string
}
