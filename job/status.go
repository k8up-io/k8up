package job

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
)

// SetConditionTrue tells the K8s controller at once that the status of the given Conditions is now "True"
func (c *Config) SetConditionTrue(condition k8upv1alpha1.ConditionType, reason k8upv1alpha1.ConditionReason) {
	c.patchConditions(metav1.ConditionTrue, reason, "The resource is ready", condition)
}

// SetConditionUnknownWithMessage tells the K8s controller at once that the status of the given Conditions is "Unknown"
func (c *Config) SetConditionUnknownWithMessage(condition k8upv1alpha1.ConditionType, reason k8upv1alpha1.ConditionReason, message string, args ...interface{}) {
	c.patchConditions(metav1.ConditionUnknown, reason, fmt.Sprintf(message, args...), condition)
}

// SetConditionTrueWithMessage tells the K8s controller at once that the status of the given Condition is now "True" and
// provides the given message.
// The arguments `message` and `args` follow the fmt.Sprintf() syntax.
func (c *Config) SetConditionTrueWithMessage(condition k8upv1alpha1.ConditionType, reason k8upv1alpha1.ConditionReason, message string, args ...interface{}) {
	c.patchConditions(metav1.ConditionTrue, reason, fmt.Sprintf(message, args...), condition)
}

// SetConditionFalseWithMessage tells the K8s controller at once that the status of the given Condition is now "False" and
// provides the given message.
// The arguments `message` and `args` follow the fmt.Sprintf() syntax.
func (c *Config) SetConditionFalseWithMessage(condition k8upv1alpha1.ConditionType, reason k8upv1alpha1.ConditionReason, message string, args ...interface{}) {
	c.patchConditions(metav1.ConditionFalse, reason, fmt.Sprintf(message, args...), condition)
}

// patchConditions patches the Status object on the K8s controller with the given Conditions
func (c *Config) patchConditions(conditionStatus metav1.ConditionStatus, reason k8upv1alpha1.ConditionReason, message string, conditions ...k8upv1alpha1.ConditionType) {
	runtimeObject := c.Obj.GetRuntimeObject()
	patch := client.MergeFrom(runtimeObject.DeepCopyObject())

	status := c.Obj.GetStatus()

	for _, condition := range conditions {
		meta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type:    condition.String(),
			Status:  conditionStatus,
			Message: message,
			Reason:  reason.String(),
		})
	}

	c.Obj.SetStatus(status)
	err := c.Client.Status().Patch(c.CTX, c.Obj.GetRuntimeObject().(client.Object), patch)
	if err != nil {
		if errors.IsNotFound(err) {
			return
		}
		c.Log.Error(err, "could not patch status conditions")
	}
}

// SetStarted sets the `c.Obj.GetStatus().Started` property to `true`.
// In the same call to the k8s API it also sets the Ready and Progressing conditions to "True"
// The arguments `message` and `args` follow the fmt.Sprintf() syntax.
func (c *Config) SetStarted(message string, args ...interface{}) {
	status := c.Obj.GetStatus()
	status.Started = true

	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:    k8upv1alpha1.ConditionReady.String(),
		Status:  metav1.ConditionTrue,
		Reason:  k8upv1alpha1.ReasonReady.String(),
		Message: fmt.Sprintf(message, args...),
	})
	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:    k8upv1alpha1.ConditionProgressing.String(),
		Status:  metav1.ConditionTrue,
		Reason:  k8upv1alpha1.ReasonStarted.String(),
		Message: "The job is progressing",
	})

	c.Obj.SetStatus(status)
	err := c.Client.Status().Update(c.CTX, c.Obj.GetRuntimeObject().(client.Object))
	if err != nil {
		if errors.IsNotFound(err) {
			return
		}
		c.Log.Error(err, "could not patch status")
	}
}

// SetFinished sets the `c.Obj.GetStatus().Finished` property to `true`.
// In the same call to the k8s API it also sets the Progressing condition to "False" with reason Finished.
func (c *Config) SetFinished(namespace, name string) {
	status := c.Obj.GetStatus()
	status.Finished = true

	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:    k8upv1alpha1.ConditionProgressing.String(),
		Status:  metav1.ConditionFalse,
		Reason:  k8upv1alpha1.ReasonFinished.String(),
		Message: fmt.Sprintf("the Job '%s/%s' ended", namespace, name),
	})

	c.Obj.SetStatus(status)
	err := c.Client.Status().Update(c.CTX, c.Obj.GetRuntimeObject().(client.Object))
	if err != nil {
		if errors.IsNotFound(err) {
			return
		}
		c.Log.Error(err, "could not patch status")
	}
}
