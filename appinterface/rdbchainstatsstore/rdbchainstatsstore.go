package rdbchainstatsstore

import (
	"errors"
	"fmt"
	"time"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"gopkg.in/robfig/cron.v2"
)

const DEFAULT_TABLE = "chain_stats"

type RDbChainStatsStore struct {
	selectRDbHandle *rdb.Handle

	table string
}

func NewRDbChainStatsStore(rdbHandle *rdb.Handle) *RDbChainStatsStore {
	return &RDbChainStatsStore{
		selectRDbHandle: rdbHandle,

		table: DEFAULT_TABLE,
	}
}

// Init initializes chain stats store DB when it is first time running
func (impl *RDbChainStatsStore) init() error {
	var err error

	var exist bool
	if exist, err = impl.isRowExist(); err != nil {
		return fmt.Errorf("error checking transaction stats row existence: %v", err)
	}

	if !exist {
		if err = impl.initRow(); err != nil {
			return fmt.Errorf("error initializing transaction stats row: %v", err)
		}
	}

	return nil
}

// isRowExist returns true if the row exists
func (impl *RDbChainStatsStore) isRowExist() (bool, error) {
	currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()

	sql, args, err := impl.selectRDbHandle.StmtBuilder.Select(
		"COUNT(*)",
	).From(
		impl.table,
	).Where(
		"date_time = ?", currentDate,
	).ToSql()
	if err != nil {
		return false, fmt.Errorf("error building transaction stats row count selection SQL: %v", err)
	}

	var count int64
	if err := impl.selectRDbHandle.QueryRow(sql, args...).Scan(&count); err != nil {
		return false, fmt.Errorf("error querying transaction stats row count: %v", err)
	}

	return count > 0, nil
}

// initRow creates one row for current day chain stats
func (impl *RDbChainStatsStore) initRow() error {
	currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()
	// Insert initial current day to the row
	sql, args, err := impl.selectRDbHandle.StmtBuilder.Insert(
		impl.table,
	).Columns(
		"date_time",
		"number_of_transactions",
	).Values(currentDate, 0).ToSql()
	if err != nil {
		return fmt.Errorf("error building getting row count insertion SQL: %v", err)
	}

	execResult, err := impl.selectRDbHandle.Exec(sql, args...)
	if err != nil {
		return fmt.Errorf("error inserting latest transaction stats SQL: %v", err)
	}
	if execResult.RowsAffected() == 0 {
		return errors.New("error executing initial latest transaction stats insertion SQL: no rows inserted")
	}

	return nil
}

func (impl *RDbChainStatsStore) UpdateCountedTransactionsWithRDbHandle() error {
	currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()

	if err := impl.init(); err != nil {
		return fmt.Errorf("error initializing transaction stats store: %v", err)
	}

	transactionsCountSubQuery := impl.selectRDbHandle.StmtBuilder.Select(
		"SUM(transaction_count)",
	).From(
		"view_blocks",
	).Where("time >= ?", currentDate)

	sql, args, err := impl.selectRDbHandle.StmtBuilder.Update(
		impl.table,
	).Set(
		"number_of_transactions", impl.selectRDbHandle.StmtBuilder.SubQuery(transactionsCountSubQuery),
	).Where(
		"date_time = ?", currentDate,
	).ToSql()
	if err != nil {
		return fmt.Errorf("error building last transaction stats update SQL: %v", err)
	}

	execResult, err := impl.selectRDbHandle.Exec(sql, args...)
	if err != nil {
		return fmt.Errorf("error executing last transaction stats update SQL: %v", err)
	}
	if execResult.RowsAffected() == 0 {
		return errors.New("error executing last transaction stats update SQL: no rows updated")
	}

	return nil
}

func RunCronJobs(rdbHandle *rdb.Handle) {
	rdbTransactionStatsStore := NewRDbChainStatsStore(rdbHandle)
	s := cron.New()

	// At minute 59 past every hour from 0 through 23
	// @every 0h0m5s
	s.AddFunc("59 0-23 * * *", func() {
		go rdbTransactionStatsStore.UpdateCountedTransactionsWithRDbHandle()
	})

	s.Start()
}
