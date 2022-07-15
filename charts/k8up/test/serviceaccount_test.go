package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/gruntwork-io/terratest/modules/helm"
)

var (
	tplServiceAccount = []string{"templates/serviceaccount.yaml"}
)

func Test_ServiceAccount_ShouldNotRender_IfDisabled(t *testing.T) {
	options := &helm.Options{
		SetValues: map[string]string{
			"serviceAccount.create": "false",
		},
	}

	renderServiceAccount(t, options, true)

}

func Test_ServiceAccount_ShouldRender_ByDefault(t *testing.T) {
	want := releaseName + "-k8up"
	options := &helm.Options{}

	sa := renderServiceAccount(t, options, false)
	assert.Equal(t, want, sa.Name, "ServiceAccount does use configured name")
}

func Test_ServiceAccount_ShouldRender_CustomName(t *testing.T) {
	want := "test"
	options := &helm.Options{
		SetValues: map[string]string{
			"serviceAccount.name": want,
		},
	}

	sa := renderServiceAccount(t, options, false)

	assert.Equal(t, want, sa.Name, "ServiceAccount does use configured name")
}

func renderServiceAccount(t *testing.T, options *helm.Options, wantErr bool) *corev1.ServiceAccount {
	output, err := helm.RenderTemplateE(t, options, helmChartPath, releaseName, tplServiceAccount)
	if wantErr {
		require.Error(t, err)
		return nil
	}
	require.NoError(t, err)
	sa := corev1.ServiceAccount{}
	helm.UnmarshalK8SYaml(t, output, &sa)
	return &sa
}
