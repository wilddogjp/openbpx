# `bpx blueprint disasm`: Specification and Script-Range Quality Requirements

This document defines the `bpx blueprint disasm` behavior and implementation requirements for reliable bytecode extraction.
The detailed disasm section previously in `docs/commands.md` is maintained here.

## Command Specification

Disassemble Kismet bytecode into human-readable text or structured JSON/TOML output.
The quality target is compatibility with UE 5.6 `SerializeExpr` / `EExprToken` semantics.
This is separate from `bpx blueprint bytecode` (raw base64 output).

```bash
bpx blueprint disasm <file.uasset> --export <n> [--format json|toml|text] [--analysis] [--entrypoint <vm>] [--max-steps <n>] [--range-source auto|export-map|ustruct-script|serial-full] [--strict-range] [--diagnostics]
bpx blueprint infer-pack <file.uasset> --export <n> [--entrypoint <vm>] [--max-steps <n>] [--out <dir>] [--range-source auto|export-map|ustruct-script|serial-full] [--strict-range] [--diagnostics]
```

- `--format json` (default): structured JSON instruction output
- `--format toml`: structured TOML instruction output
- `--format text`: one disassembly line per instruction
- `--analysis`: append inference metadata (entrypoint slice / CFG / callsite / def-use, etc.)
- `--entrypoint`: restrict analysis to instructions reachable from a VM offset (fallback to full analysis when not found)
- `--max-steps`: max steps for entrypoint-slice traversal

### Example: Text Output

```
0x0000  EX_LocalVariable        PropertyName="ReturnValue"
0x000A  EX_VirtualFunction      FunctionName="PrintString" Target=Self
0x001E    EX_StringConst        Value="Hello World"
0x002F    EX_EndFunctionParms
0x0030  EX_Return
0x0031  EX_EndOfScript
```

### Example: JSON Output

```json
{
  "file": "BP_Example.uasset",
  "export": 3,
  "objectName": "ExecuteUbergraph_BP_Example",
  "instructions": [
    {
      "offset": "0x0000",
      "vmOffset": "0x0000",
      "opcode": "EX_LocalVariable",
      "token": 0,
      "params": { "propertyName": "ReturnValue" }
    },
    {
      "offset": "0x000A",
      "vmOffset": "0x000A",
      "opcode": "EX_VirtualFunction",
      "token": 27,
      "params": { "functionName": "PrintString" },
      "children": [
        { "offset": "0x001E", "vmOffset": "0x001E", "opcode": "EX_StringConst", "token": 31, "params": { "value": "Hello World" } },
        { "offset": "0x002F", "vmOffset": "0x002F", "opcode": "EX_EndFunctionParms", "token": 22 }
      ]
    },
    { "offset": "0x0030", "vmOffset": "0x0030", "opcode": "EX_Return", "token": 4 },
    { "offset": "0x0031", "vmOffset": "0x0031", "opcode": "EX_EndOfScript", "token": 83 }
  ],
  "truncated": false,
  "decodeFailedAt": -1
}
```

### Supported Kismet Instruction Categories

