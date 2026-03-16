package test

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

var (
	tplGrafanaDashboard = []string{"templates/grafana-dashboard.yaml"}
)

func Test_GrafanaDashboard_ShouldRender_ReleaseNamespaceByDefault(t *testing.T) {
	options := withReleaseNamespace(&helm.Options{
		SetValues: map[string]string{
			"metrics.grafanaDashboard.enabled": "true",
		},
	})

	cm := renderGrafanaDashboard(t, options, false)

	assert.Equal(t, releaseNamespace, cm.Namespace, "Grafana dashboard should use the release namespace by default")
}

func Test_GrafanaDashboard_ShouldRender_OverrideNamespace(t *testing.T) {
	options := withReleaseNamespace(&helm.Options{
		SetValues: map[string]string{
			"metrics.grafanaDashboard.enabled": "true",
			"namespaceOverride":                overrideNamespace,
		},
	})

	cm := renderGrafanaDashboard(t, options, false)

	assert.Equal(t, overrideNamespace, cm.Namespace, "Grafana dashboard should use the overridden namespace")
}

func Test_GrafanaDashboard_GivenCustomNamespace_ThenRenderCustomNamespace(t *testing.T) {
	customNamespace := "monitoring"
	options := withReleaseNamespace(&helm.Options{
		SetValues: map[string]string{
			"metrics.grafanaDashboard.enabled":   "true",
			"metrics.grafanaDashboard.namespace": customNamespace,
		},
	})

	cm := renderGrafanaDashboard(t, options, false)

	assert.Equal(t, customNamespace, cm.Namespace, "Grafana dashboard should use the custom namespace when specified")
}

func Test_GrafanaDashboard_GivenCustomNamespace_WhenNamespaceOverrideDefined_ThenRenderCustomNamespace(t *testing.T) {
	customNamespace := "monitoring"
	options := withReleaseNamespace(&helm.Options{
		SetValues: map[string]string{
			"metrics.grafanaDashboard.enabled":   "true",
			"metrics.grafanaDashboard.namespace": customNamespace,
			"namespaceOverride":                  overrideNamespace,
		},
	})

	cm := renderGrafanaDashboard(t, options, false)

	assert.Equal(t, customNamespace, cm.Namespace, "Grafana dashboard custom namespace should take precedence over namespaceOverride")
}

func renderGrafanaDashboard(t *testing.T, options *helm.Options, wantErr bool) *corev1.ConfigMap {
	output, err := helm.RenderTemplateE(t, options, helmChartPath, releaseName, tplGrafanaDashboard)
	if wantErr {
		require.Error(t, err)
		return nil
	}
	require.NoError(t, err)
	cm := corev1.ConfigMap{}
	helm.UnmarshalK8SYaml(t, output, &cm)
	return &cm
}
