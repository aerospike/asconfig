#!/usr/bin/env bash
set -xeuo pipefail
cd local
git fetch --unshallow --tags --no-recurse-submodules
git submodule update --init
ls -laht
echo ref_name ${{ github.ref_name }}
git branch -v
.github/packaging/common/entrypoint.sh -c -d ${{ matrix.distro }}
.github/packaging/common/entrypoint.sh -e -d ${{ matrix.distro }}
ls -laht ../dist