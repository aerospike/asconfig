//go:build unit

package cmd

import (
	"testing"
)

type runTestDiff struct {
	flags       []string
	arguments   []string
	expectError bool
}

var testDiffArgs = []runTestDiff{
	{
		flags:       []string{},
		arguments:   []string{"not_enough_args"},
		expectError: true,
	},
	{
		flags:       []string{},
		arguments:   []string{"too", "many", "args"},
		expectError: true,
	},
	{
		flags:       []string{},
		arguments:   []string{"./bad_extension.ymml", "./bad_extension.ymml"},
		expectError: true,
	},
	{
		flags:       []string{},
		arguments:   []string{"./mismatched_extension.conf", "./mismatched_extension.yml"},
		expectError: true,
	},
	{
		flags:       []string{},
		arguments:   []string{"not_enough_args"},
		expectError: true,
	},
	{
		flags:       []string{},
		arguments:   []string{"../testdata/sources/all_flash_cluster_cr.yaml", "../testdata/sources/podspec_cr.yaml"},
		expectError: true,
	},
	{
		flags:       []string{},
		arguments:   []string{"../testdata/expected/all_flash_cluster_cr.conf", "../testdata/expected/podspec_cr.conf"},
		expectError: true,
	},
	{
		flags:       []string{},
		arguments:   []string{"../testdata/sources/all_flash_cluster_cr.yaml", "../testdata/sources/all_flash_cluster_cr.yaml"},
		expectError: false,
	},
	{
		flags:       []string{"--log-level", "debug"},
		arguments:   []string{"../testdata/expected/all_flash_cluster_cr.conf", "../testdata/expected/all_flash_cluster_cr.conf"},
		expectError: false,
	},
	{
		flags:       []string{"--log-level", "debug"},
		arguments:   []string{"../testdata/expected/all_flash_cluster_cr.conf", "../testdata/expected/all_flash_cluster_cr_info_cap.conf"},
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

// Test cases specifically for server diff functionality
var testServerDiffArgs = []runTestDiff{
	{
		flags:       []string{"--server"},
		arguments:   []string{}, // no arguments
		expectError: true,
	},
	{
		flags:       []string{"--server"},
		arguments:   []string{"file1.yaml", "file2.yaml"}, // too many arguments
		expectError: true,
	},
	{
		flags:       []string{"--server", "--format", "bad_fmt"},
		arguments:   []string{"file1.yaml"},
		expectError: true,
	},
	{
		flags:       []string{"-s", "-F", "bad_fmt"},
		arguments:   []string{"file1.yaml"},
		expectError: true,
	},
	{
		flags:       []string{"--server"},
		arguments:   []string{"./bad_extension.ymml"},
		expectError: true,
	},
	{
		flags:       []string{"--server"},
		arguments:   []string{"../testdata/sources/all_flash_cluster_cr.yaml"},
		expectError: true, // will fail due to no server connection in unit tests
	},
	{
		flags:       []string{"-s", "--log-level", "debug"},
		arguments:   []string{"../testdata/expected/all_flash_cluster_cr.conf"},
		expectError: true, // will fail due to no server connection in unit tests
	},
}

func TestRunEDiff(t *testing.T) {
	cmd := diffCmd

	for i, test := range testDiffArgs {
		cmd.ParseFlags(test.flags)
		err := cmd.RunE(cmd, test.arguments)
		if test.expectError == (err == nil) {
			t.Fatalf("case: %d, expectError: %v does not match err: %v", i, test.expectError, err)
		}
	}
}

func TestRunEServerDiff(t *testing.T) {
	cmd := newDiffCmd()

	for i, test := range testServerDiffArgs {
		// Reset flags for each test
		cmd = newDiffCmd()
		cmd.ParseFlags(test.flags)
		err := cmd.RunE(cmd, test.arguments)
		if test.expectError == (err == nil) {
			t.Fatalf("server diff case: %d, expectError: %v does not match err: %v", i, test.expectError, err)
		}
	}
}

func TestRunFileDiff(t *testing.T) {
	cmd := newDiffCmd()

	// Test valid file diff cases
	validCases := []struct {
		args        []string
		expectError bool
		description string
	}{
		{
			args:        []string{"../testdata/sources/all_flash_cluster_cr.yaml", "../testdata/sources/all_flash_cluster_cr.yaml"},
			expectError: false,
			description: "identical YAML files",
		},
		{
			args:        []string{"file1.yaml", "file2.yaml"},
			expectError: true,
			description: "non-existent files",
		},
		{
			args:        []string{"file1.yaml"},
			expectError: true,
			description: "insufficient arguments",
		},
		{
			args:        []string{"file1.yaml", "file2.yaml", "file3.yaml"},
			expectError: true,
			description: "too many arguments",
		},
	}

	for i, test := range validCases {
		err := runFileDiff(cmd, test.args)
		if test.expectError == (err == nil) {
			t.Fatalf("runFileDiff case %d (%s): expectError: %v does not match err: %v", i, test.description, test.expectError, err)
		}
	}
}

func TestRunServerDiff(t *testing.T) {
	cmd := newDiffCmd()

	// Test server diff argument validation
	validationCases := []struct {
		args        []string
		expectError bool
		description string
	}{
		{
			args:        []string{"../testdata/sources/all_flash_cluster_cr.yaml"},
			expectError: true, // will fail due to no server connection
			description: "valid single file argument",
		},
		{
			args:        []string{},
			expectError: true,
			description: "no arguments",
		},
		{
			args:        []string{"file1.yaml", "file2.yaml"},
			expectError: true,
			description: "too many arguments",
		},
		{
			args:        []string{"non_existent_file.yaml"},
			expectError: true,
			description: "non-existent file",
		},
	}

	for i, test := range validationCases {
		err := runServerDiff(cmd, test.args)
		if test.expectError == (err == nil) {
			t.Fatalf("runServerDiff case %d (%s): expectError: %v does not match err: %v", i, test.description, test.expectError, err)
		}
	}
}

func TestDiffFlagValidation(t *testing.T) {
	cmd := newDiffCmd()

	// Test that server flag is properly recognized
	cmd.ParseFlags([]string{"--server"})
	isServerMode, err := cmd.Flags().GetBool("server")
	if err != nil {
		t.Fatalf("Failed to get server flag: %v", err)
	}
	if !isServerMode {
		t.Fatalf("Server flag should be true when set")
	}

	// Test short flag
	cmd = newDiffCmd()
	cmd.ParseFlags([]string{"-s"})
	isServerMode, err = cmd.Flags().GetBool("server")
	if err != nil {
		t.Fatalf("Failed to get server flag with short option: %v", err)
	}
	if !isServerMode {
		t.Fatalf("Server flag should be true when set with short option")
	}

	// Test default value
	cmd = newDiffCmd()
	cmd.ParseFlags([]string{})
	isServerMode, err = cmd.Flags().GetBool("server")
	if err != nil {
		t.Fatalf("Failed to get default server flag: %v", err)
	}
	if isServerMode {
		t.Fatalf("Server flag should be false by default")
	}
}
