//go:build unit

package cmd

import (
	"fmt"
	"strings"
	"testing"
)

type runTestSchemaDiff struct {
	flags       []string
	arguments   []string
	expectError bool
}

var testSchemaDiffArgs = []runTestSchemaDiff{
	{
		flags:       []string{},
		arguments:   []string{},
		expectError: true, // too few args
	},
	{
		flags:       []string{},
		arguments:   []string{"6.4.0"},
		expectError: true, // too few args
	},
	{
		flags:       []string{},
		arguments:   []string{"6.4.0", "7.0.0"},
		expectError: false, // valid
	},
	{
		flags:       []string{},
		arguments:   []string{"6.4.0", "7.0.0", "8.0.0"},
		expectError: true, // too many args
	},
	{
		flags:       []string{},
		arguments:   []string{"invalid.version", "7.0.0"},
		expectError: true, // invalid version1
	},
	{
		flags:       []string{},
		arguments:   []string{"6.4.0", "invalid.version"},
		expectError: true, // invalid version2
	},
	{
		flags:       []string{"--verbose"},
		arguments:   []string{"6.4.0", "7.0.0"},
		expectError: false, // valid with details
	},
	{
		flags:       []string{"--filter-path", "service"},
		arguments:   []string{"6.4.0", "7.0.0"},
		expectError: false, // valid with filter
	},
	{
		flags:       []string{"-v", "-f", "namespaces"},
		arguments:   []string{"6.4.0", "7.0.0"},
		expectError: false, // valid with all flags
	},
}

