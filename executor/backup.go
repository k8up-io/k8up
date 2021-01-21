package executor

import (
	stderrors "errors"
	"path"
	"strconv"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/cfg"
	"github.com/vshn/k8up/job"
	"github.com/vshn/k8up/observer"
	"github.com/vshn/k8up/prebackup"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BackupExecutor creates a batch.job object on the cluster. It merges all the
// information provided by defaults and the CRDs to ensure the backup has all information to run.
type BackupExecutor struct {
	generic
	backup *k8upv1alpha1.Backup
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

// Execute triggers the actual batch.job creation on the cluster. It will also register
// a callback function on the observer so the prebackup pods can be removed after the backup
// has finished.
func (b *BackupExecutor) Execute() error {
	backupObject, ok := b.Obj.(*k8upv1alpha1.Backup)
	if !ok {
		return stderrors.New("object is not a backup")
	}

	if backupObject.Spec.Backend != nil {
		b.backup = backupObject
	}

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

	go func() {
		b.startBackup(genericJob)
	}()

	return nil
}

func (b *BackupExecutor) listPVCs(annotation string) []corev1.Volume {
	b.Log.Info("Listing all PVCs", "annotation", annotation, "namespace", b.Obj.GetMetaObject().GetNamespace())
	volumes := make([]corev1.Volume, 0)

	claimlist := &corev1.PersistentVolumeClaimList{}

	err := b.Client.List(b.CTX, claimlist, &client.ListOptions{
		Namespace: b.Obj.GetMetaObject().GetNamespace(),
	})
	if err != nil {
		return nil
	}

	for _, item := range claimlist.Items {
		annotations := item.GetAnnotations()

		tmpAnnotation, ok := annotations[annotation]

		if !b.containsAccessMode(item.Spec.AccessModes, "ReadWriteMany") && !ok {
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

	return volumes
}

func (b *BackupExecutor) getVolumeMounts(claims []corev1.Volume) []corev1.VolumeMount {
	mounts := make([]corev1.VolumeMount, len(claims))
	for i, volume := range claims {
		mounts[i] = corev1.VolumeMount{
			Name:      volume.Name,
			MountPath: path.Join(cfg.Config.MountPath, volume.Name),
			ReadOnly:  true,
		}
	}
	return mounts
}

func (b *BackupExecutor) containsAccessMode(s []corev1.PersistentVolumeAccessMode, e string) bool {
	for _, a := range s {
		if string(a) == e {
			return true
		}
	}
	return false
}

func (b *BackupExecutor) startBackup(backupJob *batchv1.Job) {
	preBackup := prebackup.NewPrebackup(b.Config)
	err := preBackup.Start()
	if err != nil {
		b.Config.Log.Error(err, "error while handling pre backup pods")
		return
	}

	b.registerBackupCallback(preBackup)
	b.RegisterJobSucceededConditionCallback()

	volumes := b.listPVCs(cfg.Config.BackupAnnotation)

	backupJob.Spec.Template.Spec.Containers[0].Env = b.setupEnvVars()
	backupJob.Spec.Template.Spec.Volumes = volumes
	backupJob.Spec.Template.Spec.ServiceAccountName = cfg.Config.ServiceAccount
	backupJob.Spec.Template.Spec.Containers[0].VolumeMounts = b.getVolumeMounts(volumes)

	err = b.CreateObjectIfNotExisting(backupJob)
	if err != nil {
		return
	}

	b.SetStarted("the job '%v/%v' was created", backupJob.Namespace, backupJob.Name)
}

func (b *BackupExecutor) registerBackupCallback(preBackup *prebackup.PreBackup) {
	name := b.GetJobNamespacedName()
	observer.GetObserver().RegisterCallback(name.String(), func(_ observer.ObservableJob) {
		preBackup.Stop()
		b.cleanupOldBackups(name)
	})
}

func (b *BackupExecutor) cleanupOldBackups(name types.NamespacedName) {
	list := &k8upv1alpha1.BackupList{}
	err := b.listOldResources(name.Namespace, list)
	if err != nil {
		return
	}

	jobs := make(jobObjectList, 0)
	for _, backup := range list.Items {
		// Avoid exportloopref
		backup := backup
		jobs = append(jobs, &backup)
	}
	var keepJobs *int = nil
	if b.backup != nil {
		keepJobs = b.backup.Spec.KeepJobs
	}
	cleanOldObjects(jobs, getKeepJobs(keepJobs), b.Config)
}

func (b *BackupExecutor) createServiceAccountAndBinding() error {
	role, sa, binding := newServiceAccountDefinition(b.Obj.GetMetaObject().GetNamespace())
	for _, obj := range []client.Object{&role, &sa, &binding} {
		if err := b.CreateObjectIfNotExisting(obj); err != nil {
			return err
		}
	}
	return nil
}

func newServiceAccountDefinition(namespace string) (rbacv1.Role, corev1.ServiceAccount, rbacv1.RoleBinding) {
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

func (b *BackupExecutor) setupEnvVars() []corev1.EnvVar {
	vars := NewEnvVarConverter()

	if b.backup != nil {
		if b.backup.Spec.Backend != nil {
			for key, value := range b.backup.Spec.Backend.GetCredentialEnv() {
				vars.SetEnvVarSource(key, value)
			}
			vars.SetString(cfg.ResticRepositoryEnvName, b.backup.Spec.Backend.String())
		}
	}

	vars.SetString("STATS_URL", cfg.Config.GlobalStatsURL)
	vars.SetString("PROM_URL", cfg.Config.PromURL)
	vars.SetString("BACKUPCOMMAND_ANNOTATION", cfg.Config.BackupCommandAnnotation)
	vars.SetString("FILEEXTENSION_ANNOTATION", cfg.Config.FileExtensionAnnotation)

	err := vars.Merge(DefaultEnv(b.Obj.GetMetaObject().GetNamespace()))
	if err != nil {
		b.Log.Error(err, "error while merging the environment variables", "name", b.Obj.GetMetaObject().GetName(), "namespace", b.Obj.GetMetaObject().GetNamespace())
	}

	return vars.Convert()
}
