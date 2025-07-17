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
	errSchemaDiffTooFewArgs  = fmt.Errorf("schema-diff requires at least %d version arguments", schemaDiffArgMin)
	errSchemaDiffTooManyArgs = fmt.Errorf("schema-diff requires no more than %d version arguments", schemaDiffArgMax)
	errInvalidSchemaVersion  = fmt.Errorf("invalid schema version")
)

func init() {
	rootCmd.AddCommand(schemaDiffCmd)
}

var schemaDiffCmd = newSchemaDiffCmd()

func newSchemaDiffCmd() *cobra.Command {
	res := &cobra.Command{
		Use:   "schema-diff [flags] <version1> <version2>",
		Short: "Compare Aerospike configuration schemas between two versions.",
		Long: `Schema-diff compares the configuration schemas between two Aerospike versions,
				showing which configuration parameters are added, removed, or changed.
				This helps understand what configuration options are going away or
				staying when upgrading between Aerospike versions.
				
				Examples:
				  asconfig schema-diff 6.4.0 7.0.0
				  asconfig schema-diff 5.7.0 6.4.0 --verbose`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Debug("Running schema-diff command")

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

			// Extract configuration properties
			props1 := extractConfigProperties(parsedSchema1)
			props2 := extractConfigProperties(parsedSchema2)

			// Get flags
			verbose, _ := cmd.Flags().GetBool("verbose")
			showDeprecated, _ := cmd.Flags().GetBool("show-deprecated")
			filterPath, _ := cmd.Flags().GetString("filter-path")

			// Compare schemas
			diffs := compareSchemasDetailed(props1, props2, verbose, showDeprecated, filterPath)

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
	res.Flags().BoolP("show-deprecated", "D", false, "Show deprecated properties")
	res.Flags().StringP("filter-path", "f", "", "Filter results to only show properties under the specified path (e.g., 'service', 'namespaces')")

	return res
}

// extractConfigProperties recursively extracts all configuration properties from a schema
func extractConfigProperties(schema map[string]interface{}) map[string]PropertyInfo {
	properties := make(map[string]PropertyInfo)

	if props, ok := schema["properties"].(map[string]interface{}); ok {
		extractPropertiesRecursive(props, "", properties)
	}

	return properties
}

// PropertyInfo holds information about a schema property
type PropertyInfo struct {
	Type           string
	Default        interface{}
	Description    string
	Dynamic        bool
	EnterpriseOnly bool
	Enum           []interface{}
	Minimum        interface{}
	Maximum        interface{}
	Required       bool
	Deprecated     bool
}

// extractPropertiesRecursive recursively extracts properties from nested objects
func extractPropertiesRecursive(props map[string]interface{}, prefix string, result map[string]PropertyInfo) {
	for key, value := range props {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		if propMap, ok := value.(map[string]interface{}); ok {
			info := PropertyInfo{}

			// Extract basic properties
			if t, ok := propMap["type"].(string); ok {
				info.Type = t
			}
			if def, ok := propMap["default"]; ok {
				info.Default = def
			}
			if desc, ok := propMap["description"].(string); ok {
				info.Description = desc
			}
			if dyn, ok := propMap["dynamic"].(bool); ok {
				info.Dynamic = dyn
			}
			if ent, ok := propMap["enterpriseOnly"].(bool); ok {
				info.EnterpriseOnly = ent
			}
			if dep, ok := propMap["deprecated"].(bool); ok {
				info.Deprecated = dep
			}
			if enum, ok := propMap["enum"].([]interface{}); ok {
				info.Enum = enum
			}
			if min, ok := propMap["minimum"]; ok {
				info.Minimum = min
			}
			if max, ok := propMap["maximum"]; ok {
				info.Maximum = max
			}

			result[fullKey] = info

			// Recursively process nested properties
			if nestedProps, ok := propMap["properties"].(map[string]interface{}); ok {
				extractPropertiesRecursive(nestedProps, fullKey, result)
			}

			// Handle array items - for cases like namespaces which are arrays
			if items, ok := propMap["items"].(map[string]interface{}); ok {
				itemsKey := fullKey + ".items"
				extractPropertiesRecursive(map[string]interface{}{"items": items}, fullKey, result)

				// If items has properties, process them
				if itemProps, ok := items["properties"].(map[string]interface{}); ok {
					extractPropertiesRecursive(itemProps, itemsKey, result)
				}
			}

			// Handle oneOf arrays
			if oneOf, ok := propMap["oneOf"].([]interface{}); ok {
				for i, oneOfItem := range oneOf {
					if oneOfMap, ok := oneOfItem.(map[string]interface{}); ok {
						oneOfKey := fmt.Sprintf("%s.oneOf.%d", fullKey, i)
						extractPropertiesRecursive(map[string]interface{}{fmt.Sprintf("%d", i): oneOfMap}, fullKey+".oneOf", result)

						// If oneOf item has properties, process them
						if oneOfProps, ok := oneOfMap["properties"].(map[string]interface{}); ok {
							extractPropertiesRecursive(oneOfProps, oneOfKey, result)
						}
					}
				}
			}
		}
	}
}

// compareSchemasDetailed compares two sets of schema properties and returns detailed differences
func compareSchemasDetailed(props1, props2 map[string]PropertyInfo, showDetails, showDeprecated bool, filterPath string) []string {
	var diffs []string

	// Get all unique keys
	allKeys := make(map[string]struct{})
	for k := range props1 {
		if filterPath == "" || strings.HasPrefix(k, filterPath) {
			allKeys[k] = struct{}{}
		}
	}
	for k := range props2 {
		if filterPath == "" || strings.HasPrefix(k, filterPath) {
			allKeys[k] = struct{}{}
		}
	}

	// Sort keys for consistent output
	var sortedKeys []string
	for k := range allKeys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	// Track counts for summary
	var added, removed, changed int

	for _, key := range sortedKeys {
		prop1, exists1 := props1[key]
		prop2, exists2 := props2[key]

		if !exists1 && exists2 {
			// Property was added
			added++
			if !showDeprecated && prop2.Deprecated {
				continue
			}

			diff := fmt.Sprintf("+ %s", key)
			if showDetails {
				diff += fmt.Sprintf(" (type: %s", prop2.Type)
				if prop2.Default != nil {
					diff += fmt.Sprintf(", default: %v", prop2.Default)
				}
				if prop2.EnterpriseOnly {
					diff += ", enterprise-only"
				}
				if prop2.Deprecated {
					diff += ", deprecated"
				}
				diff += ")"
			}
			diffs = append(diffs, diff+"\n")

		} else if exists1 && !exists2 {
			// Property was removed
			removed++
			if !showDeprecated && prop1.Deprecated {
				continue
			}

			diff := fmt.Sprintf("- %s", key)
			if showDetails {
				diff += fmt.Sprintf(" (was type: %s", prop1.Type)
				if prop1.Default != nil {
					diff += fmt.Sprintf(", default: %v", prop1.Default)
				}
				if prop1.EnterpriseOnly {
					diff += ", enterprise-only"
				}
				if prop1.Deprecated {
					diff += ", deprecated"
				}
				diff += ")"
			}
			diffs = append(diffs, diff+"\n")

		} else if exists1 && exists2 {
			// Property exists in both, check for changes
			if !reflect.DeepEqual(prop1, prop2) {
				changed++
				if !showDeprecated && (prop1.Deprecated || prop2.Deprecated) {
					continue
				}

				diff := fmt.Sprintf("~ %s", key)
				if showDetails {
					var changes []string
					if prop1.Type != prop2.Type {
						changes = append(changes, fmt.Sprintf("type: %s → %s", prop1.Type, prop2.Type))
					}
					if !reflect.DeepEqual(prop1.Default, prop2.Default) {
						changes = append(changes, fmt.Sprintf("default: %v → %v", prop1.Default, prop2.Default))
					}
					if prop1.Dynamic != prop2.Dynamic {
						changes = append(changes, fmt.Sprintf("dynamic: %v → %v", prop1.Dynamic, prop2.Dynamic))
					}
					if prop1.EnterpriseOnly != prop2.EnterpriseOnly {
						changes = append(changes, fmt.Sprintf("enterprise-only: %v → %v", prop1.EnterpriseOnly, prop2.EnterpriseOnly))
					}
					if prop1.Deprecated != prop2.Deprecated {
						changes = append(changes, fmt.Sprintf("deprecated: %v → %v", prop1.Deprecated, prop2.Deprecated))
					}
					if len(changes) > 0 {
						diff += fmt.Sprintf(" (%s)", strings.Join(changes, ", "))
					}
				}
				diffs = append(diffs, diff+"\n")
			}
		}
	}

	// Add summary at the end
	if len(diffs) > 0 {
		diffs = append(diffs, fmt.Sprintf("\nSummary: %d added, %d removed, %d changed\n", added, removed, changed))
	}

	return diffs
}
