package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOperationFixtureCustomTypeCoverage(t *testing.T) {
	operationsDir := filepath.Join("..", "..", "testdata", "golden", "operations")
	required := map[string]string{
		"prop_set_enum":               "byte_equal",
		"prop_set_enum_numeric":       "unsupported",
		"prop_set_enum_anchor":        "byte_equal",
		"prop_set_custom_struct_int":  "byte_equal",
		"prop_set_custom_struct_enum": "unsupported",
	}

	for name, wantExpect := range required {
		specPath := filepath.Join(operationsDir, name, "operation.json")
		body, err := os.ReadFile(specPath)
		if err != nil {
			t.Fatalf("read %s operation spec: %v", name, err)
		}
		var spec operationSpec
		if err := json.Unmarshal(body, &spec); err != nil {
			t.Fatalf("parse %s operation spec: %v", name, err)
		}
		if strings.TrimSpace(spec.Command) != "prop set" {
			t.Fatalf("%s command mismatch: got %q want %q", name, spec.Command, "prop set")
		}
		if strings.TrimSpace(spec.Expect) != wantExpect {
			t.Fatalf("%s expect mismatch: got %q want %q", name, spec.Expect, wantExpect)
		}
		if len(spec.Args) == 0 {
			t.Fatalf("%s has empty args", name)
		}
	}
}
