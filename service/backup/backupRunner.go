package backup

import (
	"fmt"
	"sort"
	"strconv"

	backupv1alpha1 "github.com/vshn/k8up/apis/backup/v1alpha1"
	"github.com/vshn/k8up/service"
	"github.com/vshn/k8up/service/observe"
	"github.com/vshn/k8up/service/schedule"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type backupRunner struct {
	service.CommonObjects
	config   config
	backup   *backupv1alpha1.Backup
	observer *observe.Observer
}

func newBackupRunner(backup *backupv1alpha1.Backup, common service.CommonObjects, observer *observe.Observer) *backupRunner {
	return &backupRunner{
		backup:        backup,
		observer:      observer,
		CommonObjects: common,
		config:        newConfig(),
	}
}

// Start is part of the ServiceRunner interface.
func (b *backupRunner) Start() error {
	b.Logger.Infof("New backup job received %v in namespace %v", b.backup.Name, b.backup.Namespace)
	b.backup.Status.Started = true
	b.updateBackupStatus()

	volumes := b.listPVCs(b.config.annotation)
	backupCommands := b.listBackupCommands()

	backupJob := newBackupJob(volumes, b.backup.Name, b.backup, b.config)

	if len(volumes) == 0 && len(backupCommands) == 1 {
		b.Logger.Infof("No suitable PVCs or backup commands found in %v, skipping backup", b.backup.Namespace)
		return nil
	}

	if len(backupCommands) > 1 {
		backupJob.Spec.Template.Spec.Containers[0].Args = backupCommands
	}

	go b.watchState(backupJob)

	_, err := b.K8sCli.Batch().Jobs(b.backup.Namespace).Create(backupJob)
	if err != nil {
		return err
	}

	return nil
}

// Stop is part of the ServiceRunner interface, it's needed for permanent
// services like the scheduler.
func (b *backupRunner) Stop() error { return nil }

// SameSpec checks if the CRD instance changed. This is is only viable for
// permanent services like the scheduler, that may change.
func (b *backupRunner) SameSpec(object runtime.Object) bool { return true }

func (b *backupRunner) watchState(backupJob *batchv1.Job) {
	subscription, err := b.observer.GetBroker().Subscribe(backupJob.Labels[b.config.Identifier])
	if err != nil {
		b.Logger.Errorf("Cannot watch state of backup %v", b.backup.Name)
	}

	watch := observe.WatchObjects{
		Job:     backupJob,
		JobName: observe.BackupName,
		Locker:  b.observer.GetLocker(),
		Logger:  b.Logger,
		Failedfunc: func(message observe.PodState) {
			b.backup.Status.Failed = true
			b.backup.Status.Finished = true
			b.updateBackupStatus()
		},
		Successfunc: func(message observe.PodState) {
			b.backup.Status.Finished = true
			b.updateBackupStatus()
		},
	}

	subscription.WatchLoop(watch)

	b.removeOldestBackups(b.getScheduledCRDsInNameSpace(), b.backup.Spec.KeepJobs)
}

func (b *backupRunner) listPVCs(annotation string) []corev1.Volume {
	b.Logger.Infof("Listing all PVCs with annotation %v in namespace %v", annotation, b.backup.Namespace)
	volumes := make([]corev1.Volume, 0)
	claimlist, err := b.K8sCli.Core().PersistentVolumeClaims(b.backup.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil
	}

	for _, item := range claimlist.Items {
		annotations := item.GetAnnotations()

		tmpAnnotation, ok := annotations[annotation]

		if !b.containsAccessMode(item.Spec.AccessModes, "ReadWriteMany") && !ok {
			b.Logger.Infof("PVC %v isn't RWX", item.Name)
			continue
		}

		if !ok {
			b.Logger.Infof("PVC %v doesn't have annotation, adding to list...", item.Name)
		} else if anno, _ := strconv.ParseBool(tmpAnnotation); !anno {
			b.Logger.Infof("PVC %v annotation is %v. Skipping", item.Name, tmpAnnotation)
			continue
		} else {
			b.Logger.Infof("Adding %v to list", item.Name)
		}

		tmpVol := corev1.Volume{
			Name: item.Name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: item.Name,
					ReadOnly:  true,
				},
			},
		}

		volumes = append(volumes, tmpVol)
	}

	return volumes
}

