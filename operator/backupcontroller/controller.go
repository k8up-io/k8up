package backupcontroller

import (
	"context"
	"time"

	"github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
	"github.com/k8up-io/k8up/v2/operator/queue"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BackupReconciler reconciles a Backup object
type BackupReconciler struct {
	Kube client.Client
}

// Reconcile is the entrypoint to manage the given resource.
func (r *BackupReconciler) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	log := controllerruntime.LoggerFrom(ctx)

	backup := &v1.Backup{}
	err := r.Kube.Get(ctx, req.NamespacedName, backup)
	if err != nil {
		if errors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}
		log.Error(err, "Failed to get Backup")
		return controllerruntime.Result{}, err
	}

	prebackupCond := meta.FindStatusCondition(backup.Status.Conditions, v1.ConditionPreBackupPodReady.String())
	if backup.Status.HasFinished() && prebackupCond != nil {
		if prebackupCond.Reason == v1.ReasonFinished.String() || prebackupCond.Reason == v1.ReasonFailed.String() {
			// only ignore future reconciles if we have stopped all prebackup deployments in an earlier reconciliation.
			return controllerruntime.Result{}, nil
		}
	}

	repository := cfg.Config.GetGlobalRepository()
	if backup.Spec.Backend != nil {
		repository = backup.Spec.Backend.String()
	}
	config := job.NewConfig(ctx, r.Kube, log, backup, repository)

	executor := NewBackupExecutor(config)

	log.V(1).Info("adding job to the queue")
	queue.GetExecQueue().Add(executor)
	return controllerruntime.Result{RequeueAfter: time.Second * 30}, nil
}
