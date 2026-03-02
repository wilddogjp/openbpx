package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestDataTableReadDefaultReturnsAllRows(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "golden", "operations", "dt_add_row", "before.uasset")
	table := runDataTableReadTable(t, fixturePath)

	if got := strings.TrimSpace(anyToString(table["className"])); got != "DataTable" {
		t.Fatalf("expected DataTable class, got %q", got)
	}
	if got := readRowCount(t, table); got != 3 {
		t.Fatalf("expected 3 rows, got %d", got)
	}
	if got := readRowNames(t, table); strings.Join(got, ",") != "Row_A,Row_B,Row_C" {
		t.Fatalf("unexpected row order: %v", got)
	}
}

func TestDataTableReadRowFiltering(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "golden", "operations", "dt_add_row", "before.uasset")

	t.Run("single row", func(t *testing.T) {
		table := runDataTableReadTable(t, fixturePath, "--row", "Row_B")
		if got := readRowCount(t, table); got != 1 {
			t.Fatalf("expected 1 row, got %d", got)
		}
		if got := readRowNames(t, table); strings.Join(got, ",") != "Row_B" {
			t.Fatalf("unexpected rows: %v", got)
		}
	})

	t.Run("multiple rows", func(t *testing.T) {
		table := runDataTableReadTable(t, fixturePath, "--row", "Row_C", "--row", "Row_A")
		if got := readRowCount(t, table); got != 2 {
			t.Fatalf("expected 2 rows, got %d", got)
		}
		if got := readRowNames(t, table); strings.Join(got, ",") != "Row_A,Row_C" {
			t.Fatalf("unexpected rows: %v", got)
		}
	})

	t.Run("comma-separated with duplicates", func(t *testing.T) {
		table := runDataTableReadTable(t, fixturePath, "--row", "Row_C, Row_A ,Row_C")
		if got := readRowCount(t, table); got != 2 {
			t.Fatalf("expected 2 rows, got %d", got)
		}
		if got := readRowNames(t, table); strings.Join(got, ",") != "Row_A,Row_C" {
			t.Fatalf("unexpected rows: %v", got)
		}
	})

	t.Run("missing row", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run([]string{"datatable", "read", fixturePath, "--row", "Row_Missing"}, &stdout, &stderr)
		if code == 0 {
			t.Fatalf("expected read failure for missing row")
		}
		if !strings.Contains(stderr.String(), "datatable rows not found: Row_Missing") {
			t.Fatalf("unexpected error: %s", stderr.String())
		}
	})
}

func TestDataTableCommandUsageValidation(t *testing.T) {
	t.Run("no subcommand", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run([]string{"datatable"}, &stdout, &stderr)
		if code == 0 {
			t.Fatalf("expected datatable usage failure")
		}
		if !strings.Contains(stderr.String(), "usage: bpx datatable <read|update-row|add-row|remove-row> ...") {
			t.Fatalf("unexpected usage error: %s", stderr.String())
		}
	})

	t.Run("read requires file path", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run([]string{"datatable", "read"}, &stdout, &stderr)
		if code == 0 {
			t.Fatalf("expected read usage failure")
		}
		if !strings.Contains(stderr.String(), "usage: bpx datatable read <file.uasset>") {
			t.Fatalf("unexpected read usage error: %s", stderr.String())
		}
	})

	t.Run("unknown subcommand", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run([]string{"datatable", "invalid"}, &stdout, &stderr)
		if code == 0 {
			t.Fatalf("expected unknown subcommand failure")
		}
		if !strings.Contains(stderr.String(), "unknown datatable command: invalid") {
			t.Fatalf("unexpected unknown command error: %s", stderr.String())
		}
	})
}

