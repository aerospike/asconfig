package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	asConf "github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/aerospike/asconfig/schema"

	"github.com/spf13/cobra"
)

const (
	schemaDiffArgMin = 2
	schemaDiffArgMax = 2
)

var (
	errSchemaDiffTooFewArgs  = fmt.Errorf("diff-schemas requires at least %d version arguments", schemaDiffArgMin)
	errSchemaDiffTooManyArgs = fmt.Errorf("diff-schemas requires no more than %d version arguments", schemaDiffArgMax)
	errInvalidSchemaVersion  = fmt.Errorf("invalid schema version")
)

func init() {
	rootCmd.AddCommand(diffSchemasCmd)
}

var diffSchemasCmd = newDiffSchemasCmd()

func newDiffSchemasCmd() *cobra.Command {
	res := &cobra.Command{
		Use:   "diff-schemas [flags] <version1> <version2>",
		Short: "Compare Aerospike configuration schemas between two versions.",
		Long: `Diff-schemas compares the configuration schemas between two Aerospike versions,
				showing which configuration parameters are added, removed, or changed.
				This helps understand what configuration options are going away or
				staying when upgrading between Aerospike versions.
				
				Examples:
				  asconfig diff-schemas 6.4.0 7.0.0
				  asconfig diff-schemas 5.7.0 6.4.0 --verbose`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Debug("Running diff-schemas command")

			if len(args) < schemaDiffArgMin {
				return errSchemaDiffTooFewArgs
			}

			if len(args) > schemaDiffArgMax {
				return errSchemaDiffTooManyArgs
			}

			version1 := args[0]
			version2 := args[1]

			logger.Debugf("Comparing schema from version %s to version %s", version1, version2)

			// Validate versions are supported
			supported1, err := asConf.IsSupportedVersion(version1)
			if err != nil {
				return errors.Join(errInvalidSchemaVersion, fmt.Errorf("version %s: %w", version1, err))
			}
			if !supported1 {
				return fmt.Errorf("version %s is not supported", version1)
			}

			supported2, err := asConf.IsSupportedVersion(version2)
			if err != nil {
				return errors.Join(errInvalidSchemaVersion, fmt.Errorf("version %s: %w", version2, err))
			}
			if !supported2 {
				return fmt.Errorf("version %s is not supported", version2)
			}

			// Load schemas
			schemaMap, err := schema.NewSchemaMap()
			if err != nil {
				return fmt.Errorf("failed to load schema map: %w", err)
			}

			schema1, exists := schemaMap[version1]
			if !exists {
				return fmt.Errorf("schema for version %s not found", version1)
			}

			schema2, exists := schemaMap[version2]
			if !exists {
				return fmt.Errorf("schema for version %s not found", version2)
			}

			// Parse schemas
			var parsedSchema1, parsedSchema2 map[string]interface{}
			if err := json.Unmarshal([]byte(schema1), &parsedSchema1); err != nil {
				return fmt.Errorf("failed to parse schema for version %s: %w", version1, err)
			}

			if err := json.Unmarshal([]byte(schema2), &parsedSchema2); err != nil {
				return fmt.Errorf("failed to parse schema for version %s: %w", version2, err)
			}

			// Get flags
			verbose, _ := cmd.Flags().GetBool("verbose")
			filterPath, _ := cmd.Flags().GetString("filter-path")

			// Compare schemas
			diffs := compareSchemas(parsedSchema1, parsedSchema2, verbose, filterPath)

			// Output results
			if len(diffs) == 0 {
				fmt.Printf("No differences found between schema versions %s and %s\n", version1, version2)
				return nil
			}

			fmt.Printf("Schema differences from version %s to version %s:\n", version1, version2)
			fmt.Printf("Legend: '+' = added, '-' = removed, '~' = changed\n\n")

			for _, diff := range diffs {
				fmt.Print(diff)
			}

			return nil
		},
	}

	res.Flags().BoolP("verbose", "v", false, "Show detailed information about property changes (type, default values, etc.)")
	res.Flags().StringP("filter-path", "f", "", "Filter results to only show properties under the specified path (e.g., 'service', 'namespaces')")

	return res
}

