package model

import "github.com/AstraProtocol/astra-indexing/usecase/coin"

type AccountTransferParams struct {
	Recipient string
	Sender    string
	Amount    coin.Coins
}
