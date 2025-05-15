package currency

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sync"
	"time"

	"tripcodechain_go/utils"
)

// Defines the precision for TripCoin calculations
const (
	// TCCDecimals represents decimal precision for TripCoin (similar to Ether's 18 decimals)
	TCCDecimals    = 18
	DefaultSystem  = "TCC"
	GenesisAddress = "GENESIS_ACCOUNT"

	// Denominaciones en TCC
	Quark    = 1    // Unidad atómica base
	Proton   = 1e9  // 1 billón de Quark
	TripCoin = 1e18 // Unidad estándar (1 TCC)

	// Initial supply and parameters
	InitialSupply   = 1000000 * TripCoin // 1 million TCC initial supply
	BlockReward     = 2 * TripCoin       // 2 TCC per block reward
	MinimumStake    = 100 * TripCoin     // 100 TCC minimum stake for validators
	MinimumGasPrice = 1 * Proton         // 1 Proton minimum gas price
	DefaultGasPrice = 20 * Proton        // 20 Proton default gas price
	DefaultGasLimit = 21000              // Default gas limit for basic transactions

	// Contract execution costs
	ContractDeploymentBaseCost = 32000
	ContractCallBaseCost       = 5000
	OperationCost              = 100 // Cost per operation in a contract
	StorageCost                = 200 // Cost per storage update
)

// Balance represents a cryptocurrency amount
type Balance struct {
	*big.Int
}

// Account represents a TripCoin account with balance and other metadata
type Account struct {
	Address      string   `json:"address"`      // Account address (public key)
	Balance      *Balance `json:"balance"`      // Account balance in wei
	Nonce        uint64   `json:"nonce"`        // Transaction count originated from this account
	CodeHash     string   `json:"codeHash"`     // Hash of contract code (empty for regular accounts)
	StorageRoot  string   `json:"storageRoot"`  // Root hash of account storage trie
	IsContract   bool     `json:"isContract"`   // Whether this is a contract account
	CreatedAt    string   `json:"createdAt"`    // Account creation timestamp
	LastActivity string   `json:"lastActivity"` // Last activity timestamp
	Frozen       bool     `json:"frozen"`       // Whether the account is frozen
	Stake        *Balance `json:"stake"`        // Amount staked by this account (for validators)
}

// CurrencyManager manages TripCoin balances and accounts
type CurrencyManager struct {
	symbol           string
	genesisAllocated bool
	accounts         map[string]*Account // Map of account addresses to accounts
	totalSupply      *Balance            // Total supply of TripCoin
	mutex            sync.RWMutex        // Mutex for thread-safe operations
	reservedFunds    *Balance            // Funds reserved for system operations
	gasParameters    GasParameters       // Gas pricing parameters
}

// GasParameters stores gas-related parameters
type GasParameters struct {
	MinGasPrice       *Balance  // Minimum gas price accepted
	NetworkGasPrice   *Balance  // Current network gas price
	BaseFeeMultiplier float64   // Multiplier for base fee calculation
	LastUpdated       time.Time // Last time gas price was updated
}

// NewBalance creates a new Balance with the given int64 value
func NewBalance(amount int64) *Balance {
	return &Balance{big.NewInt(amount)}
}

// NewBalanceFromString creates a new Balance from string
func NewBalanceFromString(amount string) (*Balance, error) {
	n := new(big.Int)
	_, success := n.SetString(amount, 10)
	if !success {
		return nil, fmt.Errorf("invalid balance amount: %s", amount)
	}
	return &Balance{n}, nil
}

func NewBalanceFromBigInt(amount *big.Int) *Balance {
	if amount == nil {
		return &Balance{big.NewInt(0)}
	}
	return &Balance{new(big.Int).Set(amount)}
}

// String returns the string representation of the Balance
func (b *Balance) String() string {
	if b == nil || b.Int == nil {
		return "0"
	}
	return b.Int.String()
}

