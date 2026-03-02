package uasset

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestParseBytesMinimalFixture(t *testing.T) {
	data := buildMinimalFixture(t, 6)

	asset, err := ParseBytes(data, DefaultParseOptions())
	if err != nil {
		t.Fatalf("ParseBytes failed: %v", err)
	}

	if got, want := len(asset.Names), 6; got != want {
		t.Fatalf("name count: got %d want %d", got, want)
	}
	if got, want := len(asset.Imports), 1; got != want {
		t.Fatalf("import count: got %d want %d", got, want)
	}
	if got, want := len(asset.Exports), 1; got != want {
		t.Fatalf("export count: got %d want %d", got, want)
	}

	if asset.Exports[0].ObjectName.Display(asset.Names) != "MyObject" {
		t.Fatalf("unexpected export object name: %s", asset.Exports[0].ObjectName.Display(asset.Names))
	}
}

func TestParseExportProperties(t *testing.T) {
	data := buildMinimalFixture(t, 6)
	asset, err := ParseBytes(data, DefaultParseOptions())
	if err != nil {
		t.Fatalf("ParseBytes failed: %v", err)
	}

	props := asset.ParseExportProperties(0)
	if len(props.Warnings) != 0 {
		t.Fatalf("unexpected parse warnings: %v", props.Warnings)
	}
	if got, want := len(props.Properties), 1; got != want {
		t.Fatalf("property count: got %d want %d", got, want)
	}

	p := props.Properties[0]
	if got, want := p.Name.Display(asset.Names), "MyProp"; got != want {
		t.Fatalf("property name: got %q want %q", got, want)
	}
	if got, want := p.TypeString(asset.Names), "ObjectProperty"; got != want {
		t.Fatalf("property type: got %q want %q", got, want)
	}
	if got, want := p.Size, int32(4); got != want {
		t.Fatalf("property size: got %d want %d", got, want)
	}
}

func TestParseExportPropertiesWithSerializationControlPrefix(t *testing.T) {
	data := buildFixture(t, fixtureBuildArgs{
		EngineMinor:                       6,
		NameCount:                         6,
		ImportCount:                       1,
		ExportCount:                       1,
		NamesReferencedCount:              6,
		IncludeScriptOffsets:              true,
		IncludeSerializationControlPrefix: true,
	})
	asset, err := ParseBytes(data, DefaultParseOptions())
	if err != nil {
		t.Fatalf("ParseBytes failed: %v", err)
	}

	props := asset.ParseExportProperties(0)
	if len(props.Warnings) != 0 {
		t.Fatalf("unexpected parse warnings: %v", props.Warnings)
	}
	if got, want := len(props.Properties), 1; got != want {
		t.Fatalf("property count: got %d want %d", got, want)
	}
}

func TestParseExportPropertiesWithOverridableSerializationControl(t *testing.T) {
	data := buildFixture(t, fixtureBuildArgs{
		EngineMinor:                              6,
		NameCount:                                6,
		ImportCount:                              1,
		ExportCount:                              1,
		NamesReferencedCount:                     6,
		IncludeScriptOffsets:                     true,
		IncludeSerializationControlPrefix:        true,
		SerializationControlOverridableOperation: true,
	})
	asset, err := ParseBytes(data, DefaultParseOptions())
	if err != nil {
		t.Fatalf("ParseBytes failed: %v", err)
	}

	props := asset.ParseExportProperties(0)
	if len(props.Warnings) != 0 {
		t.Fatalf("unexpected parse warnings: %v", props.Warnings)
	}
	if got, want := len(props.Properties), 1; got != want {
		t.Fatalf("property count: got %d want %d", got, want)
	}
}

func TestParseExportPropertiesIgnoresUnknownSerializationControlExtensionBits(t *testing.T) {
	data := buildFixture(t, fixtureBuildArgs{
		EngineMinor:                       6,
		NameCount:                         6,
		ImportCount:                       1,
		ExportCount:                       1,
		NamesReferencedCount:              6,
		IncludeScriptOffsets:              true,
		IncludeSerializationControlPrefix: true,
		SerializationControlRaw:           0x80,
	})
	asset, err := ParseBytes(data, DefaultParseOptions())
	if err != nil {
		t.Fatalf("ParseBytes failed: %v", err)
	}

	props := asset.ParseExportProperties(0)
	if got, want := len(props.Properties), 1; got != want {
		t.Fatalf("property count: got %d want %d", got, want)
	}
	for _, w := range props.Warnings {
		if strings.Contains(w, "unsupported serialization control extension") {
			t.Fatalf("unexpected unsupported extension warning: %v", props.Warnings)
		}
	}
}