func (b *backupRunner) containsAccessMode(s []corev1.PersistentVolumeAccessMode, e string) bool {
	for _, a := range s {
		if string(a) == e {
			return true
		}
	}
	return false
}

func (b *backupRunner) listBackupCommands() []string {
	b.Logger.Infof("Listing all pods with annotation %v in namespace %v", b.config.backupCommandAnnotation, b.backup.Namespace)

	tmp := make([]string, 0)

	pods, err := b.K8sCli.Core().Pods(b.backup.Namespace).List(metav1.ListOptions{})
	if err != nil {
		b.Logger.Errorf("Error listing backup commands: %v\n", err)
		return tmp
	}

	tmp = append(tmp, "-stdin")

	sameOwner := make(map[string]bool)

	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		annotations := pod.GetAnnotations()

		if command, ok := annotations[b.config.backupCommandAnnotation]; ok {

			fileExtension := annotations[b.config.fileExtensionAnnotation]

			owner := pod.OwnerReferences
			firstOwnerID := string(owner[0].UID)

			if _, ok := sameOwner[firstOwnerID]; !ok {
				sameOwner[firstOwnerID] = true
				args := fmt.Sprintf("%v,%v,%v,%v,%v", command, pod.Name, pod.Spec.Containers[0].Name, b.backup.Namespace, fileExtension)
				tmp = append(tmp, "-arrayOpts", args)
			}

		}
	}

	return tmp
}

func (b *backupRunner) removeOldestBackups(backups *backupv1alpha1.BackupList, maxJobs int) {
	if maxJobs == 0 {
		maxJobs = b.config.GlobalKeepJobs
	}
	numToDelete := len(backups.Items) - maxJobs
	if numToDelete <= 0 {
		return
	}

	b.Logger.Infof("Cleaning up %d/%d jobs", numToDelete, len(backups.Items))

	sort.Sort(backups)
	for i := 0; i < numToDelete; i++ {
		b.Logger.Infof("Removing job %v limit reached", backups.Items[i].Name)
		b.cleanupBackup(&backups.Items[i])
	}
}

func (b *backupRunner) getScheduledCRDsInNameSpace() *backupv1alpha1.BackupList {
	opts := metav1.ListOptions{
		LabelSelector: schedule.ScheduledLabelFilter(),
	}
	backups, err := b.BaasCLI.AppuioV1alpha1().Backups(b.backup.Namespace).List(opts)
	if err != nil {
		b.Logger.Errorf("%v", err)
		return nil
	}

	return backups
}

func (b *backupRunner) cleanupBackup(backup *backupv1alpha1.Backup) error {
	b.Logger.Infof("Cleanup backup %v", backup.Name)
	option := metav1.DeletePropagationForeground
	return b.BaasCLI.AppuioV1alpha1().Backups(backup.Namespace).Delete(backup.Name, &metav1.DeleteOptions{
		PropagationPolicy: &option,
	})
}

func (b *backupRunner) updateBackupStatus() {
	// Just overwrite the resource
	result, err := b.BaasCLI.AppuioV1alpha1().Backups(b.backup.Namespace).Get(b.backup.Name, metav1.GetOptions{})
	if err != nil {
		b.Logger.Errorf("Cannot get baas object: %v", err)
	}

	result.Status = b.backup.Status
	_, updateErr := b.BaasCLI.AppuioV1alpha1().Backups(b.backup.Namespace).Update(result)
	if updateErr != nil {
		b.Logger.Errorf("Coud not update backup resource: %v", updateErr)
	}
}
