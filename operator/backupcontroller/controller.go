package backupcontroller

import (
	"context"
	"fmt"
	"time"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
	"github.com/k8up-io/k8up/v2/operator/locker"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// BackupReconciler reconciles a Backup object
type BackupReconciler struct {
	Kube client.Client
}

func (r *BackupReconciler) NewObject() *k8upv1.Backup {
	return &k8upv1.Backup{}
}

func (r *BackupReconciler) NewObjectList() *k8upv1.BackupList {
	return &k8upv1.BackupList{}
}

func (r *BackupReconciler) Provision(ctx context.Context, obj *k8upv1.Backup) (reconcile.Result, error) {
	log := controllerruntime.LoggerFrom(ctx)

	repository := cfg.Config.GetGlobalRepository()
	if obj.Spec.Backend != nil {
		repository = obj.Spec.Backend.String()
	}
	config := job.NewConfig(r.Kube, obj, repository)
	executor := NewBackupExecutor(config)

	if err := r.ReconcileJobStatus(ctx, obj); err != nil {
		return controllerruntime.Result{}, err
	}

	if obj.Status.HasStarted() {
		log.V(1).Info("backup just started, waiting")
		return controllerruntime.Result{RequeueAfter: 5 * time.Second}, nil
	}
	if obj.Status.HasFinished() || isPrebackupFailed(obj) {
		cleanupCond := meta.FindStatusCondition(obj.Status.Conditions, k8upv1.ConditionScrubbed.String())
		if cleanupCond == nil || cleanupCond.Reason != k8upv1.ReasonSucceeded.String() {
			executor.cleanupOldBackups(ctx)
		}

		prebackupCond := meta.FindStatusCondition(obj.Status.Conditions, k8upv1.ConditionPreBackupPodReady.String())
		if prebackupCond != nil && (prebackupCond.Reason == k8upv1.ReasonFinished.String() || prebackupCond.Reason == k8upv1.ReasonFailed.String() || prebackupCond.Reason == k8upv1.ReasonNoPreBackupPodsFound.String()) {
			// only ignore future reconciles if we have stopped all prebackup deployments in an earlier reconciliation.
			return controllerruntime.Result{}, nil
		}
		executor.StopPreBackupDeployments(ctx)
		return controllerruntime.Result{RequeueAfter: time.Second * 30}, nil
	}

	lock := locker.GetForRepository(r.Kube, repository)
	didRun, err := lock.TryRun(ctx, config, executor.GetConcurrencyLimit(), executor.Execute)
	if !didRun && err == nil {
		log.Info("Skipping job due to exclusivity or concurrency limit")
	}
	return controllerruntime.Result{RequeueAfter: time.Second * 30}, err
}

// ReconcileJobStatus implements a custom job reconciliation since there can be multiple jobs per Backup (this is different
// from the implementation in the job package).
func (r *BackupReconciler) ReconcileJobStatus(ctx context.Context, obj *k8upv1.Backup) error {
	log := controllerruntime.LoggerFrom(ctx)
	ownedBy := obj.GetType().String() + "_" + obj.GetName()
	log.V(1).Info("reconciling jobs", "owned-by", ownedBy)

	jobList := batchv1.JobList{}
	if err := r.Kube.List(ctx, &jobList, client.MatchingLabels{k8upv1.LabelK8upOwnedBy: ownedBy}, client.InNamespace(obj.Namespace)); err != nil {
		return fmt.Errorf("list jobs: %w", err)
	}

	numJobs := len(jobList.Items)
	if numJobs == 0 {
		return nil
	}

	numSucceeded, numFailed, numStarted := 0, 0, 0
	for _, item := range jobList.Items {
		conditions := item.Status.Conditions
		if job.HasSucceeded(conditions) {
			numSucceeded += 1
		}
		if job.HasFailed(conditions) {
			numFailed += 1
		}
		if job.HasStarted(conditions) {
			numStarted += 1
		}
	}

	objStatus := obj.Status
	message := fmt.Sprintf("%q has %d succeeded, %d failed, and %d started jobs", ownedBy, numSucceeded, numFailed, numStarted)
	if numJobs == numSucceeded {
		job.SetSucceeded(ctx, ownedBy, obj.Namespace, obj.GetType(), &objStatus, message)
	} else if numFailed > 0 {
		job.SetFailed(ctx, ownedBy, obj.Namespace, obj.GetType(), &objStatus, message)
	} else if numStarted > 0 {
		objStatus.SetStarted(message)
	}
	obj.SetStatus(objStatus)

	log.V(1).Info("updating status")
	if err := r.Kube.Status().Update(ctx, obj); err != nil {
		return fmt.Errorf("backup status update failed: %w", err)
	}
	return nil
}

func (r *BackupReconciler) Deprovision(_ context.Context, _ *k8upv1.Backup) (controllerruntime.Result, error) {
	return controllerruntime.Result{}, nil
}
