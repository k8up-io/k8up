package cli

import (
	"encoding/json"
	"os"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/k8up-io/k8up/v2/restic/cfg"
	"github.com/k8up-io/k8up/v2/restic/dto"
	"github.com/k8up-io/k8up/v2/restic/logging"
)

const (
	prometheusNamespace = "k8up"
	prometheusSubsystem = "backup_restic"
)

// RawMetrics contains the raw metrics that can be obtained at the end of a
// backup. Webhookdata and prometheus statistics are derived from it.
type RawMetrics struct {
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
	availableSnapshots    float64
	Folder                string
	hostname              string
	ID                    string `json:"id"`
}

type BackupStats struct {
	Name          string         `json:"name,omitempty"`
	BucketName    string         `json:"bucket_name,omitempty"`
	BackupMetrics *RawMetrics    `json:"backup_metrics,omitempty"`
	Snapshots     []dto.Snapshot `json:"snapshots,omitempty"`
}

type PromMetrics struct {
	Errors             *prometheus.GaugeVec
	AvailableSnapshots prometheus.Gauge
	NewFiles           *prometheus.GaugeVec
	ChangedFiles       *prometheus.GaugeVec
	UnmodifiedFiles    *prometheus.GaugeVec
	NewDirs            *prometheus.GaugeVec
	ChangedDirs        *prometheus.GaugeVec
	UnmodifiedDirs     *prometheus.GaugeVec
	DataTransferred    *prometheus.GaugeVec
}

func newPromMetrics() *PromMetrics {

	labels := []string{
		"pvc",
		"namespace",
	}

	return &PromMetrics{
		Errors:             errorsGaugeVec(labels),
		AvailableSnapshots: availableSnapshotsGauge(),
		NewFiles:           newFilesGaugeVec(labels),
		ChangedFiles:       changedFiledGaugeVec(labels),
		UnmodifiedFiles:    unmodifiedFilesGaugeVec(labels),
		NewDirs:            newDirsGaugeVec(labels),
		ChangedDirs:        changedDirsGaugeVec(labels),
		UnmodifiedDirs:     unmodifiedDirsGaugeVec(labels),
		DataTransferred:    dataTransferredGaugeVec(labels),
	}
}

func dataTransferredGaugeVec(labels []string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: prometheusNamespace,
		Subsystem: prometheusSubsystem,
		Name:      "data_transferred_during_backup",
		Help:      "Amount of data transferred during last backup",
	}, labels)
}

func unmodifiedDirsGaugeVec(labels []string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: prometheusNamespace,
		Subsystem: prometheusSubsystem,
		Name:      "unmodified_directories_during_backup",
		Help:      "How many directories were skipped due to no modifications",
	}, labels)
}

func changedDirsGaugeVec(labels []string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: prometheusNamespace,
		Subsystem: prometheusSubsystem,
		Name:      "changed_directories_during_backup",
		Help:      "How many changed directories were backed up during the last backup",
	}, labels)
}

func newDirsGaugeVec(labels []string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: prometheusNamespace,
		Subsystem: prometheusSubsystem,
		Name:      "new_directories_during_backup",
		Help:      "How many new directories were backed up during the last backup",
	}, labels)
}

func unmodifiedFilesGaugeVec(labels []string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: prometheusNamespace,
		Subsystem: prometheusSubsystem,
		Name:      "unmodified_files_during_backup",
		Help:      "How many files were skipped due to no modifications",
	}, labels)
}

func changedFiledGaugeVec(labels []string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: prometheusNamespace,
		Subsystem: prometheusSubsystem,
		Name:      "changed_files_during_backup",
		Help:      "How many changed files were backed up during the last backup",
	}, labels)
}

func newFilesGaugeVec(labels []string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: prometheusNamespace,
		Subsystem: prometheusSubsystem,
		Name:      "new_files_during_backup",
		Help:      "How many new files were backed up during the last backup",
	}, labels)
}

func availableSnapshotsGauge() prometheus.Gauge {
	return prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: prometheusNamespace,
		Subsystem: prometheusSubsystem,
		Name:      "available_snapshots",
		Help:      "How many snapshots are available",
	})
}