func TestParseExportPropertiesSupportsLegacyPropertyTagFormat(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "ObjectProperty"},
		{Value: "MyProp"},
	}

	var raw bytes.Buffer
	w32 := func(v int32) { must(binary.Write(&raw, binary.LittleEndian, v)) }
	wu8 := func(v uint8) { must(binary.Write(&raw, binary.LittleEndian, v)) }
	wname := func(index, number int32) {
		w32(index)
		w32(number)
	}

	wname(2, 0) // property name
	wname(1, 0) // legacy type
	w32(4)      // size
	w32(0)      // array index
	wu8(0)      // has property guid
	w32(0)      // value (FPackageIndex)
	wname(0, 0) // None terminator

	asset := &Asset{
		Raw:   RawAsset{Bytes: raw.Bytes()},
		Names: names,
		Summary: PackageSummary{
			FileVersionUE4: ue4VersionUE56,
			FileVersionUE5: 1006,
		},
	}
	props := asset.ParseTaggedPropertiesRange(0, raw.Len(), false)
	if len(props.Warnings) != 0 {
		t.Fatalf("unexpected parse warnings: %v", props.Warnings)
	}
	if got, want := len(props.Properties), 1; got != want {
		t.Fatalf("property count: got %d want %d", got, want)
	}
	p := props.Properties[0]
	if got, want := p.TypeString(asset.Names), "ObjectProperty"; got != want {
		t.Fatalf("property type: got %q want %q", got, want)
	}
	if got, want := p.Size, int32(4); got != want {
		t.Fatalf("property size: got %d want %d", got, want)
	}
}

func TestParseLegacyPropertyTagFormatBoolValueFromTag(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "BoolProperty"},
		{Value: "bEnabled"},
	}

	var raw bytes.Buffer
	w32 := func(v int32) { must(binary.Write(&raw, binary.LittleEndian, v)) }
	wu8 := func(v uint8) { must(binary.Write(&raw, binary.LittleEndian, v)) }
	wname := func(index, number int32) {
		w32(index)
		w32(number)
	}

	wname(2, 0) // property name
	wname(1, 0) // legacy type
	w32(0)      // size
	w32(0)      // array index
	wu8(1)      // bool value
	wu8(0)      // has property guid
	wname(0, 0) // None terminator

	asset := &Asset{
		Raw:   RawAsset{Bytes: raw.Bytes()},
		Names: names,
		Summary: PackageSummary{
			FileVersionUE4: ue4VersionUE56,
			FileVersionUE5: 1006,
		},
	}
	props := asset.ParseTaggedPropertiesRange(0, raw.Len(), false)
	if len(props.Warnings) != 0 {
		t.Fatalf("unexpected parse warnings: %v", props.Warnings)
	}
	if got, want := len(props.Properties), 1; got != want {
		t.Fatalf("property count: got %d want %d", got, want)
	}
	p := props.Properties[0]
	if p.Flags&propertyFlagBoolTrue == 0 {
		t.Fatalf("expected BoolTrue flag from legacy bool tag")
	}
	val, ok := asset.DecodePropertyValue(p)
	if !ok {
		t.Fatalf("decode bool value failed")
	}
	if got, ok := val.(bool); !ok || !got {
		t.Fatalf("bool decode: got=%v ok=%v", val, ok)
	}
}

