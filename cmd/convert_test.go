//go:build unit

package cmd

import (
	"errors"
	"testing"

	asConf "github.com/aerospike/aerospike-management-lib/asconfig"
)

type preTestConvert struct {
	flags          []string
	arguments      []string
	expectedErrors []error
}

var preTestsConvert = []preTestConvert{
	{
		flags:          []string{"-a", "5.6.0.0"},
		arguments:      []string{"./convert_test.go"},
		expectedErrors: []error{nil},
	},
	{
		flags:     []string{"-a", ""},
		arguments: []string{"./bad_file.yaml", "too_many"},
		expectedErrors: []error{
			errTooManyArguments,
		},
	},
	{
		flags:     []string{},
		arguments: []string{"./bad_file.yaml"},
		expectedErrors: []error{
			errFileNotExist,
		},
	},
	{
		flags:     []string{},
		arguments: []string{"./convert_test.go"},
		expectedErrors: []error{
			errMissingAerospikeVersion,
		},
	},
	{
		flags:          []string{"--force"},
		arguments:      []string{"./convert_test.go"},
		expectedErrors: []error{nil},
	},
	{
		flags:     []string{"--format", "bad_fmt"},
		arguments: []string{"./convert_test.go"},
		expectedErrors: []error{
			errInvalidFormat,
		},
	},
	{
		flags:     []string{"-F", "bad_fmt"},
		arguments: []string{"./convert_test.go"},
		expectedErrors: []error{
			errInvalidFormat,
		},
	},
}

func TestPreRunConvert(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}
	cmd := newConvertCmd()

	for _, test := range preTestsConvert {
		cmd.ParseFlags(test.flags)
		err := cmd.PreRunE(cmd, test.arguments)
		for _, expectedErr := range test.expectedErrors {
			if !errors.Is(err, expectedErr) {
				t.Errorf("actual err: %v\n is not expected err: %v", err, expectedErr)
			}
		}
	}
}

func TestConvertServerYAMLOutputGuards(t *testing.T) {
	cmd := newConvertCmd()
	if err := cmd.ParseFlags([]string{"--server-yaml-output"}); err != nil {
		t.Fatalf("failed to parse convert flags: %v", err)
	}

	_, err := maybeTranslateServerYAMLOutput(cmd, asConf.AeroConfig, "8.1.1", []byte("logging: []"))
	if !errors.Is(err, errServerYAMLOutputRequiresYAML) {
		t.Fatalf("expected YAML output guard error, got: %v", err)
	}

	_, err = maybeTranslateServerYAMLOutput(cmd, asConf.YAML, "8.1.0", []byte("logging: []"))
	if !errors.Is(err, errServerYAMLOutputUnsupportedVersion) {
		t.Fatalf("expected version guard error, got: %v", err)
	}
}
