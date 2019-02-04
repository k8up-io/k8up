package prune

import (
	"sort"
	"strconv"
	"strings"

	backupv1alpha1 "github.com/vshn/k8up/apis/backup/v1alpha1"
	"github.com/vshn/k8up/config"
	"github.com/vshn/k8up/service"
	"github.com/vshn/k8up/service/observe"
	"github.com/vshn/k8up/service/schedule"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type pruneRunner struct {
	service.CommonObjects
	config   config.Global
	observer *observe.Observer
	prune    *backupv1alpha1.Prune
}

func newPruneRunner(common service.CommonObjects, config config.Global, prune *backupv1alpha1.Prune, observer *observe.Observer) *pruneRunner {
	return &pruneRunner{
		CommonObjects: common,
		config:        config,
		observer:      observer,
		prune:         prune,
	}
}

// Stop is part of the ServiceRunner interface, it's needed for permanent
// services like the scheduler.
func (p *pruneRunner) Stop() error { return nil }

// SameSpec checks if the CRD instance changed. This is is only viable for
// permanent services like the scheduler, that may change.
func (p *pruneRunner) SameSpec(object runtime.Object) bool { return true }

// Start is part of the ServiceRunner interface.
func (p *pruneRunner) Start() error {
	p.Logger.Infof("New prune job received %v in namespace %v", p.prune.Name, p.prune.Namespace)
	p.prune.Status.Started = true
	p.updatePruneStatus()

	pruneJob := p.newPruneJob(p.prune, p.config)

	go p.watchState(pruneJob)

	_, err := p.K8sCli.Batch().Jobs(p.prune.Namespace).Create(pruneJob)
	if err != nil {
		return err
	}

	return nil
}

func (p *pruneRunner) newPruneJob(prune *backupv1alpha1.Prune, config config.Global) *batchv1.Job {
	job := service.GetBasicJob(backupv1alpha1.PruneKind, p.config, &p.prune.ObjectMeta)

	job.Spec.Template.Spec.Containers[0].Args = []string{"-prune"}

	envVar := p.setUpRetention(p.prune)

	envVar = append(envVar, service.DefaultEnvs(prune.Spec.Backend, config)...)

	job.Spec.Template.Spec.Containers[0].Env = append(envVar, job.Spec.Template.Spec.Containers[0].Env...)

	return job
}

func (p *pruneRunner) updatePruneStatus() {
	// Just overwrite the resource
	result, err := p.BaasCLI.AppuioV1alpha1().Prunes(p.prune.Namespace).Get(p.prune.Name, metav1.GetOptions{})
	if err != nil {
		p.Logger.Errorf("Cannot get baas object: %v", err)
	}

	result.Status = p.prune.Status
	_, updateErr := p.BaasCLI.AppuioV1alpha1().Prunes(p.prune.Namespace).Update(result)
	if updateErr != nil {
		p.Logger.Errorf("Coud not update prune resource: %v", updateErr)
	}
}

func (p *pruneRunner) setUpRetention(prune *backupv1alpha1.Prune) []corev1.EnvVar {
	retentionRules := []corev1.EnvVar{}

	if prune.Spec.Retention.KeepLast > 0 {
		retentionRules = append(retentionRules, corev1.EnvVar{
			Name:  string(service.KeepLast),
			Value: strconv.Itoa(prune.Spec.Retention.KeepLast),
		})
	}

	if prune.Spec.Retention.KeepHourly > 0 {
		retentionRules = append(retentionRules, corev1.EnvVar{
			Name:  string(service.KeepHourly),
			Value: strconv.Itoa(prune.Spec.Retention.KeepHourly),
		})
	}

	if prune.Spec.Retention.KeepDaily > 0 {
		retentionRules = append(retentionRules, corev1.EnvVar{
			Name:  string(service.KeepDaily),
			Value: strconv.Itoa(prune.Spec.Retention.KeepDaily),
		})
	} else {
		//Set defaults
		retentionRules = append(retentionRules, corev1.EnvVar{
			Name:  string(service.KeepDaily),
			Value: strconv.Itoa(14),
		})
	}

	if prune.Spec.Retention.KeepWeekly > 0 {
		retentionRules = append(retentionRules, corev1.EnvVar{
			Name:  string(service.KeepWeekly),
			Value: strconv.Itoa(prune.Spec.Retention.KeepWeekly),
		})
	}

	if prune.Spec.Retention.KeepMonthly > 0 {
		retentionRules = append(retentionRules, corev1.EnvVar{
			Name:  string(service.KeepMonthly),
			Value: strconv.Itoa(prune.Spec.Retention.KeepMonthly),
		})
	}

	if prune.Spec.Retention.KeepYearly > 0 {
		retentionRules = append(retentionRules, corev1.EnvVar{
			Name:  string(service.KeepYearly),
			Value: strconv.Itoa(prune.Spec.Retention.KeepYearly),
		})
	}

	if len(prune.Spec.Retention.KeepTags) > 0 {
		retentionRules = append(retentionRules, corev1.EnvVar{
			Name:  string(service.KeepTag),
			Value: strings.Join(prune.Spec.Retention.KeepTags, ","),
		})
	}

	return retentionRules
}

func (p *pruneRunner) watchState(job *batchv1.Job) {
	subscription, err := p.observer.GetBroker().Subscribe(job.Labels[p.config.Identifier])
	if err != nil {
		p.Logger.Errorf("Cannot watch state of prune %v", p.prune.Name)
	}

	watch := observe.WatchObjects{
		Job:     job,
		JobName: observe.PruneName,
		Locker:  p.observer.GetLocker(),
		Logger:  p.Logger,
		Failedfunc: func(message observe.PodState) {
			p.prune.Status.Failed = true
			p.prune.Status.Finished = true
			p.updatePruneStatus()
		},
		Successfunc: func(message observe.PodState) {
			p.prune.Status.Finished = true
			p.updatePruneStatus()
		},
	}

	subscription.WatchLoop(watch)

	p.removeOldestPrunes(p.getScheduledCRDsInNameSpace(), p.prune.Spec.KeepJobs)

}

func (p *pruneRunner) getScheduledCRDsInNameSpace() *backupv1alpha1.PruneList {
	opts := metav1.ListOptions{
		LabelSelector: schedule.ScheduledLabelFilter(),
	}
	prunes, err := p.BaasCLI.AppuioV1alpha1().Prunes(p.prune.Namespace).List(opts)
	if err != nil {
		p.Logger.Errorf("%v", err)
		return nil
	}

	return prunes
}

func (p *pruneRunner) cleanupPrune(prune *backupv1alpha1.Prune) error {
	p.Logger.Infof("Cleanup prune %v", prune.Name)
	option := metav1.DeletePropagationForeground
	return p.BaasCLI.AppuioV1alpha1().Prunes(prune.Namespace).Delete(prune.Name, &metav1.DeleteOptions{
		PropagationPolicy: &option,
	})
}

func (p *pruneRunner) removeOldestPrunes(prunes *backupv1alpha1.PruneList, maxJobs int) {
	if maxJobs == 0 {
		maxJobs = p.config.GlobalKeepJobs
	}
	numToDelete := len(prunes.Items) - maxJobs
	if numToDelete <= 0 {
		return
	}

	p.Logger.Infof("Cleaning up %d/%d jobs", numToDelete, len(prunes.Items))

	sort.Sort(prunes)
	for i := 0; i < numToDelete; i++ {
		p.Logger.Infof("Removing job %v limit reached", prunes.Items[i].Name)
		p.cleanupPrune(&prunes.Items[i])
	}
}