func TestParseLegacyPropertyTagFormatWithExtensions(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "ObjectProperty"},
		{Value: "MyProp"},
	}

	var raw bytes.Buffer
	w32 := func(v int32) { must(binary.Write(&raw, binary.LittleEndian, v)) }
	wu8 := func(v uint8) { must(binary.Write(&raw, binary.LittleEndian, v)) }
	wu32 := func(v uint32) { must(binary.Write(&raw, binary.LittleEndian, v)) }
	wname := func(index, number int32) {
		w32(index)
		w32(number)
	}

	wname(2, 0) // property name
	wname(1, 0) // legacy type
	w32(4)      // size
	w32(0)      // array index
	wu8(0)      // has property guid
	wu8(propertyExtensionOverridableInfo)
	wu8(3)  // overridable operation
	wu32(1) // experimental overridable logic
	w32(0)  // value
	wname(0, 0)

	asset := &Asset{
		Raw:   RawAsset{Bytes: raw.Bytes()},
		Names: names,
		Summary: PackageSummary{
			FileVersionUE4: ue4VersionUE56,
			FileVersionUE5: 1011,
		},
	}
	props := asset.ParseTaggedPropertiesRange(0, raw.Len(), false)
	if len(props.Warnings) != 0 {
		t.Fatalf("unexpected parse warnings: %v", props.Warnings)
	}
	if got, want := len(props.Properties), 1; got != want {
		t.Fatalf("property count: got %d want %d", got, want)
	}
	p := props.Properties[0]
	if got, want := p.PropertyExtensions, uint8(propertyExtensionOverridableInfo); got != want {
		t.Fatalf("property extensions: got %d want %d", got, want)
	}
	if got, want := p.OverridableOperation, uint8(3); got != want {
		t.Fatalf("overridable operation: got %d want %d", got, want)
	}
	if !p.ExperimentalOverridableLogic {
		t.Fatalf("expected experimental overridable logic flag")
	}
}

func TestParseTaggedPropertiesKeepsOverridableExtensionDetails(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "ObjectProperty"},
		{Value: "MyProp"},
	}

	var raw bytes.Buffer
	w32 := func(v int32) { must(binary.Write(&raw, binary.LittleEndian, v)) }
	wu8 := func(v uint8) { must(binary.Write(&raw, binary.LittleEndian, v)) }
	wu32 := func(v uint32) { must(binary.Write(&raw, binary.LittleEndian, v)) }
	wname := func(index, number int32) {
		w32(index)
		w32(number)
	}

	wname(2, 0) // property name
	wname(1, 0) // type node name
	w32(0)      // type node inner count
	w32(4)      // size
	wu8(propertyFlagHasPropertyExtensions)
	wu8(propertyExtensionOverridableInfo) // extensions
	wu8(3)                                // overridable operation
	wu32(1)                               // experimental overridable logic (UBool true)
	w32(0)                                // value
	wname(0, 0)                           // None terminator

	asset := &Asset{
		Raw:     RawAsset{Bytes: raw.Bytes()},
		Names:   names,
		Summary: PackageSummary{FileVersionUE5: 1017},
	}
	props := asset.ParseTaggedPropertiesRange(0, raw.Len(), false)
	if len(props.Warnings) != 0 {
		t.Fatalf("unexpected parse warnings: %v", props.Warnings)
	}
	if got, want := len(props.Properties), 1; got != want {
		t.Fatalf("property count: got %d want %d", got, want)
	}
	p := props.Properties[0]
	if got, want := p.PropertyExtensions, uint8(propertyExtensionOverridableInfo); got != want {
		t.Fatalf("property extensions: got %d want %d", got, want)
	}
	if got, want := p.OverridableOperation, uint8(3); got != want {
		t.Fatalf("overridable operation: got %d want %d", got, want)
	}
	if !p.ExperimentalOverridableLogic {
		t.Fatalf("expected experimental overridable logic flag")
	}
}

func TestParseBytesRejectsOutOfWindowUEVersion(t *testing.T) {
	data := buildMinimalFixture(t, 6)
	binary.LittleEndian.PutUint32(data[16:20], uint32(999))
	_, err := ParseBytes(data, DefaultParseOptions())
	if err == nil {
		t.Fatalf("expected version-window rejection")
	}
}

func TestParseBytesRejectsOutOfWindowUEVersionBeforeParsingMaps(t *testing.T) {
	data := buildMinimalFixture(t, 6)
	binary.LittleEndian.PutUint32(data[16:20], uint32(9999))

	r := newByteReader(data)
	if _, err := parseSummary(r); err != nil {
		t.Fatalf("parseSummary failed: %v", err)
	}
	data = data[:r.offset()]

	_, err := ParseBytes(data, DefaultParseOptions())
	if err == nil {
		t.Fatalf("expected version-window rejection")
	}
	if !strings.Contains(err.Error(), "unsupported fileVersionUE5=9999") {
		t.Fatalf("expected version rejection before map parsing, got: %v", err)
	}
}

