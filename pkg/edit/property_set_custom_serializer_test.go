package edit

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

func TestBuildPropertySetMutationCustomStructIntGolden(t *testing.T) {
	beforePath := filepath.Join("..", "..", "testdata", "golden", "operations", "prop_set_custom_struct_int", "before.uasset")
	afterPath := filepath.Join("..", "..", "testdata", "golden", "operations", "prop_set_custom_struct_int", "after.uasset")
	beforeBytes, err := os.ReadFile(beforePath)
	if err != nil {
		t.Fatalf("read before fixture: %v", err)
	}
	afterBytes, err := os.ReadFile(afterPath)
	if err != nil {
		t.Fatalf("read after fixture: %v", err)
	}

	asset, err := uasset.ParseBytes(beforeBytes, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse before fixture: %v", err)
	}
	res, err := BuildPropertySetMutation(asset, 4, "FixtureCustom.IntVal", "42")
	if err != nil {
		t.Fatalf("build mutation: %v", err)
	}
	outBytes, err := RewriteAsset(asset, []ExportMutation{res.Mutation})
	if err != nil {
		t.Fatalf("rewrite asset: %v", err)
	}

	if !equalBytesIgnoringRanges(outBytes, afterBytes, [][2]int{{24, 20}}) {
		t.Fatalf("rewritten bytes do not match golden fixture")
	}
}

