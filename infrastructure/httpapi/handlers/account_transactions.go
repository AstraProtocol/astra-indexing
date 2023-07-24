package handlers

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/AstraProtocol/astra-indexing/external/cache"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	"github.com/AstraProtocol/astra-indexing/external/tmcosmosutils"
	"github.com/AstraProtocol/astra-indexing/external/utctime"
	utils "github.com/AstraProtocol/astra-indexing/infrastructure"
	evm_utils "github.com/AstraProtocol/astra-indexing/internal/evm"
	"github.com/AstraProtocol/astra-indexing/usecase/coin"
	"github.com/AstraProtocol/astra-indexing/usecase/event"
	"github.com/AstraProtocol/astra-indexing/usecase/model"
	"github.com/valyala/fasthttp"

	"github.com/AstraProtocol/astra-indexing/appinterface/cosmosapp"
	pagination_interface "github.com/AstraProtocol/astra-indexing/appinterface/pagination"
	"github.com/AstraProtocol/astra-indexing/appinterface/projection/view"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	blockscout_infrastructure "github.com/AstraProtocol/astra-indexing/infrastructure/blockscout"
	"github.com/AstraProtocol/astra-indexing/infrastructure/httpapi"
	"github.com/AstraProtocol/astra-indexing/infrastructure/metric/prometheus"
	"github.com/AstraProtocol/astra-indexing/projection/account_transaction"
	account_transaction_view "github.com/AstraProtocol/astra-indexing/projection/account_transaction/view"
)

type AccountTransactions struct {
	logger applogger.Logger

	cosmosClient                 cosmosapp.Client
	blockscoutClient             blockscout_infrastructure.HTTPClient
	accountTransactionsView      *account_transaction_view.AccountTransactions
	accountTransactionDataView   *account_transaction_view.AccountTransactionData
	accountTransactionsTotalView *account_transaction_view.AccountTransactionsTotal
	accountGasUsedTotalView      *account_transaction_view.AccountGasUsedTotal
	accountFeesTotalView         *account_transaction_view.AccountFeesTotal

	astraCache *cache.AstraCache
	evmUtil    evm_utils.EvmUtils
}

func NewAccountTransactions(
	logger applogger.Logger,
	rdbHandle *rdb.Handle,
	cosmosClient cosmosapp.Client,
	blockscoutClient blockscout_infrastructure.HTTPClient,
	evmUtil evm_utils.EvmUtils,
) *AccountTransactions {
	return &AccountTransactions{
		logger.WithFields(applogger.LogFields{
			"module": "AccountTransactionsHandler",
		}),

		cosmosClient,
		blockscoutClient,
		account_transaction_view.NewAccountTransactions(rdbHandle),
		account_transaction_view.NewAccountTransactionData(rdbHandle),
		account_transaction_view.NewAccountTransactionsTotal(rdbHandle),
		account_transaction_view.NewAccountGasUsedTotal(rdbHandle),
		account_transaction_view.NewAccountFeesTotal(rdbHandle),
		cache.NewCache(),
		evmUtil,
	}
}

