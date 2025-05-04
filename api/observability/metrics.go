package apiObserv

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	PromHttpTotalRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{ // metric name will be Namespace_Name
			Namespace: "http",
			Name:      "requests_total",
			Help:      "Number of HTTP request by path", // description of the metric
		},
		[]string{}) // labels to be added to the metric

	PromHttpTotalResponse = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "http",
			Name:      "responses_total",
			Help:      "Number of HTTP request by path",
		},
		[]string{})
	PromHttpResponseStatus = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "http",
		Name:      "response_status_total",
		Help:      "Total number of response with specific status code",
	},
		[]string{"path", "code"})

	PromHttpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "http",
		Name:      "response_time_seconds",
		Help:      "Duration of HTTP requests.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"path"})

	PromApplicationVersion = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "application",
		Name:      "info",
		Help:      "Application binary version",
	}, []string{"version"})
)

func PromInit() {
	prometheus.MustRegister(
		PromHttpTotalRequests,
		PromHttpResponseStatus,
		PromHttpDuration,
		PromApplicationVersion,
		PromHttpTotalResponse,
	)
}
