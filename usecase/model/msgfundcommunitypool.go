package model

import "github.com/AstraProtocol/astra-indexing/usecase/coin"

type MsgFundCommunityPoolParams struct {
	Depositor string     `json:"depositor"`
	Amount    coin.Coins `json:"amount"`
}
