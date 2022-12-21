package jobcontroller

import (
	"context"
	"fmt"
	"strconv"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/job"
	"github.com/k8up-io/k8up/v2/operator/monitoring"
	"github.com/k8up-io/k8up/v2/operator/observer"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	return ctrl.Result{}, r.Handle(ctx, jobObj)
}

func (r *JobReconciler) Handle(ctx context.Context, obj *batchv1.Job) error {

	jobEvent := observer.Create
	_, err := controllerutil.CreateOrUpdate(ctx, r.Kube, obj, func() error {
		if !obj.GetDeletionTimestamp().IsZero() {
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

	switch k8upv1.JobType(jobType) {
	case k8upv1.ArchiveType:
		fallthrough
	case k8upv1.RestoreType:
		fallthrough
	case k8upv1.BackupType:
		return r.updateOwner(ctx, obj)
	default:
		observer.GetObserver().GetUpdateChannel() <- oj
		return nil
	}
}

func (r *JobReconciler) updateOwner(ctx context.Context, batchJob *batchv1.Job) error {
	controllerReference := metav1.GetControllerOf(batchJob)
	if controllerReference == nil {
		return fmt.Errorf("job has no controller reference: %s/%s", batchJob.Namespace, batchJob.Name)
	}

	var result k8upv1.JobObject
	var jobType k8upv1.JobType
	switch controllerReference.Kind {
	case k8upv1.BackupKind:
		result = &k8upv1.Backup{}
		jobType = k8upv1.BackupType
	case k8upv1.ArchiveKind:
		result = &k8upv1.Archive{}
		jobType = k8upv1.ArchiveType
	case k8upv1.RestoreKind:
		result = &k8upv1.Restore{}
		jobType = k8upv1.RestoreType
	default:
		return fmt.Errorf("unrecognized controller kind in owner reference: %s", controllerReference.Kind)
	}

	// fetch the owner object
	err := r.Kube.Get(ctx, types.NamespacedName{Name: controllerReference.Name, Namespace: batchJob.Namespace}, result)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil // owner doesn't exist anymore, nothing to do.
		}
		return fmt.Errorf("cannot get resource: %s/%s/%s: %w", controllerReference.Kind, batchJob.Namespace, batchJob.Name, err)
	}

	log := ctrl.LoggerFrom(ctx)

	// update status conditions based on Job status
	ownerStatus := result.GetStatus()
	message := fmt.Sprintf("job '%s' has %d active, %d succeeded and %d failed pods",
		batchJob.Name, batchJob.Status.Active, batchJob.Status.Succeeded, batchJob.Status.Failed)

	successCond := FindStatusCondition(batchJob.Status.Conditions, batchv1.JobComplete)
	if successCond != nil && successCond.Status == corev1.ConditionTrue {
		if !ownerStatus.HasSucceeded() {
			// only increase success counter if new condition
			monitoring.IncSuccessCounters(batchJob.Namespace, jobType)
			log.Info("Job succeeded")
		}
		ownerStatus.SetSucceeded(message)
		ownerStatus.SetFinished(fmt.Sprintf("job '%s' completed successfully", batchJob.Name))
	}
	failedCond := FindStatusCondition(batchJob.Status.Conditions, batchv1.JobFailed)
	if failedCond != nil && failedCond.Status == corev1.ConditionTrue {
		if !ownerStatus.HasFailed() {
			// only increase fail counter if new condition
			monitoring.IncFailureCounters(batchJob.Namespace, jobType)
			log.Info("Job failed")
		}
		ownerStatus.SetFailed(message)
		ownerStatus.SetFinished(fmt.Sprintf("job '%s' has failed", batchJob.Name))
	}
	if successCond == nil && failedCond == nil {
		ownerStatus.SetStarted(message)
	}
	result.SetStatus(ownerStatus)
	return r.Kube.Status().Update(ctx, result)
}
