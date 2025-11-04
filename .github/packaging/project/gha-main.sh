#!/usr/bin/env bash
set -xeuo pipefail
DISTRO="$1"
export JF_USERNAME="$2"
export JF_TOKEN="$3"
env
cd local
git fetch --unshallow --tags --no-recurse-submodules
git submodule update --init
ls -laht
git branch -v
.github/packaging/common/entrypoint.sh -c -d $DISTRO
.github/packaging/common/entrypoint.sh -e -d $DISTRO
ls -laht ../dist
