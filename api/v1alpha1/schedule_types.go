package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ScheduleSpec defines the schedules for the various job types.
type ScheduleSpec struct {
	Restore  *RestoreSchedule `json:"restore,omitempty"`
	Backup   *BackupSchedule  `json:"backup,omitempty"`
	Archive  *ArchiveSchedule `json:"archive,omitempty"`
	Check    *CheckSchedule   `json:"check,omitempty"`
	Prune    *PruneSchedule   `json:"prune,omitempty"`
	Backend  *Backend         `json:"backend,omitempty"`
	KeepJobs *int             `json:"keepJobs,omitempty"`
	// ResourceRequirementsTemplate describes the compute resource requirements (cpu, memory, etc.)
	ResourceRequirementsTemplate corev1.ResourceRequirements `json:"resourceRequirementsTemplate,omitempty"`
}

// ScheduleCommon contains fields every schedule needs
type ScheduleCommon struct {
	Schedule              string `json:"schedule,omitempty"`
	ConcurrentRunsAllowed bool   `json:"concurrentRunsAllowed,omitempty"`
}

// SchedulableSpec defines the fields that are necessary on types which are schedulable
type SchedulableSpec struct {
	// Backend contains the restic repo where the job should backup to.
	Backend *Backend `json:"backend,omitempty"`

	// Resources describes the compute resource requirements (cpu, memory, etc.)
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
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

// ScheduleStatus defines the observed state of Schedule
type ScheduleStatus struct {
	// EffectiveSchedules displays the final schedule for each type (useful when using smart schedules).
	EffectiveSchedules map[JobType]string `json:"effectiveSchedules,omitempty"`
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

func (*Schedule) GetType() JobType {
	return ScheduleType
}

func (s *Schedule) GetStatus() *Status {
	return nil
}

func (s *Schedule) GetResources() corev1.ResourceRequirements {
	return s.Spec.ResourceRequirementsTemplate
}