func TestParseBytesAcceptsUnversionedPackage(t *testing.T) {
	data := buildMinimalFixture(t, 6)
	// Summary version fields: FileVersionUE4 / FileVersionUE5 / FileVersionLicenseeUE4.
	binary.LittleEndian.PutUint32(data[12:16], 0)
	binary.LittleEndian.PutUint32(data[16:20], 0)
	binary.LittleEndian.PutUint32(data[20:24], 0)

	asset, err := ParseBytes(data, DefaultParseOptions())
	if err != nil {
		t.Fatalf("ParseBytes failed: %v", err)
	}
	if !asset.Summary.Unversioned {
		t.Fatalf("expected unversioned summary flag")
	}
}

func TestParseSummaryClearsTransientPackageFlags(t *testing.T) {
	const persistent = uint32(0x00020000) // PKG_ContainsMap
	data := buildFixture(t, fixtureBuildArgs{
		EngineMinor:          6,
		PackageFlags:         persistent | pkgFlagNewlyCreated | pkgFlagIsSaving | pkgFlagReloadingForCooker,
		NameCount:            6,
		ImportCount:          1,
		ExportCount:          1,
		NamesReferencedCount: 6,
		IncludeScriptOffsets: true,
	})

	asset, err := ParseBytes(data, ParseOptions{
		KeepUnknown: true,
	})
	if err != nil {
		t.Fatalf("ParseBytes failed: %v", err)
	}

	if got, want := asset.Summary.PackageFlags, persistent; got != want {
		t.Fatalf("package flags: got 0x%08x want 0x%08x", got, want)
	}
}

func TestGuessAssetKindUsesClassName(t *testing.T) {
	asset := &Asset{
		Names: []NameEntry{
			{Value: "None"},
			{Value: "BP_DataTable"},
			{Value: "BlueprintGeneratedClass"},
			{Value: "DataTable"},
		},
		Imports: []ImportEntry{
			{ObjectName: NameRef{Index: 2, Number: 0}},
		},
		Exports: []ExportEntry{
			{
				ObjectName: NameRef{Index: 1, Number: 0},
				ClassIndex: -1,
			},
		},
	}

	if got, want := asset.GuessAssetKind(), "Blueprint"; got != want {
		t.Fatalf("GuessAssetKind: got %q want %q", got, want)
	}

	asset.Imports[0].ObjectName = NameRef{Index: 3, Number: 0}
	if got, want := asset.GuessAssetKind(), "DataTable"; got != want {
		t.Fatalf("GuessAssetKind(datatable): got %q want %q", got, want)
	}
}

func TestRawRoundTrip(t *testing.T) {
	data := buildMinimalFixture(t, 6)
	asset, err := ParseBytes(data, DefaultParseOptions())
	if err != nil {
		t.Fatalf("ParseBytes failed: %v", err)
	}
	out := asset.Raw.SerializeUnmodified()
	if !bytes.Equal(data, out) {
		t.Fatalf("roundtrip bytes mismatch")
	}
}

func TestParseBytesRejectsTooLargeNameCount(t *testing.T) {
	data := buildFixture(t, fixtureBuildArgs{
		EngineMinor:          6,
		NameCount:            maxTableEntries + 1,
		ImportCount:          1,
		ExportCount:          1,
		NamesReferencedCount: 6,
		IncludeScriptOffsets: true,
	})
	_, err := ParseBytes(data, DefaultParseOptions())
	if err == nil || !strings.Contains(err.Error(), "name count") {
		t.Fatalf("expected name count validation error, got: %v", err)
	}
}

func TestParseBytesRejectsTooLargeImportCount(t *testing.T) {
	data := buildFixture(t, fixtureBuildArgs{
		EngineMinor:          6,
		NameCount:            6,
		ImportCount:          maxTableEntries + 1,
		ExportCount:          1,
		NamesReferencedCount: 6,
		IncludeScriptOffsets: true,
	})
	_, err := ParseBytes(data, DefaultParseOptions())
	if err == nil || !strings.Contains(err.Error(), "import count") {
		t.Fatalf("expected import count validation error, got: %v", err)
	}
}

func TestParseBytesRejectsTooLargeExportCount(t *testing.T) {
	data := buildFixture(t, fixtureBuildArgs{
		EngineMinor:          6,
		NameCount:            6,
		ImportCount:          1,
		ExportCount:          maxTableEntries + 1,
		NamesReferencedCount: 6,
		IncludeScriptOffsets: true,
	})
	_, err := ParseBytes(data, DefaultParseOptions())
	if err == nil || !strings.Contains(err.Error(), "export count") {
		t.Fatalf("expected export count validation error, got: %v", err)
	}
}

