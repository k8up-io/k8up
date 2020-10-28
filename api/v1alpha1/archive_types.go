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

// ArchiveSpec defines the desired state of Archive
type ArchiveSpec struct {
	*RestoreSpec `json:",inline"`
}

// ArchiveStatus defines the observed state of Archive
type ArchiveStatus struct {
	K8upStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Archive is the Schema for the archives API
type Archive struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ArchiveSpec   `json:"spec,omitempty"`
	Status ArchiveStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ArchiveList contains a list of Archive
type ArchiveList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Archive `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Archive{}, &ArchiveList{})
}

func (a ArchiveSpec) CreateObject(name, namespace string) runtime.Object {
	return &Archive{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: a,
	}
}

func (a *Archive) GetRuntimeObject() runtime.Object {
	return a
}

func (a *Archive) GetMetaObject() metav1.Object {
	return a
}

func (*Archive) GetType() string {
	return "archive"
}

func (a *Archive) GetK8upStatus() *K8upStatus {
	return &a.Status.K8upStatus
}
