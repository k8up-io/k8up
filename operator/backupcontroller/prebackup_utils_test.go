package backupcontroller

import (
	"context"
	"testing"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBackupExecutor_generateDeployments_keepsTemplateMetadata(t *testing.T) {
	currentConfig := cfg.Config
	t.Cleanup(func() { cfg.Config = currentConfig })
	cfg.Config = &cfg.Configuration{
		BackupCommandAnnotation: "k8up.io/backupcommand",
		FileExtensionAnnotation: "k8up.io/file-extension",
	}

	scheme := runtime.NewScheme()
	require.NoError(t, k8upv1.AddToScheme(scheme))

	backup := &k8upv1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "ns", UID: types.UID("backup-uid")}}
	b := NewBackupExecutor(job.NewConfig(fake.NewClientBuilder().WithScheme(scheme).Build(), backup, ""))

	template := k8upv1.PreBackupPod{
		ObjectMeta: metav1.ObjectMeta{Name: "vault-backup", Namespace: "ns"},
		Spec: k8upv1.PreBackupPodSpec{
			BackupCommand: "vault operator raft snapshot save -",
			FileExtension: ".snap",
			Pod: &k8upv1.Pod{PodTemplateSpec: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"vault.hashicorp.com/agent-inject": "true",
						// A clash with K8up's own annotation: K8up's value must win.
						"k8up.io/backupcommand": "ignored",
					},
					Labels: map[string]string{
						"app": "vault-backup",
						// Same for the labels the deployment selector relies on.
						"k8up.io/preBackupPod": "ignored",
					},
				},
			}},
		},
	}

	deployments := b.generateDeployments(context.Background(), []k8upv1.PreBackupPod{template})
	require.Len(t, deployments, 1)
	podMeta := deployments[0].Spec.Template.ObjectMeta

	assert.Equal(t, map[string]string{
		"vault.hashicorp.com/agent-inject": "true",
		"k8up.io/backupcommand":            "vault operator raft snapshot save -",
		"k8up.io/file-extension":           ".snap",
	}, podMeta.Annotations)
	assert.Equal(t, map[string]string{
		"app":                      "vault-backup",
		"k8up.io/backupCommandPod": "true",
		"k8up.io/preBackupPod":     "vault-backup",
		"k8up.io/ownerBackupUID":   "backup-uid",
	}, podMeta.Labels)

	// The selector must still match the pod labels.
	for k, v := range deployments[0].Spec.Selector.MatchLabels {
		assert.Equal(t, v, podMeta.Labels[k], k)
	}
}
