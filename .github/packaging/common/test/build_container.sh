#!/usr/bin/env bash
set -xeuo pipefail

function build_container() {
  docker build \
    --build-arg=BASE_IMAGE=${distro_to_image["$1"]} \
    --build-arg=ENV_DISTRO=$1 \
    --build-arg=PKG_VERSION="$PKG_VERSION" \
    --build-arg=JF_USERNAME="$JF_USERNAME" \
    --build-arg=JF_TOKEN="$JF_TOKEN" \
    --build-arg=PACKAGE_NAME=$PACKAGE_NAME \
    --build-arg=REPO_NAME=$REPO_NAME \
    -t $REPO_NAME-pkg-tester-"$1":"$PKG_VERSION" \
    -f .github/packaging/common/test/Dockerfile .

  docker tag $REPO_NAME-pkg-tester-"$1":"$PKG_VERSION" $REPO_NAME-pkg-tester-"$1":"latest"
}


function execute_build_image() {
  export BUILD_DISTRO="$1"
  docker run \
    -e BUILD_DISTRO \
    -v $(realpath ../dist):/tmp/output \
    "$REPO_NAME-pkg-tester-$BUILD_DISTRO":"$PKG_VERSION"
}