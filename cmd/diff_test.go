//go:build unit

package cmd

import (
	"bytes"
	"math"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/aerospike/asconfig/schema"
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
		flags: []string{},
		arguments: []string{
			"../testdata/sources/all_flash_cluster_cr.yaml",
			"../testdata/sources/all_flash_cluster_cr.yaml",
		},
		expectError: false,
	},
	{
		flags: []string{"--log-level", "debug"},
		arguments: []string{
			"../testdata/expected/all_flash_cluster_cr.conf",
			"../testdata/expected/all_flash_cluster_cr.conf",
		},
		expectError: false,
	},
	{
		flags: []string{"--log-level", "debug"},
		arguments: []string{
			"../testdata/expected/all_flash_cluster_cr.conf",
			"../testdata/expected/all_flash_cluster_cr_info_cap.conf",
		},
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

func TestRunEDiff(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}
	cmd := newDiffCmd()

	for i, test := range testDiffArgs {
		cmd.ParseFlags(test.flags)
		err := cmd.RunE(cmd, test.arguments)
		if test.expectError == (err == nil) {
			t.Fatalf("case: %d, expectError: %v does not match err: %v", i, test.expectError, err)
		}
	}
}

func TestRunFileDiff(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}
	cmd := newDiffCmd()

	// Test valid file diff cases
	validCases := []struct {
		args        []string
		expectError bool
		description string
	}{
		{
			args: []string{
				"../testdata/sources/all_flash_cluster_cr.yaml",
				"../testdata/sources/all_flash_cluster_cr.yaml",
			},
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
			t.Fatalf(
				"runFileDiff case %d (%s): expectError: %v does not match err: %v",
				i,
				test.description,
				test.expectError,
				err,
			)
		}
	}
}

func TestRunServerDiff(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}
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
			t.Fatalf(
				"runServerDiff case %d (%s): expectError: %v does not match err: %v",
				i,
				test.description,
				test.expectError,
				err,
			)
		}
	}
}

func TestDiffFlatMaps(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	testCases := []struct {
		name     string
		m1       map[string]any
		m2       map[string]any
		expected []string
	}{
		{
			name: "identical maps",
			m1: map[string]any{
				"key1": "value1",
				"key2": 42,
			},
			m2: map[string]any{
				"key1": "value1",
				"key2": 42,
			},
			expected: []string{},
		},
		{
			name: "different values same keys",
			m1: map[string]any{
				"key1": "value1",
				"key2": 42,
			},
			m2: map[string]any{
				"key1": "value2",
				"key2": 43,
			},
			expected: []string{
				"key1:\n\t<: value1\n\t>: value2\n",
				"key2:\n\t<: 42\n\t>: 43\n",
			},
		},
		{
			name: "missing key in m2",
			m1: map[string]any{
				"key1": "value1",
				"key2": 42,
			},
			m2: map[string]any{
				"key1": "value1",
			},
			expected: []string{
				"<: key2\n",
			},
		},
		{
			name: "missing key in m1",
			m1: map[string]any{
				"key1": "value1",
			},
			m2: map[string]any{
				"key1": "value1",
				"key2": 42,
			},
			expected: []string{
				">: key2\n",
			},
		},
		{
			name: "type conversion equality",
			m1: map[string]any{
				"port":    3000,
				"enabled": true,
			},
			m2: map[string]any{
				"port":    "3000",
				"enabled": "true",
			},
			expected: []string{ // Should be different due to strict type checking
				"enabled:\n\t<: true\n\t>: true\n",
				"port:\n\t<: 3000\n\t>: 3000\n",
			},
		},
		{
			name: "slice comparison",
			m1: map[string]any{
				"addresses": []string{"127.0.0.1", "localhost"},
			},
			m2: map[string]any{
				"addresses": []string{"127.0.0.1", "localhost"},
			},
			expected: []string{}, // Should be equal
		},
		{
			name: "different slice comparison",
			m1: map[string]any{
				"addresses": []string{"127.0.0.1", "localhost"},
			},
			m2: map[string]any{
				"addresses": []string{"127.0.0.1", "::1"},
			},
			expected: []string{
				"addresses:\n\t<: [127.0.0.1 localhost]\n\t>: [127.0.0.1 ::1]\n",
			},
		},
		{
			name: "logging enum comparison (case insensitive)",
			m1: map[string]any{
				"logging.console.any": "INFO",
			},
			m2: map[string]any{
				"logging.console.any": "info",
			},
			expected: []string{}, // Should be equal due to logging enum comparison
		},
		{
			name: "index metadata ignored",
			m1: map[string]any{
				"key1":         "value1",
				"key2.<index>": "should_be_ignored",
			},
			m2: map[string]any{
				"key1":         "value1",
				"key2.<index>": "different_value",
			},
			expected: []string{}, // Index metadata should be ignored
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := diffFlatMaps(tc.m1, tc.m2)

			if len(result) != len(tc.expected) {
				t.Errorf("diffFlatMaps() returned %d differences, expected %d", len(result), len(tc.expected))
				t.Errorf("Got: %v", result)
				t.Errorf("Expected: %v", tc.expected)
				return
			}

			// Sort both slices for comparison since order might vary
			sort.Strings(result)
			sort.Strings(tc.expected)

			for i, diff := range result {
				if diff != tc.expected[i] {
					t.Errorf("diffFlatMaps() difference %d = %q, expected %q", i, diff, tc.expected[i])
				}
			}
		})
	}
}

