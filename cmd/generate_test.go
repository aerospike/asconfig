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

func TestGenerateServerYAMLGuards(t *testing.T) {
	tests := []struct {
		name      string
		outFormat asConf.Format
		version   string
		expected  error
	}{
		{
			name:      "missing cluster version is rejected",
			outFormat: asConf.YAML,
			version:   "",
			expected:  errServerYAMLRequiresVersion,
		},
		{
			name:      "cluster version below cutoff is rejected",
			outFormat: asConf.YAML,
			version:   "8.0.0",
			expected:  errServerYAMLUnsupportedVersion,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cmd := newGenerateCmd()
			if err := cmd.ParseFlags([]string{"--server-yaml"}); err != nil {
				t.Fatalf("failed to parse generate flags: %v", err)
			}

			_, err := maybeEmitNativeYAML(cmd, tc.outFormat, tc.version, []byte("logging: []"))
			if !errors.Is(err, tc.expected) {
				t.Fatalf("expected error %v, got: %v", tc.expected, err)
			}
		})
	}
}

// TestGenerateServerYAMLNonYAMLOutputIsNoOp ensures --server-yaml silently
// passes through when the output format is not YAML.
func TestGenerateServerYAMLNonYAMLOutputIsNoOp(t *testing.T) {
	cmd := newGenerateCmd()
	if err := cmd.ParseFlags([]string{"--server-yaml"}); err != nil {
		t.Fatalf("failed to parse generate flags: %v", err)
	}

	in := []byte("namespaces: []")
	out, err := maybeEmitNativeYAML(cmd, asConf.AeroConfig, "", in)
	if err != nil {
		t.Fatalf("expected no-op when output is not YAML, got: %v", err)
	}

	if string(out) != string(in) {
		t.Fatalf("expected bytes to pass through, got: %s", string(out))
	}
}
