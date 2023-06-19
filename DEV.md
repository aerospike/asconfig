# Developer Notes

## Adding New Tests

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
