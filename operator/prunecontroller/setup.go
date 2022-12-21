package prunecontroller

import (
	"github.com/go-logr/logr"
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// +kubebuilder:rbac:groups=k8up.io,resources=prunes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8up.io,resources=prunes/status;prunes/finalizers,verbs=get;update;patch

// SetupWithManager configures the reconciler.
func (r *PruneReconciler) SetupWithManager(mgr ctrl.Manager, l logr.Logger) error {
	name := "prune.k8up.io"
	r.Kube = mgr.GetClient()
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8upv1.Prune{}).
		Named(name).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
