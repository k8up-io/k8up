package job

import (
	"context"

	"github.com/go-logr/logr"
	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/constants"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	K8uplabel     = "k8upjob"
	K8upExclusive = "k8upjob/exclusive"
)

type Config struct {
	Client     client.Client
	Log        logr.Logger
	CTX        context.Context
	Obj        Object
	Scheme     *runtime.Scheme
	Repository string
}

type Object interface {
	GetMetaObject() metav1.Object
	GetRuntimeObject() runtime.Object
	GetK8upStatus() *k8upv1alpha1.K8upStatus
	GetType() string
}

func NewConfig(ctx context.Context, client client.Client, log logr.Logger, obj Object, scheme *runtime.Scheme) Config {
	return Config{
		Client: client,
		Log:    log,
		CTX:    ctx,
		Obj:    obj,
		Scheme: scheme,
	}
}

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
							Name:  obj.GetMetaObject().GetName(),
							Image: constants.GetBackupImage(),
						},
					},
				},
			},
		},
	}

	err := ctrl.SetControllerReference(obj.GetMetaObject(), job, config.Scheme)

	return job, err
}
