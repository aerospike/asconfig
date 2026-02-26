package cmd

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"

	asConf "github.com/aerospike/aerospike-management-lib/asconfig"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const flagServerYAML = "server-yaml"

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
	rawNamespaces, ok := root["namespaces"]
	if !ok {
		return nil
	}

	namespacesMap, ok := rawNamespaces.(map[string]any)
	if !ok {
		// Already in legacy array form (or unknown shape). Leave unchanged.
		return nil
	}

	for namespaceName, rawNamespace := range namespacesMap {
		namespaceMap, ok := rawNamespace.(map[string]any)
		if !ok {
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
	rawNetwork, ok := root["network"]
	if !ok {
		return nil
	}

	networkMap, ok := rawNetwork.(map[string]any)
	if !ok {
		return fmt.Errorf("network must be an object, found %T", rawNetwork)
	}

	return translateNamedMapInObjectToSlice(networkMap, "tls", []string{"network", "tls"})
}

func translateXDR(root map[string]any) error {
	rawXDR, ok := root["xdr"]
	if !ok {
		return nil
	}

	xdrMap, ok := rawXDR.(map[string]any)
	if !ok {
		return fmt.Errorf("xdr must be an object, found %T", rawXDR)
	}

	rawDCs, ok := xdrMap["dcs"]
	if !ok {
		return nil
	}

	dcMap, ok := rawDCs.(map[string]any)
	if !ok {
		// Already in legacy array form (or unknown shape). Leave unchanged.
		return nil
	}

	for dcName, rawDC := range dcMap {
		dcObj, ok := rawDC.(map[string]any)
		if !ok {
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
	rawLogging, ok := root["logging"]
	if !ok {
		return nil
	}

	loggingSlice, ok := rawLogging.([]any)
	if !ok {
		return nil
	}

	for i, rawLogSink := range loggingSlice {
		logSinkMap, ok := rawLogSink.(map[string]any)
		if !ok {
			return fmt.Errorf("logging.%d must be an object, found %T", i, rawLogSink)
		}

		if rawType, exists := logSinkMap["type"]; exists {
			if _, hasName := logSinkMap["name"]; !hasName {
				typeName, ok := rawType.(string)
				if !ok {
					return fmt.Errorf("logging.%d.type must be a string, found %T", i, rawType)
				}

				logSinkMap["name"] = typeName
			}

			delete(logSinkMap, "type")
		}

		if rawContexts, exists := logSinkMap["contexts"]; exists {
			contextsMap, ok := rawContexts.(map[string]any)
			if !ok {
				return fmt.Errorf("logging.%d.contexts must be an object, found %T", i, rawContexts)
			}

			for contextName, contextValue := range contextsMap {
				if _, exists := logSinkMap[contextName]; exists {
					return fmt.Errorf(
						"logging.%d context %q conflicts with existing sink field",
						i,
						contextName,
					)
				}

				logSinkMap[contextName] = contextValue
			}

			delete(logSinkMap, "contexts")
		}

		loggingSlice[i] = logSinkMap
	}

	root["logging"] = loggingSlice

	return nil
}

func translateNamedMapInObjectToSlice(parent map[string]any, key string, path []string) error {
	rawValue, ok := parent[key]
	if !ok {
		return nil
	}

	typedMap, ok := rawValue.(map[string]any)
	if !ok {
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
		itemMap, ok := rawItem.(map[string]any)
		if !ok {
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
		return 1, nil
	case "m":
		if isDurationPath(path) {
			return 60, nil
		}

		return 1_000_000, nil
	case "h":
		return 3_600, nil
	case "d":
		return 86_400, nil
	case "k":
		return 1_000, nil
	case "g":
		return 1_000_000_000, nil
	case "t":
		return 1_000_000_000_000, nil
	case "p":
		return 1_000_000_000_000_000, nil
	case "ki":
		return 1_024, nil
	case "mi":
		return 1_048_576, nil
	case "gi":
		return 1_073_741_824, nil
	case "ti":
		return 1_099_511_627_776, nil
	case "pi":
		return 1_125_899_906_842_624, nil
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
		return 0, fmt.Errorf("converted value overflows int64")
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
