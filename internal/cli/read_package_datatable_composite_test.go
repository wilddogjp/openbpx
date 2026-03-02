package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDataTableReadCompositeParentReferences(t *testing.T) {
	tests := []struct {
		name        string
		fixturePath string
	}{
		{
			name:        "add row reject fixture",
			fixturePath: filepath.Join("..", "..", "testdata", "golden", "operations", "dt_add_row_composite_reject", "before.uasset"),
		},
		{
			name:        "remove row reject fixture",
			fixturePath: filepath.Join("..", "..", "testdata", "golden", "operations", "dt_remove_row_composite_reject", "before.uasset"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			table := runDataTableReadAndFirstTable(t, tc.fixturePath)
			if got := strings.TrimSpace(anyToString(table["className"])); got != "CompositeDataTable" {
				t.Fatalf("expected CompositeDataTable class, got %q", got)
			}

			count, ok := anyToFloat64(table["compositeParentCount"])
			if !ok {
				t.Fatalf("compositeParentCount missing in output")
			}
			if count < 1 {
				t.Fatalf("compositeParentCount must be >= 1, got %v", count)
			}

			parentsRaw, ok := table["compositeParents"].([]any)
			if !ok {
				t.Fatalf("compositeParents missing in output")
			}
			if len(parentsRaw) == 0 {
				t.Fatalf("compositeParents must include at least one linked parent table")
			}

			first, ok := parentsRaw[0].(map[string]any)
			if !ok {
				t.Fatalf("first composite parent output shape mismatch")
			}
			if got := strings.TrimSpace(anyToString(first["kind"])); got == "" {
				t.Fatalf("kind is empty in composite parent entry")
			}
			if got := strings.TrimSpace(anyToString(first["resolved"])); got == "" {
				t.Fatalf("resolved is empty in composite parent entry")
			}
			if got := strings.TrimSpace(anyToString(first["targetPath"])); got == "" {
				t.Fatalf("targetPath is empty in composite parent entry")
			}
			if got := strings.TrimSpace(anyToString(first["targetObjectPath"])); got == "" {
				t.Fatalf("targetObjectPath is empty in composite parent entry")
			}
		})
	}
}

func TestDataTableReadNonCompositeHasNoCompositeParentReferences(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "golden", "operations", "dt_add_row", "before.uasset")
	table := runDataTableReadAndFirstTable(t, fixturePath)
	if got := strings.TrimSpace(anyToString(table["className"])); got != "DataTable" {
		t.Fatalf("expected DataTable class, got %q", got)
	}
	if _, exists := table["compositeParents"]; exists {
		t.Fatalf("compositeParents must not be present for non-composite datatable")
	}
	if _, exists := table["compositeParentCount"]; exists {
		t.Fatalf("compositeParentCount must not be present for non-composite datatable")
	}
}

func runDataTableReadAndFirstTable(t *testing.T, fixturePath string) map[string]any {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"datatable", "read", fixturePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("datatable read failed: %s", stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal datatable read output: %v", err)
	}

	tablesRaw, ok := payload["tables"].([]any)
	if !ok || len(tablesRaw) == 0 {
		t.Fatalf("tables not found in output")
	}
	table, ok := tablesRaw[0].(map[string]any)
	if !ok {
		t.Fatalf("table output shape mismatch")
	}
	return table
}

func anyToString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func anyToFloat64(v any) (float64, bool) {
	if f, ok := v.(float64); ok {
		return f, true
	}
	return 0, false
}
