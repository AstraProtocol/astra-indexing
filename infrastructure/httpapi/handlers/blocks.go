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
}

func NewBlocks(logger applogger.Logger, rdbHandle *rdb.Handle) *Blocks {
	return &Blocks{
		logger.WithFields(applogger.LogFields{
			"module": "BlocksHandler",
		}),

		block_view.NewBlocks(rdbHandle),
		transaction_view.NewTransactionsView(rdbHandle),
		blockevent_view.NewBlockEvents(rdbHandle),
		validator_view.NewValidatorBlockCommitments(rdbHandle),
		cache.NewCache("blocks"),
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
	if err == nil {
		identity.MaybeHeight = &height
	} else {
		identity.MaybeHash = &heightOrHashParam
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

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, block)
}

func getKeyPagination(pagination *pagination.Pagination, heightOrder view.ORDER) string {
	return fmt.Sprintf("pagination_%d_%d_%s", pagination.OffsetParams().Page, pagination.OffsetParams().Limit, heightOrder)
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

	_ = handler.astraCache.Set(blockPaginationKey,
		NewBlocksPaginationResult(blocks, *paginationResult), 2400*time.Millisecond)

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.SuccessWithPagination(ctx, blocks, paginationResult)
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

	blocks, paginationResult, err := handler.transactionsView.List(transaction_view.TransactionsListFilter{
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

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.SuccessWithPagination(ctx, blocks, paginationResult)
}

func (handler *Blocks) ListEventsByHeight(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "ListEventsByHeight"
	paginationInput, err := httpapi.ParsePagination(ctx)
	if err != nil {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, err)
		return
	}

	heightOrder := view.ORDER_ASC
	queryArgs := ctx.QueryArgs()
	if queryArgs.Has("order") {
		if string(queryArgs.Peek("order")) == "height.desc" {
			heightOrder = view.ORDER_DESC
		}
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

	blocks, paginationResult, err := handler.blockEventsView.List(blockevent_view.BlockEventsListFilter{
		MaybeBlockHeight: &blockHeight,
	}, blockevent_view.BlockEventsListOrder{
		Height: heightOrder,
	}, paginationInput)
	if err != nil {
		handler.logger.Errorf("error listing events: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.SuccessWithPagination(ctx, blocks, paginationResult)
}

func (handler *Blocks) ListCommitmentsByHeight(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "ListCommitmentsByHeight"
	paginationInput, err := httpapi.ParsePagination(ctx)
	if err != nil {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
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

	blocks, paginationResult, err := handler.validatorBlockCommitmentsView.List(
		validator_view.ValidatorBlockCommitmentsListFilter{
			MaybeBlockHeight: &blockHeight,
		}, paginationInput)
	if err != nil {
		handler.logger.Errorf("error listing block commitments: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.SuccessWithPagination(ctx, blocks, paginationResult)
}

func (handler *Blocks) ListCommitmentsByConsensusNodeAddress(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "ListCommitmentsByConsensusNodeAddress"
	paginationInput, err := httpapi.ParsePagination(ctx)
	if err != nil {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, err)
		return
	}

	addressParam := ctx.UserValue("address")
	if addressParam == nil {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("missing consensus node address"))
		return
	}
	address := addressParam.(string)

	blocks, paginationResult, err := handler.validatorBlockCommitmentsView.List(
		validator_view.ValidatorBlockCommitmentsListFilter{
			MaybeConsensusNodeAddress: &address,
		}, paginationInput)
	if err != nil {
		handler.logger.Errorf("error listing block commitments: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.SuccessWithPagination(ctx, blocks, paginationResult)
}
