package backup

import (
	"fmt"
	"strings"
	"sync"

	"git.vshn.net/vshn/baas/monitoring"
	"git.vshn.net/vshn/baas/service"

	"git.vshn.net/vshn/baas/log"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	baas8scli "git.vshn.net/vshn/baas/client/k8s/clientset/versioned"
	cron "github.com/Infowatch/cron"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

// Schedule will ensure that the backups are running accordingly.
type Schedule struct {
	k8sCli           kubernetes.Interface
	baasCLI          baas8scli.Interface
	reg              sync.Map
	logger           log.Logger
	cron             *cron.Cron
	metrics          *operatorMetrics
	clusterWideState clusterWideState
	config           config
}

// NewBackup returns a new schedule.
func NewBackup(k8sCli kubernetes.Interface, baasCLI baas8scli.Interface, logger log.Logger) *Schedule {
	cron := cron.New()
	cron.Start()
	metrics := newOperatorMetrics(monitoring.GetInstance())
	return &Schedule{
		k8sCli:  k8sCli,
		baasCLI: baasCLI,
		reg:     sync.Map{},
		logger:  logger,
		cron:    cron,
		metrics: metrics,
		clusterWideState: clusterWideState{
			repoMap:               &sync.Map{},
			runningBackupsPerRepo: &sync.Map{},
			runningPruneOnRepo:    &sync.Map{},
		},
		config: newConfig(),
	}
}

func (s *Schedule) checkObject(obj runtime.Object) (*backupv1alpha1.Backup, error) {
	backup, ok := obj.(*backupv1alpha1.Backup)
	if !ok {
		return nil, fmt.Errorf("%v is not a backup", obj.GetObjectKind())
	}
	return backup, nil
}

// Ensure satisfies Syncer interface.
func (s *Schedule) Ensure(obj runtime.Object) error {
	backup, err := s.checkObject(obj)
	if err != nil {
		return err
	}
	var ok bool
	name := backup.Namespace + "/" + backup.Name
	tmpBck, ok := s.reg.Load(name)
	var bck service.Runner

	// We are already running.
	if ok {
		bck = tmpBck.(service.Runner)
		// If not the same spec means options have changed, so we don't longer need this Backup.
		if !bck.SameSpec(backup) {
			s.logger.Infof("spec of %s changed, recreating baas worker", backup.Name)
			if err := s.Delete(name); err != nil {
				return err
			}
		} else { // We are ok, nothing changed.
			return nil
		}
	}

	// Create a Backup.
	backupCopy := backup.DeepCopy()

	err = createServiceAccountAndBinding(backupCopy, s.k8sCli, s.config)
	if err != nil {
		return err
	}

	var registerBackend = service.MergeGlobalBackendConfig(backupCopy.Spec.Backend, s.config.GlobalConfig)

	// Store how many time we've seen the same repository.
	backendString := registerBackend.String()
	var number int
	if value, ok := s.clusterWideState.repoMap.Load(backendString); ok {
		number = value.(int)
		number = number + 1
	} else {
		number = 1
	}

	// set shared states
	s.clusterWideState.repoMap.Store(backendString, number)
	s.clusterWideState.runningBackupsPerRepo.Store(backendString, 0)
	s.clusterWideState.runningPruneOnRepo.Store(backendString, 0)

	backupCopy.GlobalOverrides.RegisteredBackend = registerBackend

	bck = NewPVCBackupper(backupCopy, s.k8sCli, s.baasCLI, s.logger, s.cron, s.metrics, s.clusterWideState, s.config)

	s.reg.Store(name, bck)
	return bck.Start()
}

// Delete satisfies CRDEnsurer interface.
func (s *Schedule) Delete(name string) error {
	pkt, ok := s.reg.Load(name)
	if !ok {
		return fmt.Errorf("%v is not a backup", name)
	}

	pk := pkt.(service.Runner)
	if err := pk.Stop(); err != nil {
		return err
	}

	s.reg.Delete(name)
	return nil
}

func createServiceAccountAndBinding(backup *backupv1alpha1.Backup, k8sCli kubernetes.Interface, config config) error {

	account := newServiceAccontDefinition(backup, config)

	_, err := k8sCli.RbacV1().RoleBindings(backup.Namespace).Create(account.roleBinding)
	if err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
	}
	_, err = k8sCli.RbacV1().Roles(backup.Namespace).Create(account.role)
	if err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
	}
	_, err = k8sCli.CoreV1().ServiceAccounts(backup.Namespace).Create(account.account)

	if err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
	}

	return nil
}

// contains information that should be available to all namespaces
type clusterWideState struct {
	repoMap               *sync.Map
	runningBackupsPerRepo *sync.Map
	runningPruneOnRepo    *sync.Map
}
