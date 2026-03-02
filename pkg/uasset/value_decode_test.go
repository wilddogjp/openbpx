package uasset

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestDecodePropertyValueEnumProperty(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "EnumProperty"},
		{Value: "EMyEnum"},
		{Value: "ValueA"},
	}
	raw := make([]byte, 8)
	binary.LittleEndian.PutUint32(raw[0:4], 3) // ValueA
	binary.LittleEndian.PutUint32(raw[4:8], 0)

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 1},
			{Name: NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        8,
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected enum decode")
	}
	m := val.(map[string]any)
	if got := m["enumType"]; got != "EMyEnum" {
		t.Fatalf("enumType: got %v", got)
	}
	if got := m["value"]; got != "EMyEnum::ValueA" {
		t.Fatalf("value: got %v", got)
	}
}

func TestDecodePropertyValueSoftObjectProperty(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "SoftObjectProperty"},
		{Value: "/Game/Maps/Main"},
		{Value: "Main"},
	}

	raw := make([]byte, 0, 20)
	raw = append(raw, encodeNameRef(2, 0)...)
	raw = append(raw, encodeNameRef(3, 0)...)
	raw = append(raw, encodeFStringASCII("")...)

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected soft object decode")
	}
	m := val.(map[string]any)
	if got := m["packageName"]; got != "/Game/Maps/Main" {
		t.Fatalf("packageName: got %v", got)
	}
	if got := m["assetName"]; got != "Main" {
		t.Fatalf("assetName: got %v", got)
	}
}

func TestDecodePropertyValueSoftObjectPropertyUTF8SubPath(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "SoftObjectProperty"},
		{Value: "/Game/Maps/Main"},
		{Value: "Main"},
	}

	raw := make([]byte, 0, 64)
	raw = append(raw, encodeNameRef(2, 0)...)
	raw = append(raw, encodeNameRef(3, 0)...)
	raw = append(raw, encodeUTF8String("PersistentLevel.ActorA")...)

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected soft object utf8 sub path decode")
	}
	m := val.(map[string]any)
	if got := m["subPath"]; got != "PersistentLevel.ActorA" {
		t.Fatalf("subPath: got %v", got)
	}
}

func TestDecodePropertyValueSoftObjectPropertyLegacyAssetPathName(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "SoftObjectProperty"},
		{Value: "/Game/Maps/Main.Main"},
	}

	raw := make([]byte, 0, 48)
	raw = append(raw, encodeNameRef(2, 0)...)
	raw = append(raw, encodeFStringASCII("PersistentLevel.ActorA")...)

	asset := &Asset{
		Raw:   RawAsset{Bytes: raw},
		Names: names,
		Summary: PackageSummary{
			FileVersionUE5: 1006,
		},
	}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected legacy soft object decode")
	}
	m := val.(map[string]any)
	if got := m["assetPathName"]; got != "/Game/Maps/Main.Main" {
		t.Fatalf("assetPathName: got %v", got)
	}
	if got := m["packageName"]; got != "/Game/Maps/Main" {
		t.Fatalf("packageName: got %v", got)
	}
	if got := m["assetName"]; got != "Main" {
		t.Fatalf("assetName: got %v", got)
	}
	if got := m["subPath"]; got != "PersistentLevel.ActorA" {
		t.Fatalf("subPath: got %v", got)
	}
}

func TestDecodePropertyValueBytePropertyWithEnumTypeName(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "ByteProperty"},
		{Value: "EMyByteEnum"},
		{Value: "ValueA"},
	}
	raw := encodeNameRef(3, 0)

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 1},
			{Name: NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected byte enum decode")
	}
	m := val.(map[string]any)
	if got := m["enumType"]; got != "EMyByteEnum" {
		t.Fatalf("enumType: got %v", got)
	}
	if got := m["value"]; got != "EMyByteEnum::ValueA" {
		t.Fatalf("value: got %v", got)
	}
}

func TestDecodePropertyValueSoftObjectPropertyByHeaderIndex(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "SoftObjectProperty"},
		{Value: "/Game/Test"},
		{Value: "BP_Test"},
	}

	raw := make([]byte, 0, 24)
	raw = append(raw, 0, 0, 0, 0) // soft object path list index = 0
	raw = append(raw, encodeNameRef(2, 0)...)
	raw = append(raw, encodeNameRef(3, 0)...)
	raw = append(raw, encodeFStringASCII("")...)

	asset := &Asset{
		Raw:   RawAsset{Bytes: raw},
		Names: names,
		Summary: PackageSummary{
			FileVersionUE5:        1017,
			SoftObjectPathsCount:  1,
			SoftObjectPathsOffset: 4,
		},
	}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 0},
		},
		Size:        4,
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected indexed soft object decode")
	}
	m := val.(map[string]any)
	if got := m["softObjectPathIndex"]; got != int32(0) {
		t.Fatalf("softObjectPathIndex: got %v", got)
	}
	if got := m["packageName"]; got != "/Game/Test" {
		t.Fatalf("packageName: got %v", got)
	}
	if got := m["assetName"]; got != "BP_Test" {
		t.Fatalf("assetName: got %v", got)
	}
}

func TestDecodePropertyValueOptionalPropertyBool(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "OptionalProperty"},
		{Value: "BoolProperty"},
	}
	raw := []byte{1, 0, 0, 0, 1} // isSet=true + bool value=true

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 1},
			{Name: NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected optional property decode")
	}
	m := val.(map[string]any)
	if got := m["optionalType"]; got != "BoolProperty" {
		t.Fatalf("optionalType: got %v", got)
	}
	if got := m["isSet"]; got != true {
		t.Fatalf("isSet: got %v", got)
	}
	value := m["value"].(map[string]any)
	if got := value["type"]; got != "BoolProperty" {
		t.Fatalf("inner type: got %v", got)
	}
	if got := value["value"]; got != true {
		t.Fatalf("inner value: got %v", got)
	}
}

func TestDecodePropertyValueStructVector(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "StructProperty"},
		{Value: "Vector"},
	}
	raw := make([]byte, 24)
	binary.LittleEndian.PutUint64(raw[0:8], math.Float64bits(1.0))
	binary.LittleEndian.PutUint64(raw[8:16], math.Float64bits(2.0))
	binary.LittleEndian.PutUint64(raw[16:24], math.Float64bits(3.0))

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 1},
			{Name: NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        24,
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected struct vector decode")
	}
	m := val.(map[string]any)
	if got := m["structType"]; got != "Vector" {
		t.Fatalf("structType: got %v", got)
	}
	vec := m["value"].(map[string]any)
	if got := vec["x"]; got != 1.0 {
		t.Fatalf("x: got %v", got)
	}
	if got := vec["y"]; got != 2.0 {
		t.Fatalf("y: got %v", got)
	}
	if got := vec["z"]; got != 3.0 {
		t.Fatalf("z: got %v", got)
	}
}

