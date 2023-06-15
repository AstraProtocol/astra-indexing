package consumer

import "encoding/json"

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

type CollectedInternalTx struct {
	Value            json.Number   `json:"value"`
	Type             string        `json:"type"`
	TransactionIndex int64         `json:"transaction_index"`
	TransactionHash  string        `json:"transaction_hash"`
	TraceAddress     []interface{} `json:"trace_address"`
	ToAddressHash    string        `json:"to_address_hash"`
	Output           string        `json:"output"`
	Input            string        `json:"input"`
	Index            int64         `json:"index"`
	GasUsed          int64         `json:"gas_used"`
	Gas              int64         `json:"gas"`
	FromAddressHash  string        `json:"from_address_hash"`
	CallType         string        `json:"call_type"`
	BlockNumber      int64         `json:"block_number"`
}
