package blockscout

type AddressCounter struct {
	GasUsageCount      int64   `json:"gasUsageCount"`
	FeesCount          float64 `json:"feesCount"`
	TokenTransferCount int     `json:"tokenTransferCount"`
	TransactionCount   int64   `json:"transactionCount"`
	ValidationCount    int     `json:"validationCount"`
	Type               string  `json:"type"`
}

type AddressCounterResp struct {
	Message string         `json:"message"`
	Result  AddressCounter `json:"result"`
	Status  string         `json:"status"`
}

type TopAddressesBalanceResult struct {
	Address  string `json:"address"`
	Balance  string `json:"balance"`
	TxnCount int64  `json:"txnCount"`
}

type TopAddressesBalanceResp struct {
	HasNextPage  bool                        `json:"hasNextPage"`
	NextPagePath string                      `json:"nextPagePath"`
	Result       []TopAddressesBalanceResult `json:"result"`
}
