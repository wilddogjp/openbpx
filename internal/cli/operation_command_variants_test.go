package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOperationFixtureCommandVariants(t *testing.T) {
	operationsDir := filepath.Join("..", "..", "testdata", "golden", "operations")
	required := map[string]string{
		"prop_add_fixture_int":           "prop add",
		"prop_remove_fixture_int":        "prop remove",
		"dt_add_row_values_scalar":       "datatable add-row",
		"dt_add_row_values_mixed":        "datatable add-row",
		"dt_remove_row_base":             "datatable remove-row",
		"var_set_default_empty":          "var set-default",
		"var_set_default_unicode":        "var set-default",
		"var_set_default_long":           "var set-default",
		"var_rename_simple":              "var rename",
		"var_rename_unicode":             "var rename",
		"var_rename_with_refs":           "var rename",
		"ref_rewrite_single":             "ref rewrite",
		"ref_rewrite_multi":              "ref rewrite",
		"stringtable_remove_entry":       "stringtable remove-entry",
		"stringtable_set_namespace":      "stringtable set-namespace",
		"localization_rewrite_namespace": "localization rewrite-namespace",
		"localization_rekey":             "localization rekey",
	}

	for name, wantCommand := range required {
		specPath := filepath.Join(operationsDir, name, "operation.json")
		body, err := os.ReadFile(specPath)
		if err != nil {
			t.Fatalf("read %s operation spec: %v", name, err)
		}
		var spec operationSpec
		if err := json.Unmarshal(body, &spec); err != nil {
			t.Fatalf("parse %s operation spec: %v", name, err)
		}
		if got := strings.TrimSpace(spec.Command); got != wantCommand {
			t.Fatalf("%s command mismatch: got %q want %q", name, got, wantCommand)
		}
		if got := strings.TrimSpace(spec.Expect); got != "byte_equal" {
			t.Fatalf("%s expect mismatch: got %q want %q", name, got, "byte_equal")
		}
		if len(spec.Args) == 0 {
			t.Fatalf("%s has empty args", name)
		}
	}
}
