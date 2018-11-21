package main

import (
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

const (
	namespace = "baas"
	subsystem = "backup_restic"
)

type rawMetrics struct {
	runningBackupDuration float64
	BackupStartTimestamp  float64  `json:"backup_start_timestamp"`
	BackupEndTimestamp    float64  `json:"backup_end_timestamp"`
	Errors                float64  `json:"errors"`
	NewFiles              float64  `json:"new_files"`
	ChangedFiles          float64  `json:"changed_files"`
	UnmodifiedFiles       float64  `json:"unmodified_files"`
	NewDirs               float64  `json:"new_dirs"`
	ChangedDirs           float64  `json:"changed_dirs"`
	UnmodifiedDirs        float64  `json:"unmodified_dirs"`
	DataTransferred       float64  `json:"data_transferred"`
	MountedPVCs           []string `json:"mounted_PVCs"`
}

type resticMetrics struct {
	runningBackupDuration prometheus.Counter
	BackupStartTimestamp  prometheus.Gauge
	BackupEndTimestamp    prometheus.Gauge
	Errors                *prometheus.GaugeVec
	AvailableSnapshots    prometheus.Gauge
	NewFiles              *prometheus.GaugeVec
	ChangedFiles          *prometheus.GaugeVec
	UnmodifiedFiles       *prometheus.GaugeVec
	NewDirs               *prometheus.GaugeVec
	ChangedDirs           *prometheus.GaugeVec
	UnmodifiedDirs        *prometheus.GaugeVec
	DataTransferred       *prometheus.GaugeVec
	url                   string
	interval              int
}

func newResticMetrics(url string) *resticMetrics {

	labels := []string{
		"pvc",
		"namespace",
	}

	backupStartTimestamp := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "last_start_backup_timestamp",
		Help:      "Timestamp when the last backup was started",
	})

	errors := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "last_errors",
		Help:      "How many errors the backup or check had",
	}, labels)

	backupDuration := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "running_backup_duration",
		Help:      "How long the current backup is taking",
	})

	backupEndTimestamp := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "last_end_backup_timestamp",
		Help:      "Timestamp when the last backup was finished",
	})

	availableSnapshots := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "available_snapshots",
		Help:      "How many snapshots are available",
	})

	newFiles := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "new_files_during_backup",
		Help:      "How many new files were backed up during the last backup",
	}, labels)

	changedFiles := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "changed_files_during_backup",
		Help:      "How many changed files were backed up during the last backup",
	}, labels)

	unmodifiedFiles := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "unmodified_files_during_backup",
		Help:      "How many files were skipped due to no modifications",
	}, labels)

	newDirs := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "new_directories_during_backup",
		Help:      "How many new directories were backed up during the last backup",
	}, labels)

	changedDirs := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "changed_directories_during_backup",
		Help:      "How many changed directories were backed up during the last backup",
	}, labels)

	unmodifiedDirs := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "unmodified_directories_during_backup",
		Help:      "How many directories were skipped due to no modifications",
	}, labels)

	dataTransferred := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "data_transferred_during_backup",
		Help:      "Amount of data transferred during last backup",
	}, labels)

	return &resticMetrics{
		url:                   url,
		BackupStartTimestamp:  backupStartTimestamp,
		Errors:                errors,
		runningBackupDuration: backupDuration,
		interval:              1,
		BackupEndTimestamp:    backupEndTimestamp,
		AvailableSnapshots:    availableSnapshots,
		NewFiles:              newFiles,
		ChangedFiles:          changedFiles,
		UnmodifiedFiles:       unmodifiedFiles,
		NewDirs:               newDirs,
		ChangedDirs:           changedDirs,
		UnmodifiedDirs:        unmodifiedDirs,
		DataTransferred:       dataTransferred,
	}
}

func (r *resticMetrics) startUpdating() {
	tick := time.NewTicker(time.Duration(r.interval) * time.Second)

	for {
		select {
		case <-tick.C:
			r.runningBackupDuration.Add(float64(r.interval))
			r.Update(r.runningBackupDuration)
		}
	}
}

func (r *resticMetrics) Update(collector prometheus.Collector) {
	push.New(r.url, "restic_backup").Collector(collector).
		Grouping("instance", os.Getenv(hostname)).
		Add()
}