func TestDecodePropertyValueStructRotator(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "StructProperty"},
		{Value: "Rotator"},
	}
	raw := make([]byte, 24)
	binary.LittleEndian.PutUint64(raw[0:8], math.Float64bits(10.0))   // pitch
	binary.LittleEndian.PutUint64(raw[8:16], math.Float64bits(20.0))  // yaw
	binary.LittleEndian.PutUint64(raw[16:24], math.Float64bits(30.0)) // roll

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 1},
			{Name: NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        24,
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected struct rotator decode")
	}
	m := val.(map[string]any)
	if got := m["structType"]; got != "Rotator" {
		t.Fatalf("structType: got %v", got)
	}
	rot := m["value"].(map[string]any)
	if got := rot["pitch"]; got != 10.0 {
		t.Fatalf("pitch: got %v", got)
	}
	if got := rot["yaw"]; got != 20.0 {
		t.Fatalf("yaw: got %v", got)
	}
	if got := rot["roll"]; got != 30.0 {
		t.Fatalf("roll: got %v", got)
	}
}

func TestDecodePropertyValueArrayIntProperty(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "ArrayProperty"},
		{Value: "IntProperty"},
	}
	raw := make([]byte, 12)
	binary.LittleEndian.PutUint32(raw[0:4], 2) // count
	binary.LittleEndian.PutUint32(raw[4:8], 10)
	binary.LittleEndian.PutUint32(raw[8:12], 20)

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 1},
			{Name: NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected array decode")
	}
	m := val.(map[string]any)
	if got := m["arrayType"]; got != "IntProperty" {
		t.Fatalf("arrayType: got %v", got)
	}
	values := m["value"].([]any)
	if len(values) != 2 {
		t.Fatalf("array len: got %d", len(values))
	}
	first := values[0].(map[string]any)
	if got := first["type"]; got != "IntProperty" {
		t.Fatalf("first type: got %v", got)
	}
	if got := first["value"]; got != int32(10) {
		t.Fatalf("first value: got %v", got)
	}
}

func TestDecodePropertyValueArrayTaggedStructFallbackEditedDocumentInfo(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "ArrayProperty"},
		{Value: "StructProperty"},
		{Value: "EditedDocumentInfo"},
		{Value: "EditedObjectPath"},
		{Value: "StrProperty"},
	}

	element := make([]byte, 0, 128)
	path := encodeFStringASCII("EventGraph")
	element = append(element, encodeTaggedProperty(4, [][2]int32{{5, 0}}, int32(len(path)), 0, path)...)
	element = append(element, encodeNameRef(0, 0)...)

	raw := make([]byte, 0, 4+len(element))
	raw = append(raw, encodeInt32(1)...)
	raw = append(raw, element...)

	asset := &Asset{
		Raw:   RawAsset{Bytes: raw},
		Names: names,
		Summary: PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 1},
			{Name: NameRef{Index: 2}, InnerCount: 1},
			{Name: NameRef{Index: 3}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}

	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected array struct decode")
	}
	out := val.(map[string]any)
	if got := out["arrayType"]; got != "StructProperty" {
		t.Fatalf("arrayType: got %v", got)
	}
	if containsRawBase64(out) {
		t.Fatalf("expected no rawBase64 fallback for EditedDocumentInfo")
	}
	items := out["value"].([]any)
	if len(items) != 1 {
		t.Fatalf("array len: got %d", len(items))
	}
	first := items[0].(map[string]any)
	structValue := first["value"].(map[string]any)
	if got := structValue["structType"]; got != "EditedDocumentInfo" {
		t.Fatalf("structType: got %v", got)
	}
	fields := structValue["value"].(map[string]any)
	editedPath := fields["EditedObjectPath"].(map[string]any)
	if got := editedPath["type"]; got != "StrProperty" {
		t.Fatalf("EditedObjectPath type: got %v", got)
	}
	if got := editedPath["value"]; got != "EventGraph" {
		t.Fatalf("EditedObjectPath value: got %v", got)
	}
}

func TestDecodePropertyValueMapTaggedStructValueFallbackTimelineFloatTrack(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "MapProperty"},
		{Value: "NameProperty"},
		{Value: "StructProperty"},
		{Value: "TimelineFloatTrack"},
		{Value: "TrackName"},
		{Value: "TrackA"},
	}

	structValue := make([]byte, 0, 96)
	structValue = append(structValue, encodeTaggedProperty(5, [][2]int32{{2, 0}}, 8, 0, encodeNameRef(6, 0))...)
	structValue = append(structValue, encodeNameRef(0, 0)...)

	raw := make([]byte, 0, 32+len(structValue))
	raw = append(raw, encodeInt32(-1)...) // replace-map mode
	raw = append(raw, encodeInt32(1)...)  // one entry
	raw = append(raw, encodeNameRef(6, 0)...)
	raw = append(raw, structValue...)

	asset := &Asset{
		Raw:   RawAsset{Bytes: raw},
		Names: names,
		Summary: PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 2},
			{Name: NameRef{Index: 2}, InnerCount: 0},
			{Name: NameRef{Index: 3}, InnerCount: 1},
			{Name: NameRef{Index: 4}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}

	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected map struct value decode")
	}
	out := val.(map[string]any)
	if containsRawBase64(out) {
		t.Fatalf("expected no rawBase64 fallback for TimelineFloatTrack map value")
	}
	entries := out["value"].([]map[string]any)
	if len(entries) != 1 {
		t.Fatalf("entry len: got %d", len(entries))
	}
	entry := entries[0]
	mapValue := entry["value"].(map[string]any)
	if got := mapValue["type"]; got != "StructProperty" {
		t.Fatalf("value type: got %v", got)
	}
	structValueOut := mapValue["value"].(map[string]any)
	if got := structValueOut["structType"]; got != "TimelineFloatTrack" {
		t.Fatalf("structType: got %v", got)
	}
	fields := structValueOut["value"].(map[string]any)
	trackName := fields["TrackName"].(map[string]any)
	nameValue := trackName["value"].(map[string]any)
	if got := nameValue["name"]; got != "TrackA" {
		t.Fatalf("TrackName value: got %v", got)
	}
}

func TestDecodePropertyValueArrayTaggedStructFallbackInterpCurvePointFloat(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "ArrayProperty"},
		{Value: "StructProperty"},
		{Value: "InterpCurvePointFloat"},
		{Value: "InVal"},
		{Value: "FloatProperty"},
	}

	inVal := make([]byte, 4)
	binary.LittleEndian.PutUint32(inVal, math.Float32bits(float32(1.5)))

	element := make([]byte, 0, 96)
	element = append(element, encodeTaggedProperty(4, [][2]int32{{5, 0}}, int32(len(inVal)), 0, inVal)...)
	element = append(element, encodeNameRef(0, 0)...)

	raw := make([]byte, 0, 4+len(element))
	raw = append(raw, encodeInt32(1)...)
	raw = append(raw, element...)

	asset := &Asset{
		Raw:   RawAsset{Bytes: raw},
		Names: names,
		Summary: PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 1},
			{Name: NameRef{Index: 2}, InnerCount: 1},
			{Name: NameRef{Index: 3}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}

	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected InterpCurvePointFloat array decode")
	}
	out := val.(map[string]any)
	if containsRawBase64(out) {
		t.Fatalf("expected no rawBase64 fallback for InterpCurvePointFloat")
	}
	items := out["value"].([]any)
	if len(items) != 1 {
		t.Fatalf("array len: got %d", len(items))
	}
	structValue := items[0].(map[string]any)["value"].(map[string]any)
	fields := structValue["value"].(map[string]any)
	inValField := fields["InVal"].(map[string]any)
	if got := inValField["type"]; got != "FloatProperty" {
		t.Fatalf("InVal type: got %v", got)
	}
	if got := inValField["value"]; got != float32(1.5) {
		t.Fatalf("InVal value: got %v", got)
	}
}