func TestReadFStringRejectsOversizedNarrow(t *testing.T) {
	var b bytes.Buffer
	must(binary.Write(&b, binary.LittleEndian, int32(maxFStringBytes+1)))

	r := newByteReader(b.Bytes())
	_, err := r.readFString()
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected oversized narrow string error, got: %v", err)
	}
}

func TestReadFStringRejectsOversizedWide(t *testing.T) {
	var b bytes.Buffer
	must(binary.Write(&b, binary.LittleEndian, int32(-(maxFStringUTF16Units + 1))))

	r := newByteReader(b.Bytes())
	_, err := r.readFString()
	if err == nil || !strings.Contains(err.Error(), "too-large wide string count") {
		t.Fatalf("expected oversized wide string error, got: %v", err)
	}
}

func TestReadFStringRejectsMinInt32(t *testing.T) {
	var b bytes.Buffer
	must(binary.Write(&b, binary.LittleEndian, int32(math.MinInt32)))

	r := newByteReader(b.Bytes())
	_, err := r.readFString()
	if err == nil || !strings.Contains(err.Error(), "invalid wide string count") {
		t.Fatalf("expected invalid wide string count error, got: %v", err)
	}
}

func TestReadUBoolAllowsNonZeroTrue(t *testing.T) {
	var b bytes.Buffer
	must(binary.Write(&b, binary.LittleEndian, uint32(2)))

	r := newByteReader(b.Bytes())
	v, err := r.readUBool()
	if err != nil {
		t.Fatalf("readUBool: %v", err)
	}
	if !v {
		t.Fatalf("readUBool value: got false want true")
	}
}

func TestReadSoftObjectSubPathSupportsUTF8AndLegacyWide(t *testing.T) {
	var utf8 bytes.Buffer
	must(binary.Write(&utf8, binary.LittleEndian, int32(4)))
	utf8.WriteString("Root")
	r := newByteReader(utf8.Bytes())
	gotUTF8, err := r.readSoftObjectSubPath()
	if err != nil {
		t.Fatalf("readSoftObjectSubPath(utf8): %v", err)
	}
	if gotUTF8 != "Root" {
		t.Fatalf("utf8 sub path: got %q want %q", gotUTF8, "Root")
	}

	var wide bytes.Buffer
	must(binary.Write(&wide, binary.LittleEndian, int32(-5)))
	for _, ch := range []uint16{'R', 'o', 'o', 't', 0} {
		must(binary.Write(&wide, binary.LittleEndian, ch))
	}
	r = newByteReader(wide.Bytes())
	gotWide, err := r.readSoftObjectSubPath()
	if err != nil {
		t.Fatalf("readSoftObjectSubPath(wide): %v", err)
	}
	if gotWide != "Root" {
		t.Fatalf("wide sub path: got %q want %q", gotWide, "Root")
	}
}

func TestParseSummaryAcceptsByteSwappedTag(t *testing.T) {
	var b bytes.Buffer
	must(binary.Write(&b, binary.LittleEndian, uint32(packageFileTagSwapped)))
	must(binary.Write(&b, binary.BigEndian, int32(-9)))
	must(binary.Write(&b, binary.BigEndian, int32(864)))
	must(binary.Write(&b, binary.BigEndian, int32(522)))
	must(binary.Write(&b, binary.BigEndian, int32(1017)))
	must(binary.Write(&b, binary.BigEndian, int32(0)))

	r := newByteReader(b.Bytes())
	summary, err := parseSummary(r)
	if err == nil {
		t.Fatalf("expected EOF after partial summary payload")
	}
	if !summary.ByteSwapped {
		t.Fatalf("expected byte-swapped summary flag")
	}
	if got, want := summary.Tag, int32(int64(packageFileTag)-(1<<32)); got != want {
		t.Fatalf("summary tag: got 0x%x want 0x%x", uint32(got), uint32(want))
	}
	if got, want := summary.LegacyFileVersion, int32(-9); got != want {
		t.Fatalf("legacy version: got %d want %d", got, want)
	}
	if got, want := summary.FileVersionUE5, int32(1017); got != want {
		t.Fatalf("fileVersionUE5: got %d want %d", got, want)
	}
}