// TripCoinString returns the human-readable string representation in TripCoin
func (b *Balance) TripCoinString() string {
	if b == nil || b.Int == nil {
		return "0 TCC"
	}

	// Create a copy of the balance
	wei := new(big.Int).Set(b.Int)

	// Calculate TripCoin part and wei part
	tccUnit := new(big.Int).SetInt64(TripCoin)
	tripCoinPart := new(big.Int).Div(wei, tccUnit)
	weiPart := new(big.Int).Mod(wei, tccUnit)

	// Format with leading zeros for wei part if needed
	if weiPart.Cmp(big.NewInt(0)) == 0 {
		return fmt.Sprintf("%s TCC", tripCoinPart.String())
	}

	// Adjust wei part to have proper decimal places
	weiStr := weiPart.String()
	weiStrWithLeadingZeros := weiStr
	for len(weiStrWithLeadingZeros) < TCCDecimals {
		weiStrWithLeadingZeros = "0" + weiStrWithLeadingZeros
	}

	// Truncate unnecessary trailing zeros
	for weiStrWithLeadingZeros[len(weiStrWithLeadingZeros)-1] == '0' {
		weiStrWithLeadingZeros = weiStrWithLeadingZeros[:len(weiStrWithLeadingZeros)-1]
	}

	return fmt.Sprintf("%s.%s TCC", tripCoinPart, weiStrWithLeadingZeros)
}

// Add adds the other balance to this balance and returns the result
func (b *Balance) Add(other *Balance) *Balance {
	if other == nil || other.Int == nil {
		return b
	}
	result := new(big.Int).Add(b.Int, other.Int)
	return &Balance{result}
}

// Sub subtracts the other balance from this balance and returns the result
func (b *Balance) Sub(other *Balance) *Balance {
	if other == nil || other.Int == nil {
		return b
	}
	result := new(big.Int).Sub(b.Int, other.Int)
	return &Balance{result}
}

// Mul multiplies this balance by the other balance and returns the result
func (b *Balance) Mul(other *Balance) *Balance {
	if other == nil || other.Int == nil {
		return &Balance{big.NewInt(0)}
	}
	result := new(big.Int).Mul(b.Int, other.Int)
	return &Balance{result}
}

// Div divides this balance by the other balance and returns the result
func (b *Balance) Div(other *Balance) *Balance {
	if other == nil || other.Int == nil || other.Int.Cmp(big.NewInt(0)) == 0 {
		panic("Division by zero or nil balance")
	}
	result := new(big.Int).Div(b.Int, other.Int)
	return &Balance{result}
}

// FromTripCoin converts TripCoin float value to wei Balance
func FromTripCoin(tcc float64) *Balance {
	// Convert to Wei (TripCoin * 10^18)
	weiValue := new(big.Float).Mul(
		new(big.Float).SetFloat64(tcc),
		new(big.Float).SetFloat64(TripCoin),
	)

	// Convert big.Float to big.Int
	weiInt, _ := weiValue.Int(nil)
	return &Balance{weiInt}
}

// NewCurrencyManager creates a new CurrencyManager with initial supply
func NewCurrencyManager() *CurrencyManager {
	// Create initial supply
	initialSupplyFloat := new(big.Float).SetFloat64(InitialSupply)
	initialSupplyInt, _ := initialSupplyFloat.Int(nil)
	initialSupply := &Balance{initialSupplyInt}

	// Create reserved funds (5% of initial supply)
	reservedPercentage := 0.05
	reservedAmount := new(big.Float).Mul(
		new(big.Float).SetInt(initialSupply.Int),
		new(big.Float).SetFloat64(reservedPercentage),
	)
	reservedInt, _ := reservedAmount.Int(nil)

	manager := &CurrencyManager{
		accounts:      make(map[string]*Account),
		totalSupply:   initialSupply,
		reservedFunds: &Balance{reservedInt},
		gasParameters: GasParameters{
			MinGasPrice:       &Balance{big.NewInt(MinimumGasPrice)},
			NetworkGasPrice:   &Balance{big.NewInt(DefaultGasPrice)},
			BaseFeeMultiplier: 1.0,
			LastUpdated:       time.Now(),
		},
	}

	// Create genesis account with remaining initial supply
	remainingSupply := new(big.Int).Sub(initialSupply.Int, reservedInt)
	genesisAddress := "GENESIS_ACCOUNT"
	manager.accounts[genesisAddress] = &Account{
		Address:      genesisAddress,
		Balance:      &Balance{remainingSupply},
		Nonce:        0,
		IsContract:   false,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		LastActivity: time.Now().UTC().Format(time.RFC3339),
		Frozen:       false,
	}

	utils.LogInfo("CurrencyManager initialized with total supply of %s", initialSupply.TripCoinString())
	utils.LogInfo("Reserved funds: %s", manager.reservedFunds.TripCoinString())

	return manager
}

