package handlers

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	utils "github.com/AstraProtocol/astra-indexing/infrastructure"

	"github.com/AstraProtocol/astra-indexing/appinterface/pagination"
	"github.com/AstraProtocol/astra-indexing/external/cache"
	transactionView "github.com/AstraProtocol/astra-indexing/projection/transaction/view"

	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	blockscout_infrastructure "github.com/AstraProtocol/astra-indexing/infrastructure/blockscout"
	"github.com/AstraProtocol/astra-indexing/infrastructure/metric/prometheus"
	"github.com/valyala/fasthttp"

	"github.com/AstraProtocol/astra-indexing/appinterface/projection/view"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/infrastructure/httpapi"
	evm_utils "github.com/AstraProtocol/astra-indexing/internal/evm"
)

type Transactions struct {
	logger           applogger.Logger
	transactionsView transactionView.BlockTransactions
	blockscoutClient blockscout_infrastructure.HTTPClient
	astraCache       *cache.AstraCache
	astraLocalCache  *cache.AstraLocalCache
}

type TransactionsPaginationResult struct {
	TransactionRows  []transactionView.TransactionRow `json:"transactionRows"`
	PaginationResult pagination.Result                `json:"paginationResult"`
}

func NewTransactionsPaginationResult(transactionRows []transactionView.TransactionRow,
	paginationResult pagination.Result) *TransactionsPaginationResult {
	return &TransactionsPaginationResult{
		transactionRows,
		paginationResult,
	}
}

func NewTransactions(
	logger applogger.Logger,
	blockscoutClient blockscout_infrastructure.HTTPClient,
	rdbHandle *rdb.Handle) *Transactions {
	return &Transactions{
		logger.WithFields(applogger.LogFields{
			"module": "TransactionsHandler",
		}),
		transactionView.NewTransactionsView(rdbHandle),
		blockscoutClient,
		cache.NewCache(),
		cache.NewLocalCache("TransactionsCache"),
	}
}

func (handler *Transactions) FindByHash(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "FindTransactionByHash"
	hashParam, hashParamOk := URLValueGuard(ctx, handler.logger, "hash")
	if !hashParamOk {
		handler.logger.Errorf("invalid %s params", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid tx hash"))
		return
	}
	// handle if blockscout is disconnnected
	if string(ctx.QueryArgs().Peek("type")) == "evm" {
		if evm_utils.IsHexTx(hashParam) {
			transaction, err := handler.blockscoutClient.GetDetailEvmTxByEvmTxHash(hashParam)
			if err != nil {
				handler.logger.Errorf("error fetching detail tx by evm tx hash from blockscout: %v", err)
				ctx.QueryArgs().Del("type")
				handler.FindByHash(ctx)
				return
			}
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
			httpapi.Success(ctx, transaction)
			return
		} else {
			transaction, err := handler.blockscoutClient.GetDetailEvmTxByCosmosTxHash(hashParam)
			if err != nil {
				handler.logger.Errorf("error fetching detail tx by cosmos tx hash from blockscout: %v", err)
				ctx.QueryArgs().Del("type")
				handler.FindByHash(ctx)
				return
			}
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
			httpapi.Success(ctx, transaction)
			return
		}
	} else {
		cacheKey := fmt.Sprintf("FindByTxCosmosHash%s", hashParam)
		var transactionRow transactionView.TransactionRow
		err := handler.astraCache.Get(cacheKey, &transactionRow)
		if err == nil {
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
			httpapi.Success(ctx, transactionRow)
			return
		}
		if evm_utils.IsHexTx(hashParam) {
			transaction, err := handler.transactionsView.FindByEvmHash(hashParam)
			if err != nil {
				if errors.Is(err, rdb.ErrNoRows) {
					handler.logger.Errorf("tx not found: %s", hashParam)
					prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusNotFound), "GET", time.Since(startTime).Milliseconds())
					httpapi.NotFound(ctx)
					return
				}
				handler.logger.Errorf("error finding transactions by hash: %v", err)
				prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusNotFound), "GET", time.Since(startTime).Milliseconds())
				httpapi.NotFound(ctx)
				return
			}
			if transaction.Success {
				transaction.Status = "Indexing"
			}
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
			handler.astraCache.Set(cacheKey, transaction, utils.TIME_CACHE_MEDIUM)
			httpapi.Success(ctx, transaction)
			return
		} else {
			transaction, err := handler.transactionsView.FindByHash(hashParam)
			if err != nil {
				if errors.Is(err, rdb.ErrNoRows) {
					handler.logger.Errorf("tx not found: %s", hashParam)
					prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusNotFound), "GET", time.Since(startTime).Milliseconds())
					httpapi.NotFound(ctx)
					return
				}
				handler.logger.Errorf("error finding transactions by hash: %v", err)
				prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusNotFound), "GET", time.Since(startTime).Milliseconds())
				httpapi.NotFound(ctx)
				return
			}
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
			handler.astraCache.Set(cacheKey, transaction, utils.TIME_CACHE_MEDIUM)
			httpapi.Success(ctx, transaction)
			return
		}
	}
}

