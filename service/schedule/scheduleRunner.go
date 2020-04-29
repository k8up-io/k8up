package schedule

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/Infowatch/cron"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	backupv1alpha1 "github.com/vshn/k8up/apis/backup/v1alpha1"
	"github.com/vshn/k8up/config"
	"github.com/vshn/k8up/service"
	"github.com/vshn/k8up/service/observe"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	labels = map[string]string{
		"scheduled": "true",
	}

	promLabels = []string{
		"namespace",
		"jobType",
	}

	scheduleCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "k8up_schedules_gauge",
		Help: "How many schedules this k8up manages",
	}, []string{
		"namespace",
	})

	queueCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "k8up_jobs_queued_gauge",
		Help: "How many jobs are currently queued up",
	}, promLabels)
)

type scheduleRunner struct {
	*service.CommonObjects
	cron     *cron.Cron
	schedule *backupv1alpha1.Schedule
	observer *observe.Observer
}

func newScheduleRunner(schedule *backupv1alpha1.Schedule, common *service.CommonObjects, observer *observe.Observer) *scheduleRunner {
	return &scheduleRunner{
		CommonObjects: common,
		cron:          cron.New(),
		schedule:      schedule,
		observer:      observer,
	}
}

// TODO: cleanup of older jobs
// Stop stops the currently running schedule. It implements the ServiceRunner interface.
func (s *scheduleRunner) Stop() error {
	s.cron.Stop()
	s.decrRegisteredSchedules(s.schedule.GetNamespace())
	return nil
}

// SameSepc checks if something changed in the Spec of the schedule. It's part of the ServiceRunner interface.
func (s *scheduleRunner) SameSpec(object runtime.Object) bool {
	schedule, ok := object.(*backupv1alpha1.Schedule)
	if !ok {
		return false
	}
	return reflect.DeepEqual(s.schedule.Spec, schedule.Spec)
}