func TestDecodePropertyValueStructKnownTaggedEmptyPayload(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "StructProperty"},
		{Value: "RichCurve"},
	}
	raw := encodeNameRef(0, 0)

	asset := &Asset{
		Raw:   RawAsset{Bytes: raw},
		Names: names,
		Summary: PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 1},
			{Name: NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}

	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected known tagged empty struct decode")
	}
	out := val.(map[string]any)
	if _, has := out["rawBase64"]; has {
		t.Fatalf("expected empty tagged struct decode without rawBase64 fallback")
	}
	if got := out["structType"]; got != "RichCurve" {
		t.Fatalf("structType: got %v", got)
	}
	fields := out["value"].(map[string]any)
	if len(fields) != 0 {
		t.Fatalf("expected empty field map, got %d entries", len(fields))
	}
}

func TestDecodePropertyValueStructUnknownEmptyPayloadFallsBackRaw(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "StructProperty"},
		{Value: "ProjectOnlyStruct"},
	}
	raw := encodeNameRef(0, 0)

	asset := &Asset{
		Raw:   RawAsset{Bytes: raw},
		Names: names,
		Summary: PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 1},
			{Name: NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}

	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected fallback decode")
	}
	out := val.(map[string]any)
	if _, has := out["rawBase64"]; !has {
		t.Fatalf("expected rawBase64 fallback for unknown empty struct payload")
	}
}

func TestDecodePropertyValueArrayCustomStructStaysRawFallback(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "ArrayProperty"},
		{Value: "StructProperty"},
		{Value: "ProjectOnlyStruct"},
	}
	raw := make([]byte, 0, 16)
	raw = append(raw, encodeInt32(1)...)
	raw = append(raw, []byte{0xde, 0xad, 0xbe, 0xef}...)

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 1},
			{Name: NameRef{Index: 2}, InnerCount: 1},
			{Name: NameRef{Index: 3}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}

	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected fallback decode")
	}
	out := val.(map[string]any)
	if _, has := out["rawBase64"]; !has {
		t.Fatalf("expected rawBase64 fallback for unsupported project struct")
	}
}

func TestDecodePropertyValueArrayUserDefinedStructTagged(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "ArrayProperty"},
		{Value: "StructProperty"},
		{Value: "BlinkLightStruct"},
		{Value: "/Game/Test_Personal/Maps/Shimazu_TestEdit/BlinkLightStruct"},
		{Value: "57fd1b31-4624-8c57-4a7a-318e1fb89b72"},
		{Value: "BlinkInterval"},
		{Value: "FloatProperty"},
	}

	blinkInterval := make([]byte, 4)
	binary.LittleEndian.PutUint32(blinkInterval, math.Float32bits(float32(1.25)))

	element := make([]byte, 0, 96)
	element = append(element, encodeTaggedProperty(6, [][2]int32{{7, 0}}, int32(len(blinkInterval)), 0, blinkInterval)...)
	element = append(element, encodeNameRef(0, 0)...)

	raw := make([]byte, 0, 4+len(element))
	raw = append(raw, encodeInt32(1)...)
	raw = append(raw, element...)

	asset := &Asset{
		Raw:   RawAsset{Bytes: raw},
		Names: names,
		Summary: PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 1},
			{Name: NameRef{Index: 2}, InnerCount: 2},
			{Name: NameRef{Index: 3}, InnerCount: 1},
			{Name: NameRef{Index: 4}, InnerCount: 0},
			{Name: NameRef{Index: 5}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}

	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected user-defined struct array decode")
	}
	out := val.(map[string]any)
	if containsRawBase64(out) {
		t.Fatalf("expected no rawBase64 fallback for user-defined struct")
	}
	items := out["value"].([]any)
	if len(items) != 1 {
		t.Fatalf("array len: got %d", len(items))
	}
	structValue := items[0].(map[string]any)["value"].(map[string]any)
	if got := structValue["structType"]; got != "BlinkLightStruct(/Game/Test_Personal/Maps/Shimazu_TestEdit/BlinkLightStruct)" {
		t.Fatalf("structType: got %v", got)
	}
	fields := structValue["value"].(map[string]any)
	blink := fields["BlinkInterval"].(map[string]any)
	if got := blink["type"]; got != "FloatProperty" {
		t.Fatalf("BlinkInterval type: got %v", got)
	}
	if got := blink["value"]; got != float32(1.25) {
		t.Fatalf("BlinkInterval value: got %v", got)
	}
}

func TestDecodePropertyValueArrayLevelViewportInfo(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "ArrayProperty"},
		{Value: "StructProperty"},
		{Value: "LevelViewportInfo"},
		{Value: "CamPosition"},
		{Value: "Vector"},
		{Value: "CamRotation"},
		{Value: "Rotator"},
		{Value: "CamOrthoZoom"},
		{Value: "FloatProperty"},
		{Value: "CamUpdated"},
		{Value: "BoolProperty"},
	}

	camPos := make([]byte, 24)
	binary.LittleEndian.PutUint64(camPos[0:8], math.Float64bits(1.0))
	binary.LittleEndian.PutUint64(camPos[8:16], math.Float64bits(2.0))
	binary.LittleEndian.PutUint64(camPos[16:24], math.Float64bits(3.0))

	camRot := make([]byte, 24)
	binary.LittleEndian.PutUint64(camRot[0:8], math.Float64bits(10.0))
	binary.LittleEndian.PutUint64(camRot[8:16], math.Float64bits(20.0))
	binary.LittleEndian.PutUint64(camRot[16:24], math.Float64bits(30.0))

	camZoom := make([]byte, 4)
	binary.LittleEndian.PutUint32(camZoom[0:4], math.Float32bits(float32(4096.5)))

	element := make([]byte, 0, 256)
	element = append(element, encodeTaggedProperty(4, [][2]int32{{2, 1}, {5, 0}}, 24, 0, camPos)...)
	element = append(element, encodeTaggedProperty(6, [][2]int32{{2, 1}, {7, 0}}, 24, 0, camRot)...)
	element = append(element, encodeTaggedProperty(8, [][2]int32{{9, 0}}, 4, 0, camZoom)...)
	element = append(element, encodeTaggedProperty(10, [][2]int32{{11, 0}}, 0, propertyFlagBoolTrue|propertyFlagSkippedSerialize, nil)...)
	element = append(element, encodeNameRef(0, 0)...)

	tmp := &Asset{
		Raw:   RawAsset{Bytes: element},
		Names: names,
		Summary: PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	parsed := tmp.ParseTaggedPropertiesRange(0, len(element), false)
	if len(parsed.Warnings) > 0 {
		t.Fatalf("test fixture parse warnings: %v", parsed.Warnings)
	}

	raw := make([]byte, 0, 4+len(element))
	raw = append(raw, encodeInt32(1)...)
	raw = append(raw, element...)

	asset := &Asset{
		Raw:   RawAsset{Bytes: raw},
		Names: names,
		Summary: PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 1},
			{Name: NameRef{Index: 2}, InnerCount: 1},
			{Name: NameRef{Index: 3}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}

	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected LevelViewportInfo array decode")
	}

	out := val.(map[string]any)
	if got := out["arrayType"]; got != "StructProperty" {
		t.Fatalf("arrayType: got %v", got)
	}
	if _, hasRaw := out["rawBase64"]; hasRaw {
		t.Fatalf("expected structured decode, got rawBase64 fallback")
	}
	items := out["value"].([]any)
	if len(items) != 1 {
		t.Fatalf("array len: got %d", len(items))
	}
	first := items[0].(map[string]any)
	if got := first["type"]; got != "StructProperty" {
		t.Fatalf("first type: got %v", got)
	}
	structValue := first["value"].(map[string]any)
	if got := structValue["structType"]; got != "LevelViewportInfo" {
		t.Fatalf("structType: got %v", got)
	}
	fields := structValue["value"].(map[string]any)

	zoom := fields["CamOrthoZoom"].(map[string]any)
	if got := zoom["type"]; got != "FloatProperty" {
		t.Fatalf("CamOrthoZoom type: got %v", got)
	}
	if got := zoom["value"]; got != float32(4096.5) {
		t.Fatalf("CamOrthoZoom value: got %v", got)
	}

	updated := fields["CamUpdated"].(map[string]any)
	if got := updated["type"]; got != "BoolProperty" {
		t.Fatalf("CamUpdated type: got %v", got)
	}
	if got := updated["value"]; got != true {
		t.Fatalf("CamUpdated value: got %v", got)
	}

	position := fields["CamPosition"].(map[string]any)
	posValue := position["value"].(map[string]any)
	if got := posValue["structType"]; got != "Vector" {
		t.Fatalf("CamPosition structType: got %v", got)
	}
	vec := posValue["value"].(map[string]any)
	if got := vec["x"]; got != 1.0 {
		t.Fatalf("CamPosition.x: got %v", got)
	}
	if got := vec["y"]; got != 2.0 {
		t.Fatalf("CamPosition.y: got %v", got)
	}
	if got := vec["z"]; got != 3.0 {
		t.Fatalf("CamPosition.z: got %v", got)
	}
}