| Category | Main Opcodes | Priority |
|---|---|---|
| Control flow | `EX_Jump`, `EX_JumpIfNot`, `EX_Return`, `EX_EndOfScript`, `EX_PopExecutionFlow`, `EX_PushExecutionFlow`, `EX_ComputedJump` | Required |
| Function calls | `EX_VirtualFunction`, `EX_FinalFunction`, `EX_LocalVirtualFunction`, `EX_LocalFinalFunction`, `EX_CallMath`, `EX_EndFunctionParms` | Required |
| Variable access | `EX_LocalVariable`, `EX_InstanceVariable`, `EX_DefaultVariable`, `EX_LocalOutVariable` | Required |
| Constant literals | `EX_IntConst`, `EX_FloatConst`, `EX_DoubleConst`, `EX_StringConst`, `EX_NameConst`, `EX_ObjectConst`, `EX_VectorConst`, `EX_RotationConst`, `EX_TransformConst`, `EX_TextConst`, `EX_True`, `EX_False`, `EX_IntZero`, `EX_IntOne`, `EX_NoObject`, `EX_NoInterface` | Required |
| Casts | `EX_DynamicCast`, `EX_MetaCast`, `EX_CrossInterfaceCast`, `EX_InterfaceToObjCast`, `EX_ObjToInterfaceCast`, `EX_Cast` (`ECastToken`) | Required |
| Context | `EX_Context`, `EX_Context_FailSilent`, `EX_InterfaceContext`, `EX_StructMemberContext` | Required |
| Properties | `EX_PropertyConst`, `EX_FieldPathConst`, `EX_StructConst`, `EX_EndStructConst` | Required |
| Array/Set/Map | `EX_SetArray`, `EX_SetSet`, `EX_SetMap`, `EX_ArrayConst`, `EX_EndArrayConst`, `EX_EndSetConst`, `EX_EndMapConst` | High |
| Delegates | `EX_BindDelegate`, `EX_CallMulticastDelegate`, `EX_AddMulticastDelegate`, `EX_RemoveMulticastDelegate`, `EX_ClearMulticastDelegate` | High |
| Other | `EX_Let`, `EX_LetBool`, `EX_LetObj`, `EX_LetDelegate`, `EX_LetMulticastDelegate`, `EX_LetValueOnPersistentFrame`, `EX_Self`, `EX_Nothing`, `EX_Tracepoint`, `EX_Breakpoint` | Medium |

Unsupported opcodes must be emitted as `EX_Unknown(0xNN)` plus raw bytes, without aborting parse.
`EX_Unknown(0xNN)` is a BPX display label, not an official UE `EExprToken` symbol.

## Current Problem Statement

When running `bpx blueprint disasm` on private project samples, we observed:

- many cases where `selectedDataSize` is fixed to 9 bytes
- frequent `EX_EndOfScript not found in selected range`
- tiny selected ranges even for large functions (`serialSize` in KB to tens of KB)

Main cause:
Current logic over-prioritizes `FObjectExport.ScriptSerializationStartOffset/EndOffset` and validates only `0 <= start <= end <= serialSize` shape checks.
In UE 5.6 these offsets may reflect `SerializeScriptProperties` (tagged properties) rather than exact `UStruct::Script` bytecode boundaries.

## Required Improvements for Extraction Quality

### 1. Multi-Stage Script Range Selection (`auto`)

Implement staged candidate selection in `selectBlueprintBytecode`:

1. Candidate from `export-map` range (existing)
2. Candidate inferred from `UStruct::Script` layout
3. Add `serial-full` candidate when others are invalid/low-confidence
4. Score all candidates and pick the highest-confidence range

Minimum scoring requirements:

- detect `EX_EndOfScript(0x53)` near expected termination
- decode should continue for a meaningful length without early collapse
- treat suspicious tiny fixed ranges (such as 9 bytes) as low confidence

### 2. `UStruct::Script` Layout Inference

Infer script region from export raw payload using UE 5.6 struct serialization order (`BytecodeBufferSize` / `SerializedScriptSize` + script stream).

UE source references:

- `Runtime/CoreUObject/Private/UObject/Class.cpp` (`UStruct::Serialize`, `UStruct::SerializeExpr`)
- `Runtime/CoreUObject/Public/UObject/ScriptSerialization.h` (`SerializeExpr` macros)
- `Runtime/CoreUObject/Public/UObject/Script.h` (`EExprToken`, `EX_EndOfScript`)

Required handling:

- process both `BytecodeBufferSize` and `SerializedScriptSize`
- use existing tagged-property range info (`ParseExportProperties`) to narrow start candidates
- explicitly treat `BytecodeBufferSize == 0` as no-script functions

Note:
`ScriptSerializationStartOffset/EndOffset` comes from `MarkScriptSerializationStart/End` (`UObject::SerializeScriptProperties` range) and must not be treated as primary bytecode bounds.

### 3. Explain Selection Decisions (Diagnostics)

