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
)

// PruneSpec defines the desired state of Prune
type PruneSpec struct {
	// TODO: all the various prune options
	Tags *[]string `json:"tags,omitempty"`
}

// PruneStatus defines the observed state of Prune
type PruneStatus struct {
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
