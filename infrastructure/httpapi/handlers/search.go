package handlers

import (
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/AstraProtocol/astra-indexing/appinterface/cosmosapp"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	"github.com/AstraProtocol/astra-indexing/external/tmcosmosutils"
	blockscout_infrastructure "github.com/AstraProtocol/astra-indexing/infrastructure/blockscout"
	"github.com/AstraProtocol/astra-indexing/infrastructure/httpapi"
	"github.com/AstraProtocol/astra-indexing/infrastructure/metric/prometheus"
	evm_utils "github.com/AstraProtocol/astra-indexing/internal/evm"
	account_view "github.com/AstraProtocol/astra-indexing/projection/account/view"
	account_transaction_view "github.com/AstraProtocol/astra-indexing/projection/account_transaction/view"
	block_view "github.com/AstraProtocol/astra-indexing/projection/block/view"
	transaction_view "github.com/AstraProtocol/astra-indexing/projection/transaction/view"
	validator_view "github.com/AstraProtocol/astra-indexing/projection/validator/view"
	"github.com/valyala/fasthttp"
)

type Search struct {
	logger                       applogger.Logger
	blockscoutClient             blockscout_infrastructure.HTTPClient
	cosmosClient                 cosmosapp.Client
	blocksView                   *block_view.Blocks
	transactionsView             transaction_view.BlockTransactions
	validatorsView               *validator_view.Validators
	accountsView                 account_view.Accounts
	accountTransactionsTotalView *account_transaction_view.AccountTransactionsTotal
}

type SearchResults struct {
	Blocks       []blockscout_infrastructure.BlockResult       `json:"blocks"`
	Transactions []blockscout_infrastructure.TransactionResult `json:"transactions"`
	Addresses    []blockscout_infrastructure.AddressResult     `json:"addresses"`
	Tokens       []blockscout_infrastructure.TokenResult       `json:"tokens"`
	Validators   []blockscout_infrastructure.ValidatorResult   `json:"validators"`
	Contracts    []blockscout_infrastructure.ContractResult    `json:"contracts"`
}

func NewSearch(logger applogger.Logger, blockscoutClient blockscout_infrastructure.HTTPClient, cosmosClient cosmosapp.Client, rdbHandle *rdb.Handle) *Search {
	return &Search{
		logger.WithFields(applogger.LogFields{
			"module": "SearchHandler",
		}),
		blockscoutClient,
		cosmosClient,
		block_view.NewBlocks(rdbHandle),
		transaction_view.NewTransactionsView(rdbHandle),
		validator_view.NewValidators(rdbHandle),
		account_view.NewAccountsView(rdbHandle),
		account_transaction_view.NewAccountTransactionsTotal(rdbHandle),
	}
}

func (search *Search) Search(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "Search"

	resultsChan := make(chan []blockscout_infrastructure.SearchResult)
	keyword := string(ctx.QueryArgs().Peek("keyword"))
	var results SearchResults

	if tmcosmosutils.IsValidCosmosAddress(keyword) && strings.Contains(keyword, "valoper") {
		// If keyword is validator address (e.g: "astravaloper16mqptvptnds4098cmdmz846lmazenegc270ljs")
		// use chainindexing's validator search only
		validators, err := search.validatorsView.Search(keyword)
		if err != nil {
			if errors.Is(err, rdb.ErrNoRows) {
				validators = nil
			} else {
				search.logger.Errorf("error searching validator: %v", err)
				prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
				httpapi.InternalServerError(ctx)
				return
			}
		}
		if len(validators) > 0 {
			results.Validators = search.parseValidators(validators)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
			httpapi.Success(ctx, results)
			return
		}
	}

	var blockscoutSearchParam string
	var astraAddress string

	if tmcosmosutils.IsValidCosmosAddress(keyword) {
		// If keyword is bech32 address (e.g: "astra1g9v3fp9wkhar696e7896x6wu3hqjsy5cpxdzff")
		// Address must be converted to hex address then using blockscout's api search
		_, converted, _ := tmcosmosutils.DecodeAddressToHex(keyword)
		blockscoutSearchParam = "0x" + hex.EncodeToString(converted)
		astraAddress = keyword
	} else if evm_utils.IsHexAddress(keyword) {
		// If keyword is hex address (e.g: "0x194c37D6C9B51660e4dA668bC03Ed0E86469cDEE")
		// Address must be converted to astra address then using chainindexing search
		blockscoutSearchParam = keyword
		converted, _ := hex.DecodeString(keyword[2:])
		astraAddress, _ = tmcosmosutils.EncodeHexToAddress("astra", converted)
	}

	if tmcosmosutils.IsValidCosmosAddress(keyword) || evm_utils.IsHexAddress(keyword) {
		// Using simultaneously blockscout and chainindexing search
		go search.blockscoutClient.GetSearchResultsAsync(blockscoutSearchParam, resultsChan)

		// Using chainindexing search for astra address
		accountIdentity := account_view.AccountIdentity{}
		accountIdentity.Address = astraAddress
		accounts, err := search.accountsView.FindBy(&accountIdentity)
		if err != nil {
			if errors.Is(err, rdb.ErrNoRows) {
				accounts = nil
			} else {
				search.logger.Errorf("error searching account: %v", err)
				prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
				httpapi.InternalServerError(ctx)
				return
			}
		}

		// Get blockscout's address search result from channel
		blockscoutAddressResults := <-resultsChan

		if accounts != nil {
			// Merge blockscout and chainindexing search results
			results.Addresses = search.parseAddresses(*accounts, blockscoutAddressResults)
		} else {
			results.Addresses = blockscout_infrastructure.SearchResultsToAddresses(blockscoutAddressResults)
		}

		if accounts == nil {
			account, err := search.cosmosClient.Account(astraAddress)
			if err == nil && account.Address != "" {
				results.Addresses[0].AddressHash = blockscoutSearchParam
			}
		}

		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.Success(ctx, results)
		return
	}

	// Using simultaneously blockscout and chainindexing search
	go search.blockscoutClient.GetSearchResultsAsync(keyword, resultsChan)

	// If keyword is cosmos tx (e.g: "90FEE96EE94CA74AD67FCF155E15488B901B3AE2530EBE4D35A9E77B609EB348")
	// use chainindexing search for cosmos tx then merge with evm tx from blockscout's search result (if exist)
	transactions, err := search.transactionsView.Search(keyword)
	if err != nil {
		if errors.Is(err, rdb.ErrNoRows) {
			transactions = nil
		} else {
			search.logger.Errorf("error searching transaction: %v", err)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}
	}

	// Get blockscout's search result from channel
	blockscoutSearchResults := <-resultsChan

	if len(transactions) > 0 {
		// merge with evm tx from blockscout's search result (if exist)
		results.Transactions = search.parseTransactions(transactions, blockscoutSearchResults)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.Success(ctx, results)
		return
	}

	// Using blockscout's search results when chainindexing's search results are empty
	// mostly using for token, contract, block search or in case of keyword is hex type
	if search.isResultsEmpty(results) {
		if len(blockscoutSearchResults) > 0 {
			switch blockscoutSearchResults[0].Type {
			case "token":
				results.Tokens = blockscout_infrastructure.SearchResultsToTokens(blockscoutSearchResults)
			case "block":
				results.Blocks = blockscout_infrastructure.SearchResultsToBlocks(blockscoutSearchResults)
			case "address":
				results.Addresses = blockscout_infrastructure.SearchResultsToAddresses(blockscoutSearchResults)
			case "contract":
				results.Contracts = blockscout_infrastructure.SearchResultsToContracts(blockscoutSearchResults)
			case "transaction":
				results.Transactions = blockscout_infrastructure.SearchResultsToTransactions(blockscoutSearchResults)
			case "transaction_cosmos":
				results.Transactions = blockscout_infrastructure.SearchResultsToTransactions(blockscoutSearchResults)
			}
		}
	}

	// searching blocks in case of blockscout is slower than chainindexing
	if search.isResultsEmpty(results) {
		blocks, err := search.blocksView.Search(keyword)
		if err == nil {
			results.Blocks = search.parseBlocks(blocks)
		}
	}
	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, results)
}

