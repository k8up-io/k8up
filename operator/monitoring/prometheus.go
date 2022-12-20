package monitoring

import (
	"github.com/k8up-io/k8up/v2/api/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	promLabels = []string{
		"namespace",
		"jobType",
	}
	metricsFailureCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "k8up_jobs_failed_counter",
		Help: "The total number of jobs that failed",
	}, promLabels)

	metricsSuccessCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "k8up_jobs_successful_counter",
		Help: "The total number of jobs that went through cleanly",
	}, promLabels)

	metricsTotalCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "k8up_jobs_total",
		Help: "The total amount of all jobs run",
	}, promLabels)
)

func IncFailureCounters(namespace string, jobType v1.JobType) {
	metricsFailureCounter.WithLabelValues(namespace, jobType.String()).Inc()
	metricsTotalCounter.WithLabelValues(namespace, jobType.String()).Inc()
}

func IncSuccessCounters(namespace string, jobType v1.JobType) {
	metricsSuccessCounter.WithLabelValues(namespace, jobType.String()).Inc()
	metricsTotalCounter.WithLabelValues(namespace, jobType.String()).Inc()
}

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(metricsFailureCounter, metricsSuccessCounter, metricsTotalCounter)
}
