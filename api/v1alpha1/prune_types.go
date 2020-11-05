/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// PruneSpec needs to contain the repository information as well as the desired
// retention policies.
type PruneSpec struct {
	// Retention sets how many backups should be kept after a forget and prune
	Retention RetentionPolicy `json:"retention,omitempty"`
	Backend   *Backend        `json:"backend,omitempty"`
	KeepJobs  *int            `json:"keepJobs,omitempty"`
}

func (p PruneSpec) CreateObject(name, namespace string) runtime.Object {
	return &Prune{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: p,
	}
}

type RetentionPolicy struct {
	KeepLast    int      `json:"keepLast,omitempty"`
	KeepHourly  int      `json:"keepHourly,omitempty"`
	KeepDaily   int      `json:"keepDaily,omitempty"`
	KeepWeekly  int      `json:"keepWeekly,omitempty"`
	KeepMonthly int      `json:"keepMonthly,omitempty"`
	KeepYearly  int      `json:"keepYearly,omitempty"`
	KeepTags    []string `json:"keepTags,omitempty"`
	// Tags is a filter on what tags the policy should be applied
	// DO NOT CONFUSE THIS WITH KeepTags OR YOU'LL have a bad time
	Tags []string `json:"tags,omitempty"`
	// Hosntames is a filter on what hostnames the policy should be applied
	Hostnames []string `json:"hostnames,omitempty"`
}

// PruneStatus defines the observed state of Prune
type PruneStatus struct {
	K8upStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Prune is the Schema for the prunes API
type Prune struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PruneSpec   `json:"spec,omitempty"`
	Status PruneStatus `json:"status,omitempty"`
}

func (p *Prune) GetRuntimeObject() runtime.Object {
	return p
}

func (p *Prune) GetMetaObject() metav1.Object {
	return p
}

func (p *Prune) GetType() string {
	return "prune"
}

func (p *Prune) GetK8upStatus() *K8upStatus {
	return &p.Status.K8upStatus
}

// +kubebuilder:object:root=true

// PruneList contains a list of Prune
type PruneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Prune `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Prune{}, &PruneList{})
}
