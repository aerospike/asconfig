#!/usr/bin/env bash
# Unit tests for version_lib.sh -- the assertions the post-install smoke tests
# are built from. They run against strings, not an installed binary, so they need
# no package, no bats and no Go: `bash .github/bin/test/version_lib_test.sh`.
#
# The smoke tests themselves only run in build-and-release.yml, which is
# workflow_dispatch-only. Without this file a regression in the assertions --
# one that lets a mis-stamped binary through -- would first be observed during a
# release. These cases are the ones that regression would break.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=.github/bin/test/version_lib.sh
. "$SCRIPT_DIR/version_lib.sh"

failures=0

# What `asconfig --version` prints for a given embedded string, per
# tools-common-go's SetupRoot: split on "-", the core on a Version line and the
# last segment, when there is one, on a Build line.
report() {
	printf 'Aerospike Config\nVersion %s\n' "${1%%-*}"
	[ "$1" = "${1%%-*}" ] || printf 'Build %s\n' "${1##*-}"
}

check() {
	local want="$1" desc="$2" got=0
	shift 2
	"$@" >/dev/null 2>&1 || got=1
	if [ "$got" != "$want" ]; then
		echo "FAIL: $desc (wanted exit $want, got $got)" >&2
		failures=$((failures + 1))
	else
		echo "ok: $desc"
	fi
}

# --- expected_version_lines: the shape of the expected output ---
[ "$(expected_version_lines 0.21.4-rc1)" = "$(printf 'Version 0.21.4\nBuild rc1')" ] ||
	{ echo "FAIL: rc version yields both lines" >&2; failures=$((failures + 1)); }
[ "$(expected_version_lines 0.21.4)" = "Version 0.21.4" ] ||
	{ echo "FAIL: bare version yields no Build line" >&2; failures=$((failures + 1)); }
[ "$(expected_version_lines 0.21.4-abc123def)" = "$(printf 'Version 0.21.4\nBuild abc123def')" ] ||
	{ echo "FAIL: dev version builds from the sha" >&2; failures=$((failures + 1)); }
check 1 "unparseable core is rejected" expected_version_lines garbage
check 1 "a v prefix is rejected" expected_version_lines v0.21.4-rc1

# --- assert_version_output: what the binary actually printed ---
check 0 "matching rc stamp passes" assert_version_output "$(report 0.21.4-rc1)" 0.21.4-rc1
check 0 "matching bare stamp passes" assert_version_output "$(report 0.21.4)" 0.21.4
check 0 "matching dev stamp passes" assert_version_output "$(report 0.21.4-abc123def)" 0.21.4-abc123def
# The case rcN-as-iteration created: two packages that differ by nothing else.
check 1 "a stale rc1 binary fails an rc2 package" assert_version_output "$(report 0.21.4-rc1)" 0.21.4-rc2
check 1 "a bare binary fails an rc package" assert_version_output "$(report 0.21.4)" 0.21.4-rc1
# How a binary that fell back to `git describe` gives itself away.
check 1 "a git-describe binary fails a bare version" assert_version_output "$(report 0.21.4-15-gabc123def)" 0.21.4
check 1 "wrong core fails" assert_version_output "$(report 0.21.5-rc1)" 0.21.4-rc1
check 1 "a longer core is not a match" assert_version_output "$(report 0.21.40-rc1)" 0.21.4-rc1
check 1 "empty output fails" assert_version_output "" 0.21.4-rc1

# --- expected_version: which string to expect ---
version_file="$(mktemp)"
printf '0.21.4-rc1\n' >"$version_file"

out="$(unset EXPECTED_VERSION; expected_version "$version_file")"
[ "$out" = "0.21.4-rc1" ] ||
	{ echo "FAIL: falls back to the VERSION file (got '$out')" >&2; failures=$((failures + 1)); }
out="$(EXPECTED_VERSION=0.21.4-abc123def expected_version "$version_file")"
[ "$out" = "0.21.4-abc123def" ] ||
	{ echo "FAIL: EXPECTED_VERSION wins when the cores agree (got '$out')" >&2; failures=$((failures + 1)); }
check 1 "a disagreeing EXPECTED_VERSION fails" \
	env EXPECTED_VERSION=0.22.0-rc1 bash -c ". '$SCRIPT_DIR/version_lib.sh'; expected_version '$version_file'"
# An unresolved needs.<job>.outputs.<name> arrives as set-but-empty, and on a
# main release the VERSION file equals BUILD_VERSION -- so falling back here
# would hide the one wiring mistake this check exists to catch.
check 1 "an empty EXPECTED_VERSION fails rather than falling back" \
	env EXPECTED_VERSION= bash -c ". '$SCRIPT_DIR/version_lib.sh'; expected_version '$version_file'"
check 1 "no file and no EXPECTED_VERSION fails" \
	env -u EXPECTED_VERSION bash -c ". '$SCRIPT_DIR/version_lib.sh'; expected_version /nonexistent/VERSION"

rm -f "$version_file"

if [ "$failures" -ne 0 ]; then
	echo "$failures version_lib.sh assertion(s) behaved wrongly" >&2
	exit 1
fi
echo "all version_lib.sh assertions behave as documented"
