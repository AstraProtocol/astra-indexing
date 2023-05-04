package consumer

type CollectedEvmTx struct {
	BlockNumber       int64  `json:"block_number"`
	BlockHash         string `json:"block_hash"`
	TransactionHash   string `json:"transaction_hash"`
	Status            string `json:"status"`
	GasUsed           int64  `json:"gas_used"`
	GasPrice          int64  `json:"gas_price"`
	Gas               int64  `json:"gas"`
	CumulativeGasUsed int64  `json:"cumulative_gas_used"`
}
