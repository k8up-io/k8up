package handler

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/imdario/mergo"
	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/cfg"
	"github.com/vshn/k8up/job"
	"github.com/vshn/k8up/scheduler"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"math/big"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	scheduleFinalizerName  = "k8up.syn.tools/schedule"
	ScheduleHourlyRandom   = "@hourly-random"
	ScheduleDailyRandom    = "@daily-random"
	ScheduleYearlyRandom   = "@yearly-random"
	ScheduleAnnuallyRandom = "@annually-random"
	ScheduleMonthlyRandom  = "@monthly-random"
	ScheduleWeeklyRandom   = "@weekly-random"
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
			Schedule: s.getOrGenerateSchedule(k8upv1alpha1.ArchiveType, s.schedule.Spec.Archive.Schedule),
			Object:   s.schedule.Spec.Archive.ArchiveSpec,
		})
	}
	if s.schedule.Spec.Backup != nil {
		s.schedule.Spec.Backup.BackupSpec.Resources = s.mergeResourcesWithDefaults(s.schedule.Spec.Backup.Resources)
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			Type:     scheduler.BackupType,
			Schedule: s.getOrGenerateSchedule(k8upv1alpha1.BackupType, s.schedule.Spec.Backup.Schedule),
			Object:   s.schedule.Spec.Backup.BackupSpec,
		})
	}
	if s.schedule.Spec.Check != nil {
		s.schedule.Spec.Check.CheckSpec.Resources = s.mergeResourcesWithDefaults(s.schedule.Spec.Check.Resources)
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			Type:     scheduler.CheckType,
			Schedule: s.getOrGenerateSchedule(k8upv1alpha1.CheckType, s.schedule.Spec.Check.Schedule),
			Object:   s.schedule.Spec.Check.CheckSpec,
		})
	}
	if s.schedule.Spec.Restore != nil {
		s.schedule.Spec.Restore.RestoreSpec.Resources = s.mergeResourcesWithDefaults(s.schedule.Spec.Restore.Resources)
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			Type:     scheduler.RestoreType,
			Schedule: s.getOrGenerateSchedule(k8upv1alpha1.RestoreType, s.schedule.Spec.Restore.Schedule),
			Object:   s.schedule.Spec.Restore.RestoreSpec,
		})
	}
	if s.schedule.Spec.Prune != nil {
		s.schedule.Spec.Prune.PruneSpec.Resources = s.mergeResourcesWithDefaults(s.schedule.Spec.Prune.Resources)
		jobList.Jobs = append(jobList.Jobs, scheduler.Job{
			Type:     scheduler.PruneType,
			Schedule: s.getOrGenerateSchedule(k8upv1alpha1.PruneType, s.schedule.Spec.Prune.Schedule),
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

func (s *ScheduleHandler) getOrGenerateSchedule(jobType k8upv1alpha1.JobType, originalSchedule string) string {
	if s.schedule.Status.EffectiveSchedules == nil {
		s.schedule.Status.EffectiveSchedules = make(map[k8upv1alpha1.JobType]string)
	}
	if schedule, found := s.schedule.Status.EffectiveSchedules[jobType]; found {
		return schedule
	}
	newSchedule := s.randomizeSchedule(s.schedule.Namespace+"/"+s.schedule.Name+"@"+string(jobType), originalSchedule)
	s.schedule.Status.EffectiveSchedules[jobType] = newSchedule
	s.requireStatusUpdate = true
	return newSchedule
}

// randomizeSchedule randomizes the given originalSchedule with a seed. The originalSchedule has to be one of the supported
// '@x-random' predefined schedules, otherwise it returns originalSchedule unmodified.
func (s *ScheduleHandler) randomizeSchedule(seed, originalSchedule string) (schedule string) {
	hash := sha1.New()
	hash.Write([]byte(seed))
	sum := hex.EncodeToString(hash.Sum(nil))
	sumBase16, _ := new(big.Int).SetString(sum, 16)
	defer s.Log.V(1).Info("Randomized schedule", "seed", seed, "sum", sum, "from_schedule", originalSchedule, "new_schedule", &schedule)

	minute := new(big.Int).Set(sumBase16).Mod(sumBase16, big.NewInt(60))
	hour := new(big.Int).Set(sumBase16).Mod(sumBase16, big.NewInt(24))
	// A month can have between 27 and 31 days. Cron does not automatically schedule at the end of the month if 31 in February.
	// To not cause a skip of a scheduled backup that could potentially raise alerts, we cap the day-of-month to 27 so it fits in all months.
	dayOfMonth := new(big.Int).Set(sumBase16).Mod(sumBase16, big.NewInt(27))
	month := new(big.Int).Set(sumBase16).Mod(sumBase16, big.NewInt(12))
	weekday := new(big.Int).Set(sumBase16).Mod(sumBase16, big.NewInt(6))
	switch originalSchedule {
	case ScheduleHourlyRandom:
		return fmt.Sprintf("%d * * * *", minute)
	case ScheduleDailyRandom:
		return fmt.Sprintf("%d %d * * *", minute, hour)
	case ScheduleMonthlyRandom:
		return fmt.Sprintf("%d %d %d * *", minute, hour, dayOfMonth.Add(dayOfMonth, big.NewInt(1)))
	case ScheduleAnnuallyRandom:
	case ScheduleYearlyRandom:
		return fmt.Sprintf("%d %d %d %d *", minute, hour, dayOfMonth.Add(dayOfMonth, big.NewInt(1)), month.Add(month, big.NewInt(1)))
	case ScheduleWeeklyRandom:
		return fmt.Sprintf("%d %d * * %d", minute, hour, weekday)
	}
	return originalSchedule
}
