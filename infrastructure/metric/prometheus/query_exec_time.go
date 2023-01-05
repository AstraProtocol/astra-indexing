package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	queryExecTimeName = "query_execution_time"
	label             = "label"
	queryType         = "query_type"
)

var (
	queryExecTime = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: apiExecTimeName,
		},
		[]string{
			label,
			queryType,
		},
	)
)

func RecordQueryExecTime(methodName, queryType string, timeInMilliseconds int64) {
	queryExecTime.With(
		prometheus.Labels{
			label:     methodName,
			queryType: queryType,
		},
	).Observe(float64(timeInMilliseconds))
}
