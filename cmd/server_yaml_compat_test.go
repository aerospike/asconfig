//go:build unit

package cmd

import (
	"errors"
	"strings"
	"testing"

	asConf "github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func TestTranslateServerYAMLToLegacy(t *testing.T) {
	input := `
service:
  cluster-name: compat-cluster
  nsup-period:
    value: 1
    unit: m
  proto-fd-max:
    value: 2
    unit: m
  ticker-interval:
    value: 2
    unit: h
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
    tlsB:
      cert-file: /b.crt
    tlsA:
      cert-file: /a.crt
logging:
  - type: file
    path: /var/log/aerospike.log
    contexts:
      any: info
      misc: warning
  - type: syslog
    facility: local0
    path: /dev/log
    tag: asd
    contexts:
      any: detail
namespaces:
  nsB:
    replication-factor: 2
    default-ttl:
      value: 1
      unit: d
    sets:
      setB:
        enable-index: true
      setA:
        enable-index: false
    storage-engine:
      type: memory
      data-size:
        value: 4
        unit: g
  nsA:
    replication-factor: 2
    storage-engine:
      type: memory
      data-size: 1073741824
xdr:
  dcs:
    dcB:
      connector: true
      namespaces:
        ns2:
          forward: true
        ns1:
          forward: false
    dcA:
      connector: false
`

	out, err := TranslateServerYAMLToLegacy([]byte(input))
	if err != nil {
		t.Fatalf("TranslateServerYAMLToLegacy returned error: %v", err)
	}

	translated := map[string]any{}
	if err := yaml.Unmarshal(out, translated); err != nil {
		t.Fatalf("unable to unmarshal translated yaml: %v", err)
	}

	namespaces := mustSlice(t, translated["namespaces"], "namespaces")
	if len(namespaces) != 2 {
		t.Fatalf("expected 2 namespaces, got %d", len(namespaces))
	}

	nsA := mustMap(t, namespaces[0], "namespaces[0]")
	nsB := mustMap(t, namespaces[1], "namespaces[1]")
	assertString(t, nsA["name"], "nsA", "namespaces[0].name")
	assertString(t, nsB["name"], "nsB", "namespaces[1].name")

	sets := mustSlice(t, nsB["sets"], "namespaces[1].sets")
	if len(sets) != 2 {
		t.Fatalf("expected 2 sets for nsB, got %d", len(sets))
	}

	setA := mustMap(t, sets[0], "namespaces[1].sets[0]")
	setB := mustMap(t, sets[1], "namespaces[1].sets[1]")
	assertString(t, setA["name"], "setA", "namespaces[1].sets[0].name")
	assertString(t, setB["name"], "setB", "namespaces[1].sets[1].name")

	network := mustMap(t, translated["network"], "network")
	tls := mustSlice(t, network["tls"], "network.tls")
	if len(tls) != 2 {
		t.Fatalf("expected 2 tls entries, got %d", len(tls))
	}

	tlsA := mustMap(t, tls[0], "network.tls[0]")
	tlsB := mustMap(t, tls[1], "network.tls[1]")
	assertString(t, tlsA["name"], "tlsA", "network.tls[0].name")
	assertString(t, tlsB["name"], "tlsB", "network.tls[1].name")

	xdr := mustMap(t, translated["xdr"], "xdr")
	dcs := mustSlice(t, xdr["dcs"], "xdr.dcs")
	if len(dcs) != 2 {
		t.Fatalf("expected 2 xdr dcs, got %d", len(dcs))
	}

	dcA := mustMap(t, dcs[0], "xdr.dcs[0]")
	dcB := mustMap(t, dcs[1], "xdr.dcs[1]")
	assertString(t, dcA["name"], "dcA", "xdr.dcs[0].name")
	assertString(t, dcB["name"], "dcB", "xdr.dcs[1].name")

	dcBNamespaces := mustSlice(t, dcB["namespaces"], "xdr.dcs[1].namespaces")
	if len(dcBNamespaces) != 2 {
		t.Fatalf("expected 2 xdr namespaces for dcB, got %d", len(dcBNamespaces))
	}

	xdrNS1 := mustMap(t, dcBNamespaces[0], "xdr.dcs[1].namespaces[0]")
	xdrNS2 := mustMap(t, dcBNamespaces[1], "xdr.dcs[1].namespaces[1]")
	assertString(t, xdrNS1["name"], "ns1", "xdr.dcs[1].namespaces[0].name")
	assertString(t, xdrNS2["name"], "ns2", "xdr.dcs[1].namespaces[1].name")

	logging := mustSlice(t, translated["logging"], "logging")
	if len(logging) != 2 {
		t.Fatalf("expected 2 logging sinks, got %d", len(logging))
	}

	fileLogSink := mustMap(t, logging[0], "logging[0]")
	assertString(t, fileLogSink["name"], "/var/log/aerospike.log", "logging[0].name")
	if _, ok := fileLogSink["type"]; ok {
		t.Fatalf("expected logging[0].type to be removed")
	}
	if _, ok := fileLogSink["contexts"]; ok {
		t.Fatalf("expected logging[0].contexts to be flattened")
	}
	if _, ok := fileLogSink["path"]; ok {
		t.Fatalf("expected logging[0].path to be folded into logging[0].name")
	}
	assertString(t, fileLogSink["any"], "info", "logging[0].any")
	assertString(t, fileLogSink["misc"], "warning", "logging[0].misc")

	syslogSink := mustMap(t, logging[1], "logging[1]")
	assertString(t, syslogSink["name"], "syslog", "logging[1].name")
	assertString(t, syslogSink["facility"], "local0", "logging[1].facility")
	assertString(t, syslogSink["path"], "/dev/log", "logging[1].path")
	assertString(t, syslogSink["tag"], "asd", "logging[1].tag")
	assertString(t, syslogSink["any"], "detail", "logging[1].any")

	service := mustMap(t, translated["service"], "service")
	assertInt64(t, service["nsup-period"], 60, "service.nsup-period")
	assertInt64(t, service["proto-fd-max"], 2_000_000, "service.proto-fd-max")
	assertInt64(t, service["ticker-interval"], 7_200, "service.ticker-interval")

	assertInt64(t, nsB["default-ttl"], 86_400, "namespaces.nsB.default-ttl")
	nsBStorage := mustMap(t, nsB["storage-engine"], "namespaces.nsB.storage-engine")
	assertInt64(t, nsBStorage["data-size"], 4_000_000_000, "namespaces.nsB.storage-engine.data-size")
}

