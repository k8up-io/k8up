package executor

import (
	"path"
	"strconv"

	"github.com/vshn/k8up/constants"
	"github.com/vshn/k8up/job"
	"github.com/vshn/k8up/observer"
	"github.com/vshn/k8up/prebackup"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type BackupExecutor struct {
	generic
}

type serviceAccount struct {
	role        *rbacv1.Role
	roleBinding *rbacv1.RoleBinding
	account     *corev1.ServiceAccount
}

func NewBackupExecutor(config job.Config) *BackupExecutor {
	return &BackupExecutor{
		generic: generic{config},
	}
}

func (b *BackupExecutor) Execute() error {

	if b.Obj.GetK8upStatus().Started {
		return nil
	}

	err := b.createServiceAccountAndBinding()
	if err != nil {
		return err
	}

	job, err := job.GetGenericJob(b.Obj, b.Config)
	if err != nil {
		return err
	}

	go func() {
		b.startBackup(job)
	}()

	return nil
}

func (b *BackupExecutor) listPVCs(annotation string) []corev1.Volume {
	b.Log.Info("Listing all PVCs", "annotation", annotation, "namespace", b.Obj.GetMetaObject().GetNamespace())
	volumes := make([]corev1.Volume, 0)

	claimlist := &corev1.PersistentVolumeClaimList{}

	err := b.Client.List(b.CTX, claimlist, &client.ListOptions{})
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
			MountPath: path.Join(constants.MountPath, volume.Name),
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

func (b *BackupExecutor) startBackup(job *batchv1.Job) {

	preBackup := prebackup.NewPrebackup(b.Config)
	err := preBackup.Start()
	if err != nil {
		b.Config.Log.Error(err, "error while handling pre backup pods")
		return
	}

	name := types.NamespacedName{Namespace: b.Obj.GetMetaObject().GetNamespace(), Name: b.Obj.GetMetaObject().GetName()}

	observer.GetObserver().RegisterCallback(name.String(), preBackup.Stop)

	volumes := b.listPVCs(constants.BackupAnnotationDefault)

	job.Spec.Template.Spec.Volumes = volumes
	job.Spec.Template.Spec.ServiceAccountName = constants.ServiceAccount
	job.Spec.Template.Spec.Containers[0].VolumeMounts = b.getVolumeMounts(volumes)
	err = b.Client.Create(b.CTX, job)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			b.Config.Log.Error(err, "could not create job")
			return
		}
	}
	b.Obj.GetK8upStatus().Started = true

	err = b.Client.Status().Update(b.CTX, b.Obj.GetRuntimeObject().DeepCopyObject())
	if err != nil {
		b.Config.Log.Error(err, "could not update backup status")
	}

}

func (b *BackupExecutor) createServiceAccountAndBinding() error {
	serviceAccount := newServiceAccountDefinition(b.Obj.GetMetaObject().GetNamespace())

	err := b.Client.Create(b.CTX, serviceAccount.role)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
	}

	err = b.Client.Create(b.CTX, serviceAccount.account)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
	}

	err = b.Client.Create(b.CTX, serviceAccount.roleBinding)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
	}

	return nil

}

func newServiceAccountDefinition(namespace string) serviceAccount {
	role := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.ServiceAccount,
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
			Name:      constants.ServiceAccount + "-namespaced",
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: namespace,
				Name:      constants.ServiceAccount,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     constants.ServiceAccount,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	account := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.ServiceAccount,
			Namespace: namespace,
		},
	}

	return serviceAccount{
		role:        &role,
		roleBinding: &roleBinding,
		account:     &account,
	}
}
