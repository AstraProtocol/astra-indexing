package view

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/external/cache"
	"github.com/AstraProtocol/astra-indexing/infrastructure/metric/prometheus"
)

type ChainStats struct {
	rdbHandle  *rdb.Handle
	astraCache *cache.AstraCache
}

func NewChainStats(rdbHandle *rdb.Handle) *ChainStats {
	return &ChainStats{
		rdbHandle,
		cache.NewCache("chain_stats"),
	}
}

func (view *ChainStats) Set(metrics string, value string) error {
	var err error

	var sql string
	var sqlArgs []interface{}

	sql, sqlArgs, err = view.rdbHandle.StmtBuilder.Select(
		"1",
	).From(
		"view_chain_stats",
	).Where(
		"metrics = ?", metrics,
	).ToSql()
	if err != nil {
		return fmt.Errorf("error preparing metrics selection SQL: %v", err)
	}
	var placeholder int
	err = view.rdbHandle.QueryRow(sql, sqlArgs...).Scan(&placeholder)
	if err != nil {
		if !errors.Is(err, rdb.ErrNoRows) {
			return fmt.Errorf("error scanning metrics: %v", err)
		}
		sql, sqlArgs, err = view.rdbHandle.StmtBuilder.Insert(
			"view_chain_stats",
		).Columns(
			"metrics",
			"value",
		).Values(metrics, value).ToSql()
		if err != nil {
			return fmt.Errorf("error building metrics insertion sql: %v: %w", err, rdb.ErrBuildSQLStmt)
		}

		var execResult rdb.ExecResult
		if execResult, err = view.rdbHandle.Exec(sql, sqlArgs...); err != nil {
			return fmt.Errorf("error inserting metrics: %v", err)
		}
		if execResult.RowsAffected() != 1 {
			return errors.New("error inserting metrics: no rows inserted")
		}

		return nil
	}

	sql, sqlArgs, err = view.rdbHandle.StmtBuilder.Update(
		"view_chain_stats",
	).Set(
		"value", value,
	).Where(
		"metrics = ?", metrics,
	).ToSql()
	if err != nil {
		return fmt.Errorf("error building metrics update sql: %v", err)
	}

	var execResult rdb.ExecResult
	if execResult, err = view.rdbHandle.Exec(sql, sqlArgs...); err != nil {
		return fmt.Errorf("error updating metrics: %v", err)
	}
	if execResult.RowsAffected() != 1 {
		return errors.New("error updating metrics: no rows updated")
	}

	return nil
}

