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

// Schema field constants.
const (
	propertiesField  = "properties"
	itemsField       = "items"
	descriptionField = "description"
)

const (
	// minBoxWidth defines the minimum width for the section box.
	minBoxWidth                = 50
	compactStringEllipsisLimit = 50
)

// Display constants.
const (
	// Configuration headers.
	removedConfigHeader  = "REMOVED CONFIGURATIONS"
	newConfigHeader      = "NEW CONFIGURATIONS"
	modifiedConfigHeader = "MODIFIED CONFIGURATIONS"

	// Icons and prefixes.
	iconRemoval        = "❌"
	iconAddition       = "✅"
	iconModification   = "🔄"
	removalPrefix      = "-"
	additionPrefix     = "+"
	modificationPrefix = "~"

	// Business logic constants.
	defaultSectionName = "general"
	enterpriseOnlyText = "Enterprise Edition Only"
	allowedValuesText  = "Allowed values"
	booleanYesText     = "Yes"
	booleanNoText      = "No"
)

// SchemaChange represents a single schema change.
type SchemaChange struct {
	Path         string
	Type         ChangeType
	OldValue     any
	Value        any
	OldFullValue any
	NewFullValue any
}

// ChangeSummary groups changes by section and type.
type ChangeSummary struct {
	Sections       map[string]SectionChanges
	TotalChanges   int
	TotalAdditions int
	TotalRemovals  int
	TotalModified  int
	LowerVersion   string
	UpperVersion   string
}

// SectionChanges groups changes by operation type.
type SectionChanges struct {
	Additions     []SchemaChange
	Removals      []SchemaChange
	Modifications []SchemaChange
}

// compareSchemas compares two schema objects and returns a summary of changes.
func compareSchemas(
	schemaLower, schemaUpper map[string]any,
	lowerVersion, upperVersion string,
) (ChangeSummary, error) {
	patch, err := jsondiff.Compare(schemaLower, schemaUpper)
	if err != nil {
		return ChangeSummary{}, err
	}

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

			// For array modifications, get the full old and new array values when needed
			if isArrayIndex(op.Path) {
				parentPath := getParentPath(op.Path)
				if oldArray, ok := getValueByJSONPath(schemaLower, parentPath); ok {
					change.OldFullValue = oldArray
				}
				if newArray, ok := getValueByJSONPath(schemaUpper, parentPath); ok {
					change.NewFullValue = newArray
				}
			}
		default:
			// Handle unexpected operation types from jsondiff library
			return ChangeSummary{}, fmt.Errorf("unsupported jsondiff operation type: %q at path %s", op.Type, op.Path)
		}

		changes = append(changes, change)
	}

	// Extract valid sections dynamically from both schemas
	validSections := extractValidSections(schemaLower, schemaUpper)

	// Group changes by section and operation type
	return groupChangesBySection(changes, lowerVersion, upperVersion, validSections), nil
}

// extractValidSections dynamically extracts all top-level sections from both schemas.
func extractValidSections(schema1, schema2 map[string]any) map[string]bool {
	validSections := make(map[string]bool)

	// Extract sections from first schema
	if props1, ok := schema1[propertiesField].(map[string]any); ok {
		for section := range props1 {
			validSections[section] = true
		}
	}

	// Extract sections from second schema (in case of additions/removals)
	if props2, ok := schema2[propertiesField].(map[string]any); ok {
		for section := range props2 {
			validSections[section] = true
		}
	}

	return validSections
}

// groupChangesBySection groups schema changes by their section and operation type.
func groupChangesBySection(
	changes []SchemaChange,
	lowerVersion, upperVersion string,
	validSections map[string]bool,
) ChangeSummary {
	summary := ChangeSummary{
		Sections:     make(map[string]SectionChanges),
		LowerVersion: lowerVersion,
		UpperVersion: upperVersion,
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
		default:
			// This should never happen if the first switch statement is working correctly,
			// but handle it anyway.
			fmt.Fprintf(
				os.Stderr,
				"Warning: Skipping change with unknown type %q at path %s\n",
				change.Type,
				change.Path,
			)
		}

		summary.Sections[section] = sectionChanges
	}

	return summary
}

