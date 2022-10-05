package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
)

const (
	cacheMetrics = "cache_metric"
	appName      = "appName"
)

var (
	paramGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: cacheMetrics,
		},
		[]string{
			appName,
		},
	)
)

func RecordParam(appNameRecord string, param string) {
	paramFloat, _ := strconv.ParseFloat(param, 64)
	paramGaugeVec.With(
		prometheus.Labels{
			appName: appNameRecord,
		},
	).Set(paramFloat)
}
