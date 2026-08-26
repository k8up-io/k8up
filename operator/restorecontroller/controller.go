package restorecontroller

import (
	"context"
	"fmt"
	"time"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
	"github.com/k8up-io/k8up/v2/operator/locker"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RestoreReconciler reconciles a Restore object
type RestoreReconciler struct {
	Kube client.Client
	// KubeReader performs direct API reads so Secrets only require named GET permissions, not cache list/watch permissions.
	KubeReader client.Reader
}

func (r *RestoreReconciler) NewObject() *k8upv1.Restore {
	return &k8upv1.Restore{}
}

func (r *RestoreReconciler) NewObjectList() *k8upv1.RestoreList {
	return &k8upv1.RestoreList{}
}

func (r *RestoreReconciler) Provision(ctx context.Context, obj *k8upv1.Restore) (controllerruntime.Result, error) {
	log := controllerruntime.LoggerFrom(ctx)

	if obj.Spec.Backend != nil {
		if err := obj.Spec.Backend.ResolveSecretRefs(ctx, r.KubeReader, obj.GetNamespace(), cfg.Config.RepositorySecretName); err != nil {
			return controllerruntime.Result{}, fmt.Errorf("resolve backend Secret references: %w", err)
		}
	}
	if obj.Spec.RestoreMethod != nil {
		if err := obj.Spec.RestoreMethod.S3.ResolveSecretRefs(ctx, r.KubeReader, obj.GetNamespace(), cfg.Config.RepositorySecretName); err != nil {
			return controllerruntime.Result{}, fmt.Errorf("resolve restore S3 Secret references: %w", err)
		}
	}

	repository := cfg.Config.GetGlobalRepository()
	if obj.Spec.Backend != nil {
		repository = obj.Spec.Backend.String()
	}
	config := job.NewConfig(r.Kube, obj, repository)
	executor := NewRestoreExecutor(config)

	jobKey := types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      executor.jobName(),
	}
	if err := job.ReconcileJobStatus(ctx, jobKey, r.Kube, obj); err != nil {
		return controllerruntime.Result{}, err
	}

	if obj.Status.HasStarted() {
		log.V(1).Info("restore just started, waiting")
		return controllerruntime.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if obj.Status.HasFinished() {
		executor.cleanupOldRestores(ctx, obj)
		return controllerruntime.Result{}, nil
	}

	lock := locker.GetForRepository(r.Kube, repository)
	didRun, err := lock.TryRun(ctx, config, executor.GetConcurrencyLimit(), executor.Execute)
	if !didRun && err == nil {
		log.Info("Skipping job due to exclusivity or concurrency limit")
	}
	return controllerruntime.Result{RequeueAfter: time.Second * 30}, err
}

func (r *RestoreReconciler) Deprovision(_ context.Context, _ *k8upv1.Restore) (controllerruntime.Result, error) {
	return controllerruntime.Result{}, nil
}
