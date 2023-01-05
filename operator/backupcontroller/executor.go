package backupcontroller

import (
	"context"
	"fmt"
	"strconv"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/executor"
	"github.com/k8up-io/k8up/v2/operator/job"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

// BackupExecutor creates a batch.job object on the cluster. It merges all the
// information provided by defaults and the CRDs to ensure the backup has all information to run.
type BackupExecutor struct {
	executor.Generic
	backup *k8upv1.Backup
}

// NewBackupExecutor returns a new BackupExecutor.
func NewBackupExecutor(config job.Config) *BackupExecutor {
	return &BackupExecutor{
		Generic: executor.Generic{Config: config},
	}
}

// GetConcurrencyLimit returns the concurrent jobs limit
func (b *BackupExecutor) GetConcurrencyLimit() int {
	return cfg.Config.GlobalConcurrentBackupJobsLimit
}

// Execute triggers the actual batch.job creation on the cluster.
// It will also register a callback function on the observer so the PreBackupPods can be removed after the backup has finished.
func (b *BackupExecutor) Execute() error {
	backupObject, ok := b.Obj.(*k8upv1.Backup)
	if !ok {
		return fmt.Errorf("object is not a backup: %v", b.Obj)
	}
	b.backup = backupObject
	status := backupObject.Status

	if status.HasFailed() || status.HasSucceeded() {
		b.StopPreBackupDeployments()
		b.cleanupOldBackups(b.CTX)
		return nil
	}

	if status.HasStarted() {
		return nil // nothing to do, wait until finished
	}

	err := b.createServiceAccountAndBinding()
	if err != nil {
		return err
	}

	return b.startBackup()
}

// listAndFilterPVCs lists all PVCs in the given namespace and filters them for K8up specific usage.
// Specifically, non-RWX PVCs will be skipped, as well PVCs that have the given annotation.
func (b *BackupExecutor) listAndFilterPVCs(annotation string) ([]corev1.Volume, error) {
	volumes := make([]corev1.Volume, 0)
	claimlist := &corev1.PersistentVolumeClaimList{}

	b.Log.Info("Listing all PVCs", "annotation", annotation)
	if err := b.fetchPVCs(claimlist); err != nil {
		return volumes, err
	}

	for _, item := range claimlist.Items {
		annotations := item.GetAnnotations()

		tmpAnnotation, ok := annotations[annotation]

		if !containsAccessMode(item.Spec.AccessModes, "ReadWriteMany") && !ok {
			b.Log.Info("PVC isn't RWX", "pvc", item.GetName())
			continue
		}

		if !ok {
			b.Log.Info("PVC doesn't have annotation, adding to list", "pvc", item.GetName())
		} else if anno, _ := strconv.ParseBool(tmpAnnotation); !anno {
			b.Log.Info("PVC skipped due to annotation", "pvc", item.GetName(), "annotation", tmpAnnotation)
			continue
		} else {
			b.Log.Info("Adding to list", "pvc", item.Name)
		}

		tmpVol := corev1.Volume{
			Name: item.Name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: item.Name,
				},
			},
		}

		volumes = append(volumes, tmpVol)
	}

	return volumes, nil
}

func (b *BackupExecutor) startBackup() error {

	ready, err := b.StartPreBackup()
	if err != nil {
		return err
	}
	if !ready || b.backup.Status.IsWaitingForPreBackup() {
		return nil
	}

	volumes, err := b.listAndFilterPVCs(cfg.Config.BackupAnnotation)
	if err != nil {
		b.SetConditionFalseWithMessage(b.CTX, k8upv1.ConditionReady, k8upv1.ReasonRetrievalFailed, err.Error())
		return err
	}

	batchJob := &batchv1.Job{}
	batchJob.Name = b.backup.GetJobName()
	batchJob.Namespace = b.backup.Namespace

	_, err = controllerruntime.CreateOrUpdate(b.CTX, b.Client, batchJob, func() error {
		mutateErr := job.MutateBatchJob(batchJob, b.backup, b.Config)
		if mutateErr != nil {
			return mutateErr
		}

		batchJob.Spec.Template.Spec.Containers[0].Env = b.setupEnvVars()
		b.backup.Spec.AppendEnvFromToContainer(&batchJob.Spec.Template.Spec.Containers[0])
		batchJob.Spec.Template.Spec.Volumes = volumes
		batchJob.Spec.Template.Spec.ServiceAccountName = cfg.Config.ServiceAccount
		batchJob.Spec.Template.Spec.Containers[0].VolumeMounts = b.newVolumeMounts(volumes)
		batchJob.Spec.Template.Spec.Containers[0].Args = executor.BuildTagArgs(b.backup.Spec.Tags)
		return nil
	})

	return err
}

func (e *BackupExecutor) cleanupOldBackups(ctx context.Context) {
	e.CleanupOldResources(ctx, &k8upv1.BackupList{}, e.backup.Namespace, e.backup)
}
