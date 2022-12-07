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

// SwitchoverReconciler reconciles a Switchover object
type SwitchoverReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=rivtower.com,resources=switchovers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rivtower.com,resources=switchovers/status,verbs=get;update;patch

func (r *SwitchoverReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("switchover", req.NamespacedName)

	switchover := &v1cita.Switchover{}
	err := r.Get(ctx, req.NamespacedName, switchover)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Switchover")
		return ctrl.Result{}, err
	}

	if switchover.Status.HasFinished() {
		return ctrl.Result{}, nil
	}

	repository := cfg.Config.GetGlobalRepository()
	if switchover.Spec.Backend != nil {
		repository = switchover.Spec.Backend.String()
	}
	config := job.NewConfig(ctx, r.Client, log, switchover, r.Scheme, repository)

	backupHandler := handler.NewHandler(config)
	return ctrl.Result{RequeueAfter: time.Second * 10}, backupHandler.Handle()
}

func (r *SwitchoverReconciler) SetupWithManager(mgr ctrl.Manager, l logr.Logger) error {
	r.Client = mgr.GetClient()
	r.Scheme = mgr.GetScheme()
	r.Log = l
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1cita.Switchover{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
