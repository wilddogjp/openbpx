package edit

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

func TestParsePathSupportsPlannedSyntax(t *testing.T) {
	cases := []string{
		"A.B[0].C",
		"Map[\"k\"]",
		"Map[42]",
		"Map[true]",
	}
	for _, tc := range cases {
		if _, err := parsePath(tc); err != nil {
			t.Fatalf("parsePath(%q): %v", tc, err)
		}
	}
}

func TestParsePathRejectsUnsupportedGrammar(t *testing.T) {
	if _, err := parsePath("A..B"); err == nil {
		t.Fatalf("expected parse error for invalid path grammar")
	}
}

func TestResolveEnumNameIndexSupportsScopedAndShortForms(t *testing.T) {
	names := []uasset.NameEntry{
		{Value: "EBPXFixtureEnum::BPXEnum_ValueA"},
		{Value: "BPXEnum_ValueB"},
	}

	if got := resolveEnumNameIndex(names, "EBPXFixtureEnum", "BPXEnum_ValueA"); got != 0 {
		t.Fatalf("short enum name should resolve to scoped NameMap entry: got %d want %d", got, 0)
	}
	if got := resolveEnumNameIndex(names, "EBPXFixtureEnum", "EBPXFixtureEnum::BPXEnum_ValueA"); got != 0 {
		t.Fatalf("scoped enum name should resolve directly: got %d want %d", got, 0)
	}
	if got := resolveEnumNameIndex(names, "EBPXFixtureEnum", "BPXEnum_ValueB"); got != 1 {
		t.Fatalf("existing short enum name should resolve: got %d want %d", got, 1)
	}
	if got := resolveEnumNameIndex(names, "EBPXFixtureEnum", "BPXEnum_ValueC"); got != -1 {
		t.Fatalf("missing enum name should return -1, got %d", got)
	}
}

