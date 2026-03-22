#!/usr/bin/env bats

load "lib/utils"
load "lib/detik"

# shellcheck disable=SC2034
DETIK_CLIENT_NAME="kubectl"
# shellcheck disable=SC2034
DEBUG_DETIK="true"

docs_dir="../docs/modules/ROOT"
extracted_dir="/tmp/k8up-doc-yamls"

setup_file() {
    # Extract inline YAML blocks from docs
    bash ../e2e/extract-doc-yamls.sh "$docs_dir/pages" "$extracted_dir"
}

@test "Standalone example YAML files are valid" {
    local failures=0
    local details=""

    for f in "$docs_dir"/examples/*.yaml "$docs_dir"/examples/tutorial/*.yaml; do
        [ -f "$f" ] || continue
        if ! output=$(kubectl apply --dry-run=server -f "$f" 2>&1); then
            failures=$((failures + 1))
            details="$details\nFAIL: $f\n$output\n"
        fi
    done

    if [ "$failures" -gt 0 ]; then
        echo -e "$details"
        fail "$failures standalone example YAML file(s) failed validation"
    fi
}

@test "Inline YAML examples from docs are valid" {
    local failures=0
    local details=""

    for f in "$extracted_dir"/inline-*.yaml; do
        [ -f "$f" ] || continue
        if ! output=$(kubectl apply --dry-run=server -f "$f" 2>&1); then
            failures=$((failures + 1))
            details="$details\nFAIL: $(basename "$f")\n$output\n"
        fi
    done

    if [ "$failures" -gt 0 ]; then
        echo -e "$details"
        fail "$failures inline YAML example(s) from docs failed validation"
    fi
}
