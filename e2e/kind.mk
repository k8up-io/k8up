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
	# Applies local-path-config.yaml to kind cluster and forces restart of provisioner - can be simplified once https://github.com/kubernetes-sigs/kind/pull/3090 is merged.
	# This is necessary due to the multi node cluster. Classic k8s hostPath provisioner doesn't permit multi node and sharedFileSystemPath support is only in local-path-provisioner v0.0.23.
	@kubectl apply -n local-path-storage -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.23/deploy/local-path-storage.yaml
	@kubectl get cm -n local-path-storage local-path-config -o yaml|yq $(yq --help | grep -q eval && echo e) '.data."config.json"="{\"nodePathMap\":[],\"sharedFileSystemPath\": \"/tmp/e2e/local-path-provisioner\"}"'|kubectl apply -f -
	@kubectl delete po -n local-path-storage --all

$(KIND): export GOBIN = $(go_bin)
$(KIND): | $(go_bin)
	go install sigs.k8s.io/kind@latest
