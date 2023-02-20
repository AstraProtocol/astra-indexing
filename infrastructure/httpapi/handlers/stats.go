package handlers

import (
	"math"
	"math/big"
	"strconv"
	"time"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	"github.com/valyala/fasthttp"

	blockscout_infrastructure "github.com/AstraProtocol/astra-indexing/infrastructure/blockscout"
	"github.com/AstraProtocol/astra-indexing/infrastructure/httpapi"
	"github.com/AstraProtocol/astra-indexing/infrastructure/metric/prometheus"
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
	startTime := time.Now()
	recordMethod := "GetTransactionsHistoryChart"
	// Fetch transactions history of last 30 days
	date_range := 30
	transactionsHistoryList, err := handler.chainStatsView.GetTransactionsHistoryForChart(date_range)

	if err != nil {
		handler.logger.Errorf("error fetching transactions history: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, transactionsHistoryList)
}

func (handler *StatsHandler) GetTransactionsHistory(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetTransactionsHistory"
	// handle api's params
	var fromDate time.Time
	var endDate time.Time

	if string(ctx.QueryArgs().Peek("year")) != "" {
		year, err := strconv.ParseInt(string(ctx.QueryArgs().Peek("year")), 10, 0)
		if err != nil {
			handler.logger.Error("year param is invalid")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}
		if int(year) > time.Now().Year() {
			handler.logger.Error("year is too far")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}
		fromDate = time.Date(int(year), time.January, 1, 0, 0, 0, 0, time.UTC)
		endDate = fromDate.AddDate(1, 0, 0)
	} else {
		fromDate = time.Now().AddDate(0, 0, -365)
		endDate = time.Now().AddDate(0, 0, 1)
	}

	isDaily := false
	if string(ctx.QueryArgs().Peek("daily")) == "true" {
		isDaily = true
	}
	//

	transactionsCount, err := handler.transactionsTotalView.FindBy("-")
	if err != nil {
		handler.logger.Errorf("error fetching transaction count: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	minDate, err := handler.chainStatsView.GetMinDate()
	if err != nil {
		handler.logger.Errorf("error fetching min date of chain_stats: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	minDateTime := time.Unix(0, minDate).UTC()
	diffTime := time.Since(minDateTime)

	transactionsHistoryList, err := handler.chainStatsView.GetTransactionsHistory(fromDate, endDate)
	if err != nil {
		handler.logger.Errorf("error fetching transactions history: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}
	length := len(transactionsHistoryList)

	if isDaily {
		diffDay := int64(math.Ceil(diffTime.Hours() / 24))

		var transactionsHistoryDaily TransactionsHistoryDaily
		transactionsHistoryDaily.TransactionsHistory = transactionsHistoryList
		if length > 0 {
			transactionsHistoryDaily.DailyAverage = float32(transactionsCount / diffDay)
		}

		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.Success(ctx, transactionsHistoryDaily)
		return
	} else {
		if length == 0 {
			httpapi.Success(ctx, transactionsHistoryList)
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
			return
		}

		result := make([]chainstats_view.TransactionHistory, 0)

		checkYear := transactionsHistoryList[0].Year
		checkMonth := transactionsHistoryList[0].Month
		var monthlyTransactions int64

		for index, transactionHistory := range transactionsHistoryList {
			// init counting
			if index == 0 {
				monthlyTransactions = 0
			}
			// change checkYear
			if checkYear != transactionHistory.Year {
				checkYear = transactionHistory.Year
			}
			// counting
			if transactionHistory.Month == checkMonth {
				monthlyTransactions += transactionHistory.NumberOfTransactions
			}
			// add to result then reset counting
			if (index < length-1 && transactionHistory.Month != transactionsHistoryList[index+1].Month) || index == length-1 {
				var transactionHistoryMonthly chainstats_view.TransactionHistory
				transactionHistoryMonthly.Year = checkYear
				transactionHistoryMonthly.Month = checkMonth
				transactionHistoryMonthly.NumberOfTransactions = monthlyTransactions
				result = append(result, transactionHistoryMonthly)

				if index == length-1 {
					break
				}

				monthlyTransactions = 0
				checkMonth = transactionsHistoryList[index+1].Month
			}
		}

		diffMonth := int64(math.Ceil(diffTime.Hours() / (24 * 30)))

		var transactionsHistoryMonthly TransactionsHistoryMonthly
		transactionsHistoryMonthly.TransactionsHistory = result
		if len(result) > 0 {
			transactionsHistoryMonthly.MonthlyAverage = float32(transactionsCount / diffMonth)
		}

		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.Success(ctx, transactionsHistoryMonthly)
	}
}

func (handler *StatsHandler) GetActiveAddressesHistory(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetActiveAddressesHistory"
	// handle api's params
	var fromDate time.Time
	var endDate time.Time

	if string(ctx.QueryArgs().Peek("year")) != "" {
		year, err := strconv.ParseInt(string(ctx.QueryArgs().Peek("year")), 10, 0)
		if err != nil {
			handler.logger.Error("year param is invalid")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}
		if int(year) > time.Now().Year() {
			handler.logger.Error("year is too far")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}
		fromDate = time.Date(int(year), time.January, 1, 0, 0, 0, 0, time.UTC)
		endDate = fromDate.AddDate(1, 0, 0)
	} else {
		fromDate = time.Now().AddDate(0, 0, -365)
		endDate = time.Now().AddDate(0, 0, 1)
	}

	isDaily := false
	if string(ctx.QueryArgs().Peek("daily")) == "true" {
		isDaily = true
	}
	//

	totalActiveAddresses, err := handler.chainStatsView.GetTotalActiveAddresses()
	if err != nil {
		handler.logger.Errorf("error fetching total active addresses: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	minDate, err := handler.chainStatsView.GetMinDate()
	if err != nil {
		handler.logger.Errorf("error fetching min date of chain_stats: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	minDateTime := time.Unix(0, minDate).UTC()
	diffTime := time.Since(minDateTime)

	activeAddressesHistoryList, err := handler.chainStatsView.GetActiveAddressesHistory(fromDate, endDate)
	if err != nil {
		handler.logger.Errorf("error fetching active addresses history: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}
	length := len(activeAddressesHistoryList)

	if isDaily {
		diff_day := int64(math.Ceil(diffTime.Hours() / 24))

		var activeAddressesHistoryDaily ActiveAddressesHistoryDaily
		activeAddressesHistoryDaily.ActiveAddressesHistory = activeAddressesHistoryList
		if length > 0 {
			activeAddressesHistoryDaily.DailyAverage = float32(totalActiveAddresses / diff_day)
		}

		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.Success(ctx, activeAddressesHistoryDaily)
		return
	} else {
		if length == 0 {
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.Success(ctx, activeAddressesHistoryList)
			return
		}

		result := make([]chainstats_view.ActiveAddressHistory, 0)

		checkYear := activeAddressesHistoryList[0].Year
		checkMonth := activeAddressesHistoryList[0].Month
		var monthlyActiveAddresses int64

		for index, activeAddressesHistory := range activeAddressesHistoryList {
			// init counting
			if index == 0 {
				monthlyActiveAddresses = 0
			}
			// change checkYear
			if checkYear != activeAddressesHistory.Year {
				checkYear = activeAddressesHistory.Year
			}
			// counting
			if activeAddressesHistory.Month == checkMonth {
				monthlyActiveAddresses += activeAddressesHistory.NumberOfActiveAddresses
			}
			// add to result then reset counting
			if (index < length-1 && activeAddressesHistory.Month != activeAddressesHistoryList[index+1].Month) || index == length-1 {
				var activeAddressesHistoryMonthly chainstats_view.ActiveAddressHistory
				activeAddressesHistoryMonthly.Year = checkYear
				activeAddressesHistoryMonthly.Month = checkMonth
				activeAddressesHistoryMonthly.NumberOfActiveAddresses = monthlyActiveAddresses
				result = append(result, activeAddressesHistoryMonthly)

				if index == length-1 {
					break
				}

				monthlyActiveAddresses = 0
				checkMonth = activeAddressesHistoryList[index+1].Month
			}
		}

		diffMonth := int64(math.Ceil(diffTime.Hours() / (24 * 30)))

		var activeAddressesHistoryMonthly ActiveAddressesHistoryMonthly
		activeAddressesHistoryMonthly.ActiveAddressesHistory = result
		if len(result) > 0 {
			activeAddressesHistoryMonthly.MonthlyAverage = float32(totalActiveAddresses / diffMonth)
		}

		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.Success(ctx, activeAddressesHistoryMonthly)
	}
}

func (handler *StatsHandler) GetTotalAddressesGrowth(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetTotalAddressesGrowth"
	// handle api's params
	var fromDate time.Time
	var endDate time.Time

	if string(ctx.QueryArgs().Peek("year")) != "" {
		year, err := strconv.ParseInt(string(ctx.QueryArgs().Peek("year")), 10, 0)
		if err != nil {
			handler.logger.Error("year param is invalid")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}
		if int(year) > time.Now().Year() {
			handler.logger.Error("year is too far")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}
		fromDate = time.Date(int(year), time.January, 1, 0, 0, 0, 0, time.UTC)
		endDate = fromDate.AddDate(1, 0, 0)
	} else {
		fromDate = time.Now().AddDate(0, 0, -365)
		endDate = time.Now().AddDate(0, 0, 1)
	}

	if string(ctx.QueryArgs().Peek("daily")) != "true" {
		handler.logger.Error("only implemented for show daily")
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}
	//

	addressesCount, err := handler.accountsView.TotalAccount()
	if err != nil {
		handler.logger.Errorf("error fetching address count: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	totalAddressesHistoryList, err := handler.chainStatsView.GetTotalAddressesGrowth(fromDate, endDate)

	if err != nil {
		handler.logger.Errorf("error fetching total addresses history daily: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	if len(totalAddressesHistoryList) > 0 && totalAddressesHistoryList[0].Total == 0 {
		totalAddressesHistoryList[0].Total = addressesCount
		totalAddressesHistoryList[0].NotActive = addressesCount - totalAddressesHistoryList[0].Active
	}

	var totalAddressesHistoryDaily TotalAddressesGrowth
	totalAddressesHistoryDaily.TotalAddressesGrowth = totalAddressesHistoryList
	totalAddressesHistoryDaily.TotalAddresses = addressesCount

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, totalAddressesHistoryDaily)
}

func (handler *StatsHandler) GetGasUsedHistory(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetGasUsedHistory"
	// handle api's params
	var fromDate time.Time
	var endDate time.Time

	if string(ctx.QueryArgs().Peek("year")) != "" {
		year, err := strconv.ParseInt(string(ctx.QueryArgs().Peek("year")), 10, 0)
		if err != nil {
			handler.logger.Error("year param is invalid")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}
		if int(year) > time.Now().Year() {
			handler.logger.Error("year is too far")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}
		fromDate = time.Date(int(year), time.January, 1, 0, 0, 0, 0, time.UTC)
		endDate = fromDate.AddDate(1, 0, 0)
	} else {
		fromDate = time.Now().AddDate(0, 0, -365)
		endDate = time.Now().AddDate(0, 0, 1)
	}

	isDaily := false
	if string(ctx.QueryArgs().Peek("daily")) == "true" {
		isDaily = true
	}
	//

	totalGasUsed, err := handler.chainStatsView.GetTotalGasUsed()
	if err != nil {
		handler.logger.Errorf("error fetching total gas used: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	minDate, err := handler.chainStatsView.GetMinDate()
	if err != nil {
		handler.logger.Errorf("error fetching min date of chain_stats: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	minDateTime := time.Unix(0, minDate).UTC()
	diffTime := time.Since(minDateTime)

	totalGasUsedList, err := handler.chainStatsView.GetGasUsedHistory(fromDate, endDate)
	if err != nil {
		handler.logger.Errorf("error fetching total gas used history: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}
	length := len(totalGasUsedList)

	if isDaily {
		diffDay := int64(math.Ceil(diffTime.Hours() / 24))

		var totalGasUsedHistoryDaily TotalGasUsedHistoryDaily
		totalGasUsedHistoryDaily.TotalGasUsedHistory = totalGasUsedList
		if length > 0 {
			totalGasUsedHistoryDaily.DailyAverage = float32(totalGasUsed / diffDay)
		}

		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.Success(ctx, totalGasUsedHistoryDaily)
		return
	} else {
		if length == 0 {
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
			httpapi.Success(ctx, totalGasUsedList)
			return
		}

		result := make([]chainstats_view.TotalGasUsedHistory, 0)

		checkYear := totalGasUsedList[0].Year
		checkMonth := totalGasUsedList[0].Month
		var monthlyTotalGasUsed int64

		for index, totalGasUsedHistory := range totalGasUsedList {
			// init counting
			if index == 0 {
				monthlyTotalGasUsed = 0
			}
			// change checkYear
			if checkYear != totalGasUsedHistory.Year {
				checkYear = totalGasUsedHistory.Year
			}
			// counting
			if totalGasUsedHistory.Month == checkMonth {
				monthlyTotalGasUsed += totalGasUsedHistory.TotalGasUsed
			}
			// add to result then reset counting
			if (index < length-1 && totalGasUsedHistory.Month != totalGasUsedList[index+1].Month) || index == length-1 {
				var totalGasUsedHistoryMonthly chainstats_view.TotalGasUsedHistory
				totalGasUsedHistoryMonthly.Year = checkYear
				totalGasUsedHistoryMonthly.Month = checkMonth
				totalGasUsedHistoryMonthly.TotalGasUsed = monthlyTotalGasUsed
				result = append(result, totalGasUsedHistoryMonthly)

				if index == length-1 {
					break
				}

				monthlyTotalGasUsed = 0
				checkMonth = totalGasUsedList[index+1].Month
			}
		}

		diffMonth := int64(math.Ceil(diffTime.Hours() / (24 * 30)))

		var totalGasUsedHistoryMonthly TotalGasUsedHistoryMonthly
		totalGasUsedHistoryMonthly.TotalGasUsedHistory = result
		if len(result) > 0 {
			totalGasUsedHistoryMonthly.MonthlyAverage = float32(totalGasUsed / diffMonth)
		}

		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.Success(ctx, totalGasUsedHistoryMonthly)
	}
}

func (handler *StatsHandler) GetTotalFeeHistory(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetTotalFeeHistory"
	// handle api's params
	var fromDate time.Time
	var endDate time.Time

	if string(ctx.QueryArgs().Peek("year")) != "" {
		year, err := strconv.ParseInt(string(ctx.QueryArgs().Peek("year")), 10, 0)
		if err != nil {
			handler.logger.Error("year param is invalid")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}
		if int(year) > time.Now().Year() {
			handler.logger.Error("year is too far")
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
			httpapi.InternalServerError(ctx)
			return
		}
		fromDate = time.Date(int(year), time.January, 1, 0, 0, 0, 0, time.UTC)
		endDate = fromDate.AddDate(1, 0, 0)
	} else {
		fromDate = time.Now().AddDate(0, 0, -365)
		endDate = time.Now().AddDate(0, 0, 1)
	}

	isDaily := false
	if string(ctx.QueryArgs().Peek("daily")) == "true" {
		isDaily = true
	}
	//

	totalTransactionFees, err := handler.chainStatsView.GetTotalTransactionFees()
	if err != nil {
		handler.logger.Errorf("error fetching total transaction fees: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	minDate, err := handler.chainStatsView.GetMinDate()
	if err != nil {
		handler.logger.Errorf("error fetching min date of chain_stats: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	minDateTime := time.Unix(0, minDate).UTC()
	diffTime := time.Since(minDateTime)

	totalFeesHistoryList, err := handler.chainStatsView.GetTotalFeeHistory(fromDate, endDate)
	if err != nil {
		handler.logger.Errorf("error fetching total fee history: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}
	length := len(totalFeesHistoryList)

	if isDaily {
		diffDay := int64(math.Ceil(diffTime.Hours() / 24))

		for index := range totalFeesHistoryList {
			if index == length-1 {
				break
			}
			totalFeesHistoryList[index].TotalTransactionFees = big.NewInt(0).Sub(
				totalFeesHistoryList[index].TotalTransactionFees, totalFeesHistoryList[index+1].TotalTransactionFees,
			)
		}

		var totalFeesHistoryDaily TotalFeesHistoryDaily
		totalFeesHistoryDaily.TotalFeesHistory = totalFeesHistoryList
		if length > 0 {
			totalFeesHistoryDaily.DailyAverage = big.NewInt(0).Div(totalTransactionFees, big.NewInt(diffDay))
		}

		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.Success(ctx, totalFeesHistoryDaily)
		return
	} else {
		if length == 0 {
			prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
			httpapi.Success(ctx, totalFeesHistoryList)
			return
		}

		result := make([]chainstats_view.TotalFeeHistory, 0)

		checkYear := totalFeesHistoryList[0].Year
		checkMonth := totalFeesHistoryList[0].Month
		checkTotalTransactionFees := totalFeesHistoryList[0].TotalTransactionFees

		var totalTransactionFeesMonthly *big.Int

		for index, totalFeesHistory := range totalFeesHistoryList {
			// change checkYear
			if checkYear != totalFeesHistory.Year {
				checkYear = totalFeesHistory.Year
			}

			// add to result
			if index < length-1 && totalFeesHistory.Month != totalFeesHistoryList[index+1].Month {
				var totalFeesHistoryMonthly chainstats_view.TotalFeeHistory
				totalFeesHistoryMonthly.Year = checkYear
				totalFeesHistoryMonthly.Month = checkMonth

				totalTransactionFeesMonthly = big.NewInt(0).Sub(
					checkTotalTransactionFees, totalFeesHistoryList[index+1].TotalTransactionFees,
				)

				totalFeesHistoryMonthly.TotalTransactionFees = totalTransactionFeesMonthly

				result = append(result, totalFeesHistoryMonthly)

				checkMonth = totalFeesHistoryList[index+1].Month
				checkTotalTransactionFees = totalFeesHistoryList[index+1].TotalTransactionFees
			}

			if index == length-1 {
				var totalFeesHistoryMonthly chainstats_view.TotalFeeHistory
				totalFeesHistoryMonthly.Year = checkYear
				totalFeesHistoryMonthly.Month = checkMonth

				totalFeesHistoryMonthly.TotalTransactionFees = checkTotalTransactionFees

				result = append(result, totalFeesHistoryMonthly)
			}
		}

		diffMonth := int64(math.Ceil(diffTime.Hours() / (24 * 30)))

		var totalFeesHistoryMonthly TotalFeesHistoryMonthly
		totalFeesHistoryMonthly.TotalFeesHistory = result
		if len(result) > 0 {
			totalFeesHistoryMonthly.MonthlyAverage = big.NewInt(0).Div(totalTransactionFees, big.NewInt(diffMonth))
		}

		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.Success(ctx, totalFeesHistoryMonthly)
	}
}

func (handler *StatsHandler) GetCommonStats(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetCommonStats"

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

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, commonStats)
}

func (handler *StatsHandler) EstimateCounted(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "EstimateCounted"

	blocksCount, err := handler.blocksView.Count()
	if err != nil {
		handler.logger.Errorf("error fetching block count: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	transactionsCount, err := handler.transactionsTotalView.FindBy("-")
	if err != nil {
		handler.logger.Errorf("error fetching transaction count: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	addressesCount, err := handler.accountsView.TotalAccount()
	if err != nil {
		handler.logger.Errorf("error fetching address count: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	estimateCounted := EstimateCountedInfo{}
	estimateCounted.TotalTransactions = transactionsCount
	estimateCounted.TotalBlocks = blocksCount
	estimateCounted.TotalAddresses = addressesCount

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, estimateCounted)
}

func (handler *StatsHandler) MarketHistoryChart(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "MarketHistoryChart"

	marketHistoryChart, err := handler.blockscoutClient.MarketHistoryChart()
	if err != nil {
		handler.logger.Errorf("error getting market history chart from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, marketHistoryChart)
}

func (handler *StatsHandler) GasPriceOracle(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GasPriceOracle"

	gasPriceOracle, err := handler.blockscoutClient.GasPriceOracle()
	if err != nil {
		handler.logger.Errorf("error getting gas price oracle from blockscout: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(-1), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	httpapi.Success(ctx, gasPriceOracle)
}

type EstimateCountedInfo struct {
	TotalBlocks       int64 `json:"total_blocks"`
	TotalTransactions int64 `json:"total_transactions"`
	TotalAddresses    int64 `json:"wallet_addresses"`
}

type TransactionsHistoryDaily struct {
	TransactionsHistory []chainstats_view.TransactionHistory `json:"transactionsHistory"`
	DailyAverage        float32                              `json:"dailyAverage"`
}

type TransactionsHistoryMonthly struct {
	TransactionsHistory []chainstats_view.TransactionHistory `json:"transactionsHistory"`
	MonthlyAverage      float32                              `json:"monthlyAverage"`
}

type ActiveAddressesHistoryDaily struct {
	ActiveAddressesHistory []chainstats_view.ActiveAddressHistory `json:"activeAddressesHistory"`
	DailyAverage           float32                                `json:"dailyAverage"`
}

type ActiveAddressesHistoryMonthly struct {
	ActiveAddressesHistory []chainstats_view.ActiveAddressHistory `json:"activeAddressesHistory"`
	MonthlyAverage         float32                                `json:"monthlyAverage"`
}

type TotalAddressesGrowth struct {
	TotalAddressesGrowth []chainstats_view.TotalAddressGrowth `json:"totalAddressesGrowth"`
	TotalAddresses       int64                                `json:"totalAddresses"`
}

type TotalGasUsedHistoryDaily struct {
	TotalGasUsedHistory []chainstats_view.TotalGasUsedHistory `json:"totalGasUsedHistory"`
	DailyAverage        float32                               `json:"dailyAverage"`
}

type TotalGasUsedHistoryMonthly struct {
	TotalGasUsedHistory []chainstats_view.TotalGasUsedHistory `json:"totalGasUsedHistory"`
	MonthlyAverage      float32                               `json:"monthlyAverage"`
}

type TotalFeesHistoryDaily struct {
	TotalFeesHistory []chainstats_view.TotalFeeHistory `json:"totalFeesHistory"`
	DailyAverage     *big.Int                          `json:"dailyAverage"`
}

type TotalFeesHistoryMonthly struct {
	TotalFeesHistory []chainstats_view.TotalFeeHistory `json:"totalFeesHistory"`
	MonthlyAverage   *big.Int                          `json:"monthlyAverage"`
}
