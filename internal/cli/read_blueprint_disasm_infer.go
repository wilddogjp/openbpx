package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

type disasmInferenceResult struct {
	Version             string                        `json:"version"`
	RequestedEntryPoint int                           `json:"requestedEntryPoint"`
	SelectedEntryPoint  int                           `json:"selectedEntryPoint"`
	Slice               disasmEntrypointSlice         `json:"entrypointSlice"`
	CFG                 disasmCFG                     `json:"cfg"`
	Callsites           []disasmCallsite              `json:"callsites"`
	PersistentFrame     disasmPersistentFrameAnalysis `json:"persistentFrame"`
	Signature           disasmFunctionSignature       `json:"signature"`
	DefUse              disasmDefUseAnalysis          `json:"defUse"`
	Branches            []disasmBranchCondition       `json:"branches"`
	Structured          disasmStructuredFlow          `json:"structuredFlow"`
	Confidence          disasmConfidence              `json:"confidence"`
	ResidualTasks       []string                      `json:"residualTasks,omitempty"`
}

type disasmEntrypointSlice struct {
	Enabled             bool   `json:"enabled"`
	Found               bool   `json:"found"`
	InstructionCount    int    `json:"instructionCount"`
	ReachableBlockCount int    `json:"reachableBlockCount"`
	ReachableVMOffsets  []int  `json:"reachableVmOffsets,omitempty"`
	StepLimitHit        bool   `json:"stepLimitHit"`
	Notes               string `json:"notes,omitempty"`
}

type disasmCFG struct {
	EntryPoint int                `json:"entryPoint"`
	Nodes      []disasmBasicBlock `json:"nodes"`
	Edges      []disasmCFGEdge    `json:"edges"`
}

type disasmBasicBlock struct {
	ID             int    `json:"id"`
	StartVMOffset  int    `json:"startVmOffset"`
	EndVMOffset    int    `json:"endVmOffset"`
	InstructionCnt int    `json:"instructionCount"`
	Terminator     string `json:"terminator"`
	Confidence     string `json:"confidence"`
}

type disasmCFGEdge struct {
	FromBlockID int    `json:"fromBlockId"`
	ToBlockID   int    `json:"toBlockId,omitempty"`
	FromVM      int    `json:"fromVmOffset"`
	ToVM        int    `json:"toVmOffset,omitempty"`
	Type        string `json:"type"`
	Condition   string `json:"condition,omitempty"`
	Confidence  string `json:"confidence"`
	Notes       string `json:"notes,omitempty"`
}

type disasmCallsite struct {
	VMOffset           int    `json:"vmOffset"`
	Opcode             string `json:"opcode"`
	Receiver           string `json:"receiver"`
	Function           string `json:"function"`
	ParamCount         int    `json:"paramCount"`
	ParamExprVMOffsets []int  `json:"paramExprVmOffsets,omitempty"`
	Normalized         string `json:"normalized"`
	Confidence         string `json:"confidence"`
}

type disasmPersistentFrameVar struct {
	Name       string   `json:"name"`
	WritesAt   []int    `json:"writesAtVmOffsets"`
	ReadsAt    []int    `json:"readsAtVmOffsets"`
	SourceVars []string `json:"sourceVars,omitempty"`
}

type disasmPersistentFrameAnalysis struct {
	Variables []disasmPersistentFrameVar `json:"variables"`
}

type disasmSignatureParam struct {
	Name       string `json:"name"`
	TypeHint   string `json:"typeHint"`
	Direction  string `json:"direction"`
	Confidence string `json:"confidence"`
}

type disasmFunctionSignature struct {
	FunctionName string                 `json:"functionName"`
	Params       []disasmSignatureParam `json:"params"`
	ReturnType   string                 `json:"returnType"`
	Confidence   string                 `json:"confidence"`
	Source       string                 `json:"source"`
	Notes        []string               `json:"notes,omitempty"`
}

type disasmDefUseEvent struct {
	VMOffset   int    `json:"vmOffset"`
	Variable   string `json:"variable"`
	Kind       string `json:"kind"`
	Version    int    `json:"version"`
	Opcode     string `json:"opcode"`
	Confidence string `json:"confidence"`
}

type disasmDefUseVariable struct {
	Name         string `json:"name"`
	Definitions  int    `json:"definitions"`
	Uses         int    `json:"uses"`
	FinalVersion int    `json:"finalVersion"`
}

type disasmDefUseAnalysis struct {
	Events    []disasmDefUseEvent    `json:"events"`
	Variables []disasmDefUseVariable `json:"variables"`
}

type disasmBranchCondition struct {
	VMOffset      int    `json:"vmOffset"`
	Opcode        string `json:"opcode"`
	Condition     string `json:"condition"`
	TrueTargetVM  int    `json:"trueTargetVmOffset,omitempty"`
	FalseTargetVM int    `json:"falseTargetVmOffset,omitempty"`
	Confidence    string `json:"confidence"`
}

type disasmSwitchCase struct {
	CaseIndex int    `json:"caseIndex"`
	Key       string `json:"key"`
	TargetVM  int    `json:"targetVmOffset"`
}

type disasmSwitchStructure struct {
	VMOffset      int                `json:"vmOffset"`
	IndexExpr     string             `json:"indexExpr"`
	DoneVMOffset  int                `json:"doneVmOffset"`
	Cases         []disasmSwitchCase `json:"cases"`
	DefaultTarget int                `json:"defaultTargetVmOffset"`
	Confidence    string             `json:"confidence"`
}

type disasmComputedJumpStructure struct {
	VMOffset   int    `json:"vmOffset"`
	TargetExpr string `json:"targetExpr"`
	Confidence string `json:"confidence"`
	Notes      string `json:"notes,omitempty"`
}

type disasmStructuredFlow struct {
	Switches      []disasmSwitchStructure       `json:"switches"`
	ComputedJumps []disasmComputedJumpStructure `json:"computedJumps"`
}

type disasmConfidence struct {
	Function string `json:"function"`
	CFG      string `json:"cfg"`
	Calls    string `json:"calls"`
	DefUse   string `json:"defUse"`
	Overall  string `json:"overall"`
	Reason   string `json:"reason"`
}

