package prunecontroller

import (
	"context"
	"time"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
	"github.com/k8up-io/k8up/v2/operator/queue"
	"k8s.io/apimachinery/pkg/api/errors"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PruneReconciler reconciles a Prune object
type PruneReconciler struct {
	Kube client.Client
}

// Reconcile is the entrypoint to manage the given resource.
func (r *PruneReconciler) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	log := controllerruntime.LoggerFrom(ctx)

	prune := &k8upv1.Prune{}
	err := r.Kube.Get(ctx, req.NamespacedName, prune)
	if err != nil {
		if errors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}

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

	log.V(1).Info("adding job to the queue")
	queue.GetExecQueue().Add(executor)
	return controllerruntime.Result{RequeueAfter: time.Second * 30}, nil
}
