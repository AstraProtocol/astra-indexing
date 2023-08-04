package blockscout

import (
	"github.com/AstraProtocol/astra-indexing/external/utctime"
)

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
	CreatedContractAddressName   string          `json:"createdContractAddressName"`
	CreatedContractCodeIndexedAt utctime.UTCTime `json:"createdContractCodeIndexedAt"`
	From                         string          `json:"from"`
	FromAddressName              string          `json:"fromAddressName"`
	To                           string          `json:"to"`
	ToAddressName                string          `json:"toAddressName"`
	IsInteractWithContract       bool            `json:"isInteractWithContract"`
	Value                        string          `json:"value"`
	CumulativeGasUsed            string          `json:"cumulativeGasUsed"`
	GasLimit                     string          `json:"gasLimit"`
	GasPrice                     string          `json:"gasPrice"`
	GasUsed                      string          `json:"gasUsed"`
	TransactionFee               string          `json:"transactionFee"`
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

type TxHashWithTokenTransfers struct {
	Hash           string          `json:"hash"`
	Success        bool            `json:"success"`
	Error          string          `json:"error"`
	TokenTransfers []TokenTransfer `json:"tokenTransfers"`
}

type InternalTransaction struct {
	BlockNumber     string `json:"blockNumber"`
	CallType        string `json:"callType"`
	ContractAddress string `json:"contractAddress"`
	ErrCode         string `json:"errCode"`
	From            string `json:"from"`
	FromAddressName string `json:"fromAddressName"`
	Gas             string `json:"gas"`
	GasUsed         string `json:"gasUsed"`
	Index           string `json:"index"`
	Input           string `json:"input"`
	IsError         string `json:"isError"`
	TimeStamp       string `json:"timeStamp"`
	To              string `json:"to"`
	ToAddressName   string `json:"toAddressName"`
	TransactionHash string `json:"transactionHash"`
	Type            string `json:"type"`
	Value           string `json:"value"`
}

type TxResp struct {
	Message string         `json:"message"`
	Result  TransactionEvm `json:"result"`
	Status  string         `json:"status"`
}

type TxsResp struct {
	Message string                     `json:"message"`
	Result  []TxHashWithTokenTransfers `json:"result"`
	Status  string                     `json:"status"`
}

type InternalTxsResp struct {
	Message string                `json:"message"`
	Result  []InternalTransaction `json:"result"`
	Status  string                `json:"status"`
}
