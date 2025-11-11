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
				"AEROSPIKE SCHEMA CHANGES SUMMARY",
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
				"AEROSPIKE SCHEMA CHANGES SUMMARY",
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
				"AEROSPIKE SCHEMA CHANGES SUMMARY",
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
				"AEROSPIKE SCHEMA CHANGES SUMMARY",
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
				"AEROSPIKE SCHEMA CHANGES SUMMARY",
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
				"AEROSPIKE SCHEMA CHANGES SUMMARY",
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
				"AEROSPIKE SCHEMA CHANGES SUMMARY",
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
				"AEROSPIKE SCHEMA CHANGES SUMMARY",
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
				"AEROSPIKE SCHEMA CHANGES SUMMARY",
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
			// Capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Create command and run
			cmd := newDiffVersionsCmd()
			cmd.ParseFlags(tc.flags)
			err := runVersionsDiff(cmd, []string{tc.version1, tc.version2})

			// Restore stdout and get output
			w.Close()
			os.Stdout = oldStdout
			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := buf.String()

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
			args:        []string{"diff", "versions", firstVersion, secondVersion, "--filter-path", "service"},
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
			// Capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Run diff
			cmd := newDiffVersionsCmd()
			err := runVersionsDiff(cmd, []string{tc.version1, tc.version2})

			// Restore stdout and get output
			w.Close()
			os.Stdout = oldStdout
			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := buf.String()

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

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

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

	// Run a comprehensive diff with dynamic versions
	cmd := newDiffVersionsCmd()
	err = runVersionsDiff(cmd, []string{firstVersion, lastVersion})

	// Restore stdout and get output
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

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
		// Capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Run stable version diff
		cmd := newDiffVersionsCmd()
		err := runVersionsDiff(cmd, []string{"7.0.0", "8.1.0"})

		// Restore stdout and get output
		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

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
		{"filtered_mode", firstVersion, secondVersion, []string{"--filter-path", "service"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Run diff
			cmd := newDiffVersionsCmd()
			cmd.ParseFlags(tc.flags)
			err := runVersionsDiff(cmd, []string{tc.version1, tc.version2})

			// Restore stdout and get output
			w.Close()
			os.Stdout = oldStdout
			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := buf.String()

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Basic format checks
			if !strings.Contains(output, "AEROSPIKE SCHEMA CHANGES SUMMARY") {
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
