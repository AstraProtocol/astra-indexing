package handlers

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/AstraProtocol/astra-indexing/appinterface/pagination"
	"github.com/AstraProtocol/astra-indexing/external/cache"
	transactionView "github.com/AstraProtocol/astra-indexing/projection/transaction/view"

	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	blockscout_infrastructure "github.com/AstraProtocol/astra-indexing/infrastructure/blockscout"
	"github.com/valyala/fasthttp"

	"github.com/AstraProtocol/astra-indexing/appinterface/projection/view"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/infrastructure/httpapi"
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
	hashParam, hashParamOk := URLValueGuard(ctx, handler.logger, "hash")
	if !hashParamOk {
		return
	}
	if string(ctx.QueryArgs().Peek("type")) == "evm" {
		transaction, err := handler.blockscoutClient.GetDetailEvmTx(hashParam)
		if err != nil {
			if strings.Contains(fmt.Sprint(err), "transaction not found") {
				transaction, err := handler.transactionsView.FindByHash(hashParam)
				if err != nil {
					if errors.Is(err, rdb.ErrNoRows) {
						httpapi.NotFound(ctx)
						return
					}
					handler.logger.Errorf("error finding transactions by hash: %v", err)
					httpapi.InternalServerError(ctx)
					return
				}
				httpapi.Success(ctx, transaction)
				return
			} else {
				handler.logger.Errorf("error parsing tx response from blockscout: %v", err)
				httpapi.InternalServerError(ctx)
				return
			}
		}
		httpapi.Success(ctx, transaction)
	} else {
		transaction, err := handler.transactionsView.FindByHash(hashParam)
		if err != nil {
			if errors.Is(err, rdb.ErrNoRows) {
				httpapi.NotFound(ctx)
				return
			}
			handler.logger.Errorf("error finding transactions by hash: %v", err)
			httpapi.InternalServerError(ctx)
			return
		}
		httpapi.Success(ctx, transaction)
	}
}

func (handler *Transactions) List(ctx *fasthttp.RequestCtx) {
	paginationInput, err := httpapi.ParsePagination(ctx)
	if err != nil {
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
		httpapi.InternalServerError(ctx)
		return
	}
	_ = handler.astraCache.Set(transactionPaginationKey,
		NewTransactionsPaginationResult(blocks, *paginationResult), 2400*time.Millisecond)
	httpapi.SuccessWithPagination(ctx, blocks, paginationResult)
}