func (handler *Transactions) ListInternalTransactionsByHash(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "ListInternalTransactionsByHash"

	hashParam, hashParamOk := URLValueGuard(ctx, handler.logger, "hash")
	if !hashParamOk {
		handler.logger.Errorf("invalid %s params", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid tx hash"))
		return
	}

	if evm_utils.IsHexTx(hashParam) {
		internalTransactions, err := handler.blockscoutClient.GetListInternalTxs(hashParam)
		if err != nil {
			handler.logger.Errorf("error finding list internal transactions by hash from blockscout: %v", err)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, err)
			return
		}
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.Success(ctx, internalTransactions)
		return
	} else {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid evm tx hash"))
		return
	}
}

func (handler *Transactions) ListInternalTransactionsByHashv2(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "ListInternalTransactionsByHashv2"
	// handle api's params
	var err error
	var page int64
	var offset int64
	page = blockscout_infrastructure.DEFAULT_PAGE
	offset = blockscout_infrastructure.DEFAULT_OFFSET * 5

	queryParams := make([]string, 0)
	mappingParams := make(map[string]string)

	addressHash, accountParamOk := URLValueGuard(ctx, handler.logger, "hash")
	if !accountParamOk {
		handler.logger.Errorf("invalid %s params", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid hash param"))
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

	tokensAddressResp, err := handler.blockscoutClient.GetListInternalTxsByTxHash(addressHash, queryParams, mappingParams)
	if err != nil {
		handler.logger.Errorf("error fetching list internal txs by tx from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, err)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, tokensAddressResp)
}

func (handler *Transactions) List(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "ListTransactions"

	paginationInput, err := httpapi.ParsePagination(ctx)
	if err != nil {
		handler.logger.Errorf("invalid %s params", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	// limited page and limit number
	if paginationInput.OffsetParams().Page > 2500 {
		paginationInput.OffsetParams().Page = 2500
	}
	if paginationInput.OffsetParams().Limit > 20 {
		paginationInput.OffsetParams().Limit = 20
	}

	heightOrder := view.ORDER_ASC
	queryArgs := ctx.QueryArgs()
	if queryArgs.Has("order") {
		if string(queryArgs.Peek("order")) == "height.desc" {
			heightOrder = view.ORDER_DESC
		}
	}

	transactionPaginationKey := getKeyPagination(paginationInput, heightOrder)
	tmpTransactions := TransactionsPaginationResult{}
	err = handler.astraLocalCache.Get(transactionPaginationKey, &tmpTransactions)
	if err == nil {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.SuccessWithPagination(ctx, tmpTransactions.TransactionRows, &tmpTransactions.PaginationResult)
		return
	}
	txs, paginationResult, err := handler.transactionsView.List(transactionView.TransactionsListFilter{
		MaybeBlockHeight: nil,
	}, transactionView.TransactionsListOrder{
		Height: heightOrder,
	}, paginationInput)
	if err != nil {
		handler.logger.Errorf("error listing transactions: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusInternalServerError), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	if paginationResult.Por.TotalRecord > pagination.MAX_ELEMENTS {
		paginationResult.Por.TotalRecord = pagination.MAX_ELEMENTS
		paginationResult.Por.TotalPage()
	}

	handler.astraLocalCache.Set(transactionPaginationKey,
		NewTransactionsPaginationResult(txs, *paginationResult), utils.TIME_CACHE_FAST)

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.SuccessWithPagination(ctx, txs, paginationResult)
}

func (handler *Transactions) GetAbiByTransactionHash(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetAbiByTransactionHash"
	txParam, txParamOk := URLValueGuard(ctx, handler.logger, "hash")
	if !txParamOk {
		handler.logger.Errorf("invalid %s params", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid tx hash"))
		return
	}

	abi, err := handler.blockscoutClient.GetAbiByTransactionHash(txParam)
	if err != nil {
		handler.logger.Errorf("error fetching abi by tx hash from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusNotFound), "GET", time.Since(startTime).Milliseconds())
		httpapi.NotFound(ctx)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, abi)
}

func (handler *Transactions) GetRawTraceByTransactionHash(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetRawTraceByTransactionHash"
	txParam, txParamOk := URLValueGuard(ctx, handler.logger, "hash")
	if !txParamOk {
		handler.logger.Errorf("invalid %s params", recordMethod)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid tx hash"))
		return
	}

	rawTrace, err := handler.blockscoutClient.GetRawTraceByTxHash(txParam)
	if err != nil {
		handler.logger.Errorf("error fetching raw trace by tx hash from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, err)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, rawTrace)
}
