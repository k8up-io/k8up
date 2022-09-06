include Makefile.restic-integration.vars.mk

clean_targets += restic-integration-test-clean

.PHONY: restic-integration-test-setup
restic-integration-test-setup: minio-start restic-download ## Prepare to run the integration test for the restic module

.PHONY: restic-clean
restic-integration-test-clean: minio-stop ## Clean the integration test of the restic module

.PHONY: minio-address
minio-address: ## Get the address to connect to minio
	@echo "http://$(minio_address)"

.PHONY: minio-reset
minio-reset: minio-stop minio-delete-config minio-start ## Reset minio's configuration and data dirs and restart minio

minio-delete-config:
	rm -rf "$(minio_config)" "$(minio_data)"

.PHONY: minio-restart
minio-restart: minio-stop minio-start ## Restart minio

minio-set-alias: minio-start ## Set the alias 'restic' in mc to the minio server
	@mc alias set restic "http://$(minio_address)" "$(minio_root_user)" "$(minio_root_password)"

.PHONY: minio-start
minio-start: minio-check $(minio_pid) ## Run minio

.PHONY: minio-check
minio-check: minio-clean ## Check if minio is running
	@test -f "$(minio_pid)" && echo "Minio runs as PID $$(cat $(minio_pid))." || echo "Minio is not running."

.PHONY: minio-clean
minio-clean: ## Remove the minio PID file if the process is not running
	@./clean.sh "$(minio_pid)"

.PHONY: minio-stop
minio-stop: ## Stop minio
	@./kill.sh "$(minio_pid)"

minio-download: $(minio_path) ## Download github.com/minio/minio

restic-download: $(restic_path) ## Download github.com/restic/restic

$(minio_pid): export MINIO_ROOT_USER = $(minio_root_user)
$(minio_pid): export MINIO_ROOT_PASSWORD = $(minio_root_password)
$(minio_pid): minio-download
	@mkdir -p "$(minio_data)" "$(minio_config)"
	@./exec.sh "$(minio_pid)" \
		"$(minio_path)" \
			server "$(minio_data)" \
			"--address" "$(minio_address)" \
			"--config-dir" "$(minio_config)"
	@while ! curl --silent "http://$(minio_address)" > /dev/null; do echo "Waiting for server http://$(minio_address) to become ready"; sleep 0.5; done

$(minio_path): | $(go_bin)
	curl $(curl_args) --output "$@" "$(minio_url)"
	chmod +x "$@"
	"$@" --version

$(restic_path): | $(go_bin)
	curl $(curl_args) "$(restic_url)" | \
		bunzip2 > "$@"
	chmod +x "$@"
	"$@" version