func TestDecodePropertyValueStructPerQualityLevelInt(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "StructProperty"},
		{Value: "PerQualityLevelInt"},
	}
	raw := make([]byte, 0, 64)
	raw = append(raw, encodeInt32(0)...)   // bCooked (UBool)
	raw = append(raw, encodeInt32(100)...) // Default
	raw = append(raw, encodeInt32(2)...)   // map count
	raw = append(raw, encodeInt32(0)...)   // key 0
	raw = append(raw, encodeInt32(100)...) // value 100
	raw = append(raw, encodeInt32(3)...)   // key 3
	raw = append(raw, encodeInt32(50)...)  // value 50

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 1},
			{Name: NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected per-quality int decode")
	}
	m := val.(map[string]any)
	if got := m["structType"]; got != "PerQualityLevelInt" {
		t.Fatalf("structType: got %v", got)
	}
	fields := m["value"].(map[string]any)
	defaultField := fields["Default"].(map[string]any)
	if got := defaultField["type"]; got != "IntProperty" {
		t.Fatalf("Default type: got %v", got)
	}
	if got := defaultField["value"]; got != int32(100) {
		t.Fatalf("Default value: got %v", got)
	}
	perQuality := fields["PerQuality"].(map[string]any)
	perQualityValue := perQuality["value"].(map[string]any)
	entries := perQualityValue["value"].([]map[string]any)
	if len(entries) != 2 {
		t.Fatalf("perQuality entries: got %d", len(entries))
	}
}

func TestDecodePropertyValueStructUniqueNetIdRepl(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "StructProperty"},
		{Value: "UniqueNetIdRepl"},
		{Value: "EOS"},
	}
	raw := make([]byte, 0, 64)
	raw = append(raw, encodeInt32(10)...)     // size
	raw = append(raw, encodeNameRef(3, 0)...) // type
	raw = append(raw, encodeFStringASCII("ABCD1234")...)

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 1},
			{Name: NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected UniqueNetIdRepl decode")
	}
	m := val.(map[string]any)
	if got := m["structType"]; got != "UniqueNetIdRepl" {
		t.Fatalf("structType: got %v", got)
	}
	fields := m["value"].(map[string]any)
	sizeField := fields["Size"].(map[string]any)
	if got := sizeField["value"]; got != int32(10) {
		t.Fatalf("Size value: got %v", got)
	}
	typeField := fields["Type"].(map[string]any)["value"].(map[string]any)
	if got := typeField["name"]; got != "EOS" {
		t.Fatalf("Type name: got %v", got)
	}
	contents := fields["Contents"].(map[string]any)
	if got := contents["value"]; got != "ABCD1234" {
		t.Fatalf("Contents value: got %v", got)
	}
}

func TestDecodePropertyValueStructPerPlatformInt(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "StructProperty"},
		{Value: "PerPlatformInt"},
		{Value: "Windows"},
		{Value: "PS5"},
	}
	raw := make([]byte, 0, 64)
	raw = append(raw, encodeUBool(false)...)
	raw = append(raw, encodeInt32(60)...)
	raw = append(raw, encodeInt32(2)...)
	raw = append(raw, encodeNameRef(3, 0)...)
	raw = append(raw, encodeInt32(60)...)
	raw = append(raw, encodeNameRef(4, 0)...)
	raw = append(raw, encodeInt32(30)...)

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 1},
			{Name: NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected PerPlatformInt decode")
	}
	m := val.(map[string]any)
	if got := m["structType"]; got != "PerPlatformInt" {
		t.Fatalf("structType: got %v", got)
	}
	fields := m["value"].(map[string]any)
	defaultField := fields["Default"].(map[string]any)
	if got := defaultField["value"]; got != int32(60) {
		t.Fatalf("Default value: got %v", got)
	}
	perPlatform := fields["PerPlatform"].(map[string]any)["value"].(map[string]any)
	entries := perPlatform["value"].([]map[string]any)
	if len(entries) != 2 {
		t.Fatalf("per-platform entries: got %d", len(entries))
	}
	firstKey := entries[0]["key"].(map[string]any)["value"].(map[string]any)
	if got := firstKey["name"]; got != "Windows" {
		t.Fatalf("first key name: got %v", got)
	}
}

func TestDecodePropertyValueStructPerPlatformFloat(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "StructProperty"},
		{Value: "PerPlatformFloat"},
		{Value: "Windows"},
	}
	raw := make([]byte, 0, 48)
	raw = append(raw, encodeUBool(false)...)
	raw = append(raw, encodeInt32(int32(math.Float32bits(1.5)))...)
	raw = append(raw, encodeInt32(1)...)
	raw = append(raw, encodeNameRef(3, 0)...)
	raw = append(raw, encodeInt32(int32(math.Float32bits(1.25)))...)

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 1},
			{Name: NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected PerPlatformFloat decode")
	}
	m := val.(map[string]any)
	fields := m["value"].(map[string]any)
	defaultField := fields["Default"].(map[string]any)
	if got := defaultField["type"]; got != "FloatProperty" {
		t.Fatalf("Default type: got %v", got)
	}
	if got := defaultField["value"]; got != float32(1.5) {
		t.Fatalf("Default value: got %v", got)
	}
}

