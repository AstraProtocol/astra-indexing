package view

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/appinterface/rdbreportdashboard"
	"github.com/AstraProtocol/astra-indexing/bootstrap/config"
	"github.com/AstraProtocol/astra-indexing/external/cache"
)

type ReportDashboard struct {
	rdbHandle          *rdb.Handle
	astraCache         *cache.AstraCache
	rdbReportDashboard *rdbreportdashboard.RDbReportDashboard
	config             *config.Config
}

func NewReportDashboard(rdbHandle *rdb.Handle, config *config.Config) *ReportDashboard {
	return &ReportDashboard{
		rdbHandle:          rdbHandle,
		astraCache:         cache.NewCache(),
		rdbReportDashboard: rdbreportdashboard.NewRDbReportDashboard(rdbHandle),
		config:             config,
	}
}

func (view *ReportDashboard) UpdateReportDashboardByDate(date string) (string, error) {
	//example: currentDate = "2023-06-19"
	layout := "2006-01-02"
	tikiAddress := view.config.CronjobReportDashboard.TikiAddress
	dateTime, err := time.Parse(layout, date)
	if err != nil {
		return "NOK", err
	}
	currentDate := dateTime.Truncate(24 * time.Hour).UnixNano()
	nextDate := dateTime.Truncate(24 * time.Hour).Add(24 * time.Hour).UnixNano()
	//prevDate := dateTime.Truncate(24 * time.Hour).Add(-24 * time.Hour).UnixNano()

	err = view.rdbReportDashboard.UpdateTotalAstraWithdrawnFromTikiWithRDbHandle(currentDate, nextDate, tikiAddress)
	if err != nil {
		return "NOK", err
	}

	/*
		view.rdbReportDashboard.UpdateTotalNewAddressesWithRDbHandle(currentDate, prevDate)
		view.rdbReportDashboard.UpdateTotalTxsOfRedeemedCouponsWithRDbHandle(currentDate, nextDate)
		view.rdbReportDashboard.UpdateTotalAddressesOfRedeemedCouponsWithRDbHandle(currentDate, nextDate)
		view.rdbReportDashboard.UpdateTotalAstraOfRedeemedCouponsWithRDbHandle(currentDate, nextDate)
		view.rdbReportDashboard.UpdateTotalAstraOnchainRewardsWithRDbHandle(currentDate, nextDate)
		view.rdbReportDashboard.UpdateTotalAstraStakedWithRDbHandle(currentDate, nextDate)
		view.rdbReportDashboard.UpdateTotalStakingAddressesWithRDbHandle(currentDate, nextDate)
		view.rdbReportDashboard.UpdateTotalStakingTxsWithRDbHandle(currentDate, nextDate)
	*/

	return "OK", nil
}