func TestWriteValueByTypeStructPerQualityLevelInt(t *testing.T) {
	names := []uasset.NameEntry{
		{Value: "None"},
		{Value: "StructProperty"},
		{Value: "PerQualityLevelInt"},
	}
	asset := &uasset.Asset{
		Names: names,
		Summary: uasset.PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	node := &typeTreeNode{
		Name: "StructProperty",
		Children: []*typeTreeNode{
			{Name: "PerQualityLevelInt"},
		},
	}
	value := map[string]any{
		"structType": "PerQualityLevelInt",
		"value": map[string]any{
			"bCooked": map[string]any{
				"type":  "BoolProperty",
				"value": false,
			},
			"Default": map[string]any{
				"type":  "IntProperty",
				"value": int32(100),
			},
			"PerQuality": map[string]any{
				"type": "MapProperty(IntProperty,IntProperty)",
				"value": map[string]any{
					"keyType":   "IntProperty",
					"valueType": "IntProperty",
					"value": []map[string]any{
						{
							"key": map[string]any{"type": "IntProperty", "value": int32(0)},
							"value": map[string]any{
								"type":  "IntProperty",
								"value": int32(100),
							},
						},
						{
							"key": map[string]any{"type": "IntProperty", "value": int32(3)},
							"value": map[string]any{
								"type":  "IntProperty",
								"value": int32(50),
							},
						},
					},
				},
			},
		},
	}
	raw, err := encodeValueByType(asset, node, value, binary.LittleEndian)
	if err != nil {
		t.Fatalf("encode struct value: %v", err)
	}

	decodeAsset := &uasset.Asset{
		Raw:   uasset.RawAsset{Bytes: raw},
		Names: names,
		Summary: uasset.PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	tag := uasset.PropertyTag{
		TypeNodes: []uasset.PropertyTypeNode{
			{Name: uasset.NameRef{Index: 1}, InnerCount: 1},
			{Name: uasset.NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	decoded, ok := decodeAsset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("decode encoded struct failed")
	}
	out := decoded.(map[string]any)
	if got := out["structType"]; got != "PerQualityLevelInt" {
		t.Fatalf("structType: got %v", got)
	}
	fields := out["value"].(map[string]any)
	defaultField := fields["Default"].(map[string]any)
	if got := defaultField["value"]; got != int32(100) {
		t.Fatalf("Default value: got %v", got)
	}
}

func TestWriteValueByTypeStructUniqueNetIdRepl(t *testing.T) {
	names := []uasset.NameEntry{
		{Value: "None"},
		{Value: "StructProperty"},
		{Value: "UniqueNetIdRepl"},
		{Value: "EOS"},
	}
	asset := &uasset.Asset{
		Names: names,
		Summary: uasset.PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	node := &typeTreeNode{
		Name: "StructProperty",
		Children: []*typeTreeNode{
			{Name: "UniqueNetIdRepl"},
		},
	}
	value := map[string]any{
		"structType": "UniqueNetIdRepl",
		"value": map[string]any{
			"Size": map[string]any{
				"type":  "IntProperty",
				"value": int32(10),
			},
			"Type": map[string]any{
				"type": "NameProperty",
				"value": map[string]any{
					"name": "EOS",
				},
			},
			"Contents": map[string]any{
				"type":  "StrProperty",
				"value": "ABCD1234",
			},
		},
	}
	raw, err := encodeValueByType(asset, node, value, binary.LittleEndian)
	if err != nil {
		t.Fatalf("encode struct value: %v", err)
	}

	decodeAsset := &uasset.Asset{
		Raw:   uasset.RawAsset{Bytes: raw},
		Names: names,
		Summary: uasset.PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	tag := uasset.PropertyTag{
		TypeNodes: []uasset.PropertyTypeNode{
			{Name: uasset.NameRef{Index: 1}, InnerCount: 1},
			{Name: uasset.NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	decoded, ok := decodeAsset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("decode encoded struct failed")
	}
	out := decoded.(map[string]any)
	fields := out["value"].(map[string]any)
	contents := fields["Contents"].(map[string]any)
	if got := contents["value"]; got != "ABCD1234" {
		t.Fatalf("Contents value: got %v", got)
	}
}

func TestWriteValueByTypeStructPerPlatformInt(t *testing.T) {
	names := []uasset.NameEntry{
		{Value: "None"},
		{Value: "StructProperty"},
		{Value: "PerPlatformInt"},
		{Value: "Windows"},
	}
	asset := &uasset.Asset{
		Names: names,
		Summary: uasset.PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	node := &typeTreeNode{
		Name: "StructProperty",
		Children: []*typeTreeNode{
			{Name: "PerPlatformInt"},
		},
	}
	value := map[string]any{
		"structType": "PerPlatformInt",
		"value": map[string]any{
			"bCooked": map[string]any{
				"type":  "BoolProperty",
				"value": false,
			},
			"Default": map[string]any{
				"type":  "IntProperty",
				"value": int32(120),
			},
			"PerPlatform": map[string]any{
				"type": "MapProperty(NameProperty,IntProperty)",
				"value": map[string]any{
					"keyType":   "NameProperty",
					"valueType": "IntProperty",
					"value": []map[string]any{
						{
							"key": map[string]any{
								"type": "NameProperty",
								"value": map[string]any{
									"name": "Windows",
								},
							},
							"value": map[string]any{
								"type":  "IntProperty",
								"value": int32(60),
							},
						},
					},
				},
			},
		},
	}

	raw, err := encodeValueByType(asset, node, value, binary.LittleEndian)
	if err != nil {
		t.Fatalf("encode struct value: %v", err)
	}

	decodeAsset := &uasset.Asset{
		Raw:   uasset.RawAsset{Bytes: raw},
		Names: names,
		Summary: uasset.PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	tag := uasset.PropertyTag{
		TypeNodes: []uasset.PropertyTypeNode{
			{Name: uasset.NameRef{Index: 1}, InnerCount: 1},
			{Name: uasset.NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	decoded, ok := decodeAsset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("decode encoded struct failed")
	}
	out := decoded.(map[string]any)
	fields := out["value"].(map[string]any)
	defaultField := fields["Default"].(map[string]any)
	if got := defaultField["value"]; got != int32(120) {
		t.Fatalf("Default value: got %v", got)
	}
}

func TestWriteValueByTypeStructPerPlatformFrameRate(t *testing.T) {
	names := []uasset.NameEntry{
		{Value: "None"},
		{Value: "StructProperty"},
		{Value: "PerPlatformFrameRate"},
		{Value: "PAL"},
	}
	asset := &uasset.Asset{
		Names: names,
		Summary: uasset.PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	node := &typeTreeNode{
		Name: "StructProperty",
		Children: []*typeTreeNode{
			{Name: "PerPlatformFrameRate"},
		},
	}
	value := map[string]any{
		"structType": "PerPlatformFrameRate",
		"value": map[string]any{
			"bCooked": map[string]any{
				"type":  "BoolProperty",
				"value": false,
			},
			"Default": map[string]any{
				"type": "StructProperty(FrameRate)",
				"value": map[string]any{
					"structType": "FrameRate",
					"value": map[string]any{
						"Numerator":   int32(60),
						"Denominator": int32(1),
					},
				},
			},
			"PerPlatform": map[string]any{
				"type": "MapProperty(NameProperty,StructProperty(FrameRate))",
				"value": map[string]any{
					"keyType":   "NameProperty",
					"valueType": "StructProperty(FrameRate)",
					"value": []map[string]any{
						{
							"key": map[string]any{
								"type": "NameProperty",
								"value": map[string]any{
									"name": "PAL",
								},
							},
							"value": map[string]any{
								"type": "StructProperty(FrameRate)",
								"value": map[string]any{
									"structType": "FrameRate",
									"value": map[string]any{
										"Numerator":   int32(25),
										"Denominator": int32(1),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	raw, err := encodeValueByType(asset, node, value, binary.LittleEndian)
	if err != nil {
		t.Fatalf("encode struct value: %v", err)
	}

	decodeAsset := &uasset.Asset{
		Raw:   uasset.RawAsset{Bytes: raw},
		Names: names,
		Summary: uasset.PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	tag := uasset.PropertyTag{
		TypeNodes: []uasset.PropertyTypeNode{
			{Name: uasset.NameRef{Index: 1}, InnerCount: 1},
			{Name: uasset.NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	decoded, ok := decodeAsset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("decode encoded struct failed")
	}
	out := decoded.(map[string]any)
	fields := out["value"].(map[string]any)
	defaultField := fields["Default"].(map[string]any)
	defaultFrameRate := defaultField["value"].(map[string]any)["value"].(map[string]any)
	if got := defaultFrameRate["Numerator"]; got != int32(60) {
		t.Fatalf("Numerator: got %v", got)
	}
}

func TestWriteValueByTypeStructRemoteObjectReference(t *testing.T) {
	names := []uasset.NameEntry{
		{Value: "None"},
		{Value: "StructProperty"},
		{Value: "RemoteObjectReference"},
	}
	asset := &uasset.Asset{
		Names: names,
		Summary: uasset.PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	node := &typeTreeNode{
		Name: "StructProperty",
		Children: []*typeTreeNode{
			{Name: "RemoteObjectReference"},
		},
	}
	value := map[string]any{
		"structType": "RemoteObjectReference",
		"value": map[string]any{
			"ObjectId": map[string]any{
				"type":  "UInt64Property",
				"value": uint64(0x0102030405060708),
			},
			"ServerId": map[string]any{
				"type":  "UInt32Property",
				"value": uint32(33),
			},
		},
	}
	raw, err := encodeValueByType(asset, node, value, binary.LittleEndian)
	if err != nil {
		t.Fatalf("encode struct value: %v", err)
	}
	decodeAsset := &uasset.Asset{
		Raw:   uasset.RawAsset{Bytes: raw},
		Names: names,
		Summary: uasset.PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	tag := uasset.PropertyTag{
		TypeNodes: []uasset.PropertyTypeNode{
			{Name: uasset.NameRef{Index: 1}, InnerCount: 1},
			{Name: uasset.NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	decoded, ok := decodeAsset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("decode encoded struct failed")
	}
	out := decoded.(map[string]any)
	fields := out["value"].(map[string]any)
	serverID := fields["ServerId"].(map[string]any)
	if got := serverID["value"]; got != uint32(33) {
		t.Fatalf("ServerId: got %v", got)
	}
}

func TestWriteValueByTypeStructAnimationAttributeIdentifier(t *testing.T) {
	names := []uasset.NameEntry{
		{Value: "None"},
		{Value: "StructProperty"},
		{Value: "AnimationAttributeIdentifier"},
		{Value: "AttrA"},
		{Value: "Root"},
		{Value: "/Script/CoreUObject"},
		{Value: "FrameNumber"},
	}
	asset := &uasset.Asset{
		Names: names,
		Summary: uasset.PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	node := &typeTreeNode{
		Name: "StructProperty",
		Children: []*typeTreeNode{
			{Name: "AnimationAttributeIdentifier"},
		},
	}
	value := map[string]any{
		"structType": "AnimationAttributeIdentifier",
		"value": map[string]any{
			"Name": map[string]any{
				"type": "NameProperty",
				"value": map[string]any{
					"name": "AttrA",
				},
			},
			"BoneName": map[string]any{
				"type": "NameProperty",
				"value": map[string]any{
					"name": "Root",
				},
			},
			"BoneIndex": map[string]any{
				"type":  "IntProperty",
				"value": int32(7),
			},
			"ScriptStructPath": map[string]any{
				"type": "StructProperty(SoftObjectPath)",
				"value": map[string]any{
					"packageName": "/Script/CoreUObject",
					"assetName":   "FrameNumber",
					"subPath":     "",
				},
			},
		},
	}
	raw, err := encodeValueByType(asset, node, value, binary.LittleEndian)
	if err != nil {
		t.Fatalf("encode struct value: %v", err)
	}
	decodeAsset := &uasset.Asset{
		Raw:   uasset.RawAsset{Bytes: raw},
		Names: names,
		Summary: uasset.PackageSummary{
			FileVersionUE5: 1017,
		},
	}
	tag := uasset.PropertyTag{
		TypeNodes: []uasset.PropertyTypeNode{
			{Name: uasset.NameRef{Index: 1}, InnerCount: 1},
			{Name: uasset.NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	decoded, ok := decodeAsset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("decode encoded struct failed")
	}
	out := decoded.(map[string]any)
	fields := out["value"].(map[string]any)
	nameField := fields["Name"].(map[string]any)["value"].(map[string]any)
	if got := nameField["name"]; got != "AttrA" {
		t.Fatalf("Name: got %v", got)
	}
}

func TestWriteValueByTypeStructSoftObjectPathLegacy(t *testing.T) {
	names := []uasset.NameEntry{
		{Value: "None"},
		{Value: "StructProperty"},
		{Value: "SoftObjectPath"},
		{Value: "/Game/Maps/Main.Main"},
	}
	asset := &uasset.Asset{
		Names: names,
		Summary: uasset.PackageSummary{
			FileVersionUE5: 1006,
		},
	}
	node := &typeTreeNode{
		Name: "StructProperty",
		Children: []*typeTreeNode{
			{Name: "SoftObjectPath"},
		},
	}
	value := map[string]any{
		"structType": "SoftObjectPath",
		"value": map[string]any{
			"packageName": "/Game/Maps/Main",
			"assetName":   "Main",
			"subPath":     "PersistentLevel.ActorA",
		},
	}
	raw, err := encodeValueByType(asset, node, value, binary.LittleEndian)
	if err != nil {
		t.Fatalf("encode soft object path legacy value: %v", err)
	}
	decodeAsset := &uasset.Asset{
		Raw:   uasset.RawAsset{Bytes: raw},
		Names: names,
		Summary: uasset.PackageSummary{
			FileVersionUE5: 1006,
		},
	}
	tag := uasset.PropertyTag{
		TypeNodes: []uasset.PropertyTypeNode{
			{Name: uasset.NameRef{Index: 1}, InnerCount: 1},
			{Name: uasset.NameRef{Index: 2}, InnerCount: 0},
		},
		Size:        int32(len(raw)),
		ValueOffset: 0,
	}
	decoded, ok := decodeAsset.DecodePropertyValue(tag)
	if !ok {
		t.Fatalf("decode legacy soft object path failed")
	}
	out := decoded.(map[string]any)
	fields := out["value"].(map[string]any)
	if got := fields["assetPathName"]; got != "/Game/Maps/Main.Main" {
		t.Fatalf("assetPathName: got %v", got)
	}
	if got := fields["packageName"]; got != "/Game/Maps/Main" {
		t.Fatalf("packageName: got %v", got)
	}
	if got := fields["assetName"]; got != "Main" {
		t.Fatalf("assetName: got %v", got)
	}
	if got := fields["subPath"]; got != "PersistentLevel.ActorA" {
		t.Fatalf("subPath: got %v", got)
	}
}

func equalBytesIgnoringRanges(a, b []byte, ranges [][2]int) bool {
	if len(a) != len(b) {
		return false
	}
	if len(ranges) == 0 {
		return bytes.Equal(a, b)
	}
	mask := make([]bool, len(a))
	for _, r := range ranges {
		start := r[0]
		length := r[1]
		if start < 0 || length < 0 || start+length > len(a) {
			return false
		}
		for i := start; i < start+length; i++ {
			mask[i] = true
		}
	}
	for i := range a {
		if mask[i] {
			continue
		}
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
