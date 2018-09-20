package backup

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	hostname           = "HOSTNAME"
	resticRepository   = "RESTIC_REPOSITORY"
	resticPassword     = "RESTIC_PASSWORD"
	awsAccessKeyID     = "AWS_ACCESS_KEY_ID"
	awsSecretAccessKey = "AWS_SECRET_ACCESS_KEY"
	keepLast           = "KEEP_LAST"
	keepHourly         = "KEEP_HOURLY"
	keepDaily          = "KEEP_DAILY"
	keepWeekly         = "KEEP_WEEKLY"
	keepMonthly        = "KEEP_MONTHLY"
	keepYearly         = "KEEP_YEARLY"
	keepTag            = "KEEP_TAG"
	statsURL           = "STATS_URL"
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

func newJobDefinition(volumes []apiv1.Volume, controllerName string, backup *backupv1alpha1.Backup, config config) *batchv1.Job {
	mounts := make([]apiv1.VolumeMount, 0)
	for _, volume := range volumes {
		tmpMount := apiv1.VolumeMount{
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
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: backup.Namespace,
					Labels: map[string]string{
						"backupPod": "true",
					},
				},
				Spec: apiv1.PodSpec{
					RestartPolicy: apiv1.RestartPolicy(config.restartPolicy),
					Volumes:       volumes,
					Containers: []apiv1.Container{
						{
							Name:            config.podName,
							Image:           config.image,
							VolumeMounts:    mounts,
							Env:             setUpEnvVariables(backup, config),
							ImagePullPolicy: apiv1.PullAlways,
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

func setUpEnvVariables(backup *backupv1alpha1.Backup, config config) []apiv1.EnvVar {

	promURL := config.globalPromURL
	if backup.Spec.PromURL != "" {
		promURL = backup.Spec.PromURL
	}

	vars := make([]apiv1.EnvVar, 0)

	repoPasswordEnv := apiv1.EnvVar{
		Name:  resticPassword,
		Value: config.globalRepoPassword,
	}

	if backup.Spec.RepoPasswordSecretRef != nil {
		repoPasswordEnv = apiv1.EnvVar{
			Name: resticPassword,
			ValueFrom: &apiv1.EnvVarSource{
				SecretKeyRef: &apiv1.SecretKeySelector{
					LocalObjectReference: backup.Spec.RepoPasswordSecretRef.LocalObjectReference,
					Key:                  backup.Spec.RepoPasswordSecretRef.Key,
				},
			},
		}
	}

	vars = append(vars, []apiv1.EnvVar{
		repoPasswordEnv,
		{
			Name:  hostname,
			Value: backup.Namespace,
		},
		{
			Name:  "PROM_URL",
			Value: promURL,
		},
	}...)

	vars = append(vars, setUpRetention(backup)...)

	s3Backend := backup.GlobalOverrides.RegisteredBackend.S3
	if s3Backend != nil {
		s3Endpoint := s3Backend.Endpoint
		s3Bucket := s3Backend.Bucket

		accessKeyID := apiv1.EnvVar{
			Name:  awsAccessKeyID,
			Value: config.globalAccessKeyID,
		}
		secretKeyID := apiv1.EnvVar{
			Name:  awsSecretAccessKey,
			Value: config.globalSecretAccessKey,
		}

		if backup.Spec.Backend.S3 != nil {
			s3Backend.SecretAccessKeySecretRef = backup.Spec.Backend.S3.SecretAccessKeySecretRef
			s3Backend.AccessKeyIDSecretRef = backup.Spec.Backend.S3.AccessKeyIDSecretRef
		}

		if s3Backend.AccessKeyIDSecretRef != nil && s3Backend.SecretAccessKeySecretRef != nil {
			accessKeyID = apiv1.EnvVar{
				Name: awsAccessKeyID,
				ValueFrom: &apiv1.EnvVarSource{
					SecretKeyRef: &apiv1.SecretKeySelector{
						LocalObjectReference: s3Backend.AccessKeyIDSecretRef.LocalObjectReference,
						Key:                  s3Backend.AccessKeyIDSecretRef.Key,
					},
				},
			}
			secretKeyID = apiv1.EnvVar{
				Name: awsSecretAccessKey,
				ValueFrom: &apiv1.EnvVarSource{
					SecretKeyRef: &apiv1.SecretKeySelector{
						LocalObjectReference: s3Backend.SecretAccessKeySecretRef.LocalObjectReference,
						Key:                  s3Backend.SecretAccessKeySecretRef.Key,
					},
				},
			}
		}
		r := fmt.Sprintf("s3:%s/%s", s3Endpoint, s3Bucket)

		vars = append(vars, []apiv1.EnvVar{
			accessKeyID,
			secretKeyID,
			{
				Name:  resticRepository,
				Value: r,
			},
		}...)
	}

	if backup.Spec.StatsURL != "" {
		config.globalStatsURL = backup.Spec.StatsURL
	}

	vars = append(vars, []apiv1.EnvVar{
		{
			Name:  statsURL,
			Value: config.globalStatsURL,
		},
	}...)

	return vars
}

func setUpRetention(backup *backupv1alpha1.Backup) []apiv1.EnvVar {
	retentionRules := []apiv1.EnvVar{}

	if backup.Spec.Retention.KeepLast > 0 {
		retentionRules = append(retentionRules, apiv1.EnvVar{
			Name:  string(keepLast),
			Value: strconv.Itoa(backup.Spec.Retention.KeepLast),
		})
	}

	if backup.Spec.Retention.KeepHourly > 0 {
		retentionRules = append(retentionRules, apiv1.EnvVar{
			Name:  string(keepHourly),
			Value: strconv.Itoa(backup.Spec.Retention.KeepHourly),
		})
	}

	if backup.Spec.Retention.KeepDaily > 0 {
		retentionRules = append(retentionRules, apiv1.EnvVar{
			Name:  string(keepDaily),
			Value: strconv.Itoa(backup.Spec.Retention.KeepDaily),
		})
	} else {
		//Set defaults
		retentionRules = append(retentionRules, apiv1.EnvVar{
			Name:  string(keepDaily),
			Value: strconv.Itoa(14),
		})
	}

	if backup.Spec.Retention.KeepWeekly > 0 {
		retentionRules = append(retentionRules, apiv1.EnvVar{
			Name:  string(keepWeekly),
			Value: strconv.Itoa(backup.Spec.Retention.KeepWeekly),
		})
	}

	if backup.Spec.Retention.KeepMonthly > 0 {
		retentionRules = append(retentionRules, apiv1.EnvVar{
			Name:  string(keepMonthly),
			Value: strconv.Itoa(backup.Spec.Retention.KeepMonthly),
		})
	}

	if backup.Spec.Retention.KeepYearly > 0 {
		retentionRules = append(retentionRules, apiv1.EnvVar{
			Name:  string(keepYearly),
			Value: strconv.Itoa(backup.Spec.Retention.KeepYearly),
		})
	}

	if len(backup.Spec.Retention.KeepTags) > 0 {
		retentionRules = append(retentionRules, apiv1.EnvVar{
			Name:  string(keepTag),
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

	account := apiv1.ServiceAccount{
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
	account     *apiv1.ServiceAccount
}