func (impl *ReportDashboard) GetReportDashboardByTimeRange(from string, to string) (ReportDashboardOverall, error) {
	layout := "2006-01-02"
	fromDateTime, err := time.Parse(layout, from)
	if err != nil {
		return ReportDashboardOverall{}, err
	}
	fromDate := fromDateTime.Truncate(24 * time.Hour).UnixNano()

	toDateTime, err := time.Parse(layout, to)
	if err != nil {
		return ReportDashboardOverall{}, err
	}
	toDate := toDateTime.Truncate(24 * time.Hour).Add(24 * time.Hour).UnixNano()

	rawQuery := fmt.Sprintf("SELECT rd.date_time, rd.total_transaction_of_redeemed_coupons, rd.total_redeemed_coupon_addresses, "+
		"rd.total_asa_of_redeemed_coupons, rd.total_staking_transactions, rd.total_staking_addresses, "+
		"rd.total_asa_staked, rd.total_new_addresses, rd.total_asa_withdrawn_from_tiki, rd.total_asa_on_chain_rewards, "+
		"cs.number_of_transactions "+
		"FROM report_dashboard AS rd "+
		"INNER JOIN chain_stats AS cs ON rd.date_time = cs.date_time "+
		"WHERE rd.date_time >= %d AND rd.date_time < %d ORDER BY rd.date_time ASC", fromDate, toDate)

	reportDashboardDataList := make([]ReportDashboardData, 0)

	rowsResult, err := impl.rdbHandle.Query(rawQuery)
	if err != nil {
		return ReportDashboardOverall{}, fmt.Errorf("error executing get report dashboard by time range select SQL: %v: %w", err, rdb.ErrQuery)
	}
	defer rowsResult.Close()

	var totalAsaOfRedeemedCoupons string
	var totalAsaStaked string
	var totalAsaWithdrawnFromTiki string
	var totalAsaOnchainRewards string

	for rowsResult.Next() {
		var result ReportDashboardData
		var unixTime int64
		if err = rowsResult.Scan(
			&unixTime,
			&result.TotalTxOfRedeemedCoupons,
			&result.TotalRedeemedCouponAddresses,
			&totalAsaOfRedeemedCoupons,
			&result.TotalStakingTransactions,
			&result.TotalStakingAddresses,
			&totalAsaStaked,
			&result.TotalNewAddresses,
			&totalAsaWithdrawnFromTiki,
			&totalAsaOnchainRewards,
			&result.TotalTransactions,
		); err != nil {
			if errors.Is(err, rdb.ErrNoRows) {
				return ReportDashboardOverall{}, rdb.ErrNoRows
			}
			return ReportDashboardOverall{}, fmt.Errorf("error scanning get report dashboard by time range row: %v: %w", err, rdb.ErrQuery)
		}
		dateTime := time.Unix(0, unixTime).Format(layout)
		result.DateTime = dateTime

		totalAsaOfRedeemedCoupons, _ := strconv.ParseFloat(strings.TrimSpace(totalAsaOfRedeemedCoupons), 64)
		result.TotalAsaOfRedeemedCoupons = totalAsaOfRedeemedCoupons

		totalAsaStaked, _ := strconv.ParseFloat(strings.TrimSpace(totalAsaStaked), 64)
		result.TotalAsaStaked = totalAsaStaked

		totalAsaWithdrawnFromTiki, _ := strconv.ParseFloat(strings.TrimSpace(totalAsaWithdrawnFromTiki), 64)
		result.TotalAsaWithdrawnFromTiki = totalAsaWithdrawnFromTiki

		totalAsaOnchainRewards, _ := strconv.ParseFloat(strings.TrimSpace(totalAsaOnchainRewards), 64)
		result.TotalAsaOnchainRewards = totalAsaOnchainRewards

		reportDashboardDataList = append(reportDashboardDataList, result)
	}

	var reportDashboardOverall ReportDashboardOverall
	reportDashboardOverall.Data = reportDashboardDataList

	for _, reportDashboardData := range reportDashboardDataList {
		reportDashboardOverall.Overall.TotalAsaOfRedeemedCoupons += reportDashboardData.TotalAsaOfRedeemedCoupons
		reportDashboardOverall.Overall.TotalAsaStaked += reportDashboardData.TotalAsaStaked
		reportDashboardOverall.Overall.TotalAsaWithdrawnFromTiki += reportDashboardData.TotalAsaWithdrawnFromTiki
		reportDashboardOverall.Overall.TotalAsaOnchainRewards += reportDashboardData.TotalAsaOnchainRewards

		reportDashboardOverall.Overall.TotalNewAddresses += reportDashboardData.TotalNewAddresses
		reportDashboardOverall.Overall.TotalRedeemedCouponAddresses += reportDashboardData.TotalRedeemedCouponAddresses
		reportDashboardOverall.Overall.TotalStakingAddresses += reportDashboardData.TotalStakingAddresses
		reportDashboardOverall.Overall.TotalStakingTransactions += reportDashboardData.TotalStakingTransactions
		reportDashboardOverall.Overall.TotalTransactions += reportDashboardData.TotalTransactions
		reportDashboardOverall.Overall.TotalTxOfRedeemedCoupons += reportDashboardData.TotalTxOfRedeemedCoupons
	}

	return reportDashboardOverall, nil
}

func (impl *ReportDashboard) GetActiveAddressesByTimeRangeDirectly(from string, to string) (int64, error) {
	layout := "2006-01-02"
	fromDateTime, err := time.Parse(layout, from)
	if err != nil {
		return -1, err
	}
	fromDate := fromDateTime.Truncate(24 * time.Hour).UnixNano()

	toDateTime, err := time.Parse(layout, to)
	if err != nil {
		return -1, err
	}
	toDate := toDateTime.Truncate(24 * time.Hour).Add(24 * time.Hour).UnixNano()

	rawQuery := fmt.Sprintf("SELECT COUNT(from_address) "+
		"FROM "+
		"(SELECT DISTINCT (from_address) "+
		"FROM view_transactions "+
		"WHERE block_time >= %d "+
		"AND block_time < %d "+
		") AS tmp", fromDate, toDate)

	var totalActiveAddresses int64
	if err = impl.rdbHandle.QueryRow(rawQuery).Scan(
		&totalActiveAddresses,
	); err != nil {
		if errors.Is(err, rdb.ErrNoRows) {
			return -1, rdb.ErrNoRows
		}
		return -1, fmt.Errorf("error scanning active addresses by time range row: %v: %w", err, rdb.ErrQuery)
	}
	return totalActiveAddresses, nil
}

