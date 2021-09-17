package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +k8s:deepcopy-gen=false

// JobObject is an interface that must be implemented by all CRDs that implement a job.
type JobObject interface {
	GetMetaObject() metav1.Object
	GetRuntimeObject() runtime.Object
	GetStatus() Status
	SetStatus(s Status)
	GetType() JobType
	// GetJobName returns the name of the underlying batch/v1 job.
	GetJobName() string
	// GetResources returns the specified resource requirements
	GetResources() corev1.ResourceRequirements
	// GetPodSecurityContext returns the specified pod security context
	GetPodSecurityContext() *corev1.PodSecurityContext
}

// +k8s:deepcopy-gen=false

// JobObjectList is a sortable list of job objects
type JobObjectList []JobObject

func (jo JobObjectList) Len() int      { return len(jo) }
func (jo JobObjectList) Swap(i, j int) { jo[i], jo[j] = jo[j], jo[i] }

func (jo JobObjectList) Less(i, j int) bool {
	if jo[i].GetMetaObject().GetCreationTimestamp().Time.Equal(jo[j].GetMetaObject().GetCreationTimestamp().Time) {
		return jo[i].GetMetaObject().GetName() < jo[j].GetMetaObject().GetName()
	}
	return jo[i].GetMetaObject().GetCreationTimestamp().Time.Before(jo[j].GetMetaObject().GetCreationTimestamp().Time)
}
