package handlers

import (
	"math"
	"strconv"
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

func (handler *StatsHandler) GetTransactionsHistoryChart(ctx *fasthttp.RequestCtx) {
	// Fetch transactions history of last 30 days
	date_range := 30
	transactionsHistoryList, err := handler.chainStatsView.GetTransactionsHistoryForChart(date_range)

	if err != nil {
		handler.logger.Errorf("error fetching transactions history: %v", err)
		httpapi.InternalServerError(ctx)
		return
	}

	httpapi.Success(ctx, transactionsHistoryList)
}

func (handler *StatsHandler) GetTransactionsHistory(ctx *fasthttp.RequestCtx) {
	// handle api's params
	var err error
	var year int64
	year = int64(time.Now().Year())
	if string(ctx.QueryArgs().Peek("year")) != "" {
		year, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("year")), 10, 0)
		if err != nil {
			handler.logger.Error("year param is invalid")
			httpapi.InternalServerError(ctx)
			return
		}
		if int(year) > time.Now().Year() {
			handler.logger.Error("year is too far")
			httpapi.InternalServerError(ctx)
			return
		}
	}

	var month int64
	month = 0
	if string(ctx.QueryArgs().Peek("month")) != "" {
		month, err = strconv.ParseInt(string(ctx.QueryArgs().Peek("month")), 10, 0)
		if err != nil || month > 12 || month < 1 {
			handler.logger.Error("month param is invalid")
			httpapi.InternalServerError(ctx)
			return
		}
	}
	//

	transactionsCount, err := handler.transactionsTotalView.FindBy("-")
	if err != nil {
		handler.logger.Errorf("error fetching transaction count: %v", err)
		httpapi.InternalServerError(ctx)
		return
	}

	min_date, err := handler.chainStatsView.GetMinDate()
	if err != nil {
		handler.logger.Errorf("error fetching min date of chain_stats: %v", err)
		httpapi.InternalServerError(ctx)
		return
	}

	min_date_time := time.Unix(0, min_date).UTC()
	diff_time := time.Now().Truncate(time.Hour * 24).Sub(min_date_time)

	if month > 0 {
		from_date := time.Date(int(year), time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		end_date := from_date.AddDate(0, 1, 0)

		transactionsHistoryList, err := handler.chainStatsView.GetTransactionsHistory(from_date, end_date)

		if err != nil {
			handler.logger.Errorf("error fetching transactions history daily: %v", err)
			httpapi.InternalServerError(ctx)
			return
		}

		diff_day := int64(math.Ceil(diff_time.Hours() / 24))

		var transactionsHistoryDaily TransactionsHistoryDaily
		transactionsHistoryDaily.TransactionsHistory = transactionsHistoryList
		if len(transactionsHistoryList) > 0 {
			transactionsHistoryDaily.DailyAverage = int(transactionsCount / diff_day)
		}

		httpapi.Success(ctx, transactionsHistoryDaily)
	} else {
		from_date := time.Date(int(year), time.January, 1, 0, 0, 0, 0, time.UTC)
		end_date := from_date.AddDate(1, 0, 0)

		transactionsHistoryList, err := handler.chainStatsView.GetTransactionsHistory(from_date, end_date)

		result := make([]chainstats_view.TransactionHistory, 0)

		if err != nil {
			handler.logger.Errorf("error fetching transactions history monthly: %v", err)
			httpapi.InternalServerError(ctx)
			return
		}

		if len(transactionsHistoryList) == 0 {
			httpapi.Success(ctx, transactionsHistoryList)
			return
		}

		length := len(transactionsHistoryList)
		check_year := transactionsHistoryList[0].Year
		check_month := transactionsHistoryList[0].Month
		var monthly_transactions int64

		for index, transactionHistory := range transactionsHistoryList {
			// init counting
			if index == 0 {
				monthly_transactions = 0
			}
			// counting
			if transactionHistory.Month == check_month {
				monthly_transactions += transactionHistory.NumberOfTransactions
			}
			// add to result then reset counting
			if (index < length-1 && transactionHistory.Month != transactionsHistoryList[index+1].Month) || index == length-1 {
				var transactionHistoryMonthly chainstats_view.TransactionHistory
				transactionHistoryMonthly.Year = check_year
				transactionHistoryMonthly.Month = check_month
				transactionHistoryMonthly.NumberOfTransactions = monthly_transactions
				result = append(result, transactionHistoryMonthly)

				if index == length-1 {
					break
				}

				monthly_transactions = 0
				check_month = transactionsHistoryList[index+1].Month
			}
		}

		diff_month := int64(math.Ceil(diff_time.Hours() / (24 * 30)))

		var transactionsHistoryMonthly TransactionsHistoryMonthly
		transactionsHistoryMonthly.TransactionsHistory = result
		if len(result) > 0 {
			transactionsHistoryMonthly.MonthlyAverage = int(transactionsCount / diff_month)
		}

		httpapi.Success(ctx, transactionsHistoryMonthly)
	}
}

func (handler *StatsHandler) GetGasUsedHistoryDaily(ctx *fasthttp.RequestCtx) {
	// Fetch total gas used history of the year
	first_day_of_year := time.Date(time.Now().Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
	date_range := int(time.Since(first_day_of_year).Hours() / 24)
	totalGasUsedHistoryList, err := handler.chainStatsView.GetGasUsedHistoryByDateRange(date_range)

	if err != nil {
		handler.logger.Errorf("error fetching gas used history: %v", err)
		httpapi.InternalServerError(ctx)
		return
	}

	httpapi.Success(ctx, totalGasUsedHistoryList)
}

func (handler *StatsHandler) GetTotalFeeHistoryDaily(ctx *fasthttp.RequestCtx) {
	// Fetch total fee history of the year
	first_day_of_year := time.Date(time.Now().Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
	date_range := int(time.Since(first_day_of_year).Hours() / 24)
	totalGasUsedHistoryList, err := handler.chainStatsView.GetTotalFeeHistoryByDateRange(date_range)

	if err != nil {
		handler.logger.Errorf("error fetching gas used history: %v", err)
		httpapi.InternalServerError(ctx)
		return
	}

	httpapi.Success(ctx, totalGasUsedHistoryList)
}

func (handler *StatsHandler) GetCommonStats(ctx *fasthttp.RequestCtx) {
	commonStatsChan := make(chan blockscout_infrastructure.CommonStats)
	go handler.blockscoutClient.GetCommonStatsAsync(commonStatsChan)

	transactionsCountPerDay, err := handler.blocksView.TotalTransactionsPerDay()
	commonStats := <-commonStatsChan
	commonStats.TransactionStats.Date = time.Now().Local().String()

	if err != nil {
		handler.logger.Errorf("error fetching transactions count per day: %v", err)
		commonStats.TransactionStats.NumberOfTransactions = 0
	} else {
		commonStats.TransactionStats.NumberOfTransactions = transactionsCountPerDay
	}

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

type TransactionsHistoryDaily struct {
	TransactionsHistory []chainstats_view.TransactionHistory `json:"transactionsHistory"`
	DailyAverage        int                                  `json:"dailyAverage"`
}

type TransactionsHistoryMonthly struct {
	TransactionsHistory []chainstats_view.TransactionHistory `json:"transactionsHistory"`
	MonthlyAverage      int                                  `json:"monthlyAverage"`
}
