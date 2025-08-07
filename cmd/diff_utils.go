package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/wI2L/jsondiff"
)

// DiffOptions stores display configuration options
type DiffOptions struct {
	Verbose        bool
	FilterSections map[string]struct{}
}

// ChangeType represents the type of change operation
type ChangeType string

const (
	Addition     ChangeType = "add"
	Removal      ChangeType = "remove"
	Modification ChangeType = "replace"
)

// SchemaChange represents a single schema change
type SchemaChange struct {
	Path         string
	Type         ChangeType
	OldValue     interface{}
	Value        interface{}
	OldFullValue interface{}
	NewFullValue interface{}
}

// ChangeSummary groups changes by section and type
type ChangeSummary struct {
	Sections       map[string]SectionChanges
	TotalChanges   int
	TotalAdditions int
	TotalRemovals  int
	TotalModified  int
	NewVersion     string
	OldVersion     string
}

// SectionChanges groups changes by operation type
type SectionChanges struct {
	Additions     []SchemaChange
	Removals      []SchemaChange
	Modifications []SchemaChange
}

// compareSchemas compares two schema objects and returns a summary of changes
func compareSchemas(file1Obj, file2Obj map[string]interface{}, newVersion, oldVersion string) (ChangeSummary, error) {
	patch, err := jsondiff.Compare(file1Obj, file2Obj)
	if err != nil {
		return ChangeSummary{}, err
	}

	// Caches for full array values to avoid redundant lookups
	oldFullValuesCache := make(map[string]interface{})
	newFullValuesCache := make(map[string]interface{})
	processedArrayPaths := make(map[string]struct{})

	// Create schema changes from patch operations
	changes := make([]SchemaChange, 0, len(patch))
	for _, op := range patch {
		change := SchemaChange{
			Path:     op.Path,
			Value:    op.Value,
			OldValue: op.OldValue,
		}

		switch op.Type {
		case string(Addition):
			change.Type = Addition
		case string(Removal):
			change.Type = Removal
		case string(Modification):
			change.Type = Modification

			// For array modifications, store the full old and new array values
			if isArrayIndex(op.Path) {
				parentPath := getParentPath(op.Path)

				if _, processed := processedArrayPaths[parentPath]; !processed {
					oldArray, oldOk := getValueByJSONPath(file1Obj, parentPath)
					newArray, newOk := getValueByJSONPath(file2Obj, parentPath)

					if oldOk && newOk {
						oldFullValuesCache[parentPath] = oldArray
						newFullValuesCache[parentPath] = newArray
					}
					processedArrayPaths[parentPath] = struct{}{}
				}

				// Assign cached full values
				change.OldFullValue = oldFullValuesCache[parentPath]
				change.NewFullValue = newFullValuesCache[parentPath]
			}
		}

		changes = append(changes, change)
	}

	// Group changes by section and operation type
	return groupChangesBySection(changes, newVersion, oldVersion), nil
}

// groupChangesBySection groups schema changes by their section and operation type
func groupChangesBySection(changes []SchemaChange, newVersion, oldVersion string) ChangeSummary {
	summary := ChangeSummary{
		Sections:     make(map[string]SectionChanges),
		NewVersion:   newVersion,
		OldVersion:   oldVersion,
		TotalChanges: len(changes),
	}

	// Group changes by section and type
	for _, change := range changes {
		section := extractSection(change.Path)

		sectionChanges := summary.Sections[section]

		switch change.Type {
		case Addition:
			sectionChanges.Additions = append(sectionChanges.Additions, change)
			summary.TotalAdditions++
		case Removal:
			sectionChanges.Removals = append(sectionChanges.Removals, change)
			summary.TotalRemovals++
		case Modification:
			sectionChanges.Modifications = append(sectionChanges.Modifications, change)
			summary.TotalModified++
		}

		summary.Sections[section] = sectionChanges
	}

	return summary
}

// Path utilities
// -------------

// extractSection determines which section a path belongs to
func extractSection(path string) string {
	parts := strings.Split(path, "/")

	// Define valid top-level sections
	validSections := map[string]bool{
		"service":    true,
		"logging":    true,
		"network":    true,
		"namespaces": true,
		"mod-lua":    true,
		"security":   true,
		"xdr":        true,
	}

	// Look for valid top-level section in the path
	for _, part := range parts {
		if part == "" || part == "properties" || part == "items" || isNumeric(part) {
			continue
		}

		// If this is a valid section, return it
		if validSections[part] {
			return part
		}
	}

	// If we didn't find a valid section, return "general"
	return "general"
}

// Output formatting
// ---------------