type disasmInferInternal struct {
	reachableInst map[int]bool
	reachableEdge map[string]bool
	reachableNode map[int]bool
	cfg           disasmCFG
	edgesByVM     map[int][]disasmCFGEdge
	instByVM      map[int]disasmInstruction
	nextVMByVM    map[int]int
}

func analyzeDisasmForInference(asset *uasset.Asset, exportIndex int, objectName string, instructions []disasmInstruction, warnings []string, truncated bool, decodeFailedAt int, entryPoint int, maxSteps int) disasmInferenceResult {
	if maxSteps <= 0 {
		maxSteps = 20000
	}
	cfg, internals := buildCFG(instructions, entryPoint)
	reachInst, reachEdges, reachNodes, selectedEntry, found, stepLimitHit := sliceByEntrypoint(cfg, instructions, internals.edgesByVM, maxSteps, entryPoint)

	calls := buildCallsites(instructions, reachInst)
	persistent := buildPersistentFrameAnalysis(instructions, reachInst)
	signature := inferFunctionSignature(objectName, instructions, reachInst)
	defUse := buildDefUseAnalysis(instructions, reachInst)
	branches := buildBranchConditions(instructions, internals.nextVMByVM, reachInst)
	structured := buildStructuredFlow(instructions, internals.nextVMByVM, reachInst)
	confidence := buildConfidence(instructions, warnings, truncated, decodeFailedAt, cfg, calls, defUse, entryPoint, selectedEntry, found, stepLimitHit)

	result := disasmInferenceResult{
		Version:             "inference-v1",
		RequestedEntryPoint: entryPoint,
		SelectedEntryPoint:  selectedEntry,
		Slice: disasmEntrypointSlice{
			Enabled:             entryPoint >= 0,
			Found:               found,
			InstructionCount:    len(reachInst),
			ReachableBlockCount: len(reachNodes),
			ReachableVMOffsets:  sortedMapKeys(reachInst),
			StepLimitHit:        stepLimitHit,
		},
		CFG:             filterCFG(cfg, reachNodes, reachEdges, entryPoint >= 0),
		Callsites:       calls,
		PersistentFrame: persistent,
		Signature:       signature,
		DefUse:          defUse,
		Branches:        branches,
		Structured:      structured,
		Confidence:      confidence,
		ResidualTasks:   defaultResidualTasks(),
	}

	if entryPoint >= 0 && !found {
		result.Slice.Notes = fmt.Sprintf("entrypoint vm=0x%X was not found; full-flow analysis used", entryPoint)
	} else if entryPoint >= 0 && selectedEntry != entryPoint {
		result.Slice.Notes = fmt.Sprintf("entrypoint vm=0x%X snapped to nearest decoded instruction 0x%X", entryPoint, selectedEntry)
	}
	if stepLimitHit {
		if result.Slice.Notes == "" {
			result.Slice.Notes = "step limit hit while traversing entrypoint slice"
		} else {
			result.Slice.Notes += "; step limit hit while traversing entrypoint slice"
		}
	}
	_ = asset
	_ = exportIndex
	return result
}

func buildCFG(instructions []disasmInstruction, requestedEntry int) (disasmCFG, disasmInferInternal) {
	instByVM := make(map[int]disasmInstruction, len(instructions))
	vmOrder := make([]int, 0, len(instructions))
	for _, inst := range instructions {
		instByVM[inst.VMOffset] = inst
		vmOrder = append(vmOrder, inst.VMOffset)
	}
	sort.Ints(vmOrder)
	nextVMByVM := make(map[int]int, len(vmOrder))
	for i := 0; i+1 < len(vmOrder); i++ {
		nextVMByVM[vmOrder[i]] = vmOrder[i+1]
	}

	edgesByVM := make(map[int][]disasmCFGEdge)
	for _, inst := range instructions {
		edgesByVM[inst.VMOffset] = buildInstructionEdges(inst, nextVMByVM)
	}

	boundaries := make(map[int]struct{})
	if len(vmOrder) > 0 {
		boundaries[vmOrder[0]] = struct{}{}
	}
	if requestedEntry >= 0 {
		if _, ok := instByVM[requestedEntry]; ok {
			boundaries[requestedEntry] = struct{}{}
		}
	}
	for _, inst := range instructions {
		if isBlockTerminator(inst.Opcode) {
			if next, ok := nextVMByVM[inst.VMOffset]; ok {
				boundaries[next] = struct{}{}
			}
		}
		for _, edge := range edgesByVM[inst.VMOffset] {
			if edge.Type == "fallthrough" {
				continue
			}
			if _, ok := instByVM[edge.ToVM]; ok {
				boundaries[edge.ToVM] = struct{}{}
			}
		}
	}

	starts := make([]int, 0, len(boundaries))
	for vm := range boundaries {
		starts = append(starts, vm)
	}
	sort.Ints(starts)
	if len(starts) == 0 && len(vmOrder) > 0 {
		starts = append(starts, vmOrder[0])
	}

	vmToBlock := map[int]int{}
	nodes := make([]disasmBasicBlock, 0, len(starts))
	for i, start := range starts {
		endLimit := 1<<31 - 1
		if i+1 < len(starts) {
			endLimit = starts[i+1]
		}
		count := 0
		endVM := start
		term := ""
		for _, vm := range vmOrder {
			if vm < start || vm >= endLimit {
				continue
			}
			count++
			endVM = vm
			vmToBlock[vm] = len(nodes)
			inst := instByVM[vm]
			if isBlockTerminator(inst.Opcode) {
				term = inst.Opcode
				break
			}
		}
		if count == 0 {
			continue
		}
		if term == "" {
			term = instByVM[endVM].Opcode
		}
		node := disasmBasicBlock{
			ID:             len(nodes),
			StartVMOffset:  start,
			EndVMOffset:    endVM,
			InstructionCnt: count,
			Terminator:     term,
			Confidence:     "high",
		}
		nodes = append(nodes, node)
	}

	edges := make([]disasmCFGEdge, 0, len(instructions)*2)
	for _, node := range nodes {
		lastVM := node.EndVMOffset
		instEdges := edgesByVM[lastVM]
		for _, edge := range instEdges {
			e := edge
			e.FromBlockID = node.ID
			if _, ok := instByVM[e.ToVM]; ok {
				if bid, ok := vmToBlock[e.ToVM]; ok {
					e.ToBlockID = bid
				} else {
					e.Notes = strings.TrimSpace(strings.Join([]string{e.Notes, "target not mapped to block"}, "; "))
					e.Confidence = downgradeConfidence(e.Confidence)
				}
			}
			edges = append(edges, e)
		}
	}

	entry := 0
	if len(nodes) > 0 {
		entry = nodes[0].StartVMOffset
		if requestedEntry >= 0 {
			if _, ok := instByVM[requestedEntry]; ok {
				entry = requestedEntry
			}
		}
	}

	cfg := disasmCFG{EntryPoint: entry, Nodes: nodes, Edges: edges}
	internal := disasmInferInternal{
		reachableInst: nil,
		reachableEdge: nil,
		reachableNode: nil,
		cfg:           cfg,
		edgesByVM:     edgesByVM,
		instByVM:      instByVM,
		nextVMByVM:    nextVMByVM,
	}
	return cfg, internal
}

