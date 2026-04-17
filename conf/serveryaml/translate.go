package serveryaml

import (
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

// ToLegacy converts server-native (experimental) YAML to the legacy asconfig
// YAML shape currently understood by the aerospike-management-lib. It
// translates map-keyed collections (namespaces, sets, network.tls, xdr.dcs)
// into named slices, converts {value, unit} objects into scalar integers,
// and normalizes logging sinks.
func ToLegacy(yamlBytes []byte) ([]byte, error) {
	parsed := map[string]any{}
	if err := yaml.Unmarshal(yamlBytes, parsed); err != nil {
		return nil, err
	}

	if err := translateServerYAMLStructure(parsed); err != nil {
		return nil, err
	}

	converted, err := translateUnitValues(parsed, nil)
	if err != nil {
		return nil, err
	}

	res, ok := converted.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("translated yaml root is invalid type %T", converted)
	}

	return yaml.Marshal(res)
}

func translateServerYAMLStructure(root map[string]any) error {
	if err := translateNamespaces(root); err != nil {
		return err
	}

	if err := translateNetworkTLS(root); err != nil {
		return err
	}

	if err := translateXDR(root); err != nil {
		return err
	}

	return translateLogging(root)
}

func translateNamespaces(root map[string]any) error {
	rawNamespaces, found := root["namespaces"]
	if !found {
		return nil
	}

	namespacesMap, isMap := rawNamespaces.(map[string]any)
	if !isMap {
		// Already in legacy array form (or unknown shape). Leave unchanged.
		return nil
	}

	for namespaceName, rawNamespace := range namespacesMap {
		namespaceMap, isNamespaceMap := rawNamespace.(map[string]any)
		if !isNamespaceMap {
			return fmt.Errorf("namespaces.%s must be an object, found %T", namespaceName, rawNamespace)
		}

		if err := translateNamedMapInObjectToSlice(
			namespaceMap,
			"sets",
			[]string{"namespaces", namespaceName, "sets"},
		); err != nil {
			return err
		}
	}

	namespaceSlice, err := namedMapToNamedSlice(namespacesMap, []string{"namespaces"})
	if err != nil {
		return err
	}

	root["namespaces"] = namespaceSlice

	return nil
}

func translateNetworkTLS(root map[string]any) error {
	rawNetwork, found := root["network"]
	if !found {
		return nil
	}

	networkMap, isMap := rawNetwork.(map[string]any)
	if !isMap {
		return fmt.Errorf("network must be an object, found %T", rawNetwork)
	}

	return translateNamedMapInObjectToSlice(networkMap, "tls", []string{"network", "tls"})
}

func translateXDR(root map[string]any) error {
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

	dcMap, isDCMap := rawDCs.(map[string]any)
	if !isDCMap {
		// Already in legacy array form (or unknown shape). Leave unchanged.
		return nil
	}

	for dcName, rawDC := range dcMap {
		dcObj, isDCObject := rawDC.(map[string]any)
		if !isDCObject {
			return fmt.Errorf("xdr.dcs.%s must be an object, found %T", dcName, rawDC)
		}

		if err := translateNamedMapInObjectToSlice(
			dcObj,
			"namespaces",
			[]string{"xdr", "dcs", dcName, "namespaces"},
		); err != nil {
			return err
		}
	}

	dcSlice, err := namedMapToNamedSlice(dcMap, []string{"xdr", "dcs"})
	if err != nil {
		return err
	}

	xdrMap["dcs"] = dcSlice

	return nil
}

func translateLogging(root map[string]any) error {
	rawLogging, found := root["logging"]
	if !found {
		return nil
	}

	loggingSlice, isSlice := rawLogging.([]any)
	if !isSlice {
		return nil
	}

	for i, rawLogSink := range loggingSlice {
		translatedSink, err := translateServerLogSink(rawLogSink, i)
		if err != nil {
			return err
		}

		loggingSlice[i] = translatedSink
	}

	root["logging"] = loggingSlice

	return nil
}

