package stats

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

type PrometheusStatsCollector interface {
	CountCall(env, node, path, method string, status int)
	CountError(env, node, path string, status int)
	GetLabeledResponseTimePeersHistogramTimer(env, node, path, method string) *prometheus.Timer
}

type PrometheusStats struct {
	gaugeVec      *prometheus.GaugeVec
	counterVec    *prometheus.CounterVec
	errorsCounter *prometheus.CounterVec
	histogramVec  *prometheus.HistogramVec
}

func NewPrometheusStats(
	gaugeVec *prometheus.GaugeVec,
	counterVec *prometheus.CounterVec,
	errorsCounter *prometheus.CounterVec,
	histogramVec *prometheus.HistogramVec,
) *PrometheusStats {
	return &PrometheusStats{gaugeVec: gaugeVec, counterVec: counterVec, errorsCounter: errorsCounter, histogramVec: histogramVec}
}

func (p *PrometheusStats) CountCall(env, node, path, method string, status int) {
	code := strconv.Itoa(status)
	p.counterVec.WithLabelValues(
		env,
		node,
		path,
		method,
		code,
	).Inc()
}

// counts various errors
func (p *PrometheusStats) CountError(env, node, path string, status int) {
	code := strconv.Itoa(status)
	p.errorsCounter.WithLabelValues(
		env,
		node,
		path,
		code,
	).Inc()
}

func (p *PrometheusStats) GetLabeledResponseTimePeersHistogramTimer(env, node, path, method string) *prometheus.Timer {
	return prometheus.NewTimer(
		p.histogramVec.WithLabelValues(
			env,
			node,
			path,
			method,
		),
	)
}
