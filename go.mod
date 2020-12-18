module github.com/vshn/k8up

go 1.15

require (
	github.com/go-logr/logr v0.2.0
	github.com/imdario/mergo v0.3.11
	github.com/knadh/koanf v0.14.0
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.4
	github.com/prometheus/client_golang v1.9.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/stretchr/testify v1.6.1
	k8s.io/api v0.18.10
	k8s.io/apimachinery v0.20.1
	k8s.io/client-go v0.18.10
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
	sigs.k8s.io/controller-runtime v0.6.4
	sigs.k8s.io/controller-tools v0.4.1
	sigs.k8s.io/kustomize/kustomize/v3 v3.8.7
)
