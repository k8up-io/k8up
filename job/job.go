// Job handles the internal representation of a job and it's context.

package job

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/vshn/k8up/api/v1alpha1"
	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/cfg"

	"github.com/go-logr/logr"
	"github.com/operator-framework/operator-lib/status"
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

	// K8upTypeLabel is the label key that identifies the job type.
	K8upTypeLabel = "k8up.io/type"
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
	GetStatus() *k8upv1alpha1.Status
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
				K8uplabel:     "true",
				K8upTypeLabel: obj.GetType().String(),
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

// SetConditionTrue tells the K8s controller at once that the status of the given Conditions is now "True"
func (c *Config) SetConditionTrue(condition status.ConditionType) {
	c.patchConditions(corev1.ConditionTrue, "", condition)
}

// SetConditionFalse tells the K8s controller at once that the status of the given Condition is now "True" and
// provides the given message.
// The arguments `message` and `args` follow the fmt.Sprintf() syntax.
func (c *Config) SetConditionTrueWithMessage(condition status.ConditionType, message string, args ...interface{}) {
	c.patchConditions(corev1.ConditionTrue, fmt.Sprintf(message, args...), condition)
}

// SetConditionFalse tells the K8s controller at once that the status of the given Condition is now "False" and
// provides the given message.
// The arguments `message` and `args` follow the fmt.Sprintf() syntax.
func (c *Config) SetConditionFalse(condition status.ConditionType, message string, args ...interface{}) {
	c.patchConditions(corev1.ConditionFalse, fmt.Sprintf(message, args...), condition)
}

// patchConditions patches the Status object on the K8s controller with the given Conditions
func (c *Config) patchConditions(conditionStatus corev1.ConditionStatus, message string, conditions ...status.ConditionType) {
	runtimeObject := c.Obj.GetRuntimeObject()
	patch := client.MergeFrom(runtimeObject.DeepCopyObject())

	for _, condition := range conditions {
		c.Obj.GetStatus().Conditions.SetCondition(status.Condition{
			Type:    condition,
			Status:  conditionStatus,
			Message: message,
		})
	}

	err := c.Client.Status().Patch(c.CTX, c.Obj.GetRuntimeObject().(client.Object), patch)
	if err != nil {
		if errors.IsNotFound(err) {
			return
		}
		c.Log.Error(err, "could not patch backup conditions")
	}
}

// SetStarted sets the `c.Obj.GetStatus().Started` property to `true`.
// In the same call to the k8s API it also sets all the given conditions to "True".
// The arguments `message` and `args` follow the fmt.Sprintf() syntax.
func (c *Config) SetStarted(trueCondition status.ConditionType, message string, args ...interface{}) {
	runtimeObject := c.Obj.GetRuntimeObject()
	patch := client.MergeFrom(runtimeObject.DeepCopyObject())

	c.Obj.GetStatus().Started = true

	c.Obj.GetStatus().Conditions.SetCondition(status.Condition{
		Type:    trueCondition,
		Status:  corev1.ConditionTrue,
		Message: fmt.Sprintf(message, args...),
	})

	err := c.Client.Status().Patch(c.CTX, c.Obj.GetRuntimeObject().(client.Object), patch)
	if err != nil {
		if errors.IsNotFound(err) {
			return
		}
		c.Log.Error(err, "could not patch backup conditions")
	}
}
