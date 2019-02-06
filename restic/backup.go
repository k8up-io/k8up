package restic

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"git.vshn.net/vshn/wrestic/kubernetes"
	"git.vshn.net/vshn/wrestic/output"
	"github.com/prometheus/client_golang/prometheus"
)

// BackupStruct holds the state of a backup command.
type BackupStruct struct {
	genericCommand
	folderList     []string
	backupDir      string
	rawMetrics     []rawMetrics
	snapshotLister *ListSnapshotsStruct
	parsed         bool
	startTimeStamp int64
	endTimeStamp   int64
	snapshots      []Snapshot
}

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
	availableSnapshots    float64
	Folder                string
	hostname              string
}

type WebhookStats struct {
	Name          string     `json: "name"`
	BackupMetrics rawMetrics `json:"backup_metrics"`
	Snapshots     []Snapshot `json:"snapshots"`
}

type promMetrics struct {
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

func (p *promMetrics) toSlice() []prometheus.Collector {
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

func newBackup(backupDir string, listSnapshots *ListSnapshotsStruct) *BackupStruct {
	return &BackupStruct{
		backupDir:      backupDir,
		snapshotLister: listSnapshots,
		rawMetrics:     []rawMetrics{},
	}
}

// Backup executes a backup command.
func (b *BackupStruct) Backup() {

	if _, err := os.Stat(b.backupDir); os.IsNotExist(err) {
		fmt.Printf("Backupdir %v does not exist, skipping\n", b.backupDir)
		return
	}

	fmt.Println("backing up...")
	files, err := ioutil.ReadDir(b.backupDir)
	if err != nil {
		b.errorMessage = fmt.Errorf("Error with the backupdir: %v", err)
		return
	}
	// Build the folderlist first so every job that gets triggered later has the
	// complete list of folders to be backed up for the metrics/webhooks.
	b.folderList = make([]string, 0)
	for _, folder := range files {
		if folder.IsDir() && b.errorMessage == nil {
			b.folderList = append(b.folderList, folder.Name())
		}
	}

	for _, folder := range b.folderList {
		fmt.Printf("Starting backup for folder %v\n", folder)
		b.backupFolder(path.Join(b.backupDir, folder), folder)
		tmpMetrics := b.parse()
		tmpMetrics.Folder = folder
		tmpMetrics.hostname = os.Getenv(Hostname)
		b.rawMetrics = append(b.rawMetrics, tmpMetrics)
	}

	b.snapshots = b.snapshotLister.ListSnapshots(false)
}

func (b *BackupStruct) backupFolder(folder, folderName string) {
	args := []string{"backup", folder, "--host", os.Getenv(Hostname)}
	b.genericCommand.exec(args, commandOptions{print: true})
}

// StdinBackup triggers a backup that attaches itself to the given container
// on a Kubernetes cluster.
func (b *BackupStruct) StdinBackup(backupCommand, pod, container, namespace, fileExt string) {
	fmt.Printf("backing up via %v stdin...\n", container)
	host := os.Getenv(Hostname) + "-" + container
	args := []string{"backup", "--host", host, "--stdin", "--stdin-filename", "/" + host + fileExt}
	b.genericCommand.exec(args, commandOptions{
		print: true,
		Params: kubernetes.Params{
			Pod:           pod,
			Container:     container,
			Namespace:     namespace,
			BackupCommand: backupCommand,
		},
		stdin: true,
	})
	tmpMetrics := b.parse()
	tmpMetrics.Folder = host
	tmpMetrics.hostname = os.Getenv(Hostname)
	b.rawMetrics = append(b.rawMetrics, tmpMetrics)
	b.snapshots = b.snapshotLister.ListSnapshots(false)
}

// GetWebhookData a slice of objects that should be sent to the webhook endpoint.
func (b *BackupStruct) GetWebhookData() []output.JsonMarshaller {
	stats := make([]output.JsonMarshaller, 0)

	for _, stat := range b.rawMetrics {
		stats = append(stats, &WebhookStats{
			Name:          os.Getenv(Hostname),
			BackupMetrics: stat,
			Snapshots:     b.snapshots,
		})
	}

	return stats
}

// ToProm resturns a slice of prometheus collectors that should get sent to the
// prom push gateway.
func (b *BackupStruct) ToProm() []prometheus.Collector {
	metrics := b.newPromMetrics()
	promSlice := make([]prometheus.Collector, 0)

	for _, stat := range b.rawMetrics {
		metrics.AvailableSnapshots.Set(stat.availableSnapshots)
		metrics.ChangedDirs.WithLabelValues(stat.Folder, stat.hostname).Set(stat.ChangedDirs)
		metrics.ChangedFiles.WithLabelValues(stat.Folder, stat.hostname).Set(stat.ChangedFiles)
		metrics.Errors.WithLabelValues(stat.Folder, stat.hostname).Set(stat.Errors)
		metrics.NewDirs.WithLabelValues(stat.Folder, stat.hostname).Set(stat.NewDirs)
		metrics.NewFiles.WithLabelValues(stat.Folder, stat.hostname).Set(stat.NewFiles)
		metrics.UnmodifiedDirs.WithLabelValues(stat.Folder, stat.hostname).Set(stat.UnmodifiedDirs)
		metrics.UnmodifiedFiles.WithLabelValues(stat.Folder, stat.hostname).Set(stat.UnmodifiedFiles)
		promSlice = append(promSlice, metrics.toSlice()...)
	}
	return promSlice
}

func (b *BackupStruct) parse() rawMetrics {
	if len(b.stdOut) < 6 || b.parsed {
		return rawMetrics{}
	}
	files := strings.Fields(strings.Split(b.stdOut[len(b.stdOut)-6], ":")[1])
	dirs := strings.Fields(strings.Split(b.stdOut[len(b.stdOut)-5], ":")[1])

	var errorCount = len(b.stdErrOut)

	if errorCount > 0 {
		fmt.Println("These errors occurred during the backup of the folder:")
		for _, line := range b.stdErrOut {
			fmt.Println(line)
		}
	}

	newFiles, err := strconv.Atoi(files[0])
	changedFiles, err := strconv.Atoi(files[2])
	unmodifiedFiles, err := strconv.Atoi(files[4])

	newDirs, err := strconv.Atoi(dirs[0])
	changedDirs, err := strconv.Atoi(dirs[2])
	unmodifiedDirs, err := strconv.Atoi(dirs[4])

	if err != nil {
		errorMessage := fmt.Sprintln("There was a problem convertig the metrics: ", err)
		fmt.Println(errorMessage)
		return rawMetrics{}
	}

	if b.errorMessage != nil {
		errorCount++
	}

	fmt.Println("Get snapshots for backup metrics")
	return rawMetrics{
		NewDirs:            float64(newDirs),
		NewFiles:           float64(newFiles),
		ChangedFiles:       float64(changedFiles),
		UnmodifiedFiles:    float64(unmodifiedFiles),
		ChangedDirs:        float64(changedDirs),
		UnmodifiedDirs:     float64(unmodifiedDirs),
		Errors:             float64(errorCount),
		MountedPVCs:        b.folderList,
		availableSnapshots: float64(len(b.snapshotLister.ListSnapshots(false))),
	}
}

func (b *BackupStruct) newPromMetrics() *promMetrics {

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

	return &promMetrics{
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

// ToJson returns a byteslice which contains the json representation of the
// object.
func (w *WebhookStats) ToJson() []byte {
	jsonData, _ := json.Marshal(w)
	return jsonData
}
