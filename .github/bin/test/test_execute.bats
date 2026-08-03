#!/usr/bin/env bats
#
# Post-install smoke tests, run against an INSTALLED asconfig.
#
# The version test compares what the installed binary reports against the
# repo's VERSION file. A package labelled with one version but carrying a
# binary stamped with another (e.g. a build step that re-links it without
# VERSION and falls back to `git describe`) fails here, in this repo's own
# CI, instead of downstream in the aerospike-tools bundle.
#
# Set EXPECTED_VERSION to verify a package built from a revision other than
# the checkout the tests run from.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../../.." && pwd)"
  VERSION_FILE="$REPO_ROOT/VERSION"
}

@test "can run asconfig" {
  run asconfig --help
  [ "$status" -eq 0 ]
}

@test "asconfig reports version" {
  run asconfig --version
  [ "$status" -eq 0 ]
}

@test "asconfig reports the version from the VERSION file" {
  local expected
  if [ -n "${EXPECTED_VERSION:-}" ]; then
    expected="$EXPECTED_VERSION"
  else
    if [ ! -f "$VERSION_FILE" ]; then
      echo "no VERSION file at $VERSION_FILE; set EXPECTED_VERSION to run this test"
      return 1
    fi
    expected="$(tr -d '[:space:]' < "$VERSION_FILE")"
  fi

  # asconfig prints pre-release and build metadata on a separate "Build" line,
  # so compare the MAJOR.MINOR.PATCH core: 0.21.3-rc2 prints "Version 0.21.3".
  local base
  base="$(printf '%s' "$expected" | grep -oE '^[0-9]+\.[0-9]+\.[0-9]+')"
  if [ -z "$base" ]; then
    echo "cannot parse a MAJOR.MINOR.PATCH core out of '$expected'"
    return 1
  fi

  run asconfig --version
  [ "$status" -eq 0 ]
  echo "expected: Version $base (from $expected)"
  echo "reported:"
  echo "$output"
  printf '%s\n' "$output" | grep -qxF "Version $base"
}
