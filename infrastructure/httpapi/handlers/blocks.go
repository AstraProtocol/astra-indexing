package handlers

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/AstraProtocol/astra-indexing/appinterface/pagination"
	"github.com/AstraProtocol/astra-indexing/appinterface/projection/view"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/external/cache"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	utils "github.com/AstraProtocol/astra-indexing/infrastructure"
	blockscout_infrastructure "github.com/AstraProtocol/astra-indexing/infrastructure/blockscout"
	"github.com/AstraProtocol/astra-indexing/infrastructure/httpapi"
	"github.com/AstraProtocol/astra-indexing/infrastructure/metric/prometheus"
	block_view "github.com/AstraProtocol/astra-indexing/projection/block/view"
	blockevent_view "github.com/AstraProtocol/astra-indexing/projection/blockevent/view"
	transaction_view "github.com/AstraProtocol/astra-indexing/projection/transaction/view"
	validator_view "github.com/AstraProtocol/astra-indexing/projection/validator/view"
	"github.com/valyala/fasthttp"
)

type BlocksPaginationResult struct {
	Blocks           []block_view.Block `json:"blocks"`
	PaginationResult pagination.Result  `json:"paginationResult"`
}

func NewBlocksPaginationResult(blocks []block_view.Block,
	paginationResult pagination.Result) *BlocksPaginationResult {
	return &BlocksPaginationResult{
		blocks,
		paginationResult,
	}
}

type Blocks struct {
	logger applogger.Logger

	blocksView                    *block_view.Blocks
	transactionsView              transaction_view.BlockTransactions
	blockEventsView               *blockevent_view.BlockEvents
	validatorBlockCommitmentsView *validator_view.ValidatorBlockCommitments
	astraCache                    *cache.AstraCache
	blockscoutClient              blockscout_infrastructure.HTTPClient
}

func NewBlocks(logger applogger.Logger, rdbHandle *rdb.Handle, blockscoutClient blockscout_infrastructure.HTTPClient) *Blocks {
	return &Blocks{
		logger.WithFields(applogger.LogFields{
			"module": "BlocksHandler",
		}),

		block_view.NewBlocks(rdbHandle),
		transaction_view.NewTransactionsView(rdbHandle),
		blockevent_view.NewBlockEvents(rdbHandle),
		validator_view.NewValidatorBlockCommitments(rdbHandle),
		cache.NewCache(),
		blockscoutClient,
	}
}

