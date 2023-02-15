package blockscout

type Token struct {
	Cataloged           bool   `json:"cataloged"`
	ContractAddressHash string `json:"contractAddressHash"`
	ContractAddressName string `json:"contractAddressName"`
	Decimals            string `json:"decimals"`
	HolderCount         int64  `json:"holderCount"`
	Name                string `json:"name"`
	Symbol              string `json:"symbol"`
	TotalSupply         string `json:"totalSupply"`
	Type                string `json:"type"`
}

type ListTokenResp struct {
	HasNextPage bool    `json:"hasNextPage"`
	Result      []Token `json:"result"`
}
