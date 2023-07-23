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

type CollectedTokenTransfer struct {
	Tokens         []Token         `json:"tokens"`
	TokenTransfers []TokenTransfer `json:"token_transfers"`
}

type Token struct {
	Type                string `json:"type"`
	ContractAddressHash string `json:"contract_address_hash"`
}

type TokenTransfer struct {
	TransactionHash          string `json:"transaction_hash"`
	TokenType                string `json:"token_type"`
	TokenId                  int64  `json:"token_id"`
	TokenContractAddressHash string `json:"token_contract_address_hash"`
	ToAddressHash            string `json:"to_address_hash"`
	LogIndex                 int64  `json:"log_index"`
	FromAddressHash          string `json:"from_address_hash"`
	BlockNumber              int64  `json:"block_number"`
	BlockHash                string `json:"block_hash"`
}
