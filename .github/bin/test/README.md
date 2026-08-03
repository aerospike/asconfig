# Post-install smoke tests

`test_execute.bats` runs in CI (`Test install & execute`) on every Linux distro and macOS
runner, against the signed package after it is installed. It checks that asconfig runs and
that **the version it reports matches the repo's `VERSION` file**, so a package labelled with
one version but carrying a binary stamped with another fails here instead of downstream in
the aerospike-tools bundle.

Only the `MAJOR.MINOR.PATCH` core is compared, because pre-release and branch metadata is
printed on a separate `Build` line: `0.21.3-rc2` reports `Version 0.21.3` / `Build rc2`.

```
bats .github/bin/test/test_execute.bats
```

`test_execute.sh` is the same checks without bats. Both accept `EXPECTED_VERSION` to verify a
package built from a revision other than the checkout you run them from:

```
EXPECTED_VERSION=0.21.4 .github/bin/test/test_execute.sh
```