func sliceByEntrypoint(cfg disasmCFG, instructions []disasmInstruction, edgesByVM map[int][]disasmCFGEdge, maxSteps int, requestedEntry int) (map[int]bool, map[string]bool, map[int]bool, int, bool, bool) {
	reachableInst := make(map[int]bool, len(instructions))
	reachableEdge := make(map[string]bool)
	reachableNode := make(map[int]bool)
	if len(instructions) == 0 {
		return reachableInst, reachableEdge, reachableNode, requestedEntry, false, false
	}
	entry := cfg.EntryPoint
	found := true
	fullFlow := requestedEntry < 0
	if requestedEntry >= 0 {
		resolved, ok := resolveRequestedEntrypoint(instructions, requestedEntry, 16)
		if ok {
			entry = resolved
		} else {
			entry = cfg.EntryPoint
			found = false
			fullFlow = true
		}
	}

	queue := []int{entry}
	seen := map[int]bool{}
	steps := 0
	stepLimitHit := false
	instByVM := make(map[int]disasmInstruction, len(instructions))
	vmToNode := make(map[int]int)
	for _, inst := range instructions {
		instByVM[inst.VMOffset] = inst
	}
	for _, node := range cfg.Nodes {
		for vm := node.StartVMOffset; vm <= node.EndVMOffset; vm++ {
			if _, ok := instByVM[vm]; ok {
				vmToNode[vm] = node.ID
			}
		}
	}
	for len(queue) > 0 {
		vm := queue[0]
		queue = queue[1:]
		if seen[vm] {
			continue
		}
		seen[vm] = true
		if _, ok := instByVM[vm]; !ok {
			continue
		}
		reachableInst[vm] = true
		if bid, ok := vmToNode[vm]; ok {
			reachableNode[bid] = true
		}
		steps++
		if steps >= maxSteps {
			stepLimitHit = true
			break
		}
		for _, edge := range edgesByVM[vm] {
			if _, ok := instByVM[edge.ToVM]; ok {
				k := edgeKey(vm, edge.ToVM, edge.Type)
				reachableEdge[k] = true
				if !seen[edge.ToVM] {
					queue = append(queue, edge.ToVM)
				}
			}
		}
	}

	if fullFlow {
		for _, inst := range instructions {
			reachableInst[inst.VMOffset] = true
		}
		for _, node := range cfg.Nodes {
			reachableNode[node.ID] = true
		}
		for _, edge := range cfg.Edges {
			reachableEdge[edgeKey(edge.FromVM, edge.ToVM, edge.Type)] = true
		}
	}

	return reachableInst, reachableEdge, reachableNode, entry, found, stepLimitHit
}

func resolveRequestedEntrypoint(instructions []disasmInstruction, requested, maxSnapDistance int) (int, bool) {
	if len(instructions) == 0 {
		return requested, false
	}
	best := 0
	bestDiff := 1<<31 - 1
	exact := false
	for i, inst := range instructions {
		vm := inst.VMOffset
		diff := vm - requested
		if diff < 0 {
			diff = -diff
		}
		if i == 0 || diff < bestDiff || (diff == bestDiff && vm < best) {
			best = vm
			bestDiff = diff
		}
		if vm == requested {
			exact = true
			break
		}
	}
	if exact {
		return requested, true
	}
	if bestDiff <= maxSnapDistance {
		return best, true
	}
	return requested, false
}

func buildInstructionEdges(inst disasmInstruction, nextVMByVM map[int]int) []disasmCFGEdge {
	edges := make([]disasmCFGEdge, 0, 4)
	nextVM, hasNext := nextVMByVM[inst.VMOffset]
	add := func(to int, typ, cond, conf, notes string) {
		edges = append(edges, disasmCFGEdge{
			FromVM:     inst.VMOffset,
			ToVM:       to,
			Type:       typ,
			Condition:  cond,
			Confidence: conf,
			Notes:      notes,
		})
	}

	switch inst.Opcode {
	case "EX_Return":
		add(0, "return", "", "high", "")
	case "EX_EndOfScript":
		add(0, "end", "", "high", "")
	case "EX_Jump":
		target := getIntParam(inst.Params, "targetVmOffset", -1)
		add(target, "jump", "", confidenceForTarget(target), "")
	case "EX_JumpIfNot":
		target := getIntParam(inst.Params, "targetVmOffset", -1)
		cond := summarizeCondition(inst, "conditionVmOffset")
		if hasNext {
			add(nextVM, "branch_true", cond, "medium", "")
		}
		add(target, "branch_false", negateCondition(cond), confidenceForTarget(target), "")
	case "EX_SwitchValue":
		done := getIntParam(inst.Params, "switchDoneVmOffset", -1)
		if done >= 0 {
			add(done, "switch_done", "after_match", "medium", "synthetic post-case merge")
		}
		defaultTarget := getIntParam(inst.Params, "defaultVmOffset", -1)
		if defaultTarget >= 0 {
			add(defaultTarget, "switch_default", "default", confidenceForTarget(defaultTarget), "")
		}
		numCases := getIntParam(inst.Params, "numCases", 0)
		for i := 0; i < numCases; i++ {
			caseBodyKey := fmt.Sprintf("case_%d_bodyVmOffset", i)
			caseBodyTarget := getIntParam(inst.Params, caseBodyKey, -1)
			if caseBodyTarget >= 0 {
				add(caseBodyTarget, "switch_case", fmt.Sprintf("case[%d]", i), confidenceForTarget(caseBodyTarget), "")
			}
			nextCaseKey := fmt.Sprintf("case_%d_nextVmOffset", i)
			nextCaseTarget := getIntParam(inst.Params, nextCaseKey, -1)
			if nextCaseTarget >= 0 {
				add(nextCaseTarget, "switch_next", fmt.Sprintf("!case[%d]", i), confidenceForTarget(nextCaseTarget), "next-case dispatch")
			}
		}
	case "EX_PushExecutionFlow":
		target := getIntParam(inst.Params, "targetVmOffset", -1)
		if hasNext {
			add(nextVM, "fallthrough", "", "high", "")
		}
		add(target, "push_target", "deferred", confidenceForTarget(target), "stack target")
	case "EX_PopExecutionFlow":
		add(0, "pop_flow", "", "low", "dynamic stack target")
	case "EX_PopExecutionFlowIfNot":
		cond := summarizeCondition(inst, "conditionVmOffset")
		if hasNext {
			add(nextVM, "branch_true", cond, "medium", "")
		}
		add(0, "pop_flow_false", negateCondition(cond), "low", "dynamic stack target")
	case "EX_ComputedJump":
		add(0, "computed_jump", "", "low", "runtime-computed target")
	default:
		if hasNext {
			add(nextVM, "fallthrough", "", "high", "")
		}
	}
	return edges
}

