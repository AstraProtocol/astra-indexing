package consumer

type CollectedEvmTx struct {
	BlockNumber                int64  `json:"block_number"`
	TransactionHash            string `json:"transaction_hash"`
	TransactionIndex           int    `json:"transaction_index"`
	Value                      int    `json:"value"`
	Type                       int    `json:"type"`
	FromAddressHash            string `json:"from_address_hash"`
	ToAddressHash              string `json:"to_address_hash"`
	Status                     string `json:"status"`
	V                          int    `json:"v"`
	R                          int64  `json:"r"`
	S                          int64  `json:"s"`
	Nonce                      int    `json:"nonce"`
	Input                      string `json:"input"`
	Index                      int    `json:"index"`
	GasUsed                    int64  `json:"gas_used"`
	GasPrice                   int64  `json:"gas_price"`
	Gas                        int64  `json:"gas"`
	CumulativeGasUsed          int64  `json:"cumulative_gas_used"`
	CreatedContractAddressHash string `json:"created_contract_address_hash"`
	BlockHash                  string `json:"block_hash"`
}
