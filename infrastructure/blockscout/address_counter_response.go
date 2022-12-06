package blockscout

type AddressCounter struct {
	GasUsageCount      int64 `json:"gasUsageCount"`
	TokenTransferCount int   `json:"tokenTransferCount"`
	TransactionCount   int64 `json:"transactionCount"`
	ValidationCount    int   `json:"validationCount"`
}

type AddressCounterResp struct {
	Message string         `json:"message"`
	Result  AddressCounter `json:"result"`
	Status  string         `json:"status"`
}
