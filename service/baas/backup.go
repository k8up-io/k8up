package baas

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"

	baas8scli "git.vshn.net/vshn/baas/client/k8s/clientset/versioned"
	"git.vshn.net/vshn/baas/log"
	cron "github.com/Infowatch/cron"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var ()

// Backupper is an interface that a backup service has to
// satisfy
type Backupper interface {
	Stop() error
	SameSpec(baas *backupv1alpha1.Backup) bool
	Start() error
}

// PVCBackupper implements the Backupper interface
type PVCBackupper struct {
	backup           *backupv1alpha1.Backup
	k8sCLI           kubernetes.Interface
	baasCLI          baas8scli.Interface
	log              log.Logger
	registered       bool
	mutex            sync.Mutex
	wg               sync.WaitGroup
	cron             *cron.Cron
	cronID           cron.EntryID
	checkCronID      cron.EntryID
	config           config
	metrics          *operatorMetrics
	running          bool
	clusterWideState clusterWideState
}

type config struct {
	annotation              string
	defaultCheckSchedule    string
	podFilter               string
	backupCommandAnnotation string
}

// Stop stops the backup schedule
func (p *PVCBackupper) Stop() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.registered {
		p.log.Infof("stopped %s Backup", p.backup.Name)
	}

	p.cron.Remove(p.cronID)
	p.cron.Remove(p.checkCronID)
	p.registered = false
	return nil
}

// NewPVCBackupper returns a new PVCBackupper
func NewPVCBackupper(
	backup *backupv1alpha1.Backup,
	k8sCLI kubernetes.Interface,
	baasCLI baas8scli.Interface,
	log log.Logger,
	cron *cron.Cron,
	metrics *operatorMetrics,
	clusterWideState clusterWideState) Backupper {
	tmp := &PVCBackupper{
		backup:           backup,
		k8sCLI:           k8sCLI,
		baasCLI:          baasCLI,
		log:              log,
		mutex:            sync.Mutex{},
		cron:             cron,
		metrics:          metrics,
		clusterWideState: clusterWideState,
		wg:               sync.WaitGroup{},
	}
	tmp.initDefaults()
	conf := config{
		annotation:              viper.GetString("annotation"),
		defaultCheckSchedule:    viper.GetString("checkSchedule"),
		podFilter:               viper.GetString("podFilter"),
		backupCommandAnnotation: viper.GetString("backupCommandAnnotation"),
	}
	tmp.config = conf
	return tmp
}

func (p *PVCBackupper) initDefaults() {
	viper.SetDefault("annotation", "appuio.ch/backup")
	viper.SetDefault("checkSchedule", "0 0 * * 0")
	viper.SetDefault("podFilter", "backupPod=true")
	viper.SetDefault("backupCommandAnnotation", "appuio.ch/backupcommand")
}

// SameSpec checks if the Backup Spec was changed
func (p *PVCBackupper) SameSpec(baas *backupv1alpha1.Backup) bool {
	return reflect.DeepEqual(p.backup.Spec, baas.Spec)
}

// Start registeres the schedule for an instance of this resource
func (p *PVCBackupper) Start() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	var err error

	if p.registered {
		return fmt.Errorf("already registered")
	}

	p.registered = true

	p.log.Infof("Created %s backup schedule in namespace %s", p.backup.Name, p.backup.Namespace)
	// TODO: this library doesn't track if a job is already running
	p.cronID, err = p.cron.AddFunc(p.backup.Spec.Schedule,
		func() {
			if err := p.run(false); err != nil {
				p.log.Errorf("error backup schedule %v in %v: %s", p.backup.Name, p.backup.Namespace, err)
			}
		},
	)

	// Create check schedule
	var checkSchedule string
	if p.backup.Spec.CheckSchedule == "" {
		checkSchedule = p.config.defaultCheckSchedule
	} else {
		checkSchedule = p.backup.Spec.CheckSchedule
	}
	p.checkCronID, err = p.cron.AddFunc(checkSchedule,
		func() {
			if err := p.run(true); err != nil {
				p.log.Errorf("error check: %v", err)
			}
		})

	if err != nil {
		p.log.Errorf("Error creating schedule: ", err)
		p.Stop()
	}

	return nil
}

func (p *PVCBackupper) run(check bool) error {
	volumes := p.listPVCs(p.config.annotation)
	backupCommands := p.listBackupCommands()
	return p.runJob(volumes, backupCommands, check)
}

