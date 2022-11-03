package handlers

import (
	"encoding/hex"
	"errors"
	"strings"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	"github.com/AstraProtocol/astra-indexing/external/tmcosmosutils"
	blockscout_infrastructure "github.com/AstraProtocol/astra-indexing/infrastructure/blockscout"
	"github.com/AstraProtocol/astra-indexing/infrastructure/httpapi"
	account_transaction_view "github.com/AstraProtocol/astra-indexing/projection/account_transaction/view"
	block_view "github.com/AstraProtocol/astra-indexing/projection/block/view"
	transaction_view "github.com/AstraProtocol/astra-indexing/projection/transaction/view"
	validator_view "github.com/AstraProtocol/astra-indexing/projection/validator/view"
	"github.com/valyala/fasthttp"
)

type Search struct {
	logger                       applogger.Logger
	blockscoutClient             blockscout_infrastructure.HTTPClient
	blocksView                   *block_view.Blocks
	transactionsView             transaction_view.BlockTransactions
	validatorsView               *validator_view.Validators
	accountTransactionsTotalView *account_transaction_view.AccountTransactionsTotal
}

type SearchResults struct {
	Blocks       []blockscout_infrastructure.BlockResult       `json:"blocks"`
	Transactions []blockscout_infrastructure.TransactionResult `json:"transactions"`
	Addresses    []blockscout_infrastructure.AddressResult     `json:"addresses"`
	Tokens       []blockscout_infrastructure.TokenResult       `json:"tokens"`
	Validators   []blockscout_infrastructure.ValidatorResult   `json:"validators"`
}

func NewSearch(logger applogger.Logger, blockscoutClient blockscout_infrastructure.HTTPClient, rdbHandle *rdb.Handle) *Search {
	return &Search{
		logger.WithFields(applogger.LogFields{
			"module": "SearchHandler",
		}),
		blockscoutClient,

		block_view.NewBlocks(rdbHandle),
		transaction_view.NewTransactionsView(rdbHandle),
		validator_view.NewValidators(rdbHandle),
		account_transaction_view.NewAccountTransactionsTotal(rdbHandle),
	}
}

func (search *Search) Search(ctx *fasthttp.RequestCtx) {
	resultsChan := make(chan []blockscout_infrastructure.SearchResult)

	keyword := string(ctx.QueryArgs().Peek("keyword"))

	var results SearchResults

	if tmcosmosutils.IsValidCosmosAddress(keyword) {
		if strings.Contains(keyword, "valoper") {
			// If keyword is validator address (ex: "astravaloper16mqptvptnds4098cmdmz846lmazenegc270ljs")
			// use chainindexing search for validator
			validators, err := search.validatorsView.Search(keyword)
			if err != nil {
				if errors.Is(err, rdb.ErrNoRows) {
					validators = nil
				} else {
					search.logger.Errorf("error searching validator: %v", err)
					httpapi.InternalServerError(ctx)
					return
				}
			}
			if len(validators) > 0 {
				results.Validators = parseValidators(validators)
				results.Addresses = nil
				httpapi.Success(ctx, results)
				return
			}
		} else {
			// If keyword is bech32 address, ex: "astra1g9v3fp9wkhar696e7896x6wu3hqjsy5cpxdzff"
			// It must be converted to hex address then using blockscout's api search only
			_, converted, _ := tmcosmosutils.DecodeAddressToHex(keyword)
			hex_address := hex.EncodeToString(converted)
			blockscoutAddressResults := search.blockscoutClient.GetSearchResults("0x" + hex_address)
			results.Addresses = blockscout_infrastructure.SearchResultsToAddresses(blockscoutAddressResults)
			httpapi.Success(ctx, results)
			return
		}
	} else {
		// Otherwise using simultaneously blockscout and chainindexing search
		go search.blockscoutClient.GetSearchResultsAsync(keyword, resultsChan)
	}

	// If keyword is integer (ex: 9947), use chainindexing search for block
	blocks, err := search.blocksView.Search(keyword)
	if err != nil {
		if errors.Is(err, rdb.ErrNoRows) {
			blocks = nil
		} else {
			search.logger.Errorf("error searching block: %v", err)
			httpapi.InternalServerError(ctx)
			return
		}
	}
	if len(blocks) > 0 {
		results.Blocks = parseBlocks(blocks)
		httpapi.Success(ctx, results)
		return
	}

	blockscoutSearchResults := <-resultsChan

	// If keyword is cosmos tx (ex: "90FEE96EE94CA74AD67FCF155E15488B901B3AE2530EBE4D35A9E77B609EB348")
	// use chainindexing search for transaction then merge with evm tx from blockscout's search result
	transactions, err := search.transactionsView.Search(keyword)
	if err != nil {
		if errors.Is(err, rdb.ErrNoRows) {
			transactions = nil
		} else {
			search.logger.Errorf("error searching transaction: %v", err)
			httpapi.InternalServerError(ctx)
			return
		}
	}
	if len(transactions) > 0 {
		results.Transactions = parseTransactions(transactions, blockscoutSearchResults)
		httpapi.Success(ctx, results)
		return
	}

	// Using blockscout's search results when chainindexing's search results are empty
	// mostly token search or keyword is hex type
	if isResultsEmpty(results) {
		if len(blockscoutSearchResults) > 0 {
			switch blockscoutSearchResults[0].Type {
			case "token":
				results.Tokens = blockscout_infrastructure.SearchResultsToTokens(blockscoutSearchResults)
			case "block":
				results.Blocks = blockscout_infrastructure.SearchResultsToBlocks(blockscoutSearchResults)
			case "address":
				results.Addresses = blockscout_infrastructure.SearchResultsToAddresses(blockscoutSearchResults)
			case "transaction":
				results.Transactions = blockscout_infrastructure.SearchResultsToTransactions(blockscoutSearchResults)
			case "transaction_cosmos":
				results.Transactions = blockscout_infrastructure.SearchResultsToTransactions(blockscoutSearchResults)
			}
		}
	}

	httpapi.Success(ctx, results)
}

