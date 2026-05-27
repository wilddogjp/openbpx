package cli

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

func TestRunImportAddUpdatesImportedClassesTrailer(t *testing.T) {
	beforePath := findWidgetWriteBrushImageBeforeFixture(t)
	if beforePath == "" {
		t.Skip("widget_write_brush_image fixture not found")
	}

	tempFile := filepath.Join(t.TempDir(), "work.uasset")
	data, err := os.ReadFile(beforePath)
	if err != nil {
		t.Fatalf("read before fixture: %v", err)
	}
	if err := os.WriteFile(tempFile, data, 0o644); err != nil {
		t.Fatalf("write temp fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"import", "add", tempFile,
		"--texture", "/Game/Effects/Textures/Decals/chippedcracks",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(tempFile, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	if got, want := len(asset.Imports), 21; got != want {
		t.Fatalf("import count: got %d want %d", got, want)
	}
	importCount, flags := parseImportedClassesTrailerForTest(t, asset)
	if got, want := importCount, uint32(21); got != want {
		t.Fatalf("trailer import count: got %d want %d", got, want)
	}
	if len(flags) != 1 || flags[0] != 0x64af4 {
		t.Fatalf("trailer flags: got %#x want %#x", flags, []uint32{0x64af4})
	}
}

func TestRunBlueprintWidgetWriteBrushImageNormalizesDesignerWidgetTreeFlags(t *testing.T) {
	beforePath := findWidgetWriteBrushImageBeforeFixture(t)
	if beforePath == "" {
		t.Skip("widget_write_brush_image fixture not found")
	}

	tempFile := filepath.Join(t.TempDir(), "work.uasset")
	data, err := os.ReadFile(beforePath)
	if err != nil {
		t.Fatalf("read before fixture: %v", err)
	}
	if err := os.WriteFile(tempFile, data, 0o644); err != nil {
		t.Fatalf("write temp fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-write", tempFile,
		"--widget", "Image_22",
		"--property", "brush-image",
		"--value", "/Game/Effects/Textures/Decals/chippedcracks",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(tempFile, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	if got, want := asset.Exports[9].ObjectFlags, uint32(262153); got != want {
		t.Fatalf("designer WidgetTree objectFlags: got %d want %d", got, want)
	}
	if got, want := asset.Exports[10].ObjectFlags, uint32(8); got != want {
		t.Fatalf("generated WidgetTree objectFlags: got %d want %d", got, want)
	}
}

func TestRunBlueprintWidgetWriteBrushImageUpdatesExistingBrush(t *testing.T) {
	beforePath := findWidgetWriteBrushImageBeforeFixture(t)
	if beforePath == "" {
		t.Skip("widget_write_brush_image fixture not found")
	}

	tempFile := filepath.Join(t.TempDir(), "work.uasset")
	data, err := os.ReadFile(beforePath)
	if err != nil {
		t.Fatalf("read before fixture: %v", err)
	}
	if err := os.WriteFile(tempFile, data, 0o644); err != nil {
		t.Fatalf("write temp fixture: %v", err)
	}

	runWidgetWrite := func(texturePath string) {
		t.Helper()

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := Run([]string{
			"blueprint", "widget-write", tempFile,
			"--widget", "Image_22",
			"--property", "brush-image",
			"--value", texturePath,
		}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("widget-write %q exit code: got %d want 0 stderr=%s", texturePath, code, stderr.String())
		}
	}

	runWidgetWrite("/Game/Effects/Textures/Decals/chippedcracks")
	runWidgetWrite("/Game/Effects/Textures/Decals/concrete_normal2x2")

	asset, err := uasset.ParseFile(tempFile, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	if got, want := len(asset.Imports), 23; got != want {
		t.Fatalf("import count after second brush-image write: got %d want %d", got, want)
	}

	textureImportIdx := int32(0)
	for i, imp := range asset.Imports {
		if !textureImportIsSupported(asset, imp) {
			continue
		}
		if strings.EqualFold(imp.ObjectName.Display(asset.Names), "concrete_normal2x2") {
			textureImportIdx = int32(-(i + 1))
			break
		}
	}
	if textureImportIdx == 0 {
		t.Fatalf("concrete_normal2x2 texture import not found")
	}

	for _, exportIndex := range []int{1, 2} {
		if got := brushResourceObjectIndexForTest(t, asset, exportIndex); got != textureImportIdx {
			t.Fatalf("export %d Brush.ResourceObject index: got %d want %d", exportIndex+1, got, textureImportIdx)
		}
	}
}

func findWidgetWriteBrushImageBeforeFixture(t *testing.T) string {
	t.Helper()

	for _, root := range goldenFixtureRoots(t, "operations") {
		path := filepath.Join(root, "operations", "widget_write_brush_image", "before.uasset")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func parseImportedClassesTrailerForTest(t *testing.T, asset *uasset.Asset) (uint32, []uint32) {
	t.Helper()

	sectionBytes, _, _, present := sectionByOffset(asset, int64(asset.Summary.AssetRegistryDataOffset))
	if !present {
		t.Fatalf("asset registry section not present")
	}

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}
	for wordCount := 0; ; wordCount++ {
		trailerLen := 17 + wordCount*4
		if trailerLen > len(sectionBytes) {
			break
		}
		start := len(sectionBytes) - trailerLen
		if sectionBytes[start] != 0 {
			continue
		}
		importCount := order.Uint32(sectionBytes[start+1 : start+5])
		expectedWords := 0
		if importCount > 0 {
			expectedWords = int((importCount + 31) / 32)
		}
		if expectedWords != wordCount {
			continue
		}
		tailPos := start + 5 + wordCount*4
		if tailPos+12 != len(sectionBytes) {
			continue
		}
		if order.Uint32(sectionBytes[tailPos:tailPos+4]) != 1 ||
			order.Uint32(sectionBytes[tailPos+4:tailPos+8]) != 1 ||
			order.Uint32(sectionBytes[tailPos+8:tailPos+12]) != 0 {
			continue
		}

		flags := make([]uint32, wordCount)
		for i := 0; i < wordCount; i++ {
			flagPos := start + 5 + i*4
			flags[i] = order.Uint32(sectionBytes[flagPos : flagPos+4])
		}
		return importCount, flags
	}

	t.Fatalf("imported classes trailer not found")
	return 0, nil
}

func brushResourceObjectIndexForTest(t *testing.T, asset *uasset.Asset, exportIndex int) int32 {
	t.Helper()

	parsed := asset.ParseExportProperties(exportIndex)
	if len(parsed.Warnings) > 0 {
		t.Fatalf("parse export %d properties: %v", exportIndex+1, parsed.Warnings)
	}
	for _, prop := range parsed.Properties {
		if !strings.EqualFold(prop.Name.Display(asset.Names), "Brush") {
			continue
		}
		decoded, ok := asset.DecodePropertyValue(prop)
		if !ok {
			t.Fatalf("decode Brush on export %d", exportIndex+1)
		}
		root, ok := decoded.(map[string]any)
		if !ok {
			t.Fatalf("Brush root shape on export %d", exportIndex+1)
		}
		fields, ok := root["value"].(map[string]any)
		if !ok {
			t.Fatalf("Brush value shape on export %d", exportIndex+1)
		}
		resourceObject, ok := fields["ResourceObject"].(map[string]any)
		if !ok {
			t.Fatalf("Brush.ResourceObject wrapper missing on export %d", exportIndex+1)
		}
		resourceValue, ok := resourceObject["value"].(map[string]any)
		if !ok {
			t.Fatalf("Brush.ResourceObject value missing on export %d", exportIndex+1)
		}
		index, ok := anyInt(resourceValue["index"])
		if !ok {
			t.Fatalf("Brush.ResourceObject index missing on export %d", exportIndex+1)
		}
		return int32(index)
	}

	t.Fatalf("Brush property missing on export %d", exportIndex+1)
	return 0
}