// compareSchemas compares two schemas and returns a list of differences
func compareSchemas(schema1, schema2 map[string]interface{}, showDetails bool, filterPath string) []string {
	var diffs []string
	added, removed, changed := 0, 0, 0

	// Track visited paths to avoid duplicates
	visited := make(map[string]bool)

	// Compare properties from both schemas
	if props1, ok := schema1["properties"].(map[string]interface{}); ok {
		if props2, ok := schema2["properties"].(map[string]interface{}); ok {
			comparePaths(props1, props2, "", &diffs, &added, &removed, &changed, visited, showDetails, filterPath)
		} else {
			// Schema2 has no properties, all props1 are removed
			extractRemovedPaths(props1, "", &diffs, &removed, visited, showDetails, filterPath)
		}
	}

	// Check for properties only in schema2 (added)
	if props2, ok := schema2["properties"].(map[string]interface{}); ok {
		if _, ok := schema1["properties"].(map[string]interface{}); !ok {
			// Schema1 has no properties, all props2 are added
			extractAddedPaths(props2, "", &diffs, &added, visited, showDetails, filterPath)
		}
	}

	// Add summary
	if len(diffs) > 0 {
		diffs = append(diffs, fmt.Sprintf("\nSummary: %d added, %d removed, %d changed\n", added, removed, changed))
	}

	return diffs
}

