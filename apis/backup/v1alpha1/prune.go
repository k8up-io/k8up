package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Prune struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              *PruneSpec  `json:"spec,omitempty"`
	Status            PruneStatus `json:"status,omitempty"`
}

// ArchiveSpec specifies how the prune CRD looks like
// currently this is a simple wrapper for the RestoreSpec
// but this might get extended in the future.
type PruneSpec struct {
	// Retention sets how many backups should be kept after a forget and prune
	Retention RetentionPolicy `json:"retention,omitempty"`
	Backend   *Backend        `json:"backend,omitempty"`
	KeepJobs  int             `json:"keepJobs,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PruneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Prune `json:"items"`
}

type PruneStatus struct {
	JobStatus `json:",inline"`
}

type RetentionPolicy struct {
	KeepLast    int      `json:"keepLast,omitempty"`
	KeepHourly  int      `json:"keepHourly,omitempty"`
	KeepDaily   int      `json:"keepDaily,omitempty"`
	KeepWeekly  int      `json:"keepWeekly,omitempty"`
	KeepMonthly int      `json:"keepMonthly,omitempty"`
	KeepYearly  int      `json:"keepYearly,omitempty"`
	KeepTags    []string `json:"keepTags,omitempty"`
}

func (p PruneList) Len() int      { return len(p.Items) }
func (p PruneList) Swap(i, j int) { p.Items[i], p.Items[j] = p.Items[j], p.Items[i] }

func (p PruneList) Less(i, j int) bool {

	if p.Items[i].CreationTimestamp.Equal(&p.Items[j].CreationTimestamp) {
		return p.Items[i].Name < p.Items[j].Name
	}

	return p.Items[i].CreationTimestamp.Before(&p.Items[j].CreationTimestamp)
}
