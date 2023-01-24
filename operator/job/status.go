package job

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
)

// SetConditionTrue tells the K8s controller at once that the status of the given Conditions is now "True"
func (c *Config) SetConditionTrue(ctx context.Context, condition k8upv1.ConditionType, reason k8upv1.ConditionReason) {
	c.patchConditions(ctx, metav1.ConditionTrue, reason, "The resource is ready", condition)
}

// SetConditionUnknownWithMessage tells the K8s controller at once that the status of the given Conditions is "Unknown"
func (c *Config) SetConditionUnknownWithMessage(ctx context.Context, condition k8upv1.ConditionType, reason k8upv1.ConditionReason, message string, args ...interface{}) {
	c.patchConditions(ctx, metav1.ConditionUnknown, reason, fmt.Sprintf(message, args...), condition)
}

// SetConditionTrueWithMessage tells the K8s controller at once that the status of the given Condition is now "True" and
// provides the given message.
// The arguments `message` and `args` follow the fmt.Sprintf() syntax.
func (c *Config) SetConditionTrueWithMessage(ctx context.Context, condition k8upv1.ConditionType, reason k8upv1.ConditionReason, message string, args ...interface{}) {
	c.patchConditions(ctx, metav1.ConditionTrue, reason, fmt.Sprintf(message, args...), condition)
}

// SetConditionFalseWithMessage tells the K8s controller at once that the status of the given Condition is now "False" and
// provides the given message.
// The arguments `message` and `args` follow the fmt.Sprintf() syntax.
func (c *Config) SetConditionFalseWithMessage(ctx context.Context, condition k8upv1.ConditionType, reason k8upv1.ConditionReason, message string, args ...interface{}) {
	c.patchConditions(ctx, metav1.ConditionFalse, reason, fmt.Sprintf(message, args...), condition)
}

// patchConditions patches the Status object on the K8s controller with the given Conditions
func (c *Config) patchConditions(ctx context.Context, conditionStatus metav1.ConditionStatus, reason k8upv1.ConditionReason, message string, condition k8upv1.ConditionType) {
	log := controllerruntime.LoggerFrom(ctx)
	status := c.Obj.GetStatus()
	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:    condition.String(),
		Status:  conditionStatus,
		Message: message,
		Reason:  reason.String(),
	})
	c.Obj.SetStatus(status)

	err := c.Client.Status().Update(ctx, c.Obj)
	if err != nil {
		if errors.IsNotFound(err) {
			return
		}
		log.Error(err, "could not patch status condition")
	}
}

// SetStarted marks the job as started and updates the status.
// The arguments `message` and `args` follow the fmt.Sprintf() syntax.
func (c *Config) SetStarted(ctx context.Context, message string, args ...interface{}) {
	log := controllerruntime.LoggerFrom(ctx)

	status := c.Obj.GetStatus()
	status.SetStarted(fmt.Sprintf(message, args...))
	c.Obj.SetStatus(status)

	patch := client.MergeFrom(c.Obj.DeepCopyObject().(client.Object))
	err := c.Client.Status().Patch(ctx, c.Obj, patch)
	if err != nil {
		if errors.IsNotFound(err) {
			return
		}
		log.Error(err, "could not patch status")
	}
}

// SetFinished marks the job as finished and updates the status.
func (c *Config) SetFinished(ctx context.Context, namespace, name string) {
	log := controllerruntime.LoggerFrom(ctx)

	status := c.Obj.GetStatus()
	status.SetFinished(fmt.Sprintf("the Job '%s/%s' ended", namespace, name))
	c.Obj.SetStatus(status)

	patch := client.MergeFrom(c.Obj.DeepCopyObject().(client.Object))
	err := c.Client.Status().Patch(ctx, c.Obj, patch)
	if err != nil {
		if errors.IsNotFound(err) {
			return
		}
		log.Error(err, "could not patch status")
	}
}
