package schedulecontroller

import (
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
)

// getEffectiveSchedule tries to find the actual schedule definition for the given job type and original schedule.
// If originalSchedule is standard or non-standard cron syntax, it returns itself.
// If originalSchedule is a K8up specific smart/random schedule, then it finds the generated schedule in one of the matching EffectiveSchedules.
// If there are none matching, a new EffectiveSchedule is added with originalSchedule translated to a generated schedule.
func (s *ScheduleHandler) getEffectiveSchedule(jobType k8upv1.JobType, originalSchedule k8upv1.ScheduleDefinition) k8upv1.ScheduleDefinition {

	isStandardOrNotRandom := !originalSchedule.IsNonStandard() || !originalSchedule.IsRandom()
	if isStandardOrNotRandom {
		return originalSchedule
	}

	randomizedSchedule, err := s.createRandomSchedule(jobType, originalSchedule)
	if err != nil {
		s.Log.Info("Could not randomize schedule, continuing with original schedule", "schedule", originalSchedule, "error", err.Error())
		return originalSchedule
	}
	s.setEffectiveSchedule(jobType, randomizedSchedule)
	return randomizedSchedule
}

// setEffectiveSchedule will create or update the EffectiveSchedule in the Status for the given jobType with the given schedule definition.
// The EffectiveSchedules aren't persisted or updated in this function, update the Schedule's Status for that.
func (s *ScheduleHandler) setEffectiveSchedule(jobType k8upv1.JobType, schedule k8upv1.ScheduleDefinition) {
	newList := make([]k8upv1.EffectiveSchedule, 0)
	found := false
	for _, effectiveSchedule := range s.schedule.Status.EffectiveSchedules {
		if effectiveSchedule.JobType == jobType {
			effectiveSchedule.GeneratedSchedule = schedule
			found = true
		}
		newList = append(newList, effectiveSchedule)
	}
	if !found {
		newList = append(newList, k8upv1.EffectiveSchedule{JobType: jobType, GeneratedSchedule: schedule})
	}
	s.schedule.Status.EffectiveSchedules = newList
}

// cleanupEffectiveSchedules removes elements in the EffectiveSchedule list that match the job type, but aren't randomized.
// This is needed in case the schedule spec has changed from randomized to standard cron syntax.
// To persist the changes in Kubernetes, call synchronizeEffectiveSchedulesResources().
func (s *ScheduleHandler) cleanupEffectiveSchedules(jobType k8upv1.JobType, newSchedule k8upv1.ScheduleDefinition) {
	newList := make([]k8upv1.EffectiveSchedule, 0)
	for _, effectiveSchedule := range s.schedule.Status.EffectiveSchedules {
		if effectiveSchedule.JobType != jobType {
			newList = append(newList, effectiveSchedule)
			continue
		}
		if newSchedule.IsRandom() {
			newList = append(newList, effectiveSchedule)
			continue
		}
		s.Log.V(1).Info("removing from effective schedule", "type", jobType, "schedule", k8upv1.MapToNamespacedName(s.schedule))
	}
	s.schedule.Status.EffectiveSchedules = newList
}