// Managment Accounts

// CreateAccount creates a new account with the given address
func (cm *CurrencyManager) CreateAccount(address string) *Account {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Check if account already exists
	if _, exists := cm.accounts[address]; exists {
		return cm.accounts[address]
	}

	// Create new account with zero balance
	account := &Account{
		Address:      address,
		Balance:      &Balance{big.NewInt(0)},
		Nonce:        0,
		IsContract:   false,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		LastActivity: time.Now().UTC().Format(time.RFC3339),
		Frozen:       false,
		Stake:        &Balance{big.NewInt(0)},
	}

	cm.accounts[address] = account
	utils.LogInfo("New account created: %s", address)

	return account
}

// GetAccount returns an account by address or creates a new one if it doesn't exist
func (cm *CurrencyManager) GetAccount(address string) *Account {
	cm.mutex.RLock()
	account, exists := cm.accounts[address]
	cm.mutex.RUnlock()

	if !exists {
		return cm.CreateAccount(address)
	}

	return account
}

func (cm *CurrencyManager) AccountExists(address string) bool {
	_, exists := cm.accounts[address]
	return exists
}

// TransferFunds transfers funds from one account to another
func (cm *CurrencyManager) TransferFunds(from, to string, amount *Balance) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Validate amount
	if amount.Int.Cmp(big.NewInt(0)) <= 0 {
		return fmt.Errorf("invalid amount: must be positive")
	}

	// Get accounts
	fromAccount, exists := cm.accounts[from]
	if !exists {
		return fmt.Errorf("source account does not exist: %s", from)
	}

	// Check if account is frozen
	if fromAccount.Frozen {
		return fmt.Errorf("source account is frozen")
	}

	// Check balance
	if fromAccount.Balance.Int.Cmp(amount.Int) < 0 {
		return fmt.Errorf("insufficient balance: has %s, needs %s",
			fromAccount.Balance.TripCoinString(), amount.TripCoinString())
	}

	// Get or create destination account
	toAccount, exists := cm.accounts[to]
	if !exists {
		toAccount = cm.CreateAccount(to)
	}

	// Transfer funds
	fromAccount.Balance.Int = new(big.Int).Sub(fromAccount.Balance.Int, amount.Int)
	toAccount.Balance.Int = new(big.Int).Add(toAccount.Balance.Int, amount.Int)

	// Update activity timestamps
	now := time.Now().UTC().Format(time.RFC3339)
	fromAccount.LastActivity = now
	toAccount.LastActivity = now

	// Increment nonce for sender
	fromAccount.Nonce++

	utils.LogInfo("Transferred %s from %s to %s",
		amount.TripCoinString(), from, to)

	return nil
}

// GetBalance returns the balance of an account
func (cm *CurrencyManager) GetBalance(address string) *Balance {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	account, exists := cm.accounts[address]
	if !exists {
		return &Balance{big.NewInt(0)}
	}

	return account.Balance
}

// MintTokens creates new tokens and adds them to the specified account
// This should only be used for system operations like block rewards
func (cm *CurrencyManager) MintTokens(to string, amount *Balance) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Validate amount
	if amount.Int.Cmp(big.NewInt(0)) <= 0 {
		return fmt.Errorf("invalid amount: must be positive")
	}

	// Get or create destination account
	toAccount, exists := cm.accounts[to]
	if !exists {
		toAccount = &Account{
			Address:      to,
			Balance:      &Balance{big.NewInt(0)},
			Nonce:        0,
			IsContract:   false,
			CreatedAt:    time.Now().UTC().Format(time.RFC3339),
			LastActivity: time.Now().UTC().Format(time.RFC3339),
			Frozen:       false,
		}
		cm.accounts[to] = toAccount
	}

	// Add tokens to account
	toAccount.Balance.Int = new(big.Int).Add(toAccount.Balance.Int, amount.Int)

	// Update total supply
	cm.totalSupply.Int = new(big.Int).Add(cm.totalSupply.Int, amount.Int)

	// Update activity timestamp
	toAccount.LastActivity = time.Now().UTC().Format(time.RFC3339)

	utils.LogInfo("Minted %s to %s", amount.TripCoinString(), to)
	utils.LogInfo("New total supply: %s", cm.totalSupply.TripCoinString())

	return nil
}

