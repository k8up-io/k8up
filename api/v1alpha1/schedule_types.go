package v1alpha1

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/vshn/k8up/api/v1alpha1/backend"
)

type (
	// +kubebuilder:object:root=true
	// +kubebuilder:subresource:status

	// Schedule is the Schema for the schedules API
	Schedule struct {
		metav1.TypeMeta   `json:",inline"`
		metav1.ObjectMeta `json:"metadata,omitempty"`

		Spec   ScheduleSpec   `json:"spec,omitempty"`
		Status ScheduleStatus `json:"status,omitempty"`
	}

	// +kubebuilder:object:root=true

	// ScheduleList contains a list of Schedule
	ScheduleList struct {
		metav1.TypeMeta `json:",inline"`
		metav1.ListMeta `json:"metadata,omitempty"`
		Items           []Schedule `json:"items"`
	}

	// ScheduleSpec defines the schedules for the various job types.
	ScheduleSpec struct {
		Restore  *RestoreSchedule `json:"restore,omitempty"`
		Backup   *BackupSchedule  `json:"backup,omitempty"`
		Archive  *ArchiveSchedule `json:"archive,omitempty"`
		Check    *CheckSchedule   `json:"check,omitempty"`
		Prune    *PruneSchedule   `json:"prune,omitempty"`
		Backend  *backend.Backend `json:"backend,omitempty"`
		KeepJobs *int             `json:"keepJobs,omitempty"`
		// ResourceRequirementsTemplate describes the compute resource requirements (cpu, memory, etc.)
		ResourceRequirementsTemplate corev1.ResourceRequirements `json:"resourceRequirementsTemplate,omitempty"`
	}

	// ScheduleDefinition is the actual cron-type expression that defines the interval of the actions.
	ScheduleDefinition string

	// ScheduleCommon contains fields every schedule needs
	ScheduleCommon struct {
		Schedule              ScheduleDefinition `json:"schedule,omitempty"`
		ConcurrentRunsAllowed bool               `json:"concurrentRunsAllowed,omitempty"`
	}
	// RestoreSchedule manages schedules for the restore service
	RestoreSchedule struct {
		RestoreSpec     `json:",inline"`
		*ScheduleCommon `json:",inline"`
	}
	// BackupSchedule manages schedules for the backup service
	BackupSchedule struct {
		BackupSpec      `json:",inline"`
		*ScheduleCommon `json:",inline"`
	}
	// ArchiveSchedule manages schedules for the archival service
	ArchiveSchedule struct {
		ArchiveSpec     `json:",inline"`
		*ScheduleCommon `json:",inline"`
	}
	// CheckSchedule manages the schedules for the checks
	CheckSchedule struct {
		CheckSpec       `json:",inline"`
		*ScheduleCommon `json:",inline"`
	}
	// PruneSchedule manages the schedules for the prunes
	PruneSchedule struct {
		PruneSpec       `json:",inline"`
		*ScheduleCommon `json:",inline"`
	}
	// ScheduleStatus defines the observed state of Schedule
	ScheduleStatus struct {
		// EffectiveSchedules displays the final schedule for each type (useful when using smart schedules).
		EffectiveSchedules map[JobType]ScheduleDefinition `json:"effectiveSchedules,omitempty"`

		// Conditions provide a standard mechanism for higher-level status reporting from a controller.
		// They are an extension mechanism which allows tools and other controllers to collect summary information about
		// resources without needing to understand resource-specific status details.
		Conditions []metav1.Condition `json:"conditions,omitempty"`
	}
)

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

// GetStatus retrieves the Status property
func (s *Schedule) GetStatus() Status {
	return Status{Conditions: s.Status.Conditions}
}

// SetStatus sets the Status.Conditions property
func (s *Schedule) SetStatus(status Status) {
	s.Status.Conditions = status.Conditions
}

func (s *Schedule) GetResources() corev1.ResourceRequirements {
	return s.Spec.ResourceRequirementsTemplate
}

// String casts the value to string.
// "aScheduleDefinition.String()" and "string(aScheduleDefinition)" are equivalent.
func (s ScheduleDefinition) String() string {
	return string(s)
}

// IsNonStandard returns true if the value begins with "@",
// indicating a special definition.
// Two examples are '@daily' and '@daily-random'.
func (s ScheduleDefinition) IsNonStandard() bool {
	return strings.HasPrefix(string(s), "@")
}

// IsRandom is true if the value is a special definition (as indicated by IsNonStandard)
// and if it ends with '-random'.
// Two examples are '@daily-random' and '@weekly-random'.
func (s ScheduleDefinition) IsRandom() bool {
	return s.IsNonStandard() && strings.HasSuffix(string(s), "-random")
}

// IsEqualTo returns true if other has the same non-standard smart schedule as s.
func (s ScheduleDefinition) IsEqualTo(other ScheduleDefinition) bool {
	if !s.IsRandom() {
		return false
	}
	return strings.EqualFold(s.String(), other.String())
}
