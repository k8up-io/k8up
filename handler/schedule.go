package handler

import (
	"fmt"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/job"
	"github.com/vshn/k8up/scheduler"
	"k8s.io/apimachinery/pkg/types"
)

const (
	scheduleFinalizerName = "k8up.syn.tools/schedule"
)

// ScheduleHandler handles the reconciles for the schedules. Schedules are a special
// type of k8up objects as they will only trigger jobs indirectly.
type ScheduleHandler struct {
	schedule *k8upv1alpha1.Schedule
	job.Config
}

// NewScheduleHandler will return a new ScheduleHanlder.
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
		err := removeFinalizer(s.CTX, s.schedule, scheduleFinalizerName, s.Client)
		if err != nil {
			return fmt.Errorf("error while removing the finalizer: %w", err)
		}

		scheduler.GetScheduler().RemoveSchedules(namespacedName.String())

		return nil
	}

	var err error

	jobList := s.createJobList()

	err = scheduler.GetScheduler().AddSchedules(jobList)
	if err != nil {
		return fmt.Errorf("cannot add to cron: %w", err)
	}

	if !contains(s.schedule.GetFinalizers(), scheduleFinalizerName) {
		err = addFinalizer(s.CTX, s.schedule, scheduleFinalizerName, s.Client)
		if err != nil {
			return fmt.Errorf("error while adding finalizer: %w", err)
		}
	}

	return nil
}

func (s *ScheduleHandler) createJobList() scheduler.JobList {
	jobList := scheduler.JobList{
		Config: s.Config,
		Jobs:   make([]scheduler.Job, 0),
	}

	if s.schedule.Spec.Archive != nil {
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			Type:     scheduler.ArchiveType,
			Schedule: s.schedule.Spec.Archive.Schedule,
			Object:   s.schedule.Spec.Archive.ArchiveSpec,
		})
	}
	if s.schedule.Spec.Backup != nil {
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			Type:     scheduler.BackupType,
			Schedule: s.schedule.Spec.Backup.Schedule,
			Object:   s.schedule.Spec.Backup.BackupSpec,
		})
	}
	if s.schedule.Spec.Check != nil {
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			Type:     scheduler.CheckType,
			Schedule: s.schedule.Spec.Check.Schedule,
			Object:   s.schedule.Spec.Check.CheckSpec,
		})
	}
	if s.schedule.Spec.Restore != nil {
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			Type:     scheduler.RestoreType,
			Schedule: s.schedule.Spec.Restore.Schedule,
			Object:   s.schedule.Spec.Restore.RestoreSpec,
		})
	}

	return jobList
}
