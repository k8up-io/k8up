package controllers

import (
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;create;update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// ReconcilerSetup is a common interface to configure reconcilers.
type ReconcilerSetup interface {
	SetupWithManager(mgr ctrl.Manager, l logr.Logger) error
}
