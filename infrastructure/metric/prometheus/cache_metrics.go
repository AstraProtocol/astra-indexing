package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
)

const (
	appName = "appName"
)

var (
	paramGaugeVecHit = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cache_hits_metric",
		},
		[]string{
			appName,
		},
	)

	paramGaugeVecMissed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cache_missed_metric",
		},
		[]string{
			appName,
		},
	)

	paramGaugeVecInsertion = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cache_insertion_metric",
		},
		[]string{
			appName,
		},
	)

	paramGaugeVecEviction = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cache_eviction_metric",
		},
		[]string{
			appName,
		},
	)
)

func RecordCacheHits(appNameRecord string, param string) {
	paramFloat, _ := strconv.ParseFloat(param, 64)
	paramGaugeVecHit.With(
		prometheus.Labels{
			appName: appNameRecord,
		},
	).Set(paramFloat)
}

func RecordCacheMissed(appNameRecord string, param string) {
	paramFloat, _ := strconv.ParseFloat(param, 64)
	paramGaugeVecMissed.With(
		prometheus.Labels{
			appName: appNameRecord,
		},
	).Set(paramFloat)
}

func RecordCacheInsertion(appNameRecord string, param string) {
	paramFloat, _ := strconv.ParseFloat(param, 64)
	paramGaugeVecInsertion.With(
		prometheus.Labels{
			appName: appNameRecord,
		},
	).Set(paramFloat)
}

func RecordCacheEviction(appNameRecord string, param string) {
	paramFloat, _ := strconv.ParseFloat(param, 64)
	paramGaugeVecEviction.With(
		prometheus.Labels{
			appName: appNameRecord,
		},
	).Set(paramFloat)
}