Add extraction rationale fields to `bpx blueprint bytecode` / `disasm` JSON output.

Proposed fields:

- `rangeSource`: `export-map` | `ustruct-script` | `serial-full`
- `rangeConfidence`: `high` | `medium` | `low`
- `rangeScore`: numeric score
- `rangeDiagnostics`: detailed reasons (`endOfScriptFound`, `decodeErrorCount`, `candidateSizes`, etc.)
- `truncated`: whether disassembly was truncated
- `decodeFailedAt`: serialized offset where decode failed (`-1` on success)

### 4. Better Recovery on Imperfect Ranges

Even if the selected range is incomplete, maximize analytical utility:

- attempt resynchronization to known tokens after `EX_Unknown` instead of immediate stop
- always expose `truncated` / `decodeFailedAt` on interruption
- allow diagnostics summary in `text` output footer

### 5. CLI Control Flags

Keep explicit range controls for reproducibility and troubleshooting:

- `--range-source auto|export-map|ustruct-script|serial-full`
- `--strict-range` (exit non-zero on incomplete range)
- `--diagnostics` (emit candidate evaluation details)
- `--analysis` (emit inference-oriented JSON metadata)
- `--entrypoint` (slice from a given entrypoint)
- `--max-steps` (entrypoint analysis step limit)

## Inference Support (Implemented)

`bpx blueprint disasm --analysis` and `bpx blueprint infer-pack` currently output:

1. entrypoint slice (reachable instruction set)
2. basic blocks / CFG (nodes and edges)
3. normalized callsites (receiver/function/paramCount)
4. persistent-frame read/write map
5. UFunction signature inference (heuristic)
6. def-use / lightweight SSA-style events
7. branch condition summaries (`expr@vm` format)
8. structured summaries for switch / computed jump
9. confidence scores for function/CFG/calls/def-use
10. full infer-pack file bundle

## Remaining Tasks

Include `RESIDUAL_TASKS.md` in infer-pack output and keep unresolved points explicit.
Current major tasks:

- precise target reconstruction for computed jumps (runtime stack dependent)
- higher-fidelity case-key reconstruction for switch expressions
- complete UFunction type reconstruction (currently disasm-heuristic centered)
- full CFG structuring (loop normalization / SSA optimization)

## Test Requirements

### Required Automated Tests

- candidate-selection unit tests for `selectBlueprintBytecode`
- fixed verification of `rangeSource` and `selectedDataSize` on known fixtures
- regression tests for `EX_EndOfScript` detection presence and warning counts

### Real-Data Regression Tests (Local)

- pin representative blueprints from the Lyra sample project
- track ratio of `selectedDataSize == 9` in disasm output and compare before/after
- verify major instruction-count growth for representative functions (`ExecuteUbergraph_*`, etc.)

## Recommended Implementation Order

1. Add diagnostics fields first (make current behavior observable)
2. Add `auto` multi-candidate selection with `serial-full` fallback
3. Implement `UStruct::Script` inference
4. Expand instruction decoding to closer `SerializeExpr` coverage
5. Expand regression fixtures

## UE 5.6 Alignment Points (Evidence)

- Canonical `EExprToken` definitions:
  - `Runtime/CoreUObject/Public/UObject/Script.h`
  - examples: `EX_VirtualFunction=0x1B`, `EX_FinalFunction=0x1C`, `EX_Cast=0x38`, `EX_EndOfScript=0x53`
- Bytecode serialization order:
  - `Runtime/CoreUObject/Private/UObject/Class.cpp` (`UStruct::Serialize`)
  - starts with `BytecodeBufferSize` and `SerializedScriptSize`, then script stream via `SerializeExpr()`
- Meaning of `ScriptSerializationStart/EndOffset`:
  - `Runtime/CoreUObject/Private/UObject/Obj.cpp` (`UObject::SerializeScriptProperties`)
  - `Runtime/CoreUObject/Private/UObject/LinkerLoad.cpp` (`MarkScriptSerializationStart/End`)
  - Linker comments also indicate these offsets track the `SerializeScriptProperties()` section.
