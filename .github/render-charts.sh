#!/bin/bash

set -eo pipefail

chartYaml="${1}"
chartName=$(dirname "${chartYaml}")

echo "::group::Render chart ${chartName}"
helm template "${chartName}"
echo "::endgroup::"
