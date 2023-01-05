package restorecontroller

import (
	"context"
	"time"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
	"github.com/k8up-io/k8up/v2/operator/locker"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RestoreReconciler reconciles a Restore object
type RestoreReconciler struct {
	Kube   client.Client
	Locker *locker.Locker
}

func (r *RestoreReconciler) NewObject() *k8upv1.Restore {
	return &k8upv1.Restore{}
}

func (r *RestoreReconciler) NewObjectList() *k8upv1.RestoreList {
	return &k8upv1.RestoreList{}
}

func (r *RestoreReconciler) Provision(ctx context.Context, restore *k8upv1.Restore) (controllerruntime.Result, error) {
	log := controllerruntime.LoggerFrom(ctx)

	if restore.Status.HasStarted() {
		return controllerruntime.Result{}, nil
	}

	repository := cfg.Config.GetGlobalRepository()
	if restore.Spec.Backend != nil {
		repository = restore.Spec.Backend.String()
	}
	config := job.NewConfig(ctx, r.Kube, log, restore, repository)
	executor := NewRestoreExecutor(config)

	if restore.Status.HasFinished() {
		executor.cleanupOldRestores(ctx, restore)
		return controllerruntime.Result{}, nil
	}

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

func (r *RestoreReconciler) Deprovision(_ context.Context, _ *k8upv1.Restore) (controllerruntime.Result, error) {
	return controllerruntime.Result{}, nil
}
