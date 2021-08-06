module github.com/vshn/k8up

go 1.16

require (
	// When updating crd-ref-docs, verify that there were no changes from Elastic to hostile licenses.
	github.com/elastic/crd-ref-docs v0.0.7
	github.com/firepear/qsplit/v2 v2.5.0
	github.com/go-ini/ini v1.62.0 // indirect
	github.com/go-logr/glogr v0.1.0
	github.com/go-logr/logr v0.4.0
	github.com/go-logr/zapr v0.4.0
	github.com/imdario/mergo v0.3.12
	github.com/knadh/koanf v1.2.1
	github.com/minio/minio-go v6.0.14+incompatible
	github.com/prometheus/client_golang v1.11.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/stretchr/testify v1.7.0
	github.com/urfave/cli/v2 v2.3.0
	go.uber.org/zap v1.18.1
	k8s.io/api v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	k8s.io/utils v0.0.0-20210722164352-7f3ee0f31471
	sigs.k8s.io/controller-runtime v0.9.5
	sigs.k8s.io/controller-runtime/tools/setup-envtest v0.0.0-20210802150722-c0a5babc6854
	sigs.k8s.io/controller-tools v0.5.0
	sigs.k8s.io/kind v0.11.1
	sigs.k8s.io/kustomize/kustomize/v3 v3.8.7
)
