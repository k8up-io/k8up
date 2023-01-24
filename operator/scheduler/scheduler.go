package scheduler

import (
	"context"
	"fmt"
	"sync"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/robfig/cron/v3"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

type (
	// Scheduler handles all the schedules.
	Scheduler struct {
		cron      *cron.Cron
		schedules sync.Map
	}
	scheduleRef struct {
		EntryID  cron.EntryID
		Schedule k8upv1.ScheduleDefinition
		Runnable func(ctx context.Context)
	}
)

var scheduler = newScheduler()

// GetScheduler returns the scheduler singleton instance.
func GetScheduler() *Scheduler {
	return scheduler
}

func newScheduler() *Scheduler {
	s := &Scheduler{
		cron: cron.New(),
	}
	s.cron.Start()
	return s
}

func (s *Scheduler) SetSchedule(ctx context.Context, key string, schedule k8upv1.ScheduleDefinition, fn func(ctx context.Context)) error {
	existingRaw, exists := s.schedules.Load(key)
	if exists {
		s.cron.Remove(existingRaw.(*scheduleRef).EntryID)
	}
	id, err := s.cron.AddFunc(schedule.String(), func() {
		runCtx := context.Background()
		log := controllerruntime.LoggerFrom(runCtx).WithName("scheduler")
		log.Info("Running schedule", "cron", schedule.String(), "key", key)
		fn(runCtx)
	})
	if err != nil {
		return fmt.Errorf("cannot set schedule: %w", err)
	}
	newRef := &scheduleRef{
		EntryID:  id,
		Schedule: schedule,
		Runnable: fn,
	}
	s.schedules.Store(key, newRef)
	log := controllerruntime.LoggerFrom(ctx)
	log.V(1).Info("Set schedule", "cron", newRef.Schedule, "key", key)
	return nil
}

func (s *Scheduler) RemoveSchedule(ctx context.Context, key string) {
	raw, loaded := s.schedules.LoadAndDelete(key)
	if !loaded {
		return
	}
	ref := raw.(*scheduleRef)
	log := controllerruntime.LoggerFrom(ctx)
	s.cron.Remove(ref.EntryID)
	log.V(1).Info("Removed schedule", "cron", ref.Schedule, "key", key)
}

func (s *Scheduler) HasSchedule(key string) bool {
	_, loaded := s.schedules.Load(key)
	return loaded
}
