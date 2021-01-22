module github.com/vshn/k8up

go 1.15

require (
	github.com/go-logr/logr v0.4.0
	github.com/go-logr/zapr v0.2.0
	github.com/imdario/mergo v0.3.11
	github.com/knadh/koanf v0.15.0
	github.com/prometheus/client_golang v1.9.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/stretchr/testify v1.7.0
	go.uber.org/zap v1.16.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009
	sigs.k8s.io/controller-runtime v0.8.1
	sigs.k8s.io/controller-tools v0.4.1
	sigs.k8s.io/kustomize/kustomize/v3 v3.8.7
)
