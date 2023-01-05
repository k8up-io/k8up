package prunecontroller

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/k8up-io/k8up/v2/operator/executor"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
)

// PruneExecutor will execute the batch.job for Prunes.
type PruneExecutor struct {
	executor.Generic
}

// NewPruneExecutor will return a new executor for Prune jobs.
func NewPruneExecutor(config job.Config) *PruneExecutor {
	return &PruneExecutor{
		Generic: executor.Generic{Config: config},
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

	batchJob := &batchv1.Job{}
	batchJob.Name = prune.GetJobName()
	batchJob.Namespace = prune.Namespace

	_, err := controllerutil.CreateOrUpdate(p.CTX, p.Client, batchJob, func() error {
		mutateErr := job.MutateBatchJob(batchJob, prune, p.Config)
		if mutateErr != nil {
			return mutateErr
		}

		batchJob.Spec.Template.Spec.Containers[0].Env = p.setupEnvVars(prune)
		prune.Spec.AppendEnvFromToContainer(&batchJob.Spec.Template.Spec.Containers[0])
		batchJob.Spec.Template.Spec.Containers[0].Args = append([]string{"-prune"}, executor.BuildTagArgs(prune.Spec.Retention.Tags)...)
		batchJob.Labels[job.K8upExclusive] = "true"
		return nil
	})
	if err != nil {
		p.SetConditionFalseWithMessage(p.CTX, k8upv1.ConditionReady, k8upv1.ReasonCreationFailed, "could not create job: %v", err)
		return err
	}

	p.SetStarted(p.CTX, "the job '%v/%v' was created", batchJob.Namespace, batchJob.Name)
	return nil
}

// Exclusive should return true for jobs that can't run while other jobs run.
func (p *PruneExecutor) Exclusive() bool {
	return true
}

func (p *PruneExecutor) cleanupOldPrunes(ctx context.Context, prune *k8upv1.Prune) {
	p.CleanupOldResources(ctx, &k8upv1.PruneList{}, prune.Namespace, prune)
}

func (p *PruneExecutor) setupEnvVars(prune *k8upv1.Prune) []corev1.EnvVar {
	vars := executor.NewEnvVarConverter()

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

	err := vars.Merge(executor.DefaultEnv(p.Obj.GetNamespace()))
	if err != nil {
		p.Log.Error(err, "error while merging the environment variables", "name", p.Obj.GetName(), "namespace", p.Obj.GetNamespace())
	}

	return vars.Convert()
}
