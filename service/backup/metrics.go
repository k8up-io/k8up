package backup

import (
	"git.vshn.net/vshn/baas/monitoring"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "baas"
	subsystem = "backup_operator"
)

// OperatorMetrics holds the collectors
type operatorMetrics struct {
	RunningBackups prometheus.Gauge
}

// newOperatorMetrics creates and registers all the metrics that the operator
// has
func newOperatorMetrics(endPoint monitoring.MonitorEndpoint) *operatorMetrics {
	runningBackups := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "running_backups",
		Help:      "How many backups are running right now",
	})

	endPoint.Register(runningBackups)

	return &operatorMetrics{
		RunningBackups: runningBackups,
	}
}
