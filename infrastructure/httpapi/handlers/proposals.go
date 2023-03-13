package handlers

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/AstraProtocol/astra-indexing/appinterface/pagination"
	"github.com/AstraProtocol/astra-indexing/external/cache"
	"github.com/AstraProtocol/astra-indexing/infrastructure"

	"github.com/AstraProtocol/astra-indexing/appinterface/projection/rdbparambase/types"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"

	"github.com/AstraProtocol/astra-indexing/appinterface/cosmosapp"
	param_view "github.com/AstraProtocol/astra-indexing/appinterface/projection/rdbparambase/view"
	"github.com/AstraProtocol/astra-indexing/appinterface/projection/view"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/infrastructure/httpapi"
	"github.com/AstraProtocol/astra-indexing/infrastructure/metric/prometheus"
	proposal_view "github.com/AstraProtocol/astra-indexing/projection/proposal/view"
	"github.com/AstraProtocol/astra-indexing/usecase/coin"
	"github.com/valyala/fasthttp"
)

type Proposals struct {
	logger applogger.Logger

	cosmosClient       cosmosapp.Client
	proposalsView      proposal_view.Proposals
	votesView          proposal_view.Votes
	depositorsView     proposal_view.Depositors
	proposalParamsView param_view.Params

	totalBonded              coin.Coin
	totalBondedLastUpdatedAt time.Time
	astraCache               *cache.AstraCache
}

func NewProposals(logger applogger.Logger, rdbHandle *rdb.Handle, cosmosClient cosmosapp.Client) *Proposals {
	return &Proposals{
		logger,

		cosmosClient,
		proposal_view.NewProposalsView(rdbHandle),
		proposal_view.NewVotesView(rdbHandle),
		proposal_view.NewDepositorsView(rdbHandle),
		param_view.NewParamsView(rdbHandle, proposal_view.PARAMS_TABLE_NAME),

		coin.Coin{},
		time.Unix(int64(0), int64(0)),
		cache.NewCache(),
	}
}

type ProposalPaginationResult struct {
	Proposals        []proposal_view.ProposalWithMonikerRow `json:"proposalWithMonikerRow"`
	PaginationResult pagination.Result                      `json:"paginationResult"`
}

func NewProposalPaginationResult(proposalRows []proposal_view.ProposalWithMonikerRow,
	paginationResult pagination.Result) *ProposalPaginationResult {
	return &ProposalPaginationResult{
		proposalRows,
		paginationResult,
	}
}

func (handler *Proposals) FindById(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "ProposalFindById"

	idParam, idParamOk := URLValueGuard(ctx, handler.logger, "id")
	if !idParamOk {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("id param is invalid"))
		return
	}
	var tmpProposal ProposalDetails
	proposalKey := fmt.Sprintf("proposal_%s", idParam)
	err := handler.astraCache.Get(proposalKey, &tmpProposal)
	if err == nil {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("id param is invalid"))
		httpapi.Success(ctx, tmpProposal)
		return
	}
	proposal, err := handler.proposalsView.FindById(idParam)
	if err != nil {
		if errors.Is(err, rdb.ErrNoRows) {
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusNotFound), "GET", time.Since(startTime).Milliseconds())
			httpapi.NotFound(ctx)
			return
		}
		handler.logger.Errorf("error finding proposal by id: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusInternalServerError), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	tally := cosmosapp.Tally{}
	if proposal.Tally != nil {
		tallyMap := proposal.Tally.(map[string]interface{})
		tally.No = tallyMap["no"].(string)
		tally.Yes = tallyMap["yes"].(string)
		tally.Abstain = tallyMap["abstain"].(string)
		tally.NoWithVeto = tallyMap["no_with_veto"].(string)
	} else {
		tally.No = "0"
		tally.Yes = "0"
		tally.Abstain = "0"
		tally.NoWithVeto = "0"
	}

	if handler.totalBondedLastUpdatedAt.Add(1 * time.Hour).Before(time.Now()) {
		var queryTotalBondedErr error
		handler.totalBonded, queryTotalBondedErr = handler.cosmosClient.TotalBondedBalance()
		if queryTotalBondedErr != nil {
			handler.logger.Errorf("error retrieving total bonded balance: %v", queryTotalBondedErr)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusInternalServerError), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}

		handler.totalBondedLastUpdatedAt = time.Now()
	}
	quorumStr, queryQuorumErr := handler.proposalParamsView.FindBy(types.ParamAccessor{
		Module: "gov",
		Key:    "quorum",
	})
	if queryQuorumErr != nil {
		handler.logger.Errorf("error retrieving gov quorum param: %v", queryQuorumErr)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusInternalServerError), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	quorum, parseQuorumOk := new(big.Float).SetString(quorumStr)
	if !parseQuorumOk {
		handler.logger.Errorf("error parsing gov quorum param to big.Float: %v", parseQuorumOk)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("error parsing gov quorum"))
		return
	}
	totalBonded, parseTotalBondedOk := new(big.Float).SetString(handler.totalBonded.Amount.String())
	if !parseTotalBondedOk {
		handler.logger.Errorf("error parsing total bonded balance to big.Float: %v", parseTotalBondedOk)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("error parsing total bonded balance"))
		return
	}
	requiredVotingPower := new(big.Float).Mul(totalBonded, quorum)

	totalVotedPower := big.NewInt(0)
	for _, votedPowerStr := range []string{
		tally.Yes,
		tally.No,
		tally.NoWithVeto,
		tally.Abstain,
	} {
		votedPower, parseVotedPowerOk := new(big.Int).SetString(votedPowerStr, 10)
		if !parseVotedPowerOk {
			handler.logger.Error("error parsing voted power")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
			httpapi.BadRequest(ctx, errors.New("error parsing voted power"))
			return
		}

		totalVotedPower = new(big.Int).Add(totalVotedPower, votedPower)
	}

	proposalDetails := ProposalDetails{
		proposal,
		requiredVotingPower.Text('f', 0),
		totalVotedPower.Text(10),
		ProposalVotedPowerResult{
			Yes:        tally.Yes,
			Abstain:    tally.Abstain,
			No:         tally.No,
			NoWithVeto: tally.NoWithVeto,
		},
	}
	handler.astraCache.Set(proposalKey, proposalDetails, infrastructure.TIME_CACHE_FAST)
	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, proposalDetails)
}

