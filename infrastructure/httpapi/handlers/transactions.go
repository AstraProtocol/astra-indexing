package handlers

import (
	"errors"
	"strconv"
	"time"

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
		cache.NewCache("transactions"),
	}
}

func (handler *Transactions) FindByHash(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "FindTransactionByHash"

	hashParam, hashParamOk := URLValueGuard(ctx, handler.logger, "hash")
	if !hashParamOk {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		return
	}
	// handle if blockscout is disconnnected
	if string(ctx.QueryArgs().Peek("type")) == "evm" {
		if evm_utils.IsHexTx(hashParam) {
			transaction, err := handler.blockscoutClient.GetDetailEvmTxByEvmTxHash(hashParam)
			if err != nil {
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
				ctx.QueryArgs().Del("type")
				handler.FindByHash(ctx)
				return
			}
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
			httpapi.Success(ctx, transaction)
			return
		}
	} else {
		if evm_utils.IsHexTx(hashParam) {
			transaction, err := handler.transactionsView.FindByEvmHash(hashParam)
			if err != nil {
				if errors.Is(err, rdb.ErrNoRows) {
					prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
					httpapi.NotFound(ctx)
					return
				}
				handler.logger.Errorf("error finding transactions by hash: %v", err)
				prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
				httpapi.InternalServerError(ctx)
				return
			}
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
			httpapi.Success(ctx, transaction)
			return
		} else {
			transaction, err := handler.transactionsView.FindByHash(hashParam)
			if err != nil {
				if errors.Is(err, rdb.ErrNoRows) {
					prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
					httpapi.NotFound(ctx)
					return
				}
				handler.logger.Errorf("error finding transactions by hash: %v", err)
				prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
				httpapi.InternalServerError(ctx)
				return
			}
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
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
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		return
	}

	if evm_utils.IsHexTx(hashParam) {
		internalTransactions, err := handler.blockscoutClient.GetListInternalTxs(hashParam)
		if err != nil {
			handler.logger.Errorf("error finding list internal transactions by hash: %v", err)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			return
		}
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.Success(ctx, internalTransactions)
		return
	} else {
		handler.logger.Errorf("param: %s is not evm tx hash", hashParam)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}
}

func (handler *Transactions) List(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "ListTransactions"

	paginationInput, err := httpapi.ParsePagination(ctx)
	if err != nil {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
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
	err = handler.astraCache.Get(transactionPaginationKey, &tmpTransactions)
	if err == nil {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.SuccessWithPagination(ctx, tmpTransactions.TransactionRows, &tmpTransactions.PaginationResult)
		return
	}
	blocks, paginationResult, err := handler.transactionsView.List(transactionView.TransactionsListFilter{
		MaybeBlockHeight: nil,
	}, transactionView.TransactionsListOrder{
		Height: heightOrder,
	}, paginationInput)
	if err != nil {
		handler.logger.Errorf("error listing transactions: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	if paginationResult.Por.TotalRecord > pagination.MAX_ELEMENTS {
		paginationResult.Por.TotalRecord = pagination.MAX_ELEMENTS
		paginationResult.Por.TotalPage()
	}

	_ = handler.astraCache.Set(transactionPaginationKey,
		NewTransactionsPaginationResult(blocks, *paginationResult), 2400*time.Millisecond)

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.SuccessWithPagination(ctx, blocks, paginationResult)
}
