package handler

import (
	"fmt"
	"strconv"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/job"
	"github.com/vshn/k8up/observer"
)

const (
	jobFinalizerName string = "k8up.syn.tools/jobobserver"
)

// JobHandler handles the reconciles for the batchv1.job objects that are
// observed by the operator.
type JobHandler struct {
	job *batchv1.Job
	job.Config
	requireSpecUpdate bool
}

// NewJobHandler returns a new JobHandler.
func NewJobHandler(config job.Config, job *batchv1.Job) *JobHandler {
	return &JobHandler{
		job:    job,
		Config: config,
	}
}

// Handle extracts some information from the batchv1.job that make observations
// easier.
func (j *JobHandler) Handle() error {

	jobEvent := observer.Create

	if _, exists := j.job.GetLabels()[job.K8uplabel]; !exists {
		return nil
	}

	if j.job.GetDeletionTimestamp() != nil && contains(j.job.GetFinalizers(), jobFinalizerName) {
		jobEvent = observer.Delete
		controllerutil.RemoveFinalizer(j.job, jobFinalizerName)
		j.requireSpecUpdate = true
	} else {
		if j.job.Status.Active > 0 {
			jobEvent = observer.Running
			controllerutil.AddFinalizer(j.job, jobFinalizerName)
			j.requireSpecUpdate = true
		}

		if j.job.Status.Succeeded > 0 {
			jobEvent = observer.Succeeded
		}

		if j.job.Status.Failed > 0 {
			jobEvent = observer.Failed
		}

	}

	exclusive, err := strconv.ParseBool(j.job.GetLabels()[job.K8upExclusive])
	if err != nil {
		exclusive = false
	}

	jobType := v1alpha1.ScheduleType
	if j.Config.Obj != nil {
		jobType = j.Config.Obj.GetType()
	}

	oj := observer.ObservableJob{
		Job:       j.job,
		JobType:   jobType,
		Exclusive: exclusive,
		Event:     jobEvent,
	}

	if j.requireSpecUpdate {
		if err := j.Client.Update(j.CTX, j.job); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("error updating resource: %w", err)
		}
	}
	observer.GetObserver().GetUpdateChannel() <- oj
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
