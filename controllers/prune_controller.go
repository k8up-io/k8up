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
// +kubebuilder:rbac:groups=backup.appuio.ch,resources=prunes/status,verbs=get;update;patch

func (r *PruneReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("prune", req.NamespacedName)

	prune := &k8upv1alpha1.Prune{}
	err := r.Get(ctx, req.NamespacedName, prune)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if prune.Status.Started {
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

func (r *PruneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8upv1alpha1.Prune{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
