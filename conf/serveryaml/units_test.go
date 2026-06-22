//go:build unit

package serveryaml

import (
	"math"
	"strings"
	"testing"
)

// TestGetUnitMultiplierAllUnits exercises every supported unit to make sure
// the multipliers line up with Aerospike's documented conversions. The
// multiplier math is simple but this test locks the table in place so
// renaming or reordering cases won't silently change a multiplier.
func TestGetUnitMultiplierAllUnits(t *testing.T) {
	// Cases that are unambiguous: the unit token itself decides the value.
	sizeCases := []struct {
		unit     string
		expected int64
	}{
		{"s", 1},
		{"S", 1},
		{"h", 3_600},
		{"d", 86_400},
		{"k", 1_000},
		{"K", 1_000},
		{"g", 1_000_000_000},
		{"G", 1_000_000_000},
		{"t", 1_000_000_000_000},
		{"p", 1_000_000_000_000_000},
		{"ki", 1_024},
		{"mi", 1_048_576},
		{"gi", 1_073_741_824},
		{"ti", 1_099_511_627_776},
		{"pi", 1_125_899_906_842_624},
	}

	for _, tc := range sizeCases {
		t.Run("size_unit_"+tc.unit, func(t *testing.T) {
			got, err := getUnitMultiplier(tc.unit, []string{"namespaces", "test", "data-size"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.expected {
				t.Fatalf("unit %q: expected %d, got %d", tc.unit, tc.expected, got)
			}
		})
	}
}

// TestGetUnitMultiplierMIsDurationAware captures the one ambiguous unit ("m")
// which is minutes in duration contexts but megabytes everywhere else. This
// also pins down the -ms exception: "m" on a field whose name ends in -ms
// must NOT be interpreted as minutes.
func TestGetUnitMultiplierMIsDurationAware(t *testing.T) {
	cases := []struct {
		name     string
		path     []string
		expected int64
	}{
		{
			name:     "no context defaults to mega",
			path:     nil,
			expected: 1_000_000,
		},
		{
			name:     "size-like field is mega",
			path:     []string{"namespaces", "test", "storage-engine", "data-size"},
			expected: 1_000_000,
		},
		{
			name:     "ttl is minutes",
			path:     []string{"namespaces", "test", "default-ttl"},
			expected: 60,
		},
		{
			name:     "period is minutes",
			path:     []string{"namespaces", "test", "nsup-period"},
			expected: 60,
		},
		{
			name:     "timeout is minutes",
			path:     []string{"network", "heartbeat", "connect-timeout"},
			expected: 60,
		},
		{
			name:     "interval is minutes",
			path:     []string{"xdr", "dcs", "dc1", "ship-versions-interval"},
			expected: 60,
		},
		{
			name:     "sleep is minutes",
			path:     []string{"service", "migrate-sleep"},
			expected: 60,
		},
		{
			name:     "refresh is minutes",
			path:     []string{"network", "tls", "tls1", "tls-refresh-period"},
			expected: 60,
		},
		{
			name:     "-ms suffix suppresses duration interpretation",
			path:     []string{"service", "timeout-ms"},
			expected: 1_000_000,
		},
		{
			name:     "numeric trailing path segments are skipped when looking up field",
			path:     []string{"xdr", "dcs", "0", "ship-versions-interval", "0"},
			expected: 60,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getUnitMultiplier("m", tc.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.expected {
				t.Fatalf("expected %d, got %d", tc.expected, got)
			}
		})
	}
}

func TestGetUnitMultiplierUnknownUnit(t *testing.T) {
	_, err := getUnitMultiplier("zz", []string{"namespaces"})
	if err == nil {
		t.Fatalf("expected error for unknown unit, got nil")
	}

	if !strings.Contains(err.Error(), "unsupported unit") {
		t.Fatalf("expected unsupported unit error, got: %v", err)
	}
}

// TestTryConvertUnitObjectOnlyConvertsExactShape makes sure objects that
// happen to have value/unit plus extra fields are NOT converted away. This
// protects user-specified objects like storage-engine that might otherwise
// look unit-like to a naïve implementation.
func TestTryConvertUnitObjectOnlyConvertsExactShape(t *testing.T) {
	cases := []struct {
		name      string
		obj       map[string]any
		converted bool
	}{
		{
			name:      "value+unit converts",
			obj:       map[string]any{"value": 4, "unit": "g"},
			converted: true,
		},
		{
			name:      "extra keys skip conversion",
			obj:       map[string]any{"value": 4, "unit": "g", "extra": "x"},
			converted: false,
		},
		{
			name:      "missing unit skip conversion",
			obj:       map[string]any{"value": 4},
			converted: false,
		},
		{
			name:      "missing value skip conversion",
			obj:       map[string]any{"unit": "g"},
			converted: false,
		},
		{
			name:      "empty map skip conversion",
			obj:       map[string]any{},
			converted: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, wasUnit, err := tryConvertUnitObject(tc.obj, []string{"namespaces", "test", "data-size"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if wasUnit != tc.converted {
				t.Fatalf("expected wasUnit=%v, got %v", tc.converted, wasUnit)
			}
		})
	}
}

func TestTryConvertUnitObjectErrorsOnBadShape(t *testing.T) {
	cases := []struct {
		name    string
		obj     map[string]any
		wantErr string
	}{
		{
			name:    "value is not numeric",
			obj:     map[string]any{"value": []any{1, 2}, "unit": "g"},
			wantErr: "unsupported numeric type",
		},
		{
			name:    "unit is not a string",
			obj:     map[string]any{"value": 4, "unit": 123},
			wantErr: "unit must be a string",
		},
		{
			name:    "unit is unknown",
			obj:     map[string]any{"value": 4, "unit": "zz"},
			wantErr: "unsupported unit",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, wasUnit, err := tryConvertUnitObject(tc.obj, []string{"namespaces", "test", "data-size"})
			if err == nil {
				t.Fatalf("expected error, got nil")
			}

			if !wasUnit {
				t.Fatalf("expected wasUnit=true so caller knows the object looked unit-like")
			}

			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

func TestMultiplyInt64CheckedOverflow(t *testing.T) {
	if _, err := multiplyInt64Checked(math.MaxInt64, 2); err == nil {
		t.Fatalf("expected overflow error, got nil")
	}

	if _, err := multiplyInt64Checked(math.MinInt64, 2); err == nil {
		t.Fatalf("expected overflow error on negative side, got nil")
	}

	got, err := multiplyInt64Checked(3, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != 15 {
		t.Fatalf("expected 15, got %d", got)
	}
}

// TestAsInt64AllNumericTypes runs the conversion against every concrete type
// the yaml.v3 decoder might hand us so that new numeric decoder quirks don't
// regress silently.
func TestAsInt64AllNumericTypes(t *testing.T) {
	cases := []struct {
		name     string
		value    any
		expected int64
	}{
		{"int", int(5), 5},
		{"int8", int8(5), 5},
		{"int16", int16(5), 5},
		{"int32", int32(5), 5},
		{"int64", int64(5), 5},
		{"uint", uint(5), 5},
		{"uint8", uint8(5), 5},
		{"uint16", uint16(5), 5},
		{"uint32", uint32(5), 5},
		{"uint64", uint64(5), 5},
		{"float32 whole", float32(5), 5},
		{"float64 whole", float64(5), 5},
		{"string numeric", "5", 5},
		{"negative int", -7, -7},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := asInt64(tc.value)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.expected {
				t.Fatalf("expected %d, got %d", tc.expected, got)
			}
		})
	}
}

func TestAsInt64RejectsNonIntegerValues(t *testing.T) {
	cases := []struct {
		name    string
		value   any
		wantErr string
	}{
		{"bool is unsupported", true, "unsupported numeric type"},
		{"nil is unsupported", nil, "unsupported numeric type"},
		{"slice is unsupported", []any{1}, "unsupported numeric type"},
		{"map is unsupported", map[string]any{"k": 1}, "unsupported numeric type"},
		{"fractional float", 5.5, "not an integer"},
		{"NaN float", math.NaN(), "not an integer"},
		{"inf float is out of range", math.Inf(1), "exceeds int64 range"},
		{"non-numeric string", "abc", "not a valid integer"},
		{"uint64 above int64 max", uint64(math.MaxUint64), "exceeds int64 max"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := asInt64(tc.value)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}

			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

// TestTranslateUnitValuesRecursesIntoSlices makes sure unit objects embedded
// inside list elements (not just maps) are still converted. This is the path
// that namespaces[] -> storage-engine{value,unit} takes once namespaces have
// been translated to a slice by ToLegacy.
func TestTranslateUnitValuesRecursesIntoSlices(t *testing.T) {
	root := map[string]any{
		"namespaces": []any{
			map[string]any{
				"name": "test",
				"storage-engine": map[string]any{
					"type": "memory",
					"data-size": map[string]any{
						"value": 4,
						"unit":  "g",
					},
				},
			},
		},
	}

	converted, err := translateUnitValues(root, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rootMap, ok := converted.(map[string]any)
	if !ok {
		t.Fatalf("expected root to remain a map, got %T", converted)
	}

	namespaces, ok := rootMap["namespaces"].([]any)
	if !ok {
		t.Fatalf("expected namespaces to be a slice")
	}

	ns := namespaces[0].(map[string]any)
	storage := ns["storage-engine"].(map[string]any)
	got, err := asInt64(storage["data-size"])
	if err != nil {
		t.Fatalf("expected data-size to be numeric, got: %v", err)
	}

	if got != 4_000_000_000 {
		t.Fatalf("expected 4g -> 4_000_000_000, got %d", got)
	}
}
