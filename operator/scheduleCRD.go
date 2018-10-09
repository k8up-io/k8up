package operator

import (
	"github.com/spotahome/kooper/client/crd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
)

// scheduleCRD is the archive CRD
type scheduleCRD struct {
	clients
}

func newScheduleCRD(clients clients) *scheduleCRD {
	return &scheduleCRD{
		clients: clients,
	}
}

// Initialize satisfies resource.crd interface.
func (s *scheduleCRD) Initialize() error {
	scheduleCRD := crd.Conf{
		Kind:       backupv1alpha1.ScheduleKind,
		NamePlural: backupv1alpha1.SchedulePlural,
		Group:      backupv1alpha1.SchemeGroupVersion.Group,
		Version:    backupv1alpha1.SchemeGroupVersion.Version,
		Scope:      backupv1alpha1.NamespaceScope,
	}

	return s.crdCli.EnsurePresent(scheduleCRD)
}

// GetListerWatcher satisfies resource.crd interface (and retrieve.Retriever).
// All namespaces.
func (s *scheduleCRD) GetListerWatcher() cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return s.baasCLI.AppuioV1alpha1().Schedules("").List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return s.baasCLI.AppuioV1alpha1().Schedules("").Watch(options)
		},
	}
}

// GetObject satisfies resource.crd interface (and retrieve.Retriever).
func (s *scheduleCRD) GetObject() runtime.Object {
	return &backupv1alpha1.Schedule{}
}
