package model

import (
	"github.com/AstraProtocol/astra-indexing/usecase/coin"
)

type MsgBeginRedelegateParams struct {
	DelegatorAddress    string    `json:"delegatorAddress"`
	ValidatorSrcAddress string    `json:"validatorSrcAddress"`
	ValidatorDstAddress string    `json:"validatorDstAddress"`
	Amount              coin.Coin `json:"amount"`
	AutoClaimedRewards  coin.Coin `json:"autoClaimedRewards"`
}
