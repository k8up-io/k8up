module github.com/vshn/k8up

go 1.15

require (
	github.com/go-logr/logr v0.3.0
	github.com/imdario/mergo v0.3.11
	github.com/knadh/koanf v0.14.0
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.4
	github.com/operator-framework/operator-lib v0.1.0
	github.com/prometheus/client_golang v1.9.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/stretchr/testify v1.6.1
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v0.19.2
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
	sigs.k8s.io/controller-runtime v0.7.0
	sigs.k8s.io/controller-tools v0.4.1
	sigs.k8s.io/kustomize/kustomize/v3 v3.8.7
)
