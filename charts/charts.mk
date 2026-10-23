# This executes `make clean prepare` for every dir found in charts/ that has a Makefile.
.PHONY: chart-prepare
chart-prepare: release-prepare ## Prepare the Helm charts
	@find charts -type f -name Makefile | sed 's|/[^/]*$$||' | xargs -I '%' make -C '%' clean prepare

.PHONY: chart-docs
chart-docs: ## Creates the Chart READMEs from template and values.yaml files
	@go tool -modfile charts/go.mod github.com/norwoodj/helm-docs/cmd/helm-docs \
		--template-files ./.github/helm-docs-header.gotmpl.md \
		--template-files README.gotmpl.md \
		--template-files ./.github/helm-docs-footer.gotmpl.md

.PHONY: chart-lint
chart-lint: chart-prepare chart-docs ## Lint charts
	@echo 'Check for uncommitted changes ...'
	git diff --exit-code

.PHONY: chart-test
chart-test:  ## Run unit tests for charts
	cd charts && go test ./...
