package handlers

import (
	"errors"
	"fmt"
	"github.com/AstraProtocol/astra-indexing/appinterface/pagination"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	validator_view "github.com/AstraProtocol/astra-indexing/projection/validator/view"
	"github.com/jellydator/ttlcache/v3"
	"github.com/valyala/fasthttp"
	"strconv"
	"time"

	"github.com/AstraProtocol/astra-indexing/appinterface/projection/view"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/infrastructure/httpapi"
	block_view "github.com/AstraProtocol/astra-indexing/projection/block/view"
	blockevent_view "github.com/AstraProtocol/astra-indexing/projection/blockevent/view"
	transaction_view "github.com/AstraProtocol/astra-indexing/projection/transaction/view"
)

var (
	cachePage = ttlcache.New[string, BlocksPaginationResult](
		ttlcache.WithTTL[string, BlocksPaginationResult](2 * time.Second),
	)

	cacheDetail = ttlcache.New[string, *block_view.Block](
		ttlcache.WithTTL[string, *block_view.Block](1 * time.Minute),
	)
)

type BlocksPaginationResult struct {
	blocks           []block_view.Block
	paginationResult *pagination.PaginationResult
}

type Blocks struct {
	logger applogger.Logger

	blocksView                    *block_view.Blocks
	transactionsView              transaction_view.BlockTransactions
	blockEventsView               *blockevent_view.BlockEvents
	validatorBlockCommitmentsView *validator_view.ValidatorBlockCommitments
}

func NewBlocks(logger applogger.Logger, rdbHandle *rdb.Handle) *Blocks {
	go cacheDetail.Start()
	go cachePage.Start()
	return &Blocks{
		logger.WithFields(applogger.LogFields{
			"module": "BlocksHandler",
		}),

		block_view.NewBlocks(rdbHandle),
		transaction_view.NewTransactionsView(rdbHandle),
		blockevent_view.NewBlockEvents(rdbHandle),
		validator_view.NewValidatorBlockCommitments(rdbHandle),
	}
}

func (handler *Blocks) FindBy(ctx *fasthttp.RequestCtx) {
	heightOrHashParam, heightOrHashParamOk := URLValueGuard(ctx, handler.logger, "height-or-hash")
	if !heightOrHashParamOk {
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
			httpapi.NotFound(ctx)
			return
		}
		handler.logger.Errorf("error finding block by height or hash: %v", err)
		httpapi.InternalServerError(ctx)
		return
	}

	httpapi.Success(ctx, block)
}

func getKeyBlockPagination(pagination *pagination.Pagination, heightOrder view.ORDER) string {
	return fmt.Sprintf("%d_%d_%s", pagination.OffsetParams().Page, pagination.OffsetParams().Limit, heightOrder)
}

func (handler *Blocks) List(ctx *fasthttp.RequestCtx) {
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

	blockPaginationKey := getKeyBlockPagination(paginationInput, heightOrder)
	tmpBlockPage := cachePage.Get(blockPaginationKey)
	if tmpBlockPage != nil {
		tmpBlockPageValue := tmpBlockPage.Value()
		httpapi.SuccessWithPagination(ctx, tmpBlockPageValue.blocks, tmpBlockPageValue.paginationResult)
		return
	}
	blocks, paginationResult, err := handler.blocksView.List(block_view.BlocksListOrder{
		Height: heightOrder,
	}, paginationInput)
	if err != nil {
		handler.logger.Errorf("error listing blocks: %v", err)
		httpapi.InternalServerError(ctx)
		return
	}
	cachePage.Set(blockPaginationKey, BlocksPaginationResult{
		blocks:           blocks,
		paginationResult: paginationResult,
	}, 2*time.Second)
	httpapi.SuccessWithPagination(ctx, blocks, paginationResult)
}

func (handler *Blocks) ListTransactionsByHeight(ctx *fasthttp.RequestCtx) {
	paginationInput, err := httpapi.ParsePagination(ctx)
	if err != nil {
		httpapi.BadRequest(ctx, err)
		return
	}

	blockHeightParam, blockHeightParamOk := URLValueGuard(ctx, handler.logger, "height")
	if !blockHeightParamOk {
		return
	}
	blockHeight, err := strconv.ParseInt(blockHeightParam, 10, 64)
	if err != nil {
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
		httpapi.InternalServerError(ctx)
		return
	}

	httpapi.SuccessWithPagination(ctx, blocks, paginationResult)
}

func (handler *Blocks) ListEventsByHeight(ctx *fasthttp.RequestCtx) {
	paginationInput, err := httpapi.ParsePagination(ctx)
	if err != nil {
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
		return
	}
	blockHeight, err := strconv.ParseInt(blockHeightParam, 10, 64)
	if err != nil {
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
		httpapi.InternalServerError(ctx)
		return
	}

	httpapi.SuccessWithPagination(ctx, blocks, paginationResult)
}

func (handler *Blocks) ListCommitmentsByHeight(ctx *fasthttp.RequestCtx) {
	paginationInput, err := httpapi.ParsePagination(ctx)
	if err != nil {
		httpapi.BadRequest(ctx, err)
		return
	}

	blockHeightParam, blockHeightParamOk := URLValueGuard(ctx, handler.logger, "height")
	if !blockHeightParamOk {
		return
	}
	blockHeight, err := strconv.ParseInt(blockHeightParam, 10, 64)
	if err != nil {
		httpapi.BadRequest(ctx, errors.New("invalid block height"))
		return
	}

	blocks, paginationResult, err := handler.validatorBlockCommitmentsView.List(
		validator_view.ValidatorBlockCommitmentsListFilter{
			MaybeBlockHeight: &blockHeight,
		}, paginationInput)
	if err != nil {
		handler.logger.Errorf("error listing block commitments: %v", err)
		httpapi.InternalServerError(ctx)
		return
	}

	httpapi.SuccessWithPagination(ctx, blocks, paginationResult)
}

func (handler *Blocks) ListCommitmentsByConsensusNodeAddress(ctx *fasthttp.RequestCtx) {
	paginationInput, err := httpapi.ParsePagination(ctx)
	if err != nil {
		httpapi.BadRequest(ctx, err)
		return
	}

	addressParam := ctx.UserValue("address")
	if addressParam == nil {
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
		httpapi.InternalServerError(ctx)
		return
	}

	httpapi.SuccessWithPagination(ctx, blocks, paginationResult)
}
