//go:build unit

package cmd

import (
	"sort"
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

// Unit tests for the valuesEqual function and related improvements
func TestValuesEqual(t *testing.T) {
	testCases := []struct {
		name     string
		v1       any
		v2       any
		expected bool
	}{
		// Basic equality tests
		{
			name:     "identical strings",
			v1:       "test",
			v2:       "test",
			expected: true,
		},
		{
			name:     "different strings",
			v1:       "test1",
			v2:       "test2",
			expected: false,
		},
		{
			name:     "identical integers",
			v1:       42,
			v2:       42,
			expected: true,
		},
		{
			name:     "different integers",
			v1:       42,
			v2:       43,
			expected: false,
		},

		// Type conversion tests
		{
			name:     "int and string same value",
			v1:       42,
			v2:       "42",
			expected: true,
		},
		{
			name:     "float and string same value",
			v1:       3.14,
			v2:       "3.14",
			expected: true,
		},
		{
			name:     "boolean and string same value",
			v1:       true,
			v2:       "true",
			expected: true,
		},
		{
			name:     "boolean and string different case",
			v1:       true,
			v2:       "TRUE",
			expected: true,
		},

		// Slice comparison tests
		{
			name:     "identical string slices",
			v1:       []string{"a", "b", "c"},
			v2:       []string{"a", "b", "c"},
			expected: true,
		},
		{
			name:     "different string slices",
			v1:       []string{"a", "b", "c"},
			v2:       []string{"a", "b", "d"},
			expected: false,
		},
		{
			name:     "slice vs non-slice",
			v1:       []string{"a", "b"},
			v2:       "a b",
			expected: false,
		},
		{
			name:     "empty slices",
			v1:       []string{},
			v2:       []string{},
			expected: true,
		},

		// Numeric type conversion tests
		{
			name:     "int32 vs int64 same value",
			v1:       int32(100),
			v2:       int64(100),
			expected: true,
		},
		{
			name:     "float32 vs float64 same value",
			v1:       float32(3.14),
			v2:       float64(3.14),
			expected: true,
		},

		// Edge cases
		{
			name:     "nil values",
			v1:       nil,
			v2:       nil,
			expected: true,
		},
		{
			name:     "nil vs non-nil",
			v1:       nil,
			v2:       "test",
			expected: false,
		},
		{
			name:     "zero values",
			v1:       0,
			v2:       "0",
			expected: true,
		},
		{
			name:     "boolean false vs string",
			v1:       false,
			v2:       "false",
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := valuesEqual(tc.v1, tc.v2)
			if result != tc.expected {
				t.Errorf("valuesEqual(%v, %v) = %v, expected %v", tc.v1, tc.v2, result, tc.expected)
			}
		})
	}
}

func TestIsSlice(t *testing.T) {
	testCases := []struct {
		name     string
		value    any
		expected bool
	}{
		{
			name:     "string slice",
			value:    []string{"a", "b", "c"},
			expected: true,
		},
		{
			name:     "int slice",
			value:    []int{1, 2, 3},
			expected: true,
		},
		{
			name:     "empty slice",
			value:    []string{},
			expected: true,
		},
		{
			name:     "string (not slice)",
			value:    "test",
			expected: false,
		},
		{
			name:     "int (not slice)",
			value:    42,
			expected: false,
		},
		{
			name:     "nil",
			value:    nil,
			expected: false,
		},
		{
			name:     "map (not slice)",
			value:    map[string]int{"a": 1},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isSlice(tc.value)
			if result != tc.expected {
				t.Errorf("isSlice(%v) = %v, expected %v", tc.value, result, tc.expected)
			}
		})
	}
}

