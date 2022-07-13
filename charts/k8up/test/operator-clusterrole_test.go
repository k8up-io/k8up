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
	tplRbac = []string{"templates/operator-clusterrole.yaml"}
)

func Test_RBAC_GivenDefaultSetting_WhenRenderTemplate_ThenRenderRbacWithReplacedValues(t *testing.T) {
	options := &helm.Options{
		//Logger: logger.Discard,
	}

	output := helm.RenderTemplate(t, options, helmChartPath, releaseName, tplRbac)

	docs := strings.Split(output, "\n---\n")
	assert.Len(t, docs, 1, "resources in file")

	for _, doc := range docs {
		obj := unstructured.Unstructured{}
		helm.UnmarshalK8SYaml(t, doc, &obj.Object)
		labels := obj.GetLabels()
		assert.Contains(t, labels, "app.kubernetes.io/name")
		assert.Contains(t, labels, "app.kubernetes.io/instance")
		assert.Contains(t, labels, "app.kubernetes.io/managed-by")
		assert.NotContains(t, labels, "app.kubernetes.io/version")

		name := obj.GetName()
		assert.Contains(t, name, strings.Join([]string{releaseName, chartName}, "-"))

	}
}

func Test_RBAC_GivenRbacDisabled_WhenRenderTemplate_ThenDontRenderRbacRules(t *testing.T) {
	options := &helm.Options{
		SetValues: map[string]string{
			"rbac.create": "false",
		},
	}

	_, err := helm.RenderTemplateE(t, options, helmChartPath, releaseName, tplRbac)
	require.Error(t, err)
}