// printChangeSummary prints a formatted summary of schema changes
func printChangeSummary(summary ChangeSummary, options DiffOptions) {
	// Print header with versions
	printHeader(summary, options)

	// Define the order of sections to display
	sectionOrder := []string{
		"service",
		"logging",
		"network",
		"namespaces",
		"mod-lua",
		"security",
		"xdr",
		"general",
	}

	// Print changes by component in defined order
	for _, section := range sectionOrder {
		changes, exists := summary.Sections[section]
		if !exists {
			continue // Skip if no changes for this section
		}

		// Skip if filtering is enabled and this component is not in the filter
		if len(options.FilterSections) > 0 {
			if _, ok := options.FilterSections[section]; !ok {
				continue
			}
		}

		printSectionChanges(section, changes, options)
	}
}

// printHeader prints the header information for the schema changes
func printHeader(summary ChangeSummary, options DiffOptions) {
	if options.Verbose {
		fmt.Printf("\n╔════════════════════════════════════════════════════════════════╗\n")
		fmt.Printf("║              AEROSPIKE SCHEMA CHANGES SUMMARY                  ║\n")
		fmt.Printf("╚════════════════════════════════════════════════════════════════╝\n\n")
	} else {
		fmt.Printf("AEROSPIKE SCHEMA CHANGES SUMMARY\n\n")
	}

	fmt.Printf("Comparing: %s → %s\n", summary.NewVersion, summary.OldVersion)
	fmt.Printf("Total changes: %d (%d additions, %d removals, %d modifications)\n\n",
		summary.TotalChanges, summary.TotalAdditions, summary.TotalRemovals, summary.TotalModified)
}

// printSectionChanges prints changes for a specific section
func printSectionChanges(section string, changes SectionChanges, options DiffOptions) {
	// Section header
	if options.Verbose {
		fmt.Printf("\n╭─────────────────────────────────────────────────────────────╮\n")
		fmt.Printf("│ SECTION: %-48s │\n", strings.ToUpper(section))
		fmt.Printf("╰─────────────────────────────────────────────────────────────╯\n")
	} else {
		fmt.Printf("\n[SECTION: %s]\n", strings.ToUpper(section))
	}

	// Print removals for this section
	printRemovals(changes.Removals, options)

	// Print additions for this section
	printAdditions(changes.Additions, options)

	// Print modifications for this section
	printModifications(changes.Modifications, options)
}

// printRemovals prints removal changes
func printRemovals(removals []SchemaChange, options DiffOptions) {
	if len(removals) == 0 {
		return
	}

	if options.Verbose {
		fmt.Printf("\n  REMOVED CONFIGURATIONS:\n")
	} else {
		fmt.Printf("REMOVED CONFIGURATIONS:\n")
	}

	for _, change := range removals {
		// Filter by section if specified
		section := extractSection(change.Path)
		if len(options.FilterSections) > 0 {
			if _, ok := options.FilterSections[section]; !ok {
				continue
			}
		}

		path := formatPath(change.Path)
		if options.Verbose {
			fmt.Printf("  ❌ %s\n", path)
		} else {
			fmt.Printf("- %s\n", path)
		}
	}
}

// printAdditions prints addition changes
func printAdditions(additions []SchemaChange, options DiffOptions) {
	if len(additions) == 0 {
		return
	}

	if options.Verbose {
		fmt.Printf("\n  NEW CONFIGURATIONS:\n")
	} else {
		fmt.Printf("NEW CONFIGURATIONS:\n")
	}

	for _, change := range additions {
		// Filter by section if specified
		section := extractSection(change.Path)
		if len(options.FilterSections) > 0 {
			if _, ok := options.FilterSections[section]; !ok {
				continue
			}
		}

		path := formatPath(change.Path)

		if options.Verbose {
			fmt.Printf("  ✅ %s\n", path)
		} else {
			fmt.Printf("+ %s\n", path)
		}

		// Print additional details in verbose mode or when using emojis
		if options.Verbose {
			printValueProperties(change.Value, options)
		}
	}
}

