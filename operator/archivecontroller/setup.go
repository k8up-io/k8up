package archivecontroller

import (
	v1 "github.com/k8up-io/k8up/v2/api/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// +kubebuilder:rbac:groups=k8up.io,resources=archives,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8up.io,resources=archives/status;archives/finalizers,verbs=get;update;patch

// SetupWithManager configures the reconciler.
func (r *ArchiveReconciler) SetupWithManager(mgr controllerruntime.Manager) error {
	name := "archive.k8up.io"
	r.Kube = mgr.GetClient()
	return controllerruntime.NewControllerManagedBy(mgr).
		For(&v1.Archive{}).
		Named(name).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