func (handler *AccountTransactions) GetCounters(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetCounters"
	accountParam, accountParamOk := URLValueGuard(ctx, handler.logger, "account")
	if !accountParamOk {
		handler.logger.Errorf("invalid %s params", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid account param"))
		return
	}

	addressCounterRespChan := make(chan blockscout_infrastructure.AddressCounterResp)
	var blockscoutSearchParam string
	var astraAddress string

	// Using simultaneously blockscout get address counters api
	if evm_utils.IsHexAddress(accountParam) {
		blockscoutSearchParam = accountParam
		converted, _ := hex.DecodeString(accountParam[2:])
		astraAddress, _ = tmcosmosutils.EncodeHexToAddress("astra", converted)
	} else {
		if tmcosmosutils.IsValidCosmosAddress(accountParam) {
			astraAddress = accountParam
			_, converted, _ := tmcosmosutils.DecodeAddressToHex(accountParam)
			blockscoutSearchParam = "0x" + hex.EncodeToString(converted)
		}
	}
	go handler.blockscoutClient.GetAddressCountersAsync(blockscoutSearchParam, addressCounterRespChan)

	numberOfTxs, err := handler.accountTransactionsTotalView.Total.FindBy(fmt.Sprintf("%s:-", astraAddress))

	blockscoutAddressCounterResp := <-addressCounterRespChan
	addressCounter := blockscoutAddressCounterResp.Result

	if err == nil && addressCounter.Type != "contractaddress" {
		addressCounter.TransactionCount = numberOfTxs
	}

	totalGasUsed, err := handler.accountGasUsedTotalView.Total.FindBy(strings.ToLower(blockscoutSearchParam))
	if err == nil && addressCounter.Type != "contractaddress" {
		addressCounter.GasUsageCount = totalGasUsed
	}

	/*
		totalFees, err := handler.accountFeesTotalView.Total.FindBy(strings.ToLower(blockscoutSearchParam))
		if err == nil {
			// Convert fees unit from microAstra to Astra
			fees := float64(totalFees) / 1000000
			addressCounter.FeesCount = fees
		}
	*/
	addressCounter.FeesCount = 0

	if addressCounter.Type == "" {
		addressCounter.Type = "address"
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, addressCounter)
}

func (handler *AccountTransactions) GetTopAddressesBalance(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetTopAddressesBalance"
	// handle api's params
	var err error
	var page int64
	var offset int64
	page = blockscout_infrastructure.DEFAULT_PAGE
	offset = blockscout_infrastructure.DEFAULT_OFFSET

	queryParams := make([]string, 0)
	mappingParams := make(map[string]string)

	if string(ctx.QueryArgs().Peek("blockscout")) != "true" {
		handler.logger.Errorf("invalid %s blockscout param", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid blockscout param"))
		return
	}

	if string(ctx.QueryArgs().Peek("page")) != "" {
		page, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("page")), 10, 0)
		if err != nil || page <= 0 {
			handler.logger.Errorf("invalid %s page param", recordMethod)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, errors.New("invalid page param"))
			return
		}
	}
	queryParams = append(queryParams, "page")
	mappingParams["page"] = strconv.FormatInt(page, 10)

	if string(ctx.QueryArgs().Peek("offset")) != "" {
		offset, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("offset")), 10, 0)
		if err != nil || offset <= 0 {
			handler.logger.Errorf("invalid %s offset param", recordMethod)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, errors.New("invalid offset param"))
			return
		}
	}
	queryParams = append(queryParams, "offset")
	mappingParams["offset"] = strconv.FormatInt(offset, 10)

	if string(ctx.QueryArgs().Peek("fetched_coin_balance")) != "" {
		queryParams = append(queryParams, "fetched_coin_balance")
		mappingParams["fetched_coin_balance"] = string(ctx.QueryArgs().Peek("fetched_coin_balance"))
	}

	if string(ctx.QueryArgs().Peek("hash")) != "" {
		queryParams = append(queryParams, "hash")
		mappingParams["hash"] = string(ctx.QueryArgs().Peek("hash"))
	}
	//

	topAddressesBalanceResp, err := handler.blockscoutClient.GetTopAddressesBalance(queryParams, mappingParams)
	if err != nil {
		handler.logger.Errorf("error fetching top addresses balance from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, err)
		return
	}

	identities := make([]string, 0)
	for _, topAddressesBalanceResult := range topAddressesBalanceResp.Result {
		if evm_utils.IsHexAddress(topAddressesBalanceResult.Address) {
			converted, _ := hex.DecodeString(topAddressesBalanceResult.Address[2:])
			astraAddress, _ := tmcosmosutils.EncodeHexToAddress("astra", converted)
			identities = append(identities, astraAddress+":-")
		}
	}

	mappingAddressTotal, err := handler.accountTransactionsTotalView.Total.FindByList(identities)
	if err != nil {
		handler.logger.Errorf("addresses not found: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusNotFound), "GET", time.Since(startTime).Milliseconds())
		httpapi.NotFound(ctx)
		return
	}

	for index, topAddressesBalanceResult := range topAddressesBalanceResp.Result {
		if evm_utils.IsHexAddress(topAddressesBalanceResult.Address) {
			converted, _ := hex.DecodeString(topAddressesBalanceResult.Address[2:])
			astraAddress, _ := tmcosmosutils.EncodeHexToAddress("astra", converted)
			topAddressesBalanceResp.Result[index].TxnCount = mappingAddressTotal[astraAddress+":-"]
		}
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, topAddressesBalanceResp)
}

func (handler *AccountTransactions) ListByAccount(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "ListTxsByAccount"

	pagination, err := httpapi.ParsePagination(ctx)
	if err != nil {
		handler.logger.Errorf("invalid %s params", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	account, accountOk := URLValueGuard(ctx, handler.logger, "account")
	if !accountOk {
		handler.logger.Errorf("invalid %s account param", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid account param"))
		return
	}

	if evm_utils.IsHexAddress(account) {
		converted, err := hex.DecodeString(account[2:])
		if err != nil {
			handler.logger.Errorf("%s: error convert %s to bytes", recordMethod, account)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			ctx.SetStatusCode(fasthttp.StatusBadRequest)
			return
		}
		account, err = tmcosmosutils.EncodeHexToAddress("astra", converted)
		if err != nil {
			handler.logger.Errorf("%s: error encode hex address %s to astra address", recordMethod, account)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			ctx.SetStatusCode(fasthttp.StatusBadRequest)
			return
		}
	}

	queryArgs := ctx.QueryArgs()

	idOrder := view.ORDER_ASC
	if queryArgs.Has("order") {
		if string(queryArgs.Peek("order")) == "height.desc" {
			idOrder = view.ORDER_DESC
		}
	}

	memo := ""
	if queryArgs.Has("memo") {
		memo = string(queryArgs.Peek("memo"))
	}

	includingInternalTx := ""
	if queryArgs.Has("includingInternalTx") {
		includingInternalTx = string(queryArgs.Peek("includingInternalTx"))
	}

	txType := ""
	if queryArgs.Has("txType") {
		txType = string(queryArgs.Peek("txType"))
	}

	filter := account_transaction_view.AccountTransactionsListFilter{
		Account:             account,
		Memo:                memo,
		IncludingInternalTx: includingInternalTx,
		TxType:              txType,
	}

	order := account_transaction_view.AccountTransactionsListOrder{
		Id: idOrder,
	}

	cacheKeyResult := fmt.Sprintf(
		"ListByAccountResult%s%s%s%s%s%d%d",
		account,
		memo,
		includingInternalTx,
		txType,
		idOrder,
		pagination.OffsetParams().Page,
		pagination.OffsetParams().Limit,
	)
	cacheKeyPagination := fmt.Sprintf(
		"ListByAccountPagination%d%d",
		pagination.OffsetParams().Page,
		pagination.OffsetParams().Limit,
	)
	var resultCache interface{}
	var paginationCache *pagination_interface.Result

	err = handler.astraCache.Get(cacheKeyResult, &resultCache)
	if err == nil {
		err = handler.astraCache.Get(cacheKeyPagination, &paginationCache)
		if err == nil {
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
			httpapi.SuccessWithPagination(ctx, resultCache, paginationCache)
			return
		}
	}

	blocks, paginationResult, err := handler.accountTransactionsView.List(
		filter, order, pagination,
	)
	if err != nil {
		handler.logger.Errorf("error listing account transactions: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusInternalServerError), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, err)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	handler.astraCache.Set(cacheKeyResult, blocks, utils.TIME_CACHE_FAST)
	handler.astraCache.Set(cacheKeyPagination, paginationResult, utils.TIME_CACHE_FAST)
	httpapi.SuccessWithPagination(ctx, blocks, paginationResult)
}

func (handler *AccountTransactions) GetInternalTxsByAddressHash(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetInternalTxsByAddressHash"
	// handle api's params
	var err error
	var page int64
	var offset int64
	page = blockscout_infrastructure.DEFAULT_PAGE
	offset = blockscout_infrastructure.DEFAULT_OFFSET

	queryParams := make([]string, 0)
	mappingParams := make(map[string]string)

	addressHash, accountParamOk := URLValueGuard(ctx, handler.logger, "account")
	if !accountParamOk {
		handler.logger.Errorf("invalid %s params", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid account param"))
		return
	}

	if string(ctx.QueryArgs().Peek("blockscout")) != "true" {
		handler.logger.Errorf("invalid %s blockscout param", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid blockscout param"))
		return
	}

	if string(ctx.QueryArgs().Peek("page")) != "" {
		page, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("page")), 10, 0)
		if err != nil || page <= 0 {
			handler.logger.Errorf("invalid %s page param", recordMethod)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, errors.New("invalid page param"))
			return
		}
	}
	queryParams = append(queryParams, "page")
	mappingParams["page"] = strconv.FormatInt(page, 10)

	if string(ctx.QueryArgs().Peek("offset")) != "" {
		offset, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("offset")), 10, 0)
		if err != nil || offset <= 0 {
			handler.logger.Errorf("invalid %s offset param", recordMethod)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, errors.New("invalid offset param"))
			return
		}
	}
	queryParams = append(queryParams, "offset")
	mappingParams["offset"] = strconv.FormatInt(offset, 10)

	if string(ctx.QueryArgs().Peek("block_number")) != "" {
		queryParams = append(queryParams, "block_number")
		mappingParams["block_number"] = string(ctx.QueryArgs().Peek("block_number"))
	}

	if string(ctx.QueryArgs().Peek("index")) != "" {
		queryParams = append(queryParams, "index")
		mappingParams["index"] = string(ctx.QueryArgs().Peek("index"))
	}

	if string(ctx.QueryArgs().Peek("transaction_index")) != "" {
		queryParams = append(queryParams, "transaction_index")
		mappingParams["transaction_index"] = string(ctx.QueryArgs().Peek("transaction_index"))
	}
	//

	tokensAddressResp, err := handler.blockscoutClient.GetListInternalTxsByAddressHash(addressHash, queryParams, mappingParams)
	if err != nil {
		handler.logger.Errorf("error fetching list internal txs by address from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, err)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, tokensAddressResp)
}

func (handler *AccountTransactions) SyncAccountInternalTxsByTxHash(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "SyncAccountInternalTxsByTxHash"

	txHash, txHashParamOk := URLValueGuard(ctx, handler.logger, "txhash")
	if !txHashParamOk {
		handler.logger.Errorf("invalid %s params", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid txhash param"))
		return
	}

	internalTxs, err := handler.blockscoutClient.GetListInternalTxs(txHash)
	if err != nil {
		handler.logger.Errorf("error sync account internal txs by tx hash: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, err)
		return
	}

	//sync internal txs
	rewardType := map[string]bool{
		"sendReward":        true,
		"redeemReward":      true,
		"exchange":          true,
		"exchangeWithValue": true,
	}
	accountTransactionRows := make([]account_transaction_view.AccountTransactionBaseRow, 0)
	txs := make([]account_transaction_view.TransactionRow, 0)
	fee := coin.MustNewCoins(coin.MustNewCoinFromString("aastra", "0"))
	evmType := handler.evmUtil.GetMethodNameFromMethodId(internalTxs[0].Input[2:10])
	for _, internalTx := range internalTxs {
		if internalTx.CallType != "call" {
			continue
		}
		if internalTx.Value == "0" {
			continue
		}
		if internalTx.From == "" || internalTx.To == "" {
			continue
		}
		//ignore if internal tx is not reward tx
		if !rewardType[evmType] {
			continue
		}
		//ignore if internal tx is same data with parent tx
		//blockscout approach
		if internalTx.Index == "0" {
			continue
		}

		blockNumber, _ := strconv.ParseInt(internalTx.BlockNumber, 10, 64)
		index, _ := strconv.Atoi(internalTx.Index)
		transactionInfo := account_transaction.NewTransactionInfo(
			account_transaction_view.AccountTransactionBaseRow{
				Account:      "",
				BlockHeight:  blockNumber,
				BlockHash:    "",
				BlockTime:    utctime.UTCTime{},
				Hash:         internalTx.TransactionHash,
				MessageTypes: []string{},
				Success:      true,
			},
		)
		converted, _ := hex.DecodeString(internalTx.From[2:])
		fromAstraAddr, _ := tmcosmosutils.EncodeHexToAddress("astra", converted)

		converted, _ = hex.DecodeString(internalTx.To[2:])
		toAstraAddr, _ := tmcosmosutils.EncodeHexToAddress("astra", converted)

		transactionInfo.AddAccount(fromAstraAddr)
		transactionInfo.AddAccount(toAstraAddr)

		transactionInfo.Row.FromAddress = strings.ToLower(internalTx.From)
		transactionInfo.Row.ToAddress = strings.ToLower(internalTx.To)

		transactionInfo.AddMessageTypes(event.MSG_ETHEREUM_TX)

		blockHash := ""
		timeStamp, _ := strconv.ParseInt(internalTx.TimeStamp, 10, 64)
		blockTime := utctime.FromUnixNano(time.Unix(timeStamp, 0).UnixNano())
		gasWanted, _ := strconv.Atoi(internalTx.Gas)
		gasUsed, _ := strconv.Atoi(internalTx.GasUsed)

		transactionInfo.FillBlockInfo(blockHash, blockTime)

		//parse internal tx message content
		legacyTx := model.LegacyTx{
			Type:  internalTx.CallType,
			Gas:   internalTx.GasUsed,
			To:    internalTx.To,
			Value: internalTx.Value,
			Data:  internalTx.Input,
		}
		rawMsgEthereumTx := model.RawMsgEthereumTx{
			Type: event.MSG_ETHEREUM_INTERNAL_TX,
			Size: 0,
			From: internalTx.From,
			Hash: internalTx.TransactionHash,
			Data: legacyTx,
		}
		params := model.MsgEthereumTxParams{
			RawMsgEthereumTx: rawMsgEthereumTx,
		}
		evmEvent := event.NewMsgEthereumTx(event.MsgCommonParams{
			BlockHeight: blockNumber,
			TxHash:      internalTx.TransactionHash,
			TxSuccess:   true,
			MsgIndex:    index,
		}, params)
		tmpMessage := account_transaction_view.TransactionRowMessage{
			Type:    event.MSG_ETHEREUM_TX,
			EvmType: evmType,
			Content: evmEvent,
		}

		tx := account_transaction_view.TransactionRow{
			BlockHeight:   blockNumber,
			BlockTime:     blockTime,
			BlockHash:     blockHash,
			Hash:          internalTx.TransactionHash,
			Index:         index,
			Success:       true,
			Code:          0,
			Log:           "",
			Fee:           fee,
			FeePayer:      "",
			FeeGranter:    "",
			GasWanted:     gasWanted,
			GasUsed:       gasUsed,
			Memo:          "",
			TimeoutHeight: 0,
			Messages:      make([]account_transaction_view.TransactionRowMessage, 0),
			EvmHash:       internalTx.TransactionHash,
			RewardTxType:  evmType,
			FromAddress:   strings.ToLower(internalTx.From),
			ToAddress:     strings.ToLower(internalTx.To),
		}
		tx.Messages = append(tx.Messages, tmpMessage)
		txs = append(txs, tx)
		accountTransactionRows = append(accountTransactionRows, transactionInfo.ToRowsIncludingInternalTx()...)
	}
	err = handler.accountTransactionsView.InsertAll(accountTransactionRows)
	if err == nil {
		err = handler.accountTransactionDataView.InsertAll(txs)
		if err != nil {
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, err)
			return
		}
	} else {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, err)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.SuccessNotWrappedResult(ctx, txs)
}

func (handler *AccountTransactions) GetListTokenTransfersByAddressHash(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetListTokenTransfersByAddressHash"
	// handle api's params
	var err error
	var page int64
	var offset int64
	page = blockscout_infrastructure.DEFAULT_PAGE
	offset = blockscout_infrastructure.DEFAULT_OFFSET

	queryParams := make([]string, 0)
	mappingParams := make(map[string]string)

	addressHash, accountParamOk := URLValueGuard(ctx, handler.logger, "account")
	if !accountParamOk {
		handler.logger.Errorf("invalid %s params", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid account param"))
		return
	}

	if string(ctx.QueryArgs().Peek("blockscout")) != "true" {
		handler.logger.Errorf("invalid %s blockscout param", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid blockscout param"))
		return
	}

	if string(ctx.QueryArgs().Peek("page")) != "" {
		page, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("page")), 10, 0)
		if err != nil || page <= 0 {
			handler.logger.Errorf("invalid %s page param", recordMethod)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, errors.New("invalid page param"))
			return
		}
	}
	queryParams = append(queryParams, "page")
	mappingParams["page"] = strconv.FormatInt(page, 10)

	if string(ctx.QueryArgs().Peek("offset")) != "" {
		offset, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("offset")), 10, 0)
		if err != nil || offset <= 0 {
			handler.logger.Errorf("invalid %s offset param", recordMethod)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, errors.New("invalid offset param"))
			return
		}
	}
	queryParams = append(queryParams, "offset")
	mappingParams["offset"] = strconv.FormatInt(offset, 10)

	if string(ctx.QueryArgs().Peek("block_number")) != "" {
		queryParams = append(queryParams, "block_number")
		mappingParams["block_number"] = string(ctx.QueryArgs().Peek("block_number"))
	}

	if string(ctx.QueryArgs().Peek("index")) != "" {
		queryParams = append(queryParams, "index")
		mappingParams["index"] = string(ctx.QueryArgs().Peek("index"))
	}
	//

	tokensAddressResp, err := handler.blockscoutClient.GetListTokenTransfersByAddressHash(addressHash, queryParams, mappingParams)
	if err != nil {
		handler.logger.Errorf("error fetching list token transfers by address: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, err)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, tokensAddressResp)
}
