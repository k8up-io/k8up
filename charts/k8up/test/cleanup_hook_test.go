package test

import (
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	tplCleanupHook = []string{"templates/cleanup-hook.yaml"}
)

func Test_CleanupHook_ShouldRender_ReleaseNamespaceByDefault(t *testing.T) {
	options := withReleaseNamespace(&helm.Options{})

	docs := renderCleanupHookDocs(t, options)

	sa := parseUnstructured(t, docs[0])
	assert.Equal(t, releaseNamespace, sa.GetNamespace(), "cleanup ServiceAccount should use the release namespace by default")

	crbDoc := parseUnstructured(t, docs[2])
	subjects, _ := getSubjectNamespace(t, crbDoc)
	assert.Equal(t, releaseNamespace, subjects, "cleanup ClusterRoleBinding subject should use the release namespace by default")

	job := parseUnstructured(t, docs[3])
	assert.Equal(t, releaseNamespace, job.GetNamespace(), "cleanup Job should use the release namespace by default")
}

func Test_CleanupHook_ShouldRender_OverrideNamespace(t *testing.T) {
	options := withReleaseNamespace(&helm.Options{
		SetValues: map[string]string{
			"namespaceOverride": overrideNamespace,
		},
	})

	docs := renderCleanupHookDocs(t, options)

	sa := parseUnstructured(t, docs[0])
	assert.Equal(t, overrideNamespace, sa.GetNamespace(), "cleanup ServiceAccount should use the overridden namespace")

	crbDoc := parseUnstructured(t, docs[2])
	subjects, _ := getSubjectNamespace(t, crbDoc)
	assert.Equal(t, overrideNamespace, subjects, "cleanup ClusterRoleBinding subject should use the overridden namespace")

	job := parseUnstructured(t, docs[3])
	assert.Equal(t, overrideNamespace, job.GetNamespace(), "cleanup Job should use the overridden namespace")
}

func renderCleanupHookDocs(t *testing.T, options *helm.Options) []string {
	output, err := helm.RenderTemplateE(t, options, helmChartPath, releaseName, tplCleanupHook)
	require.NoError(t, err)
	docs := strings.Split(output, "---")
	var nonEmpty []string
	for _, doc := range docs {
		trimmed := strings.TrimSpace(doc)
		if trimmed != "" {
			nonEmpty = append(nonEmpty, trimmed)
		}
	}
	return nonEmpty
}

func parseUnstructured(t *testing.T, doc string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	helm.UnmarshalK8SYaml(t, doc, &obj.Object)
	return obj
}

func getSubjectNamespace(t *testing.T, obj *unstructured.Unstructured) (string, bool) {
	subjects, found, err := unstructured.NestedSlice(obj.Object, "subjects")
	require.NoError(t, err)
	require.True(t, found, "subjects field not found")
	require.NotEmpty(t, subjects)
	subject := subjects[0].(map[string]interface{})
	ns, ok := subject["namespace"].(string)
	return ns, ok
}
