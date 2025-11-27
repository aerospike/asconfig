package cmd

import (
	"encoding/json"
	"fmt"
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

// ChangeTarget represents what type of element is being changed.
type ChangeTarget int

const (
	Property ChangeTarget = iota
	ArrayItem
)

// Schema field constants.
const (
	propertiesField  = "properties"
	itemsField       = "items"
	descriptionField = "description"
)

const (
	// minBoxWidth defines the minimum width for the section box.
	minBoxWidth = 50

	// compactStringEllipsisLimit defines the maximum string length before truncation in compact mode.
	// Set to 50 characters to balance readability with terminal width constraints.
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
	Sections          map[string]SectionChanges
	AvailableSections []string
	TotalChanges      int
	TotalAdditions    int
	TotalRemovals     int
	TotalModified     int
	LowerVersion      string
	UpperVersion      string
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
	summary, err := groupChangesBySection(changes, lowerVersion, upperVersion, validSections)
	if err != nil {
		return ChangeSummary{}, fmt.Errorf("failed to group changes by section: %w", err)
	}
	return summary, nil
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
) (ChangeSummary, error) {
	summary := ChangeSummary{
		Sections:          make(map[string]SectionChanges),
		LowerVersion:      lowerVersion,
		UpperVersion:      upperVersion,
		TotalChanges:      len(changes),
		AvailableSections: make([]string, 0, len(validSections)),
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
			// Fail fast on unknown change types instead of silently skipping
			return ChangeSummary{}, fmt.Errorf("unknown change type %q at path %s", change.Type, change.Path)
		}

		summary.Sections[section] = sectionChanges
	}

	// Capture the complete list of available sections (including those without changes)
	availableSet := make(map[string]struct{}, len(validSections))
	for section := range validSections {
		summary.AvailableSections = append(summary.AvailableSections, section)
		availableSet[section] = struct{}{}
	}

	// Ensure the synthetic general section is available for filtering when present
	if _, exists := summary.Sections[defaultSectionName]; exists {
		if _, alreadyIncluded := availableSet[defaultSectionName]; !alreadyIncluded {
			summary.AvailableSections = append(summary.AvailableSections, defaultSectionName)
		}
	}
	sort.Strings(summary.AvailableSections)

	return summary, nil
}

