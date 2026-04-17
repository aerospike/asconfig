package serveryaml

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	loggingFieldType       = "type"
	loggingFieldName       = "name"
	loggingFieldPath       = "path"
	loggingFieldFacility   = "facility"
	loggingFieldTag        = "tag"
	loggingFieldContexts   = "contexts"
	loggingSinkTypeConsole = "console"
	loggingSinkTypeFile    = "file"
	loggingSinkTypeSyslog  = "syslog"
)

var loggingReservedFields = map[string]struct{}{
	loggingFieldType:     {},
	loggingFieldName:     {},
	loggingFieldPath:     {},
	loggingFieldFacility: {},
	loggingFieldTag:      {},
	loggingFieldContexts: {},
}

// loggingLevels are the set of values considered "log level" strings when
// flattening or collecting logging contexts. Exposed via IsLoggingLevel so
// callers outside this package can share the same vocabulary.
var loggingLevels = map[string]bool{
	"critical": true,
	"warning":  true,
	"info":     true,
	"debug":    true,
	"detail":   true,
}

// IsLoggingLevel reports whether the supplied value is a recognized aerospike
// logging level. Comparison is case-insensitive.
func IsLoggingLevel(value any) bool {
	level, ok := value.(string)
	if !ok {
		return false
	}

	return loggingLevels[strings.ToLower(level)]
}

// FromLegacy converts legacy asconfig YAML to the server-native (experimental)
// YAML shape. It translates named slices (namespaces, sets, network.tls,
// xdr.dcs) into maps keyed by name, and normalizes legacy logging sinks into
// type + contexts form.
func FromLegacy(yamlBytes []byte) ([]byte, error) {
	parsed := map[string]any{}
	if err := yaml.Unmarshal(yamlBytes, parsed); err != nil {
		return nil, err
	}

	if err := translateLegacyYAMLStructure(parsed); err != nil {
		return nil, err
	}

	return yaml.Marshal(parsed)
}

func translateLegacyYAMLStructure(root map[string]any) error {
	if err := translateLegacyNamespaces(root); err != nil {
		return err
	}

	if err := translateLegacyNetworkTLS(root); err != nil {
		return err
	}

	if err := translateLegacyXDR(root); err != nil {
		return err
	}

	return translateLegacyLogging(root)
}

func translateLegacyNamespaces(root map[string]any) error {
	rawNamespaces, found := root["namespaces"]
	if !found {
		return nil
	}

	namespaceSlice, isSlice := rawNamespaces.([]any)
	if !isSlice {
		// Already in server map form (or unknown shape). Leave unchanged.
		return nil
	}

	for i, rawNamespace := range namespaceSlice {
		namespaceMap, isNamespaceMap := rawNamespace.(map[string]any)
		if !isNamespaceMap {
			return fmt.Errorf("namespaces.%d must be an object, found %T", i, rawNamespace)
		}

		if err := translateNamedSliceInObjectToMap(
			namespaceMap,
			"sets",
			[]string{"namespaces", strconv.Itoa(i), "sets"},
		); err != nil {
			return err
		}
	}

	namespaceMap, err := namedSliceToNamedMap(namespaceSlice, []string{"namespaces"})
	if err != nil {
		return err
	}

	root["namespaces"] = namespaceMap

	return nil
}

func translateLegacyNetworkTLS(root map[string]any) error {
	rawNetwork, found := root["network"]
	if !found {
		return nil
	}

	networkMap, isMap := rawNetwork.(map[string]any)
	if !isMap {
		return fmt.Errorf("network must be an object, found %T", rawNetwork)
	}

	return translateNamedSliceInObjectToMap(networkMap, "tls", []string{"network", "tls"})
}

