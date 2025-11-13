//go:build unit

package cmd

import (
	"bytes"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/aerospike/asconfig/schema"
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

	// Get available versions dynamically from schema
	schemaMap, err := schema.NewSchemaMap()
	if err != nil {
		t.Fatalf("Failed to load schema map: %v", err)
	}

	var availableVersions []string
	for version := range schemaMap {
		availableVersions = append(availableVersions, version)
	}

	if len(availableVersions) < 5 {
		t.Fatalf("Expected at least 5 versions, got %d", len(availableVersions))
	}

	// Sort to get consistent test versions
	sort.Strings(availableVersions)

	// Take a sample of versions for testing (first few and last few)
	var testVersions []string
	if len(availableVersions) >= 5 {
		testVersions = append(testVersions, availableVersions[0])                        // First version
		testVersions = append(testVersions, availableVersions[1])                        // Second version
		testVersions = append(testVersions, availableVersions[len(availableVersions)/2]) // Middle version
		testVersions = append(testVersions, availableVersions[len(availableVersions)-2]) // Second to last
		testVersions = append(testVersions, availableVersions[len(availableVersions)-1]) // Last version
	} else {
		testVersions = availableVersions
	}

	// Also ensure stable versions are included if they exist
	stableVersions := []string{"7.0.0", "8.0.0", "8.1.0"}
	for _, stableVersion := range stableVersions {
		found := false
		for _, available := range availableVersions {
			if available == stableVersion {
				found = true
				break
			}
		}
		if found {
			// Add to test versions if not already included
			alreadyIncluded := false
			for _, test := range testVersions {
				if test == stableVersion {
					alreadyIncluded = true
					break
				}
			}
			if !alreadyIncluded {
				testVersions = append(testVersions, stableVersion)
			}
		}
	}

	testCases := []struct {
		name             string
		flags            []string
		expectedVersions []string
		checkFormat      func(string) bool
	}{
		{
			name:             "simple format contains expected versions",
			flags:            []string{},
			expectedVersions: testVersions,
			checkFormat: func(output string) bool {
				// Simple format: versions separated by newlines, no headers
				lines := strings.Split(strings.TrimSpace(output), "\n")
				return len(lines) >= 5 && // Should have at least 5 versions
					!strings.Contains(output, "Available Aerospike Server Versions:") && // No header
					!strings.Contains(output, "Total:") // No total count
			},
		},
		{
			name:             "verbose format contains expected versions and formatting",
			flags:            []string{"--verbose"},
			expectedVersions: testVersions,
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
			name:             "short verbose flag works",
			flags:            []string{"-v"},
			expectedVersions: testVersions[:3], // Just test first 3 for this case
			checkFormat: func(output string) bool {
				// Should be same as --verbose
				return strings.Contains(output, "Available Aerospike Server Versions:") &&
					strings.Contains(output, "Total:")
			},
		},
		{
			name:             "stable versions must be present",
			flags:            []string{},
			expectedVersions: []string{"7.0.0", "8.0.0", "8.1.0"}, // These stable versions must exist
			checkFormat: func(output string) bool {
				// Simple format check
				return !strings.Contains(output, "Available Aerospike Server Versions:")
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

			// Verify versions are sorted (check first two available versions)
			if len(testVersions) >= 2 {
				firstPos := strings.Index(output, testVersions[0])
				secondPos := strings.Index(output, testVersions[1])
				if firstPos > secondPos && firstPos != -1 && secondPos != -1 {
					t.Error("Versions are not properly sorted")
				}
			}
		})
	}
}

func TestListVersionsCount(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	// Get expected count from schema
	schemaMap, err := schema.NewSchemaMap()
	if err != nil {
		t.Fatalf("Failed to load schema map: %v", err)
	}
	expectedCount := len(schemaMap)

	cmd := newListVersionsCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err = cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should match the expected count from schema
	if len(lines) != expectedCount {
		t.Errorf("Expected %d versions, got %d", expectedCount, len(lines))
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

	// Count numbered lines in verbose output using a more specific pattern
	// Match lines that start with optional whitespace, digits, period, and space
	numberedLinePattern := regexp.MustCompile(`^\s*\d+\.\s+`)
	numberedLines := 0
	for _, line := range strings.Split(verboseOutput, "\n") {
		if numberedLinePattern.MatchString(line) {
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
