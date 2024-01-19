//go:build unit

package cmd

import (
	"errors"
	"testing"
)

func TestRunEGenerate(t *testing.T) {
	type runTestGen struct {
		flags       []string
		arguments   []string
		expectError error
	}

	var testGenArgs = []runTestGen{
		{
			flags:       []string{},
			arguments:   []string{"too", "many", "args"},
			expectError: errTooManyArguments,
		},
		{
			flags:       []string{"--format", "bad_fmt"},
			arguments:   []string{},
			expectError: errInvalidFormat,
		},
		{
			flags:       []string{"-F", "bad_fmt"},
			arguments:   []string{},
			expectError: errInvalidFormat,
		},
	}
	cmd := newGenerateCmd()

	for i, test := range testGenArgs {
		cmd.ParseFlags(test.flags)
		err := cmd.PreRunE(cmd, test.arguments)
		if !errors.Is(err, test.expectError) {
			t.Fatalf("case: %d, expectError: %v does not match err: %v", i, test.expectError, err)
		}
	}
}
