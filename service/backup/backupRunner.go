package backup

import (
	"fmt"
	"sort"
	"strconv"

	backupv1alpha1 "github.com/vshn/k8up/apis/backup/v1alpha1"
	"github.com/vshn/k8up/service"
	"github.com/vshn/k8up/service/observe"
	"github.com/vshn/k8up/service/schedule"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

type backupRunner struct {
	service.CommonObjects
	config             config
	backup             *backupv1alpha1.Backup
	observer           *observe.Observer
	runningDeployments []appsv1.Deployment
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

	b.startPodTemplates()

	backupJob := newBackupJob(volumes, b.backup.Name, b.backup, b.config)

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
			b.stopPodTemplates()
		},
		Successfunc: func(message observe.PodState) {
			b.backup.Status.Finished = true
			b.updateBackupStatus()
			b.stopPodTemplates()
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
			b.Logger.Infof("PVC %v/%v isn't RWX", item.GetNamespace(), item.GetName())
			continue
		}

		if !ok {
			b.Logger.Infof("PVC %v/%v doesn't have annotation, adding to list...", item.GetNamespace(), item.GetName())
		} else if anno, _ := strconv.ParseBool(tmpAnnotation); !anno {
			b.Logger.Infof("PVC %v/%v annotation is %v. Skipping", item.GetNamespace(), item.GetName(), tmpAnnotation)
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

// startPodTemplates will start and wait all pods defined in the template and
// wait for at least one replication to be available for each pod.
func (b *backupRunner) startPodTemplates() {
	b.runningDeployments = b.getDeployments()
	for _, deployment := range b.runningDeployments {

		name := fmt.Sprintf("%v/%v", deployment.GetNamespace(), deployment.GetName())

		b.Logger.Infof("Creating command pod %v\n", name)
		runningDeployment, err := b.K8sCli.Apps().Deployments(b.backup.GetNamespace()).Create(&deployment)
		if err != nil {
			b.Logger.Errorf("error creating command pod %v: %v\n", name, err)
			continue
		}

		watcher, err := b.K8sCli.Apps().Deployments(b.backup.GetNamespace()).Watch(
			metav1.SingleObject(runningDeployment.ObjectMeta),
		)
		if err != nil {
			b.Logger.Errorf("cannot watch deployment: %v", err)
			continue
		}

		for event := range watcher.ResultChan() {
			runningDeployment = event.Object.(*appsv1.Deployment)

			switch event.Type {
			case watch.Modified:

				last := b.getLastDeploymentCondition(runningDeployment)

				if last != nil {
					// if the deadline can't be respected https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#progress-deadline-seconds
					if last.Type == "Progressing" && last.Status == "False" && last.Reason == "ProgressDeadlineExceeded" {
						b.Logger.Errorf("error starting pre backup pod %v: %v", name, last.Message)
						watcher.Stop()
					}
				}

				// Wait until at least one replica is available and continue
				if runningDeployment.Status.AvailableReplicas > 0 {
					watcher.Stop()
				}

				b.Logger.Infof("waiting for command pod %v to get ready", name)

			case watch.Error:

				last := b.getLastDeploymentCondition(runningDeployment)

				if last != nil {
					b.Logger.Errorf("there was an error while starting pre backup pod %v: %v", name, last.Message)

				} else {
					b.Logger.Errorf("there was an unknown error while starting pre backup pod %v", name)
				}

				watcher.Stop()

			default:
				b.Logger.Errorf("unexpected event during %v watching: %v ", name, event.Type)
				watcher.Stop()
			}
		}

	}
}

func (b *backupRunner) getLastDeploymentCondition(deployment *appsv1.Deployment) *appsv1.DeploymentCondition {
	conditions := deployment.Status.Conditions

	if len(conditions) > 0 {
		return &conditions[len(conditions)-1]
	}
	return nil
}

func (b *backupRunner) stopPodTemplates() {
	for _, set := range b.runningDeployments {
		b.Logger.Infof("removing command pod %v/%v", set.GetNamespace(), set.GetName())
		option := metav1.DeletePropagationForeground
		err := b.K8sCli.Apps().Deployments(b.backup.GetNamespace()).Delete(set.GetName(), &metav1.DeleteOptions{
			PropagationPolicy: &option,
		})
		if err != nil {
			if errors.IsNotFound(err) {
				b.Logger.Infof("deployment already removed: ", err)
			} else {
				b.Logger.Errorf("could not remove deployment: %v", err)
			}
		}
	}
}
