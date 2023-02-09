// Job handles the internal representation of a job and it's context.

package job

import (
	"context"
	"crypto/sha256"
	"fmt"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/monitoring"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// K8uplabel is a label that is required for the operator to differentiate
	// batchv1.job objects managed by k8up from others.
	K8uplabel = "k8upjob"
	// K8upExclusive is needed to determine if a given job is considered exclusive or not.
	K8upExclusive = "k8upjob/exclusive"
)

// Config represents the whole context for a given job. It contains everything
// that is necessary to handle the job.
type Config struct {
	Client     client.Client
	Obj        k8upv1.JobObject
	Repository string
}

// NewConfig returns a new configuration.
func NewConfig(client client.Client, obj k8upv1.JobObject, repository string) Config {
	return Config{
		Client:     client,
		Obj:        obj,
		Repository: repository,
	}
}

// MutateBatchJob mutates the given Job with generic spec applicable to all K8up-spawned Jobs.
func MutateBatchJob(batchJob *batchv1.Job, jobObj k8upv1.JobObject, config Config) error {
	batchJob.Labels = labels.Merge(batchJob.Labels, labels.Set{
		K8uplabel:                  "true",
		k8upv1.LabelK8upType:       jobObj.GetType().String(),
		k8upv1.LabelK8upOwnedBy:    jobObj.GetType().String() + "_" + jobObj.GetName(),
		k8upv1.LabelRepositoryHash: Sha256Hash(config.Repository),
	})

	batchJob.Spec.ActiveDeadlineSeconds = config.Obj.GetActiveDeadlineSeconds()
	batchJob.Spec.Template.Labels = labels.Merge(batchJob.Spec.Template.Labels, labels.Set{
		K8uplabel: "true",
	})
	batchJob.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyOnFailure
	batchJob.Spec.Template.Spec.SecurityContext = jobObj.GetPodSecurityContext()

	containers := batchJob.Spec.Template.Spec.Containers
	if len(containers) == 0 {
		containers = make([]corev1.Container, 1)
	}
	containers[0].Name = config.Obj.GetType().String()
	containers[0].Image = cfg.Config.BackupImage
	containers[0].Command = cfg.Config.BackupCommandRestic
	containers[0].Resources = config.Obj.GetResources()
	batchJob.Spec.Template.Spec.Containers = containers

	return controllerruntime.SetControllerReference(jobObj, batchJob, config.Client.Scheme())
}

func ReconcileJobStatus(ctx context.Context, key types.NamespacedName, client client.Client, obj k8upv1.JobObject) error {
	log := controllerruntime.LoggerFrom(ctx)
	log.V(1).Info("reconciling job", "key", key)

	batchJob := &batchv1.Job{}
	err := client.Get(ctx, key, batchJob)
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("unable to get job: %w", err)
		}
		log.V(1).Info("job not found", "key", key)
		return nil
	}

	UpdateStatus(ctx, batchJob, obj)

	log.V(1).Info("updating status")
	if err := client.Status().Update(ctx, obj); err != nil {
		return fmt.Errorf("obj status update failed: %w", err)
	}
	return nil
}

// UpdateStatus retrieves status of batchJob and sets status of obj accordingly.
func UpdateStatus(ctx context.Context, batchJob *batchv1.Job, obj k8upv1.JobObject) {
	// update status conditions based on Job status
	objStatus := obj.GetStatus()
	message := fmt.Sprintf("job '%s' has %d active, %d succeeded and %d failed pods",
		batchJob.Name, batchJob.Status.Active, batchJob.Status.Succeeded, batchJob.Status.Failed)

	if HasSucceeded(batchJob.Status.Conditions) {
		SetSucceeded(ctx, batchJob.Name, batchJob.Namespace, obj.GetType(), &objStatus, message)
	}
	if HasFailed(batchJob.Status.Conditions) {
		SetFailed(ctx, batchJob.Name, batchJob.Namespace, obj.GetType(), &objStatus, message)
	}
	if HasStarted(batchJob.Status.Conditions) {
		objStatus.SetStarted(message)
	}
	obj.SetStatus(objStatus)
}

func HasSucceeded(conditions []batchv1.JobCondition) bool {
	successCond := FindStatusCondition(conditions, batchv1.JobComplete)
	return successCond != nil && successCond.Status == corev1.ConditionTrue
}
func HasFailed(conditions []batchv1.JobCondition) bool {
	failedCond := FindStatusCondition(conditions, batchv1.JobFailed)
	return failedCond != nil && failedCond.Status == corev1.ConditionTrue
}
func HasStarted(conditions []batchv1.JobCondition) bool {
	successCond := FindStatusCondition(conditions, batchv1.JobComplete)
	failedCond := FindStatusCondition(conditions, batchv1.JobFailed)
	return successCond == nil && failedCond == nil
}

func SetSucceeded(ctx context.Context, name, ns string, typ k8upv1.JobType, objStatus *k8upv1.Status, message string) {
	log := controllerruntime.LoggerFrom(ctx)

	if !objStatus.HasSucceeded() {
		// only increase success counter if new condition
		monitoring.IncSuccessCounters(ns, typ)
		log.Info("Job succeeded")
	}
	objStatus.SetSucceeded(message)
	objStatus.SetFinished(fmt.Sprintf("job '%s' completed successfully", name))
}
func SetFailed(ctx context.Context, name, ns string, typ k8upv1.JobType, objStatus *k8upv1.Status, message string) {
	log := controllerruntime.LoggerFrom(ctx)

	if !objStatus.HasFailed() {
		// only increase fail counter if new condition
		monitoring.IncFailureCounters(ns, typ)
		log.Info("Job failed")
	}
	objStatus.SetFailed(message)
	objStatus.SetFinished(fmt.Sprintf("job '%s' has failed", name))
}

// FindStatusCondition finds the condition with the given type in the batchv1.JobCondition slice.
// Returns nil if not found.
func FindStatusCondition(conditions []batchv1.JobCondition, conditionType batchv1.JobConditionType) *batchv1.JobCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

// Sha256Hash returns the SHA256 hash string of the given string
// Returns empty string if v is empty.
// The returned hash is shortened to 63 characters to fit into a label.
func Sha256Hash(v string) string {
	if v == "" {
		return ""
	}
	h := sha256.New()
	h.Write([]byte(v))
	return fmt.Sprintf("%x", h.Sum(nil))[:63]
}
