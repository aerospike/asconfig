#!/usr/bin/env bash
# Same post-install checks as test_execute.bats, for hosts without bats.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

asconfig --help
asconfig --version

expected="${EXPECTED_VERSION:-$(tr -d '[:space:]' < "$REPO_ROOT/VERSION")}"
base="$(printf '%s' "$expected" | grep -oE '^[0-9]+\.[0-9]+\.[0-9]+')"
[ -n "$base" ] || { echo "cannot parse a MAJOR.MINOR.PATCH core out of '$expected'" >&2; exit 1; }

asconfig --version | grep -qxF "Version $base" || {
  echo "installed asconfig does not report Version ${base} (expected from ${expected})" >&2
  asconfig --version >&2
  exit 1
}
echo "asconfig reports Version ${base}"
