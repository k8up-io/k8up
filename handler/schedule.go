package handler

import (
	"fmt"

	"github.com/imdario/mergo"
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

	s.deduplicateJobs(jobList)

	scheduler.GetScheduler().RemoveSchedules(namespacedName)
	err = scheduler.GetScheduler().SyncSchedules(jobList)
	if err != nil {
		s.SetConditionFalseWithMessage(k8upv1alpha1.ConditionReady, k8upv1alpha1.ReasonFailed, "cannot add to cron: %v", err.Error())
		return s.updateStatus()
	}

	s.SetConditionTrue(k8upv1alpha1.ConditionReady, k8upv1alpha1.ReasonReady)

	if !controllerutil.ContainsFinalizer(s.schedule, scheduleFinalizerName) {
		controllerutil.AddFinalizer(s.schedule, scheduleFinalizerName)
		return s.updateSchedule()
	}

	return s.updateStatus()
}

func (s *ScheduleHandler) createJobList() scheduler.JobList {
	jobList := scheduler.JobList{
		Config: s.Config,
		Jobs:   make([]scheduler.Job, 0),
	}

	if archive := s.schedule.Spec.Archive; archive != nil {
		jobTemplate := archive.DeepCopy()
		s.mergeWithDefaults(&jobTemplate.RunnableSpec)
		jobType := k8upv1alpha1.ArchiveType
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			JobType:  jobType,
			Schedule: s.getEffectiveSchedule(jobType, jobTemplate.Schedule),
			Object:   jobTemplate.ArchiveSpec,
		})
	}
	if backup := s.schedule.Spec.Backup; backup != nil {
		backupTemplate := backup.DeepCopy()
		s.mergeWithDefaults(&backupTemplate.RunnableSpec)
		jobType := k8upv1alpha1.BackupType
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			JobType:  jobType,
			Schedule: s.getEffectiveSchedule(jobType, backupTemplate.Schedule),
			Object:   backupTemplate.BackupSpec,
		})
	}
	if check := s.schedule.Spec.Check; check != nil {
		checkTemplate := check.DeepCopy()
		s.mergeWithDefaults(&checkTemplate.RunnableSpec)
		jobType := k8upv1alpha1.CheckType
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			JobType:  jobType,
			Schedule: s.getEffectiveSchedule(jobType, checkTemplate.Schedule),
			Object:   checkTemplate.CheckSpec,
		})
	}
	if restore := s.schedule.Spec.Restore; restore != nil {
		restoreTemplate := restore.DeepCopy()
		s.mergeWithDefaults(&restoreTemplate.RunnableSpec)
		jobType := k8upv1alpha1.RestoreType
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			JobType:  jobType,
			Schedule: s.getEffectiveSchedule(jobType, restoreTemplate.Schedule),
			Object:   restoreTemplate.RestoreSpec,
		})
	}
	if prune := s.schedule.Spec.Prune; prune != nil {
		pruneTemplate := prune.DeepCopy()
		s.mergeWithDefaults(&pruneTemplate.RunnableSpec)
		jobType := k8upv1alpha1.PruneType
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			JobType:  jobType,
			Schedule: s.getEffectiveSchedule(jobType, pruneTemplate.Schedule),
			Object:   pruneTemplate.PruneSpec,
		})
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

func (s *ScheduleHandler) updateStatus() error {
	err := s.Client.Status().Update(s.CTX, s.schedule)
	if err != nil {
		s.Log.Error(err, "Could not update SyncConfig.", "name", s.schedule)
		return err
	}
	s.Log.Info("Updated SyncConfig status.")
	return nil
}

func (s *ScheduleHandler) getEffectiveSchedule(jobType k8upv1alpha1.JobType, originalSchedule k8upv1alpha1.ScheduleDefinition) k8upv1alpha1.ScheduleDefinition {
	if s.schedule.Status.EffectiveSchedules == nil {
		s.schedule.Status.EffectiveSchedules = make(map[k8upv1alpha1.JobType]k8upv1alpha1.ScheduleDefinition)
	}
	if existingSchedule, found := s.schedule.Status.EffectiveSchedules[jobType]; found {
		return existingSchedule
	}

	isStandardOrNotRandom := !originalSchedule.IsNonStandard() || !originalSchedule.IsRandom()
	if isStandardOrNotRandom {
		return originalSchedule
	}

	randomizedSchedule, err := s.getRandomSchedule(jobType, originalSchedule)
	if err != nil {
		s.Log.Info("Could not randomize schedule, continuing with original schedule", "schedule", originalSchedule, "error", err.Error())
		return originalSchedule
	}
	s.setEffectiveSchedule(jobType, randomizedSchedule)
	return randomizedSchedule
}

func (s *ScheduleHandler) getRandomSchedule(jobType k8upv1alpha1.JobType, originalSchedule k8upv1alpha1.ScheduleDefinition) (k8upv1alpha1.ScheduleDefinition, error) {
	seed := s.createSeed(s.schedule, jobType)
	randomizedSchedule, err := randomizeSchedule(seed, originalSchedule)
	if err != nil {
		return originalSchedule, err
	}

	s.Log.V(1).Info("Randomized schedule", "seed", seed, "from_schedule", originalSchedule, "effective_schedule", randomizedSchedule)
	return randomizedSchedule, nil
}

func (s *ScheduleHandler) setEffectiveSchedule(jobType k8upv1alpha1.JobType, schedule k8upv1alpha1.ScheduleDefinition) {
	s.schedule.Status.EffectiveSchedules[jobType] = schedule
	s.requireStatusUpdate = true
}

func (s *ScheduleHandler) needsDeduplication(list scheduler.JobList) bool {
	for _, j := range list.Jobs {
		if j.JobType.IsExclusive() {
			return true
		}
	}
	return false
}

func (s *ScheduleHandler) deduplicateJobs(list scheduler.JobList) scheduler.JobList {
	if !s.needsDeduplication(list) {
		return list
	}
	allSchedules := &k8upv1alpha1.ScheduleList{}
	_ = s.Client.List(s.CTX, allSchedules)

	var deduplicatedList []scheduler.Job



	for _, j := range list.Jobs {

		switch j.JobType {
		case k8upv1alpha1.CheckType:
			allJobs := &k8upv1alpha1.CheckList{}

			err := s.Client.List(s.CTX, allJobs)
			if err != nil {
				s.Log.Error(err, "could not fetch jobs")
				// TODO: add a condition?
				continue
			}
			if s.hasJobSameBackendAsExistingJobs(allJobs) {
				continue
			}
			deduplicatedList = append(deduplicatedList, j)
		default:
			deduplicatedList = append(deduplicatedList, j)
			continue
		}
	}
	list.Jobs = deduplicatedList
	return list
}

func (s *ScheduleHandler) hasJobSameBackendAsExistingJobs(jobs *k8upv1alpha1.CheckList) bool {
	for _, item := range jobs.Items {
		if backend := item.Spec.Backend; backend != nil && backend.IsEqualTo(s.schedule.Spec.Backend) {
			return true
		}
	}
	return false
}

func (s *ScheduleHandler) hasJobSameScheduleAsExistingJobs(schedules k8upv1alpha1.ScheduleList) bool {
	for _, item := range schedules.Items {
		if s.schedule.Name !=
	}
	return false
}
