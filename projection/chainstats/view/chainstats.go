package view

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
)

type ChainStats struct {
	rdbHandle *rdb.Handle
}

func NewChainStats(rdbHandle *rdb.Handle) *ChainStats {
	return &ChainStats{
		rdbHandle,
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

	return transactionHistoryList, nil
}

func (view *ChainStats) GetTransactionsHistory(from_date time.Time, end_date time.Time) ([]TransactionHistory, error) {
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

	return transactionHistoryList, nil
}

func (view *ChainStats) GetActiveAddressesHistory(from_date time.Time, end_date time.Time) ([]ActiveAddressHistory, error) {
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

	return activeAddressHistoryList, nil
}

func (view *ChainStats) GetTotalAddressesGrowth(from_date time.Time, end_date time.Time) ([]TotalAddressGrowth, error) {
	sql, sqlArgs, err := view.rdbHandle.StmtBuilder.Select(
		"date_time",
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
			&totalAddressGrowth.NumberOfAddresses,
		); err != nil {
			if errors.Is(err, rdb.ErrNoRows) {
				return nil, rdb.ErrNoRows
			}
			return nil, fmt.Errorf("error scanning total addresses growth by date range row: %v: %w", err, rdb.ErrQuery)
		}
		totalAddressGrowth.Date = strings.Split(time.Unix(0, unixTime).UTC().String(), " ")[0]
		totalAddressGrowth.Month = strings.Split(totalAddressGrowth.Date, "-")[1]
		totalAddressGrowth.Year = strings.Split(totalAddressGrowth.Date, "-")[0]
		totalAddressGrowthList = append(totalAddressGrowthList, totalAddressGrowth)
	}

	length := len(totalAddressGrowthList)
	for index := range totalAddressGrowthList {
		if index < length-1 {
			totalAddressGrowthList[index].Growth = totalAddressGrowthList[index].NumberOfAddresses - totalAddressGrowthList[index+1].NumberOfAddresses
		} else {
			totalAddressGrowthList[index].Growth = 0
		}
	}

	return totalAddressGrowthList, nil
}

func (view *ChainStats) GetGasUsedHistory(from_date time.Time, end_date time.Time) ([]TotalGasUsedHistory, error) {
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

	return totalGasUsedHistoryList, nil
}

func (view *ChainStats) GetTotalFeeHistory(from_date time.Time, end_date time.Time) ([]TotalFeeHistory, error) {
	sql, sqlArgs, err := view.rdbHandle.StmtBuilder.Select(
		"date_time",
		"total_fee",
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
		var unixTime int64

		if err = rowsResult.Scan(
			&unixTime,
			&totalFeeHistory.TotalTransactionFees,
		); err != nil {
			if errors.Is(err, rdb.ErrNoRows) {
				return nil, rdb.ErrNoRows
			}
			return nil, fmt.Errorf("error scanning total transactions fee history by date range row: %v: %w", err, rdb.ErrQuery)
		}

		totalFeeHistory.Date = strings.Split(time.Unix(0, unixTime).UTC().String(), " ")[0]
		totalFeeHistory.Month = strings.Split(totalFeeHistory.Date, "-")[1]
		totalFeeHistory.Year = strings.Split(totalFeeHistory.Date, "-")[0]
		totalFeeHistoryList = append(totalFeeHistoryList, totalFeeHistory)
	}

	return totalFeeHistoryList, nil
}

func (view *ChainStats) GetMinDate() (int64, error) {
	sql, _, err := view.rdbHandle.StmtBuilder.Select("MIN(date_time)").From(
		"chain_stats",
	).ToSql()
	if err != nil {
		return 0, fmt.Errorf("error building min date selection sql: %v", err)
	}

	result := view.rdbHandle.QueryRow(sql)
	var min_date *int64
	if err := result.Scan(&min_date); err != nil {
		return 0, fmt.Errorf("error scanning min date selection query: %v", err)
	}

	if min_date == nil {
		return 0, nil
	}
	return *min_date, nil
}

func (view *ChainStats) GetTotalActiveAddresses() (int64, error) {
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
	return *total, nil
}

func (view *ChainStats) GetTotalGasUsed() (int64, error) {
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
	return *total, nil
}

func (view *ChainStats) GetTotalTransactionFees() (int64, error) {
	sql, _, err := view.rdbHandle.StmtBuilder.Select("SUM(total_fee)").From(
		"chain_stats",
	).ToSql()
	if err != nil {
		return 0, fmt.Errorf("error building total transactions fee selection sql: %v", err)
	}

	result := view.rdbHandle.QueryRow(sql)
	var total *int64
	if err := result.Scan(&total); err != nil {
		return 0, fmt.Errorf("error scanning total transactions fee selection query: %v", err)
	}

	if total == nil {
		return 0, nil
	}
	return *total, nil
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
	Date              string `json:"date"`
	Month             string `json:"month"`
	Year              string `json:"year"`
	NumberOfAddresses int64  `json:"numberOfAddresses"`
	Growth            int64  `json:"growth"`
}

type TotalGasUsedHistory struct {
	Date         string `json:"date"`
	Month        string `json:"month"`
	Year         string `json:"year"`
	TotalGasUsed int64  `json:"totalGasUsed"`
}

type TotalFeeHistory struct {
	Date                 string `json:"date"`
	Month                string `json:"month"`
	Year                 string `json:"year"`
	TotalTransactionFees int64  `json:"totalTransactionFees"`
}