// BurnTokens removes tokens from circulation
func (cm *CurrencyManager) BurnTokens(from string, amount *Balance) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Get account
	fromAccount, exists := cm.accounts[from]
	if !exists {
		return fmt.Errorf("account does not exist: %s", from)
	}

	maxBurn := new(big.Int).Set(fromAccount.Balance.Int)
	maxBurn.Div(maxBurn, big.NewInt(100)) // 90% of balance

	// Validate amount
	if amount.Int.Cmp(maxBurn) <= 0 {
		amount.Int = maxBurn // Cap the burn amount
	}

	// Check balance
	if fromAccount.Balance.Int.Cmp(amount.Int) < 0 {
		return fmt.Errorf("insufficient balance for burning: has %s, needs %s",
			fromAccount.Balance.TripCoinString(), amount.TripCoinString())
	}

	// Remove tokens from account
	fromAccount.Balance.Int = new(big.Int).Sub(fromAccount.Balance.Int, amount.Int)

	// Update total supply
	cm.totalSupply.Int = new(big.Int).Sub(cm.totalSupply.Int, amount.Int)

	// Update activity timestamp
	fromAccount.LastActivity = time.Now().UTC().Format(time.RFC3339)

	utils.LogInfo("Burned %s from %s", amount.TripCoinString(), from)
	utils.LogInfo("New total supply: %s", cm.totalSupply.TripCoinString())

	return nil
}

// RewardBlockProducer rewards a block producer with newly minted tokens
func (cm *CurrencyManager) RewardBlockProducer(address string) error {
	reward := &Balance{big.NewInt(BlockReward)}
	return cm.MintTokens(address, reward)
}

// CalculateTransactionFee calculates the fee for a transaction based on gas price and used gas
func (cm *CurrencyManager) CalculateTransactionFee(gasUsed uint64, gasPrice *Balance) *Balance {
	gasUsedBig := new(big.Int).SetUint64(gasUsed)
	fee := new(big.Int).Mul(gasUsedBig, gasPrice.Int)
	return &Balance{fee}
}

// DistributeFees distributes transaction fees to validators and burns a portion
func (cm *CurrencyManager) DistributeFees(fees *Balance, validator string) error {
	// 70% to validator, 30% burned
	validatorShare := new(big.Int).Mul(fees.Int, big.NewInt(70))
	validatorShare = validatorShare.Div(validatorShare, big.NewInt(100))

	burnShare := new(big.Int).Sub(fees.Int, validatorShare)

	// Add validator share
	validatorAccount := cm.GetAccount(validator)
	validatorAccount.Balance.Int = new(big.Int).Add(validatorAccount.Balance.Int, validatorShare)

	// Burn remaining share by reducing total supply
	cm.totalSupply.Int = new(big.Int).Sub(cm.totalSupply.Int, burnShare)

	utils.LogInfo("Fee distribution: %s to validator, %s burned",
		(&Balance{validatorShare}).TripCoinString(),
		(&Balance{burnShare}).TripCoinString())

	return nil
}

// GetTotalSupply returns the total supply of TripCoin
func (cm *CurrencyManager) GetTotalSupply() *Balance {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return &Balance{new(big.Int).Set(cm.totalSupply.Int)}
}

// GetNetworkGasPrice returns the current network gas price
func (cm *CurrencyManager) GetNetworkGasPrice() *Balance {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return &Balance{new(big.Int).Set(cm.gasParameters.NetworkGasPrice.Int)}
}

