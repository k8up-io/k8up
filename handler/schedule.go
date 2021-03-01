package handler

import (
	"fmt"

	"github.com/imdario/mergo"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/cfg"
	"github.com/vshn/k8up/job"
	"github.com/vshn/k8up/scheduler"
)

// ScheduleHandler handles the reconciles for the schedules. Schedules are a special
// type of k8up objects as they will only trigger jobs indirectly.
type ScheduleHandler struct {
	schedule           *k8upv1alpha1.Schedule
	effectiveSchedules map[k8upv1alpha1.JobType]k8upv1alpha1.EffectiveSchedule
	job.Config
	requireStatusUpdate bool
}

// NewScheduleHandler will return a new ScheduleHandler.
func NewScheduleHandler(
	config job.Config, schedule *k8upv1alpha1.Schedule,
	effectiveSchedules map[k8upv1alpha1.JobType]k8upv1alpha1.EffectiveSchedule) *ScheduleHandler {

	return &ScheduleHandler{
		schedule:           schedule,
		effectiveSchedules: effectiveSchedules,
		Config:             config,
	}
}

// Handle handles the schedule management. It's responsible for adding and removing the
// jobs from the internal cron library.
func (s *ScheduleHandler) Handle() error {

	if s.schedule.GetDeletionTimestamp() != nil {
		return s.finalizeSchedule()
	}

	var err error

	jobList := s.createJobList()

	err = scheduler.GetScheduler().SyncSchedules(jobList)
	if err != nil {
		s.SetConditionFalseWithMessage(k8upv1alpha1.ConditionReady, k8upv1alpha1.ReasonFailed, "cannot add to cron: %v", err.Error())
		return err
	}

	if err := s.synchronizeEffectiveSchedulesResources(); err != nil {
		// at this point, conditions are already set and updated.
		return err
	}

	s.SetConditionTrue(k8upv1alpha1.ConditionReady, k8upv1alpha1.ReasonReady)

	if !controllerutil.ContainsFinalizer(s.schedule, k8upv1alpha1.ScheduleFinalizerName) {
		controllerutil.AddFinalizer(s.schedule, k8upv1alpha1.ScheduleFinalizerName)
		return s.updateSchedule()
	}
	return nil
}

func (s *ScheduleHandler) createJobList() scheduler.JobList {
	jobList := scheduler.JobList{
		Config: s.Config,
		Jobs:   make([]scheduler.Job, 0),
	}

	for jobType, jb := range map[k8upv1alpha1.JobType]k8upv1alpha1.ScheduleSpecInterface{
		k8upv1alpha1.PruneType:   s.schedule.Spec.Prune,
		k8upv1alpha1.BackupType:  s.schedule.Spec.Backup,
		k8upv1alpha1.CheckType:   s.schedule.Spec.Check,
		k8upv1alpha1.RestoreType: s.schedule.Spec.Restore,
		k8upv1alpha1.ArchiveType: s.schedule.Spec.Archive,
	} {
		if k8upv1alpha1.IsNil(jb) {
			s.cleanupEffectiveSchedules(jobType, "")
			continue
		}
		template := jb.GetDeepCopy()
		s.mergeWithDefaults(template.GetRunnableSpec())
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			JobType:  jobType,
			Schedule: s.getEffectiveSchedule(jobType, template.GetSchedule()),
			Object:   template.GetObjectCreator(),
		})
		s.cleanupEffectiveSchedules(jobType, template.GetSchedule())
	}

	return jobList
}

func (s *ScheduleHandler) mergeWithDefaults(specInstance *k8upv1alpha1.RunnableSpec) {
	s.mergeResourcesWithDefaults(specInstance)
	s.mergeBackendWithDefaults(specInstance)
}

func (s *ScheduleHandler) mergeResourcesWithDefaults(specInstance *k8upv1alpha1.RunnableSpec) {
	resources := &specInstance.Resources

	if err := mergo.Merge(resources, s.schedule.Spec.ResourceRequirementsTemplate); err != nil {
		s.Log.Info("could not merge specific resources with schedule defaults", "err", err.Error(), "schedule", s.Obj.GetMetaObject().GetName(), "namespace", s.Obj.GetMetaObject().GetNamespace())
	}
	if err := mergo.Merge(resources, cfg.Config.GetGlobalDefaultResources()); err != nil {
		s.Log.Info("could not merge specific resources with global defaults", "err", err.Error(), "schedule", s.Obj.GetMetaObject().GetName(), "namespace", s.Obj.GetMetaObject().GetNamespace())
	}
}

func (s *ScheduleHandler) mergeBackendWithDefaults(specInstance *k8upv1alpha1.RunnableSpec) {
	if specInstance.Backend == nil {
		specInstance.Backend = s.schedule.Spec.Backend.DeepCopy()
		return
	}

	if err := mergo.Merge(specInstance.Backend, s.schedule.Spec.Backend); err != nil {
		s.Log.Info("could not merge the schedule's backend with the resource's backend", "err", err.Error(), "schedule", s.Obj.GetMetaObject().GetName(), "namespace", s.Obj.GetMetaObject().GetNamespace())
	}
}

func (s *ScheduleHandler) updateSchedule() error {
	if err := s.Client.Update(s.CTX, s.schedule); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error updating resource %s/%s: %w", s.schedule.Namespace, s.schedule.Name, err)
	}
	return nil
}

func (s *ScheduleHandler) createRandomSchedule(jobType k8upv1alpha1.JobType, originalSchedule k8upv1alpha1.ScheduleDefinition) (k8upv1alpha1.ScheduleDefinition, error) {
	seed := s.createSeed(s.schedule, jobType)
	randomizedSchedule, err := randomizeSchedule(seed, originalSchedule)
	if err != nil {
		return originalSchedule, err
	}

	s.Log.V(1).Info("Randomized schedule", "seed", seed, "from_schedule", originalSchedule, "effective_schedule", randomizedSchedule)
	return randomizedSchedule, nil
}

// finalizeSchedule ensures that all associated resources are cleaned up.
// It also removes the schedule definitions from internal scheduler.
func (s *ScheduleHandler) finalizeSchedule() error {
	namespacedName := k8upv1alpha1.MapToNamespacedName(s.schedule)
	controllerutil.RemoveFinalizer(s.schedule, k8upv1alpha1.ScheduleFinalizerName)
	scheduler.GetScheduler().RemoveSchedules(namespacedName)
	for jobType := range s.effectiveSchedules {
		s.cleanupEffectiveSchedules(jobType, "")
	}
	if err := s.synchronizeEffectiveSchedulesResources(); err != nil {
		return err
	}
	return s.updateSchedule()
}