// printModifications prints modification changes
func printModifications(modifications []SchemaChange, options DiffOptions) {
	if len(modifications) == 0 {
		return
	}

	if options.Verbose {
		fmt.Printf("\n  MODIFIED CONFIGURATIONS:\n")
	} else {
		fmt.Printf("MODIFIED CONFIGURATIONS:\n")
	}

	processedArrays := make(map[string]bool)

	for _, change := range modifications {
		// Filter by section if specified
		section := extractSection(change.Path)
		if len(options.FilterSections) > 0 {
			if _, ok := options.FilterSections[section]; !ok {
				continue
			}
		}

		// Handle full array changes
		if isArrayIndex(change.Path) && change.OldFullValue != nil && change.NewFullValue != nil {
			parentPath := getParentPath(change.Path)
			if processedArrays[parentPath] {
				continue // Already processed this array
			}
			processedArrays[parentPath] = true

			// Format the parent path for display (e.g., "security.required")
			displayPath := formatPath(parentPath)

			if options.Verbose {
				fmt.Printf("  🔄 %s (array changed)\n", displayPath)
				fmt.Printf("     → Changed from: %v\n", change.OldFullValue)
				fmt.Printf("     → Changed to: %v\n", change.NewFullValue)
			} else {
				fmt.Printf("~ %s (array changed)\n", displayPath)
				fmt.Printf("  From: %v\n", change.OldFullValue)
				fmt.Printf("  To: %v\n", change.NewFullValue)
			}
			continue // Skip individual element printing
		}

		// Original logic for non-array modifications or object array element changes
		path := formatPath(change.Path)

		if options.Verbose {
			fmt.Printf("  🔄 %s\n", path)
			fmt.Printf("     → Changed from: %v\n", formatNumericValue(change.OldValue))
			fmt.Printf("     → Changed to: %v\n", formatNumericValue(change.Value))
		} else {
			fmt.Printf("~ %s\n", path)
			fmt.Printf("  From: %v\n", formatNumericValue(change.OldValue))
			fmt.Printf("  To: %v\n", formatNumericValue(change.Value))
		}

		// Additional context for certain changes
		if options.Verbose {
			printChangeExplanation(change, options)
		}
	}
}

// printChangeExplanation prints explanatory text for certain types of changes
func printChangeExplanation(change SchemaChange, options DiffOptions) {
	prefix := "     → "
	if !options.Verbose {
		prefix = "  Note: "
	}

	// Add explanation for enterprise flag changes
	if strings.Contains(change.Path, "enterpriseOnly") {
		oldVal, oldOk := change.OldValue.(bool)
		newVal, newOk := change.Value.(bool)

		if oldOk && newOk {
			if oldVal && !newVal {
				fmt.Printf("%sNow available in Community Edition\n", prefix)
			} else if !oldVal && newVal {
				fmt.Printf("%sNow requires Enterprise Edition\n", prefix)
			}
		}
	}

	// Add explanation for min/max changes
	if strings.Contains(change.Path, "minimum") || strings.Contains(change.Path, "maximum") {
		fmt.Printf("%sConfiguration limit changed\n", prefix)
	}
}

// printValueProperties prints properties of a value
func printValueProperties(value interface{}, options DiffOptions) {
	if propMap, ok := value.(map[string]interface{}); ok {
		prefix := "     → "
		if !options.Verbose {
			prefix = "  "
		}

		// Print default value if exists
		if defaultVal, exists := propMap["default"]; exists {
			fmt.Printf("%sDefault: %v\n", prefix, defaultVal)
		}

		// Print description if not empty (only in verbose mode for plain text)
		if desc, exists := propMap["description"].(string); exists && desc != "" {
			if options.Verbose {
				fmt.Printf("%sDescription: %s\n", prefix, desc)
			}
		}

		// Print if it's enterprise only
		if enterpriseOnly, exists := propMap["enterpriseOnly"].(bool); exists {
			if enterpriseOnly {
				fmt.Printf("%sEnterprise Edition Only: Yes\n", prefix)
			} else if options.Verbose {
				fmt.Printf("%sEnterprise Edition Only: No\n", prefix)
			}
		}

		// Print if it's dynamic (can be changed at runtime)
		if dynamic, exists := propMap["dynamic"].(bool); exists {
			if dynamic {
				fmt.Printf("%sDynamic: Yes\n", prefix)
			} else {
				fmt.Printf("%sDynamic: No\n", prefix)
			}
		}

		printObjectProperties(propMap, prefix, options)
	}
}