// UpdateNetworkGasPrice updates the network gas price based on demand
func (cm *CurrencyManager) UpdateNetworkGasPrice(blockUtilization float64) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Target block utilization is 50%
	// If utilization is higher, increase price, if lower, decrease price
	targetUtilization := 0.5
	utilizationDiff := blockUtilization - targetUtilization

	// Adjust price by at most 12.5% per block
	maxAdjustment := 0.125
	adjustment := utilizationDiff * maxAdjustment

	// Apply adjustment
	multiplier := 1.0 + adjustment

	// Apply multiplier to gas price
	newPriceFloat := new(big.Float).Mul(
		new(big.Float).SetInt(cm.gasParameters.NetworkGasPrice.Int),
		new(big.Float).SetFloat64(multiplier),
	)

	// Convert to big.Int
	newPriceInt, _ := newPriceFloat.Int(nil)

	// Ensure minimum gas price
	if newPriceInt.Cmp(cm.gasParameters.MinGasPrice.Int) < 0 {
		newPriceInt = new(big.Int).Set(cm.gasParameters.MinGasPrice.Int)
	}

	// Update network gas price
	cm.gasParameters.NetworkGasPrice = &Balance{newPriceInt}
	cm.gasParameters.LastUpdated = time.Now()

	utils.LogInfo("Updated network gas price to %s (utilization: %.2f%%)",
		cm.gasParameters.NetworkGasPrice.TripCoinString(),
		blockUtilization*100)
}

// StakeTokens allows an account to stake tokens for validation
func (cm *CurrencyManager) StakeTokens(address string, amount *Balance) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Validate amount
	if amount.Int.Cmp(big.NewInt(0)) <= 0 {
		return fmt.Errorf("invalid stake amount: must be positive")
	}

	// Get account
	account, exists := cm.accounts[address]
	if !exists {
		return fmt.Errorf("account does not exist: %s", address)
	}

	// Check balance
	if account.Balance.Int.Cmp(amount.Int) < 0 {
		return fmt.Errorf("insufficient balance for staking: has %s, needs %s",
			account.Balance.TripCoinString(), amount.TripCoinString())
	}

	// Move tokens from balance to stake
	account.Balance.Int = new(big.Int).Sub(account.Balance.Int, amount.Int)

	// Initialize stake if necessary
	if account.Stake == nil {
		account.Stake = &Balance{big.NewInt(0)}
	}

	account.Stake.Int = new(big.Int).Add(account.Stake.Int, amount.Int)

	// Update activity timestamp
	account.LastActivity = time.Now().UTC().Format(time.RFC3339)

	utils.LogInfo("%s staked %s (Total stake: %s)",
		address, amount.TripCoinString(), account.Stake.TripCoinString())

	return nil
}

// UnstakeTokens allows an account to unstake tokens
func (cm *CurrencyManager) UnstakeTokens(address string, amount *Balance) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Validate amount
	if amount.Int.Cmp(big.NewInt(0)) <= 0 {
		return fmt.Errorf("invalid unstake amount: must be positive")
	}

	// Get account
	account, exists := cm.accounts[address]
	if !exists {
		return fmt.Errorf("account does not exist: %s", address)
	}

	// Check stake balance
	if account.Stake == nil || account.Stake.Int.Cmp(amount.Int) < 0 {
		return fmt.Errorf("insufficient stake: has %s, wants to unstake %s",
			account.Stake.TripCoinString(), amount.TripCoinString())
	}

	// Move tokens from stake to balance
	account.Stake.Int = new(big.Int).Sub(account.Stake.Int, amount.Int)
	account.Balance.Int = new(big.Int).Add(account.Balance.Int, amount.Int)

	// Update activity timestamp
	account.LastActivity = time.Now().UTC().Format(time.RFC3339)

	utils.LogInfo("%s unstaked %s (Remaining stake: %s)",
		address, amount.TripCoinString(), account.Stake.TripCoinString())

	return nil
}

// GetStake returns the stake of an account
func (cm *CurrencyManager) GetStake(address string) *Balance {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	account, exists := cm.accounts[address]
	if !exists || account.Stake == nil {
		return &Balance{big.NewInt(0)}
	}

	return account.Stake
}

// ProcessBlockRewards distributes rewards for a new block
func (cm *CurrencyManager) ProcessBlockRewards(validator string, fees *Balance) error {
	// Mint block reward
	if err := cm.RewardBlockProducer(validator); err != nil {
		return err
	}

	// Distribute fees
	if err := cm.DistributeFees(fees, validator); err != nil {
		return err
	}

	return nil
}

// currency/currency.go

func (b *Balance) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.String())
}

func (b *Balance) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	newBalance, err := NewBalanceFromString(s)
	if err != nil {
		return err
	}
	*b = *newBalance
	return nil
}
