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

	"k8s.io/apimachinery/pkg/types"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupReconciler reconciles a Backup object
type BackupReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=k8up.syn.tools,resources=backups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8up.syn.tools,resources=backups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;

func (r *BackupReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("backup", req.NamespacedName)

	backup := &k8upv1alpha1.Backup{}
	err := r.Get(ctx, req.NamespacedName, backup)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Backup resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Backup")
		return ctrl.Result{}, err
	}

	if backup.Status.Started {
		err := r.checkJob(ctx, backup, log)
		if err != nil {
			return ctrl.Result{Requeue: true}, nil
		}
	}

	job := &batchv1.Job{}
	err = r.Get(ctx, types.NamespacedName{Name: backup.Name, Namespace: backup.Namespace}, job)
	if err != nil && errors.IsNotFound(err) {
		// TODO: queue for execution
		log.Info("Queue up backup job")

		backup.Status.Started = true

		err := r.Status().Update(ctx, backup)
		if err != nil {
			log.Error(err, "Status cannot be updated")
		}

		job := r.backupJob(backup)
		err = r.Create(ctx, job)
		if err != nil {
			log.Error(err, "could not create job")
		}

		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Job")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *BackupReconciler) backupJob(backup *k8upv1alpha1.Backup) *batchv1.Job {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backup.Name,
			Namespace: backup.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "busybox",
							Command: []string{
								"sleep",
								"30",
							},
						},
					},
				},
			},
		},
	}

	err := ctrl.SetControllerReference(backup, job, r.Scheme)
	if err != nil {
		r.Log.Error(err, "Set controller reference")
	}

	return job
}

func (r *BackupReconciler) checkJob(ctx context.Context, backup *k8upv1alpha1.Backup, log logr.Logger) error {
	job := &batchv1.Job{}
	err := r.Get(ctx, types.NamespacedName{Name: backup.Name, Namespace: backup.Namespace}, job)
	if err != nil && errors.IsNotFound(err) {
		r.Log.Info("job is not yet ready")
		return err
	} else if err != nil {
		r.Log.Error(err, "could not get job")
		return err
	}

	if job.Status.Active > 0 {
		log.Info("job is running")
	}

	if job.Status.Succeeded > 0 {
		log.Info("job succeeded")
	}

	if job.Status.Failed > 0 {
		log.Info("job failed")
	}

	return nil

}

func (r *BackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8upv1alpha1.Backup{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
