package job

import (
	"context"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultImage = "busybox"
)

type Config struct {
	Client    client.Client
	Log       logr.Logger
	CTX       context.Context
	Obj       metav1.Object
	Scheme    *runtime.Scheme
	Name      string
	Exclusive bool
}

func GetGenericJob(obj metav1.Object, scheme *runtime.Scheme) (*batchv1.Job, error) {

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:  obj.GetName(),
							Image: defaultImage,
							Command: []string{
								"sleep",
								"30",
							},
						},
					},
				},
			},
		},
	}

	err := ctrl.SetControllerReference(obj, job, scheme)

	return job, err
}
