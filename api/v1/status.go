package v1

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Status defines the observed state of a generic K8up job. It is used for the
// operator to determine what to do.
type Status struct {
	Started   bool `json:"started,omitempty"`
	Finished  bool `json:"finished,omitempty"`
	Exclusive bool `json:"exclusive,omitempty"`

	// Conditions provide a standard mechanism for higher-level status reporting from a controller.
	// They are an extension mechanism which allows tools and other controllers to collect summary information about
	// resources without needing to understand resource-specific status details.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// HasFailed returns true in the following cases:
//
// * If ConditionCompleted is true with any other reason than ReasonSucceeded.
//
// * If ConditionPreBackupPodReady is false with any of the "failed" reasons.
func (in Status) HasFailed() bool {
	if in.HasFailedPreBackup() {
		return true
	}
	completedCond := meta.FindStatusCondition(in.Conditions, ConditionCompleted.String())
	if completedCond != nil && !matchAnyReason(*completedCond, ReasonSucceeded) {
		return completedCond.Status == metav1.ConditionTrue
	}
	return false
}

// HasSucceeded returns true if all cases are true:
//
// * If ConditionCompleted is true with ReasonSucceeded.
//
// * If ConditionPreBackupPodReady has no failure reason.
func (in Status) HasSucceeded() bool {
	if in.HasFailedPreBackup() {
		return false
	}
	completedCond := meta.FindStatusCondition(in.Conditions, ConditionCompleted.String())
	if completedCond != nil && matchAnyReason(*completedCond, ReasonSucceeded) {
		return completedCond.Status == metav1.ConditionTrue
	}
	return false
}

// HasFinished returns true if either HasFailed() or HasSucceeded() return true.
func (in Status) HasFinished() bool {
	return in.HasFailed() || in.HasSucceeded()
}

// HasFailedPreBackup returns true if ConditionPreBackupPodReady is false with any of the "failed" reasons.
func (in Status) HasFailedPreBackup() bool {
	preBackupCond := meta.FindStatusCondition(in.Conditions, ConditionPreBackupPodReady.String())
	// Failed pre backups also count as finished
	if preBackupCond != nil && isPreBackupFailed(*preBackupCond) {
		return preBackupCond.Status == metav1.ConditionFalse
	}
	return false
}

// HasStarted returns true if ConditionProgressing is true with ReasonStarted, false otherwise.
func (in Status) HasStarted() bool {
	condition := meta.FindStatusCondition(in.Conditions, ConditionProgressing.String())
	if condition != nil && matchAnyReason(*condition, ReasonStarted) {
		return condition.Status == metav1.ConditionTrue
	}
	return false
}

// IsWaitingForPreBackup returns true if the ConditionPreBackupPodReady is Unknown with ReasonWaiting, false otherwise.
func (in Status) IsWaitingForPreBackup() bool {
	condition := meta.FindStatusCondition(in.Conditions, ConditionPreBackupPodReady.String())
	if condition != nil && matchAnyReason(*condition, ReasonWaiting) {
		return condition.Status == metav1.ConditionUnknown
	}
	return false
}

// SetStarted sets ConditionReady to true with ReasonReady and ConditionProgressing with ReasonStarted.
// The given message parameter is set as the message for ConditionReady.
// It also sets the deprecated Started flag to true.
func (in *Status) SetStarted(message string) {
	in.Started = true
	meta.SetStatusCondition(&in.Conditions, metav1.Condition{
		Type:    ConditionReady.String(),
		Status:  metav1.ConditionTrue,
		Reason:  ReasonReady.String(),
		Message: message,
	})
	meta.SetStatusCondition(&in.Conditions, metav1.Condition{
		Type:    ConditionProgressing.String(),
		Status:  metav1.ConditionTrue,
		Reason:  ReasonStarted.String(),
		Message: "The job is progressing",
	})
}

// SetFinished sets ConditionProgressing to false with ReasonFinished.
// It also sets the deprecated Finished flag to true.
func (in *Status) SetFinished(message string) {
	in.Finished = true
	meta.SetStatusCondition(&in.Conditions, metav1.Condition{
		Type:    ConditionProgressing.String(),
		Status:  metav1.ConditionFalse,
		Reason:  ReasonFinished.String(),
		Message: message,
	})
	meta.RemoveStatusCondition(&in.Conditions, ConditionReady.String())
}

// SetFailed sets ConditionCompleted to true with ReasonFailed.
func (in *Status) SetFailed(message string) {
	meta.SetStatusCondition(&in.Conditions, metav1.Condition{
		Type:    ConditionCompleted.String(),
		Status:  metav1.ConditionTrue,
		Reason:  ReasonFailed.String(),
		Message: message,
	})
}

// SetSucceeded sets ConditionCompleted to true with ReasonSucceeded.
func (in *Status) SetSucceeded(message string) {
	meta.SetStatusCondition(&in.Conditions, metav1.Condition{
		Type:    ConditionCompleted.String(),
		Status:  metav1.ConditionTrue,
		Reason:  ReasonSucceeded.String(),
		Message: message,
	})
}

// SetCondition sets a generic condition, overwriting existing one by type if present.
func (in *Status) SetCondition(typ ConditionType, reason ConditionReason, status metav1.ConditionStatus, message string) {
	meta.SetStatusCondition(&in.Conditions, metav1.Condition{
		Type:    typ.String(),
		Status:  status,
		Reason:  reason.String(),
		Message: message,
	})
}

func isPreBackupFailed(condition metav1.Condition) bool {
	return !matchAnyReason(condition,
		ReasonSucceeded,
		ReasonWaiting,
		ReasonNoPreBackupPodsFound,
		ReasonReady)
}

func matchAnyReason(condition metav1.Condition, reasons ...ConditionReason) bool {
	for _, reason := range reasons {
		if condition.Reason == reason.String() {
			return true
		}
	}
	return false
}
