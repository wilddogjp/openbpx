package cli

import (
	"encoding/binary"
	"sort"
	"strings"
	"testing"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

func TestTextDisplayFromDecodedHistoryTransform(t *testing.T) {
	history := map[string]any{
		"historyType":     "Transform",
		"historyTypeCode": uint8(10),
		"transformType":   "ToUpper",
		"sourceText": map[string]any{
			"historyType":     "Base",
			"historyTypeCode": uint8(0),
			"sourceString":    "hello",
			"value":           "hello",
		},
	}

	got := textDisplayFromDecodedHistory(history)
	if got != "HELLO" {
		t.Fatalf("display string: got %q want %q", got, "HELLO")
	}
}

func TestTextDisplayFromDecodedHistoryNamedFormat(t *testing.T) {
	history := map[string]any{
		"historyType": "NamedFormat",
		"formatText": map[string]any{
			"historyType":  "Base",
			"sourceString": "Hello {Name}",
		},
		"arguments": []map[string]any{
			{
				"name": "Name",
				"value": map[string]any{
					"type": "Text",
					"value": map[string]any{
						"historyType":     "Base",
						"historyTypeCode": uint8(0),
						"sourceString":    "World",
					},
				},
			},
		},
	}

	got := textDisplayFromDecodedHistory(history)
	if got != "Hello World" {
		t.Fatalf("display string: got %q want %q", got, "Hello World")
	}
}

func TestTextDisplayFromDecodedHistoryOrderedFormat(t *testing.T) {
	history := map[string]any{
		"historyType": "OrderedFormat",
		"formatText": map[string]any{
			"historyType":  "Base",
			"sourceString": "{0} / {1}",
		},
		"arguments": []any{
			map[string]any{"type": "Int", "value": int64(7)},
			map[string]any{"type": "String", "value": "Days"},
		},
	}

	got := textDisplayFromDecodedHistory(history)
	if got != "7 / Days" {
		t.Fatalf("display string: got %q want %q", got, "7 / Days")
	}
}

func TestTextDisplayFromDecodedHistoryStringTableEntryWithoutResolvedString(t *testing.T) {
	history := map[string]any{
		"historyType": "StringTableEntry",
		"tableIdName": "UI",
		"key":         "Title",
	}

	got := textDisplayFromDecodedHistory(history)
	if got != "" {
		t.Fatalf("display string: got %q want empty", got)
	}
}

func TestTextDisplayFromDecodedHistoryAsNumberDoesNotUseHeuristicSourceValue(t *testing.T) {
	history := map[string]any{
		"historyType": "AsNumber",
		"sourceValue": map[string]any{
			"type":  "Int",
			"value": int64(42),
		},
	}

	got := textDisplayFromDecodedHistory(history)
	if got != "" {
		t.Fatalf("display string: got %q want empty", got)
	}
}

func TestCollectLocalizationFromValueStructAndArray(t *testing.T) {
	ctx := localizationCollectContext{
		export:     2,
		objectName: "BP_Test",
		className:  "BlueprintGeneratedClass",
	}
	value := map[string]any{
		"structType": "TestStruct",
		"value": map[string]any{
			"Title": map[string]any{
				"type": "TextProperty",
				"value": map[string]any{
					"historyType":     "Base",
					"historyTypeCode": uint8(0),
					"namespace":       "UI",
					"key":             "Title",
					"sourceString":    "Title Text",
					"value":           "Title Text",
				},
			},
			"Items": map[string]any{
				"type": "ArrayProperty(TextProperty)",
				"value": map[string]any{
					"arrayType": "TextProperty",
					"value": []map[string]any{
						{
							"type": "TextProperty",
							"value": map[string]any{
								"historyType":     "Base",
								"historyTypeCode": uint8(0),
								"sourceString":    "ItemA",
								"value":           "ItemA",
							},
						},
						{
							"type": "TextProperty",
							"value": map[string]any{
								"historyType":            "None",
								"historyTypeCode":        uint8(255),
								"cultureInvariantString": "ItemB",
								"value":                  "ItemB",
							},
						},
					},
				},
			},
		},
	}

	var out []map[string]any
	collectLocalizationFromValue(ctx, "Root", "StructProperty", value, false, &out)
	if len(out) != 3 {
		t.Fatalf("entry count: got %d want 3", len(out))
	}

	paths := make([]string, 0, len(out))
	for _, entry := range out {
		paths = append(paths, entry["path"].(string))
	}
	sort.Strings(paths)
	want := []string{"Root.Items[0]", "Root.Items[1]", "Root.Title"}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("path[%d]: got %q want %q", i, paths[i], want[i])
		}
	}
}

