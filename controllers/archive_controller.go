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

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/handler"
	"github.com/k8up-io/k8up/v2/operator/job"
)

// ArchiveReconciler reconciles a Archive object
type ArchiveReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=k8up.io,resources=archives,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8up.io,resources=archives/status;archives/finalizers,verbs=get;update;patch

// Reconcile is the entrypoint to manage the given resource.
func (r *ArchiveReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("archive", req.NamespacedName)

	archive := &k8upv1.Archive{}
	err := r.Get(ctx, req.NamespacedName, archive)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Archive")
		return ctrl.Result{}, err
	}

	if archive.Status.HasFinished() {
		return ctrl.Result{}, nil
	}

	repository := cfg.Config.GetGlobalRepository()
	if archive.Spec.Backend != nil {
		repository = archive.Spec.Backend.String()
	}
	if archive.Spec.RestoreSpec == nil {
		archive.Spec.RestoreSpec = &k8upv1.RestoreSpec{}
	}
	config := job.NewConfig(ctx, r.Client, log, archive, r.Scheme, repository)

	archiveHandler := handler.NewHandler(config)
	return ctrl.Result{RequeueAfter: time.Second * 30}, archiveHandler.Handle()
}

// SetupWithManager configures the reconciler.
func (r *ArchiveReconciler) SetupWithManager(mgr ctrl.Manager, l logr.Logger) error {
	r.Client = mgr.GetClient()
	r.Scheme = mgr.GetScheme()
	r.Log = l
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8upv1.Archive{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
