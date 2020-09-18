package handler

import (
	"fmt"
	"time"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/executor"
	"github.com/vshn/k8up/job"
	"github.com/vshn/k8up/queue"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

type BackupHandler struct {
	job.Config
	backup *k8upv1alpha1.Backup
}

func NewBackupHandler(config job.Config, backup *k8upv1alpha1.Backup) *BackupHandler {
	return &BackupHandler{
		Config: config,
		backup: backup,
	}
}

func (b *BackupHandler) handle() error {

	backupJob := &batchv1.Job{}
	err := b.Client.Get(b.CTX, types.NamespacedName{Name: b.backup.Status.BackupJobName, Namespace: b.backup.Namespace}, backupJob)
	if err != nil && errors.IsNotFound(err) {
		return b.queueJob(backupJob)
	} else if err != nil {
		b.Log.Error(err, "Failed to get Job")
		return err
	}

	if b.backup.Status.Started {
		b.checkJob(backupJob)
	}

	return nil
}

func (b *BackupHandler) queueJob(backupJob *batchv1.Job) error {
	b.Log.Info("Queue up backup job")

	jobName := fmt.Sprintf("backupjob-%d", time.Now().Unix())

	queue.Queue.Add(&queue.QueuedJob{
		Job: executor.NewBackupExecutor(b.Config, jobName),
	})

	b.backup.Status.Started = true
	b.backup.Status.BackupJobName = jobName

	err := b.Client.Status().Update(b.CTX, b.backup)
	if err != nil {
		b.Log.Error(err, "Status cannot be updated")
	}
	return nil
}

func (b *BackupHandler) checkJob(job *batchv1.Job) {
	if job.Status.Active > 0 {
		b.Log.Info("job is running")
	}

	if job.Status.Succeeded > 0 {
		b.Log.Info("job succeeded")
	}

	if job.Status.Failed > 0 {
		b.Log.Info("job failed")
	}

}
