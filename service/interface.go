package service

import "k8s.io/apimachinery/pkg/runtime"

// Handler is the interface a service has to implement. These are the functions
// that get triggered by the kooper framework as soon as a change is found or
// after the configured resync period.
type Handler interface {
	// Ensure will ensure that the service is correcly registered in a schedule
	Ensure(pt runtime.Object) error
	// Delete will stop and delete the object from the operator. Kubernetes will
	// handle the deletion of all child items.
	Delete(name string) error
}

// Runner is an interface that a backup service has to
// satisfy. Runners actually DO the jobs (restore,archive, backup, etc).
type Runner interface {
	Stop() error
	// SameSpec should check if the spec has changed and the runner has to be
	// recreated. This is mostly necessary for long running things like the
	// scheduler. One-time jobs don't usually need this and can just return
	// true.
	SameSpec(object runtime.Object) bool
	Start() error
}
