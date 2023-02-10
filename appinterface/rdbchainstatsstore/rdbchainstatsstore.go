package rdbchainstatsstore

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/bootstrap/config"
	cosmosapp_infrastructure "github.com/AstraProtocol/astra-indexing/infrastructure/cosmosapp"
	"github.com/AstraProtocol/astra-indexing/infrastructure/metric/prometheus"
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
		return fmt.Errorf("error checking chain stats row existence: %v", err)
	}

	if !exist {
		if err = impl.initRow(); err != nil {
			return fmt.Errorf("error initializing chain stats row: %v", err)
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
		return false, fmt.Errorf("error building chain stats row count selection SQL: %v", err)
	}

	var count int64
	if err := impl.selectRDbHandle.QueryRow(sql, args...).Scan(&count); err != nil {
		return false, fmt.Errorf("error querying chain stats row count: %v", err)
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
		"total_gas_used",
		"total_fee",
		"total_addresses",
		"active_addresses",
	).Values(currentDate, 0, 0, 0, 0, 0).ToSql()
	if err != nil {
		return fmt.Errorf("error building getting chain stats insertion SQL: %v", err)
	}

	execResult, err := impl.selectRDbHandle.Exec(sql, args...)
	if err != nil {
		return fmt.Errorf("error inserting latest chain stats SQL: %v", err)
	}
	if execResult.RowsAffected() == 0 {
		return errors.New("error executing initial latest chain stats insertion SQL: no rows affected")
	}

	return nil
}

func (impl *RDbChainStatsStore) UpdateCountedTransactionsWithRDbHandle(currentDate int64) error {
	startTime := time.Now()
	recordMethod := "UpdateCountedTransactionsWithRDbHandle"

	if err := impl.init(); err != nil {
		return fmt.Errorf("error initializing chain stats store: %v", err)
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
		return fmt.Errorf("error building transaction stats update SQL: %v", err)
	}

	execResult, err := impl.selectRDbHandle.Exec(sql, args...)
	if err != nil {
		return fmt.Errorf("error executing transaction stats update SQL: %v", err)
	}
	if execResult.RowsAffected() == 0 {
		return errors.New("error executing transaction stats update SQL: no rows affected")
	}

	prometheus.RecordApiExecTime(recordMethod, "cronjob", "update", time.Since(startTime).Milliseconds())

	return nil
}

func (impl *RDbChainStatsStore) UpdateTotalGasUsedWithRDbHandle(currentDate int64) error {
	startTime := time.Now()
	recordMethod := "UpdateTotalGasUsedWithRDbHandle"

	if err := impl.init(); err != nil {
		return fmt.Errorf("error initializing chain stats store: %v", err)
	}

	gasUsedCountSubQuery := impl.selectRDbHandle.StmtBuilder.Select(
		"SUM(gas_used)",
	).From(
		"view_transactions",
	).Where("block_time >= ?", currentDate)

	sql, args, err := impl.selectRDbHandle.StmtBuilder.Update(
		impl.table,
	).Set(
		"total_gas_used", impl.selectRDbHandle.StmtBuilder.SubQuery(gasUsedCountSubQuery),
	).Where(
		"date_time = ?", currentDate,
	).ToSql()
	if err != nil {
		return fmt.Errorf("error building gas used stats update SQL: %v", err)
	}

	execResult, err := impl.selectRDbHandle.Exec(sql, args...)
	if err != nil {
		return fmt.Errorf("error executing gas used stats update SQL: %v", err)
	}
	if execResult.RowsAffected() == 0 {
		return errors.New("error executing gas used stats update SQL: no rows affected")
	}

	prometheus.RecordApiExecTime(recordMethod, "cronjob", "update", time.Since(startTime).Milliseconds())

	return nil
}