func TestDecodePropertyValueStructPerPlatformFrameRate(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "StructProperty"},
		{Value: "PerPlatformFrameRate"},
		{Value: "PAL"},
	}
	raw := make([]byte, 0, 64)
	raw = append(raw, encodeUBool(false)...)
	raw = append(raw, encodeInt32(60)...)
	raw = append(raw, encodeInt32(1)...)
	raw = append(raw, encodeInt32(1)...)
	raw = append(raw, encodeNameRef(3, 0)...)
	raw = append(raw, encodeInt32(25)...)
	raw = append(raw, encodeInt32(1)...)

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 1},
			{Name: NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected PerPlatformFrameRate decode")
	}
	m := val.(map[string]any)
	fields := m["value"].(map[string]any)
	defaultField := fields["Default"].(map[string]any)
	if got := defaultField["type"]; got != "StructProperty(FrameRate)" {
		t.Fatalf("Default type: got %v", got)
	}
	defaultValue := defaultField["value"].(map[string]any)["value"].(map[string]any)
	if got := defaultValue["Numerator"]; got != int32(60) {
		t.Fatalf("Numerator: got %v", got)
	}
	if got := defaultValue["Denominator"]; got != int32(1) {
		t.Fatalf("Denominator: got %v", got)
	}
}

func TestDecodePropertyValueStructRemoteObjectReference(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "StructProperty"},
		{Value: "RemoteObjectReference"},
	}
	raw := make([]byte, 0, 16)
	raw = append(raw, encodeInt64(int64(0x0102030405060708))...)
	raw = append(raw, encodeInt32(42)...)

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 1},
			{Name: NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected RemoteObjectReference decode")
	}
	m := val.(map[string]any)
	fields := m["value"].(map[string]any)
	objectID := fields["ObjectId"].(map[string]any)
	if got := objectID["value"]; got != uint64(0x0102030405060708) {
		t.Fatalf("ObjectId: got %v", got)
	}
	serverID := fields["ServerId"].(map[string]any)
	if got := serverID["value"]; got != uint32(42) {
		t.Fatalf("ServerId: got %v", got)
	}
}

func TestDecodePropertyValueStructAnimationAttributeIdentifier(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "StructProperty"},
		{Value: "AnimationAttributeIdentifier"},
		{Value: "AttrA"},
		{Value: "Root"},
		{Value: "/Script/CoreUObject"},
		{Value: "FrameNumber"},
	}
	raw := make([]byte, 0, 96)
	raw = append(raw, encodeNameRef(3, 0)...)
	raw = append(raw, encodeNameRef(4, 0)...)
	raw = append(raw, encodeInt32(7)...)
	raw = append(raw, encodeNameRef(5, 0)...)
	raw = append(raw, encodeNameRef(6, 0)...)
	raw = append(raw, encodeFStringASCII("")...)

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 1},
			{Name: NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected AnimationAttributeIdentifier decode")
	}
	m := val.(map[string]any)
	fields := m["value"].(map[string]any)
	nameField := fields["Name"].(map[string]any)["value"].(map[string]any)
	if got := nameField["name"]; got != "AttrA" {
		t.Fatalf("Name: got %v", got)
	}
	boneIndex := fields["BoneIndex"].(map[string]any)
	if got := boneIndex["value"]; got != int32(7) {
		t.Fatalf("BoneIndex: got %v", got)
	}
	pathField := fields["ScriptStructPath"].(map[string]any)["value"].(map[string]any)
	if got := pathField["packageName"]; got != "/Script/CoreUObject" {
		t.Fatalf("ScriptStructPath.packageName: got %v", got)
	}
	if got := pathField["assetName"]; got != "FrameNumber" {
		t.Fatalf("ScriptStructPath.assetName: got %v", got)
	}
}

func TestDecodePropertyValueArrayIntPropertyOverridableDelta(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "ArrayProperty"},
		{Value: "IntProperty"},
	}
	raw := make([]byte, 0, 32)
	raw = append(raw, encodeInt32(-1)...) // NumReplaced = INDEX_NONE
	raw = append(raw, encodeInt32(1)...)  // Removed count
	raw = append(raw, encodeInt32(10)...)
	raw = append(raw, encodeInt32(0)...) // Modified count
	raw = append(raw, encodeInt32(0)...) // Shadowed count (UE5.6+)
	raw = append(raw, encodeInt32(1)...) // Added count
	raw = append(raw, encodeInt32(20)...)

	asset := &Asset{
		Raw:   RawAsset{Bytes: raw},
		Names: names,
		Summary: PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 1},
			{Name: NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected overridable array decode")
	}
	m := val.(map[string]any)
	if got := m["overridable"]; got != true {
		t.Fatalf("overridable: got %v", got)
	}
	removed := m["removed"].([]any)
	if len(removed) != 1 {
		t.Fatalf("removed len: got %d", len(removed))
	}
	added := m["added"].([]any)
	if len(added) != 1 {
		t.Fatalf("added len: got %d", len(added))
	}
}

func TestDecodePropertyValueMapIntToIntReplaceMode(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "MapProperty"},
		{Value: "IntProperty"},
	}
	raw := make([]byte, 0, 24)
	raw = append(raw, encodeInt32(-1)...) // KeysToRemove = INDEX_NONE (replace map)
	raw = append(raw, encodeInt32(2)...)  // NumEntries
	raw = append(raw, encodeInt32(1)...)
	raw = append(raw, encodeInt32(10)...)
	raw = append(raw, encodeInt32(2)...)
	raw = append(raw, encodeInt32(20)...)

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 2},
			{Name: NameRef{Index: 2}, InnerCount: 0},
			{Name: NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected replace-map decode")
	}
	m := val.(map[string]any)
	if got := m["replaceMap"]; got != true {
		t.Fatalf("replaceMap: got %v", got)
	}
	values := m["value"].([]map[string]any)
	if len(values) != 2 {
		t.Fatalf("value len: got %d", len(values))
	}
}

func TestDecodePropertyValueMapIntToIntOverridableDelta(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "MapProperty"},
		{Value: "IntProperty"},
	}
	raw := make([]byte, 0, 64)
	raw = append(raw, encodeInt32(-1)...)  // Replaced = INDEX_NONE (delta mode)
	raw = append(raw, encodeInt32(1)...)   // Removed count
	raw = append(raw, encodeInt32(10)...)  // Removed key
	raw = append(raw, encodeInt32(1)...)   // Modified count
	raw = append(raw, encodeInt32(20)...)  // Modified key
	raw = append(raw, encodeInt32(200)...) // Modified value
	raw = append(raw, encodeInt32(1)...)   // Shadowed count (UE5.6+)
	raw = append(raw, encodeInt32(30)...)  // Shadowed key
	raw = append(raw, encodeInt32(300)...) // Shadowed value
	raw = append(raw, encodeInt32(1)...)   // Added count
	raw = append(raw, encodeInt32(40)...)  // Added key
	raw = append(raw, encodeInt32(400)...) // Added value

	asset := &Asset{
		Raw:   RawAsset{Bytes: raw},
		Names: names,
		Summary: PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 2},
			{Name: NameRef{Index: 2}, InnerCount: 0},
			{Name: NameRef{Index: 2}, InnerCount: 0},
		},
		Size:                         int32(len(raw)),
		ValueOffset:                  0,
		ExperimentalOverridableLogic: true,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected overridable-map decode")
	}
	m := val.(map[string]any)
	if got := m["overridable"]; got != true {
		t.Fatalf("overridable: got %v", got)
	}
	if got := m["replaceMap"]; got != false {
		t.Fatalf("replaceMap: got %v", got)
	}
	if got := len(m["removed"].([]any)); got != 1 {
		t.Fatalf("removed len: got %d", got)
	}
	if got := len(m["modified"].([]map[string]any)); got != 1 {
		t.Fatalf("modified len: got %d", got)
	}
	if got := len(m["shadowed"].([]map[string]any)); got != 1 {
		t.Fatalf("shadowed len: got %d", got)
	}
	if got := len(m["added"].([]map[string]any)); got != 1 {
		t.Fatalf("added len: got %d", got)
	}
}

