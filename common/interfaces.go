package common

import (
	"math/big"
	"time"
)

// Balance representa un saldo de moneda en la blockchain
type Balance struct {
	*big.Int
}

// CurrencyManagerInterface define métodos que el CurrencyManager debe implementar
type CurrencyManagerInterface interface {
	CreateAccount(address string) any
	GetAccount(address string) any
	TransferFunds(from, to string, amount *Balance) error
	GetBalance(address string) *Balance
	MintTokens(to string, amount *Balance) error
	BurnTokens(from string, amount *Balance) error
	RewardBlockProducer(address string) error
	CalculateTransactionFee(gasUsed uint64, gasPrice *Balance) *Balance
	DistributeFees(fees *Balance, validator string) error
	GetTotalSupply() *Balance
	GetNetworkGasPrice() *Balance
	UpdateNetworkGasPrice(blockUtilization float64)
	StakeTokens(address string, amount *Balance) error
	UnstakeTokens(address string, amount *Balance) error
	GetStake(address string) *Balance
	ProcessBlockRewards(validator string, fees *Balance) error
}

// BlockchainInterface define métodos que el Blockchain debe implementar
type BlockchainInterface interface {
	CreateBlock() interface{}
	AddBlock(block interface{}) bool
	GetLength() int
	GetBlockType() interface{}
	GetLastBlock() interface{}
	GetDifficulty() int
	GetBlocks() []interface{}
	ReplaceChain(newBlocks []interface{}) bool
	SetConsensus(consensus interface{})
	GetConsensus() interface{}
	RegisterSystemContract(name string, address string)
}

// ContractManagerInterface define métodos que el ContractManager debe implementar
type ContractManagerInterface interface {
	CreateContract(creator string, code interface{}, initialState interface{}, initialBalance *Balance) (interface{}, error)
	GetContract(address string) (interface{}, error)
}

// SystemContractInterface define métodos que un SystemContract debe implementar
type SystemContractInterface interface {
	GetName() string
	GetAddress() string
	GetState() map[string]string
	GetCreatedAt() time.Time
}

// NewBalance crea una nueva instancia de Balance
func NewBalance(amount int64) *Balance {
	return &Balance{big.NewInt(amount)}
}

// NewBalanceFromString creates a new Balance from string
func NewBalanceFromString(amount string) (*Balance, error) {
	n := new(big.Int)
	_, success := n.SetString(amount, 10)
	if !success {
		return nil, nil
	}
	return &Balance{n}, nil
}

// NewBalanceFromBigInt creates a new Balance from a big.Int
func NewBalanceFromBigInt(amount *big.Int) *Balance {
	if amount == nil {
		return &Balance{big.NewInt(0)}
	}
	return &Balance{new(big.Int).Set(amount)}
}
