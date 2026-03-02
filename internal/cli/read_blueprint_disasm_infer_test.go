package cli

import "testing"

func makeFieldPath(name string) map[string]any {
	return map[string]any{
		"segments": []any{name},
	}
}

func TestAnalyzeDisasmForInferenceEntrypointSliceFallbacksToFullWhenMissing(t *testing.T) {
	instructions := []disasmInstruction{
		{VMOffset: 0, Opcode: "EX_Tracepoint", Token: 0x5E},
		{VMOffset: 1, Opcode: "EX_Tracepoint", Token: 0x5E},
		{VMOffset: 2, Opcode: "EX_EndOfScript", Token: 0x53},
	}
	inf := analyzeDisasmForInference(nil, 1, "TestFn", instructions, nil, false, -1, 999, 100)
	if inf.Slice.Found {
		t.Fatalf("expected entrypoint not found")
	}
	if inf.Slice.InstructionCount != len(instructions) {
		t.Fatalf("slice instruction count: got %d want %d", inf.Slice.InstructionCount, len(instructions))
	}
	if len(inf.Slice.ReachableVMOffsets) != len(instructions) {
		t.Fatalf("reachable vm count: got %d want %d", len(inf.Slice.ReachableVMOffsets), len(instructions))
	}
}

func TestAnalyzeDisasmForInferenceBuildsCFGAndBranches(t *testing.T) {
	instructions := []disasmInstruction{
		{VMOffset: 0, Opcode: "EX_JumpIfNot", Token: 0x07, Params: map[string]any{"targetVmOffset": 4, "conditionVmOffset": 1}},
		{VMOffset: 1, Opcode: "EX_True", Token: 0x27},
		{VMOffset: 2, Opcode: "EX_Jump", Token: 0x06, Params: map[string]any{"targetVmOffset": 5}},
		{VMOffset: 3, Opcode: "EX_False", Token: 0x28},
		{VMOffset: 4, Opcode: "EX_Tracepoint", Token: 0x5E},
		{VMOffset: 5, Opcode: "EX_Return", Token: 0x04},
		{VMOffset: 6, Opcode: "EX_Nothing", Token: 0x0B},
		{VMOffset: 7, Opcode: "EX_EndOfScript", Token: 0x53},
	}
	inf := analyzeDisasmForInference(nil, 1, "GraphFn", instructions, nil, false, -1, 0, 100)
	if !inf.Slice.Found {
		t.Fatalf("expected entrypoint found")
	}
	if len(inf.CFG.Nodes) == 0 {
		t.Fatalf("expected cfg nodes")
	}
	if len(inf.CFG.Edges) == 0 {
		t.Fatalf("expected cfg edges")
	}
	if len(inf.Branches) == 0 {
		t.Fatalf("expected branch conditions")
	}
}

func TestAnalyzeDisasmForInferenceTracksJumpTargetAtVMZero(t *testing.T) {
	instructions := []disasmInstruction{
		{VMOffset: 0, Opcode: "EX_EndOfScript", Token: 0x53},
		{VMOffset: 1, Opcode: "EX_Jump", Token: 0x06, Params: map[string]any{"targetVmOffset": 0}},
	}
	inf := analyzeDisasmForInference(nil, 1, "JumpToZero", instructions, nil, false, -1, 1, 100)
	if !inf.Slice.Found {
		t.Fatalf("expected entrypoint to be found")
	}
	found := false
	for _, vm := range inf.Slice.ReachableVMOffsets {
		if vm == 0 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected VM offset 0 to be reachable, got %#v", inf.Slice.ReachableVMOffsets)
	}
}

func TestAnalyzeDisasmForInferenceEntrypointSliceSnapsToNearestInstruction(t *testing.T) {
	instructions := []disasmInstruction{
		{VMOffset: 0, Opcode: "EX_Tracepoint", Token: 0x5E},
		{VMOffset: 10, Opcode: "EX_Tracepoint", Token: 0x5E},
		{VMOffset: 20, Opcode: "EX_EndOfScript", Token: 0x53},
	}
	inf := analyzeDisasmForInference(nil, 1, "SnapFn", instructions, nil, false, -1, 12, 100)
	if !inf.Slice.Found {
		t.Fatalf("expected snapped entrypoint to be treated as found")
	}
	if inf.SelectedEntryPoint != 10 {
		t.Fatalf("selected entrypoint: got %d want 10", inf.SelectedEntryPoint)
	}
	if inf.Confidence.Function != "medium" {
		t.Fatalf("function confidence: got %s want medium", inf.Confidence.Function)
	}
}

