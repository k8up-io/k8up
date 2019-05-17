#!/usr/bin/env bash

push () {
  echo "pushing image to registry"
  docker push ${IMAGE_NAME}:${IMAGE_TAG}
}

docs() {
  echo "generating docs and pushing it"
  docker run \
    -v ${PWD}:/docs \
    -e GITHUB_TOKEN=${GITHUB_TOKEN} \
    squidfunk/mkdocs-material \
      gh-deploy \
      --clean \
      --force \
      --remote-name https://${GITHUB_TOKEN}@github.com/vshn/k8up.git
}

if [ "$1" == "docs" ]; then
  docs
fi

if [ "$1" == "push" ]; then
  push
fi

if [ "$1" == "docspush" ]; then
  push
  docs
fi
