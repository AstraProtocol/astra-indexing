package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
)

const (
	paramName  = "prams"
	paramLabel = "prams"
)

var (
	paramGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: paramName,
		},
		[]string{
			paramLabel,
		},
	)
)

func RecordParam(paramName string, param string) {
	paramFloat, _ := strconv.ParseFloat(param, 64)
	paramGaugeVec.With(
		prometheus.Labels{
			paramLabel: paramName,
		},
	).Set(paramFloat)
}
