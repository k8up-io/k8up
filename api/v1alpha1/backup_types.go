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
	// Backend contains the restic repo where the job should backup to.
	Backend *Backend `json:"backend,omitempty"`
	// KeepJobs amount of jobs to keep for later analysis
	KeepJobs *int `json:"keepJobs,omitempty"`

	// PromURL sets a prometheus push URL where the backup container send metrics to
	// +optional
	PromURL string `json:"promURL,omitempty"`
	// StatsURL sets an arbitrary URL where the wrestic container posts metrics and
	// information about the snapshots to. This is in addition to the prometheus
	// pushgateway.
	StatsURL string `json:"statsURL,omitempty"`
	// Tags is a list of arbitrary tags that get added to the backup via Restic's tagging system
	Tags []string `json:"tags,omitempty"`
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
	Started   bool `json:"started,omitempty"`
	Finished  bool `json:"finished,omitempty"`
	Exclusive bool `json:"exclusive,omitempty"`
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

func (b BackupSpec) CreateObject(name, namespace string) runtime.Object {
	return &Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: b,
	}
}
