package executor

import (
	"errors"
	"github.com/vshn/k8up/cfg"
	"strconv"
	"strings"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/job"
	"github.com/vshn/k8up/observer"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// Execute creates the actual batch.job on the k8s api.
func (p *PruneExecutor) Execute() error {
	prune, ok := p.Obj.(*k8upv1alpha1.Prune)
	if !ok {
		return errors.New("object is not a prune")
	}

	if prune.GetK8upStatus().Started {
		return nil
	}

	jobObj, err := job.GetGenericJob(prune, p.Config)
	jobObj.GetLabels()[job.K8upExclusive] = "true"
	if err != nil {
		return err
	}

	p.startPrune(jobObj, prune)

	return nil
}

// Exclusive should return true for jobs that can't run while other jobs run.
func (p *PruneExecutor) Exclusive() bool {
	return true
}

func (p *PruneExecutor) startPrune(job *batchv1.Job, prune *k8upv1alpha1.Prune) {
	name := types.NamespacedName{Namespace: p.Obj.GetMetaObject().GetNamespace(), Name: p.Obj.GetMetaObject().GetName()}
	p.setPruneCallback(name, prune)

	job.Spec.Template.Spec.Containers[0].Env = p.setupEnvVars(prune)
	job.Spec.Template.Spec.Containers[0].Args = []string{"-prune"}

	if err := p.Client.Create(p.CTX, job); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			p.Log.Error(err, "could not create job")
			return
		}
	}

	if err := p.setJobStatusStarted(); err != nil {
		p.Log.Error(err, "could not update prune status")
	}
}

func (p *PruneExecutor) setJobStatusStarted() error {
	original := p.Obj
	new := p.Obj.GetRuntimeObject().DeepCopyObject().(*k8upv1alpha1.Prune)
	new.GetK8upStatus().Started = true

	return p.Client.Status().Patch(p.CTX, new, client.MergeFrom(original.GetRuntimeObject()))
}

func (p *PruneExecutor) setPruneCallback(name types.NamespacedName, prune *k8upv1alpha1.Prune) {
	observer.GetObserver().RegisterCallback(name.String(), func() {
		p.cleanupOldPrunes(name, prune)
	})
}

func (p *PruneExecutor) cleanupOldPrunes(name types.NamespacedName, prune *k8upv1alpha1.Prune) {
	pruneList := &k8upv1alpha1.PruneList{}
	err := p.Client.List(p.CTX, pruneList, &client.ListOptions{
		Namespace: name.Namespace,
	})
	if err != nil {
		p.Log.Error(err, "could not list objects to cleanup old prunes", "Namespace", name.Namespace)
	}

	jobs := make(jobObjectList, len(pruneList.Items))
	for i, prune := range pruneList.Items {
		jobs[i] = &prune
	}

	var keepJobs *int = prune.Spec.KeepJobs

	err = cleanOldObjects(jobs, getKeepJobs(keepJobs), p.Config)
	if err != nil {
		p.Log.Error(err, "could not delete old prunes", "namespace", name.Namespace)
	}
}

func (p *PruneExecutor) setupEnvVars(prune *k8upv1alpha1.Prune) []corev1.EnvVar {
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
