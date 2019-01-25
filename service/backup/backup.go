package backup

import (
	"fmt"
	"strings"

	backupv1alpha1 "github.com/vshn/k8up/apis/backup/v1alpha1"
	"github.com/vshn/k8up/service"
	"github.com/vshn/k8up/service/observe"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

// Backup holds the state of the backup handler. It implements the ServiceHandler interface.
type Backup struct {
	service.CommonObjects
	config   config
	observer *observe.Observer
}

// NewBackup returns a new backup handler
func NewBackup(common service.CommonObjects, observer *observe.Observer) *Backup {
	return &Backup{
		CommonObjects: common,
		observer:      observer,
		config:        newConfig(),
	}
}

func (b *Backup) checkObject(obj runtime.Object) (*backupv1alpha1.Backup, error) {
	backup, ok := obj.(*backupv1alpha1.Backup)
	if !ok {
		return nil, fmt.Errorf("%v is not a backup", obj.GetObjectKind())
	}
	return backup, nil
}

// Ensure is part of the ServiceHandler interface
func (b *Backup) Ensure(obj runtime.Object) error {
	backup, err := b.checkObject(obj)
	if err != nil {
		return err
	}

	if backup.Status.Started {
		return nil
	}

	backupCopy := backup.DeepCopy()

	err = createServiceAccountAndBinding(backupCopy, b.K8sCli, b.config)
	if err != nil {
		return err
	}

	newBackup := newBackupRunner(backupCopy, b.CommonObjects, b.observer)
	return newBackup.Start()

}

// Delete is part of the ServiceHandler interface. It's needed for permanent
// services, like the scheduler.
func (b *Backup) Delete(name string) error {
	return nil
}

func createServiceAccountAndBinding(backup *backupv1alpha1.Backup, k8sCli kubernetes.Interface, config config) error {

	account := newServiceAccountDefinition(backup, config)

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
