package rdbreportdashboard

import (
	"errors"
	"fmt"
	"time"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/infrastructure/metric/prometheus"
)

const DEFAULT_TABLE = "report_dashboard"
const SUCCESS = "success"
const FAIL = "fail"

type RDbReportDashboard struct {
	selectRDbHandle *rdb.Handle

	table string
}

func NewRDbReportDashboard(rdbHandle *rdb.Handle) *RDbReportDashboard {
	return &RDbReportDashboard{
		selectRDbHandle: rdbHandle,

		table: DEFAULT_TABLE,
	}
}

// Init initializes report dashboard DB when it is first time running
func (impl *RDbReportDashboard) init() error {
	var err error

	var exist bool
	if exist, err = impl.isRowExist(); err != nil {
		return fmt.Errorf("error checking report dashboard row existence: %v", err)
	}

	if !exist {
		if err = impl.initRow(); err != nil {
			return fmt.Errorf("error initializing chain stats row: %v", err)
		}
	}

	return nil
}

// isRowExist returns true if the row exists
func (impl *RDbReportDashboard) isRowExist() (bool, error) {
	currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()

	sql, args, err := impl.selectRDbHandle.StmtBuilder.Select(
		"COUNT(*)",
	).From(
		impl.table,
	).Where(
		"date_time = ?", currentDate,
	).ToSql()
	if err != nil {
		return false, fmt.Errorf("error building report dashboard row count selection SQL: %v", err)
	}

	var count int64
	if err := impl.selectRDbHandle.QueryRow(sql, args...).Scan(&count); err != nil {
		return false, fmt.Errorf("error querying report dashboard row count: %v", err)
	}

	return count > 0, nil
}

// initRow creates one row for current day report dashboard
func (impl *RDbReportDashboard) initRow() error {
	currentDate := time.Now().Truncate(24 * time.Hour).UnixNano()
	// Insert initial current day to the row
	sql, args, err := impl.selectRDbHandle.StmtBuilder.Insert(
		impl.table,
	).Columns(
		"date_time",
	).Values(currentDate).ToSql()
	if err != nil {
		return fmt.Errorf("error building getting report dashboard insertion SQL: %v", err)
	}

	execResult, err := impl.selectRDbHandle.Exec(sql, args...)
	if err != nil {
		return fmt.Errorf("error inserting latest report dashboard SQL: %v", err)
	}
	if execResult.RowsAffected() == 0 {
		return errors.New("error executing initial latest report dashboard insertion SQL: no rows affected")
	}

	return nil
}

func (impl *RDbReportDashboard) UpdateTotalAstraOnchainRewardsWithRDbHandle(currentDate int64) error {
	startTime := time.Now()
	recordMethod := "UpdateTotalAstraOnchainRewardsWithRDbHandle"

	if err := impl.init(); err != nil {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return fmt.Errorf("error initializing report dashboard %v", err)
	}

	rawQuery := fmt.Sprintf("CAST(SUM(amount)/pow(10,18) AS VARCHAR) FROM( "+
		"SELECT "+
		"DISTINCT ON (hash) hash, "+
		"CAST(CAST(CAST(CAST(value ->> 'content' AS jsonb) ->> 'params' AS jsonb) ->> 'data' AS jsonb) ->> 'value' AS numeric) AS amount "+
		"FROM "+
		"view_account_transaction_data, "+
		"jsonb_array_elements(view_account_transaction_data.messages) elems "+
		"WHERE "+
		"block_time >= %d AND "+
		"reward_tx_type = '%s' AND block_hash = '') AS tmp", currentDate, "sendReward")

	astraOnchainRewardsCountSubQuery := impl.selectRDbHandle.StmtBuilder.Select(rawQuery)
	sql, args, err := impl.selectRDbHandle.StmtBuilder.Update(
		impl.table,
	).Set(
		"total_asa_on_chain_rewards", impl.selectRDbHandle.StmtBuilder.SubQuery(astraOnchainRewardsCountSubQuery),
	).Where(
		"date_time = ?", currentDate,
	).ToSql()
	if err != nil {
		return fmt.Errorf("error building total astra onchain rewards update SQL: %v", err)
	}

	execResult, err := impl.selectRDbHandle.Exec(sql, args...)
	if err != nil {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return fmt.Errorf("error executing astra onchain rewards update SQL: %v", err)
	}
	if execResult.RowsAffected() == 0 {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return errors.New("error executing astra onchain update SQL: no rows affected")
	}

	prometheus.RecordApiExecTime(recordMethod, SUCCESS, "cronjob", time.Since(startTime).Milliseconds())
	return nil
}
