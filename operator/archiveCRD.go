package operator

import (
	"github.com/spotahome/kooper/client/crd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
)

// archiveCRD is the archive CRD
type archiveCRD struct {
	clients
}

func newArchiveCRD(clients clients) *archiveCRD {
	return &archiveCRD{
		clients: clients,
	}
}

// Initialize satisfies resource.crd interface.
func (a *archiveCRD) Initialize() error {
	archiveCRD := crd.Conf{
		Kind:       backupv1alpha1.ArchiveKind,
		NamePlural: backupv1alpha1.ArchivePlural,
		Group:      backupv1alpha1.SchemeGroupVersion.Group,
		Version:    backupv1alpha1.SchemeGroupVersion.Version,
		Scope:      backupv1alpha1.NamespaceScope,
	}

	return a.crdCli.EnsurePresent(archiveCRD)
}

// GetListerWatcher satisfies resource.crd interface (and retrieve.Retriever).
// All namespaces.
func (a *archiveCRD) GetListerWatcher() cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return a.baasCLI.AppuioV1alpha1().Archives("").List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return a.baasCLI.AppuioV1alpha1().Archives("").Watch(options)
		},
	}
}

// GetObject satisfies resource.crd interface (and retrieve.Retriever).
func (a *archiveCRD) GetObject() runtime.Object {
	return &backupv1alpha1.Archive{}
}