func (impl *ReportDashboard) GetStakingAddressesByTimeRangeDirectly(from string, to string) (int64, error) {
	layout := "2006-01-02"
	fromDateTime, err := time.Parse(layout, from)
	if err != nil {
		return -1, err
	}
	fromDate := fromDateTime.Truncate(24 * time.Hour).UnixNano()

	toDateTime, err := time.Parse(layout, to)
	if err != nil {
		return -1, err
	}
	toDate := toDateTime.Truncate(24 * time.Hour).Add(24 * time.Hour).UnixNano()

	rawQuery := fmt.Sprintf("SELECT COUNT (*) FROM(SELECT DISTINCT CAST(value ->> 'content' AS jsonb) ->> 'delegatorAddress' "+
		"FROM "+
		"view_transactions, "+
		"jsonb_array_elements(view_transactions.messages) elems "+
		"WHERE "+
		"block_time >= %d AND "+
		"block_time < %d AND "+
		"value->>'type'='%s') AS tmp", fromDate, toDate, "/cosmos.staking.v1beta1.MsgDelegate")

	var totalStakingAddresses int64
	if err = impl.rdbHandle.QueryRow(rawQuery).Scan(
		&totalStakingAddresses,
	); err != nil {
		if errors.Is(err, rdb.ErrNoRows) {
			return -1, rdb.ErrNoRows
		}
		return -1, fmt.Errorf("error scanning staking addresses by time range row: %v: %w", err, rdb.ErrQuery)
	}
	return totalStakingAddresses, nil
}

func (impl *ReportDashboard) GetAddressesOfRedeemedCouponsByTimeRangeDirectly(from string, to string) (int64, error) {
	layout := "2006-01-02"
	fromDateTime, err := time.Parse(layout, from)
	if err != nil {
		return -1, err
	}
	fromDate := fromDateTime.Truncate(24 * time.Hour).UnixNano()

	toDateTime, err := time.Parse(layout, to)
	if err != nil {
		return -1, err
	}
	toDate := toDateTime.Truncate(24 * time.Hour).Add(24 * time.Hour).UnixNano()

	rawQuery := fmt.Sprintf("SELECT COUNT(*) "+
		"FROM (SELECT DISTINCT from_address "+
		"FROM view_transactions "+
		"WHERE "+
		"block_time >= %d AND "+
		"block_time < %d AND "+
		"tx_type = '%s') AS dt", fromDate, toDate, "exchangeWithValue")

	var totalAddressesOfRedeemedCoupons int64
	if err = impl.rdbHandle.QueryRow(rawQuery).Scan(
		&totalAddressesOfRedeemedCoupons,
	); err != nil {
		if errors.Is(err, rdb.ErrNoRows) {
			return -1, rdb.ErrNoRows
		}
		return -1, fmt.Errorf("error scanning addresses of redeemed coupons by time range row: %v: %w", err, rdb.ErrQuery)
	}
	return totalAddressesOfRedeemedCoupons, nil
}

type ReportDashboardData struct {
	DateTime                     string  `json:"dateTime,omitempty"`
	TotalTxOfRedeemedCoupons     int64   `json:"totalTxOfRedeemedCoupons"`
	TotalRedeemedCouponAddresses int64   `json:"totalRedeemedCouponAddresses"`
	TotalAsaOfRedeemedCoupons    float64 `json:"totalAsaOfRedeemedCoupons"`
	TotalStakingTransactions     int64   `json:"totalStakingTransactions"`
	TotalStakingAddresses        int64   `json:"totalStakingAddresses"`
	TotalAsaStaked               float64 `json:"totalAsaStaked"`
	TotalNewAddresses            int64   `json:"totalNewAddresses"`
	TotalAsaWithdrawnFromTiki    float64 `json:"totalAsaWithdrawnFromTiki"`
	TotalAsaOnchainRewards       float64 `json:"totalAsaOnchainRewards"`
	TotalTransactions            int64   `json:"totalTransactions"`
	TotalActiveAddresses         int64   `json:"totalActiveAddresses,omitempty"`
	TotalUpToDateAddresses       int64   `json:"totalUpToDateAddresses,omitempty"`
	TotalUpToDateTransactions    int64   `json:"totalUpToDateTransactions,omitempty"`
}

type ReportDashboardOverall struct {
	Data    []ReportDashboardData `json:"data"`
	Overall ReportDashboardData   `json:"overall"`
}
