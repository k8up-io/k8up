tools_dir ?= $(e2etest_dir)

###
### Assets
###

KIND ?= $(tools_dir)/kind

$(KIND): export GOOS = $(shell go env GOOS)
$(KIND): export GOARCH = $(shell go env GOARCH)
$(KIND):
	@mkdir -p $(tools_dir)
	cd $(PROJECT_ROOT_DIR)/tools && go build -o $@ sigs.k8s.io/kind
