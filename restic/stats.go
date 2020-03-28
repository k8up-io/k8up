package restic

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/prometheus/client_golang/prometheus"
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
	Name          string      `json:"name,omitempty"`
	BucketName    string      `json:"bucket_name,omitempty"`
	BackupMetrics *RawMetrics `json:"backup_metrics,omitempty"`
	Snapshots     []Snapshot  `json:"snapshots,omitempty"`
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

	errors := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "last_errors",
		Help:      "How many errors the backup or check had",
	}, labels)

	availableSnapshots := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "available_snapshots",
		Help:      "How many snapshots are available",
	})

	newFiles := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "new_files_during_backup",
		Help:      "How many new files were backed up during the last backup",
	}, labels)

	changedFiles := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "changed_files_during_backup",
		Help:      "How many changed files were backed up during the last backup",
	}, labels)

	unmodifiedFiles := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "unmodified_files_during_backup",
		Help:      "How many files were skipped due to no modifications",
	}, labels)

	newDirs := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "new_directories_during_backup",
		Help:      "How many new directories were backed up during the last backup",
	}, labels)

	changedDirs := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "changed_directories_during_backup",
		Help:      "How many changed directories were backed up during the last backup",
	}, labels)

	unmodifiedDirs := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "unmodified_directories_during_backup",
		Help:      "How many directories were skipped due to no modifications",
	}, labels)

	dataTransferred := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "data_transferred_during_backup",
		Help:      "Amount of data transferred during last backup",
	}, labels)

	return &PromMetrics{
		Errors:             errors,
		AvailableSnapshots: availableSnapshots,
		NewFiles:           newFiles,
		ChangedFiles:       changedFiles,
		UnmodifiedFiles:    unmodifiedFiles,
		NewDirs:            newDirs,
		ChangedDirs:        changedDirs,
		UnmodifiedDirs:     unmodifiedDirs,
		DataTransferred:    dataTransferred,
	}
}

// please ensure that there's a current list of snapshots in the restic instance before calling this.
func (r *Restic) parseSummary(summary backupSummary, errorCount int, folder string, startTimestamp, endTimestamp int64) *RawMetrics {
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
		hostname:              os.Getenv(Hostname),
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

func (b *BackupStats) ToJson() []byte {
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

	folders := []string{}

	files, err := ioutil.ReadDir(os.Getenv(BackupDirEnv))
	if err != nil {
		r.logger.WithName("MountCollector").Error(err, "can't list mounted folders for stats")
	}

	for _, f := range files {
		if f.IsDir() {
			folders = append(folders, f.Name())
		}
	}

	return folders
}
