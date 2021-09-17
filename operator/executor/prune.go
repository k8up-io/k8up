package executor

import (
	"errors"
	"strconv"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	k8upv1 "github.com/k8up-io/k8up/api/v1"
	"github.com/k8up-io/k8up/operator/cfg"
	"github.com/k8up-io/k8up/operator/job"
	"github.com/k8up-io/k8up/operator/observer"
)

// PruneExecutor will execute the batch.job for Prunes.
type PruneExecutor struct {
	generic
}

// NewPruneExecutor will return a new executor for Prune jobs.
func NewPruneExecutor(config job.Config) *PruneExecutor {
	return &PruneExecutor{
		generic: generic{config},
	}
}

// GetConcurrencyLimit returns the concurrent jobs limit
func (p *PruneExecutor) GetConcurrencyLimit() int {
	return cfg.Config.GlobalConcurrentPruneJobsLimit
}

// Execute creates the actual batch.job on the k8s api.
func (p *PruneExecutor) Execute() error {
	prune, ok := p.Obj.(*k8upv1.Prune)
	if !ok {
		return errors.New("object is not a prune")
	}

	if prune.GetStatus().Started {
		return nil
	}

	jobObj, err := job.GenerateGenericJob(prune, p.Config)
	if err != nil {
		p.SetConditionFalseWithMessage(k8upv1.ConditionReady, k8upv1.ReasonCreationFailed, "could not get job template: %v", err)
		return err
	}
	jobObj.GetLabels()[job.K8upExclusive] = "true"

	p.startPrune(jobObj, prune)

	return nil
}

// Exclusive should return true for jobs that can't run while other jobs run.
func (p *PruneExecutor) Exclusive() bool {
	return true
}

func (p *PruneExecutor) startPrune(pruneJob *batchv1.Job, prune *k8upv1.Prune) {
	p.registerPruneCallback(prune)
	p.RegisterJobSucceededConditionCallback()

	pruneJob.Spec.Template.Spec.Containers[0].Env = p.setupEnvVars(prune)
	pruneJob.Spec.Template.Spec.Containers[0].Args = []string{"-prune"}

	if err := p.Client.Create(p.CTX, pruneJob); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			p.Log.Error(err, "could not create job")
			p.SetConditionFalseWithMessage(k8upv1.ConditionReady, k8upv1.ReasonCreationFailed, "could not create job: %v", err)
			return
		}
	}

	p.SetStarted("the job '%v/%v' was created", pruneJob.Namespace, pruneJob.Name)
}

func (p *PruneExecutor) registerPruneCallback(prune *k8upv1.Prune) {
	name := p.GetJobNamespacedName()
	observer.GetObserver().RegisterCallback(name.String(), func(_ observer.ObservableJob) {
		p.cleanupOldPrunes(name, prune)
	})
}

func (p *PruneExecutor) cleanupOldPrunes(name types.NamespacedName, prune *k8upv1.Prune) {
	p.cleanupOldResources(&k8upv1.PruneList{}, name, prune)
}

func (p *PruneExecutor) setupEnvVars(prune *k8upv1.Prune) []corev1.EnvVar {
	vars := NewEnvVarConverter()

	// FIXME(mw): this is ugly

	if prune.Spec.Retention.KeepLast > 0 {
		vars.SetString("KEEP_LAST", strconv.Itoa(prune.Spec.Retention.KeepLast))
	}

	if prune.Spec.Retention.KeepHourly > 0 {
		vars.SetString("KEEP_HOURLY", strconv.Itoa(prune.Spec.Retention.KeepHourly))
	}

	if prune.Spec.Retention.KeepDaily > 0 {
		vars.SetString("KEEP_DAILY", strconv.Itoa(prune.Spec.Retention.KeepDaily))
	} else {
		vars.SetString("KEEP_DAILY", "14")
	}

	if prune.Spec.Retention.KeepWeekly > 0 {
		vars.SetString("KEEP_WEEKLY", strconv.Itoa(prune.Spec.Retention.KeepWeekly))
	}

	if prune.Spec.Retention.KeepMonthly > 0 {
		vars.SetString("KEEP_MONTHLY", strconv.Itoa(prune.Spec.Retention.KeepMonthly))
	}

	if prune.Spec.Retention.KeepYearly > 0 {
		vars.SetString("KEEP_YEARLY", strconv.Itoa(prune.Spec.Retention.KeepYearly))
	}

	if len(prune.Spec.Retention.KeepTags) > 0 {
		vars.SetString("KEEP_TAGS", strings.Join(prune.Spec.Retention.KeepTags, ","))
	}

	if prune.Spec.Backend != nil {
		for key, value := range prune.Spec.Backend.GetCredentialEnv() {
			vars.SetEnvVarSource(key, value)
		}
		vars.SetString(cfg.ResticRepositoryEnvName, prune.Spec.Backend.String())
	}

	vars.SetString("PROM_URL", cfg.Config.PromURL)

	err := vars.Merge(DefaultEnv(p.Obj.GetMetaObject().GetNamespace()))
	if err != nil {
		p.Log.Error(err, "error while merging the environment variables", "name", p.Obj.GetMetaObject().GetName(), "namespace", p.Obj.GetMetaObject().GetNamespace())
	}

	return vars.Convert()
}
