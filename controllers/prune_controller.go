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

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/cfg"
	"github.com/vshn/k8up/handler"
	"github.com/vshn/k8up/job"
)

// PruneReconciler reconciles a Prune object
type PruneReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=backup.appuio.ch,resources=prunes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.appuio.ch,resources=prunes/status;prunes/finalizers,verbs=get;update;patch

// Reconcile is the entrypoint to manage the given resource.
func (r *PruneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("prune", req.NamespacedName)

	prune := &k8upv1alpha1.Prune{}
	err := r.Get(ctx, req.NamespacedName, prune)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if prune.Status.HasFinished() || prune.Status.HasStarted() {
		return ctrl.Result{}, nil
	}

	repository := cfg.GetGlobalRepository()
	if prune.Spec.Backend != nil {
		repository = prune.Spec.Backend.String()
	}
	config := job.NewConfig(ctx, r.Client, log, prune, r.Scheme, repository)

	pruneHandler := handler.NewHandler(config)
	return ctrl.Result{RequeueAfter: time.Second * 30}, pruneHandler.Handle()
}

// SetupWithManager configures the reconciler.
func (r *PruneReconciler) SetupWithManager(mgr ctrl.Manager, l logr.Logger) error {
	r.Client = mgr.GetClient()
	r.Scheme = mgr.GetScheme()
	r.Log = l
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8upv1alpha1.Prune{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
