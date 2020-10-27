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

type BackupSpec struct {
	Test           string `json:"test,omitempty"`
	BackupTemplate `json:",inline,omitempty"`
}

type BackupTemplate struct {
	Tags    *[]string `json:"tags,omitempty"`
	Backend Backend   `json:"backend,omitempty"`
	Env     Env       `json:"env,omitempty"`
}

type Env struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

type BackupStatus struct {
	K8upStatus `json:",inline"`
}

// K8upStatus defines the observed state of a generic K8up job
type K8upStatus struct {
	Started   bool   `json:"started,omitempty"`
	Finished  bool   `json:"finished,omitempty"`
	JobName   string `json:"jobName,omitempty"`
	Exclusive bool   `json:"exclusive,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Backup is the Schema for the backups API
type Backup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupSpec   `json:"spec,omitempty"`
	Status BackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BackupList contains a list of Backup
type BackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Backup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Backup{}, &BackupList{})
}

func (b *Backup) GetRuntimeObject() runtime.Object {
	return b
}

func (b *Backup) GetMetaObject() metav1.Object {
	return b
}

func (*Backup) GetType() string {
	return "backup"
}

func (b *Backup) GetK8upStatus() *K8upStatus {
	return &b.Status.K8upStatus
}
