package blockscout

import (
	"github.com/AstraProtocol/astra-indexing/appinterface/cosmosapp"
)

type Address struct {
	Balance             string                    `json:"balance"`
	ContractName        string                    `json:"contractName"`
	CreationTransaction string                    `json:"creationTransaction"`
	Creator             string                    `json:"creator"`
	LastBalanceUpdate   int64                     `json:"lastBalanceUpdate"`
	TokenName           string                    `json:"tokenName"`
	TokenSymbol         string                    `json:"tokenSymbol"`
	Type                string                    `json:"type"`
	Verified            bool                      `json:"verified"`
	VestingBalances     cosmosapp.VestingBalances `json:"vestingBalances"`
}

type AddressResp struct {
	Message string  `json:"message"`
	Result  Address `json:"result"`
	Status  string  `json:"status"`
}
