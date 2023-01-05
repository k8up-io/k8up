package schedulecontroller

import (
	"fmt"

	"github.com/imdario/mergo"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
	"github.com/k8up-io/k8up/v2/operator/scheduler"
)

// ScheduleHandler handles the reconciles for the schedules. Schedules are a special
// type of k8up objects as they will only trigger jobs indirectly.
type ScheduleHandler struct {
	schedule *k8upv1.Schedule
	job.Config
}

// NewScheduleHandler will return a new ScheduleHandler.
func NewScheduleHandler(config job.Config, schedule *k8upv1.Schedule) *ScheduleHandler {

	return &ScheduleHandler{
		schedule: schedule,
		Config:   config,
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
		s.SetConditionFalseWithMessage(s.CTX, k8upv1.ConditionReady, k8upv1.ReasonFailed, "cannot add to cron: %v", err.Error())
		return err
	}

	if err := s.Client.Status().Update(s.CTX, s.schedule); err != nil {
		// Update effective schedules.
		return err
	}

	s.SetConditionTrue(s.CTX, k8upv1.ConditionReady, k8upv1.ReasonReady)

	if controllerutil.ContainsFinalizer(s.schedule, k8upv1.LegacyScheduleFinalizerName) {
		controllerutil.AddFinalizer(s.schedule, k8upv1.ScheduleFinalizerName)
		controllerutil.RemoveFinalizer(s.schedule, k8upv1.LegacyScheduleFinalizerName)
		return s.updateSchedule()
	}

	if !controllerutil.ContainsFinalizer(s.schedule, k8upv1.ScheduleFinalizerName) {
		controllerutil.AddFinalizer(s.schedule, k8upv1.ScheduleFinalizerName)
		return s.updateSchedule()
	}
	return nil
}

func (s *ScheduleHandler) createJobList() scheduler.JobList {
	jobList := scheduler.JobList{
		Config: s.Config,
		Jobs:   make([]scheduler.Job, 0),
	}

	for jobType, jb := range map[k8upv1.JobType]k8upv1.ScheduleSpecInterface{
		k8upv1.PruneType:   s.schedule.Spec.Prune,
		k8upv1.BackupType:  s.schedule.Spec.Backup,
		k8upv1.CheckType:   s.schedule.Spec.Check,
		k8upv1.RestoreType: s.schedule.Spec.Restore,
		k8upv1.ArchiveType: s.schedule.Spec.Archive,
	} {
		if k8upv1.IsNil(jb) {
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

func (s *ScheduleHandler) mergeWithDefaults(specInstance *k8upv1.RunnableSpec) {
	s.mergeResourcesWithDefaults(specInstance)
	s.mergeBackendWithDefaults(specInstance)
	s.mergeSecurityContextWithDefaults(specInstance)
}

func (s *ScheduleHandler) mergeResourcesWithDefaults(specInstance *k8upv1.RunnableSpec) {
	resources := &specInstance.Resources

	if err := mergo.Merge(resources, s.schedule.Spec.ResourceRequirementsTemplate); err != nil {
		s.Log.Info("could not merge specific resources with schedule defaults", "err", err.Error(), "schedule", s.Obj.GetName(), "namespace", s.Obj.GetNamespace())
	}
	if err := mergo.Merge(resources, cfg.Config.GetGlobalDefaultResources()); err != nil {
		s.Log.Info("could not merge specific resources with global defaults", "err", err.Error(), "schedule", s.Obj.GetName(), "namespace", s.Obj.GetNamespace())
	}
}

func (s *ScheduleHandler) mergeBackendWithDefaults(specInstance *k8upv1.RunnableSpec) {
	if specInstance.Backend == nil {
		specInstance.Backend = s.schedule.Spec.Backend.DeepCopy()
		return
	}

	if err := mergo.Merge(specInstance.Backend, s.schedule.Spec.Backend); err != nil {
		s.Log.Info("could not merge the schedule's backend with the resource's backend", "err", err.Error(), "schedule", s.Obj.GetName(), "namespace", s.Obj.GetNamespace())
	}
}

func (s *ScheduleHandler) mergeSecurityContextWithDefaults(specInstance *k8upv1.RunnableSpec) {
	if specInstance.PodSecurityContext == nil {
		specInstance.PodSecurityContext = s.schedule.Spec.PodSecurityContext.DeepCopy()
		return
	}
	if s.schedule.Spec.PodSecurityContext == nil {
		return
	}

	if err := mergo.Merge(specInstance.PodSecurityContext, s.schedule.Spec.PodSecurityContext); err != nil {
		s.Log.Info("could not merge the schedule's security context with the resource's security context", "err", err.Error(), "schedule", s.Obj.GetName(), "namespace", s.Obj.GetNamespace())
	}
}

func (s *ScheduleHandler) updateSchedule() error {
	if err := s.Client.Update(s.CTX, s.schedule); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error updating resource %s/%s: %w", s.schedule.Namespace, s.schedule.Name, err)
	}
	return nil
}

func (s *ScheduleHandler) createRandomSchedule(jobType k8upv1.JobType, originalSchedule k8upv1.ScheduleDefinition) (k8upv1.ScheduleDefinition, error) {
	seed := createSeed(s.schedule, jobType)
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
	namespacedName := k8upv1.MapToNamespacedName(s.schedule)
	controllerutil.RemoveFinalizer(s.schedule, k8upv1.ScheduleFinalizerName)
	scheduler.GetScheduler().RemoveSchedules(namespacedName)
	return s.updateSchedule()
}
