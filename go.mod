module github.com/vshn/k8up

go 1.15

require (
	github.com/go-logr/logr v0.1.0
	github.com/imdario/mergo v0.3.9
	github.com/knadh/koanf v0.14.0
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.3
	github.com/prometheus/client_golang v1.8.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/stretchr/testify v1.6.1
	k8s.io/api v0.18.10
	k8s.io/apimachinery v0.18.10
	k8s.io/client-go v0.18.10
	k8s.io/utils v0.0.0-20200603063816-c1c6865ac451
	sigs.k8s.io/controller-runtime v0.6.4
	sigs.k8s.io/controller-tools v0.4.1
	sigs.k8s.io/kustomize/kustomize/v3 v3.8.7
)
