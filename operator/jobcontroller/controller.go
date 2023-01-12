package jobcontroller

import (
	"context"
	"fmt"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/monitoring"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	jobFinalizerName string = "k8up.io/jobobserver"
)

// JobReconciler reconciles a Job object
type JobReconciler struct {
	Kube client.Client
}

func (r *JobReconciler) NewObject() *batchv1.Job {
	return &batchv1.Job{}
}

func (r *JobReconciler) NewObjectList() *batchv1.JobList {
	return &batchv1.JobList{}
}

func (r *JobReconciler) Deprovision(ctx context.Context, obj *batchv1.Job) (ctrl.Result, error) {
	return ctrl.Result{}, r.removeFinalizer(ctx, obj)
}

func (r *JobReconciler) Provision(ctx context.Context, obj *batchv1.Job) (ctrl.Result, error) {
	finalizerErr := r.removeFinalizer(ctx, obj)
	if finalizerErr != nil {
		return ctrl.Result{}, finalizerErr
	}
	return ctrl.Result{}, r.Handle(ctx, obj)
}

func (r *JobReconciler) Handle(ctx context.Context, obj *batchv1.Job) error {
	owner, err := r.fetchOwner(ctx, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil // owner doesn't exist anymore, nothing to do.
		}
		return err
	}
	if !owner.GetDeletionTimestamp().IsZero() {
		return nil // owner got deleted, probably from cleanup. Nothing to do.
	}

	if err := r.updateOwner(ctx, obj, owner); err != nil {
		return fmt.Errorf("could not update owner: %w", err)
	}
	return nil
}

func (r *JobReconciler) fetchOwner(ctx context.Context, batchJob *batchv1.Job) (k8upv1.JobObject, error) {
	controllerReference := metav1.GetControllerOf(batchJob)
	if controllerReference == nil {
		return nil, fmt.Errorf("job has no controller reference: %s/%s", batchJob.Namespace, batchJob.Name)
	}

	var result k8upv1.JobObject
	switch controllerReference.Kind {
	case k8upv1.BackupKind:
		result = &k8upv1.Backup{}
	case k8upv1.ArchiveKind:
		result = &k8upv1.Archive{}
	case k8upv1.RestoreKind:
		result = &k8upv1.Restore{}
	case k8upv1.CheckKind:
		result = &k8upv1.Check{}
	case k8upv1.PruneKind:
		result = &k8upv1.Prune{}
	default:
		return nil, fmt.Errorf("unrecognized controller kind in owner reference: %s", controllerReference.Kind)
	}

	// fetch the owner object
	err := r.Kube.Get(ctx, types.NamespacedName{Name: controllerReference.Name, Namespace: batchJob.Namespace}, result)
	if err != nil {
		return nil, fmt.Errorf("cannot get resource: %s/%s/%s: %w", controllerReference.Kind, batchJob.Namespace, batchJob.Name, err)
	}
	return result, nil
}

func (r *JobReconciler) updateOwner(ctx context.Context, batchJob *batchv1.Job, owner k8upv1.JobObject) error {
	log := ctrl.LoggerFrom(ctx)

	// update status conditions based on Job status
	ownerStatus := owner.GetStatus()
	message := fmt.Sprintf("job '%s' has %d active, %d succeeded and %d failed pods",
		batchJob.Name, batchJob.Status.Active, batchJob.Status.Succeeded, batchJob.Status.Failed)

	successCond := FindStatusCondition(batchJob.Status.Conditions, batchv1.JobComplete)
	if successCond != nil && successCond.Status == corev1.ConditionTrue {
		if !ownerStatus.HasSucceeded() {
			// only increase success counter if new condition
			monitoring.IncSuccessCounters(batchJob.Namespace, owner.GetType())
			log.Info("Job succeeded")
		}
		ownerStatus.SetSucceeded(message)
		ownerStatus.SetFinished(fmt.Sprintf("job '%s' completed successfully", batchJob.Name))
	}
	failedCond := FindStatusCondition(batchJob.Status.Conditions, batchv1.JobFailed)
	if failedCond != nil && failedCond.Status == corev1.ConditionTrue {
		if !ownerStatus.HasFailed() {
			// only increase fail counter if new condition
			monitoring.IncFailureCounters(batchJob.Namespace, owner.GetType())
			log.Info("Job failed")
		}
		ownerStatus.SetFailed(message)
		ownerStatus.SetFinished(fmt.Sprintf("job '%s' has failed", batchJob.Name))
	}
	if successCond == nil && failedCond == nil {
		ownerStatus.SetStarted(message)
	}
	owner.SetStatus(ownerStatus)
	return r.Kube.Status().Update(ctx, owner)
}

func (r *JobReconciler) removeFinalizer(ctx context.Context, obj *batchv1.Job) error {
	_, err := controllerutil.CreateOrUpdate(ctx, r.Kube, obj, func() error {
		// update to a new K8up version: Ensure that all finalizers get removed.
		controllerutil.RemoveFinalizer(obj, jobFinalizerName)
		controllerutil.RemoveFinalizer(obj, "k8up.syn.tools/jobobserver") // legacy finalizer
		return nil
	})
	if err != nil {
		return fmt.Errorf("could not update finalizers: %w", err)
	}
	return nil
}
