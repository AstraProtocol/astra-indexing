package view

import (
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
