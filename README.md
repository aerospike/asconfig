# Asconfig

A CLI tool for managing Aerospike configuration files.

## Overview

Asconfig allows you to manage and create Aerospike configuration using a versioned schema directory.
For more information and usage examples see the [Aerospike Configuration Tool docs](https://docs.aerospike.com/tools/asconfig).

## Build

Build asconfig using the included makefile and display usage information.

```shell
git clone https://github.com/aerospike/asconfig.git
cd asconfig
make
./bin/asconfig --help
```

The built binary is available at bin/asconfig.

You can also build asconfig using `go build`.

```shell
git clone https://github.com/aerospike/asconfig.git
cd asconfig
git submodule update --init
go build -o ./bin/asconfig
./bin/asconfig --help
```

Install os uninstall asconfig.

```shell
make install
```

```shell
make uninstall
```

Cleanup build/test files.

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
A path to an Aerospike feature key file should be defined at the `FEATKEY` environemnt variable.
For more information about the feature key file see the [feature-key docs](https://docs.aerospike.com/server/operations/configure/feature-key).

```shell
FEATKEY=/path/to/aerospike/features.conf make integration
```

### All Tests

```shell
FEATKEY=/path/to/aerospike/features.conf make test
```

### Test Coverage

```shell
FEATKEY=/path/to/aerospike/features.conf make view-coverage
```

## Developer Notes

### Adding New Tests

The asconfig integration tests rely on the configuration files in testdata/sources, testdata/expected, and testdata/cases.
To add new integration test cases from an existing Aerospike configuration file. Use the testgen tool in testutils.

Below is an example of generating a new testcase directory from test_aerospike.conf.
Obfuscate sensitive fields with -obfuscate.
-original-version=5.7.0.21 records the Aerospike server version the config is used with.
-aerospike-version=5.7.0.17 the Aerospike version for the docker container used in the integration tests.

Any test cases in testdata/cases are pulled in automatically by the integration_test.go when the tests run.

```shell
go run testutils/main/testgen.go  -output=./testdata/cases -obfuscate -aerospike-version=5.7.0.17 -original-version=5.7.0.21 --overwrite /Users/me/Desktop/test_aerospike.conf
```