func TestAnalyzeDisasmForInferenceExtractsCallsiteAndPersistentFrame(t *testing.T) {
	instructions := []disasmInstruction{
		{VMOffset: 0, Opcode: "EX_LetValueOnPersistentFrame", Token: 0x64, Params: map[string]any{"destinationFieldPath": makeFieldPath("K2Node_Event_InItem"), "rhsVmOffset": 1}},
		{VMOffset: 1, Opcode: "EX_LocalVariable", Token: 0x00, Params: map[string]any{"fieldPath": makeFieldPath("InItem")}},
		{VMOffset: 2, Opcode: "EX_LocalFinalFunction", Token: 0x46, Params: map[string]any{"functionResolved": "export:43:ExecuteUbergraph_BP", "paramCount": 1, "paramExprVmOffsets": []any{3.0}}},
		{VMOffset: 3, Opcode: "EX_IntConst", Token: 0x1D, Params: map[string]any{"value": 10}},
		{VMOffset: 4, Opcode: "EX_EndFunctionParms", Token: 0x16},
		{VMOffset: 5, Opcode: "EX_LocalVariable", Token: 0x00, Params: map[string]any{"fieldPath": makeFieldPath("K2Node_Event_InItem")}},
		{VMOffset: 6, Opcode: "EX_Return", Token: 0x04},
		{VMOffset: 7, Opcode: "EX_Nothing", Token: 0x0B},
		{VMOffset: 8, Opcode: "EX_EndOfScript", Token: 0x53},
	}
	inf := analyzeDisasmForInference(nil, 1, "WrapperFn", instructions, nil, false, -1, -1, 100)
	if len(inf.Callsites) != 1 {
		t.Fatalf("callsites: got %d want 1", len(inf.Callsites))
	}
	if inf.Callsites[0].ParamCount != 1 {
		t.Fatalf("param count: got %d want 1", inf.Callsites[0].ParamCount)
	}
	if len(inf.PersistentFrame.Variables) != 1 {
		t.Fatalf("persistent vars: got %d want 1", len(inf.PersistentFrame.Variables))
	}
	v := inf.PersistentFrame.Variables[0]
	if v.Name != "K2Node_Event_InItem" {
		t.Fatalf("persistent name: got %s", v.Name)
	}
	if len(v.WritesAt) != 1 || v.WritesAt[0] != 0 {
		t.Fatalf("writes: got %#v want [0]", v.WritesAt)
	}
	if len(v.SourceVars) != 1 || v.SourceVars[0] != "InItem" {
		t.Fatalf("source vars: got %#v want [InItem]", v.SourceVars)
	}
}

func TestInferFunctionSignatureUsesOutParamAsReturn(t *testing.T) {
	instructions := []disasmInstruction{
		{VMOffset: 0, Opcode: "EX_LetValueOnPersistentFrame", Token: 0x64, Params: map[string]any{"destinationFieldPath": makeFieldPath("K2Node_CustomEvent_NewParam"), "rhsVmOffset": 1}},
		{VMOffset: 1, Opcode: "EX_LocalVariable", Token: 0x00, Params: map[string]any{"fieldPath": makeFieldPath("NewParam")}},
		{VMOffset: 2, Opcode: "EX_LetBool", Token: 0x14},
		{VMOffset: 3, Opcode: "EX_LocalOutVariable", Token: 0x48, Params: map[string]any{"fieldPath": makeFieldPath("NewParam")}},
		{VMOffset: 4, Opcode: "EX_True", Token: 0x27},
		{VMOffset: 5, Opcode: "EX_Return", Token: 0x04},
		{VMOffset: 6, Opcode: "EX_Nothing", Token: 0x0B},
		{VMOffset: 7, Opcode: "EX_EndOfScript", Token: 0x53},
	}
	sig := inferFunctionSignature("IsFirstPerson", instructions, nil)
	if sig.ReturnType != "bool" {
		t.Fatalf("return type: got %s want bool", sig.ReturnType)
	}
	if len(sig.Params) == 0 {
		t.Fatalf("expected inferred params")
	}
}

