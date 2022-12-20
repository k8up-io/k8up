package archivecontroller

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

// ArchiveReconciler reconciles a Archive object
type ArchiveReconciler struct {
	Kube client.Client
}

// Reconcile is the entrypoint to manage the given resource.
func (r *ArchiveReconciler) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	log := controllerruntime.LoggerFrom(ctx).WithValues("archive", req.NamespacedName)

	archive := &k8upv1.Archive{}
	err := r.Kube.Get(ctx, req.NamespacedName, archive)
	if err != nil {
		if errors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}
		log.Error(err, "Failed to get Archive")
		return controllerruntime.Result{}, err
	}

	repository := cfg.Config.GetGlobalRepository()
	if archive.Spec.Backend != nil {
		repository = archive.Spec.Backend.String()
	}
	if archive.Spec.RestoreSpec == nil {
		archive.Spec.RestoreSpec = &k8upv1.RestoreSpec{}
	}
	config := job.NewConfig(ctx, r.Kube, log, archive, repository)

	executor := NewArchiveExecutor(config)

	if archive.Status.HasFinished() {
		executor.cleanupOldArchives(executor.GetJobNamespacedName(), archive)
		return controllerruntime.Result{}, nil
	}

	log.V(1).Info("adding job to the queue")
	queue.GetExecQueue().Add(executor)
	return controllerruntime.Result{RequeueAfter: time.Second * 30}, nil
}
