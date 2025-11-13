//go:build unit

package cmd

import (
	"bytes"
	"os"
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
			expectedAdded: []string{
				"enterpriseOnly", // This is commonly added in newer versions
			},
			expectedRemoved: []string{
				// We'll check for any removals dynamically
			},
			expectedModified: []string{
				"required", // Schema required fields often change
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
			for _, expected := range tc.expectedAdded {
				if !strings.Contains(output, "✅ "+expected) && !strings.Contains(output, "+ "+expected) {
					t.Errorf("Expected addition not found: %s\nOutput:\n%s", expected, output)
				}
			}

			// Check for expected removals
			for _, expected := range tc.expectedRemoved {
				if !strings.Contains(output, "❌ "+expected) && !strings.Contains(output, "- "+expected) {
					t.Errorf("Expected removal not found: %s\nOutput:\n%s", expected, output)
				}
			}

			// Check for expected modifications
			for _, expected := range tc.expectedModified {
				if !strings.Contains(output, "🔄 "+expected) && !strings.Contains(output, "~ "+expected) {
					t.Errorf("Expected modification not found: %s\nOutput:\n%s", expected, output)
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
			header:  "NEW CONFIGURATIONS",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureStdout(func() {
				options := DiffOptions{Verbose: tt.verbose}
				path := formatPath(tt.change.Path)
				var prefix string
				if tt.verbose {
					prefix = "✅"
				} else {
					prefix = "+"
				}
				printArrayChange(tt.change, path, tt.header, prefix, options)
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

// TestInvalidChangeType tests that invalid change types are handled gracefully
func TestInvalidChangeType(t *testing.T) {
	// Create a mock schema change with invalid type
	changes := []SchemaChange{
		{
			Path: "/test/path",
			Type: ChangeType("invalid_type"), // This should trigger the default case
		},
	}

	validSections := map[string]bool{"test": true}
	summary := groupChangesBySection(changes, "1.0.0", "2.0.0", validSections)

	// The change should be ignored due to invalid type
	// Total counts should be 0
	if summary.TotalAdditions != 0 || summary.TotalRemovals != 0 || summary.TotalModified != 0 {
		t.Errorf(
			"Expected all totals to be 0 for invalid change type, got: additions=%d, removals=%d, modifications=%d",
			summary.TotalAdditions,
			summary.TotalRemovals,
			summary.TotalModified,
		)
	}

	// The change should not appear in any section
	for _, sectionChanges := range summary.Sections {
		if len(sectionChanges.Additions) > 0 || len(sectionChanges.Removals) > 0 ||
			len(sectionChanges.Modifications) > 0 {
			t.Error("Invalid change type should not be added to any section")
		}
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
