package operator

import (
	"github.com/spotahome/kooper/client/crd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	baas8scli "git.vshn.net/vshn/baas/client/k8s/clientset/versioned"
)

// baasCRD is the baas CRD
type baasCRD struct {
	crdCli   crd.Interface
	kubecCli kubernetes.Interface
	baasCli  baas8scli.Interface
}

func newBaasCRD(baasCli baas8scli.Interface, crdCli crd.Interface, kubeCli kubernetes.Interface) *baasCRD {
	return &baasCRD{
		crdCli:   crdCli,
		baasCli:  baasCli,
		kubecCli: kubeCli,
	}
}

// baasCRD satisfies resource.crd interface.
func (p *baasCRD) Initialize() error {
	baasCRD := crd.Conf{
		Kind:       backupv1alpha1.BackupKind,
		NamePlural: backupv1alpha1.BackupPlural,
		Group:      backupv1alpha1.SchemeGroupVersion.Group,
		Version:    backupv1alpha1.SchemeGroupVersion.Version,
		Scope:      backupv1alpha1.BackupScope,
	}

	return p.crdCli.EnsurePresent(baasCRD)
}

// GetListerWatcher satisfies resource.crd interface (and retrieve.Retriever).
// All namespaces.
func (p *baasCRD) GetListerWatcher() cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return p.baasCli.AppuioV1alpha1().Backups("").List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return p.baasCli.AppuioV1alpha1().Backups("").Watch(options)
		},
	}
}

// GetObject satisfies resource.crd interface (and retrieve.Retriever).
func (p *baasCRD) GetObject() runtime.Object {
	return &backupv1alpha1.Backup{}
}
