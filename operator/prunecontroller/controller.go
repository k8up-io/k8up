package prunecontroller

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

// PruneReconciler reconciles a Prune object
type PruneReconciler struct {
	Kube   client.Client
	Locker *locker.Locker
}

func (r *PruneReconciler) NewObject() *k8upv1.Prune {
	return &k8upv1.Prune{}
}

func (r *PruneReconciler) NewObjectList() *k8upv1.PruneList {
	return &k8upv1.PruneList{}
}

func (r *PruneReconciler) Provision(ctx context.Context, prune *k8upv1.Prune) (controllerruntime.Result, error) {
	log := controllerruntime.LoggerFrom(ctx)

	if prune.Status.HasStarted() {
		return controllerruntime.Result{}, nil
	}
	repository := cfg.Config.GetGlobalRepository()
	if prune.Spec.Backend != nil {
		repository = prune.Spec.Backend.String()
	}
	config := job.NewConfig(ctx, r.Kube, log, prune, repository)
	executor := NewPruneExecutor(config)

	if prune.Status.HasFinished() {
		executor.cleanupOldPrunes(ctx, prune)
		return controllerruntime.Result{}, nil
	}

	running, err := r.Locker.IsAnyJobRunningForRepository(repository)
	if err != nil {
		return controllerruntime.Result{RequeueAfter: time.Second * 30}, err
	}

	shouldRun := !running
	if shouldRun {
		return controllerruntime.Result{RequeueAfter: time.Second * 8}, executor.Execute(ctx)
	} else {
		log.Info("Delaying prune task, another job is running")
	}
	return controllerruntime.Result{RequeueAfter: time.Second * 30}, nil
}

func (r *PruneReconciler) Deprovision(_ context.Context, _ *k8upv1.Prune) (controllerruntime.Result, error) {
	return controllerruntime.Result{}, nil
}