func translateLegacyXDR(root map[string]any) error {
	rawXDR, found := root["xdr"]
	if !found {
		return nil
	}

	xdrMap, isMap := rawXDR.(map[string]any)
	if !isMap {
		return fmt.Errorf("xdr must be an object, found %T", rawXDR)
	}

	rawDCs, hasDCs := xdrMap["dcs"]
	if !hasDCs {
		return nil
	}

	dcSlice, isSlice := rawDCs.([]any)
	if !isSlice {
		// Already in server map form (or unknown shape). Leave unchanged.
		return nil
	}

	for i, rawDC := range dcSlice {
		dcMap, isDCMap := rawDC.(map[string]any)
		if !isDCMap {
			return fmt.Errorf("xdr.dcs.%d must be an object, found %T", i, rawDC)
		}

		if err := translateNamedSliceInObjectToMap(
			dcMap,
			"namespaces",
			[]string{"xdr", "dcs", strconv.Itoa(i), "namespaces"},
		); err != nil {
			return err
		}
	}

	dcMap, err := namedSliceToNamedMap(dcSlice, []string{"xdr", "dcs"})
	if err != nil {
		return err
	}

	xdrMap["dcs"] = dcMap

	return nil
}

func translateLegacyLogging(root map[string]any) error {
	rawLogging, found := root["logging"]
	if !found {
		return nil
	}

	loggingSlice, isSlice := rawLogging.([]any)
	if !isSlice {
		return nil
	}

	for i, rawSink := range loggingSlice {
		translatedSink, err := translateLegacyLogSink(rawSink, i)
		if err != nil {
			return err
		}

		loggingSlice[i] = translatedSink
	}

	root["logging"] = loggingSlice

	return nil
}

func translateLegacyLogSink(rawSink any, index int) (map[string]any, error) {
	sinkMap, isSinkMap := rawSink.(map[string]any)
	if !isSinkMap {
		return nil, fmt.Errorf("logging.%d must be an object, found %T", index, rawSink)
	}

	if err := setLegacyLoggingType(sinkMap, index); err != nil {
		return nil, err
	}

	contexts, err := collectLegacyLoggingContexts(sinkMap, index)
	if err != nil {
		return nil, err
	}

	if len(contexts) > 0 {
		sinkMap[loggingFieldContexts] = contexts
	}

	return sinkMap, nil
}

func setLegacyLoggingType(sinkMap map[string]any, index int) error {
	if err := normalizeExistingLegacyLoggingType(sinkMap, index); err != nil {
		return err
	}

	rawName, hasName := sinkMap[loggingFieldName]
	if !hasName {
		return ensureLegacyFileSinkHasPath(sinkMap, index)
	}

	name, isString := rawName.(string)
	if !isString {
		return fmt.Errorf("logging.%d.name must be a string, found %T", index, rawName)
	}

	sinkType := legacyLoggingSinkTypeFromName(name)
	if _, hasType := sinkMap[loggingFieldType]; !hasType {
		sinkMap[loggingFieldType] = sinkType
	}

	resolvedType, _ := sinkMap[loggingFieldType].(string)
	if resolvedType == loggingSinkTypeFile {
		if _, hasPath := sinkMap[loggingFieldPath]; !hasPath && !isLegacyLoggingNamedType(name) {
			sinkMap[loggingFieldPath] = name
		}
	}

	delete(sinkMap, loggingFieldName)

	return ensureLegacyFileSinkHasPath(sinkMap, index)
}

func normalizeExistingLegacyLoggingType(sinkMap map[string]any, index int) error {
	rawType, hasType := sinkMap[loggingFieldType]
	if !hasType {
		return nil
	}

	typeName, isString := rawType.(string)
	if !isString {
		return fmt.Errorf("logging.%d.type must be a string, found %T", index, rawType)
	}

	normalizedType := normalizeLoggingSinkType(typeName)
	if isLegacyLoggingNamedType(normalizedType) {
		sinkMap[loggingFieldType] = normalizedType

		return nil
	}

	// Some legacy payloads encode file sink paths in `type`; normalize to
	// server-native `type: file` plus explicit `path`.
	sinkMap[loggingFieldType] = loggingSinkTypeFile
	if _, hasPath := sinkMap[loggingFieldPath]; !hasPath {
		sinkMap[loggingFieldPath] = typeName
	}

	return nil
}

