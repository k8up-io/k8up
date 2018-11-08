package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Schedule holds schedule information about all schedulable services.
type Schedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              *ScheduleSpec `json:"spec,omitempty"`
}

// ScheduleSpec contains the schedule specifications
type ScheduleSpec struct {
	Restore  *RestoreSchedule `json:"restore,omitempty"`
	Backup   *BackupSchedule  `json:"backup,omitempty"`
	Archive  *ArchiveSchedule `json:"archive,omitempty"`
	Check    *CheckSchedule   `json:"check,omitempty"`
	Prune    *PruneSchedule   `json:"prune,omitempty"`
	Backend  *Backend         `json:"backend,omitempty"`
	KeepJobs int              `json:"keepJobs,omitempty"`
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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ScheduleList is a list of schedule resources
type ScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Schedule `json:"items"`
}

func (s ScheduleList) Len() int      { return len(s.Items) }
func (s ScheduleList) Swap(i, j int) { s.Items[i], s.Items[j] = s.Items[j], s.Items[i] }

func (s ScheduleList) Less(i, j int) bool {

	if s.Items[i].CreationTimestamp.Equal(&s.Items[j].CreationTimestamp) {
		return s.Items[i].Name < s.Items[j].Name
	}

	return s.Items[i].CreationTimestamp.Before(&s.Items[j].CreationTimestamp)
}
