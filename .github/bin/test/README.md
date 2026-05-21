# Post-install smoke tests

The CI workflow verifies the install inline: `asconfig --help` and `asconfig --version`.

`test_execute.sh` and `test_execute.bats` are for local verification after you install the
package yourself (for example from CI artifacts or a local `.deb` / `.rpm`).
