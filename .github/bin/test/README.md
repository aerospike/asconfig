# Post-install smoke tests

`test_execute.bats` runs in CI (`Test install & execute`) on every Linux distro and macOS
runner, against the signed package after it is installed. It checks that asconfig runs and
that **the version it reports matches the repo's `VERSION` file**, so a package labelled with
one version but carrying a binary stamped with another fails here instead of downstream in
the aerospike-tools bundle.

Both printed lines are compared. `SetupRoot` splits the embedded string on `-`, so
`0.21.4-rc1` reports `Version 0.21.4` / `Build rc1` and a bare `0.21.4` reports no `Build`
line at all. Asserting the `Build` line is what catches a stale rc binary inside a later
rc's package, where the two differ only by the iteration.

```
bats .github/bin/test/test_execute.bats
```

`test_execute.sh` is the same checks without bats; both share the assertions in
`version_lib.sh`.

Both accept `EXPECTED_VERSION` to verify a package built from a revision other than the
checkout you run them from. CI sets it to the workflow's `BUILD_VERSION`, which on a dev
build carries the commit sha instead of the `rcN` in the file. It still has to agree with
the VERSION file on `MAJOR.MINOR.PATCH`, so a run wired to the wrong workflow output fails
rather than testing against whatever it was handed.

```
EXPECTED_VERSION=0.21.4-rc1 .github/bin/test/test_execute.sh
```
