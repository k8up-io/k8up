package test

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
)

var (
	tplClusterRoleBinding = []string{"templates/clusterrolebinding.yaml"}
)

func Test_ClusterRoleBinding_ShouldRender_SubjectNamespaceFromRelease(t *testing.T) {
	options := withReleaseNamespace(&helm.Options{})

	crb := renderClusterRoleBinding(t, options, false)

	require.Len(t, crb.Subjects, 1)
	assert.Equal(t, releaseNamespace, crb.Subjects[0].Namespace, "ClusterRoleBinding subject should use the release namespace by default")
}

func Test_ClusterRoleBinding_ShouldRender_SubjectNamespaceOverride(t *testing.T) {
	options := withReleaseNamespace(&helm.Options{
		SetValues: map[string]string{
			"namespaceOverride": overrideNamespace,
		},
	})

	crb := renderClusterRoleBinding(t, options, false)

	require.Len(t, crb.Subjects, 1)
	assert.Equal(t, overrideNamespace, crb.Subjects[0].Namespace, "ClusterRoleBinding subject should use the overridden namespace")
}

func renderClusterRoleBinding(t *testing.T, options *helm.Options, wantErr bool) *rbacv1.ClusterRoleBinding {
	output, err := helm.RenderTemplateE(t, options, helmChartPath, releaseName, tplClusterRoleBinding)
	if wantErr {
		require.Error(t, err)
		return nil
	}
	require.NoError(t, err)
	crb := rbacv1.ClusterRoleBinding{}
	helm.UnmarshalK8SYaml(t, output, &crb)
	return &crb
}
