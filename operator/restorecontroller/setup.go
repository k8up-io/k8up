package restorecontroller

import (
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// +kubebuilder:rbac:groups=k8up.io,resources=restores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8up.io,resources=restores/status;restores/finalizers,verbs=get;update;patch

// SetupWithManager configures the reconciler.
func SetupWithManager(mgr controllerruntime.Manager) error {
	name := "restore.k8up.io"
	r := &RestoreReconciler{Kube: mgr.GetClient()}
	return controllerruntime.NewControllerManagedBy(mgr).
		For(&k8upv1.Restore{}).
		Named(name).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
