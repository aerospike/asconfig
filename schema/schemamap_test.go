//go:build unit

package schema

import (
	"strings"
	"testing"
)

// TestNewExperimentalSchemaMapReturnsNativeSchemas makes sure the new
// experimental schemas submodule is embedded and reachable via the exported
// helper. Bumping the submodule without rebuilding asconfig would silently
// break --server-yaml otherwise.
func TestNewExperimentalSchemaMapReturnsNativeSchemas(t *testing.T) {
	schemas, err := NewExperimentalSchemaMap()
	if err != nil {
		t.Fatalf("NewExperimentalSchemaMap errored: %v", err)
	}

	if len(schemas) == 0 {
		t.Fatalf("expected at least one embedded experimental schema")
	}

	required := []string{"8.1.1", "8.1.2"}
	for _, v := range required {
		body, ok := schemas[v]
		if !ok {
			t.Fatalf("expected embedded experimental schema for %s, available: %v", v, keys(schemas))
		}

		// Sanity-check the embedded bytes look like a JSON schema rather
		// than, say, a markdown README that accidentally got globbed in.
		if !strings.Contains(body, "\"$schema\"") {
			t.Fatalf("embedded schema %s does not appear to be JSON-schema shaped", v)
		}
	}
}

// TestNewSchemaMapStillLoadsLegacySchemas checks the original helper still
// works after adding the experimental one. If //go:embed started picking up
// the wrong directory tree we'd want to catch it here, not in a flaky CI run.
func TestNewSchemaMapStillLoadsLegacySchemas(t *testing.T) {
	schemas, err := NewSchemaMap()
	if err != nil {
		t.Fatalf("NewSchemaMap errored: %v", err)
	}

	if len(schemas) == 0 {
		t.Fatalf("expected at least one embedded legacy schema")
	}

	// Legacy schemas have historically included 7.0.0 and above; this is
	// indirectly asserted by cmd/diff_test.go which expects 7.0.0 -> 8.1.0
	// to work. Keep it tight here but not so tight that adding a new
	// version breaks the test.
	if _, ok := schemas["7.0.0"]; !ok {
		t.Fatalf("expected embedded legacy schema for 7.0.0, available: %v", keys(schemas))
	}
}

// TestExperimentalAndLegacyDoNotCollide documents that schema.SchemaMap is
// produced by two independent collectors with disjoint embed roots. If the
// roots ever overlap (e.g. someone points //go:embed at `schemas/json`
// instead of the aerospike-server subfolder) this test will notice when the
// two maps start returning the same 8.1.x entries with identical bodies.
func TestExperimentalAndLegacyDoNotCollide(t *testing.T) {
	legacy, err := NewSchemaMap()
	if err != nil {
		t.Fatalf("NewSchemaMap errored: %v", err)
	}

	experimental, err := NewExperimentalSchemaMap()
	if err != nil {
		t.Fatalf("NewExperimentalSchemaMap errored: %v", err)
	}

	legacy811, hasLegacy := legacy["8.1.1"]
	experimental811, hasExperimental := experimental["8.1.1"]

	if !hasLegacy || !hasExperimental {
		t.Skipf("legacy and experimental both need 8.1.1 to compare; skipping")
	}

	if legacy811 == experimental811 {
		t.Fatalf("expected legacy and experimental 8.1.1 schemas to differ")
	}
}

func keys(m SchemaMap) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	return out
}
