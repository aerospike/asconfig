package serveryaml

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestToLegacyBasic(t *testing.T) {
	input := `
service:
  cluster-name: compat-cluster
network:
  service:
    port: 3000
  heartbeat:
    mode: mesh
    port: 3002
    interval: 150
    timeout: 10
  fabric:
    port: 3001
  tls:
    tls1:
      cert-file: /a.crt
logging:
  - type: console
    contexts:
      any: info
  - type: file
    path: /var/log/aerospike.log
    contexts:
      namespace: detail
  - type: syslog
    facility: local0
    path: /dev/log
    tag: asd
    contexts:
      any: warning
namespaces:
  test:
    replication-factor: 2
    sets:
      set1:
        enable-index: true
    storage-engine:
      type: memory
      data-size:
        value: 4
        unit: g
    nsup-period:
      value: 30
      unit: s
xdr:
  dcs:
    dc1:
      namespaces:
        test:
          forward: true
`

	out, err := ToLegacy([]byte(input))
	if err != nil {
		t.Fatalf("ToLegacy returned error: %v", err)
	}

	translated := map[string]any{}
	if err := yaml.Unmarshal(out, translated); err != nil {
		t.Fatalf("unable to unmarshal translated yaml: %v", err)
	}

	namespaces := mustSlice(t, translated["namespaces"], "namespaces")
	if len(namespaces) != 1 {
		t.Fatalf("expected 1 namespace, got %d", len(namespaces))
	}

	ns := mustMap(t, namespaces[0], "namespaces[0]")
	assertString(t, ns["name"], "test", "namespaces[0].name")

	storage := mustMap(t, ns["storage-engine"], "namespaces[0].storage-engine")
	assertInt64(t, storage["data-size"], 4*1_000_000_000, "namespaces[0].storage-engine.data-size")

	assertInt64(t, ns["nsup-period"], 30, "namespaces[0].nsup-period")

	logging := mustSlice(t, translated["logging"], "logging")
	if len(logging) != 3 {
		t.Fatalf("expected 3 logging sinks, got %d", len(logging))
	}

	consoleSink := mustMap(t, logging[0], "logging[0]")
	assertString(t, consoleSink["name"], "console", "logging[0].name")
	assertString(t, consoleSink["any"], "info", "logging[0].any")
}

func TestToLegacyUnsupportedUnit(t *testing.T) {
	input := `
namespaces:
  test:
    nsup-period:
      value: 1
      unit: bad-unit
`

	_, err := ToLegacy([]byte(input))
	if err == nil {
		t.Fatalf("expected unsupported unit error, got nil")
	}

	if !strings.Contains(err.Error(), "unsupported unit") {
		t.Fatalf("expected unsupported unit error, got: %v", err)
	}
}

func TestToLegacyOverflow(t *testing.T) {
	input := `
namespaces:
  test:
    storage-engine:
      type: memory
      data-size:
        value: 9223372036854775807
        unit: k
`

	_, err := ToLegacy([]byte(input))
	if err == nil {
		t.Fatalf("expected overflow error, got nil")
	}

	if !strings.Contains(err.Error(), "overflows int64") {
		t.Fatalf("expected overflow error, got: %v", err)
	}
}

