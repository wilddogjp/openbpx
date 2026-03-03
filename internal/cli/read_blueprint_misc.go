package cli

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

type blueprintBytecodeSelection struct {
	Export           uasset.ExportEntry
	Data             []byte
	DataStart        int
	UsingScriptRange bool
	RangeSource      string
	RangeConfidence  string
	RangeScore       int
	Diagnostics      bytecodeRangeDiagnostics
}

type disasmInstruction struct {
	Offset   int            `json:"offset"`
	VMOffset int            `json:"vmOffset"`
	Token    uint8          `json:"token"`
	Opcode   string         `json:"opcode"`
	Params   map[string]any `json:"params,omitempty"`
}

type disasmResult struct {
	Instructions   []disasmInstruction
	Warnings       []string
	Truncated      bool
	DecodeFailedAt int
	VMSize         int
}

type bytecodeRangeSource string

const (
	bytecodeRangeAuto       bytecodeRangeSource = "auto"
	bytecodeRangeExportMap  bytecodeRangeSource = "export-map"
	bytecodeRangeUStruct    bytecodeRangeSource = "ustruct-script"
	bytecodeRangeSerialFull bytecodeRangeSource = "serial-full"
	defaultRangeSource                          = bytecodeRangeAuto
)

type bytecodeSelectionOptions struct {
	RangeSource bytecodeRangeSource
	StrictRange bool
}

type bytecodeRangeDiagnostics struct {
	Candidates []map[string]any `json:"candidates"`
}

type bytecodeRangeAnalysis struct {
	DataSize              int
	EndOfScriptCount      int
	EndOfScriptLast       bool
	InstructionCount      int
	WarningsCount         int
	LastInstructionToken  uint8
	LastInstructionOffset int
}

func runBlueprint(args []string, stdout, stderr io.Writer) int {
	return dispatchSubcommand(
		args,
		stdout,
		stderr,
		"usage: bpx blueprint <info|bytecode|disasm|trace|call-args|refs|search|scan-functions|infer-pack> ...",
		"unknown blueprint command: %s\n",
		subcommandSpec{Name: "info", Run: runBlueprintInfo},
		subcommandSpec{Name: "bytecode", Run: runBlueprintBytecode},
		subcommandSpec{Name: "disasm", Run: runBlueprintDisasm},
		subcommandSpec{Name: "trace", Run: runBlueprintTrace},
		subcommandSpec{Name: "call-args", Run: runBlueprintCallArgs},
		subcommandSpec{Name: "refs", Run: runBlueprintRefs},
		subcommandSpec{Name: "search", Run: runBlueprintSearch},
		subcommandSpec{Name: "scan-functions", Run: runBlueprintScanFunctions},
		subcommandSpec{Name: "infer-pack", Run: runBlueprintInferPack},
	)
}

func runBlueprintInfo(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("blueprint info", stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("export", 0, "optional 1-based export index")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: bpx blueprint info <file.uasset> [--export <n>]")
		return 1
	}
	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	targets := make([]int, 0, 4)
	if *exportIndex > 0 {
		idx, err := asset.ResolveExportIndex(*exportIndex)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		targets = append(targets, idx)
	} else {
		for i, exp := range asset.Exports {
			if asset.ResolveClassName(exp) == "Blueprint" {
				targets = append(targets, i)
			}
		}
	}

	infos := make([]map[string]any, 0, len(targets))
	functionExportCount := 0
	for _, exp := range asset.Exports {
		if strings.Contains(strings.ToLower(asset.ResolveClassName(exp)), "function") {
			functionExportCount++
		}
	}
	for _, idx := range targets {
		infos = append(infos, exportReadInfo(asset, idx))
	}

	return printJSON(stdout, map[string]any{
		"file":                 file,
		"blueprintExportCount": len(infos),
		"functionExportCount":  functionExportCount,
		"blueprints":           infos,
	})
}

