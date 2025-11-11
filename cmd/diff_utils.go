package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/wI2L/jsondiff"
)

// DiffOptions stores display configuration options.
type DiffOptions struct {
	Verbose        bool
	FilterSections map[string]struct{}
}

// ChangeType represents the type of change operation.
type ChangeType string

const (
	Addition     ChangeType = "add"
	Removal      ChangeType = "remove"
	Modification ChangeType = "replace"
)

// SchemaChange represents a single schema change.
type SchemaChange struct {
	Path         string
	Type         ChangeType
	OldValue     interface{}
	Value        interface{}
	OldFullValue interface{}
	NewFullValue interface{}
}

// ChangeSummary groups changes by section and type.
type ChangeSummary struct {
	Sections       map[string]SectionChanges
	TotalChanges   int
	TotalAdditions int
	TotalRemovals  int
	TotalModified  int
	NewVersion     string
	OldVersion     string
}

// SectionChanges groups changes by operation type.
type SectionChanges struct {
	Additions     []SchemaChange
	Removals      []SchemaChange
	Modifications []SchemaChange
}

// compareSchemas compares two schema objects and returns a summary of changes.
func compareSchemas(
	schemaLower, schemaUpper map[string]interface{},
	newVersion, oldVersion string,
) (ChangeSummary, error) {
	patch, err := jsondiff.Compare(schemaLower, schemaUpper)
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
					oldArray, oldOk := getValueByJSONPath(schemaLower, parentPath)
					newArray, newOk := getValueByJSONPath(schemaUpper, parentPath)

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

	// Extract valid sections dynamically from both schemas
	validSections := extractValidSections(schemaLower, schemaUpper)

	// Group changes by section and operation type
	return groupChangesBySection(changes, newVersion, oldVersion, validSections), nil
}

// extractValidSections dynamically extracts all top-level sections from both schemas.
func extractValidSections(schema1, schema2 map[string]interface{}) map[string]bool {
	validSections := make(map[string]bool)

	// Extract sections from first schema
	if props1, ok := schema1["properties"].(map[string]interface{}); ok {
		for section := range props1 {
			validSections[section] = true
		}
	}

	// Extract sections from second schema (in case of additions/removals)
	if props2, ok := schema2["properties"].(map[string]interface{}); ok {
		for section := range props2 {
			validSections[section] = true
		}
	}

	return validSections
}

