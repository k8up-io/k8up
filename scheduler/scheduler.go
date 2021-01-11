package scheduler

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/job"
)

type (
	// ObjectCreator defines an interface that each schedulable newJobs must implement.
	// The simplest implementation is that the concrete object just returns itself.
	ObjectCreator interface {
		CreateObject(name, namespace string) runtime.Object
	}
	// JobList contains a slice of jobs and job.Config to actually apply the
	// the newJobs objects.
	JobList struct {
		Jobs   []Job
		Config job.Config
	}
	// Job contains all necessary information to create a schedule.
	Job struct {
		JobType  k8upv1alpha1.JobType
		Schedule k8upv1alpha1.ScheduleDefinition
		Object   ObjectCreator
	}
	// Scheduler handles all the schedules.
	Scheduler struct {
		cron                *cron.Cron
		registeredSchedules map[string][]scheduleRef
		mutex               sync.Mutex
	}
	scheduleRef struct {
		EntryID  cron.EntryID
		JobType  k8upv1alpha1.JobType
		Schedule k8upv1alpha1.ScheduleDefinition
		Command  func()
	}
)

var (
	scheduler *Scheduler

	scheduleGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "k8up_schedules_gauge",
		Help: "How many schedules this k8up manages",
	}, []string{
		"namespace",
	})
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(scheduleGauge)
}

// GetScheduler returns the scheduler singleton instance.
func GetScheduler() *Scheduler {
	if scheduler == nil {
		scheduler = newScheduler()
		scheduler.cron.Start()
	}

	return scheduler
}

func newScheduler() *Scheduler {
	return &Scheduler{
		cron:                cron.New(),
		registeredSchedules: make(map[string][]scheduleRef),
		mutex:               sync.Mutex{},
	}
}

// SyncSchedules will add the given schedule to the running cron.
func (s *Scheduler) SyncSchedules(jobs JobList) error {
	namespacedName := types.NamespacedName{
		Name:      jobs.Config.Obj.GetMetaObject().GetName(),
		Namespace: jobs.Config.Obj.GetMetaObject().GetNamespace(),
	}

	s.RemoveSchedules(namespacedName)
	return s.addSchedules(jobs, namespacedName)
}

// addSchedules registers all the jobs in the job list in the internal cron scheduler by invoking addSchedule.
func (s *Scheduler) addSchedules(jobs JobList, namespacedName types.NamespacedName) error {
	config := jobs.Config

	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, schedulableJob := range jobs.Jobs {
		config.Log.Info("registering schedule for", "type", schedulableJob.JobType, "schedule", schedulableJob.Schedule)
		err := s.addSchedule(schedulableJob, namespacedName, s.getScheduleCallback(config, namespacedName, schedulableJob))
		if err != nil {
			return err
		}
	}

	s.incRegisteredSchedulesGauge(namespacedName.Namespace)
	return nil
}

func (s *Scheduler) getScheduleCallback(config job.Config, namespacedName types.NamespacedName, schedulableJob Job) func() {
	return func() {
		config.Log.Info("running schedule for", "job", schedulableJob.JobType)
		s.createObject(schedulableJob.JobType, namespacedName.Namespace, schedulableJob.Object, config)
	}
}

// addSchedule adds the given newJobs to the cron scheduler
func (s *Scheduler) addSchedule(jb Job, namespacedName types.NamespacedName, cmd func()) error {
	id, err := s.cron.AddFunc(jb.Schedule.String(), cmd)
	if err != nil {
		return err
	}
	schedules := s.registeredSchedules[namespacedName.String()]
	s.registeredSchedules[namespacedName.String()] = append(schedules, scheduleRef{
		EntryID:  id,
		JobType:  jb.JobType,
		Schedule: jb.Schedule,
		Command:  cmd,
	})
	return nil
}

// RemoveSchedules will remove the schedules of the given types.NamespacedName if existing.
func (s *Scheduler) RemoveSchedules(namespacedName types.NamespacedName) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for _, ref := range s.registeredSchedules[namespacedName.String()] {
		s.cron.Remove(ref.EntryID)
	}
	delete(s.registeredSchedules, namespacedName.String())

	s.decRegisteredSchedulesGauge(namespacedName.Namespace)
}

func (s *Scheduler) createObject(jobType k8upv1alpha1.JobType, namespace string, obj ObjectCreator, config job.Config) {

	name := fmt.Sprintf("%sjob-%d", jobType, time.Now().Unix())

	rtObj := obj.CreateObject(name, namespace)

	jobObject, ok := rtObj.(job.Object)
	if !ok {
		config.Log.Error(errors.New("cannot cast object"), "object is not a valid objectMeta")
		return
	}

	err := controllerutil.SetOwnerReference(config.Obj.GetMetaObject(), jobObject.GetMetaObject(), config.Scheme)
	if err != nil {
		config.Log.Error(err, "cannot set owner on object", "name", jobObject.GetMetaObject().GetName())
	}

	err = config.Client.Create(config.CTX, rtObj)
	if err != nil {
		config.Log.Error(err, "could not trigger k8up newJobs", "name", namespace+"/"+name)
	}

}

func (s *Scheduler) incRegisteredSchedulesGauge(namespace string) {
	scheduleGauge.WithLabelValues(namespace).Inc()
}

func (s *Scheduler) decRegisteredSchedulesGauge(namespace string) {
	scheduleGauge.WithLabelValues(namespace).Dec()
}