// validateFilterSections validates that the provided filter sections exist in the available sections.
func validateFilterSections(filterSections map[string]struct{}, availableSections map[string]SectionChanges) error {
	var invalidSections []string
	var validSections []string

	// Collect all available section names
	for section := range availableSections {
		validSections = append(validSections, section)
	}
	sort.Strings(validSections)

	// Check each filter section
	for filterSection := range filterSections {
		if _, exists := availableSections[filterSection]; !exists {
			invalidSections = append(invalidSections, filterSection)
		}
	}

	// If there are invalid sections, return an error with helpful information
	if len(invalidSections) > 0 {
		sort.Strings(invalidSections)
		return fmt.Errorf(
			"invalid filter section(s): %s. Available sections: %s",
			strings.Join(invalidSections, ", "),
			strings.Join(validSections, ", "),
		)
	}

	return nil
}

// extractSection determines which section a path belongs to.
func extractSection(path string, validSections map[string]bool) string {
	parts := strings.Split(path, "/")

	// Look for valid top-level section in the path
	for _, part := range parts {
		if part == "" || part == propertiesField || part == itemsField || isNumeric(part) {
			continue
		}

		// If this is a valid section, return it
		if _, ok := validSections[part]; ok {
			return part
		}
	}

	// If we didn't find a valid section, return "general"
	return defaultSectionName
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
		fmt.Fprintf(os.Stdout, "║              AEROSPIKE CONFIGURATION CHANGES SUMMARY                  ║\n")
		fmt.Fprintf(os.Stdout, "╚════════════════════════════════════════════════════════════════╝\n\n")
	} else {
		fmt.Fprintf(os.Stdout, "AEROSPIKE CONFIGURATION CHANGES SUMMARY\n\n")
	}

	fmt.Fprintf(os.Stdout, "Comparing: %s → %s\n", summary.LowerVersion, summary.UpperVersion)
	fmt.Fprintf(os.Stdout, "Total changes: %d (%d additions, %d removals, %d modifications)\n\n",
		summary.TotalChanges, summary.TotalAdditions, summary.TotalRemovals, summary.TotalModified)
}

