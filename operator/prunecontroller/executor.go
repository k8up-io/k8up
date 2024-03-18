package prunecontroller

import (
	"context"
	"github.com/k8up-io/k8up/v2/operator/utils"
	"strconv"
	"strings"

	"github.com/k8up-io/k8up/v2/operator/executor"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
)

const _dataDirName = "k8up-dir"

// PruneExecutor will execute the batch.job for Prunes.
type PruneExecutor struct {
	executor.Generic
	prune *k8upv1.Prune
}

// NewPruneExecutor will return a new executor for Prune jobs.
func NewPruneExecutor(config job.Config) *PruneExecutor {
	return &PruneExecutor{
		Generic: executor.Generic{Config: config},
		prune:   config.Obj.(*k8upv1.Prune),
	}
}

// Execute creates the actual batch.job on the k8s api.
func (p *PruneExecutor) Execute(ctx context.Context) error {
	batchJob := &batchv1.Job{}
	batchJob.Name = p.jobName()
	batchJob.Namespace = p.prune.Namespace

	_, err := controllerutil.CreateOrUpdate(ctx, p.Client, batchJob, func() error {
		mutateErr := job.MutateBatchJob(batchJob, p.prune, p.Config)
		if mutateErr != nil {
			return mutateErr
		}

		batchJob.Spec.Template.Spec.Containers[0].Env = p.setupEnvVars(ctx, p.prune)
		batchJob.Spec.Template.Spec.ServiceAccountName = cfg.Config.ServiceAccount
		p.prune.Spec.AppendEnvFromToContainer(&batchJob.Spec.Template.Spec.Containers[0])
		batchJob.Spec.Template.Spec.Containers[0].VolumeMounts = p.attachMoreVolumeMounts()
		batchJob.Spec.Template.Spec.Volumes = p.attachMoreVolumes()
		batchJob.Labels[job.K8upExclusive] = "true"

		args, argsErr := p.setupArgs()
		batchJob.Spec.Template.Spec.Containers[0].Args = args

		return argsErr
	})
	if err != nil {
		p.SetConditionFalseWithMessage(ctx, k8upv1.ConditionReady, k8upv1.ReasonCreationFailed, "could not create job: %v", err)
		return err
	}

	p.SetStarted(ctx, "the job '%v/%v' was created", batchJob.Namespace, batchJob.Name)
	return nil
}

func (p *PruneExecutor) jobName() string {
	return k8upv1.PruneType.String() + "-" + p.prune.Name
}

func (p *PruneExecutor) setupArgs() ([]string, error) {
	args := p.appendOptionsArgs()

	args = append(args, "-prune")
	if len(p.prune.Spec.Retention.Tags) > 0 {
		args = append(args, executor.BuildTagArgs(p.prune.Spec.Retention.Tags)...)
	}

	return args, nil
}

// Exclusive should return true for jobs that can't run while other jobs run.
func (p *PruneExecutor) Exclusive() bool {
	return true
}

func (p *PruneExecutor) cleanupOldPrunes(ctx context.Context, prune *k8upv1.Prune) {
	p.CleanupOldResources(ctx, &k8upv1.PruneList{}, prune.Namespace, prune)
}

func (p *PruneExecutor) setupEnvVars(ctx context.Context, prune *k8upv1.Prune) []corev1.EnvVar {
	log := controllerruntime.LoggerFrom(ctx)
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
		log.Error(err, "error while merging the environment variables", "name", p.Obj.GetName(), "namespace", p.Obj.GetNamespace())
	}

	return vars.Convert()
}

func (p *PruneExecutor) appendOptionsArgs() []string {
	var args []string

	args = append(args, []string{"--varDir", cfg.Config.PodVarDir}...)

	if p.prune.Spec.Backend.Options != nil {
		if p.prune.Spec.Backend.Options.CACert != "" {
			args = append(args, []string{"--caCert", p.prune.Spec.Backend.Options.CACert}...)
		}
		if p.prune.Spec.Backend.Options.ClientCert != "" && p.prune.Spec.Backend.Options.ClientKey != "" {
			args = append(
				args,
				[]string{
					"--clientCert",
					p.prune.Spec.Backend.Options.ClientCert,
					"--clientKey",
					p.prune.Spec.Backend.Options.ClientKey,
				}...,
			)
		}
	}

	return args
}

func (p *PruneExecutor) attachMoreVolumes() []corev1.Volume {
	ku8pVolume := corev1.Volume{
		Name:         _dataDirName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}

	if utils.ZeroLen(p.prune.Spec.Volumes) {
		return []corev1.Volume{ku8pVolume}
	}

	moreVolumes := make([]corev1.Volume, 0, len(*p.prune.Spec.Volumes)+1)
	moreVolumes = append(moreVolumes, ku8pVolume)
	for _, v := range *p.prune.Spec.Volumes {
		vol := v

		var volumeSource corev1.VolumeSource
		if vol.PersistentVolumeClaim != nil {
			volumeSource.PersistentVolumeClaim = vol.PersistentVolumeClaim
		} else if vol.Secret != nil {
			volumeSource.Secret = vol.Secret
		} else if vol.ConfigMap != nil {
			volumeSource.ConfigMap = vol.ConfigMap
		} else {
			continue
		}

		moreVolumes = append(moreVolumes, corev1.Volume{
			Name:         vol.Name,
			VolumeSource: volumeSource,
		})
	}

	return moreVolumes
}

func (p *PruneExecutor) attachMoreVolumeMounts() []corev1.VolumeMount {
	var volumeMount []corev1.VolumeMount

	if p.prune.Spec.Backend.S3 != nil && !utils.ZeroLen(p.prune.Spec.Backend.S3.VolumeMounts) {
		volumeMount = *p.prune.Spec.Backend.S3.VolumeMounts
	}
	if p.prune.Spec.Backend.Rest != nil && !utils.ZeroLen(p.prune.Spec.Backend.Rest.VolumeMounts) {
		volumeMount = *p.prune.Spec.Backend.Rest.VolumeMounts
	}

	ku8pVolumeMount := corev1.VolumeMount{Name: _dataDirName, MountPath: cfg.Config.PodVarDir}
	volumeMount = append(volumeMount, ku8pVolumeMount)

	return volumeMount
}
