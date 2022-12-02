package executor

import (
	"context"
	stderrors "errors"
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
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
		b.RegisterJobSucceededConditionCallback() // ensure that completed jobs can complete backups between operator restarts.
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

func (b *BackupExecutor) prepareVolumes() ([]corev1.Volume, error) {
	volumes := []corev1.Volume{
		{
			Name: "backup-source",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: fmt.Sprintf("datadir-%s-0", b.backup.Spec.Node),
					ReadOnly:  false,
				},
			},
		},
	}
	if b.backup.Spec.Backend.Local != nil {
		// create same pvc as node's pvc
		// add to volumes
	}
	if b.backup.Spec.DataType.State != nil {
		volumes = append(volumes, corev1.Volume{
			Name: "cita-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fmt.Sprintf("%s-config", b.backup.Spec.Node),
					},
				},
			},
		})
	}
	return volumes, nil
}

func (b *BackupExecutor) startBackup(backupJob *batchv1.Job) error {
	node := NewCITANode(b.CTX, b.Client, b.backup.Namespace, b.backup.Spec.Node)
	stopped, err := node.Stop()
	if err != nil {
		return err
	}
	if !stopped {
		return nil
	}
	//if err != nil {
	//	return err
	//}
	//if !ready || b.backup.Status.IsWaitingForPreBackup() {
	//	return nil
	//}

	//b.registerBackupCallback()
	b.registerCITANodeCallback()
	b.RegisterJobSucceededConditionCallback()

	volumes, err := b.prepareVolumes()
	//volumes, err := b.listAndFilterPVCs(cfg.Config.BackupAnnotation)
	if err != nil {
		b.SetConditionFalseWithMessage(k8upv1.ConditionReady, k8upv1.ReasonRetrievalFailed, err.Error())
		return err
	}

	backupJob.Spec.Template.Spec.Containers[0].Env = b.setupEnvVars()
	b.backup.Spec.AppendEnvFromToContainer(&backupJob.Spec.Template.Spec.Containers[0])
	backupJob.Spec.Template.Spec.Volumes = volumes
	backupJob.Spec.Template.Spec.ServiceAccountName = cfg.Config.ServiceAccount
	if b.backup.Spec.DataType.Full != nil {
		backupJob.Spec.Template.Spec.Containers[0].VolumeMounts = b.newVolumeMounts(volumes)
	}
	if b.backup.Spec.DataType.State != nil {
		backupJob.Spec.Template.Spec.Containers[0].VolumeMounts = b.newVolumeMountsForState()
	}
	if b.backup.Spec.Backend.Local != nil {
		backupJob.Spec.Template.Spec.Containers[0].VolumeMounts = append(backupJob.Spec.Template.Spec.Containers[0].VolumeMounts)
	}

	//backupJob.Spec.Template.Spec.Containers[0].Args = BuildTagArgs(b.backup.Spec.Tags)
	args, err := b.args()
	if err != nil {
		return err
	}
	backupJob.Spec.Template.Spec.Containers[0].Args = args

	if err = b.CreateObjectIfNotExisting(backupJob); err == nil {
		b.SetStarted("the job '%v/%v' was created", backupJob.Namespace, backupJob.Name)
	}
	return err

}

func (b *BackupExecutor) args() ([]string, error) {
	var args []string
	if len(b.backup.Spec.Tags) > 0 {
		args = append(args, BuildTagArgs(b.backup.Spec.Tags)...)
	}
	crypto, consensus, err := b.GetCryptoAndConsensus(b.backup.Namespace, b.backup.Spec.Node)
	if err != nil {
		return nil, err
	}
	switch {
	case b.backup.Spec.DataType.Full != nil:
		args = append(args, "-dataType", "full")
		args = append(args, BuildIncludePathArgs(b.backup.Spec.DataType.Full.IncludePaths)...)
	case b.backup.Spec.DataType.State != nil:
		args = append(args, "-dataType", "state")
		args = append(args, "-blockHeight", strconv.FormatInt(b.backup.Spec.DataType.State.BlockHeight, 10))
		// todo:
		args = append(args, "-crypto", crypto)
		args = append(args, "-consensus", consensus)
		args = append(args, "-backupDir", "/state_data")
	default:
		return nil, fmt.Errorf("undefined backup data type on '%v/%v'", b.backup.Namespace, b.backup.Name)
	}
	return args, nil
}

func (b *BackupExecutor) cleanupOldBackups(name types.NamespacedName) {
	b.cleanupOldResources(&k8upv1.BackupList{}, name, b.backup)
}

func (b *BackupExecutor) startCITANode(ctx context.Context, client client.Client, namespace, name string) {
	NewCITANode(ctx, client, namespace, name).Start()
}