func TestBuildPropertySetMutationRejectsTypeMismatch(t *testing.T) {
	data := buildEditFixture(t, "hello")
	asset, err := uasset.ParseBytes(data, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	_, err = BuildPropertySetMutation(asset, 0, "MyStr", "123")
	if err == nil {
		t.Fatalf("expected type mismatch error")
	}
}

func TestRewriteAssetRecalculatesOffsetsForVariableLengthEdit(t *testing.T) {
	data := buildEditFixture(t, "hello")
	asset, err := uasset.ParseBytes(data, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	res, err := BuildPropertySetMutation(asset, 0, "MyStr", "\"hello-world-long\"")
	if err != nil {
		t.Fatalf("build mutation: %v", err)
	}

	outBytes, err := RewriteAsset(asset, []ExportMutation{res.Mutation})
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	outAsset, err := uasset.ParseBytes(outBytes, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten fixture: %v", err)
	}

	delta := int64(len(res.Mutation.Payload)) - asset.Exports[0].SerialSize
	if got, want := outAsset.Exports[1].SerialOffset, asset.Exports[1].SerialOffset+delta; got != want {
		t.Fatalf("export2 serial offset: got %d want %d", got, want)
	}
	if got, want := outAsset.Summary.BulkDataStartOffset, asset.Summary.BulkDataStartOffset+delta; got != want {
		t.Fatalf("bulkDataStartOffset: got %d want %d", got, want)
	}
	if got, want := outAsset.Exports[0].ScriptSerializationStartOffset, asset.Exports[0].ScriptSerializationStartOffset; got != want {
		t.Fatalf("script start offset: got %d want %d", got, want)
	}
	if got, want := outAsset.Exports[0].ScriptSerializationEndOffset, asset.Exports[0].ScriptSerializationEndOffset+delta; got != want {
		t.Fatalf("script end offset: got %d want %d", got, want)
	}

	oldSecond := asset.Raw.Bytes[asset.Exports[1].SerialOffset : asset.Exports[1].SerialOffset+asset.Exports[1].SerialSize]
	newSecond := outAsset.Raw.Bytes[outAsset.Exports[1].SerialOffset : outAsset.Exports[1].SerialOffset+outAsset.Exports[1].SerialSize]
	if !bytes.Equal(oldSecond, newSecond) {
		t.Fatalf("second export payload was unexpectedly modified")
	}

	props := outAsset.ParseExportProperties(0)
	if len(props.Warnings) > 0 || len(props.Properties) == 0 {
		t.Fatalf("unexpected parse warnings after rewrite: %v", props.Warnings)
	}
	v, ok := outAsset.DecodePropertyValue(props.Properties[0])
	if !ok {
		t.Fatalf("decode rewritten property failed")
	}
	if got, want := v, "hello-world-long"; got != want {
		t.Fatalf("rewritten value: got %v want %v", got, want)
	}
}

func TestRewriteAssetNoMutationIsByteIdentical(t *testing.T) {
	data := buildEditFixture(t, "hello")
	asset, err := uasset.ParseBytes(data, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	out, err := RewriteAsset(asset, nil)
	if err != nil {
		t.Fatalf("rewrite without mutation: %v", err)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("no-mutation rewrite must be byte-identical")
	}
}

func TestWriteFStringEmptyUsesZeroLength(t *testing.T) {
	w := newByteWriter(binary.LittleEndian, 8)
	w.writeFString("")
	if got, want := w.bytes(), []byte{0, 0, 0, 0}; !bytes.Equal(got, want) {
		t.Fatalf("empty FString encoding mismatch: got %v want %v", got, want)
	}
}

type testExport struct {
	objectNameIndex int32
	payload         []byte
}

func buildEditFixture(t *testing.T, strValue string) []byte {
	t.Helper()

	names := []string{
		"None",
		"Default__BP_Test",
		"HelperExport",
		"ObjectProperty",
		"MyProp",
		"BlueprintGeneratedClass",
		"CoreUObject",
		"StrProperty",
		"MyStr",
		"/Game/TestAsset",
	}
	nameMap := buildNameMap(t, names)
	importMap := buildImportMap(t)

	payload1 := buildStringPropertyPayload(t, 8, 7, strValue)
	payload2 := buildObjectPropertyPayload(t, 4, 3)
	tail := []byte("BULKDATA_PAYLOAD")

	exports := []testExport{
		{objectNameIndex: 1, payload: payload1},
		{objectNameIndex: 2, payload: payload2},
	}

	summaryTemplate := buildSummary(t, summaryBuildArgs{
		NameOffset:           0,
		ImportOffset:         0,
		ExportOffset:         0,
		TotalHeaderSize:      0,
		AssetRegistryOffset:  0,
		BulkDataStartOffset:  0,
		EngineMinor:          6,
		PackageFlags:         0,
		NameCount:            int32(len(names)),
		ImportCount:          int32(len(importMap) / 36),
		ExportCount:          int32(len(exports)),
		NamesReferencedCount: int32(len(names)),
	})
	exportMapTemplate := buildExportMap(t, exports, 0)

	summarySize := len(summaryTemplate)
	nameOffset := int32(summarySize)
	importOffset := int32(summarySize + len(nameMap))
	exportOffset := int32(summarySize + len(nameMap) + len(importMap))
	totalHeader := int32(summarySize + len(nameMap) + len(importMap) + len(exportMapTemplate))

	serialBase := int64(totalHeader)
	exportMap := buildExportMap(t, exports, serialBase)

	serialCursor := serialBase
	for _, e := range exports {
		serialCursor += int64(len(e.payload))
	}
	assetRegistry := int32(serialCursor)
	bulkDataStart := int64(serialCursor)

	summary := buildSummary(t, summaryBuildArgs{
		NameOffset:           nameOffset,
		ImportOffset:         importOffset,
		ExportOffset:         exportOffset,
		TotalHeaderSize:      totalHeader,
		AssetRegistryOffset:  assetRegistry,
		BulkDataStartOffset:  bulkDataStart,
		EngineMinor:          6,
		PackageFlags:         0,
		NameCount:            int32(len(names)),
		ImportCount:          1,
		ExportCount:          int32(len(exports)),
		NamesReferencedCount: int32(len(names)),
	})

	var out bytes.Buffer
	out.Write(summary)
	out.Write(nameMap)
	out.Write(importMap)
	out.Write(exportMap)
	for _, e := range exports {
		out.Write(e.payload)
	}
	out.Write(tail)
	return out.Bytes()
}

type summaryBuildArgs struct {
	NameOffset           int32
	ImportOffset         int32
	ExportOffset         int32
	TotalHeaderSize      int32
	AssetRegistryOffset  int32
	BulkDataStartOffset  int64
	EngineMinor          uint16
	PackageFlags         uint32
	NameCount            int32
	ImportCount          int32
	ExportCount          int32
	NamesReferencedCount int32
}

func buildSummary(t *testing.T, args summaryBuildArgs) []byte {
	t.Helper()

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

	w32(1) // custom version count
	for i := 1; i <= 16; i++ {
		b.WriteByte(byte(i))
	}
	w32(1)

	wstr("/Game/TestAsset")
	wu32(args.PackageFlags)
	w32(args.NameCount)
	w32(args.NameOffset)

	w32(0) // SoftObjectPathsCount
	w32(0) // SoftObjectPathsOffset
	wstr("")

	w32(0) // GatherableTextDataCount
	w32(0) // GatherableTextDataOffset
	w32(args.ExportCount)
	w32(args.ExportOffset)
	w32(args.ImportCount)
	w32(args.ImportOffset)

	w32(0) // CellExportCount
	w32(0) // CellExportOffset
	w32(0) // CellImportCount
	w32(0) // CellImportOffset

	w32(0) // MetaDataOffset
	w32(0) // DependsOffset
	w32(0) // SoftPackageReferencesCount
	w32(0) // SoftPackageReferencesOffset
	w32(0) // SearchableNamesOffset
	w32(0) // ThumbnailTableOffset

	wguid() // PersistentGUID

	w32(1) // GenerationCount
	w32(args.ExportCount)
	w32(args.NameCount)

	wengine(args.EngineMinor)
	wengine(args.EngineMinor)

	wu32(0) // CompressionFlags
	w32(0)  // CompressedChunks
	wu32(0) // PackageSource
	w32(0)  // AdditionalPackagesToCook

	w32(args.AssetRegistryOffset)
	w64(args.BulkDataStartOffset)
	w32(0) // WorldTileInfoDataOffset
	w32(0) // ChunkIDs
	w32(0) // PreloadDependencyCount
	w32(0) // PreloadDependencyOffset
	w32(args.NamesReferencedCount)
	w64(-1) // PayloadTocOffset
	w32(-1) // DataResourceOffset

	return b.Bytes()
}

func buildExportMap(t *testing.T, entries []testExport, serialBase int64) []byte {
	t.Helper()
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
		w32(-1) // ClassIndex => import 1
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

func buildImportMap(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	w32 := func(v int32) { must(binary.Write(&b, binary.LittleEndian, v)) }
	wu32 := func(v uint32) { must(binary.Write(&b, binary.LittleEndian, v)) }
	wname := func(index, number int32) {
		w32(index)
		w32(number)
	}
	wname(6, 0) // ClassPackage
	wname(5, 0) // ClassName
	w32(0)      // OuterIndex
	wname(1, 0) // ObjectName
	wname(0, 0) // PackageName
	wu32(0)     // bImportOptional
	return b.Bytes()
}

func buildNameMap(t *testing.T, names []string) []byte {
	t.Helper()
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

func buildStringPropertyPayload(t *testing.T, propNameIndex, typeNameIndex int32, value string) []byte {
	t.Helper()
	var b bytes.Buffer
	w32 := func(v int32) { must(binary.Write(&b, binary.LittleEndian, v)) }
	w8 := func(v uint8) { must(binary.Write(&b, binary.LittleEndian, v)) }
	wname := func(index, number int32) {
		w32(index)
		w32(number)
	}
	wstr := func(s string) {
		must(binary.Write(&b, binary.LittleEndian, int32(len(s)+1)))
		b.WriteString(s)
		b.WriteByte(0)
	}

	w8(0) // class serialization control extension
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
	_ = wstr
	return b.Bytes()
}

func buildObjectPropertyPayload(t *testing.T, propNameIndex, typeNameIndex int32) []byte {
	t.Helper()
	var b bytes.Buffer
	w32 := func(v int32) { must(binary.Write(&b, binary.LittleEndian, v)) }
	w8 := func(v uint8) { must(binary.Write(&b, binary.LittleEndian, v)) }
	wname := func(index, number int32) {
		w32(index)
		w32(number)
	}

	w8(0) // class serialization control extension
	wname(propNameIndex, 0)
	wname(typeNameIndex, 0)
	w32(0)
	w32(4)
	w8(0)
	w32(0)
	wname(0, 0)
	return b.Bytes()
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
