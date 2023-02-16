package blockscout

type AccountAbiResp struct {
	Message string `json:"message"`
	Result  string `json:"result"`
	Status  string `json:"status"`
}

type TxAbiResp struct {
	Message string    `json:"message"`
	Result  AbiResult `json:"result"`
	Status  string    `json:"status"`
}

type AbiResult struct {
	Abi      interface{} `json:"abi"`
	Verified bool        `json:"verified"`
}