func (impl *RDbChainStatsStore) UpdateTotalFeeWithRDbHandle(currentDate int64, config *config.Config) error {
	startTime := time.Now()
	recordMethod := "UpdateTotalFeeWithRDbHandle"

	if err := impl.init(); err != nil {
		return fmt.Errorf("error initializing chain stats store: %v", err)
	}

	cosmosAppClient := cosmosapp_infrastructure.NewHTTPClient(
		config.CosmosApp.HTTPRPCUrl,
		config.Blockchain.BondingDenom,
	)

	totalFeeBurnResp, err := cosmosAppClient.TotalFeeBurn()
	if err != nil {
		return fmt.Errorf("error getting total fee burn from api: %v", err)
	}

	totalFeeBurn, _ := new(big.Float).SetString(totalFeeBurnResp.TotalFeeBurn.Amount)
	totalFeeUsed := new(big.Float).Mul(totalFeeBurn, new(big.Float).SetInt64(2))

	result := new(big.Int)
	totalFeeUsed.Int(result)

	sql, args, err := impl.selectRDbHandle.StmtBuilder.Update(
		impl.table,
	).Set(
		"total_fee", result.String(),
	).Where(
		"date_time = ?", currentDate,
	).ToSql()
	if err != nil {
		return fmt.Errorf("error building fee stats update SQL: %v", err)
	}

	execResult, err := impl.selectRDbHandle.Exec(sql, args...)
	if err != nil {
		return fmt.Errorf("error executing fee stats update SQL: %v", err)
	}
	if execResult.RowsAffected() == 0 {
		return errors.New("error executing fee stats update SQL: no rows affected")
	}

	prometheus.RecordApiExecTime(recordMethod, "cronjob", "update", time.Since(startTime).Milliseconds())

	return nil
}

func (impl *RDbChainStatsStore) UpdateTotalAddressesWithRDbHandle(currentDate int64) error {
	startTime := time.Now()
	recordMethod := "UpdateTotalAddressesWithRDbHandle"

	if err := impl.init(); err != nil {
		return fmt.Errorf("error initializing chain stats store: %v", err)
	}

	totalAddressesSubQuery := impl.selectRDbHandle.StmtBuilder.Select(
		"MAX(account_number)",
	).From("view_accounts")

	sql, args, err := impl.selectRDbHandle.StmtBuilder.Update(
		impl.table,
	).Set(
		"total_addresses", impl.selectRDbHandle.StmtBuilder.SubQuery(totalAddressesSubQuery),
	).Where(
		"date_time = ?", currentDate,
	).ToSql()
	if err != nil {
		return fmt.Errorf("error building total addresses stats update SQL: %v", err)
	}

	execResult, err := impl.selectRDbHandle.Exec(sql, args...)
	if err != nil {
		return fmt.Errorf("error executing total addresses stats update SQL: %v", err)
	}
	if execResult.RowsAffected() == 0 {
		return errors.New("error executing total addresses stats update SQL: no rows affected")
	}

	prometheus.RecordApiExecTime(recordMethod, "cronjob", "update", time.Since(startTime).Milliseconds())

	return nil
}

func (impl *RDbChainStatsStore) UpdateActiveAddressesWithRDbHandle(currentDate int64) error {
	startTime := time.Now()
	recordMethod := "UpdateActiveAddressesWithRDbHandle"

	if err := impl.init(); err != nil {
		return fmt.Errorf("error initializing chain stats store: %v", err)
	}

	sql := "UPDATE chain_stats SET active_addresses = (SELECT COUNT(*) FROM (SELECT DISTINCT (from_address) FROM view_transactions WHERE block_time >= $1) AS temp) WHERE date_time = $1"

	execResult, err := impl.selectRDbHandle.Exec(sql, currentDate)
	if err != nil {
		return fmt.Errorf("error executing active addresses stats update SQL: %v", err)
	}
	if execResult.RowsAffected() == 0 {
		return errors.New("error executing active addresses stats update SQL: no rows affected")
	}

	prometheus.RecordApiExecTime(recordMethod, "cronjob", "update", time.Since(startTime).Milliseconds())

	return nil
}
