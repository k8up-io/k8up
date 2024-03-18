setup_envtest_bin = $(go_bin)/setup-envtest
envtest_crd_dir ?= $(WORK_DIR)/crds

clean_targets += .envtest-clean

# Prepare binary
$(setup_envtest_bin): export GOBIN = $(go_bin)
$(setup_envtest_bin): | $(go_bin)
	go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: integration-test
# operator module {
integration-test: export KUBEBUILDER_ATTACH_CONTROL_PLANE_OUTPUT = $(INTEGRATION_TEST_DEBUG_OUTPUT)
integration-test: export ENVTEST_CRD_DIR = $(envtest_crd_dir)
# }
# restic module {
integration-test: export RESTIC_BINARY = $(restic_path)
integration-test: export RESTIC_PASSWORD = $(restic_password)
integration-test: export RESTIC_REPOSITORY = s3:http://$(minio_address)/test
integration-test: export AWS_ACCESS_KEY_ID = $(minio_root_user)
integration-test: export AWS_SECRET_ACCESS_KEY = $(minio_root_password)
integration-test: export RESTORE_S3ENDPOINT = http://$(minio_address)/restore
integration-test: export RESTORE_ACCESSKEYID = $(minio_root_user)
integration-test: export RESTORE_SECRETACCESSKEY = $(minio_root_password)
integration-test: export BACKUP_DIR = $(backup_dir)
integration-test: export RESTORE_DIR = $(restore_dir)
integration-test: export STATS_URL = $(stats_url)
# }
integration-test: $(setup_envtest_bin) generate restic-integration-test-setup .envtest_crds ## Run integration tests with envtest
	$(setup_envtest_bin) $(ENVTEST_ADDITIONAL_FLAGS) use '$(ENVTEST_K8S_VERSION)!'
	@chmod -R +w $(go_bin)/k8s
	export KUBEBUILDER_ASSETS="$$($(setup_envtest_bin) $(ENVTEST_ADDITIONAL_FLAGS) use -i -p path '$(ENVTEST_K8S_VERSION)!')" && \
	go test -tags=integration -coverprofile cover.out -covermode atomic ./...

$(envtest_crd_dir):
	@mkdir -p $@

.envtest_crds: | $(envtest_crd_dir)
	@cp -r config/crd/apiextensions.k8s.io/v1/* $(envtest_crd_dir)/

.PHONY: .envtest-clean
.envtest-clean:
# setup-envtest removes write permission from the files it generates, so they have to be restored in order to delete the directory
	chmod +rwx -R -f $(integrationtest_dir) || true
	rm -rf $(setup_envtest_bin) $(envtest_crd_dir) $(integrationtest_dir)