func TestParseLocResBytesOptimizedCRC32(t *testing.T) {
	var data []byte
	data = append(data, encodeLocResMagic()...)
	data = append(data, byte(locResVersionOptimizedCRC32))
	offsetPos := len(data)
	data = appendInt64LE(data, 0) // localized string array offset placeholder

	data = appendUint32LE(data, 1) // entries count
	data = appendUint32LE(data, 1) // namespace count

	data = appendTextKeyCRC32(data, "NS")
	data = appendUint32LE(data, 1) // key count
	data = appendTextKeyCRC32(data, "Key")
	data = appendUint32LE(data, 0x12345678) // source hash
	data = appendInt32LE(data, 0)           // localized string index

	locArrayOffset := int64(len(data))
	patchInt64LE(data, offsetPos, locArrayOffset)

	data = appendInt32LE(data, 1)                // localized string count
	data = appendFStringANSI(data, "Translated") // localized string
	data = appendInt32LE(data, 1)                // ref count

	locres, err := parseLocResBytes(data)
	if err != nil {
		t.Fatalf("parse locres: %v", err)
	}
	if locres.Version != locResVersionOptimizedCRC32 {
		t.Fatalf("version: got %d want %d", locres.Version, locResVersionOptimizedCRC32)
	}
	entry, ok := locres.lookup("NS", "Key")
	if !ok {
		t.Fatalf("expected NS/Key to be present")
	}
	if entry.SourceStringHash != 0x12345678 {
		t.Fatalf("source hash: got 0x%08x", entry.SourceStringHash)
	}
	if entry.LocalizedString != "Translated" {
		t.Fatalf("localized string: got %q", entry.LocalizedString)
	}
}

func TestResolveLocalizationEntryUsesLocRes(t *testing.T) {
	entry := map[string]any{
		"namespace":     "UI",
		"key":           "Title",
		"historyType":   "Base",
		"sourceString":  "Source Text",
		"displayString": "Source Text",
	}
	locres := &locResResource{
		Version: locResVersionOptimizedCRC32,
		Entries: map[string]map[string]locResEntry{
			"UI": {
				"Title": {
					SourceStringHash: ueTextSourceStringHash("Source Text"),
					LocalizedString:  "TranslatedJA",
				},
			},
		},
	}

	missing := resolveLocalizationEntry(entry, "ja", locres)
	if missing {
		t.Fatalf("expected entry to resolve")
	}
	if got := entry["resolvedString"]; got != "TranslatedJA" {
		t.Fatalf("resolvedString: got %v", got)
	}
	if got, _ := entry["resolvedBy"].(string); got != "locres" {
		t.Fatalf("resolvedBy: got %q", got)
	}
}

func TestResolveLocalizationEntryUsesLocResWithEmptyNamespace(t *testing.T) {
	entry := map[string]any{
		"namespace":     "",
		"key":           "Title",
		"historyType":   "Base",
		"sourceString":  "Source Text",
		"displayString": "Source Text",
	}
	locres := &locResResource{
		Version: locResVersionOptimizedCRC32,
		Entries: map[string]map[string]locResEntry{
			"": {
				"Title": {
					SourceStringHash: ueTextSourceStringHash("Source Text"),
					LocalizedString:  "TranslatedNoNS",
				},
			},
		},
	}

	missing := resolveLocalizationEntry(entry, "ja", locres)
	if missing {
		t.Fatalf("expected entry to resolve")
	}
	if got := entry["resolvedString"]; got != "TranslatedNoNS" {
		t.Fatalf("resolvedString: got %v", got)
	}
	if got, _ := entry["resolvedBy"].(string); got != "locres" {
		t.Fatalf("resolvedBy: got %q", got)
	}
}

func TestResolveLocalizationEntryUsesLocResWithPackageNamespaceFallback(t *testing.T) {
	entry := map[string]any{
		"namespace":     "[AAD6319626B3D6CF084106B6335F9196]",
		"key":           "F0104A94415AF6C24568F498776973F4",
		"historyType":   "Base",
		"sourceString":  "Game Start",
		"displayString": "Game Start",
	}
	locres := &locResResource{
		Version: locResVersionOptimizedCRC32,
		Entries: map[string]map[string]locResEntry{
			"": {
				"F0104A94415AF6C24568F498776973F4": {
					SourceStringHash: ueTextSourceStringHash("Game Start"),
					LocalizedString:  "ゲームスタート",
				},
			},
		},
	}

	missing := resolveLocalizationEntry(entry, "ja", locres)
	if missing {
		t.Fatalf("expected entry to resolve")
	}
	if got := entry["resolvedString"]; got != "ゲームスタート" {
		t.Fatalf("resolvedString: got %v", got)
	}
	if got, _ := entry["resolvedBy"].(string); got != "locres" {
		t.Fatalf("resolvedBy: got %q", got)
	}
	if got, _ := entry["locresNamespace"].(string); got != "" {
		t.Fatalf("locresNamespace: got %q want empty string", got)
	}
}

