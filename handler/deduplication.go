package handler

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/cfg"
)

type deduplicationContext struct {
	jobType           k8upv1alpha1.JobType
	backendString     string
	originalSchedule  k8upv1alpha1.ScheduleDefinition
	effectiveSchedule k8upv1alpha1.ScheduleDefinition
}

// tryDeduplicateJob will try to deduplicate the job given in the context.
// It returns true if the job is successfully deduplicated, false otherwise.
func (s *ScheduleHandler) tryDeduplicateJob(ctx *deduplicationContext) bool {

	if s.isDeduplicated(ctx) {
		return true
	}

	list, err := s.fetchEffectiveSchedules()
	if err != nil {
		s.Log.V(1).Info("ignoring job for deduplication", "job", ctx.jobType)
		return false
	}

	existingSchedule, found := s.searchExistingSchedulesForDeduplication(list, ctx)
	if found {
		ctx.effectiveSchedule = existingSchedule.Spec.GeneratedSchedule
		s.effectiveSchedules[ctx.jobType] = existingSchedule
	} else {
		s.getOrGenerateEffectiveSchedule(ctx)
	}

	s.upsertEffectiveScheduleInternally(ctx)
	return found
}

// fetchEffectiveSchedules fetches the EffectiveSchedules from the configured operator namespace.
// Logs the error and returns an empty list on errors.
func (s *ScheduleHandler) fetchEffectiveSchedules() ([]k8upv1alpha1.EffectiveSchedule, error) {
	list := &k8upv1alpha1.EffectiveScheduleList{}
	if err := s.Client.List(s.CTX, list, client.InNamespace(cfg.Config.OperatorNamespace)); err != nil {
		s.Log.Error(err, "could not EffectiveSchedules")
		return []k8upv1alpha1.EffectiveSchedule{}, err
	}
	s.Log.V(1).Info("fetched EffectiveSchedules", "count", len(list.Items))
	return list.Items, nil
}

// searchExistingSchedulesForDeduplication searches for an EffectiveSchedule that has a ScheduleRef which matches the given criteria.
// It returns true along with the element matching.
// Otherwise false with an empty object.
func (s *ScheduleHandler) searchExistingSchedulesForDeduplication(esList []k8upv1alpha1.EffectiveSchedule, ctx *deduplicationContext) (k8upv1alpha1.EffectiveSchedule, bool) {
	for _, es := range esList {
		if es.IsSameType(ctx.jobType, ctx.backendString, ctx.originalSchedule) {
			s.Log.Info("deduplicated schedule", "type", ctx.jobType, "backend", ctx.backendString)
			return es, true
		}
	}
	return k8upv1alpha1.EffectiveSchedule{}, false
}

// isDeduplicated returns true if any ScheduleRef with any pre-fetched EffectiveSchedule matches the given parameters.
// If it returns true, the given job should not be added to the scheduler.
func (s *ScheduleHandler) isDeduplicated(ctx *deduplicationContext) bool {
	for _, es := range s.effectiveSchedules {
		for i, ref := range es.Spec.ScheduleRefs {
			if i == 0 {
				// The first entry shouldn't be excluded from the internal scheduler
				continue
			}
			if s.schedule.IsReferencedBy(ref) && es.IsSameType(ctx.jobType, ctx.backendString, ctx.originalSchedule) {
				return true
			}
		}
	}
	return false
}
