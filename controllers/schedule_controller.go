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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/operator/cfg"
	"github.com/vshn/k8up/operator/handler"
	"github.com/vshn/k8up/operator/job"
)

// ScheduleReconciler reconciles a Schedule object
type ScheduleReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=backup.appuio.ch,resources=schedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.appuio.ch,resources=schedules/status;schedules/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.appuio.ch,resources=effectiveschedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.appuio.ch,resources=effectiveschedules/finalizers,verbs=update

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

	effectiveSchedules, err := r.fetchEffectiveSchedules(ctx, schedule)
	if err != nil {
		requeueAfter := 60 * time.Second
		r.Log.Info("could not retrieve list of effective schedules", "error", err.Error(), "retry_after", requeueAfter)
		return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, err
	}

	repository := cfg.Config.GetGlobalRepository()
	if schedule.Spec.Backend != nil {
		repository = schedule.Spec.Backend.String()
	}
	config := job.NewConfig(ctx, r.Client, log, schedule, r.Scheme, repository)

	return ctrl.Result{}, handler.NewScheduleHandler(config, schedule, effectiveSchedules).Handle()
}

// SetupWithManager configures the reconciler.
func (r *ScheduleReconciler) SetupWithManager(mgr ctrl.Manager, l logr.Logger) error {
	r.Client = mgr.GetClient()
	r.Scheme = mgr.GetScheme()
	r.Log = l
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8upv1alpha1.Schedule{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}

// fetchEffectiveSchedules retrieves a list of EffectiveSchedules and filter the one that matches the given schedule.
// Returns an error if the listing failed, but empty map when no matching EffectiveSchedule object was found.
func (r *ScheduleReconciler) fetchEffectiveSchedules(ctx context.Context, schedule *k8upv1alpha1.Schedule) (map[k8upv1alpha1.JobType]k8upv1alpha1.EffectiveSchedule, error) {
	list := k8upv1alpha1.EffectiveScheduleList{}
	err := r.Client.List(ctx, &list, client.InNamespace(cfg.Config.OperatorNamespace))
	if err != nil {
		return map[k8upv1alpha1.JobType]k8upv1alpha1.EffectiveSchedule{}, err
	}
	return filterEffectiveSchedulesForReferencesOfSchedule(list, schedule), nil
}

// filterEffectiveSchedulesForReferencesOfSchedule iterates over the given list of EffectiveSchedules and returns results that reference the given schedule in their spec.
// It returns an empty map if no element matches.
func filterEffectiveSchedulesForReferencesOfSchedule(list k8upv1alpha1.EffectiveScheduleList, schedule *k8upv1alpha1.Schedule) map[k8upv1alpha1.JobType]k8upv1alpha1.EffectiveSchedule {
	filtered := map[k8upv1alpha1.JobType]k8upv1alpha1.EffectiveSchedule{}
	for _, es := range list.Items {
		if es.GetDeletionTimestamp() != nil {
			continue
		}
		for _, jobRef := range es.Spec.ScheduleRefs {
			if schedule.IsReferencedBy(jobRef) {
				filtered[es.Spec.JobType] = es
			}
		}
	}
	return filtered
}
