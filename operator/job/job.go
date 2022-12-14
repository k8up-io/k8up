// Job handles the internal representation of a job and it's context.

package job

import (
	"context"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// K8uplabel is a label that is required for the operator to differentiate
	// batchv1.job objects managed by k8up from others.
	K8uplabel = "k8upjob"
	// K8upExclusive is needed to determine if a given job is considered exclusive or not.
	K8upExclusive = "k8upjob/exclusive"
)

// Config represents the whole context for a given job. It contains everything
// that is necessary to handle the job.
type Config struct {
	Client     client.Client
	Log        logr.Logger
	CTX        context.Context
	Obj        k8upv1.JobObject
	Scheme     *runtime.Scheme
	Repository string
}

// NewConfig returns a new configuration.
func NewConfig(ctx context.Context, client client.Client, log logr.Logger, obj k8upv1.JobObject, scheme *runtime.Scheme, repository string) Config {
	return Config{
		Client:     client,
		Log:        log,
		CTX:        ctx,
		Obj:        obj,
		Scheme:     scheme,
		Repository: repository,
	}
}

// GenerateGenericJob returns a generic batchv1.job for further use.
func GenerateGenericJob(obj k8upv1.JobObject, config Config) (*batchv1.Job, error) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      obj.GetJobName(),
			Namespace: obj.GetNamespace(),
			Labels: map[string]string{
				K8uplabel:            "true",
				k8upv1.LabelK8upType: obj.GetType().String(),
			},
		},
		Spec: batchv1.JobSpec{
			ActiveDeadlineSeconds: obj.GetActiveDeadlineSeconds(),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						K8uplabel: "true",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:      obj.GetType().String(),
							Image:     cfg.Config.BackupImage,
							Command:   cfg.Config.BackupCommandRestic,
							Resources: config.Obj.GetResources(),
						},
					},
					SecurityContext: obj.GetPodSecurityContext(),
				},
			},
		},
	}

	err := ctrl.SetControllerReference(obj, job, config.Client.Scheme())

	return job, err
}