func runBlueprintBytecode(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("blueprint bytecode", stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("export", 0, "1-based export index")
	rangeSourceRaw := fs.String("range-source", string(defaultRangeSource), "bytecode range source: auto, export-map, ustruct-script, serial-full")
	strictRange := fs.Bool("strict-range", false, "treat incomplete bytecode range selection as error")
	diagnostics := fs.Bool("diagnostics", false, "include range selection diagnostics")
	full := fs.Bool("full", false, "include full base64 payload")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || *exportIndex <= 0 {
		fmt.Fprintln(stderr, "usage: bpx blueprint bytecode <file.uasset> --export <n> [--range-source auto|export-map|ustruct-script|serial-full] [--strict-range] [--diagnostics]")
		return 1
	}
	rangeSource, err := parseBytecodeRangeSource(*rangeSourceRaw)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	selection, err := selectBlueprintBytecode(asset, *exportIndex, bytecodeSelectionOptions{
		RangeSource: rangeSource,
		StrictRange: *strictRange,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	resp := map[string]any{
		"file":               file,
		"export":             *exportIndex,
		"objectName":         selection.Export.ObjectName.Display(asset.Names),
		"className":          asset.ResolveClassName(selection.Export),
		"serialOffset":       selection.Export.SerialOffset,
		"serialSize":         selection.Export.SerialSize,
		"scriptStartOffset":  selection.Export.ScriptSerializationStartOffset,
		"scriptEndOffset":    selection.Export.ScriptSerializationEndOffset,
		"usingScriptRange":   selection.UsingScriptRange,
		"rangeSource":        selection.RangeSource,
		"rangeConfidence":    selection.RangeConfidence,
		"rangeScore":         selection.RangeScore,
		"selectedDataOffset": selection.DataStart,
		"selectedDataSize":   len(selection.Data),
	}
	if *diagnostics {
		resp["rangeDiagnostics"] = selection.Diagnostics
	}
	addBase64Data(resp, selection.Data, *full)
	return printJSON(stdout, resp)
}

func runBlueprintDisasm(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("blueprint disasm", stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("export", 0, "1-based export index")
	allowOutputFormats(fs, "output format: json, toml, or text", structuredOutputFormatJSON, structuredOutputFormatTOML, "text")
	analysis := fs.Bool("analysis", false, "include inference-oriented analysis payload")
	entrypoint := fs.Int("entrypoint", -1, "optional entrypoint VM offset for slice analysis")
	maxSteps := fs.Int("max-steps", 20000, "maximum traversal steps for entrypoint slice")
	rangeSourceRaw := fs.String("range-source", string(defaultRangeSource), "bytecode range source: auto, export-map, ustruct-script, serial-full")
	strictRange := fs.Bool("strict-range", false, "treat incomplete bytecode range selection as error")
	diagnostics := fs.Bool("diagnostics", false, "include range selection diagnostics")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || *exportIndex <= 0 {
		fmt.Fprintln(stderr, "usage: bpx blueprint disasm <file.uasset> --export <n> [--format json|toml|text] [--analysis] [--entrypoint <vm>] [--max-steps <n>] [--range-source auto|export-map|ustruct-script|serial-full] [--strict-range] [--diagnostics]")
		return 1
	}
	outFormat := outputFormatFromFlagSet(fs)
	rangeSource, err := parseBytecodeRangeSource(*rangeSourceRaw)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	selection, err := selectBlueprintBytecode(asset, *exportIndex, bytecodeSelectionOptions{
		RangeSource: rangeSource,
		StrictRange: *strictRange,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	disasm := disassembleBytecodeWithAsset(selection.Data, asset.Names, asset)
	objectName := selection.Export.ObjectName.Display(asset.Names)
	displayInstructions := disasm.Instructions
	var inf *disasmInferenceResult
	if *analysis || *entrypoint >= 0 {
		v := analyzeDisasmForInference(
			asset,
			*exportIndex,
			objectName,
			disasm.Instructions,
			disasm.Warnings,
			disasm.Truncated,
			disasm.DecodeFailedAt,
			*entrypoint,
			*maxSteps,
		)
		inf = &v
		if *entrypoint >= 0 && len(v.Slice.ReachableVMOffsets) > 0 {
			reachable := make(map[int]bool, len(v.Slice.ReachableVMOffsets))
			for _, vm := range v.Slice.ReachableVMOffsets {
				reachable[vm] = true
			}
			filtered := make([]disasmInstruction, 0, len(disasm.Instructions))
			for _, inst := range disasm.Instructions {
				if reachable[inst.VMOffset] {
					filtered = append(filtered, inst)
				}
			}
			if len(filtered) > 0 {
				displayInstructions = filtered
			}
		}
	}

	if outFormat == "text" {
		text := renderDisasmText(displayInstructions)
		if _, err := io.WriteString(stdout, text); err != nil {
			fmt.Fprintf(stderr, "error: write output: %v\n", err)
			return 1
		}
		if *analysis && inf != nil {
			fmt.Fprintf(
				stderr,
				"info: analysis overall=%s cfg=%s calls=%s def-use=%s entrypoint(found=%t selected=0x%X)\n",
				inf.Confidence.Overall,
				inf.Confidence.CFG,
				inf.Confidence.Calls,
				inf.Confidence.DefUse,
				inf.Slice.Found,
				inf.SelectedEntryPoint,
			)
		}
		if *entrypoint >= 0 && inf != nil {
			fmt.Fprintf(
				stderr,
				"info: entrypoint slice requested=0x%X selected=0x%X found=%t instructions=%d\n",
				*entrypoint,
				inf.SelectedEntryPoint,
				inf.Slice.Found,
				len(displayInstructions),
			)
		}
		if len(disasm.Warnings) > 0 {
			fmt.Fprintf(stderr, "warning: %s\n", strings.Join(disasm.Warnings, "; "))
		}
		if *diagnostics {
			fmt.Fprintf(
				stderr,
				"info: range source=%s confidence=%s score=%d selected=%dB truncated=%t decodeFailedAt=%d\n",
				selection.RangeSource,
				selection.RangeConfidence,
				selection.RangeScore,
				len(selection.Data),
				disasm.Truncated,
				disasm.DecodeFailedAt,
			)
		}
		return 0
	}

	resp := map[string]any{
		"file":               file,
		"export":             *exportIndex,
		"objectName":         objectName,
		"className":          asset.ResolveClassName(selection.Export),
		"usingScriptRange":   selection.UsingScriptRange,
		"rangeSource":        selection.RangeSource,
		"rangeConfidence":    selection.RangeConfidence,
		"rangeScore":         selection.RangeScore,
		"selectedDataOffset": selection.DataStart,
		"selectedDataSize":   len(selection.Data),
		"instructions":       displayInstructions,
		"warnings":           disasm.Warnings,
		"truncated":          disasm.Truncated,
		"decodeFailedAt":     disasm.DecodeFailedAt,
	}
	if *analysis && inf != nil {
		resp["analysis"] = inf
	}
	if *entrypoint >= 0 && inf != nil {
		resp["entrypointSlice"] = inf.Slice
	}
	if *diagnostics {
		resp["rangeDiagnostics"] = selection.Diagnostics
	}
	return printJSON(stdout, resp)
}

func runBlueprintInferPack(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("blueprint infer-pack", stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("export", 0, "1-based export index")
	entrypoint := fs.Int("entrypoint", -1, "optional entrypoint VM offset for slice analysis")
	maxSteps := fs.Int("max-steps", 20000, "maximum traversal steps for entrypoint slice")
	outDir := fs.String("out", "", "output directory (default: testdata/reports/bpx_infer_pack_<timestamp>)")
	rangeSourceRaw := fs.String("range-source", string(defaultRangeSource), "bytecode range source: auto, export-map, ustruct-script, serial-full")
	strictRange := fs.Bool("strict-range", false, "treat incomplete bytecode range selection as error")
	diagnostics := fs.Bool("diagnostics", false, "include range selection diagnostics in disasm json")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || *exportIndex <= 0 {
		fmt.Fprintln(stderr, "usage: bpx blueprint infer-pack <file.uasset> --export <n> [--entrypoint <vm>] [--max-steps <n>] [--out <dir>] [--range-source auto|export-map|ustruct-script|serial-full] [--strict-range] [--diagnostics]")
		return 1
	}

	rangeSource, err := parseBytecodeRangeSource(*rangeSourceRaw)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	selection, err := selectBlueprintBytecode(asset, *exportIndex, bytecodeSelectionOptions{
		RangeSource: rangeSource,
		StrictRange: *strictRange,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	disasm := disassembleBytecodeWithAsset(selection.Data, asset.Names, asset)
	objectName := selection.Export.ObjectName.Display(asset.Names)
	inference := analyzeDisasmForInference(
		asset,
		*exportIndex,
		objectName,
		disasm.Instructions,
		disasm.Warnings,
		disasm.Truncated,
		disasm.DecodeFailedAt,
		*entrypoint,
		*maxSteps,
	)
	var diag *bytecodeRangeDiagnostics
	if *diagnostics {
		diag = &selection.Diagnostics
	}
	outPath, err := writeInferPackFiles(file, objectName, selection, disasm, inference, *outDir, diag)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	return printJSON(stdout, map[string]any{
		"file":               file,
		"export":             *exportIndex,
		"objectName":         objectName,
		"outDir":             outPath,
		"entrypoint":         *entrypoint,
		"selectedEntrypoint": inference.SelectedEntryPoint,
		"rangeSource":        selection.RangeSource,
		"rangeConfidence":    selection.RangeConfidence,
		"rangeScore":         selection.RangeScore,
		"instructionCount":   len(disasm.Instructions),
		"cfgBlockCount":      len(inference.CFG.Nodes),
		"callsiteCount":      len(inference.Callsites),
		"overallConfidence":  inference.Confidence.Overall,
	})
}

func selectBlueprintBytecode(asset *uasset.Asset, exportIndex int, opts bytecodeSelectionOptions) (blueprintBytecodeSelection, error) {
	idx, err := asset.ResolveExportIndex(exportIndex)
	if err != nil {
		return blueprintBytecodeSelection{}, err
	}
	exp := asset.Exports[idx]
	serialStart := int(exp.SerialOffset)
	serialEnd := int(exp.SerialOffset + exp.SerialSize)
	if serialStart < 0 || serialEnd < serialStart || serialEnd > len(asset.Raw.Bytes) {
		return blueprintBytecodeSelection{}, fmt.Errorf("export serial range out of file bounds")
	}

	if opts.RangeSource == "" {
		opts.RangeSource = defaultRangeSource
	}

	serialData := asset.Raw.Bytes[serialStart:serialEnd]
	type candidate struct {
		Source           bytecodeRangeSource
		Start            int
		End              int
		UsingScriptRange bool
		Meta             map[string]any
	}
	candidates := make([]candidate, 0, 3)

	addCandidate := func(c candidate) {
		if c.Start < serialStart || c.End > serialEnd || c.End <= c.Start {
			return
		}
		candidates = append(candidates, c)
	}

	if opts.RangeSource == bytecodeRangeAuto || opts.RangeSource == bytecodeRangeExportMap {
		if exp.ScriptSerializationEndOffset > exp.ScriptSerializationStartOffset &&
			exp.ScriptSerializationStartOffset >= 0 &&
			exp.ScriptSerializationEndOffset <= exp.SerialSize {
			start := int(exp.SerialOffset + exp.ScriptSerializationStartOffset)
			end := int(exp.SerialOffset + exp.ScriptSerializationEndOffset)
			addCandidate(candidate{
				Source:           bytecodeRangeExportMap,
				Start:            start,
				End:              end,
				UsingScriptRange: true,
				Meta: map[string]any{
					"scriptStartOffset": exp.ScriptSerializationStartOffset,
					"scriptEndOffset":   exp.ScriptSerializationEndOffset,
				},
			})
		}
	}

	if opts.RangeSource == bytecodeRangeAuto || opts.RangeSource == bytecodeRangeUStruct {
		relStart, relEnd, bytecodeBufferSize, serializedScriptSize, ok := inferUStructScriptRange(serialData, asset.Names)
		if ok {
			addCandidate(candidate{
				Source:           bytecodeRangeUStruct,
				Start:            serialStart + relStart,
				End:              serialStart + relEnd,
				UsingScriptRange: false,
				Meta: map[string]any{
					"headerOffsetWithinExport": relStart - 8,
					"bytecodeBufferSize":       bytecodeBufferSize,
					"serializedScriptSize":     serializedScriptSize,
				},
			})
		}
	}

	if opts.RangeSource == bytecodeRangeAuto || opts.RangeSource == bytecodeRangeSerialFull {
		addCandidate(candidate{
			Source:           bytecodeRangeSerialFull,
			Start:            serialStart,
			End:              serialEnd,
			UsingScriptRange: false,
		})
	}

	if len(candidates) == 0 {
		return blueprintBytecodeSelection{}, fmt.Errorf("no bytecode range candidate available for source=%s", opts.RangeSource)
	}

	diagnostics := bytecodeRangeDiagnostics{Candidates: make([]map[string]any, 0, len(candidates))}
	bestIdx := -1
	bestScore := -1 << 30
	bestConfidence := "low"
	for i, c := range candidates {
		data := asset.Raw.Bytes[c.Start:c.End]
		analysis := analyzeBytecodeRange(data, asset.Names)
		score, confidence := scoreBytecodeRange(c.Source, analysis)
		diag := map[string]any{
			"source":                c.Source,
			"start":                 c.Start,
			"end":                   c.End,
			"size":                  len(data),
			"score":                 score,
			"confidence":            confidence,
			"endOfScriptCount":      analysis.EndOfScriptCount,
			"endOfScriptAtLast":     analysis.EndOfScriptLast,
			"instructionCount":      analysis.InstructionCount,
			"warningsCount":         analysis.WarningsCount,
			"lastInstructionToken":  analysis.LastInstructionToken,
			"lastInstructionOffset": analysis.LastInstructionOffset,
		}
		if c.Meta != nil {
			diag["meta"] = c.Meta
		}
		diagnostics.Candidates = append(diagnostics.Candidates, diag)
		if score > bestScore {
			bestScore = score
			bestConfidence = confidence
			bestIdx = i
		}
	}

	if bestIdx < 0 {
		return blueprintBytecodeSelection{}, fmt.Errorf("failed to score bytecode range candidates")
	}
	selected := candidates[bestIdx]
	selectedData := asset.Raw.Bytes[selected.Start:selected.End]
	if opts.StrictRange && (len(selectedData) == 0 || selectedData[len(selectedData)-1] != 0x53) {
		return blueprintBytecodeSelection{}, fmt.Errorf(
			"incomplete bytecode range selected (source=%s size=%d): EX_EndOfScript not at range end",
			selected.Source,
			len(selectedData),
		)
	}

	return blueprintBytecodeSelection{
		Export:           exp,
		Data:             selectedData,
		DataStart:        selected.Start,
		UsingScriptRange: selected.UsingScriptRange,
		RangeSource:      string(selected.Source),
		RangeConfidence:  bestConfidence,
		RangeScore:       bestScore,
		Diagnostics:      diagnostics,
	}, nil
}

func parseBytecodeRangeSource(raw string) (bytecodeRangeSource, error) {
	switch bytecodeRangeSource(strings.ToLower(strings.TrimSpace(raw))) {
	case bytecodeRangeAuto:
		return bytecodeRangeAuto, nil
	case bytecodeRangeExportMap:
		return bytecodeRangeExportMap, nil
	case bytecodeRangeUStruct:
		return bytecodeRangeUStruct, nil
	case bytecodeRangeSerialFull:
		return bytecodeRangeSerialFull, nil
	default:
		return "", fmt.Errorf("unsupported range source: %s", raw)
	}
}

func inferUStructScriptRange(serialData []byte, names []uasset.NameEntry) (start, end, bytecodeBufferSize, serializedScriptSize int, ok bool) {
	if len(serialData) < 9 {
		return 0, 0, 0, 0, false
	}

	bestStart := 0
	bestEnd := 0
	bestBytecodeSize := 0
	bestSerializedSize := 0
	bestTail := len(serialData) + 1
	found := false

	for pos := 0; pos+8 <= len(serialData); pos++ {
		bytecodeSize := int(int32(binary.LittleEndian.Uint32(serialData[pos : pos+4])))
		storageSize := int(int32(binary.LittleEndian.Uint32(serialData[pos+4 : pos+8])))
		if bytecodeSize <= 0 || storageSize <= 0 || bytecodeSize > len(serialData) {
			continue
		}
		rangeStart := pos + 8
		rangeEnd := rangeStart + storageSize
		if rangeEnd > len(serialData) {
			continue
		}
		if serialData[rangeEnd-1] != 0x53 {
			continue
		}

		disasm := disassembleBytecode(serialData[rangeStart:rangeEnd], names)
		if disasm.Truncated || len(disasm.Instructions) == 0 {
			continue
		}
		if disasm.Instructions[len(disasm.Instructions)-1].Token != 0x53 {
			continue
		}
		if disasm.VMSize != bytecodeSize {
			continue
		}

		tail := len(serialData) - rangeEnd
		if !found || tail < bestTail || (tail == bestTail && storageSize > bestSerializedSize) {
			found = true
			bestTail = tail
			bestStart = rangeStart
			bestEnd = rangeEnd
			bestBytecodeSize = bytecodeSize
			bestSerializedSize = storageSize
		}
	}

	if !found {
		return 0, 0, 0, 0, false
	}
	return bestStart, bestEnd, bestBytecodeSize, bestSerializedSize, true
}

func analyzeBytecodeRange(data []byte, names []uasset.NameEntry) bytecodeRangeAnalysis {
	res := disassembleBytecode(data, names)
	analysis := bytecodeRangeAnalysis{
		DataSize:         len(data),
		InstructionCount: len(res.Instructions),
		WarningsCount:    len(res.Warnings),
	}
	for _, inst := range res.Instructions {
		if inst.Token == 0x53 {
			analysis.EndOfScriptCount++
		}
	}
	if len(res.Instructions) > 0 {
		last := res.Instructions[len(res.Instructions)-1]
		analysis.LastInstructionToken = last.Token
		analysis.LastInstructionOffset = last.Offset
	}
	analysis.EndOfScriptLast = len(data) > 0 && data[len(data)-1] == 0x53
	return analysis
}

func scoreBytecodeRange(source bytecodeRangeSource, analysis bytecodeRangeAnalysis) (int, string) {
	score := 0
	switch source {
	case bytecodeRangeUStruct:
		score += 15
	case bytecodeRangeSerialFull:
		score += 2
	case bytecodeRangeExportMap:
		score -= 5
	}
	if analysis.EndOfScriptLast {
		score += 80
	} else if analysis.EndOfScriptCount > 0 {
		score += 25
	}
	if analysis.DataSize >= 16 {
		score += 8
	} else {
		score -= 20
	}
	if analysis.EndOfScriptCount == 1 {
		score += 4
	} else if analysis.EndOfScriptCount > 6 {
		score -= 10
	}
	if analysis.WarningsCount == 0 {
		score += 5
	} else {
		score -= analysis.WarningsCount
	}
	if analysis.InstructionCount > 8 {
		score += 3
	}

	confidence := "low"
	if analysis.EndOfScriptLast && analysis.DataSize >= 16 {
		confidence = "high"
	} else if analysis.EndOfScriptCount > 0 && analysis.DataSize >= 16 {
		confidence = "medium"
	}
	return score, confidence
}

func disassembleBytecode(data []byte, names []uasset.NameEntry) disasmResult {
	return disassembleBytecodeWithAsset(data, names, nil)
}

func renderDisasmText(instructions []disasmInstruction) string {
	var b strings.Builder
	for _, inst := range instructions {
		if inst.VMOffset > 0 || inst.Offset == 0 {
			fmt.Fprintf(&b, "0x%04X [vm=0x%04X]  %s", inst.Offset, inst.VMOffset, inst.Opcode)
		} else {
			fmt.Fprintf(&b, "0x%04X  %s", inst.Offset, inst.Opcode)
		}
		if len(inst.Params) > 0 {
			body, err := json.Marshal(inst.Params)
			if err == nil {
				fmt.Fprintf(&b, "  %s", string(body))
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func readNullTerminatedString(data []byte, start int) (string, int, bool) {
	if start < 0 || start > len(data) {
		return "", start, false
	}
	for i := start; i < len(data); i++ {
		if data[i] == 0 {
			return string(data[start:i]), i + 1, true
		}
	}
	return "", len(data), false
}

func readNullTerminatedUTF16String(data []byte, start int) (string, int, bool) {
	if start < 0 || start > len(data) {
		return "", start, false
	}
	units := make([]uint16, 0, 16)
	for i := start; i+1 < len(data); i += 2 {
		u := binary.LittleEndian.Uint16(data[i : i+2])
		if u == 0 {
			return string(utf16.Decode(units)), i + 2, true
		}
		units = append(units, u)
	}
	return "", len(data), false
}

func kismetOpcodeName(token byte) string {
	if name, ok := kismetOpcodeNames[token]; ok {
		return name
	}
	return fmt.Sprintf("EX_Unknown(0x%02X)", token)
}

var kismetOpcodeNames = map[byte]string{
	0x00: "EX_LocalVariable",
	0x01: "EX_InstanceVariable",
	0x02: "EX_DefaultVariable",
	0x04: "EX_Return",
	0x06: "EX_Jump",
	0x07: "EX_JumpIfNot",
	0x09: "EX_Assert",
	0x0B: "EX_Nothing",
	0x0C: "EX_NothingInt32",
	0x0F: "EX_Let",
	0x11: "EX_BitFieldConst",
	0x12: "EX_ClassContext",
	0x13: "EX_MetaCast",
	0x14: "EX_LetBool",
	0x15: "EX_EndParmValue",
	0x16: "EX_EndFunctionParms",
	0x17: "EX_Self",
	0x18: "EX_Skip",
	0x19: "EX_Context",
	0x1A: "EX_Context_FailSilent",
	0x1B: "EX_VirtualFunction",
	0x1C: "EX_FinalFunction",
	0x1D: "EX_IntConst",
	0x1E: "EX_FloatConst",
	0x1F: "EX_StringConst",
	0x20: "EX_ObjectConst",
	0x21: "EX_NameConst",
	0x22: "EX_RotationConst",
	0x23: "EX_VectorConst",
	0x24: "EX_ByteConst",
	0x25: "EX_IntZero",
	0x26: "EX_IntOne",
	0x27: "EX_True",
	0x28: "EX_False",
	0x29: "EX_TextConst",
	0x2A: "EX_NoObject",
	0x2B: "EX_TransformConst",
	0x2C: "EX_IntConstByte",
	0x2D: "EX_NoInterface",
	0x2E: "EX_DynamicCast",
	0x2F: "EX_StructConst",
	0x30: "EX_EndStructConst",
	0x31: "EX_SetArray",
	0x32: "EX_EndArray",
	0x33: "EX_PropertyConst",
	0x34: "EX_UnicodeStringConst",
	0x35: "EX_Int64Const",
	0x36: "EX_UInt64Const",
	0x37: "EX_DoubleConst",
	0x38: "EX_Cast",
	0x39: "EX_SetSet",
	0x3A: "EX_EndSet",
	0x3B: "EX_SetMap",
	0x3C: "EX_EndMap",
	0x3D: "EX_SetConst",
	0x3E: "EX_EndSetConst",
	0x3F: "EX_MapConst",
	0x40: "EX_EndMapConst",
	0x41: "EX_Vector3fConst",
	0x42: "EX_StructMemberContext",
	0x43: "EX_LetMulticastDelegate",
	0x44: "EX_LetDelegate",
	0x45: "EX_LocalVirtualFunction",
	0x46: "EX_LocalFinalFunction",
	0x48: "EX_LocalOutVariable",
	0x4A: "EX_DeprecatedOp4A",
	0x4B: "EX_InstanceDelegate",
	0x4C: "EX_PushExecutionFlow",
	0x4D: "EX_PopExecutionFlow",
	0x4E: "EX_ComputedJump",
	0x4F: "EX_PopExecutionFlowIfNot",
	0x50: "EX_Breakpoint",
	0x51: "EX_InterfaceContext",
	0x52: "EX_ObjToInterfaceCast",
	0x53: "EX_EndOfScript",
	0x54: "EX_CrossInterfaceCast",
	0x55: "EX_InterfaceToObjCast",
	0x5A: "EX_WireTracepoint",
	0x5B: "EX_SkipOffsetConst",
	0x5C: "EX_AddMulticastDelegate",
	0x5D: "EX_ClearMulticastDelegate",
	0x5E: "EX_Tracepoint",
	0x5F: "EX_LetObj",
	0x60: "EX_LetWeakObjPtr",
	0x61: "EX_BindDelegate",
	0x62: "EX_RemoveMulticastDelegate",
	0x63: "EX_CallMulticastDelegate",
	0x64: "EX_LetValueOnPersistentFrame",
	0x65: "EX_ArrayConst",
	0x66: "EX_EndArrayConst",
	0x67: "EX_SoftObjectConst",
	0x68: "EX_CallMath",
	0x69: "EX_SwitchValue",
	0x6A: "EX_InstrumentationEvent",
	0x6B: "EX_ArrayGetByRef",
	0x6C: "EX_ClassSparseDataVariable",
	0x6D: "EX_FieldPathConst",
	0x70: "EX_AutoRtfmTransact",
	0x71: "EX_AutoRtfmStopTransact",
	0x72: "EX_AutoRtfmAbortIfNot",
}

func runBlueprintScanFunctions(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("blueprint scan-functions", stderr)
	opts := registerCommonFlags(fs)
	pattern := fs.String("pattern", "*.uasset", "glob pattern")
	recursive := fs.Bool("recursive", true, "scan recursively")
	nameLike := fs.String("name-like", "", "optional regular expression for function names")
	aggregate := fs.Bool("aggregate", true, "include aggregate counts")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: bpx blueprint scan-functions <directory> --recursive [--name-like <regex>] [--aggregate]")
		return 1
	}
	rootDir := fs.Arg(0)
	files, err := uasset.CollectAssetFiles(rootDir, *pattern, *recursive)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	var re *regexp.Regexp
	if *nameLike != "" {
		re, err = regexp.Compile(*nameLike)
		if err != nil {
			fmt.Fprintf(stderr, "error: compile regex: %v\n", err)
			return 1
		}
	}

	functionNameCounts := map[string]int{}
	functionDirCounts := map[string]int{}
	fileToFunctions := map[string][]string{}
	parseFailures := make([]map[string]string, 0, 8)
	functionFileCount := 0

	for _, file := range files {
		asset, err := uasset.ParseFile(file, *opts)
		if err != nil {
			parseFailures = append(parseFailures, map[string]string{"file": file, "error": err.Error()})
			continue
		}
		functions := make([]string, 0, 16)
		for _, exp := range asset.Exports {
			className := strings.ToLower(asset.ResolveClassName(exp))
			if !strings.Contains(className, "function") {
				continue
			}
			name := exp.ObjectName.Display(asset.Names)
			if re != nil && !re.MatchString(name) {
				continue
			}
			functions = append(functions, name)
		}
		if len(functions) == 0 {
			continue
		}

		sort.Strings(functions)
		functionFileCount++
		fileToFunctions[file] = functions
		rel, relErr := filepath.Rel(rootDir, file)
		dirKey := filepath.Dir(file)
		if relErr == nil {
			dirKey = topDirFromRelative(rel)
		}
		for _, fn := range functions {
			functionNameCounts[fn]++
			functionDirCounts[dirKey]++
		}
	}

	resp := map[string]any{
		"directory":         rootDir,
		"pattern":           *pattern,
		"recursive":         *recursive,
		"nameLike":          *nameLike,
		"aggregate":         *aggregate,
		"fileCount":         len(files),
		"functionFileCount": functionFileCount,
		"parseFailCount":    len(parseFailures),
		"parseFailures":     parseFailures,
		"fileToFunctions":   fileToFunctions,
	}
	if *aggregate {
		resp["functionNameCounts"] = functionNameCounts
		resp["functionDirCounts"] = functionDirCounts
	}
	return printJSON(stdout, resp)
}

func runEnum(args []string, stdout, stderr io.Writer) int {
	return dispatchSubcommand(
		args,
		stdout,
		stderr,
		"usage: bpx enum <list|write-value> ...",
		"unknown enum command: %s\n",
		subcommandSpec{Name: "list", Run: runEnumList},
		subcommandSpec{Name: "write-value", Run: runEnumWriteValue},
	)
}

func runEnumList(args []string, stdout, stderr io.Writer) int {
	file, asset, ok := parseSingleAssetCommand(args, "enum list", "usage: bpx enum list <file.uasset>", stderr)
	if !ok {
		return 1
	}
	items := collectExportsByClassKeyword(asset, []string{"enum"})
	return printJSON(stdout, map[string]any{
		"file":  file,
		"count": len(items),
		"enums": items,
	})
}

func runStruct(args []string, stdout, stderr io.Writer) int {
	return dispatchSubcommand(
		args,
		stdout,
		stderr,
		"usage: bpx struct <definition|details> ...",
		"unknown struct command: %s\n",
		subcommandSpec{Name: "definition", Run: runStructDefinition},
		subcommandSpec{Name: "details", Run: runStructDetails},
	)
}

func runStructDefinition(args []string, stdout, stderr io.Writer) int {
	file, asset, ok := parseSingleAssetCommand(args, "struct definition", "usage: bpx struct definition <file.uasset>", stderr)
	if !ok {
		return 1
	}
	items := collectExportsByClassKeyword(asset, []string{"struct"})
	return printJSON(stdout, map[string]any{
		"file":    file,
		"count":   len(items),
		"structs": items,
	})
}

func runStructDetails(args []string, stdout, stderr io.Writer) int {
	return runGenericExportInfo(args, stdout, stderr, "struct details", "usage: bpx struct details <file.uasset> --export <n>")
}

func runStringTable(args []string, stdout, stderr io.Writer) int {
	return dispatchSubcommand(
		args,
		stdout,
		stderr,
		"usage: bpx stringtable <read|write-entry|remove-entry|set-namespace> ...",
		"unknown stringtable command: %s\n",
		subcommandSpec{Name: "read", Run: runStringTableRead},
		subcommandSpec{Name: "write-entry", Run: runStringTableWriteEntry},
		subcommandSpec{Name: "remove-entry", Run: runStringTableRemoveEntry},
		subcommandSpec{Name: "set-namespace", Run: runStringTableSetNamespace},
	)
}

func runStringTableRead(args []string, stdout, stderr io.Writer) int {
	file, asset, ok := parseSingleAssetCommand(args, "stringtable read", "usage: bpx stringtable read <file.uasset>", stderr)
	if !ok {
		return 1
	}
	items := collectExportsByClassKeyword(asset, []string{"stringtable"})
	return printJSON(stdout, map[string]any{
		"file":         file,
		"stringTables": items,
		"count":        len(items),
	})
}

func runClass(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: bpx class <file.uasset> --export <n>")
		return 1
	}
	return runGenericExportInfo(args, stdout, stderr, "class", "usage: bpx class <file.uasset> --export <n>")
}

func runLevel(args []string, stdout, stderr io.Writer) int {
	return dispatchSubcommand(
		args,
		stdout,
		stderr,
		"usage: bpx level <info|var-list|var-set> ...",
		"unknown level command: %s\n",
		subcommandSpec{Name: "info", Run: runLevelInfo},
		subcommandSpec{Name: "var-list", Run: runLevelVarList},
		subcommandSpec{Name: "var-set", Run: runLevelVarSet},
	)
}

func runLevelInfo(args []string, stdout, stderr io.Writer) int {
	return runGenericExportInfo(args, stdout, stderr, "level info", "usage: bpx level info <file.umap> --export <n>")
}

func runRaw(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: bpx raw <file.uasset> --export <n>")
		return 1
	}
	fs := newFlagSet("raw", stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("export", 0, "1-based export index")
	full := fs.Bool("full", false, "include full base64 payload")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || *exportIndex <= 0 {
		fmt.Fprintln(stderr, "usage: bpx raw <file.uasset> --export <n>")
		return 1
	}
	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	idx, err := asset.ResolveExportIndex(*exportIndex)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	exp := asset.Exports[idx]
	start := int(exp.SerialOffset)
	end := int(exp.SerialOffset + exp.SerialSize)
	if start < 0 || end < start || end > len(asset.Raw.Bytes) {
		fmt.Fprintln(stderr, "error: export serial range out of file bounds")
		return 1
	}
	data := asset.Raw.Bytes[start:end]
	resp := map[string]any{
		"file":         file,
		"export":       *exportIndex,
		"objectName":   exp.ObjectName.Display(asset.Names),
		"className":    asset.ResolveClassName(exp),
		"serialOffset": exp.SerialOffset,
		"serialSize":   exp.SerialSize,
	}
	addBase64Data(resp, data, *full)
	return printJSON(stdout, resp)
}

func runMetadata(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: bpx metadata <file.uasset> --export <n> | bpx metadata <set-root|set-object> ...")
		return 1
	}
	switch args[0] {
	case "set-root":
		return runMetadataSetRoot(args[1:], stdout, stderr)
	case "set-object":
		return runMetadataSetObject(args[1:], stdout, stderr)
	}
	return runGenericExportInfo(args, stdout, stderr, "metadata", "usage: bpx metadata <file.uasset> --export <n>")
}

func runGenericExportInfo(args []string, stdout, stderr io.Writer, command, usage string) int {
	fs := newFlagSet(command, stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("export", 0, "1-based export index")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || *exportIndex <= 0 {
		fmt.Fprintln(stderr, usage)
		return 1
	}
	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	idx, err := asset.ResolveExportIndex(*exportIndex)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	return printJSON(stdout, map[string]any{
		"file":   file,
		"export": exportReadInfo(asset, idx),
	})
}
