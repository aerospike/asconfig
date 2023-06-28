//go:build unit
// +build unit

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
			errFileNotExist,
			errInvalidAerospikeVersion,
			errUnsupportedAerospikeVersion,
		},
	},
	{
		flags:     []string{},
		arguments: []string{"./bad_file.yaml", "too_many"},
		expectedErrors: []error{
			errTooManyArguments,
			errFileNotExist,
			errInvalidAerospikeVersion,
			errUnsupportedAerospikeVersion,
		},
	},
	{
		flags:     []string{},
		arguments: []string{"./convert_test.go"},
		expectedErrors: []error{
			errInvalidAerospikeVersion,
			errUnsupportedAerospikeVersion,
		},
	},
	{
		flags:          []string{"--force"},
		arguments:      []string{"./convert_test.go"},
		expectedErrors: []error{nil},
	},
}

func TestPreRunConvert(t *testing.T) {
	cmd := convertCmd

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
