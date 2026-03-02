package cli

import (
	"bytes"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

func TestDataTableRowFilterFlagSetAndNormalized(t *testing.T) {
	var f dataTableRowFilterFlag
	inputs := []string{
		" Row_C , Row_A ,, Row_C ",
		"Row_B",
	}
	for _, in := range inputs {
		if err := f.Set(in); err != nil {
			t.Fatalf("Set(%q) returned error: %v", in, err)
		}
	}

	if got := f.String(); got != "Row_C,Row_A,Row_C,Row_B" {
		t.Fatalf("String() = %q", got)
	}
	if got := strings.Join(f.Normalized(), ","); got != "Row_A,Row_B,Row_C" {
		t.Fatalf("Normalized() = %q", got)
	}

	var empty dataTableRowFilterFlag
	if empty.Normalized() != nil {
		t.Fatalf("expected nil normalized slice for empty filter")
	}
}

func TestFilterDataTableRows(t *testing.T) {
	rows := []map[string]any{
		{"rowName": "Row_A", "rowIndex": 0},
		{"rowName": "Row_B", "rowIndex": 1},
		{"rowName": "Row_C", "rowIndex": 2},
	}
	filterSet := map[string]struct{}{
		"Row_C": {},
		"Row_A": {},
	}
	matched := map[string]bool{}

	filtered := filterDataTableRows(rows, filterSet, matched)
	if len(filtered) != 2 {
		t.Fatalf("filtered row count = %d, want 2", len(filtered))
	}
	if got := strings.Join(rowNames(filtered), ","); got != "Row_A,Row_C" {
		t.Fatalf("filtered order = %q", got)
	}
	if !matched["Row_A"] || !matched["Row_C"] {
		t.Fatalf("matched map not updated correctly: %v", matched)
	}
	if matched["Row_B"] {
		t.Fatalf("unexpected matched entry for Row_B")
	}
}

func TestDecodeObjectIndexVariants(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want int32
		ok   bool
	}{
		{name: "int32", in: int32(-3), want: -3, ok: true},
		{name: "int", in: 7, want: 7, ok: true},
		{name: "int64", in: int64(9), want: 9, ok: true},
		{name: "float64", in: float64(-5), want: -5, ok: true},
		{
			name: "nested map",
			in: map[string]any{
				"value": map[string]any{
					"index": float64(11),
				},
			},
			want: 11,
			ok:   true,
		},
		{name: "unsupported type", in: "11", want: 0, ok: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := decodeObjectIndex(tc.in)
			if ok != tc.ok {
				t.Fatalf("decodeObjectIndex(%#v) ok=%v want %v", tc.in, ok, tc.ok)
			}
			if got != tc.want {
				t.Fatalf("decodeObjectIndex(%#v) = %d want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestMarshalDataTableFlatRows(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		payload := sampleDataTablePayload()
		body, err := marshalDataTableFlatRows(payload, ',')
		if err != nil {
			t.Fatalf("marshalDataTableFlatRows failed: %v", err)
		}

		records, err := csv.NewReader(strings.NewReader(body)).ReadAll()
		if err != nil {
			t.Fatalf("read csv output: %v", err)
		}
		if len(records) != 3 {
			t.Fatalf("record count = %d, want 3", len(records))
		}
		if got := strings.Join(records[0], ","); got != "file,tableIndex,tableName,rowIndex,rowName,propertyName,propertyType,value" {
			t.Fatalf("unexpected header: %q", got)
		}
		if got := strings.Join(records[1], ","); got != "fixture.uasset,1,DT_Main,0,Row_A,Score,IntProperty,99" {
			t.Fatalf("unexpected first row: %q", got)
		}
		if got := records[2][7]; got != `{"rank":"S"}` {
			t.Fatalf("unexpected structured value field: %q", got)
		}
	})

	t.Run("missing tables", func(t *testing.T) {
		if _, err := marshalDataTableFlatRows(map[string]any{}, ','); err == nil {
			t.Fatalf("expected error for payload without tables")
		}
	})

	t.Run("tables type mismatch", func(t *testing.T) {
		if _, err := marshalDataTableFlatRows(map[string]any{"tables": []any{}}, ','); err == nil {
			t.Fatalf("expected type mismatch error")
		}
	})
}

func TestEmitDataTableReadOutput(t *testing.T) {
	t.Run("json out file", func(t *testing.T) {
		outPath := filepath.Join(t.TempDir(), "rows.json")
		payload := sampleDataTablePayload()
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := emitDataTableReadOutput(&stdout, &stderr, "fixture.uasset", payload, "json", outPath)
		if code != 0 {
			t.Fatalf("emitDataTableReadOutput failed: %s", stderr.String())
		}

		var ack map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &ack); err != nil {
			t.Fatalf("unmarshal ack: %v", err)
		}
		if anyToString(ack["format"]) != "json" {
			t.Fatalf("unexpected ack format: %v", ack["format"])
		}
		if anyToString(ack["out"]) != outPath {
			t.Fatalf("unexpected ack out path: %v", ack["out"])
		}

		body, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("read output file: %v", err)
		}
		if !strings.HasSuffix(string(body), "\n") {
			t.Fatalf("json output should end with newline")
		}
	})

	t.Run("csv out file", func(t *testing.T) {
		outPath := filepath.Join(t.TempDir(), "rows.csv")
		payload := sampleDataTablePayload()
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := emitDataTableReadOutput(&stdout, &stderr, "fixture.uasset", payload, "csv", outPath)
		if code != 0 {
			t.Fatalf("emitDataTableReadOutput failed: %s", stderr.String())
		}

		body, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("read output file: %v", err)
		}
		text := string(body)
		if !strings.Contains(text, "file,tableIndex,tableName,rowIndex,rowName,propertyName,propertyType,value") {
			t.Fatalf("csv header missing: %s", text)
		}
		if !strings.Contains(text, ",Row_A,") {
			t.Fatalf("row payload missing: %s", text)
		}
	})

	t.Run("unsupported format", func(t *testing.T) {
		payload := sampleDataTablePayload()
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := emitDataTableReadOutput(&stdout, &stderr, "fixture.uasset", payload, "xml", "")
		if code == 0 {
			t.Fatalf("expected unsupported format error")
		}
		if !strings.Contains(stderr.String(), "unsupported read format: xml") {
			t.Fatalf("unexpected error: %s", stderr.String())
		}
	})

	t.Run("json marshal error", func(t *testing.T) {
		payload := map[string]any{
			"bad": make(chan int),
		}
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := emitDataTableReadOutput(
			&stdout,
			&stderr,
			"fixture.uasset",
			payload,
			"json",
			filepath.Join(t.TempDir(), "rows.json"),
		)
		if code == 0 {
			t.Fatalf("expected marshal error")
		}
		if !strings.Contains(stderr.String(), "marshal json") {
			t.Fatalf("unexpected marshal error: %s", stderr.String())
		}
	})

	t.Run("csv payload type mismatch", func(t *testing.T) {
		payload := map[string]any{
			"file":   "fixture.uasset",
			"tables": []any{},
		}
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := emitDataTableReadOutput(&stdout, &stderr, "fixture.uasset", payload, "csv", "")
		if code == 0 {
			t.Fatalf("expected csv payload type mismatch")
		}
		if !strings.Contains(stderr.String(), "tables payload type mismatch") {
			t.Fatalf("unexpected csv payload error: %s", stderr.String())
		}
	})
}

