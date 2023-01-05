package backupcontroller

import (
	"context"
	"time"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
	"github.com/k8up-io/k8up/v2/operator/locker"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// BackupReconciler reconciles a Backup object
type BackupReconciler struct {
	Kube   client.Client
	Locker *locker.Locker
}

func (r *BackupReconciler) NewObject() *k8upv1.Backup {
	return &k8upv1.Backup{}
}

func (r *BackupReconciler) NewObjectList() *k8upv1.BackupList {
	return &k8upv1.BackupList{}
}

func (r *BackupReconciler) Provision(ctx context.Context, obj *k8upv1.Backup) (reconcile.Result, error) {
	log := controllerruntime.LoggerFrom(ctx)

	prebackupCond := meta.FindStatusCondition(obj.Status.Conditions, k8upv1.ConditionPreBackupPodReady.String())
	if obj.Status.HasFinished() && prebackupCond != nil {
		if prebackupCond.Reason == k8upv1.ReasonFinished.String() || prebackupCond.Reason == k8upv1.ReasonFailed.String() || prebackupCond.Reason == k8upv1.ReasonNoPreBackupPodsFound.String() {
			// only ignore future reconciles if we have stopped all prebackup deployments in an earlier reconciliation.
			return controllerruntime.Result{}, nil
		}
	}

	repository := cfg.Config.GetGlobalRepository()
	if obj.Spec.Backend != nil {
		repository = obj.Spec.Backend.String()
	}
	config := job.NewConfig(ctx, r.Kube, log, obj, repository)
	executor := NewBackupExecutor(config)

	shouldRun, err := r.Locker.ShouldRunJob(config, executor.GetConcurrencyLimit())
	if err != nil {
		return controllerruntime.Result{RequeueAfter: time.Second * 30}, err
	}
	if shouldRun {
		return controllerruntime.Result{RequeueAfter: time.Second * 30}, executor.Execute(ctx)
	} else {
		log.Info("Skipping job due to exclusivity or concurrency limit")
	}
	return controllerruntime.Result{RequeueAfter: time.Second * 30}, nil
}

func (r *BackupReconciler) Deprovision(_ context.Context, _ *k8upv1.Backup) (controllerruntime.Result, error) {
	return controllerruntime.Result{}, nil
}
