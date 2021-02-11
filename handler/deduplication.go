package handler

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/cfg"
)

// searchExistingSchedulesForDeduplication lists all EffectiveSchedules and searches for an EffectiveSchedule that has a ScheduleRef which matches the given type and backend.
// If an element is found, it's added to the internal map of EffectiveSchedules.
// It returns true if found along with the generatedSchedule in the spec.
// Otherwise false with an empty string.
func (s *ScheduleHandler) searchExistingSchedulesForDeduplication(jobType k8upv1alpha1.JobType, backendString string) (k8upv1alpha1.ScheduleDefinition, bool) {
	list := &k8upv1alpha1.EffectiveScheduleList{}
	err := s.Client.List(s.CTX, list, client.InNamespace(cfg.Config.OperatorNamespace))
	if err != nil {
		s.Log.Error(err, "could not fetch resources, ignoring deduplication")
		return "", false
	}
	for _, es := range list.Items {
		if es.Spec.JobType == jobType && es.Spec.BackendString == backendString {
			s.Log.Info("deduplicated schedule", "type", jobType, "backend", backendString)
			s.effectiveSchedules[jobType] = es
			return es.Spec.GeneratedSchedule, true
		}
	}
	return "", false
}

// isDeduplicated returns true if any ScheduleRef with any pre-fetched EffectiveSchedule matches the given parameters.
// If it returns true, the given job should not be added to the scheduler.
func (s *ScheduleHandler) isDeduplicated(jobType k8upv1alpha1.JobType, backendString string) bool {
	for _, es := range s.effectiveSchedules {
		for i, ref := range es.Spec.ScheduleRefs {
			if i == 0 {
				// The first entry shouldn't be excluded from the internal scheduler
				continue
			}
			if s.schedule.IsReferencedBy(ref) && es.IsSameType(jobType, backendString) {
				return true
			}
		}
	}
	return false
}
