package model

import "github.com/AstraProtocol/astra-indexing/usecase/coin"

type CompleteBondingParams struct {
	Delegator string
	Validator string
	Amount    coin.Coins
}
