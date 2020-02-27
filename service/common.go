package service

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/spotahome/kooper/client/crd"
	backupv1alpha1 "github.com/vshn/k8up/apis/backup/v1alpha1"
	baas8scli "github.com/vshn/k8up/client/k8s/clientset/versioned"
	"github.com/vshn/k8up/config"
	"github.com/vshn/k8up/log"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Shared constants between the various services
const (
	Hostname    = "HOSTNAME"
	KeepLast    = "KEEP_LAST"
	KeepHourly  = "KEEP_HOURLY"
	KeepDaily   = "KEEP_DAILY"
	KeepWeekly  = "KEEP_WEEKLY"
	KeepMonthly = "KEEP_MONTHLY"
	KeepYearly  = "KEEP_YEARLY"
	KeepTag     = "KEEP_TAG"
	StatsURL    = "STATS_URL"
	RestorePath = "/restore"
	PromURL     = "PROM_URL"
)

type CommonObjects struct {
	BaasCLI baas8scli.Interface
	CrdCli  crd.Interface
	K8sCli  kubernetes.Interface
	Logger  log.Logger
}

func NewOwnerReference(object metav1.Object, kind string) metav1.OwnerReference {
	return metav1.OwnerReference{
		UID:        object.GetUID(),
		APIVersion: backupv1alpha1.SchemeGroupVersion.String(),
		Kind:       kind,
		Name:       object.GetName(),
	}
}

// PseudoUUID is used to generate IDs for baas related pods/jobs
func PseudoUUID() string {

	b := make([]byte, 16)
	rand.Read(b)

	return fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func GetRepository(obj interface{}) string {
	switch obj.(type) {
	case *corev1.Pod:
		pod, _ := obj.(*corev1.Pod)
		// baas pods only have one container
		for _, env := range pod.Spec.Containers[0].Env {
			if env.Name == backupv1alpha1.ResticRepository {
				return env.Value
			}
		}
	case *backupv1alpha1.Backend:
		backend, _ := obj.(*backupv1alpha1.Backend)
		if backend == nil {
			return ""
		}
		return backend.String()
	}

	return ""
}

func GetBasicJob(kind string, config config.Global, object metav1.Object) *batchv1.Job {

	t := time.Now().Unix()
	namePrefix := strings.ToLower(kind)
	nameJob := fmt.Sprintf("%vjob-%d", namePrefix, t)
	namePod := fmt.Sprintf("%vpod-%d", namePrefix, t)

	labels := map[string]string{
		config.Label:      "true",
		config.Identifier: PseudoUUID(),
	}
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameJob,
			Namespace: object.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{
				NewOwnerReference(object, kind),
			},
			Labels: labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &config.BackOffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      namePod,
					Namespace: object.GetNamespace(),
					Labels:    labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicy(config.RestartPolicy),
					Containers: []corev1.Container{
						{
							Name:  namePod,
							Image: config.Image,
							Env: []corev1.EnvVar{
								{
									Name:  Hostname,
									Value: object.GetNamespace(),
								},
							},
							ImagePullPolicy: corev1.PullAlways,
							TTY:             true,
							Stdin:           true,
						},
					},
				},
			},
		},
	}
}

func DefaultEnvs(backend *backupv1alpha1.Backend, config config.Global) []corev1.EnvVar {

	if backend == nil {
		backend = &backupv1alpha1.Backend{S3: &backupv1alpha1.S3Spec{}}
	}

	backendTmp := &backupv1alpha1.Backend{S3: &backupv1alpha1.S3Spec{}}
	if backend.S3 != nil {
		backendTmp = backend
	}

	vars := backendTmp.S3.BackupEnvs(config)

	password := &backupv1alpha1.Backend{}
	if backend.RepoPasswordSecretRef != nil {
		password.RepoPasswordSecretRef = backend.RepoPasswordSecretRef
	}

	vars = append(vars, password.PasswordEnvVar(config))

	vars = append(vars, []corev1.EnvVar{
		{
			Name:  StatsURL,
			Value: config.GlobalStatsURL,
		},
	}...)
	return vars
}

// BuildTagArgs will prepend "--tag " to every element in the given []string
func BuildTagArgs(tagList []string) []string {
	var args []string
	for i := range tagList {
		args = append(args, "--tag", tagList[i])
	}
	return args
}