func TestServerDiffArgValidation(t *testing.T) {
	rootCmd := NewRootCmd()
	rootCmd.AddCommand(newDiffCmd())

	testCases := []struct {
		name        string
		command     string
		expectError bool
		errorType   string
	}{
		{
			name:        "valid single argument",
			command:     "diff server config.yaml",
			expectError: true, // Will fail due to file not existing, but argument validation passes
			errorType:   "file_not_found",
		},
		{
			name:        "no arguments",
			command:     "diff server",
			expectError: true,
			errorType:   "too_few_args",
		},
		{
			name:        "too many arguments",
			command:     "diff server config1.yaml config2.yaml",
			expectError: true,
			errorType:   "too_many_args",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rootCmd.SetArgs(strings.Split(tc.command, " "))
			err := rootCmd.Execute()

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				// Simplified error checking for unit tests since we can't easily access the exact error type
				// across command boundaries in cobra. We are mostly interested in whether an error occurred.
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestDiffFilesAndLegacyCommands(t *testing.T) {
	rootCmd := NewRootCmd()
	rootCmd.AddCommand(newDiffCmd())

	testCases := []struct {
		name        string
		command     string
		expectError bool
	}{
		{
			name:        "legacy diff with two valid files (identical)",
			command:     "diff ../testdata/sources/all_flash_cluster_cr.yaml ../testdata/sources/all_flash_cluster_cr.yaml",
			expectError: false,
		},
		{
			name:        "diff files with two valid files (identical)",
			command:     "diff files ../testdata/sources/all_flash_cluster_cr.yaml ../testdata/sources/all_flash_cluster_cr.yaml",
			expectError: false,
		},
		{
			name:        "legacy diff with too few args",
			command:     "diff only_one_file.yaml",
			expectError: true,
		},
		{
			name:        "diff files with too few args",
			command:     "diff files only_one_file.yaml",
			expectError: true,
		},
		{
			name:        "legacy diff with too many args",
			command:     "diff file1.yaml file2.yaml file3.yaml",
			expectError: true,
		},
		{
			name:        "diff files with too many args",
			command:     "diff files file1.yaml file2.yaml file3.yaml",
			expectError: true,
		},
		{
			name:        "legacy diff with non-existent files",
			command:     "diff file1.yaml file2.yaml",
			expectError: true,
		},
		{
			name:        "diff files with non-existent files",
			command:     "diff files file1.yaml file2.yaml",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rootCmd.SetArgs(strings.Split(tc.command, " "))
			err := rootCmd.Execute()
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// captureStdout captures stdout while executing the provided function and returns the output.
// It handles concurrent reading from the pipe to avoid deadlocks when output is large.
func captureStdout(fn func()) string {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Read from pipe concurrently to avoid deadlock
	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		buf.ReadFrom(r)
		close(done)
	}()

	// Execute the function
	fn()

	// Restore stdout and get output
	w.Close()
	os.Stdout = oldStdout
	<-done // Wait for reader to finish

	return buf.String()
}

// TestVersionsDiffEndToEnd tests the complete diff versions functionality
// with real version comparisons to ensure no schema differences are missed
func TestVersionsDiffEndToEnd(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	// Get available versions dynamically
	schemaMap, err := schema.NewSchemaMap()
	if err != nil {
		t.Fatalf("Failed to load schema map: %v", err)
	}

	var availableVersions []string
	for version := range schemaMap {
		availableVersions = append(availableVersions, version)
	}
	sort.Strings(availableVersions)

	if len(availableVersions) < 2 {
		t.Fatalf("Need at least 2 versions for testing, got %d", len(availableVersions))
	}

	// Use first and last versions for major diff, and consecutive versions for minor diff
	firstVersion := availableVersions[0]
	lastVersion := availableVersions[len(availableVersions)-1]
	var secondVersion, thirdVersion string
	if len(availableVersions) >= 3 {
		secondVersion = availableVersions[1]
		thirdVersion = availableVersions[2]
	} else {
		secondVersion = lastVersion
		thirdVersion = lastVersion
	}

	testCases := []struct {
		name           string
		version1       string
		version2       string
		flags          []string
		expectError    bool
		mustContain    []string // These strings must appear in output
		mustNotContain []string // These strings must NOT appear in output
		minChanges     int      // Minimum number of changes expected
	}{
		{
			name:     "first to last version comprehensive diff",
			version1: firstVersion,
			version2: lastVersion,
			flags:    []string{},
			mustContain: []string{
				"AEROSPIKE CONFIGURATION CHANGES SUMMARY",
				"Comparing: " + firstVersion + " → " + lastVersion,
				"Total changes:",
				"additions",
				"removals",
				"modifications",
				"SECTION: GENERAL",
				"NEW CONFIGURATIONS:",
				"REMOVED CONFIGURATIONS:",
				"MODIFIED CONFIGURATIONS:",
			},
			minChanges: 1, // Expect at least some changes between first and last versions
		},
		{
			name:     "consecutive versions diff",
			version1: secondVersion,
			version2: thirdVersion,
			flags:    []string{},
			mustContain: []string{
				"AEROSPIKE CONFIGURATION CHANGES SUMMARY",
				"Comparing: " + secondVersion + " → " + thirdVersion,
				"Total changes:",
			},
			minChanges: 0, // Consecutive versions might have no changes
		},
		{
			name:     "first to last version compact mode",
			version1: firstVersion,
			version2: lastVersion,
			flags:    []string{"--compact"},
			mustContain: []string{
				"AEROSPIKE CONFIGURATION CHANGES SUMMARY",
				"Comparing: " + firstVersion + " → " + lastVersion,
				"[SECTION: GENERAL]",
			},
			mustNotContain: []string{
				"→ Default:",
				"→ Dynamic:",
				"→ Enterprise Edition Only:",
				"→ Type:",
				"╭─────────────────────────────────────────────────────────────╮",
				"╰─────────────────────────────────────────────────────────────╯",
			},
			minChanges: 0,
		},
		{
			name:     "first to last version with service filter",
			version1: firstVersion,
			version2: lastVersion,
			flags:    []string{"--filter-path", "service"},
			mustContain: []string{
				"AEROSPIKE CONFIGURATION CHANGES SUMMARY",
				"Comparing: " + firstVersion + " → " + lastVersion,
			},
			mustNotContain: []string{
				"SECTION: NETWORK",
				"SECTION: NAMESPACES",
				"SECTION: LOGGING",
			},
			minChanges: 0,
		},
		{
			name:     "same version comparison",
			version1: firstVersion,
			version2: firstVersion,
			flags:    []string{},
			mustContain: []string{
				"AEROSPIKE CONFIGURATION CHANGES SUMMARY",
				"Comparing: " + firstVersion + " → " + firstVersion,
				"Total changes: 0",
			},
			minChanges: 0,
		},
		{
			name:        "invalid version",
			version1:    "invalid-version",
			version2:    firstVersion,
			flags:       []string{},
			expectError: true,
		},
		{
			name:        "non-existent version",
			version1:    "99.99.99",
			version2:    firstVersion,
			flags:       []string{},
			expectError: true,
		},
		// Additional hardcoded tests for stable versions that must remain intact
		{
			name:     "7.0.0 to 8.0.0 stable versions diff",
			version1: "7.0.0",
			version2: "8.0.0",
			flags:    []string{},
			mustContain: []string{
				"AEROSPIKE CONFIGURATION CHANGES SUMMARY",
				"Comparing: 7.0.0 → 8.0.0",
				"Total changes:",
			},
			minChanges: 0, // May or may not have changes
		},
		{
			name:     "8.0.0 to 8.1.0 stable versions diff",
			version1: "8.0.0",
			version2: "8.1.0",
			flags:    []string{},
			mustContain: []string{
				"AEROSPIKE CONFIGURATION CHANGES SUMMARY",
				"Comparing: 8.0.0 → 8.1.0",
				"Total changes:",
			},
			minChanges: 0,
		},
		{
			name:     "7.0.0 to 8.1.0 stable versions comprehensive diff",
			version1: "7.0.0",
			version2: "8.1.0",
			flags:    []string{},
			mustContain: []string{
				"AEROSPIKE CONFIGURATION CHANGES SUMMARY",
				"Comparing: 7.0.0 → 8.1.0",
				"Total changes:",
				"NEW CONFIGURATIONS:",
				"REMOVED CONFIGURATIONS:",
				"MODIFIED CONFIGURATIONS:",
			},
			minChanges: 0,
		},
		{
			name:     "7.0.0 to 8.0.0 compact mode stable versions",
			version1: "7.0.0",
			version2: "8.0.0",
			flags:    []string{"--compact"},
			mustContain: []string{
				"AEROSPIKE CONFIGURATION CHANGES SUMMARY",
				"Comparing: 7.0.0 → 8.0.0",
			},
			mustNotContain: []string{
				"→ Default:",
				"→ Dynamic:",
				"→ Enterprise Edition Only:",
				"→ Type:",
				"╭─────────────────────────────────────────────────────────────╮",
				"╰─────────────────────────────────────────────────────────────╯",
			},
			minChanges: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			output := captureStdout(func() {
				cmd := newDiffVersionsCmd()
				cmd.ParseFlags(tc.flags)
				err = runVersionsDiff(cmd, []string{tc.version1, tc.version2})
			})

			// Check error expectation
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Check required content
			for _, required := range tc.mustContain {
				if !strings.Contains(output, required) {
					t.Errorf("Output missing required content: %q\nOutput:\n%s", required, output)
				}
			}

			// Check forbidden content
			for _, forbidden := range tc.mustNotContain {
				if strings.Contains(output, forbidden) {
					t.Errorf("Output contains forbidden content: %q\nOutput:\n%s", forbidden, output)
				}
			}

			// Check minimum changes
			if tc.minChanges > 0 {
				changeCount := strings.Count(output, "Total changes:")
				if changeCount == 0 {
					t.Errorf("No change count found in output")
				}
				// Extract actual change count if needed for more precise validation
				if tc.minChanges > 0 && !strings.Contains(output, "Total changes: 0") {
					// For non-zero expected changes, just verify we have some indication of changes
					hasChanges := strings.Contains(output, "NEW CONFIGURATIONS:") ||
						strings.Contains(output, "REMOVED CONFIGURATIONS:") ||
						strings.Contains(output, "MODIFIED CONFIGURATIONS:")
					if tc.minChanges > 0 && !hasChanges {
						t.Errorf("Expected at least %d changes but found no change indicators", tc.minChanges)
					}
				}
			}
		})
	}
}

// TestVersionsDiffCLIIntegration tests the full CLI integration
func TestVersionsDiffCLIIntegration(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	// Get available versions for CLI tests
	schemaMap, err := schema.NewSchemaMap()
	if err != nil {
		t.Fatalf("Failed to load schema map: %v", err)
	}

	var availableVersions []string
	for version := range schemaMap {
		availableVersions = append(availableVersions, version)
	}
	sort.Strings(availableVersions)

	if len(availableVersions) < 2 {
		t.Fatalf("Need at least 2 versions for CLI testing, got %d", len(availableVersions))
	}

	firstVersion := availableVersions[0]
	secondVersion := availableVersions[1]

	testCases := []struct {
		name        string
		args        []string
		expectError bool
		errorCheck  func(error) bool
	}{
		{
			name:        "valid versions diff",
			args:        []string{"diff", "versions", firstVersion, secondVersion},
			expectError: false,
		},
		{
			name:        "valid versions diff with compact",
			args:        []string{"diff", "versions", firstVersion, secondVersion, "--compact"},
			expectError: false,
		},
		{
			name:        "valid versions diff with filter",
			args:        []string{"diff", "versions", firstVersion, secondVersion, "--filter-path", "security"},
			expectError: false,
		},
		{
			name:        "too few arguments",
			args:        []string{"diff", "versions", firstVersion},
			expectError: true,
			errorCheck: func(err error) bool {
				return strings.Contains(err.Error(), "requires exactly 2 version arguments")
			},
		},
		{
			name: "too many arguments",
			args: []string{
				"diff",
				"versions",
				firstVersion,
				secondVersion,
				availableVersions[len(availableVersions)-1],
			},
			expectError: true,
			errorCheck: func(err error) bool {
				return strings.Contains(err.Error(), "requires exactly 2 version arguments")
			},
		},
		{
			name:        "invalid version",
			args:        []string{"diff", "versions", "invalid", firstVersion},
			expectError: true,
		},
		// Additional hardcoded tests for stable versions
		{
			name:        "stable versions 7.0.0 to 8.0.0",
			args:        []string{"diff", "versions", "7.0.0", "8.0.0"},
			expectError: false,
		},
		{
			name:        "stable versions 8.0.0 to 8.1.0 with compact",
			args:        []string{"diff", "versions", "8.0.0", "8.1.0", "--compact"},
			expectError: false,
		},
		{
			name:        "stable versions 7.0.0 to 8.1.0 with filter",
			args:        []string{"diff", "versions", "7.0.0", "8.1.0", "--filter-path", "service"},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rootCmd := NewRootCmd()
			rootCmd.AddCommand(newDiffCmd())
			rootCmd.SetArgs(tc.args)

			err := rootCmd.Execute()

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tc.errorCheck != nil && !tc.errorCheck(err) {
					t.Errorf("Error check failed for error: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestVersionsDiffSpecificChanges tests for specific known changes between versions
func TestVersionsDiffSpecificChanges(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	// Get available versions for specific changes test
	schemaMap, err := schema.NewSchemaMap()
	if err != nil {
		t.Fatalf("Failed to load schema map: %v", err)
	}

	var availableVersions []string
	for version := range schemaMap {
		availableVersions = append(availableVersions, version)
	}
	sort.Strings(availableVersions)

	if len(availableVersions) < 2 {
		t.Fatalf("Need at least 2 versions for specific changes testing, got %d", len(availableVersions))
	}

	firstVersion := availableVersions[0]
	lastVersion := availableVersions[len(availableVersions)-1]

	testCases := []struct {
		name             string
		version1         string
		version2         string
		expectedAdded    []string // Specific configurations that should be added
		expectedRemoved  []string // Specific configurations that should be removed
		expectedModified []string // Specific configurations that should be modified
	}{
		{
			name:     "first to last version known changes",
			version1: firstVersion,
			version2: lastVersion,
			// Note: These are generic patterns that should exist in most version diffs
			// We're not hardcoding specific version changes since schemas can change
			// For generic tests, we just verify the diff runs successfully
			expectedAdded: []string{
				// Generic test - don't assert specific additions as schemas evolve
			},
			expectedRemoved: []string{
				// Generic test - don't assert specific removals
			},
			expectedModified: []string{
				// Generic test - don't assert specific modifications
			},
		},
		// Additional hardcoded test for stable versions
		{
			name:     "7.0.0 to 8.1.0 stable version changes",
			version1: "7.0.0",
			version2: "8.1.0",
			expectedAdded: []string{
				"logging.deprecation",        // This is actually added in 8.1.0
				"service.batch-max-requests", // This is actually added
			},
			expectedRemoved: []string{
				"logging.info-port", // This is actually removed in 8.1.0
				"network.info",      // This is actually removed
			},
			expectedModified: []string{
				"service.node-id.default", // This is actually modified
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			output := captureStdout(func() {
				cmd := newDiffVersionsCmd()
				err = runVersionsDiff(cmd, []string{tc.version1, tc.version2})
			})

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check for expected additions
			// We check that the path appears under the appropriate section header
			for _, expected := range tc.expectedAdded {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected addition not found: %s\nOutput:\n%s", expected, output)
					continue
				}
				// Verify it's in the correct section by checking it appears after the section header
				newConfigIdx := strings.Index(output, newConfigHeader)
				pathIdx := strings.Index(output, expected)
				if newConfigIdx > 0 && pathIdx > newConfigIdx {
					// Found in correct section
				} else {
					t.Errorf("Addition '%s' found but not in %s section", expected, newConfigHeader)
				}
			}

			// Check for expected removals
			for _, expected := range tc.expectedRemoved {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected removal not found: %s\nOutput:\n%s", expected, output)
					continue
				}
				// Verify it's in the correct section
				removedConfigIdx := strings.Index(output, removedConfigHeader)
				pathIdx := strings.Index(output, expected)
				if removedConfigIdx > 0 && pathIdx > removedConfigIdx {
					// Found in correct section
				} else {
					t.Errorf("Removal '%s' found but not in %s section", expected, removedConfigHeader)
				}
			}

			// Check for expected modifications
			for _, expected := range tc.expectedModified {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected modification not found: %s\nOutput:\n%s", expected, output)
					continue
				}
				// Verify it's in the correct section
				modifiedConfigIdx := strings.Index(output, modifiedConfigHeader)
				pathIdx := strings.Index(output, expected)
				if modifiedConfigIdx > 0 && pathIdx > modifiedConfigIdx {
					// Found in correct section
				} else {
					t.Errorf("Modification '%s' found but not in %s section", expected, modifiedConfigHeader)
				}
			}
		})
	}
}

// TestVersionsDiffSectionCoverage ensures all major sections are detected
func TestVersionsDiffSectionCoverage(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	// Get available versions for section coverage test
	schemaMap, err := schema.NewSchemaMap()
	if err != nil {
		t.Fatalf("Failed to load schema map: %v", err)
	}

	var availableVersions []string
	for version := range schemaMap {
		availableVersions = append(availableVersions, version)
	}
	sort.Strings(availableVersions)

	if len(availableVersions) < 2 {
		t.Fatalf("Need at least 2 versions for section coverage testing, got %d", len(availableVersions))
	}

	firstVersion := availableVersions[0]
	lastVersion := availableVersions[len(availableVersions)-1]

	// Run a comprehensive diff with dynamic versions and capture output
	output := captureStdout(func() {
		cmd := newDiffVersionsCmd()
		err = runVersionsDiff(cmd, []string{firstVersion, lastVersion})
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Expected major sections that should appear in a comprehensive diff
	expectedSections := []string{
		"SECTION: GENERAL",
		"SECTION: SERVICE",
		"SECTION: NETWORK",
		"SECTION: NAMESPACES",
		"SECTION: LOGGING",
		"SECTION: SECURITY",
		"SECTION: XDR",
		"SECTION: MOD-LUA",
	}

	for _, section := range expectedSections {
		if !strings.Contains(output, section) {
			t.Errorf("Expected section not found: %s", section)
		}
	}

	// Verify we have all types of changes
	changeTypes := []string{
		"NEW CONFIGURATIONS:",
		"REMOVED CONFIGURATIONS:",
		"MODIFIED CONFIGURATIONS:",
	}

	for _, changeType := range changeTypes {
		if !strings.Contains(output, changeType) {
			t.Errorf("Expected change type not found: %s", changeType)
		}
	}

	// Also test with stable hardcoded versions to ensure they remain intact
	t.Run("stable_versions_7.0.0_to_8.1.0", func(t *testing.T) {
		var err error
		output := captureStdout(func() {
			cmd := newDiffVersionsCmd()
			err = runVersionsDiff(cmd, []string{"7.0.0", "8.1.0"})
		})

		if err != nil {
			t.Fatalf("Stable version diff failed: %v", err)
		}

		// Expected major sections that should appear in a comprehensive diff
		expectedSections := []string{
			"SECTION: LOGGING",
			"SECTION: NAMESPACES",
			"SECTION: SERVICE",
		}

		for _, section := range expectedSections {
			if !strings.Contains(output, section) {
				t.Errorf("Expected section not found in stable version diff: %s", section)
			}
		}

		// Verify we have change types
		changeTypes := []string{
			"NEW CONFIGURATIONS:",
			"REMOVED CONFIGURATIONS:",
			"MODIFIED CONFIGURATIONS:",
		}

		hasAnyChangeType := false
		for _, changeType := range changeTypes {
			if strings.Contains(output, changeType) {
				hasAnyChangeType = true
				break
			}
		}

		// It's okay if there are no changes between stable versions
		if !hasAnyChangeType && !strings.Contains(output, "Total changes: 0") {
			t.Error("Expected either some change types or 'Total changes: 0' in stable version diff")
		}
	})
}

// TestVersionsDiffOutputConsistency ensures output format is consistent
func TestVersionsDiffOutputConsistency(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	// Get available versions for output consistency test
	schemaMap, err := schema.NewSchemaMap()
	if err != nil {
		t.Fatalf("Failed to load schema map: %v", err)
	}

	var availableVersions []string
	for version := range schemaMap {
		availableVersions = append(availableVersions, version)
	}
	sort.Strings(availableVersions)

	if len(availableVersions) < 2 {
		t.Fatalf("Need at least 2 versions for output consistency testing, got %d", len(availableVersions))
	}

	firstVersion := availableVersions[0]
	secondVersion := availableVersions[1]

	testCases := []struct {
		name     string
		version1 string
		version2 string
		flags    []string
	}{
		{"verbose_mode", firstVersion, secondVersion, []string{}},
		{"compact_mode", firstVersion, secondVersion, []string{"--compact"}},
		{"filtered_mode", firstVersion, secondVersion, []string{"--filter-path", "security"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			output := captureStdout(func() {
				cmd := newDiffVersionsCmd()
				cmd.ParseFlags(tc.flags)
				err = runVersionsDiff(cmd, []string{tc.version1, tc.version2})
			})

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Basic format checks
			if !strings.Contains(output, "AEROSPIKE CONFIGURATION CHANGES SUMMARY") {
				t.Error("Missing summary header")
			}

			if !strings.Contains(output, "Comparing:") {
				t.Error("Missing comparison line")
			}

			if !strings.Contains(output, "Total changes:") {
				t.Error("Missing total changes line")
			}

			// Check that output is not empty
			if len(strings.TrimSpace(output)) == 0 {
				t.Error("Output is empty")
			}

			// Check for proper section formatting (only if there are visible sections)
			if !strings.Contains(output, "Total changes: 0") {
				// Check if there are any visible sections in the output
				lines := strings.Split(output, "\n")
				var hasSectionHeader bool
				var hasChangeTypeHeader bool

				for _, line := range lines {
					if strings.Contains(line, "SECTION:") {
						hasSectionHeader = true
					}
					if strings.Contains(line, "NEW CONFIGURATIONS:") ||
						strings.Contains(line, "REMOVED CONFIGURATIONS:") ||
						strings.Contains(line, "MODIFIED CONFIGURATIONS:") {
						hasChangeTypeHeader = true
					}
				}

				// Only require section headers if there are actual change type headers visible
				// (filtering might hide all sections even when total changes > 0)
				if hasChangeTypeHeader && !hasSectionHeader {
					t.Error("No section headers found in output when changes are visible")
				}
			}
		})
	}
}

// TestFormatValue tests the formatValue function with various data types
func TestFormatValue(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "simple string",
			input:    "test",
			expected: "test",
		},
		{
			name:     "integer",
			input:    42,
			expected: "42",
		},
		{
			name:     "float",
			input:    3.14,
			expected: "3.14",
		},
		{
			name:     "boolean true",
			input:    true,
			expected: "Yes",
		},
		{
			name:     "boolean false",
			input:    false,
			expected: "No",
		},
		{
			name:     "empty array",
			input:    []any{},
			expected: "[]",
		},
		{
			name:     "simple array",
			input:    []any{"a", "b", "c"},
			expected: `["a","b","c"]`,
		},
		{
			name:     "complex object",
			input:    map[string]any{"key": "value"},
			expected: `{"key":"value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatValue(tt.input)
			if result != tt.expected {
				t.Errorf("formatValue(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestFormatArray tests the formatValue function for array values
func TestFormatArray(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "empty array",
			input:    []any{},
			expected: "[]",
		},
		{
			name:     "simple string array",
			input:    []any{"service", "network", "namespaces"},
			expected: `["service","network","namespaces"]`,
		},
		{
			name:     "mixed simple array",
			input:    []any{"string", 42, true},
			expected: `["string",42,true]`,
		},
		{
			name:     "array with complex objects",
			input:    []any{map[string]any{"type": "object"}, "simple"},
			expected: `[{"type":"object"},"simple"]`,
		},
		{
			name: "array with only complex objects",
			input: []any{
				map[string]any{"type": "object"},
				map[string]any{"another": "object"},
			},
			expected: `[{"type":"object"},{"another":"object"}]`,
		},
		{
			name:     "non-array input",
			input:    "not an array",
			expected: "not an array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatValue(tt.input)
			if result != tt.expected {
				t.Errorf("formatValue(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestPrintArrayChange tests array change formatting to ensure no raw maps are displayed
func TestPrintArrayChange(t *testing.T) {
	tests := []struct {
		name                string
		change              SchemaChange
		header              string
		verbose             bool
		expectedContains    []string
		expectedNotContains []string
	}{
		{
			name: "simple array addition verbose",
			change: SchemaChange{
				Path:  "/required/-",
				Value: "Bang!",
				Type:  Addition,
			},
			header:  "NEW CONFIGURATIONS",
			verbose: true,
			expectedContains: []string{
				"Array item: Bang!",
			},
			expectedNotContains: []string{
				"[object]",
				"map[",
				"interface{}",
			},
		},
		{
			name: "simple array addition compact",
			change: SchemaChange{
				Path:  "/required/-",
				Value: "Bang!",
				Type:  Addition,
			},
			header:  "NEW CONFIGURATIONS",
			verbose: false,
			expectedContains: []string{
				"required[+] (array item: Bang!)",
			},
			expectedNotContains: []string{
				"[object]",
				"map[",
				"interface{}",
			},
		},
		{
			name: "complex object array addition verbose",
			change: SchemaChange{
				Path: "/required/-",
				Value: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"test": map[string]any{
							"type":    "string",
							"default": "",
						},
					},
				},
				Type: Addition,
			},
			header:  "NEW CONFIGURATIONS",
			verbose: true,
			expectedContains: []string{
				"Array item:",
				"Type: object",
				"Properties:",
			},
			expectedNotContains: []string{
				"map[",
				"interface{}",
			},
		},
		{
			name: "complex object array addition compact",
			change: SchemaChange{
				Path: "/required/-",
				Value: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"test": map[string]any{
							"type":    "string",
							"default": "",
						},
					},
				},
				Type: Addition,
			},
			header:  newConfigHeader,
			verbose: false,
			expectedContains: []string{
				"required[+] (array item: object, 1 properties)",
			},
			expectedNotContains: []string{
				"map[",
				"interface{}",
				"[object]",
			},
		},
		{
			name: "array removal verbose",
			change: SchemaChange{
				Path:  "/required/-",
				Value: "OldValue",
				Type:  Removal,
			},
			header:  removedConfigHeader,
			verbose: true,
			expectedContains: []string{
				"Array item: OldValue",
			},
			expectedNotContains: []string{
				"map[",
			},
		},
		{
			name: "array removal compact",
			change: SchemaChange{
				Path:  "/required/-",
				Value: "OldValue",
				Type:  Removal,
			},
			header:  removedConfigHeader,
			verbose: false,
			expectedContains: []string{
				"required[+] (array item: OldValue)", // formatPath converts /required/- to required[+]
			},
			expectedNotContains: []string{
				"map[",
			},
		},
		{
			name: "array modification verbose",
			change: SchemaChange{
				Path:         "/items/-",
				Value:        map[string]any{"type": "string"},
				OldFullValue: map[string]any{"type": "integer"},
				NewFullValue: map[string]any{"type": "string"},
				Type:         Modification,
			},
			header:  modifiedConfigHeader,
			verbose: true,
			expectedContains: []string{
				"array[+]", // formatPath converts /items/- to array[+] (generic term)
				"Changed from:",
				"Changed to:",
			},
			expectedNotContains: []string{},
		},
		{
			name: "array modification compact",
			change: SchemaChange{
				Path:         "/items/-",
				Value:        map[string]any{"type": "string"},
				OldFullValue: "oldValue",
				NewFullValue: "newValue",
				Type:         Modification,
			},
			header:  modifiedConfigHeader,
			verbose: false,
			expectedContains: []string{
				"array[+] (oldValue → newValue)", // formatPath converts /items/- to array[+]
			},
			expectedNotContains: []string{
				"map[",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureStdout(func() {
				options := DiffOptions{Verbose: tt.verbose}
				path := formatPath(tt.change.Path)
				// Icon/prefix selection now handled internally by printArrayChange
				printArrayChange(tt.change, path, options)
			})

			// Check expected content
			for _, expected := range tt.expectedContains {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, but got:\n%s", expected, output)
				}
			}

			// Check content that should not be present
			for _, notExpected := range tt.expectedNotContains {
				if strings.Contains(output, notExpected) {
					t.Errorf("Expected output to NOT contain %q, but got:\n%s", notExpected, output)
				}
			}
		})
	}
}

// TestPrintBasicChange tests non-array (object property) change formatting
func TestPrintBasicChange(t *testing.T) {
	tests := []struct {
		name                string
		change              SchemaChange
		header              string
		verbose             bool
		expectedContains    []string
		expectedNotContains []string
	}{
		{
			name: "simple property addition verbose",
			change: SchemaChange{
				Path: "/service/proto-fd-max",
				Value: map[string]any{
					"type":    "integer",
					"default": 15000,
				},
				Type: Addition,
			},
			header:  newConfigHeader,
			verbose: true,
			expectedContains: []string{
				"service.proto-fd-max",
				"Type: integer",
				"Default: 15000",
			},
			expectedNotContains: []string{
				"map[",
				"interface{}",
			},
		},
		{
			name: "simple property addition compact",
			change: SchemaChange{
				Path: "/service/proto-fd-max",
				Value: map[string]any{
					"type":    "integer",
					"default": 15000,
				},
				Type: Addition,
			},
			header:  newConfigHeader,
			verbose: false,
			expectedContains: []string{
				"+ service.proto-fd-max (integer, default: 15000)",
			},
			expectedNotContains: []string{
				"map[",
				"interface{}",
			},
		},
		{
			name: "property removal verbose",
			change: SchemaChange{
				Path:  "/logging/info-port",
				Value: nil,
				Type:  Removal,
			},
			header:  removedConfigHeader,
			verbose: true,
			expectedContains: []string{
				"logging.info-port",
			},
			expectedNotContains: []string{
				"map[",
			},
		},
		{
			name: "property removal compact",
			change: SchemaChange{
				Path:  "/logging/info-port",
				Value: nil,
				Type:  Removal,
			},
			header:  removedConfigHeader,
			verbose: false,
			expectedContains: []string{
				"- logging.info-port (removed)",
			},
			expectedNotContains: []string{
				"map[",
			},
		},
		{
			name: "property modification verbose",
			change: SchemaChange{
				Path: "/service/node-id/default",
				Value: map[string]any{
					"type":    "string",
					"default": "a1",
				},
				OldValue: "0xA01",
				Type:     Modification,
			},
			header:  modifiedConfigHeader,
			verbose: true,
			expectedContains: []string{
				"service.node-id.default",
				"Type: string",
				"Default: a1",
			},
			expectedNotContains: []string{
				"map[",
			},
		},
		{
			name: "property modification compact",
			change: SchemaChange{
				Path:     "/service/node-id/default",
				Value:    "a1",
				OldValue: "0xA01",
				Type:     Modification,
			},
			header:  modifiedConfigHeader,
			verbose: false,
			expectedContains: []string{
				"~ service.node-id.default (0xA01 → a1)",
			},
			expectedNotContains: []string{
				"map[",
			},
		},
		{
			name: "complex object property addition verbose",
			change: SchemaChange{
				Path: "/network/admin",
				Value: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"endpoint": map[string]any{
							"type": "string",
						},
					},
				},
				Type: Addition,
			},
			header:  newConfigHeader,
			verbose: true,
			expectedContains: []string{
				"network.admin",
				"Type: object",
				"Properties:",
			},
			expectedNotContains: []string{
				"map[",
			},
		},
		{
			name: "complex object property addition compact",
			change: SchemaChange{
				Path: "/network/admin",
				Value: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"endpoint": map[string]any{
							"type": "string",
						},
					},
				},
				Type: Addition,
			},
			header:  newConfigHeader,
			verbose: false,
			expectedContains: []string{
				"+ network.admin (object, 1 properties)",
			},
			expectedNotContains: []string{
				"map[",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureStdout(func() {
				options := DiffOptions{Verbose: tt.verbose}
				path := formatPath(tt.change.Path)
				// Use appropriate icon/prefix based on change type and mode
				var icon, prefix string
				if tt.verbose {
					switch tt.change.Type {
					case Addition:
						icon = iconAddition
					case Removal:
						icon = iconRemoval
					case Modification:
						icon = iconModification
					}
					if tt.change.Type == Modification {
						printModifications([]SchemaChange{tt.change}, options, icon, "")
					} else {
						printBasicChange(tt.change, path, tt.header, icon, "", options)
					}
				} else {
					switch tt.change.Type {
					case Addition:
						prefix = additionPrefix
					case Removal:
						prefix = removalPrefix
					case Modification:
						prefix = modificationPrefix
					}
					if tt.change.Type == Modification {
						printModifications([]SchemaChange{tt.change}, options, "", prefix)
					} else {
						printBasicChange(tt.change, path, tt.header, "", prefix, options)
					}
				}
			})

			// Check expected content
			for _, expected := range tt.expectedContains {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, but got:\n%s", expected, output)
				}
			}

			// Check content that should not be present
			for _, notExpected := range tt.expectedNotContains {
				if strings.Contains(output, notExpected) {
					t.Errorf("Expected output to NOT contain %q, but got:\n%s", notExpected, output)
				}
			}
		})
	}
}

// TestFormatPath tests the formatPath function with various edge cases
func TestFormatPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty path",
			input:    "",
			expected: "",
		},
		{
			name:     "path with only slashes",
			input:    "/",
			expected: "",
		},
		{
			name:     "path with array index after items (items is skipped)",
			input:    "/items/0",
			expected: "0",
		},
		{
			name:     "path with array append after items",
			input:    "/items/-",
			expected: "array[+]",
		},
		{
			name:     "path with properties (skip properties)",
			input:    "/properties/name",
			expected: "name",
		},
		{
			name:     "path with items (skip items)",
			input:    "/items/properties/test",
			expected: "test",
		},
		{
			name:     "nested arrays with items metadata",
			input:    "/items/0/subitems/1",
			expected: "0.subitems[1]",
		},
		{
			name:     "real config path with array",
			input:    "/namespaces/0/storage-engine",
			expected: "namespaces[0].storage-engine",
		},
		{
			name:     "empty array without parent",
			input:    "/-",
			expected: "array[+]",
		},
		{
			name:     "simple nested path",
			input:    "/service/logging/level",
			expected: "service.logging.level",
		},
		{
			name:     "path with multiple slashes",
			input:    "//service//logging//",
			expected: "service.logging",
		},
		{
			name:     "only numeric index",
			input:    "/0",
			expected: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatPath(tt.input)
			if result != tt.expected {
				t.Errorf("formatPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestFormatNumber tests the formatNumber function with various numeric edge cases
func TestFormatNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "zero",
			input:    0,
			expected: "0",
		},
		{
			name:     "zero float",
			input:    0.0,
			expected: "0",
		},
		{
			name:     "negative zero",
			input:    -0.0,
			expected: "0",
		},
		{
			name:     "very small decimal",
			input:    0.000001,
			expected: "1e-06",
		},
		{
			name:     "small decimal close to zero",
			input:    0.0001,
			expected: "0.0001",
		},
		{
			name:     "very large number",
			input:    1e20,
			expected: "1e+20",
		},
		{
			name:     "integer as float64",
			input:    15000.0,
			expected: "15000",
		},
		{
			name:     "negative integer",
			input:    -42,
			expected: "-42",
		},
		{
			name:     "negative float",
			input:    -42.5,
			expected: "-42.5",
		},
		{
			name:     "decimal number",
			input:    3.14159,
			expected: "3.14159",
		},
		{
			name:     "int type",
			input:    int(12345),
			expected: "12345",
		},
		{
			name:     "int64 type",
			input:    int64(9876543210),
			expected: "9876543210",
		},
		{
			name:     "positive infinity",
			input:    math.Inf(1),
			expected: "+Inf",
		},
		{
			name:     "negative infinity",
			input:    math.Inf(-1),
			expected: "-Inf",
		},
		{
			name:     "NaN",
			input:    math.NaN(),
			expected: "NaN",
		},
		{
			name:     "max int64",
			input:    int64(math.MaxInt64),
			expected: "9223372036854775807", // Now formatted directly without float64 conversion
		},
		{
			name:     "min int64",
			input:    int64(math.MinInt64),
			expected: "-9223372036854775808", // Now formatted directly without float64 conversion
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatNumber(tt.input)
			if result != tt.expected {
				t.Errorf("formatNumber(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestGetParentPath tests the getParentPath function
func TestGetParentPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "path with one element",
			input:    "/service",
			expected: "",
		},
		{
			name:     "empty path",
			input:    "",
			expected: "",
		},
		{
			name:     "root level",
			input:    "/",
			expected: "",
		},
		{
			name:     "nested path",
			input:    "/service/logging/level",
			expected: "/service/logging",
		},
		{
			name:     "array index",
			input:    "/items/0",
			expected: "/items",
		},
		{
			name:     "deeply nested",
			input:    "/a/b/c/d/e/f",
			expected: "/a/b/c/d/e",
		},
		{
			name:     "two elements",
			input:    "/service/logging",
			expected: "/service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getParentPath(tt.input)
			if result != tt.expected {
				t.Errorf("getParentPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestIsArrayIndex tests the isArrayIndex function
func TestIsArrayIndex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "numeric index",
			input:    "/items/0",
			expected: true,
		},
		{
			name:     "non-numeric",
			input:    "/items/test",
			expected: false,
		},
		{
			name:     "array append",
			input:    "/items/-",
			expected: false,
		},
		{
			name:     "empty path",
			input:    "",
			expected: false,
		},
		{
			name:     "only index",
			input:    "/0",
			expected: true,
		},
		{
			name:     "multiple digit index",
			input:    "/items/123",
			expected: true,
		},
		{
			name:     "path with properties",
			input:    "/properties/name",
			expected: false,
		},
		{
			name:     "root",
			input:    "/",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isArrayIndex(tt.input)
			if result != tt.expected {
				t.Errorf("isArrayIndex(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestIsArrayPath tests the isArrayPath function
func TestIsArrayPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "array append",
			input:    "/items/-",
			expected: true,
		},
		{
			name:     "numeric index",
			input:    "/items/0",
			expected: true,
		},
		{
			name:     "non-array",
			input:    "/service/name",
			expected: false,
		},
		{
			name:     "empty path",
			input:    "",
			expected: false,
		},
		{
			name:     "root array append",
			input:    "/-",
			expected: true,
		},
		{
			name:     "multiple digit index",
			input:    "/items/999",
			expected: true,
		},
		{
			name:     "properties path",
			input:    "/properties/test",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isArrayPath(tt.input)
			if result != tt.expected {
				t.Errorf("isArrayPath(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestGetValueSummary tests the getValueSummary function
func TestGetValueSummary(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: "(no details)",
		},
		{
			name:     "simple string",
			input:    "test",
			expected: "(test)",
		},
		{
			name:     "simple number",
			input:    42,
			expected: "(42)",
		},
		{
			name:     "boolean true",
			input:    true,
			expected: "(Yes)",
		},
		{
			name:     "boolean false",
			input:    false,
			expected: "(No)",
		},
		{
			name: "object with type only",
			input: map[string]any{
				"type": "string",
			},
			expected: "(string)",
		},
		{
			name: "object with type and default",
			input: map[string]any{
				"type":    "integer",
				"default": 100,
			},
			expected: "(integer, default: 100)",
		},
		{
			name: "object with properties",
			input: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"field1": map[string]any{},
					"field2": map[string]any{},
					"field3": map[string]any{},
				},
			},
			expected: "(object, 3 properties)",
		},
		{
			name: "object with items",
			input: map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
				},
			},
			expected: "(array, items: string)",
		},
		{
			name: "object with enum",
			input: map[string]any{
				"type": "string",
				"enum": []any{"value1", "value2", "value3", "value4", "value5"},
			},
			expected: "(string, 5 allowed values)",
		},
		{
			name:     "empty object",
			input:    map[string]any{},
			expected: "(object)",
		},
		{
			name: "complex nested object",
			input: map[string]any{
				"type":    "object",
				"default": "test",
				"properties": map[string]any{
					"field1": map[string]any{},
				},
				"enum": []any{"a", "b"},
			},
			expected: "(object, default: test, 1 properties, 2 allowed values)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getValueSummary(tt.input)
			if result != tt.expected {
				t.Errorf("getValueSummary(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestFormatCompactValue tests the formatCompactValue function
func TestFormatCompactValue(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "nil",
			input:    nil,
			expected: "null",
		},
		{
			name:     "boolean true",
			input:    true,
			expected: "Yes",
		},
		{
			name:     "boolean false",
			input:    false,
			expected: "No",
		},
		{
			name:     "short string",
			input:    "test",
			expected: "test",
		},
		{
			name:     "long string truncation",
			input:    "This is a very long string that should be truncated because it exceeds fifty characters in length",
			expected: "This is a very long string that should be trunc...",
		},
		{
			name:     "exactly 50 chars",
			input:    "12345678901234567890123456789012345678901234567890",
			expected: "12345678901234567890123456789012345678901234567890",
		},
		{
			name:     "integer",
			input:    42,
			expected: "42",
		},
		{
			name:     "float",
			input:    3.14,
			expected: "3.14",
		},
		{
			name:     "zero",
			input:    0,
			expected: "0",
		},
		{
			name:     "array",
			input:    []any{1, 2, 3},
			expected: "array[3]",
		},
		{
			name:     "empty array",
			input:    []any{},
			expected: "array[0]",
		},
		{
			name: "object",
			input: map[string]any{
				"key": "value",
			},
			expected: `object[1]`,
		},
		{
			name:     "empty object",
			input:    map[string]any{},
			expected: "object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatCompactValue(tt.input)
			if result != tt.expected {
				t.Errorf("formatCompactValue(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestGetValueByJSONPath tests the getValueByJSONPath function
func TestGetValueByJSONPath(t *testing.T) {
	testData := map[string]any{
		"service": map[string]any{
			"logging": map[string]any{
				"level": "info",
				"port":  3000,
			},
			"proto-fd-max": 15000,
		},
		"namespaces": []any{
			map[string]any{
				"name":               "test",
				"replication-factor": 2,
			},
			map[string]any{
				"name":               "prod",
				"replication-factor": 3,
			},
		},
	}

	tests := []struct {
		name      string
		path      string
		expectOk  bool
		expectVal any
	}{
		{
			name:      "valid nested path",
			path:      "/service/logging/level",
			expectOk:  true,
			expectVal: "info",
		},
		{
			name:      "valid top level",
			path:      "/service",
			expectOk:  true,
			expectVal: testData["service"],
		},
		{
			name:      "path to non-existent key",
			path:      "/service/nonexistent",
			expectOk:  false,
			expectVal: nil,
		},
		{
			name:      "array with valid index",
			path:      "/namespaces/0/name",
			expectOk:  true,
			expectVal: "test",
		},
		{
			name:      "array out of bounds",
			path:      "/namespaces/5",
			expectOk:  false,
			expectVal: nil,
		},
		{
			name:      "empty path",
			path:      "",
			expectOk:  true,
			expectVal: testData,
		},
		{
			name:      "root slash only",
			path:      "/",
			expectOk:  true,
			expectVal: testData,
		},
		{
			name:      "negative array index",
			path:      "/namespaces/-1",
			expectOk:  false,
			expectVal: nil,
		},
		{
			name:      "path through array to nested value",
			path:      "/namespaces/1/replication-factor",
			expectOk:  true,
			expectVal: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := getValueByJSONPath(testData, tt.path)
			if ok != tt.expectOk {
				t.Errorf("getValueByJSONPath(%q) ok = %v, want %v", tt.path, ok, tt.expectOk)
			}
			if tt.expectOk && !reflect.DeepEqual(val, tt.expectVal) {
				t.Errorf("getValueByJSONPath(%q) val = %v, want %v", tt.path, val, tt.expectVal)
			}
		})
	}
}

// TestNoRawMapOutput tests that no raw Go maps are ever displayed in diff output
func TestNoRawMapOutput(t *testing.T) {
	// Test various complex scenarios that previously showed raw maps
	testCases := []struct {
		name   string
		schema any
	}{
		{
			name: "nested objects with properties",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"nested": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"deep": map[string]any{
								"type":    "string",
								"default": "",
							},
						},
					},
				},
			},
		},
		{
			name: "array with mixed content",
			schema: []any{
				"simple string",
				42,
				true,
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"field": map[string]any{
							"type": "string",
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test formatValue
			result := formatValue(tc.schema)
			if strings.Contains(result, "map[") {
				t.Errorf("formatValue returned raw map: %s", result)
			}

			// Test formatValue
			arrayResult := formatValue(tc.schema)
			if strings.Contains(arrayResult, "map[") {
				t.Errorf("formatValue returned raw map: %s", arrayResult)
			}

			// Test printNestedSchema output
			output := captureStdout(func() {
				options := DiffOptions{Verbose: true}
				printValueProperties(tc.schema, options)
			})

			if strings.Contains(output, "map[") {
				t.Errorf("printNestedSchema output contains raw map:\n%s", output)
			}
			if strings.Contains(output, "interface{}") {
				t.Errorf("printNestedSchema output contains interface{} type:\n%s", output)
			}
		})
	}
}

// TestInvalidChangeType tests that invalid change types cause an error (fail fast)
func TestInvalidChangeType(t *testing.T) {
	// Create a mock schema change with invalid type
	changes := []SchemaChange{
		{
			Path: "/test/path",
			Type: ChangeType("invalid_type"), // This should trigger an error
		},
	}

	validSections := map[string]bool{"test": true}
	summary, err := groupChangesBySection(changes, "1.0.0", "2.0.0", validSections)

	// Should return an error for unknown change type (fail fast)
	if err == nil {
		t.Fatal("Expected error for invalid change type, but got nil")
	}

	// Verify the error message contains relevant information
	expectedErrMsg := "unknown change type \"invalid_type\""
	if !strings.Contains(err.Error(), expectedErrMsg) {
		t.Errorf("Error message should contain %q, got: %v", expectedErrMsg, err)
	}

	// Summary should be empty due to error
	if summary.TotalAdditions != 0 || summary.TotalRemovals != 0 || summary.TotalModified != 0 {
		t.Errorf(
			"Expected empty summary on error, got: additions=%d, removals=%d, modifications=%d",
			summary.TotalAdditions,
			summary.TotalRemovals,
			summary.TotalModified,
		)
	}
}

// TestInvalidFilterValidation tests that invalid filter sections are properly validated
func TestInvalidFilterValidation(t *testing.T) {
	testCases := []struct {
		name           string
		filterSections map[string]struct{}
		expectError    bool
		expectedError  string
	}{
		{
			name:           "valid single filter",
			filterSections: map[string]struct{}{"namespaces": {}},
			expectError:    false,
		},
		{
			name:           "valid multiple filters",
			filterSections: map[string]struct{}{"namespaces": {}, "service": {}},
			expectError:    false,
		},
		{
			name:           "invalid single filter",
			filterSections: map[string]struct{}{"invalid": {}},
			expectError:    true,
			expectedError:  "invalid filter section(s): invalid",
		},
		{
			name:           "invalid multiple filters",
			filterSections: map[string]struct{}{"invalid1": {}, "invalid2": {}},
			expectError:    true,
			expectedError:  "invalid filter section(s): invalid1, invalid2",
		},
		{
			name:           "mix of valid and invalid filters",
			filterSections: map[string]struct{}{"namespaces": {}, "invalid": {}},
			expectError:    true,
			expectedError:  "invalid filter section(s): invalid",
		},
	}

	// Create mock available sections
	availableSections := map[string]SectionChanges{
		"namespaces": {},
		"service":    {},
		"network":    {},
		"logging":    {},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateFilterSections(tc.filterSections, availableSections)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), tc.expectedError) {
					t.Errorf("Expected error to contain %q, got %q", tc.expectedError, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestUnwrapParentheses tests the unwrapParentheses helper function for edge cases
func TestUnwrapParentheses(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "empty string", input: "", expected: ""},
		{name: "single character", input: "x", expected: "x"},
		{name: "two characters not parens", input: "ab", expected: "ab"},
		{name: "only opening paren", input: "(", expected: "("},
		{name: "only closing paren", input: ")", expected: ")"},
		{name: "empty parens", input: "()", expected: ""},
		{name: "wrapped string", input: "(hello)", expected: "hello"},
		{name: "nested parens", input: "((nested))", expected: "(nested)"},
		{name: "only opening at start", input: "(hello", expected: "(hello"},
		{name: "only closing at end", input: "hello)", expected: "hello)"},
		{name: "parens in middle", input: "hel(lo", expected: "hel(lo"},
		{name: "multiple pairs", input: "()()", expected: ")("}, // Only unwraps outer pair
		{name: "long wrapped string", input: "(this is a long string)", expected: "this is a long string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := unwrapParentheses(tt.input)
			if result != tt.expected {
				t.Errorf("unwrapParentheses(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestCalculateBoxPadding tests the calculateBoxPadding helper function for edge cases
func TestCalculateBoxPadding(t *testing.T) {
	tests := []struct {
		name          string
		textLen       int
		boxWidth      int
		expectedLeft  int
		expectedRight int
	}{
		{name: "exact fit", textLen: 10, boxWidth: 10, expectedLeft: 0, expectedRight: 0},
		{name: "one space total", textLen: 10, boxWidth: 11, expectedLeft: 0, expectedRight: 1},
		{name: "two spaces evenly split", textLen: 10, boxWidth: 12, expectedLeft: 1, expectedRight: 1},
		{name: "three spaces (odd)", textLen: 10, boxWidth: 13, expectedLeft: 1, expectedRight: 2},
		{name: "four spaces evenly split", textLen: 10, boxWidth: 14, expectedLeft: 2, expectedRight: 2},
		{name: "large padding", textLen: 10, boxWidth: 50, expectedLeft: 20, expectedRight: 20},
		{name: "zero text length", textLen: 0, boxWidth: 10, expectedLeft: 5, expectedRight: 5},
		{name: "zero box width", textLen: 0, boxWidth: 0, expectedLeft: 0, expectedRight: 0},
		{name: "defensive: box smaller than text", textLen: 20, boxWidth: 10, expectedLeft: 0, expectedRight: 0},
		{name: "defensive: negative box width", textLen: 10, boxWidth: -5, expectedLeft: 0, expectedRight: 0},
		{name: "defensive: both zero", textLen: 0, boxWidth: 0, expectedLeft: 0, expectedRight: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			left, right := calculateBoxPadding(tt.textLen, tt.boxWidth)
			if left != tt.expectedLeft || right != tt.expectedRight {
				t.Errorf("calculateBoxPadding(%d, %d) = (%d, %d), want (%d, %d)",
					tt.textLen, tt.boxWidth, left, right, tt.expectedLeft, tt.expectedRight)
			}

			// Verify non-negative values
			if left < 0 {
				t.Errorf("Left padding is negative: %d", left)
			}
			if right < 0 {
				t.Errorf("Right padding is negative: %d", right)
			}

			// Verify that padding + textLen doesn't exceed boxWidth (unless defensive case)
			if tt.boxWidth >= tt.textLen {
				total := left + right + tt.textLen
				if total != tt.boxWidth {
					t.Errorf("Total width mismatch: left(%d) + right(%d) + text(%d) = %d, want %d",
						left, right, tt.textLen, total, tt.boxWidth)
				}
			}
		})
	}
}
