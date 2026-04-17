//go:build unit

package serveryaml

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestToLegacyPassesThroughAlreadyLegacyShapes makes sure that when the input
// already looks like legacy asconfig YAML (slices instead of maps) the
// translator leaves the shape alone. This is important because asconfig is
// expected to be robust to users pointing --server-yaml at a legacy file.
func TestToLegacyPassesThroughAlreadyLegacyShapes(t *testing.T) {
	input := `
namespaces:
  - name: test
    replication-factor: 2
xdr:
  dcs:
    - name: dc1
      connector: true
`

	out, err := ToLegacy([]byte(input))
	if err != nil {
		t.Fatalf("ToLegacy returned error: %v", err)
	}

	parsed := map[string]any{}
	if err := yaml.Unmarshal(out, parsed); err != nil {
		t.Fatalf("unable to unmarshal translated yaml: %v", err)
	}

	namespaces := mustSlice(t, parsed["namespaces"], "namespaces")
	if len(namespaces) != 1 {
		t.Fatalf("expected 1 namespace slice entry, got %d", len(namespaces))
	}

	ns := mustMap(t, namespaces[0], "namespaces[0]")
	assertString(t, ns["name"], "test", "namespaces[0].name")

	xdr := mustMap(t, parsed["xdr"], "xdr")
	dcs := mustSlice(t, xdr["dcs"], "xdr.dcs")
	if len(dcs) != 1 {
		t.Fatalf("expected 1 dc slice entry, got %d", len(dcs))
	}
}

func TestToLegacyRejectsNonObjectNamespace(t *testing.T) {
	input := "namespaces:\n  test: not-an-object\n"
	_, err := ToLegacy([]byte(input))
	if err == nil {
		t.Fatalf("expected error when a namespace entry is not an object")
	}

	if !strings.Contains(err.Error(), "namespaces.test must be an object") {
		t.Fatalf("expected namespace shape error, got: %v", err)
	}
}

func TestToLegacyRejectsNonMapNetwork(t *testing.T) {
	input := "network: not-a-map\n"
	_, err := ToLegacy([]byte(input))
	if err == nil {
		t.Fatalf("expected error when network is not an object")
	}

	if !strings.Contains(err.Error(), "network must be an object") {
		t.Fatalf("expected network shape error, got: %v", err)
	}
}

func TestToLegacyHandlesMissingOptionalSections(t *testing.T) {
	// Barest possible document; every optional translator branch should
	// short-circuit without touching the document.
	input := "service:\n  cluster-name: empty\n"

	out, err := ToLegacy([]byte(input))
	if err != nil {
		t.Fatalf("ToLegacy returned error: %v", err)
	}

	parsed := map[string]any{}
	if err := yaml.Unmarshal(out, parsed); err != nil {
		t.Fatalf("unable to unmarshal translated yaml: %v", err)
	}

	svc := mustMap(t, parsed["service"], "service")
	assertString(t, svc["cluster-name"], "empty", "service.cluster-name")

	if _, hasNs := parsed["namespaces"]; hasNs {
		t.Fatalf("expected no namespaces key when absent from input")
	}
}

func TestToLegacyXDRDCsAlreadyAsSlice(t *testing.T) {
	input := `
xdr:
  dcs:
    - name: dc1
      namespaces:
        test:
          forward: true
`

	out, err := ToLegacy([]byte(input))
	if err != nil {
		t.Fatalf("ToLegacy returned error: %v", err)
	}

	parsed := map[string]any{}
	if err := yaml.Unmarshal(out, parsed); err != nil {
		t.Fatalf("unable to unmarshal translated yaml: %v", err)
	}

	// dcs stays a slice, but dc1.namespaces (map form) does NOT get rewritten
	// because we only translate the nested namespace collection when dcs was
	// originally a map. That's intentional: this path mirrors what the
	// server-native wrapper would produce.
	xdr := mustMap(t, parsed["xdr"], "xdr")
	dcs := mustSlice(t, xdr["dcs"], "xdr.dcs")
	if len(dcs) != 1 {
		t.Fatalf("expected 1 dc, got %d", len(dcs))
	}
}

func TestToLegacyXDRMissingDCs(t *testing.T) {
	input := "xdr:\n  src-id: 1\n"

	out, err := ToLegacy([]byte(input))
	if err != nil {
		t.Fatalf("ToLegacy returned error: %v", err)
	}

	parsed := map[string]any{}
	if err := yaml.Unmarshal(out, parsed); err != nil {
		t.Fatalf("unable to unmarshal translated yaml: %v", err)
	}

	xdr := mustMap(t, parsed["xdr"], "xdr")
	if _, hasDCs := xdr["dcs"]; hasDCs {
		t.Fatalf("expected no dcs key when absent from input")
	}
}