func (handler *Blocks) FindBy(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "FindByBlock"
	heightOrHashParam, heightOrHashParamOk := URLValueGuard(ctx, handler.logger, "height-or-hash")
	if !heightOrHashParamOk {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		return
	}

	height, err := strconv.ParseInt(heightOrHashParam, 10, 64)
	var identity block_view.BlockIdentity
	var cacheKey string
	var tmpBlock block_view.Block
	if err == nil {
		identity.MaybeHeight = &height
		cacheKey = fmt.Sprintf("block_%d", height)
	} else {
		identity.MaybeHash = &heightOrHashParam
		cacheKey = fmt.Sprintf("block_%s", heightOrHashParam)
	}

	err = handler.astraCache.Get(cacheKey, &tmpBlock)
	if err == nil {
		httpapi.Success(ctx, tmpBlock)
		return
	}
	block, err := handler.blocksView.FindBy(&identity)
	if err != nil {
		if errors.Is(err, rdb.ErrNoRows) {
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.NotFound(ctx)
			return
		}
		handler.logger.Errorf("error finding block by height or hash: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	_ = handler.astraCache.Set(cacheKey, block, utils.TIME_CACHE_MEDIUM)
	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, block)
}

func getKeyPagination(pagination *pagination.Pagination, heightOrder view.ORDER) string {
	return fmt.Sprintf("pagination_%d_%d_%s", pagination.OffsetParams().Page, pagination.OffsetParams().Limit, heightOrder)
}

func getKeyPaginationByHeight(pagination *pagination.Pagination, heightOrder view.ORDER, blockHeight int64) string {
	return fmt.Sprintf("%d_pagination_%d_%d_%s", blockHeight, pagination.OffsetParams().Page, pagination.OffsetParams().Limit, heightOrder)
}

func (handler *Blocks) List(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "ListBlocks"
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

	blockPaginationKey := getKeyPagination(paginationInput, heightOrder)
	tmpBlockPage := BlocksPaginationResult{}
	err = handler.astraCache.Get(blockPaginationKey, &tmpBlockPage)
	if err == nil {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.SuccessWithPagination(ctx, tmpBlockPage.Blocks, &tmpBlockPage.PaginationResult)
		return
	}
	blocks, paginationResult, err := handler.blocksView.List(block_view.BlocksListOrder{
		Height: heightOrder,
	}, paginationInput)
	if err != nil {
		handler.logger.Errorf("error listing blocks: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	if paginationResult.Por.TotalRecord > pagination.MAX_ELEMENTS {
		paginationResult.Por.TotalRecord = pagination.MAX_ELEMENTS
		paginationResult.Por.TotalPage()
	}

	_ = handler.astraCache.Set(blockPaginationKey, NewBlocksPaginationResult(blocks, *paginationResult), utils.TIME_CACHE_FAST)

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.SuccessWithPagination(ctx, blocks, paginationResult)
}

func (handler *Blocks) EthBlockNumber(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "EthBlockNumber"

	cacheKey := "EthBlockNumber"

	var tmpEthBlockNumber blockscout_infrastructure.EthBlockNumber

	err := handler.astraCache.Get(cacheKey, &tmpEthBlockNumber)
	if err == nil {
		httpapi.Success(ctx, tmpEthBlockNumber)
		return
	}

	ethBlockNumber, err := handler.blockscoutClient.EthBlockNumber()
	if err != nil {
		handler.logger.Errorf("error fetching eth block number: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}
	_ = handler.astraCache.Set(cacheKey, ethBlockNumber, utils.TIME_CACHE_FAST)
	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, ethBlockNumber)
}

func (handler *Blocks) ListTransactionsByHeight(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "ListTransactionsByHeight"
	paginationInput, err := httpapi.ParsePagination(ctx)
	if err != nil {
		httpapi.BadRequest(ctx, err)
		return
	}

	blockHeightParam, blockHeightParamOk := URLValueGuard(ctx, handler.logger, "height")
	if !blockHeightParamOk {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		return
	}
	blockHeight, err := strconv.ParseInt(blockHeightParam, 10, 64)
	if err != nil {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("invalid block height"))
		return
	}

	heightOrder := view.ORDER_ASC
	queryArgs := ctx.QueryArgs()
	if queryArgs.Has("order") {
		if string(queryArgs.Peek("order")) == "height.desc" {
			heightOrder = view.ORDER_DESC
		}
	}
	transactionPaginationKey := getKeyPaginationByHeight(paginationInput, heightOrder, blockHeight)
	tmpTransactions := TransactionsPaginationResult{}
	err = handler.astraCache.Get(transactionPaginationKey, &tmpTransactions)
	if err == nil {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.SuccessWithPagination(ctx, tmpTransactions.TransactionRows, &tmpTransactions.PaginationResult)
		return
	}

	txs, paginationResult, err := handler.transactionsView.List(transaction_view.TransactionsListFilter{
		MaybeBlockHeight: &blockHeight,
	}, transaction_view.TransactionsListOrder{
		Height: heightOrder,
	}, paginationInput)
	if err != nil {
		handler.logger.Errorf("error listing transactions: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	_ = handler.astraCache.Set(transactionPaginationKey,
		NewTransactionsPaginationResult(txs, *paginationResult), utils.TIME_CACHE_MEDIUM)

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.SuccessWithPagination(ctx, txs, paginationResult)
}