// printObjectProperties prints the properties of an object value
func printObjectProperties(propMap map[string]interface{}, prefix string, options DiffOptions) {
	// Handle object type specifically to show its structure
	if typeVal, exists := propMap["type"]; exists && typeVal == "object" {
		// Check if the object has properties defined
		if properties, hasProps := propMap["properties"].(map[string]interface{}); hasProps && options.Verbose {
			fmt.Printf("%sObject with properties:\n", prefix)

			// Sort property names for consistent output
			propNames := make([]string, 0, len(properties))
			for name := range properties {
				propNames = append(propNames, name)
			}
			sort.Strings(propNames)

			// Print each property with its type
			for _, name := range propNames {
				prop := properties[name]
				if propObj, isPropObj := prop.(map[string]interface{}); isPropObj {
					propType := "unknown"
					if t, hasType := propObj["type"]; hasType {
						propType = fmt.Sprintf("%v", t)
					}

					// Show property with its type
					indent := prefix + "   "
					fmt.Printf("%s• %s: %s", indent, name, propType)

					// Add property description if available and in verbose mode
					if propDesc, hasDesc := propObj["description"].(string); hasDesc && propDesc != "" && options.Verbose {
						fmt.Printf(" - %s", propDesc)
					}
					fmt.Println()

					// In verbose mode, show default value if present
					if options.Verbose {
						if defVal, hasDef := propObj["default"]; hasDef {
							fmt.Printf("%s  Default: %v\n", indent, defVal)
						}
					}
				}
			}
		} else if options.Verbose {
			// Just show it's an object type
			fmt.Printf("%sType: object\n", prefix)

			// If additionalProperties is set to true or an object, mention that
			if addProps, hasAddProps := propMap["additionalProperties"]; hasAddProps {
				if addPropsBool, isBool := addProps.(bool); isBool && addPropsBool {
					fmt.Printf("%sAllows additional properties of any type\n", prefix)
				} else if _, isObj := addProps.(map[string]interface{}); isObj {
					fmt.Printf("%sAllows additional properties with constraints\n", prefix)
				}
			}
		}
	} else if options.Verbose {
		// In verbose mode, print additional properties
		// Print type information
		if typeVal, exists := propMap["type"]; exists {
			fmt.Printf("%sType: %v\n", prefix, typeVal)
		}

		// Print min/max values
		if minVal, exists := propMap["minimum"]; exists {
			fmt.Printf("%sMinimum: %v\n", prefix, minVal)
		}
		if maxVal, exists := propMap["maximum"]; exists {
			fmt.Printf("%sMaximum: %v\n", prefix, maxVal)
		}

		// Print enum values if they exist
		if enumVals, exists := propMap["enum"].([]interface{}); exists {
			fmt.Printf("%sAllowed values: %v\n", prefix, enumVals)
		}
	}
}

// Utility functions
// ---------------

// formatPath formats a JSON path into a more human-readable form
func formatPath(path string) string {
	parts := strings.Split(path, "/")
	var result []string

	// Process each part
	for i := 0; i < len(parts); i++ {
		part := parts[i]

		// Skip empty parts and schema markers
		if part == "" || part == "properties" {
			continue
		}

		// Handle array indices
		if part == "items" {
			continue
		} else if i < len(parts)-1 && isNumeric(parts[i+1]) {
			// It's an array index
			result = append(result, part+"["+parts[i+1]+"]")
			i++ // Skip the index part
		} else if part == "-" {
			// Handle the special "-" index (append to array)
			if len(result) > 0 {
				lastIdx := len(result) - 1
				result[lastIdx] = result[lastIdx] + "[+]"
			} else {
				result = append(result, "array[+]")
			}
		} else {
			result = append(result, part)
		}
	}

	return strings.Join(result, ".")
}

// formatNumericValue formats numeric values to avoid exponential notation for large integers
func formatNumericValue(value interface{}) interface{} {
	switch v := value.(type) {
	case float64:
		// Check if the float64 represents an integer (no fractional part)
		if v == float64(int64(v)) {
			return int64(v)
		}
		return v
	case float32:
		if v == float32(int32(v)) {
			return int32(v)
		}
		return v
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return v
	default:
		return value
	}
}

// isNumeric checks if a string is numeric
func isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// getValueByJSONPath retrieves a value from a map[string]interface{} using a JSON pointer path
func getValueByJSONPath(data map[string]interface{}, path string) (interface{}, bool) {
	parts := strings.Split(path, "/")
	current := interface{}(data)

	for _, part := range parts {
		if part == "" {
			continue
		}

		// Handle array indexing
		if isNumeric(part) {
			if arr, ok := current.([]interface{}); ok {
				index, _ := strconv.Atoi(part)
				if index >= 0 && index < len(arr) {
					current = arr[index]
				} else {
					return nil, false
				}
			} else {
				return nil, false
			}
		} else if m, ok := current.(map[string]interface{}); ok {
			// Handle map key
			if val, exists := m[part]; exists {
				current = val
			} else {
				return nil, false
			}
		} else {
			return nil, false
		}
	}
	return current, true
}

// isArrayIndex checks if a path points to an array index
func isArrayIndex(path string) bool {
	parts := strings.Split(path, "/")
	if len(parts) < 1 {
		return false
	}
	// Check if the last part is numeric (an array index)
	return isNumeric(parts[len(parts)-1])
}

// getParentPath returns the parent path of a given JSON path
func getParentPath(path string) string {
	parts := strings.Split(path, "/")
	// Remove the last part (the array index)
	return strings.Join(parts[:len(parts)-1], "/")
}
