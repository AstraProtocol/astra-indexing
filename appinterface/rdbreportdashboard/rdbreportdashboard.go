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
	table           string
}

func NewRDbReportDashboard(rdbHandle *rdb.Handle) *RDbReportDashboard {
	return &RDbReportDashboard{
		selectRDbHandle: rdbHandle,
		table:           DEFAULT_TABLE,
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
		return fmt.Errorf("error executing total astra onchain rewards update SQL: %v", err)
	}
	if execResult.RowsAffected() == 0 {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return errors.New("error executing total astra onchain update SQL: no rows affected")
	}

	prometheus.RecordApiExecTime(recordMethod, SUCCESS, "cronjob", time.Since(startTime).Milliseconds())
	return nil
}

func (impl *RDbReportDashboard) UpdateTotalAstraWithdrawnFromTikiWithRDbHandle(currentDate int64, tikiAddress string) error {
	startTime := time.Now()
	recordMethod := "UpdateTotalAstraWithdrawnFromTikiWithRDbHandle"

	if err := impl.init(); err != nil {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return fmt.Errorf("error initializing report dashboard %v", err)
	}

	rawQuery := fmt.Sprintf(
		"CAST(SUM(CAST(CAST(CAST(CAST(value ->> 'content' AS jsonb) ->> 'amount' AS json) ->> 0 AS jsonb) ->> 'amount' AS numeric))/pow(10,18) AS VARCHAR) "+
			"FROM "+
			"view_transactions, "+
			"jsonb_array_elements(view_transactions.messages) elems "+
			"WHERE "+
			"block_time >= %d AND "+
			"value->>'type'='/cosmos.bank.v1beta1.MsgSend' AND "+
			"from_address='%s'", currentDate, tikiAddress)

	astraWithdrawnFromTikiCountSubQuery := impl.selectRDbHandle.StmtBuilder.Select(rawQuery)
	sql, args, err := impl.selectRDbHandle.StmtBuilder.Update(
		impl.table,
	).Set(
		"total_asa_withdrawn_from_tiki", impl.selectRDbHandle.StmtBuilder.SubQuery(astraWithdrawnFromTikiCountSubQuery),
	).Where(
		"date_time = ?", currentDate,
	).ToSql()
	if err != nil {
		return fmt.Errorf("error building total astra withdrawn from Tiki update SQL: %v", err)
	}

	execResult, err := impl.selectRDbHandle.Exec(sql, args...)
	if err != nil {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return fmt.Errorf("error executing total astra withdrawn from Tiki update SQL: %v", err)
	}
	if execResult.RowsAffected() == 0 {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return errors.New("error executing total astra withdrawn from Tiki update SQL: no rows affected")
	}

	prometheus.RecordApiExecTime(recordMethod, SUCCESS, "cronjob", time.Since(startTime).Milliseconds())
	return nil
}

func (impl *RDbReportDashboard) UpdateTotalAstraOfRedeemedCouponsWithRDbHandle(currentDate int64) error {
	startTime := time.Now()
	recordMethod := "UpdateTotalAstraOfRedeemedCouponsWithRDbHandle"

	if err := impl.init(); err != nil {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return fmt.Errorf("error initializing report dashboard %v", err)
	}

	rawQuery := fmt.Sprintf(
		"CAST(SUM(CAST(CAST(CAST(CAST(value ->> 'content' AS jsonb) ->> 'params' AS jsonb) ->> 'data' AS jsonb) ->> 'value' AS numeric))/pow(10,18) AS VARCHAR) "+
			"FROM "+
			"view_transactions, "+
			"jsonb_array_elements(view_transactions.messages) elems "+
			"WHERE "+
			"block_time >= %d AND "+
			"tx_type = '%s'", currentDate, "exchangeWithValue")

	astraOfRedeemedCouponsCountSubQuery := impl.selectRDbHandle.StmtBuilder.Select(rawQuery)
	sql, args, err := impl.selectRDbHandle.StmtBuilder.Update(
		impl.table,
	).Set(
		"total_asa_of_redeemed_coupons", impl.selectRDbHandle.StmtBuilder.SubQuery(astraOfRedeemedCouponsCountSubQuery),
	).Where(
		"date_time = ?", currentDate,
	).ToSql()
	if err != nil {
		return fmt.Errorf("error building total astra of redeemed coupons update SQL: %v", err)
	}

	execResult, err := impl.selectRDbHandle.Exec(sql, args...)
	if err != nil {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return fmt.Errorf("error executing total astra of redeemed coupons update SQL: %v", err)
	}
	if execResult.RowsAffected() == 0 {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return errors.New("error executing total astra of redeemed coupons update SQL: no rows affected")
	}

	prometheus.RecordApiExecTime(recordMethod, SUCCESS, "cronjob", time.Since(startTime).Milliseconds())
	return nil
}

