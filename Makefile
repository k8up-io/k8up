CODE_GENERATOR_IMAGE := slok/kube-code-generator:v1.11.2
DIRECTORY := $(PWD)
CODE_GENERATOR_PACKAGE := github.com/vshn/k8up

generate:
	docker run --rm -it \
	-v $(DIRECTORY):/go/src/$(CODE_GENERATOR_PACKAGE) \
	-e PROJECT_PACKAGE=$(CODE_GENERATOR_PACKAGE) \
	-e CLIENT_GENERATOR_OUT=$(CODE_GENERATOR_PACKAGE)/client/k8s \
	-e APIS_ROOT=$(CODE_GENERATOR_PACKAGE)/apis \
	-e GROUPS_VERSION="backup:v1alpha1" \
	-e GENERATION_TARGETS="deepcopy,client" \
	$(CODE_GENERATOR_IMAGE)
