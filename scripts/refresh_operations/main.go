package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/wilddogjp/bpx/internal/cli"
)

type operationDef struct {
	Name      string
	Command   string
	Args      map[string]any
	Expect    string
	Notes     string
	Procedure string
}

type operationSpecFile struct {
	Command       string           `json:"command"`
	Args          map[string]any   `json:"args"`
	UEProcedure   string           `json:"ue_procedure"`
	Expect        string           `json:"expect"`
	Notes         string           `json:"notes"`
	IgnoreOffsets []map[string]any `json:"ignore_offsets,omitempty"`
}

const packageFileTag = uint32(0x9E2A83C1)

func main() {
	repoRoot, err := os.Getwd()
	if err != nil {
		die("getwd", err)
	}
	operationsDir := filepath.Join(repoRoot, "testdata", "golden", "operations")
	if err := os.MkdirAll(operationsDir, 0o755); err != nil {
		die("mkdir operations dir", err)
	}

	defs := buildOperationDefs()
	sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })

	// Prepare deterministic byte-equal fixture pairs for supported write commands.
	propBefore := buildCLIFixture("hello")
	propAfter, err := runInPlaceCLI(
		propBefore,
		[]string{"prop", "set", "{file}", "--export", "1", "--path", "MyStr", "--value", `"changed"`},
	)
	if err != nil {
		die("build prop_set_int pair", err)
	}
	varBefore := buildCLIFixture("hello")
	varAfter, err := runInPlaceCLI(
		varBefore,
		[]string{"var", "set-default", "{file}", "--name", "MyStr", "--value", `"changed"`},
	)
	if err != nil {
		die("build var_set_default_int pair", err)
	}

	templateBefore := propBefore
	templateAfter := propAfter

	for _, def := range defs {
		dir := filepath.Join(operationsDir, def.Name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			die("mkdir operation dir", err)
		}

		beforePath := filepath.Join(dir, "before.uasset")
		afterPath := filepath.Join(dir, "after.uasset")
		specPath := filepath.Join(dir, "operation.json")
		readmePath := filepath.Join(dir, "README.md")

		if def.Name == "prop_set_int" {
			mustWriteFile(beforePath, propBefore)
			mustWriteFile(afterPath, propAfter)
		} else if def.Name == "var_set_default_int" {
			mustWriteFile(beforePath, varBefore)
			mustWriteFile(afterPath, varAfter)
		} else {
			if _, err := os.Stat(beforePath); err != nil {
				mustWriteFile(beforePath, templateBefore)
			}
			if _, err := os.Stat(afterPath); err != nil {
				mustWriteFile(afterPath, templateAfter)
			}
		}

		spec := operationSpecFile{
			Command:     def.Command,
			Args:        def.Args,
			UEProcedure: def.Procedure,
			Expect:      def.Expect,
			Notes:       def.Notes,
		}
		body, err := json.MarshalIndent(spec, "", "  ")
		if err != nil {
			die("marshal operation.json", err)
		}
		body = append(body, '\n')
		mustWriteFile(specPath, body)

		readme := fmt.Sprintf("# %s\n\nThis operation fixture pair is managed in-repo.\n\n- command: `%s`\n- expect: `%s`\n- output: `before.uasset`, `after.uasset`, `operation.json`\n", def.Name, def.Command, def.Expect)
		mustWriteFile(readmePath, []byte(readme))
	}
}

