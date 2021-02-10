package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

type (
	// JobType defines what job type this is.
	JobType string

	// ConditionType defines what condition type this is.
	ConditionType string

	// ConditionReason is a static/programmatic representation of the cause of a status condition.
	ConditionReason string

	// +k8s:deepcopy-gen=false

	// ScheduleSpecInterface represents a Job for internal use.
	ScheduleSpecInterface interface {
		GetDeepCopy() ScheduleSpecInterface
		GetRunnableSpec() *RunnableSpec
		GetSchedule() ScheduleDefinition
		GetObjectCreator() ObjectCreator
	}

	// +k8s:deepcopy-gen=false

	// ObjectCreator defines an interface that each schedulable Job must implement.
	// The simplest implementation is that the concrete object just returns itself.
	ObjectCreator interface {
		CreateObject(name, namespace string) runtime.Object
	}
)

// The jobs types that k8up deals with
const (
	BackupType   JobType = "backup"
	CheckType    JobType = "check"
	ArchiveType  JobType = "archive"
	RestoreType  JobType = "restore"
	PruneType    JobType = "prune"
	ScheduleType JobType = "schedule"

	// ConditionCompleted is given when the resource has completed its main function.
	ConditionCompleted ConditionType = "Completed"
	// ConditionReady is given when all preconditions are met.
	ConditionReady ConditionType = "Ready"
	// ConditionScrubbed is given when the resource has done its housework to clean up similar but outdated resources.
	ConditionScrubbed ConditionType = "Scrubbed"
	// ConditionProgressing is given when the resource is in the process of doing its main function.
	ConditionProgressing ConditionType = "Progressing"

	// ReasonReady indicates the condition is ready for work
	ReasonReady ConditionReason = "Ready"
	// ReasonStarted indicates the resource has started progressing
	ReasonStarted ConditionReason = "Started"
	// ReasonFinished indicates the resource has finished the work without specifying its success.
	ReasonFinished ConditionReason = "Finished"
	// ReasonSucceeded indicates the condition is succeeded
	ReasonSucceeded ConditionReason = "Succeeded"
	// ReasonFailed indicates there was a general failure not further categorized
	ReasonFailed ConditionReason = "Failed"
	// ReasonCreationFailed indicates that a dependent resource could not be created
	ReasonCreationFailed ConditionReason = "CreationFailed"
	// ReasonCreationFailed indicates that a dependent resource could not be created
	ReasonUpdateFailed ConditionReason = "UpdateFailed"
	// ReasonCreationFailed indicates that a dependent resource could not be deleted
	ReasonDeletionFailed ConditionReason = "DeletionFailed"
	// ReasonRetrievalFailed indicates that dependent resource(s) could not be retrieved for further processing
	ReasonRetrievalFailed ConditionReason = "RetrievalFailed"

	// LabelK8upType is the label key that identifies the job type
	LabelK8upType = "k8up.syn.tools/type"
	// LabelManagedBy identifies the tool being used to manage the operation of a resource
	LabelManagedBy = "app.kubernetes.io/managed-by"
)

// String casts the value to string.
// "aJobType.String()" and "string(aJobType)" are equivalent.
func (j JobType) String() string {
	return string(j)
}

// String casts the value to string.
// "c.String()" and "string(c)" are equivalent.
func (c ConditionType) String() string {
	return string(c)
}

// String casts the value to string.
// "r.String()" and "string(r)" are equivalent.
func (r ConditionReason) String() string {
	return string(r)
}

// GetNamespacedName translates the given object meta into NamespacedName object
func GetNamespacedName(obj metav1.Object) types.NamespacedName {
	return types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}
}

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
