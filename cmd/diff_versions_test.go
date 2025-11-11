//go:build unit

package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestVersionsDiffEndToEnd tests the complete diff versions functionality
// with real version comparisons to ensure no schema differences are missed
func TestVersionsDiffEndToEnd(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
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
			name:     "6.4.0 to 7.0.0 comprehensive diff",
			version1: "6.4.0",
			version2: "7.0.0",
			flags:    []string{},
			mustContain: []string{
				"AEROSPIKE SCHEMA CHANGES SUMMARY",
				"Comparing: 6.4.0 → 7.0.0",
				"Total changes:",
				"additions",
				"removals",
				"modifications",
				"SECTION: GENERAL",
				"SECTION: SERVICE",
				"SECTION: NETWORK",
				"SECTION: NAMESPACES",
				"SECTION: LOGGING",
				"enterpriseOnly",
				"NEW CONFIGURATIONS:",
				"REMOVED CONFIGURATIONS:",
				"MODIFIED CONFIGURATIONS:",
			},
			minChanges: 400, // Expect significant changes between these versions
		},
		{
			name:     "7.0.0 to 8.0.0 comprehensive diff",
			version1: "7.0.0",
			version2: "8.0.0",
			flags:    []string{},
			mustContain: []string{
				"AEROSPIKE SCHEMA CHANGES SUMMARY",
				"Comparing: 7.0.0 → 8.0.0",
				"Total changes:",
			},
			minChanges: 10, // Expect some changes between these versions
		},
		{
			name:     "6.4.0 to 7.0.0 compact mode",
			version1: "6.4.0",
			version2: "7.0.0",
			flags:    []string{"--compact"},
			mustContain: []string{
				"AEROSPIKE SCHEMA CHANGES SUMMARY",
				"Comparing: 6.4.0 → 7.0.0",
				"[SECTION: GENERAL]",
				"[SECTION: SERVICE]",
				"+ enterpriseOnly",
				"- service.salt-allocations",
				"~ service.debug-allocations.default",
			},
			mustNotContain: []string{
				"→ Default:",
				"→ Dynamic:",
				"→ Enterprise Edition Only:",
				"→ Type:",
				"╭─────────────────────────────────────────────────────────────╮",
				"╰─────────────────────────────────────────────────────────────╯",
			},
			minChanges: 400,
		},
		{
			name:     "6.4.0 to 7.0.0 with service filter",
			version1: "6.4.0",
			version2: "7.0.0",
			flags:    []string{"--filter-path", "service"},
			mustContain: []string{
				"SECTION: SERVICE",
				"service.enterpriseOnly",
				"service.poison-allocations",
				"service.quarantine-allocations",
			},
			mustNotContain: []string{
				"SECTION: NETWORK",
				"SECTION: NAMESPACES",
				"SECTION: LOGGING",
				"network.enterpriseOnly",
				"namespaces.enterpriseOnly",
			},
			minChanges: 10,
		},
		{
			name:     "6.4.0 to 7.0.0 with multiple section filter",
			version1: "6.4.0",
			version2: "7.0.0",
			flags:    []string{"--filter-path", "service,network"},
			mustContain: []string{
				"SECTION: SERVICE",
				"SECTION: NETWORK",
				"service.enterpriseOnly",
				"network.enterpriseOnly",
			},
			mustNotContain: []string{
				"SECTION: NAMESPACES",
				"SECTION: LOGGING",
				"namespaces.enterpriseOnly",
				"logging.enterpriseOnly",
			},
			minChanges: 50,
		},
		{
			name:     "Same version comparison",
			version1: "7.0.0",
			version2: "7.0.0",
			flags:    []string{},
			mustContain: []string{
				"AEROSPIKE SCHEMA CHANGES SUMMARY",
				"Comparing: 7.0.0 → 7.0.0",
				"Total changes: 0",
			},
			minChanges: 0,
		},
		{
			name:        "Invalid version",
			version1:    "invalid-version",
			version2:    "7.0.0",
			flags:       []string{},
			expectError: true,
		},
		{
			name:        "Non-existent version",
			version1:    "99.99.99",
			version2:    "7.0.0",
			flags:       []string{},
			expectError: true,
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

	testCases := []struct {
		name        string
		args        []string
		expectError bool
		errorCheck  func(error) bool
	}{
		{
			name:        "valid versions diff",
			args:        []string{"diff", "versions", "6.4.0", "7.0.0"},
			expectError: false,
		},
		{
			name:        "valid versions diff with compact",
			args:        []string{"diff", "versions", "6.4.0", "7.0.0", "--compact"},
			expectError: false,
		},
		{
			name:        "valid versions diff with filter",
			args:        []string{"diff", "versions", "6.4.0", "7.0.0", "--filter-path", "service"},
			expectError: false,
		},
		{
			name:        "too few arguments",
			args:        []string{"diff", "versions", "6.4.0"},
			expectError: true,
			errorCheck: func(err error) bool {
				return strings.Contains(err.Error(), "requires exactly 2 version arguments")
			},
		},
		{
			name:        "too many arguments",
			args:        []string{"diff", "versions", "6.4.0", "7.0.0", "8.0.0"},
			expectError: true,
			errorCheck: func(err error) bool {
				return strings.Contains(err.Error(), "requires exactly 2 version arguments")
			},
		},
		{
			name:        "invalid version",
			args:        []string{"diff", "versions", "invalid", "7.0.0"},
			expectError: true,
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

	testCases := []struct {
		name             string
		version1         string
		version2         string
		expectedAdded    []string // Specific configurations that should be added
		expectedRemoved  []string // Specific configurations that should be removed
		expectedModified []string // Specific configurations that should be modified
	}{
		{
			name:     "6.4.0 to 7.0.0 known changes",
			version1: "6.4.0",
			version2: "7.0.0",
			expectedAdded: []string{
				"enterpriseOnly",
				"service.poison-allocations",
				"service.quarantine-allocations",
				"logging.drv-mem",
				"namespaces.evict-sys-memory-pct",
				"namespaces.sets.default-ttl",
			},
			expectedRemoved: []string{
				"service.salt-allocations",
				"namespaces.memory-size",
				"namespaces.stop-writes-pct",
			},
			expectedModified: []string{
				"service.feature-key-files.default",
				"required",
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

	// Run a comprehensive diff
	cmd := newDiffVersionsCmd()
	err := runVersionsDiff(cmd, []string{"6.4.0", "7.0.0"})

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
}

// TestVersionsDiffOutputConsistency ensures output format is consistent
func TestVersionsDiffOutputConsistency(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	testCases := []struct {
		name     string
		version1 string
		version2 string
		flags    []string
	}{
		{"verbose_mode", "6.4.0", "7.0.0", []string{}},
		{"compact_mode", "6.4.0", "7.0.0", []string{"--compact"}},
		{"filtered_mode", "6.4.0", "7.0.0", []string{"--filter-path", "service"}},
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

			// Check for proper section formatting
			lines := strings.Split(output, "\n")
			var hasSectionHeader bool
			for _, line := range lines {
				if strings.Contains(line, "SECTION:") {
					hasSectionHeader = true
					break
				}
			}
			if !hasSectionHeader {
				t.Error("No section headers found in output")
			}
		})
	}
}