// groupChangesBySection groups schema changes by their section and operation type.
func groupChangesBySection(
	changes []SchemaChange,
	newVersion, oldVersion string,
	validSections map[string]bool,
) ChangeSummary {
	summary := ChangeSummary{
		Sections:     make(map[string]SectionChanges),
		NewVersion:   newVersion,
		OldVersion:   oldVersion,
		TotalChanges: len(changes),
	}

	// Group changes by section and type
	for _, change := range changes {
		section := extractSection(change.Path, validSections)

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

// extractSection determines which section a path belongs to.
func extractSection(path string, validSections map[string]bool) string {
	parts := strings.Split(path, "/")

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

// printChangeSummary prints a formatted summary of schema changes.
func printChangeSummary(summary ChangeSummary, options DiffOptions) {
	// Print header with versions
	printHeader(summary, options)

	// Get all sections and sort them alphabetically
	var sections []string
	for section := range summary.Sections {
		sections = append(sections, section)
	}
	sort.Strings(sections)

	// Print changes by section in alphabetical order
	for _, section := range sections {
		changes := summary.Sections[section]

		// Skip if filtering is enabled and this section is not in the filter
		if len(options.FilterSections) > 0 {
			if _, ok := options.FilterSections[section]; !ok {
				continue
			}
		}

		printSectionChanges(section, changes, options)
	}
}

// printHeader prints the header information for the schema changes.
func printHeader(summary ChangeSummary, options DiffOptions) {
	if options.Verbose {
		fmt.Fprintf(os.Stdout, "\n╔════════════════════════════════════════════════════════════════╗\n")
		fmt.Fprintf(os.Stdout, "║              AEROSPIKE SCHEMA CHANGES SUMMARY                  ║\n")
		fmt.Fprintf(os.Stdout, "╚════════════════════════════════════════════════════════════════╝\n\n")
	} else {
		fmt.Fprintf(os.Stdout, "AEROSPIKE SCHEMA CHANGES SUMMARY\n\n")
	}

	fmt.Fprintf(os.Stdout, "Comparing: %s → %s\n", summary.NewVersion, summary.OldVersion)
	fmt.Fprintf(os.Stdout, "Total changes: %d (%d additions, %d removals, %d modifications)\n\n",
		summary.TotalChanges, summary.TotalAdditions, summary.TotalRemovals, summary.TotalModified)
}

// printSectionChanges prints changes for a specific section.
func printSectionChanges(section string, changes SectionChanges, options DiffOptions) {
	// Section header
	if options.Verbose {
		fmt.Fprintf(os.Stdout, "\n╭─────────────────────────────────────────────────────────────╮\n")
		fmt.Fprintf(os.Stdout, "│ SECTION: %-50s │\n", strings.ToUpper(section))
		fmt.Fprintf(os.Stdout, "╰─────────────────────────────────────────────────────────────╯\n")
	} else {
		fmt.Fprintf(os.Stdout, "\n[SECTION: %s]\n", strings.ToUpper(section))
	}

	// Print all changes for this section in a unified way
	printAllChanges(changes, options)
}

// printAllChanges prints all types of changes in a unified, optimized way.
func printAllChanges(changes SectionChanges, options DiffOptions) {
	// Define change configurations
	changeConfigs := []struct {
		changes []SchemaChange
		header  string
		icon    string
		prefix  string
	}{
		{changes.Removals, "REMOVED CONFIGURATIONS", "❌", "-"},
		{changes.Additions, "NEW CONFIGURATIONS", "✅", "+"},
		{changes.Modifications, "MODIFIED CONFIGURATIONS", "🔄", "~"},
	}

	// Process each change type
	for _, config := range changeConfigs {
		if len(config.changes) == 0 {
			continue
		}

		// Print header
		if options.Verbose {
			fmt.Fprintf(os.Stdout, "\n  %s:\n", config.header)
		} else {
			fmt.Fprintf(os.Stdout, "%s:\n", config.header)
		}

		// Handle modifications with array processing
		if config.header == "MODIFIED CONFIGURATIONS" {
			printModificationsOptimized(config.changes, options, config.icon, config.prefix)
			continue
		}

		// Handle removals and additions
		printSimpleChanges(config.changes, config.header, config.icon, config.prefix, options)
	}
}

// printSimpleChanges handles removals and additions.
func printSimpleChanges(changes []SchemaChange, header, icon, prefix string, options DiffOptions) {
	for _, change := range changes {
		path := formatPath(change.Path)
		if options.Verbose {
			fmt.Fprintf(os.Stdout, "  %s %s\n", icon, path)
			// Print additional details for additions
			if header == "NEW CONFIGURATIONS" {
				printValueProperties(change.Value, options)
			}
		} else {
			fmt.Fprintf(os.Stdout, "%s %s\n", prefix, path)
		}
	}
}

// printModificationsOptimized handles modifications with array optimization.
func printModificationsOptimized(modifications []SchemaChange, options DiffOptions, icon, prefix string) {
	processedArrays := make(map[string]bool)

	for _, change := range modifications {
		// Handle full array changes
		if isArrayIndex(change.Path) && change.OldFullValue != nil && change.NewFullValue != nil {
			parentPath := getParentPath(change.Path)
			if processedArrays[parentPath] {
				continue // Already processed this array
			}
			processedArrays[parentPath] = true

			displayPath := formatPath(parentPath)
			if options.Verbose {
				fmt.Fprintf(os.Stdout, "  %s %s (array changed)\n", icon, displayPath)
				fmt.Fprintf(os.Stdout, "     → Changed from: %v\n", change.OldFullValue)
				fmt.Fprintf(os.Stdout, "     → Changed to: %v\n", change.NewFullValue)
			} else {
				fmt.Fprintf(os.Stdout, "%s %s (array changed)\n", prefix, displayPath)
			}
			continue
		}

		// Handle regular modifications
		path := formatPath(change.Path)
		if options.Verbose {
			fmt.Fprintf(os.Stdout, "  %s %s\n", icon, path)
			printChangeDetails(change)
			printValueProperties(change.Value, options)
		} else {
			fmt.Fprintf(os.Stdout, "%s %s\n", prefix, path)
		}
	}
}

// printChangeDetails prints details about a specific change.
func printChangeDetails(change SchemaChange) {
	prefix := "     → "
	if strings.Contains(change.Path, "minimum") || strings.Contains(change.Path, "maximum") {
		fmt.Fprintf(os.Stdout, "%sConfiguration limit changed\n", prefix)
	}
}

// printValueProperties prints properties dynamically without any hardcoding.
func printValueProperties(value interface{}, options DiffOptions) {
	propMap, ok := value.(map[string]interface{})
	if !ok {
		return
	}

	prefix := "  "
	if options.Verbose {
		prefix = "     → "
	}

	// Convert to JSON and back to get a clean, consistent representation
	jsonBytes, err := json.Marshal(propMap)
	if err != nil {
		return
	}

	var cleanMap map[string]interface{}
	if jsonMarshalErr := json.Unmarshal(jsonBytes, &cleanMap); jsonMarshalErr != nil {
		return
	}

	// Get sorted keys for consistent output
	keys := make([]string, 0, len(cleanMap))
	for key := range cleanMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Print each property with minimal logic
	for _, key := range keys {
		val := cleanMap[key]

		// Skip empty values
		if val == nil || (key == "description" && val == "") {
			continue
		}

		// Skip complex nested objects in non-verbose mode (except for simple properties)
		if !options.Verbose && isComplexProperty(val) {
			continue
		}

		// Format key name (camelCase to readable)
		displayName := formatKeyName(key)
		displayValue := formatValue(val)

		if displayValue != "" {
			fmt.Fprintf(os.Stdout, "%s%s: %s\n", prefix, displayName, displayValue)
		}
	}
}

// isComplexProperty determines if a property is complex and should be hidden in non-verbose mode.
func isComplexProperty(value interface{}) bool {
	switch v := value.(type) {
	case map[string]interface{}:
		return true
	case []interface{}:
		const maxArraySizeForNonVerbose = 3
		return len(v) > maxArraySizeForNonVerbose // Hide large arrays in non-verbose mode
	default:
		return false
	}
}

// formatKeyName converts camelCase keys to human-readable format.
func formatKeyName(key string) string {
	// Handle common special cases
	switch key {
	case "enterpriseOnly":
		return "Enterprise Edition Only"
	case "enum":
		return "Allowed values"
	default:
		// Simple camelCase to Title Case conversion
		if len(key) == 0 {
			return key
		}
		return strings.ToUpper(key[:1]) + key[1:]
	}
}

// formatValue formats any value for display using JSON marshaling for consistency.
func formatValue(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case bool:
		if v {
			return "Yes"
		}
		return "No"
	case string:
		if v == "" {
			return ""
		}
		return v
	case float64, int, int64:
		return fmt.Sprintf("%v", v)
	case []interface{}:
		// Format arrays compactly
		if len(v) == 0 {
			return "[]"
		}
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(jsonBytes)
	case map[string]interface{}:
		// For objects, just show that it's an object
		return "[object]"
	default:
		// Use JSON marshaling for consistent formatting
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(jsonBytes)
	}
}

// Utility functions
// ---------------

// formatPath formats a JSON path into a more human-readable form.
func formatPath(path string) string {
	parts := strings.Split(path, "/")
	var result []string

	// Process each part
	for i := 0; i < len(parts); i++ {
		part := parts[i]

		// Skip empty parts and schema markers
		if part == "" || part == "properties" || part == "items" {
			continue
		}

		// Handle array index notation
		if i < len(parts)-1 && isNumeric(parts[i+1]) {
			// It's an array index
			result = append(result, part+"["+parts[i+1]+"]")
			i++ // Skip the index part
			continue
		}

		// Handle the special "-" index (append to array)
		if part == "-" {
			if len(result) > 0 {
				lastIdx := len(result) - 1
				result[lastIdx] += "[+]"
			} else {
				result = append(result, "array[+]")
			}
			continue
		}

		// Regular part
		result = append(result, part)
	}

	return strings.Join(result, ".")
}

// isNumeric checks if a string is numeric.
func isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// getValueByJSONPath retrieves a value from a map[string]interface{} using a JSON pointer path.
func getValueByJSONPath(data map[string]interface{}, path string) (interface{}, bool) {
	parts := strings.Split(path, "/")
	current := interface{}(data)

	for _, part := range parts {
		if part == "" {
			continue
		}

		var ok bool
		current, ok = processPathPart(current, part)
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// processPathPart processes a single part of the JSON path.
func processPathPart(current interface{}, part string) (interface{}, bool) {
	if isNumeric(part) {
		return processArrayIndex(current, part)
	}
	return processMapKey(current, part)
}

// processArrayIndex handles array indexing for numeric parts.
func processArrayIndex(current interface{}, part string) (interface{}, bool) {
	arr, ok := current.([]interface{})
	if !ok {
		return nil, false
	}

	index, _ := strconv.Atoi(part)
	if index < 0 || index >= len(arr) {
		return nil, false
	}

	return arr[index], true
}

// processMapKey handles map key access.
func processMapKey(current interface{}, part string) (interface{}, bool) {
	m, ok := current.(map[string]interface{})
	if !ok {
		return nil, false
	}

	val, exists := m[part]
	return val, exists
}

// isArrayIndex checks if a path points to an array index.
func isArrayIndex(path string) bool {
	parts := strings.Split(path, "/")
	if len(parts) < 1 {
		return false
	}
	// Check if the last part is numeric (an array index)
	return isNumeric(parts[len(parts)-1])
}

// getParentPath returns the parent path of a given JSON path.
func getParentPath(path string) string {
	parts := strings.Split(path, "/")
	// Remove the last part (the array index)
	return strings.Join(parts[:len(parts)-1], "/")
}
