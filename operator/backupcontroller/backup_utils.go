package backupcontroller

import (
	"path"

	"github.com/k8up-io/k8up/v2/operator/executor"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	role, sa, binding := newServiceAccountDefinition(b.backup.Namespace)
	for _, obj := range []client.Object{&role, &sa, &binding} {
		if err := b.CreateObjectIfNotExisting(obj); err != nil {
			return err
		}
	}
	return nil
}

func newServiceAccountDefinition(namespace string) (rbacv1.Role, corev1.ServiceAccount, rbacv1.RoleBinding) {
	role := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.Config.PodExecRoleName,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"pods",
					"pods/exec",
				},
				Verbs: []string{
					"*",
				},
			},
		},
	}

	roleBinding := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.Config.PodExecRoleName + "-namespaced",
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: namespace,
				Name:      cfg.Config.ServiceAccount,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     cfg.Config.ServiceAccount,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	sa := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.Config.ServiceAccount,
			Namespace: namespace,
		},
	}

	return role, sa, roleBinding
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
