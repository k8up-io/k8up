package backupcontroller

import (
	"context"
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
	return &BackupExecutor{Generic: executor.Generic{Config: config}, backup: config.Obj.(*k8upv1.Backup)}
}

// GetConcurrencyLimit returns the concurrent jobs limit
func (b *BackupExecutor) GetConcurrencyLimit() int {
	return cfg.Config.GlobalConcurrentBackupJobsLimit
}

// Execute triggers the actual batch.job creation on the cluster.
// It will also register a callback function on the observer so the PreBackupPods can be removed after the backup has finished.
func (b *BackupExecutor) Execute(ctx context.Context) error {

	status := b.backup.Status

	if status.HasFailed() || status.HasSucceeded() {
		b.cleanupOldBackups(ctx)
		b.StopPreBackupDeployments(ctx)
		return nil
	}

	if status.HasStarted() {
		return nil // nothing to do, wait until finished
	}

	err := b.createServiceAccountAndBinding(ctx)
	if err != nil {
		return err
	}

	return b.startBackup(ctx)
}

// listAndFilterPVCs lists all PVCs in the given namespace and filters them for K8up specific usage.
// Specifically, non-RWX PVCs will be skipped, as well PVCs that have the given annotation.
func (b *BackupExecutor) listAndFilterPVCs(ctx context.Context, annotation string) ([]corev1.Volume, error) {
	log := controllerruntime.LoggerFrom(ctx)
	volumes := make([]corev1.Volume, 0)
	claimlist := &corev1.PersistentVolumeClaimList{}

	log.Info("Listing all PVCs", "annotation", annotation)
	if err := b.fetchPVCs(ctx, claimlist); err != nil {
		return volumes, err
	}

	for _, item := range claimlist.Items {
		annotations := item.GetAnnotations()

		tmpAnnotation, ok := annotations[annotation]

		if !containsAccessMode(item.Spec.AccessModes, "ReadWriteMany") && !ok {
			log.Info("PVC isn't RWX", "pvc", item.GetName())
			continue
		}

		if !ok {
			log.Info("PVC doesn't have annotation, adding to list", "pvc", item.GetName())
		} else if anno, _ := strconv.ParseBool(tmpAnnotation); !anno {
			log.Info("PVC skipped due to annotation", "pvc", item.GetName(), "annotation", tmpAnnotation)
			continue
		} else {
			log.Info("Adding to list", "pvc", item.Name)
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

func (b *BackupExecutor) startBackup(ctx context.Context) error {

	ready, err := b.StartPreBackup(ctx)
	if err != nil {
		return err
	}
	if !ready || b.backup.Status.IsWaitingForPreBackup() {
		return nil
	}

	volumes, err := b.listAndFilterPVCs(ctx, cfg.Config.BackupAnnotation)
	if err != nil {
		b.Generic.SetConditionFalseWithMessage(ctx, k8upv1.ConditionReady, k8upv1.ReasonRetrievalFailed, err.Error())
		return err
	}

	batchJob := &batchv1.Job{}
	batchJob.Name = b.backup.GetJobName()
	batchJob.Namespace = b.backup.Namespace

	_, err = controllerruntime.CreateOrUpdate(ctx, b.Generic.Config.Client, batchJob, func() error {
		mutateErr := job.MutateBatchJob(batchJob, b.backup, b.Generic.Config)
		if mutateErr != nil {
			return mutateErr
		}

		vars, setupErr := b.setupEnvVars()
		if setupErr != nil {
			return setupErr
		}
		batchJob.Spec.Template.Spec.Containers[0].Env = vars
		b.backup.Spec.AppendEnvFromToContainer(&batchJob.Spec.Template.Spec.Containers[0])
		batchJob.Spec.Template.Spec.Volumes = volumes
		batchJob.Spec.Template.Spec.ServiceAccountName = cfg.Config.ServiceAccount
		batchJob.Spec.Template.Spec.Containers[0].VolumeMounts = b.newVolumeMounts(volumes)
		batchJob.Spec.Template.Spec.Containers[0].Args = executor.BuildTagArgs(b.backup.Spec.Tags)
		return nil
	})

	return err
}

func (b *BackupExecutor) cleanupOldBackups(ctx context.Context) {
	b.Generic.CleanupOldResources(ctx, &k8upv1.BackupList{}, b.backup.Namespace, b.backup)
}
