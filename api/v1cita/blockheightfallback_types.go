package v1cita

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
)

type BlockHeightFallbackSpec struct {
	k8upv1.RunnableSpec `json:",inline"`

	K8upCommon `json:",inline"`

	// Chain
	Chain string `json:"chain,omitempty"`
	// Node
	Node string `json:"node,omitempty"`
	// BlockHeight
	BlockHeight int64 `json:"blockHeight"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=bhf

// BlockHeightFallback is the Schema for the blockheightfallbacks API
type BlockHeightFallback struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BlockHeightFallbackSpec `json:"spec,omitempty"`
	Status k8upv1.Status           `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BlockHeightFallbackList contains a list of BlockHeightFallback
type BlockHeightFallbackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BlockHeightFallback `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BlockHeightFallback{}, &BlockHeightFallbackList{})
}

func (b *BlockHeightFallback) GetRuntimeObject() runtime.Object {
	return b
}

func (b *BlockHeightFallback) GetMetaObject() metav1.Object {
	return b
}

func (*BlockHeightFallback) GetType() k8upv1.JobType {
	return FallbackType
}

// GetJobName returns the name of the underlying batch/v1 job.
func (b *BlockHeightFallback) GetJobName() string {
	return b.GetType().String() + "-" + b.Name
}

// GetStatus retrieves the Status property
func (b *BlockHeightFallback) GetStatus() k8upv1.Status {
	return b.Status
}

// SetStatus sets the Status property
func (b *BlockHeightFallback) SetStatus(status k8upv1.Status) {
	b.Status = status
}

// GetResources returns the resource requirements
func (b *BlockHeightFallback) GetResources() corev1.ResourceRequirements {
	return b.Spec.Resources
}

// GetPodSecurityContext returns the pod security context
func (b *BlockHeightFallback) GetPodSecurityContext() *corev1.PodSecurityContext {
	return b.Spec.PodSecurityContext
}

// GetActiveDeadlineSeconds implements JobObject
func (b *BlockHeightFallback) GetActiveDeadlineSeconds() *int64 {
	return b.Spec.ActiveDeadlineSeconds
}

// GetFailedJobsHistoryLimit returns failed jobs history limit.
// Returns KeepJobs if unspecified.
func (b *BlockHeightFallback) GetFailedJobsHistoryLimit() *int {
	if b.Spec.FailedJobsHistoryLimit != nil {
		return b.Spec.FailedJobsHistoryLimit
	}
	return b.Spec.KeepJobs
}

// GetSuccessfulJobsHistoryLimit returns successful jobs history limit.
// Returns KeepJobs if unspecified.
func (b *BlockHeightFallback) GetSuccessfulJobsHistoryLimit() *int {
	if b.Spec.SuccessfulJobsHistoryLimit != nil {
		return b.Spec.SuccessfulJobsHistoryLimit
	}
	return b.Spec.KeepJobs
}

// GetJobObjects returns a sortable list of jobs
func (b *BlockHeightFallbackList) GetJobObjects() k8upv1.JobObjectList {
	items := make(k8upv1.JobObjectList, len(b.Items))
	for i := range b.Items {
		items[i] = &b.Items[i]
	}
	return items
}

func (b *BlockHeightFallbackSpec) CreateObject(name, namespace string) runtime.Object {
	return &BlockHeightFallback{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: *b,
	}
}
