package operator

import (
	"github.com/spotahome/kooper/client/crd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
)

// backupCRD is the baas CRD
type backupCRD struct {
	clients
}

func newBackupCRD(clients clients) *backupCRD {
	return &backupCRD{
		clients: clients,
	}
}

// backupCRD satisfies resource.crd interface.
func (p *backupCRD) Initialize() error {
	backupCRD := crd.Conf{
		Kind:       backupv1alpha1.BackupKind,
		NamePlural: backupv1alpha1.BackupPlural,
		Group:      backupv1alpha1.SchemeGroupVersion.Group,
		Version:    backupv1alpha1.SchemeGroupVersion.Version,
		Scope:      backupv1alpha1.NamespaceScope,
	}

	return p.crdCli.EnsurePresent(backupCRD)
}

// GetListerWatcher satisfies resource.crd interface (and retrieve.Retriever).
// All namespaces.
func (p *backupCRD) GetListerWatcher() cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return p.baasCLI.AppuioV1alpha1().Backups("").List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return p.baasCLI.AppuioV1alpha1().Backups("").Watch(options)
		},
	}
}

// GetObject satisfies resource.crd interface (and retrieve.Retriever).
func (p *backupCRD) GetObject() runtime.Object {
	return &backupv1alpha1.Backup{}
}
