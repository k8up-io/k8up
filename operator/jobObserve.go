package operator

import (
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

// jobObserve is a dummy "CRD"
type jobObserve struct {
	clients
}

func newJobObserve(clients clients) *jobObserve {
	return &jobObserve{
		clients: clients,
	}
}

func (j *jobObserve) Initialize() error {
	return nil
}

// GetListerWatcher satisfies resource.crd interface (and retrieve.Retriever).
// All namespaces.
func (j *jobObserve) GetListerWatcher() cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return j.kubeCli.BatchV1().Jobs("").List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return j.kubeCli.BatchV1().Jobs("").Watch(options)
		},
	}
}

// GetObject satisfies resource.crd interface (and retrieve.Retriever).
func (j *jobObserve) GetObject() runtime.Object {
	return &batchv1.Job{}
}
