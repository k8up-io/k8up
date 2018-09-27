package backup

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

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

func newJobDefinition(volumes []corev1.Volume, controllerName string, backup *backupv1alpha1.Backup, config config) *batchv1.Job {
	mounts := make([]corev1.VolumeMount, 0)
	for _, volume := range volumes {
		tmpMount := corev1.VolumeMount{
			Name:      volume.Name,
			MountPath: path.Join(config.dataPath, volume.Name),
		}
		mounts = append(mounts, tmpMount)
	}

	// We want job names for a given nominal start time to have a deterministic name to avoid the same job being created twice
	name := fmt.Sprintf("%s-%d", config.jobName, time.Now().Unix())

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			OwnerReferences: []metav1.OwnerReference{
				newOwnerReference(backup),
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: backup.Namespace,
					Labels: map[string]string{
						"backupPod": "true",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicy(config.restartPolicy),
					Volumes:       volumes,
					Containers: []corev1.Container{
						{
							Name:            config.podName,
							Image:           config.image,
							VolumeMounts:    mounts,
							Env:             setUpEnvVariables(backup, config),
							ImagePullPolicy: corev1.PullAlways,
							TTY:             true,
							Stdin:           true,
						},
					},
					ServiceAccountName: "pod-executor",
				},
			},
		},
	}
}

func setUpEnvVariables(backup *backupv1alpha1.Backup, config config) []corev1.EnvVar {

	promURL := config.globalPromURL
	if backup.Spec.PromURL != "" {
		promURL = backup.Spec.PromURL
	}

	vars := make([]corev1.EnvVar, 0)

	repoPasswordEnv := service.BuildRepoPasswordVar(backup.Spec.RepoPasswordSecretRef, config.GlobalConfig)

	vars = append(vars, []corev1.EnvVar{
		repoPasswordEnv,
		{
			Name:  service.Hostname,
			Value: backup.Namespace,
		},
		{
			Name:  "PROM_URL",
			Value: promURL,
		},
	}...)

	vars = append(vars, setUpRetention(backup)...)

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

func setUpRetention(backup *backupv1alpha1.Backup) []corev1.EnvVar {
	retentionRules := []corev1.EnvVar{}

	if backup.Spec.Retention.KeepLast > 0 {
		retentionRules = append(retentionRules, corev1.EnvVar{
			Name:  string(service.KeepLast),
			Value: strconv.Itoa(backup.Spec.Retention.KeepLast),
		})
	}

	if backup.Spec.Retention.KeepHourly > 0 {
		retentionRules = append(retentionRules, corev1.EnvVar{
			Name:  string(service.KeepHourly),
			Value: strconv.Itoa(backup.Spec.Retention.KeepHourly),
		})
	}

	if backup.Spec.Retention.KeepDaily > 0 {
		retentionRules = append(retentionRules, corev1.EnvVar{
			Name:  string(service.KeepDaily),
			Value: strconv.Itoa(backup.Spec.Retention.KeepDaily),
		})
	} else {
		//Set defaults
		retentionRules = append(retentionRules, corev1.EnvVar{
			Name:  string(service.KeepDaily),
			Value: strconv.Itoa(14),
		})
	}

	if backup.Spec.Retention.KeepWeekly > 0 {
		retentionRules = append(retentionRules, corev1.EnvVar{
			Name:  string(service.KeepWeekly),
			Value: strconv.Itoa(backup.Spec.Retention.KeepWeekly),
		})
	}

	if backup.Spec.Retention.KeepMonthly > 0 {
		retentionRules = append(retentionRules, corev1.EnvVar{
			Name:  string(service.KeepMonthly),
			Value: strconv.Itoa(backup.Spec.Retention.KeepMonthly),
		})
	}

	if backup.Spec.Retention.KeepYearly > 0 {
		retentionRules = append(retentionRules, corev1.EnvVar{
			Name:  string(service.KeepYearly),
			Value: strconv.Itoa(backup.Spec.Retention.KeepYearly),
		})
	}

	if len(backup.Spec.Retention.KeepTags) > 0 {
		retentionRules = append(retentionRules, corev1.EnvVar{
			Name:  string(service.KeepTag),
			Value: strings.Join(backup.Spec.Retention.KeepTags, ","),
		})
	}

	return retentionRules
}

func newServiceAccontDefinition(backup *backupv1alpha1.Backup, config config) serviceAccount {
	role := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.podExecRoleName,
			Namespace: backup.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				newOwnerReference(backup),
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
				newOwnerReference(backup),
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
				newOwnerReference(backup),
			},
		},
	}

	return serviceAccount{
		role:        &role,
		roleBinding: &roleBinding,
		account:     &account,
	}
}

func newOwnerReference(backup *backupv1alpha1.Backup) metav1.OwnerReference {
	return metav1.OwnerReference{
		UID:        backup.GetUID(),
		APIVersion: backupv1alpha1.SchemeGroupVersion.String(),
		Kind:       backupv1alpha1.BackupKind,
		Name:       backup.Name,
	}
}

type serviceAccount struct {
	role        *rbacv1.Role
	roleBinding *rbacv1.RoleBinding
	account     *corev1.ServiceAccount
}