func (impl *RDbReportDashboard) UpdateTotalTxsOfRedeemedCouponsWithRDbHandle(currentDate int64) error {
	startTime := time.Now()
	recordMethod := "UpdateTotalTxsOfRedeemedCouponsWithRDbHandle"

	if err := impl.init(); err != nil {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return fmt.Errorf("error initializing report dashboard %v", err)
	}

	rawQuery := fmt.Sprintf(
		"COUNT(*) "+
			"FROM (SELECT DISTINCT evm_hash "+
			"FROM view_transactions "+
			"WHERE "+
			"block_time >= %d AND "+
			"tx_type = '%s') AS dt", currentDate, "exchangeWithValue")

	txsOfRedeemedCouponsCountSubQuery := impl.selectRDbHandle.StmtBuilder.Select(rawQuery)
	sql, args, err := impl.selectRDbHandle.StmtBuilder.Update(
		impl.table,
	).Set(
		"total_transaction_of_redeemed_coupons", impl.selectRDbHandle.StmtBuilder.SubQuery(txsOfRedeemedCouponsCountSubQuery),
	).Where(
		"date_time = ?", currentDate,
	).ToSql()
	if err != nil {
		return fmt.Errorf("error building total txs of redeemed coupons update SQL: %v", err)
	}

	execResult, err := impl.selectRDbHandle.Exec(sql, args...)
	if err != nil {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return fmt.Errorf("error executing total txs of redeemed coupons update SQL: %v", err)
	}
	if execResult.RowsAffected() == 0 {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return errors.New("error executing total txs of redeemed coupons update SQL: no rows affected")
	}

	prometheus.RecordApiExecTime(recordMethod, SUCCESS, "cronjob", time.Since(startTime).Milliseconds())
	return nil
}

func (impl *RDbReportDashboard) UpdateTotalAddressesOfRedeemedCouponsWithRDbHandle(currentDate int64) error {
	startTime := time.Now()
	recordMethod := "UpdateTotalAddressesOfRedeemedCouponsWithRDbHandle"

	if err := impl.init(); err != nil {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return fmt.Errorf("error initializing report dashboard %v", err)
	}

	rawQuery := fmt.Sprintf(
		"COUNT(*) "+
			"FROM (SELECT DISTINCT from_address "+
			"FROM view_transactions "+
			"WHERE "+
			"block_time >= %d AND "+
			"tx_type = '%s') AS dt", currentDate, "exchangeWithValue")

	addressesOfRedeemedCouponsCountSubQuery := impl.selectRDbHandle.StmtBuilder.Select(rawQuery)
	sql, args, err := impl.selectRDbHandle.StmtBuilder.Update(
		impl.table,
	).Set(
		"total_redeemed_coupon_addresses", impl.selectRDbHandle.StmtBuilder.SubQuery(addressesOfRedeemedCouponsCountSubQuery),
	).Where(
		"date_time = ?", currentDate,
	).ToSql()
	if err != nil {
		return fmt.Errorf("error building total addresses of redeemed coupons update SQL: %v", err)
	}

	execResult, err := impl.selectRDbHandle.Exec(sql, args...)
	if err != nil {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return fmt.Errorf("error executing total addresses of redeemed coupons update SQL: %v", err)
	}
	if execResult.RowsAffected() == 0 {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return errors.New("error executing total addresses of redeemed coupons update SQL: no rows affected")
	}

	prometheus.RecordApiExecTime(recordMethod, SUCCESS, "cronjob", time.Since(startTime).Milliseconds())
	return nil
}