func buildOperationDefs() []operationDef {
	defs := []operationDef{
		{Name: "prop_add", Command: "prop add", Args: map[string]any{"export": 5, "spec": `{"name":"bCanBeDamaged","type":"BoolProperty","value":false}`}, Expect: "byte_equal", Notes: "validated by operation-equivalence test", Procedure: "add bCanBeDamaged override tag"},
		{Name: "prop_add_fixture_int", Command: "prop add", Args: map[string]any{"export": 5, "spec": `{"name":"FixtureInt","type":"IntProperty","value":42}`}, Expect: "byte_equal", Notes: "validated by operation-equivalence test", Procedure: "add FixtureInt override tag"},
		{Name: "prop_remove", Command: "prop remove", Args: map[string]any{"export": 5, "path": "bCanBeDamaged"}, Expect: "byte_equal", Notes: "validated by operation-equivalence test", Procedure: "remove bCanBeDamaged override tag"},
		{Name: "prop_remove_fixture_int", Command: "prop remove", Args: map[string]any{"export": 5, "path": "FixtureInt"}, Expect: "byte_equal", Notes: "validated by operation-equivalence test", Procedure: "remove FixtureInt override tag"},

		// Prop set fixtures (current repository fixtures are not yet all writable by bpx; keep explicit unsupported expectations except validated smoke pair).
		{Name: "prop_set_array_element", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureValue", "value": "1"}, Expect: "unsupported", Notes: "fixture migration pending for array element path", Procedure: "prepare writable array fixture and compare against UE output"},
		{Name: "prop_set_bool", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureValue", "value": "true"}, Expect: "unsupported", Notes: "fixture migration pending for bool path", Procedure: "prepare writable bool fixture and compare against UE output"},
		{Name: "prop_set_color", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureColor", "value": `{"R":1,"G":0,"B":0,"A":1}`}, Expect: "unsupported", Notes: "fixture migration pending for struct path", Procedure: "prepare writable color fixture and compare against UE output"},
		{Name: "prop_set_double", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureValue", "value": "2.718281828"}, Expect: "unsupported", Notes: "fixture migration pending for double path", Procedure: "prepare writable double fixture and compare against UE output"},
		{Name: "prop_set_enum", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureEnum", "value": `"ValueA"`}, Expect: "unsupported", Notes: "fixture migration pending for enum path", Procedure: "prepare writable enum fixture and compare against UE output"},
		{Name: "prop_set_enum_numeric", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureEnum", "value": "1"}, Expect: "unsupported", Notes: "fixture migration pending for enum numeric path", Procedure: "prepare writable enum fixture and compare against UE output"},
		{Name: "prop_set_enum_anchor", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureEnumAnchor", "value": `"ValueA"`}, Expect: "unsupported", Notes: "fixture migration pending for enum anchor path", Procedure: "prepare writable enum fixture and compare against UE output"},
		{Name: "prop_set_float", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureValue", "value": "3.14"}, Expect: "unsupported", Notes: "fixture migration pending for float path", Procedure: "prepare writable float fixture and compare against UE output"},
		{Name: "prop_set_float_special", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureValue", "value": "1e-38"}, Expect: "unsupported", Notes: "fixture migration pending for float special path", Procedure: "prepare writable float fixture and compare against UE output"},
		{Name: "prop_set_int", Command: "prop set", Args: map[string]any{"export": 1, "path": "MyStr", "value": `"changed"`}, Expect: "byte_equal", Notes: "validated by operation-equivalence test", Procedure: "update CDO string property through prop set"},
		{Name: "prop_set_int64", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureValue", "value": "9223372036854775807"}, Expect: "unsupported", Notes: "fixture migration pending for int64 path", Procedure: "prepare writable int64 fixture and compare against UE output"},
		{Name: "prop_set_int_max", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureValue", "value": "2147483647"}, Expect: "unsupported", Notes: "fixture migration pending for int max path", Procedure: "prepare writable int fixture and compare against UE output"},
		{Name: "prop_set_int_min", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureValue", "value": "-2147483648"}, Expect: "unsupported", Notes: "fixture migration pending for int min path", Procedure: "prepare writable int fixture and compare against UE output"},
		{Name: "prop_set_int_negative", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureValue", "value": "-1"}, Expect: "unsupported", Notes: "fixture migration pending for negative int path", Procedure: "prepare writable int fixture and compare against UE output"},
		{Name: "prop_set_map_value", Command: "prop set", Args: map[string]any{"export": 1, "path": `MyMap["key"]`, "value": "99"}, Expect: "unsupported", Notes: "fixture migration pending for map path", Procedure: "prepare writable map fixture and compare against UE output"},
		{Name: "prop_set_custom_struct_int", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureCustom.IntVal", "value": "42"}, Expect: "unsupported", Notes: "custom struct tagged re-encoding is not implemented yet", Procedure: "update custom struct int field"},
		{Name: "prop_set_custom_struct_enum", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureCustom.EnumVal", "value": `"BPXEnum_ValueB"`}, Expect: "unsupported", Notes: "custom struct tagged re-encoding is not implemented yet", Procedure: "update custom struct enum field"},
		{Name: "prop_set_name", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureName", "value": `"NewName"`}, Expect: "unsupported", Notes: "fixture migration pending for name path", Procedure: "prepare writable name fixture and compare against UE output"},
		{Name: "prop_set_nested_array_struct", Command: "prop set", Args: map[string]any{"export": 1, "path": "InnerArray[0].StrVal", "value": `"new"`}, Expect: "unsupported", Notes: "fixture migration pending for nested array path", Procedure: "prepare writable nested fixture and compare against UE output"},
		{Name: "prop_set_nested_struct", Command: "prop set", Args: map[string]any{"export": 1, "path": "Inner.IntVal", "value": "42"}, Expect: "unsupported", Notes: "fixture migration pending for nested struct path", Procedure: "prepare writable nested fixture and compare against UE output"},
		{Name: "prop_set_rotator", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureRotator", "value": `{"Pitch":45,"Yaw":90,"Roll":180}`}, Expect: "unsupported", Notes: "fixture migration pending for rotator path", Procedure: "prepare writable rotator fixture and compare against UE output"},
		{Name: "prop_set_soft_object", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureSoft", "value": `{"packageName":"/Game/New","assetName":"Asset"}`}, Expect: "unsupported", Notes: "fixture migration pending for soft object path", Procedure: "prepare writable soft object fixture and compare against UE output"},
		{Name: "prop_set_string_diff_len", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureValue", "value": `"Hello World"`}, Expect: "unsupported", Notes: "fixture migration pending for string diff-len path", Procedure: "prepare writable string fixture and compare against UE output"},
		{Name: "prop_set_string_empty", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureValue", "value": `""`}, Expect: "unsupported", Notes: "fixture migration pending for empty-string path", Procedure: "prepare writable string fixture and compare against UE output"},
		{Name: "prop_set_string_same_len", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureValue", "value": `"World"`}, Expect: "unsupported", Notes: "fixture migration pending for same-length string path", Procedure: "prepare writable string fixture and compare against UE output"},
		{Name: "prop_set_string_unicode", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureValue", "value": `"テスト"`}, Expect: "unsupported", Notes: "fixture migration pending for unicode string path", Procedure: "prepare writable unicode string fixture and compare against UE output"},
		{Name: "prop_set_text", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureText", "value": `"Changed"`}, Expect: "unsupported", Notes: "fixture migration pending for text path", Procedure: "prepare writable text fixture and compare against UE output"},
		{Name: "prop_set_transform", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureTransform", "value": `{"Translation":{"X":1,"Y":2,"Z":3}}`}, Expect: "unsupported", Notes: "fixture migration pending for transform path", Procedure: "prepare writable transform fixture and compare against UE output"},
		{Name: "prop_set_vector", Command: "prop set", Args: map[string]any{"export": 1, "path": "FixtureVector", "value": `{"X":1.5,"Y":-2.3,"Z":100.0}`}, Expect: "unsupported", Notes: "fixture migration pending for vector path", Procedure: "prepare writable vector fixture and compare against UE output"},

		{Name: "dt_update_int", Command: "datatable update-row", Args: map[string]any{"row": "Row_A", "values": `{"Score":999}`}, Expect: "unsupported", Notes: "datatable update-row is not implemented yet", Procedure: "update one integer column"},
		{Name: "dt_update_float", Command: "datatable update-row", Args: map[string]any{"row": "Row_B", "values": `{"Rate":0.5}`}, Expect: "unsupported", Notes: "datatable update-row is not implemented yet", Procedure: "update one float column"},
		{Name: "dt_update_string", Command: "datatable update-row", Args: map[string]any{"row": "Row_A", "values": `{"Name":"NewName"}`}, Expect: "unsupported", Notes: "datatable update-row is not implemented yet", Procedure: "update one string column"},
		{Name: "dt_update_multi_field", Command: "datatable update-row", Args: map[string]any{"row": "Row_A", "values": `{"Score":50,"Rate":0.1}`}, Expect: "unsupported", Notes: "datatable update-row is not implemented yet", Procedure: "update multiple columns"},
		{Name: "dt_update_complex", Command: "datatable update-row", Args: map[string]any{"row": "Row_A", "values": `{"Tags":["TagA","TagB"]}`}, Expect: "unsupported", Notes: "datatable update-row is not implemented yet", Procedure: "update complex row value"},
		{Name: "dt_add_row", Command: "datatable add-row", Args: map[string]any{"row": "Row_A_1"}, Expect: "byte_equal", Notes: "validated by operation-equivalence test", Procedure: "add one row with existing name base"},
		{Name: "dt_add_row_values_scalar", Command: "datatable add-row", Args: map[string]any{"row": "Row_A_1", "values": `{"Score":123}`}, Expect: "byte_equal", Notes: "validated by operation-equivalence test", Procedure: "add one row with scalar value update"},
		{Name: "dt_add_row_values_mixed", Command: "datatable add-row", Args: map[string]any{"row": "Row_B_1", "values": `{"Score":7,"Rate":0.25,"Label":"Row_B_added","Mode":"BPXEnum_ValueB"}`}, Expect: "byte_equal", Notes: "validated by operation-equivalence test", Procedure: "add one row with mixed-type value updates"},
		{Name: "dt_remove_row", Command: "datatable remove-row", Args: map[string]any{"row": "Row_A_1"}, Expect: "byte_equal", Notes: "validated by operation-equivalence test", Procedure: "remove one row"},
		{Name: "dt_remove_row_base", Command: "datatable remove-row", Args: map[string]any{"row": "Row_B"}, Expect: "byte_equal", Notes: "validated by operation-equivalence test", Procedure: "remove one base row"},
		{Name: "dt_add_row_composite_reject", Command: "datatable add-row", Args: map[string]any{"row": "Row_A_1"}, Expect: "unsupported", Notes: "composite datatable write is rejected", Procedure: "reject add-row against CompositeDataTable"},
		{Name: "dt_remove_row_composite_reject", Command: "datatable remove-row", Args: map[string]any{"row": "Row_A"}, Expect: "unsupported", Notes: "composite datatable write is rejected", Procedure: "reject remove-row against CompositeDataTable"},
		{Name: "dt_update_row_composite_reject", Command: "datatable update-row", Args: map[string]any{"row": "Row_A", "values": `{"Score":999}`}, Expect: "unsupported", Notes: "composite datatable write is rejected", Procedure: "reject update-row against CompositeDataTable"},

		{Name: "metadata_set_tooltip", Command: "metadata set-root", Args: map[string]any{"export": 1, "key": "ToolTip", "value": "Updated"}, Expect: "unsupported", Notes: "metadata set-root is not implemented yet", Procedure: "set root metadata tooltip"},
		{Name: "metadata_set_category", Command: "metadata set-root", Args: map[string]any{"export": 1, "key": "Category", "value": "Gameplay"}, Expect: "unsupported", Notes: "metadata set-root is not implemented yet", Procedure: "set root metadata category"},
		{Name: "export_set_header", Command: "export set-header", Args: map[string]any{"index": 1, "fields": `{"objectFlags":1}`}, Expect: "unsupported", Notes: "export set-header is not implemented yet", Procedure: "set export header fields"},
		{Name: "package_set_flags", Command: "package set-flags", Args: map[string]any{"flags": "PKG_FilterEditorOnly"}, Expect: "unsupported", Notes: "package set-flags is not implemented yet", Procedure: "set package flags"},

		{Name: "var_set_default_int", Command: "var set-default", Args: map[string]any{"name": "MyStr", "value": `"changed"`}, Expect: "byte_equal", Notes: "validated by operation-equivalence test", Procedure: "update default variable value through var set-default"},
		{Name: "var_set_default_empty", Command: "var set-default", Args: map[string]any{"name": "MyStr", "value": `""`}, Expect: "byte_equal", Notes: "validated by operation-equivalence test", Procedure: "update default variable value to empty string"},
		{Name: "var_set_default_unicode", Command: "var set-default", Args: map[string]any{"name": "MyStr", "value": `"テスト"`}, Expect: "byte_equal", Notes: "validated by operation-equivalence test", Procedure: "update default variable value to unicode string"},
		{Name: "var_set_default_long", Command: "var set-default", Args: map[string]any{"name": "MyStr", "value": `"Lorem ipsum dolor sit amet var-default"`}, Expect: "byte_equal", Notes: "validated by operation-equivalence test", Procedure: "update default variable value to long string"},
		{Name: "var_set_default_string", Command: "var set-default", Args: map[string]any{"name": "FixtureValue", "value": `"NewTitle"`}, Expect: "unsupported", Notes: "fixture migration pending for string default path", Procedure: "update default string variable"},
		{Name: "var_set_default_vector", Command: "var set-default", Args: map[string]any{"name": "FixtureValue", "value": `{"X":1,"Y":2,"Z":3}`}, Expect: "unsupported", Notes: "fixture migration pending for vector default path", Procedure: "update default vector variable"},

		{Name: "var_rename_simple", Command: "var rename", Args: map[string]any{"from": "OldVar", "to": "NewVar"}, Expect: "unsupported", Notes: "var rename is not implemented yet", Procedure: "rename simple variable"},
		{Name: "var_rename_with_refs", Command: "var rename", Args: map[string]any{"from": "UsedVar", "to": "RenamedVar"}, Expect: "unsupported", Notes: "var rename is not implemented yet", Procedure: "rename variable with graph references"},
		{Name: "var_rename_unicode", Command: "var rename", Args: map[string]any{"from": "体力", "to": "HP"}, Expect: "unsupported", Notes: "var rename is not implemented yet", Procedure: "rename unicode variable"},
		{Name: "ref_rewrite_single", Command: "ref rewrite", Args: map[string]any{"from": "/Game/Old/Mesh", "to": "/Game/New/Mesh"}, Expect: "unsupported", Notes: "ref rewrite is not implemented yet", Procedure: "rewrite one soft reference"},
		{Name: "ref_rewrite_multi", Command: "ref rewrite", Args: map[string]any{"from": "/Game/OldDir", "to": "/Game/NewDir"}, Expect: "unsupported", Notes: "ref rewrite is not implemented yet", Procedure: "rewrite references by directory"},
	}
	return defs
}

func runInPlaceCLI(initial []byte, argvTemplate []string) ([]byte, error) {
	tempDir, err := os.MkdirTemp("", "bpx-opgen-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "fixture.uasset")
	if err := os.WriteFile(filePath, initial, 0o644); err != nil {
		return nil, err
	}

	argv := make([]string, 0, len(argvTemplate))
	for _, part := range argvTemplate {
		if part == "{file}" {
			argv = append(argv, filePath)
		} else {
			argv = append(argv, part)
		}
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run(argv, &stdout, &stderr)
	if code != 0 {
		return nil, fmt.Errorf("argv=%v code=%d stderr=%s", argv, code, stderr.String())
	}

	out, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Build a minimal yet writable .uasset fixture containing a CDO String property `MyStr`.
// This mirrors the deterministic fixture used by write command tests.
func buildCLIFixture(strValue string) []byte {
	names := []string{
		"None",
		"Default__BP_Test",
		"ObjectProperty",
		"MyProp",
		"BlueprintGeneratedClass",
		"CoreUObject",
		"StrProperty",
		"MyStr",
	}
	nameMap := buildCLINameMap(names)
	importMap := buildCLIImportMap()

	payload := buildCLIStringPropertyPayload(7, 6, strValue)
	exports := []cliExport{{objectNameIndex: 1, payload: payload}}

	summaryTemplate := buildCLISummary(cliSummaryArgs{
		NameOffset:           0,
		ImportOffset:         0,
		ExportOffset:         0,
		TotalHeaderSize:      0,
		BulkDataStartOffset:  0,
		EngineMinor:          6,
		NameCount:            int32(len(names)),
		ImportCount:          1,
		ExportCount:          1,
		NamesReferencedCount: int32(len(names)),
	})
	exportMapTemplate := buildCLIExportMap(exports, 0)

	summarySize := len(summaryTemplate)
	nameOffset := int32(summarySize)
	importOffset := int32(summarySize + len(nameMap))
	exportOffset := int32(summarySize + len(nameMap) + len(importMap))
	totalHeader := int32(summarySize + len(nameMap) + len(importMap) + len(exportMapTemplate))
	serialBase := int64(totalHeader)
	exportMap := buildCLIExportMap(exports, serialBase)
	bulkStart := serialBase + int64(len(payload))

	summary := buildCLISummary(cliSummaryArgs{
		NameOffset:           nameOffset,
		ImportOffset:         importOffset,
		ExportOffset:         exportOffset,
		TotalHeaderSize:      totalHeader,
		BulkDataStartOffset:  bulkStart,
		EngineMinor:          6,
		NameCount:            int32(len(names)),
		ImportCount:          1,
		ExportCount:          1,
		NamesReferencedCount: int32(len(names)),
	})

	var out bytes.Buffer
	out.Write(summary)
	out.Write(nameMap)
	out.Write(importMap)
	out.Write(exportMap)
	out.Write(payload)
	out.Write([]byte("TAIL"))
	return out.Bytes()
}

type cliExport struct {
	objectNameIndex int32
	payload         []byte
}

type cliSummaryArgs struct {
	NameOffset           int32
	ImportOffset         int32
	ExportOffset         int32
	TotalHeaderSize      int32
	BulkDataStartOffset  int64
	EngineMinor          uint16
	NameCount            int32
	ImportCount          int32
	ExportCount          int32
	NamesReferencedCount int32
}

func buildCLISummary(args cliSummaryArgs) []byte {
	var b bytes.Buffer
	w32 := func(v int32) { must(binary.Write(&b, binary.LittleEndian, v)) }
	wu32 := func(v uint32) { must(binary.Write(&b, binary.LittleEndian, v)) }
	w64 := func(v int64) { must(binary.Write(&b, binary.LittleEndian, v)) }
	wu16 := func(v uint16) { must(binary.Write(&b, binary.LittleEndian, v)) }
	wstr := func(s string) {
		must(binary.Write(&b, binary.LittleEndian, int32(len(s)+1)))
		b.WriteString(s)
		b.WriteByte(0)
	}
	wguid := func() { b.Write(make([]byte, 16)) }
	wengine := func(minor uint16) {
		wu16(5)
		wu16(minor)
		wu16(0)
		wu32(0)
		wstr("test")
	}

	wu32(packageFileTag)
	w32(-9)
	w32(864)
	w32(522)
	w32(1017)
	w32(0)
	b.Write(make([]byte, 20))
	w32(args.TotalHeaderSize)

	w32(1)
	for i := 0; i < 16; i++ {
		b.WriteByte(byte(i + 1))
	}
	w32(1)

	wstr("/Game/TestAsset")
	wu32(0)
	w32(args.NameCount)
	w32(args.NameOffset)
	w32(0)
	w32(0)
	wstr("")
	w32(0)
	w32(0)
	w32(args.ExportCount)
	w32(args.ExportOffset)
	w32(args.ImportCount)
	w32(args.ImportOffset)
	w32(0)
	w32(0)
	w32(0)
	w32(0)
	w32(0)
	w32(0)
	w32(0)
	w32(0)
	w32(0)
	w32(0)
	wguid()
	w32(1)
	w32(args.ExportCount)
	w32(args.NameCount)
	wengine(args.EngineMinor)
	wengine(args.EngineMinor)
	wu32(0)
	w32(0)
	wu32(0)
	w32(0)
	w32(0)
	w64(args.BulkDataStartOffset)
	w32(0)
	w32(0)
	w32(0)
	w32(0)
	w32(args.NamesReferencedCount)
	w64(-1)
	w32(-1)
	return b.Bytes()
}

func buildCLIExportMap(entries []cliExport, serialBase int64) []byte {
	var b bytes.Buffer
	w32 := func(v int32) { must(binary.Write(&b, binary.LittleEndian, v)) }
	wu32 := func(v uint32) { must(binary.Write(&b, binary.LittleEndian, v)) }
	w64 := func(v int64) { must(binary.Write(&b, binary.LittleEndian, v)) }
	wbool := func(v bool) {
		if v {
			wu32(1)
		} else {
			wu32(0)
		}
	}
	wname := func(index, number int32) {
		w32(index)
		w32(number)
	}

	cursor := serialBase
	for _, entry := range entries {
		w32(-1)
		w32(0)
		w32(0)
		w32(0)
		wname(entry.objectNameIndex, 0)
		wu32(0)
		w64(int64(len(entry.payload)))
		w64(cursor)
		wbool(false)
		wbool(false)
		wbool(false)
		wbool(false)
		wu32(0)
		wbool(false)
		wbool(true)
		wbool(false)
		w32(-1)
		w32(0)
		w32(0)
		w32(0)
		w32(0)
		w64(0)
		w64(int64(len(entry.payload)))
		cursor += int64(len(entry.payload))
	}
	return b.Bytes()
}

func buildCLIImportMap() []byte {
	var b bytes.Buffer
	w32 := func(v int32) { must(binary.Write(&b, binary.LittleEndian, v)) }
	wu32 := func(v uint32) { must(binary.Write(&b, binary.LittleEndian, v)) }
	wname := func(index, number int32) {
		w32(index)
		w32(number)
	}
	wname(5, 0)
	wname(4, 0)
	w32(0)
	wname(1, 0)
	wname(0, 0)
	wu32(0)
	return b.Bytes()
}

func buildCLINameMap(names []string) []byte {
	var b bytes.Buffer
	for _, name := range names {
		must(binary.Write(&b, binary.LittleEndian, int32(len(name)+1)))
		b.WriteString(name)
		b.WriteByte(0)
		must(binary.Write(&b, binary.LittleEndian, uint16(0)))
		must(binary.Write(&b, binary.LittleEndian, uint16(0)))
	}
	return b.Bytes()
}

func buildCLIStringPropertyPayload(propNameIndex, typeNameIndex int32, value string) []byte {
	var b bytes.Buffer
	w32 := func(v int32) { must(binary.Write(&b, binary.LittleEndian, v)) }
	w8 := func(v uint8) { must(binary.Write(&b, binary.LittleEndian, v)) }
	wname := func(index, number int32) {
		w32(index)
		w32(number)
	}

	w8(0)
	wname(propNameIndex, 0)
	wname(typeNameIndex, 0)
	w32(0)

	var valueBuf bytes.Buffer
	must(binary.Write(&valueBuf, binary.LittleEndian, int32(len(value)+1)))
	valueBuf.WriteString(value)
	valueBuf.WriteByte(0)

	w32(int32(valueBuf.Len()))
	w8(0)
	b.Write(valueBuf.Bytes())
	wname(0, 0)
	return b.Bytes()
}

func mustWriteFile(path string, data []byte) {
	if err := os.WriteFile(path, data, 0o644); err != nil {
		die("write file", fmt.Errorf("%s: %w", path, err))
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func die(context string, err error) {
	fmt.Fprintf(os.Stderr, "error: %s: %v\n", context, err)
	os.Exit(1)
}