func TestBuildInstructionEdgesSwitchValueUsesCaseBodies(t *testing.T) {
	inst := disasmInstruction{
		VMOffset: 100,
		Opcode:   "EX_SwitchValue",
		Params: map[string]any{
			"numCases":            2,
			"switchDoneVmOffset":  500,
			"defaultVmOffset":     400,
			"case_0_bodyVmOffset": 200,
			"case_0_nextVmOffset": 300,
			"case_1_bodyVmOffset": 250,
			"case_1_nextVmOffset": 400,
		},
	}

	edges := buildInstructionEdges(inst, map[int]int{100: 101})
	has := func(to int, typ string) bool {
		for _, e := range edges {
			if e.ToVM == to && e.Type == typ {
				return true
			}
		}
		return false
	}

	if !has(200, "switch_case") || !has(250, "switch_case") {
		t.Fatalf("expected switch_case edges to case bodies, got %#v", edges)
	}
	if !has(400, "switch_default") {
		t.Fatalf("expected switch_default edge to default term, got %#v", edges)
	}
	if !has(500, "switch_done") {
		t.Fatalf("expected switch_done edge to post-switch join, got %#v", edges)
	}
}

func TestBuildStructuredFlowSwitchUsesDefaultAndCaseBodyTargets(t *testing.T) {
	flow := buildStructuredFlow([]disasmInstruction{
		{
			VMOffset: 10,
			Opcode:   "EX_SwitchValue",
			Params: map[string]any{
				"numCases":            2,
				"switchDoneVmOffset":  500,
				"defaultVmOffset":     400,
				"case_0_bodyVmOffset": 200,
				"case_0_nextVmOffset": 300,
				"case_1_bodyVmOffset": 250,
				"case_1_nextVmOffset": 350,
			},
		},
	}, map[int]int{10: 11}, nil)

	if len(flow.Switches) != 1 {
		t.Fatalf("switch count: got %d want 1", len(flow.Switches))
	}
	sw := flow.Switches[0]
	if sw.DefaultTarget != 400 {
		t.Fatalf("default target: got %d want 400", sw.DefaultTarget)
	}
	if len(sw.Cases) != 2 {
		t.Fatalf("case count: got %d want 2", len(sw.Cases))
	}
	if sw.Cases[0].TargetVM != 200 || sw.Cases[1].TargetVM != 250 {
		t.Fatalf("case targets: got %#v", sw.Cases)
	}
}

func TestAnalyzeDisasmForInferenceMissingEntrypointLowersConfidence(t *testing.T) {
	instructions := []disasmInstruction{
		{VMOffset: 0, Opcode: "EX_LetValueOnPersistentFrame", Token: 0x64, Params: map[string]any{"destinationFieldPath": makeFieldPath("K2Node_Event_InItem"), "rhsVmOffset": 1}},
		{VMOffset: 1, Opcode: "EX_LocalVariable", Token: 0x00, Params: map[string]any{"fieldPath": makeFieldPath("InItem")}},
		{VMOffset: 2, Opcode: "EX_LocalFinalFunction", Token: 0x46, Params: map[string]any{"functionResolved": "export:43:ExecuteUbergraph_BP", "paramCount": 1, "paramExprVmOffsets": []any{3.0}}},
		{VMOffset: 3, Opcode: "EX_IntConst", Token: 0x1D, Params: map[string]any{"value": 10}},
		{VMOffset: 4, Opcode: "EX_EndFunctionParms", Token: 0x16},
		{VMOffset: 5, Opcode: "EX_Return", Token: 0x04},
		{VMOffset: 6, Opcode: "EX_Nothing", Token: 0x0B},
		{VMOffset: 7, Opcode: "EX_EndOfScript", Token: 0x53},
	}

	inf := analyzeDisasmForInference(nil, 1, "WrapperFn", instructions, nil, false, -1, 999, 100)
	if inf.Confidence.Function != "low" {
		t.Fatalf("function confidence: got %s want low", inf.Confidence.Function)
	}
	if inf.Confidence.Overall != "low" {
		t.Fatalf("overall confidence: got %s want low", inf.Confidence.Overall)
	}
}
