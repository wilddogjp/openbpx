package cli

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

const packageFileTag = uint32(0x9E2A83C1)

func TestRunPropSetDryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "sample.uasset")
	orig := buildCLIFixture(t, "hello")
	if err := os.WriteFile(assetPath, orig, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"prop", "set", assetPath, "--export", "1", "--path", "MyStr", "--value", "\"changed\"", "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"dryRun": true`) {
		t.Fatalf("dry-run response missing: %s", stdout.String())
	}
	after, err := os.ReadFile(assetPath)
	if err != nil {
		t.Fatalf("read fixture after run: %v", err)
	}
	if !bytes.Equal(orig, after) {
		t.Fatalf("dry-run modified source file")
	}
}

func TestRunPropSetBackupWritesBackupFile(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "sample.uasset")
	orig := buildCLIFixture(t, "hello")
	if err := os.WriteFile(assetPath, orig, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"prop", "set", assetPath, "--export", "1", "--path", "MyStr", "--value", "\"changed\"", "--backup"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%s", code, stderr.String())
	}
	backupPath := assetPath + ".backup"
	backup, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if !bytes.Equal(backup, orig) {
		t.Fatalf("backup bytes mismatch")
	}
	after, err := os.ReadFile(assetPath)
	if err != nil {
		t.Fatalf("read rewritten asset: %v", err)
	}
	if bytes.Equal(after, orig) {
		t.Fatalf("prop set did not modify asset bytes")
	}
}

func TestRunVarSetDefaultDryRunUsesCDOExport(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "sample.uasset")
	orig := buildCLIFixture(t, "hello")
	if err := os.WriteFile(assetPath, orig, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"var", "set-default", assetPath, "--name", "MyStr", "--value", "\"changed\"", "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"export": 1`) {
		t.Fatalf("expected CDO export in response: %s", stdout.String())
	}
}

func TestRunLevelVarListResolvesActorSelectorVariants(t *testing.T) {
	mapPath := filepath.Join("..", "..", "testdata", "golden", "parse", "L_Minimal.umap")
	selectors := []string{
		"LyraWorldSettings",
		"PersistentLevel.LyraWorldSettings",
		"4",
	}
	for _, selector := range selectors {
		t.Run(selector, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run([]string{"level", "var-list", mapPath, "--actor", selector}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit code: got %d stderr=%s", code, stderr.String())
			}

			var payload map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
				t.Fatalf("decode json: %v\nstdout=%s", err, stdout.String())
			}

			actorRaw, ok := payload["actor"].(map[string]any)
			if !ok {
				t.Fatalf("actor payload missing: %#v", payload["actor"])
			}
			if got, want := int(actorRaw["export"].(float64)), 4; got != want {
				t.Fatalf("actor export: got %d want %d", got, want)
			}
			if got, want := actorRaw["objectName"], "LyraWorldSettings"; got != want {
				t.Fatalf("actor objectName: got %v want %v", got, want)
			}
		})
	}
}

func TestRunLevelVarListRejectsAmbiguousActorSelector(t *testing.T) {
	mapPath := filepath.Join("..", "..", "testdata", "golden", "parse", "L_Minimal.umap")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"level", "var-list", mapPath, "--actor", "Polys"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected ambiguous selector to fail")
	}
	if !strings.Contains(stderr.String(), "ambiguous") {
		t.Fatalf("expected ambiguous error, stderr=%s", stderr.String())
	}
}

func TestRunLevelVarSetDryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "L_Minimal.umap")
	orig, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "parse", "L_Minimal.umap"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(assetPath, orig, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"level", "var-set", assetPath, "--actor", "LyraWorldSettings", "--path", "NavigationSystemConfig", "--value", "0", "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"dryRun": true`) {
		t.Fatalf("dry-run response missing: %s", stdout.String())
	}
	after, err := os.ReadFile(assetPath)
	if err != nil {
		t.Fatalf("read fixture after run: %v", err)
	}
	if !bytes.Equal(orig, after) {
		t.Fatalf("dry-run modified source file")
	}
}