func translateServerLogSink(rawLogSink any, index int) (map[string]any, error) {
	logSinkMap, isMap := rawLogSink.(map[string]any)
	if !isMap {
		return nil, fmt.Errorf("logging.%d must be an object, found %T", index, rawLogSink)
	}

	if err := setServerLoggingName(logSinkMap, index); err != nil {
		return nil, err
	}

	if err := flattenServerLoggingContexts(logSinkMap, index); err != nil {
		return nil, err
	}

	return logSinkMap, nil
}

func setServerLoggingName(logSinkMap map[string]any, index int) error {
	rawType, hasType := logSinkMap[loggingFieldType]
	if !hasType {
		return nil
	}

	typeName, isString := rawType.(string)
	if !isString {
		return fmt.Errorf("logging.%d.type must be a string, found %T", index, rawType)
	}

	normalizedType := normalizeLoggingSinkType(typeName)

	if _, hasName := logSinkMap[loggingFieldName]; !hasName {
		if normalizedType == loggingSinkTypeFile {
			pathValue, err := requiredLoggingPath(logSinkMap, index)
			if err != nil {
				return err
			}

			logSinkMap[loggingFieldName] = pathValue
			delete(logSinkMap, loggingFieldPath)
		} else {
			logSinkMap[loggingFieldName] = normalizedType
		}
	}

	delete(logSinkMap, loggingFieldType)

	return nil
}

func requiredLoggingPath(logSinkMap map[string]any, index int) (string, error) {
	rawPath, hasPath := logSinkMap[loggingFieldPath]
	if !hasPath {
		return "", fmt.Errorf("logging.%d.file sink missing required path", index)
	}

	pathValue, isString := rawPath.(string)
	if !isString {
		return "", fmt.Errorf("logging.%d.path must be a string, found %T", index, rawPath)
	}

	return pathValue, nil
}

func flattenServerLoggingContexts(logSinkMap map[string]any, index int) error {
	rawContexts, hasContexts := logSinkMap[loggingFieldContexts]
	if !hasContexts {
		return nil
	}

	contextsMap, isMap := rawContexts.(map[string]any)
	if !isMap {
		return fmt.Errorf("logging.%d.contexts must be an object, found %T", index, rawContexts)
	}

	for contextName, contextValue := range contextsMap {
		if _, hasField := logSinkMap[contextName]; hasField {
			return fmt.Errorf("logging.%d context %q conflicts with existing sink field", index, contextName)
		}

		logSinkMap[contextName] = contextValue
	}

	delete(logSinkMap, loggingFieldContexts)

	return nil
}

func translateNamedMapInObjectToSlice(parent map[string]any, key string, path []string) error {
	rawValue, found := parent[key]
	if !found {
		return nil
	}

	typedMap, isMap := rawValue.(map[string]any)
	if !isMap {
		// Already in legacy array form (or unknown shape). Leave unchanged.
		return nil
	}

	asSlice, err := namedMapToNamedSlice(typedMap, path)
	if err != nil {
		return err
	}

	parent[key] = asSlice

	return nil
}

func namedMapToNamedSlice(namedMap map[string]any, path []string) ([]any, error) {
	names := sortedKeys(namedMap)

	items := make([]any, 0, len(names))
	for _, name := range names {
		rawItem := namedMap[name]
		itemMap, isMap := rawItem.(map[string]any)
		if !isMap {
			return nil, fmt.Errorf("%s.%s must be an object, found %T", formatNodePath(path), name, rawItem)
		}

		itemMap["name"] = name
		items = append(items, itemMap)
	}

	return items, nil
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	// Sort for deterministic output; callers rely on this for reproducibility.
	sortStrings(keys)

	return keys
}

func sortStrings(keys []string) {
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
}

func formatNodePath(path []string) string {
	if len(path) == 0 {
		return "(root)"
	}

	out := path[0]
	for i := 1; i < len(path); i++ {
		out += "." + path[i]
	}

	return out
}

func pathWithIndex(path []string, index int) []string {
	appended := make([]string, 0, len(path)+1)
	appended = append(appended, path...)
	appended = append(appended, strconv.Itoa(index))

	return appended
}
