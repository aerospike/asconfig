//go:build unit

package serveryaml

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestIsLoggingLevel(t *testing.T) {
	cases := []struct {
		name     string
		value    any
		expected bool
	}{
		{"critical", "critical", true},
		{"warning", "warning", true},
		{"info", "info", true},
		{"debug", "debug", true},
		{"detail", "detail", true},
		{"uppercase is accepted", "DEBUG", true},
		{"mixed case is accepted", "Detail", true},
		{"unknown level", "verbose", false},
		{"empty string", "", false},
		{"non-string value", 3, false},
		{"nil", nil, false},
		{"bool", true, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := IsLoggingLevel(tc.value); got != tc.expected {
				t.Fatalf("IsLoggingLevel(%v): expected %v, got %v", tc.value, tc.expected, got)
			}
		})
	}
}

func TestFromLegacyRejectsDuplicateNames(t *testing.T) {
	input := `
namespaces:
  - name: test
    replication-factor: 2
  - name: test
    replication-factor: 3
`

	_, err := FromLegacy([]byte(input))
	if err == nil {
		t.Fatalf("expected duplicate name error, got nil")
	}

	if !strings.Contains(err.Error(), "duplicate name") {
		t.Fatalf("expected duplicate name error, got: %v", err)
	}
}

func TestFromLegacyRejectsMissingName(t *testing.T) {
	input := `
namespaces:
  - replication-factor: 2
`

	_, err := FromLegacy([]byte(input))
	if err == nil {
		t.Fatalf("expected missing name error, got nil")
	}

	if !strings.Contains(err.Error(), "missing required name field") {
		t.Fatalf("expected missing name error, got: %v", err)
	}
}

func TestFromLegacyRejectsNonStringName(t *testing.T) {
	input := `
namespaces:
  - name: 42
    replication-factor: 2
`

	_, err := FromLegacy([]byte(input))
	if err == nil {
		t.Fatalf("expected non-string name error, got nil")
	}

	if !strings.Contains(err.Error(), "name must be a string") {
		t.Fatalf("expected non-string name error, got: %v", err)
	}
}

func TestFromLegacyRejectsNonObjectNamespaceEntry(t *testing.T) {
	input := `
namespaces:
  - not-a-map
`

	_, err := FromLegacy([]byte(input))
	if err == nil {
		t.Fatalf("expected non-object entry error, got nil")
	}

	if !strings.Contains(err.Error(), "namespaces.0 must be an object") {
		t.Fatalf("expected non-object entry error, got: %v", err)
	}
}

func TestFromLegacyRejectsNonMapNetwork(t *testing.T) {
	input := "network: not-a-map\n"
	_, err := FromLegacy([]byte(input))
	if err == nil {
		t.Fatalf("expected network shape error, got nil")
	}

	if !strings.Contains(err.Error(), "network must be an object") {
		t.Fatalf("expected network shape error, got: %v", err)
	}
}

func TestFromLegacyRejectsNonMapXDR(t *testing.T) {
	input := "xdr: not-a-map\n"
	_, err := FromLegacy([]byte(input))
	if err == nil {
		t.Fatalf("expected xdr shape error, got nil")
	}

	if !strings.Contains(err.Error(), "xdr must be an object") {
		t.Fatalf("expected xdr shape error, got: %v", err)
	}
}

// TestFromLegacyPreservesAlreadyNativeShape documents that running FromLegacy
// on a doc that is already in server-native shape does nothing destructive.
// This protects `asconfig convert yaml -> yaml --server-yaml` from silently
// corrupting a file whose source happened to already be native.
func TestFromLegacyPreservesAlreadyNativeShape(t *testing.T) {
	input := `
namespaces:
  test:
    replication-factor: 2
    sets:
      set1:
        enable-index: true
xdr:
  dcs:
    dc1:
      namespaces:
        test:
          forward: true
`

	out, err := FromLegacy([]byte(input))
	if err != nil {
		t.Fatalf("FromLegacy returned error: %v", err)
	}

	parsed := map[string]any{}
	if err := yaml.Unmarshal(out, parsed); err != nil {
		t.Fatalf("unable to unmarshal translated yaml: %v", err)
	}

	ns := mustMap(t, parsed["namespaces"], "namespaces")
	nsTest := mustMap(t, ns["test"], "namespaces.test")
	sets := mustMap(t, nsTest["sets"], "namespaces.test.sets")
	if _, hasSet := sets["set1"]; !hasSet {
		t.Fatalf("expected namespaces.test.sets.set1 to be preserved")
	}
}

