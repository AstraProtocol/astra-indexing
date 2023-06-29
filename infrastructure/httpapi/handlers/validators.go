package handlers

import (
	"errors"
	"fmt"
	"strings"

	"github.com/AstraProtocol/astra-indexing/appinterface/pagination"
	"github.com/AstraProtocol/astra-indexing/external/cache"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	"github.com/AstraProtocol/astra-indexing/external/primptr"
	"github.com/AstraProtocol/astra-indexing/infrastructure"

	"github.com/valyala/fasthttp"

	"github.com/AstraProtocol/astra-indexing/appinterface/cosmosapp"
	"github.com/AstraProtocol/astra-indexing/appinterface/projection/view"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/appinterface/tendermint"
	"github.com/AstraProtocol/astra-indexing/infrastructure/httpapi"
	block_view "github.com/AstraProtocol/astra-indexing/projection/block/view"
	chainstats_view "github.com/AstraProtocol/astra-indexing/projection/chainstats/view"
	"github.com/AstraProtocol/astra-indexing/projection/validator/constants"
	validator_view "github.com/AstraProtocol/astra-indexing/projection/validator/view"
)

// When we have a large number of blocks, we would like only take the recent N most blocks (blocks in 7 recent days),
// in order to calculate the averageBlockTime.
//
// Assume average block generation time is 6 seconds per block.
// Then in recent 7 days, number of estimated generated block will be:
//
// nRecentBlocks: n (block) = 7(day) * 24(hour/day) * 3600(sec/hour) / 6(sec/block)
// const nRecentBlocksInInt = 100800

type Validators struct {
	logger applogger.Logger

	validatorAddressPrefix string
	consNodeAddressPrefix  string

	cosmosAppClient         cosmosapp.Client
	tendermintClient        tendermint.Client
	validatorsView          *validator_view.Validators
	validatorActivitiesView *validator_view.ValidatorActivities
	chainStatsView          *chainstats_view.ChainStats
	blockView               *block_view.Blocks
	astraCache              *cache.AstraCache
}

func NewValidators(
	logger applogger.Logger,
	validatorAddressPrefix string,
	consNodeAddressPrefix string,
	cosmosAppClient cosmosapp.Client,
	tendermintClient tendermint.Client,
	rdbHandle *rdb.Handle,
) *Validators {
	return &Validators{
		logger.WithFields(applogger.LogFields{
			"module": "ValidatorsHandler",
		}),

		validatorAddressPrefix,
		consNodeAddressPrefix,

		cosmosAppClient,
		tendermintClient,
		validator_view.NewValidators(rdbHandle),
		validator_view.NewValidatorActivities(rdbHandle),
		chainstats_view.NewChainStats(rdbHandle),
		block_view.NewBlocks(rdbHandle),
		cache.NewCache(),
	}
}

func (handler *Validators) FindBy(ctx *fasthttp.RequestCtx) {
	addressParams, addressParamsOk := URLValueGuard(ctx, handler.logger, "address")
	if !addressParamsOk {
		return
	}
	var identity validator_view.ValidatorIdentity
	if strings.HasPrefix(addressParams, handler.validatorAddressPrefix) {
		identity = validator_view.ValidatorIdentity{
			MaybeOperatorAddress: &addressParams,
		}
	} else if strings.HasPrefix(addressParams, handler.consNodeAddressPrefix) {
		identity = validator_view.ValidatorIdentity{
			MaybeConsensusNodeAddress: &addressParams,
		}
	} else {
		httpapi.BadRequest(ctx, errors.New("invalid address"))
		return
	}

	validatorCacheKey := fmt.Sprintf("validatorAddress_%s", addressParams)
	var tmpValidatorDetail ValidatorDetails
	err := handler.astraCache.Get(validatorCacheKey, &tmpValidatorDetail)
	if err == nil {
		httpapi.Success(ctx, tmpValidatorDetail)
		return
	}
	rawValidator, err := handler.validatorsView.FindBy(identity)
	if err != nil {
		if errors.Is(err, rdb.ErrNoRows) {
			httpapi.NotFound(ctx)
			return
		}
		handler.logger.Errorf("error finding validator by operator address: %v", err)
		httpapi.InternalServerError(ctx)
		return
	}

	validator := ValidatorDetails{
		ValidatorRow: rawValidator,

		Tokens:         "0",
		SelfDelegation: "0",
	}

	validatorData, err := handler.cosmosAppClient.Validator(validator.OperatorAddress)
	if err != nil {
		handler.logger.Errorf("error getting validator details: %v", err)
	} else {
		validator.Tokens = validatorData.Tokens
	}

	delegation, err := handler.cosmosAppClient.Delegation(validator.InitialDelegatorAddress, validator.OperatorAddress)
	if err != nil {
		handler.logger.Errorf("error getting self delegation record: %v", err)
	} else if delegation != nil {
		validator.SelfDelegation = delegation.Balance.Amount
	}

	_ = handler.astraCache.Set(validatorCacheKey, validator, infrastructure.TIME_CACHE_FAST)
	httpapi.Success(ctx, validator)
}

