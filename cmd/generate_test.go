//go:build unit

package cmd

import (
	"errors"
	"testing"

	asConf "github.com/aerospike/aerospike-management-lib/asconfig"
)

func TestRunEGenerate(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}
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

func TestGenerateServerYAMLOutputGuards(t *testing.T) {
	cmd := newGenerateCmd()
	if err := cmd.ParseFlags([]string{"--server-yaml-output"}); err != nil {
		t.Fatalf("failed to parse generate flags: %v", err)
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
