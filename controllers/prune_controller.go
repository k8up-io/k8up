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
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/constants"
	"github.com/vshn/k8up/handler"
	"github.com/vshn/k8up/job"
)

// PruneReconciler reconciles a Prune object
type PruneReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=backup.appuio.ch,resources=prunes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.appuio.ch,resources=prunes/status,verbs=get;update;patch

func (r *PruneReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("prune", req.NamespacedName)

	prune := &k8upv1alpha1.Prune{}
	err := r.Get(ctx, req.NamespacedName, prune)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if prune.Status.Started {
		return ctrl.Result{}, nil
	}

	repository := constants.GetGlobalRepository()
	if prune.Spec.Backend != nil {
		repository = prune.Spec.Backend.String()
	}
	config := job.NewConfig(ctx, r.Client, log, prune, r.Scheme, repository)

	pruneHandler := handler.NewHandler(config)

	return ctrl.Result{RequeueAfter: time.Second * 30}, pruneHandler.Handle()
}

func (r *PruneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8upv1alpha1.Prune{}).
		Complete(r)
}
