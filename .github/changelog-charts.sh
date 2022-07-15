#!/bin/bash

set -eo pipefail

chart="${1}"

tagPattern="${chart}-(.+)"
chartLabel="chart:${chart}"

echo ::group::Configuring changelog generator
jq '.tag_resolver.filter.pattern="'$tagPattern'" | .tag_resolver.transformer.pattern="'$tagPattern'" | .categories[].labels += ["'$chartLabel'"]' \
  .github/changelog-charts.json | tee .github/configuration.json
echo ::endgroup::
