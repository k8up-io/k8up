package checkcontroller

import (
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/locker"
	"github.com/k8up-io/k8up/v2/operator/reconciler"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// +kubebuilder:rbac:groups=k8up.io,resources=checks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8up.io,resources=checks/status;checks/finalizers,verbs=get;update;patch

// SetupWithManager configures the reconciler.
func SetupWithManager(mgr ctrl.Manager) error {
	name := "check.k8up.io"
	r := reconciler.NewReconciler[*k8upv1.Check, *k8upv1.CheckList](mgr.GetClient(), &CheckReconciler{
		Kube:   mgr.GetClient(),
		Locker: &locker.Locker{Kube: mgr.GetClient()},
	})
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8upv1.Check{}).
		Named(name).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
