package blockscout

type TokenAddress struct {
	Balance         string `json:"balance"`
	ContractAddress string `json:"contractAddress"`
	Decimals        string `json:"decimals"`
	Name            string `json:"name"`
	Symbol          string `json:"symbol"`
	Type            string `json:"type"`
}

type TokensAddressResp struct {
	HasNextPage  bool           `json:"hasNextPage"`
	NextPagePath string         `json:"nextPagePath"`
	Result       []TokenAddress `json:"result"`
}
