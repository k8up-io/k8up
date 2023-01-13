package schedulecontroller

import (
	"context"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
	"github.com/k8up-io/k8up/v2/operator/scheduler"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ScheduleReconciler reconciles a Schedule object
type ScheduleReconciler struct {
	Kube client.Client
}

func (r *ScheduleReconciler) NewObject() *k8upv1.Schedule {
	return &k8upv1.Schedule{}
}

func (r *ScheduleReconciler) NewObjectList() *k8upv1.ScheduleList {
	return &k8upv1.ScheduleList{}
}

func (r *ScheduleReconciler) Provision(ctx context.Context, schedule *k8upv1.Schedule) (controllerruntime.Result, error) {
	log := controllerruntime.LoggerFrom(ctx)

	repository := cfg.Config.GetGlobalRepository()
	if schedule.Spec.Backend != nil {
		repository = schedule.Spec.Backend.String()
	}
	if schedule.Spec.Archive != nil && schedule.Spec.Archive.RestoreSpec == nil {
		schedule.Spec.Archive.RestoreSpec = &k8upv1.RestoreSpec{}
	}
	config := job.NewConfig(r.Kube, schedule, repository)

	return controllerruntime.Result{}, NewScheduleHandler(config, schedule, log).Handle(ctx)
}

func (r *ScheduleReconciler) Deprovision(ctx context.Context, obj *k8upv1.Schedule) (controllerruntime.Result, error) {
	for _, jobType := range []k8upv1.JobType{k8upv1.PruneType, k8upv1.ArchiveType, k8upv1.RestoreType, k8upv1.BackupType, k8upv1.CheckType} {
		key := keyOf(obj, jobType)
		scheduler.GetScheduler().RemoveSchedule(ctx, key)
	}
	controllerutil.RemoveFinalizer(obj, k8upv1.ScheduleFinalizerName)
	return controllerruntime.Result{}, r.Kube.Update(ctx, obj)
}