func TestTranslateServerYAMLToLegacyInvalidUnit(t *testing.T) {
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
namespaces:
  test:
    replication-factor: 2
    nsup-period:
      value: 1
      unit: bad-unit
    storage-engine:
      type: memory
      data-size: 1073741824
`

	_, err := TranslateServerYAMLToLegacy([]byte(input))
	if err == nil {
		t.Fatalf("expected invalid unit error, got nil")
	}

	if !strings.Contains(err.Error(), "unsupported unit") {
		t.Fatalf("expected unsupported unit error, got: %v", err)
	}
}

func TestTranslateServerYAMLToLegacyOverflow(t *testing.T) {
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
namespaces:
  test:
    replication-factor: 2
    storage-engine:
      type: memory
      data-size:
        value: 9223372036854775807
        unit: k
`

	_, err := TranslateServerYAMLToLegacy([]byte(input))
	if err == nil {
		t.Fatalf("expected overflow error, got nil")
	}

	if !strings.Contains(err.Error(), "overflows int64") {
		t.Fatalf("expected overflow error, got: %v", err)
	}
}

func TestTranslateLegacyYAMLToServerYAML(t *testing.T) {
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

	out, err := TranslateLegacyYAMLToServerYAML([]byte(input))
	if err != nil {
		t.Fatalf("TranslateLegacyYAMLToServerYAML returned error: %v", err)
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
	fileContexts := mustMap(t, fileSink["contexts"], "logging[1].contexts")
	assertString(t, fileContexts["namespace"], "detail", "logging[1].contexts.namespace")

	syslogSink := mustMap(t, logging[2], "logging[2]")
	assertString(t, syslogSink["type"], "syslog", "logging[2].type")
	assertString(t, syslogSink["facility"], "local0", "logging[2].facility")
	assertString(t, syslogSink["path"], "/dev/log", "logging[2].path")
	assertString(t, syslogSink["tag"], "asd", "logging[2].tag")
	syslogContexts := mustMap(t, syslogSink["contexts"], "logging[2].contexts")
	assertString(t, syslogContexts["any"], "warning", "logging[2].contexts.any")

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

func TestMaybeTranslateServerYAMLOutputGuards(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool(flagServerYAMLOutput, false, "")
	if err := cmd.ParseFlags([]string{"--server-yaml-output"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	_, err := maybeTranslateServerYAMLOutput(cmd, asConf.AeroConfig, "8.1.1", []byte("logging: []"))
	if !errors.Is(err, errServerYAMLOutputRequiresYAML) {
		t.Fatalf("expected YAML output guard error, got: %v", err)
	}

	_, err = maybeTranslateServerYAMLOutput(cmd, asConf.YAML, "8.1.0", []byte("logging: []"))
	if !errors.Is(err, errServerYAMLOutputUnsupportedVersion) {
		t.Fatalf("expected version guard error, got: %v", err)
	}

	out, err := maybeTranslateServerYAMLOutput(
		cmd,
		asConf.YAML,
		"8.1.1",
		[]byte("logging:\n  - name: console\n    any: info\n"),
	)
	if err != nil {
		t.Fatalf("expected successful output translation, got: %v", err)
	}

	if !strings.Contains(string(out), "type: console") {
		t.Fatalf("expected translated output to include server logging type, got:\n%s", string(out))
	}
}

func TestMaybeTranslateServerYAMLOutputMapsLegacyFileSink(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool(flagServerYAMLOutput, false, "")
	if err := cmd.ParseFlags([]string{"--server-yaml-output"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	out, err := maybeTranslateServerYAMLOutput(
		cmd,
		asConf.YAML,
		"8.1.1",
		[]byte("logging:\n  - name: /var/log/aerospike/aerospike.log\n    any: info\n"),
	)
	if err != nil {
		t.Fatalf("expected successful output translation, got: %v", err)
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

func TestMaybeTranslateServerYAMLOutputMapsLegacyTypePathSink(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool(flagServerYAMLOutput, false, "")
	if err := cmd.ParseFlags([]string{"--server-yaml-output"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	out, err := maybeTranslateServerYAMLOutput(
		cmd,
		asConf.YAML,
		"8.1.1",
		[]byte("logging:\n  - type: /var/log/aerospike/aerospike.log\n    any: info\n"),
	)
	if err != nil {
		t.Fatalf("expected successful output translation, got: %v", err)
	}

	outText := string(out)
	if !strings.Contains(outText, "type: file") {
		t.Fatalf("expected translated output to include file logging type, got:\n%s", outText)
	}

	if !strings.Contains(outText, "path: /var/log/aerospike/aerospike.log") {
		t.Fatalf("expected translated output to include file logging path, got:\n%s", outText)
	}

	if strings.Contains(outText, "type: /var/log/aerospike/aerospike.log") {
		t.Fatalf("expected legacy type path not to be emitted as logging type, got:\n%s", outText)
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
