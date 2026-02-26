//go:build unit

package cmd

import (
	"bytes"
	"strings"
	"testing"
)

// TestArrayChangeFormatter tests the formatter logic independently of rendering
func TestArrayChangeFormatter(t *testing.T) {
	tests := []struct {
		name          string
		change        SchemaChange
		path          string
		expectMod     bool
		expectSymbol  string
		expectCompact string
	}{
		{
			name: "simple array addition",
			change: SchemaChange{
				Type:  Addition,
				Path:  "/items/-",
				Value: "test-value",
			},
			path:          "items[+]",
			expectMod:     false,
			expectSymbol:  "+",
			expectCompact: "test-value",
		},
		{
			name: "array modification",
			change: SchemaChange{
				Type:         Modification,
				Path:         "/items/0",
				OldFullValue: "old",
				NewFullValue: "new",
			},
			path:          "items[0]",
			expectMod:     true,
			expectSymbol:  "~",
			expectCompact: "old → new",
		},
		{
			name: "complex object array addition",
			change: SchemaChange{
				Type: Addition,
				Path: "/items/-",
				Value: map[string]any{
					"type":    "string",
					"default": "test",
				},
			},
			path:      "items[+]",
			expectMod: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewArrayChangeFormatter(tt.change, tt.path)

			// Test IsModification
			if got := formatter.IsModification(); got != tt.expectMod {
				t.Errorf("IsModification() = %v, want %v", got, tt.expectMod)
			}

			// Test GetSymbol (ASCII)
			if tt.expectSymbol != "" {
				if got := formatter.GetSymbol(false); got != tt.expectSymbol {
					t.Errorf("GetSymbol(false) = %q, want %q", got, tt.expectSymbol)
				}
			}

			// Test FormatValue (compact)
			if tt.expectCompact != "" {
				got := formatter.FormatValue(true)
				if !strings.Contains(got, tt.expectCompact) {
					t.Errorf("FormatValue(true) = %q, should contain %q", got, tt.expectCompact)
				}
			}
		})
	}
}

// TestVerboseRenderer tests verbose rendering output
func TestVerboseRenderer(t *testing.T) {
	tests := []struct {
		name          string
		change        SchemaChange
		path          string
		expectedParts []string
		notExpected   []string
	}{
		{
			name: "simple array addition",
			change: SchemaChange{
				Type:  Addition,
				Path:  "/items/-",
				Value: "Bang!",
			},
			path: "items[+]",
			expectedParts: []string{
				"✅",
				"items[+]",
				"Array item: Bang!",
			},
			notExpected: []string{
				"map[",
				"interface{}",
			},
		},
		{
			name: "array modification",
			change: SchemaChange{
				Type:         Modification,
				Path:         "/items/0",
				OldFullValue: []any{"old1", "old2"},
				NewFullValue: []any{"new1", "new2"},
			},
			path: "items[0]",
			expectedParts: []string{
				"🔄",
				"items[0]",
				"Changed from:",
				"Changed to:",
			},
		},
		{
			name: "complex object addition",
			change: SchemaChange{
				Type: Addition,
				Path: "/network/admin",
				Value: map[string]any{
					"type":    "object",
					"default": 8080,
				},
			},
			path: "network.admin",
			expectedParts: []string{
				"✅",
				"network.admin",
				"Array item:",
				"Type: object",
				"Default: 8080",
			},
			notExpected: []string{
				"map[",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			renderer := NewVerboseRenderer(&buf)

			output, err := renderer.RenderArrayChange(tt.change, tt.path)
			if err != nil {
				t.Fatalf("RenderArrayChange() error = %v", err)
			}

			// Check expected content
			for _, expected := range tt.expectedParts {
				if !strings.Contains(output, expected) {
					t.Errorf("Output missing expected content %q.\nGot:\n%s", expected, output)
				}
			}

			// Check unwanted content
			for _, notExpected := range tt.notExpected {
				if strings.Contains(output, notExpected) {
					t.Errorf("Output contains unwanted content %q.\nGot:\n%s", notExpected, output)
				}
			}
		})
	}
}

// TestCompactRenderer tests compact rendering output
func TestCompactRenderer(t *testing.T) {
	tests := []struct {
		name           string
		change         SchemaChange
		path           string
		expectedFormat string
	}{
		{
			name: "simple addition",
			change: SchemaChange{
				Type:  Addition,
				Path:  "/items/-",
				Value: "test",
			},
			path:           "items[+]",
			expectedFormat: "+ items[+] (array item: test)",
		},
		{
			name: "simple removal",
			change: SchemaChange{
				Type:  Removal,
				Path:  "/items/0",
				Value: "removed",
			},
			path:           "items[0]",
			expectedFormat: "- items[0] (array item: removed)",
		},
		{
			name: "modification",
			change: SchemaChange{
				Type:         Modification,
				Path:         "/items/0",
				OldFullValue: 100,
				NewFullValue: 200,
			},
			path:           "items[0]",
			expectedFormat: "~ items[0] (100 → 200)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			renderer := NewCompactRenderer(&buf)

			output, err := renderer.RenderArrayChange(tt.change, tt.path)
			if err != nil {
				t.Fatalf("RenderArrayChange() error = %v", err)
			}

			// Normalize whitespace for comparison
			normalized := strings.TrimSpace(output)
			if normalized != tt.expectedFormat {
				t.Errorf("Output mismatch.\nGot:  %q\nWant: %q", normalized, tt.expectedFormat)
			}
		})
	}
}

// TestRenderChange tests the main entry point
func TestRenderChange(t *testing.T) {
	change := SchemaChange{
		Type:  Addition,
		Path:  "/items/-",
		Value: "test",
	}

	tests := []struct {
		name    string
		verbose bool
		expect  []string
	}{
		{
			name:    "verbose mode",
			verbose: true,
			expect:  []string{"✅", "Array item:"},
		},
		{
			name:    "compact mode",
			verbose: false,
			expect:  []string{"+", "array item:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			options := DiffOptions{Verbose: tt.verbose}

			err := RenderChange(&buf, change, "items[+]", options)
			if err != nil {
				t.Fatalf("RenderChange() error = %v", err)
			}

			output := buf.String()
			for _, expected := range tt.expect {
				if !strings.Contains(output, expected) {
					t.Errorf("Output missing %q.\nGot: %s", expected, output)
				}
			}
		})
	}
}

// BenchmarkRendering compares rendering performance
func BenchmarkVerboseRendering(b *testing.B) {
	change := SchemaChange{
		Type: Addition,
		Path: "/network/admin",
		Value: map[string]any{
			"type":    "object",
			"default": 8080,
			"properties": map[string]any{
				"endpoint": map[string]any{"type": "string"},
				"port":     map[string]any{"type": "integer"},
			},
		},
	}

	var buf bytes.Buffer
	renderer := NewVerboseRenderer(&buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		_, _ = renderer.RenderArrayChange(change, "network.admin")
	}
}

func BenchmarkCompactRendering(b *testing.B) {
	change := SchemaChange{
		Type:  Addition,
		Path:  "/items/-",
		Value: "test-value",
	}

	var buf bytes.Buffer
	renderer := NewCompactRenderer(&buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		_, _ = renderer.RenderArrayChange(change, "items[+]")
	}
}