type ValidatorPaginationResult struct {
	ValidatorRowWithAPY []validatorRowWithAPY `json:"validatorRowWithAPY"`
	PaginationResult    pagination.Result     `json:"paginationResult"`
}

func NewValidatorPaginationResult(validators []validatorRowWithAPY,
	paginationResult pagination.Result) *ValidatorPaginationResult {
	return &ValidatorPaginationResult{
		validators,
		paginationResult,
	}
}

func (handler *Validators) List(ctx *fasthttp.RequestCtx) {
	paginationParse, err := httpapi.ParsePagination(ctx)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	queryArgs := ctx.QueryArgs()
	order := validator_view.ValidatorsListOrder{
		MaybeStatus:              primptr.String(view.ORDER_ASC),
		MaybeJoinedAtBlockHeight: primptr.String(view.ORDER_ASC),
	}
	if queryArgs.Has("order") {
		rawOrderArgs := queryArgs.PeekMulti("order")
		for _, rawOrderArg := range rawOrderArgs {
			orderArg := string(rawOrderArg)
			if orderArg == "power" {
				order.MaybePower = primptr.String(view.ORDER_ASC)
			} else if orderArg == "power.desc" {
				order.MaybePower = primptr.String(view.ORDER_DESC)
			} else if orderArg == "commission" {
				order.MaybeCommission = primptr.String(view.ORDER_ASC)
			} else if orderArg == "commission.desc" {
				order.MaybeCommission = primptr.String(view.ORDER_DESC)
			} else {
				handler.logger.Errorf("error listing validators: invalid order: %s", orderArg)
				httpapi.BadRequest(ctx, errors.New("invalid order"))
				return
			}
		}
	}
	keyCacheValidator := fmt.Sprintf("validatorList_%d_%d_%v",
		paginationParse.OffsetParams().Page,
		paginationParse.OffsetParams().Limit, order.ToStr())

	var tmpValidatorPaginationResult ValidatorPaginationResult
	err = handler.astraCache.Get(keyCacheValidator, &tmpValidatorPaginationResult)
	if err == nil {
		httpapi.SuccessWithPagination(ctx,
			tmpValidatorPaginationResult.ValidatorRowWithAPY,
			&tmpValidatorPaginationResult.PaginationResult)
		return
	}

	validators, paginationResult, err := handler.validatorsView.List(
		validator_view.ValidatorsListFilter{}, order, paginationParse,
	)
	if err != nil {
		handler.logger.Errorf("error listing validators: %v", err)
		httpapi.InternalServerError(ctx)
		return
	}

	validatorsWithAPY := make([]validatorRowWithAPY, 0, len(validators))
	for _, validator := range validators {
		if validator.Status != constants.BONDED {
			validatorsWithAPY = append(validatorsWithAPY, validatorRowWithAPY{
				validator,
			})
			continue
		}
		validatorsWithAPY = append(validatorsWithAPY, validatorRowWithAPY{
			validator,
		})
	}

	_ = handler.astraCache.Set(keyCacheValidator,
		NewValidatorPaginationResult(validatorsWithAPY, *paginationResult),
		infrastructure.TIME_CACHE_FAST)

	httpapi.SuccessWithPagination(ctx, validatorsWithAPY, paginationResult)
}

type validatorRowWithAPY struct {
	validator_view.ListValidatorRow
}

