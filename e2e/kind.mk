kind_marker := $(TESTBIN_DIR)/.kind-setup_complete

curl_args ?= --location --fail --silent --show-error

.DEFAULT_TARGET: kind-setup

.PHONY: kind-setup
kind-setup: export KUBECONFIG = $(KIND_KUBECONFIG)
kind-setup: $(kind_marker) ## Creates the kind cluster

.PHONY: kind-clean
kind-clean: export KUBECONFIG = $(KIND_KUBECONFIG)
kind-clean: ## Remove the kind Cluster
	@$(KIND) delete cluster --name $(KIND_CLUSTER) || true
	@rm $(KIND) $(kind_marker) $(KIND_KUBECONFIG) || true

###
### Artifacts
###

$(KIND): export KUBECONFIG = $(KIND_KUBECONFIG)
$(KIND): $(testbin_created)
	curl $(curl_args) --output "$(KIND)" "https://kind.sigs.k8s.io/dl/v$(KIND_VERSION)/kind-$$(uname)-amd64"
	@chmod +x $(KIND)

$(KIND_KUBECONFIG): export KUBECONFIG = $(KIND_KUBECONFIG)
$(KIND_KUBECONFIG): $(KIND)
	$(KIND) create cluster --name $(KIND_CLUSTER) --image kindest/node:$(KIND_NODE_VERSION)
	@kubectl version
	@kubectl cluster-info

$(kind_marker): export KUBECONFIG = $(KIND_KUBECONFIG)
$(kind_marker): $(KIND_KUBECONFIG)
	@kubectl config use-context kind-$(KIND_CLUSTER)
	@touch $(kind_marker)

$(testbin_created):
	mkdir -p $(TESTBIN_DIR)
	# a marker file must be created, because the date of the
	# directory may update when content in it is created/updated,
	# which would cause a rebuild / re-initialization of dependants
	@touch $(testbin_created)
