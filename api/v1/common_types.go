package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	}

	// +k8s:deepcopy-gen=false

)

// The job types that k8up deals with
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
	// ConditionPreBackupPodReady is True if Deployments for all Container definitions were created and are ready
	ConditionPreBackupPodReady ConditionType = "PreBackupPodReady"

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
	// ReasonUpdateFailed indicates that a dependent resource could not be created
	ReasonUpdateFailed ConditionReason = "UpdateFailed"
	// ReasonDeletionFailed indicates that a dependent resource could not be deleted
	ReasonDeletionFailed ConditionReason = "DeletionFailed"
	// ReasonRetrievalFailed indicates that dependent resource(s) could not be retrieved for further processing
	ReasonRetrievalFailed ConditionReason = "RetrievalFailed"

	// ReasonNoPreBackupPodsFound is given when no PreBackupPods are found in the same namespace
	ReasonNoPreBackupPodsFound ConditionReason = "NoPreBackupPodsFound"
	// ReasonWaiting is given when PreBackupPods are waiting to be started
	ReasonWaiting ConditionReason = "Waiting"

	// LabelK8upType is the label key that identifies the job type
	LabelK8upType = "k8up.io/type"
	// LabelK8upOwnedBy is a label used to indicated which resource owns this resource to make it easy to fetch owned resources.
	LabelK8upOwnedBy = "k8up.io/owned-by"
	// Deprecated: LegacyLabelK8upType is the former label key that identified the job type
	LegacyLabelK8upType = "k8up.syn.tools/type"
	// LabelManagedBy identifies the tool being used to manage the operation of a resource
	LabelManagedBy = "app.kubernetes.io/managed-by"
	// LabelRepositoryHash is the label key that identifies the Restic repository
	LabelRepositoryHash = "k8up.io/repository-hash"

	// AnnotationK8upHostname is an annotation one can set on RWO PVCs to try to back up them on the specified node.
	AnnotationK8upHostname = "k8up.io/hostname"
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

// MapToNamespacedName translates the given object meta into NamespacedName object
func MapToNamespacedName(obj metav1.Object) types.NamespacedName {
	return types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}
}