func TestBuildDataTableReadResponseWithoutDataTables(t *testing.T) {
	asset := &uasset.Asset{
		Names: []uasset.NameEntry{
			{Value: "Blueprint"},
			{Value: "BP_Test"},
		},
		Imports: []uasset.ImportEntry{
			{ObjectName: uasset.NameRef{Index: 0, Number: 0}},
		},
		Exports: []uasset.ExportEntry{
			{
				ClassIndex: uasset.PackageIndex(-1),
				ObjectName: uasset.NameRef{Index: 1, Number: 0},
			},
		},
	}

	resp, err := buildDataTableReadResponse("fixture.uasset", asset, 0, nil)
	if err != nil {
		t.Fatalf("buildDataTableReadResponse failed: %v", err)
	}
	if got, ok := resp["tableCount"].(int); !ok || got != 0 {
		t.Fatalf("tableCount = %v (%T), want 0", resp["tableCount"], resp["tableCount"])
	}

	_, err = buildDataTableReadResponse("fixture.uasset", asset, 0, []string{"Row_B", "Row_A"})
	if err == nil {
		t.Fatalf("expected missing row filter error")
	}
	if !strings.Contains(err.Error(), "datatable rows not found: Row_A, Row_B") {
		t.Fatalf("unexpected missing row error: %v", err)
	}
}

