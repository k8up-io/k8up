package baas

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	"github.com/spf13/viper"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	image                 string
	dataPath              string
	jobName               string
	podName               string
	restartPolicy         string
	promURL               string
	podExecRoleName       string
	podExecAccountName    string
	globalAccessKeyID     string
	globalSecretAccessKey string
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
)

func init() {
	viper.SetDefault("image", "172.30.1.1:5000/myproject/restic")
	viper.SetDefault("dataPath", "/data")
	viper.SetDefault("jobName", "backupjob")
	viper.SetDefault("podName", "backupjob-pod")
	viper.SetDefault("restartPolicy", "OnFailure")
	viper.SetDefault("PromURL", "http://127.0.0.1/")
	viper.SetDefault("PodExecRoleName", "pod-executor")
	viper.SetDefault("PodExecAccountName", "pod-executor")
	viper.SetDefault("GlobalAccessKeyID", "")
	viper.SetDefault("GlobalSecretAccessKey", "")
}

func getConfig() {
	image = viper.GetString("image")
	dataPath = viper.GetString("dataPath")
	jobName = viper.GetString("jobName")
	podName = viper.GetString("podName")
	restartPolicy = viper.GetString("restartPolicy")
	promURL = viper.GetString("PromURL")
	podExecRoleName = viper.GetString("PodExecRoleName")
	podExecAccountName = viper.GetString("PodExecAccountName")
	globalAccessKeyID = viper.GetString("GlobalAccessKeyID")
	globalSecretAccessKey = viper.GetString("GlobalSecretAccessKey")
}

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

func newJobDefinition(volumes []apiv1.Volume, controllerName string, backup *backupv1alpha1.Backup) *batchv1.Job {
	getConfig()
	mounts := make([]apiv1.VolumeMount, 0)
	for _, volume := range volumes {
		tmpMount := apiv1.VolumeMount{
			Name:      volume.Name,
			MountPath: path.Join(dataPath, volume.Name),
		}
		mounts = append(mounts, tmpMount)
	}

	// We want job names for a given nominal start time to have a deterministic name to avoid the same job being created twice
	name := fmt.Sprintf("%s-%d", jobName, time.Now().Unix())

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
					RestartPolicy: apiv1.RestartPolicy(restartPolicy),
					Volumes:       volumes,
					Containers: []apiv1.Container{
						{
							Name:            podName,
							Image:           image,
							VolumeMounts:    mounts,
							Env:             setUpEnvVariables(backup),
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

func setUpEnvVariables(backup *backupv1alpha1.Backup) []apiv1.EnvVar {

	if backup.Spec.PromURL != "" {
		promURL = backup.Spec.PromURL
	}

	vars := make([]apiv1.EnvVar, 0)
	vars = append(vars, []apiv1.EnvVar{
		{
			Name: resticPassword,
			ValueFrom: &apiv1.EnvVarSource{
				SecretKeyRef: &apiv1.SecretKeySelector{
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: "backup-repo",
					},
					Key: "password",
				},
			},
		},
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

	if backup.Spec.Backend.S3 != nil {
		s3Backend := backup.Spec.Backend.S3
		r := fmt.Sprintf("s3:%s/%s", s3Backend.Endpoint, s3Backend.Bucket)

		accessKeyID := apiv1.EnvVar{
			Name:  awsAccessKeyID,
			Value: globalAccessKeyID,
		}
		secretKeyID := apiv1.EnvVar{
			Name:  awsSecretAccessKey,
			Value: globalSecretAccessKey,
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
		vars = append(vars, []apiv1.EnvVar{
			accessKeyID,
			secretKeyID,
		}...)

		vars = append(vars, []apiv1.EnvVar{
			{
				Name:  resticRepository,
				Value: r,
			},
		}...)
		return vars
	}

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

func newServiceAccontDefinition(backup *backupv1alpha1.Backup) serviceAccount {
	getConfig()

	role := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podExecRoleName,
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
			Name:      podExecRoleName + "-namespaced",
			Namespace: backup.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				newOwnerReference(backup),
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: backup.Namespace,
				Name:      podExecRoleName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     podExecRoleName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	account := apiv1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podExecAccountName,
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