func TestParseBytesUnversionedPropertySerializationOmitsScriptOffsets(t *testing.T) {
	data := buildFixture(t, fixtureBuildArgs{
		EngineMinor:          6,
		PackageFlags:         pkgFlagUnversionedProps,
		NameCount:            6,
		ImportCount:          1,
		ExportCount:          1,
		NamesReferencedCount: 6,
		IncludeScriptOffsets: false,
	})

	asset, err := ParseBytes(data, DefaultParseOptions())
	if err != nil {
		t.Fatalf("ParseBytes failed: %v", err)
	}
	if got := asset.Exports[0].ScriptSerializationStartOffset; got != 0 {
		t.Fatalf("script start offset: got %d want 0", got)
	}
	if got := asset.Exports[0].ScriptSerializationEndOffset; got != 0 {
		t.Fatalf("script end offset: got %d want 0", got)
	}
}

func TestSummaryFeatureGuards(t *testing.T) {
	s := PackageSummary{FileVersionUE5: 1012}
	if !s.SupportsPropertyTagCompleteTypeName() {
		t.Fatalf("expected property tag complete type-name support")
	}
	if !s.SupportsTopLevelAssetPathSoftObjectPath() {
		t.Fatalf("expected soft object path top-level asset path support")
	}
	if !s.SupportsScriptSerializationOffsets() {
		t.Fatalf("expected script serialization offset support")
	}

	s.FileVersionUE5 = 1011
	if s.SupportsPropertyTagCompleteTypeName() {
		t.Fatalf("expected no complete type-name support")
	}
	s.FileVersionUE5 = 1007
	if !s.SupportsTopLevelAssetPathSoftObjectPath() {
		t.Fatalf("expected top-level asset path soft object support at 1007")
	}
	if s.SupportsSoftObjectPathListInSummary() {
		t.Fatalf("expected no soft object path list support before 1008")
	}
	s.FileVersionUE5 = 1008
	if !s.SupportsSoftObjectPathListInSummary() {
		t.Fatalf("expected soft object path list support at 1008")
	}
	s.FileVersionUE5 = 1009
	if s.SupportsScriptSerializationOffsets() {
		t.Fatalf("expected no script serialization offset support before 1010")
	}
	s.FileVersionUE5 = 1010
	s.PackageFlags = pkgFlagUnversionedProps
	if s.SupportsScriptSerializationOffsets() {
		t.Fatalf("expected no script serialization offset support with unversioned properties")
	}
}

func TestIsUE56VersionChecks(t *testing.T) {
	s := PackageSummary{
		FileVersionUE5: 1017,
		CustomVersions: []CustomVersion{{}},
		SavedByEngineVersion: EngineVersion{
			Major: 5,
			Minor: 6,
		},
	}
	if !s.IsUE56() {
		t.Fatalf("expected UE5.6 summary to pass")
	}

	s.SavedByEngineVersion.Minor = 5
	if s.IsUE56() {
		t.Fatalf("expected non-5.6 saved version to fail")
	}

	s.SavedByEngineVersion = EngineVersion{}
	if !s.IsUE56() {
		t.Fatalf("expected zeroed engine version to use custom-version fallback")
	}

	s.CustomVersions = nil
	if !s.IsUE56() {
		t.Fatalf("expected empty saved/compatible engine version to remain UE5.6-compatible")
	}

	s.CompatibleEngineVersion = EngineVersion{Major: 5, Minor: 5}
	if s.IsUE56() {
		t.Fatalf("expected non-5.6 compatible version to fail")
	}

	s.CompatibleEngineVersion = EngineVersion{Major: 5, Minor: 6}
	if !s.IsUE56() {
		t.Fatalf("expected 5.6 compatible version to pass")
	}
}

func TestValidateSupportedVersionWindow(t *testing.T) {
	if err := validateSupportedVersionWindow(PackageSummary{FileVersionUE5: 1000}); err != nil {
		t.Fatalf("validate lower bound: %v", err)
	}
	if err := validateSupportedVersionWindow(PackageSummary{FileVersionUE5: 1017}); err != nil {
		t.Fatalf("validate upper bound: %v", err)
	}
	if err := validateSupportedVersionWindow(PackageSummary{FileVersionUE5: 999}); err == nil {
		t.Fatalf("expected lower-than-supported version rejection")
	}
	if err := validateSupportedVersionWindow(PackageSummary{FileVersionUE5: 9999}); err == nil {
		t.Fatalf("expected higher-than-supported version rejection")
	}
}

