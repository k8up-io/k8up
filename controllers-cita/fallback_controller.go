package controllers_cita

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/k8up-io/k8up/v2/api/v1cita"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/handler"
	"github.com/k8up-io/k8up/v2/operator/job"
)

// BlockHeightFallbackReconciler reconciles a BlockHeightFallback object
type BlockHeightFallbackReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=rivtower.com,resources=blockheightfallbacks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rivtower.com,resources=blockheightfallbacks/status,verbs=get;update;patch

func (r *BlockHeightFallbackReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("fallback", req.NamespacedName)

	bhf := &v1cita.BlockHeightFallback{}
	err := r.Get(ctx, req.NamespacedName, bhf)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get BlockHeightFallback")
		return ctrl.Result{}, err
	}

	if bhf.Status.HasFinished() {
		return ctrl.Result{}, nil
	}

	repository := cfg.Config.GetGlobalRepository()
	if bhf.Spec.Backend != nil {
		repository = bhf.Spec.Backend.String()
	}
	config := job.NewConfig(ctx, r.Client, log, bhf, r.Scheme, repository)

	backupHandler := handler.NewHandler(config)
	return ctrl.Result{RequeueAfter: time.Second * 30}, backupHandler.Handle()
}

func (r *BlockHeightFallbackReconciler) SetupWithManager(mgr ctrl.Manager, l logr.Logger) error {
	r.Client = mgr.GetClient()
	r.Scheme = mgr.GetScheme()
	r.Log = l
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1cita.BlockHeightFallback{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
