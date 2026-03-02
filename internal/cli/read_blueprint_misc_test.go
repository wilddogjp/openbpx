package cli

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

func TestDisassembleBytecodeParsesStringConstAndEnd(t *testing.T) {
	data := []byte{
		0x1F, 'H', 'i', 0x00, // EX_StringConst
		0x53, // EX_EndOfScript
	}
	res := disassembleBytecode(data, nil)
	if len(res.Instructions) != 2 {
		t.Fatalf("instruction count: got %d want 2", len(res.Instructions))
	}
	if res.Instructions[0].Opcode != "EX_StringConst" {
		t.Fatalf("opcode[0]: got %s", res.Instructions[0].Opcode)
	}
	if got := res.Instructions[0].Params["value"]; got != "Hi" {
		t.Fatalf("string value: got %#v", got)
	}
	if res.Instructions[1].Opcode != "EX_EndOfScript" {
		t.Fatalf("opcode[1]: got %s", res.Instructions[1].Opcode)
	}
}

func TestDisassembleBytecodeParsesNameConst(t *testing.T) {
	data := []byte{
		0x21,          // EX_NameConst
		0x01, 0, 0, 0, // name index
		0x00, 0, 0, 0, // number
		0x53, // EX_EndOfScript
	}
	names := []uasset.NameEntry{
		{Value: "None"},
		{Value: "MyName"},
	}
	res := disassembleBytecode(data, names)
	if len(res.Instructions) < 1 {
		t.Fatalf("expected at least one instruction")
	}
	name, _ := res.Instructions[0].Params["name"].(string)
	if name != "MyName" {
		t.Fatalf("name const decode: got %q", name)
	}
}

func TestDisassembleBytecodeInstrumentationInlineEventUsesType4(t *testing.T) {
	data := []byte{
		0x6A,                   // EX_InstrumentationEvent
		0x04,                   // InlineEvent
		0x01, 0x00, 0x00, 0x00, // FScriptName index
		0x00, 0x00, 0x00, 0x00, // FScriptName number
		0x53, // EX_EndOfScript
	}
	names := []uasset.NameEntry{
		{Value: "None"},
		{Value: "InlineName"},
	}

	res := disassembleBytecode(data, names)
	if res.Truncated {
		t.Fatalf("unexpected truncated decode")
	}
	if got, want := len(res.Instructions), 2; got != want {
		t.Fatalf("instruction count: got %d want %d", got, want)
	}
	inst := res.Instructions[0]
	if got := inst.Params["eventName"]; got != "InlineName" {
		t.Fatalf("eventName: got %v", got)
	}
	if got := res.Instructions[1].VMOffset; got != 10 {
		t.Fatalf("end vm offset: got %d want 10", got)
	}
}

func TestDisassembleBytecodeInstrumentationEventType1HasNoInlineName(t *testing.T) {
	data := []byte{
		0x6A, // EX_InstrumentationEvent
		0x01, // ClassScope (not InlineEvent)
		0x53, // EX_EndOfScript
	}
	res := disassembleBytecode(data, nil)
	if res.Truncated {
		t.Fatalf("unexpected truncated decode")
	}
	if got, want := len(res.Instructions), 2; got != want {
		t.Fatalf("instruction count: got %d want %d", got, want)
	}
	if _, ok := res.Instructions[0].Params["eventName"]; ok {
		t.Fatalf("unexpected inline eventName payload for eventType=1")
	}
}

func TestDisassembleBytecodeFieldPathUsesVirtualPointerVMSize(t *testing.T) {
	data := []byte{
		0x00,                   // EX_LocalVariable
		0x01, 0x00, 0x00, 0x00, // field-path segment count
		0x01, 0x00, 0x00, 0x00, // FScriptName index
		0x00, 0x00, 0x00, 0x00, // FScriptName number
		0x00, 0x00, 0x00, 0x00, // owner index
		0x53, // EX_EndOfScript
	}
	names := []uasset.NameEntry{
		{Value: "None"},
		{Value: "TestProperty"},
	}

	res := disassembleBytecode(data, names)
	if len(res.Instructions) != 2 {
		t.Fatalf("instruction count: got %d want 2", len(res.Instructions))
	}
	if got := res.Instructions[1].VMOffset; got != 9 {
		t.Fatalf("vm offset after field-path pointer: got %d want 9", got)
	}
}

func TestRenderDisasmText(t *testing.T) {
	lines := renderDisasmText([]disasmInstruction{
		{Offset: 0, Opcode: "EX_IntConst", Params: map[string]any{"value": int32(42)}},
	})
	if !strings.Contains(lines, "EX_IntConst") {
		t.Fatalf("missing opcode in rendered text: %s", lines)
	}
	if !strings.Contains(lines, "\"value\":42") {
		t.Fatalf("missing params in rendered text: %s", lines)
	}
}