func TestResolveLocalizationEntrySourceHashMismatch(t *testing.T) {
	entry := map[string]any{
		"namespace":     "UI",
		"key":           "Title",
		"historyType":   "Base",
		"sourceString":  "Source Text",
		"displayString": "Source Text",
	}
	locres := &locResResource{
		Version: locResVersionOptimizedCRC32,
		Entries: map[string]map[string]locResEntry{
			"UI": {
				"Title": {
					SourceStringHash: ueTextSourceStringHash("Different Source"),
					LocalizedString:  "TranslatedJA",
				},
			},
		},
	}

	missing := resolveLocalizationEntry(entry, "ja", locres)
	if !missing {
		t.Fatalf("expected entry to be unresolved by source hash mismatch")
	}
	if got, _ := entry["resolvedBy"].(string); got != "sourceHashMismatch" {
		t.Fatalf("resolvedBy: got %q want sourceHashMismatch", got)
	}
}

func TestResolveLocalizationEntryStringTableUsesLocResLookup(t *testing.T) {
	entry := map[string]any{
		"namespace":     "",
		"key":           "UI.Title",
		"historyType":   "StringTableEntry",
		"displayString": "Source Text",
	}
	locres := &locResResource{
		Version: locResVersionOptimizedCRC32,
		Entries: map[string]map[string]locResEntry{
			"": {
				"UI.Title": {
					SourceStringHash: ueTextSourceStringHash("Source Text"),
					LocalizedString:  "TranslatedJA",
				},
			},
		},
	}

	missing := resolveLocalizationEntry(entry, "ja", locres)
	if missing {
		t.Fatalf("string table entry should resolve from locres")
	}
	if got, _ := entry["resolvedBy"].(string); got != "locres" {
		t.Fatalf("resolvedBy: got %q want locres", got)
	}
	if got, _ := entry["resolvedString"].(string); got != "TranslatedJA" {
		t.Fatalf("resolvedString: got %q want %q", got, "TranslatedJA")
	}
}

func TestResolveLocalizationEntryStringTableUsesTableIDNamespaceFallback(t *testing.T) {
	entry := map[string]any{
		"key":         "StartButton",
		"historyType": "StringTableEntry",
		"tableIdName": "UI_Table",
	}
	locres := &locResResource{
		Version: locResVersionOptimizedCRC32,
		Entries: map[string]map[string]locResEntry{
			"UI_Table": {
				"StartButton": {
					LocalizedString: "開始",
				},
			},
		},
	}

	missing := resolveLocalizationEntry(entry, "ja", locres)
	if missing {
		t.Fatalf("string table entry should resolve via table namespace fallback")
	}
	if got, _ := entry["resolvedBy"].(string); got != "locres" {
		t.Fatalf("resolvedBy: got %q want locres", got)
	}
	if got, _ := entry["resolvedString"].(string); got != "開始" {
		t.Fatalf("resolvedString: got %q want %q", got, "開始")
	}
}

