package backupcontroller

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/k8up-io/k8up/v2/operator/utils"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/executor"
	"github.com/k8up-io/k8up/v2/operator/job"
	"github.com/k8up-io/k8up/v2/restic/kubernetes"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BackupExecutor creates a batch.job object on the cluster. It merges all the
// information provided by defaults and the CRDs to ensure the backup has all information to run.
type BackupExecutor struct {
	executor.Generic
	backup *k8upv1.Backup
}

// NewBackupExecutor returns a new BackupExecutor.
func NewBackupExecutor(config job.Config) *BackupExecutor {
	return &BackupExecutor{Generic: executor.Generic{Config: config}, backup: config.Obj.(*k8upv1.Backup)}
}

// GetConcurrencyLimit returns the concurrent jobs limit
func (b *BackupExecutor) GetConcurrencyLimit() int {
	return cfg.Config.GlobalConcurrentBackupJobsLimit
}

// Execute triggers the actual batch.job creation on the cluster.
// It will also register a callback function on the observer so the PreBackupPods can be removed after the backup has finished.
func (b *BackupExecutor) Execute(ctx context.Context) error {
	err := b.createServiceAccountAndBinding(ctx)
	if err != nil {
		return err
	}

	return b.startBackup(ctx)
}

type backupItem struct {
	volume      corev1.Volume
	node        string
	tolerations []corev1.Toleration
	targetPod   string
	resticArgs  utils.JsonArgsArray
}

// listAndFilterPVCs lists all PVCs in the given namespace and filters them for K8up specific usage.
// Specifically, non-RWX PVCs will be skipped, as well PVCs that have the given annotation.
func (b *BackupExecutor) listAndFilterPVCs(ctx context.Context, annotation string) ([]backupItem, error) {
	log := controllerruntime.LoggerFrom(ctx)

	pods := &corev1.PodList{}
	pvcPodMap := make(map[string]corev1.Pod)
	labelselector, _ := labels.Parse("!" + job.K8uplabel)
	if err := b.Client.List(ctx, pods, client.InNamespace(b.backup.Namespace), client.MatchingLabelsSelector{Selector: labelselector}); err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			log.V(1).Info("Ignoring Pod which is not running", "pod", pod.GetName())
			continue
		}
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				pvcPodMap[volume.PersistentVolumeClaim.ClaimName] = pod
				log.V(1).Info("pvc pod map", "claimName", volume.PersistentVolumeClaim.ClaimName, "pod", pod.GetName())
			}
		}
	}

	backupItems := make([]backupItem, 0)
	claimlist := &corev1.PersistentVolumeClaimList{}

	log.Info("Listing all PVCs", "annotation", annotation)
	if err := b.fetchPVCs(ctx, claimlist); err != nil {
		return backupItems, err
	}

	for _, pvc := range claimlist.Items {
		if pvc.Status.Phase != corev1.ClaimBound {
			log.Info("PVC is not bound, skipping PVC", "pvc", pvc.GetName())
			continue
		}

		backupAnnotation, hasBackupAnnotation := pvc.GetAnnotations()[annotation]

		isRWO := containsAccessMode(pvc.Spec.AccessModes, corev1.ReadWriteOnce)
		if !containsAccessMode(pvc.Spec.AccessModes, corev1.ReadWriteMany) && !isRWO && !hasBackupAnnotation {
			log.Info("PVC is neither RWX nor RWO and has no backup annotation, skipping PVC", "pvc", pvc.GetName())
			continue
		}

		if !hasBackupAnnotation {
			if cfg.Config.SkipWithoutAnnotation {
				log.Info("PVC doesn't have annotation and BACKUP_SKIP_WITHOUT_ANNOTATION is true, skipping PVC", "pvc", pvc.GetName())
				continue
			} else {
				log.Info("PVC doesn't have annotation, adding to list", "pvc", pvc.GetName())
			}
		} else if shouldBackup, _ := strconv.ParseBool(backupAnnotation); !shouldBackup {
			log.Info("PVC skipped due to annotation", "pvc", pvc.GetName(), "annotation", backupAnnotation)
			continue
		} else {
			log.Info("Adding to list", "pvc", pvc.Name)
		}

		bi := backupItem{
			volume: corev1.Volume{
				Name: pvc.Name,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvc.Name,
					},
				},
			},
		}

		resticArgsAnnotation, hasResticArgsAnnotation := pvc.GetAnnotations()[cfg.Config.BackupResticArgsAnnotation]
		if hasResticArgsAnnotation {
			var argsArray utils.JsonArgsArray
			if err := argsArray.UnmarshalJSON([]byte(resticArgsAnnotation)); err != nil {
				log.Error(err, "failed to parse restic backup args from the annotation, skipping pvc", "annotation", resticArgsAnnotation)
				continue
			} else {
				log.Info("adding restic args from annotation", "annotation", resticArgsAnnotation)
				bi.resticArgs = argsArray
			}
		}

		if pod, ok := pvcPodMap[pvc.GetName()]; ok {
			bi.node = pod.Spec.NodeName
			bi.tolerations = pod.Spec.Tolerations
			bi.targetPod = pod.GetName()

			log.V(1).Info("PVC mounted at pod", "pvc", pvc.GetName(), "targetPod", bi.targetPod, "node", bi.node, "tolerations", bi.tolerations)
		} else if isRWO {
			pv := &corev1.PersistentVolume{}
			if err := b.Client.Get(ctx, types.NamespacedName{Name: pvc.Spec.VolumeName}, pv); err != nil {
				log.Error(err, "unable to get PV, skipping pvc", "pvc", pvc.GetName(), "pv", pvc.Spec.VolumeName)
				continue
			}

			bi.node = findNode(pv, pvc)
			if bi.node == "" {
				log.Info("RWO PVC not bound and no PV node affinity set, adding", "pvc", pvc.GetName(), "affinity", pv.Spec.NodeAffinity)
			}
			log.V(1).Info("node found in PV or PVC", "pvc", pvc.GetName(), "node", bi.node)
		} else {
			log.Info("RWX PVC with no specific node", "pvc", pvc.GetName())
		}

		backupItems = append(backupItems, bi)
	}

	return backupItems, nil
}

