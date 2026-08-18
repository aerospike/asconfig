#!/usr/bin/env bats
#
# Post-install smoke tests, run against an INSTALLED asconfig.
#
# The version test compares what the installed binary reports -- both the
# Version and the Build line -- against EXPECTED_VERSION when set, otherwise
# the repo's VERSION file. CI always sets it, to the workflow's BUILD_VERSION,
# which is the string the binary was stamped with; a bare local run takes the
# file. When both are present their MAJOR.MINOR.PATCH cores must agree, so a
# step wired to the wrong workflow output fails instead of testing against
# whatever it was handed.
#
# A package labelled with one version but carrying a binary stamped with
# another (a build step that re-links without VERSION and falls back to
# `git describe`, or a stale rc binary in a later rc's package) fails here, in
# this repo's own CI, instead of downstream in the aerospike-tools bundle.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../../.." && pwd)"
  VERSION_FILE="$REPO_ROOT/VERSION"
  load "$BATS_TEST_DIRNAME/version_lib.sh"
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
  expected="$(expected_version "$VERSION_FILE")"

  run asconfig --version
  [ "$status" -eq 0 ]
  echo "expected (from $expected):"
  expected_version_lines "$expected"
  echo "reported:"
  echo "$output"
  assert_version_output "$output" "$expected"
}