func (search *Search) parseValidators(data []validator_view.ValidatorRow) []blockscout_infrastructure.ValidatorResult {
	var validators []blockscout_infrastructure.ValidatorResult
	for _, validator_data := range data {
		var validator blockscout_infrastructure.ValidatorResult
		validator.Jailed = validator_data.Jailed
		validator.Status = validator_data.Status
		validator.ConsensusNodeAddress = validator_data.ConsensusNodeAddress
		validator.InitialDelegatorAddress = validator_data.InitialDelegatorAddress
		validator.Moniker = validator_data.Moniker
		_, converted, _ := tmcosmosutils.DecodeAddressToHex(validator_data.InitialDelegatorAddress)
		validator.InitialDelegatorAddressHash = "0x" + hex.EncodeToString(converted)
		validators = append(validators, validator)
	}
	return validators
}

func (search *Search) parseBlocks(data []block_view.Block) []blockscout_infrastructure.BlockResult {
	var blocks []blockscout_infrastructure.BlockResult
	for _, block_data := range data {
		var block blockscout_infrastructure.BlockResult
		block.BlockHash = block_data.Hash
		block.BlockNumber = int(block_data.Height)
		block.InsertedAt = block_data.Time
		blocks = append(blocks, block)
	}
	return blocks
}

func (search *Search) parseAddresses(data account_view.AccountRow, blockscout_data []blockscout_infrastructure.SearchResult) []blockscout_infrastructure.AddressResult {
	var addresses []blockscout_infrastructure.AddressResult
	var address blockscout_infrastructure.AddressResult
	address.Address = data.Address
	_, converted, _ := tmcosmosutils.DecodeAddressToHex(address.Address)
	address.AddressHash = "0x" + hex.EncodeToString(converted)
	for _, result := range blockscout_data {
		if address.AddressHash == result.AddressHash {
			address.Name = result.Name
		}
	}
	addresses = append(addresses, address)
	return addresses
}

func (search *Search) parseTransactions(data []transaction_view.TransactionRow, blockscout_data []blockscout_infrastructure.SearchResult) []blockscout_infrastructure.TransactionResult {
	var transactions []blockscout_infrastructure.TransactionResult
	for _, transaction_data := range data {
		var transaction blockscout_infrastructure.TransactionResult
		transaction.CosmosHash = transaction_data.Hash
		transaction.InsertedAt = transaction_data.BlockTime
		transaction.EvmHash = transaction_data.EvmHash
		if transaction.EvmHash == "" {
			for _, result := range blockscout_data {
				if transaction.CosmosHash == result.CosmosHash {
					transaction.EvmHash = result.TxHash
				}
			}
		}
		transactions = append(transactions, transaction)
	}
	return transactions
}

func (search *Search) isResultsEmpty(results SearchResults) bool {
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
	if results.Contracts != nil {
		return false
	}
	return true
}