// comparePaths recursively compares property paths in two schemas
func comparePaths(props1, props2 map[string]interface{}, prefix string, diffs *[]string, added, removed, changed *int, visited map[string]bool, showDetails bool, filterPath string) {
	// Get all unique keys from both schemas
	allKeys := make(map[string]bool)
	for key := range props1 {
		allKeys[key] = true
	}
	for key := range props2 {
		allKeys[key] = true
	}

	// Sort keys for consistent output
	var sortedKeys []string
	for key := range allKeys {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

	for _, key := range sortedKeys {
		fullPath := key
		if prefix != "" {
			fullPath = prefix + "." + key
		}

		// Skip if not matching filter
		if filterPath != "" && !strings.HasPrefix(fullPath, filterPath) {
			continue
		}

		// Skip if already visited
		if visited[fullPath] {
			continue
		}
		visited[fullPath] = true

		prop1, exists1 := props1[key]
		prop2, exists2 := props2[key]

		if !exists1 && exists2 {
			// Property added
			*added++
			*diffs = append(*diffs, formatPropertyDiff("+", fullPath, nil, prop2, showDetails)+"\n")

			// Recursively handle nested properties
			if prop2Map, ok := prop2.(map[string]interface{}); ok {
				handleNestedProperties(prop2Map, fullPath, "+", diffs, added, removed, changed, visited, showDetails, filterPath)
			}

		} else if exists1 && !exists2 {
			// Property removed
			*removed++
			*diffs = append(*diffs, formatPropertyDiff("-", fullPath, prop1, nil, showDetails)+"\n")

			// Recursively handle nested properties
			if prop1Map, ok := prop1.(map[string]interface{}); ok {
				handleNestedProperties(prop1Map, fullPath, "-", diffs, added, removed, changed, visited, showDetails, filterPath)
			}

		} else if exists1 && exists2 {
			// Property exists in both - check for changes
			if !reflect.DeepEqual(prop1, prop2) {
				*changed++
				*diffs = append(*diffs, formatPropertyDiff("~", fullPath, prop1, prop2, showDetails)+"\n")
			}

			// Recursively compare nested properties
			if prop1Map, ok := prop1.(map[string]interface{}); ok {
				if prop2Map, ok := prop2.(map[string]interface{}); ok {
					if nestedProps1, ok := prop1Map["properties"].(map[string]interface{}); ok {
						if nestedProps2, ok := prop2Map["properties"].(map[string]interface{}); ok {
							comparePaths(nestedProps1, nestedProps2, fullPath, diffs, added, removed, changed, visited, showDetails, filterPath)
						}
					}

					// Handle array items
					handleArrayItems(prop1Map, prop2Map, fullPath, diffs, added, removed, changed, visited, showDetails, filterPath)

					// Handle oneOf arrays
					handleOneOfArrays(prop1Map, prop2Map, fullPath, diffs, added, removed, changed, visited, showDetails, filterPath)
				}
			}
		}
	}
}

// handleNestedProperties processes nested properties for added/removed items
func handleNestedProperties(propMap map[string]interface{}, parentPath, operation string, diffs *[]string, added, removed, changed *int, visited map[string]bool, showDetails bool, filterPath string) {
	if nestedProps, ok := propMap["properties"].(map[string]interface{}); ok {
		for key, value := range nestedProps {
			fullPath := parentPath + "." + key
			if filterPath == "" || strings.HasPrefix(fullPath, filterPath) {
				if !visited[fullPath] {
					visited[fullPath] = true
					if operation == "+" {
						*added++
					} else {
						*removed++
					}
					*diffs = append(*diffs, formatPropertyDiff(operation, fullPath, nil, value, showDetails)+"\n")

					if valueMap, ok := value.(map[string]interface{}); ok {
						handleNestedProperties(valueMap, fullPath, operation, diffs, added, removed, changed, visited, showDetails, filterPath)
					}
				}
			}
		}
	}

	// Handle array items
	if items, ok := propMap["items"].(map[string]interface{}); ok {
		itemsPath := parentPath + ".items"
		if filterPath == "" || strings.HasPrefix(itemsPath, filterPath) {
			if !visited[itemsPath] {
				visited[itemsPath] = true
				if operation == "+" {
					*added++
				} else {
					*removed++
				}
				*diffs = append(*diffs, formatPropertyDiff(operation, itemsPath, nil, items, showDetails)+"\n")
				handleNestedProperties(items, itemsPath, operation, diffs, added, removed, changed, visited, showDetails, filterPath)
			}
		}
	}

	// Handle oneOf arrays
	if oneOf, ok := propMap["oneOf"].([]interface{}); ok {
		for i, oneOfItem := range oneOf {
			if oneOfMap, ok := oneOfItem.(map[string]interface{}); ok {
				oneOfPath := fmt.Sprintf("%s.oneOf.%d", parentPath, i)
				if filterPath == "" || strings.HasPrefix(oneOfPath, filterPath) {
					if !visited[oneOfPath] {
						visited[oneOfPath] = true
						if operation == "+" {
							*added++
						} else {
							*removed++
						}
						*diffs = append(*diffs, formatPropertyDiff(operation, oneOfPath, nil, oneOfMap, showDetails)+"\n")
						handleNestedProperties(oneOfMap, oneOfPath, operation, diffs, added, removed, changed, visited, showDetails, filterPath)
					}
				}
			}
		}
	}
}

// handleArrayItems compares array items between two properties
func handleArrayItems(prop1Map, prop2Map map[string]interface{}, parentPath string, diffs *[]string, added, removed, changed *int, visited map[string]bool, showDetails bool, filterPath string) {
	items1, exists1 := prop1Map["items"].(map[string]interface{})
	items2, exists2 := prop2Map["items"].(map[string]interface{})

	itemsPath := parentPath + ".items"

	if filterPath != "" && !strings.HasPrefix(itemsPath, filterPath) {
		return
	}

	if !exists1 && exists2 {
		// Items added
		if !visited[itemsPath] {
			visited[itemsPath] = true
			*added++
			*diffs = append(*diffs, formatPropertyDiff("+", itemsPath, nil, items2, showDetails)+"\n")
			handleNestedProperties(items2, itemsPath, "+", diffs, added, removed, changed, visited, showDetails, filterPath)
		}
	} else if exists1 && !exists2 {
		// Items removed
		if !visited[itemsPath] {
			visited[itemsPath] = true
			*removed++
			*diffs = append(*diffs, formatPropertyDiff("-", itemsPath, items1, nil, showDetails)+"\n")
			handleNestedProperties(items1, itemsPath, "-", diffs, added, removed, changed, visited, showDetails, filterPath)
		}
	} else if exists1 && exists2 {
		// Items exist in both - compare them
		if !reflect.DeepEqual(items1, items2) && !visited[itemsPath] {
			visited[itemsPath] = true
			*changed++
			*diffs = append(*diffs, formatPropertyDiff("~", itemsPath, items1, items2, showDetails)+"\n")
		}

		// Compare nested properties within items
		if itemProps1, ok := items1["properties"].(map[string]interface{}); ok {
			if itemProps2, ok := items2["properties"].(map[string]interface{}); ok {
				comparePaths(itemProps1, itemProps2, itemsPath, diffs, added, removed, changed, visited, showDetails, filterPath)
			}
		}
	}
}

// handleOneOfArrays compares oneOf arrays between two properties
func handleOneOfArrays(prop1Map, prop2Map map[string]interface{}, parentPath string, diffs *[]string, added, removed, changed *int, visited map[string]bool, showDetails bool, filterPath string) {
	oneOf1, exists1 := prop1Map["oneOf"].([]interface{})
	oneOf2, exists2 := prop2Map["oneOf"].([]interface{})

	if !exists1 && !exists2 {
		return
	}

	maxLen := 0
	if exists1 && len(oneOf1) > maxLen {
		maxLen = len(oneOf1)
	}
	if exists2 && len(oneOf2) > maxLen {
		maxLen = len(oneOf2)
	}

	for i := 0; i < maxLen; i++ {
		oneOfPath := fmt.Sprintf("%s.oneOf.%d", parentPath, i)

		if filterPath != "" && !strings.HasPrefix(oneOfPath, filterPath) {
			continue
		}

		if visited[oneOfPath] {
			continue
		}

		var item1, item2 map[string]interface{}
		exists1Item := exists1 && i < len(oneOf1)
		exists2Item := exists2 && i < len(oneOf2)

		if exists1Item {
			item1, _ = oneOf1[i].(map[string]interface{})
		}
		if exists2Item {
			item2, _ = oneOf2[i].(map[string]interface{})
		}

		if !exists1Item && exists2Item {
			// OneOf item added
			visited[oneOfPath] = true
			*added++
			*diffs = append(*diffs, formatPropertyDiff("+", oneOfPath, nil, item2, showDetails)+"\n")
			if item2 != nil {
				handleNestedProperties(item2, oneOfPath, "+", diffs, added, removed, changed, visited, showDetails, filterPath)
			}
		} else if exists1Item && !exists2Item {
			// OneOf item removed
			visited[oneOfPath] = true
			*removed++
			*diffs = append(*diffs, formatPropertyDiff("-", oneOfPath, item1, nil, showDetails)+"\n")
			if item1 != nil {
				handleNestedProperties(item1, oneOfPath, "-", diffs, added, removed, changed, visited, showDetails, filterPath)
			}
		} else if exists1Item && exists2Item && item1 != nil && item2 != nil {
			// OneOf item exists in both - compare them
			if !reflect.DeepEqual(item1, item2) {
				visited[oneOfPath] = true
				*changed++
				*diffs = append(*diffs, formatPropertyDiff("~", oneOfPath, item1, item2, showDetails)+"\n")
			}

			// Compare nested properties within oneOf item
			if itemProps1, ok := item1["properties"].(map[string]interface{}); ok {
				if itemProps2, ok := item2["properties"].(map[string]interface{}); ok {
					comparePaths(itemProps1, itemProps2, oneOfPath, diffs, added, removed, changed, visited, showDetails, filterPath)
				}
			}
		}
	}
}

// formatPropertyDiff formats a property difference for output
func formatPropertyDiff(operation, path string, oldProp, newProp interface{}, showDetails bool) string {
	diff := fmt.Sprintf("%s %s", operation, path)

	if !showDetails {
		return diff
	}

	// Extract property details for verbose output
	var details []string

	if operation == "+" && newProp != nil {
		if propMap, ok := newProp.(map[string]interface{}); ok {
			if propType, ok := propMap["type"].(string); ok {
				details = append(details, fmt.Sprintf("type: %s", propType))
			}
			if defaultVal, ok := propMap["default"]; ok {
				details = append(details, fmt.Sprintf("default: %v", defaultVal))
			}
			if enterpriseOnly, ok := propMap["enterpriseOnly"].(bool); ok && enterpriseOnly {
				details = append(details, "enterprise-only")
			}
		}
	} else if operation == "-" && oldProp != nil {
		if propMap, ok := oldProp.(map[string]interface{}); ok {
			if propType, ok := propMap["type"].(string); ok {
				details = append(details, fmt.Sprintf("was type: %s", propType))
			}
			if defaultVal, ok := propMap["default"]; ok {
				details = append(details, fmt.Sprintf("default: %v", defaultVal))
			}
			if enterpriseOnly, ok := propMap["enterpriseOnly"].(bool); ok && enterpriseOnly {
				details = append(details, "enterprise-only")
			}
		}
	} else if operation == "~" && oldProp != nil && newProp != nil {
		oldMap, oldOk := oldProp.(map[string]interface{})
		newMap, newOk := newProp.(map[string]interface{})

		if oldOk && newOk {
			// Compare types
			if oldType, ok1 := oldMap["type"].(string); ok1 {
				if newType, ok2 := newMap["type"].(string); ok2 && oldType != newType {
					details = append(details, fmt.Sprintf("type: %s → %s", oldType, newType))
				}
			}

			// Compare defaults
			if oldDefault, ok1 := oldMap["default"]; ok1 {
				if newDefault, ok2 := newMap["default"]; ok2 && !reflect.DeepEqual(oldDefault, newDefault) {
					details = append(details, fmt.Sprintf("default: %v → %v", oldDefault, newDefault))
				}
			} else if newDefault, ok2 := newMap["default"]; ok2 {
				details = append(details, fmt.Sprintf("default: <none> → %v", newDefault))
			}

			// Compare enterprise-only
			oldEnt, oldEntOk := oldMap["enterpriseOnly"].(bool)
			newEnt, newEntOk := newMap["enterpriseOnly"].(bool)
			if oldEntOk && newEntOk && oldEnt != newEnt {
				details = append(details, fmt.Sprintf("enterprise-only: %v → %v", oldEnt, newEnt))
			} else if !oldEntOk && newEntOk && newEnt {
				details = append(details, "enterprise-only: false → true")
			} else if oldEntOk && !newEntOk && oldEnt {
				details = append(details, "enterprise-only: true → false")
			}

			// Compare dynamic
			oldDyn, oldDynOk := oldMap["dynamic"].(bool)
			newDyn, newDynOk := newMap["dynamic"].(bool)
			if oldDynOk && newDynOk && oldDyn != newDyn {
				details = append(details, fmt.Sprintf("dynamic: %v → %v", oldDyn, newDyn))
			}
		}
	}

	if len(details) > 0 {
		diff += fmt.Sprintf(" (%s)", strings.Join(details, ", "))
	}

	return diff
}

// extractAddedPaths extracts all paths from a schema as "added" properties
func extractAddedPaths(props map[string]interface{}, prefix string, diffs *[]string, added *int, visited map[string]bool, showDetails bool, filterPath string) {
	for key, value := range props {
		fullPath := key
		if prefix != "" {
			fullPath = prefix + "." + key
		}

		if filterPath != "" && !strings.HasPrefix(fullPath, filterPath) {
			continue
		}

		if !visited[fullPath] {
			visited[fullPath] = true
			*added++
			*diffs = append(*diffs, formatPropertyDiff("+", fullPath, nil, value, showDetails)+"\n")

			if valueMap, ok := value.(map[string]interface{}); ok {
				handleNestedProperties(valueMap, fullPath, "+", diffs, added, nil, nil, visited, showDetails, filterPath)
			}
		}
	}
}

// extractRemovedPaths extracts all paths from a schema as "removed" properties
func extractRemovedPaths(props map[string]interface{}, prefix string, diffs *[]string, removed *int, visited map[string]bool, showDetails bool, filterPath string) {
	for key, value := range props {
		fullPath := key
		if prefix != "" {
			fullPath = prefix + "." + key
		}

		if filterPath != "" && !strings.HasPrefix(fullPath, filterPath) {
			continue
		}

		if !visited[fullPath] {
			visited[fullPath] = true
			*removed++
			*diffs = append(*diffs, formatPropertyDiff("-", fullPath, value, nil, showDetails)+"\n")

			if valueMap, ok := value.(map[string]interface{}); ok {
				handleNestedProperties(valueMap, fullPath, "-", diffs, nil, removed, nil, visited, showDetails, filterPath)
			}
		}
	}
}
