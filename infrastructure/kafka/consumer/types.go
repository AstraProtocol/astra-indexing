package consumer

type CollectedEvmTxs struct {
	Value                      int    `json:"value"`
	V                          int    `json:"v"`
	Type                       int    `json:"type"`
	TransactionIndex           int    `json:"transaction_index"`
	TransactionHash            string `json:"transaction_hash"`
	ToAddressHash              string `json:"to_address_hash"`
	Status                     string `json:"status"`
	S                          int64  `json:"s"`
	R                          int64  `json:"r"`
	Nonce                      int    `json:"nonce"`
	Input                      string `json:"input"`
	Index                      int    `json:"index"`
	Hash                       string `json:"hash"`
	GasUsed                    int64  `json:"gas_used"`
	GasPrice                   int64  `json:"gas_price"`
	Gas                        int64  `json:"gas"`
	FromAddressHash            string `json:"from_address_hash"`
	CumulativeGasUsed          int64  `json:"cumulative_gas_used"`
	CreatedContractAddressHash string `json:"created_contract_address_hash"`
	BlockNumber                int64  `json:"block_number"`
	BlockHash                  string `json:"block_hash"`
}
