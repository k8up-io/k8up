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
	backup      *backupv1alpha1.Backup
	k8sCLI      kubernetes.Interface
	baasCLI     baas8scli.Interface
	log         log.Logger
	running     bool
	mutex       sync.Mutex
	stopC       chan struct{}
	cron        *cron.Cron
	cronID      cron.EntryID
	checkCronID cron.EntryID
	config      config
	metrics     *operatorMetrics
}

type config struct {
	annotation           string
	defaultCheckSchedule string
	podFilter            string
}

// Stop stops the backup schedule
func (p *PVCBackupper) Stop() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.running {
		close(p.stopC)
		p.log.Infof("stopped %s Backup", p.backup.Name)
	}

	p.cron.Remove(p.cronID)
	p.cron.Remove(p.checkCronID)
	p.running = false
	return nil
}

// NewPVCBackupper returns a new PVCBackupper
func NewPVCBackupper(
	backup *backupv1alpha1.Backup,
	k8sCLI kubernetes.Interface,
	baasCLI baas8scli.Interface,
	log log.Logger,
	cron *cron.Cron,
	metrics *operatorMetrics) Backupper {
	tmp := &PVCBackupper{
		backup:  backup,
		k8sCLI:  k8sCLI,
		baasCLI: baasCLI,
		log:     log,
		mutex:   sync.Mutex{},
		cron:    cron,
		metrics: metrics,
	}
	tmp.initDefaults()
	conf := config{
		annotation:           viper.GetString("annotation"),
		defaultCheckSchedule: viper.GetString("checkSchedule"),
		podFilter:            viper.GetString("podFilter"),
	}
	tmp.config = conf
	return tmp
}

func (p *PVCBackupper) initDefaults() {
	viper.SetDefault("annotation", "appuio.ch/backup")
	viper.SetDefault("checkSchedule", "0 0 * * 0")
	viper.SetDefault("podFilter", "backupPod=true")
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

	if p.running {
		return fmt.Errorf("already running")
	}

	p.stopC = make(chan struct{})
	p.running = true

	p.log.Infof("started %s backup worker", p.backup.Name)
	// TODO: this library doesn't track if a job is already running
	p.cronID, err = p.cron.AddFunc(p.backup.Spec.Schedule,
		func() {
			if err := p.run(); err != nil {
				p.log.Errorf("error backup worker: %s", err)
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
			if err := p.runCheck(); err != nil {
				p.log.Errorf("error check: %v", err)
			}
		})

	if err != nil {
		p.log.Errorf("Error creating schedule: ", err)
		p.Stop()
	}

	return nil
}

func (p *PVCBackupper) run() error {
	volumes := p.listPVCs(p.config.annotation)
	return p.runJob(volumes, false)
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

func (p *PVCBackupper) runJob(volumes []apiv1.Volume, check bool) error {
	backupJob := newJobDefinition(volumes, p.backup.Name, p.backup)

	if check {
		backupJob.Spec.Template.Spec.Containers[0].Args = []string{"-check"}
	}

	p.log.Infof("Creating Job %v in namespace %v", backupJob.Name, p.backup.Namespace)
	_, err := p.k8sCLI.Batch().Jobs(p.backup.Namespace).Create(backupJob)
	if err != nil {
		return err
	}

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
	p.backup.Status.LastBackupDate = &startTime

	count := 0

	p.metrics.RunningBackups.Inc()
	defer p.metrics.RunningBackups.Dec()
	for job.Status.Active > 0 || job.Status.StartTime == nil {
		//TODO: use select case and channels for more responsivenes
		//or maybe a controller that observes that job?
		count++
		if !p.running {
			return nil
		}

		// Reduce verbosity a bit
		if count%100 == 0 {
			p.log.Infof("Wating for job %v to finish, running status: %v", p.backup.Name, job.Status.Active)
		}
		time.Sleep(10 * time.Second)
		job, err = p.fetchJobObject(job)

		jobFilter := "job-name=" + job.Name

		// Check if the container had any restartarts or errors
		if p.podErrors(jobFilter) {
			err = fmt.Errorf("Container in job %v has errors or restarted, assuming failure", p.backup.Name)
		}

		if job.Status.Failed > 0 {
			err = fmt.Errorf("Container failed")
		}
		if err != nil {
			return err
		}
	}

	var status string
	var returnVal error

	if job.Status.Succeeded == 0 {
		status = "The backup failed."
		returnVal = fmt.Errorf("The backup failed")
	} else {
		status = "The backup was successful."
		returnVal = nil
	}

	p.log.Infof("%v Cleaning up", status)

	p.backup.Status.LastBackupDuration = time.Since(startTime.Time).Seconds()

	defer p.updateMetrics()

	return returnVal
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

func (p *PVCBackupper) runCheck() error {
	volumes := p.listPVCs(p.config.annotation)
	return p.runJob(volumes, true)
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
