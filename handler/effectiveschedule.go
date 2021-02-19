package handler

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/cfg"
)

func (s *ScheduleHandler) newEffectiveSchedule(jobType k8upv1alpha1.JobType) k8upv1alpha1.EffectiveSchedule {
	return k8upv1alpha1.EffectiveSchedule{
		ObjectMeta: v1.ObjectMeta{
			Namespace: cfg.Config.OperatorNamespace,
			Name:      fmt.Sprintf("%s-%s", jobType.String(), rand.String(16)),
			Labels: labels.Set{
				k8upv1alpha1.LabelManagedBy: "k8up",
			},
		},
	}
}

// synchronizeEffectiveSchedulesResources ensures that the effective schedules are created, updated or deleted depending on the Spec.
// If no Schedule references the EffectiveSchedule resource anymore, it will be deleted.
// On errors, the Ready condition will be set to false.
func (s *ScheduleHandler) synchronizeEffectiveSchedulesResources() error {
	newEffectiveSchedules := map[k8upv1alpha1.JobType]k8upv1alpha1.EffectiveSchedule{}
	for jobType, es := range s.effectiveSchedules {
		if len(es.Spec.ScheduleRefs) == 0 {
			if err := s.DeleteResource(&es); err != nil {
				return err
			}
			continue
		}
		if err := s.UpsertResource(&es); err != nil {
			return err
		}
		newEffectiveSchedules[jobType] = es
	}
	s.effectiveSchedules = newEffectiveSchedules
	return nil
}

// getEffectiveSchedule tries to find the actual schedule definition for the given job type and original schedule.
// If originalSchedule is standard or non-standard cron syntax, it returns itself.
// If originalSchedule is a K8up specific smart/random schedule, then it finds the generated schedule in one of the matching EffectiveSchedules.
// If there are none matching, a new EffectiveSchedule is added with originalSchedule translated to a generated schedule.
func (s *ScheduleHandler) getEffectiveSchedule(jobType k8upv1alpha1.JobType, originalSchedule k8upv1alpha1.ScheduleDefinition) (k8upv1alpha1.ScheduleDefinition, bool) {
	isStandardOrNotRandom := !originalSchedule.IsNonStandard() || !originalSchedule.IsRandom()
	if isStandardOrNotRandom {
		return originalSchedule, true
	}

	return s.findExistingSchedule(jobType)
}

// findExistingSchedule searches in the pre-fetched EffectiveSchedules and tries to find a generated schedule definition for the given schedule object and jobType.
// It returns empty string and false if none were found.
func (s *ScheduleHandler) findExistingSchedule(jobType k8upv1alpha1.JobType, originalSchedule k8upv1alpha1.ScheduleDefinition) (k8upv1alpha1.ScheduleDefinition, bool) {
	es, found := s.effectiveSchedules[jobType]
	if found {
		for _, ref := range es.Spec.ScheduleRefs {
			if s.schedule.IsReferencedBy(ref) && es.Spec.OriginalSchedule == originalSchedule {
				s.Log.V(1).Info("using generated schedule",
					"name", k8upv1alpha1.GetNamespacedName(&es).String(),
					"schedule", es.Spec.GeneratedSchedule,
					"type", jobType)
				return es.Spec.GeneratedSchedule, true
			}
		}
	}
	return "", false
}

// upsertEffectiveScheduleInternally will create or update the EffectiveSchedule for the given jobType with the given schedule definition.
// The EffectiveSchedules aren't persisted or updated in this function, use synchronizeEffectiveSchedulesResources() for that.
func (s *ScheduleHandler) upsertEffectiveScheduleInternally(jobType k8upv1alpha1.JobType, schedule k8upv1alpha1.ScheduleDefinition, backendString string) {
	es, found := s.effectiveSchedules[jobType]
	if !found {
		es = s.newEffectiveSchedule(jobType)
	}
	es.Spec.GeneratedSchedule = schedule
	es.Spec.JobType = jobType
	es.Spec.OriginalSchedule = originalSchedule
	es.Spec.AddScheduleRef(k8upv1alpha1.ScheduleRef{
		Name:      s.schedule.Name,
		Namespace: s.schedule.Namespace,
	})
	s.effectiveSchedules[jobType] = es
}

// UpsertResource updates the given object if it exists. If it fails with not existing error, it will be created.
// If both operation failed, the error is logged and Ready condition will be set to False.
func (s *ScheduleHandler) UpsertResource(obj client.Object) error {
	name := k8upv1alpha1.GetNamespacedName(obj).String()
	if updateErr := s.Client.Update(s.CTX, obj); updateErr != nil {
		if errors.IsNotFound(updateErr) {
			if createErr := s.Client.Create(s.CTX, obj); createErr != nil {
				s.Log.Error(updateErr, "could not create resource", "name", name)
				s.SetConditionFalseWithMessage(k8upv1alpha1.ConditionReady, k8upv1alpha1.ReasonCreationFailed,
					"could not create resource '%s': %s", name, updateErr.Error())
				return createErr
			}
			s.Log.Info("created resource", "name", name)
			return nil
		}
		s.Log.Error(updateErr, "could not update resource", "name", name)
		s.SetConditionFalseWithMessage(k8upv1alpha1.ConditionReady, k8upv1alpha1.ReasonUpdateFailed,
			"could not update resource '%s': %s", name, updateErr.Error())
		return updateErr
	}
	s.Log.Info("updated resource", "name", name, "kind", obj.GetObjectKind().GroupVersionKind().Kind)
	return nil
}

// cleanupEffectiveSchedules removes elements in the EffectiveSchedule list that match the job type, but aren't randomized.
// This is needed in case the schedule spec has changed from randomized to standard cron syntax.
// To persist the changes in Kubernetes, call synchronizeEffectiveSchedulesResources().
func (s *ScheduleHandler) cleanupEffectiveSchedules(jobType k8upv1alpha1.JobType, newSchedule k8upv1alpha1.ScheduleDefinition) {
	es, found := s.effectiveSchedules[jobType]
	if !found {
		return
	}
	var schedules []k8upv1alpha1.ScheduleRef
	for _, ref := range es.Spec.ScheduleRefs {
		if s.schedule.IsReferencedBy(ref) && es.Spec.OriginalSchedule != newSchedule {
			s.Log.V(1).Info("removing from effective schedule", "type", jobType, "schedule", k8upv1alpha1.GetNamespacedName(s.schedule))
			continue
		}
		schedules = append(schedules, ref)
	}
	es.Spec.ScheduleRefs = schedules
	s.effectiveSchedules[jobType] = es
}

// DeleteResource will delete the given resource.
// Errors will be logged, and the Ready condition set to False.
func (s *ScheduleHandler) DeleteResource(obj client.Object) error {
	s.Log.Info("deleting resource", "name", k8upv1alpha1.GetNamespacedName(obj), "kind", obj.GetObjectKind().GroupVersionKind().Kind)
	err := s.Client.Delete(s.CTX, obj)
	if err != nil {
		s.Log.Info("could not delete resource", "error", err.Error())
		s.SetConditionFalseWithMessage(k8upv1alpha1.ConditionReady, k8upv1alpha1.ReasonDeletionFailed, "could not delete %s: %s", obj.GetObjectKind().GroupVersionKind().Kind, err.Error())
	}
	return err
}
