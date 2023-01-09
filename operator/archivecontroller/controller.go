package archivecontroller

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

// ArchiveReconciler reconciles Archive objects
type ArchiveReconciler struct {
	Kube client.Client
}

func (r *ArchiveReconciler) NewObject() *k8upv1.Archive {
	return &k8upv1.Archive{}
}

func (r *ArchiveReconciler) NewObjectList() *k8upv1.ArchiveList {
	return &k8upv1.ArchiveList{}
}

func (r *ArchiveReconciler) Provision(ctx context.Context, archive *k8upv1.Archive) (controllerruntime.Result, error) {
	log := controllerruntime.LoggerFrom(ctx)

	if archive.Status.HasStarted() {
		return controllerruntime.Result{}, nil
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
		executor.cleanupOldArchives(ctx, archive)
		return controllerruntime.Result{}, nil
	}

	lock := locker.GetForRepository(r.Kube, repository)
	didRun, err := lock.TryRun(ctx, config, executor.GetConcurrencyLimit(), executor.Execute)
	if !didRun && err == nil {
		log.Info("Skipping job due to exclusivity or concurrency limit")
	}
	return controllerruntime.Result{RequeueAfter: time.Second * 30}, err
}

func (r *ArchiveReconciler) Deprovision(_ context.Context, _ *k8upv1.Archive) (controllerruntime.Result, error) {
	return controllerruntime.Result{}, nil
}
