package operator

import (
	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	"github.com/spotahome/kooper/client/crd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

// checkCRD is the archive checkCRD
type checkCRD struct {
	clients
}

func newCheckCRD(clients clients) *checkCRD {
	return &checkCRD{
		clients: clients,
	}
}

// Initialize satisfies resource.crd interface.
func (c *checkCRD) Initialize() error {
	checkCRD := crd.Conf{
		Kind:       backupv1alpha1.CheckKind,
		NamePlural: backupv1alpha1.CheckPlural,
		Group:      backupv1alpha1.SchemeGroupVersion.Group,
		Version:    backupv1alpha1.SchemeGroupVersion.Version,
		Scope:      backupv1alpha1.NamespaceScope,
	}

	return c.crdCli.EnsurePresent(checkCRD)
}

// GetListerWatcher satisfies resource.crd interface (and retrieve.Retriever).
// All namespaces.
func (c *checkCRD) GetListerWatcher() cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return c.baasCLI.AppuioV1alpha1().Checks("").List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return c.baasCLI.AppuioV1alpha1().Checks("").Watch(options)
		},
	}
}

// GetObject satisfies resource.crd interface (and retrieve.Retriever).
func (c *checkCRD) GetObject() runtime.Object {
	return &backupv1alpha1.Check{}
}
