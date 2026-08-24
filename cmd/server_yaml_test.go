//go:build unit

package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	asConf "github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/spf13/cobra"

	"github.com/aerospike/asconfig/conf/serveryaml"
)

// TestTranslateNativeYAMLForDiffNoOpWhenFlagOff pins down the invariant that
// `diff files` without --server-yaml never rewrites input bytes. Anything
// else would break mixed-format diffs where one side happens to be YAML.
func TestTranslateNativeYAMLForDiffNoOpWhenFlagOff(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool(flagServerYAML, false, "")

	in := []byte("namespaces:\n- name: test\n")
	out, err := translateNativeYAMLForDiff(cmd, asConf.YAML, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(out) != string(in) {
		t.Fatalf("expected bytes to pass through, got: %s", string(out))
	}
}

// TestTranslateNativeYAMLForDiffNoOpForConfInput covers the common case where
// `diff files --server-yaml a.conf b.yaml` is given. The conf side should
// not get translated because the flag only describes YAML shape, and conf
// isn't YAML.
func TestTranslateNativeYAMLForDiffNoOpForConfInput(t *testing.T) {
	cmd := newDiffCmd()
	if err := cmd.ParseFlags([]string{"--server-yaml"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	in := []byte("service {\n  cluster-name \"test\"\n}\n")
	out, err := translateNativeYAMLForDiff(cmd, asConf.AeroConfig, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(out) != string(in) {
		t.Fatalf("expected conf bytes to pass through unchanged, got: %s", string(out))
	}
}

// TestTranslateNativeYAMLForDiffConvertsYAML ensures we actually run the
// native-to-legacy translator when the flag is on and the format is YAML.
// The shape check relies on map-keyed namespaces becoming a named slice,
// which is the primary translation the diff command relies on.
func TestTranslateNativeYAMLForDiffConvertsYAML(t *testing.T) {
	cmd := newDiffCmd()
	if err := cmd.ParseFlags([]string{"--server-yaml"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	in := []byte("namespaces:\n  test:\n    replication-factor: 2\n")
	out, err := translateNativeYAMLForDiff(cmd, asConf.YAML, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	outStr := string(out)
	if !strings.Contains(outStr, "- name: test") && !strings.Contains(outStr, "name: test") {
		t.Fatalf("expected translated output to contain named namespace entry, got:\n%s", outStr)
	}
}

// TestTranslateNativeYAMLForDiffPropagatesTranslatorErrors makes sure the
// diff wiring doesn't accidentally swallow translator errors (for example,
// a malformed XDR section).
func TestTranslateNativeYAMLForDiffPropagatesTranslatorErrors(t *testing.T) {
	cmd := newDiffCmd()
	if err := cmd.ParseFlags([]string{"--server-yaml"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	in := []byte("xdr: not-an-object\n")
	_, err := translateNativeYAMLForDiff(cmd, asConf.YAML, in)
	if err == nil {
		t.Fatalf("expected translator error to propagate, got nil")
	}

	if !strings.Contains(err.Error(), "xdr must be an object") {
		t.Fatalf("expected translator error, got: %v", err)
	}
}

// TestIsServerYAMLFlagSet covers the quiet-failure paths of
// isServerYAMLFlagSet: a nil command and a command without the flag
// registered. Both should return (false, nil) rather than panicking.
func TestIsServerYAMLFlagSet(t *testing.T) {
	got, err := isServerYAMLFlagSet(nil)
	if err != nil {
		t.Fatalf("nil command should not error, got: %v", err)
	}

	if got {
		t.Fatalf("nil command should report flag unset")
	}

	cmd := &cobra.Command{}
	got, err = isServerYAMLFlagSet(cmd)
	if err != nil {
		t.Fatalf("command without flag should not error, got: %v", err)
	}

	if got {
		t.Fatalf("command without flag registered should report unset")
	}

	cmd = &cobra.Command{}
	cmd.Flags().Bool(flagServerYAML, true, "")

	got, err = isServerYAMLFlagSet(cmd)
	if err != nil {
		t.Fatalf("registered flag with default true should not error, got: %v", err)
	}

	if !got {
		t.Fatalf("expected default-true flag to register as set")
	}
}

// TestPrepareYAMLForParseMissingVersion asserts that --server-yaml without a
// resolved server version fails loudly on the input side. This complements
// TestConvertServerYAMLGuards which only exercises the output helper.
func TestPrepareYAMLForParseMissingVersion(t *testing.T) {
	cmd := newValidateCmd()
	if err := cmd.ParseFlags([]string{"--server-yaml"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	_, err := prepareYAMLForParse(cmd, asConf.YAML, "", []byte("service: {}\n"))
	if !errors.Is(err, errServerYAMLRequiresVersion) {
		t.Fatalf("expected errServerYAMLRequiresVersion, got: %v", err)
	}
}

// TestPrepareYAMLForParseUnsupportedVersion mirrors
// TestConvertServerYAMLGuards for the input side so both entry points are
// version-gated consistently.
func TestPrepareYAMLForParseUnsupportedVersion(t *testing.T) {
	cmd := newValidateCmd()
	if err := cmd.ParseFlags([]string{"--server-yaml"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	_, err := prepareYAMLForParse(cmd, asConf.YAML, "8.0.0", []byte("service: {}\n"))
	if !errors.Is(err, errServerYAMLUnsupportedVersion) {
		t.Fatalf("expected errServerYAMLUnsupportedVersion, got: %v", err)
	}
}

// TestFormatServerYAMLValidationErrorsGroupsByContext locks down the output
// format users see when the experimental schema rejects their file. It
// filters out the noisy "number_one_of" stanza that gojsonschema emits for
// every oneOf branch.
func TestFormatServerYAMLValidationErrorsGroupsByContext(t *testing.T) {
	verrs := []serveryaml.ValidationError{
		{
			Context:     "(root)",
			ErrType:     "additional_property_not_allowed",
			Description: "not-a-real-context is not an allowed property",
		},
		{
			Context:     "namespaces.test",
			ErrType:     "required",
			Description: "replication-factor is required",
		},
		{
			Context:     "namespaces.test",
			ErrType:     "number_one_of",
			Description: "noise",
		},
	}

	err := formatServerYAMLValidationErrors(verrs)
	if !errors.Is(err, errServerYAMLSchemaRejection) {
		t.Fatalf("expected wrapped errServerYAMLSchemaRejection, got: %v", err)
	}

	msg := err.Error()
	if !strings.Contains(msg, "context: (root)") {
		t.Fatalf("expected root context header, got: %s", msg)
	}

	if !strings.Contains(msg, "context: namespaces.test") {
		t.Fatalf("expected namespaces.test context header, got: %s", msg)
	}

	if !strings.Contains(msg, "additional_property_not_allowed") {
		t.Fatalf("expected schema error type in output, got: %s", msg)
	}

	if strings.Contains(msg, "number_one_of") {
		t.Fatalf("expected number_one_of noise to be filtered out, got: %s", msg)
	}
}

// TestConvertServerYAMLEndToEnd wires the full convertConfig pipeline in a
// configuration where it matters most: reading a legacy .conf and emitting
// server-native YAML. This is the path the 8.1.1/8.1.2 integration tests
// exercise, but keeping a unit-level version protects against regressions
// that only surface on CI machines that run the integration suite.
func TestConvertServerYAMLEndToEnd(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	srcPath := "../testdata/cases/server812/server812.conf"
	if _, err := os.Stat(srcPath); err != nil {
		t.Skipf("fixture %s not available: %v", srcPath, err)
	}

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "out.yaml")

	cmd := newConvertCmd()
	if err := cmd.ParseFlags([]string{
		"--aerospike-version", "8.1.2",
		"--server-yaml",
		"--output", outPath,
	}); err != nil {
		t.Fatalf("failed to parse convert flags: %v", err)
	}

	if err := cmd.PreRunE(cmd, []string{srcPath}); err != nil {
		t.Fatalf("PreRunE failed: %v", err)
	}

	if err := cmd.RunE(cmd, []string{srcPath}); err != nil {
		t.Fatalf("convert RunE failed: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read converted output: %v", err)
	}

	body := string(data)
	// namespaces should be map-keyed in native YAML, not a list of named
	// objects. These are the easiest signals that --server-yaml actually
	// drove the output format.
	if !strings.Contains(body, "namespaces:") {
		t.Fatalf("expected namespaces block in output, got:\n%s", body)
	}

	if strings.Contains(body, "- name:") {
		t.Fatalf("native yaml output should not contain named slice entries, got:\n%s", body)
	}
}

// TestConvertServerYAMLEndToEndYAMLToConf is the mirror test: a server-native
// YAML fixture read with --server-yaml should round-trip back to a legacy
// .conf without errors. This is the conf -> yaml -> conf loop the
// integration suite exercises, but it confirms the wiring at the cobra
// layer too.
func TestConvertServerYAMLEndToEndYAMLToConf(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	srcPath := "../testdata/cases/server812/server812.experimental.yaml"
	if _, err := os.Stat(srcPath); err != nil {
		t.Skipf("fixture %s not available: %v", srcPath, err)
	}

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "out.conf")

	cmd := newConvertCmd()
	if err := cmd.ParseFlags([]string{
		"--aerospike-version", "8.1.2",
		"--server-yaml",
		"--output", outPath,
		"--format", "yaml",
	}); err != nil {
		t.Fatalf("failed to parse convert flags: %v", err)
	}

	if err := cmd.PreRunE(cmd, []string{srcPath}); err != nil {
		t.Fatalf("PreRunE failed: %v", err)
	}

	if err := cmd.RunE(cmd, []string{srcPath}); err != nil {
		t.Fatalf("convert RunE failed: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read converted output: %v", err)
	}

	body := string(data)
	if !strings.Contains(body, "namespace ns1") {
		t.Fatalf("expected converted .conf to contain 'namespace ns1', got:\n%s", body)
	}

	if !strings.Contains(body, "cluster-name") {
		t.Fatalf("expected converted .conf to contain service.cluster-name, got:\n%s", body)
	}
}

// TestServerYAMLValidatesInputFlagBehavior pins down the helper that tells the
// validate/convert commands when they can safely skip the legacy validator.
// If this contract drifts, users will either see double-validation failures
// or lose schema validation entirely, both of which are regressions.
func TestServerYAMLValidatesInputFlagBehavior(t *testing.T) {
	cases := []struct {
		name       string
		flagSet    bool
		flagArg    []string
		srcFormat  asConf.Format
		expected   bool
		configured bool
	}{
		{
			name:       "flag off, yaml input",
			flagSet:    true,
			flagArg:    nil,
			srcFormat:  asConf.YAML,
			expected:   false,
			configured: true,
		},
		{
			name:       "flag on, yaml input",
			flagSet:    true,
			flagArg:    []string{"--server-yaml"},
			srcFormat:  asConf.YAML,
			expected:   true,
			configured: true,
		},
		{
			name:       "flag on, conf input",
			flagSet:    true,
			flagArg:    []string{"--server-yaml"},
			srcFormat:  asConf.AeroConfig,
			expected:   false,
			configured: true,
		},
		{
			name:       "flag not registered on command",
			flagSet:    false,
			flagArg:    nil,
			srcFormat:  asConf.YAML,
			expected:   false,
			configured: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			if tc.configured {
				cmd.Flags().Bool(flagServerYAML, false, "")
			}

			if len(tc.flagArg) > 0 {
				if err := cmd.ParseFlags(tc.flagArg); err != nil {
					t.Fatalf("failed to parse flag: %v", err)
				}
			}

			got, err := serverYAMLValidatesInput(cmd, tc.srcFormat)
			if err != nil {
				t.Fatalf("serverYAMLValidatesInput errored: %v", err)
			}

			if got != tc.expected {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

// TestConvertServerYAMLAcceptsNativeOnlyFields is the regression test for the
// double-validation fix. A native 8.1.x fixture may contain fields that are
// not present in the legacy schema; with the fix, --server-yaml should still
// accept it because the native schema validated the file and the legacy
// validator is skipped.
func TestConvertServerYAMLAcceptsNativeOnlyFields(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	srcPath := "../testdata/cases/server812/server812.experimental.yaml"
	if _, err := os.Stat(srcPath); err != nil {
		t.Skipf("fixture %s not available: %v", srcPath, err)
	}

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "out.conf")

	cmd := newConvertCmd()
	if err := cmd.ParseFlags([]string{
		"--aerospike-version", "8.1.2",
		"--server-yaml",
		"--output", outPath,
		"--format", "yaml",
	}); err != nil {
		t.Fatalf("failed to parse convert flags: %v", err)
	}

	// Deliberately omit --force. The whole point is that --server-yaml alone
	// should be sufficient; users should not have to reach for --force to
	// convert a native YAML that happens to exercise native-only fields.
	if err := cmd.PreRunE(cmd, []string{srcPath}); err != nil {
		t.Fatalf("PreRunE failed: %v", err)
	}

	if err := cmd.RunE(cmd, []string{srcPath}); err != nil {
		t.Fatalf("convert RunE failed without --force: %v", err)
	}
}
