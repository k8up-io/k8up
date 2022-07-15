package test

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/helm"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
)

var (
	tplServiceMonitor = []string{"templates/prometheus/servicemonitor.yaml"}
)

func Test_ServiceMonitor_GivenEnabled_WhenIntervalDefined_ThenRenderNewInterval(t *testing.T) {
	expectedInterval := "1m10s"
	options := &helm.Options{
		SetValues: map[string]string{
			"metrics.serviceMonitor.enabled":        "true",
			"metrics.serviceMonitor.scrapeInterval": expectedInterval,
		},
	}

	output := helm.RenderTemplate(t, options, helmChartPath, releaseName, tplServiceMonitor)
	monitor := monitoringv1.ServiceMonitor{}
	helm.UnmarshalK8SYaml(t, output, &monitor)

	assert.Equal(t, monitoringv1.Duration(expectedInterval), monitor.Spec.Endpoints[0].Interval)
}

func Test_ServiceMonitor_GivenEnabled_WhenNamespaceDefined_ThenRenderNewNamespace(t *testing.T) {
	expectedNamespace := "alternative"
	options := &helm.Options{
		SetValues: map[string]string{
			"metrics.serviceMonitor.enabled":   "true",
			"metrics.serviceMonitor.namespace": expectedNamespace,
		},
	}

	output := helm.RenderTemplate(t, options, helmChartPath, releaseName, tplServiceMonitor)
	monitor := monitoringv1.ServiceMonitor{}
	helm.UnmarshalK8SYaml(t, output, &monitor)

	assert.Equal(t, expectedNamespace, monitor.Namespace)
}

func Test_ServiceMonitor_GivenEnabled_WhenAdditionalLabelsDefined_ThenRenderMoreLabels(t *testing.T) {
	expectedLabelKey := "my-custom-label"
	expectedLabelValue := "my-value"
	options := &helm.Options{
		ValuesFiles: []string{"testdata/labels.yaml"},
	}

	output := helm.RenderTemplate(t, options, helmChartPath, releaseName, tplServiceMonitor)
	monitor := monitoringv1.ServiceMonitor{}
	helm.UnmarshalK8SYaml(t, output, &monitor)

	assert.Equal(t, expectedLabelValue, monitor.Labels[expectedLabelKey])
}
