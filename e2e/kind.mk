kind_marker := $(e2etest_dir)/.kind-setup_complete

curl_args ?= --location --fail --silent --show-error

.DEFAULT_TARGET: kind-setup

.PHONY: kind-setup
kind-setup: export KUBECONFIG = $(KIND_KUBECONFIG)
kind-setup: $(kind_marker) $(e2etest_dir_created) ## Creates the kind cluster

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
	@mkdir -p debug/data/pvc-subject
	$(KIND) create cluster \
		--name $(KIND_CLUSTER) \
		--image kindest/node:$(KIND_NODE_VERSION) \
		--config definitions/kind/config.yaml
	@kubectl version
	@kubectl cluster-info

$(kind_marker): export KUBECONFIG = $(KIND_KUBECONFIG)
$(kind_marker): $(KIND_KUBECONFIG)
	@kubectl config use-context kind-$(KIND_CLUSTER)
	@touch $(kind_marker)