// printSectionChanges prints changes for a specific section.
func printSectionChanges(section string, changes SectionChanges, options DiffOptions) {
	// Section header
	if options.Verbose {
		printSectionBox(section)
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
		{changes.Removals, removedConfigHeader, iconRemoval, removalPrefix},
		{changes.Additions, newConfigHeader, iconAddition, additionPrefix},
		{changes.Modifications, modifiedConfigHeader, iconModification, modificationPrefix},
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
		if config.header == modifiedConfigHeader {
			printModifications(config.changes, options, config.icon, config.prefix)
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

		// Check if this is an array addition/removal and show the array contents
		if isArrayPath(change.Path) && change.Value != nil {
			printArrayChange(change, path, icon, prefix, options)
		} else {
			printNormalChange(change, path, header, icon, prefix, options)
		}
	}
}

// printArrayChange handles array additions and removals with content display.
func printArrayChange(change SchemaChange, path, icon, prefix string, options DiffOptions) {
	if options.Verbose {
		printArrayChangeVerbose(change, path, icon, options)
		return
	}

	printArrayChangeCompact(change, path, prefix)
}

// printArrayChangeVerbose handles verbose array change display.
func printArrayChangeVerbose(change SchemaChange, path, icon string, options DiffOptions) {
	fmt.Fprintf(os.Stdout, "  %s %s\n", icon, path)

	// For array additions/removals, show the individual item being added/removed
	if itemMap, isMap := change.Value.(map[string]any); isMap {
		// Complex object - show as formatted metadata, not recursive traversal
		fmt.Fprintf(os.Stdout, "     → Array item:\n")
		printValueProperties(itemMap, options)
	} else {
		// Simple value - show as single line
		fmt.Fprintf(os.Stdout, "     → Array item: %s\n", formatValue(change.Value))
	}
}

// printArrayChangeCompact handles compact array change display.
func printArrayChangeCompact(change SchemaChange, path, prefix string) {
	summary := getValueSummary(change.Value)
	// For array changes, clarify it's an array item
	// Remove leading '(' and trailing ')' if present
	summaryContent := summary
	if len(summary) >= 2 && summary[0] == '(' && summary[len(summary)-1] == ')' {
		summaryContent = summary[1 : len(summary)-1]
	}
	fmt.Fprintf(os.Stdout, "%s %s (array item: %s)\n", prefix, path, summaryContent)
}

// printNormalChange handles non-array changes.
func printNormalChange(change SchemaChange, path, header, icon, prefix string, options DiffOptions) {
	if options.Verbose {
		fmt.Fprintf(os.Stdout, "  %s %s\n", icon, path)
		// Print additional details for additions
		if header == newConfigHeader {
			printValueProperties(change.Value, options)
		}
	} else {
		// Compact mode: one line per change with summary
		if header == newConfigHeader {
			summary := getValueSummary(change.Value)
			fmt.Fprintf(os.Stdout, "%s %s %s\n", prefix, path, summary)
		} else {
			// Removal
			fmt.Fprintf(os.Stdout, "%s %s (removed)\n", prefix, path)
		}
	}
}

// printModifications handles modification display.
func printModifications(modifications []SchemaChange, options DiffOptions, icon, prefix string) {
	for _, change := range modifications {
		path := formatPath(change.Path)

		if options.Verbose {
			fmt.Fprintf(os.Stdout, "  %s %s\n", icon, path)

			// For array changes, show the full array values
			if isArrayIndex(change.Path) && change.OldFullValue != nil && change.NewFullValue != nil {
				fmt.Fprintf(os.Stdout, "     → Changed from: %s\n", formatArrayValue(change.OldFullValue))
				fmt.Fprintf(os.Stdout, "     → Changed to: %s\n", formatArrayValue(change.NewFullValue))
			} else {
				printChangeDetails(change)
				printValueProperties(change.Value, options)
			}
		} else {
			// Compact mode: one line with old → new values
			oldVal := formatCompactValue(change.OldValue)
			newVal := formatCompactValue(change.Value)
			fmt.Fprintf(os.Stdout, "%s %s (%s → %s)\n", prefix, path, oldVal, newVal)
		}
	}
}

// printChangeDetails prints details about a specific change.
func printChangeDetails(change SchemaChange) {
	prefix := "     → "
	if strings.Contains(change.Path, "minimum") || strings.Contains(change.Path, "maximum") {
		if change.OldValue != nil && change.Value != nil {
			fmt.Fprintf(os.Stdout, "%sConfiguration limit changed from: %s to: %s\n", prefix,
				formatValue(change.OldValue), formatValue(change.Value))
		} else {
			fmt.Fprintf(os.Stdout, "%sConfiguration limit changed\n", prefix)
		}
	} else if change.OldValue != nil && change.Value != nil {
		// Show the actual change for other modifications
		fmt.Fprintf(os.Stdout, "%sChanged from: %s to: %s\n", prefix,
			formatValue(change.OldValue), formatValue(change.Value))
	}
}

// printValueProperties prints properties dynamically without any hardcoding.
func printValueProperties(value any, options DiffOptions) {
	propMap, ok := value.(map[string]any)
	if !ok {
		return
	}

	prefix := "  "
	if options.Verbose {
		prefix = "     → "
	}

	// Get sorted keys for consistent output
	keys := make([]string, 0, len(propMap))
	for key := range propMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Print each property with minimal logic
	for _, key := range keys {
		val := propMap[key]

		// Skip empty values
		if shouldSkipProperty(key, val) {
			continue
		}

		printSingleProperty(key, val, prefix)
	}
}

// shouldSkipProperty determines if a property should be skipped during printing.
func shouldSkipProperty(key string, val any) bool {
	return val == nil || (key == descriptionField && val == "")
}

// printSingleProperty prints a single property with appropriate formatting.
func printSingleProperty(key string, val any, prefix string) {
	displayName := formatKeyName(key)

	// Handle complex nested structures (maps and arrays) - expand them hierarchically
	// but do NOT call schema traversal functions (to avoid infinite recursion)
	switch v := val.(type) {
	case map[string]any:
		// Print the key and expand the map hierarchically
		fmt.Fprintf(os.Stdout, "%s%s:\n", prefix, displayName)
		printNestedData(v, prefix+"  ")
	case []any:
		// Print the key and expand the array hierarchically
		fmt.Fprintf(os.Stdout, "%s%s:\n", prefix, displayName)
		printNestedDataArray(v, prefix+"  ")
	default:
		// Simple value - format as string
		displayValue := formatValue(val)
		if displayValue != "" {
			fmt.Fprintf(os.Stdout, "%s%s: %s\n", prefix, displayName, displayValue)
		}
	}
}

// printNestedData prints a map hierarchically without schema traversal.
// This is used for displaying metadata structures like "properties" fields.
func printNestedData(data map[string]any, prefix string) {
	// Get sorted keys for consistent output
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Print each key-value pair
	for _, key := range keys {
		val := data[key]
		if val == nil {
			continue
		}

		displayName := formatKeyName(key)

		switch v := val.(type) {
		case map[string]any:
			// Nested map - recurse
			fmt.Fprintf(os.Stdout, "%s%s:\n", prefix, displayName)
			printNestedData(v, prefix+"  ")
		case []any:
			// Array - print elements
			fmt.Fprintf(os.Stdout, "%s%s:\n", prefix, displayName)
			printNestedDataArray(v, prefix+"  ")
		default:
			// Simple value
			displayValue := formatValue(v)
			if displayValue != "" {
				fmt.Fprintf(os.Stdout, "%s%s: %s\n", prefix, displayName, displayValue)
			}
		}
	}
}

// printNestedDataArray prints an array hierarchically.
func printNestedDataArray(arr []any, prefix string) {
	for i, item := range arr {
		switch v := item.(type) {
		case map[string]any:
			// Object in array
			fmt.Fprintf(os.Stdout, "%s[%d]:\n", prefix, i)
			printNestedData(v, prefix+"  ")
		case []any:
			// Nested array
			fmt.Fprintf(os.Stdout, "%s[%d]:\n", prefix, i)
			printNestedDataArray(v, prefix+"  ")
		default:
			// Simple value
			displayValue := formatValue(item)
			if displayValue != "" {
				fmt.Fprintf(os.Stdout, "%s[%d]: %s\n", prefix, i, displayValue)
			}
		}
	}
}

// formatKeyName converts camelCase keys to human-readable format.
func formatKeyName(key string) string {
	// Handle common special cases
	switch key {
	case "enterpriseOnly":
		return enterpriseOnlyText
	case "enum":
		return allowedValuesText
	default:
		// Simple camelCase to Title Case conversion
		if len(key) == 0 {
			return key
		}
		return strings.ToUpper(key[:1]) + key[1:]
	}
}

// getValueSummary generates a one-line summary for a value in compact mode.
// Returns format like: (type, default: value) or (object with N properties).
func getValueSummary(val any) string {
	if val == nil {
		return "(no details)"
	}

	valMap, isMap := val.(map[string]any)
	if !isMap {
		// Simple value
		return fmt.Sprintf("(%s)", formatCompactValue(val))
	}

	// Extract key fields from the map
	var parts []string
	addTypeInfo(valMap, &parts)
	addDefaultInfo(valMap, &parts)
	addPropertiesInfo(valMap, &parts)
	addItemsInfo(valMap, &parts)
	addEnumInfo(valMap, &parts)

	if len(parts) == 0 {
		return "(object)"
	}

	return fmt.Sprintf("(%s)", strings.Join(parts, ", "))
}

// addTypeInfo adds type information to the summary parts.
func addTypeInfo(valMap map[string]any, parts *[]string) {
	if typeVal, ok := valMap["type"]; ok && typeVal != nil {
		*parts = append(*parts, formatCompactValue(typeVal))
	}
}

// addDefaultInfo adds default value information to the summary parts.
func addDefaultInfo(valMap map[string]any, parts *[]string) {
	if defaultVal, ok := valMap["default"]; ok && defaultVal != nil {
		*parts = append(*parts, fmt.Sprintf("default: %s", formatCompactValue(defaultVal)))
	}
}

// addPropertiesInfo adds property count information to the summary parts.
func addPropertiesInfo(valMap map[string]any, parts *[]string) {
	if props, ok := valMap[propertiesField].(map[string]any); ok && len(props) > 0 {
		*parts = append(*parts, fmt.Sprintf("%d properties", len(props)))
	}
}

// addItemsInfo adds array items information to the summary parts.
func addItemsInfo(valMap map[string]any, parts *[]string) {
	var ok bool
	var items any
	if items, ok = valMap[itemsField]; !ok || items == nil {
		return
	}

	var itemMap map[string]any
	if itemMap, ok = items.(map[string]any); !ok {
		return
	}

	var itemType any
	if itemType, ok = itemMap["type"]; ok {
		*parts = append(*parts, fmt.Sprintf("items: %s", formatCompactValue(itemType)))
	} else {
		*parts = append(*parts, "has items")
	}
}

// addEnumInfo adds enum values count information to the summary parts.
func addEnumInfo(valMap map[string]any, parts *[]string) {
	var ok bool
	var enumVal any
	if enumVal, ok = valMap["enum"]; !ok {
		return
	}

	var enumArr []any
	if enumArr, ok = enumVal.([]any); ok && len(enumArr) > 0 {
		*parts = append(*parts, fmt.Sprintf("%d allowed values", len(enumArr)))
	}
}

// formatCompactValue formats a value for compact one-line display.
func formatCompactValue(val any) string {
	if val == nil {
		return "null"
	}

	switch v := val.(type) {
	case string:
		if len(v) > compactStringEllipsisLimit {
			return v[:compactStringEllipsisLimit-3] + "..."
		}
		return v
	case bool:
		if v {
			return booleanYesText
		}
		return booleanNoText
	case float64:
		// Check if it's an integer value
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return fmt.Sprintf("%.2f", v)
	case []any:
		return fmt.Sprintf("array[%d]", len(v))
	case map[string]any:
		if len(v) == 0 {
			return "object"
		}
		return fmt.Sprintf("object[%d]", len(v))
	default:
		str := fmt.Sprintf("%v", val)
		if len(str) > compactStringEllipsisLimit {
			return str[:compactStringEllipsisLimit-3] + "..."
		}
		return str
	}
}

// formatValue formats any value for display.
func formatValue(value any) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case bool:
		if v {
			return booleanYesText
		}
		return booleanNoText
	case string:
		if v == "" {
			return ""
		}
		return v
	case float64, int, int64:
		return formatNumber(v)
	case []any:
		return formatArray(v)
	case map[string]any:
		// Use JSON marshaling to show actual content
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			// If JSON marshaling fails, show the raw Go representation
			fmt.Fprintf(os.Stderr, "Warning: Failed to marshal map to JSON: %v\n", err)
			return fmt.Sprintf("%#v", v)
		}
		return string(jsonBytes)
	default:
		// Use JSON marshaling for consistent formatting
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			// If JSON marshaling fails, show the raw Go representation
			fmt.Fprintf(os.Stderr, "Warning: Failed to marshal value to JSON: %v\n", err)
			return fmt.Sprintf("%#v", v)
		}
		return string(jsonBytes)
	}
}

