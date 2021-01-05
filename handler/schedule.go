package handler

import (
	"fmt"
	"strings"

	"github.com/imdario/mergo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/cfg"
	"github.com/vshn/k8up/job"
	"github.com/vshn/k8up/scheduler"
)

const (
	scheduleFinalizerName = "k8up.syn.tools/schedule"
)

// ScheduleHandler handles the reconciles for the schedules. Schedules are a special
// type of k8up objects as they will only trigger jobs indirectly.
type ScheduleHandler struct {
	schedule *k8upv1alpha1.Schedule
	job.Config
	requireSpecUpdate   bool
	requireStatusUpdate bool
}

// NewScheduleHandler will return a new ScheduleHandler.
func NewScheduleHandler(config job.Config, schedule *k8upv1alpha1.Schedule) *ScheduleHandler {
	return &ScheduleHandler{
		schedule: schedule,
		Config:   config,
	}
}

// Handle handles the schedule management. It's responsible for adding and removing the
// jobs from the internal cron library.
func (s *ScheduleHandler) Handle() error {

	namespacedName := types.NamespacedName{Name: s.schedule.GetName(), Namespace: s.schedule.GetNamespace()}

	if s.schedule.GetDeletionTimestamp() != nil {
		controllerutil.RemoveFinalizer(s.schedule, scheduleFinalizerName)
		scheduler.GetScheduler().RemoveSchedules(namespacedName)

		return s.updateSchedule()
	}

	var err error

	jobList := s.createJobList()

	scheduler.GetScheduler().RemoveSchedules(namespacedName)
	err = scheduler.GetScheduler().SyncSchedules(jobList)
	if err != nil {
		return fmt.Errorf("cannot add to cron: %w", err)
	}

	if !controllerutil.ContainsFinalizer(s.schedule, scheduleFinalizerName) {
		controllerutil.AddFinalizer(s.schedule, scheduleFinalizerName)
		s.requireSpecUpdate = true
	}

	if s.requireSpecUpdate {
		return s.updateSchedule()
	}
	if s.requireStatusUpdate {
		return s.updateStatus()
	}
	return nil
}

func (s *ScheduleHandler) createJobList() scheduler.JobList {
	jobList := scheduler.JobList{
		Config: s.Config,
		Jobs:   make([]scheduler.Job, 0),
	}

	if archive := s.schedule.Spec.Archive; archive != nil {
		s.mergeWithDefaults(&archive.SchedulableSpec)
		jobType := k8upv1alpha1.ArchiveType
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			JobType:  jobType,
			Schedule: s.getEffectiveSchedule(jobType, archive.Schedule),
			Object:   archive.ArchiveSpec,
		})
	}
	if backup := s.schedule.Spec.Backup; backup != nil {
		s.mergeWithDefaults(&backup.SchedulableSpec)
		jobType := k8upv1alpha1.BackupType
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			JobType:  jobType,
			Schedule: s.getEffectiveSchedule(k8upv1alpha1.BackupType, backup.Schedule),
			Object:   backup.BackupSpec,
		})
	}
	if check := s.schedule.Spec.Check; check != nil {
		s.mergeWithDefaults(&check.SchedulableSpec)
		jobType := k8upv1alpha1.CheckType
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			JobType:  jobType,
			Schedule: s.getEffectiveSchedule(k8upv1alpha1.CheckType, check.Schedule),
			Object:   check.CheckSpec,
		})
	}
	if restore := s.schedule.Spec.Restore; restore != nil {
		s.mergeWithDefaults(&restore.SchedulableSpec)
		jobType := k8upv1alpha1.RestoreType
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			JobType:  jobType,
			Schedule: s.getEffectiveSchedule(k8upv1alpha1.RestoreType, restore.Schedule),
			Object:   restore.RestoreSpec,
		})
	}
	if prune := s.schedule.Spec.Prune; prune != nil {
		s.mergeWithDefaults(&prune.SchedulableSpec)
		jobType := k8upv1alpha1.PruneType
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			JobType:  jobType,
			Schedule: s.getEffectiveSchedule(k8upv1alpha1.PruneType, prune.Schedule),
			Object:   prune.PruneSpec,
		})
	}

	return jobList
}

func (s *ScheduleHandler) mergeWithDefaults(spec *k8upv1alpha1.SchedulableSpec) {
	s.mergeResourcesWithDefaults(&spec.Resources)

	if spec.Backend == nil {
		spec.Backend = new(k8upv1alpha1.Backend)
	}
	s.mergeBackendWithDefaults(spec.Backend)
}

func (s *ScheduleHandler) mergeResourcesWithDefaults(resources *corev1.ResourceRequirements) {
	if err := mergo.Merge(&s.schedule.Spec.ResourceRequirementsTemplate, cfg.Config.GetGlobalDefaultResources()); err != nil {
		s.Log.Info("could not merge specific resources with global defaults", "err", err.Error(), "schedule", s.Obj.GetMetaObject().GetName(), "namespace", s.Obj.GetMetaObject().GetNamespace())
	}
	if err := mergo.Merge(resources, s.schedule.Spec.ResourceRequirementsTemplate); err != nil {
		s.Log.Info("could not merge specific resources with schedule defaults", "err", err.Error(), "schedule", s.Obj.GetMetaObject().GetName(), "namespace", s.Obj.GetMetaObject().GetNamespace())
	}
}

func (s *ScheduleHandler) mergeBackendWithDefaults(backend *k8upv1alpha1.Backend) {
	if err := mergo.Merge(backend, s.schedule.Spec.Backend); err != nil {
		s.Log.Info("could not merge the schedule's backend with the resource's backend", "err", err.Error(), "schedule", s.Obj.GetMetaObject().GetName(), "namespace", s.Obj.GetMetaObject().GetNamespace())
	}
}

func (s *ScheduleHandler) updateSchedule() error {
	if err := s.Client.Update(s.CTX, s.schedule); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error updating resource %s/%s: %w", s.schedule.Namespace, s.schedule.Name, err)
	}
	return nil
}

func (s *ScheduleHandler) updateStatus() error {
	err := s.Client.Status().Update(s.CTX, s.schedule)
	if err != nil {
		s.Log.Error(err, "Could not update SyncConfig.", "name", s.schedule)
		return err
	}
	s.Log.Info("Updated SyncConfig status.")
	return nil
}

func (s *ScheduleHandler) getEffectiveSchedule(jobType k8upv1alpha1.JobType, originalSchedule string) string {
	if s.schedule.Status.EffectiveSchedules == nil {
		s.schedule.Status.EffectiveSchedules = make(map[k8upv1alpha1.JobType]string)
	}
	if existingSchedule, found := s.schedule.Status.EffectiveSchedules[jobType]; found {
		return existingSchedule
	}
	if !strings.HasSuffix(originalSchedule, "-random") {
		return originalSchedule
	}
	schedule := originalSchedule
	seed := s.createSeed(s.schedule, jobType)
	schedule, err := randomizeSchedule(seed, originalSchedule)
	if err != nil {
		s.Log.Info("Could not randomize schedule, ignoring and try original schedule", "error", err.Error())
	} else {
		s.Log.V(1).Info("Randomized schedule", "seed", seed, "from_schedule", originalSchedule, "effective_schedule", schedule)
	}
	s.schedule.Status.EffectiveSchedules[jobType] = schedule
	s.requireStatusUpdate = true
	return schedule
}