func parseBlocks(data []block_view.Block) []blockscout_infrastructure.BlockResult {
	var blocks []blockscout_infrastructure.BlockResult
	for _, block_data := range data {
		var block blockscout_infrastructure.BlockResult
		block.BlockHash = "0x" + block_data.Hash
		block.BlockNumber = int(block_data.Height)
		block.InsertedAt = block_data.Time
		blocks = append(blocks, block)
	}
	return blocks
}

func parseValidators(data []validator_view.ValidatorRow) []blockscout_infrastructure.ValidatorResult {
	var validators []blockscout_infrastructure.ValidatorResult
	for _, validator_data := range data {
		var validator blockscout_infrastructure.ValidatorResult
		validator.Jailed = validator_data.Jailed
		validator.Status = validator_data.Status
		validator.ConsensusNodeAddress = validator_data.ConsensusNodeAddress
		validator.InitialDelegatorAddress = validator_data.InitialDelegatorAddress
		_, converted, _ := tmcosmosutils.DecodeAddressToHex(validator_data.InitialDelegatorAddress)
		validator.InitialDelegatorAddressHash = "0x" + hex.EncodeToString(converted)
		validators = append(validators, validator)
	}
	return validators
}

func parseTransactions(data []transaction_view.TransactionRow, blockscout_data []blockscout_infrastructure.SearchResult) []blockscout_infrastructure.TransactionResult {
	var transactions []blockscout_infrastructure.TransactionResult
	for _, transaction_data := range data {
		var transaction blockscout_infrastructure.TransactionResult
		transaction.CosmosHash = transaction_data.Hash
		transaction.InsertedAt = transaction_data.BlockTime
		for _, result := range blockscout_data {
			if transaction.CosmosHash == result.CosmosHash {
				transaction.EvmHash = result.TxHash
			}
		}
		transactions = append(transactions, transaction)
	}
	return transactions
}

func isResultsEmpty(results SearchResults) bool {
	if results.Addresses != nil {
		return false
	}
	if results.Blocks != nil {
		return false
	}
	if results.Tokens != nil {
		return false
	}
	if results.Transactions != nil {
		return false
	}
	if results.Validators != nil {
		return false
	}
	return true
}
