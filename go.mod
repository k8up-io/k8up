module github.com/vshn/k8up

go 1.13

require (
	github.com/go-logr/logr v0.1.0
	github.com/google/martian v2.1.0+incompatible
	github.com/joyent/triton-go v1.8.5
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/smartystreets/goconvey v1.6.4
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v0.18.6
	sigs.k8s.io/controller-runtime v0.6.2
	sigs.k8s.io/structured-merge-diff/v3 v3.0.0
)
