package restic

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"git.vshn.net/vshn/wrestic/kubernetes"
	"git.vshn.net/vshn/wrestic/output"
	"github.com/prometheus/client_golang/prometheus"
)

// BackupStruct holds the state of a backup command.
type BackupStruct struct {
	genericCommand
	folderList        []string
	backupDir         string
	rawMetrics        []rawMetrics // used to derive prom and webhook information
	snapshotLister    *ListSnapshotsStruct
	parsed            bool
	startTimeStamp    int64
	endTimeStamp      int64
	snapshots         []Snapshot
	stdinErrorMessage error
	liveOutput        chan string
	webhookSender     WebhookSender
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
	ID                    string `json:"id"`
}

type WebhookStats struct {
	Name          string      `json:"name,omitempty"`
	BucketName    string      `json:"bucket_name,omitempty"`
	BackupMetrics *rawMetrics `json:"backup_metrics,omitempty"`
	Snapshots     []Snapshot  `json:"snapshots,omitempty"`
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

type backupEnvelope struct {
	MessageType string `json:"message_type,omitempty"`
	backupStatus
	backupSummary
	backupError
}

type backupStatus struct {
	PercentDone  float64  `json:"percent_done"`
	TotalFiles   int      `json:"total_files"`
	FilesDone    int      `json:"files_done"`
	TotalBytes   int      `json:"total_bytes"`
	BytesDone    int      `json:"bytes_done"`
	CurrentFiles []string `json:"current_files"`
	ErrorCount   int      `json:"error_count"`
}

type backupSummary struct {
	MessageType         string  `json:"message_type"`
	FilesNew            int     `json:"files_new"`
	FilesChanged        int     `json:"files_changed"`
	FilesUnmodified     int     `json:"files_unmodified"`
	DirsNew             int     `json:"dirs_new"`
	DirsChanged         int     `json:"dirs_changed"`
	DirsUnmodified      int     `json:"dirs_unmodified"`
	DataBlobs           int     `json:"data_blobs"`
	TreeBlobs           int     `json:"tree_blobs"`
	DataAdded           int64   `json:"data_added"`
	TotalFilesProcessed int     `json:"total_files_processed"`
	TotalBytesProcessed int     `json:"total_bytes_processed"`
	TotalDuration       float64 `json:"total_duration"`
	SnapshotID          string  `json:"snapshot_id"`
}

type backupError struct {
	Error struct {
		Op   string `json:"Op"`
		Path string `json:"Path"`
		Err  int    `json:"Err"`
	} `json:"error"`
	During string `json:"during"`
	Item   string `json:"item"`
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

func newBackup(backupDir string, listSnapshots *ListSnapshotsStruct, webhookSender WebhookSender) *BackupStruct {
	return &BackupStruct{
		backupDir:      backupDir,
		snapshotLister: listSnapshots,
		rawMetrics:     []rawMetrics{},
		liveOutput:     make(chan string, 0),
		webhookSender:  webhookSender,
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

	parsedSummary := make(chan rawMetrics, 0)

	for _, folder := range b.folderList {
		fmt.Printf("Starting backup for folder %v\n", folder)
		go b.parse(folder, parsedSummary) //needs to contain the whole metrics logic from now on.
		b.backupFolder(path.Join(b.backupDir, folder), folder)

		b.sendPostFolderWebhook(os.Getenv(Hostname), folder, parsedSummary)
	}

	close(parsedSummary)

	b.snapshots = b.snapshotLister.ListSnapshots(false)

}

func (b *BackupStruct) backupFolder(folder, folderName string) {
	args := []string{"backup", folder, "--host", os.Getenv(Hostname), "--json"}
	b.genericCommand.exec(args, commandOptions{print: false, output: b.liveOutput})
}

// StdinBackup triggers a backup that attaches itself to the given container
// on a Kubernetes cluster.
func (b *BackupStruct) StdinBackup(backupCommand, pod, container, namespace, fileExt string) {
	fmt.Printf("backing up via %v stdin...\n", container)
	host := os.Getenv(Hostname) + "-" + container
	args := []string{"backup", "--host", host, "--stdin", "--stdin-filename", "/" + host + fileExt, "--json"}
	parsedSummary := make(chan rawMetrics, 0)
	go b.parse(host, parsedSummary)
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

	// Because stdin backups don't have folders we'll use the hostname +
	// container name distingquish them
	b.sendPostFolderWebhook(host, host, parsedSummary)

	close(parsedSummary)

	b.snapshots = b.snapshotLister.ListSnapshots(false)
	if b.errorMessage != nil {
		b.stdinErrorMessage = b.errorMessage
		b.errorMessage = nil
	}

}

func (b *BackupStruct) sendPostFolderWebhook(host string, folder string, parsedSummary chan rawMetrics) {
	tmpMetrics := <-parsedSummary
	tmpMetrics.Folder = folder
	tmpMetrics.hostname = os.Getenv(Hostname)

	fmt.Printf("sending webhook ")
	b.webhookSender.TriggerHook(&WebhookStats{
		Name:          tmpMetrics.hostname,
		BucketName:    getBucket(),
		BackupMetrics: &tmpMetrics,
	})
}

// GetWebhookData a slice of objects that should be sent to the webhook endpoint.
func (b *BackupStruct) GetWebhookData() []output.JsonMarshaller {
	stats := make([]output.JsonMarshaller, 0)

	stats = append(stats, &WebhookStats{
		Name:       os.Getenv(Hostname),
		BucketName: getBucket(),
		Snapshots:  b.snapshotLister.ListSnapshots(false),
	})

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

func (b *BackupStruct) parse(folder string, parsedSummary chan rawMetrics) {

	i := 0
	errorCount := 0
	startTimestamp := time.Now().Unix()

	for message := range b.liveOutput {
		be := &backupEnvelope{}
		err := json.Unmarshal([]byte(message), be)

		if err != nil {
			fmt.Printf("could not parse restic output: %v\n", err)
		}

		switch be.MessageType {
		case "error":
			errorCount++
			b.parseError(be.backupError)
		case "status":
			// Restic does the json output with 60hz, which is a bit much...
			if i%60 == 0 {
				b.parseStatus(be.backupStatus)
			}
			i++
		case "summary":
			endTimeStamp := time.Now().Unix()
			b.parseSummary(be.backupSummary, errorCount, folder, startTimestamp, endTimeStamp)
			// send back the last item in the metrics slice
			parsedSummary <- b.rawMetrics[len(b.rawMetrics)-1]
		}
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

// GetError returns if there was an error either in the stdin or normal backup.
func (b *BackupStruct) GetError() error {
	var pvcErr error
	var stdinErr error
	var finalError error
	if b.errorMessage != nil {
		pvcErr = fmt.Errorf("pvc backup error: %v", b.errorMessage)
		finalError = pvcErr
	}

	if b.stdinErrorMessage != nil {
		stdinErr = fmt.Errorf("stdin backup error: %v", b.stdinErrorMessage)
		finalError = stdinErr
	}

	if b.stdinErrorMessage != nil && b.errorMessage != nil {
		finalError = fmt.Errorf("%v\n%v", pvcErr, stdinErr)
	}

	return finalError
}

func (b *BackupStruct) parseStatus(status backupStatus) {
	percent := status.PercentDone * 100
	fmt.Printf("done: %.2f%% \n", percent)
}

func (b *BackupStruct) parseSummary(summary backupSummary, errorCount int, folder string, startTimestamp, endTimestamp int64) {
	fmt.Printf("backup finished! new files: %v changed files: %v bytes added: %v\n", summary.FilesNew, summary.FilesChanged, summary.DataAdded)
	b.rawMetrics = append(b.rawMetrics, rawMetrics{
		NewDirs:               float64(summary.DirsNew),
		NewFiles:              float64(summary.FilesNew),
		ChangedFiles:          float64(summary.FilesChanged),
		UnmodifiedFiles:       float64(summary.FilesUnmodified),
		ChangedDirs:           float64(summary.DirsChanged),
		UnmodifiedDirs:        float64(summary.DirsUnmodified),
		Errors:                float64(errorCount),
		MountedPVCs:           b.folderList,
		availableSnapshots:    float64(len(b.snapshotLister.ListSnapshots(false))),
		Folder:                folder,
		hostname:              os.Getenv(Hostname),
		BackupStartTimestamp:  float64(startTimestamp),
		BackupEndTimestamp:    float64(endTimestamp),
		runningBackupDuration: summary.TotalDuration,
		DataTransferred:       float64(summary.DataAdded),
		ID:                    summary.SnapshotID,
	})
}

func (b *BackupStruct) parseError(fileErrs backupError) {
	fmt.Printf("error cannot %v on file %v\n", fileErrs.Error.Op, fileErrs.Item)
}
