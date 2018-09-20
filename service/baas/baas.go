package baas

import (
	"strings"
	"sync"

	"git.vshn.net/vshn/baas/monitoring"

	"git.vshn.net/vshn/baas/log"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	baas8scli "git.vshn.net/vshn/baas/client/k8s/clientset/versioned"
	cron "github.com/Infowatch/cron"
	"k8s.io/client-go/kubernetes"
)

// Syncer is the interface that each Baas implementation has to satisfy.
type Syncer interface {
	// EnsureBackup will ensure that the backup schedule is correctly registered
	EnsureBackup(pt *backupv1alpha1.Backup) error
	// DeleteBackup will stop and delete the schedule. Kubernetes will handle
	// the deletion of all child items.
	DeleteBackup(name string) error
}

// Baas will ensure that the backups are running accordingly.
type Baas struct {
	k8sCli           kubernetes.Interface
	baasCLI          baas8scli.Interface
	reg              sync.Map
	logger           log.Logger
	cron             *cron.Cron
	metrics          *operatorMetrics
	clusterWideState clusterWideState
	config           config
}

// NewBaas returns a new baas.
func NewBaas(k8sCli kubernetes.Interface, baasCLI baas8scli.Interface, logger log.Logger) *Baas {
	cron := cron.New()
	cron.Start()
	metrics := newOperatorMetrics(monitoring.GetInstance())
	return &Baas{
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

// EnsureBackup satisfies Syncer interface.
func (b *Baas) EnsureBackup(backup *backupv1alpha1.Backup) error {
	var ok bool
	name := backup.Namespace + "/" + backup.Name
	tmpBck, ok := b.reg.Load(name)
	var bck Backupper

	// We are already running.
	if ok {
		bck = tmpBck.(Backupper)
		// If not the same spec means options have changed, so we don't longer need this Backup.
		if !bck.SameSpec(backup) {
			b.logger.Infof("spec of %s changed, recreating baas worker", backup.Name)
			if err := b.DeleteBackup(name); err != nil {
				return err
			}
		} else { // We are ok, nothing changed.
			return nil
		}
	}

	// Create a Backup.
	backupCopy := backup.DeepCopy()

	err := createServiceAccountAndBinding(backupCopy, b.k8sCli, b.config)
	if err != nil {
		return err
	}

	var registerBackend = new(backupv1alpha1.Backend)
	registerBackend.S3 = &backupv1alpha1.S3Spec{}
	if backupCopy.Spec.Backend.S3 != nil {
		registerBackend.S3.Bucket = backupCopy.Spec.Backend.S3.Bucket
		registerBackend.S3.Endpoint = backupCopy.Spec.Backend.S3.Endpoint
	} else {
		registerBackend.S3.Bucket = ""
		registerBackend.S3.Endpoint = ""
	}

	if registerBackend.S3.Bucket == "" {
		registerBackend.S3.Bucket = b.config.globalS3Bucket
	}
	if registerBackend.S3.Endpoint == "" {
		registerBackend.S3.Endpoint = b.config.globalS3Endpoint
	}

	// Store how many time we've seen the same repository.
	backendString := registerBackend.String()
	var number int
	if value, ok := b.clusterWideState.repoMap.Load(backendString); ok {
		number = value.(int)
		number = number + 1
	} else {
		number = 1
	}

	// set shared states
	b.clusterWideState.repoMap.Store(backendString, number)
	b.clusterWideState.runningBackupsPerRepo.Store(backendString, 0)
	b.clusterWideState.runningPruneOnRepo.Store(backendString, 0)

	backupCopy.GlobalOverrides.RegisteredBackend = registerBackend

	bck = NewPVCBackupper(backupCopy, b.k8sCli, b.baasCLI, b.logger, b.cron, b.metrics, b.clusterWideState, b.config)

	b.reg.Store(name, bck)
	return bck.Start()
}

// DeleteBackup satisfies Syncer interface.
func (b *Baas) DeleteBackup(name string) error {
	pkt, ok := b.reg.Load(name)
	if !ok {
		return nil
	}

	pk := pkt.(Backupper)
	if err := pk.Stop(); err != nil {
		return err
	}

	b.reg.Delete(name)
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
