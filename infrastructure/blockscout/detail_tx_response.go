package blockscout

import "github.com/AstraProtocol/astra-indexing/external/utctime"

type Log struct {
	Address     string   `json:"address"`
	AddressName string   `json:"addressName"`
	Data        string   `json:"data"`
	Index       string   `json:"index"`
	Topics      []string `json:"topics"`
}

type TokenTransfer struct {
	Amount               string `json:"amount"`
	Decimals             string `json:"decimals"`
	FromAddress          string `json:"fromAddress"`
	FromAddressName      string `json:"fromAddressName"`
	LogIndex             string `json:"logIndex"`
	ToAddress            string `json:"toAddress"`
	ToAddressName        string `json:"toAddressName"`
	TokenContractAddress string `json:"tokenContractAddress"`
	TokenName            string `json:"tokenName"`
	TokenSymbol          string `json:"tokenSymbol"`
	TokenId              string `json:"tokenId"`
	TokenType            string `json:"tokenType"`
}

type TransactionEvm struct {
	BlockHeight                  int64           `json:"blockHeight"`
	BlockHash                    string          `json:"blockHash"`
	BlockTime                    utctime.UTCTime `json:"blockTime"`
	Confirmations                int64           `json:"confirmations"`
	Hash                         string          `json:"hash"`
	CosmosHash                   string          `json:"cosmosHash"`
	Index                        int             `json:"index"`
	Success                      bool            `json:"success"`
	Error                        string          `json:"error"`
	RevertReason                 string          `json:"revertReason"`
	CreatedContractAddressHash   string          `json:"createdContractAddressHash"`
	CreatedContractCodeIndexedAt utctime.UTCTime `json:"createdContractCodeIndexedAt"`
	From                         string          `json:"from"`
	To                           string          `json:"to"`
	Value                        string          `json:"value"`
	CumulativeGasUsed            string          `json:"cumulativeGasUsed"`
	GasLimit                     string          `json:"gasLimit"`
	GasPrice                     string          `json:"gasPrice"`
	GasUsed                      string          `json:"gasUsed"`
	MaxFeePerGas                 string          `json:"maxFeePerGas"`
	MaxPriorityFeePerGas         string          `json:"maxPriorityFeePerGas"`
	Input                        string          `json:"input"`
	Nonce                        int             `json:"nonce"`
	R                            string          `json:"r"`
	S                            string          `json:"s"`
	V                            string          `json:"v"`
	Type                         int             `json:"type"`
	Logs                         []Log           `json:"logs"`
	TokenTransfers               []TokenTransfer `json:"tokenTransfers"`
}

type TxResp struct {
	Message string         `json:"message"`
	Result  TransactionEvm `json:"result"`
	Status  string         `json:"status"`
}
