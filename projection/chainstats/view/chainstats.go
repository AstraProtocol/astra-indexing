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

func (view *ChainStats) GetTransactionsHistoryByDateRange(date_range int) ([]TransactionHistory, error) {
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

func (view *ChainStats) GetGasUsedHistoryByDateRange(date_range int) ([]TotalGasUsedHistory, error) {
	latest := time.Now().Truncate(24 * time.Hour)
	earliest := latest.Add(-time.Duration(date_range) * 24 * time.Hour)

	sql, sqlArgs, err := view.rdbHandle.StmtBuilder.Select(
		"date_time",
		"total_gas_used",
	).From(
		"chain_stats",
	).Where(
		"date_time >= ? AND date_time <= ?", earliest.UnixNano(), latest.UnixNano(),
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
		totalGasUsedHistoryList = append(totalGasUsedHistoryList, totalGasUsedHistory)
	}

	return totalGasUsedHistoryList, nil
}

func (view *ChainStats) GetTotalFeeHistoryByDateRange(date_range int) ([]TotalFeeHistory, error) {
	latest := time.Now().Truncate(24 * time.Hour)
	earliest := latest.Add(-time.Duration(date_range) * 24 * time.Hour)

	sql, sqlArgs, err := view.rdbHandle.StmtBuilder.Select(
		"date_time",
		"total_fee",
	).From(
		"chain_stats",
	).Where(
		"date_time >= ? AND date_time <= ?", earliest.UnixNano(), latest.UnixNano(),
	).OrderBy(
		"date_time DESC",
	).ToSql()
	if err != nil {
		return nil, fmt.Errorf("error building total fee history by date range select SQL: %v, %w", err, rdb.ErrBuildSQLStmt)
	}

	rowsResult, err := view.rdbHandle.Query(sql, sqlArgs...)
	if err != nil {
		return nil, fmt.Errorf("error executing total fee history by date range select SQL: %v: %w", err, rdb.ErrQuery)
	}
	defer rowsResult.Close()

	totalFeeHistoryList := make([]TotalFeeHistory, 0)
	for rowsResult.Next() {
		var totalFeeHistory TotalFeeHistory
		var unixTime int64

		if err = rowsResult.Scan(
			&unixTime,
			&totalFeeHistory.TotalFee,
		); err != nil {
			if errors.Is(err, rdb.ErrNoRows) {
				return nil, rdb.ErrNoRows
			}
			return nil, fmt.Errorf("error scanning total fee history by date range row: %v: %w", err, rdb.ErrQuery)
		}

		totalFeeHistory.Date = strings.Split(time.Unix(0, unixTime).UTC().String(), " ")[0]
		totalFeeHistoryList = append(totalFeeHistoryList, totalFeeHistory)
	}

	return totalFeeHistoryList, nil
}

type ValidatorStatsRow struct {
	Metrics string
	Value   string
}

type TransactionHistory struct {
	Date                 string `json:"date"`
	NumberOfTransactions int64  `json:"numberOfTransactions"`
}

type TotalGasUsedHistory struct {
	Date         string `json:"date"`
	TotalGasUsed int64  `json:"totalGasUsed"`
}

type TotalFeeHistory struct {
	Date     string `json:"date"`
	TotalFee int64  `json:"totalFee"`
}