/*
func (handler *Validators) getAverageBlockTime() (*big.Float, error) {
	// Average block time calculation
	//
	// Case A: totalBlockCount <= nRecentBlocks, calculate with blocks from Genesis block to Latest block
	// Case B: totalBlockCount > nRecentBlocks, calculate with n recent blocks
	var totalBlockTime = big.NewInt(0)
	var totalBlockCount = big.NewInt(1)

	if rawTotalBlockTime, err := handler.chainStatsView.FindBy(chainstats.TOTAL_BLOCK_TIME); err != nil {
		return nil, fmt.Errorf("error fetching total block time: %v", err)
	} else {
		if rawTotalBlockTime != "" {
			var ok bool
			if totalBlockTime, ok = new(big.Int).SetString(rawTotalBlockTime, 10); !ok {
				return nil, errors.New("error converting total block time from string to big.Int")
			}
		}

		if rawTotalBlockCount, err := handler.chainStatsView.FindBy(chainstats.TOTAL_BLOCK_COUNT); err != nil {
			return nil, fmt.Errorf("error fetching total block time: %v", err)
		} else {
			if rawTotalBlockCount != "" {
				var ok bool
				if totalBlockCount, ok = new(big.Int).SetString(rawTotalBlockCount, 10); !ok {
					return nil, fmt.Errorf("error converting total block count from string to big.Int")
				}
			}
		}
	}

	nRecentBlocks := big.NewInt(nRecentBlocksInInt)

	hasNBlocksSinceGenesis := (totalBlockCount.Cmp(nRecentBlocks) == 1)

	var averageBlockTimeMilliSecond *big.Float
	// Determine case A or case B
	if hasNBlocksSinceGenesis {
		// Case B
		latestBlockHeight, err := handler.blockView.Count()
		if err != nil {
			return nil, fmt.Errorf("error fetching latest block height: %v", err)
		}
		latestBlockIdentity := block_view.BlockIdentity{MaybeHeight: &latestBlockHeight}
		latestBlock, err := handler.blockView.FindBy(&latestBlockIdentity)
		if err != nil {
			return nil, fmt.Errorf("error fetching latest block: %v", err)
		}

		// Find the nth block before latest block
		startBlockHeight := latestBlockHeight - nRecentBlocks.Int64()
		startBlockIdentity := block_view.BlockIdentity{MaybeHeight: &startBlockHeight}
		startBlock, err := handler.blockView.FindBy(&startBlockIdentity)
		if err != nil {
			return nil, fmt.Errorf("error fetching the start block: %v", err)
		}

		// Calculate total time in generating n recent blocks
		nRecentBlocksTotalTime := latestBlock.Time.UnixNano() - startBlock.Time.UnixNano()

		nRecentBlocksTotalTimeMilliSecond := new(big.Float).Quo(
			new(big.Float).SetInt64(nRecentBlocksTotalTime),
			new(big.Float).SetInt64(int64(1000000)),
		)
		averageBlockTimeMilliSecond = new(big.Float).Quo(
			nRecentBlocksTotalTimeMilliSecond,
			new(big.Float).SetInt(nRecentBlocks),
		)

	} else {
		// Case A
		totalBlockTimeMilliSecond := new(big.Float).Quo(
			new(big.Float).SetInt(totalBlockTime),
			new(big.Float).SetInt64(int64(1000000)),
		)
		averageBlockTimeMilliSecond = new(big.Float).Quo(
			totalBlockTimeMilliSecond,
			new(big.Float).SetInt(totalBlockCount),
		)
	}

	averageBlockTime := new(big.Float).Quo(
		averageBlockTimeMilliSecond,
		big.NewFloat(1000),
	)

	return averageBlockTime, nil
}
*/

type ListValidatorsRowPaginationResult struct {
	ListValidatorRow []validator_view.ListValidatorRow `json:"listValidatorRow"`
	PaginationResult pagination.Result                 `json:"paginationResult"`
}

func NewListValidatorsRowPaginationResult(listValidatorRow []validator_view.ListValidatorRow,
	paginationResult pagination.Result) *ListValidatorsRowPaginationResult {
	return &ListValidatorsRowPaginationResult{
		listValidatorRow,
		paginationResult,
	}
}

