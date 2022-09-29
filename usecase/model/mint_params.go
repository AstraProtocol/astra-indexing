package model

import "github.com/AstraProtocol/astra-indexing/usecase/coin"

type MintParams struct {
	BondedRatio      string
	Inflation        string
	AnnualProvisions coin.DecCoin
	Amount           coin.Coins
}
