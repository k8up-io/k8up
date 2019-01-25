package operator

import (
	backupv1alpha1 "github.com/vshn/k8up/apis/backup/v1alpha1"
	"github.com/spotahome/kooper/client/crd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

// restoreCRD
type restoreCRD struct {
	clients
}

func newRestoreCRD(clients clients) *restoreCRD {
	return &restoreCRD{
		clients: clients,
	}
}

// restoreCRD satisfies resource.crd interface.
func (r *restoreCRD) Initialize() error {
	restoreCRD := crd.Conf{
		Kind:       backupv1alpha1.RestoreKind,
		NamePlural: backupv1alpha1.RestorePlural,
		Group:      backupv1alpha1.SchemeGroupVersion.Group,
		Version:    backupv1alpha1.SchemeGroupVersion.Version,
		Scope:      backupv1alpha1.NamespaceScope,
	}

	return r.crdCli.EnsurePresent(restoreCRD)
}

// GetListerWatcher satisfies resource.crd interface (and retrieve.Retriever).
// All namespaces.
func (r *restoreCRD) GetListerWatcher() cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return r.baasCLI.AppuioV1alpha1().Restores("").List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return r.baasCLI.AppuioV1alpha1().Restores("").Watch(options)
		},
	}
}

// GetObject satisfies resource.crd interface (and retrieve.Retriever).
func (r *restoreCRD) GetObject() runtime.Object {
	return &backupv1alpha1.Restore{}
}
