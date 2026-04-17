//go:build unit

package cmd

import (
	"errors"
	"testing"

	asConf "github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/spf13/cobra"
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

func TestConvertServerYAMLGuards(t *testing.T) {
	tests := []struct {
		name      string
		outFormat asConf.Format
		version   string
		expected  error
	}{
		{
			name:      "missing version is rejected",
			outFormat: asConf.YAML,
			version:   "",
			expected:  errServerYAMLRequiresVersion,
		},
		{
			name:      "versions below cutoff are rejected",
			outFormat: asConf.YAML,
			version:   "8.0.0",
			expected:  errServerYAMLUnsupportedVersion,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cmd := newConvertCmd()
			if err := cmd.ParseFlags([]string{"--server-yaml"}); err != nil {
				t.Fatalf("failed to parse convert flags: %v", err)
			}

			_, err := maybeEmitNativeYAML(cmd, tc.outFormat, tc.version, []byte("logging: []"))
			if !errors.Is(err, tc.expected) {
				t.Fatalf("expected error %v, got: %v", tc.expected, err)
			}
		})
	}
}

// TestConvertServerYAMLNonYAMLSideIsNoOp ensures that when --server-yaml is
// set but the relevant side (input or output) is not YAML, the helper is a
// no-op. This is what lets `convert --server-yaml conf -> yaml` and `convert
// --server-yaml yaml -> conf` both work: the flag applies only to the YAML
// side of the operation.
func TestConvertServerYAMLNonYAMLSideIsNoOp(t *testing.T) {
	t.Run("input side is conf", func(t *testing.T) {
		cmd := newConvertCmd()
		if err := cmd.ParseFlags([]string{"--server-yaml"}); err != nil {
			t.Fatalf("failed to parse convert flags: %v", err)
		}

		in := []byte("# legacy conf input\n")
		out, err := prepareYAMLForParse(cmd, asConf.AeroConfig, "", in)
		if err != nil {
			t.Fatalf("expected no-op when source is not YAML, got: %v", err)
		}

		if string(out) != string(in) {
			t.Fatalf("expected bytes to pass through, got: %s", string(out))
		}
	})

	t.Run("output side is conf", func(t *testing.T) {
		cmd := newConvertCmd()
		if err := cmd.ParseFlags([]string{"--server-yaml"}); err != nil {
			t.Fatalf("failed to parse convert flags: %v", err)
		}

		in := []byte("namespaces: []")
		out, err := maybeEmitNativeYAML(cmd, asConf.AeroConfig, "", in)
		if err != nil {
			t.Fatalf("expected no-op when output is not YAML, got: %v", err)
		}

		if string(out) != string(in) {
			t.Fatalf("expected bytes to pass through, got: %s", string(out))
		}
	})
}

func TestConvertServerYAMLFlagOff(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool(flagServerYAML, false, "")

	if err := cmd.ParseFlags([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	out, err := maybeEmitNativeYAML(cmd, asConf.YAML, "", []byte("namespaces: []"))
	if err != nil {
		t.Fatalf("expected no-op when flag is off, got: %v", err)
	}

	if string(out) != "namespaces: []" {
		t.Fatalf("expected bytes to pass through, got: %s", string(out))
	}
}
