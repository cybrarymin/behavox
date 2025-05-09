package observ

import (
	data "github.com/cybrarymin/behavox/internal/models"
	"github.com/prometheus/client_golang/prometheus"
)

// Api http related metrics
var (
	PromHttpTotalRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{ // metric name will be Namespace_Name
			Namespace: "http",
			Name:      "requests_total",
			Help:      "Number of HTTP request", // description of the metric
		},
		[]string{}) // labels to be added to the metric

	PromHttpTotalPathRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{ // metric name will be Namespace_Name
			Namespace: "http",
			Name:      "requests_path_total",
			Help:      "Number of HTTP request by path", // description of the metric
		},
		[]string{"path"}) // labels to be added to the metric

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

// Worker event consumer related metrics
var (
	PromEventTotalProcessed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "worker",
		Name:      "events_processed_total",
		Help:      "Total number of events processed so far",
	}, []string{})

	PromEventTotalProcessStatus = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "worker",
		Name:      "events_processed_status_total",
		Help:      "total number of events processed based on status",
	}, []string{"event_process_status", "event_type"})

	PromEventRetryCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "worker",
		Name:      "events_retry_total",
		Help:      "Total Number of event processing retries",
	}, []string{"event_type"})

	PromEventProcessingDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "worker",
		Name:      "events_processing_duration_seconds",
		Help:      "Duration of event processing in seconds",
		Buckets:   prometheus.DefBuckets,
	}, []string{"event_type"})
)

// EventQueue related metrics
var (
	PromEventQueueCapacity = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "queue",
		Name:      "total_capacity",
		Help:      "total capacity of the queue",
	}, []string{})

	PromEventQueueWaitTime = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "queue",
		Name:      "wait_time_seconds",
		Help:      "Time events spend waiting in queue before processing",
		Buckets:   prometheus.DefBuckets,
	}, []string{"event_type"})
)

func PromInit(eq *data.EventQueue, appVersion string) {
	// Event Queue Gauge function
	PromEventQueueSize := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "queue",
		Name:      "current_size",
		Help:      "number of events inside the queue",
	}, func() float64 {
		return float64(len(eq.Events))
	})
	// setting eventQueue maximum capacity metric
	PromEventQueueCapacity.WithLabelValues().Set(float64(eq.Capacity))

	// setting application version metric
	PromApplicationVersion.WithLabelValues(appVersion).Set(1)

	prometheus.MustRegister(
		PromHttpTotalRequests,
		PromHttpTotalPathRequests,
		PromHttpResponseStatus,
		PromHttpDuration,
		PromApplicationVersion,
		PromHttpTotalResponse,
		PromEventTotalProcessed,
		PromEventTotalProcessStatus,
		PromEventProcessingDuration,
		PromEventQueueSize,
		PromEventQueueCapacity,
		PromEventQueueWaitTime,
		PromEventRetryCount,
	)
}
