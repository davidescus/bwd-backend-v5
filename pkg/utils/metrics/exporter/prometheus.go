package exporter

import (
	"fmt"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	hostname, _ = os.Hostname()
)

func GetCounter(namespace, metricName string, labelNames []string) *prometheus.CounterVec {
	return getAndRegisterCounterVec(namespace, metricName, labelNames)
}

func GetGauge(namespace, metricName string, labelNames []string) *prometheus.GaugeVec {
	return getAndRegisterGaugeVec(namespace, metricName, labelNames)
}

func GetSummary(namespace, metricName string, labelNames []string) *prometheus.SummaryVec {
	return getAndRegisterSummaryVec(namespace, metricName, labelNames)
}

func GetHistogram(namespace, metricName string, labelNames []string) *prometheus.HistogramVec {
	return getAndRegisterHistogramVec(namespace, metricName, labelNames)
}

func getAndRegisterCounterVec(ns, metricName string, labelNames []string) *prometheus.CounterVec {
	options := prometheus.CounterOpts{
		Namespace: ns,
		Name:      metricName,
		ConstLabels: prometheus.Labels{
			"hostname": hostname,
		},
	}
	counter := prometheus.NewCounterVec(options, labelNames)
	prometheus.MustRegister(counter)

	return counter
}

func getAndRegisterGaugeVec(ns, metricName string, labelNames []string) *prometheus.GaugeVec {
	options := prometheus.GaugeOpts{
		Namespace: ns,
		Name:      metricName,
		ConstLabels: prometheus.Labels{
			"hostname": hostname,
		},
	}
	gauge := prometheus.NewGaugeVec(options, labelNames)
	prometheus.MustRegister(gauge)

	return gauge
}

func getAndRegisterSummaryVec(ns, metricName string, labelNames []string) *prometheus.SummaryVec {
	options := prometheus.SummaryOpts{
		Namespace: ns,
		Name:      metricName,
		ConstLabels: prometheus.Labels{
			"hostname": hostname,
		},
		Objectives: map[float64]float64{0.5: 0.05, 0.75: 0.025, 0.9: 0.01, 0.95: 0.005, 0.99: 0.001, 0.999: 0.0001},
	}
	summary := prometheus.NewSummaryVec(options, labelNames)
	prometheus.MustRegister(summary)

	return summary
}

func getAndRegisterHistogramVec(ns, metricName string, labelNames []string) *prometheus.HistogramVec {
	options := prometheus.HistogramOpts{
		Namespace: ns,
		Name:      metricName,
		ConstLabels: prometheus.Labels{
			"hostname": hostname,
		},
		Buckets: []float64{10, 50, 100, 250, 500}, // expressed in units/MS not as a percentage
	}
	histogram := prometheus.NewHistogramVec(options, labelNames)
	prometheus.MustRegister(histogram)

	return histogram
}

func GetExporter(port string) {
	server := http.NewServeMux()
	server.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(fmt.Sprintf(":%s", port), server)
}
