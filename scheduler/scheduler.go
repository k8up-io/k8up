// scheduler ensures that scheduled jobs will be added to the queue

package scheduler

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/robfig/cron/v3"
	"github.com/vshn/k8up/job"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	BackupType  Type = "backup"
	CheckType   Type = "check"
	ArchiveType Type = "archive"
	RestoreType Type = "restore"
	PruneType   Type = "prune"
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

// ObjectCreator defines an interface that each schedulable job must implement.
// The simplest implementation is that the concrete object just returns itself.
type ObjectCreator interface {
	CreateObject(name, namespace string) runtime.Object
}

// Type defines what schedule type this is.
type Type string

// Job contains all necessary information to create a schedule.
type Job struct {
	Type     Type
	Schedule string
	Object   ObjectCreator
}

// JobList contains a slice of jobs and job.Config to actually apply the
// the job objects.
type JobList struct {
	Jobs   []Job
	Config job.Config
}

// Scheduler handles all the schedules.
type Scheduler struct {
	cron                *cron.Cron
	registeredSchedules map[string][]int
	mutex               sync.Mutex
}

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(scheduleGauge)
}

// GetScheduler returns the scheduler singleton instance.
func GetScheduler() *Scheduler {
	if scheduler == nil {
		scheduler = &Scheduler{
			cron:                cron.New(),
			registeredSchedules: make(map[string][]int),
			mutex:               sync.Mutex{},
		}
		scheduler.cron.Start()
	}

	return scheduler
}

// AddSchedules will add the given schedule to the running cron.
func (s *Scheduler) AddSchedules(jobs JobList) error {

	namespacedName := types.NamespacedName{
		Name:      jobs.Config.Obj.GetMetaObject().GetName(),
		Namespace: jobs.Config.Obj.GetMetaObject().GetNamespace(),
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if len(s.registeredSchedules[namespacedName.String()]) > 0 {
		return nil
	}

	jobIDs := make([]int, len(jobs.Jobs))

	for i, jb := range jobs.Jobs {

		jobs.Config.Log.Info("registering schedule for", "type", jb.Type, "schedule", jb.Schedule)

		id, err := s.cron.AddFunc(jb.Schedule, func() {
			jobs.Config.Log.Info("running schedule for", "jb", jb.Type)
			s.createObject(jb.Type, namespacedName.Namespace, jb.Object, jobs.Config)
		})
		if err != nil {
			return err
		}
		jobIDs[i] = int(id)
	}

	s.registeredSchedules[namespacedName.String()] = jobIDs

	s.incRegisteredSchedulesGauge(namespacedName.Namespace)
	return nil
}

// RemoveSchedules will remove the schedules with the given types.NamespacedName if existing.
func (s *Scheduler) RemoveSchedules(namespacedName types.NamespacedName) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	schedules, found := s.registeredSchedules[namespacedName.String()]
	if !found {
		return
	}
	for _, id := range schedules {
		s.cron.Remove(cron.EntryID(id))
	}
	delete(s.registeredSchedules, namespacedName.String())

	s.decRegisteredSchedulesGauge(namespacedName.Namespace)
}

func (s *Scheduler) createObject(jobType Type, namespace string, obj ObjectCreator, config job.Config) {

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
		config.Log.Error(err, "could not trigger k8up job", "name", namespace+"/"+name)
	}

}

func (s *Scheduler) incRegisteredSchedulesGauge(namespace string) {
	scheduleGauge.WithLabelValues(namespace).Inc()
}

func (s *Scheduler) decRegisteredSchedulesGauge(namespace string) {
	scheduleGauge.WithLabelValues(namespace).Dec()
}