// formatNumber formats numeric values avoiding scientific notation while keeping raw numbers.
func formatNumber(value any) string {
	var num float64

	switch v := value.(type) {
	case float64:
		num = v
	case int:
		num = float64(v)
	case int64:
		num = float64(v)
	default:
		return fmt.Sprintf("%v", value)
	}

	// Handle special cases for zero
	if num == 0 {
		return "0"
	}

	// For very small decimal numbers, use limited precision
	if num < 1 && num > -1 && num != 0 {
		return fmt.Sprintf("%.6g", num)
	}

	// For integers (whole numbers), show them as integers
	if num == float64(int64(num)) {
		return fmt.Sprintf("%.0f", num)
	}

	// For decimal numbers, show with limited precision to avoid scientific notation
	return fmt.Sprintf("%.6g", num)
}

// formatArray formats array values using JSON marshaling - no hiding, no abstractions.
func formatArray(arr []any) string {
	// Always use JSON marshaling to show the actual content
	jsonBytes, err := json.Marshal(arr)
	if err != nil {
		// If JSON marshaling fails, show the raw Go representation instead of hiding content
		fmt.Fprintf(os.Stderr, "Warning: Failed to marshal array to JSON: %v\n", err)
		return fmt.Sprintf("%#v", arr)
	}
	return string(jsonBytes)
}