func TestFromLegacy(t *testing.T) {
	input := `
logging:
  - name: console
    any: info
  - name: /var/log/aerospike.log
    namespace: detail
  - name: syslog
    any: warning
    facility: local0
    path: /dev/log
    tag: asd
network:
  tls:
    - name: tls1
      cert-file: /a.crt
namespaces:
  - name: test
    replication-factor: 2
    sets:
      - name: set1
        enable-index: true
xdr:
  dcs:
    - name: dc1
      namespaces:
        - name: test
          forward: true
`

	out, err := FromLegacy([]byte(input))
	if err != nil {
		t.Fatalf("FromLegacy returned error: %v", err)
	}

	translated := map[string]any{}
	if err := yaml.Unmarshal(out, translated); err != nil {
		t.Fatalf("unable to unmarshal translated yaml: %v", err)
	}

	logging := mustSlice(t, translated["logging"], "logging")
	if len(logging) != 3 {
		t.Fatalf("expected 3 logging sinks, got %d", len(logging))
	}

	consoleSink := mustMap(t, logging[0], "logging[0]")
	assertString(t, consoleSink["type"], "console", "logging[0].type")
	if _, ok := consoleSink["name"]; ok {
		t.Fatalf("expected logging[0].name to be removed")
	}

	consoleContexts := mustMap(t, consoleSink["contexts"], "logging[0].contexts")
	assertString(t, consoleContexts["any"], "info", "logging[0].contexts.any")

	fileSink := mustMap(t, logging[1], "logging[1]")
	assertString(t, fileSink["type"], "file", "logging[1].type")
	assertString(t, fileSink["path"], "/var/log/aerospike.log", "logging[1].path")

	syslogSink := mustMap(t, logging[2], "logging[2]")
	assertString(t, syslogSink["type"], "syslog", "logging[2].type")

	namespaces := mustMap(t, translated["namespaces"], "namespaces")
	ns := mustMap(t, namespaces["test"], "namespaces.test")
	sets := mustMap(t, ns["sets"], "namespaces.test.sets")
	set := mustMap(t, sets["set1"], "namespaces.test.sets.set1")
	if enabled, ok := set["enable-index"].(bool); !ok || !enabled {
		t.Fatalf("expected namespaces.test.sets.set1.enable-index to be true")
	}

	network := mustMap(t, translated["network"], "network")
	tls := mustMap(t, network["tls"], "network.tls")
	tls1 := mustMap(t, tls["tls1"], "network.tls.tls1")
	assertString(t, tls1["cert-file"], "/a.crt", "network.tls.tls1.cert-file")

	xdr := mustMap(t, translated["xdr"], "xdr")
	dcs := mustMap(t, xdr["dcs"], "xdr.dcs")
	dc := mustMap(t, dcs["dc1"], "xdr.dcs.dc1")
	dcNamespaces := mustMap(t, dc["namespaces"], "xdr.dcs.dc1.namespaces")
	dcNS := mustMap(t, dcNamespaces["test"], "xdr.dcs.dc1.namespaces.test")
	if forward, ok := dcNS["forward"].(bool); !ok || !forward {
		t.Fatalf("expected xdr.dcs.dc1.namespaces.test.forward to be true")
	}
}

func TestFromLegacyMapsLegacyFileSink(t *testing.T) {
	out, err := FromLegacy([]byte("logging:\n  - name: /var/log/aerospike/aerospike.log\n    any: info\n"))
	if err != nil {
		t.Fatalf("FromLegacy returned error: %v", err)
	}

	outText := string(out)
	if !strings.Contains(outText, "type: file") {
		t.Fatalf("expected translated output to include file logging type, got:\n%s", outText)
	}

	if !strings.Contains(outText, "path: /var/log/aerospike/aerospike.log") {
		t.Fatalf("expected translated output to include file logging path, got:\n%s", outText)
	}

	if strings.Contains(outText, "type: /var/log/aerospike/aerospike.log") {
		t.Fatalf("expected legacy file path not to be emitted as logging type, got:\n%s", outText)
	}
}

func TestFromLegacyMapsLegacyTypePathSink(t *testing.T) {
	out, err := FromLegacy([]byte("logging:\n  - type: /var/log/aerospike/aerospike.log\n    any: info\n"))
	if err != nil {
		t.Fatalf("FromLegacy returned error: %v", err)
	}

	outText := string(out)
	if !strings.Contains(outText, "type: file") {
		t.Fatalf("expected translated output to include file logging type, got:\n%s", outText)
	}

	if !strings.Contains(outText, "path: /var/log/aerospike/aerospike.log") {
		t.Fatalf("expected translated output to include file logging path, got:\n%s", outText)
	}
}

