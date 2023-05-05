package backupcontroller

import (
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/reconciler"
	batchv1 "k8s.io/api/batch/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// +kubebuilder:rbac:groups=k8up.io,resources=backups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8up.io,resources=backups/status;backups/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8up.io,resources=prebackuppods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8up.io,resources=prebackuppods/status;prebackuppods/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=list;get;watch
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=persistentvolumes,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;delete;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=bind,resourceNames=k8up-executor

// SetupWithManager configures the reconciler.
func SetupWithManager(mgr controllerruntime.Manager) error {
	name := "backup.k8up.io"
	r := reconciler.NewReconciler[*k8upv1.Backup, *k8upv1.BackupList](mgr.GetClient(), &BackupReconciler{
		Kube: mgr.GetClient(),
	})
	return controllerruntime.NewControllerManagedBy(mgr).
		Named(name).
		For(&k8upv1.Backup{}).
		Owns(&batchv1.Job{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
