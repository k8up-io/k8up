package executor

import (
	stderrors "errors"
	"strconv"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	k8upv1 "github.com/k8up-io/k8up/api/v1"
	"github.com/k8up-io/k8up/operator/cfg"
	"github.com/k8up-io/k8up/operator/job"
)

// BackupExecutor creates a batch.job object on the cluster. It merges all the
// information provided by defaults and the CRDs to ensure the backup has all information to run.
type BackupExecutor struct {
	generic
	backup *k8upv1.Backup
}

// NewBackupExecutor returns a new BackupExecutor.
func NewBackupExecutor(config job.Config) *BackupExecutor {
	return &BackupExecutor{
		generic: generic{config},
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
		return stderrors.New("object is not a backup")
	}
	b.backup = backupObject

	if b.Obj.GetStatus().Started {
		return nil
	}

	err := b.createServiceAccountAndBinding()
	if err != nil {
		return err
	}

	genericJob, err := job.GenerateGenericJob(b.Obj, b.Config)
	if err != nil {
		return err
	}

	return b.startBackup(genericJob)
}

// listAndFilterPVCs lists all PVCs in the given namespace and filters them for K8up specific usage.
// Specifically, non-RWX PVCs will be skipped, as well PVCs that have the given annotation.
func (b *BackupExecutor) listAndFilterPVCs(annotation string) ([]corev1.Volume, error) {
	volumes := make([]corev1.Volume, 0)
	claimlist := &corev1.PersistentVolumeClaimList{}

	b.Log.Info("Listing all PVCs", "annotation", annotation, "namespace", b.Obj.GetMetaObject().GetNamespace())
	if err := b.fetchPVCs(claimlist); err != nil {
		return volumes, err
	}

	for _, item := range claimlist.Items {
		annotations := item.GetAnnotations()

		tmpAnnotation, ok := annotations[annotation]

		if !containsAccessMode(item.Spec.AccessModes, "ReadWriteMany") && !ok {
			b.Log.Info("PVC isn't RWX", "namespace", item.GetNamespace(), "name", item.GetName())
			continue
		}

		if !ok {
			b.Log.Info("PVC doesn't have annotation, adding to list", "namespace", item.GetNamespace(), "name", item.GetName())
		} else if anno, _ := strconv.ParseBool(tmpAnnotation); !anno {
			b.Log.Info("PVC skipped due to annotation", "namespace", item.GetNamespace(), "name", item.GetName(), "annotation", tmpAnnotation)
			continue
		} else {
			b.Log.Info("Adding to list", "namespace", item.GetNamespace(), "name", item.Name)
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

func (b *BackupExecutor) startBackup(backupJob *batchv1.Job) error {

	ready, err := b.StartPreBackup()
	if err != nil {
		return err
	}
	if !ready || b.backup.Status.IsWaitingForPreBackup() {
		return nil
	}

	b.registerBackupCallback()
	b.RegisterJobSucceededConditionCallback()

	volumes, err := b.listAndFilterPVCs(cfg.Config.BackupAnnotation)
	if err != nil {
		b.SetConditionFalseWithMessage(k8upv1.ConditionReady, k8upv1.ReasonRetrievalFailed, err.Error())
		return err
	}

	backupJob.Spec.Template.Spec.Containers[0].Env = b.setupEnvVars()
	backupJob.Spec.Template.Spec.Volumes = volumes
	backupJob.Spec.Template.Spec.ServiceAccountName = cfg.Config.ServiceAccount
	backupJob.Spec.Template.Spec.Containers[0].VolumeMounts = b.newVolumeMounts(volumes)

	if err = b.CreateObjectIfNotExisting(backupJob); err == nil {
		b.SetStarted("the job '%v/%v' was created", backupJob.Namespace, backupJob.Name)
	}
	return err

}

func (b *BackupExecutor) cleanupOldBackups(name types.NamespacedName) {
	b.cleanupOldResources(&k8upv1.BackupList{}, name, b.backup)
}