// formatArrayValue is an alias for formatValue for backward compatibility.
func formatArrayValue(value any) string {
	return formatValue(value)
}

// Utility functions
// ---------------

// printSectionBox prints a dynamically-sized box around the section name.
// The box width adjusts to accommodate the section name while maintaining a minimum width.
func printSectionBox(section string) {
	upperSection := strings.ToUpper(section)
	sectionText := "SECTION: " + upperSection

	// Calculate the required width (text + minimum padding on each side)
	const minPadding = 2 // 1 space on each side
	requiredWidth := len(sectionText) + minPadding

	// Use the larger of minimum width or required width
	boxWidth := minBoxWidth
	if requiredWidth > minBoxWidth {
		boxWidth = requiredWidth
	}

	// Calculate padding for centering the text (split evenly between left and right)
	const halves = 2 // Divide by 2 to split padding evenly
	totalPadding := boxWidth - len(sectionText)
	leftPadding := totalPadding / halves
	rightPadding := totalPadding - leftPadding

	// Create the box borders
	topBorder := "╭" + strings.Repeat("─", boxWidth) + "╮"
	bottomBorder := "╰" + strings.Repeat("─", boxWidth) + "╯"

	// Print the box
	fmt.Fprintf(os.Stdout, "\n%s\n", topBorder)
	fmt.Fprintf(os.Stdout, "│%s%s%s│\n",
		strings.Repeat(" ", leftPadding),
		sectionText,
		strings.Repeat(" ", rightPadding))
	fmt.Fprintf(os.Stdout, "%s\n", bottomBorder)
}

