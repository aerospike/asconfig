//go:build integration
// +build integration

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type testFile struct {
	source      string
	destination string
	expected    string
	arguments   []string
}

const (
	binaryPath      = "testdata/asconfig"
	sourcePath      = "testdata/sources"
	expectedPath    = "testdata/expected"
	destinationPath = "testdata/destinations"
)

func TestMain(m *testing.M) {
	if _, err := os.Stat(destinationPath); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(destinationPath, 0777)
		if err != nil {
			panic(err)
		}
	}

	code := m.Run()

	err := os.RemoveAll(destinationPath)
	if err != nil {
		panic(err)
	}

	os.Exit(code)
}

var testFiles = []testFile{
	{
		source:      filepath.Join(sourcePath, "all_flash_cluster_cr.yaml"),
		destination: filepath.Join(destinationPath, "all_flash_cluster_cr.conf"),
		expected:    filepath.Join(expectedPath, "all_flash_cluster_cr.conf"),
		arguments:   []string{binaryPath, "convert", "-a", "6.2.0.2"},
	},
	{
		source:      filepath.Join(sourcePath, "dim_nostorage_cluster_cr.yaml"),
		destination: filepath.Join(destinationPath, "dim_nostorage_cluster_cr.conf"),
		expected:    filepath.Join(expectedPath, "dim_nostorage_cluster_cr.conf"),
		arguments:   []string{binaryPath, "convert", "-a", "6.2.0.2"},
	},
	{
		source:      filepath.Join(sourcePath, "hdd_dii_storage_cluster_cr.yaml"),
		destination: filepath.Join(destinationPath, "hdd_dii_storage_cluster_cr.conf"),
		expected:    filepath.Join(expectedPath, "hdd_dii_storage_cluster_cr.conf"),
		arguments:   []string{binaryPath, "convert", "-a", "6.2.0.2"},
	},
	{
		source:      filepath.Join(sourcePath, "hdd_dim_storage_cluster_cr.yaml"),
		destination: filepath.Join(destinationPath, "hdd_dim_storage_cluster_cr.conf"),
		expected:    filepath.Join(expectedPath, "hdd_dim_storage_cluster_cr.conf"),
		arguments:   []string{binaryPath, "convert", "-a", "6.2.0.2"},
	},
	{
		source:      filepath.Join(sourcePath, "host_network_cluster_cr.yaml"),
		destination: filepath.Join(destinationPath, "host_network_cluster_cr.conf"),
		expected:    filepath.Join(expectedPath, "host_network_cluster_cr.conf"),
		arguments:   []string{binaryPath, "convert", "-a", "6.2.0.2"},
	},
	{
		source:      filepath.Join(sourcePath, "ldap_cluster_cr.yaml"),
		destination: filepath.Join(destinationPath, "ldap_cluster_cr.conf"),
		expected:    filepath.Join(expectedPath, "ldap_cluster_cr.conf"),
		arguments:   []string{binaryPath, "convert", "-a", "6.2.0.2"},
	},
	{
		source:      filepath.Join(sourcePath, "pmem_cluster_cr.yaml"),
		destination: filepath.Join(destinationPath, "pmem_cluster_cr.conf"),
		expected:    filepath.Join(expectedPath, "pmem_cluster_cr.conf"),
		arguments:   []string{binaryPath, "convert", "-a", "6.2.0.2"},
	},
	{
		source:      filepath.Join(sourcePath, "podspec_cr.yaml"),
		destination: filepath.Join(destinationPath, "podspec_cr.conf"),
		expected:    filepath.Join(expectedPath, "podspec_cr.conf"),
		arguments:   []string{binaryPath, "convert", "-a", "6.2.0.2"},
	},
	{
		source:      filepath.Join(sourcePath, "rack_enabled_cluster_cr.yaml"),
		destination: filepath.Join(destinationPath, "rack_enabled_cluster_cr.conf"),
		expected:    filepath.Join(expectedPath, "rack_enabled_cluster_cr.conf"),
		arguments:   []string{binaryPath, "convert", "-a", "6.2.0.2"},
	},
	{
		source:      filepath.Join(sourcePath, "sc_mode_cluster_cr.yaml"),
		destination: filepath.Join(destinationPath, "sc_mode_cluster_cr.conf"),
		expected:    filepath.Join(expectedPath, "sc_mode_cluster_cr.conf"),
		arguments:   []string{binaryPath, "convert", "-a", "6.2.0.2"},
	},
	{
		source:      filepath.Join(sourcePath, "shadow_device_cluster_cr.yaml"),
		destination: filepath.Join(destinationPath, "shadow_device_cluster_cr.conf"),
		expected:    filepath.Join(expectedPath, "shadow_device_cluster_cr.conf"),
		arguments:   []string{binaryPath, "convert", "-a", "6.2.0.2"},
	},
	{
		source:      filepath.Join(sourcePath, "shadow_file_cluster_cr.yaml"),
		destination: filepath.Join(destinationPath, "shadow_file_cluster_cr.conf"),
		expected:    filepath.Join(expectedPath, "shadow_file_cluster_cr.conf"),
		arguments:   []string{binaryPath, "convert", "-a", "6.2.0.2"},
	},
	{
		source:      filepath.Join(sourcePath, "ssd_storage_cluster_cr.yaml"),
		destination: filepath.Join(destinationPath, "ssd_storage_cluster_cr.conf"),
		expected:    filepath.Join(expectedPath, "ssd_storage_cluster_cr.conf"),
		arguments:   []string{binaryPath, "convert", "-a", "6.2.0.2"},
	},
	{
		source:      filepath.Join(sourcePath, "tls_cluster_cr.yaml"),
		destination: filepath.Join(destinationPath, "tls_cluster_cr.conf"),
		expected:    filepath.Join(expectedPath, "tls_cluster_cr.conf"),
		arguments:   []string{binaryPath, "convert", "-a", "6.2.0.2"},
	},
	{
		source:      filepath.Join(sourcePath, "xdr_dst_cluster_cr.yaml"),
		destination: filepath.Join(destinationPath, "xdr_dst_cluster_cr.conf"),
		expected:    filepath.Join(expectedPath, "xdr_dst_cluster_cr.conf"),
		arguments:   []string{binaryPath, "convert", "-a", "6.2.0.2"},
	},
	{
		source:      filepath.Join(sourcePath, "xdr_src_cluster_cr.yaml"),
		destination: filepath.Join(destinationPath, "xdr_src_cluster_cr.conf"),
		expected:    filepath.Join(expectedPath, "xdr_src_cluster_cr.conf"),
		arguments:   []string{binaryPath, "convert", "-a", "5.0.0.0"},
	},
	{
		source:      filepath.Join(sourcePath, "missing_heartbeat_mode.yaml"),
		destination: filepath.Join(destinationPath, "missing_heartbeat_mode.conf"),
		expected:    filepath.Join(expectedPath, "missing_heartbeat_mode.conf"),
		arguments:   []string{binaryPath, "convert", "-a", "6.2.0.2", "-f"},
	},
}

func TestRootCommand(t *testing.T) {
	for _, tf := range testFiles {
		var err error

		arguments := append(tf.arguments, tf.source, tf.destination)
		oldArgs := os.Args[:]
		os.Args = arguments
		main()
		os.Args = oldArgs

		expected, err := os.ReadFile(tf.expected)
		if err != nil {
			t.Error(err)
		}

		actual, err := os.ReadFile(tf.destination)
		if err != nil {
			t.Error(err)
		}

		if string(actual) != string(expected) {
			t.Errorf("\nACTUAL:\n%s\nEXPECTED:\n%s", actual, expected)
		}
	}
}
