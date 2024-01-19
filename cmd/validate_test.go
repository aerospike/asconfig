//go:build unit

package cmd

import (
	"testing"
)

type runTestValidate struct {
	flags       []string
	arguments   []string
	expectError bool
}

var testValidateArgs = []runTestValidate{
	{
		flags:       []string{},
		arguments:   []string{"too", "many", "args"},
		expectError: true,
	},
	{
		// missing arg to -a-aerospike-version
		flags:       []string{"--aerospike-version"},
		arguments:   []string{"../testdata/sources/all_flash_cluster_cr.yaml"},
		expectError: true,
	},
	{
		flags:       []string{"--aerospike-version"},
		arguments:   []string{"./bad_extension.ymml"},
		expectError: true,
	},
	{
		flags:       []string{"--aerospike-version", "bad_version"},
		arguments:   []string{"../testdata/sources/all_flash_cluster_cr.yaml"},
		expectError: true,
	},
	{
		flags:       []string{"--aerospike-version", "6.4.0"},
		arguments:   []string{"./fake_file.yaml"},
		expectError: true,
	},
	{
		flags:       []string{"--aerospike-version", "6.4.0"},
		arguments:   []string{"../testdata/cases/server64/server64.yaml"},
		expectError: false,
	},
	{
		flags:       []string{"--log-level", "debug", "--aerospike-version", "7.0.0"},
		arguments:   []string{"../testdata/cases/server70/server70.conf"},
		expectError: false,
	},
	{
		flags:       []string{"--format", "bad_fmt"},
		arguments:   []string{},
		expectError: true,
	},
	{
		flags:       []string{"-F", "bad_fmt"},
		arguments:   []string{},
		expectError: true,
	},
}

func TestRunEValidate(t *testing.T) {
	cmd := validateCmd

	for i, test := range testValidateArgs {
		cmd.Parent().ParseFlags(test.flags)
		cmd.ParseFlags(test.flags)
		err := cmd.RunE(cmd, test.arguments)
		if test.expectError == (err == nil) {
			t.Fatalf("case: %d, expectError: %v does not match err: %v", i, test.expectError, err)
		}
	}
}
