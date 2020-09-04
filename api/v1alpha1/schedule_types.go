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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ScheduleSpec defines the desired state of Schedule
type ScheduleSpec struct {
	Cron            *string         `json:"cron,omitempty"`
	BackupTemplate  *NamespacedName `json:"backupTemplate,omitempty"`
	ArchiveTemplate *NamespacedName `json:"archiveTemplate,omitempty"`
	CheckTemplate   *NamespacedName `json:"checkTemplate,omitempty"`
	PruneTemplate   *NamespacedName `json:"pruneTemplate,omitempty"`
	KeepJobs        *int            `json:"keepJobs,omitempty"`
}

type NamespacedName struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
}

// String returns the general purpose string representation
func (n NamespacedName) String() string {
	return fmt.Sprintf("%s%s%s", n.Namespace, "/", n.Name)
}

// ScheduleStatus defines the observed state of Schedule
type ScheduleStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Schedule is the Schema for the schedules API
type Schedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScheduleSpec   `json:"spec,omitempty"`
	Status ScheduleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ScheduleList contains a list of Schedule
type ScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Schedule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Schedule{}, &ScheduleList{})
}
