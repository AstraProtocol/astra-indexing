package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	apiExecTimeName            = "api_execution_time"
	apiExecTimeProjectionLabel = "api"
	httpCode                   = "code"
	httpMethod                 = "method"
)

var (
	apiExecTime = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: apiExecTimeName,
		},
		[]string{
			apiExecTimeProjectionLabel,
			httpCode,
			httpMethod,
		},
	)
)

func RecordApiExecTime(apiName, code, method string, timeInMilliseconds int64) {
	apiExecTime.With(
		prometheus.Labels{
			apiExecTimeProjectionLabel: apiName,
			httpCode:                   code,
			httpMethod:                 method,
		},
	).Observe(float64(timeInMilliseconds))
}
