//go:build unit

package cmd

import (
	"bytes"
	"strings"
	"testing"
)

type runTestList struct {
	flags       []string
	arguments   []string
	expectError bool
}

var testListArgs = []runTestList{
	{
		flags:       []string{},
		arguments:   []string{},
		expectError: false, // Should show help
	},
	{
		flags:       []string{},
		arguments:   []string{"versions"},
		expectError: false,
	},
	{
		flags:       []string{"--verbose"},
		arguments:   []string{"versions"},
		expectError: false,
	},
	{
		flags:       []string{"-v"},
		arguments:   []string{"versions"},
		expectError: false,
	},
}

func TestRunEList(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	for i, test := range testListArgs {
		// Create a fresh command instance for each test case
		cmd := newListCmd()

		cmd.ParseFlags(test.flags)
		err := cmd.RunE(cmd, test.arguments)
		if test.expectError == (err == nil) {
			t.Fatalf("case: %d, expectError: %v does not match err: %v", i, test.expectError, err)
		}
	}
}

func TestRunEListVersions(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	type runTestListVersions struct {
		flags       []string
		arguments   []string
		expectError bool
	}

	var testListVersionsArgs = []runTestListVersions{
		{
			flags:       []string{},
			arguments:   []string{},
			expectError: false,
		},
		{
			flags:       []string{"--verbose"},
			arguments:   []string{},
			expectError: false,
		},
		{
			flags:       []string{"-v"},
			arguments:   []string{},
			expectError: false,
		},
	}

	for i, test := range testListVersionsArgs {
		// Create a fresh command instance for each test case
		cmd := newListVersionsCmd()

		cmd.ParseFlags(test.flags)
		err := cmd.RunE(cmd, test.arguments)
		if test.expectError == (err == nil) {
			t.Fatalf("case: %d, expectError: %v does not match err: %v", i, test.expectError, err)
		}
	}
}

func TestListVersionsOutput(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	testCases := []struct {
		name             string
		flags            []string
		expectedVersions []string
		checkFormat      func(string) bool
	}{
		{
			name:  "simple format contains expected versions",
			flags: []string{},
			expectedVersions: []string{
				"4.0.0", "5.0.0", "6.0.0", "7.0.0", "8.0.0", "8.1.0", "8.1.1",
			},
			checkFormat: func(output string) bool {
				// Simple format: versions separated by newlines, no headers
				lines := strings.Split(strings.TrimSpace(output), "\n")
				return len(lines) > 10 && // Should have many versions
					!strings.Contains(output, "Available Aerospike Server Versions:") && // No header
					!strings.Contains(output, "Total:") // No total count
			},
		},
		{
			name:  "verbose format contains expected versions and formatting",
			flags: []string{"--verbose"},
			expectedVersions: []string{
				"4.0.0", "5.0.0", "6.0.0", "7.0.0", "8.0.0", "8.1.0", "8.1.1",
			},
			checkFormat: func(output string) bool {
				// Verbose format: should have header, numbered list, and total
				return strings.Contains(output, "Available Aerospike Server Versions:") &&
					strings.Contains(output, "====================================") &&
					strings.Contains(output, "Total:") &&
					strings.Contains(output, "1. ") && // Numbered list
					strings.Contains(output, "versions")
			},
		},
		{
			name:  "short verbose flag works",
			flags: []string{"-v"},
			expectedVersions: []string{
				"4.0.0", "5.0.0", "6.0.0", "7.0.0", "8.0.0",
			},
			checkFormat: func(output string) bool {
				// Should be same as --verbose
				return strings.Contains(output, "Available Aerospike Server Versions:") &&
					strings.Contains(output, "Total:")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newListVersionsCmd()

			// Set up output capture
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			// Parse flags and run command
			if err := cmd.ParseFlags(tc.flags); err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			err := cmd.RunE(cmd, []string{})
			if err != nil {
				t.Fatalf("Command failed: %v", err)
			}

			output := buf.String()

			// Check that expected versions are present
			for _, version := range tc.expectedVersions {
				if !strings.Contains(output, version) {
					t.Errorf("Expected version %s not found in output", version)
				}
			}

			// Check format
			if tc.checkFormat != nil && !tc.checkFormat(output) {
				t.Errorf("Output format check failed for %s. Output:\n%s", tc.name, output)
			}

			// Verify versions are sorted (check a few key ones)
			if strings.Index(output, "4.0.0") > strings.Index(output, "5.0.0") {
				t.Error("Versions are not properly sorted")
			}
		})
	}
}

func TestListVersionsCount(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	cmd := newListVersionsCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have a reasonable number of versions (at least 20, probably more)
	if len(lines) < 20 {
		t.Errorf("Expected at least 20 versions, got %d", len(lines))
	}

	// Test verbose format shows correct count
	cmd = newListVersionsCmd()
	buf.Reset()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.ParseFlags([]string{"--verbose"})

	err = cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("Verbose command failed: %v", err)
	}

	verboseOutput := buf.String()

	// Extract the total count from verbose output
	if !strings.Contains(verboseOutput, "Total:") {
		t.Error("Verbose output should contain total count")
	}

	// Count numbered lines in verbose output
	numberedLines := 0
	for _, line := range strings.Split(verboseOutput, "\n") {
		if strings.Contains(line, ". ") && len(strings.TrimSpace(line)) > 3 {
			numberedLines++
		}
	}

	if numberedLines != len(lines) {
		t.Errorf(
			"Verbose format numbered lines (%d) should match simple format line count (%d)",
			numberedLines,
			len(lines),
		)
	}
}

func TestListCommandHelp(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	cmd := newListCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Running list without subcommand should show help
	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("List command should not error when showing help: %v", err)
	}

	output := buf.String()

	// Should contain help information
	expectedHelpContent := []string{
		"Usage:",
		"list",
		"Available Commands:",
		"versions",
		"List available Aerospike server versions",
	}

	for _, expected := range expectedHelpContent {
		if !strings.Contains(output, expected) {
			t.Errorf("Help output should contain '%s'. Output:\n%s", expected, output)
		}
	}
}
