tools_dir ?= $(e2etest_dir)

###
### Assets
###

CRD_REF_DOCS_BIN ?= $(tools_dir)/crd-ref-docs

$(CRD_REF_DOCS_BIN): export GOOS = $(shell go env GOOS)
$(CRD_REF_DOCS_BIN): export GOARCH = $(shell go env GOARCH)
$(CRD_REF_DOCS_BIN):
	@mkdir -p $(tools_dir)
	cd $(PROJECT_ROOT_DIR)/tools && go build -o $@ github.com/elastic/crd-ref-docs

KIND ?= $(tools_dir)/kind

$(KIND): export GOOS = $(shell go env GOOS)
$(KIND): export GOARCH = $(shell go env GOARCH)
$(KIND):
	@mkdir -p $(tools_dir)
	cd $(PROJECT_ROOT_DIR)/tools && go build -o $@ sigs.k8s.io/kind
