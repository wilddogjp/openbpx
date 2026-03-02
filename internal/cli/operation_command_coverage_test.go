package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOperationFixtureCommandCoverage(t *testing.T) {
	operationsDir := filepath.Join("..", "..", "testdata", "golden", "operations")
	entries, err := os.ReadDir(operationsDir)
	if err != nil {
		t.Fatalf("read operations dir: %v", err)
	}

	requiredByteEqualMin := map[string]int{
		"prop set":                       20,
		"prop add":                       2,
		"prop remove":                    2,
		"var set-default":                4,
		"var rename":                     3,
		"ref rewrite":                    2,
		"stringtable remove-entry":       1,
		"stringtable set-namespace":      1,
		"localization rewrite-namespace": 1,
		"localization rekey":             1,
		"datatable add-row":              3,
		"datatable remove-row":           2,
	}
	byteEqualCountByCommand := map[string]int{}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		specPath := filepath.Join(operationsDir, entry.Name(), "operation.json")
		body, err := os.ReadFile(specPath)
		if err != nil {
			t.Fatalf("read %s operation spec: %v", entry.Name(), err)
		}
		var spec operationSpec
		if err := json.Unmarshal(body, &spec); err != nil {
			t.Fatalf("parse %s operation spec: %v", entry.Name(), err)
		}
		command := strings.TrimSpace(spec.Command)
		if command == "" {
			t.Fatalf("%s operation command is empty", entry.Name())
		}
		expect := strings.TrimSpace(spec.Expect)
		if expect == "" {
			expect = "byte_equal"
		}
		if expect == "byte_equal" {
			byteEqualCountByCommand[command]++
		}
	}

	for command, minCount := range requiredByteEqualMin {
		got := byteEqualCountByCommand[command]
		if got < minCount {
			t.Fatalf("insufficient byte_equal operation coverage for %q: got %d want >= %d", command, got, minCount)
		}
	}
}
