# Asconfig

A CLI tool for managing Aerospike configuration files.

## Overview

Asconfig allows you to validate and compare Aerospike configuration using a versioned YAML schema directory.
For more information and usage examples see the [Aerospike Configuration Tool docs](https://docs.aerospike.com/tools/asconfig).

## Build

Build asconfig using the included makefile and display usage information.

```shell
git clone https://github.com/aerospike/asconfig.git
cd asconfig
git submodule update --init
make
./bin/asconfig --help
```

The built binary is available at bin/asconfig.

Install or uninstall asconfig.

```shell
make install
```

```shell
make uninstall
```

Cleanup build and test files.

```shell
make clean
```

Building rpm, deb, and tar packages is also done using the makefile.
You will have to install fpm and rpmbuild to build all of these.

```shell
make rpm deb tar
```

The packages will be available in the pkg/ directory.

## Testing

Asconfig has unit and integration tests.

You can run the tests using the make file.

### Unit Tests

```shell
make unit
```

### Integration Tests

Integration tests require that docker is installed and running.
A path to an Aerospike feature key file should be defined at the `FEATKEY_DIR` environment variable.
For more information about the feature key file see the [feature-key docs](https://docs.aerospike.com/server/operations/configure/feature-key).

```shell
FEATKEY_DIR=/path/to/aerospike/features/dir make integration
```

### All Tests

```shell
FEATKEY_DIR=/path/to/aerospike/features/dir make test
```

### Test Coverage

```shell
FEATKEY_DIR=/path/to/aerospike/features/dir make view-coverage
```