func (view *ChainStats) FindBy(metrics string) (string, error) {
	sql, sqlArgs, err := view.rdbHandle.StmtBuilder.Select(
		"value",
	).From(
		"view_chain_stats",
	).Where(
		"metrics = ?", metrics,
	).ToSql()
	if err != nil {
		return "", fmt.Errorf("error preparing metrics selection SQL: %v", err)
	}

	var value string
	if err := view.rdbHandle.QueryRow(sql, sqlArgs...).Scan(&value); err != nil {
		if errors.Is(err, rdb.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("error getting metrics: %v", err)
	}

	return value, nil
}

func (view *ChainStats) GetTransactionsHistoryForChart(date_range int) ([]TransactionHistory, error) {
	cacheKey := fmt.Sprintf("GetTransactionsHistoryForChart_%d", date_range)
	tmpTransactionHistoryList := []TransactionHistory{}

	err := view.astraCache.Get(cacheKey, &tmpTransactionHistoryList)
	if err == nil {
		return tmpTransactionHistoryList, nil
	}

	startTime := time.Now()
	recordMethod := "GetTransactionsHistoryForChart"

	latest := time.Now().Truncate(24 * time.Hour)
	earliest := latest.Add(-time.Duration(date_range) * 24 * time.Hour)

	sql, sqlArgs, err := view.rdbHandle.StmtBuilder.Select(
		"date_time",
		"number_of_transactions",
	).From(
		"chain_stats",
	).Where(
		"date_time >= ? AND date_time <= ?", earliest.UnixNano(), latest.UnixNano(),
	).OrderBy(
		"date_time DESC",
	).ToSql()
	if err != nil {
		return nil, fmt.Errorf("error building transactions history by date range select SQL: %v, %w", err, rdb.ErrBuildSQLStmt)
	}

	rowsResult, err := view.rdbHandle.Query(sql, sqlArgs...)
	if err != nil {
		return nil, fmt.Errorf("error executing transactions history by date range select SQL: %v: %w", err, rdb.ErrQuery)
	}
	defer rowsResult.Close()

	transactionHistoryList := make([]TransactionHistory, 0)
	for rowsResult.Next() {
		var transactionHistory TransactionHistory
		var unixTime int64

		if err = rowsResult.Scan(
			&unixTime,
			&transactionHistory.NumberOfTransactions,
		); err != nil {
			if errors.Is(err, rdb.ErrNoRows) {
				return nil, rdb.ErrNoRows
			}
			return nil, fmt.Errorf("error scanning transactions history by date range row: %v: %w", err, rdb.ErrQuery)
		}

		transactionHistory.Date = strings.Split(time.Unix(0, unixTime).UTC().String(), " ")[0]
		transactionHistoryList = append(transactionHistoryList, transactionHistory)
	}

	prometheus.RecordApiExecTime(recordMethod, "chainstats", "query", time.Since(startTime).Milliseconds())

	view.astraCache.Set(cacheKey, transactionHistoryList, 5*60*1000*time.Millisecond)

	return transactionHistoryList, nil
}

func (view *ChainStats) GetTransactionsHistory(from_date time.Time, end_date time.Time) ([]TransactionHistory, error) {
	cacheKey := fmt.Sprintf("GetTransactionsHistory_%d_%d", from_date.UnixNano(), end_date.UnixNano())
	tmpTransactionHistoryList := []TransactionHistory{}

	err := view.astraCache.Get(cacheKey, &tmpTransactionHistoryList)
	if err == nil {
		return tmpTransactionHistoryList, nil
	}

	startTime := time.Now()
	recordMethod := "GetTransactionsHistory"

	sql, sqlArgs, err := view.rdbHandle.StmtBuilder.Select(
		"date_time",
		"number_of_transactions",
	).From(
		"chain_stats",
	).Where(
		"date_time >= ? AND date_time < ?", from_date.UnixNano(), end_date.UnixNano(),
	).OrderBy(
		"date_time DESC",
	).ToSql()
	if err != nil {
		return nil, fmt.Errorf("error building transactions history by date range select SQL: %v, %w", err, rdb.ErrBuildSQLStmt)
	}

	rowsResult, err := view.rdbHandle.Query(sql, sqlArgs...)
	if err != nil {
		return nil, fmt.Errorf("error executing transactions history by date range select SQL: %v: %w", err, rdb.ErrQuery)
	}
	defer rowsResult.Close()

	transactionHistoryList := make([]TransactionHistory, 0)
	for rowsResult.Next() {
		var transactionHistory TransactionHistory
		var unixTime int64

		if err = rowsResult.Scan(
			&unixTime,
			&transactionHistory.NumberOfTransactions,
		); err != nil {
			if errors.Is(err, rdb.ErrNoRows) {
				return nil, rdb.ErrNoRows
			}
			return nil, fmt.Errorf("error scanning transactions history by date range row: %v: %w", err, rdb.ErrQuery)
		}

		transactionHistory.Date = strings.Split(time.Unix(0, unixTime).UTC().String(), " ")[0]
		transactionHistory.Month = strings.Split(transactionHistory.Date, "-")[1]
		transactionHistory.Year = strings.Split(transactionHistory.Date, "-")[0]
		transactionHistoryList = append(transactionHistoryList, transactionHistory)
	}

	prometheus.RecordApiExecTime(recordMethod, "chainstats", "query", time.Since(startTime).Milliseconds())

	view.astraCache.Set(cacheKey, transactionHistoryList, 10*60*1000*time.Millisecond)

	return transactionHistoryList, nil
}

func (view *ChainStats) GetActiveAddressesHistory(from_date time.Time, end_date time.Time) ([]ActiveAddressHistory, error) {
	cacheKey := fmt.Sprintf("GetActiveAddressesHistory_%d_%d", from_date.UnixNano(), end_date.UnixNano())
	tmpActiveAddressHistoryList := []ActiveAddressHistory{}

	err := view.astraCache.Get(cacheKey, &tmpActiveAddressHistoryList)
	if err == nil {
		return tmpActiveAddressHistoryList, nil
	}

	startTime := time.Now()
	recordMethod := "GetActiveAddressesHistory"

	sql, sqlArgs, err := view.rdbHandle.StmtBuilder.Select(
		"date_time",
		"active_addresses",
	).From(
		"chain_stats",
	).Where(
		"date_time >= ? AND date_time < ?", from_date.UnixNano(), end_date.UnixNano(),
	).OrderBy(
		"date_time DESC",
	).ToSql()
	if err != nil {
		return nil, fmt.Errorf("error building active addresses history by date range select SQL: %v, %w", err, rdb.ErrBuildSQLStmt)
	}

	rowsResult, err := view.rdbHandle.Query(sql, sqlArgs...)
	if err != nil {
		return nil, fmt.Errorf("error executing active addresses history by date range select SQL: %v: %w", err, rdb.ErrQuery)
	}
	defer rowsResult.Close()

	activeAddressHistoryList := make([]ActiveAddressHistory, 0)
	for rowsResult.Next() {
		var activeAddressHistory ActiveAddressHistory
		var unixTime int64

		if err = rowsResult.Scan(
			&unixTime,
			&activeAddressHistory.NumberOfActiveAddresses,
		); err != nil {
			if errors.Is(err, rdb.ErrNoRows) {
				return nil, rdb.ErrNoRows
			}
			return nil, fmt.Errorf("error scanning active addresses history by date range row: %v: %w", err, rdb.ErrQuery)
		}

		activeAddressHistory.Date = strings.Split(time.Unix(0, unixTime).UTC().String(), " ")[0]
		activeAddressHistory.Month = strings.Split(activeAddressHistory.Date, "-")[1]
		activeAddressHistory.Year = strings.Split(activeAddressHistory.Date, "-")[0]
		activeAddressHistoryList = append(activeAddressHistoryList, activeAddressHistory)
	}

	prometheus.RecordApiExecTime(recordMethod, "chainstats", "query", time.Since(startTime).Milliseconds())

	view.astraCache.Set(cacheKey, activeAddressHistoryList, 10*60*1000*time.Millisecond)

	return activeAddressHistoryList, nil
}

func (view *ChainStats) GetTotalAddressesGrowth(from_date time.Time, end_date time.Time) ([]TotalAddressGrowth, error) {
	cacheKey := fmt.Sprintf("GetTotalAddressesGrowth_%d_%d", from_date.UnixNano(), end_date.UnixNano())
	tmpTotalAddressGrowthList := []TotalAddressGrowth{}

	err := view.astraCache.Get(cacheKey, &tmpTotalAddressGrowthList)
	if err == nil {
		return tmpTotalAddressGrowthList, nil
	}

	startTime := time.Now()
	recordMethod := "GetTotalAddressesGrowth"

	sql, sqlArgs, err := view.rdbHandle.StmtBuilder.Select(
		"date_time",
		"active_addresses",
		"total_addresses",
	).From(
		"chain_stats",
	).Where(
		"date_time >= ? AND date_time < ?", from_date.UnixNano(), end_date.UnixNano(),
	).OrderBy(
		"date_time DESC",
	).ToSql()
	if err != nil {
		return nil, fmt.Errorf("error building total addresses growth by date range select SQL: %v, %w", err, rdb.ErrBuildSQLStmt)
	}

	rowsResult, err := view.rdbHandle.Query(sql, sqlArgs...)
	if err != nil {
		return nil, fmt.Errorf("error executing total addresses growth by date range select SQL: %v: %w", err, rdb.ErrQuery)
	}
	defer rowsResult.Close()

	totalAddressGrowthList := make([]TotalAddressGrowth, 0)
	for rowsResult.Next() {
		var totalAddressGrowth TotalAddressGrowth
		var unixTime int64

		if err = rowsResult.Scan(
			&unixTime,
			&totalAddressGrowth.Active,
			&totalAddressGrowth.Total,
		); err != nil {
			if errors.Is(err, rdb.ErrNoRows) {
				return nil, rdb.ErrNoRows
			}
			return nil, fmt.Errorf("error scanning total addresses growth by date range row: %v: %w", err, rdb.ErrQuery)
		}
		totalAddressGrowth.NotActive = totalAddressGrowth.Total - totalAddressGrowth.Active
		totalAddressGrowth.Date = strings.Split(time.Unix(0, unixTime).UTC().String(), " ")[0]
		totalAddressGrowth.Month = strings.Split(totalAddressGrowth.Date, "-")[1]
		totalAddressGrowth.Year = strings.Split(totalAddressGrowth.Date, "-")[0]
		totalAddressGrowthList = append(totalAddressGrowthList, totalAddressGrowth)
	}

	prometheus.RecordApiExecTime(recordMethod, "chainstats", "query", time.Since(startTime).Milliseconds())

	view.astraCache.Set(cacheKey, totalAddressGrowthList, 10*60*1000*time.Millisecond)

	return totalAddressGrowthList, nil
}

func (view *ChainStats) GetGasUsedHistory(from_date time.Time, end_date time.Time) ([]TotalGasUsedHistory, error) {
	cacheKey := fmt.Sprintf("GetGasUsedHistory_%d_%d", from_date.UnixNano(), end_date.UnixNano())
	tmpTotalGasUsedHistoryList := []TotalGasUsedHistory{}

	err := view.astraCache.Get(cacheKey, &tmpTotalGasUsedHistoryList)
	if err == nil {
		return tmpTotalGasUsedHistoryList, nil
	}

	startTime := time.Now()
	recordMethod := "GetGasUsedHistory"

	sql, sqlArgs, err := view.rdbHandle.StmtBuilder.Select(
		"date_time",
		"total_gas_used",
	).From(
		"chain_stats",
	).Where(
		"date_time >= ? AND date_time < ?", from_date.UnixNano(), end_date.UnixNano(),
	).OrderBy(
		"date_time DESC",
	).ToSql()
	if err != nil {
		return nil, fmt.Errorf("error building total gas used history by date range select SQL: %v, %w", err, rdb.ErrBuildSQLStmt)
	}

	rowsResult, err := view.rdbHandle.Query(sql, sqlArgs...)
	if err != nil {
		return nil, fmt.Errorf("error executing total gas used history by date range select SQL: %v: %w", err, rdb.ErrQuery)
	}
	defer rowsResult.Close()

	totalGasUsedHistoryList := make([]TotalGasUsedHistory, 0)
	for rowsResult.Next() {
		var totalGasUsedHistory TotalGasUsedHistory
		var unixTime int64

		if err = rowsResult.Scan(
			&unixTime,
			&totalGasUsedHistory.TotalGasUsed,
		); err != nil {
			if errors.Is(err, rdb.ErrNoRows) {
				return nil, rdb.ErrNoRows
			}
			return nil, fmt.Errorf("error scanning total gas used history by date range row: %v: %w", err, rdb.ErrQuery)
		}

		totalGasUsedHistory.Date = strings.Split(time.Unix(0, unixTime).UTC().String(), " ")[0]
		totalGasUsedHistory.Month = strings.Split(totalGasUsedHistory.Date, "-")[1]
		totalGasUsedHistory.Year = strings.Split(totalGasUsedHistory.Date, "-")[0]
		totalGasUsedHistoryList = append(totalGasUsedHistoryList, totalGasUsedHistory)
	}

	prometheus.RecordApiExecTime(recordMethod, "chainstats", "query", time.Since(startTime).Milliseconds())

	view.astraCache.Set(cacheKey, totalGasUsedHistoryList, 10*60*1000*time.Millisecond)

	return totalGasUsedHistoryList, nil
}

func (view *ChainStats) GetTotalFeeHistory(from_date time.Time, end_date time.Time) ([]TotalFeeHistory, error) {
	cacheKey := fmt.Sprintf("GetTotalFeeHistory_%d_%d", from_date.UnixNano(), end_date.UnixNano())
	tmpTotalFeeHistoryList := []TotalFeeHistory{}

	err := view.astraCache.Get(cacheKey, &tmpTotalFeeHistoryList)
	if err == nil {
		return tmpTotalFeeHistoryList, nil
	}

	startTime := time.Now()
	recordMethod := "GetTotalFeeHistory"

	sql, sqlArgs, err := view.rdbHandle.StmtBuilder.Select(
		"date_time",
		"CAST(total_fee AS VARCHAR)",
	).From(
		"chain_stats",
	).Where(
		"date_time >= ? AND date_time < ?", from_date.UnixNano(), end_date.UnixNano(),
	).OrderBy(
		"date_time DESC",
	).ToSql()
	if err != nil {
		return nil, fmt.Errorf("error building total transaction fees history by date range select SQL: %v, %w", err, rdb.ErrBuildSQLStmt)
	}

	rowsResult, err := view.rdbHandle.Query(sql, sqlArgs...)
	if err != nil {
		return nil, fmt.Errorf("error executing total transaction fees history by date range select SQL: %v: %w", err, rdb.ErrQuery)
	}
	defer rowsResult.Close()

	totalFeeHistoryList := make([]TotalFeeHistory, 0)
	for rowsResult.Next() {
		var totalFeeHistory TotalFeeHistory
		var totalTransactionFees string
		var unixTime int64

		if err = rowsResult.Scan(
			&unixTime,
			&totalTransactionFees,
		); err != nil {
			if errors.Is(err, rdb.ErrNoRows) {
				return nil, rdb.ErrNoRows
			}
			return nil, fmt.Errorf("error scanning total transactions fee history by date range row: %v: %w", err, rdb.ErrQuery)
		}

		result, isValid := big.NewInt(0).SetString(totalTransactionFees, 10)
		if !isValid {
			result = big.NewInt(0)
		}
		totalFeeHistory.TotalTransactionFees = result
		totalFeeHistory.Date = strings.Split(time.Unix(0, unixTime).UTC().String(), " ")[0]
		totalFeeHistory.Month = strings.Split(totalFeeHistory.Date, "-")[1]
		totalFeeHistory.Year = strings.Split(totalFeeHistory.Date, "-")[0]
		totalFeeHistoryList = append(totalFeeHistoryList, totalFeeHistory)
	}

	prometheus.RecordApiExecTime(recordMethod, "chainstats", "query", time.Since(startTime).Milliseconds())

	view.astraCache.Set(cacheKey, totalFeeHistoryList, 10*60*1000*time.Millisecond)

	return totalFeeHistoryList, nil
}

func (view *ChainStats) GetMinDate() (int64, error) {
	cacheKey := "GetMinDate"
	var tmpMinDate int64

	err := view.astraCache.Get(cacheKey, &tmpMinDate)
	if err == nil {
		return tmpMinDate, nil
	}

	startTime := time.Now()
	recordMethod := "GetMinDate"

	sql, _, err := view.rdbHandle.StmtBuilder.Select("MIN(date_time)").From(
		"chain_stats",
	).ToSql()
	if err != nil {
		return 0, fmt.Errorf("error building min date selection sql: %v", err)
	}

	result := view.rdbHandle.QueryRow(sql)
	var minDate *int64
	if err := result.Scan(&minDate); err != nil {
		return 0, fmt.Errorf("error scanning min date selection query: %v", err)
	}

	if minDate == nil {
		return 0, nil
	}

	prometheus.RecordApiExecTime(recordMethod, "chainstats", "query", time.Since(startTime).Milliseconds())

	view.astraCache.Set(cacheKey, *minDate, 365*24*time.Hour)

	return *minDate, nil
}

func (view *ChainStats) GetTotalActiveAddresses() (int64, error) {
	cacheKey := "GetTotalActiveAddresses"
	var tmpTotalActiveAddresses int64

	err := view.astraCache.Get(cacheKey, &tmpTotalActiveAddresses)
	if err == nil {
		return tmpTotalActiveAddresses, nil
	}

	startTime := time.Now()
	recordMethod := "GetTotalActiveAddresses"

	sql, _, err := view.rdbHandle.StmtBuilder.Select("SUM(active_addresses)").From(
		"chain_stats",
	).ToSql()
	if err != nil {
		return 0, fmt.Errorf("error building total active addresses selection sql: %v", err)
	}

	result := view.rdbHandle.QueryRow(sql)
	var total *int64
	if err := result.Scan(&total); err != nil {
		return 0, fmt.Errorf("error scanning total active addresses selection query: %v", err)
	}

	if total == nil {
		return 0, nil
	}

	prometheus.RecordApiExecTime(recordMethod, "chainstats", "query", time.Since(startTime).Milliseconds())

	view.astraCache.Set(cacheKey, *total, 10*60*1000*time.Millisecond)

	return *total, nil
}

func (view *ChainStats) GetTotalGasUsed() (int64, error) {
	cacheKey := "GetTotalGasUsed"
	var tmpTotalGasUsed int64

	err := view.astraCache.Get(cacheKey, &tmpTotalGasUsed)
	if err == nil {
		return tmpTotalGasUsed, nil
	}

	startTime := time.Now()
	recordMethod := "GetTotalGasUsed"

	sql, _, err := view.rdbHandle.StmtBuilder.Select("SUM(total_gas_used)").From(
		"chain_stats",
	).ToSql()
	if err != nil {
		return 0, fmt.Errorf("error building total gas used selection sql: %v", err)
	}

	result := view.rdbHandle.QueryRow(sql)
	var total *int64
	if err := result.Scan(&total); err != nil {
		return 0, fmt.Errorf("error scanning total gas used selection query: %v", err)
	}

	if total == nil {
		return 0, nil
	}

	prometheus.RecordApiExecTime(recordMethod, "chainstats", "query", time.Since(startTime).Milliseconds())

	view.astraCache.Set(cacheKey, *total, 10*60*1000*time.Millisecond)

	return *total, nil
}

func (view *ChainStats) GetTotalTransactionFees() (*big.Int, error) {
	cacheKey := "GetTotalTransactionFees"
	var tmpTotalTransactionFees *big.Int

	err := view.astraCache.Get(cacheKey, &tmpTotalTransactionFees)
	if err == nil {
		return tmpTotalTransactionFees, nil
	}

	startTime := time.Now()
	recordMethod := "GetTotalTransactionFees"

	sql, _, err := view.rdbHandle.StmtBuilder.Select("CAST(SUM(total_fee) AS VARCHAR)").From(
		"chain_stats",
	).ToSql()
	if err != nil {
		return big.NewInt(0), fmt.Errorf("error building total transactions fee selection sql: %v", err)
	}

	result := view.rdbHandle.QueryRow(sql)
	var total string
	if err := result.Scan(&total); err != nil {
		return big.NewInt(0), fmt.Errorf("error scanning total transactions fee selection query: %v", err)
	}

	totalBigInt, isValid := big.NewInt(0).SetString(total, 10)
	if !isValid {
		totalBigInt = big.NewInt(0)
	}

	prometheus.RecordApiExecTime(recordMethod, "chainstats", "query", time.Since(startTime).Milliseconds())

	view.astraCache.Set(cacheKey, totalBigInt, 10*60*1000*time.Millisecond)

	return totalBigInt, nil
}

type ValidatorStatsRow struct {
	Metrics string
	Value   string
}

type TransactionHistory struct {
	Date                 string `json:"date"`
	Month                string `json:"month"`
	Year                 string `json:"year"`
	NumberOfTransactions int64  `json:"numberOfTransactions"`
}

type ActiveAddressHistory struct {
	Date                    string `json:"date"`
	Month                   string `json:"month"`
	Year                    string `json:"year"`
	NumberOfActiveAddresses int64  `json:"numberOfActiveAddresses"`
}

type TotalAddressGrowth struct {
	Date      string `json:"date"`
	Month     string `json:"month"`
	Year      string `json:"year"`
	Total     int64  `json:"total"`
	Active    int64  `json:"active"`
	NotActive int64  `json:"notActive"`
}

type TotalGasUsedHistory struct {
	Date         string `json:"date"`
	Month        string `json:"month"`
	Year         string `json:"year"`
	TotalGasUsed int64  `json:"totalGasUsed"`
}

type TotalFeeHistory struct {
	Date                 string   `json:"date"`
	Month                string   `json:"month"`
	Year                 string   `json:"year"`
	TotalTransactionFees *big.Int `json:"totalTransactionFees"`
}
