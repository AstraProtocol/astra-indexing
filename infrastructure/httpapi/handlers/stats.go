package handlers

import (
	"time"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	"github.com/valyala/fasthttp"

	blockscout_infrastructure "github.com/AstraProtocol/astra-indexing/infrastructure/blockscout"
	"github.com/AstraProtocol/astra-indexing/infrastructure/httpapi"
	account_view "github.com/AstraProtocol/astra-indexing/projection/account/view"
	block_view "github.com/AstraProtocol/astra-indexing/projection/block/view"
	chainstats_view "github.com/AstraProtocol/astra-indexing/projection/chainstats/view"
	transaction_view "github.com/AstraProtocol/astra-indexing/projection/transaction/view"
)

type StatsHandler struct {
	logger applogger.Logger

	blockscoutClient      blockscout_infrastructure.HTTPClient
	blocksView            *block_view.Blocks
	accountsView          account_view.Accounts
	chainStatsView        *chainstats_view.ChainStats
	transactionsTotalView transaction_view.TransactionsTotal
}

func NewStatsHandler(
	logger applogger.Logger,
	blockscoutClient blockscout_infrastructure.HTTPClient,
	rdbHandle *rdb.Handle,
) *StatsHandler {
	return &StatsHandler{
		logger.WithFields(applogger.LogFields{
			"module": "StatsHandler",
		}),

		blockscoutClient,
		block_view.NewBlocks(rdbHandle),
		account_view.NewAccountsView(rdbHandle),
		chainstats_view.NewChainStats(rdbHandle),
		transaction_view.NewTransactionsTotalView(rdbHandle),
	}
}

func (handler *StatsHandler) GetTransactionsHistory(ctx *fasthttp.RequestCtx) {
	// Fetch transactions history of last 30 days
	date_range := 30
	transactionsHistoryList, err := handler.chainStatsView.GetTransactionsHistoryByDateRange(date_range)

	if err != nil {
		handler.logger.Errorf("error fetching transactions history: %v", err)
		httpapi.InternalServerError(ctx)
		return
	}

	httpapi.Success(ctx, transactionsHistoryList)
}

func (handler *StatsHandler) GetCommonStats(ctx *fasthttp.RequestCtx) {
	commonStatsChan := make(chan blockscout_infrastructure.CommonStats)
	go handler.blockscoutClient.GetCommonStatsAsync(commonStatsChan)

	transactionsCountPerDay, err := handler.blocksView.TotalTransactionsPerDay()
	if err != nil {
		handler.logger.Errorf("error fetching transactions count per day: %v", err)
		httpapi.InternalServerError(ctx)
		return
	}

	commonStats := <-commonStatsChan
	commonStats.TransactionStats.NumberOfTransactions = transactionsCountPerDay
	commonStats.TransactionStats.Date = time.Now().Local().String()

	httpapi.Success(ctx, commonStats)
}

func (handler *StatsHandler) EstimateCounted(ctx *fasthttp.RequestCtx) {
	blocksCount, err := handler.blocksView.Count()
	if err != nil {
		handler.logger.Errorf("error fetching block count: %v", err)
		httpapi.InternalServerError(ctx)
		return
	}

	transactionsCount, err := handler.transactionsTotalView.FindBy("-")
	if err != nil {
		handler.logger.Errorf("error fetching transaction count: %v", err)
		httpapi.InternalServerError(ctx)
		return
	}

	addressesCount, err := handler.accountsView.TotalAccount()
	if err != nil {
		handler.logger.Errorf("error fetching address count: %v", err)
		httpapi.InternalServerError(ctx)
		return
	}

	estimateCounted := EstimateCountedInfo{}
	estimateCounted.TotalTransactions = transactionsCount
	estimateCounted.TotalBlocks = blocksCount
	estimateCounted.TotalAddresses = addressesCount

	httpapi.Success(ctx, estimateCounted)
}

type EstimateCountedInfo struct {
	TotalBlocks       int64 `json:"total_blocks"`
	TotalTransactions int64 `json:"total_transactions"`
	TotalAddresses    int64 `json:"wallet_addresses"`
}