func TestRunESchemaDiff(t *testing.T) {
	for i, test := range testSchemaDiffArgs {
		t.Run(fmt.Sprintf("test_%d", i), func(t *testing.T) {
			cmd := newSchemaDiffCmd()

			// Set flags
			for j := 0; j < len(test.flags); j++ {
				if test.flags[j] == "--verbose" || test.flags[j] == "-v" {
					cmd.Flags().Set("verbose", "true")
				} else if test.flags[j] == "--filter-path" || test.flags[j] == "-f" {
					if j+1 < len(test.flags) {
						cmd.Flags().Set("filter-path", test.flags[j+1])
						j++ // skip next flag as it's the value
					}
				}
			}

			// Execute command
			err := cmd.RunE(cmd, test.arguments)

			if test.expectError && err == nil {
				t.Errorf("Expected error but got none")
			} else if !test.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestExtractConfigProperties(t *testing.T) {
	testSchema := map[string]interface{}{
		"properties": map[string]interface{}{
			"service": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"cluster-name": map[string]interface{}{
						"type":        "string",
						"default":     "",
						"description": "Name of the cluster",
						"dynamic":     true,
					},
					"auto-pin": map[string]interface{}{
						"type":    "string",
						"default": "none",
						"enum":    []interface{}{"none", "cpu", "numa", "adq"},
					},
				},
			},
			"namespaces": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Namespace name",
						},
						"storage-engine": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"type": map[string]interface{}{
									"type":    "string",
									"default": "memory",
									"enum":    []interface{}{"memory", "device"},
								},
								"devices": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"type": "string",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	props := extractConfigProperties(testSchema)

	// Check that service properties are extracted
	if _, exists := props["service"]; !exists {
		t.Error("Expected 'service' property to be extracted")
	}

	if _, exists := props["service.cluster-name"]; !exists {
		t.Error("Expected 'service.cluster-name' property to be extracted")
	}

	if _, exists := props["service.auto-pin"]; !exists {
		t.Error("Expected 'service.auto-pin' property to be extracted")
	}

	if _, exists := props["namespaces"]; !exists {
		t.Error("Expected 'namespaces' property to be extracted")
	}

	// Check that namespace items properties are extracted
	if _, exists := props["namespaces.items.name"]; !exists {
		t.Error("Expected 'namespaces.items.name' property to be extracted")
	}

	if _, exists := props["namespaces.items.storage-engine"]; !exists {
		t.Error("Expected 'namespaces.items.storage-engine' property to be extracted")
	}

	if _, exists := props["namespaces.items.storage-engine.type"]; !exists {
		t.Error("Expected 'namespaces.items.storage-engine.type' property to be extracted")
	}

	if _, exists := props["namespaces.items.storage-engine.devices"]; !exists {
		t.Error("Expected 'namespaces.items.storage-engine.devices' property to be extracted")
	}

	// Check property details
	clusterName := props["service.cluster-name"]
	if clusterName.Type != "string" {
		t.Errorf("Expected cluster-name type to be 'string', got '%s'", clusterName.Type)
	}

	if clusterName.Default != "" {
		t.Errorf("Expected cluster-name default to be empty string, got '%v'", clusterName.Default)
	}

	if !clusterName.Dynamic {
		t.Error("Expected cluster-name to be dynamic")
	}

	autoPin := props["service.auto-pin"]
	if len(autoPin.Enum) != 4 {
		t.Errorf("Expected auto-pin to have 4 enum values, got %d", len(autoPin.Enum))
	}

	// Check namespace items properties
	namespaceName := props["namespaces.items.name"]
	if namespaceName.Type != "string" {
		t.Errorf("Expected namespace name type to be 'string', got '%s'", namespaceName.Type)
	}

	storageEngineType := props["namespaces.items.storage-engine.type"]
	if storageEngineType.Type != "string" {
		t.Errorf("Expected storage-engine type to be 'string', got '%s'", storageEngineType.Type)
	}

	if storageEngineType.Default != "memory" {
		t.Errorf("Expected storage-engine type default to be 'memory', got '%v'", storageEngineType.Default)
	}
}

func TestExtractConfigPropertiesWithComplexArrays(t *testing.T) {
	// Test with more complex nested structure similar to real Aerospike schema
	testSchema := map[string]interface{}{
		"properties": map[string]interface{}{
			"namespaces": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"storage-engine": map[string]interface{}{
							"oneOf": []interface{}{
								map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"type": map[string]interface{}{
											"type":    "string",
											"default": "memory",
											"enum":    []interface{}{"memory"},
										},
									},
								},
								map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"type": map[string]interface{}{
											"type":    "string",
											"default": "device",
											"enum":    []interface{}{"device"},
										},
										"devices": map[string]interface{}{
											"type": "array",
											"items": map[string]interface{}{
												"type": "string",
											},
										},
										"filesize": map[string]interface{}{
											"type":    "integer",
											"default": 0,
											"minimum": 1048576,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	props := extractConfigProperties(testSchema)

	// Check that oneOf structures are handled
	if _, exists := props["namespaces.items.storage-engine"]; !exists {
		t.Error("Expected 'namespaces.items.storage-engine' property to be extracted")
	}

	// Check that oneOf nested properties are extracted
	if _, exists := props["namespaces.items.storage-engine.oneOf.0.type"]; !exists {
		t.Error("Expected 'namespaces.items.storage-engine.oneOf.0.type' property to be extracted")
	}

	if _, exists := props["namespaces.items.storage-engine.oneOf.1.type"]; !exists {
		t.Error("Expected 'namespaces.items.storage-engine.oneOf.1.type' property to be extracted")
	}

	if _, exists := props["namespaces.items.storage-engine.oneOf.1.devices"]; !exists {
		t.Error("Expected 'namespaces.items.storage-engine.oneOf.1.devices' property to be extracted")
	}

	if _, exists := props["namespaces.items.storage-engine.oneOf.1.filesize"]; !exists {
		t.Error("Expected 'namespaces.items.storage-engine.oneOf.1.filesize' property to be extracted")
	}
}

func TestExtractConfigPropertiesEdgeCases(t *testing.T) {
	// Test edge cases
	testCases := []struct {
		name   string
		schema map[string]interface{}
		check  func(map[string]PropertyInfo) bool
	}{
		{
			name:   "empty schema",
			schema: map[string]interface{}{},
			check: func(props map[string]PropertyInfo) bool {
				return len(props) == 0
			},
		},
		{
			name: "schema without properties",
			schema: map[string]interface{}{
				"type": "object",
			},
			check: func(props map[string]PropertyInfo) bool {
				return len(props) == 0
			},
		},
		{
			name: "schema with non-object properties",
			schema: map[string]interface{}{
				"properties": "not an object",
			},
			check: func(props map[string]PropertyInfo) bool {
				return len(props) == 0
			},
		},
		{
			name: "deeply nested structure",
			schema: map[string]interface{}{
				"properties": map[string]interface{}{
					"level1": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"level2": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"level3": map[string]interface{}{
										"type":    "string",
										"default": "deep",
									},
								},
							},
						},
					},
				},
			},
			check: func(props map[string]PropertyInfo) bool {
				_, exists := props["level1.level2.level3"]
				return exists
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			props := extractConfigProperties(tc.schema)
			if !tc.check(props) {
				t.Errorf("Test case '%s' failed", tc.name)
			}
		})
	}
}

func TestCompareSchemasDetailed(t *testing.T) {
	props1 := map[string]PropertyInfo{
		"service.cluster-name": {
			Type:    "string",
			Default: "",
			Dynamic: true,
		},
		"service.removed-prop": {
			Type:    "boolean",
			Default: false,
		},
		"service.changed-prop": {
			Type:    "integer",
			Default: 10,
		},
		"namespaces.items.name": {
			Type:    "string",
			Default: "",
		},
		"namespaces.items.storage-engine.type": {
			Type:    "string",
			Default: "memory",
			Enum:    []interface{}{"memory", "device"},
		},
	}

	props2 := map[string]PropertyInfo{
		"service.cluster-name": {
			Type:    "string",
			Default: "",
			Dynamic: true,
		},
		"service.new-prop": {
			Type:    "string",
			Default: "new",
		},
		"service.changed-prop": {
			Type:    "integer",
			Default: 20, // Changed default value
		},
		"namespaces.items.name": {
			Type:    "string",
			Default: "",
		},
		"namespaces.items.storage-engine.type": {
			Type:    "string",
			Default: "device", // Changed default
			Enum:    []interface{}{"memory", "device"},
		},
		"namespaces.items.new-namespace-prop": {
			Type:    "boolean",
			Default: false,
		},
	}

	diffs := compareSchemasDetailed(props1, props2, false, "")

	if len(diffs) == 0 {
		t.Error("Expected differences to be found")
	}

	// Check that we have the expected changes
	diffText := strings.Join(diffs, "")

	if !strings.Contains(diffText, "+ service.new-prop") {
		t.Error("Expected to find added property 'service.new-prop'")
	}

	if !strings.Contains(diffText, "- service.removed-prop") {
		t.Error("Expected to find removed property 'service.removed-prop'")
	}

	if !strings.Contains(diffText, "~ service.changed-prop") {
		t.Error("Expected to find changed property 'service.changed-prop'")
	}

	if !strings.Contains(diffText, "+ namespaces.items.new-namespace-prop") {
		t.Error("Expected to find added namespace property 'namespaces.items.new-namespace-prop'")
	}

	if !strings.Contains(diffText, "~ namespaces.items.storage-engine.type") {
		t.Error("Expected to find changed namespace property 'namespaces.items.storage-engine.type'")
	}

	// Test with filter
	filteredDiffs := compareSchemasDetailed(props1, props2, false, "service.cluster")
	filteredText := strings.Join(filteredDiffs, "")

	if strings.Contains(filteredText, "service.new-prop") {
		t.Error("Expected filtered results to not contain 'service.new-prop'")
	}

	// Test with namespace filter
	namespaceDiffs := compareSchemasDetailed(props1, props2, false, "namespaces")
	namespaceText := strings.Join(namespaceDiffs, "")

	if !strings.Contains(namespaceText, "namespaces.items.new-namespace-prop") {
		t.Error("Expected namespace filtered results to contain 'namespaces.items.new-namespace-prop'")
	}

	if strings.Contains(namespaceText, "service.new-prop") {
		t.Error("Expected namespace filtered results to not contain 'service.new-prop'")
	}
}

func TestCompareSchemasDetailedWithDetailedOutput(t *testing.T) {
	props1 := map[string]PropertyInfo{
		"service.debug-allocations": {
			Type:    "string",
			Default: "none",
			Enum:    []interface{}{"none", "transient", "persistent", "all"},
		},
		"service.feature-key-file": {
			Type:    "string",
			Default: "/opt/aerospike/data/features.conf",
		},
	}

	props2 := map[string]PropertyInfo{
		"service.debug-allocations": {
			Type:    "boolean",
			Default: false,
		},
		"service.feature-key-file": {
			Type:           "string",
			Default:        "/etc/aerospike/features.conf",
			EnterpriseOnly: true,
		},
	}

	diffs := compareSchemasDetailed(props1, props2, true, "")
	diffText := strings.Join(diffs, "")

	// Check that detailed output includes type changes
	if !strings.Contains(diffText, "type: string → boolean") {
		t.Error("Expected detailed output to show type change")
	}

	// Check that detailed output includes default changes
	if !strings.Contains(diffText, "default: none → false") {
		t.Error("Expected detailed output to show default change")
	}

	if !strings.Contains(diffText, "default: /opt/aerospike/data/features.conf → /etc/aerospike/features.conf") {
		t.Error("Expected detailed output to show default path change")
	}

	// Check that detailed output includes enterprise-only changes
	if !strings.Contains(diffText, "enterprise-only: false → true") {
		t.Error("Expected detailed output to show enterprise-only change")
	}
}

func TestRealWorldSchemaComparison(t *testing.T) {
	// Test with actual schema structure similar to what we have in the real files
	schema1 := map[string]interface{}{
		"properties": map[string]interface{}{
			"service": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"cluster-name": map[string]interface{}{
						"type":    "string",
						"default": "",
					},
				},
			},
			"namespaces": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type": "string",
						},
						"storage-engine": map[string]interface{}{
							"oneOf": []interface{}{
								map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"type": map[string]interface{}{
											"type":    "string",
											"default": "memory",
											"enum":    []interface{}{"memory"},
										},
									},
								},
								map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"type": map[string]interface{}{
											"type":    "string",
											"default": "device",
											"enum":    []interface{}{"device"},
										},
										"devices": map[string]interface{}{
											"type": "array",
											"items": map[string]interface{}{
												"type": "string",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	schema2 := map[string]interface{}{
		"properties": map[string]interface{}{
			"service": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"cluster-name": map[string]interface{}{
						"type":    "string",
						"default": "",
					},
				},
			},
			"namespaces": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type": "string",
						},
						"storage-engine": map[string]interface{}{
							"oneOf": []interface{}{
								map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"type": map[string]interface{}{
											"type":    "string",
											"default": "memory",
											"enum":    []interface{}{"memory"},
										},
									},
								},
								map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"type": map[string]interface{}{
											"type":    "string",
											"default": "device",
											"enum":    []interface{}{"device"},
										},
										"devices": map[string]interface{}{
											"type": "array",
											"items": map[string]interface{}{
												"type": "string",
											},
										},
										"filesize": map[string]interface{}{
											"type":    "integer",
											"default": 0,
											"minimum": 1048576,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	props1 := extractConfigProperties(schema1)
	props2 := extractConfigProperties(schema2)

	diffs := compareSchemasDetailed(props1, props2, false, "namespaces")

	if len(diffs) == 0 {
		t.Error("Expected to find differences in namespace storage-engine")
	}

	diffText := strings.Join(diffs, "")

	// Should detect the addition of filesize property
	if !strings.Contains(diffText, "filesize") {
		t.Error("Expected to find filesize property addition in namespace storage-engine")
	}
}
