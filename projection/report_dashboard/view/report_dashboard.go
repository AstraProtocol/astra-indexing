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
	tikiAddress := view.config.CronjobReportDashboard.TikiAddress
	dateTime, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "NOK", err
	}
	currentDate := dateTime.Truncate(24 * time.Hour).UnixNano()
	nextDate := dateTime.Truncate(24 * time.Hour).Add(24 * time.Hour).UnixNano()
	prevDate := dateTime.Truncate(24 * time.Hour).Add(-24 * time.Hour).UnixNano()

	view.rdbReportDashboard.UpdateTotalNewAddressesWithRDbHandle(currentDate, prevDate)
	view.rdbReportDashboard.UpdateTotalTxsOfRedeemedCouponsWithRDbHandle(currentDate, nextDate)
	view.rdbReportDashboard.UpdateTotalAddressesOfRedeemedCouponsWithRDbHandle(currentDate, nextDate)
	view.rdbReportDashboard.UpdateTotalAstraOfRedeemedCouponsWithRDbHandle(currentDate, nextDate)
	view.rdbReportDashboard.UpdateTotalAstraOnchainRewardsWithRDbHandle(currentDate, nextDate)
	view.rdbReportDashboard.UpdateTotalAstraStakedWithRDbHandle(currentDate, nextDate)
	view.rdbReportDashboard.UpdateTotalAstraWithdrawnFromTikiWithRDbHandle(currentDate, nextDate, tikiAddress)
	view.rdbReportDashboard.UpdateTotalStakingAddressesWithRDbHandle(currentDate, nextDate)
	view.rdbReportDashboard.UpdateTotalStakingTxsWithRDbHandle(currentDate, nextDate)

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
		"cs.number_of_transactions, cs.total_addresses, cs.active_addresses "+
		"FROM report_dashboard AS rd "+
		"INNER JOIN chain_stats AS cs ON rd.date_time = cs.date_time "+
		"WHERE rd.date_time >= %d AND rd.date_time < %d", fromDate, toDate)

	reportDashboardDataList := make([]ReportDashboardData, 0)

	rowsResult, err := impl.rdbHandle.Query(rawQuery)
	if err != nil {
		return ReportDashboardOverall{}, fmt.Errorf("error executing get report dashboard by time range select SQL: %v: %w", err, rdb.ErrQuery)
	}
	defer rowsResult.Close()

	for rowsResult.Next() {
		var result ReportDashboardData
		if err = rowsResult.Scan(
			&result.DateTime,
			&result.TotalTxOfRedeemedCoupons,
			&result.TotalRedeemedCouponAddresses,
			&result.TotalAsaOfRedeemedCoupons,
			&result.TotalStakingTransactions,
			&result.TotalStakingAddresses,
			&result.TotalAsaStaked,
			&result.TotalNewAddresses,
			&result.TotalAsaWithdrawnFromTiki,
			&result.TotalAsaOnchainRewards,
			&result.TotalTransactions,
			&result.TotalAddresses,
			&result.TotalActiveAddresses,
		); err != nil {
			if errors.Is(err, rdb.ErrNoRows) {
				return ReportDashboardOverall{}, rdb.ErrNoRows
			}
			return ReportDashboardOverall{}, fmt.Errorf("error scanning get report dashboard by time range row: %v: %w", err, rdb.ErrQuery)
		}
		reportDashboardDataList = append(reportDashboardDataList, result)
	}
	var reportDashboardOverall ReportDashboardOverall
	reportDashboardOverall.Data = reportDashboardDataList

	totalAsaOfRedeemedCouponsOverall := float64(0)
	totalAsaStakedOverall := float64(0)
	totalAsaWithdrawnFromTikiOverall := float64(0)
	totalAsaOnchainRewardsOverall := float64(0)

	for _, reportDashboardData := range reportDashboardDataList {
		totalAsaOfRedeemedCoupons, _ := strconv.ParseFloat(strings.TrimSpace(reportDashboardData.TotalAsaOfRedeemedCoupons), 64)
		totalAsaStaked, _ := strconv.ParseFloat(strings.TrimSpace(reportDashboardData.TotalAsaStaked), 64)
		totalAsaWithdrawnFromTiki, _ := strconv.ParseFloat(strings.TrimSpace(reportDashboardData.TotalAsaWithdrawnFromTiki), 64)
		totalAsaOnchainRewards, _ := strconv.ParseFloat(strings.TrimSpace(reportDashboardData.TotalAsaOnchainRewards), 64)

		totalAsaOfRedeemedCouponsOverall += totalAsaOfRedeemedCoupons
		totalAsaStakedOverall += totalAsaStaked
		totalAsaWithdrawnFromTikiOverall += totalAsaWithdrawnFromTiki
		totalAsaOnchainRewardsOverall += totalAsaOnchainRewards

		reportDashboardOverall.Overall.TotalActiveAddresses += reportDashboardData.TotalActiveAddresses
		reportDashboardOverall.Overall.TotalAddresses += reportDashboardData.TotalAddresses
		reportDashboardOverall.Overall.TotalNewAddresses += reportDashboardData.TotalNewAddresses
		reportDashboardOverall.Overall.TotalRedeemedCouponAddresses += reportDashboardData.TotalRedeemedCouponAddresses
		reportDashboardOverall.Overall.TotalStakingAddresses += reportDashboardData.TotalStakingAddresses
		reportDashboardOverall.Overall.TotalStakingTransactions += reportDashboardData.TotalStakingTransactions
		reportDashboardOverall.Overall.TotalTransactions += reportDashboardData.TotalTransactions
		reportDashboardOverall.Overall.TotalTxOfRedeemedCoupons += reportDashboardData.TotalTxOfRedeemedCoupons
	}
	reportDashboardOverall.Overall.TotalAsaOfRedeemedCoupons = fmt.Sprint(totalAsaOfRedeemedCouponsOverall)
	reportDashboardOverall.Overall.TotalAsaStaked = fmt.Sprint(totalAsaStakedOverall)
	reportDashboardOverall.Overall.TotalAsaWithdrawnFromTiki = fmt.Sprint(totalAsaWithdrawnFromTikiOverall)
	reportDashboardOverall.Overall.TotalAsaOnchainRewards = fmt.Sprint(totalAsaOnchainRewardsOverall)

	return reportDashboardOverall, nil
}

type ReportDashboardData struct {
	DateTime                     int64  `json:"dateTime,omitempty"`
	TotalTxOfRedeemedCoupons     int64  `json:"totalTxOfRedeemedCoupons"`
	TotalRedeemedCouponAddresses int64  `json:"totalRedeemedCouponAddresses"`
	TotalAsaOfRedeemedCoupons    string `json:"totalAsaOfRedeemedCoupons"`
	TotalStakingTransactions     int64  `json:"totalStakingTransactions"`
	TotalStakingAddresses        int64  `json:"totalStakingAddresses"`
	TotalAsaStaked               string `json:"totalAsaStaked"`
	TotalNewAddresses            int64  `json:"totalNewAddresses"`
	TotalAsaWithdrawnFromTiki    string `json:"totalAsaWithdrawnFromTiki"`
	TotalAsaOnchainRewards       string `json:"totalAsaOnchainRewards"`
	TotalTransactions            int64  `json:"totalTransactions"`
	TotalAddresses               int64  `json:"totalAddresses"`
	TotalActiveAddresses         int64  `json:"totalActiveAddresses"`
}

type ReportDashboardOverall struct {
	Data    []ReportDashboardData `json:"data"`
	Overall ReportDashboardData   `json:"overall"`
}