func TestReadCustomVersionsEnums(t *testing.T) {
	var b bytes.Buffer
	w32 := func(v int32) { must(binary.Write(&b, binary.LittleEndian, v)) }
	wu32 := func(v uint32) { must(binary.Write(&b, binary.LittleEndian, v)) }

	w32(2)
	wu32(7)
	w32(3)
	wu32(0xDEADBEEF)
	w32(9)

	r := newByteReader(b.Bytes())
	versions, err := readCustomVersions(r, -2)
	if err != nil {
		t.Fatalf("readCustomVersions enums: %v", err)
	}
	if got, want := len(versions), 2; got != want {
		t.Fatalf("count: got %d want %d", got, want)
	}
	if got, want := versions[0].Version, int32(3); got != want {
		t.Fatalf("version[0]: got %d want %d", got, want)
	}
	if got, want := binary.LittleEndian.Uint32(versions[0].Key[12:16]), uint32(7); got != want {
		t.Fatalf("tag[0]: got 0x%x want 0x%x", got, want)
	}
	if got, want := binary.LittleEndian.Uint32(versions[1].Key[12:16]), uint32(0xDEADBEEF); got != want {
		t.Fatalf("tag[1]: got 0x%x want 0x%x", got, want)
	}
}

func TestReadCustomVersionsGuids(t *testing.T) {
	var b bytes.Buffer
	w32 := func(v int32) { must(binary.Write(&b, binary.LittleEndian, v)) }
	wstr := func(s string) {
		must(binary.Write(&b, binary.LittleEndian, int32(len(s)+1)))
		b.WriteString(s)
		b.WriteByte(0)
	}

	w32(1)
	for i := 1; i <= 16; i++ {
		b.WriteByte(byte(i))
	}
	w32(42)
	wstr("Friendly")

	r := newByteReader(b.Bytes())
	versions, err := readCustomVersions(r, -4)
	if err != nil {
		t.Fatalf("readCustomVersions guids: %v", err)
	}
	if got, want := len(versions), 1; got != want {
		t.Fatalf("count: got %d want %d", got, want)
	}
	if got, want := versions[0].Version, int32(42); got != want {
		t.Fatalf("version: got %d want %d", got, want)
	}
	if got, want := versions[0].Key[0], byte(1); got != want {
		t.Fatalf("guid[0]: got %d want %d", got, want)
	}
}

func buildMinimalFixture(t *testing.T, engineMinor uint16) []byte {
	return buildFixture(t, fixtureBuildArgs{
		EngineMinor:                       engineMinor,
		NameCount:                         6,
		ImportCount:                       1,
		ExportCount:                       1,
		NamesReferencedCount:              6,
		IncludeScriptOffsets:              true,
		IncludeSerializationControlPrefix: true,
	})
}

type fixtureBuildArgs struct {
	EngineMinor          uint16
	PackageFlags         uint32
	NameCount            int32
	ImportCount          int32
	ExportCount          int32
	NamesReferencedCount int32
	IncludeScriptOffsets bool

	IncludeSerializationControlPrefix        bool
	SerializationControlOverridableOperation bool
	SerializationControlRaw                  uint8
}

func buildFixture(t *testing.T, args fixtureBuildArgs) []byte {
	t.Helper()

	names := []string{"None", "MyObject", "ObjectProperty", "MyProp", "BlueprintGeneratedClass", "CoreUObject"}
	nameMap := buildNameMap(t, names)
	importMap := buildImportMap(t)
	propertyData := buildPropertyData(t, propertyDataBuildArgs{
		IncludeSerializationControlPrefix:        args.IncludeSerializationControlPrefix,
		SerializationControlOverridableOperation: args.SerializationControlOverridableOperation,
		SerializationControlRaw:                  args.SerializationControlRaw,
	})

	summaryTemplate := buildSummary(t, summaryBuildArgs{
		NameOffset:           0,
		ImportOffset:         0,
		ExportOffset:         0,
		TotalHeaderSize:      0,
		BulkDataStartOffset:  0,
		EngineMinor:          args.EngineMinor,
		PackageFlags:         args.PackageFlags,
		NameCount:            args.NameCount,
		ImportCount:          args.ImportCount,
		ExportCount:          args.ExportCount,
		NamesReferencedCount: args.NamesReferencedCount,
	})

	exportMapTemplate := buildExportMap(t, exportBuildArgs{
		SerialSize:           int64(len(propertyData)),
		SerialOffset:         0,
		IncludeScriptOffsets: args.IncludeScriptOffsets,
	})
	summarySize := len(summaryTemplate)
	nameOffset := int32(summarySize)
	importOffset := int32(summarySize + len(nameMap))
	exportOffset := int32(summarySize + len(nameMap) + len(importMap))
	totalHeader := int32(summarySize + len(nameMap) + len(importMap) + len(exportMapTemplate))
	serialOffset := int64(totalHeader)
	fullSize := int64(totalHeader + int32(len(propertyData)))

	exportMap := buildExportMap(t, exportBuildArgs{
		SerialSize:           int64(len(propertyData)),
		SerialOffset:         serialOffset,
		IncludeScriptOffsets: args.IncludeScriptOffsets,
	})
	summary := buildSummary(t, summaryBuildArgs{
		NameOffset:           nameOffset,
		ImportOffset:         importOffset,
		ExportOffset:         exportOffset,
		TotalHeaderSize:      totalHeader,
		BulkDataStartOffset:  fullSize,
		EngineMinor:          args.EngineMinor,
		PackageFlags:         args.PackageFlags,
		NameCount:            args.NameCount,
		ImportCount:          args.ImportCount,
		ExportCount:          args.ExportCount,
		NamesReferencedCount: args.NamesReferencedCount,
	})

	var out bytes.Buffer
	out.Write(summary)
	out.Write(nameMap)
	out.Write(importMap)
	out.Write(exportMap)
	out.Write(propertyData)
	return out.Bytes()
}

