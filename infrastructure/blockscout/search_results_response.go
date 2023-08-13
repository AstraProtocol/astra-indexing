package blockscout

import (
	"encoding/hex"

	"github.com/AstraProtocol/astra-indexing/external/tmcosmosutils"
	"github.com/AstraProtocol/astra-indexing/external/utctime"
)

type SearchResult struct {
	AddressHash string          `json:"address_hash"`
	BlockHash   string          `json:"block_hash"`
	BlockNumber int             `json:"block_number"`
	CosmosHash  string          `json:"cosmos_hash"`
	HolderCount int             `json:"holder_count"`
	InsertedAt  utctime.UTCTime `json:"inserted_at"`
	Name        string          `json:"name"`
	Symbol      string          `json:"symbol"`
	TxHash      string          `json:"tx_hash"`
	Type        string          `json:"type"`
}

type TransactionResult struct {
	EvmHash    string          `json:"evmHash"`
	CosmosHash string          `json:"cosmosHash"`
	InsertedAt utctime.UTCTime `json:"insertedAt"`
}

type AddressResult struct {
	AddressHash string `json:"addressHash"`
	Address     string `json:"address"`
	Name        string `json:"name"`
}

type ValidatorResult struct {
	OperatorAddress             string `json:"operatorAddress"`
	ConsensusNodeAddress        string `json:"consensusNodeAddress"`
	InitialDelegatorAddress     string `json:"initialDelegatorAddress"`
	InitialDelegatorAddressHash string `json:"initialDelegatorAddressHash"`
	Moniker                     string `json:"moniker"`
	Status                      string `json:"status"`
	Jailed                      bool   `json:"jailed"`
}

type TokenResult struct {
	AddressHash string          `json:"addressHash"`
	HolderCount int             `json:"holderCount"`
	Name        string          `json:"name"`
	Symbol      string          `json:"symbol"`
	InsertedAt  utctime.UTCTime `json:"insertedAt"`
}

type BlockResult struct {
	BlockHash   string          `json:"blockHash"`
	BlockNumber int             `json:"blockNumber"`
	InsertedAt  utctime.UTCTime `json:"insertedAt"`
}

type ContractResult struct {
	AddressHash string          `json:"addressHash"`
	Name        string          `json:"name"`
	InsertedAt  utctime.UTCTime `json:"insertedAt"`
}

func (result *SearchResult) ToAddress() AddressResult {
	var address AddressResult
	converted, _ := hex.DecodeString(result.AddressHash[2:])
	astraAddress, _ := tmcosmosutils.EncodeHexToAddress("astra", converted)
	address.Address = astraAddress
	address.AddressHash = result.AddressHash
	address.Name = result.Name
	return address
}

func (result *SearchResult) ToContract() ContractResult {
	var contract ContractResult
	contract.AddressHash = result.AddressHash
	contract.Name = result.Name
	contract.InsertedAt = result.InsertedAt
	return contract
}

func (result *SearchResult) ToToken() TokenResult {
	var token TokenResult
	token.AddressHash = result.AddressHash
	token.HolderCount = result.HolderCount
	token.Name = result.Name
	token.Symbol = result.Symbol
	token.InsertedAt = result.InsertedAt
	return token
}

func (result *SearchResult) ToTransaction() TransactionResult {
	var transaction TransactionResult
	transaction.EvmHash = result.TxHash
	transaction.CosmosHash = result.CosmosHash
	transaction.InsertedAt = result.InsertedAt
	return transaction
}

func (result *SearchResult) ToBlock() BlockResult {
	var block BlockResult
	block.BlockHash = result.BlockHash
	block.BlockNumber = result.BlockNumber
	block.InsertedAt = result.InsertedAt
	return block
}
