package test

import (
	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
)

var (
	helmChartPath     = ".."
	releaseName       = "test-release"
	releaseNamespace  = "test-namespace"
	chartName         = "k8up"
	overrideNamespace = "operations"
)

func withReleaseNamespace(options *helm.Options) *helm.Options {
	if options == nil {
		options = &helm.Options{}
	}
	options.KubectlOptions = &k8s.KubectlOptions{Namespace: releaseNamespace}
	return options
}
