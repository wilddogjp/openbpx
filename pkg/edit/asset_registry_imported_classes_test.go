package edit

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

func TestInsertImportEntriesUpdatesAssetRegistryImportedClassesTrailer(t *testing.T) {
	fixturePath := findBrushImageBeforeFixture(t)
	if fixturePath == "" {
		t.Skip("widget_write_brush_image fixture not found")
	}

	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	asset, err := uasset.ParseBytes(data, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	packageEntry := mustBuildImportEntry(asset, "/Script/CoreUObject", "Package", 0, "/Script/CoreUObject")
	withPackage, err := InsertImportEntries(asset, 9, []uasset.ImportEntry{packageEntry})
	if err != nil {
		t.Fatalf("insert package import: %v", err)
	}
	assetWithPackage, err := uasset.ParseBytes(withPackage, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse package-inserted asset: %v", err)
	}

	textureLikeEntry := mustBuildImportEntry(assetWithPackage, "/Script/UMG", "UserWidget", uasset.PackageIndex(-10), "Default__UserWidget")
	withTextureLike, err := InsertImportEntries(assetWithPackage, 17, []uasset.ImportEntry{textureLikeEntry})
	if err != nil {
		t.Fatalf("insert second import: %v", err)
	}
	rewritten, err := uasset.ParseBytes(withTextureLike, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}

	trailer := mustParseImportedClassesTrailer(t, rewritten)
	if got, want := trailer.ImportCount, uint32(21); got != want {
		t.Fatalf("importCount: got %d want %d", got, want)
	}
	if got, want := trailer.ImportFlags, []uint32{0x64af4}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("importFlags: got %#x want %#x", got, want)
	}
	if trailer.TailA != 1 || trailer.TailB != 1 || trailer.TailC != 0 {
		t.Fatalf("tail ints: got (%d,%d,%d) want (1,1,0)", trailer.TailA, trailer.TailB, trailer.TailC)
	}
}

func TestInsertImportEntriesUpdatesAssetRegistryImportedClassesTrailerWidgetAddFixture(t *testing.T) {
	fixturePath := findWidgetAddBeforeFixture(t, "widget_add_image_canvaspanel")
	if fixturePath == "" {
		t.Skip("widget_add_image_canvaspanel fixture not found")
	}

	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	asset, err := uasset.ParseBytes(data, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	widgetTreeEntry := mustBuildImportEntry(asset, "/Script/CoreUObject", "Class", uasset.PackageIndex(-14), "WidgetTree")
	rewrittenBytes, err := InsertImportEntries(asset, 5, []uasset.ImportEntry{widgetTreeEntry})
	if err != nil {
		t.Fatalf("insert WidgetTree import: %v", err)
	}
	rewritten, err := uasset.ParseBytes(rewrittenBytes, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}

	trailer := mustParseImportedClassesTrailer(t, rewritten)
	if got, want := trailer.ImportCount, uint32(20); got != want {
		t.Fatalf("importCount: got %d want %d", got, want)
	}
	if got, want := trailer.TailA, uint32(2); got != want {
		t.Fatalf("tailA: got %d want %d", got, want)
	}
	if got, want := trailer.TailB, uint32(3); got != want {
		t.Fatalf("tailB: got %d want %d", got, want)
	}
	if got, want := trailer.TailC, uint32(0); got != want {
		t.Fatalf("tailC: got %d want %d", got, want)
	}
}

func TestInsertImportEntriesRejectsUnsupportedImportedClassesTrailer(t *testing.T) {
	fixturePath := findBrushImageBeforeFixture(t)
	if fixturePath == "" {
		t.Skip("widget_write_brush_image fixture not found")
	}

	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	asset, err := uasset.ParseBytes(data, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	_, sectionStart, sectionEnd, present := assetRegistrySectionByOffset(asset, int64(asset.Summary.AssetRegistryDataOffset))
	if !present {
		t.Fatalf("asset registry section not present")
	}
	mutated := append([]byte(nil), data...)
	mutated[sectionEnd-1] = 1
	if sectionEnd-sectionStart < 21 {
		t.Fatalf("asset registry section too small for trailer mutation")
	}

	mutatedAsset, err := uasset.ParseBytes(mutated, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse mutated fixture: %v", err)
	}
	entry := mustBuildImportEntry(mutatedAsset, "/Script/CoreUObject", "Package", 0, "/Script/CoreUObject")
	_, err = InsertImportEntries(mutatedAsset, 9, []uasset.ImportEntry{entry})
	if err == nil {
		t.Fatalf("expected unsupported trailer to fail")
	}
	if !strings.Contains(err.Error(), "asset registry imported classes trailer layout is unsupported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func findBrushImageBeforeFixture(t *testing.T) string {
	t.Helper()

	for _, root := range goldenFixtureRoots(t, "operations") {
		path := filepath.Join(root, "operations", "widget_write_brush_image", "before.uasset")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func findWidgetAddBeforeFixture(t *testing.T, opName string) string {
	t.Helper()

	for _, root := range goldenFixtureRoots(t, "operations") {
		path := filepath.Join(root, "operations", opName, "before.uasset")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func mustBuildImportEntry(asset *uasset.Asset, classPackage, className string, outerIndex uasset.PackageIndex, objectName string) uasset.ImportEntry {
	classPackageIndex := findTestNameIndexByValue(asset.Names, classPackage)
	classNameIndex := findTestNameIndexByValue(asset.Names, className)
	objectNameIndex := findTestNameIndexByValue(asset.Names, objectName)
	noneIndex := findTestNameIndexByValue(asset.Names, "None")
	if classPackageIndex < 0 || classNameIndex < 0 || objectNameIndex < 0 || noneIndex < 0 {
		panic("required import entry names are missing from fixture")
	}
	return uasset.ImportEntry{
		ClassPackage: uasset.NameRef{Index: int32(classPackageIndex)},
		ClassName:    uasset.NameRef{Index: int32(classNameIndex)},
		OuterIndex:   outerIndex,
		ObjectName:   uasset.NameRef{Index: int32(objectNameIndex)},
		PackageName:  uasset.NameRef{Index: int32(noneIndex)},
	}
}

func mustParseImportedClassesTrailer(t *testing.T, asset *uasset.Asset) *assetRegistryImportedClassesTrailer {
	t.Helper()

	sectionBytes, _, _, present := assetRegistrySectionByOffset(asset, int64(asset.Summary.AssetRegistryDataOffset))
	if !present {
		t.Fatalf("asset registry section not present")
	}

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}
	trailer, _, err := parseAssetRegistryImportedClassesTrailer(sectionBytes, order)
	if err != nil {
		t.Fatalf("parse imported classes trailer: %v", err)
	}
	return trailer
}
