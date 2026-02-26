package cmd

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"

	lib "github.com/aerospike/aerospike-management-lib"
	asConf "github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	flagServerYAML       = "server-yaml"
	flagServerYAMLOutput = "server-yaml-output"

	minServerYAMLOutputVersion = "8.1.1"

	unitSecond           int64 = 1
	unitMinute           int64 = 60
	unitHour             int64 = 3_600
	unitDay              int64 = 86_400
	unitKilo             int64 = 1_000
	unitMega             int64 = 1_000_000
	unitGiga             int64 = 1_000_000_000
	unitTera             int64 = 1_000_000_000_000
	unitPeta             int64 = 1_000_000_000_000_000
	unitKibi             int64 = 1_024
	unitMebi             int64 = 1_048_576
	unitGibi             int64 = 1_073_741_824
	unitTebi             int64 = 1_099_511_627_776
	unitPebi             int64 = 1_125_899_906_842_624
	loggingFieldType           = "type"
	loggingFieldName           = "name"
	loggingFieldPath           = "path"
	loggingFieldFacility       = "facility"
	loggingFieldTag            = "tag"
	loggingFieldContexts       = "contexts"
)

var loggingReservedFields = map[string]struct{}{
	loggingFieldType:     {},
	loggingFieldName:     {},
	loggingFieldPath:     {},
	loggingFieldFacility: {},
	loggingFieldTag:      {},
	loggingFieldContexts: {},
}

// maybeTranslateServerYAMLInput translates YAML input to the legacy asconfig
// YAML shape when the user enables --server-yaml.
func maybeTranslateServerYAMLInput(cmd *cobra.Command, srcFormat asConf.Format, cfgData []byte) ([]byte, error) {
	if srcFormat != asConf.YAML || cmd == nil {
		return cfgData, nil
	}

	translate, err := cmd.Flags().GetBool(flagServerYAML)
	if err != nil {
		return nil, err
	}

	if !translate {
		return cfgData, nil
	}

	return TranslateServerYAMLToLegacy(cfgData)
}

// maybeTranslateServerYAMLOutput translates YAML output from legacy asconfig YAML
// to server experimental YAML when --server-yaml-output is enabled.
func maybeTranslateServerYAMLOutput(
	cmd *cobra.Command,
	outFormat asConf.Format,
	asVersion string,
	cfgData []byte,
) ([]byte, error) {
	if cmd == nil {
		return cfgData, nil
	}

	translate, err := cmd.Flags().GetBool(flagServerYAMLOutput)
	if err != nil {
		return nil, err
	}

	if !translate {
		return cfgData, nil
	}

	if outFormat != asConf.YAML {
		return nil, errServerYAMLOutputRequiresYAML
	}

	if asVersion == "" {
		return nil, errMissingAerospikeVersion
	}

	supportedVersion, err := isServerYAMLOutputVersionSupported(asVersion)
	if err != nil {
		return nil, err
	}

	if !supportedVersion {
		return nil, fmt.Errorf("%w: got %s", errServerYAMLOutputUnsupportedVersion, asVersion)
	}

	return TranslateLegacyYAMLToServerYAML(cfgData)
}

func isServerYAMLOutputVersionSupported(version string) (bool, error) {
	version = strings.TrimPrefix(version, "ee-")

	compareResult, err := lib.CompareVersions(version, minServerYAMLOutputVersion)
	if err != nil {
		return false, err
	}

	return compareResult >= 0, nil
}