func isBlockTerminator(opcode string) bool {
	switch opcode {
	case "EX_Jump", "EX_JumpIfNot", "EX_Return", "EX_EndOfScript", "EX_SwitchValue", "EX_ComputedJump", "EX_PopExecutionFlow", "EX_PopExecutionFlowIfNot":
		return true
	default:
		return false
	}
}

func buildCallsites(instructions []disasmInstruction, reachable map[int]bool) []disasmCallsite {
	isCall := func(op string) bool {
		switch op {
		case "EX_VirtualFunction", "EX_FinalFunction", "EX_LocalVirtualFunction", "EX_LocalFinalFunction", "EX_CallMath", "EX_CallMulticastDelegate":
			return true
		default:
			return false
		}
	}
	calls := make([]disasmCallsite, 0, 32)
	for i, inst := range instructions {
		if len(reachable) > 0 && !reachable[inst.VMOffset] {
			continue
		}
		if !isCall(inst.Opcode) {
			continue
		}
		fn := firstNonEmpty(
			getStringParam(inst.Params, "functionResolved"),
			getStringParam(inst.Params, "functionName"),
			inst.Opcode,
		)
		receiver := inferReceiver(instructions, i)
		paramCount := getIntParam(inst.Params, "paramCount", 0)
		paramRoots := getIntSliceParam(inst.Params, "paramExprVmOffsets")
		confidence := "medium"
		if fn != "" && paramCount >= 0 {
			confidence = "high"
		}
		normalized := fmt.Sprintf("%s.%s(%d args)", receiver, fn, paramCount)
		calls = append(calls, disasmCallsite{
			VMOffset:           inst.VMOffset,
			Opcode:             inst.Opcode,
			Receiver:           receiver,
			Function:           fn,
			ParamCount:         paramCount,
			ParamExprVMOffsets: paramRoots,
			Normalized:         normalized,
			Confidence:         confidence,
		})
	}
	return calls
}