func TestCollectGatherableLocalizationEntries(t *testing.T) {
	raw := make([]byte, 0, 256)
	raw = append(raw, 0, 0, 0, 0) // prefix bytes before section offset

	section := make([]byte, 0, 252)
	section = appendFStringANSI(section, "UI")
	section = appendFStringANSI(section, "HELLO")
	section = appendLocMetadataObject(section, map[string]any{
		"gender": "neutral",
	})
	section = appendInt32LE(section, 1) // source site context count
	section = appendFStringANSI(section, "BTN_HELLO")
	section = appendFStringANSI(section, "/Game/UI/WBP_Hello")
	section = appendUBool(section, true)
	section = appendUBool(section, false)
	section = appendLocMetadataObject(section, map[string]any{
		"path": "/Game/UI/WBP_Hello",
	})
	section = appendLocMetadataObject(section, map[string]any{
		"plural": true,
	})
	raw = append(raw, section...)

	asset := &uasset.Asset{
		Raw: uasset.RawAsset{Bytes: raw},
		Summary: uasset.PackageSummary{
			GatherableTextDataCount:  1,
			GatherableTextDataOffset: 4,
		},
	}

	entries, warnings := collectGatherableLocalizationEntries(asset, 0, true)
	if len(warnings) != 0 {
		t.Fatalf("warnings: %v", warnings)
	}
	if len(entries) != 1 {
		t.Fatalf("entry count: got %d want 1", len(entries))
	}
	entry := entries[0]
	if got, _ := entry["source"].(string); got != "GatherableTextData" {
		t.Fatalf("source: got %q", got)
	}
	if got, _ := entry["namespace"].(string); got != "UI" {
		t.Fatalf("namespace: got %q", got)
	}
	if got, _ := entry["key"].(string); got != "BTN_HELLO" {
		t.Fatalf("key: got %q", got)
	}
	if got, _ := entry["sourceString"].(string); got != "HELLO" {
		t.Fatalf("sourceString: got %q", got)
	}
	if got, _ := entry["path"].(string); got != "/Game/UI/WBP_Hello" {
		t.Fatalf("path: got %q", got)
	}
	if got, _ := entry["historyType"].(string); got != "Base" {
		t.Fatalf("historyType: got %q", got)
	}
}

func TestCollectGatherableLocalizationEntriesWithExportFilter(t *testing.T) {
	asset := &uasset.Asset{
		Summary: uasset.PackageSummary{
			GatherableTextDataCount:  1,
			GatherableTextDataOffset: 32,
		},
	}

	entries, warnings := collectGatherableLocalizationEntries(asset, 1, false)
	if len(entries) != 0 {
		t.Fatalf("entries: got %d want 0", len(entries))
	}
	if len(warnings) == 0 {
		t.Fatalf("expected warning for --export filter")
	}
	if !strings.Contains(warnings[0], "omitted") {
		t.Fatalf("warning: got %q", warnings[0])
	}
}

func TestStripPackageNamespace(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "[ABCD]", want: ""},
		{input: "UI [ABCD]", want: "UI"},
		{input: "UI[ABCD]", want: "UI"},
		{input: "[ABCD]Menu", want: "[ABCD]Menu"},
		{input: "Menu [A][B]", want: "Menu [A]"},
		{input: "PlainNamespace", want: "PlainNamespace"},
		{input: "[Broken", want: "[Broken"},
	}
	for _, tc := range tests {
		if got := stripPackageNamespace(tc.input); got != tc.want {
			t.Fatalf("stripPackageNamespace(%q): got %q want %q", tc.input, got, tc.want)
		}
	}
}

func appendUint32LE(dst []byte, v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return append(dst, b...)
}

func appendInt32LE(dst []byte, v int32) []byte {
	return appendUint32LE(dst, uint32(v))
}

func appendInt64LE(dst []byte, v int64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(v))
	return append(dst, b...)
}

func appendFStringANSI(dst []byte, s string) []byte {
	dst = appendInt32LE(dst, int32(len(s)+1))
	dst = append(dst, []byte(s)...)
	return append(dst, 0)
}

func appendTextKeyCRC32(dst []byte, key string) []byte {
	dst = appendUint32LE(dst, 0) // discarded hash
	return appendFStringANSI(dst, key)
}

func appendUBool(dst []byte, v bool) []byte {
	if v {
		return appendUint32LE(dst, 1)
	}
	return appendUint32LE(dst, 0)
}

func appendLocMetadataObject(dst []byte, fields map[string]any) []byte {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	dst = appendInt32LE(dst, int32(len(keys)))
	for _, key := range keys {
		dst = appendFStringANSI(dst, key)
		dst = appendLocMetadataValue(dst, fields[key])
	}
	return dst
}

func appendLocMetadataValue(dst []byte, value any) []byte {
	switch t := value.(type) {
	case string:
		dst = appendInt32LE(dst, locMetadataTypeString)
		return appendFStringANSI(dst, t)
	case bool:
		dst = appendInt32LE(dst, locMetadataTypeBoolean)
		return appendUBool(dst, t)
	case map[string]any:
		dst = appendInt32LE(dst, locMetadataTypeObject)
		return appendLocMetadataObject(dst, t)
	case []any:
		dst = appendInt32LE(dst, locMetadataTypeArray)
		dst = appendInt32LE(dst, int32(len(t)))
		for _, item := range t {
			dst = appendLocMetadataValue(dst, item)
		}
		return dst
	default:
		panic("unsupported metadata test value")
	}
}
