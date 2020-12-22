package v1alpha1

import (
	// requires k8s 1.19+: metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/operator-framework/operator-lib/status" // to be replaces with `metav1` from above
)

// Status defines the observed state of a generic K8up job. It is used for the
// operator to determine what to do.
type Status struct {
	Started   bool `json:"started,omitempty"`
	Finished  bool `json:"finished,omitempty"`
	Exclusive bool `json:"exclusive,omitempty"`

	Conditions status.Conditions `json:"conditions,omitempty"`
	// requires K8s 1.19+: Conditions metav1.Conditions `json:"conditions,omitempty"`
}