func TestSlicesEqual(t *testing.T) {
	testCases := []struct {
		name     string
		v1       any
		v2       any
		expected bool
	}{
		{
			name:     "identical string slices",
			v1:       []string{"a", "b", "c"},
			v2:       []string{"a", "b", "c"},
			expected: true,
		},
		{
			name:     "different string slices",
			v1:       []string{"a", "b", "c"},
			v2:       []string{"a", "b", "d"},
			expected: false,
		},
		{
			name:     "different length slices",
			v1:       []string{"a", "b"},
			v2:       []string{"a", "b", "c"},
			expected: false,
		},
		{
			name:     "empty slices",
			v1:       []string{},
			v2:       []string{},
			expected: true,
		},
		{
			name:     "slice vs non-slice",
			v1:       []string{"a", "b"},
			v2:       "test",
			expected: false,
		},
		{
			name:     "int slices",
			v1:       []int{1, 2, 3},
			v2:       []int{1, 2, 3},
			expected: true,
		},
		{
			name:     "different int slices",
			v1:       []int{1, 2, 3},
			v2:       []int{1, 2, 4},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := slicesEqual(tc.v1, tc.v2)
			if result != tc.expected {
				t.Errorf("slicesEqual(%v, %v) = %v, expected %v", tc.v1, tc.v2, result, tc.expected)
			}
		})
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
			expected: []string{}, // Should be equal due to type conversion
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
	testCases := []struct {
		name        string
		args        []string
		expectError bool
		errorType   string
	}{
		{
			name:        "valid single argument",
			args:        []string{"config.yaml"},
			expectError: true, // Will fail due to file not existing, but argument validation passes
			errorType:   "file_not_found",
		},
		{
			name:        "no arguments",
			args:        []string{},
			expectError: true,
			errorType:   "too_few_args",
		},
		{
			name:        "too many arguments",
			args:        []string{"config1.yaml", "config2.yaml"},
			expectError: true,
			errorType:   "too_many_args",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newDiffCmd()
			err := runServerDiff(cmd, tc.args)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}

				// Check specific error types
				switch tc.errorType {
				case "too_few_args":
					if err != errDiffServerTooFewArgs {
						t.Errorf("Expected errDiffServerTooFewArgs, got: %v", err)
					}
				case "too_many_args":
					if err != errDiffServerTooManyArgs {
						t.Errorf("Expected errDiffServerTooManyArgs, got: %v", err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// Test that validates the fix for the slice comparison panic
func TestSliceComparisonNoPanic(t *testing.T) {
	// This test ensures that the valuesEqual function doesn't panic when comparing slices
	testCases := []struct {
		name string
		v1   any
		v2   any
	}{
		{
			name: "string slices",
			v1:   []string{"a", "b", "c"},
			v2:   []string{"a", "b", "c"},
		},
		{
			name: "int slices",
			v1:   []int{1, 2, 3},
			v2:   []int{1, 2, 3},
		},
		{
			name: "slice vs non-slice",
			v1:   []string{"a", "b"},
			v2:   "test",
		},
		{
			name: "both different slices",
			v1:   []string{"a", "b"},
			v2:   []int{1, 2},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("valuesEqual panicked: %v", r)
				}
			}()

			// This should not panic
			result := valuesEqual(tc.v1, tc.v2)
			t.Logf("valuesEqual(%v, %v) = %v", tc.v1, tc.v2, result)
		})
	}
}

// Test that validates order-agnostic slice comparison
func TestSlicesEqualOrderAgnostic(t *testing.T) {
	testCases := []struct {
		name     string
		v1       any
		v2       any
		expected bool
	}{
		{
			name:     "same order string slices",
			v1:       []string{"a", "b", "c"},
			v2:       []string{"a", "b", "c"},
			expected: true,
		},
		{
			name:     "different order string slices",
			v1:       []string{"a", "b", "c"},
			v2:       []string{"c", "a", "b"},
			expected: true,
		},
		{
			name:     "different order int slices",
			v1:       []int{1, 2, 3},
			v2:       []int{3, 1, 2},
			expected: true,
		},
		{
			name:     "different order interface slices",
			v1:       []interface{}{"x", "y", "z"},
			v2:       []interface{}{"z", "x", "y"},
			expected: true,
		},
		{
			name:     "different elements",
			v1:       []string{"a", "b", "c"},
			v2:       []string{"a", "b", "d"},
			expected: false,
		},
		{
			name:     "different lengths",
			v1:       []string{"a", "b"},
			v2:       []string{"a", "b", "c"},
			expected: false,
		},
		{
			name:     "empty slices",
			v1:       []string{},
			v2:       []string{},
			expected: true,
		},
		{
			name:     "mixed types same values",
			v1:       []string{"1", "2", "3"},
			v2:       []int{3, 1, 2},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := slicesEqual(tc.v1, tc.v2)
			if result != tc.expected {
				t.Errorf("slicesEqual(%v, %v) = %v, expected %v", tc.v1, tc.v2, result, tc.expected)
			}
		})
	}
}
