package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/cfg"
	"github.com/vshn/k8up/handler"
	"github.com/vshn/k8up/job"
)

// ScheduleReconciler reconciles a Schedule object
type ScheduleReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=backup.appuio.ch,resources=schedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.appuio.ch,resources=schedules/status,verbs=get;update;patch

// Reconcile is the entrypoint to manage the given resource.
func (r *ScheduleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("schedule", req.NamespacedName)

	schedule := &k8upv1alpha1.Schedule{}
	err := r.Client.Get(ctx, req.NamespacedName, schedule)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	repository := cfg.GetGlobalRepository()
	if schedule.Spec.Backend != nil {
		repository = schedule.Spec.Backend.String()
	}
	config := job.NewConfig(ctx, r.Client, log, schedule, r.Scheme, repository)

	return ctrl.Result{}, handler.NewScheduleHandler(config, schedule).Handle()
}

func (r *ScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8upv1alpha1.Schedule{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