func (handler *Validators) ListActive(ctx *fasthttp.RequestCtx) {
	var err error

	paginationParse, err := httpapi.ParsePagination(ctx)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	queryArgs := ctx.QueryArgs()
	order := validator_view.ValidatorsListOrder{
		MaybeStatus:              primptr.String(view.ORDER_ASC),
		MaybeJoinedAtBlockHeight: primptr.String(view.ORDER_ASC),
	}
	if queryArgs.Has("order") {
		rawOrderArgs := queryArgs.PeekMulti("order")
		for _, rawOrderArg := range rawOrderArgs {
			orderArg := string(rawOrderArg)
			if orderArg == "power" {
				order.MaybePower = primptr.String(view.ORDER_ASC)
			} else if orderArg == "power.desc" {
				order.MaybePower = primptr.String(view.ORDER_DESC)
			} else if orderArg == "commission" {
				order.MaybeCommission = primptr.String(view.ORDER_ASC)
			} else if orderArg == "commission.desc" {
				order.MaybeCommission = primptr.String(view.ORDER_DESC)
			} else {
				handler.logger.Errorf("error listing active validators: invalid order: %s", orderArg)
				httpapi.BadRequest(ctx, errors.New("invalid order"))
				return
			}
		}
	}

	keyCacheListActive := fmt.Sprintf("ValidatorListActive_%d_%d_%s",
		paginationParse.OffsetParams().Page, paginationParse.OffsetParams().Limit, order.ToStr())

	var tmpListValidatorRowPaginationResult ListValidatorsRowPaginationResult

	err = handler.astraCache.Get(keyCacheListActive, &tmpListValidatorRowPaginationResult)
	if err == nil {
		httpapi.SuccessWithPagination(ctx, tmpListValidatorRowPaginationResult.ListValidatorRow,
			&tmpListValidatorRowPaginationResult.PaginationResult)
		return
	}

	validators, paginationResult, err := handler.validatorsView.List(validator_view.ValidatorsListFilter{
		MaybeStatuses: []constants.Status{
			constants.BONDED,
			constants.JAILED,
			constants.UNBONDING,
		},
	}, order, paginationParse)
	if err != nil {
		handler.logger.Errorf("error listing active validators: %v", err)
		httpapi.InternalServerError(ctx)
		return
	}

	_ = handler.astraCache.Set(keyCacheListActive,
		NewListValidatorsRowPaginationResult(validators, *paginationResult), infrastructure.TIME_CACHE_FAST)
	httpapi.SuccessWithPagination(ctx, validators, paginationResult)
}

type ListActivitiesActivityRowPaginationResult struct {
	ValidatorActivityRow []validator_view.ValidatorActivityRow `json:"validatorActivityRow"`
	PaginationResult     pagination.Result                     `json:"paginationResult"`
}

func NewValidatorActivityRowPaginationResult(listValidatorRow []validator_view.ValidatorActivityRow,
	paginationResult pagination.Result) *ListActivitiesActivityRowPaginationResult {
	return &ListActivitiesActivityRowPaginationResult{
		listValidatorRow,
		paginationResult,
	}
}

func (handler *Validators) ListActivities(ctx *fasthttp.RequestCtx) {
	paginationParse, err := httpapi.ParsePagination(ctx)
	if err != nil {
		httpapi.BadRequest(ctx, err)
		return
	}

	addressParams, addressParamsOk := URLValueGuard(ctx, handler.logger, "address")
	if !addressParamsOk {
		return
	}
	var filter validator_view.ValidatorActivitiesListFilter
	if strings.HasPrefix(addressParams, handler.validatorAddressPrefix) {
		filter = validator_view.ValidatorActivitiesListFilter{
			MaybeOperatorAddress: &addressParams,
		}
	} else if strings.HasPrefix(addressParams, handler.consNodeAddressPrefix) {
		filter = validator_view.ValidatorActivitiesListFilter{
			MaybeConsensusNodeAddress: &addressParams,
		}
	} else {
		httpapi.BadRequest(ctx, errors.New("invalid address"))
		return
	}

	order := validator_view.ValidatorActivitiesListOrder{}
	queryArgs := ctx.QueryArgs()
	if queryArgs.Has("order") {
		orderArg := string(queryArgs.Peek("order"))
		if orderArg == "height" {
			order.MaybeBlockHeight = primptr.String(view.ORDER_ASC)
		} else if orderArg == "height.desc" {
			order.MaybeBlockHeight = primptr.String(view.ORDER_DESC)
		} else {
			httpapi.BadRequest(ctx, errors.New("invalid order"))
			return
		}
	}

	cacheKeyListActivities := "validator_ListActivities_"
	if order.MaybeBlockHeight != nil {
		cacheKeyListActivities = "validator_ListActivities_" + *order.MaybeBlockHeight
	}
	var tmpListActivitiesRow ListActivitiesActivityRowPaginationResult

	err = handler.astraCache.Get(cacheKeyListActivities, &tmpListActivitiesRow)
	if err == nil {
		httpapi.SuccessWithPagination(ctx, tmpListActivitiesRow.ValidatorActivityRow,
			&tmpListActivitiesRow.PaginationResult)
		return
	}

	validators, paginationResult, err := handler.validatorActivitiesView.List(filter, order, paginationParse)
	if err != nil {
		handler.logger.Errorf("error listing activities: %v", err)
		httpapi.InternalServerError(ctx)
		return
	}
	_ = handler.astraCache.Set(cacheKeyListActivities,
		NewValidatorActivityRowPaginationResult(validators, *paginationResult),
		infrastructure.TIME_CACHE_FAST)
	httpapi.SuccessWithPagination(ctx, validators, paginationResult)
}

type ValidatorDetails struct {
	*validator_view.ValidatorRow

	Tokens         string `json:"tokens"`
	SelfDelegation string `json:"selfDelegation"`
}