type summaryBuildArgs struct {
	NameOffset           int32
	ImportOffset         int32
	ExportOffset         int32
	TotalHeaderSize      int32
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

	wstr("") // LocalizationId

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

	wguid() // PersistentGuid

	w32(1) // GenerationCount
	w32(args.ExportCount)
	w32(args.NameCount)

	wengine(args.EngineMinor)
	wengine(args.EngineMinor)

	wu32(0) // CompressionFlags
	w32(0)  // CompressedChunks
	wu32(0) // PackageSource
	w32(0)  // AdditionalPackagesToCook

	w32(0) // AssetRegistryDataOffset
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

type exportBuildArgs struct {
	SerialSize           int64
	SerialOffset         int64
	IncludeScriptOffsets bool
}

func buildExportMap(t *testing.T, args exportBuildArgs) []byte {
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

	w32(-1) // ClassIndex => import 1
	w32(0)  // SuperIndex
	w32(0)  // TemplateIndex
	w32(0)  // OuterIndex
	wname(1, 0)
	wu32(0)
	w64(args.SerialSize)
	w64(args.SerialOffset)
	wbool(false) // bForcedExport
	wbool(false) // bNotForClient
	wbool(false) // bNotForServer
	wbool(false) // bIsInheritedInstance
	wu32(0)      // PackageFlags
	wbool(false) // bNotAlwaysLoadedForEditorGame
	wbool(true)  // bIsAsset
	wbool(false) // bGeneratePublicHash
	w32(-1)
	w32(0)
	w32(0)
	w32(0)
	w32(0)
	if args.IncludeScriptOffsets {
		w64(0)               // ScriptSerializationStartOffset
		w64(args.SerialSize) // ScriptSerializationEndOffset
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

	wname(5, 0) // ClassPackage
	wname(4, 0) // ClassName
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

type propertyDataBuildArgs struct {
	IncludeSerializationControlPrefix        bool
	SerializationControlOverridableOperation bool
	SerializationControlRaw                  uint8
}

func buildPropertyData(t *testing.T, args propertyDataBuildArgs) []byte {
	t.Helper()
	var b bytes.Buffer
	w32 := func(v int32) { must(binary.Write(&b, binary.LittleEndian, v)) }
	w8 := func(v uint8) { must(binary.Write(&b, binary.LittleEndian, v)) }
	wname := func(index, number int32) {
		w32(index)
		w32(number)
	}

	if args.IncludeSerializationControlPrefix {
		control := args.SerializationControlRaw
		if control == 0 {
			control = 0x00
			if args.SerializationControlOverridableOperation {
				control |= 0x02
			}
		}
		w8(control)
		if control&0x02 != 0 {
			w8(1) // EOverriddenPropertyOperation::Replace
		}
	}

	wname(3, 0) // Property name: MyProp
	wname(2, 0) // Type node name: ObjectProperty
	w32(0)      // Type node inner count
	w32(4)      // Size
	w8(0)       // Flags
	w32(0)      // Value (FPackageIndex)
	wname(0, 0) // None terminator
	return b.Bytes()
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