func TestToLegacyRejectsNonMapXDR(t *testing.T) {
	_, err := ToLegacy([]byte("xdr: not-a-map\n"))
	if err == nil {
		t.Fatalf("expected error when xdr is not an object")
	}

	if !strings.Contains(err.Error(), "xdr must be an object") {
		t.Fatalf("expected xdr shape error, got: %v", err)
	}
}

func TestToLegacyRejectsNonMapXDRDC(t *testing.T) {
	_, err := ToLegacy([]byte("xdr:\n  dcs:\n    dc1: not-a-map\n"))
	if err == nil {
		t.Fatalf("expected error when dc entry is not an object")
	}

	if !strings.Contains(err.Error(), "xdr.dcs.dc1 must be an object") {
		t.Fatalf("expected dc shape error, got: %v", err)
	}
}

func TestToLegacyLoggingContextFieldConflict(t *testing.T) {
	// Using a key that's also valid on the sink itself ("tag") would
	// silently clobber sink metadata, so the translator errors loudly.
	input := `
logging:
  - type: syslog
    tag: asd
    facility: local0
    path: /dev/log
    contexts:
      tag: warning
`

	_, err := ToLegacy([]byte(input))
	if err == nil {
		t.Fatalf("expected conflict error, got nil")
	}

	if !strings.Contains(err.Error(), "conflicts with existing sink field") {
		t.Fatalf("expected conflict error, got: %v", err)
	}
}

func TestToLegacyLoggingNonStringType(t *testing.T) {
	input := `
logging:
  - type: 42
    path: /tmp/x.log
`

	_, err := ToLegacy([]byte(input))
	if err == nil {
		t.Fatalf("expected type shape error, got nil")
	}

	if !strings.Contains(err.Error(), "logging.0.type must be a string") {
		t.Fatalf("expected type shape error, got: %v", err)
	}
}

func TestToLegacyFileSinkMissingPath(t *testing.T) {
	input := `
logging:
  - type: file
    contexts:
      any: info
`

	_, err := ToLegacy([]byte(input))
	if err == nil {
		t.Fatalf("expected missing path error, got nil")
	}

	if !strings.Contains(err.Error(), "file sink missing required path") {
		t.Fatalf("expected missing path error, got: %v", err)
	}
}

func TestToLegacyLoggingSinkNotAMap(t *testing.T) {
	input := `
logging:
  - not-a-map
`

	_, err := ToLegacy([]byte(input))
	if err == nil {
		t.Fatalf("expected sink shape error, got nil")
	}

	if !strings.Contains(err.Error(), "logging.0 must be an object") {
		t.Fatalf("expected sink shape error, got: %v", err)
	}
}

func TestToLegacyLoggingNonSliceIsPassedThrough(t *testing.T) {
	// If logging comes through as a map (e.g. a malformed doc) the translator
	// leaves it alone; we rely on validation to catch it. This test just
	// pins that behavior so it doesn't regress into a surprise error.
	input := "logging:\n  console: info\n"

	_, err := ToLegacy([]byte(input))
	if err != nil {
		t.Fatalf("expected no error for non-slice logging, got: %v", err)
	}
}

// TestToLegacyContextsMustBeMap checks that a logging sink with a malformed
// contexts field (e.g. a string instead of an object) produces a clear error
// rather than silently dropping the field.
func TestToLegacyContextsMustBeMap(t *testing.T) {
	input := `
logging:
  - type: console
    contexts: info
`

	_, err := ToLegacy([]byte(input))
	if err == nil {
		t.Fatalf("expected contexts shape error, got nil")
	}

	if !strings.Contains(err.Error(), "contexts must be an object") {
		t.Fatalf("expected contexts shape error, got: %v", err)
	}
}

func TestFormatNodePath(t *testing.T) {
	cases := []struct {
		path     []string
		expected string
	}{
		{nil, "(root)"},
		{[]string{}, "(root)"},
		{[]string{"namespaces"}, "namespaces"},
		{[]string{"namespaces", "test", "sets"}, "namespaces.test.sets"},
	}

	for _, tc := range cases {
		if got := formatNodePath(tc.path); got != tc.expected {
			t.Fatalf("formatNodePath(%v) = %q, want %q", tc.path, got, tc.expected)
		}
	}
}

func TestSortedKeysIsDeterministic(t *testing.T) {
	// The deterministic ordering is load-bearing for diff output; translator
	// tests elsewhere rely on it implicitly, so make it explicit here.
	input := map[string]any{
		"c": 1,
		"a": 1,
		"b": 1,
	}

	got := sortedKeys(input)
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("expected %d keys, got %d", len(want), len(got))
	}

	for i, k := range want {
		if got[i] != k {
			t.Fatalf("index %d: expected %q, got %q", i, k, got[i])
		}
	}
}