func (handler *Proposals) List(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "ListProposals"

	var err error
	pagePagination, err := httpapi.ParsePagination(ctx)
	if err != nil {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	idOrder := view.ORDER_ASC
	queryArgs := ctx.QueryArgs()
	if queryArgs.Has("order") {
		if string(queryArgs.Peek("order")) == "id.desc" {
			idOrder = view.ORDER_DESC
		}
	}
	proposalKey := "Proposals_" + idOrder
	filter := proposal_view.ProposalListFilter{
		MaybeStatus:          nil,
		MaybeProposerAddress: nil,
	}
	if queryArgs.Has("status") {
		status := string(queryArgs.Peek("status"))
		filter.MaybeStatus = &status
		proposalKey += "_" + status
	}
	if queryArgs.Has("proposerAddress") {
		address := string(queryArgs.Peek("proposerAddress"))
		filter.MaybeProposerAddress = &address
		proposalKey += "_" + address
	}

	var tmpProposalCache ProposalPaginationResult
	err = handler.astraCache.Get(proposalKey, &tmpProposalCache)
	if err == nil {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.SuccessWithPagination(ctx, tmpProposalCache.Proposals, &tmpProposalCache.PaginationResult)
		return
	}

	proposals, paginationResult, err := handler.proposalsView.List(filter, proposal_view.ProposalListOrder{
		Id: idOrder,
	}, pagePagination)
	if err != nil {
		handler.logger.Errorf("error listing proposals: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusInternalServerError), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}
	handler.astraCache.Set(proposalKey, NewProposalPaginationResult(proposals, *paginationResult), infrastructure.TIME_CACHE_FAST)
	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.SuccessWithPagination(ctx, proposals, paginationResult)
}

type VotesPaginationResult struct {
	Votes            []proposal_view.VoteWithMonikerRow `json:"voteWithMonikerRow"`
	PaginationResult pagination.Result                  `json:"paginationResult"`
}

func NewVotesPaginationResult(voteRows []proposal_view.VoteWithMonikerRow,
	paginationResult pagination.Result) *VotesPaginationResult {
	return &VotesPaginationResult{
		voteRows,
		paginationResult,
	}
}

func (handler *Proposals) ListVotesById(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "ProposalListVotesById"

	idParam, idParamOk := URLValueGuard(ctx, handler.logger, "id")
	if !idParamOk {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("id param is invalid"))
		return
	}
	parsePagination, paginationError := httpapi.ParsePagination(ctx)
	if paginationError != nil {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	voteAtOrder := view.ORDER_ASC
	queryArgs := ctx.QueryArgs()
	if queryArgs.Has("order") {
		if string(queryArgs.Peek("order")) == "voteAt.desc" {
			voteAtOrder = view.ORDER_DESC
		}
	}
	filters := proposal_view.Filters{}
	if queryArgs.Has("answer") {
		filters.Answer = string(queryArgs.Peek("answer"))
	}
	if queryArgs.Has("voterAddress") {
		filters.Address = string(queryArgs.Peek("voterAddress"))
	}

	voteCacheKey := fmt.Sprintf("voteById_%s_%s_%s", idParam, voteAtOrder, filters.ToStr())
	var tmpVoteCache VotesPaginationResult
	err := handler.astraCache.Get(voteCacheKey, &tmpVoteCache)
	if err == nil {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.SuccessWithPagination(ctx, tmpVoteCache.Votes, &tmpVoteCache.PaginationResult)
		return
	}

	votes, paginationResult, err := handler.votesView.ListByProposalId(idParam, proposal_view.VoteListOrder{
		VoteAtBlockHeight: voteAtOrder,
	}, filters, parsePagination)
	if err != nil {
		handler.logger.Errorf("error listing proposal votes: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusInternalServerError), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}
	handler.astraCache.Set(voteCacheKey, NewVotesPaginationResult(votes, *paginationResult), infrastructure.TIME_CACHE_FAST)
	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.SuccessWithPagination(ctx, votes, paginationResult)
}

type DepositPaginationResult struct {
	Depositor        []proposal_view.DepositorWithMonikerRow `json:"depositorWithMonikerRow"`
	PaginationResult pagination.Result                       `json:"paginationResult"`
}

func NewDepositPaginationResult(depositorRows []proposal_view.DepositorWithMonikerRow,
	paginationResult pagination.Result) *DepositPaginationResult {
	return &DepositPaginationResult{
		depositorRows,
		paginationResult,
	}
}

func (handler *Proposals) ListDepositorsById(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "ProposalListDepositorsById"

	idParam, idParamOk := URLValueGuard(ctx, handler.logger, "id")
	if !idParamOk {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("id param is invalid"))
		return
	}

	parsePagination, paginationError := httpapi.ParsePagination(ctx)
	if paginationError != nil {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	depositAtOrder := view.ORDER_ASC
	queryArgs := ctx.QueryArgs()
	if queryArgs.Has("order") {
		if string(queryArgs.Peek("order")) == "depositAt.desc" {
			depositAtOrder = view.ORDER_DESC
		}
	}
	filters := proposal_view.Filters{}
	if queryArgs.Has("depositorAddress") {
		filters.Address = string(queryArgs.Peek("depositorAddress"))
	}

	depositCacheKey := fmt.Sprintf("depositById_%s_%s_%s", idParam, depositAtOrder, filters.ToStr())
	var tmpDepositorCache DepositPaginationResult

	err := handler.astraCache.Get(depositCacheKey, &tmpDepositorCache)
	if err == nil {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.SuccessWithPagination(ctx, tmpDepositorCache.Depositor, &tmpDepositorCache.PaginationResult)
		return
	}

	depositors, paginationResult, err := handler.depositorsView.ListByProposalId(idParam, proposal_view.DepositorListOrder{
		DepositAtBlockHeight: depositAtOrder,
	}, filters, parsePagination)
	if err != nil {
		handler.logger.Errorf("error listing proposal votes: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusInternalServerError), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}
	handler.astraCache.Set(depositCacheKey, NewDepositPaginationResult(depositors, *paginationResult), infrastructure.TIME_CACHE_FAST)
	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.SuccessWithPagination(ctx, depositors, paginationResult)
}

func (handler *Proposals) UpdateTally(ctx *fasthttp.RequestCtx) {
	idParam, idParamOk := URLValueGuard(ctx, handler.logger, "id")
	if !idParamOk {
		return
	}

	idParamInt, err := strconv.Atoi(idParam)
	if err != nil {
		httpapi.Success(ctx, "NOK")
		return
	}

	for id := 1; id <= idParamInt; id++ {
		tally, _ := handler.cosmosClient.ProposalTally(strconv.Itoa(id))
		if tally.Abstain == "" {
			handler.proposalsView.UpdateTally(strconv.Itoa(id), nil)
		} else {
			handler.proposalsView.UpdateTally(strconv.Itoa(id), tally)
		}
		time.Sleep(time.Second)
	}

	httpapi.Success(ctx, "OK")
}

type ProposalDetails struct {
	*proposal_view.ProposalWithMonikerRow

	RequiredVotingPower string                   `json:"requiredVotingPower"`
	TotalVotedPower     string                   `json:"totalVotedPower"`
	VotedPowerResult    ProposalVotedPowerResult `json:"votedPowerResult"`
}

type ProposalVotedPowerResult struct {
	Yes        string `json:"yes"`
	Abstain    string `json:"abstain"`
	No         string `json:"no"`
	NoWithVeto string `json:"noWithVeto"`
}