func (p *PVCBackupper) sharedRepo() bool {
	value, _ := p.clusterWideState.repoMap.Load(p.backup.Spec.Backend.String())
	number := value.(int)
	if number > 1 {
		return true
	}
	return false
}

func (p *PVCBackupper) cleanupJob(job *batchv1.Job) error {
	p.log.Infof("Cleanup job %v", job.Name)
	option := metav1.DeletePropagationForeground
	return p.k8sCLI.Batch().Jobs(p.backup.Namespace).Delete(job.Name, &metav1.DeleteOptions{
		PropagationPolicy: &option,
	})
}

func (p *PVCBackupper) fetchJobObject(job *batchv1.Job) (*batchv1.Job, error) {
	updatedJob, err := p.k8sCLI.Batch().Jobs(p.backup.Namespace).Get(job.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return updatedJob, nil
}

func (p *PVCBackupper) listPVCs(annotation string) []apiv1.Volume {
	p.log.Infof("Listing all PVCs with annotation %v in namespace %v", annotation, p.backup.Namespace)
	volumes := make([]apiv1.Volume, 0)
	claimlist, err := p.k8sCLI.Core().PersistentVolumeClaims(p.backup.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil
	}

	for _, item := range claimlist.Items {
		annotations := item.GetAnnotations()

		if !p.containsAccessMode(item.Spec.AccessModes, "ReadWriteMany") {
			p.log.Infof("PVC %v isn't RWX", item.Name)
			continue
		}

		if tmpAnnotation, ok := annotations[annotation]; !ok {
			p.log.Infof("PVC %v doesn't have annotation, adding to list...", item.Name)
		} else if anno, _ := strconv.ParseBool(tmpAnnotation); !anno {
			p.log.Infof("PVC %v annotation is %v. Skipping", item.Name, tmpAnnotation)
			continue
		} else {
			p.log.Infof("Adding %v to list", item.Name)
		}

		tmpVol := apiv1.Volume{
			Name: item.Name,
		}

		tmpVol.PersistentVolumeClaim = &apiv1.PersistentVolumeClaimVolumeSource{
			ClaimName: item.Name,
			ReadOnly:  true,
		}
		volumes = append(volumes, tmpVol)
	}

	return volumes
}

func (p *PVCBackupper) runJob(volumes []apiv1.Volume, backupCommands []string, check bool) error {
	backupJob := newJobDefinition(volumes, p.backup.Name, p.backup)

	if len(volumes) == 0 && len(backupCommands) == 1 {
		p.log.Infof("No suitable PVCs or backup commands found in %v, skipping backup", p.backup.Namespace)
		return nil
	}

	if check {
		backupJob.Spec.Template.Spec.Containers[0].Args = []string{"-check"}
	} else if len(backupCommands) > 1 {
		backupJob.Spec.Template.Spec.Containers[0].Args = backupCommands
	}

	// Check if this specific backup is already running. This doesn't have
	// anything todo with the prune locking, which is clusterwide.
	if p.running {
		p.log.Infof("Backup still running in %v, skipping...", p.backup.Namespace)
		return nil
	}

	// Be sure no prune is running before starting the job
	p.waitForPrune()
	p.incrementRunningBackup()
	p.log.Infof("Creating Job %v in namespace %v", backupJob.Name, p.backup.Namespace)
	_, err := p.k8sCLI.Batch().Jobs(p.backup.Namespace).Create(backupJob)
	if err != nil {
		return err
	}
	p.running = true
	defer func() {
		p.running = false
	}()

	podFilter := p.config.podFilter

	// Need to query the job again so it gets updated
	job, err := p.fetchJobObject(backupJob)

	defer func() {
		// This has to be put into an anonymous func because it would evaluate the
		// getJobsInNameSpace too early.
		p.removeOldestJobs(p.getJobsInNameSpace(p.backup.Namespace, podFilter), p.backup.Spec.KeepJobs)
	}()

	// wrap it to a k8s native object
	startTime := metav1.Time{
		Time: time.Now(),
	}
	p.backup.Status.LastBackupStart = startTime.Format("2006-01-02 15:04:05")

	p.metrics.RunningBackups.Inc()
	defer p.metrics.RunningBackups.Dec()

	job, err = p.waitForJobTofinish(job)
	p.decrementRunningBackup()

	if !p.pruneRunning() && !check {
		p.incrementRunningPrune()

		// Wait for all backups to finish
		for p.getRunningBackups() != 0 {
			time.Sleep(time.Second * 10)
		}

		backupJob.Name = fmt.Sprintf("%s-%d", "prunejob", time.Now().Unix())
		backupJob.Spec.Template.Spec.Containers[0].Args = []string{"-prune"}

		p.log.Infof("Creating Job %v in namespace %v", backupJob.Name, p.backup.Namespace)
		_, err := p.k8sCLI.Batch().Jobs(p.backup.Namespace).Create(backupJob)
		if err != nil {
			p.log.Errorf("%v", err)
			return err
		}

		job, err := p.fetchJobObject(backupJob)
		if err != nil {
			p.log.Errorf("%v", err)
		}

		job, err = p.waitForJobTofinish(job)
		if err != nil {
			p.log.Errorf("%v", err)
		}

		p.decrementRunningPrune()
	}

	var status string
	var returnVal error

	if job == nil {
		status = "The job was deleted before completion."
		returnVal = fmt.Errorf("the job was deleted before completion")
	} else if job.Status.Succeeded == 0 {
		status = "The backup failed."
		returnVal = fmt.Errorf("The backup failed")
	} else if err != nil {
		status = "The backup failed. With error " + err.Error()
		returnVal = err
	} else {
		status = fmt.Sprintf("The backup %v in namespace %v was successful.", p.backup.Name, p.backup.Namespace)
		returnVal = nil
	}

	p.log.Infof("%v Cleaning up", status)

	p.backup.Status.LastBackupEnd = time.Now().Format("2006-01-02 15:04:05")

	defer p.updateMetrics()

	return returnVal
}

func (p *PVCBackupper) incrementRunningBackup() {
	backendString := p.backup.Spec.Backend.String()
	value, _ := p.clusterWideState.runningBackupsPerRepo.Load(backendString)
	number := value.(int)
	number = number + 1
	p.clusterWideState.runningBackupsPerRepo.Store(backendString, number)
}

func (p *PVCBackupper) decrementRunningBackup() {
	backendString := p.backup.Spec.Backend.String()
	value, _ := p.clusterWideState.runningBackupsPerRepo.Load(backendString)
	number := value.(int)
	number = number - 1
	p.clusterWideState.runningBackupsPerRepo.Store(backendString, number)
}

func (p *PVCBackupper) incrementRunningPrune() {
	p.wg.Add(1)
	backendString := p.backup.Spec.Backend.String()
	value, _ := p.clusterWideState.runningPruneOnRepo.Load(backendString)
	number := value.(int)
	number = number + 1
	p.clusterWideState.runningPruneOnRepo.Store(backendString, number)
}

func (p *PVCBackupper) decrementRunningPrune() {
	p.wg.Done()
	backendString := p.backup.Spec.Backend.String()
	value, _ := p.clusterWideState.runningPruneOnRepo.Load(backendString)
	number := value.(int)
	number = number - 1
	p.clusterWideState.runningPruneOnRepo.Store(backendString, number)
}

func (p *PVCBackupper) getRunningBackups() int {
	backendString := p.backup.Spec.Backend.String()
	value, _ := p.clusterWideState.runningBackupsPerRepo.Load(backendString)
	number := value.(int)
	return number
}

func (p *PVCBackupper) waitForJobTofinish(job *batchv1.Job) (*batchv1.Job, error) {
	var err error
	count := 0

	for job.Status.Active > 0 || job.Status.StartTime == nil {
		//TODO: use select case and channels for more responsivenes
		//or maybe a controller that observes that job?
		count++
		if !p.registered {
			return nil, nil
		}

		// Reduce verbosity a bit
		if count%100 == 0 {
			p.log.Infof("Wating for job %v to finish, running status: %v", p.backup.Name, job.Status.Active)
		}
		time.Sleep(10 * time.Second)
		job, err = p.fetchJobObject(job)

		if job == nil {
			return nil, fmt.Errorf("Job got removed while running, please check if the backup resource was removed")
		}

		jobFilter := "job-name=" + job.Name

		// Check if the container had any restartarts or errors
		if p.podErrors(jobFilter) {
			err = fmt.Errorf("Container in job %v has errors or restarted, assuming failure", p.backup.Name)
		}

		if job.Status.Failed > 0 {
			err = fmt.Errorf("Container failed")
		}
		if err != nil {
			return job, err
		}
	}
	return job, nil
}

func (p *PVCBackupper) pruneRunning() bool {
	backendString := p.backup.Spec.Backend.String()
	value, _ := p.clusterWideState.runningPruneOnRepo.Load(backendString)
	number := value.(int)
	if number > 0 {
		return true
	}
	return false
}

func (p *PVCBackupper) waitForPrune() {
	p.mutex.Lock()
	if p.pruneRunning() && p.sharedRepo() {
		p.log.Infof("Waiting for prune to finish in shared repository")
		p.wg.Wait()
	}
	p.mutex.Unlock()
}

func (p *PVCBackupper) podErrors(filter string) bool {
	opts := metav1.ListOptions{
		LabelSelector: filter,
	}
	pod, err := p.k8sCLI.Core().Pods(p.backup.Namespace).List(opts)
	if err != nil {
		return true
	}
	if len(pod.Items) == 0 {
		return true
	}

	for _, podItem := range pod.Items {
		for _, status := range podItem.Status.ContainerStatuses {
			if status.RestartCount > 0 {
				return true
			}
			if (status.State.Waiting != nil && strings.Contains(status.State.Waiting.Message, "error")) ||
				(status.State.Terminated != nil && strings.Contains(status.State.Terminated.Message, "error")) {
				return true
			}
		}
	}

	return false
}

func (p *PVCBackupper) updateMetrics() {
	p.updateCRD()
	p.updatePrometheus()
}

func (p *PVCBackupper) updateCRD() {
	// Just overwrite the resource
	result, err := p.baasCLI.AppuioV1alpha1().Backups(p.backup.Namespace).Get(p.backup.Name, metav1.GetOptions{})
	if err != nil {
		p.log.Errorf("Cannot get baas object: %v", err)
	}

	result.Status = p.backup.Status
	_, updateErr := p.baasCLI.AppuioV1alpha1().Backups(p.backup.Namespace).Update(result)
	if updateErr != nil {
		p.log.Errorf("Coud not update backup resource: %v", updateErr)
	}
}

func (p *PVCBackupper) removeOldestJobs(jobs []batchv1.Job, maxJobs int32) {
	numToDelete := len(jobs) - int(maxJobs)
	if numToDelete <= 0 {
		return
	}

	p.log.Infof("Cleaning up %d/%d jobs", numToDelete, len(jobs))

	sort.Sort(byJobStartTime(jobs))
	for i := 0; i < numToDelete; i++ {
		p.log.Infof("Removing job %v limit reached", jobs[i].Name)
		p.cleanupJob(&jobs[i])
	}
}

func (p *PVCBackupper) getJobsInNameSpace(namespace, filter string) []batchv1.Job {
	opts := metav1.ListOptions{
		LabelSelector: filter,
	}
	jobs, err := p.k8sCLI.Batch().Jobs(p.backup.Namespace).List(opts)
	if err != nil {
		p.log.Errorf("%v", err)
		return nil
	}

	return jobs.Items
}

func (p *PVCBackupper) containsAccessMode(s []apiv1.PersistentVolumeAccessMode, e string) bool {
	for _, a := range s {
		if string(a) == e {
			return true
		}
	}
	return false
}

func (p *PVCBackupper) updatePrometheus() {
	// TODO: TBD
}

func (p *PVCBackupper) listBackupCommands() []string {
	p.log.Infof("Listing all pods with annotation %v in namespace %v", p.config.backupCommandAnnotation, p.backup.Namespace)

	tmp := make([]string, 0)

	pods, err := p.k8sCLI.Core().Pods(p.backup.Namespace).List(metav1.ListOptions{})
	if err != nil {
		p.log.Errorf("Error listing backup commands: %v\n", err)
		return tmp
	}

	tmp = append(tmp, "-stdin")

	sameOwner := make(map[string]bool)

	for _, pod := range pods.Items {
		annotations := pod.GetAnnotations()

		if command, ok := annotations[p.config.backupCommandAnnotation]; ok {

			owner := pod.OwnerReferences
			firstOwnerID := string(owner[0].UID)

			if _, ok := sameOwner[firstOwnerID]; !ok {
				sameOwner[firstOwnerID] = true
				args := fmt.Sprintf("\"%v,%v,%v,%v\"", command, pod.Name, pod.Spec.Containers[0].Name, p.backup.Namespace)
				tmp = append(tmp, "-arrayOpts", args)
			}

		}
	}

	return tmp
}
