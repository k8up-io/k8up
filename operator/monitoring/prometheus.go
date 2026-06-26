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

	scheduleGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "k8up_schedules_gauge",
		Help: "How many schedules this k8up manages",
	}, []string{
		"namespace",
	})

	scheduleLastJobSucceeded = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "k8up_schedule_last_job_succeeded",
		Help: "1 if the most recent job triggered by this schedule succeeded, 0 if it failed.",
	}, []string{"namespace", "schedule", "jobType"})
)

func IncFailureCounters(namespace string, jobType v1.JobType) {
	metricsFailureCounter.WithLabelValues(namespace, jobType.String()).Inc()
	metricsTotalCounter.WithLabelValues(namespace, jobType.String()).Inc()
}

func IncSuccessCounters(namespace string, jobType v1.JobType) {
	metricsSuccessCounter.WithLabelValues(namespace, jobType.String()).Inc()
	metricsTotalCounter.WithLabelValues(namespace, jobType.String()).Inc()
}

func IncRegisteredSchedulesGauge(namespace string) {
	scheduleGauge.WithLabelValues(namespace).Inc()
}

func DecRegisteredSchedulesGauge(namespace string) {
	scheduleGauge.WithLabelValues(namespace).Dec()
}

// SetScheduleLastJobStatus records whether the last job triggered by a schedule
// succeeded (1) or failed (0). Call this after each scheduled job completes.
func SetScheduleLastJobStatus(namespace, scheduleName string, jobType v1.JobType, succeeded bool) {
	val := 0.0
	if succeeded {
		val = 1.0
	}
	scheduleLastJobSucceeded.WithLabelValues(namespace, scheduleName, jobType.String()).Set(val)
}

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(
		metricsFailureCounter,
		metricsSuccessCounter,
		metricsTotalCounter,
		scheduleGauge,
		scheduleLastJobSucceeded,
	)
}
