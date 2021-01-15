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

// ArchiveReconciler reconciles a Archive object
type ArchiveReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=backup.appuio.ch,resources=archives,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.appuio.ch,resources=archives/status,verbs=get;update;patch

// Reconcile is the entrypoint to manage the given resource.
func (r *ArchiveReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("archive", req.NamespacedName)

	archive := &k8upv1alpha1.Archive{}
	err := r.Get(ctx, req.NamespacedName, archive)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Archive")
		return ctrl.Result{}, err
	}

	repository := cfg.GetGlobalRepository()
	if archive.Spec.Backend != nil {
		repository = archive.Spec.Backend.String()
	}
	config := job.NewConfig(ctx, r.Client, log, archive, r.Scheme, repository)

	archiveHandler := handler.NewHandler(config)
	return ctrl.Result{RequeueAfter: time.Second * 30}, archiveHandler.Handle()
}

func (r *ArchiveReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8upv1alpha1.Archive{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
