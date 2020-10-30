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

// CheckSpec defines the desired state of Check
type CheckSpec struct {
	PromURL  string   `json:"promURL,omitempty"`
	Backend  *Backend `json:"backend,omitempty"`
	KeepJobs *int     `json:"keepJobs,omitempty"`
}

// CheckStatus defines the observed state of Check
type CheckStatus struct {
	K8upStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Check is the Schema for the checks API
type Check struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CheckSpec   `json:"spec,omitempty"`
	Status CheckStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CheckList contains a list of Check
type CheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Check `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Check{}, &CheckList{})
}

func (c *Check) GetRuntimeObject() runtime.Object {
	return c
}

func (c *Check) GetMetaObject() metav1.Object {
	return c
}

func (c *Check) GetType() string {
	return "check"
}

func (c *Check) GetK8upStatus() *K8upStatus {
	return &c.Status.K8upStatus
}

func (c CheckSpec) CreateObject(name, namespace string) runtime.Object {
	return &Check{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: c,
	}
}
