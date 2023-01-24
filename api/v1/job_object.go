package v1

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +k8s:deepcopy-gen=false

// JobObject is an interface that must be implemented by all CRDs that implement a job.
type JobObject interface {
	client.Object
	GetStatus() Status
	SetStatus(s Status)
	GetType() JobType
	// GetResources returns the specified resource requirements
	GetResources() corev1.ResourceRequirements
	// GetPodSecurityContext returns the specified pod security context
	GetPodSecurityContext() *corev1.PodSecurityContext
	// GetActiveDeadlineSeconds returns the specified active deadline seconds timeout.
	GetActiveDeadlineSeconds() *int64
}

// +k8s:deepcopy-gen=false

// JobObjectList is a sortable list of job objects
type JobObjectList []JobObject

func (jo JobObjectList) Len() int      { return len(jo) }
func (jo JobObjectList) Swap(i, j int) { jo[i], jo[j] = jo[j], jo[i] }

func (jo JobObjectList) Less(i, j int) bool {
	if jo[i].GetCreationTimestamp().Time.Equal(jo[j].GetCreationTimestamp().Time) {
		return jo[i].GetName() < jo[j].GetName()
	}
	return jo[i].GetCreationTimestamp().Time.Before(jo[j].GetCreationTimestamp().Time)
}
