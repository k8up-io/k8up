package schedulecontroller

import (
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/reconciler"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// +kubebuilder:rbac:groups=k8up.io,resources=schedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8up.io,resources=schedules/status;schedules/finalizers,verbs=get;update;patch
// The following permissions are just for backwards compatibility.
// +kubebuilder:rbac:groups=k8up.io,resources=effectiveschedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8up.io,resources=effectiveschedules/finalizers,verbs=update

// SetupWithManager configures the reconciler.
func SetupWithManager(mgr ctrl.Manager) error {
	name := "schedule.k8up.io"
	r := reconciler.NewReconciler[*k8upv1.Schedule, *k8upv1.ScheduleList](mgr.GetClient(), &ScheduleReconciler{
		Kube: mgr.GetClient(),
	})
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8upv1.Schedule{}).
		Named(name).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
