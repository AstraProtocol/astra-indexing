package handlers

import (
	"errors"

	"github.com/AstraProtocol/astra-indexing/appinterface/rdb"
	"github.com/AstraProtocol/astra-indexing/bootstrap/config"
	applogger "github.com/AstraProtocol/astra-indexing/external/logger"
	"github.com/AstraProtocol/astra-indexing/infrastructure/httpapi"
	report_dashboard_view "github.com/AstraProtocol/astra-indexing/projection/report_dashboard/view"
	"github.com/valyala/fasthttp"
)

type ReportDashboardHandler struct {
	logger              applogger.Logger
	reportDashboardView *report_dashboard_view.ReportDashboard
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
