package cosmosapp

import (
	"errors"

	"github.com/AstraProtocol/astra-indexing/usecase/coin"
	"github.com/AstraProtocol/astra-indexing/usecase/model"
)

type Client interface {
	Account(accountAddress string) (*Account, error)
	Balances(accountAddress string) (coin.Coins, error)
	BondedBalance(accountAddress string) (coin.Coins, error)
	RedelegatingBalance(accountAddress string) (coin.Coins, error)
	UnbondingBalance(accountAddress string) (coin.Coins, error)

	TotalRewards(accountAddress string) (coin.DecCoins, error)
	Commission(validatorAddress string) (coin.DecCoins, error)

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
}

var ErrAccountNotFound = errors.New("account not found")
var ErrAccountNoDelegation = errors.New("account has no delegation")
var ErrProposalNotFound = errors.New("proposal not found")
var ErrTotalFeeBurnNotFound = errors.New("total fee burn not found")
var ErrVestingBalancesnNotFound = errors.New("vesting balances not found")
