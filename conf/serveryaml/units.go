package serveryaml

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

const (
	unitSecond int64 = 1
	unitMinute int64 = 60
	unitHour   int64 = 3_600
	unitDay    int64 = 86_400
	unitKilo   int64 = 1_000
	unitMega   int64 = 1_000_000
	unitGiga   int64 = 1_000_000_000
	unitTera   int64 = 1_000_000_000_000
	unitPeta   int64 = 1_000_000_000_000_000
	unitKibi   int64 = 1_024
	unitMebi   int64 = 1_048_576
	unitGibi   int64 = 1_073_741_824
	unitTebi   int64 = 1_099_511_627_776
	unitPebi   int64 = 1_125_899_906_842_624
)

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
			translated, err := translateUnitValues(value, pathWithIndex(path, i))
			if err != nil {
				return nil, err
			}

			typedNode[i] = translated
		}

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
