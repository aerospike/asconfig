//go:build unit

package cmd

import (
	"os"
	"testing"
)

type runTestValidate struct {
	flags       []string
	arguments   []string
	expectError bool
}

var testValidateArgs = []runTestValidate{
	{
		flags:       []string{},
		arguments:   []string{"too", "many", "args"},
		expectError: true,
	},
	{
		// missing arg to -a-aerospike-version
		flags:       []string{"--aerospike-version"},
		arguments:   []string{"../testdata/sources/all_flash_cluster_cr.yaml"},
		expectError: true,
	},
	{
		flags:       []string{"--aerospike-version"},
		arguments:   []string{"./bad_extension.ymml"},
		expectError: true,
	},
	{
		flags:       []string{"--aerospike-version", "bad_version"},
		arguments:   []string{"../testdata/sources/all_flash_cluster_cr.yaml"},
		expectError: true,
	},
	{
		flags:       []string{"--aerospike-version", "6.4.0"},
		arguments:   []string{"./fake_file.yaml"},
		expectError: true,
	},
	{
		flags:       []string{"--aerospike-version", "6.4.0"},
		arguments:   []string{"../testdata/cases/server64/server64.yaml"},
		expectError: false,
	},
	{
		flags:       []string{"--aerospike-version", "7.0.0"},
		arguments:   []string{"../testdata/cases/server70/server70.yaml"},
		expectError: false,
	},
	{
		flags:       []string{"--format", "bad_fmt"},
		arguments:   []string{},
		expectError: true,
	},
	{
		flags:       []string{"-F", "bad_fmt"},
		arguments:   []string{},
		expectError: true,
	},
}

func TestRunEValidate(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	for i, test := range testValidateArgs {
		// Create a fresh command instance for each test case
		cmd := newValidateCmd()

		cmd.ParseFlags(test.flags)
		err := cmd.RunE(cmd, test.arguments)
		if test.expectError == (err == nil) {
			t.Fatalf("case: %d, expectError: %v does not match err: %v", i, test.expectError, err)
		}
	}
}

func TestRunEValidateServerYAMLCompat(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	serverYAML := `
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
logging:
  - type: console
    contexts:
      any: info
namespaces:
  test:
    replication-factor: 2
    storage-engine:
      type: memory
      data-size:
        value: 4
        unit: g
`

	tmpFile, err := os.CreateTemp("", "asconfig-server-yaml-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(serverYAML); err != nil {
		t.Fatalf("failed to write temp yaml file: %v", err)
	}

	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp yaml file: %v", err)
	}

	cmd := newValidateCmd()
	cmd.ParseFlags([]string{
		"--aerospike-version", "8.1.0",
		"--format", "yaml",
		"--server-yaml",
	})

	if err := cmd.RunE(cmd, []string{tmpFile.Name()}); err != nil {
		t.Fatalf("expected translated server yaml to validate, got error: %v", err)
	}
}