func TestDecodePropertyValueStructNiagaraVariableBase(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "StructProperty"},
		{Value: "NiagaraVariableBase"},
		{Value: "ClassStructOrEnum"},
		{Value: "ObjectProperty"},
		{Value: "UnderlyingType"},
		{Value: "UInt16Property"},
		{Value: "Flags"},
		{Value: "ByteProperty"},
		{Value: "User.Scale"},
	}

	underlyingType := make([]byte, 2)
	binary.LittleEndian.PutUint16(underlyingType, uint16(2))

	raw := make([]byte, 0, 128)
	raw = append(raw, encodeNameRef(9, 0)...)
	raw = append(raw, encodeTaggedProperty(3, [][2]int32{{4, 0}}, 4, 0, encodeInt32(-1))...)
	raw = append(raw, encodeTaggedProperty(5, [][2]int32{{6, 0}}, 2, 0, underlyingType)...)
	raw = append(raw, encodeTaggedProperty(7, [][2]int32{{8, 0}}, 1, 0, []byte{0})...)
	raw = append(raw, encodeNameRef(0, 0)...)

	asset := &Asset{
		Raw:   RawAsset{Bytes: raw},
		Names: names,
		Summary: PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 1},
			{Name: NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}

	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected NiagaraVariableBase struct decode")
	}
	out := val.(map[string]any)
	if got := out["structType"]; got != "NiagaraVariableBase" {
		t.Fatalf("structType: got %v", got)
	}
	if containsRawBase64(out) {
		t.Fatalf("expected no rawBase64 fallback for NiagaraVariableBase")
	}
	inner := out["value"].(map[string]any)
	if got := inner["name"]; got != "User.Scale" {
		t.Fatalf("name: got %v", got)
	}
	typeDef := inner["typeDefHandle"].(map[string]any)
	if got := typeDef["structType"]; got != "NiagaraTypeDefinition" {
		t.Fatalf("typeDef structType: got %v", got)
	}
}

func TestDecodePropertyValueMapNiagaraVariableBaseToNiagaraVariant(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "MapProperty"},
		{Value: "StructProperty"},
		{Value: "NiagaraVariableBase"},
		{Value: "NiagaraVariant"},
		{Value: "ClassStructOrEnum"},
		{Value: "ObjectProperty"},
		{Value: "UnderlyingType"},
		{Value: "UInt16Property"},
		{Value: "Flags"},
		{Value: "ByteProperty"},
		{Value: "Object"},
		{Value: "DataInterface"},
		{Value: "Bytes"},
		{Value: "ArrayProperty"},
		{Value: "CurrentMode"},
		{Value: "EnumProperty"},
		{Value: "ENiagaraVariantMode"},
		{Value: "IntProperty"},
		{Value: "User.Scale"},
		{Value: "Bytes"},
	}

	underlyingType := make([]byte, 2)
	binary.LittleEndian.PutUint16(underlyingType, uint16(2))

	key := make([]byte, 0, 160)
	key = append(key, encodeNameRef(19, 0)...)
	key = append(key, encodeTaggedProperty(5, [][2]int32{{6, 0}}, 4, 0, encodeInt32(-1))...)
	key = append(key, encodeTaggedProperty(7, [][2]int32{{8, 0}}, 2, 0, underlyingType)...)
	key = append(key, encodeTaggedProperty(9, [][2]int32{{10, 0}}, 1, 0, []byte{0})...)
	key = append(key, encodeNameRef(0, 0)...)

	bytesPayload := make([]byte, 0, 8)
	bytesPayload = append(bytesPayload, encodeInt32(2)...)
	bytesPayload = append(bytesPayload, 0x12, 0x34)

	value := make([]byte, 0, 192)
	value = append(value, encodeTaggedProperty(11, [][2]int32{{6, 0}}, 4, 0, encodeInt32(-1))...)
	value = append(value, encodeTaggedProperty(12, [][2]int32{{6, 0}}, 4, 0, encodeInt32(-1))...)
	value = append(value, encodeTaggedProperty(13, [][2]int32{{14, 1}, {10, 0}}, int32(len(bytesPayload)), 0, bytesPayload)...)
	value = append(value, encodeTaggedProperty(15, [][2]int32{{16, 2}, {17, 0}, {18, 0}}, 8, 0, encodeNameRef(20, 0))...)
	value = append(value, encodeNameRef(0, 0)...)

	raw := make([]byte, 0, 8+len(key)+len(value))
	raw = append(raw, encodeInt32(0)...)
	raw = append(raw, encodeInt32(1)...)
	raw = append(raw, key...)
	raw = append(raw, value...)

	asset := &Asset{
		Raw:   RawAsset{Bytes: raw},
		Names: names,
		Summary: PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 2},
			{Name: NameRef{Index: 2}, InnerCount: 1},
			{Name: NameRef{Index: 3}, InnerCount: 0},
			{Name: NameRef{Index: 2}, InnerCount: 1},
			{Name: NameRef{Index: 4}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}

	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected Niagara map decode")
	}
	out := val.(map[string]any)
	if containsRawBase64(out) {
		t.Fatalf("expected no rawBase64 fallback for Niagara map")
	}
	entries := out["value"].([]map[string]any)
	if len(entries) != 1 {
		t.Fatalf("value len: got %d", len(entries))
	}
	entry := entries[0]
	keyValue := entry["key"].(map[string]any)["value"].(map[string]any)
	if got := keyValue["structType"]; got != "NiagaraVariableBase" {
		t.Fatalf("key structType: got %v", got)
	}
	keyFields := keyValue["value"].(map[string]any)
	if got := keyFields["name"]; got != "User.Scale" {
		t.Fatalf("key name: got %v", got)
	}

	valueStruct := entry["value"].(map[string]any)["value"].(map[string]any)
	if got := valueStruct["structType"]; got != "NiagaraVariant" {
		t.Fatalf("value structType: got %v", got)
	}
	valueFields := valueStruct["value"].(map[string]any)
	currentMode := valueFields["CurrentMode"].(map[string]any)["value"].(map[string]any)
	if got := currentMode["value"]; got != "ENiagaraVariantMode::Bytes" {
		t.Fatalf("CurrentMode: got %v", got)
	}
}

