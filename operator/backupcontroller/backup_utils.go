package backupcontroller

import (
	"context"
	"fmt"
	"github.com/k8up-io/k8up/v2/operator/executor"
	"github.com/k8up-io/k8up/v2/operator/utils"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"path"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k8up-io/k8up/v2/operator/cfg"
)

const _dataDirName = "k8up-dir"

func (b *BackupExecutor) fetchPVCs(ctx context.Context, list client.ObjectList) error {
	return b.Config.Client.List(ctx, list, client.InNamespace(b.backup.Namespace))
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
	_, err := controllerruntime.CreateOrUpdate(ctx, b.Config.Client, sa, func() error {
		return nil
	})
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}
	roleBinding := &rbacv1.RoleBinding{}
	roleBinding.Name = cfg.Config.PodExecRoleName + "-namespaced"
	roleBinding.Namespace = b.backup.Namespace
	_, err = controllerruntime.CreateOrUpdate(ctx, b.Config.Client, roleBinding, func() error {
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

func (b *BackupExecutor) setupArgs() []string {
	args := []string{"--varDir", cfg.Config.PodVarDir}
	if len(b.backup.Spec.Tags) > 0 {
		args = append(args, executor.BuildTagArgs(b.backup.Spec.Tags)...)
	}
	args = append(args, b.appendTLSOptionsArgs()...)

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

func (b *BackupExecutor) attachMoreVolumes() []corev1.Volume {
	ku8pVolume := corev1.Volume{
		Name:         _dataDirName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}

	if utils.ZeroLen(b.backup.Spec.Volumes) {
		return []corev1.Volume{ku8pVolume}
	}

	moreVolumes := make([]corev1.Volume, 0, len(*b.backup.Spec.Volumes)+1)
	moreVolumes = append(moreVolumes, ku8pVolume)
	for _, v := range *b.backup.Spec.Volumes {
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

		addVolume := corev1.Volume{
			Name:         vol.Name,
			VolumeSource: volumeSource,
		}
		moreVolumes = append(moreVolumes, addVolume)
	}

	return moreVolumes
}

func (b *BackupExecutor) attachMoreVolumeMounts() []corev1.VolumeMount {
	var volumeMount []corev1.VolumeMount

	if b.backup.Spec.Backend != nil && !utils.ZeroLen(b.backup.Spec.Backend.VolumeMounts) {
		volumeMount = *b.backup.Spec.Backend.VolumeMounts
	}

	addVolumeMount := corev1.VolumeMount{
		Name:      _dataDirName,
		MountPath: cfg.Config.PodVarDir,
	}
	volumeMount = append(volumeMount, addVolumeMount)

	return volumeMount
}

func (b *BackupExecutor) appendTLSOptionsArgs() []string {
	var args []string

	if !(b.backup.Spec.Backend != nil && b.backup.Spec.Backend.TLSOptions != nil) {
		return args
	}

	if b.backup.Spec.Backend.TLSOptions.CACert != "" {
		args = append(args, []string{"-caCert", b.backup.Spec.Backend.TLSOptions.CACert}...)
	}
	if b.backup.Spec.Backend.TLSOptions.ClientCert != "" && b.backup.Spec.Backend.TLSOptions.ClientKey != "" {
		addMoreArgs := []string{
			"-clientCert",
			b.backup.Spec.Backend.TLSOptions.ClientCert,
			"-clientKey",
			b.backup.Spec.Backend.TLSOptions.ClientKey,
		}
		args = append(args, addMoreArgs...)
	}

	return args
}
