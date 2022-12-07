package v1cita

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
)

// SwitchoverSpec defines the desired state of Switchover
// Role conversion for two nodes in the same k8s cluster
type SwitchoverSpec struct {
	k8upv1.RunnableSpec `json:",inline"`

	K8upCommon `json:",inline"`

	// Chain
	Chain string `json:"chain"`
	// SourceNode
	SourceNode string `json:"sourceNode"`
	// DestNode
	DestNode string `json:"destNode"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Switchover is the Schema for the switchovers API
type Switchover struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SwitchoverSpec `json:"spec,omitempty"`
	Status k8upv1.Status  `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SwitchoverList contains a list of Switchover
type SwitchoverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Switchover `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Switchover{}, &SwitchoverList{})
}

func (s *Switchover) GetRuntimeObject() runtime.Object {
	return s
}

func (s *Switchover) GetMetaObject() metav1.Object {
	return s
}

func (*Switchover) GetType() k8upv1.JobType {
	return SwitchoverType
}

// GetJobName returns the name of the underlying batch/v1 job.
func (s *Switchover) GetJobName() string {
	return s.GetType().String() + "-" + s.Name
}

// GetStatus retrieves the Status property
func (s *Switchover) GetStatus() k8upv1.Status {
	return s.Status
}

// SetStatus sets the Status property
func (s *Switchover) SetStatus(status k8upv1.Status) {
	s.Status = status
}

// GetResources returns the resource requirements
func (s *Switchover) GetResources() corev1.ResourceRequirements {
	return s.Spec.Resources
}

// GetPodSecurityContext returns the pod security context
func (s *Switchover) GetPodSecurityContext() *corev1.PodSecurityContext {
	return s.Spec.PodSecurityContext
}

// GetActiveDeadlineSeconds implements JobObject
func (s *Switchover) GetActiveDeadlineSeconds() *int64 {
	return s.Spec.ActiveDeadlineSeconds
}

// GetFailedJobsHistoryLimit returns failed jobs history limit.
// Returns KeepJobs if unspecified.
func (s *Switchover) GetFailedJobsHistoryLimit() *int {
	if s.Spec.FailedJobsHistoryLimit != nil {
		return s.Spec.FailedJobsHistoryLimit
	}
	return s.Spec.KeepJobs
}

// GetSuccessfulJobsHistoryLimit returns successful jobs history limit.
// Returns KeepJobs if unspecified.
func (s *Switchover) GetSuccessfulJobsHistoryLimit() *int {
	if s.Spec.SuccessfulJobsHistoryLimit != nil {
		return s.Spec.SuccessfulJobsHistoryLimit
	}
	return s.Spec.KeepJobs
}

// GetJobObjects returns a sortable list of jobs
func (s *SwitchoverList) GetJobObjects() k8upv1.JobObjectList {
	items := make(k8upv1.JobObjectList, len(s.Items))
	for i := range s.Items {
		items[i] = &s.Items[i]
	}
	return items
}

func (s *SwitchoverSpec) CreateObject(name, namespace string) runtime.Object {
	return &Switchover{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: *s,
	}
}
