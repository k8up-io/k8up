#!/bin/bash

set -exuo pipefail

cpu_arch() {
  case $(uname -m) in
      i386)    echo "386" ;;
      i686)    echo "386" ;;
      x86_64)  echo "amd64" ;;
      arm)     echo "arm" ;;
      armv7l)  echo "arm" ;;
      aarch64) echo "arm64" ;;
      *)       exit 1 ;;
  esac
}

os() {
  case $(uname -s) in
      Darwin)   echo "darwin" ;;
      Linux)    echo "linux"  ;;
      *)        exit 1 ;;
  esac
}

restic_version() {
  grep -e 'restic/restic' < go.mod \
  | grep -oe '[0-9]*\.[0-9]*\.[0-9]*'
}

fetch_restic() {
  local RESTIC_DEST="${1}"
  local RESTIC_VERSION="${2-$(restic_version)}"
  local RESTIC_OS="${3-$(os)}"
  local RESTIC_ARCH="${4-$(cpu_arch)}"

  curl \
      --silent \
      --location \
      "https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/restic_${RESTIC_VERSION}_${RESTIC_OS}_${RESTIC_ARCH}.bz2" \
    | bzip2 -d \
    > "${RESTIC_DEST}"
  chmod a+x "${RESTIC_DEST}"
}

fetch_restic "${@}"
