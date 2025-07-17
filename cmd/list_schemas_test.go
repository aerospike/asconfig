//go:build unit

package cmd

import (
	"strings"
	"testing"
)

func TestListSchemasCmd(t *testing.T) {
	testCases := []struct {
		name        string
		flags       map[string]string
		expectError bool
		checkOutput func(string) bool
	}{
		{
			name:        "simple format",
			flags:       map[string]string{},
			expectError: false,
			checkOutput: func(output string) bool {
				// Simple format should have versions separated by spaces
				return strings.Contains(output, "6.4.0") && strings.Contains(output, "7.0.0")
			},
		},
		{
			name:        "table format",
			flags:       map[string]string{"format": "table"},
			expectError: false,
			checkOutput: func(output string) bool {
				// Table format should contain headers and numbered list
				return strings.Contains(output, "Available Aerospike Schema Versions:") &&
					strings.Contains(output, "Total:") &&
					strings.Contains(output, "6.4.0") &&
					strings.Contains(output, "7.0.0")
			},
		},
		{
			name:        "invalid format",
			flags:       map[string]string{"format": "invalid"},
			expectError: false, // Command should still run but use default behavior
			checkOutput: func(output string) bool {
				// Should default to simple format
				return strings.Contains(output, "6.4.0") && strings.Contains(output, "7.0.0")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newListSchemasCmd()

			// Set flags
			for key, value := range tc.flags {
				cmd.Flags().Set(key, value)
			}

			// Capture output
			var output strings.Builder
			cmd.SetOut(&output)
			cmd.SetErr(&output)

			// Execute command
			err := cmd.RunE(cmd, []string{})

			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			} else if !tc.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tc.checkOutput != nil && !tc.checkOutput(output.String()) {
				t.Errorf("Output check failed. Output: %s", output.String())
			}
		})
	}
}

func TestListSchemasDoesNotIncludeReadme(t *testing.T) {
	cmd := newListSchemasCmd()

	var output strings.Builder
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Errorf("Command failed: %v", err)
	}

	outputStr := output.String()
	if strings.Contains(strings.ToLower(outputStr), "readme") {
		t.Errorf("Output should not contain README files, but got: %s", outputStr)
	}
}
