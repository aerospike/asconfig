#!/usr/bin/env bash
set -xeuo pipefail

function assert_dynamic_deps() {
  local allowed="libc.so.6 libm.so.6 libpthread.so.0 libdl.so.2 librt.so.1
    libresolv.so.2 ld-linux-x86-64.so.2 ld-linux-aarch64.so.1"

  local lib fail=0
  local needed
  needed=$(readelf -d "$GIT_DIR/bin/asconfig" | awk '/\(NEEDED\)/ { gsub(/[][]/, "", $NF); print $NF }')
  echo "asconfig DT_NEEDED:" $needed
  for lib in $needed; do
    if ! printf '%s\n' $allowed | grep -qxF "$lib"; then
      echo "asconfig has unexpected dynamic dependency $lib; link it statically or add it to the allowlist and the package depends" >&2
      fail=1
    fi
  done
  return $fail
}

function build_packages(){
  if [ "${ENV_DISTRO:-}" = "" ]; then
    echo "ENV_DISTRO is not set"
    return
  fi
  if [ -d /opt/golang/go ]; then
    export GOROOT=/opt/golang/go
    export PATH="/opt/golang/go/bin:$PATH"
  fi
  GIT_DIR=$(git rev-parse --show-toplevel)

  # Version embedded in the binary must match the package version (CI passes PKG_VERSION).
  # Export before `make` so the root Makefile's VERSION is used for -ldflags.
  VERSION=${PKG_VERSION:-$(git describe --tags --always --abbrev=9)}
  export VERSION
  echo "build_package.sh version: ${VERSION}"

  # build
  cd "$GIT_DIR" || exit 1
  make clean
  make

  assert_dynamic_deps

  # package
  cd $GIT_DIR/pkg || exit 1
  make clean
  echo "building package for $BUILD_DISTRO"

  if [[ "$ENV_DISTRO" == *"ubuntu"* ]]; then
    make deb
  elif [[ "$ENV_DISTRO" == *"debian"* ]]; then
    make deb
  elif [[ "$ENV_DISTRO" == *"el"* ]]; then
    make rpm
  elif [[ "$ENV_DISTRO" == *"amzn"* ]]; then
    make rpm
  else
    make tar
  fi

  mkdir -p /tmp/output/"$ENV_DISTRO"
  cp -a "$GIT_DIR"/pkg/target/* /tmp/output/"$ENV_DISTRO"
}