func ensureLegacyFileSinkHasPath(sinkMap map[string]any, index int) error {
	rawType, hasType := sinkMap[loggingFieldType]
	if !hasType {
		return nil
	}

	sinkType, isString := rawType.(string)
	if !isString {
		return fmt.Errorf("logging.%d.type must be a string, found %T", index, rawType)
	}

	if sinkType != loggingSinkTypeFile {
		return nil
	}

	if _, hasPath := sinkMap[loggingFieldPath]; !hasPath {
		return fmt.Errorf("logging.%d.file sink missing required path", index)
	}

	return nil
}

func normalizeLoggingSinkType(rawType string) string {
	return strings.ToLower(strings.TrimSpace(rawType))
}

func legacyLoggingSinkTypeFromName(name string) string {
	normalizedName := normalizeLoggingSinkType(name)
	switch normalizedName {
	case loggingSinkTypeConsole, loggingSinkTypeFile, loggingSinkTypeSyslog:
		return normalizedName
	default:
		// Legacy asconfig models file sinks as name: "/path/to/file.log".
		return loggingSinkTypeFile
	}
}

func isLegacyLoggingNamedType(name string) bool {
	normalizedName := normalizeLoggingSinkType(name)

	return normalizedName == loggingSinkTypeConsole ||
		normalizedName == loggingSinkTypeFile ||
		normalizedName == loggingSinkTypeSyslog
}

func collectLegacyLoggingContexts(sinkMap map[string]any, index int) (map[string]any, error) {
	contexts := map[string]any{}

	if rawContexts, hasContexts := sinkMap[loggingFieldContexts]; hasContexts {
		existingContexts, isMap := rawContexts.(map[string]any)
		if !isMap {
			return nil, fmt.Errorf("logging.%d.contexts must be an object, found %T", index, rawContexts)
		}

		for key, value := range existingContexts {
			contexts[key] = value
		}
	}

	for key, value := range sinkMap {
		if _, reserved := loggingReservedFields[key]; reserved {
			continue
		}

		if IsLoggingLevel(value) {
			contexts[key] = value
			delete(sinkMap, key)
		}
	}

	return contexts, nil
}

func translateNamedSliceInObjectToMap(parent map[string]any, key string, path []string) error {
	rawValue, found := parent[key]
	if !found {
		return nil
	}

	namedSlice, isSlice := rawValue.([]any)
	if !isSlice {
		// Already in server map form (or unknown shape). Leave unchanged.
		return nil
	}

	asMap, err := namedSliceToNamedMap(namedSlice, path)
	if err != nil {
		return err
	}

	parent[key] = asMap

	return nil
}

func namedSliceToNamedMap(namedSlice []any, path []string) (map[string]any, error) {
	res := make(map[string]any, len(namedSlice))

	for i, rawItem := range namedSlice {
		itemMap, isMap := rawItem.(map[string]any)
		if !isMap {
			return nil, fmt.Errorf("%s.%d must be an object, found %T", formatNodePath(path), i, rawItem)
		}

		rawName, hasName := itemMap["name"]
		if !hasName {
			return nil, fmt.Errorf("%s.%d missing required name field", formatNodePath(path), i)
		}

		name, isString := rawName.(string)
		if !isString {
			return nil, fmt.Errorf("%s.%d.name must be a string, found %T", formatNodePath(path), i, rawName)
		}

		if _, duplicate := res[name]; duplicate {
			return nil, fmt.Errorf("%s contains duplicate name %q", formatNodePath(path), name)
		}

		delete(itemMap, "name")
		res[name] = itemMap
	}

	return res, nil
}
