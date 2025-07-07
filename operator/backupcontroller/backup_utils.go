package backupcontroller

import (
	"context"
	"fmt"
	"maps"
	"path"
	"slices"

	"github.com/k8up-io/k8up/v2/operator/executor"
	"github.com/k8up-io/k8up/v2/operator/utils"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k8up-io/k8up/v2/operator/cfg"
)

func (b *BackupExecutor) fetchPVCs(ctx context.Context, list *corev1.PersistentVolumeClaimList) (err error) {
	err = nil
	if b.backup.Spec.LabelSelectors == nil {
		return b.Client.List(ctx, list, client.InNamespace(b.backup.Namespace))
	}

	labelSelectors := b.backup.Spec.LabelSelectors
	uniquePVCs := make(map[string]corev1.PersistentVolumeClaim)

	for _, labelSelector := range labelSelectors {
		selector, err := metav1.LabelSelectorAsSelector(&labelSelector)
		if err != nil {
			return fmt.Errorf("cannot convert labelSelector %v to selector: %w", labelSelector, err)
		}

		options := client.ListOptions{
			LabelSelector: selector,
		}

		matchingPVCs := &corev1.PersistentVolumeClaimList{}
		err = b.Client.List(ctx, matchingPVCs, client.InNamespace(b.backup.Namespace), &options)

		if err != nil {
			return fmt.Errorf("cannot list PVCs using labelSelector %v: %w", labelSelector, err)
		}

		for _, pvc := range matchingPVCs.Items {
			uniquePVCs[pvc.Name] = pvc
		}

	}

	list.Items = slices.Collect(maps.Values(uniquePVCs))

	return err

}

// This is intentionally omitting annotation logic, as it's handled outside, in the restic modules
func (b *BackupExecutor) fetchCandidatePods(ctx context.Context, list *corev1.PodList) (err error) {

	if b.backup.Spec.LabelSelectors == nil {
		return b.Client.List(ctx, list, client.InNamespace(b.backup.Namespace))
	}

	// pods created by startPreBackup will not have the user-defined labels,
	// while existing, annotated pods will. We need to fetch both.
	ownerUID := string(b.Obj.GetUID())
	ownerSelector := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "k8up.io/ownerBackupUID",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{ownerUID},
			},
		},
	}

	labelSelectors := b.backup.Spec.LabelSelectors
	labelSelectors = append(labelSelectors, ownerSelector)
	uniquePods := make(map[string]corev1.Pod)

	for _, labelSelector := range labelSelectors {
		selector, err := metav1.LabelSelectorAsSelector(&labelSelector)
		if err != nil {
			return fmt.Errorf("cannot convert labelSelector %v to selector: %w", labelSelector, err)
		}

		options := client.ListOptions{
			LabelSelector: selector,
		}

		matchingPods := &corev1.PodList{}
		err = b.Client.List(ctx, matchingPods, client.InNamespace(b.backup.Namespace), &options)

		if err != nil {
			return fmt.Errorf("cannot list Pods using labelSelector %v: %w", labelSelector, err)
		}

		for _, pod := range matchingPods.Items {
			uniquePods[pod.Name] = pod
		}

	}

	list.Items = slices.Collect(maps.Values(uniquePods))

	return err
}

func (b *BackupExecutor) newVolumeMounts(claims []corev1.Volume) []corev1.VolumeMount {
	mounts := make([]corev1.VolumeMount, len(claims))
	for i, volume := range claims {
		mounts[i] = corev1.VolumeMount{
			Name:      volume.Name,
			MountPath: path.Join(cfg.Config.MountPath, volume.Name),
			ReadOnly:  true,
		}
	}
	return mounts
}

func containsAccessMode(s []corev1.PersistentVolumeAccessMode, e corev1.PersistentVolumeAccessMode) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func (b *BackupExecutor) createServiceAccountAndBinding(ctx context.Context) error {
	sa := &corev1.ServiceAccount{}
	sa.Name = cfg.Config.ServiceAccount
	sa.Namespace = b.backup.Namespace
	_, err := controllerruntime.CreateOrUpdate(ctx, b.Client, sa, func() error {
		return nil
	})
	if err != nil {
		return err
	}

	roleBinding := &rbacv1.RoleBinding{}
	roleBinding.Name = cfg.Config.PodExecRoleName + "-namespaced"
	roleBinding.Namespace = b.backup.Namespace
	_, err = controllerruntime.CreateOrUpdate(ctx, b.Client, roleBinding, func() error {
		roleBinding.Subjects = []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: b.backup.Namespace,
				Name:      sa.Name,
			},
		}
		roleBinding.RoleRef = rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "k8up-executor",
			APIGroup: "rbac.authorization.k8s.io",
		}
		return nil
	})
	return err
}

func (b *BackupExecutor) setupArgs(userArgs []string) []string {
	args := []string{"-varDir", cfg.Config.PodVarDir}
	if len(b.backup.Spec.Tags) > 0 {
		args = append(args, executor.BuildTagArgs(b.backup.Spec.Tags)...)
	}
	if b.backup.Spec.Backend != nil {
		args = append(args, utils.AppendTLSOptionsArgs(b.backup.Spec.Backend.TLSOptions)...)
	}
	args = append(args, userArgs...)

	return args
}

func (b *BackupExecutor) setupEnvVars() ([]corev1.EnvVar, error) {
	vars := executor.NewEnvVarConverter()

	if b.backup != nil {
		if b.backup.Spec.Backend != nil {
			for key, value := range b.backup.Spec.Backend.GetCredentialEnv() {
				vars.SetEnvVarSource(key, value)
			}
			vars.SetString(cfg.ResticRepositoryEnvName, b.backup.Spec.Backend.String())
		}
	}

	vars.SetStringOrDefault("STATS_URL", b.backup.Spec.StatsURL, cfg.Config.GlobalStatsURL)
	vars.SetStringOrDefault("PROM_URL", b.backup.Spec.PromURL, cfg.Config.PromURL)
	vars.SetString("BACKUPCOMMAND_ANNOTATION", cfg.Config.BackupCommandAnnotation)
	vars.SetString("FILEEXTENSION_ANNOTATION", cfg.Config.FileExtensionAnnotation)

	err := vars.Merge(executor.DefaultEnv(b.backup.GetNamespace()))
	if err != nil {
		return nil, fmt.Errorf("cannot merge environment variables: %w", err)
	}
	return vars.Convert(), nil
}