func TestRunLevelVarSetBackupWritesBackupFile(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "L_Minimal.umap")
	orig, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "parse", "L_Minimal.umap"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(assetPath, orig, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"level", "var-set", assetPath, "--actor", "LyraWorldSettings", "--path", "NavigationSystemConfig", "--value", "0", "--backup"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%s", code, stderr.String())
	}

	backupPath := assetPath + ".backup"
	backup, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if !bytes.Equal(backup, orig) {
		t.Fatalf("backup bytes mismatch")
	}

	var verifyStdout bytes.Buffer
	var verifyStderr bytes.Buffer
	verifyCode := Run([]string{"level", "var-list", assetPath, "--actor", "LyraWorldSettings"}, &verifyStdout, &verifyStderr)
	if verifyCode != 0 {
		t.Fatalf("verify command failed: code=%d stderr=%s", verifyCode, verifyStderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(verifyStdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode verify json: %v\nstdout=%s", err, verifyStdout.String())
	}
	propsRaw, ok := payload["properties"].([]any)
	if !ok {
		t.Fatalf("properties payload missing: %#v", payload["properties"])
	}
	found := false
	for _, raw := range propsRaw {
		prop, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if prop["name"] != "NavigationSystemConfig" {
			continue
		}
		value, ok := prop["value"].(map[string]any)
		if !ok {
			t.Fatalf("navigation value missing: %#v", prop["value"])
		}
		if got, want := int(value["index"].(float64)), 0; got != want {
			t.Fatalf("navigation index: got %d want %d", got, want)
		}
		found = true
		break
	}
	if !found {
		t.Fatalf("NavigationSystemConfig property not found in level vars output")
	}
}

func TestCollectDeclaredVariablesFallbackParsesRawNewVariables(t *testing.T) {
	asset := buildDeclaredVarsFallbackAsset(t)
	declared, warnings := collectDeclaredVariables(asset)

	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if got := declared["FooVar"]; got != "" {
		t.Fatalf("declared variable type: got %q want empty", got)
	}
	if len(declared) != 1 {
		t.Fatalf("declared variable count: got %d want 1", len(declared))
	}
}

func TestRunPackageSetFlagsUpdatesSummary(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "sample.uasset")
	orig, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "parse", "BP_Empty.uasset"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(assetPath, orig, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"package", "set-flags", assetPath, "--flags", "PKG_ContainsMap|PKG_RuntimeGenerated"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(assetPath, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten fixture: %v", err)
	}
	if got, want := asset.Summary.PackageFlags, uint32(0x20020000); got != want {
		t.Fatalf("package flags: got 0x%08x want 0x%08x", got, want)
	}
}

func TestRunPackageSetFlagsMasksTransientAndInMemoryFlags(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "sample.uasset")
	orig, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "parse", "BP_Empty.uasset"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(assetPath, orig, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"package", "set-flags", assetPath, "--flags", "PKG_CompiledIn|PKG_ContainsMap|PKG_IsSaving"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(assetPath, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten fixture: %v", err)
	}
	if got, want := asset.Summary.PackageFlags, uint32(0x00020000); got != want {
		t.Fatalf("package flags: got 0x%08x want 0x%08x", got, want)
	}
}

func TestParsePackageFlagsValueUE56Names(t *testing.T) {
	tests := []struct {
		raw  string
		want uint32
	}{
		{raw: "PKG_ContainsMap", want: 0x00020000},
		{raw: "PKG_RequiresLocalizationGather", want: 0x00040000},
		{raw: "PKG_RuntimeGenerated", want: 0x20000000},
		{raw: "containsscript|dynamicimports", want: 0x10200000},
		{raw: "PKG_ContainsNoAsset", want: 0x00000400},
	}
	for _, tt := range tests {
		got, err := parsePackageFlagsValue(tt.raw)
		if err != nil {
			t.Fatalf("parsePackageFlagsValue(%q): %v", tt.raw, err)
		}
		if got != tt.want {
			t.Fatalf("parsePackageFlagsValue(%q): got 0x%08x want 0x%08x", tt.raw, got, tt.want)
		}
	}
}

func TestRunPackageSetFlagsRejectsShapeSensitiveBits(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "sample.uasset")
	orig, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "parse", "BP_Empty.uasset"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(assetPath, orig, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"package", "set-flags", assetPath, "--flags", "PKG_FilterEditorOnly"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit for shape-sensitive flag update")
	}
	if !strings.Contains(stderr.String(), "not supported") {
		t.Fatalf("expected unsupported error, got stderr=%s", stderr.String())
	}

	asset, err := uasset.ParseFile(assetPath, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse original fixture: %v", err)
	}
	if got, want := asset.Summary.PackageFlags, uint32(0x00040000); got != want {
		t.Fatalf("package flags changed unexpectedly: got 0x%08x want 0x%08x", got, want)
	}
}

