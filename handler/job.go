package handler

import (
	"context"
	"fmt"
	"strconv"

	"github.com/vshn/k8up/cfg"
	"github.com/vshn/k8up/job"
	"github.com/vshn/k8up/observer"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	jobFinalizerName string = "k8up.syn.tools/jobobserver"
)

// JobHandler handles the reconciles for the batchv1.job objects that are
// observed by the operator.
type JobHandler struct {
	job *batchv1.Job
	job.Config
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
		err := removeFinalizer(j.CTX, j.job, jobFinalizerName, j.Client)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("error removing finalizer: %w", err)
		}
	} else {
		if j.job.Status.Active > 0 {
			jobEvent = observer.Running
			err := addFinalizer(j.CTX, j.job, jobFinalizerName, j.Client)
			if err != nil {
				return err
			}
		}

		if j.job.Status.Succeeded > 0 {
			jobEvent = observer.Suceeded
		}

		if j.job.Status.Failed > 0 {
			jobEvent = observer.Failed
		}

	}

	exclusive, err := strconv.ParseBool(j.job.GetLabels()[job.K8upExclusive])
	if err != nil {
		exclusive = false
	}

	jobType := cfg.Job
	if j.Config.Obj != nil {
		jobType = j.Config.Obj.GetType()
	}

	oj := observer.ObservableJob{
		Job:       j.job,
		JobType:   jobType,
		Exclusive: exclusive,
		Event:     jobEvent,
	}

	observer.GetObserver().GetUpdateChannel() <- oj
	return nil
}

func addFinalizer(ctx context.Context, obj controllerutil.Object, name string, client client.Client) error {
	controllerutil.AddFinalizer(obj, name)

	// Update CR
	err := client.Update(ctx, obj)
	if err != nil {
		return err
	}
	return nil
}

func removeFinalizer(ctx context.Context, obj controllerutil.Object, name string, client client.Client) error {
	controllerutil.RemoveFinalizer(obj, name)
	err := client.Update(ctx, obj)
	if err != nil {
		return err
	}
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
