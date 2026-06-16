package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOperationFixtureCommandCoverage(t *testing.T) {
	requiredByteEqualMin := map[string]int{
		"write":                            2,
		"prop set":                         26,
		"prop add":                         2,
		"prop remove":                      2,
		"var set-default":                  6,
		"var rename":                       3,
		"ref rewrite":                      2,
		"name add":                         2,
		"name set":                         2,
		"name remove":                      1,
		"metadata set-object":              2,
		"enum write-value":                 2,
		"stringtable write-entry":          2,
		"stringtable remove-entry":         1,
		"stringtable set-namespace":        1,
		"localization set-source":          2,
		"localization set-id":              2,
		"localization set-stringtable-ref": 1,
		"localization rewrite-namespace":   1,
		"localization rekey":               1,
		"datatable add-row":                3,
		"datatable remove-row":             2,
		"datatable update-row":             4,
		"metadata set-root":                1,
		"package set-flags":                2,
	}
	requiredErrorEqualMin := map[string]int{
		"prop set":                         2,
		"datatable add-row":                1,
		"datatable remove-row":             1,
		"datatable update-row":             2,
		"metadata set-root":                1,
		"name remove":                      2,
		"enum write-value":                 1,
		"level var-set":                    3,
		"localization set-stringtable-ref": 1,
	}

	roots := goldenFixtureRoots(t, "operations")
	if len(roots) == 0 {
		t.Fatalf("no operations fixture roots found")
	}

	for _, root := range roots {
		root := root
		t.Run(filepath.Base(root), func(t *testing.T) {
			operationsDir := filepath.Join(root, "operations")
			entries, err := os.ReadDir(operationsDir)
			if err != nil {
				t.Fatalf("read operations dir: %v", err)
			}

			byteEqualCountByCommand := map[string]int{}
			errorEqualCountByCommand := map[string]int{}
			for _, opDir := range listOperationSpecDirs(entries, operationsDir) {
				specPath := filepath.Join(opDir, "operation.json")
				body, err := os.ReadFile(specPath)
				if err != nil {
					t.Fatalf("read %s operation spec: %v", filepath.Base(opDir), err)
				}
				var spec operationSpec
				if err := json.Unmarshal(body, &spec); err != nil {
					t.Fatalf("parse %s operation spec: %v", filepath.Base(opDir), err)
				}
				command := strings.TrimSpace(spec.Command)
				if command == "" {
					t.Fatalf("%s operation command is empty", filepath.Base(opDir))
				}
				expect := strings.TrimSpace(spec.Expect)
				if expect == "" {
					expect = "byte_equal"
				}
				if expect == "byte_equal" {
					byteEqualCountByCommand[command]++
					continue
				}
				if expect == "error_equal" {
					errorEqualCountByCommand[command]++
				}
			}

			for command, minCount := range requiredByteEqualMin {
				got := byteEqualCountByCommand[command]
				if got < minCount {
					t.Fatalf("insufficient byte_equal operation coverage for %q: got %d want >= %d", command, got, minCount)
				}
			}
			for command, minCount := range requiredErrorEqualMin {
				got := errorEqualCountByCommand[command]
				if got < minCount {
					t.Fatalf("insufficient error_equal operation coverage for %q: got %d want >= %d", command, got, minCount)
				}
			}
		})
	}
}
