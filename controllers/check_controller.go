package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	k8upv1 "github.com/k8up-io/k8up/api/v1"
	"github.com/k8up-io/k8up/operator/cfg"
	"github.com/k8up-io/k8up/operator/handler"
	"github.com/k8up-io/k8up/operator/job"
)

// CheckReconciler reconciles a Check object
type CheckReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=k8up.io,resources=checks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8up.io,resources=checks/status;checks/finalizers,verbs=get;update;patch

// Reconcile is the entrypoint to manage the given resource.
func (r *CheckReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("check", req.NamespacedName)

	check := &k8upv1.Check{}
	err := r.Get(ctx, req.NamespacedName, check)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Check")
		return ctrl.Result{}, err
	}

	if check.Status.HasFinished() {
		return ctrl.Result{}, nil
	}

	repository := cfg.Config.GetGlobalRepository()
	if check.Spec.Backend != nil {
		repository = check.Spec.Backend.String()
	}

	config := job.NewConfig(ctx, r.Client, logger, check, r.Scheme, repository)

	checkHandler := handler.NewHandler(config)
	return ctrl.Result{RequeueAfter: time.Second * 30}, checkHandler.Handle()
}

// SetupWithManager configures the reconciler.
func (r *CheckReconciler) SetupWithManager(mgr ctrl.Manager, l logr.Logger) error {
	r.Client = mgr.GetClient()
	r.Scheme = mgr.GetScheme()
	r.Log = l
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8upv1.Check{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
