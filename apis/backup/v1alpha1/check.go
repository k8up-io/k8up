package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Check defines the check CRD
type Check struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              *CheckSpec  `json:"spec,omitempty"`
	Status            CheckStatus `json:"status,omitempty"`
	GlobalOverrides   *GlobalOverrides
}

// CheckSpec only needs to hold the repository information
// for which the check should run.
type CheckSpec struct {
	PromURL  string   `json:"promURL,omitempty"`
	Backend  *Backend `json:"backend,omitempty"`
	KeepJobs int      `json:"keepJobs,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Check `json:"items,omitempty"`
}

type CheckStatus struct {
	JobStatus `json:",inline"`
}
