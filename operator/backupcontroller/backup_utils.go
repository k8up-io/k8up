package backupcontroller

import (
	"path"

	"github.com/k8up-io/k8up/v2/operator/executor"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/k8up-io/k8up/v2/operator/cfg"
)

func (b *BackupExecutor) fetchPVCs(list client.ObjectList) error {
	return b.Client.List(b.CTX, list, client.InNamespace(b.backup.Namespace))
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

func containsAccessMode(s []corev1.PersistentVolumeAccessMode, e string) bool {
	for _, a := range s {
		if string(a) == e {
			return true
		}
	}
	return false
}

func (b *BackupExecutor) createServiceAccountAndBinding() error {
	sa := &corev1.ServiceAccount{}
	sa.Name = cfg.Config.ServiceAccount
	sa.Namespace = b.backup.Namespace
	_, err := controllerruntime.CreateOrUpdate(b.CTX, b.Client, sa, func() error {
		return nil
	})
	if err != nil {
		return err
	}

	role := &rbacv1.Role{}
	role.Name = cfg.Config.PodExecRoleName
	role.Namespace = b.backup.Namespace
	_, err = controllerruntime.CreateOrUpdate(b.CTX, b.Client, role, func() error {
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "pods/exec"},
				Verbs:     []string{"*"},
			},
		}
		return nil
	})

	if err != nil {
		return err
	}
	roleBinding := &rbacv1.RoleBinding{}
	roleBinding.Name = cfg.Config.PodExecRoleName + "-namespaced"
	roleBinding.Namespace = b.backup.Namespace
	_, err = controllerruntime.CreateOrUpdate(b.CTX, b.Client, roleBinding, func() error {
		roleBinding.Subjects = []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: b.backup.Namespace,
				Name:      sa.Name,
			},
		}
		roleBinding.RoleRef = rbacv1.RoleRef{
			Kind:     "Role",
			Name:     role.Name,
			APIGroup: "rbac.authorization.k8s.io",
		}
		return nil
	})
	return err
}

func (b *BackupExecutor) setupEnvVars() []corev1.EnvVar {
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

	err := vars.Merge(executor.DefaultEnv(b.Obj.GetNamespace()))
	if err != nil {
		b.Log.Error(err, "error while merging the environment variables", "name", b.Obj.GetName(), "namespace", b.Obj.GetNamespace())
	}

	return vars.Convert()
}