func TestRunExportSetHeaderUpdatesObjectFlags(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "sample.uasset")
	orig, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "parse", "BP_Empty.uasset"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(assetPath, orig, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"export", "set-header", assetPath, "--index", "1", "--fields", `{"objectFlags":1}`}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(assetPath, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten fixture: %v", err)
	}
	if len(asset.Exports) == 0 {
		t.Fatalf("fixture has no exports after rewrite")
	}
	if got, want := asset.Exports[0].ObjectFlags, uint32(1); got != want {
		t.Fatalf("object flags: got 0x%08x want 0x%08x", got, want)
	}
}

type cliExport struct {
	objectNameIndex int32
	payload         []byte
}

func buildCLIFixture(t *testing.T, strValue string) []byte {
	t.Helper()

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
	nameMap := buildCLINameMap(t, names)
	importMap := buildCLIImportMap(t)

	payload := buildCLIStringPropertyPayload(t, 7, 6, strValue)
	exports := []cliExport{{objectNameIndex: 1, payload: payload}}

	summaryTemplate := buildCLISummary(t, cliSummaryArgs{
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
	exportMapTemplate := buildCLIExportMap(t, exports, 0)

	summarySize := len(summaryTemplate)
	nameOffset := int32(summarySize)
	importOffset := int32(summarySize + len(nameMap))
	exportOffset := int32(summarySize + len(nameMap) + len(importMap))
	totalHeader := int32(summarySize + len(nameMap) + len(importMap) + len(exportMapTemplate))
	serialBase := int64(totalHeader)
	exportMap := buildCLIExportMap(t, exports, serialBase)
	bulkStart := serialBase + int64(len(payload))

	summary := buildCLISummary(t, cliSummaryArgs{
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

func buildCLISummary(t *testing.T, args cliSummaryArgs) []byte {
	t.Helper()
	var b bytes.Buffer
	w32 := func(v int32) { mustCLI(binary.Write(&b, binary.LittleEndian, v)) }
	wu32 := func(v uint32) { mustCLI(binary.Write(&b, binary.LittleEndian, v)) }
	w64 := func(v int64) { mustCLI(binary.Write(&b, binary.LittleEndian, v)) }
	wu16 := func(v uint16) { mustCLI(binary.Write(&b, binary.LittleEndian, v)) }
	wstr := func(s string) {
		mustCLI(binary.Write(&b, binary.LittleEndian, int32(len(s)+1)))
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

func buildCLIExportMap(t *testing.T, entries []cliExport, serialBase int64) []byte {
	t.Helper()
	var b bytes.Buffer
	w32 := func(v int32) { mustCLI(binary.Write(&b, binary.LittleEndian, v)) }
	wu32 := func(v uint32) { mustCLI(binary.Write(&b, binary.LittleEndian, v)) }
	w64 := func(v int64) { mustCLI(binary.Write(&b, binary.LittleEndian, v)) }
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

func buildCLIImportMap(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	w32 := func(v int32) { mustCLI(binary.Write(&b, binary.LittleEndian, v)) }
	wu32 := func(v uint32) { mustCLI(binary.Write(&b, binary.LittleEndian, v)) }
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

func buildCLINameMap(t *testing.T, names []string) []byte {
	t.Helper()
	var b bytes.Buffer
	for _, name := range names {
		mustCLI(binary.Write(&b, binary.LittleEndian, int32(len(name)+1)))
		b.WriteString(name)
		b.WriteByte(0)
		mustCLI(binary.Write(&b, binary.LittleEndian, uint16(0)))
		mustCLI(binary.Write(&b, binary.LittleEndian, uint16(0)))
	}
	return b.Bytes()
}

func buildCLIStringPropertyPayload(t *testing.T, propNameIndex, typeNameIndex int32, value string) []byte {
	t.Helper()
	var b bytes.Buffer
	w32 := func(v int32) { mustCLI(binary.Write(&b, binary.LittleEndian, v)) }
	w8 := func(v uint8) { mustCLI(binary.Write(&b, binary.LittleEndian, v)) }
	wname := func(index, number int32) {
		w32(index)
		w32(number)
	}

	w8(0)
	wname(propNameIndex, 0)
	wname(typeNameIndex, 0)
	w32(0)

	var valueBuf bytes.Buffer
	mustCLI(binary.Write(&valueBuf, binary.LittleEndian, int32(len(value)+1)))
	valueBuf.WriteString(value)
	valueBuf.WriteByte(0)

	w32(int32(valueBuf.Len()))
	w8(0)
	b.Write(valueBuf.Bytes())
	wname(0, 0)
	return b.Bytes()
}

func mustCLI(err error) {
	if err != nil {
		panic(err)
	}
}

func buildDeclaredVarsFallbackAsset(t *testing.T) *uasset.Asset {
	t.Helper()

	names := []uasset.NameEntry{
		{Value: "None"},
		{Value: "BP_Test"},
		{Value: "Blueprint"},
		{Value: "NewVariables"},
		{Value: "ArrayProperty"},
		{Value: "StructProperty"},
		{Value: "VarName"},
		{Value: "NameProperty"},
		{Value: "FooVar"},
	}

	var item bytes.Buffer
	writeNameRef := func(buf *bytes.Buffer, index, number int32) {
		mustCLI(binary.Write(buf, binary.LittleEndian, index))
		mustCLI(binary.Write(buf, binary.LittleEndian, number))
	}
	writeNameRef(&item, 6, 0) // VarName
	writeNameRef(&item, 7, 0) // NameProperty
	mustCLI(binary.Write(&item, binary.LittleEndian, int32(0)))
	mustCLI(binary.Write(&item, binary.LittleEndian, int32(8)))
	mustCLI(binary.Write(&item, binary.LittleEndian, uint8(0)))
	writeNameRef(&item, 8, 0) // FooVar
	writeNameRef(&item, 0, 0) // None

	var arrayPayload bytes.Buffer
	mustCLI(binary.Write(&arrayPayload, binary.LittleEndian, int32(1)))
	arrayPayload.Write(item.Bytes())

	var exportPayload bytes.Buffer
	mustCLI(binary.Write(&exportPayload, binary.LittleEndian, uint8(0))) // class serialization control
	writeNameRef(&exportPayload, 3, 0)                                   // NewVariables
	writeNameRef(&exportPayload, 4, 0)                                   // ArrayProperty
	mustCLI(binary.Write(&exportPayload, binary.LittleEndian, int32(1)))
	writeNameRef(&exportPayload, 5, 0) // StructProperty
	mustCLI(binary.Write(&exportPayload, binary.LittleEndian, int32(0)))
	mustCLI(binary.Write(&exportPayload, binary.LittleEndian, int32(arrayPayload.Len())))
	mustCLI(binary.Write(&exportPayload, binary.LittleEndian, uint8(0)))
	exportPayload.Write(arrayPayload.Bytes())
	writeNameRef(&exportPayload, 0, 0) // None terminator for export properties

	raw := exportPayload.Bytes()
	return &uasset.Asset{
		Raw: rawAsset(raw),
		Summary: uasset.PackageSummary{
			FileVersionUE4: 522,
			FileVersionUE5: 1017,
		},
		Names: names,
		Imports: []uasset.ImportEntry{
			{ObjectName: uasset.NameRef{Index: 2, Number: 0}},
		},
		Exports: []uasset.ExportEntry{
			{
				ClassIndex:                     -1,
				ObjectName:                     uasset.NameRef{Index: 1, Number: 0},
				SerialOffset:                   0,
				SerialSize:                     int64(len(raw)),
				ScriptSerializationStartOffset: 0,
				ScriptSerializationEndOffset:   int64(len(raw)),
			},
		},
	}
}

func rawAsset(raw []byte) uasset.RawAsset {
	return uasset.RawAsset{Bytes: raw}
}
