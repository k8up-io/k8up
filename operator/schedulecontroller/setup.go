package schedulecontroller

import (
	"github.com/go-logr/logr"
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// +kubebuilder:rbac:groups=k8up.io,resources=schedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8up.io,resources=schedules/status;schedules/finalizers,verbs=get;update;patch
// The following permissions are just for backwards compatibility.
// +kubebuilder:rbac:groups=k8up.io,resources=effectiveschedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8up.io,resources=effectiveschedules/finalizers,verbs=update

// SetupWithManager configures the reconciler.
func (r *ScheduleReconciler) SetupWithManager(mgr ctrl.Manager, _ logr.Logger) error {
	name := "schedule.k8up.io"
	r.Kube = mgr.GetClient()
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8upv1.Schedule{}).
		Named(name).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
