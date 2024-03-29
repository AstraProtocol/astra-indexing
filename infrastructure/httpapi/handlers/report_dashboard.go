package handlers

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/bootstrap/config"
	"github.com/AstraProtocol/astra-indexing/external/cache"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	utils "github.com/AstraProtocol/astra-indexing/infrastructure"
	"github.com/AstraProtocol/astra-indexing/infrastructure/httpapi"
	"github.com/AstraProtocol/astra-indexing/infrastructure/metric/prometheus"
	account_view "github.com/AstraProtocol/astra-indexing/projection/account/view"
	report_dashboard_view "github.com/AstraProtocol/astra-indexing/projection/report_dashboard/view"
	transaction_view "github.com/AstraProtocol/astra-indexing/projection/transaction/view"
	"github.com/valyala/fasthttp"
)

type ReportDashboardHandler struct {
	logger                applogger.Logger
	reportDashboardView   *report_dashboard_view.ReportDashboard
	transactionsTotalView transaction_view.TransactionsTotal
	accountsView          account_view.Accounts
	astraCache            *cache.AstraCache
}

func NewReportDashboardHandler(
	logger applogger.Logger,
	rdbHandle *rdb.Handle,
	config *config.Config,
) *ReportDashboardHandler {
	return &ReportDashboardHandler{
		logger.WithFields(applogger.LogFields{
			"module": "ReportDashboardHandler",
		}),
		report_dashboard_view.NewReportDashboard(rdbHandle, config),
		transaction_view.NewTransactionsTotalView(rdbHandle),
		account_view.NewAccountsView(rdbHandle),
		cache.NewCache(),
	}
}

func (handler *ReportDashboardHandler) UpdateReportDashboardByDate(ctx *fasthttp.RequestCtx) {
	if string(ctx.QueryArgs().Peek("date")) != "" {
		date := string(ctx.QueryArgs().Peek("date"))
		status, err := handler.reportDashboardView.UpdateReportDashboardByDate(date)
		if err != nil {
			httpapi.BadRequest(ctx, err)
		}
		httpapi.Success(ctx, status)
	} else {
		httpapi.BadRequest(ctx, errors.New("date param is required"))
		return
	}
}

func (handler *ReportDashboardHandler) GetReportDashboardByTimeRange(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	recordMethod := "GetReportDashboardByTimeRange"

	layout := "2006-01-02"

	var fromDate string
	var toDate string

	if string(ctx.QueryArgs().Peek("fromDate")) == "" {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.BadRequest(ctx, errors.New("fromDate param is required"))
		return
	}
	if string(ctx.QueryArgs().Peek("toDate")) == "" {
		toDate = time.Now().Format(layout)
	} else {
		toDate = string(ctx.QueryArgs().Peek("toDate"))
	}

	fromDate = string(ctx.QueryArgs().Peek("fromDate"))

	cacheKey := fmt.Sprintf("GetReportDashboardByTimeRange%s%s", fromDate, toDate)
	var reportDashboardOverallCache report_dashboard_view.ReportDashboardOverall
	err := handler.astraCache.Get(cacheKey, &reportDashboardOverallCache)
	if err == nil {
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
		httpapi.SuccessNotWrappedResult(ctx, reportDashboardOverallCache)
		return
	}

	reportDashboardOverall, err := handler.reportDashboardView.GetReportDashboardByTimeRange(fromDate, toDate)
	if err != nil {
		handler.logger.Errorf("error get report dashboard by time range: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}

	totalUpToDateTransactions, err := handler.transactionsTotalView.FindBy("-")
	if err != nil {
		handler.logger.Errorf("error get total up-to-date transactions: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusInternalServerError), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}
	reportDashboardOverall.Overall.TotalUpToDateTransactions = totalUpToDateTransactions

	totalUpToDateAddresses, err := handler.accountsView.TotalAccount()
	if err != nil {
		handler.logger.Errorf("error get total up-to-date addresses: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusInternalServerError), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}
	reportDashboardOverall.Overall.TotalUpToDateAddresses = totalUpToDateAddresses

	totalActiveAddresses, err := handler.reportDashboardView.GetActiveAddressesByTimeRangeDirectly(fromDate, toDate)
	if err != nil {
		handler.logger.Errorf("error get active addresses by time range: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}
	reportDashboardOverall.Overall.TotalActiveAddresses = totalActiveAddresses

	totalStakingAddresses, err := handler.reportDashboardView.GetStakingAddressesByTimeRangeDirectly(fromDate, toDate)
	if err != nil {
		handler.logger.Errorf("error get staking addresses by time range: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}
	reportDashboardOverall.Overall.TotalStakingAddresses = totalStakingAddresses

	totalRedeemedCouponsAddresses, err := handler.reportDashboardView.GetAddressesOfRedeemedCouponsByTimeRangeDirectly(fromDate, toDate)
	if err != nil {
		handler.logger.Errorf("error get redeemed coupons addresses by time range: %v", err)
		prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(fasthttp.StatusBadRequest), "GET", time.Since(startTime).Milliseconds())
		httpapi.InternalServerError(ctx)
		return
	}
	reportDashboardOverall.Overall.TotalRedeemedCouponAddresses = totalRedeemedCouponsAddresses

	prometheus.RecordApiExecTime(recordMethod, strconv.Itoa(200), "GET", time.Since(startTime).Milliseconds())
	handler.astraCache.Set(cacheKey, reportDashboardOverall, utils.TIME_CACHE_LONG)
	httpapi.SuccessNotWrappedResult(ctx, reportDashboardOverall)
}