func TestFromLegacyLoggingTypeNonString(t *testing.T) {
	input := `
logging:
  - type: 7
`

	_, err := FromLegacy([]byte(input))
	if err == nil {
		t.Fatalf("expected type shape error, got nil")
	}

	if !strings.Contains(err.Error(), "logging.0.type must be a string") {
		t.Fatalf("expected type shape error, got: %v", err)
	}
}

func TestFromLegacyLoggingFileWithoutPath(t *testing.T) {
	// A legacy logging sink with explicit type: file but no path and no name
	// is invalid - it doesn't round-trip to a meaningful native sink.
	input := `
logging:
  - type: file
    any: info
`

	_, err := FromLegacy([]byte(input))
	if err == nil {
		t.Fatalf("expected missing path error, got nil")
	}

	if !strings.Contains(err.Error(), "file sink missing required path") {
		t.Fatalf("expected missing path error, got: %v", err)
	}
}

// TestFromLegacySyslogAndConsoleRoundTrip exercises the common case where a
// legacy sink names the type (not a path). Both should produce native output
// with a `type` field and no path for non-file sinks.
func TestFromLegacySyslogAndConsoleRoundTrip(t *testing.T) {
	input := `
logging:
  - name: console
    any: info
  - name: syslog
    facility: local0
    path: /dev/log
    tag: asd
    any: warning
`

	out, err := FromLegacy([]byte(input))
	if err != nil {
		t.Fatalf("FromLegacy returned error: %v", err)
	}

	parsed := map[string]any{}
	if err := yaml.Unmarshal(out, parsed); err != nil {
		t.Fatalf("unable to unmarshal translated yaml: %v", err)
	}

	sinks := mustSlice(t, parsed["logging"], "logging")
	if len(sinks) != 2 {
		t.Fatalf("expected 2 sinks, got %d", len(sinks))
	}

	console := mustMap(t, sinks[0], "logging[0]")
	assertString(t, console["type"], "console", "logging[0].type")
	if _, hasPath := console["path"]; hasPath {
		t.Fatalf("console sink should not carry a path, got %#v", console["path"])
	}

	consoleContexts := mustMap(t, console["contexts"], "logging[0].contexts")
	assertString(t, consoleContexts["any"], "info", "logging[0].contexts.any")

	syslog := mustMap(t, sinks[1], "logging[1]")
	assertString(t, syslog["type"], "syslog", "logging[1].type")
	assertString(t, syslog["path"], "/dev/log", "logging[1].path")
	assertString(t, syslog["facility"], "local0", "logging[1].facility")
	assertString(t, syslog["tag"], "asd", "logging[1].tag")

	syslogContexts := mustMap(t, syslog["contexts"], "logging[1].contexts")
	assertString(t, syslogContexts["any"], "warning", "logging[1].contexts.any")
}

// TestFromLegacyRoundTripWithToLegacy runs a legacy document through FromLegacy
// and then back through ToLegacy to make sure the pair is stable on the
// interesting shape-bearing fields. This is the closest thing we have to a
// "fixture stays valid after both translators" guarantee at the unit level.
func TestFromLegacyRoundTripWithToLegacy(t *testing.T) {
	legacy := `
namespaces:
  - name: test
    replication-factor: 2
    sets:
      - name: set1
        enable-index: true
    storage-engine:
      type: memory
      data-size: 4000000000
network:
  service:
    port: 3000
  heartbeat:
    mode: mesh
    port: 3002
logging:
  - name: console
    any: info
xdr:
  dcs:
    - name: dc1
      connector: true
      namespaces:
        - name: test
          forward: true
`

	native, err := FromLegacy([]byte(legacy))
	if err != nil {
		t.Fatalf("FromLegacy returned error: %v", err)
	}

	reverted, err := ToLegacy(native)
	if err != nil {
		t.Fatalf("ToLegacy(FromLegacy(...)) returned error: %v", err)
	}

	parsed := map[string]any{}
	if err := yaml.Unmarshal(reverted, parsed); err != nil {
		t.Fatalf("unable to unmarshal round-tripped yaml: %v", err)
	}

	nsSlice := mustSlice(t, parsed["namespaces"], "namespaces")
	if len(nsSlice) != 1 {
		t.Fatalf("expected 1 namespace, got %d", len(nsSlice))
	}

	ns := mustMap(t, nsSlice[0], "namespaces[0]")
	assertString(t, ns["name"], "test", "namespaces[0].name")

	sets := mustSlice(t, ns["sets"], "namespaces[0].sets")
	if len(sets) != 1 {
		t.Fatalf("expected 1 set, got %d", len(sets))
	}

	set := mustMap(t, sets[0], "namespaces[0].sets[0]")
	assertString(t, set["name"], "set1", "namespaces[0].sets[0].name")
}
