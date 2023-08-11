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

func SearchResultsToAddresses(data []SearchResult) []AddressResult {
	var addresses []AddressResult
	for _, address_data := range data {
		var address AddressResult
		converted, _ := hex.DecodeString(address_data.AddressHash[2:])
		astraAddress, _ := tmcosmosutils.EncodeHexToAddress("astra", converted)
		address.Address = astraAddress
		address.AddressHash = address_data.AddressHash
		address.Name = address_data.Name
		addresses = append(addresses, address)
	}
	return addresses
}

func SearchResultsToContracts(data []SearchResult) []ContractResult {
	var contracts []ContractResult
	for _, contract_data := range data {
		var contract ContractResult
		contract.AddressHash = contract_data.AddressHash
		contract.Name = contract_data.Name
		contract.InsertedAt = contract_data.InsertedAt
		contracts = append(contracts, contract)
	}
	return contracts
}

func SearchResultsToTokens(data []SearchResult) []TokenResult {
	var tokens []TokenResult
	for _, token_data := range data {
		if token_data.AddressHash == "" {
			continue
		}
		var token TokenResult
		token.AddressHash = token_data.AddressHash
		token.HolderCount = token_data.HolderCount
		token.Name = token_data.Name
		token.Symbol = token_data.Symbol
		token.InsertedAt = token_data.InsertedAt
		tokens = append(tokens, token)
	}
	return tokens
}

func SearchResultsToTransactions(data []SearchResult) []TransactionResult {
	var transactions []TransactionResult
	for _, transaction_data := range data {
		var transaction TransactionResult
		transaction.EvmHash = transaction_data.TxHash
		transaction.CosmosHash = transaction_data.CosmosHash
		transaction.InsertedAt = transaction_data.InsertedAt
		transactions = append(transactions, transaction)
	}
	return transactions
}

func SearchResultsToBlocks(data []SearchResult) []BlockResult {
	var blocks []BlockResult
	for _, block_data := range data {
		var block BlockResult
		block.BlockHash = block_data.BlockHash
		block.BlockNumber = block_data.BlockNumber
		block.InsertedAt = block_data.InsertedAt
		blocks = append(blocks, block)
	}
	return blocks
}