func buildPersistentFrameAnalysis(instructions []disasmInstruction, reachable map[int]bool) disasmPersistentFrameAnalysis {
	type agg struct {
		writes map[int]bool
		reads  map[int]bool
		src    map[string]bool
	}
	vars := map[string]*agg{}

	ensure := func(name string) *agg {
		a, ok := vars[name]
		if ok {
			return a
		}
		a = &agg{writes: map[int]bool{}, reads: map[int]bool{}, src: map[string]bool{}}
		vars[name] = a
		return a
	}

	for idx, inst := range instructions {
		if len(reachable) > 0 && !reachable[inst.VMOffset] {
			continue
		}
		if inst.Opcode == "EX_LetValueOnPersistentFrame" {
			dest := extractFieldName(inst.Params, "destinationFieldPath")
			if dest == "" {
				continue
			}
			a := ensure(dest)
			a.writes[inst.VMOffset] = true
			rhs := rhsSourceVar(instructions, idx)
			if rhs != "" {
				a.src[rhs] = true
			}
		}
	}

	if len(vars) == 0 {
		return disasmPersistentFrameAnalysis{}
	}

	for _, inst := range instructions {
		if len(reachable) > 0 && !reachable[inst.VMOffset] {
			continue
		}
		field := extractFieldName(inst.Params, "fieldPath")
		if field == "" {
			continue
		}
		if a, ok := vars[field]; ok {
			a.reads[inst.VMOffset] = true
		}
	}

	out := make([]disasmPersistentFrameVar, 0, len(vars))
	for name, a := range vars {
		writes := sortedMapKeys(a.writes)
		reads := sortedMapKeys(a.reads)
		srcs := sortedStringKeys(a.src)
		out = append(out, disasmPersistentFrameVar{
			Name:       name,
			WritesAt:   writes,
			ReadsAt:    reads,
			SourceVars: srcs,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return disasmPersistentFrameAnalysis{Variables: out}
}

func inferFunctionSignature(objectName string, instructions []disasmInstruction, reachable map[int]bool) disasmFunctionSignature {
	params := make(map[string]disasmSignatureParam)
	outs := make(map[string]disasmSignatureParam)
	boolAssigned := map[string]bool{}
	returnType := "void"
	notes := make([]string, 0, 4)

	addParam := func(name, dir string) {
		if name == "" || isInternalVarName(name) {
			return
		}
		typeHint, typeConfidence := inferTypeHint(name)
		entry := disasmSignatureParam{Name: name, TypeHint: typeHint, Direction: dir, Confidence: typeConfidence}
		if dir == "out" {
			if _, ok := outs[name]; !ok {
				outs[name] = entry
			}
			return
		}
		if _, ok := params[name]; !ok {
			params[name] = entry
		}
	}

	for i, inst := range instructions {
		if len(reachable) > 0 && !reachable[inst.VMOffset] {
			continue
		}
		switch inst.Opcode {
		case "EX_LetValueOnPersistentFrame":
			rhs := rhsSourceVar(instructions, i)
			addParam(rhs, "in")
		case "EX_LocalOutVariable":
			name := extractFieldName(inst.Params, "fieldPath")
			addParam(name, "out")
		case "EX_LetBool":
			if i+1 < len(instructions) {
				next := instructions[i+1]
				if next.Opcode == "EX_LocalOutVariable" || next.Opcode == "EX_LocalVariable" {
					name := extractFieldName(next.Params, "fieldPath")
					if name != "" {
						boolAssigned[name] = true
					}
				}
			}
		}
	}

	if len(outs) == 1 {
		for name, out := range outs {
			if boolAssigned[name] {
				out.TypeHint = "bool"
				out.Confidence = "medium"
			}
			if name == "ReturnValue" || name == "NewParam" {
				returnType = out.TypeHint
				delete(outs, name)
				notes = append(notes, fmt.Sprintf("inferred return from out param %q", name))
			}
		}
	}

	paramList := make([]disasmSignatureParam, 0, len(params)+len(outs))
	for _, p := range params {
		paramList = append(paramList, p)
	}
	for _, p := range outs {
		paramList = append(paramList, p)
	}
	sort.Slice(paramList, func(i, j int) bool {
		if paramList[i].Direction == paramList[j].Direction {
			return paramList[i].Name < paramList[j].Name
		}
		return paramList[i].Direction < paramList[j].Direction
	})

	confidence := "low"
	if len(paramList) > 0 {
		confidence = "medium"
	}
	if returnType != "void" {
		confidence = "medium"
	}
	if len(paramList) >= 2 {
		confidence = "high"
	}
	if len(paramList) == 0 {
		notes = append(notes, "no stable parameter candidates found; signature may be wrapper-only or fully ubergraph-driven")
	}

	return disasmFunctionSignature{
		FunctionName: objectName,
		Params:       paramList,
		ReturnType:   returnType,
		Confidence:   confidence,
		Source:       "heuristic-disasm-v1",
		Notes:        notes,
	}
}

func buildDefUseAnalysis(instructions []disasmInstruction, reachable map[int]bool) disasmDefUseAnalysis {
	versions := map[string]int{}
	defs := map[string]int{}
	uses := map[string]int{}
	events := make([]disasmDefUseEvent, 0, len(instructions)*2)

	recordUse := func(vm int, op, name string) {
		if name == "" {
			return
		}
		v := versions[name]
		uses[name]++
		events = append(events, disasmDefUseEvent{
			VMOffset:   vm,
			Variable:   name,
			Kind:       "use",
			Version:    v,
			Opcode:     op,
			Confidence: "medium",
		})
	}
	recordDef := func(vm int, op, name string) {
		if name == "" {
			return
		}
		versions[name]++
		defs[name]++
		events = append(events, disasmDefUseEvent{
			VMOffset:   vm,
			Variable:   name,
			Kind:       "def",
			Version:    versions[name],
			Opcode:     op,
			Confidence: "medium",
		})
	}

	for i, inst := range instructions {
		if len(reachable) > 0 && !reachable[inst.VMOffset] {
			continue
		}
		switch inst.Opcode {
		case "EX_LocalVariable", "EX_InstanceVariable", "EX_DefaultVariable", "EX_ClassSparseDataVariable":
			recordUse(inst.VMOffset, inst.Opcode, extractFieldName(inst.Params, "fieldPath"))
		case "EX_LocalOutVariable":
			recordUse(inst.VMOffset, inst.Opcode, extractFieldName(inst.Params, "fieldPath"))
		case "EX_Let", "EX_LetValueOnPersistentFrame":
			recordDef(inst.VMOffset, inst.Opcode, extractFieldName(inst.Params, "destinationFieldPath"))
		case "EX_LetBool", "EX_LetObj", "EX_LetWeakObjPtr", "EX_LetMulticastDelegate", "EX_LetDelegate":
			if i+1 < len(instructions) {
				n := instructions[i+1]
				if n.Opcode == "EX_LocalVariable" || n.Opcode == "EX_LocalOutVariable" || n.Opcode == "EX_InstanceVariable" {
					recordDef(n.VMOffset, inst.Opcode, extractFieldName(n.Params, "fieldPath"))
				}
			}
		}
	}

	vars := make([]disasmDefUseVariable, 0, len(versions))
	for name, v := range versions {
		vars = append(vars, disasmDefUseVariable{
			Name:         name,
			Definitions:  defs[name],
			Uses:         uses[name],
			FinalVersion: v,
		})
	}
	sort.Slice(vars, func(i, j int) bool { return vars[i].Name < vars[j].Name })
	sort.Slice(events, func(i, j int) bool {
		if events[i].VMOffset == events[j].VMOffset {
			return events[i].Kind < events[j].Kind
		}
		return events[i].VMOffset < events[j].VMOffset
	})

	return disasmDefUseAnalysis{Events: events, Variables: vars}
}

func buildBranchConditions(instructions []disasmInstruction, nextVMByVM map[int]int, reachable map[int]bool) []disasmBranchCondition {
	out := make([]disasmBranchCondition, 0, 32)
	for _, inst := range instructions {
		if len(reachable) > 0 && !reachable[inst.VMOffset] {
			continue
		}
		switch inst.Opcode {
		case "EX_JumpIfNot":
			cond := summarizeCondition(inst, "conditionVmOffset")
			falseTarget := getIntParam(inst.Params, "targetVmOffset", -1)
			trueTarget := nextVMByVM[inst.VMOffset]
			out = append(out, disasmBranchCondition{
				VMOffset:      inst.VMOffset,
				Opcode:        inst.Opcode,
				Condition:     cond,
				TrueTargetVM:  trueTarget,
				FalseTargetVM: falseTarget,
				Confidence:    "medium",
			})
		case "EX_PopExecutionFlowIfNot":
			cond := summarizeCondition(inst, "conditionVmOffset")
			trueTarget := nextVMByVM[inst.VMOffset]
			out = append(out, disasmBranchCondition{
				VMOffset:      inst.VMOffset,
				Opcode:        inst.Opcode,
				Condition:     cond,
				TrueTargetVM:  trueTarget,
				FalseTargetVM: 0,
				Confidence:    "low",
			})
		}
	}
	return out
}

func buildStructuredFlow(instructions []disasmInstruction, nextVMByVM map[int]int, reachable map[int]bool) disasmStructuredFlow {
	switches := make([]disasmSwitchStructure, 0, 8)
	computed := make([]disasmComputedJumpStructure, 0, 4)

	for _, inst := range instructions {
		if len(reachable) > 0 && !reachable[inst.VMOffset] {
			continue
		}
		switch inst.Opcode {
		case "EX_SwitchValue":
			numCases := getIntParam(inst.Params, "numCases", 0)
			cases := make([]disasmSwitchCase, 0, numCases)
			for i := 0; i < numCases; i++ {
				target := getIntParam(inst.Params, fmt.Sprintf("case_%d_bodyVmOffset", i), -1)
				if target < 0 {
					target = getIntParam(inst.Params, fmt.Sprintf("case_%d_nextVmOffset", i), -1)
				}
				cases = append(cases, disasmSwitchCase{CaseIndex: i, Key: fmt.Sprintf("case[%d]", i), TargetVM: target})
			}
			indexExpr := "<expr>"
			if vm := getIntParam(inst.Params, "indexTermVmOffset", -1); vm >= 0 {
				indexExpr = fmt.Sprintf("expr@0x%X", vm)
			}
			defaultTarget := getIntParam(inst.Params, "defaultVmOffset", -1)
			if defaultTarget < 0 {
				defaultTarget = nextVMByVM[inst.VMOffset]
			}
			switches = append(switches, disasmSwitchStructure{
				VMOffset:      inst.VMOffset,
				IndexExpr:     indexExpr,
				DoneVMOffset:  getIntParam(inst.Params, "switchDoneVmOffset", -1),
				Cases:         cases,
				DefaultTarget: defaultTarget,
				Confidence:    "medium",
			})
		case "EX_ComputedJump":
			targetExpr := "<expr>"
			if vm := getIntParam(inst.Params, "operandVmOffset", -1); vm >= 0 {
				targetExpr = fmt.Sprintf("expr@0x%X", vm)
			}
			computed = append(computed, disasmComputedJumpStructure{
				VMOffset:   inst.VMOffset,
				TargetExpr: targetExpr,
				Confidence: "low",
				Notes:      "exact computed jump target depends on runtime VM stack",
			})
		}
	}

	return disasmStructuredFlow{Switches: switches, ComputedJumps: computed}
}

func buildConfidence(instructions []disasmInstruction, warnings []string, truncated bool, decodeFailedAt int, cfg disasmCFG, calls []disasmCallsite, defUse disasmDefUseAnalysis, entrypointRequested int, selectedEntrypoint int, entrypointFound bool, stepLimitHit bool) disasmConfidence {
	unknownCount := 0
	for _, inst := range instructions {
		if strings.HasPrefix(inst.Opcode, "EX_Unknown") {
			unknownCount++
		}
	}
	function := "high"
	reason := "decoded instruction stream is stable"
	if truncated || decodeFailedAt >= 0 {
		function = "low"
		reason = "disasm decode is truncated"
	} else if entrypointRequested >= 0 && !entrypointFound {
		function = "low"
		reason = fmt.Sprintf("requested entrypoint 0x%X was not found; full-flow fallback used", entrypointRequested)
	} else if entrypointRequested >= 0 && selectedEntrypoint != entrypointRequested {
		function = "medium"
		reason = fmt.Sprintf("requested entrypoint 0x%X snapped to 0x%X", entrypointRequested, selectedEntrypoint)
	} else if unknownCount > 0 || len(warnings) > 0 {
		function = "medium"
		reason = "warnings or unknown opcodes are present"
	}
	if stepLimitHit {
		if function == "high" {
			function = "medium"
		}
		if reason == "" {
			reason = "entrypoint traversal step limit hit"
		} else {
			reason += "; entrypoint traversal step limit hit"
		}
	}

	cfgConf := "high"
	for _, e := range cfg.Edges {
		if strings.Contains(e.Type, "computed") || strings.Contains(e.Type, "pop_flow") {
			cfgConf = "medium"
			break
		}
	}
	if cfgConf == "medium" && function == "low" {
		cfgConf = "low"
	}

	callConf := "low"
	highCalls := 0
	for _, c := range calls {
		if c.Confidence == "high" {
			highCalls++
		}
	}
	if len(calls) == 0 {
		callConf = "low"
	} else if highCalls*2 >= len(calls) {
		callConf = "high"
	} else {
		callConf = "medium"
	}

	defUseConf := "medium"
	if len(defUse.Events) == 0 {
		defUseConf = "low"
	}

	overall := minConfidence(function, cfgConf, callConf, defUseConf)
	return disasmConfidence{
		Function: function,
		CFG:      cfgConf,
		Calls:    callConf,
		DefUse:   defUseConf,
		Overall:  overall,
		Reason:   reason,
	}
}

func defaultResidualTasks() []string {
	return []string{
		"Computed-jump exact target recovery is still heuristic (runtime stack dependent).",
		"Switch case key expression reconstruction is currently summarized, not full AST-level pretty-print.",
		"Type/signature inference is heuristic because UFunction child-property decoding is not fully wired for all assets.",
		"CFG is token-stream based and not yet SSA-optimized; loop canonicalization remains manual.",
	}
}

func filterCFG(cfg disasmCFG, reachableNodes map[int]bool, reachableEdges map[string]bool, sliceEnabled bool) disasmCFG {
	if !sliceEnabled {
		return cfg
	}
	nodes := make([]disasmBasicBlock, 0, len(cfg.Nodes))
	for _, n := range cfg.Nodes {
		if reachableNodes[n.ID] {
			nodes = append(nodes, n)
		}
	}
	edges := make([]disasmCFGEdge, 0, len(cfg.Edges))
	for _, e := range cfg.Edges {
		if reachableEdges[edgeKey(e.FromVM, e.ToVM, e.Type)] {
			edges = append(edges, e)
		}
	}
	cfg.Nodes = nodes
	cfg.Edges = edges
	return cfg
}

func summarizeCondition(inst disasmInstruction, key string) string {
	vm := getIntParam(inst.Params, key, -1)
	if vm >= 0 {
		return fmt.Sprintf("expr@0x%X", vm)
	}
	return "<expr>"
}

func negateCondition(cond string) string {
	if cond == "" || cond == "<expr>" {
		return "!<expr>"
	}
	return "!(" + cond + ")"
}

func confidenceForTarget(target int) string {
	if target <= 0 {
		return "low"
	}
	return "high"
}

func inferReceiver(instructions []disasmInstruction, idx int) string {
	if idx <= 0 {
		return "self"
	}
	for i := idx - 1; i >= 0 && i >= idx-6; i-- {
		inst := instructions[i]
		switch inst.Opcode {
		case "EX_InstanceVariable", "EX_LocalVariable", "EX_DefaultVariable", "EX_LocalOutVariable":
			if name := extractFieldName(inst.Params, "fieldPath"); name != "" {
				return name
			}
		case "EX_Context", "EX_ClassContext", "EX_Context_FailSilent":
			if name := extractFieldName(inst.Params, "rValueFieldPath"); name != "" {
				return name
			}
		}
	}
	return "self"
}

func rhsSourceVar(instructions []disasmInstruction, idx int) string {
	for i := idx + 1; i < len(instructions) && i <= idx+4; i++ {
		n := instructions[i]
		if n.Opcode == "EX_LocalVariable" || n.Opcode == "EX_InstanceVariable" {
			if name := extractFieldName(n.Params, "fieldPath"); name != "" {
				return name
			}
		}
	}
	return ""
}

func inferTypeHint(name string) (string, string) {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "axis") || strings.Contains(lower, "elapsed") || strings.Contains(lower, "delta") || strings.Contains(lower, "damage") || strings.Contains(lower, "battery") || strings.Contains(lower, "speed") || strings.Contains(lower, "acceleration"):
		return "float", "medium"
	case strings.Contains(lower, "actionvalue"):
		return "FInputActionValue", "medium"
	case strings.Contains(lower, "sourceaction"):
		return "const UInputAction*", "medium"
	case strings.Contains(lower, "key"):
		return "FKey", "medium"
	case strings.Contains(lower, "name") || strings.Contains(lower, "item") || strings.Contains(lower, "notify") || strings.Contains(lower, "id"):
		return "FName", "low"
	case strings.HasPrefix(lower, "b") || strings.Contains(lower, "is") || strings.Contains(lower, "find"):
		return "bool", "low"
	default:
		return "auto", "low"
	}
}

func isInternalVarName(name string) bool {
	if name == "" {
		return true
	}
	lower := strings.ToLower(name)
	return strings.HasPrefix(lower, "callfunc_") || strings.HasPrefix(lower, "temp_")
}

func extractFieldName(params map[string]any, key string) string {
	if len(params) == 0 {
		return ""
	}
	raw, ok := params[key]
	if !ok {
		return ""
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	segRaw, ok := obj["segments"]
	if !ok {
		return ""
	}
	if segs, ok := segRaw.([]any); ok && len(segs) > 0 {
		if s, ok := segs[0].(string); ok {
			return s
		}
	}
	if segs, ok := segRaw.([]string); ok && len(segs) > 0 {
		return segs[0]
	}
	return ""
}

func getIntParam(params map[string]any, key string, def int) int {
	if params == nil {
		return def
	}
	raw, ok := params[key]
	if !ok || raw == nil {
		return def
	}
	switch v := raw.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case uint32:
		return int(v)
	case uint64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		i, err := v.Int64()
		if err == nil {
			return int(i)
		}
	}
	return def
}

func getStringParam(params map[string]any, key string) string {
	if params == nil {
		return ""
	}
	raw, ok := params[key]
	if !ok || raw == nil {
		return ""
	}
	if s, ok := raw.(string); ok {
		return s
	}
	return ""
}

func getIntSliceParam(params map[string]any, key string) []int {
	if params == nil {
		return nil
	}
	raw, ok := params[key]
	if !ok || raw == nil {
		return nil
	}
	switch arr := raw.(type) {
	case []int:
		return append([]int(nil), arr...)
	case []any:
		out := make([]int, 0, len(arr))
		for _, item := range arr {
			switch v := item.(type) {
			case int:
				out = append(out, v)
			case int32:
				out = append(out, int(v))
			case int64:
				out = append(out, int(v))
			case float64:
				out = append(out, int(v))
			}
		}
		return out
	default:
		return nil
	}
}

func sortedMapKeys(m map[int]bool) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

func sortedStringKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func edgeKey(from, to int, typ string) string {
	return fmt.Sprintf("%d:%d:%s", from, to, typ)
}

func downgradeConfidence(in string) string {
	switch in {
	case "high":
		return "medium"
	case "medium":
		return "low"
	default:
		return "low"
	}
}

func minConfidence(values ...string) string {
	level := map[string]int{"low": 0, "medium": 1, "high": 2}
	best := "high"
	min := 2
	for _, v := range values {
		s := level[v]
		if s < min {
			min = s
			best = v
		}
	}
	return best
}

func writeInferPackFiles(assetPath, objectName string, selection blueprintBytecodeSelection, disasm disasmResult, inference disasmInferenceResult, outDir string, diagnostics *bytecodeRangeDiagnostics) (string, error) {
	if strings.TrimSpace(outDir) == "" {
		ts := fmt.Sprintf("%d", time.Now().UTC().UnixNano())
		outDir = filepath.Join("testdata", "reports", "bpx_infer_pack_"+ts)
	}

	writeDir := outDir
	commitDir := false
	if info, err := os.Stat(outDir); err == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("output path is not a directory: %s", outDir)
		}
	} else if os.IsNotExist(err) {
		parent := filepath.Dir(outDir)
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return "", fmt.Errorf("create output parent directory: %w", err)
		}
		tmpDir, err := os.MkdirTemp(parent, "."+filepath.Base(outDir)+".tmp-*")
		if err != nil {
			return "", fmt.Errorf("create output staging directory: %w", err)
		}
		writeDir = tmpDir
		commitDir = true
		defer func() {
			if commitDir {
				_ = os.RemoveAll(writeDir)
			}
		}()
	} else {
		return "", fmt.Errorf("inspect output directory: %w", err)
	}

	payload := map[string]any{
		"file":               assetPath,
		"export":             selection.Export,
		"objectName":         objectName,
		"rangeSource":        selection.RangeSource,
		"rangeConfidence":    selection.RangeConfidence,
		"rangeScore":         selection.RangeScore,
		"selectedDataOffset": selection.DataStart,
		"selectedDataSize":   len(selection.Data),
		"instructions":       disasm.Instructions,
		"warnings":           disasm.Warnings,
		"truncated":          disasm.Truncated,
		"decodeFailedAt":     disasm.DecodeFailedAt,
		"inference":          inference,
	}
	if diagnostics != nil {
		payload["rangeDiagnostics"] = diagnostics
	}
	if err := writeJSONFile(filepath.Join(writeDir, "disasm_inference.json"), payload); err != nil {
		return "", err
	}
	if err := writeJSONFile(filepath.Join(writeDir, "cfg.json"), inference.CFG); err != nil {
		return "", err
	}
	if err := writeJSONFile(filepath.Join(writeDir, "signature.json"), inference.Signature); err != nil {
		return "", err
	}
	if err := writeJSONFile(filepath.Join(writeDir, "persistent_frame.json"), inference.PersistentFrame); err != nil {
		return "", err
	}
	if err := writeJSONFile(filepath.Join(writeDir, "def_use.json"), inference.DefUse); err != nil {
		return "", err
	}
	if err := writeJSONFile(filepath.Join(writeDir, "branches.json"), inference.Branches); err != nil {
		return "", err
	}
	if err := writeJSONFile(filepath.Join(writeDir, "structured_flow.json"), inference.Structured); err != nil {
		return "", err
	}
	if err := writeTSVFile(filepath.Join(writeDir, "callsites.tsv"), []string{"vmOffset", "opcode", "receiver", "function", "paramCount", "confidence", "normalized"}, func() [][]string {
		rows := make([][]string, 0, len(inference.Callsites))
		for _, c := range inference.Callsites {
			rows = append(rows, []string{fmt.Sprintf("%d", c.VMOffset), c.Opcode, c.Receiver, c.Function, fmt.Sprintf("%d", c.ParamCount), c.Confidence, c.Normalized})
		}
		return rows
	}()); err != nil {
		return "", err
	}
	if err := writeTSVFile(filepath.Join(writeDir, "entrypoint_slice.tsv"), []string{"vmOffset", "opcode"}, func() [][]string {
		rows := make([][]string, 0, len(disasm.Instructions))
		reachable := map[int]bool{}
		instOffsets := make(map[int]bool, len(disasm.Instructions))
		for _, inst := range disasm.Instructions {
			instOffsets[inst.VMOffset] = true
		}
		for _, e := range inference.CFG.Edges {
			reachable[e.FromVM] = true
			if instOffsets[e.ToVM] {
				reachable[e.ToVM] = true
			}
		}
		for _, inst := range disasm.Instructions {
			if inference.Slice.Enabled && !reachable[inst.VMOffset] {
				continue
			}
			rows = append(rows, []string{strconv.Itoa(inst.VMOffset), inst.Opcode})
		}
		return rows
	}()); err != nil {
		return "", err
	}

	summary := strings.Builder{}
	summary.WriteString("# infer-pack summary\n\n")
	summary.WriteString(fmt.Sprintf("- Asset: `%s`\n", assetPath))
	summary.WriteString(fmt.Sprintf("- Range source: `%s` (%s, score=%d)\n", selection.RangeSource, selection.RangeConfidence, selection.RangeScore))
	summary.WriteString(fmt.Sprintf("- EntryPoint requested: `%d` selected: `%d` found=%t\n", inference.RequestedEntryPoint, inference.SelectedEntryPoint, inference.Slice.Found))
	summary.WriteString(fmt.Sprintf("- Instruction count: `%d`\n", len(disasm.Instructions)))
	summary.WriteString(fmt.Sprintf("- CFG blocks: `%d`, edges: `%d`\n", len(inference.CFG.Nodes), len(inference.CFG.Edges)))
	summary.WriteString(fmt.Sprintf("- Callsites: `%d`\n", len(inference.Callsites)))
	summary.WriteString(fmt.Sprintf("- Confidence overall: `%s` (%s)\n", inference.Confidence.Overall, inference.Confidence.Reason))
	if len(inference.ResidualTasks) > 0 {
		summary.WriteString("\n## Residual tasks\n")
		for _, task := range inference.ResidualTasks {
			summary.WriteString("- " + task + "\n")
		}
	}
	if err := writeFileAtomically(filepath.Join(writeDir, "SUMMARY.md"), []byte(summary.String()), 0o644); err != nil {
		return "", fmt.Errorf("write SUMMARY.md: %w", err)
	}

	residual := strings.Builder{}
	residual.WriteString("# Residual Tasks\n\n")
	residual.WriteString("以下は今回の実装で未解決、または精度改善が必要な項目です。\n\n")
	for i, task := range inference.ResidualTasks {
		residual.WriteString(fmt.Sprintf("%d. %s\n", i+1, task))
	}
	if err := writeFileAtomically(filepath.Join(writeDir, "RESIDUAL_TASKS.md"), []byte(residual.String()), 0o644); err != nil {
		return "", fmt.Errorf("write RESIDUAL_TASKS.md: %w", err)
	}

	if commitDir {
		if err := os.Rename(writeDir, outDir); err != nil {
			return "", fmt.Errorf("commit output directory: %w", err)
		}
		commitDir = false
	}
	return outDir, nil
}

func writeJSONFile(path string, payload any) error {
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filepath.Base(path), err)
	}
	b = append(b, '\n')
	if err := writeFileAtomically(path, b, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func writeTSVFile(path string, headers []string, rows [][]string) error {
	var b strings.Builder
	if len(headers) > 0 {
		b.WriteString(strings.Join(headers, "\t"))
		b.WriteByte('\n')
	}
	for _, row := range rows {
		b.WriteString(strings.Join(row, "\t"))
		b.WriteByte('\n')
	}
	if err := writeFileAtomically(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
