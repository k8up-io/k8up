package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type (
	// +kubebuilder:object:root=true
	// +kubebuilder:printcolumn:name="Schedule Namespace",type="string",JSONPath=`.spec.scheduleRefs[0].namespace`,description="Schedule Namespace"
	// +kubebuilder:printcolumn:name="Schedule Name",type="string",JSONPath=`.spec.scheduleRefs[0].name`,description="Schedule Name"
	// +kubebuilder:printcolumn:name="Generated Schedule",type="string",JSONPath=`.spec.generatedSchedule`,description="Generated Schedule"
	// +kubebuilder:printcolumn:name="Job Type",type="string",JSONPath=`.spec.jobType`,description="Job Type"
	// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

	// EffectiveSchedule is the Schema to persist schedules generated from Randomized schedules.
	EffectiveSchedule struct {
		metav1.TypeMeta   `json:",inline"`
		metav1.ObjectMeta `json:"metadata,omitempty"`

		Spec EffectiveScheduleSpec `json:"spec,omitempty"`
	}

	// +kubebuilder:object:root=true

	// EffectiveScheduleList contains a list of EffectiveSchedule
	EffectiveScheduleList struct {
		metav1.TypeMeta `json:",inline"`
		metav1.ListMeta `json:"metadata,omitempty"`
		Items           []EffectiveSchedule `json:"items"`
	}

	// EffectiveScheduleSpec defines the desired state of EffectiveSchedule
	EffectiveScheduleSpec struct {
		// GeneratedSchedule is the effective schedule that is added to Cron
		GeneratedSchedule ScheduleDefinition `json:"generatedSchedule,omitempty"`
		// OriginalSchedule is the original user-defined schedule definition in the Schedule object.
		OriginalSchedule ScheduleDefinition `json:"originalSchedule,omitempty"`
		// JobType defines to which job type this schedule applies
		JobType JobType `json:"jobType,omitempty"`
		// ScheduleRefs holds a list of schedules for which the generated schedule applies to.
		// The list may omit entries that aren't generated from smart schedules.
		ScheduleRefs []ScheduleRef `json:"scheduleRefs,omitempty"`
	}

	// ScheduleRef represents a reference to a Schedule resource
	ScheduleRef struct {
		Name      string `json:"name,omitempty"`
		Namespace string `json:"namespace,omitempty"`
	}
)

// AddScheduleRef adds the given newRef to the existing ScheduleRefs if not already existing.
func (in *EffectiveScheduleSpec) AddScheduleRef(newRef ScheduleRef) {
	for _, ref := range in.ScheduleRefs {
		if ref.Name == newRef.Name && ref.Namespace == newRef.Namespace {
			return
		}
	}
	in.ScheduleRefs = append(in.ScheduleRefs, newRef)
}

func init() {
	SchemeBuilder.Register(&EffectiveSchedule{}, &EffectiveScheduleList{})
}
