package model

import (
	"github.com/AstraProtocol/astra-indexing/usecase/coin"
)

type MsgCreateValidatorParams struct {
	Description       ValidatorDescription `json:"description"`
	Commission        ValidatorCommission  `json:"commission"`
	MinSelfDelegation string               `json:"minSelfDelegation"`
	DelegatorAddress  string               `json:"delegatorAddress"`
	ValidatorAddress  string               `json:"validatorAddress"`
	TendermintPubkey  string               `json:"tendermintPubkey"`
	Amount            coin.Coin            `json:"amount"`
}
