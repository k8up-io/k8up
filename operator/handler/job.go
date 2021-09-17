package handler

import (
	"fmt"
	"strconv"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	k8upv1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/operator/job"
	"github.com/vshn/k8up/operator/observer"
)

const (
	jobFinalizerName string = "k8up.io/jobobserver"

	// Deprecated
	legacyJobFinalizerName string = "k8up.syn.tools/jobobserver"
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

	finalizers := j.job.GetFinalizers()
	deletionTimestamp := j.job.GetDeletionTimestamp()
	if deletionTimestamp != nil &&
		(contains(finalizers, jobFinalizerName) || contains(finalizers, legacyJobFinalizerName)) {
		jobEvent = observer.Delete
		controllerutil.RemoveFinalizer(j.job, jobFinalizerName)
		controllerutil.RemoveFinalizer(j.job, legacyJobFinalizerName)
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

	jobType, exists := j.job.GetLabels()[k8upv1.LabelK8upType]
	if !exists {
		jobType, exists = j.job.GetLabels()[k8upv1.LegacyLabelK8upType]
	}
	if !exists {
		jobType = k8upv1.ScheduleType.String()
	}

	oj := observer.ObservableJob{
		Job:       j.job,
		JobType:   k8upv1.JobType(jobType),
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