// Start checks what various schedules are defined and create the accordingly.
func (s *scheduleRunner) Start() error {

	scheduleCopy := s.schedule.DeepCopy()

	ownerReference := []metav1.OwnerReference{
		service.NewOwnerReference(scheduleCopy, backupv1alpha1.ScheduleKind),
	}

	locker := s.observer.GetLocker()

	if scheduleCopy.Spec.Restore != nil {
		if scheduleCopy.Spec.Restore.Backend == nil {
			scheduleCopy.Spec.Restore.Backend = &backupv1alpha1.Backend{}
		}
		scheduleCopy.Spec.Restore.Backend.Merge(config.New(), scheduleCopy.Spec.Backend)

		s.Logger.Infof("Registering restore schedule %v in namespace %v", scheduleCopy.Name, scheduleCopy.Namespace)

		s.cron.AddFunc(scheduleCopy.Spec.Restore.Schedule, func() {

			newRestore := backupv1alpha1.Restore{
				ObjectMeta: metav1.ObjectMeta{
					Name:            fmt.Sprintf("scheduled-restore-%v-%v", scheduleCopy.Name, time.Now().Unix()),
					OwnerReferences: ownerReference,
					Labels:          labels,
				},
				Spec: scheduleCopy.Spec.Restore.RestoreSpec.DeepCopy(),
			}

			if newRestore.Spec.KeepJobs == 0 {
				newRestore.Spec.KeepJobs = scheduleCopy.Spec.KeepJobs
			}
			s.waitForLock(newRestore.GetName(), service.GetRepository(newRestore.Spec.Backend), []observe.JobName{observe.PruneName, observe.CheckName}, "restore")

			_, err := s.BaasCLI.AppuioV1alpha1().Restores(scheduleCopy.Namespace).Create(&newRestore)
			if err != nil {
				s.Logger.Errorf("Error creating the restore schedule %v in namespace %v: %v", scheduleCopy.Name, scheduleCopy.Namespace, err)
			}
		})
	}

	if scheduleCopy.Spec.Prune != nil {
		if scheduleCopy.Spec.Prune.Backend == nil {
			scheduleCopy.Spec.Prune.Backend = &backupv1alpha1.Backend{}
		}
		scheduleCopy.Spec.Prune.Backend.Merge(config.New(), scheduleCopy.Spec.Backend)

		s.Logger.Infof("Registering prune schedule %v in namespace %v", scheduleCopy.Name, scheduleCopy.Namespace)
		s.cron.AddFunc(scheduleCopy.Spec.Prune.Schedule, func() {

			repoString := service.GetRepository(scheduleCopy.Spec.Prune.Backend)

			if locker.IsLocked(repoString, observe.PruneName) {
				s.Logger.Infof("Prunejob on repo %v still running, skipping", repoString)
				return
			}

			// Increase the prune semaphore beforehand so no new jobs will start
			// while we're waiting here.
			newLock := locker.Increment(repoString, observe.PruneName)

			// Decrement the semaphore again after the job was created.
			defer locker.Decrement(newLock)

			newPrune := backupv1alpha1.Prune{
				ObjectMeta: metav1.ObjectMeta{
					Name:            fmt.Sprintf("scheduled-prune-%v-%v", scheduleCopy.Name, time.Now().Unix()),
					OwnerReferences: ownerReference,
					Labels:          labels,
				},
				Spec: scheduleCopy.Spec.Prune.PruneSpec.DeepCopy(),
			}

			if newPrune.Spec.KeepJobs == 0 {
				newPrune.Spec.KeepJobs = scheduleCopy.Spec.KeepJobs
			}
			s.waitForLock(newPrune.GetName(), repoString, []observe.JobName{observe.BackupName, observe.CheckName, observe.RestoreName}, "prune")

			_, err := s.BaasCLI.AppuioV1alpha1().Prunes(scheduleCopy.Namespace).Create(&newPrune)
			if err != nil {
				s.Logger.Errorf("Error creating the prune schedule %v in namespace %v: %v", scheduleCopy.Name, scheduleCopy.Namespace, err)
			}
		})
	}

	if scheduleCopy.Spec.Check != nil {
		if scheduleCopy.Spec.Check.Backend == nil {
			scheduleCopy.Spec.Check.Backend = &backupv1alpha1.Backend{}
		}
		scheduleCopy.Spec.Check.Backend.Merge(config.New(), scheduleCopy.Spec.Backend)

		s.Logger.Infof("Registering check schedule %v in namespace %v", scheduleCopy.Name, scheduleCopy.Namespace)
		s.cron.AddFunc(scheduleCopy.Spec.Check.Schedule, func() {

			newCheck := backupv1alpha1.Check{
				ObjectMeta: metav1.ObjectMeta{
					Name:            fmt.Sprintf("scheduled-check-%v-%v", scheduleCopy.Name, time.Now().Unix()),
					OwnerReferences: ownerReference,
					Labels:          labels,
				},
				Spec: scheduleCopy.Spec.Check.CheckSpec.DeepCopy(),
			}

			if newCheck.Spec.KeepJobs == 0 {
				newCheck.Spec.KeepJobs = scheduleCopy.Spec.KeepJobs
			}
			if locker.IsLocked(service.GetRepository(newCheck.Spec.Backend), observe.CheckName) {
				s.Logger.Infof("Checkjob on repo %v still running, skipping", service.GetRepository(newCheck.Spec.Backend))
				return
			}

			s.waitForLock(newCheck.GetName(), service.GetRepository(newCheck.Spec.Backend), []observe.JobName{observe.PruneName, observe.BackupName, observe.RestoreName}, "check")

			_, err := s.BaasCLI.AppuioV1alpha1().Checks(scheduleCopy.Namespace).Create(&newCheck)
			if err != nil {
				s.Logger.Errorf("Error creating the check schedule %v in namespace %v: %v", scheduleCopy.Name, scheduleCopy.Namespace, err)
			}
		})
	}

	if scheduleCopy.Spec.Backup != nil {
		if scheduleCopy.Spec.Backup.Backend == nil {
			scheduleCopy.Spec.Backup.Backend = &backupv1alpha1.Backend{}
		}
		scheduleCopy.Spec.Backup.Backend.Merge(config.New(), scheduleCopy.Spec.Backend)

		s.Logger.Infof("Registering backup schedule %v in namespace %v", scheduleCopy.Name, scheduleCopy.Namespace)
		s.cron.AddFunc(scheduleCopy.Spec.Backup.Schedule, func() {
			newBackup := backupv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:            fmt.Sprintf("scheduled-backup-%v-%v", scheduleCopy.Name, time.Now().Unix()),
					OwnerReferences: ownerReference,
					Labels:          labels,
				},
				Spec: scheduleCopy.Spec.Backup.BackupSpec.DeepCopy(),
			}

			if newBackup.Spec.KeepJobs == 0 {
				newBackup.Spec.KeepJobs = scheduleCopy.Spec.KeepJobs
			}
			s.waitForLock(newBackup.GetName(), service.GetRepository(newBackup.Spec.Backend), []observe.JobName{observe.PruneName, observe.CheckName}, "backup")

			_, err := s.BaasCLI.AppuioV1alpha1().Backups(scheduleCopy.Namespace).Create(&newBackup)
			if err != nil {
				s.Logger.Errorf("Error creating the backup schedule %v in namespace %v: %v", scheduleCopy.Name, scheduleCopy.Namespace, err)
			}
		})
	}

	if scheduleCopy.Spec.Archive != nil {
		if scheduleCopy.Spec.Archive.Backend == nil {
			scheduleCopy.Spec.Archive.Backend = &backupv1alpha1.Backend{}
		}
		scheduleCopy.Spec.Archive.Backend.Merge(config.New(), scheduleCopy.Spec.Backend)

		s.Logger.Infof("Registering archive schedule %v in namespace %v", scheduleCopy.Name, scheduleCopy.Namespace)
		s.cron.AddFunc(scheduleCopy.Spec.Archive.Schedule, func() {
			newArchive := backupv1alpha1.Archive{
				ObjectMeta: metav1.ObjectMeta{
					Name:            fmt.Sprintf("scheduled-archive-%v-%v", scheduleCopy.Name, time.Now().Unix()),
					OwnerReferences: ownerReference,
					Labels:          labels,
				},
				Spec: scheduleCopy.Spec.Archive.ArchiveSpec.DeepCopy(),
			}

			if newArchive.Spec.KeepJobs == 0 {
				newArchive.Spec.KeepJobs = scheduleCopy.Spec.KeepJobs
			}
			s.waitForLock(newArchive.GetName(), service.GetRepository(newArchive.Spec.Backend), []observe.JobName{observe.PruneName, observe.CheckName}, "archive")

			_, err := s.BaasCLI.AppuioV1alpha1().Archives(scheduleCopy.Namespace).Create(&newArchive)
			if err != nil {
				s.Logger.Errorf("Error creating the archive schedule %v in namespace %v: %v", scheduleCopy.Name, scheduleCopy.Namespace, err)
			}
		})
	}

	s.cron.Start()

	s.incrRegisteredSchedules(s.schedule.GetNamespace())

	return nil
}

// ScheduledLabelFilter returns a string which filters for scheduled pods. This
// is mainly needed for the cleanup.
func ScheduledLabelFilter() string {
	filter := make([]string, 0)
	for k, v := range labels {
		filter = append(filter, k+"="+v)
	}
	return strings.Join(filter, ", ")
}

func (s *scheduleRunner) waitForLock(name string, backend string, jobs []observe.JobName, jobType string) {
	queue := queueCount.WithLabelValues(s.schedule.GetNamespace(), jobType)

	s.Logger.Infof("%v for repo %v is queued waiting for jobs %v to finish", name, backend, jobs)
	queue.Inc()
	defer queue.Dec()
	s.observer.GetLocker().WaitForRun(backend, jobs)
	s.Logger.Infof("All blocking jobs on %v for %v are now finished", backend, name)
}

func (s *scheduleRunner) incrRegisteredSchedules(namespace string) {
	scheduleCount.WithLabelValues(namespace).Inc()
}

func (s *scheduleRunner) decrRegisteredSchedules(namespace string) {
	scheduleCount.WithLabelValues(namespace).Dec()
}