// formatPath formats a JSON path into a more human-readable form.
func formatPath(path string) string {
	parts := strings.Split(path, "/")
	var result []string

	// Process each part
	for i := 0; i < len(parts); i++ {
		part := parts[i]

		// Skip empty parts and schema markers
		if part == "" || part == propertiesField || part == itemsField {
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

// getValueByJSONPath retrieves a value from a map[string]any using a JSON pointer path.
func getValueByJSONPath(data map[string]any, path string) (any, bool) {
	parts := strings.Split(path, "/")
	current := any(data)

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

// Path utilities
// -------------

// processPathPart processes a single part of the JSON path.
func processPathPart(current any, part string) (any, bool) {
	if isNumeric(part) {
		return processArrayIndex(current, part)
	}
	return processMapKey(current, part)
}

// processArrayIndex handles array indexing for numeric parts.
func processArrayIndex(current any, part string) (any, bool) {
	arr, ok := current.([]any)
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
func processMapKey(current any, part string) (any, bool) {
	m, ok := current.(map[string]any)
	if !ok {
		return nil, false
	}

	val, exists := m[part]
	return val, exists
}

// isArrayIndex checks if a path points to an array index.
func isArrayIndex(path string) bool {
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return false
	}
	// Check if the last part is numeric (an array index)
	lastPart := parts[len(parts)-1]
	return lastPart != "" && isNumeric(lastPart)
}

// isArrayPath checks if a path represents an array field (including array additions with "-").
func isArrayPath(path string) bool {
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return false
	}
	// Check if the last part is "-" (array addition) or if it's a numeric index
	lastPart := parts[len(parts)-1]
	return lastPart == "-" || (lastPart != "" && isNumeric(lastPart))
}

// getParentPath returns the parent path of a given JSON path.
func getParentPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) <= 1 {
		return ""
	}
	// Remove the last part (the array index)
	return strings.Join(parts[:len(parts)-1], "/")
}
