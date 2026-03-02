package edit

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

func TestBuildPropertySetMutationLevelViewportInfoArrayLeaf(t *testing.T) {
	data := buildLevelViewportFixture(t, 4096.5)
	asset, err := uasset.ParseBytes(data, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	res, err := BuildPropertySetMutation(asset, 0, "EditorViews[0].CamOrthoZoom", "2048.25")
	if err != nil {
		t.Fatalf("build mutation: %v", err)
	}
	if got := res.ByteDelta; got != 0 {
		t.Fatalf("byte delta: got %d want 0", got)
	}

	outBytes, err := RewriteAsset(asset, []ExportMutation{res.Mutation})
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	outAsset, err := uasset.ParseBytes(outBytes, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten fixture: %v", err)
	}
	props := outAsset.ParseExportProperties(0)
	if len(props.Warnings) > 0 {
		t.Fatalf("parse warnings: %v", props.Warnings)
	}

	var editorViews *uasset.PropertyTag
	for i := range props.Properties {
		p := &props.Properties[i]
		if p.Name.Display(outAsset.Names) == "EditorViews" {
			editorViews = p
			break
		}
	}
	if editorViews == nil {
		t.Fatalf("EditorViews property not found")
	}

	decoded, ok := outAsset.DecodePropertyValue(*editorViews)
	if !ok {
		t.Fatalf("decode EditorViews failed")
	}

	arrayValue := decoded.(map[string]any)
	items := arrayValue["value"].([]any)
	first := items[0].(map[string]any)
	structValue := first["value"].(map[string]any)
	fields := structValue["value"].(map[string]any)
	camZoom := fields["CamOrthoZoom"].(map[string]any)
	zoomValue, ok := camZoom["value"].(float32)
	if !ok {
		t.Fatalf("CamOrthoZoom decoded type: got %T", camZoom["value"])
	}
	if diff := math.Abs(float64(zoomValue) - 2048.25); diff > 0.0001 {
		t.Fatalf("CamOrthoZoom value: got %v want 2048.25", zoomValue)
	}
}

func buildLevelViewportFixture(t *testing.T, initialZoom float32) []byte {
	t.Helper()

	names := []string{
		"None",
		"Default__BP_Test",
		"HelperExport",
		"ObjectProperty",
		"MyProp",
		"BlueprintGeneratedClass",
		"CoreUObject",
		"ArrayProperty",
		"EditorViews",
		"StructProperty",
		"LevelViewportInfo",
		"CamPosition",
		"Vector",
		"CamRotation",
		"Rotator",
		"CamOrthoZoom",
		"FloatProperty",
		"CamUpdated",
		"BoolProperty",
		"/Game/TestAsset",
	}

	nameMap := buildNameMap(t, names)
	importMap := buildImportMap(t)
	payload1 := buildLevelViewportPropertyPayload(t, 8, initialZoom)
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

	summary := buildSummary(t, summaryBuildArgs{
		NameOffset:           nameOffset,
		ImportOffset:         importOffset,
		ExportOffset:         exportOffset,
		TotalHeaderSize:      totalHeader,
		AssetRegistryOffset:  int32(serialCursor),
		BulkDataStartOffset:  serialCursor,
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

func buildLevelViewportPropertyPayload(t *testing.T, propNameIndex int32, zoom float32) []byte {
	t.Helper()

	var b bytes.Buffer
	w32 := func(v int32) { must(binary.Write(&b, binary.LittleEndian, v)) }
	w8 := func(v uint8) { must(binary.Write(&b, binary.LittleEndian, v)) }
	wname := func(index, number int32) {
		w32(index)
		w32(number)
	}

	payload := buildLevelViewportArrayPayload(t, zoom)

	w8(0) // class serialization control extension
	wname(propNameIndex, 0)
	wname(7, 0) // ArrayProperty
	w32(1)
	wname(9, 0) // StructProperty
	w32(1)
	wname(10, 0) // LevelViewportInfo
	w32(0)
	w32(int32(len(payload)))
	w8(0)
	b.Write(payload)
	wname(0, 0)
	return b.Bytes()
}

func buildLevelViewportArrayPayload(t *testing.T, zoom float32) []byte {
	t.Helper()

	var b bytes.Buffer
	w32 := func(v int32) { must(binary.Write(&b, binary.LittleEndian, v)) }
	w8 := func(v uint8) { must(binary.Write(&b, binary.LittleEndian, v)) }
	wname := func(index, number int32) {
		w32(index)
		w32(number)
	}

	writeField := func(nameIndex int32, typeNodes [][2]int32, size int32, flags uint8, value []byte) {
		wname(nameIndex, 0)
		for _, node := range typeNodes {
			wname(node[0], 0)
			w32(node[1])
		}
		w32(size)
		w8(flags)
		if len(value) > 0 {
			b.Write(value)
		}
	}

	encodeVec3 := func(x, y, z float64) []byte {
		raw := make([]byte, 24)
		binary.LittleEndian.PutUint64(raw[0:8], math.Float64bits(x))
		binary.LittleEndian.PutUint64(raw[8:16], math.Float64bits(y))
		binary.LittleEndian.PutUint64(raw[16:24], math.Float64bits(z))
		return raw
	}

	zoomRaw := make([]byte, 4)
	binary.LittleEndian.PutUint32(zoomRaw, math.Float32bits(zoom))

	w32(1) // array count
	writeField(11, [][2]int32{{9, 1}, {12, 0}}, 24, 0, encodeVec3(1, 2, 3))
	writeField(13, [][2]int32{{9, 1}, {14, 0}}, 24, 0, encodeVec3(10, 20, 30))
	writeField(15, [][2]int32{{16, 0}}, 4, 0, zoomRaw)
	writeField(17, [][2]int32{{18, 0}}, 0, propertyFlagBoolTrue|propertyFlagSkippedSerialize, nil)
	wname(0, 0)
	return b.Bytes()
}
