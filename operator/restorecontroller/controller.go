package restorecontroller

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

// RestoreReconciler reconciles a Restore object
type RestoreReconciler struct {
	Kube client.Client
}

// Reconcile is the entrypoint to manage the given resource.
func (r *RestoreReconciler) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	log := controllerruntime.LoggerFrom(ctx).WithValues("restore", req.NamespacedName)

	restore := &k8upv1.Restore{}
	err := r.Kube.Get(ctx, req.NamespacedName, restore)
	if err != nil {
		if errors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}
		return controllerruntime.Result{}, err
	}

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
		executor.cleanupOldRestores(executor.GetJobNamespacedName(), restore)
		return controllerruntime.Result{}, nil
	}

	log.V(1).Info("adding job to the queue")
	queue.GetExecQueue().Add(executor)
	return controllerruntime.Result{RequeueAfter: time.Second * 30}, nil
}
