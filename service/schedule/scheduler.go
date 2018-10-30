package schedule

import (
	"fmt"
	"sync"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	"git.vshn.net/vshn/baas/service"
	"git.vshn.net/vshn/baas/service/observe"
	"k8s.io/apimachinery/pkg/runtime"
)

// Schedule holds the state of the schedule handler. It implements the ServiceHandler interface.
type Schedule struct {
	service.CommonObjects
	schedules sync.Map
	observer  *observe.Observer
}

// NewSchedule returns a new scheduler handler.
func NewSchedule(common service.CommonObjects, observer *observe.Observer) *Schedule {
	return &Schedule{
		CommonObjects: common,
		schedules:     sync.Map{},
		observer:      observer,
	}
}

// Ensure is part of the ServiceHandler interface
func (s *Schedule) Ensure(obj runtime.Object) error {
	schedule, err := checkObject(obj)
	if err != nil {
		return err
	}

	name := fmt.Sprintf("%v/%v", schedule.Namespace, schedule.Name)

	tmpSch, ok := s.schedules.Load(name)
	var sch service.Runner

	// We are already running.
	if ok {
		sch = tmpSch.(service.Runner)
		// If not the same spec means options have changed, so we don't longer need this schedule.
		if !sch.SameSpec(schedule) {
			s.Logger.Infof("spec of %s changed, recreating schedule", schedule.Name)
			if err := s.Delete(name); err != nil {
				return err
			}
		} else { // We are ok, nothing changed.
			return nil
		}
	}

	scheduleCopy := schedule.DeepCopy()

	newScheduler := newScheduleRunner(scheduleCopy, &s.CommonObjects)
	s.schedules.Store(name, newScheduler)
	return newScheduler.Start()
}

// Delete is part of the ServiceHandler interface.
func (s *Schedule) Delete(name string) error {
	schedule, ok := s.schedules.Load(name)
	if !ok {
		return fmt.Errorf("%v is not a schedule", name)
	}

	pk := schedule.(service.Runner)
	if err := pk.Stop(); err != nil {
		return err
	}

	s.schedules.Delete(name)
	return nil
}

func checkObject(obj runtime.Object) (*backupv1alpha1.Schedule, error) {
	schedule, ok := obj.(*backupv1alpha1.Schedule)
	if !ok {
		return nil, fmt.Errorf("%v is not a schedule", obj.GetObjectKind())
	}
	return schedule, nil
}
