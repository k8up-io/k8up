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

// byJobStartTime sorts a list of jobs by start timestamp, using their names as a tie breaker.
type byJobStartTime []batchv1.Job

func (o byJobStartTime) Len() int      { return len(o) }
func (o byJobStartTime) Swap(i, j int) { o[i], o[j] = o[j], o[i] }

func (o byJobStartTime) Less(i, j int) bool {
	if o[j].Status.StartTime == nil {
		return o[i].Status.StartTime != nil
	}

	if o[i].Status.StartTime.Equal(o[j].Status.StartTime) {
		return o[i].Name < o[j].Name
	}

	return o[i].Status.StartTime.Before(o[j].Status.StartTime)
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

	job := service.GetBasicJob("backup", config.GlobalConfig, &backup.ObjectMeta)

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

	vars := make([]corev1.EnvVar, 0)

	repoPasswordEnv := service.BuildRepoPasswordVar(backup.GlobalOverrides.RegisteredBackend.RepoPasswordSecretRef, config.GlobalConfig)

	vars = append(vars, []corev1.EnvVar{
		repoPasswordEnv,
		{
			Name:  service.PromURL,
			Value: promURL,
		},
	}...)

	s3Backend := service.BuildS3EnvVars(backup.GlobalOverrides.RegisteredBackend.S3, config.GlobalConfig)

	vars = append(vars, s3Backend...)

	if backup.Spec.StatsURL != "" {
		config.GlobalStatsURL = backup.Spec.StatsURL
	}

	vars = append(vars, []corev1.EnvVar{
		{
			Name:  service.StatsURL,
			Value: config.GlobalStatsURL,
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
