package controllers

import (
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcilerSetup is a common interface to configure reconcilers.
type ReconcilerSetup interface {
	SetupWithManager(mgr ctrl.Manager, l logr.Logger) error
}
