package backup

import (
	"path"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	"git.vshn.net/vshn/baas/service"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type byCreationTime []backupv1alpha1.Backup

func (b byCreationTime) Len() int      { return len(b) }
func (b byCreationTime) Swap(i, j int) { b[i], b[j] = b[j], b[i] }

func (b byCreationTime) Less(i, j int) bool {

	if b[i].CreationTimestamp.Equal(&b[j].CreationTimestamp) {
		return b[i].Name < b[j].Name
	}

	return b[i].CreationTimestamp.Before(&b[j].CreationTimestamp)
}

func newBackupJob(volumes []corev1.Volume, controllerName string, backup *backupv1alpha1.Backup, config config) *batchv1.Job {
	mounts := make([]corev1.VolumeMount, 0)
	for _, volume := range volumes {
		tmpMount := corev1.VolumeMount{
			Name:      volume.Name,
			MountPath: path.Join(config.dataPath, volume.Name),
		}
		mounts = append(mounts, tmpMount)
	}

	job := service.GetBasicJob("backup", config.Global, &backup.ObjectMeta)

	finalEnv := append(job.Spec.Template.Spec.Containers[0].Env, setUpEnvVariables(backup, config)...)

	job.Spec.Template.Spec.Volumes = volumes
	job.Spec.Template.Spec.ServiceAccountName = "pod-executor"
	job.Spec.Template.Spec.Containers[0].VolumeMounts = mounts
	job.Spec.Template.Spec.Containers[0].Env = finalEnv

	return job
}

// TODO: move most of this into service, too
func setUpEnvVariables(backup *backupv1alpha1.Backup, config config) []corev1.EnvVar {

	promURL := config.GlobalPromURL
	if backup.Spec.PromURL != "" {
		promURL = backup.Spec.PromURL
	}

	vars := service.DefaultEnvs(backup.Spec.Backend, config.Global)

	vars = append(vars, []corev1.EnvVar{
		{
			Name:  service.PromURL,
			Value: promURL,
		},
	}...)
	return vars
}

func newServiceAccountDefinition(backup *backupv1alpha1.Backup, config config) serviceAccount {
	role := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.podExecRoleName,
			Namespace: backup.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				service.NewOwnerReference(backup),
			},
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
			Name:      config.podExecRoleName + "-namespaced",
			Namespace: backup.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				service.NewOwnerReference(backup),
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: backup.Namespace,
				Name:      config.podExecRoleName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     config.podExecRoleName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	account := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.podExecAccountName,
			Namespace: backup.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				service.NewOwnerReference(backup),
			},
		},
	}

	return serviceAccount{
		role:        &role,
		roleBinding: &roleBinding,
		account:     &account,
	}
}

type serviceAccount struct {
	role        *rbacv1.Role
	roleBinding *rbacv1.RoleBinding
	account     *corev1.ServiceAccount
}