// validateFilterSections validates that the provided filter sections exist in the available sections.
func validateFilterSections(filterSections map[string]struct{}, availableSections []string) error {
	var invalidSections []string

	availableLookup := make(map[string]struct{}, len(availableSections))
	for _, section := range availableSections {
		availableLookup[section] = struct{}{}
	}

	// Collect all available section names
	validSections := make([]string, len(availableSections))
	copy(validSections, availableSections)

	// Check each filter section
	for filterSection := range filterSections {
		if _, exists := availableLookup[filterSection]; !exists {
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

// ============================================================================
// FORMATTING FUNCTIONS (Pure functions that return strings)
// ============================================================================

// formatChange is the master dispatcher that routes changes to appropriate formatters.
// This is the single decision point for all change formatting.
func formatChange(change SchemaChange, verbose bool) (string, error) {
	target := getChangeTarget(change)

	switch change.Type {
	case Addition:
		if target == ArrayItem {
			return formatArrayItemAddition(change, verbose), nil
		}
		return formatPropertyAddition(change, verbose), nil
	case Removal:
		if target == ArrayItem {
			return formatArrayItemRemoval(change, verbose), nil
		}
		return formatPropertyRemoval(change, verbose), nil
	case Modification:
		if target == ArrayItem {
			return formatArrayItemModification(change, verbose), nil
		}
		return formatPropertyModification(change, verbose), nil
	default:
		return "", fmt.Errorf("unknown change type %q at path %s", change.Type, change.Path)
	}
}

// getChangeTarget determines if a change affects a property or an array item.
func getChangeTarget(change SchemaChange) ChangeTarget {
	if isArrayPath(change.Path) {
		return ArrayItem
	}
	return Property
}

// formatPropertyAddition formats the addition of a new configuration property.
func formatPropertyAddition(change SchemaChange, verbose bool) string {
	path := formatPath(change.Path)

	if verbose {
		icon := iconAddition
		var result strings.Builder
		result.WriteString(fmt.Sprintf("  %s %s\n", icon, path))
		if change.Value != nil {
			result.WriteString(formatValueDetails(change.Value))
		}
		return result.String()
	}

	prefix := additionPrefix
	summary := getValueSummary(change.Value)
	return fmt.Sprintf("%s %s %s\n", prefix, path, summary)
}

// formatPropertyRemoval formats the removal of a configuration property.
func formatPropertyRemoval(change SchemaChange, verbose bool) string {
	path := formatPath(change.Path)

	if verbose {
		icon := iconRemoval
		return fmt.Sprintf("  %s %s\n", icon, path)
	}

	prefix := removalPrefix
	return fmt.Sprintf("%s %s (removed)\n", prefix, path)
}

// formatPropertyModification formats the modification of a configuration property.
func formatPropertyModification(change SchemaChange, verbose bool) string {
	path := formatPath(change.Path)

	if verbose {
		icon := iconModification
		var result strings.Builder
		result.WriteString(fmt.Sprintf("  %s %s\n", icon, path))
		result.WriteString(formatModificationDetails(change))
		if change.Value != nil {
			result.WriteString(formatValueDetails(change.Value))
		}
		return result.String()
	}

	prefix := modificationPrefix
	oldVal := formatCompactValue(change.OldValue)
	newVal := formatCompactValue(change.Value)
	return fmt.Sprintf("%s %s (%s → %s)\n", prefix, path, oldVal, newVal)
}

// formatArrayItemAddition formats the addition of an array item.
func formatArrayItemAddition(change SchemaChange, verbose bool) string {
	path := formatPath(change.Path)

	if verbose {
		icon := iconAddition
		var result strings.Builder
		result.WriteString(fmt.Sprintf("  %s %s\n", icon, path))

		if itemMap, isMap := change.Value.(map[string]any); isMap {
			result.WriteString("     → Array item:\n")
			result.WriteString(formatValueDetails(itemMap))
		} else {
			result.WriteString(fmt.Sprintf("     → Array item: %s\n", formatValue(change.Value)))
		}
		return result.String()
	}

	prefix := additionPrefix
	summary := getValueSummary(change.Value)
	summaryContent := unwrapParentheses(summary)
	return fmt.Sprintf("%s %s (array item: %s)\n", prefix, path, summaryContent)
}

// formatArrayItemRemoval formats the removal of an array item.
func formatArrayItemRemoval(change SchemaChange, verbose bool) string {
	path := formatPath(change.Path)

	if verbose {
		icon := iconRemoval
		var result strings.Builder
		result.WriteString(fmt.Sprintf("  %s %s\n", icon, path))

		if itemMap, isMap := change.Value.(map[string]any); isMap {
			result.WriteString("     → Array item:\n")
			result.WriteString(formatValueDetails(itemMap))
		} else {
			result.WriteString(fmt.Sprintf("     → Array item: %s\n", formatValue(change.Value)))
		}
		return result.String()
	}

	prefix := removalPrefix
	summary := getValueSummary(change.Value)
	summaryContent := unwrapParentheses(summary)
	return fmt.Sprintf("%s %s (array item: %s)\n", prefix, path, summaryContent)
}

// formatArrayItemModification formats the modification of an array item.
func formatArrayItemModification(change SchemaChange, verbose bool) string {
	path := formatPath(change.Path)

	if verbose {
		icon := iconModification
		var result strings.Builder
		result.WriteString(fmt.Sprintf("  %s %s\n", icon, path))

		// For array modifications, show the full array values if available
		if change.OldFullValue != nil && change.NewFullValue != nil {
			result.WriteString(fmt.Sprintf("     → Changed from: %s\n", formatValue(change.OldFullValue)))
			result.WriteString(fmt.Sprintf("     → Changed to: %s\n", formatValue(change.NewFullValue)))
		} else {
			result.WriteString(formatModificationDetails(change))
		}
		return result.String()
	}

	prefix := modificationPrefix
	oldVal := formatCompactValue(change.OldFullValue)
	newVal := formatCompactValue(change.NewFullValue)
	if oldVal == "" {
		oldVal = formatCompactValue(change.OldValue)
	}
	if newVal == "" {
		newVal = formatCompactValue(change.Value)
	}
	return fmt.Sprintf("%s %s (%s → %s)\n", prefix, path, oldVal, newVal)
}

// formatModificationDetails formats the details of a modification change.
func formatModificationDetails(change SchemaChange) string {
	prefix := "     → "
	if strings.Contains(change.Path, "minimum") || strings.Contains(change.Path, "maximum") {
		if change.OldValue != nil && change.Value != nil {
			return fmt.Sprintf("%sConfiguration limit changed from: %s to: %s\n", prefix,
				formatValue(change.OldValue), formatValue(change.Value))
		}
		return fmt.Sprintf("%sConfiguration limit changed\n", prefix)
	}

	if change.OldValue != nil && change.Value != nil {
		return fmt.Sprintf("%sChanged from: %s to: %s\n", prefix,
			formatValue(change.OldValue), formatValue(change.Value))
	}

	return ""
}

// formatValueDetails formats detailed information about a value for verbose output.
func formatValueDetails(value any) string {
	propMap, ok := value.(map[string]any)
	if !ok {
		return ""
	}

	prefix := "     → "
	var result strings.Builder

	// Get sorted keys for consistent output
	keys := make([]string, 0, len(propMap))
	for key := range propMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Format each property
	for _, key := range keys {
		val := propMap[key]

		// Skip empty values
		if shouldSkipProperty(key, val) {
			continue
		}

		result.WriteString(formatSingleProperty(key, val, prefix))
	}

	return result.String()
}

// ============================================================================
// RENDERING FUNCTIONS
// ============================================================================

// renderChangeSummary renders a formatted summary of schema changes.
func renderChangeSummary(summary ChangeSummary, options DiffOptions) {
	// Render header with versions
	renderHeader(summary, options)

	// Get all sections and sort them alphabetically
	var sections []string
	for section := range summary.Sections {
		sections = append(sections, section)
	}
	sort.Strings(sections)

	// Render changes by section in alphabetical order
	for _, section := range sections {
		changes := summary.Sections[section]

		// Skip if filtering is enabled and this section is not in the filter
		if len(options.FilterSections) > 0 {
			if _, ok := options.FilterSections[section]; !ok {
				continue
			}
		}

		renderSectionChanges(section, changes, options)
	}
}

// renderHeader renders the header information for the schema changes.
func renderHeader(summary ChangeSummary, options DiffOptions) {
	if options.Verbose {
		// Use dynamic box sizing for the header
		headerText := "AEROSPIKE CONFIGURATION CHANGES SUMMARY"

		// Calculate the required width (text + minimum padding on each side)
		const minPadding = 2 // Total: 1 space on each side minimum
		requiredWidth := len(headerText) + minPadding

		// Use the same minimum box width as renderSectionBox for consistency
		boxWidth := minBoxWidth
		if requiredWidth > minBoxWidth {
			boxWidth = requiredWidth
		}

		// Calculate padding for centering
		leftPadding, rightPadding := calculateBoxPadding(len(headerText), boxWidth)

		// Create the box borders
		border := strings.Repeat("═", boxWidth)

		// Render the header box
		headerBox := fmt.Sprintf("\n╔%s╗\n║%s%s%s║\n╚%s╝\n\n",
			border,
			strings.Repeat(" ", leftPadding),
			headerText,
			strings.Repeat(" ", rightPadding),
			border)
		renderOutput("%s", headerBox)
	} else {
		renderOutput("AEROSPIKE CONFIGURATION CHANGES SUMMARY\n\n")
	}

	summaryInfo := fmt.Sprintf(
		"Comparing: %s → %s\nTotal changes: %d (%d additions, %d removals, %d modifications)\n\n",
		summary.LowerVersion,
		summary.UpperVersion,
		summary.TotalChanges,
		summary.TotalAdditions,
		summary.TotalRemovals,
		summary.TotalModified,
	)
	renderOutput("%s", summaryInfo)
}

// renderSectionChanges renders changes for a specific section.
func renderSectionChanges(section string, changes SectionChanges, options DiffOptions) {
	// Section header
	if options.Verbose {
		renderSectionBox(section)
	} else {
		renderOutput("\n[SECTION: %s]\n", strings.ToUpper(section))
	}

	// Render all changes for this section
	renderAllChanges(changes, options)
}

// renderSectionBox renders a dynamically-sized box around the section name.
// The box width adjusts to accommodate the section name while maintaining a minimum width.
// The box will never be smaller than minBoxWidth or the text + minimum padding, whichever is larger.
func renderSectionBox(section string) {
	upperSection := strings.ToUpper(section)
	sectionText := "SECTION: " + upperSection

	// Calculate the required width (text + minimum padding on each side)
	const minPadding = 2 // Total: 1 space on each side minimum
	requiredWidth := len(sectionText) + minPadding

	// Use the larger of minimum width or required width
	// This ensures: boxWidth >= requiredWidth >= len(sectionText) + minPadding
	boxWidth := minBoxWidth
	if requiredWidth > boxWidth {
		boxWidth = requiredWidth
	}

	// Calculate padding for centering (with defensive checks)
	leftPadding, rightPadding := calculateBoxPadding(len(sectionText), boxWidth)

	// Create the box borders
	topBorder := "╭" + strings.Repeat("─", boxWidth) + "╮"
	bottomBorder := "╰" + strings.Repeat("─", boxWidth) + "╯"

	// Render the box
	sectionBox := fmt.Sprintf("\n%s\n│%s%s%s│\n%s\n",
		topBorder,
		strings.Repeat(" ", leftPadding),
		sectionText,
		strings.Repeat(" ", rightPadding),
		bottomBorder)
	renderOutput("%s", sectionBox)
}

// renderAllChanges renders all types of changes in a unified way.
func renderAllChanges(changes SectionChanges, options DiffOptions) {
	// Define change configurations
	changeConfigs := []struct {
		changes []SchemaChange
		header  string
	}{
		{changes.Removals, removedConfigHeader},
		{changes.Additions, newConfigHeader},
		{changes.Modifications, modifiedConfigHeader},
	}

	// Process each change type
	for _, config := range changeConfigs {
		if len(config.changes) == 0 {
			continue
		}

		// Render header
		if options.Verbose {
			renderOutput("\n  %s:\n", config.header)
		} else {
			renderOutput("%s:\n", config.header)
		}

		// Format and render each change
		for _, change := range config.changes {
			formattedChange, err := formatChange(change, options.Verbose)
			if err != nil {
				renderError("Error formatting change: %v\n", err)
				continue
			}
			renderOutput("%s", formattedChange)
		}
	}
}

// ============================================================================
// FORMATTING HELPERS - Value & Property Formatting
// ============================================================================

// shouldSkipProperty determines if a property should be skipped during rendering.
func shouldSkipProperty(key string, val any) bool {
	return val == nil || (key == descriptionField && val == "")
}

// formatSingleProperty formats a single property with appropriate formatting.
func formatSingleProperty(key string, val any, prefix string) string {
	displayName := formatKeyName(key)

	// Handle complex nested structures (maps and arrays) - expand them hierarchically
	switch v := val.(type) {
	case map[string]any:
		// Render the key and expand the map hierarchically
		var result strings.Builder
		result.WriteString(fmt.Sprintf("%s%s:\n", prefix, displayName))
		result.WriteString(formatNestedData(v, prefix+"  "))
		return result.String()
	case []any:
		// Render the key and expand the array hierarchically
		var result strings.Builder
		result.WriteString(fmt.Sprintf("%s%s:\n", prefix, displayName))
		result.WriteString(formatNestedDataArray(v, prefix+"  "))
		return result.String()
	default:
		// Simple value - format as string
		displayValue := formatValue(val)
		if displayValue != "" {
			return fmt.Sprintf("%s%s: %s\n", prefix, displayName, displayValue)
		}
		return ""
	}
}

// formatNestedData formats a map hierarchically without schema traversal.
func formatNestedData(data map[string]any, prefix string) string {
	var result strings.Builder

	// Get sorted keys for consistent output
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Format each key-value pair
	for _, key := range keys {
		val := data[key]
		if val == nil {
			continue
		}

		displayName := formatKeyName(key)

		switch v := val.(type) {
		case map[string]any:
			// Nested map - recurse
			result.WriteString(fmt.Sprintf("%s%s:\n", prefix, displayName))
			result.WriteString(formatNestedData(v, prefix+"  "))
		case []any:
			// Array - format elements
			result.WriteString(fmt.Sprintf("%s%s:\n", prefix, displayName))
			result.WriteString(formatNestedDataArray(v, prefix+"  "))
		default:
			// Simple value
			displayValue := formatValue(v)
			if displayValue != "" {
				result.WriteString(fmt.Sprintf("%s%s: %s\n", prefix, displayName, displayValue))
			}
		}
	}

	return result.String()
}

// formatNestedDataArray formats an array hierarchically.
func formatNestedDataArray(arr []any, prefix string) string {
	var result strings.Builder

	for i, item := range arr {
		switch v := item.(type) {
		case map[string]any:
			// Object in array
			result.WriteString(fmt.Sprintf("%s[%d]:\n", prefix, i))
			result.WriteString(formatNestedData(v, prefix+"  "))
		case []any:
			// Nested array
			result.WriteString(fmt.Sprintf("%s[%d]:\n", prefix, i))
			result.WriteString(formatNestedDataArray(v, prefix+"  "))
		default:
			// Simple value
			displayValue := formatValue(item)
			if displayValue != "" {
				result.WriteString(fmt.Sprintf("%s[%d]: %s\n", prefix, i, displayValue))
			}
		}
	}

	return result.String()
}

// formatKeyName converts camelCase keys to human-readable format.
func formatKeyName(key string) string {
	// Only format known metadata field names for display
	// DO NOT modify actual configuration property names (they must match the schema exactly)
	switch key {
	case "type":
		return "Type"
	case "default":
		return "Default"
	case "description":
		return "Description"
	case "minimum":
		return "Minimum"
	case "maximum":
		return "Maximum"
	case "minLength":
		return "Min Length"
	case "maxLength":
		return "Max Length"
	case "pattern":
		return "Pattern"
	case "format":
		return "Format"
	case "required":
		return "Required"
	case "additionalProperties":
		return "AdditionalProperties"
	case "enterpriseOnly":
		return enterpriseOnlyText
	case "enum":
		return allowedValuesText
	case "properties":
		return "Properties"
	case "items":
		return "Items"
	case "dynamic":
		return "Dynamic"
	default:
		// For actual configuration property names, return as-is
		// DO NOT capitalize or modify them
		return key
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
			renderWarning("Failed to marshal map to JSON: %v\n", err)
			return fmt.Sprintf("%#v", v)
		}
		return string(jsonBytes)
	default:
		// Use JSON marshaling for consistent formatting
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			// If JSON marshaling fails, show the raw Go representation
			renderWarning("Failed to marshal value to JSON: %v\n", err)
			return fmt.Sprintf("%#v", v)
		}
		return string(jsonBytes)
	}
}

// formatNumber formats numeric values avoiding scientific notation while keeping raw numbers.
func formatNumber(value any) string {
	// Handle integer types directly to avoid float64 precision loss
	switch v := value.(type) {
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		// Handle special cases for zero
		if v == 0 {
			return "0"
		}

		// For very small decimal numbers, use limited precision
		if v < 1 && v > -1 && v != 0 {
			return fmt.Sprintf("%.6g", v)
		}

		// For integers (whole numbers), show them as integers without scientific notation
		if v == float64(int64(v)) {
			return fmt.Sprintf("%.0f", v)
		}

		// For decimal numbers, show with limited precision to avoid scientific notation
		return fmt.Sprintf("%.6g", v)
	default:
		return fmt.Sprintf("%v", value)
	}
}

// formatArray formats array values using JSON marshaling - no hiding, no abstractions.
func formatArray(arr []any) string {
	// Always use JSON marshaling to show the actual content
	jsonBytes, err := json.Marshal(arr)
	if err != nil {
		// If JSON marshaling fails, show the raw Go representation instead of hiding content
		renderWarning("Failed to marshal array to JSON: %v\n", err)
		return fmt.Sprintf("%#v", arr)
	}
	return string(jsonBytes)
}

// ============================================================================
// PATH UTILITIES - JSON Path Processing
// ============================================================================

// formatPath formats a JSON path into a more human-readable form.
func formatPath(path string) string {
	parts := strings.Split(path, "/")
	result := make([]string, 0, len(parts)) // Pre-allocate with capacity

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

// ============================================================================
// GENERAL UTILITIES - Miscellaneous Helpers
// ============================================================================

// unwrapParentheses removes leading '(' and trailing ')' from a string if both are present.
func unwrapParentheses(s string) string {
	// Check length before accessing indices to prevent panic
	if len(s) >= 2 && s[0] == '(' && s[len(s)-1] == ')' {
		return s[1 : len(s)-1]
	}
	return s
}

// calculateBoxPadding calculates left and right padding for centering text in a box.
// Returns (leftPadding, rightPadding) ensuring both are non-negative.
func calculateBoxPadding(textLen, boxWidth int) (int, int) {
	// Defensive check: ensure boxWidth is at least as wide as the text
	if boxWidth < textLen {
		// If somehow boxWidth is too small, return minimal padding
		return 0, 0
	}
	const halves = 2 // Divide by 2 to split padding evenly
	totalPadding := boxWidth - textLen
	leftPadding := totalPadding / halves
	rightPadding := totalPadding - leftPadding

	// Defensive check: ensure non-negative values (should never happen with above logic, but safe)
	if leftPadding < 0 {
		leftPadding = 0
	}
	if rightPadding < 0 {
		rightPadding = 0
	}

	return leftPadding, rightPadding
}