func (impl *RDbReportDashboard) UpdateTotalAstraStakedWithRDbHandle(currentDate int64) error {
	startTime := time.Now()
	recordMethod := "UpdateTotalAstraStakedWithRDbHandle"

	if err := impl.init(); err != nil {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return fmt.Errorf("error initializing report dashboard %v", err)
	}

	rawQuery := fmt.Sprintf(
		"CAST(SUM(CAST(CAST(CAST(value ->> 'content' AS jsonb) ->> 'amount' AS jsonb) ->>'amount' AS numeric))/pow(10,18) AS VARCHAR) "+
			"FROM "+
			"view_transactions, "+
			"jsonb_array_elements(view_transactions.messages) elems "+
			"WHERE "+
			"block_time >= %d AND "+
			"value->>'type'='%s'", currentDate, "/cosmos.staking.v1beta1.MsgDelegate")

	astraStakedCountSubQuery := impl.selectRDbHandle.StmtBuilder.Select(rawQuery)
	sql, args, err := impl.selectRDbHandle.StmtBuilder.Update(
		impl.table,
	).Set(
		"total_asa_staked", impl.selectRDbHandle.StmtBuilder.SubQuery(astraStakedCountSubQuery),
	).Where(
		"date_time = ?", currentDate,
	).ToSql()
	if err != nil {
		return fmt.Errorf("error building total astra staked update SQL: %v", err)
	}

	execResult, err := impl.selectRDbHandle.Exec(sql, args...)
	if err != nil {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return fmt.Errorf("error executing total astra staked update SQL: %v", err)
	}
	if execResult.RowsAffected() == 0 {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return errors.New("error executing total astra staked update SQL: no rows affected")
	}

	prometheus.RecordApiExecTime(recordMethod, SUCCESS, "cronjob", time.Since(startTime).Milliseconds())
	return nil
}

func (impl *RDbReportDashboard) UpdateTotalStakingTxsWithRDbHandle(currentDate int64) error {
	startTime := time.Now()
	recordMethod := "UpdateTotalStakingTxsWithRDbHandle"

	if err := impl.init(); err != nil {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return fmt.Errorf("error initializing report dashboard %v", err)
	}

	rawQuery := fmt.Sprintf(
		"COUNT ( * ) FROM "+
			"view_transactions, "+
			"jsonb_array_elements(view_transactions.messages) elems "+
			"WHERE "+
			"block_time >= %d AND "+
			"value->>'type'='%s'", currentDate, "/cosmos.staking.v1beta1.MsgDelegate")

	txsStakedCountSubQuery := impl.selectRDbHandle.StmtBuilder.Select(rawQuery)
	sql, args, err := impl.selectRDbHandle.StmtBuilder.Update(
		impl.table,
	).Set(
		"total_staking_transactions", impl.selectRDbHandle.StmtBuilder.SubQuery(txsStakedCountSubQuery),
	).Where(
		"date_time = ?", currentDate,
	).ToSql()
	if err != nil {
		return fmt.Errorf("error building total staking txs update SQL: %v", err)
	}

	execResult, err := impl.selectRDbHandle.Exec(sql, args...)
	if err != nil {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return fmt.Errorf("error executing total staking txs update SQL: %v", err)
	}
	if execResult.RowsAffected() == 0 {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return errors.New("error executing total staking txs update SQL: no rows affected")
	}

	prometheus.RecordApiExecTime(recordMethod, SUCCESS, "cronjob", time.Since(startTime).Milliseconds())
	return nil
}

func (impl *RDbReportDashboard) UpdateTotalStakingAddressesWithRDbHandle(currentDate int64) error {
	startTime := time.Now()
	recordMethod := "UpdateTotalStakingAddressesWithRDbHandle"

	if err := impl.init(); err != nil {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return fmt.Errorf("error initializing report dashboard %v", err)
	}

	rawQuery := fmt.Sprintf(
		"COUNT (*) FROM(SELECT DISTINCT CAST(value ->> 'content' AS jsonb) ->> 'delegatorAddress' "+
			"FROM "+
			"view_transactions, "+
			"jsonb_array_elements(view_transactions.messages) elems "+
			"WHERE "+
			"block_time >= %d AND "+
			"value->>'type'='%s') AS tmp", currentDate, "/cosmos.staking.v1beta1.MsgDelegate")

	addressesStakedCountSubQuery := impl.selectRDbHandle.StmtBuilder.Select(rawQuery)
	sql, args, err := impl.selectRDbHandle.StmtBuilder.Update(
		impl.table,
	).Set(
		"total_staking_addresses", impl.selectRDbHandle.StmtBuilder.SubQuery(addressesStakedCountSubQuery),
	).Where(
		"date_time = ?", currentDate,
	).ToSql()
	if err != nil {
		return fmt.Errorf("error building total staking addresses update SQL: %v", err)
	}

	execResult, err := impl.selectRDbHandle.Exec(sql, args...)
	if err != nil {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return fmt.Errorf("error executing total staking addresses update SQL: %v", err)
	}
	if execResult.RowsAffected() == 0 {
		prometheus.RecordApiExecTime(recordMethod, FAIL, "cronjob", time.Since(startTime).Milliseconds())
		return errors.New("error executing total staking addresses update SQL: no rows affected")
	}

	prometheus.RecordApiExecTime(recordMethod, SUCCESS, "cronjob", time.Since(startTime).Milliseconds())
	return nil
}
