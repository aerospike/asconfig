//go:build unit
// +build unit

package cmd

import (
	"errors"
	"testing"
)

type preTest struct {
	flags          []string
	arguments      []string
	expectedErrors []error
}

var preTests = []preTest{
	{
		flags:          []string{"-a", "5.6.0.0"},
		arguments:      []string{"./root_test.go"},
		expectedErrors: []error{nil},
	},
	{
		flags:     []string{"-a", "5.6.0.0"},
		arguments: []string{},
		expectedErrors: []error{
			errNotEnoughArguments,
		},
	},
	{
		flags:     []string{"-a", ""},
		arguments: []string{"./bad_file.yaml", "./", "too_many"},
		expectedErrors: []error{
			errTooManyArguments,
			errFileNotExist,
			errFileisDir,
			errInvalidAerospikeVersion,
			errUnsupportedAerospikeVersion,
		},
	},
}

func TestPreRun(t *testing.T) {
	cmd := newRootCmd()

	for _, test := range preTests {
		cmd.ParseFlags(test.flags)
		err := cmd.PreRunE(cmd, test.arguments)
		for _, expectedErr := range test.expectedErrors {
			if !errors.Is(err, expectedErr) {
				t.Errorf("actual err: %v\n is not expected err: %v", err, expectedErr)
			}
		}
	}
}
