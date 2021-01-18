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

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/cfg"
	"github.com/vshn/k8up/handler"
	"github.com/vshn/k8up/job"
)

// BackupReconciler reconciles a Backup object
type BackupReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=backup.appuio.ch,resources=backups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.appuio.ch,resources=backups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.appuio.ch,resources=prebackuppods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.appuio.ch,resources=prebackuppods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs="*"
// +kubebuilder:rbac:groups=core,resources=pods/exec,verbs="*"
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;create;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;create;delete

// Reconcile is the entrypoint to manage the given resource.
func (r *BackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("backup", req.NamespacedName)

	backup := &k8upv1alpha1.Backup{}
	err := r.Get(ctx, req.NamespacedName, backup)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Backup")
		return ctrl.Result{}, err
	}

	repository := cfg.GetGlobalRepository()
	if backup.Spec.Backend != nil {
		repository = backup.Spec.Backend.String()
	}
	config := job.NewConfig(ctx, r.Client, log, backup, r.Scheme, repository)

	backupHandler := handler.NewHandler(config)

	return ctrl.Result{RequeueAfter: time.Second * 30}, backupHandler.Handle()
}

func (r *BackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8upv1alpha1.Backup{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
