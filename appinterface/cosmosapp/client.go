package cosmosapp

import (
	"errors"

	"github.com/AstraProtocol/astra-indexing/usecase/coin"
	"github.com/AstraProtocol/astra-indexing/usecase/model"
)

type Client interface {
	Account(accountAddress string) (*Account, error)

	Balances(accountAddress string) (coin.Coins, error)
	BalancesAsync(accountAddress string, balancesChan chan coin.Coins)

	BondedBalance(accountAddress string) (coin.Coins, error)
	BondedBalanceAsync(accountAddress string, bondedBalanceChan chan coin.Coins)

	RedelegatingBalance(accountAddress string) (coin.Coins, error)
	RedelegatingBalanceAsync(accountAddress string, redelegatingBalanceChan chan coin.Coins)

	UnbondingBalance(accountAddress string) (coin.Coins, error)
	UnbondingBalanceAsync(accountAddress string, unbondingBalanceChan chan coin.Coins)

	TotalRewards(accountAddress string) (coin.DecCoins, error)
	TotalRewardsAsync(accountAddress string, rewardBalanceChan chan coin.DecCoins)

	Commission(validatorAddress string) (coin.DecCoins, error)
	CommissionAsync(validatorAddress string, commissionBalanceChan chan coin.DecCoins)

	Validator(validatorAddress string) (*Validator, error)
	Delegation(delegator string, validator string) (*DelegationResponse, error)
	TotalBondedBalance() (coin.Coin, error)

	AnnualProvisions() (coin.DecCoin, error)

	Proposals() ([]Proposal, error)
	ProposalById(id string) (Proposal, error)
	ProposalTally(id string) (Tally, error)
	DepositParams() (Params, error)

	Tx(txHash string) (*model.Tx, error)

	TotalFeeBurn() (TotalFeeBurn, error)

	VestingBalances(account string) (VestingBalances, error)
	VestingBalancesAsync(account string, vestingBalancesChan chan VestingBalances)

	BlockInfo(height string) (*BlockInfo, error)
}

var ErrAccountNotFound = errors.New("account not found")
var ErrAccountNoDelegation = errors.New("account has no delegation")
var ErrProposalNotFound = errors.New("proposal not found")
var ErrTotalFeeBurnNotFound = errors.New("total fee burn not found")
var ErrVestingBalancesNotFound = errors.New("vesting balances not found")
var ErrBlockInfoNotFound = errors.New("block info not found")
