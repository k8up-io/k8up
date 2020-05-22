package restic

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/go-logr/logr"
	"github.com/vshn/wrestic/kubernetes"
)

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

// Backup backup to the repository. It will loop through all subfolders of
// backupdir and trigger a snapshot for each of them.
func (r *Restic) Backup(backupDir string, tags ArrayOpts) error {
	backuplogger := r.logger.WithName("backup")

	backuplogger.Info("starting backup")

	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		backuplogger.Info("backupdir does not exist, skipping", "dirname", backupDir)
		return nil
	}

	files, err := ioutil.ReadDir(backupDir)
	if err != nil {
		return fmt.Errorf("Error with the backupdir: %v", err)
	}

	// we need to ignore any non folder things in the directory
	for _, folder := range files {
		if folder.IsDir() {
			err := r.folderBackup(path.Join(backupDir, folder.Name()), backuplogger, tags)
			if err != nil {
				return err
			}
		}
	}

	backuplogger.Info("backup finished, sending snapshot list")
	r.sendPostWebhook()

	return nil

}

func (r *Restic) folderBackup(folder string, backuplogger logr.Logger, tags ArrayOpts) error {

	outputWriter := r.newParseBackupOutput(backuplogger, folder)

	backuplogger.Info("starting backup for folder", "foldername", path.Base(folder))

	opts := CommandOptions{
		Path: r.resticPath,
		Args: []string{
			"backup",
			folder,
			"--host",
			os.Getenv(Hostname),
			"--json",
		},
		StdOut: outputWriter,
		StdErr: outputWriter,
	}

	return r.triggerBackup(folder, backuplogger, tags, opts, nil)
}

func (r *Restic) newParseBackupOutput(log logr.Logger, folder string) io.Writer {

	progressLogger := log.WithName("progress")

	return &outputWrapper{
		parser: &backupOutputParser{
			folder:      folder,
			log:         progressLogger,
			summaryfunc: r.sendBackupStats,
		},
	}

}

func (r *Restic) sendBackupStats(summary backupSummary, errorCount int, folder string, startTimestamp, endTimestamp int64) {

	metrics := r.parseSummary(summary, errorCount, folder, 1, time.Now().Unix())

	currentStats := &BackupStats{
		BackupMetrics: metrics,
		Name:          os.Getenv(Hostname),
		BucketName:    r.bucket,
	}

	err := r.statsHandler.SendWebhook(currentStats)
	if err != nil {
		r.logger.Error(err, "webhook send failed")
	}
	err = r.statsHandler.SendPrometheus(currentStats)
	if err != nil {
		r.logger.Error(err, "prometheus send failed")
	}

}

func (r *Restic) sendPostWebhook() {
	err := r.Snapshots(nil)
	if err != nil {
		r.logger.Error(err, "cannot fetch current snapshot list for webhook")
	}
	stats := &BackupStats{
		BucketName: r.bucket,
		Name:       os.Getenv(Hostname),
		Snapshots:  r.snapshots,
	}

	err = r.statsHandler.SendWebhook(stats)
	if err != nil {
		r.logger.Error(err, "webhook send failed")
	}

}

func (r *Restic) triggerBackup(name string, logger logr.Logger, tags ArrayOpts, opts CommandOptions, data *kubernetes.ExecData) error {
	if len(tags) > 0 {
		opts.Args = append(opts.Args, tags.BuildArgs()...)
	}

	cmd := NewCommand(r.ctx, logger, opts)
	cmd.Configure()

	cmd.Start()

	// All std* io has to be finished before calling Wait() as it will block
	// otherwise.
	if data != nil {
		// wait for data to finish writing, before waiting for the command
		<-data.Done
	}

	cmd.Wait()

	return cmd.FatalError
}
