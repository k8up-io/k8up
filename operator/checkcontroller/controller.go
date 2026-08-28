package checkcontroller

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

// CheckReconciler reconciles a Check object
type CheckReconciler struct {
	Kube client.Client
	// KubeReader performs direct API reads so Secrets only require named GET permissions, not cache list/watch permissions.
	KubeReader client.Reader
}

func (r *CheckReconciler) NewObject() *k8upv1.Check {
	return &k8upv1.Check{}
}

func (r *CheckReconciler) NewObjectList() *k8upv1.CheckList {
	return &k8upv1.CheckList{}
}

func (r *CheckReconciler) Provision(ctx context.Context, obj *k8upv1.Check) (controllerruntime.Result, error) {
	log := controllerruntime.LoggerFrom(ctx)

	if obj.Spec.Backend != nil {
		if err := obj.Spec.Backend.ResolveSecretRefs(ctx, r.KubeReader, obj.GetNamespace(), cfg.Config.RepositorySecretName); err != nil {
			return controllerruntime.Result{}, fmt.Errorf("resolve backend Secret references: %w", err)
		}
	}

	repository := cfg.Config.GetGlobalRepository()
	if obj.Spec.Backend != nil {
		repository = obj.Spec.Backend.String()
	}

	config := job.NewConfig(r.Kube, obj, repository)

	executor := NewCheckExecutor(config)

	jobKey := types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      executor.jobName(),
	}
	if err := job.ReconcileJobStatus(ctx, jobKey, r.Kube, obj); err != nil {
		return controllerruntime.Result{}, err
	}

	if obj.Status.HasStarted() {
		log.V(1).Info("check just started, waiting")
		return controllerruntime.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if obj.Status.HasFinished() {
		executor.cleanupOldChecks(ctx, obj)
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
