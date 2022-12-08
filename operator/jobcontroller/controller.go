package jobcontroller

import (
	"context"
	"fmt"
	"strconv"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/job"
	"github.com/k8up-io/k8up/v2/operator/observer"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	jobFinalizerName string = "k8up.io/jobobserver"

	// Deprecated: Migrate to jobFinalizerName as the new finalizer name
	legacyJobFinalizerName string = "k8up.syn.tools/jobobserver"
)

// JobReconciler reconciles a Job object
type JobReconciler struct {
	Kube client.Client
}

// Reconcile is the entrypoint to manage the given resource.
func (r *JobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	jobObj := &batchv1.Job{}

	err := r.Kube.Get(ctx, req.NamespacedName, jobObj)
	if err != nil {

		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	return ctrl.Result{}, r.Handle(ctx, jobObj)
}

func (r *JobReconciler) Handle(ctx context.Context, obj *batchv1.Job) error {

	jobEvent := observer.Create
	_, err := controllerutil.CreateOrUpdate(ctx, r.Kube, obj, func() error {
		if obj.GetDeletionTimestamp().IsZero() {
			jobEvent = observer.Delete
			controllerutil.RemoveFinalizer(obj, jobFinalizerName)
			controllerutil.RemoveFinalizer(obj, legacyJobFinalizerName)
			return nil
		}
		if obj.Status.Active > 0 {
			jobEvent = observer.Running
			controllerutil.AddFinalizer(obj, jobFinalizerName)
		}
		if obj.Status.Succeeded > 0 {
			jobEvent = observer.Succeeded
		}
		if obj.Status.Failed > 0 {
			jobEvent = observer.Failed
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("could not update finalizers: %w", err)
	}

	exclusive, err := strconv.ParseBool(obj.GetLabels()[job.K8upExclusive])
	if err != nil {
		exclusive = false
	}

	jobType, exists := obj.GetLabels()[k8upv1.LabelK8upType]
	if !exists {
		jobType, exists = obj.GetLabels()[k8upv1.LegacyLabelK8upType]
	}
	if !exists {
		jobType = k8upv1.ScheduleType.String()
	}

	oj := observer.ObservableJob{
		Job:       obj,
		JobType:   k8upv1.JobType(jobType),
		Exclusive: exclusive,
		Event:     jobEvent,
	}

	observer.GetObserver().GetUpdateChannel() <- oj
	return nil
}
