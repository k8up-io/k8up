/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/vshn/k8up/observer"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	jobFinalizerName string = "jobObserver.k8up.syn.tools"
)

// JobReconciler reconciles a Job object
type JobReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get;update;patch

func (r *JobReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("job", req.NamespacedName)

	job := &batchv1.Job{}

	//TODO: logic to figure out if the job belongs to k8up

	jobEvent := observer.Create

	err := r.Client.Get(ctx, req.NamespacedName, job)
	if err != nil {
		return reconcile.Result{}, err
	}

	if job.GetDeletionTimestamp() != nil && contains(job.GetFinalizers(), jobFinalizerName) {
		jobEvent = observer.Delete
		err := r.addFinalizer(ctx, log, job)
		if err != nil {
			if errors.IsNotFound(err) {
				log.Error(err, "job was not found")
				return reconcile.Result{}, nil
			}
			return reconcile.Result{}, err
		}
	} else {
		if job.Status.Active > 0 {
			jobEvent = observer.Running
		}

		if job.Status.Succeeded > 0 {
			jobEvent = observer.Suceeded
		}

		if job.Status.Failed > 0 {
			jobEvent = observer.Failed
		}

		err := r.addFinalizer(ctx, log, job)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	oj := observer.ObservableJob{
		Job:       job,
		Exclusive: false,
		Event:     jobEvent,
	}

	observer.GetObserver().GetUpdateChannel() <- oj

	log.Info("job detected")

	return ctrl.Result{}, nil
}

func (r *JobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.Job{}).
		Complete(r)
}

func (r *JobReconciler) addFinalizer(ctx context.Context, reqLogger logr.Logger, j *batchv1.Job) error {
	reqLogger.Info("adding Finalizer for the job")
	controllerutil.AddFinalizer(j, jobFinalizerName)

	// Update CR
	err := r.Client.Update(ctx, j)
	if err != nil {
		reqLogger.Error(err, "failed to update job with finalizer")
		return err
	}
	return nil
}

func (r *JobReconciler) removeFinalizer(ctx context.Context, reqLogger logr.Logger, j *batchv1.Job) error {
	controllerutil.RemoveFinalizer(j, jobFinalizerName)
	err := r.Client.Update(ctx, j)
	if err != nil {
		return err
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
