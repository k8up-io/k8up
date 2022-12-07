package executor

import (
	"context"
	stderrors "errors"
	"fmt"
	"strconv"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	citav1 "github.com/k8up-io/k8up/v2/api/v1cita"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
	"github.com/k8up-io/k8up/v2/operator/observer"
)

// CITABackupExecutor creates a batch.job object on the cluster. It merges all the
// information provided by defaults and the CRDs to ensure the backup has all information to run.
type CITABackupExecutor struct {
	generic
	backup *citav1.Backup
}

// NewCITABackupExecutor returns a new BackupExecutor.
func NewCITABackupExecutor(config job.Config) *CITABackupExecutor {
	return &CITABackupExecutor{
		generic: generic{config},
	}
}

// GetConcurrencyLimit returns the concurrent jobs limit
func (b *CITABackupExecutor) GetConcurrencyLimit() int {
	return cfg.Config.GlobalConcurrentBackupJobsLimit
}

// Execute triggers the actual batch.job creation on the cluster.
// It will also register a callback function on the observer so the PreBackupPods can be removed after the backup has finished.
func (b *CITABackupExecutor) Execute() error {
	backupObject, ok := b.Obj.(*citav1.Backup)
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

func (b *CITABackupExecutor) startBackup(backupJob *batchv1.Job) error {
	node := NewCITANode(b.CTX, b.Client, b.backup.Namespace, b.backup.Spec.Node)
	stopped, err := node.Stop()
	if err != nil {
		return err
	}
	if !stopped {
		return nil
	}

	//b.registerBackupCallback()
	b.registerCITANodeCallback()
	b.RegisterJobSucceededConditionCallback()

	volumes, err := b.prepareVolumes()
	if err != nil {
		b.SetConditionFalseWithMessage(k8upv1.ConditionReady, k8upv1.ReasonRetrievalFailed, err.Error())
		return err
	}

	backupJob.Spec.Template.Spec.Containers[0].Env = b.setupEnvVars()
	b.backup.Spec.AppendEnvFromToContainer(&backupJob.Spec.Template.Spec.Containers[0])
	backupJob.Spec.Template.Spec.Volumes = volumes
	backupJob.Spec.Template.Spec.ServiceAccountName = cfg.Config.ServiceAccount
	if b.backup.Spec.DataType.Full != nil {
		backupJob.Spec.Template.Spec.Containers[0].VolumeMounts = b.newVolumeMountsForFull()
	}
	if b.backup.Spec.DataType.State != nil {
		backupJob.Spec.Template.Spec.Containers[0].VolumeMounts = b.newVolumeMountsForState()
	}
	if b.backup.Spec.Backend.Local != nil {
		// mount new pvc
		backupJob.Spec.Template.Spec.Containers[0].VolumeMounts = append(backupJob.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "backup-dest",
			ReadOnly:  false,
			MountPath: b.backup.Spec.Backend.Local.MountPath,
		})
	}

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

func (b *CITABackupExecutor) registerCITANodeCallback() {
	name := b.GetJobNamespacedName()
	observer.GetObserver().RegisterCallback(name.String(), func(_ observer.ObservableJob) {
		//b.StopPreBackupDeployments()
		//b.cleanupOldBackups(name)
		b.startCITANode(b.CTX, b.Client, b.backup.Namespace, b.backup.Spec.Node)
	})
}

func (b *CITABackupExecutor) startCITANode(ctx context.Context, client client.Client, namespace, name string) {
	NewCITANode(ctx, client, namespace, name).Start()
}

func (b *CITABackupExecutor) prepareVolumes() ([]corev1.Volume, error) {
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
		pvcInfo, err := NewCITANode(b.CTX, b.Client, b.backup.Namespace, b.backup.Spec.Node).GetPVCInfo()
		if err != nil {
			return nil, err
		}
		destPVC := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      b.backup.Name,
				Namespace: b.backup.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources:        pvcInfo,
				StorageClassName: pointer.String(b.backup.Spec.Backend.Local.StorageClass),
			},
		}
		err = ctrl.SetControllerReference(b.backup, destPVC, b.Scheme)
		if err != nil {
			return nil, err
		}
		err = b.CreateObjectIfNotExisting(destPVC)
		if err != nil {
			return nil, err
		}
		// add to volumes
		volumes = append(volumes, corev1.Volume{
			Name: "backup-dest",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: destPVC.Name,
					ReadOnly:  false,
				},
			}})
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

func (b *CITABackupExecutor) newVolumeMountsForFull() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "backup-source",
			MountPath: "/data/backup-source",
			ReadOnly:  true,
		},
	}
}

func (b *CITABackupExecutor) newVolumeMountsForState() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "backup-source",
			MountPath: "/data/backup-source",
			ReadOnly:  false,
		},
		{
			Name:      "cita-config",
			MountPath: "/cita-config",
			ReadOnly:  true,
		},
	}
}

func (b *CITABackupExecutor) args() ([]string, error) {
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

func (b *CITABackupExecutor) createServiceAccountAndBinding() error {
	role, sa, binding := newCITAServiceAccountDefinition(b.backup.Namespace)
	for _, obj := range []client.Object{&role, &sa, &binding} {
		if err := b.CreateObjectIfNotExisting(obj); err != nil {
			return err
		}
	}
	return nil
}

func newCITAServiceAccountDefinition(namespace string) (rbacv1.Role, corev1.ServiceAccount, rbacv1.RoleBinding) {
	role := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.Config.PodExecRoleName,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"pods",
					"pods/exec",
				},
				Verbs: []string{
					"*",
				},
			},
			{
				Verbs: []string{
					"get",
					"list",
					"watch",
					"update",
				},
				APIGroups: []string{
					"apps",
				},
				Resources: []string{
					"statefulsets",
				},
			},
		},
	}

	roleBinding := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.Config.PodExecRoleName + "-namespaced",
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: namespace,
				Name:      cfg.Config.ServiceAccount,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     cfg.Config.ServiceAccount,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	sa := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.Config.ServiceAccount,
			Namespace: namespace,
		},
	}

	return role, sa, roleBinding
}

func (b *CITABackupExecutor) setupEnvVars() []corev1.EnvVar {
	vars := NewEnvVarConverter()

	if b.backup != nil {
		if b.backup.Spec.Backend != nil {
			for key, value := range b.backup.Spec.Backend.GetCredentialEnv() {
				vars.SetEnvVarSource(key, value)
			}
			vars.SetString(cfg.ResticRepositoryEnvName, b.backup.Spec.Backend.String())
		}
	}

	vars.SetStringOrDefault("STATS_URL", b.backup.Spec.StatsURL, cfg.Config.GlobalStatsURL)
	vars.SetStringOrDefault("PROM_URL", b.backup.Spec.PromURL, cfg.Config.PromURL)
	vars.SetString("BACKUPCOMMAND_ANNOTATION", cfg.Config.BackupCommandAnnotation)
	vars.SetString("FILEEXTENSION_ANNOTATION", cfg.Config.FileExtensionAnnotation)

	err := vars.Merge(DefaultEnv(b.Obj.GetMetaObject().GetNamespace()))
	if err != nil {
		b.Log.Error(err, "error while merging the environment variables", "name", b.Obj.GetMetaObject().GetName(), "namespace", b.Obj.GetMetaObject().GetNamespace())
	}

	return vars.Convert()
}