func TestKismetOpcodeNameUsesUE56TokenMap(t *testing.T) {
	if got := kismetOpcodeName(0x38); got != "EX_Cast" {
		t.Fatalf("opcode 0x38: got %s want EX_Cast", got)
	}
	if got := kismetOpcodeName(0x53); got != "EX_EndOfScript" {
		t.Fatalf("opcode 0x53: got %s want EX_EndOfScript", got)
	}
}

func TestDisassembleBytecodeDoesNotStopOnMidEndToken(t *testing.T) {
	data := []byte{0x53, 0x53}
	res := disassembleBytecode(data, nil)
	if len(res.Instructions) != 2 {
		t.Fatalf("instruction count: got %d want 2", len(res.Instructions))
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("warnings: got %v want none", res.Warnings)
	}
}

func TestDisassembleBytecodeParsesJumpIfNotWithVMOffsets(t *testing.T) {
	data := []byte{
		0x07,                   // EX_JumpIfNot
		0x08, 0x00, 0x00, 0x00, // target vm offset
		0x27, // EX_True (condition)
		0x53, // EX_EndOfScript
	}
	res := disassembleBytecode(data, nil)
	if len(res.Instructions) != 3 {
		t.Fatalf("instruction count: got %d want 3", len(res.Instructions))
	}
	jump := res.Instructions[0]
	if jump.Opcode != "EX_JumpIfNot" {
		t.Fatalf("opcode: got %s want EX_JumpIfNot", jump.Opcode)
	}
	if got := jump.VMOffset; got != 0 {
		t.Fatalf("jump vm offset: got %d want 0", got)
	}
	target, ok := jump.Params["targetVmOffset"].(int)
	if !ok {
		t.Fatalf("target vm offset type: %#v", jump.Params["targetVmOffset"])
	}
	if target != 8 {
		t.Fatalf("target vm offset: got %d want 8", target)
	}
	cond, ok := jump.Params["conditionVmOffset"].(int)
	if !ok {
		t.Fatalf("condition vm offset type: %#v", jump.Params["conditionVmOffset"])
	}
	if cond != 5 {
		t.Fatalf("condition vm offset: got %d want 5", cond)
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("warnings: got %v want none", res.Warnings)
	}
}

func TestDisassembleBytecodeParsesVirtualFunctionAndParams(t *testing.T) {
	data := []byte{
		0x1B,                   // EX_VirtualFunction
		0x01, 0x00, 0x00, 0x00, // FScriptName index
		0x00, 0x00, 0x00, 0x00, // FScriptName number
		0x1D,                   // EX_IntConst
		0x2A, 0x00, 0x00, 0x00, // value 42
		0x16, // EX_EndFunctionParms
		0x53, // EX_EndOfScript
	}
	names := []uasset.NameEntry{
		{Value: "None"},
		{Value: "PrintString"},
	}

	res := disassembleBytecode(data, names)
	if len(res.Instructions) != 4 {
		t.Fatalf("instruction count: got %d want 4", len(res.Instructions))
	}
	call := res.Instructions[0]
	if call.Opcode != "EX_VirtualFunction" {
		t.Fatalf("opcode: got %s want EX_VirtualFunction", call.Opcode)
	}
	fnName, ok := call.Params["functionName"].(string)
	if !ok {
		t.Fatalf("function name type: %#v", call.Params["functionName"])
	}
	if fnName != "PrintString" {
		t.Fatalf("function name: got %q want PrintString", fnName)
	}
	paramCount, ok := call.Params["paramCount"].(int)
	if !ok {
		t.Fatalf("param count type: %#v", call.Params["paramCount"])
	}
	if paramCount != 1 {
		t.Fatalf("param count: got %d want 1", paramCount)
	}
	if res.Truncated {
		t.Fatalf("unexpected truncated decode")
	}
}

func TestDisassembleBytecodeWithAssetResolvesFinalFunctionIndex(t *testing.T) {
	data := []byte{
		0x1C,                   // EX_FinalFunction
		0xFF, 0xFF, 0xFF, 0xFF, // import index -1
		0x16, // EX_EndFunctionParms
		0x53, // EX_EndOfScript
	}
	asset := &uasset.Asset{
		Names: []uasset.NameEntry{
			{Value: "None"},
			{Value: "TestFunction"},
		},
		Imports: []uasset.ImportEntry{
			{
				ObjectName: uasset.NameRef{Index: 1, Number: 0},
			},
		},
	}

	res := disassembleBytecodeWithAsset(data, asset.Names, asset)
	if len(res.Instructions) < 1 {
		t.Fatalf("expected at least one instruction")
	}
	call := res.Instructions[0]
	if call.Opcode != "EX_FinalFunction" {
		t.Fatalf("opcode: got %s want EX_FinalFunction", call.Opcode)
	}
	resolved, ok := call.Params["functionResolved"].(string)
	if !ok {
		t.Fatalf("functionResolved type: %#v", call.Params["functionResolved"])
	}
	if resolved != "import:1:TestFunction" {
		t.Fatalf("functionResolved: got %q want import:1:TestFunction", resolved)
	}
}

