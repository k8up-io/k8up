package checkcontroller

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

// CheckReconciler reconciles a Check object
type CheckReconciler struct {
	Kube client.Client
}

// Reconcile is the entrypoint to manage the given resource.
func (r *CheckReconciler) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	log := controllerruntime.LoggerFrom(ctx)

	check := &k8upv1.Check{}
	err := r.Kube.Get(ctx, req.NamespacedName, check)
	if err != nil {
		if errors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}
		log.Error(err, "Failed to get Check")
		return controllerruntime.Result{}, err
	}

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
		executor.cleanupOldChecks(executor.GetJobNamespacedName(), check)
		return controllerruntime.Result{}, nil
	}

	log.V(1).Info("adding job to the queue")
	queue.GetExecQueue().Add(executor)
	return controllerruntime.Result{RequeueAfter: time.Second * 30}, nil
}