// findNode tries to find a PVs NodeAffinity for a specific hostname. If found will return that.
// If not it will try to return the value of the k8up.io/hostname annotation on the PVC. If this is not set, will return
// empty string.
func findNode(pv *corev1.PersistentVolume, pvc corev1.PersistentVolumeClaim) string {
	hostnameAnnotation := pvc.Annotations[k8upv1.AnnotationK8upHostname]
	if pv.Spec.NodeAffinity == nil || pv.Spec.NodeAffinity.Required == nil {
		return hostnameAnnotation
	}
	for _, term := range pv.Spec.NodeAffinity.Required.NodeSelectorTerms {
		for _, matchExpr := range term.MatchExpressions {
			if matchExpr.Key == corev1.LabelHostname && matchExpr.Operator == corev1.NodeSelectorOpIn {
				return matchExpr.Values[0]
			}
		}
	}
	return hostnameAnnotation
}

func (b *BackupExecutor) startBackup(ctx context.Context) error {
	ready, err := b.StartPreBackup(ctx)
	if err != nil {
		return err
	}
	if !ready || b.backup.Status.IsWaitingForPreBackup() {
		return nil
	}

	backupItems, err := b.listAndFilterPVCs(ctx, cfg.Config.BackupAnnotation)
	if err != nil {
		b.SetConditionFalseWithMessage(ctx, k8upv1.ConditionReady, k8upv1.ReasonRetrievalFailed, "%s", err.Error())
		return err
	}

	type jobItem struct {
		job           *batchv1.Job
		targetPods    []string
		volumes       []corev1.Volume
		resticArgs    []string
		skipPreBackup bool
	}
	backupJobs := map[string]jobItem{}
	for index, item := range backupItems {
		jobName := item.node + strings.Join(item.resticArgs, ",")
		if _, ok := backupJobs[jobName]; !ok {
			backupJobs[jobName] = jobItem{
				job:           b.createJob(strconv.Itoa(index), item.node, item.tolerations),
				targetPods:    make([]string, 0),
				volumes:       make([]corev1.Volume, 0),
				resticArgs:    []string(item.resticArgs),
				skipPreBackup: true,
			}
		}

		j := backupJobs[jobName]
		if item.targetPod != "" {
			j.targetPods = append(j.targetPods, item.targetPod)
		}
		j.volumes = append(j.volumes, item.volume)
		backupJobs[jobName] = j
	}

	log := controllerruntime.LoggerFrom(ctx)
	// validating that the correct backup annotations exist is already handled in the restic module
	// we only need to pass it a list of pods it should go through

	podLister := kubernetes.NewPodLister(ctx, b.Client, cfg.Config.BackupCommandAnnotation, "", "", b.backup.Namespace, nil, false, log)
	viableBackupPods, err := podLister.ListPods()
	if err != nil {
		log.Error(err, "could not list pods", "namespace", b.backup.Namespace)
		return fmt.Errorf("could not list pods: %w", err)
	}

	viableBackupPodNames := make([]string, 0, len(viableBackupPods))
	for _, pod := range viableBackupPods {
		viableBackupPodNames = append(viableBackupPodNames, pod.PodName)
	}

	backupPods := &corev1.PodList{}
	err = b.fetchCandidatePods(ctx, backupPods)
	if err != nil {
		return fmt.Errorf("failed to fetch candidate pods: %w", err)
	}

	for i := len(backupPods.Items) - 1; i >= 0; i-- {
		pod := backupPods.Items[i]
		if !slices.Contains(viableBackupPodNames, pod.Name) {
			backupPods.Items = append(backupPods.Items[:i], backupPods.Items[i+1:]...)
		}
	}

	if len(backupPods.Items) > 0 {
		targetPods := make([]string, 0)
		for _, pod := range backupPods.Items {
			if slices.Contains(viableBackupPodNames, pod.Name) {
				targetPods = append(targetPods, pod.Name)
			}
		}
		backupJobs["prebackup"] = jobItem{
			job:           b.createJob("prebackup", "", nil),
			targetPods:    targetPods,
			volumes:       make([]corev1.Volume, 0),
			skipPreBackup: false,
		}
	}

	index := 0
	for name, batchJob := range backupJobs {
		_, err = controllerruntime.CreateOrUpdate(ctx, b.Client, batchJob.job, func() error {
			mutateErr := job.MutateBatchJob(ctx, batchJob.job, b.backup, b.Config, b.Client)
			if mutateErr != nil {
				return mutateErr
			}

			vars, setupErr := b.setupEnvVars()
			if setupErr != nil {
				return setupErr
			}
			batchJob.job.Spec.Template.Spec.Containers[0].Env = append(batchJob.job.Spec.Template.Spec.Containers[0].Env, vars...)
			if len(batchJob.targetPods) > 0 {
				batchJob.job.Spec.Template.Spec.Containers[0].Env = append(batchJob.job.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
					Name:  "TARGET_PODS",
					Value: strings.Join(batchJob.targetPods, ","),
				})
			}
			if batchJob.skipPreBackup {
				batchJob.job.Spec.Template.Spec.Containers[0].Env = append(batchJob.job.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
					Name:  "SKIP_PREBACKUP",
					Value: "true",
				})
			}
			// each job sleeps for index seconds to avoid concurrent restic repository creation. Not the prettiest way but it works and a repository
			// is created only once usually.
			if name == "prebackup" || index != 0 {
				batchJob.job.Spec.Template.Spec.Containers[0].Env = append(batchJob.job.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
					Name:  "SLEEP_DURATION",
					Value: (3 * time.Second).String(),
				})
			}
			b.backup.Spec.AppendEnvFromToContainer(&batchJob.job.Spec.Template.Spec.Containers[0])
			batchJob.job.Spec.Template.Spec.Volumes = append(batchJob.job.Spec.Template.Spec.Volumes, batchJob.volumes...)
			batchJob.job.Spec.Template.Spec.Volumes = append(batchJob.job.Spec.Template.Spec.Volumes, utils.AttachEmptyDirVolumes(b.backup.Spec.Volumes)...)
			batchJob.job.Spec.Template.Spec.Containers[0].VolumeMounts = append(b.newVolumeMounts(batchJob.volumes), b.attachTLSVolumeMounts()...)
			batchJob.job.Spec.BackoffLimit = ptr.To(int32(cfg.Config.GlobalBackoffLimit))

			batchJob.job.Spec.Template.Spec.Containers[0].Args = b.setupArgs(batchJob.resticArgs)

			if batchJob.job.Spec.Template.Spec.ServiceAccountName == "" {
				batchJob.job.Spec.Template.Spec.ServiceAccountName = cfg.Config.ServiceAccount
			}

			index++
			return nil
		})
		if err != nil {
			return fmt.Errorf("unable to createOrUpdate(%q): %w", batchJob.job.Name, err)
		}
	}

	if len(backupJobs) == 0 {
		status := b.Obj.GetStatus()
		status.SetSucceeded("nothing to backup")
		b.Obj.SetStatus(status)
		return b.Client.Status().Update(ctx, b.Obj)
	}

	return nil
}

