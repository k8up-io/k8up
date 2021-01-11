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

// RestoreReconciler reconciles a Restore object
type RestoreReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=backup.appuio.ch,resources=restores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.appuio.ch,resources=restores/status,verbs=get;update;patch

func (r *RestoreReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("restore", req.NamespacedName)

	restore := &k8upv1alpha1.Restore{}
	err := r.Get(ctx, req.NamespacedName, restore)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if restore.Status.Started {
		return ctrl.Result{}, nil
	}

	repository := cfg.GetGlobalRepository()
	if restore.Spec.Backend != nil {
		repository = restore.Spec.Backend.String()
	}
	config := job.NewConfig(ctx, r.Client, log, restore, r.Scheme, repository)

	restoreHandler := handler.NewHandler(config)

	return ctrl.Result{RequeueAfter: time.Second * 30}, restoreHandler.Handle()
}

func (r *RestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8upv1alpha1.Restore{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
