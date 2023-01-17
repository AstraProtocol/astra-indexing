package handlers

import (
	"errors"
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
	hashParam, hashParamOk := URLValueGuard(ctx, handler.logger, "hash")
	if !hashParamOk {
		return
	}
	if string(ctx.QueryArgs().Peek("type")) == "evm" {
		transactionEvmRespChan := make(chan blockscout_infrastructure.TxResp)
		if evm_utils.IsHexTx(hashParam) {
			go handler.blockscoutClient.GetDetailEvmTxByEvmTxHashAsync(hashParam, transactionEvmRespChan)
			transaction, err := handler.transactionsView.FindByEvmHash(hashParam)
			transactionEvmResp := <-transactionEvmRespChan
			if transactionEvmResp.Status == "0" || transactionEvmResp.Status == "" {
				httpapi.Success(ctx, transaction)
				return
			}

			if err != nil {
				if errors.Is(err, rdb.ErrNoRows) {
					httpapi.NotFound(ctx)
					return
				}
				handler.logger.Errorf("error finding transactions by hash: %v", err)
				httpapi.InternalServerError(ctx)
				return
			}

			transactionEvmResp.Result.TransactionFee = transaction.Fee.AmountOf("aastra").BigInt().String()
			httpapi.Success(ctx, transactionEvmResp.Result)
			return
		} else {
			go handler.blockscoutClient.GetDetailEvmTxByCosmosTxHashAsync(hashParam, transactionEvmRespChan)
			transaction, err := handler.transactionsView.FindByHash(hashParam)
			transactionEvmResp := <-transactionEvmRespChan
			if transactionEvmResp.Status == "0" || transactionEvmResp.Status == "" {
				httpapi.Success(ctx, transaction)
				return
			}

			if err != nil {
				if errors.Is(err, rdb.ErrNoRows) {
					httpapi.NotFound(ctx)
					return
				}
				handler.logger.Errorf("error finding transactions by hash: %v", err)
				httpapi.InternalServerError(ctx)
				return
			}

			transactionEvmResp.Result.TransactionFee = transaction.Fee.AmountOf("aastra").BigInt().String()
			httpapi.Success(ctx, transactionEvmResp.Result)
			return
		}
	} else {
		if evm_utils.IsHexTx(hashParam) {
			transaction, err := handler.transactionsView.FindByEvmHash(hashParam)
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
		}
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

	if paginationResult.Por.TotalRecord > pagination.MAX_ELEMENTS {
		paginationResult.Por.TotalRecord = pagination.MAX_ELEMENTS
		paginationResult.Por.TotalPage()
	}

	_ = handler.astraCache.Set(transactionPaginationKey,
		NewTransactionsPaginationResult(blocks, *paginationResult), 2400*time.Millisecond)

	httpapi.SuccessWithPagination(ctx, blocks, paginationResult)
}