func (b *BackupExecutor) createJob(name, node string, tolerations []corev1.Toleration) *batchv1.Job {
	batchJob := &batchv1.Job{}
	batchJob.Name = b.jobName(name)
	batchJob.Namespace = b.backup.Namespace
	batchJob.Spec.Template.Spec.Volumes = make([]corev1.Volume, 0)
	if node != "" {
		batchJob.Spec.Template.Spec.NodeSelector = map[string]string{
			corev1.LabelHostname: node,
		}
	}
	batchJob.Spec.Template.Spec.Tolerations = tolerations
	return batchJob
}

func (b *BackupExecutor) cleanupOldBackups(ctx context.Context) {
	b.CleanupOldResources(ctx, &k8upv1.BackupList{}, b.backup.Namespace, b.backup)
}

func (b *BackupExecutor) jobName(name string) string {
	return k8upv1.BackupType.String() + "-" + b.backup.Name + "-" + name
}

func (b *BackupExecutor) attachTLSVolumeMounts() []corev1.VolumeMount {
	var tlsVolumeMounts []corev1.VolumeMount
	if b.backup.Spec.Backend != nil && !utils.ZeroLen(b.backup.Spec.Backend.VolumeMounts) {
		tlsVolumeMounts = append(tlsVolumeMounts, *b.backup.Spec.Backend.VolumeMounts...)
	}

	return utils.AttachEmptyDirVolumeMounts(cfg.Config.PodVarDir, &tlsVolumeMounts)
}
