package operator

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

// podObserve is a dummy "CRD"
type podObserve struct {
	clients
}

func newPodObserve(clients clients) *podObserve {
	return &podObserve{
		clients: clients,
	}
}

func (p *podObserve) Initialize() error {
	return nil
}

// GetListerWatcher satisfies resource.crd interface (and retrieve.Retriever).
// All namespaces.
func (p *podObserve) GetListerWatcher() cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return p.kubeCli.CoreV1().Pods("").List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return p.kubeCli.CoreV1().Pods("").Watch(options)
		},
	}
}

// GetObject satisfies resource.crd interface (and retrieve.Retriever).
func (p *podObserve) GetObject() runtime.Object {
	return &corev1.Pod{}
}