func TestIsSupportedVersion(t *testing.T) {
	cases := []struct {
		version  string
		expected bool
	}{
		{"", false},
		{"7.2.0", false},
		{"8.0.0", false},
		{"8.1.0", true},
		{"8.1.2", true},
		{"9.0.0", true},
		{"ee-8.1.0", true},
	}

	for _, tc := range cases {
		got, err := IsSupportedVersion(tc.version)
		if err != nil {
			t.Fatalf("IsSupportedVersion(%q) errored: %v", tc.version, err)
		}

		if got != tc.expected {
			t.Fatalf("IsSupportedVersion(%q): expected %v, got %v", tc.version, tc.expected, got)
		}
	}
}

func TestValidateRejectsUnknownTopLevelKey(t *testing.T) {
	verrs, err := Validate([]byte("not-a-real-context: true\n"), "8.1.0")
	if err != nil {
		t.Fatalf("Validate errored: %v", err)
	}

	if len(verrs) == 0 {
		t.Fatalf("expected validation errors for unknown top-level key, got none")
	}
}

func TestValidateRequiresVersion(t *testing.T) {
	_, err := Validate([]byte("service: {}\n"), "")
	if err == nil {
		t.Fatalf("expected missing version error, got nil")
	}
}

// TestLoadSchemaResolvesPerVersion documents how the resolver maps common
// asconfig-style version strings onto the embedded native schemas. It's what
// keeps "asconfig cares about major.minor, not patch" honest as new schemas
// land (e.g. the addition of 8.1.2 in TOOLS-3291).
func TestLoadSchemaResolvesPerVersion(t *testing.T) {
	cases := []struct {
		name             string
		version          string
		expectedResolved string
	}{
		{name: "exact 8.1.1", version: "8.1.1", expectedResolved: "8.1.1"},
		{name: "exact 8.1.2", version: "8.1.2", expectedResolved: "8.1.2"},
		{name: "full 8.1.2.0 resolves to 8.1.2", version: "8.1.2.0", expectedResolved: "8.1.2"},
		{name: "ee prefix strips", version: "ee-8.1.2", expectedResolved: "8.1.2"},
		{name: "8.1.0 falls back to lowest 8.1.x", version: "8.1.0", expectedResolved: "8.1.1"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			body, resolved, err := loadSchemaResolved(tc.version)
			if err != nil {
				t.Fatalf("loadSchemaResolved(%q) errored: %v", tc.version, err)
			}

			if body == "" {
				t.Fatalf("loadSchemaResolved(%q) returned empty body", tc.version)
			}

			if resolved != tc.expectedResolved {
				t.Fatalf("loadSchemaResolved(%q): expected %q, got %q", tc.version, tc.expectedResolved, resolved)
			}
		})
	}
}

func mustMap(t *testing.T, value any, path string) map[string]any {
	t.Helper()

	m, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s expected map[string]any, got %T", path, value)
	}

	return m
}

func mustSlice(t *testing.T, value any, path string) []any {
	t.Helper()

	s, ok := value.([]any)
	if !ok {
		t.Fatalf("%s expected []any, got %T", path, value)
	}

	return s
}

func assertString(t *testing.T, value any, expected, path string) {
	t.Helper()

	s, ok := value.(string)
	if !ok {
		t.Fatalf("%s expected string, got %T", path, value)
	}

	if s != expected {
		t.Fatalf("%s expected %q, got %q", path, expected, s)
	}
}

func assertInt64(t *testing.T, value any, expected int64, path string) {
	t.Helper()

	got, err := asInt64(value)
	if err != nil {
		t.Fatalf("%s expected numeric value, got %T (%v)", path, value, err)
	}

	if got != expected {
		t.Fatalf("%s expected %d, got %d", path, expected, got)
	}
}