func errorsGaugeVec(labels []string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: prometheusNamespace,
		Subsystem: prometheusSubsystem,
		Name:      "last_errors",
		Help:      "How many errors the backup or check had",
	}, labels)
}

// please ensure that there's a current list of snapshots in the restic instance before calling this.
func (r *Restic) parseSummary(summary logging.BackupSummary, errorCount int, folder string, startTimestamp, endTimestamp int64) *RawMetrics {
	return &RawMetrics{
		NewDirs:               float64(summary.DirsNew),
		NewFiles:              float64(summary.FilesNew),
		ChangedFiles:          float64(summary.FilesChanged),
		UnmodifiedFiles:       float64(summary.FilesUnmodified),
		ChangedDirs:           float64(summary.DirsChanged),
		UnmodifiedDirs:        float64(summary.DirsUnmodified),
		Errors:                float64(errorCount),
		MountedPVCs:           r.getMountedFolders(),
		availableSnapshots:    float64(len(r.snapshots)),
		Folder:                folder,
		hostname:              cfg.Config.Hostname,
		BackupStartTimestamp:  float64(startTimestamp),
		BackupEndTimestamp:    float64(endTimestamp),
		runningBackupDuration: summary.TotalDuration,
		DataTransferred:       float64(summary.DataAdded),
		ID:                    summary.SnapshotID,
	}
}

func (r *RawMetrics) prometheus() *PromMetrics {
	metrics := newPromMetrics()

	metrics.AvailableSnapshots.Set(r.availableSnapshots)
	metrics.ChangedDirs.WithLabelValues(r.Folder, r.hostname).Set(r.ChangedDirs)
	metrics.ChangedFiles.WithLabelValues(r.Folder, r.hostname).Set(r.ChangedFiles)
	metrics.Errors.WithLabelValues(r.Folder, r.hostname).Set(r.Errors)
	metrics.NewDirs.WithLabelValues(r.Folder, r.hostname).Set(r.NewDirs)
	metrics.NewFiles.WithLabelValues(r.Folder, r.hostname).Set(r.NewFiles)
	metrics.UnmodifiedDirs.WithLabelValues(r.Folder, r.hostname).Set(r.UnmodifiedDirs)
	metrics.UnmodifiedFiles.WithLabelValues(r.Folder, r.hostname).Set(r.UnmodifiedFiles)

	return metrics
}

func (b *BackupStats) ToJSON() []byte {
	jsonData, _ := json.Marshal(b)
	return jsonData
}

func (b *BackupStats) ToProm() []prometheus.Collector {
	return b.BackupMetrics.prometheus().ToProm()
}

func (p *PromMetrics) ToProm() []prometheus.Collector {
	return []prometheus.Collector{
		p.Errors,
		p.AvailableSnapshots,
		p.NewFiles,
		p.ChangedFiles,
		p.UnmodifiedFiles,
		p.NewDirs,
		p.ChangedDirs,
		p.UnmodifiedDirs,
		p.DataTransferred,
	}
}

func (r *Restic) getMountedFolders() []string {
	files, err := os.ReadDir(cfg.Config.BackupDir)
	if err != nil {
		log := r.logger.WithName("MountCollector")
		if os.IsNotExist(err) {
			log.Info("stats mount dir doesn't exist, skipping stats", "dir", cfg.Config.BackupDir)
		} else {
			log.Error(err, "can't list mounted folders for stats")
		}
		return []string{}
	}

	folders := make([]string, 0)
	for _, f := range files {
		if f.IsDir() {
			folders = append(folders, f.Name())
		}
	}

	return folders
}

type RestoreStats struct {
	RestoreLocation string   `json:"restore_location,omitempty"`
	SnapshotID      string   `json:"snapshot_ID,omitempty"`
	RestoredFiles   []string `json:"restored_files,omitempty"`
}

func (r *RestoreStats) ToJSON() []byte {
	jsonData, _ := json.Marshal(r)
	return jsonData
}
