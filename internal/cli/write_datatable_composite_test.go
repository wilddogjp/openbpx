package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDataTableWriteCommandsRejectCompositeDataTable(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "golden", "operations", "dt_add_row_composite_reject", "before.uasset")
	originalBytes, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read composite fixture: %v", err)
	}

	tests := []struct {
		name string
		argv []string
	}{
		{
			name: "add-row",
			argv: []string{"datatable", "add-row", "--row", "Row_A_1"},
		},
		{
			name: "remove-row",
			argv: []string{"datatable", "remove-row", "--row", "Row_A"},
		},
		{
			name: "update-row",
			argv: []string{"datatable", "update-row", "--row", "Row_A", "--values", `{"Score":999}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			workPath := filepath.Join(tempDir, "work.uasset")
			if err := os.WriteFile(workPath, originalBytes, 0o644); err != nil {
				t.Fatalf("write temp fixture: %v", err)
			}

			argv := make([]string, 0, len(tt.argv)+2)
			argv = append(argv, tt.argv[0], tt.argv[1], workPath)
			argv = append(argv, tt.argv[2:]...)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(argv, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("expected command failure for composite datatable write: argv=%v stdout=%s", argv, stdout.String())
			}

			errText := stderr.String()
			hasCompositeHint := strings.Contains(errText, "CompositeDataTable") || strings.Contains(errText, "writable DataTable export not found")
			if !hasCompositeHint || !strings.Contains(errText, "DataTable only") {
				t.Fatalf("unexpected error text: %s", errText)
			}

			afterBytes, err := os.ReadFile(workPath)
			if err != nil {
				t.Fatalf("read work file after failure: %v", err)
			}
			if !bytes.Equal(originalBytes, afterBytes) {
				t.Fatalf("composite write failure must not modify bytes")
			}
		})
	}
}