func TestDecodePropertyValueUInt64PropertyPreservesUnsignedRange(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "UInt64Property"},
	}
	raw := make([]byte, 8)
	binary.LittleEndian.PutUint64(raw, math.MaxUint64)

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected uint64 decode")
	}
	got, ok := val.(uint64)
	if !ok {
		t.Fatalf("decoded type: got %T want uint64", val)
	}
	if got != math.MaxUint64 {
		t.Fatalf("decoded value: got %d want %d", got, uint64(math.MaxUint64))
	}
}

func TestDecodePropertyValueFieldPathOwnerResolution(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "FieldPathProperty"},
		{Value: "MyField"},
		{Value: "MyStruct"},
	}
	raw := make([]byte, 0, 16)
	raw = append(raw, encodeInt32(1)...)
	raw = append(raw, encodeNameRef(2, 0)...)
	raw = append(raw, encodeInt32(-1)...)

	asset := &Asset{
		Raw:   RawAsset{Bytes: raw},
		Names: names,
		Imports: []ImportEntry{
			{ObjectName: NameRef{Index: 3, Number: 0}},
		},
	}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected field path decode")
	}
	m := val.(map[string]any)
	if got := m["owner"]; got != int32(-1) {
		t.Fatalf("owner: got %v", got)
	}
	if got := m["resolvedOwner"]; got != "import:1:MyStruct" {
		t.Fatalf("resolvedOwner: got %v", got)
	}
}

func TestDecodePropertyValueLazyObjectPropertyGUIDFallback(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "LazyObjectProperty"},
	}
	raw := []byte{
		0x78, 0x56, 0x34, 0x12,
		0xbc, 0x9a, 0xf0, 0xde,
		0x11, 0x22, 0x33, 0x44,
		0x55, 0x66, 0x77, 0x88,
	}
	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected lazy object guid fallback decode")
	}
	m := val.(map[string]any)
	if got := m["guid"]; got != "12345678-9abc-def0-1122-334455667788" {
		t.Fatalf("guid: got %v", got)
	}
}

func TestDecodePropertyValueTextPropertyBase(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "TextProperty"},
	}
	raw := make([]byte, 0, 64)
	raw = append(raw, 0, 0, 0, 0) // flags
	raw = append(raw, 0)          // history type: Base
	raw = append(raw, encodeFStringASCII("")...)
	raw = append(raw, encodeFStringASCII("")...)
	raw = append(raw, encodeFStringASCII("Hello")...)

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected text decode")
	}
	m := val.(map[string]any)
	if got := m["historyType"]; got != "Base" {
		t.Fatalf("historyType: got %v", got)
	}
	if got := m["value"]; got != "Hello" {
		t.Fatalf("value: got %v", got)
	}
}

func TestDecodePropertyValueTextPropertyNoneCultureInvariant(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "TextProperty"},
	}
	raw := make([]byte, 0, 32)
	raw = append(raw, 0, 0, 0, 0) // flags
	raw = append(raw, 255)        // history type: None
	raw = append(raw, encodeUBool(true)...)
	raw = append(raw, encodeFStringASCII("Invariant")...)

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected text none decode")
	}
	m := val.(map[string]any)
	if got := m["historyType"]; got != "None" {
		t.Fatalf("historyType: got %v", got)
	}
	if got := m["hasCultureInvariantString"]; got != true {
		t.Fatalf("hasCultureInvariantString: got %v", got)
	}
	if got := m["value"]; got != "Invariant" {
		t.Fatalf("value: got %v", got)
	}
}

func TestDecodePropertyValueTextPropertyTransform(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "TextProperty"},
	}
	raw := make([]byte, 0, 96)
	raw = append(raw, 0, 0, 0, 0) // flags
	raw = append(raw, 10)         // history type: Transform
	raw = append(raw, encodeTextHistoryBase("", "", "hello")...)
	raw = append(raw, 1) // ToUpper

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected transform text decode")
	}
	m := val.(map[string]any)
	if got := m["historyType"]; got != "Transform" {
		t.Fatalf("historyType: got %v", got)
	}
	if got := m["transformType"]; got != "ToUpper" {
		t.Fatalf("transformType: got %v", got)
	}
	sourceText := m["sourceText"].(map[string]any)
	if got := sourceText["historyType"]; got != "Base" {
		t.Fatalf("sourceText historyType: got %v", got)
	}
	if got := sourceText["value"]; got != "hello" {
		t.Fatalf("sourceText value: got %v", got)
	}
}

func TestDecodePropertyValueTextPropertyStringTableEntry(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "TextProperty"},
		{Value: "UI_Table"},
	}
	raw := make([]byte, 0, 40)
	raw = append(raw, 0, 0, 0, 0) // flags
	raw = append(raw, 11)         // history type: StringTableEntry
	raw = append(raw, encodeNameRef(2, 0)...)
	raw = append(raw, encodeFStringASCII("StartButton")...)

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected string table text decode")
	}
	m := val.(map[string]any)
	if got := m["historyType"]; got != "StringTableEntry" {
		t.Fatalf("historyType: got %v", got)
	}
	if got := m["tableIdName"]; got != "UI_Table" {
		t.Fatalf("tableIdName: got %v", got)
	}
	if got := m["key"]; got != "StartButton" {
		t.Fatalf("key: got %v", got)
	}
}

func TestDecodePropertyValueTextPropertyTextGenerator(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "TextProperty"},
		{Value: "MyGenerator"},
	}
	raw := make([]byte, 0, 40)
	raw = append(raw, 0, 0, 0, 0) // flags
	raw = append(raw, 12)         // history type: TextGenerator
	raw = append(raw, encodeNameRef(2, 0)...)
	raw = append(raw, encodeInt32(3)...)
	raw = append(raw, []byte{0x01, 0x02, 0x03}...)

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected text generator decode")
	}
	m := val.(map[string]any)
	if got := m["historyType"]; got != "TextGenerator" {
		t.Fatalf("historyType: got %v", got)
	}
	if got := m["generatorTypeName"]; got != "MyGenerator" {
		t.Fatalf("generatorTypeName: got %v", got)
	}
	if got := m["generatorContentsBase64"]; got != "AQID" {
		t.Fatalf("generatorContentsBase64: got %v", got)
	}
}

func TestDecodePropertyValueTextPropertyNamedFormat(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "TextProperty"},
	}
	raw := make([]byte, 0, 160)
	raw = append(raw, 0, 0, 0, 0) // flags
	raw = append(raw, 1)          // history type: NamedFormat
	raw = append(raw, encodeTextHistoryBase("", "", "Hello {Name}")...)
	raw = append(raw, encodeInt32(1)...)             // argument count
	raw = append(raw, encodeFStringASCII("Name")...) // argument name
	raw = append(raw, 4)                             // argument type: Text
	raw = append(raw, encodeTextHistoryBase("", "", "World")...)

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected named-format text decode")
	}
	m := val.(map[string]any)
	if got := m["historyType"]; got != "NamedFormat" {
		t.Fatalf("historyType: got %v", got)
	}
	formatText := m["formatText"].(map[string]any)
	if got := formatText["value"]; got != "Hello {Name}" {
		t.Fatalf("formatText value: got %v", got)
	}
	arguments := m["arguments"].([]map[string]any)
	if len(arguments) != 1 {
		t.Fatalf("arguments len: got %d", len(arguments))
	}
	if got := arguments[0]["name"]; got != "Name" {
		t.Fatalf("argument name: got %v", got)
	}
	argValue := arguments[0]["value"].(map[string]any)
	if got := argValue["type"]; got != "Text" {
		t.Fatalf("argument type: got %v", got)
	}
	textValue := argValue["value"].(map[string]any)
	if got := textValue["value"]; got != "World" {
		t.Fatalf("argument text value: got %v", got)
	}
}

