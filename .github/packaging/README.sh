#!/usr/bin/env bash
# This repo is intended to be invoked on Linux with git and docker installed
# Your working directory should be the root of the git repository

# To build the packaging container, use
.github/packaging/common/entrypoint.sh -c -d el9

# To execute the build, use
.github/packaging/common/entrypoint.sh -e -d el9

# This will produce packages in ../dist relative to your current working directory
# $ ls ../dist/el9
# aerospike-asconfig-0.19.0-173-gde57889.el9.aarch64.rpm