func TestParseBytecodeRangeSource(t *testing.T) {
	got, err := parseBytecodeRangeSource("ustruct-script")
	if err != nil {
		t.Fatalf("parse range source: %v", err)
	}
	if got != bytecodeRangeUStruct {
		t.Fatalf("range source: got %s want %s", got, bytecodeRangeUStruct)
	}
	if _, err := parseBytecodeRangeSource("bad-source"); err == nil {
		t.Fatalf("expected parse error for invalid source")
	}
}

func TestSelectBlueprintBytecodeAutoPrefersUStructScript(t *testing.T) {
	serial := make([]byte, 80)
	// Insert a UStruct script header candidate:
	// - BytecodeBufferSize = 6
	// - SerializedScriptSize = 6
	// - Script bytes end with EX_EndOfScript.
	headerOffset := 20
	binary.LittleEndian.PutUint32(serial[headerOffset:headerOffset+4], 6)
	binary.LittleEndian.PutUint32(serial[headerOffset+4:headerOffset+8], 6)
	copy(serial[headerOffset+8:headerOffset+14], []byte{0x25, 0x26, 0x27, 0x28, 0x17, 0x53})

	asset := &uasset.Asset{
		Raw: uasset.RawAsset{Bytes: serial},
		Exports: []uasset.ExportEntry{
			{
				SerialOffset:                   0,
				SerialSize:                     int64(len(serial)),
				ScriptSerializationStartOffset: 0,
				ScriptSerializationEndOffset:   9,
			},
		},
	}

	selection, err := selectBlueprintBytecode(asset, 1, bytecodeSelectionOptions{
		RangeSource: bytecodeRangeAuto,
		StrictRange: true,
	})
	if err != nil {
		t.Fatalf("select bytecode: %v", err)
	}
	if selection.RangeSource != string(bytecodeRangeUStruct) {
		t.Fatalf("range source: got %s want %s", selection.RangeSource, bytecodeRangeUStruct)
	}
	if selection.DataStart != headerOffset+8 {
		t.Fatalf("data start: got %d want %d", selection.DataStart, headerOffset+8)
	}
	if len(selection.Data) != 6 {
		t.Fatalf("data size: got %d want 6", len(selection.Data))
	}
}

func TestSelectBlueprintBytecodeRespectsExportMapOverride(t *testing.T) {
	serial := []byte{
		0, 0, 0, 0, 0, 0, 0, 0, 0, // export-map selected range
		0x53, 0, 0, 0,
	}
	asset := &uasset.Asset{
		Raw: uasset.RawAsset{Bytes: serial},
		Exports: []uasset.ExportEntry{
			{
				SerialOffset:                   0,
				SerialSize:                     int64(len(serial)),
				ScriptSerializationStartOffset: 0,
				ScriptSerializationEndOffset:   9,
			},
		},
	}

	selection, err := selectBlueprintBytecode(asset, 1, bytecodeSelectionOptions{
		RangeSource: bytecodeRangeExportMap,
	})
	if err != nil {
		t.Fatalf("select bytecode: %v", err)
	}
	if selection.RangeSource != string(bytecodeRangeExportMap) {
		t.Fatalf("range source: got %s want %s", selection.RangeSource, bytecodeRangeExportMap)
	}
	if !selection.UsingScriptRange {
		t.Fatalf("expected using script range for export-map source")
	}
	if len(selection.Data) != 9 {
		t.Fatalf("data size: got %d want 9", len(selection.Data))
	}
}

func TestSelectBlueprintBytecodeStrictRangeRejectsIncompleteRange(t *testing.T) {
	serial := make([]byte, 12)
	asset := &uasset.Asset{
		Raw: uasset.RawAsset{Bytes: serial},
		Exports: []uasset.ExportEntry{
			{
				SerialOffset:                   0,
				SerialSize:                     int64(len(serial)),
				ScriptSerializationStartOffset: 0,
				ScriptSerializationEndOffset:   9,
			},
		},
	}

	_, err := selectBlueprintBytecode(asset, 1, bytecodeSelectionOptions{
		RangeSource: bytecodeRangeExportMap,
		StrictRange: true,
	})
	if err == nil {
		t.Fatalf("expected strict range error")
	}
	if !strings.Contains(err.Error(), "incomplete bytecode range") {
		t.Fatalf("unexpected error: %v", err)
	}
}
