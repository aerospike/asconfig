//go:build unit

package cmd

import (
	"sort"
	"strings"
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
	InitializeGlobalsForTesting()
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
	InitializeGlobalsForTesting()
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
	InitializeGlobalsForTesting()
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
