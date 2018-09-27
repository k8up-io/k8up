package service

import "k8s.io/apimachinery/pkg/runtime"

// CRDEnsurer is the interface every CRD operator has to implement
type CRDEnsurer interface {
	// Ensure will ensure that the service is correcly registered
	Ensure(pt runtime.Object) error
	// Delete will stop and delete the object from the operator. Kubernetes will
	// handle the deletion of all child items.
	Delete(name string) error
}

// Runner is an interface that a backup service has to
// satisfy
type Runner interface {
	Stop() error
	SameSpec(object runtime.Object) bool
	Start() error
}