func TestDecodePropertyValueTextPropertyAsNumber(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "TextProperty"},
	}
	raw := make([]byte, 0, 120)
	raw = append(raw, 0, 0, 0, 0) // flags
	raw = append(raw, 4)          // history type: AsNumber
	raw = append(raw, 0)          // source value type: Int
	raw = append(raw, encodeInt64(42)...)
	raw = append(raw, encodeUBool(true)...) // has format options
	raw = append(raw, encodeUBool(true)...) // always sign
	raw = append(raw, encodeUBool(false)...)
	raw = append(raw, 1) // HalfFromZero
	raw = append(raw, encodeInt32(1)...)
	raw = append(raw, encodeInt32(10)...)
	raw = append(raw, encodeInt32(0)...)
	raw = append(raw, encodeInt32(2)...)
	raw = append(raw, encodeFStringASCII("ja-JP")...)

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected as-number text decode")
	}
	m := val.(map[string]any)
	if got := m["historyType"]; got != "AsNumber" {
		t.Fatalf("historyType: got %v", got)
	}
	sourceValue := m["sourceValue"].(map[string]any)
	if got := sourceValue["type"]; got != "Int" {
		t.Fatalf("sourceValue type: got %v", got)
	}
	if got := sourceValue["value"]; got != int64(42) {
		t.Fatalf("sourceValue value: got %v", got)
	}
	if got := m["targetCulture"]; got != "ja-JP" {
		t.Fatalf("targetCulture: got %v", got)
	}
	options := m["formatOptions"].(map[string]any)
	if got := options["alwaysSign"]; got != true {
		t.Fatalf("alwaysSign: got %v", got)
	}
	if got := options["useGrouping"]; got != false {
		t.Fatalf("useGrouping: got %v", got)
	}
}

func TestDecodePropertyValueTextPropertyAsDateTimeCustom(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "TextProperty"},
	}
	raw := make([]byte, 0, 120)
	raw = append(raw, 0, 0, 0, 0) // flags
	raw = append(raw, 9)          // history type: AsDateTime
	raw = append(raw, encodeInt64(123456789)...)
	raw = append(raw, 5) // date style: Custom
	raw = append(raw, 5) // time style: Custom
	raw = append(raw, encodeFStringASCII("yyyy/MM/dd HH:mm")...)
	raw = append(raw, encodeFStringASCII("UTC")...)
	raw = append(raw, encodeFStringASCII("en-US")...)

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	val, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected as-datetime text decode")
	}
	m := val.(map[string]any)
	if got := m["historyType"]; got != "AsDateTime" {
		t.Fatalf("historyType: got %v", got)
	}
	if got := m["dateStyle"]; got != "Custom" {
		t.Fatalf("dateStyle: got %v", got)
	}
	if got := m["customPattern"]; got != "yyyy/MM/dd HH:mm" {
		t.Fatalf("customPattern: got %v", got)
	}
	if got := m["timeZone"]; got != "UTC" {
		t.Fatalf("timeZone: got %v", got)
	}
}

func TestDecodePropertyValueSkippedSerializeNonBoolIsRejected(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "IntProperty"},
	}
	raw := make([]byte, 4)
	binary.LittleEndian.PutUint32(raw, 123)

	asset := &Asset{Raw: RawAsset{Bytes: raw}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
		Flags:       propertyFlagSkippedSerialize,
	}
	if _, ok := asset.DecodePropertyValue(tag); ok {
		t.Fatalf("expected skipped-serialize int property decode to be rejected")
	}
}

func TestDecodePropertyValueSkippedSerializeBoolUsesFlag(t *testing.T) {
	names := []NameEntry{
		{Value: "None"},
		{Value: "BoolProperty"},
	}
	asset := &Asset{Raw: RawAsset{Bytes: []byte{}}, Names: names}
	tag := PropertyTag{
		TypeNodes: []PropertyTypeNode{
			{Name: NameRef{Index: 1}, InnerCount: 0},
		},
		Size:        0,
		ValueOffset: 0,
		Flags:       propertyFlagSkippedSerialize | propertyFlagBoolTrue,
	}
	v, ok := asset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("expected skipped-serialize bool property decode")
	}
	if got, castOK := v.(bool); !castOK || !got {
		t.Fatalf("bool value: got %#v", v)
	}
}

func encodeTaggedProperty(nameIndex int32, typeNodes [][2]int32, size int32, flags uint8, value []byte) []byte {
	out := make([]byte, 0, 64+len(value))
	out = append(out, encodeNameRef(nameIndex, 0)...)
	for _, node := range typeNodes {
		out = append(out, encodeNameRef(node[0], 0)...)
		out = append(out, encodeInt32(node[1])...)
	}
	out = append(out, encodeInt32(size)...)
	out = append(out, flags)
	out = append(out, value...)
	return out
}

func encodeNameRef(index, number int32) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(index))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(number))
	return buf
}

func encodeFStringASCII(s string) []byte {
	if s == "" {
		return []byte{0, 0, 0, 0}
	}
	out := make([]byte, 4+len(s)+1)
	binary.LittleEndian.PutUint32(out[0:4], uint32(len(s)+1))
	copy(out[4:], []byte(s))
	out[len(out)-1] = 0
	return out
}

func encodeUTF8String(s string) []byte {
	if s == "" {
		return []byte{0, 0, 0, 0}
	}
	out := make([]byte, 4+len(s))
	binary.LittleEndian.PutUint32(out[0:4], uint32(len(s)))
	copy(out[4:], []byte(s))
	return out
}

func encodeInt32(v int32) []byte {
	out := make([]byte, 4)
	binary.LittleEndian.PutUint32(out[0:4], uint32(v))
	return out
}

func encodeInt64(v int64) []byte {
	out := make([]byte, 8)
	binary.LittleEndian.PutUint64(out[0:8], uint64(v))
	return out
}

func encodeUBool(v bool) []byte {
	if v {
		return []byte{1, 0, 0, 0}
	}
	return []byte{0, 0, 0, 0}
}

func encodeTextHistoryBase(namespace, key, value string) []byte {
	raw := make([]byte, 0, 32+len(namespace)+len(key)+len(value))
	raw = append(raw, 0, 0, 0, 0) // flags
	raw = append(raw, 0)          // history type: Base
	raw = append(raw, encodeFStringASCII(namespace)...)
	raw = append(raw, encodeFStringASCII(key)...)
	raw = append(raw, encodeFStringASCII(value)...)
	return raw
}

func containsRawBase64(v any) bool {
	switch t := v.(type) {
	case map[string]any:
		if raw, ok := t["rawBase64"].(string); ok && raw != "" {
			return true
		}
		for _, child := range t {
			if containsRawBase64(child) {
				return true
			}
		}
	case []any:
		for _, child := range t {
			if containsRawBase64(child) {
				return true
			}
		}
	case []map[string]any:
		for _, child := range t {
			if containsRawBase64(child) {
				return true
			}
		}
	}
	return false
}
