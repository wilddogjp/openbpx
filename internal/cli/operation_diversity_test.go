package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOperationFixtureDiversity(t *testing.T) {
	required := []string{
		"prop_set_string_same_len",
		"prop_set_string_diff_len",
		"prop_set_string_empty",
		"prop_set_string_long_expand",
		"prop_set_string_shrink",
		"prop_set_enum",
		"prop_set_enum_anchor",
		"prop_set_vector_axis_x",
		"prop_set_rotator_axis_roll",
		"prop_set_array_replace_longer",
		"prop_set_array_replace_empty",
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

			present := map[string]bool{}
			dirs := listOperationSpecDirs(entries, operationsDir)

			type runResult struct {
				ByteDelta int `json:"byteDelta"`
			}

			var zeroCount, positiveCount, negativeCount int
			for _, opDir := range dirs {
				specPath := filepath.Join(opDir, "operation.json")
				beforePath := filepath.Join(opDir, "before.uasset")

				specBytes, err := os.ReadFile(specPath)
				if err != nil {
					t.Fatalf("read operation spec: %v", err)
				}
				var spec operationSpec
				if err := json.Unmarshal(specBytes, &spec); err != nil {
					t.Fatalf("parse operation spec: %v", err)
				}
				if strings.TrimSpace(spec.Command) != "prop set" {
					continue
				}
				if strings.TrimSpace(spec.Expect) != "byte_equal" {
					continue
				}

				opName := filepath.Base(opDir)
				present[opName] = true

				beforeBytes, err := os.ReadFile(beforePath)
				if err != nil {
					t.Fatalf("read before fixture: %v", err)
				}
				tempDir := t.TempDir()
				target := filepath.Join(tempDir, "work.uasset")
				if err := os.WriteFile(target, beforeBytes, 0o644); err != nil {
					t.Fatalf("write temp fixture: %v", err)
				}

				argv, err := buildOperationArgv(spec, target)
				if err != nil {
					t.Fatalf("build operation argv: %v", err)
				}

				var stdout bytes.Buffer
				var stderr bytes.Buffer
				code := Run(argv, &stdout, &stderr)
				if code != 0 {
					t.Fatalf("operation command failed (code=%d): op=%s argv=%v stderr=%s", code, opName, argv, stderr.String())
				}

				var result runResult
				if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
					t.Fatalf("parse operation output JSON for %s: %v", opName, err)
				}
				switch {
				case result.ByteDelta == 0:
					zeroCount++
				case result.ByteDelta > 0:
					positiveCount++
				default:
					negativeCount++
				}
			}

			for _, name := range required {
				if !present[name] {
					t.Fatalf("required diversity fixture is missing: %s", name)
				}
			}

			if zeroCount < 8 {
				t.Fatalf("insufficient fixed-length coverage: zero byteDelta ops=%d", zeroCount)
			}
			if positiveCount < 4 {
				t.Fatalf("insufficient growing variable-length coverage: positive byteDelta ops=%d", positiveCount)
			}
			if negativeCount < 3 {
				t.Fatalf("insufficient shrinking variable-length coverage: negative byteDelta ops=%d", negativeCount)
			}
		})
	}
}
