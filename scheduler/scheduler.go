// scheduler ensures that scheduled jobs will be added to the queue

package scheduler

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/vshn/k8up/job"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
)

type ObjectCreator interface {
	CreateObject(name, namespace string) runtime.Object
}

type Type string

type Job struct {
	Type     Type
	Schedule string
	Object   ObjectCreator
}

type JobList struct {
	Jobs   []Job
	Config job.Config
}

type Scheduler struct {
	cron                *cron.Cron
	registeredSchedules map[string][]int
	mutex               sync.Mutex
}

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

// AddSchedule will add the given schedule to the running cron. It returns the
// id of the job. That can be used to remove it later.
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

	for i, job := range jobs.Jobs {

		jobs.Config.Log.Info("registering schedule for", "type", job.Type)

		jobType := job.Type
		obj := job.Object
		config := jobs.Config

		id, err := s.cron.AddFunc(job.Schedule, func() {
			jobs.Config.Log.Info("running schedule for", "job", jobType)
			s.createObject(jobType, namespacedName.Namespace, obj, config)
		})
		if err != nil {
			return err
		}
		jobIDs[i] = int(id)
	}

	s.registeredSchedules[namespacedName.String()] = jobIDs

	return nil
}

func (s *Scheduler) RemoveSchedules(name string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, id := range s.registeredSchedules[name] {
		s.cron.Remove(cron.EntryID(id))
	}
	delete(s.registeredSchedules, name)
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
