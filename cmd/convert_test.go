//go:build unit

package cmd

import (
	"errors"
	"testing"
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