// TranslateServerYAMLToLegacy converts server experimental YAML to the legacy
// asconfig YAML shape currently understood by asconfig/mgmt-lib.
func TranslateServerYAMLToLegacy(cfgData []byte) ([]byte, error) {
	parsed := map[string]any{}
	if err := yaml.Unmarshal(cfgData, parsed); err != nil {
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

// TranslateLegacyYAMLToServerYAML converts legacy asconfig YAML to server
// experimental YAML shape.
func TranslateLegacyYAMLToServerYAML(cfgData []byte) ([]byte, error) {
	parsed := map[string]any{}
	if err := yaml.Unmarshal(cfgData, parsed); err != nil {
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
	rawName, hasName := sinkMap[loggingFieldName]
	if !hasName {
		return nil
	}

	name, isString := rawName.(string)
	if !isString {
		return fmt.Errorf("logging.%d.name must be a string, found %T", index, rawName)
	}

	if _, hasType := sinkMap[loggingFieldType]; !hasType {
		sinkMap[loggingFieldType] = name
	}

	delete(sinkMap, loggingFieldName)

	return nil
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

		if isLoggingLevelValue(value) {
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

func isLoggingLevelValue(value any) bool {
	level, ok := value.(string)
	if !ok {
		return false
	}

	return LoggingEnum[strings.ToLower(level)]
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

	if _, hasName := logSinkMap[loggingFieldName]; !hasName {
		typeName, isString := rawType.(string)
		if !isString {
			return fmt.Errorf("logging.%d.type must be a string, found %T", index, rawType)
		}

		logSinkMap[loggingFieldName] = typeName
	}

	delete(logSinkMap, loggingFieldType)

	return nil
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
	names := make([]string, 0, len(namedMap))
	for name := range namedMap {
		names = append(names, name)
	}

	sort.Strings(names)

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

func translateUnitValues(node any, path []string) (any, error) {
	switch typedNode := node.(type) {
	case map[string]any:
		if converted, wasUnitObject, err := tryConvertUnitObject(typedNode, path); err != nil {
			return nil, err
		} else if wasUnitObject {
			return converted, nil
		}

		for key, value := range typedNode {
			translated, err := translateUnitValues(value, append(path, key))
			if err != nil {
				return nil, err
			}

			typedNode[key] = translated
		}

		return typedNode, nil
	case []any:
		for i, value := range typedNode {
			translated, err := translateUnitValues(value, append(path, strconv.Itoa(i)))
			if err != nil {
				return nil, err
			}

			typedNode[i] = translated
		}

		return typedNode, nil
	case string:
		return typedNode, nil
	default:
		return typedNode, nil
	}
}

func tryConvertUnitObject(obj map[string]any, path []string) (int64, bool, error) {
	rawValue, hasValue := obj["value"]
	rawUnit, hasUnit := obj["unit"]
	if !hasValue || !hasUnit || len(obj) != 2 {
		return 0, false, nil
	}

	valueInt, err := asInt64(rawValue)
	if err != nil {
		return 0, true, fmt.Errorf("%s.value: %w", formatNodePath(path), err)
	}

	unit, ok := rawUnit.(string)
	if !ok {
		return 0, true, fmt.Errorf("%s.unit must be a string, found %T", formatNodePath(path), rawUnit)
	}

	multiplier, err := getUnitMultiplier(unit, path)
	if err != nil {
		return 0, true, err
	}

	result, err := multiplyInt64Checked(valueInt, multiplier)
	if err != nil {
		return 0, true, fmt.Errorf("%s: %w", formatNodePath(path), err)
	}

	return result, true, nil
}

func getUnitMultiplier(unit string, path []string) (int64, error) {
	switch strings.ToLower(unit) {
	case "s":
		return unitSecond, nil
	case "m":
		if isDurationPath(path) {
			return unitMinute, nil
		}

		return unitMega, nil
	case "h":
		return unitHour, nil
	case "d":
		return unitDay, nil
	case "k":
		return unitKilo, nil
	case "g":
		return unitGiga, nil
	case "t":
		return unitTera, nil
	case "p":
		return unitPeta, nil
	case "ki":
		return unitKibi, nil
	case "mi":
		return unitMebi, nil
	case "gi":
		return unitGibi, nil
	case "ti":
		return unitTebi, nil
	case "pi":
		return unitPebi, nil
	default:
		return 0, fmt.Errorf("%s: unsupported unit %q", formatNodePath(path), unit)
	}
}

func isDurationPath(path []string) bool {
	field := currentFieldName(path)
	if field == "" {
		return false
	}

	// Fields that already represent milliseconds use SI multipliers.
	if strings.HasSuffix(field, "-ms") {
		return false
	}

	durationTokens := []string{
		"ttl",
		"period",
		"timeout",
		"delay",
		"interval",
		"duration",
		"sleep",
		"age",
		"refresh",
	}

	for _, token := range durationTokens {
		if strings.Contains(field, token) {
			return true
		}
	}

	return false
}

func currentFieldName(path []string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if _, err := strconv.Atoi(path[i]); err == nil {
			continue
		}

		return strings.ToLower(path[i])
	}

	return ""
}

func multiplyInt64Checked(value, multiplier int64) (int64, error) {
	product := new(big.Int).Mul(big.NewInt(value), big.NewInt(multiplier))
	if !product.IsInt64() {
		return 0, errors.New("converted value overflows int64")
	}

	return product.Int64(), nil
}

func asInt64(value any) (int64, error) {
	switch typedValue := value.(type) {
	case int:
		return int64(typedValue), nil
	case int8:
		return int64(typedValue), nil
	case int16:
		return int64(typedValue), nil
	case int32:
		return int64(typedValue), nil
	case int64:
		return typedValue, nil
	case uint:
		if typedValue > math.MaxInt64 {
			return 0, fmt.Errorf("value %d exceeds int64 max", typedValue)
		}

		return int64(typedValue), nil
	case uint8:
		return int64(typedValue), nil
	case uint16:
		return int64(typedValue), nil
	case uint32:
		return int64(typedValue), nil
	case uint64:
		if typedValue > math.MaxInt64 {
			return 0, fmt.Errorf("value %d exceeds int64 max", typedValue)
		}

		return int64(typedValue), nil
	case float32:
		floatValue := float64(typedValue)
		if floatValue != math.Trunc(floatValue) {
			return 0, fmt.Errorf("value %v is not an integer", typedValue)
		}

		if floatValue > math.MaxInt64 || floatValue < math.MinInt64 {
			return 0, fmt.Errorf("value %v exceeds int64 range", typedValue)
		}

		return int64(floatValue), nil
	case float64:
		if typedValue != math.Trunc(typedValue) {
			return 0, fmt.Errorf("value %v is not an integer", typedValue)
		}

		if typedValue > math.MaxInt64 || typedValue < math.MinInt64 {
			return 0, fmt.Errorf("value %v exceeds int64 range", typedValue)
		}

		return int64(typedValue), nil
	case string:
		parsedValue, err := strconv.ParseInt(typedValue, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("value %q is not a valid integer", typedValue)
		}

		return parsedValue, nil
	default:
		return 0, fmt.Errorf("unsupported numeric type %T", value)
	}
}

func formatNodePath(path []string) string {
	if len(path) == 0 {
		return "(root)"
	}

	return strings.Join(path, ".")
}
