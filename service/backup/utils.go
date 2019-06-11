package backup

import (
	"path"

	backupv1alpha1 "github.com/vshn/k8up/apis/backup/v1alpha1"
	"github.com/vshn/k8up/service"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newBackupJob(volumes []corev1.Volume, controllerName string, backup *backupv1alpha1.Backup, config config) *batchv1.Job {
	mounts := make([]corev1.VolumeMount, 0)
	for _, volume := range volumes {
		tmpMount := corev1.VolumeMount{
			Name:      volume.Name,
			MountPath: path.Join(config.dataPath, volume.Name),
		}
		mounts = append(mounts, tmpMount)
	}

	job := service.GetBasicJob(backupv1alpha1.BackupKind, config.Global, &backup.ObjectMeta)

	finalEnv := append(job.Spec.Template.Spec.Containers[0].Env, setUpEnvVariables(backup, config)...)

	job.Spec.Template.Spec.Volumes = volumes
	job.Spec.Template.Spec.ServiceAccountName = "pod-executor"
	job.Spec.Template.Spec.Containers[0].VolumeMounts = mounts
	job.Spec.Template.Spec.Containers[0].Env = finalEnv

	return job
}

// TODO: evaluate if it makes sense to make these functions part of backupv1alpha1.Backu
func setUpEnvVariables(backup *backupv1alpha1.Backup, config config) []corev1.EnvVar {

	promURL := config.GlobalPromURL
	if backup.Spec.PromURL != "" {
		promURL = backup.Spec.PromURL
	}

	statsURL := config.GlobalStatsURL
	if backup.Spec.StatsURL != "" {
		statsURL = backup.Spec.StatsURL
	}

	vars := service.DefaultEnvs(backup.Spec.Backend, config.Global)

	vars = append(vars, []corev1.EnvVar{
		{
			Name:  service.PromURL,
			Value: promURL,
		},
		// TODO: This is a hack as the statsurl is already set in DefaultEnvs.
		// But DefaultEnvs doesn't have the necessary information to do the
		// actual merge... As k8s will apply the last env var it finds this
		// actually works but should be fixed properly sometime, to avoid
		// problems.
		{
			Name:  service.StatsURL,
			Value: statsURL,
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
				service.NewOwnerReference(backup, backupv1alpha1.BackupKind),
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
				service.NewOwnerReference(backup, backupv1alpha1.BackupKind),
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
				service.NewOwnerReference(backup, backupv1alpha1.BackupKind),
			},
		},
	}

	return serviceAccount{
		role:        &role,
		roleBinding: &roleBinding,
		account:     &account,
	}
}

func (b *backupRunner) getDeployments() []appsv1.Deployment {
	tmp := []appsv1.Deployment{}
	replicas := int32(1)

	templates, err := b.BaasCLI.Appuio().PreBackupPods(b.backup.GetNamespace()).List(metav1.ListOptions{})

	if err != nil {
		b.Logger.Errorf("could not list podtemplates: %v", err)
	} else {
		for _, template := range templates.Items {

			template.Spec.Pod.PodTemplateSpec.ObjectMeta.Annotations = map[string]string{
				b.config.backupCommandAnnotation: template.Spec.BackupCommand,
				b.config.fileExtensionAnnotation: template.Spec.FileExtension,
			}

			podLabels := map[string]string{
				"backupCommandPod": "true",
				"preBackupPod":     template.GetName(),
			}

			template.Spec.Pod.PodTemplateSpec.ObjectMeta.Labels = podLabels

			dep := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      template.GetName(),
					Namespace: template.GetNamespace(),
					OwnerReferences: []metav1.OwnerReference{
						service.NewOwnerReference(b.backup, backupv1alpha1.BackupKind),
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Template: template.Spec.Pod.PodTemplateSpec,
					Selector: &metav1.LabelSelector{
						MatchLabels: podLabels,
					},
				},
			}

			tmp = append(tmp, dep)
		}
	}

	return tmp

}

type serviceAccount struct {
	role        *rbacv1.Role
	roleBinding *rbacv1.RoleBinding
	account     *corev1.ServiceAccount
}
