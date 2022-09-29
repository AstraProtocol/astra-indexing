package model

import "github.com/AstraProtocol/astra-indexing/usecase/coin"

type MsgWithdrawValidatorCommissionParams struct {
	ValidatorAddress string     `json:"validatorAddress"`
	RecipientAddress string     `json:"recipientAddress"`
	Amount           coin.Coins `json:"amount"`
}
