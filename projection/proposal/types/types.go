package types

import "github.com/AstraProtocol/astra-indexing/usecase/coin"

type CommunityPoolSpendData struct {
	RecipientAddress string     `json:"recipient"`
	Amount           coin.Coins `json:"amount"`
}
