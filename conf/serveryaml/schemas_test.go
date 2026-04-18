//go:build unit

package serveryaml

import (
	"errors"
	"strings"
	"testing"

	"github.com/aerospike/asconfig/schema"
)

func TestLoadSchemaRejectsEmptyVersion(t *testing.T) {
	_, err := LoadSchema("")
	if !errors.Is(err, ErrMissingVersion) {
		t.Fatalf("expected ErrMissingVersion, got: %v", err)
	}
}

func TestLoadSchemaRejectsInvalidVersion(t *testing.T) {
	_, err := LoadSchema("not-a-version")
	if err == nil {
		t.Fatalf("expected error for malformed version")
	}

	if !strings.Contains(err.Error(), "invalid aerospike-server-version") {
		t.Fatalf("expected invalid version error, got: %v", err)
	}
}

// TestLoadSchemaNoMatchReturnsError pins down what happens when asconfig is
// asked for a version that is entirely outside the range of embedded schemas.
// Right now the embedded schemas only cover the 8.1.x minor, so a 7.x request
// has no exact match, no `<= target` match, and no same-minor fallback.
// That's expected to surface as a clear "no schema found" error rather than
// silently returning something unrelated.
func TestLoadSchemaNoMatchReturnsError(t *testing.T) {
	_, err := LoadSchema("7.0.0")
	if err == nil {
		t.Fatalf("expected no-schema error, got nil")
	}

	if !strings.Contains(err.Error(), "no native yaml schema found") {
		t.Fatalf("expected no-schema error, got: %v", err)
	}
}

func TestResolveSchemaVersionPicksHighestMatch(t *testing.T) {
	schemas := schema.SchemaMap{
		"8.1.1": "{}",
		"8.1.2": "{}",
		"8.2.0": "{}",
	}

	got := resolveSchemaVersion(schemas, "8.1.9")
	if got != "8.1.2" {
		t.Fatalf("expected 8.1.2 (highest <= 8.1.9), got %q", got)
	}
}

func TestResolveSchemaVersionEmptyWhenAllAbove(t *testing.T) {
	schemas := schema.SchemaMap{
		"8.1.1": "{}",
		"8.1.2": "{}",
	}

	got := resolveSchemaVersion(schemas, "8.0.0")
	if got != "" {
		t.Fatalf("expected empty string when every schema is newer than target, got %q", got)
	}
}

func TestLowestSchemaForSameMinor(t *testing.T) {
	schemas := schema.SchemaMap{
		"8.1.1": "{}",
		"8.1.2": "{}",
		"8.2.0": "{}",
	}

	got := lowestSchemaForSameMinor(schemas, "8.1.0")
	if got != "8.1.1" {
		t.Fatalf("expected 8.1.1 as lowest same-minor for 8.1.0, got %q", got)
	}

	got = lowestSchemaForSameMinor(schemas, "9.0.0")
	if got != "" {
		t.Fatalf("expected empty string when minor has no schemas, got %q", got)
	}
}

func TestExtractBaseVersion(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"three segments", "8.1.2", "8.1.2", false},
		{"four segments truncates", "8.1.2.0", "8.1.2", false},
		{"two segments rejected", "8.1", "", true},
		{"empty rejected", "", "", true},
		{"garbage rejected", "abc", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractBaseVersion(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestSanitizeExperimentalSchemaRemovesUnsupportedLookahead(t *testing.T) {
	// The unsupported pattern lives inside a schema literal; we stub a
	// minimal fragment that contains it and make sure it's replaced.
	input := `{"pattern": "^(?!null$)[a-zA-Z0-9_\\-$]+$"}`

	got := sanitizeExperimentalSchema(input)
	if strings.Contains(got, "(?!") {
		t.Fatalf("expected negative lookahead to be removed, got: %s", got)
	}

	// Patterns without the lookahead must be untouched.
	unchanged := `{"pattern": "^[a-z]+$"}`
	if sanitizeExperimentalSchema(unchanged) != unchanged {
		t.Fatalf("expected unchanged pattern to be preserved")
	}
}

// TestLoadExperimentalSchemasReturnsMultipleVersions is a sanity check that
// the schema submodule is embedded as expected and both 8.1.1 and 8.1.2 are
// reachable. If this ever starts failing it usually means the submodule
// wasn't updated in sync with a code change that depends on it.
func TestLoadExperimentalSchemasReturnsMultipleVersions(t *testing.T) {
	schemas, err := loadExperimentalSchemas()
	if err != nil {
		t.Fatalf("loadExperimentalSchemas errored: %v", err)
	}

	required := []string{"8.1.1", "8.1.2"}
	for _, v := range required {
		if _, ok := schemas[v]; !ok {
			t.Fatalf("expected embedded schema for %s, available keys: %v", v, keys(schemas))
		}
	}
}

func TestValidateSurfacesYAMLParseErrors(t *testing.T) {
	// Unparseable YAML should surface a clear error rather than producing
	// an empty slice of validation failures. Mixing a scalar with a
	// mapping under the same key is a syntactic error.
	bad := []byte("service: foo\n  heartbeat: bar\n")

	_, err := Validate(bad, "8.1.1")
	if err == nil {
		t.Fatalf("expected yaml parse error, got nil")
	}
}

func TestValidateRejectsBadVersion(t *testing.T) {
	_, err := Validate([]byte("service: {}\n"), "garbage")
	if err == nil {
		t.Fatalf("expected error for invalid version, got nil")
	}

	if !strings.Contains(err.Error(), "invalid aerospike-server-version") {
		t.Fatalf("expected invalid version error, got: %v", err)
	}
}

// TestValidationErrorError locks the Error() formatting in place because
// cmd/server_yaml.go expects to be able to group and print these errors.
func TestValidationErrorError(t *testing.T) {
	v := ValidationError{
		Context:     "(root)",
		Field:       "service",
		ErrType:     "required",
		Description: "service is required",
	}

	got := v.Error()
	if !strings.Contains(got, "description: service is required") {
		t.Fatalf("expected description in output, got: %s", got)
	}

	if !strings.Contains(got, "error-type: required") {
		t.Fatalf("expected error-type in output, got: %s", got)
	}
}

func keys(m schema.SchemaMap) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	return out
}
