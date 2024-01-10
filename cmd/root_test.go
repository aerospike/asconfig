package cmd

import (
	"errors"
	"testing"
)

type preTestRoot struct {
	flags          []string
	arguments      []string
	expectedErrors []error
}

var preTestsRoot = []preTestRoot{
	{
		flags:          []string{"-l", "info"},
		arguments:      []string{},
		expectedErrors: []error{nil},
	},
	{
		flags:          []string{"-l", "panic"},
		arguments:      []string{},
		expectedErrors: []error{nil},
	},
	{
		flags:     []string{"--log-level", "bad_level"},
		arguments: []string{},
		expectedErrors: []error{
			errInvalidLogLevel,
		},
	},
}

func TestPersistentPreRunRoot(t *testing.T) {
	cmd := newRootCmd()

	for _, test := range preTestsRoot {
		cmd.ParseFlags(test.flags)
		err := cmd.PersistentPreRunE(cmd, test.arguments)
		for _, expectedErr := range test.expectedErrors {
			if !errors.Is(err, expectedErr) {
				t.Errorf("%v\n actual err: %v\n is not expected err: %v", test.flags, err, expectedErr)
			}
		}
	}
}
