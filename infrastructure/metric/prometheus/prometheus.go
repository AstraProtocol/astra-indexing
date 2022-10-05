package prometheus

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

func Run(path, port string) error {
	register := prometheus.DefaultRegisterer
	register.MustRegister(projectionExecTime)
	register.MustRegister(projectionLatestHeight)
	register.MustRegister(apiExecTime)
	register.MustRegister(paramGaugeVec)
	handler := promhttp.InstrumentMetricHandler(
		register, promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{}),
	)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), handler); err != nil {
		return err
	}

	return nil
}
