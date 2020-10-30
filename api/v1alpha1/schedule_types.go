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
	"k8s.io/apimachinery/pkg/runtime"
)

// ScheduleSpec defines the desired state of Schedule
type ScheduleSpec struct {
	Restore  *RestoreSchedule `json:"restore,omitempty"`
	Backup   *BackupSchedule  `json:"backup,omitempty"`
	Archive  *ArchiveSchedule `json:"archive,omitempty"`
	Check    *CheckSchedule   `json:"check,omitempty"`
	Prune    *PruneSchedule   `json:"prune,omitempty"`
	Backend  *Backend         `json:"backend,omitempty"`
	KeepJobs *int             `json:"keepJobs,omitempty"`
}

// ScheduleCommon contains fields every schedule needs
type ScheduleCommon struct {
	Schedule              string `json:"schedule,omitempty"`
	ConcurrentRunsAllowed bool   `json:"concurrentRunsAllowed,omitempty"`
}

// RestoreSchedule manages schedules for the restore service
type RestoreSchedule struct {
	RestoreSpec     `json:",inline"`
	*ScheduleCommon `json:",inline"`
}

// BackupSchedule manages schedules for the backup service
type BackupSchedule struct {
	BackupSpec      `json:",inline"`
	*ScheduleCommon `json:",inline"`
}

// ArchiveSchedule manages schedules for the archival service
type ArchiveSchedule struct {
	ArchiveSpec     `json:",inline"`
	*ScheduleCommon `json:",inline"`
}

// CheckSchedule manages the schedules for the checks
type CheckSchedule struct {
	CheckSpec       `json:",inline"`
	*ScheduleCommon `json:",inline"`
}

type PruneSchedule struct {
	PruneSpec       `json:",inline"`
	*ScheduleCommon `json:",inline"`
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

func (s *Schedule) GetRuntimeObject() runtime.Object {
	return s
}

func (s *Schedule) GetMetaObject() metav1.Object {
	return s
}

func (*Schedule) GetType() string {
	return "schedule"
}

func (s *Schedule) GetK8upStatus() *K8upStatus {
	return nil
}
