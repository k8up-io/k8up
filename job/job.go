// Job handles the internal representation of a job and it's context.

package job

import (
	"context"

	"github.com/vshn/k8up/cfg"

	"github.com/go-logr/logr"
	"github.com/vshn/k8up/api/v1alpha1"
	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
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
	// K8upExclusive is needed to determine if a given job is consideret exclusive or not.
	K8upExclusive = "k8upjob/exclusive"
)

// Config represents the whole context for a given job. It contains everything
// that is necessary to handle the job.
type Config struct {
	Client     client.Client
	Log        logr.Logger
	CTX        context.Context
	Obj        Object
	Scheme     *runtime.Scheme
	Repository string
}

// Object is an interface that must be implemented by all CRDs that implement a
// job.
type Object interface {
	GetMetaObject() metav1.Object
	GetRuntimeObject() runtime.Object
	GetK8upStatus() *k8upv1alpha1.K8upStatus
	GetType() v1alpha1.JobType
	GetResources() corev1.ResourceRequirements
}

// NewConfig returns a new configuration.
func NewConfig(ctx context.Context, client client.Client, log logr.Logger, obj Object, scheme *runtime.Scheme, repository string) Config {
	return Config{
		Client:     client,
		Log:        log,
		CTX:        ctx,
		Obj:        obj,
		Scheme:     scheme,
		Repository: repository,
	}
}

// GetGenericJob returns a generic batchv1.job for further use.
func GetGenericJob(obj Object, config Config) (*batchv1.Job, error) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      obj.GetMetaObject().GetName(),
			Namespace: obj.GetMetaObject().GetNamespace(),
			Labels: map[string]string{
				K8uplabel: "true",
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:      obj.GetMetaObject().GetName(),
							Image:     cfg.Config.BackupImage,
							Resources: config.Obj.GetResources(),
						},
					},
				},
			},
		},
	}

	err := ctrl.SetControllerReference(obj.GetMetaObject(), job, config.Scheme)

	return job, err
}
