package prunecontroller

import (
	"context"
	"time"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
	"github.com/k8up-io/k8up/v2/operator/locker"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PruneReconciler reconciles a Prune object
type PruneReconciler struct {
	Kube client.Client
}

func (r *PruneReconciler) NewObject() *k8upv1.Prune {
	return &k8upv1.Prune{}
}

func (r *PruneReconciler) NewObjectList() *k8upv1.PruneList {
	return &k8upv1.PruneList{}
}

func (r *PruneReconciler) Provision(ctx context.Context, obj *k8upv1.Prune) (controllerruntime.Result, error) {
	log := controllerruntime.LoggerFrom(ctx)

	repository := cfg.Config.GetGlobalRepository()
	if obj.Spec.Backend != nil {
		repository = obj.Spec.Backend.String()
	}
	config := job.NewConfig(r.Kube, obj, repository)
	executor := NewPruneExecutor(config)

	jobKey := types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      executor.jobName(),
	}
	if err := job.ReconcileJobStatus(ctx, jobKey, r.Kube, obj); err != nil {
		return controllerruntime.Result{}, err
	}

	if obj.Status.HasStarted() {
		log.V(1).Info("prune just started, waiting")
		return controllerruntime.Result{RequeueAfter: 5 * time.Second}, nil
	}
	if obj.Status.HasFinished() {
		executor.cleanupOldPrunes(ctx, obj)
		return controllerruntime.Result{}, nil
	}

	lock := locker.GetForRepository(r.Kube, repository)
	didRun, err := lock.TryRunExclusively(ctx, executor.Execute)
	if !didRun && err == nil {
		log.Info("Delaying prune task, another job is running")
	}
	return controllerruntime.Result{RequeueAfter: time.Second * 8}, err
}

func (r *PruneReconciler) Deprovision(_ context.Context, _ *k8upv1.Prune) (controllerruntime.Result, error) {
	return controllerruntime.Result{}, nil
}
