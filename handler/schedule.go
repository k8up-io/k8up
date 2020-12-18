package handler

import (
	"fmt"
	"github.com/imdario/mergo"
	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/cfg"
	"github.com/vshn/k8up/job"
	"github.com/vshn/k8up/scheduler"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
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

	err = scheduler.GetScheduler().AddSchedules(jobList)
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

	if s.schedule.Spec.Archive != nil {
		s.schedule.Spec.Archive.ArchiveSpec.Resources = s.mergeResourcesWithDefaults(s.schedule.Spec.Archive.Resources)
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			Type:     scheduler.ArchiveType,
			Schedule: s.getEffectiveSchedule(k8upv1alpha1.ArchiveType, s.schedule.Spec.Archive.Schedule),
			Object:   s.schedule.Spec.Archive.ArchiveSpec,
		})
	}
	if s.schedule.Spec.Backup != nil {
		s.schedule.Spec.Backup.BackupSpec.Resources = s.mergeResourcesWithDefaults(s.schedule.Spec.Backup.Resources)
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			Type:     scheduler.BackupType,
			Schedule: s.getEffectiveSchedule(k8upv1alpha1.BackupType, s.schedule.Spec.Backup.Schedule),
			Object:   s.schedule.Spec.Backup.BackupSpec,
		})
	}
	if s.schedule.Spec.Check != nil {
		s.schedule.Spec.Check.CheckSpec.Resources = s.mergeResourcesWithDefaults(s.schedule.Spec.Check.Resources)
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			Type:     scheduler.CheckType,
			Schedule: s.getEffectiveSchedule(k8upv1alpha1.CheckType, s.schedule.Spec.Check.Schedule),
			Object:   s.schedule.Spec.Check.CheckSpec,
		})
	}
	if s.schedule.Spec.Restore != nil {
		s.schedule.Spec.Restore.RestoreSpec.Resources = s.mergeResourcesWithDefaults(s.schedule.Spec.Restore.Resources)
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			Type:     scheduler.RestoreType,
			Schedule: s.getEffectiveSchedule(k8upv1alpha1.RestoreType, s.schedule.Spec.Restore.Schedule),
			Object:   s.schedule.Spec.Restore.RestoreSpec,
		})
	}
	if s.schedule.Spec.Prune != nil {
		s.schedule.Spec.Prune.PruneSpec.Resources = s.mergeResourcesWithDefaults(s.schedule.Spec.Prune.Resources)
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			Type:     scheduler.PruneType,
			Schedule: s.getEffectiveSchedule(k8upv1alpha1.PruneType, s.schedule.Spec.Prune.Schedule),
			Object:   s.schedule.Spec.Prune.PruneSpec,
		})
	}

	return jobList
}

func (s *ScheduleHandler) mergeResourcesWithDefaults(resources corev1.ResourceRequirements) corev1.ResourceRequirements {
	if err := mergo.Merge(&s.schedule.Spec.ResourceRequirementsTemplate, cfg.Config.GetGlobalDefaultResources()); err != nil {
		s.Log.Info("could not merge specific resources with global defaults", "err", err.Error())
	}
	if err := mergo.Merge(&resources, s.schedule.Spec.ResourceRequirementsTemplate); err != nil {
		s.Log.Info("could not merge specific resources with schedule defaults", "err", err.Error())
	}
	return resources
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
