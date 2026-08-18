#!/usr/bin/env bash
# Same post-install checks as test_execute.bats, for hosts without bats.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
# shellcheck source=.github/bin/test/version_lib.sh
. "$SCRIPT_DIR/version_lib.sh"

asconfig --help
asconfig --version

expected="$(expected_version "$REPO_ROOT/VERSION")"
out="$(asconfig --version 2>&1)"
assert_version_output "$out" "$expected"
# Assert first, then report: a command substitution's failure is discarded by
# `set -e`, so expected_version_lines must not be the thing that validates.
lines="$(expected_version_lines "$expected")"
echo "asconfig reports $(printf '%s' "$lines" | tr '\n' ' ') (from $expected)"