func TestParseDataTableRows(t *testing.T) {
	t.Run("secondary header candidate selected", func(t *testing.T) {
		raw := make([]byte, 0, 24)
		raw = appendInt32LEDT(raw, 0) // primary candidate
		raw = appendInt32LEDT(raw, 1) // secondary candidate
		raw = appendNameRefLEDT(raw, 1, 0)
		raw = appendNameRefLEDT(raw, 0, 0) // NAME_None terminator

		asset := makeSyntheticDataTableAsset(raw, 0, int64(len(raw)))
		rows, warnings := parseDataTableRows(asset, 0, uasset.PropertyListResult{EndOffset: 0})
		if len(rows) != 1 {
			t.Fatalf("row count = %d, want 1", len(rows))
		}
		if got := anyToString(rows[0]["rowName"]); got != "Row_A" {
			t.Fatalf("row name = %q", got)
		}
		if len(warnings) == 0 || !strings.Contains(warnings[0], "selected from secondary header") {
			t.Fatalf("expected secondary-header warning, got %v", warnings)
		}
	})

	t.Run("row section start out of range", func(t *testing.T) {
		asset := makeSyntheticDataTableAsset(make([]byte, 16), 8, 4)
		rows, warnings := parseDataTableRows(asset, 0, uasset.PropertyListResult{EndOffset: 0})
		if len(rows) != 0 {
			t.Fatalf("expected no rows, got %d", len(rows))
		}
		if len(warnings) == 0 || !strings.Contains(warnings[0], "row section start out of range") {
			t.Fatalf("unexpected warnings: %v", warnings)
		}
	})

	t.Run("row count read unexpected EOF", func(t *testing.T) {
		asset := makeSyntheticDataTableAsset(make([]byte, 3), 0, 3)
		rows, warnings := parseDataTableRows(asset, 0, uasset.PropertyListResult{EndOffset: 0})
		if len(rows) != 0 {
			t.Fatalf("expected no rows, got %d", len(rows))
		}
		if len(warnings) == 0 || !strings.Contains(warnings[0], "read row count: unexpected EOF") {
			t.Fatalf("unexpected warnings: %v", warnings)
		}
	})

	t.Run("invalid row count candidate", func(t *testing.T) {
		raw := appendInt32LEDT(nil, -3)
		asset := makeSyntheticDataTableAsset(raw, 0, int64(len(raw)))
		rows, warnings := parseDataTableRows(asset, 0, uasset.PropertyListResult{EndOffset: 0})
		if len(rows) != 0 {
			t.Fatalf("expected no rows, got %d", len(rows))
		}
		if len(warnings) == 0 || !strings.Contains(warnings[0], "no valid datatable row count candidate") {
			t.Fatalf("unexpected warnings: %v", warnings)
		}
	})
}

func TestParseDataTableRowsWithCountStartOffsetOutOfBounds(t *testing.T) {
	asset := makeSyntheticDataTableAsset(make([]byte, 4), 0, 4)
	result := parseDataTableRowsWithCount(asset, 0, 4, 8, 1)
	if result.complete {
		t.Fatalf("expected incomplete parse result")
	}
	if len(result.rows) != 0 {
		t.Fatalf("expected no rows, got %d", len(result.rows))
	}
	if len(result.warnings) == 0 || !strings.Contains(result.warnings[0], "start offset out of bounds") {
		t.Fatalf("unexpected warnings: %v", result.warnings)
	}
}

func sampleDataTablePayload() map[string]any {
	return map[string]any{
		"file": "fixture.uasset",
		"tables": []map[string]any{
			{
				"index":      1,
				"objectName": "DT_Main",
				"rows": []map[string]any{
					{
						"rowIndex": 0,
						"rowName":  "Row_A",
						"properties": []map[string]any{
							{"name": "Score", "type": "IntProperty", "value": 99},
							{"name": "Meta", "type": "StructProperty", "value": map[string]any{"rank": "S"}},
						},
					},
				},
			},
		},
	}
}

func makeSyntheticDataTableAsset(raw []byte, serialOffset, serialSize int64) *uasset.Asset {
	return &uasset.Asset{
		Raw: uasset.RawAsset{Bytes: raw},
		Summary: uasset.PackageSummary{
			FileVersionUE5: 1017,
		},
		Names: []uasset.NameEntry{
			{Value: "None"},
			{Value: "Row_A"},
		},
		Exports: []uasset.ExportEntry{
			{
				SerialOffset: serialOffset,
				SerialSize:   serialSize,
			},
		},
	}
}

func appendInt32LEDT(dst []byte, v int32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(v))
	return append(dst, buf...)
}

func appendNameRefLEDT(dst []byte, index, number int32) []byte {
	dst = appendInt32LEDT(dst, index)
	dst = appendInt32LEDT(dst, number)
	return dst
}

func rowNames(rows []map[string]any) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, anyToString(row["rowName"]))
	}
	return out
}
