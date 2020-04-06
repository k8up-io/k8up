package operator

import (
	"github.com/spotahome/kooper/client/crd"
	backupv1alpha1 "github.com/vshn/k8up/apis/backup/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

// pruneCRD is the archive pruneCRD
type pruneCRD struct {
	clients
}

func newPruneCRD(clients clients) *pruneCRD {
	return &pruneCRD{
		clients: clients,
	}
}

// Initialize satisfies resource.crd interface.
func (p *pruneCRD) Initialize() error {
	pruneCRD := crd.Conf{
		Kind:       backupv1alpha1.PruneKind,
		NamePlural: backupv1alpha1.PrunePlural,
		Group:      backupv1alpha1.SchemeGroupVersion.Group,
		Version:    backupv1alpha1.SchemeGroupVersion.Version,
		Scope:      backupv1alpha1.NamespaceScope,
	}

	return p.crdCli.EnsurePresent(pruneCRD)
}

// GetListerWatcher satisfies resource.crd interface (and retrieve.Retriever).
// All namespaces.
func (p *pruneCRD) GetListerWatcher() cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return p.baasCLI.AppuioV1alpha1().Prunes("").List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return p.baasCLI.AppuioV1alpha1().Prunes("").Watch(options)
		},
	}
}

// GetObject satisfies resource.crd interface (and retrieve.Retriever).
func (p *pruneCRD) GetObject() runtime.Object {
	return &backupv1alpha1.Prune{}
}
