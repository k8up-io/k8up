package job

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1 "github.com/k8up-io/k8up/api/v1"
)

// SetConditionTrue tells the K8s controller at once that the status of the given Conditions is now "True"
func (c *Config) SetConditionTrue(condition k8upv1.ConditionType, reason k8upv1.ConditionReason) {
	c.patchConditions(metav1.ConditionTrue, reason, "The resource is ready", condition)
}

// SetConditionUnknownWithMessage tells the K8s controller at once that the status of the given Conditions is "Unknown"
func (c *Config) SetConditionUnknownWithMessage(condition k8upv1.ConditionType, reason k8upv1.ConditionReason, message string, args ...interface{}) {
	c.patchConditions(metav1.ConditionUnknown, reason, fmt.Sprintf(message, args...), condition)
}

// SetConditionTrueWithMessage tells the K8s controller at once that the status of the given Condition is now "True" and
// provides the given message.
// The arguments `message` and `args` follow the fmt.Sprintf() syntax.
func (c *Config) SetConditionTrueWithMessage(condition k8upv1.ConditionType, reason k8upv1.ConditionReason, message string, args ...interface{}) {
	c.patchConditions(metav1.ConditionTrue, reason, fmt.Sprintf(message, args...), condition)
}

// SetConditionFalseWithMessage tells the K8s controller at once that the status of the given Condition is now "False" and
// provides the given message.
// The arguments `message` and `args` follow the fmt.Sprintf() syntax.
func (c *Config) SetConditionFalseWithMessage(condition k8upv1.ConditionType, reason k8upv1.ConditionReason, message string, args ...interface{}) {
	c.patchConditions(metav1.ConditionFalse, reason, fmt.Sprintf(message, args...), condition)
}

// patchConditions patches the Status object on the K8s controller with the given Conditions
func (c *Config) patchConditions(conditionStatus metav1.ConditionStatus, reason k8upv1.ConditionReason, message string, conditions ...k8upv1.ConditionType) {
	runtimeObject := c.Obj.GetRuntimeObject()
	patch := client.MergeFrom(runtimeObject.DeepCopyObject().(client.Object))

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

// SetStarted marks the job as started and updates the status.
// The arguments `message` and `args` follow the fmt.Sprintf() syntax.
func (c *Config) SetStarted(message string, args ...interface{}) {
	status := c.Obj.GetStatus()
	status.SetStarted(fmt.Sprintf(message, args...))
	c.Obj.SetStatus(status)

	err := c.Client.Status().Update(c.CTX, c.Obj.GetRuntimeObject().(client.Object))
	if err != nil {
		if errors.IsNotFound(err) {
			return
		}
		c.Log.Error(err, "could not patch status")
	}
}

// SetFinished marks the job as finished and updates the status.
func (c *Config) SetFinished(namespace, name string) {
	status := c.Obj.GetStatus()
	status.SetFinished(fmt.Sprintf("the Job '%s/%s' ended", namespace, name))
	c.Obj.SetStatus(status)

	err := c.Client.Status().Update(c.CTX, c.Obj.GetRuntimeObject().(client.Object))
	if err != nil {
		if errors.IsNotFound(err) {
			return
		}
		c.Log.Error(err, "could not patch status")
	}
}

// GroupByStatus groups jobs by the running state
func GroupByStatus(jobs []k8upv1.JobObject) (running []k8upv1.JobObject, failed []k8upv1.JobObject, successful []k8upv1.JobObject) {
	running = make([]k8upv1.JobObject, 0, len(jobs))
	successful = make([]k8upv1.JobObject, 0, len(jobs))
	failed = make([]k8upv1.JobObject, 0, len(jobs))
	for _, job := range jobs {
		if job.GetStatus().HasSucceeded() {
			successful = append(successful, job)
			continue
		}
		if job.GetStatus().HasFailed() {
			failed = append(failed, job)
			continue
		}
		running = append(running, job)
	}
	return
}
