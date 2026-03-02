package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type operationIgnoreRange struct {
	Offset int    `json:"offset"`
	Length int    `json:"length"`
	Reason string `json:"reason"`
}

type operationSpec struct {
	Command       string                 `json:"command"`
	Args          map[string]any         `json:"args"`
	UEProcedure   string                 `json:"ue_procedure"`
	Expect        string                 `json:"expect"`
	Notes         string                 `json:"notes"`
	IgnoreOffsets []operationIgnoreRange `json:"ignore_offsets"`
}

func TestOperationEquivalence(t *testing.T) {
	operationsDir := filepath.Join("..", "..", "testdata", "golden", "operations")
	entries, err := os.ReadDir(operationsDir)
	if err != nil {
		t.Fatalf("read operations dir: %v", err)
	}

	dirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, filepath.Join(operationsDir, entry.Name()))
		}
	}
	sort.Strings(dirs)
	if len(dirs) == 0 {
		t.Fatalf("no operation fixture directories found")
	}

	byteEqualCount := 0
	for _, opDir := range dirs {
		opDir := opDir
		t.Run(filepath.Base(opDir), func(t *testing.T) {
			specPath := filepath.Join(opDir, "operation.json")
			beforePath := filepath.Join(opDir, "before.uasset")
			afterPath := filepath.Join(opDir, "after.uasset")

			specBytes, err := os.ReadFile(specPath)
			if err != nil {
				t.Fatalf("read operation spec: %v", err)
			}
			var spec operationSpec
			if err := json.Unmarshal(specBytes, &spec); err != nil {
				t.Fatalf("parse operation spec: %v", err)
			}
			if strings.TrimSpace(spec.Command) == "" {
				t.Fatalf("operation command must not be empty")
			}
			if len(spec.Args) == 0 {
				t.Fatalf("operation args must not be empty")
			}

			beforeBytes, err := os.ReadFile(beforePath)
			if err != nil {
				t.Fatalf("read before fixture: %v", err)
			}
			afterBytes, err := os.ReadFile(afterPath)
			if err != nil {
				t.Fatalf("read after fixture: %v", err)
			}

			tempDir := t.TempDir()
			tempFile := filepath.Join(tempDir, "work.uasset")
			if err := os.WriteFile(tempFile, beforeBytes, 0o644); err != nil {
				t.Fatalf("write temp fixture: %v", err)
			}

			argv, err := buildOperationArgv(spec, tempFile)
			if err != nil {
				t.Fatalf("build operation argv: %v", err)
			}

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(argv, &stdout, &stderr)

			expect := strings.TrimSpace(spec.Expect)
			if expect == "" {
				expect = "byte_equal"
			}

			switch expect {
			case "unsupported":
				if code == 0 {
					t.Skipf("operation fixture marked unsupported is now implemented: argv=%v", argv)
				}
				return
			case "byte_equal":
				byteEqualCount++
				if code != 0 {
					t.Fatalf("operation command failed (code=%d): argv=%v stderr=%s", code, argv, stderr.String())
				}
			default:
				t.Fatalf("unsupported expect value: %s", expect)
			}

			actualBytes, err := os.ReadFile(tempFile)
			if err != nil {
				t.Fatalf("read command output bytes: %v", err)
			}

			match, err := equalBytesWithIgnoredOffsets(actualBytes, afterBytes, spec.IgnoreOffsets)
			if err != nil {
				t.Fatalf("compare output bytes: %v", err)
			}
			if !match {
				t.Fatalf("byte mismatch for operation fixture\nargv=%v", argv)
			}
		})
	}

	if byteEqualCount == 0 {
		t.Fatalf("no byte_equal operation fixtures found")
	}
}

func buildOperationArgv(spec operationSpec, targetFile string) ([]string, error) {
	parts := strings.Fields(spec.Command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	argv := append([]string{}, parts...)
	argv = append(argv, targetFile)

	keys := make([]string, 0, len(spec.Args))
	for key := range spec.Args {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value, err := formatOperationArgValue(spec.Args[key])
		if err != nil {
			return nil, fmt.Errorf("format --%s: %w", key, err)
		}
		argv = append(argv, "--"+key, value)
	}
	return argv, nil
}

func formatOperationArgValue(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case bool:
		if x {
			return "true", nil
		}
		return "false", nil
	case float64:
		if x == math.Trunc(x) {
			return strconv.FormatInt(int64(x), 10), nil
		}
		return strconv.FormatFloat(x, 'g', -1, 64), nil
	case nil:
		return "null", nil
	default:
		buf, err := json.Marshal(x)
		if err != nil {
			return "", err
		}
		return string(buf), nil
	}
}

func equalBytesWithIgnoredOffsets(actual, expected []byte, ignored []operationIgnoreRange) (bool, error) {
	if len(actual) != len(expected) {
		return false, nil
	}

	left := append([]byte(nil), actual...)
	right := append([]byte(nil), expected...)
	for _, item := range ignored {
		if item.Offset < 0 || item.Length < 0 {
			return false, fmt.Errorf("invalid ignore range: offset=%d length=%d", item.Offset, item.Length)
		}
		end := item.Offset + item.Length
		if end > len(left) {
			return false, fmt.Errorf("ignore range out of bounds: offset=%d length=%d size=%d", item.Offset, item.Length, len(left))
		}
		for i := item.Offset; i < end; i++ {
			left[i] = 0
			right[i] = 0
		}
	}

	return bytes.Equal(left, right), nil
}
