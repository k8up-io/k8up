#!/bin/bash
# Extract YAML blocks from AsciiDoc files and write them as individual files
# to a target directory for validation.
#
# Usage: extract-doc-yamls.sh <docs-dir> <output-dir>

set -euo pipefail

docs_dir="${1:?Usage: extract-doc-yamls.sh <docs-dir> <output-dir>}"
output_dir="${2:?Usage: extract-doc-yamls.sh <docs-dir> <output-dir>}"

mkdir -p "$output_dir"
rm -f "$output_dir"/inline-*.yaml

block_num=0

for adoc_file in $(find "$docs_dir" -name '*.adoc' | sort); do
    in_yaml=0
    yaml_started=0
    yaml_buf=""
    rel_path="${adoc_file#"$docs_dir"/}"

    while IFS= read -r line; do
        if [[ "$line" == "[source,yaml]" ]]; then
            in_yaml=1
            yaml_started=0
            yaml_buf=""
            continue
        fi

        if (( in_yaml )) && [[ "$line" == "----" ]]; then
            if (( ! yaml_started )); then
                # Opening delimiter
                yaml_started=1
                continue
            fi

            # Closing delimiter — process the block
            in_yaml=0
            yaml_started=0

            # Skip blocks that are include directives
            if echo "$yaml_buf" | grep -q '^include::'; then
                continue
            fi

            # Skip blocks with "Abridged" or placeholder comments suggesting incomplete examples
            if echo "$yaml_buf" | grep -qi 'abridged'; then
                continue
            fi

            # Skip blocks with YAML ellipsis (...) used as abbreviation
            if echo "$yaml_buf" | grep -q '^  \.\.\.$'; then
                continue
            fi

            # Only keep blocks that have apiVersion (complete K8s resources)
            if ! echo "$yaml_buf" | grep -q '^apiVersion:'; then
                continue
            fi

            block_num=$((block_num + 1))
            outfile="$output_dir/inline-$(echo "$rel_path" | tr '/' '-')-${block_num}.yaml"
            echo "$yaml_buf" > "$outfile"
            continue
        fi

        if (( in_yaml && yaml_started )); then
            if [[ -z "$yaml_buf" ]]; then
                yaml_buf="$line"
            else
                yaml_buf="$yaml_buf
$line"
            fi
        fi
    done < "$adoc_file"
done

echo "Extracted $block_num inline YAML blocks to $output_dir"
