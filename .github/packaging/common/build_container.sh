#!/usr/bin/env bash
set -xeuo pipefail

function build_container() {
  docker build \
    --build-arg=BASE_IMAGE=${distro_to_image["$1"]} \
    --build-arg=ENV_DISTRO="$1" \
    --build-arg=REPO_NAME="$REPO_NAME" \
    -t "$REPO_NAME-pkg-builder-$1":"$PKG_VERSION" \
    -f .github/packaging/common/Dockerfile .
}

function execute_build_image() {
  export BUILD_DISTRO="$1"
  docker run \
    -e BUILD_DISTRO \
    -v "$(realpath ../dist)":/tmp/output \
    "$REPO_NAME-pkg-builder-$BUILD_DISTRO":"$PKG_VERSION"
  ls -laht ../dist
}