package cli

import (
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/go-logr/logr"

	"github.com/k8up-io/k8up/v2/restic/cfg"
	"github.com/k8up-io/k8up/v2/restic/kubernetes"
	"github.com/k8up-io/k8up/v2/restic/logging"
)

// Backup backup to the repository. It will loop through all subfolders of
// backupdir and trigger a snapshot for each of them.
func (r *Restic) Backup(backupDir string, tags ArrayOpts) error {
	backuplogger := r.logger.WithName("backup")

	backuplogger.Info("starting backup")

	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		backuplogger.Info("backupdir does not exist, skipping. Sending snapshot list", "dirname", backupDir)
		r.sendSnapshotList()
		return nil
	}

	files, err := os.ReadDir(backupDir)
	if err != nil {
		return fmt.Errorf("can't read backupdir '%s': %w", backupDir, err)
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
	r.sendSnapshotList()

	return nil
}

func (r *Restic) folderBackup(folder string, backuplogger logr.Logger, tags ArrayOpts) error {

	outputWriter := r.newParseBackupOutput(backuplogger, folder)

	backuplogger.Info("starting backup for folder", "foldername", path.Base(folder))

	flags := Combine(r.globalFlags, Flags{
		"--host": {cfg.Config.Hostname},
		"--json": {},
	})

	opts := CommandOptions{
		Path:   r.resticPath,
		Args:   flags.ApplyToCommand("backup", folder),
		StdOut: outputWriter,
		StdErr: outputWriter,
	}

	return r.triggerBackup(backuplogger, tags, opts, nil)
}

func (r *Restic) newParseBackupOutput(log logr.Logger, folder string) io.Writer {

	progressLogger := log.WithName("progress")

	return logging.NewBackupOutputParser(progressLogger, folder, r.sendBackupStats)
}

func (r *Restic) sendBackupStats(summary logging.BackupSummary, errorCount int, folder string, _, _ int64) {

	metrics := r.parseSummary(summary, errorCount, folder, 1, time.Now().Unix())

	currentStats := &BackupStats{
		BackupMetrics: metrics,
		Name:          cfg.Config.Hostname,
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

// sendSnapshotList sends the current list of snapshots to a webhook and the k8s cluster
func (r *Restic) sendSnapshotList() {
	err := r.Snapshots(nil)
	if err != nil {
		r.logger.Error(err, "cannot fetch current snapshot list for webhook")
	}
	stats := &BackupStats{
		BucketName: r.bucket,
		Name:       cfg.Config.Hostname,
		Snapshots:  r.snapshots,
	}

	err = r.statsHandler.SendWebhook(stats)
	if err != nil {
		r.logger.Error(err, "webhook send failed")
	}

	err = kubernetes.SyncSnapshotList(r.ctx, r.snapshots, cfg.Config.Hostname, cfg.Config.ResticRepository)
	if err != nil {
		r.logger.Error(err, "cannot sync snapshots to the cluster")
	}
}

func (r *Restic) triggerBackup(logger logr.Logger, tags ArrayOpts, opts CommandOptions, data *kubernetes.ExecData) error {
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
