curl_args ?= --location --fail --silent --show-error

KIND ?= $(go_bin)/kind

.PHONY: kind-setup
kind-setup: export KUBECONFIG = $(KIND_KUBECONFIG)
kind-setup: $(KIND_KUBECONFIG) | $(e2etest_dir) ## Creates the kind cluster

.PHONY: kind-clean
kind-clean: export KUBECONFIG = $(KIND_KUBECONFIG)
kind-clean: $(KIND) ## Remove the kind Cluster
	@$(KIND) delete cluster --name $(KIND_CLUSTER) || true
	@rm -rf $(KIND) $(kind_marker) $(KIND_KUBECONFIG)

###
### Artifacts
###

$(KIND_KUBECONFIG): export KUBECONFIG = $(KIND_KUBECONFIG)
$(KIND_KUBECONFIG): $(KIND)
	@mkdir -p e2e/debug/data/pvc-subject
	$(KIND) create cluster \
		--name $(KIND_CLUSTER) \
		--image kindest/node:$(KIND_NODE_VERSION) \
		--config e2e/definitions/kind/config.yaml
	@kubectl version
	@kubectl cluster-info
	@kubectl config use-context kind-$(KIND_CLUSTER)

$(KIND): export GOBIN = $(go_bin)
$(KIND): | $(go_bin)
	go install sigs.k8s.io/kind@latest
