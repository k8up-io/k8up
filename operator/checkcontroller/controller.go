package checkcontroller

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

// CheckReconciler reconciles a Check object
type CheckReconciler struct {
	Kube   client.Client
	Locker *locker.Locker
}

func (r *CheckReconciler) NewObject() *k8upv1.Check {
	return &k8upv1.Check{}
}

func (r *CheckReconciler) NewObjectList() *k8upv1.CheckList {
	return &k8upv1.CheckList{}
}

func (r *CheckReconciler) Provision(ctx context.Context, check *k8upv1.Check) (controllerruntime.Result, error) {
	log := controllerruntime.LoggerFrom(ctx)

	if check.Status.HasStarted() {
		return controllerruntime.Result{}, nil
	}

	repository := cfg.Config.GetGlobalRepository()
	if check.Spec.Backend != nil {
		repository = check.Spec.Backend.String()
	}

	config := job.NewConfig(ctx, r.Kube, log, check, repository)

	executor := NewCheckExecutor(config)

	if check.Status.HasFinished() {
		executor.cleanupOldChecks(ctx, check)
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

func (r *CheckReconciler) Deprovision(_ context.Context, _ *k8upv1.Check) (controllerruntime.Result, error) {
	return controllerruntime.Result{}, nil
}
