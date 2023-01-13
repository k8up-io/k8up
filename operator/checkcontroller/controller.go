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
	Kube client.Client
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

	config := job.NewConfig(r.Kube, check, repository)

	executor := NewCheckExecutor(config)

	if check.Status.HasFinished() {
		executor.cleanupOldChecks(ctx, check)
		return controllerruntime.Result{}, nil
	}

	lock := locker.GetForRepository(r.Kube, repository)
	didRun, err := lock.TryRunExclusively(ctx, executor.Execute)
	if !didRun && err == nil {
		log.Info("Delaying check task, another job is running")
	}
	return controllerruntime.Result{RequeueAfter: time.Second * 30}, err
}

func (r *CheckReconciler) Deprovision(_ context.Context, _ *k8upv1.Check) (controllerruntime.Result, error) {
	return controllerruntime.Result{}, nil
}