func TestDataTableReadFormatOutput(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "golden", "operations", "dt_add_row", "before.uasset")

	t.Run("csv stdout", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run([]string{"datatable", "read", fixturePath, "--format", "csv", "--row", "Row_B"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("datatable read failed: %s", stderr.String())
		}
		out := stdout.String()
		if !strings.Contains(out, "file,tableIndex,tableName,rowIndex,rowName,propertyName,propertyType,value") {
			t.Fatalf("csv header not found: %s", out)
		}
		if !strings.Contains(out, ",Row_B,") {
			t.Fatalf("filtered row not found in csv output: %s", out)
		}
		if strings.Contains(out, ",Row_A,") || strings.Contains(out, ",Row_C,") {
			t.Fatalf("unexpected rows in csv output: %s", out)
		}
	})

	t.Run("tsv out file", func(t *testing.T) {
		outPath := filepath.Join(t.TempDir(), "rows.tsv")
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run([]string{"datatable", "read", fixturePath, "--format", "tsv", "--row", "Row_C", "--out", outPath}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("datatable read failed: %s", stderr.String())
		}
		var ack map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &ack); err != nil {
			t.Fatalf("parse read ack json: %v", err)
		}
		if anyToString(ack["format"]) != "tsv" {
			t.Fatalf("unexpected ack format: %v", ack["format"])
		}
		if anyToString(ack["out"]) != outPath {
			t.Fatalf("unexpected ack out path: %v", ack["out"])
		}

		body, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("read out file: %v", err)
		}
		text := string(body)
		if !strings.Contains(text, "\t") {
			t.Fatalf("tsv output does not contain tab delimiter: %s", text)
		}
		if !strings.Contains(text, "\tRow_C\t") {
			t.Fatalf("filtered row not found in tsv output: %s", text)
		}
		if strings.Contains(text, "\tRow_A\t") || strings.Contains(text, "\tRow_B\t") {
			t.Fatalf("unexpected rows in tsv output: %s", text)
		}
	})

	t.Run("toml out file", func(t *testing.T) {
		outPath := filepath.Join(t.TempDir(), "rows.toml")
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run([]string{"datatable", "read", fixturePath, "--format", "toml", "--row", "Row_B", "--out", outPath}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("datatable read failed: %s", stderr.String())
		}
		var ack map[string]any
		if err := toml.Unmarshal(stdout.Bytes(), &ack); err != nil {
			t.Fatalf("parse read ack toml: %v", err)
		}
		if anyToString(ack["format"]) != "toml" {
			t.Fatalf("unexpected ack format: %v", ack["format"])
		}
		if anyToString(ack["out"]) != outPath {
			t.Fatalf("unexpected ack out path: %v", ack["out"])
		}

		body, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("read out file: %v", err)
		}
		var payload map[string]any
		if err := toml.Unmarshal(body, &payload); err != nil {
			t.Fatalf("parse toml output: %v", err)
		}
		tables, ok := payload["tables"].([]any)
		if !ok || len(tables) != 1 {
			t.Fatalf("unexpected tables payload: %#v", payload["tables"])
		}
		table, ok := tables[0].(map[string]any)
		if !ok {
			t.Fatalf("unexpected table payload type: %T", tables[0])
		}
		rows, ok := table["rows"].([]any)
		if !ok || len(rows) != 1 {
			t.Fatalf("unexpected row payload: %#v", table["rows"])
		}
		row, ok := rows[0].(map[string]any)
		if !ok {
			t.Fatalf("unexpected row type: %T", rows[0])
		}
		if got := anyToString(row["rowName"]); got != "Row_B" {
			t.Fatalf("unexpected filtered row: %q", got)
		}
	})
}

func TestDataTableDecodeSubcommandIsRejected(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "golden", "operations", "dt_add_row", "before.uasset")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"datatable", "decode", fixturePath}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected decode subcommand to be rejected")
	}
	if !strings.Contains(stderr.String(), "unknown datatable command: decode") {
		t.Fatalf("unexpected decode error: %s", stderr.String())
	}
}

func runDataTableReadTable(t *testing.T, fixturePath string, extraArgs ...string) map[string]any {
	t.Helper()

	argv := []string{"datatable", "read", fixturePath}
	argv = append(argv, extraArgs...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(argv, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("datatable read failed: %s", stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal datatable read output: %v", err)
	}
	return firstTable(t, payload)
}

func firstTable(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()

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

func readRowCount(t *testing.T, table map[string]any) int {
	t.Helper()
	count, ok := table["rowCount"].(float64)
	if !ok {
		t.Fatalf("rowCount missing or invalid type: %T", table["rowCount"])
	}
	return int(count)
}

func readRowNames(t *testing.T, table map[string]any) []string {
	t.Helper()
	rowsRaw, ok := table["rows"].([]any)
	if !ok {
		t.Fatalf("rows missing or invalid type: %T", table["rows"])
	}
	out := make([]string, 0, len(rowsRaw))
	for _, raw := range rowsRaw {
		row, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("row shape mismatch: %T", raw)
		}
		out = append(out, anyToString(row["rowName"]))
	}
	return out
}
