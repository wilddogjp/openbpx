package cli

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

// ---------------------------------------------------------------------------
// bpx blueprint widget-read
// ---------------------------------------------------------------------------

func runBlueprintWidgetRead(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("blueprint widget-read", stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("export", 0, "optional 1-based WidgetBlueprint export index")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: bpx blueprint widget-read <file.uasset> [--export <n>]")
		return 1
	}
	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	bpExports := findWidgetBlueprintExports(asset)
	if *exportIndex > 0 {
		idx, err := asset.ResolveExportIndex(*exportIndex)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		found := false
		for _, bpIdx := range bpExports {
			if bpIdx == idx {
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(stderr, "error: export %d is not a WidgetBlueprint (class: %s)\n",
				*exportIndex, asset.ResolveClassName(asset.Exports[idx]))
			return 1
		}
		bpExports = []int{idx}
	}

	blueprints := make([]map[string]any, 0, len(bpExports))
	for _, bpIdx := range bpExports {
		entry, err := buildWidgetBlueprintEntry(asset, bpIdx)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		blueprints = append(blueprints, entry)
	}

	return printJSON(stdout, map[string]any{
		"file":             file,
		"widgetBlueprints": blueprints,
	})
}

// ---------------------------------------------------------------------------
// WidgetBlueprint discovery
// ---------------------------------------------------------------------------

// findWidgetBlueprintExports returns 0-based export indices whose class is
// "WidgetBlueprint".
func findWidgetBlueprintExports(asset *uasset.Asset) []int {
	var out []int
	for i, exp := range asset.Exports {
		if strings.EqualFold(asset.ResolveClassName(exp), "WidgetBlueprint") {
			out = append(out, i)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Build one WidgetBlueprint entry
// ---------------------------------------------------------------------------

func buildWidgetBlueprintEntry(asset *uasset.Asset, bpIdx int) (map[string]any, error) {
	exp := asset.Exports[bpIdx]
	bpProps := asset.ParseExportProperties(bpIdx)

	// Decode blueprint-level properties to find WidgetTree and GeneratedClass refs.
	decoded := decodeAllProperties(asset, bpProps.Properties)

	// Find GeneratedClass export (needed to locate the generated WidgetTree).
	genClassIdx := -1
	if v, ok := decoded["GeneratedClass"]; ok {
		genClassIdx = widgetExportIndexFromDecoded(v) - 1 // to 0-based
	}

	// Collect WidgetTree exports associated with this blueprint.
	treeIndices := findWidgetTreeExports(asset, bpIdx, genClassIdx)

	trees := make([]map[string]any, 0, len(treeIndices))
	for _, treeIdx := range treeIndices {
		treeEntry, err := buildWidgetTreeEntry(asset, bpIdx, genClassIdx, treeIdx)
		if err != nil {
			return nil, fmt.Errorf("tree export %d: %w", treeIdx+1, err)
		}
		trees = append(trees, treeEntry)
	}
	logicalWidgets, err := buildLogicalWidgetEntries(asset, bpIdx, genClassIdx, treeIndices)
	if err != nil {
		return nil, fmt.Errorf("logical widgets: %w", err)
	}

	entry := map[string]any{
		"export":             bpIdx + 1,
		"objectName":         exp.ObjectName.Display(asset.Names),
		"className":          asset.ResolveClassName(exp),
		"properties":         toPropertyOutputs(asset, bpProps.Properties, true),
		"treeCount":          len(trees),
		"widgetTrees":        trees,
		"logicalWidgetCount": len(logicalWidgets),
		"logicalWidgets":     logicalWidgets,
		"warnings":           bpProps.Warnings,
	}
	if genClassIdx >= 0 && genClassIdx < len(asset.Exports) {
		genClassExp := asset.Exports[genClassIdx]
		entry["generatedClassExport"] = genClassIdx + 1
		entry["generatedClassObjectName"] = genClassExp.ObjectName.Display(asset.Names)
		entry["generatedClassClassName"] = asset.ResolveClassName(genClassExp)
	}
	return entry, nil
}

type widgetLogicalTreeState struct {
	role         string
	rootless     bool
	state        *widgetHierarchyState
	exportByPath map[string]int
	ownedExports map[int]bool
}

type widgetLogicalRecord struct {
	path          string
	objectName    string
	className     string
	widgetExports []int
	slotExports   []int
	presentRoles  []string
	slotOnlyRoles []string
	parentPath    string
	childOrdinal  int
	hasOrdinal    bool
	summaryRole   string
	widgetSummary map[string]any
	slotSummary   map[string]any
}

func buildLogicalWidgetEntries(asset *uasset.Asset, bpIdx, genClassIdx int, treeIndices []int) ([]map[string]any, error) {

	treeStates := make([]widgetLogicalTreeState, 0, len(treeIndices))
	records := map[string]*widgetLogicalRecord{}

	for _, treeIdx := range treeIndices {
		treeExp := asset.Exports[treeIdx]
		outerIdx := resolveOuterExportIndex(asset, treeExp)
		outerClassName := ""
		if outerIdx >= 0 && outerIdx < len(asset.Exports) {
			outerClassName = asset.ResolveClassName(asset.Exports[outerIdx])
		}
		role := widgetTreeRole(bpIdx, genClassIdx, outerIdx, outerClassName)

		treeProps := asset.ParseExportProperties(treeIdx)
		treeDecoded := decodeAllProperties(asset, treeProps.Properties)
		rootWidgetExport := 0
		if v, ok := treeDecoded["RootWidget"]; ok {
			rootWidgetExport = widgetExportIndexFromDecoded(v)
		}
		allWidgetExports := normalizeWidgetExportList(rootWidgetExport, widgetExportIndicesFromDecodedArray(treeDecoded["AllWidgets"]))
		state, err := buildWidgetHierarchyState(asset, rootWidgetExport, allWidgetExports)
		if err != nil {
			return nil, fmt.Errorf("tree export %d: %w", treeIdx+1, err)
		}

		exportByPath := make(map[string]int, len(state.orderedExports))
		ownedExports := make(map[int]bool, len(state.orderedExports))
		for _, exportIdx := range state.orderedExports {
			ownedExports[exportIdx] = true
			path := state.paths[exportIdx]
			if path != "" {
				exportByPath[strings.ToLower(path)] = exportIdx
			}
		}
		treeStates = append(treeStates, widgetLogicalTreeState{
			role:         role,
			rootless:     role == "generated" && state.rootWidgetExport == 0,
			state:        state,
			exportByPath: exportByPath,
			ownedExports: ownedExports,
		})

		for _, exportIdx := range state.orderedExports {
			w := state.widgetByExport[exportIdx]
			if w == nil {
				continue
			}
			path := state.paths[exportIdx]
			if path == "" {
				continue
			}
			key := strings.ToLower(path)
			rec := records[key]
			if rec == nil {
				rec = &widgetLogicalRecord{
					path:       path,
					objectName: w.objectName,
					className:  w.className,
				}
				records[key] = rec
			}
			if widgetReadSummaryRoleRank(role) > widgetReadSummaryRoleRank(rec.summaryRole) {
				rec.summaryRole = role
				rec.widgetSummary = widgetReadWidgetSummary(asset, w.className, w.decoded)
				if slotInfo, ok := state.slotByChild[exportIdx]; ok {
					rec.slotSummary = widgetReadSlotSummary(slotInfo.decoded)
				} else {
					rec.slotSummary = nil
				}
			}
			rec.widgetExports = appendUniqueInt(rec.widgetExports, exportIdx)
			if w.slotExport > 0 {
				rec.slotExports = appendUniqueInt(rec.slotExports, w.slotExport)
			}
			rec.presentRoles = appendUniqueString(rec.presentRoles, role)
			if parentIdx, ok := state.parentOf[exportIdx]; ok {
				parentPath := state.paths[parentIdx]
				if parentPath != "" {
					for childOrdinal, childExport := range state.childrenOf[parentIdx] {
						if childExport == exportIdx {
							rec.parentPath = parentPath
							rec.childOrdinal = childOrdinal
							rec.hasOrdinal = true
							break
						}
					}
				}
			}
		}
	}

	for _, rec := range records {
		if !rec.hasOrdinal {
			continue
		}
		for _, tree := range treeStates {
			if containsStringFold(rec.presentRoles, tree.role) {
				continue
			}
			parentExport := tree.exportByPath[strings.ToLower(rec.parentPath)]
			if parentExport <= 0 {
				continue
			}
			parentWidget := tree.state.widgetByExport[parentExport]
			if parentWidget == nil {
				continue
			}
			slotRefs := widgetExportIndicesFromDecodedArray(parentWidget.decoded["Slots"])
			if rec.childOrdinal < 0 || rec.childOrdinal >= len(slotRefs) {
				continue
			}
			slotExport := slotRefs[rec.childOrdinal]
			if slotExport <= 0 {
				continue
			}
			rec.slotExports = appendUniqueInt(rec.slotExports, slotExport)
			rec.slotOnlyRoles = appendUniqueString(rec.slotOnlyRoles, tree.role)
		}
	}

	mergeLogicalWidgetRecordsByPathSuffix(records, treeStates)

	keys := make([]string, 0, len(records))
	for key := range records {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	out := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		rec := records[key]
		slices.Sort(rec.widgetExports)
		slices.Sort(rec.slotExports)
		slices.Sort(rec.presentRoles)
		slices.Sort(rec.slotOnlyRoles)

		entry := map[string]any{
			"path":          rec.path,
			"objectName":    rec.objectName,
			"className":     rec.className,
			"widgetExports": append([]int(nil), rec.widgetExports...),
			"slotExports":   append([]int(nil), rec.slotExports...),
			"presentIn":     append([]string(nil), rec.presentRoles...),
		}
		if len(rec.slotOnlyRoles) > 0 {
			entry["slotOnlyIn"] = append([]string(nil), rec.slotOnlyRoles...)
		}
		mergeWidgetReadSummary(entry, rec.widgetSummary)
		mergeWidgetReadSummary(entry, rec.slotSummary)
		out = append(out, entry)
	}
	return out, nil
}

func mergeLogicalWidgetRecordsByPathSuffix(records map[string]*widgetLogicalRecord, treeStates []widgetLogicalTreeState) {
	if len(records) == 0 {
		return
	}

	keys := make([]string, 0, len(records))
	for key := range records {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	for _, generatedKey := range keys {
		generated := records[generatedKey]
		if generated == nil || !logicalWidgetRecordPresentOnlyIn(generated, "generated") {
			continue
		}

		candidateKey := ""
		for _, designerKey := range keys {
			designer := records[designerKey]
			if designer == nil || !logicalWidgetRecordPresentOnlyIn(designer, "designer") {
				continue
			}
			if !widgetLogicalRecordUsesRootlessGeneratedPath(generated, treeStates) {
				continue
			}
			if !strings.EqualFold(designer.objectName, generated.objectName) || !strings.EqualFold(designer.className, generated.className) {
				continue
			}
			if !widgetPathHasSegmentSuffix(designer.path, generated.path) {
				continue
			}
			if len(splitWidgetPathSegments(designer.path)) != len(splitWidgetPathSegments(generated.path))+1 {
				continue
			}
			if candidateKey != "" {
				candidateKey = ""
				break
			}
			candidateKey = designerKey
		}
		if candidateKey == "" {
			continue
		}

		designer := records[candidateKey]
		designer.widgetExports = appendUniqueIntSlice(designer.widgetExports, generated.widgetExports)
		designer.slotExports = appendUniqueIntSlice(designer.slotExports, generated.slotExports)
		designer.presentRoles = appendUniqueStringSlice(designer.presentRoles, generated.presentRoles)
		designer.slotOnlyRoles = appendUniqueStringSlice(designer.slotOnlyRoles, generated.slotOnlyRoles)
		if widgetReadSummaryRoleRank(generated.summaryRole) > widgetReadSummaryRoleRank(designer.summaryRole) {
			designer.summaryRole = generated.summaryRole
			designer.widgetSummary = generated.widgetSummary
			designer.slotSummary = generated.slotSummary
		}
		delete(records, generatedKey)
	}
}

func widgetLogicalRecordUsesRootlessGeneratedPath(rec *widgetLogicalRecord, treeStates []widgetLogicalTreeState) bool {
	if rec == nil {
		return false
	}
	for _, tree := range treeStates {
		if tree.role != "generated" || !tree.rootless {
			continue
		}
		for _, exportIdx := range rec.widgetExports {
			if tree.ownedExports[exportIdx] {
				return true
			}
		}
	}
	return false
}

func logicalWidgetRecordPresentOnlyIn(rec *widgetLogicalRecord, role string) bool {
	if rec == nil || len(rec.presentRoles) != 1 {
		return false
	}
	return strings.EqualFold(rec.presentRoles[0], role)
}

func widgetPathHasSegmentSuffix(fullPath, suffix string) bool {
	full := splitWidgetPathSegments(fullPath)
	tail := splitWidgetPathSegments(suffix)
	if len(full) == 0 || len(tail) == 0 || len(tail) > len(full) {
		return false
	}
	offset := len(full) - len(tail)
	for i := range tail {
		if !strings.EqualFold(full[offset+i], tail[i]) {
			return false
		}
	}
	return true
}

func splitWidgetPathSegments(path string) []string {
	raw := strings.Split(strings.TrimSpace(path), "/")
	out := make([]string, 0, len(raw))
	for _, segment := range raw {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		out = append(out, segment)
	}
	return out
}

func appendUniqueIntSlice(dst []int, values []int) []int {
	for _, value := range values {
		dst = appendUniqueInt(dst, value)
	}
	return dst
}

func appendUniqueStringSlice(dst []string, values []string) []string {
	for _, value := range values {
		dst = appendUniqueString(dst, value)
	}
	return dst
}

// ---------------------------------------------------------------------------
// WidgetTree discovery
// ---------------------------------------------------------------------------

// findWidgetTreeExports returns 0-based export indices of WidgetTree exports
// whose outer is the WidgetBlueprint (bpIdx) or its GeneratedClass (genClassIdx).
func findWidgetTreeExports(asset *uasset.Asset, bpIdx, genClassIdx int) []int {
	var out []int
	for i, exp := range asset.Exports {
		if !strings.EqualFold(asset.ResolveClassName(exp), "WidgetTree") {
			continue
		}
		outerIdx := resolveOuterExportIndex(asset, exp)
		if outerIdx == bpIdx || (genClassIdx >= 0 && outerIdx == genClassIdx) {
			out = append(out, i)
		}
	}
	return out
}

// resolveOuterExportIndex returns the 0-based export index that the given
// export's OuterIndex points to, or -1 if it points to an import or null.
func resolveOuterExportIndex(asset *uasset.Asset, exp uasset.ExportEntry) int {
	idx := int32(exp.OuterIndex)
	if idx > 0 {
		return int(idx) - 1
	}
	return -1
}

// ---------------------------------------------------------------------------
// Build one WidgetTree entry
// ---------------------------------------------------------------------------

func buildWidgetTreeEntry(asset *uasset.Asset, bpIdx, genClassIdx, treeIdx int) (map[string]any, error) {
	exp := asset.Exports[treeIdx]
	treeProps := asset.ParseExportProperties(treeIdx)
	decoded := decodeAllProperties(asset, treeProps.Properties)

	// Determine the outer export name for labeling (e.g. "WBP_Test" vs "WBP_Test_C").
	outerName := ""
	outerClassName := ""
	outerIdx := resolveOuterExportIndex(asset, exp)
	if outerIdx >= 0 && outerIdx < len(asset.Exports) {
		outerName = asset.Exports[outerIdx].ObjectName.Display(asset.Names)
		outerClassName = asset.ResolveClassName(asset.Exports[outerIdx])
	}
	role := widgetTreeRole(bpIdx, genClassIdx, outerIdx, outerClassName)

	// Extract RootWidget and AllWidgets from decoded properties.
	rootWidgetExport := 0 // 1-based, 0 means null
	if v, ok := decoded["RootWidget"]; ok {
		rootWidgetExport = widgetExportIndexFromDecoded(v)
	}

	allWidgetExports := normalizeWidgetExportList(rootWidgetExport, widgetExportIndicesFromDecodedArray(decoded["AllWidgets"]))

	// Build the widget hierarchy for this tree.
	widgets, err := buildWidgetHierarchy(asset, rootWidgetExport, allWidgetExports)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"export":           treeIdx + 1,
		"ownerExport":      outerIdx + 1,
		"ownerObjectName":  outerName,
		"ownerClassName":   outerClassName,
		"role":             role,
		"rootWidgetExport": rootWidgetExport,
		"widgetCount":      len(widgets),
		"widgets":          widgets,
		"warnings":         treeProps.Warnings,
	}, nil
}

// ---------------------------------------------------------------------------
// Widget hierarchy construction
// ---------------------------------------------------------------------------

type widgetSlotData struct {
	exportIndex   int
	className     string
	props         []uasset.PropertyTag
	decoded       map[string]any
	warnings      []string
	parentExport  int
	contentExport int
}

type widgetHierarchyWidget struct {
	exportIndex int // 1-based
	objectName  string
	className   string
	props       []uasset.PropertyTag
	decoded     map[string]any
	warnings    []string
	slotExport  int // 1-based, 0 if none
}

type widgetHierarchyState struct {
	rootWidgetExport int
	orderedExports   []int
	widgetByExport   map[int]*widgetHierarchyWidget
	parentOf         map[int]int
	childrenOf       map[int][]int
	slotByChild      map[int]*widgetSlotData
	slotByExport     map[int]*widgetSlotData
	paths            map[int]string
}

func buildWidgetHierarchy(asset *uasset.Asset, rootWidgetExport int, allWidgetExports []int) ([]map[string]any, error) {
	state, err := buildWidgetHierarchyState(asset, rootWidgetExport, allWidgetExports)
	if err != nil {
		return nil, err
	}
	return state.widgetOutputs(asset), nil
}

func buildWidgetHierarchyState(asset *uasset.Asset, rootWidgetExport int, allWidgetExports []int) (*widgetHierarchyState, error) {
	if len(allWidgetExports) == 0 {
		return &widgetHierarchyState{
			rootWidgetExport: rootWidgetExport,
			orderedExports:   []int{},
			widgetByExport:   map[int]*widgetHierarchyWidget{},
			parentOf:         map[int]int{},
			childrenOf:       map[int][]int{},
			slotByChild:      map[int]*widgetSlotData{},
			slotByExport:     map[int]*widgetSlotData{},
			paths:            map[int]string{},
		}, nil
	}

	// allWidgetSet for fast membership check (1-based export indices).
	allWidgetSet := make(map[int]bool, len(allWidgetExports))
	for _, idx := range allWidgetExports {
		allWidgetSet[idx] = true
	}

	widgetByExport := make(map[int]*widgetHierarchyWidget, len(allWidgetExports))
	for _, exportIdx := range allWidgetExports {
		zeroIdx := exportIdx - 1
		if zeroIdx < 0 || zeroIdx >= len(asset.Exports) {
			continue
		}
		exp := asset.Exports[zeroIdx]
		parsed := asset.ParseExportProperties(zeroIdx)
		decoded := decodeAllProperties(asset, parsed.Properties)

		slotExport := 0
		if v, ok := decoded["Slot"]; ok {
			slotExport = widgetExportIndexFromDecoded(v)
		}

		widgetByExport[exportIdx] = &widgetHierarchyWidget{
			exportIndex: exportIdx,
			objectName:  exp.ObjectName.Display(asset.Names),
			className:   asset.ResolveClassName(exp),
			props:       parsed.Properties,
			decoded:     decoded,
			warnings:    parsed.Warnings,
			slotExport:  slotExport,
		}
	}

	// Build parent-child relationships by following Slot → Parent/Content.
	// parentOf[childExport] = parentExport (all 1-based).
	parentOf := make(map[int]int)
	// childrenOf[parentExport] = [childExport, ...]
	childrenOf := make(map[int][]int)
	// slotInfo[childExport] = parsed slot data
	slotByChild := make(map[int]*widgetSlotData)
	slotByExport := make(map[int]*widgetSlotData)

	for _, w := range widgetByExport {
		if w.slotExport <= 0 {
			continue
		}
		slotZeroIdx := w.slotExport - 1
		if slotZeroIdx < 0 || slotZeroIdx >= len(asset.Exports) {
			continue
		}
		slotExp := asset.Exports[slotZeroIdx]
		slotParsed := asset.ParseExportProperties(slotZeroIdx)
		slotDecoded := decodeAllProperties(asset, slotParsed.Properties)

		parentExport := 0
		if v, ok := slotDecoded["Parent"]; ok {
			parentExport = widgetExportIndexFromDecoded(v)
		}
		contentExport := 0
		if v, ok := slotDecoded["Content"]; ok {
			contentExport = widgetExportIndexFromDecoded(v)
		}

		slotInfo := &widgetSlotData{
			exportIndex:   w.slotExport,
			className:     asset.ResolveClassName(slotExp),
			props:         slotParsed.Properties,
			decoded:       slotDecoded,
			warnings:      slotParsed.Warnings,
			parentExport:  parentExport,
			contentExport: contentExport,
		}
		slotByExport[w.slotExport] = slotInfo

		if parentExport > 0 && contentExport > 0 && allWidgetSet[parentExport] && allWidgetSet[contentExport] {
			parentOf[contentExport] = parentExport
			slotByChild[contentExport] = slotInfo
		}
	}

	for _, parentExport := range allWidgetExports {
		w := widgetByExport[parentExport]
		if w == nil {
			continue
		}
		slotRefs := widgetExportIndicesFromDecodedArray(w.decoded["Slots"])
		orderedChildren := orderChildExportsForParent(slotRefs, slotByExport, childrenOf[parentExport])
		if len(orderedChildren) == 0 {
			continue
		}
		childrenOf[parentExport] = orderedChildren
		for _, childExport := range orderedChildren {
			parentOf[childExport] = parentExport
		}
	}

	// Build paths via DFS from root.
	paths := make(map[int]string, len(allWidgetExports))
	seen := make(map[int]bool, len(allWidgetExports))
	var buildPath func(exportIdx int, prefix string)
	buildPath = func(exportIdx int, prefix string) {
		w := widgetByExport[exportIdx]
		if w == nil || seen[exportIdx] {
			return
		}
		seen[exportIdx] = true
		path := w.objectName
		if prefix != "" {
			path = prefix + "/" + w.objectName
		}
		paths[exportIdx] = path
		for _, childIdx := range childrenOf[exportIdx] {
			buildPath(childIdx, path)
		}
	}

	// Start from root widget if known.
	if rootWidgetExport > 0 && widgetByExport[rootWidgetExport] != nil {
		buildPath(rootWidgetExport, "")
	}
	// Handle any orphans (widgets not reachable from root).
	for _, exportIdx := range allWidgetExports {
		if _, ok := paths[exportIdx]; !ok {
			buildPath(exportIdx, "")
		}
	}

	return &widgetHierarchyState{
		rootWidgetExport: rootWidgetExport,
		orderedExports:   append([]int(nil), allWidgetExports...),
		widgetByExport:   widgetByExport,
		parentOf:         parentOf,
		childrenOf:       childrenOf,
		slotByChild:      slotByChild,
		slotByExport:     slotByExport,
		paths:            paths,
	}, nil
}

func (s *widgetHierarchyState) widgetOutputs(asset *uasset.Asset) []map[string]any {
	if s == nil {
		return []map[string]any{}
	}
	widgets := make([]map[string]any, 0, len(s.orderedExports))
	for _, exportIdx := range s.orderedExports {
		w := s.widgetByExport[exportIdx]
		if w == nil {
			continue
		}

		isRoot := exportIdx == s.rootWidgetExport
		childNames := make([]string, 0)
		for _, childIdx := range s.childrenOf[exportIdx] {
			if cw := s.widgetByExport[childIdx]; cw != nil {
				childNames = append(childNames, cw.objectName)
			}
		}

		entry := map[string]any{
			"export":       exportIdx,
			"objectName":   w.objectName,
			"className":    w.className,
			"path":         s.paths[exportIdx],
			"isRoot":       isRoot,
			"parentExport": s.parentOf[exportIdx],
			"childExports": append([]int(nil), s.childrenOf[exportIdx]...),
			"children":     childNames,
			"slotExport":   w.slotExport,
			"properties":   toPropertyOutputs(asset, w.props, true),
			"warnings":     w.warnings,
		}
		mergeWidgetReadSummary(entry, widgetReadWidgetSummary(asset, w.className, w.decoded))
		if entry["parentExport"] == 0 {
			delete(entry, "parentExport")
		}
		if w.slotExport <= 0 {
			delete(entry, "slotExport")
		}
		if len(s.childrenOf[exportIdx]) == 0 {
			delete(entry, "childExports")
		}

		if parentIdx, ok := s.parentOf[exportIdx]; ok {
			if pw := s.widgetByExport[parentIdx]; pw != nil {
				entry["parent"] = pw.objectName
			}
		}

		if sd, ok := s.slotByChild[exportIdx]; ok {
			slotEntry := map[string]any{
				"export":     sd.exportIndex,
				"className":  sd.className,
				"properties": toPropertyOutputs(asset, sd.props, true),
				"warnings":   sd.warnings,
			}
			mergeWidgetReadSummary(slotEntry, widgetReadSlotSummary(sd.decoded))
			entry["slot"] = slotEntry
			mergeWidgetReadSummary(entry, widgetReadSlotSummary(sd.decoded))
		}

		widgets = append(widgets, entry)
	}
	return widgets
}

// ---------------------------------------------------------------------------
// Decoded ObjectProperty helpers
// ---------------------------------------------------------------------------

// widgetExportIndexFromDecoded extracts a 1-based export index from a decoded
// ObjectProperty value ({"index": N, "resolved": "..."}). Returns 0 if the
// value does not point to an export.
func widgetExportIndexFromDecoded(decoded any) int {
	m, ok := decoded.(map[string]any)
	if !ok {
		return 0
	}
	idx, ok := extractIntLike(m["index"])
	if !ok || idx <= 0 {
		return 0
	}
	return idx
}

// widgetExportIndicesFromDecodedArray extracts 1-based export indices from a
// decoded ArrayProperty(ObjectProperty) value.
func widgetExportIndicesFromDecodedArray(decoded any) []int {
	m, ok := decoded.(map[string]any)
	if !ok {
		return nil
	}
	arr, ok := m["value"].([]any)
	if !ok {
		return nil
	}
	out := make([]int, 0, len(arr))
	for _, elem := range arr {
		em, ok := elem.(map[string]any)
		if !ok {
			continue
		}
		idx := widgetExportIndexFromDecoded(em["value"])
		if idx > 0 {
			out = append(out, idx)
		}
	}
	return out
}

func mergeWidgetReadSummary(dst map[string]any, summary map[string]any) {
	if dst == nil || len(summary) == 0 {
		return
	}
	for key, value := range summary {
		dst[key] = value
	}
}

func widgetReadSummaryRoleRank(role string) int {
	switch {
	case strings.EqualFold(role, "designer"):
		return 3
	case strings.EqualFold(role, "generated"):
		return 2
	case strings.TrimSpace(role) != "":
		return 1
	default:
		return 0
	}
}

func widgetReadWidgetSummary(asset *uasset.Asset, className string, decoded map[string]any) map[string]any {
	if len(decoded) == 0 {
		return nil
	}
	out := map[string]any{}
	if text := widgetReadTextSummary(decoded["Text"]); len(text) > 0 {
		out["text"] = text
	}
	if brush := widgetReadBrushSummary(asset, decoded["Brush"]); len(brush) > 0 {
		out["brush"] = brush
	}
	if textStyle := widgetReadTextStyleSummary(asset, className, decoded); len(textStyle) > 0 {
		out["textStyle"] = textStyle
	}
	if richTextStyle := widgetReadRichTextStyleSummary(asset, className, decoded); len(richTextStyle) > 0 {
		out["richTextStyle"] = richTextStyle
	}
	if buttonStyle := widgetReadButtonStyleSummary(asset, className, decoded); len(buttonStyle) > 0 {
		out["buttonStyle"] = buttonStyle
	}
	if value, ok := widgetReadFocusableSummary(className, decoded); ok {
		out["isFocusable"] = value
	}
	if borderStyle := widgetReadBorderStyleSummary(className, decoded); len(borderStyle) > 0 {
		out["borderStyle"] = borderStyle
	}
	if progressBar := widgetReadProgressBarSummary(className, decoded); len(progressBar) > 0 {
		out["progressBar"] = progressBar
	}
	if slider := widgetReadSliderSummary(className, decoded); len(slider) > 0 {
		out["slider"] = slider
	}
	if spacer := widgetReadSpacerSummary(className, decoded); len(spacer) > 0 {
		out["spacer"] = spacer
	}
	if scrollBar := widgetReadScrollBarSummary(className, decoded); len(scrollBar) > 0 {
		out["scrollBar"] = scrollBar
	}
	if checkBox := widgetReadCheckBoxSummary(className, decoded); len(checkBox) > 0 {
		out["checkBox"] = checkBox
	}
	if editableText := widgetReadEditableTextSummary(className, decoded); len(editableText) > 0 {
		out["editableText"] = editableText
	}
	if editableTextBox := widgetReadEditableTextBoxSummary(className, decoded); len(editableTextBox) > 0 {
		out["editableTextBox"] = editableTextBox
	}
	if multiLineEditableTextBox := widgetReadMultiLineEditableTextBoxSummary(className, decoded); len(multiLineEditableTextBox) > 0 {
		out["multiLineEditableTextBox"] = multiLineEditableTextBox
	}
	if spinBox := widgetReadSpinBoxSummary(className, decoded); len(spinBox) > 0 {
		out["spinBox"] = spinBox
	}
	if comboBoxString := widgetReadComboBoxStringSummary(className, decoded); len(comboBoxString) > 0 {
		out["comboBoxString"] = comboBoxString
	}
	if listView := widgetReadListViewSummary(asset, className, decoded); len(listView) > 0 {
		out["listView"] = listView
	}
	if tileView := widgetReadTileViewSummary(className, decoded); len(tileView) > 0 {
		out["tileView"] = tileView
	}
	if scrollBox := widgetReadScrollBoxSummary(className, decoded); len(scrollBox) > 0 {
		out["scrollBox"] = scrollBox
	}
	if sizeBox := widgetReadSizeBoxSummary(className, decoded); len(sizeBox) > 0 {
		out["sizeBox"] = sizeBox
	}
	if scaleBox := widgetReadScaleBoxSummary(className, decoded); len(scaleBox) > 0 {
		out["scaleBox"] = scaleBox
	}
	if wrapBox := widgetReadWrapBoxSummary(className, decoded); len(wrapBox) > 0 {
		out["wrapBox"] = wrapBox
	}
	if widgetSwitcher := widgetReadWidgetSwitcherSummary(className, decoded); len(widgetSwitcher) > 0 {
		out["widgetSwitcher"] = widgetSwitcher
	}
	if retainerBox := widgetReadRetainerBoxSummary(className, decoded); len(retainerBox) > 0 {
		out["retainerBox"] = retainerBox
	}
	if backgroundBlur := widgetReadBackgroundBlurSummary(className, decoded); len(backgroundBlur) > 0 {
		out["backgroundBlur"] = backgroundBlur
	}
	if safeZone := widgetReadSafeZoneSummary(className, decoded); len(safeZone) > 0 {
		out["safeZone"] = safeZone
	}
	if invalidationBox := widgetReadInvalidationBoxSummary(className, decoded); len(invalidationBox) > 0 {
		out["invalidationBox"] = invalidationBox
	}
	if uniformGridPanel := widgetReadUniformGridPanelSummary(className, decoded); len(uniformGridPanel) > 0 {
		out["uniformGridPanel"] = uniformGridPanel
	}
	if value, ok := widgetReadEnumValue(decoded["Visibility"]); ok {
		out["visibility"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["RenderOpacity"]); ok {
		out["renderOpacity"] = value
	}
	if value, ok := widgetReadEnumValue(decoded["Placement"]); ok {
		out["menuAnchorPlacement"] = value
	}
	if value, ok := widgetReadFloatArraySummary(decoded["RowFill"]); ok {
		out["gridRowFill"] = value
	}
	if value, ok := widgetReadFloatArraySummary(decoded["ColumnFill"]); ok {
		out["gridColumnFill"] = value
	}
	return out
}

func widgetReadFocusableSummary(className string, decoded map[string]any) (bool, bool) {
	switch {
	case strings.EqualFold(className, "Button"),
		strings.EqualFold(className, "CheckBox"),
		strings.EqualFold(className, "Slider"):
		return widgetReadBoolValue(decoded["IsFocusable"])
	case strings.EqualFold(className, "ComboBoxString"),
		strings.EqualFold(className, "ScrollBox"),
		strings.EqualFold(className, "ListView"),
		strings.EqualFold(className, "TileView"),
		strings.EqualFold(className, "TreeView"):
		return widgetReadBoolValue(decoded["bIsFocusable"])
	default:
		return false, false
	}
}

func widgetReadSlotSummary(decoded map[string]any) map[string]any {
	if len(decoded) == 0 {
		return nil
	}
	out := map[string]any{}
	if layout, ok := widgetReadLayoutSummary(decoded["LayoutData"]); ok {
		out["slotLayout"] = layout
	}
	if padding, ok := widgetReadMarginSummary(decoded["Padding"]); ok {
		out["slotPadding"] = padding
	}
	if size, ok := widgetReadSlateChildSizeSummary(decoded["Size"]); ok {
		out["slotSize"] = size
	}
	if value, ok := widgetReadEnumValue(decoded["HorizontalAlignment"]); ok {
		out["slotHorizontalAlignment"] = value
	}
	if value, ok := widgetReadEnumValue(decoded["VerticalAlignment"]); ok {
		out["slotVerticalAlignment"] = value
	}
	if value, ok := widgetReadIntValue(decoded["Row"]); ok {
		out["slotRow"] = value
	}
	if value, ok := widgetReadIntValue(decoded["Column"]); ok {
		out["slotColumn"] = value
	}
	if value, ok := widgetReadIntValue(decoded["RowSpan"]); ok {
		out["slotRowSpan"] = value
	}
	if value, ok := widgetReadIntValue(decoded["ColumnSpan"]); ok {
		out["slotColumnSpan"] = value
	}
	if value, ok := widgetReadIntValue(decoded["Layer"]); ok {
		out["slotLayer"] = value
	}
	if value, ok := widgetReadVector2Summary(decoded["Nudge"]); ok {
		out["slotNudge"] = value
	}
	return out
}

func widgetReadTextSummary(raw any) map[string]any {
	value := widgetDecodedStructValue(raw)
	if value == nil {
		return nil
	}
	out := map[string]any{}
	for _, key := range []string{"sourceString", "displayString", "namespace", "key", "historyType", "value"} {
		if v, ok := widgetDecodedStructField(value, key); ok {
			out[key] = v
		}
	}
	if flags, ok := widgetDecodedStructField(value, "flags"); ok {
		out["flags"] = flags
	}
	return out
}

func widgetReadTextStyleSummary(asset *uasset.Asset, className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "TextBlock") {
		return nil
	}
	return widgetReadTextStyleFieldsSummary(asset, decoded)
}

func widgetReadRichTextStyleSummary(asset *uasset.Asset, className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "RichTextBlock") {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadObjectRefSummary(asset, decoded["TextStyleSet"]); ok {
		out["textStyleSet"] = value
		if path, ok := value["path"].(string); ok && path != "" {
			out["textStyleSetPath"] = path
		}
	}
	if values, ok := widgetReadObjectRefArraySummary(asset, decoded["DecoratorClasses"]); ok {
		out["decoratorClasses"] = values
		paths := make([]string, 0, len(values))
		for _, value := range values {
			path, _ := value["path"].(string)
			if path != "" {
				paths = append(paths, path)
			}
		}
		if len(paths) > 0 {
			out["decoratorClassPaths"] = paths
		}
	}
	if value, ok := widgetReadBoolValue(decoded["bOverrideDefaultStyle"]); ok {
		out["overrideDefaultStyle"] = value
	}
	if value, ok := widgetReadEnumValue(decoded["Justification"]); ok {
		out["justification"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["AutoWrapText"]); ok {
		out["autoWrapText"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["WrapTextAt"]); ok {
		out["wrapTextAt"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["LineHeightPercentage"]); ok {
		out["lineHeightPercentage"] = value
	}
	if style := widgetReadTextStyleFieldsSummary(asset, widgetDecodedStructValue(decoded["DefaultTextStyleOverride"])); len(style) > 0 {
		out["defaultTextStyleOverride"] = style
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadTextStyleFieldsSummary(asset *uasset.Asset, decoded map[string]any) map[string]any {
	if len(decoded) == 0 {
		return nil
	}
	out := map[string]any{}
	if font, ok := widgetReadFontSummary(asset, decoded["Font"]); ok {
		out["font"] = font
	}
	if value, ok := widgetReadSlateColorSummary(decoded["ColorAndOpacity"]); ok {
		out["colorAndOpacity"] = value
	}
	if value, ok := widgetReadEnumValue(decoded["Justification"]); ok {
		out["justification"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["AutoWrapText"]); ok {
		out["autoWrapText"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["WrapTextAt"]); ok {
		out["wrapTextAt"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["LineHeightPercentage"]); ok {
		out["lineHeightPercentage"] = value
	}
	if value, ok := widgetReadVector2Summary(decoded["ShadowOffset"]); ok {
		out["shadowOffset"] = value
	}
	if value, ok := widgetReadLinearColorSummary(decoded["ShadowColorAndOpacity"]); ok {
		out["shadowColorAndOpacity"] = value
	}
	if value := widgetReadBrushSummary(asset, decoded["StrikeBrush"]); len(value) > 0 {
		out["strikeBrush"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadButtonStyleSummary(asset *uasset.Asset, className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "Button") {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadLinearColorSummary(decoded["BackgroundColor"]); ok {
		out["backgroundColor"] = value
	}
	if value, ok := widgetReadLinearColorSummary(decoded["ColorAndOpacity"]); ok {
		out["colorAndOpacity"] = value
	}
	if value := widgetReadButtonBrushesSummary(asset, decoded["WidgetStyle"]); len(value) > 0 {
		out["brushes"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadBorderStyleSummary(className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "Border") {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadMarginSummary(decoded["Padding"]); ok {
		out["padding"] = value
	}
	if value, ok := widgetReadLinearColorSummary(decoded["BrushColor"]); ok {
		out["brushColor"] = value
	}
	if value, ok := widgetReadLinearColorSummary(decoded["ContentColorAndOpacity"]); ok {
		out["contentColorAndOpacity"] = value
	}
	if value, ok := widgetReadEnumValue(decoded["HorizontalAlignment"]); ok {
		out["horizontalAlignment"] = value
	}
	if value, ok := widgetReadEnumValue(decoded["VerticalAlignment"]); ok {
		out["verticalAlignment"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadProgressBarSummary(className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "ProgressBar") {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadFloatValue(decoded["Percent"]); ok {
		out["percent"] = value
	}
	if value, ok := widgetReadLinearColorSummary(decoded["FillColorAndOpacity"]); ok {
		out["fillColorAndOpacity"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadSliderSummary(className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "Slider") {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadFloatValue(decoded["Value"]); ok {
		out["value"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["MinValue"]); ok {
		out["minValue"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["MaxValue"]); ok {
		out["maxValue"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["StepSize"]); ok {
		out["stepSize"] = value
	}
	if value, ok := widgetReadEnumValue(decoded["Orientation"]); ok {
		out["orientation"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadSpacerSummary(className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "Spacer") {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadVector2Summary(decoded["Size"]); ok {
		out["size"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadScrollBarSummary(className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "ScrollBar") {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadVector2Summary(decoded["Thickness"]); ok {
		out["thickness"] = value
	}
	if value, ok := widgetReadEnumValue(decoded["Orientation"]); ok {
		out["orientation"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadCheckBoxSummary(className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "CheckBox") {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadEnumValue(decoded["CheckedState"]); ok {
		out["checkedState"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadEditableTextSummary(className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "EditableText") {
		return nil
	}
	out := map[string]any{}
	if value := widgetReadTextSummary(decoded["HintText"]); len(value) > 0 {
		out["hintText"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["IsReadOnly"]); ok {
		out["isReadOnly"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["IsPassword"]); ok {
		out["isPassword"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["MinimumDesiredWidth"]); ok {
		out["minimumDesiredWidth"] = value
	}
	if value, ok := widgetReadEnumValue(decoded["Justification"]); ok {
		out["justification"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadEditableTextBoxSummary(className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "EditableTextBox") {
		return nil
	}
	out := map[string]any{}
	if value := widgetReadTextSummary(decoded["HintText"]); len(value) > 0 {
		out["hintText"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["IsReadOnly"]); ok {
		out["isReadOnly"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["IsPassword"]); ok {
		out["isPassword"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["MinimumDesiredWidth"]); ok {
		out["minimumDesiredWidth"] = value
	}
	if value, ok := widgetReadEnumValue(decoded["Justification"]); ok {
		out["justification"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadMultiLineEditableTextBoxSummary(className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "MultiLineEditableTextBox") {
		return nil
	}
	out := map[string]any{}
	if value := widgetReadTextSummary(decoded["HintText"]); len(value) > 0 {
		out["hintText"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["bIsReadOnly"]); ok {
		out["isReadOnly"] = value
	}
	if value, ok := widgetReadEnumValue(decoded["Justification"]); ok {
		out["justification"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadSpinBoxSummary(className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "SpinBox") {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadFloatValue(decoded["Value"]); ok {
		out["value"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["MinValue"]); ok {
		out["minValue"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["MaxValue"]); ok {
		out["maxValue"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["Delta"]); ok {
		out["delta"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadComboBoxStringSummary(className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "ComboBoxString") {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadStringValue(decoded["SelectedOption"]); ok {
		out["selectedOption"] = value
	}
	if value, ok := widgetReadStringArraySummary(decoded["DefaultOptions"]); ok {
		out["defaultOptions"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadListViewSummary(asset *uasset.Asset, className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "ListView") && !strings.EqualFold(className, "TileView") && !strings.EqualFold(className, "TreeView") {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadEnumValue(decoded["Orientation"]); ok {
		out["orientation"] = value
	}
	if value, ok := widgetReadEnumValue(decoded["SelectionMode"]); ok {
		out["selectionMode"] = value
	}
	if value, ok := widgetReadEnumValue(decoded["ConsumeMouseWheel"]); ok {
		out["consumeMouseWheel"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["bIsFocusable"]); ok {
		out["isFocusable"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["bReturnFocusToSelection"]); ok {
		out["returnFocusToSelection"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["bClearScrollVelocityOnSelection"]); ok {
		out["clearScrollVelocityOnSelection"] = value
	}
	if value, ok := widgetReadEnumValue(decoded["ScrollIntoViewAlignment"]); ok {
		out["scrollIntoViewAlignment"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["WheelScrollMultiplier"]); ok {
		out["wheelScrollMultiplier"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["bEnableScrollAnimation"]); ok {
		out["enableScrollAnimation"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["AllowOverscroll"]); ok {
		out["allowOverscroll"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["bEnableRightClickScrolling"]); ok {
		out["enableRightClickScrolling"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["bEnableTouchScrolling"]); ok {
		out["enableTouchScrolling"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["bIsPointerScrollingEnabled"]); ok {
		out["isPointerScrollingEnabled"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["bIsGamepadScrollingEnabled"]); ok {
		out["isGamepadScrollingEnabled"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["HorizontalEntrySpacing"]); ok {
		out["horizontalEntrySpacing"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["VerticalEntrySpacing"]); ok {
		out["verticalEntrySpacing"] = value
	}
	if value, ok := widgetReadMarginSummary(decoded["ScrollBarPadding"]); ok {
		out["scrollBarPadding"] = value
	}
	if value, ok := widgetReadObjectRefSummary(asset, decoded["EntryWidgetClass"]); ok {
		out["entryWidgetClass"] = value
		if path, ok := value["path"].(string); ok && path != "" {
			out["entryWidgetClassPath"] = path
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadTileViewSummary(className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "TileView") {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadFloatValue(decoded["EntryWidth"]); ok {
		out["entryWidth"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["EntryHeight"]); ok {
		out["entryHeight"] = value
	}
	if value, ok := widgetReadEnumValue(decoded["ScrollbarDisabledVisibility"]); ok {
		out["scrollbarDisabledVisibility"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["bEntrySizeIncludesEntrySpacing"]); ok {
		out["entrySizeIncludesEntrySpacing"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadScrollBoxSummary(className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "ScrollBox") {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadEnumValue(decoded["Orientation"]); ok {
		out["orientation"] = value
	}
	if value, ok := widgetReadEnumValue(decoded["ScrollBarVisibility"]); ok {
		out["scrollBarVisibility"] = value
	}
	if value, ok := widgetReadEnumValue(decoded["ConsumeMouseWheel"]); ok {
		out["consumeMouseWheel"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadSizeBoxSummary(className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "SizeBox") {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadFloatValue(decoded["WidthOverride"]); ok {
		out["widthOverride"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["HeightOverride"]); ok {
		out["heightOverride"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["MinDesiredWidth"]); ok {
		out["minDesiredWidth"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["MinDesiredHeight"]); ok {
		out["minDesiredHeight"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["MaxDesiredWidth"]); ok {
		out["maxDesiredWidth"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["MaxDesiredHeight"]); ok {
		out["maxDesiredHeight"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["MinAspectRatio"]); ok {
		out["minAspectRatio"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["MaxAspectRatio"]); ok {
		out["maxAspectRatio"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadScaleBoxSummary(className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "ScaleBox") {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadEnumValue(decoded["Stretch"]); ok {
		out["stretch"] = value
	}
	if value, ok := widgetReadEnumValue(decoded["StretchDirection"]); ok {
		out["stretchDirection"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["UserSpecifiedScale"]); ok {
		out["userSpecifiedScale"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["IgnoreInheritedScale"]); ok {
		out["ignoreInheritedScale"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadWrapBoxSummary(className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "WrapBox") {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadFloatValue(decoded["WrapSize"]); ok {
		out["wrapSize"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["bExplicitWrapSize"]); ok {
		out["explicitWrapSize"] = value
	}
	if value, ok := widgetReadVector2Summary(decoded["InnerSlotPadding"]); ok {
		out["innerSlotPadding"] = value
	}
	if value, ok := widgetReadEnumValue(decoded["Orientation"]); ok {
		out["orientation"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadWidgetSwitcherSummary(className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "WidgetSwitcher") {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadIntValue(decoded["ActiveWidgetIndex"]); ok {
		out["activeWidgetIndex"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadRetainerBoxSummary(className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "RetainerBox") {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadBoolValue(decoded["bRetainRender"]); ok {
		out["retainRender"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["RenderOnInvalidation"]); ok {
		out["renderOnInvalidation"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["RenderOnPhase"]); ok {
		out["renderOnPhase"] = value
	}
	if value, ok := widgetReadIntValue(decoded["Phase"]); ok {
		out["phase"] = value
	}
	if value, ok := widgetReadIntValue(decoded["PhaseCount"]); ok {
		out["phaseCount"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadBackgroundBlurSummary(className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "BackgroundBlur") {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadFloatValue(decoded["BlurStrength"]); ok {
		out["blurStrength"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["bApplyAlphaToBlur"]); ok {
		out["applyAlphaToBlur"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadSafeZoneSummary(className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "SafeZone") {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadBoolValue(decoded["PadLeft"]); ok {
		out["padLeft"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["PadRight"]); ok {
		out["padRight"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["PadTop"]); ok {
		out["padTop"] = value
	}
	if value, ok := widgetReadBoolValue(decoded["PadBottom"]); ok {
		out["padBottom"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadInvalidationBoxSummary(className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "InvalidationBox") {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadBoolValue(decoded["bCanCache"]); ok {
		out["canCache"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadUniformGridPanelSummary(className string, decoded map[string]any) map[string]any {
	if !strings.EqualFold(className, "UniformGridPanel") {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadFloatValue(decoded["MinDesiredSlotWidth"]); ok {
		out["minDesiredSlotWidth"] = value
	}
	if value, ok := widgetReadFloatValue(decoded["MinDesiredSlotHeight"]); ok {
		out["minDesiredSlotHeight"] = value
	}
	if value, ok := widgetReadMarginSummary(decoded["SlotPadding"]); ok {
		out["slotPadding"] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadBrushSummary(asset *uasset.Asset, raw any) map[string]any {
	fields := widgetDecodedStructValue(raw)
	if fields == nil {
		return nil
	}
	out := map[string]any{}
	if value, ok := widgetReadEnumValue(fields["ImageType"]); ok {
		out["imageType"] = value
	}
	if value, ok := widgetReadEnumValue(fields["DrawAs"]); ok {
		out["drawAs"] = value
	}
	if imageSize, ok := widgetReadVector2Summary(fields["ImageSize"]); ok {
		out["imageSize"] = imageSize
	}
	if margin, ok := widgetReadMarginSummary(fields["Margin"]); ok {
		out["margin"] = margin
	}
	if tintColor, ok := widgetReadSlateColorSummary(fields["TintColor"]); ok {
		out["tintColor"] = tintColor
	}
	if resourceObject, ok := widgetReadObjectRefSummary(asset, fields["ResourceObject"]); ok {
		out["resourceObject"] = resourceObject
		if path, ok := resourceObject["path"].(string); ok && path != "" {
			out["resourceObjectPath"] = path
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func widgetReadSlateColorSummary(raw any) (map[string]any, bool) {
	fields := widgetDecodedStructValue(raw)
	if fields == nil {
		return nil, false
	}
	color, ok := widgetReadLinearColorSummary(fields["SpecifiedColor"])
	useRule, hasUseRule := widgetReadEnumValue(fields["ColorUseRule"])
	if !ok && !hasUseRule {
		return nil, false
	}
	if !ok {
		color = map[string]any{}
	}
	if hasUseRule {
		color["colorUseRule"] = useRule
	}
	return color, true
}

func widgetReadLinearColorSummary(raw any) (map[string]any, bool) {
	fields := widgetDecodedStructValue(raw)
	if fields == nil {
		return nil, false
	}
	if value, ok := fields["value"].(map[string]any); ok {
		fields = value
	}
	r, ok := widgetReadFloatValue(fields["r"])
	if !ok {
		return nil, false
	}
	g, _ := widgetReadFloatValue(fields["g"])
	b, _ := widgetReadFloatValue(fields["b"])
	a, _ := widgetReadFloatValue(fields["a"])
	return map[string]any{
		"r": r,
		"g": g,
		"b": b,
		"a": a,
	}, true
}

func widgetReadButtonBrushesSummary(asset *uasset.Asset, raw any) map[string]any {
	fields := widgetDecodedStructValue(raw)
	if fields == nil {
		return nil
	}
	out := map[string]any{}
	for _, state := range []struct {
		field string
		key   string
	}{
		{field: "Normal", key: "normal"},
		{field: "Hovered", key: "hovered"},
		{field: "Pressed", key: "pressed"},
		{field: "Disabled", key: "disabled"},
	} {
		if brush := widgetReadBrushSummary(asset, fields[state.field]); len(brush) > 0 {
			out[state.key] = brush
		}
	}
	return out
}

func widgetReadFontSummary(asset *uasset.Asset, raw any) (map[string]any, bool) {
	fields := widgetDecodedStructValue(raw)
	if fields == nil {
		return nil, false
	}

	out := map[string]any{}
	if value, ok := widgetReadIntValue(fields["Size"]); ok {
		out["size"] = value
	}
	if value, ok := widgetReadIntValue(fields["LetterSpacing"]); ok {
		out["letterSpacing"] = value
	}
	if value, ok := widgetReadEnumValue(fields["FontFallback"]); ok {
		out["fontFallback"] = value
	}
	if value, ok := widgetReadObjectRefSummary(asset, fields["FontObject"]); ok {
		out["fontObject"] = value
		if path, ok := value["path"].(string); ok && path != "" {
			out["fontObjectPath"] = path
		}
	}
	if value, ok := widgetReadNameValue(fields["TypefaceFontName"]); ok {
		out["typefaceFontName"] = value
	}
	if value, ok := widgetReadFontOutlineSummary(asset, fields["OutlineSettings"]); ok {
		out["outlineSettings"] = value
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func widgetReadLayoutSummary(raw any) (map[string]any, bool) {
	fields := widgetDecodedStructValue(raw)
	if fields == nil {
		return nil, false
	}
	anchors := widgetDecodedStructValue(fields["Anchors"])
	offsets := widgetDecodedStructValue(fields["Offsets"])
	minimum := widgetDecodedStructValue(anchors["Minimum"])
	maximum := widgetDecodedStructValue(anchors["Maximum"])
	alignment, _ := widgetReadVector2Summary(fields["Alignment"])

	out := map[string]any{
		"position": []float32{
			widgetDecodedScalarFloat(offsets["Left"]),
			widgetDecodedScalarFloat(offsets["Top"]),
		},
		"size": []float32{
			widgetDecodedScalarFloat(offsets["Right"]),
			widgetDecodedScalarFloat(offsets["Bottom"]),
		},
		"anchors": []float32{
			widgetDecodedScalarFloat(widgetReadVectorComponent(minimum, "X", "x")),
			widgetDecodedScalarFloat(widgetReadVectorComponent(minimum, "Y", "y")),
			widgetDecodedScalarFloat(widgetReadVectorComponent(maximum, "X", "x")),
			widgetDecodedScalarFloat(widgetReadVectorComponent(maximum, "Y", "y")),
		},
	}
	if alignment != nil {
		out["alignment"] = []float32{
			widgetDecodedScalarFloat(alignment["x"]),
			widgetDecodedScalarFloat(alignment["y"]),
		}
	} else {
		out["alignment"] = []float32{0, 0}
	}
	return out, true
}

func widgetReadFontOutlineSummary(asset *uasset.Asset, raw any) (map[string]any, bool) {
	fields := widgetDecodedStructValue(raw)
	if fields == nil {
		return nil, false
	}

	out := map[string]any{}
	if value, ok := widgetReadIntValue(fields["OutlineSize"]); ok {
		out["outlineSize"] = value
	}
	if value, ok := widgetReadLinearColorSummary(fields["OutlineColor"]); ok {
		out["outlineColor"] = value
	}
	if value, ok := widgetReadObjectRefSummary(asset, fields["OutlineMaterial"]); ok {
		out["outlineMaterial"] = value
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func widgetReadMarginSummary(raw any) (map[string]any, bool) {
	fields := widgetDecodedStructValue(raw)
	if fields == nil {
		return nil, false
	}
	if rawBase64, ok := widgetDecodedStructField(fields, "rawBase64"); ok {
		if summary, ok := widgetReadMarginRawSummary(fmt.Sprint(rawBase64)); ok {
			return summary, true
		}
	}
	return map[string]any{
		"left":   widgetDecodedScalarFloat(fields["Left"]),
		"top":    widgetDecodedScalarFloat(fields["Top"]),
		"right":  widgetDecodedScalarFloat(fields["Right"]),
		"bottom": widgetDecodedScalarFloat(fields["Bottom"]),
	}, true
}

func widgetReadVector2Summary(raw any) (map[string]any, bool) {
	fields := widgetDecodedStructValue(raw)
	if fields == nil {
		return nil, false
	}
	if x, ok := widgetDecodedStructField(fields, "x"); ok {
		y, _ := widgetDecodedStructField(fields, "y")
		return map[string]any{
			"x": widgetDecodedScalarFloat(x),
			"y": widgetDecodedScalarFloat(y),
		}, true
	}
	if x, ok := widgetDecodedStructField(fields, "X"); ok {
		y, _ := widgetDecodedStructField(fields, "Y")
		return map[string]any{
			"x": widgetDecodedScalarFloat(x),
			"y": widgetDecodedScalarFloat(y),
		}, true
	}
	if rawBase64, ok := widgetDecodedStructField(fields, "rawBase64"); ok {
		if summary, ok := widgetReadVector2RawSummary(fmt.Sprint(rawBase64)); ok {
			return summary, true
		}
		return map[string]any{"rawBase64": rawBase64}, true
	}
	return nil, false
}

func widgetReadFloatArraySummary(raw any) ([]float32, bool) {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	items, err := toAnySliceLocal(m["value"])
	if err != nil || len(items) == 0 {
		return nil, false
	}
	out := make([]float32, 0, len(items))
	for _, item := range items {
		switch current := item.(type) {
		case map[string]any:
			if value, ok := widgetReadFloatValue(current["value"]); ok {
				out = append(out, value)
			} else if value, ok := widgetReadFloatValue(current); ok {
				out = append(out, value)
			}
		default:
			if value, ok := widgetReadFloatValue(current); ok {
				out = append(out, value)
			}
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func widgetReadStringValue(raw any) (string, bool) {
	switch value := raw.(type) {
	case string:
		return value, true
	case map[string]any:
		if value == nil {
			return "", false
		}
		v, ok := value["value"].(string)
		return v, ok
	default:
		return "", false
	}
}

func widgetReadStringArraySummary(raw any) ([]string, bool) {
	decoded, ok := raw.(map[string]any)
	if !ok || decoded == nil {
		return nil, false
	}
	items, err := anySliceLocal(decoded["value"])
	if err != nil {
		return nil, false
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		switch value := item.(type) {
		case string:
			out = append(out, value)
		case map[string]any:
			if str, ok := value["value"].(string); ok {
				out = append(out, str)
			}
		}
	}
	return out, true
}

func toAnySliceLocal(raw any) ([]any, error) {
	items, ok := raw.([]any)
	if ok {
		return items, nil
	}
	return nil, fmt.Errorf("value is not an array")
}

func widgetReadSlateChildSizeSummary(raw any) (map[string]any, bool) {
	fields, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	if inner, ok := fields["value"].(map[string]any); ok {
		fields = inner
	}
	sizeRule, ok := widgetReadEnumValue(fields["SizeRule"])
	if !ok {
		return nil, false
	}
	value, _ := widgetReadFloatValue(fields["Value"])
	return map[string]any{
		"rule":  sizeRule,
		"value": value,
	}, true
}

func widgetReadMarginRawSummary(raw string) (map[string]any, bool) {
	body, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil || len(body) < 16 {
		return nil, false
	}
	return map[string]any{
		"left":   math.Float32frombits(binary.LittleEndian.Uint32(body[0:4])),
		"top":    math.Float32frombits(binary.LittleEndian.Uint32(body[4:8])),
		"right":  math.Float32frombits(binary.LittleEndian.Uint32(body[8:12])),
		"bottom": math.Float32frombits(binary.LittleEndian.Uint32(body[12:16])),
	}, true
}

func widgetReadVector2RawSummary(raw string) (map[string]any, bool) {
	body, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil || len(body) < 8 {
		return nil, false
	}
	return map[string]any{
		"x": math.Float32frombits(binary.LittleEndian.Uint32(body[0:4])),
		"y": math.Float32frombits(binary.LittleEndian.Uint32(body[4:8])),
	}, true
}

func widgetReadVectorComponent(fields map[string]any, upperKey, lowerKey string) any {
	if len(fields) == 0 {
		return nil
	}
	if value, ok := widgetDecodedStructField(fields, upperKey); ok {
		return value
	}
	if value, ok := widgetDecodedStructField(fields, lowerKey); ok {
		return value
	}
	return nil
}

func widgetReadObjectRefSummary(asset *uasset.Asset, raw any) (map[string]any, bool) {
	value := widgetDecodedStructValue(raw)
	if value == nil {
		return nil, false
	}
	out := cloneAnyMapLocal(value)
	if asset == nil {
		return out, true
	}
	index, ok := extractIntLike(out["index"])
	if !ok {
		return out, true
	}
	if index < 0 {
		importIdx := -index - 1
		if importIdx >= 0 && importIdx < len(asset.Imports) {
			out["path"] = resolveImportTargetPath(asset, asset.Imports[importIdx])
		}
	}
	return out, true
}

func widgetReadObjectRefArraySummary(asset *uasset.Asset, raw any) ([]map[string]any, bool) {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	items, err := toAnySliceLocal(m["value"])
	if err != nil || len(items) == 0 {
		return nil, false
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		summary, ok := widgetReadObjectRefSummary(asset, item)
		if !ok {
			continue
		}
		out = append(out, summary)
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func widgetReadNameValue(raw any) (string, bool) {
	value := widgetDecodedStructValue(raw)
	if value == nil {
		return "", false
	}
	name, ok := widgetDecodedStructField(value, "name")
	if !ok {
		return "", false
	}
	text := strings.TrimSpace(fmt.Sprint(name))
	return text, text != ""
}

func widgetReadEnumValue(raw any) (string, bool) {
	value := widgetDecodedStructValue(raw)
	if value == nil {
		return "", false
	}
	enumValue, ok := widgetDecodedStructField(value, "value")
	if !ok {
		return "", false
	}
	text := strings.TrimSpace(fmt.Sprint(enumValue))
	return text, text != ""
}

func widgetReadFloatValue(raw any) (float32, bool) {
	switch value := raw.(type) {
	case float64:
		return float32(value), true
	case float32:
		return value, true
	case int:
		return float32(value), true
	case map[string]any:
		if inner, ok := widgetDecodedStructField(value, "value"); ok {
			return widgetReadFloatValue(inner)
		}
	}
	return 0, false
}

func widgetReadBoolValue(raw any) (bool, bool) {
	switch value := raw.(type) {
	case bool:
		return value, true
	case map[string]any:
		if inner, ok := widgetDecodedStructField(value, "value"); ok {
			return widgetReadBoolValue(inner)
		}
	}
	return false, false
}

func widgetReadIntValue(raw any) (int, bool) {
	switch value := raw.(type) {
	case int:
		return value, true
	case int8:
		return int(value), true
	case int16:
		return int(value), true
	case int32:
		return int(value), true
	case int64:
		return int(value), true
	case float32:
		return int(value), true
	case float64:
		return int(value), true
	case map[string]any:
		if inner, ok := widgetDecodedStructField(value, "value"); ok {
			return widgetReadIntValue(inner)
		}
	}
	return 0, false
}

func normalizeWidgetExportList(rootWidgetExport int, allWidgetExports []int) []int {
	out := make([]int, 0, len(allWidgetExports)+1)
	seen := make(map[int]bool, len(allWidgetExports)+1)
	for _, idx := range allWidgetExports {
		if idx <= 0 || seen[idx] {
			continue
		}
		seen[idx] = true
		out = append(out, idx)
	}
	if rootWidgetExport > 0 && !seen[rootWidgetExport] {
		out = append([]int{rootWidgetExport}, out...)
	}
	return out
}

func orderChildExportsForParent(slotExportRefs []int, slotByExport map[int]*widgetSlotData, fallback []int) []int {
	out := make([]int, 0, len(slotExportRefs)+len(fallback))
	seen := make(map[int]bool, len(slotExportRefs)+len(fallback))
	for _, slotExport := range slotExportRefs {
		slotInfo := slotByExport[slotExport]
		if slotInfo == nil || slotInfo.contentExport <= 0 || seen[slotInfo.contentExport] {
			continue
		}
		seen[slotInfo.contentExport] = true
		out = append(out, slotInfo.contentExport)
	}
	for _, childExport := range fallback {
		if childExport <= 0 || seen[childExport] {
			continue
		}
		seen[childExport] = true
		out = append(out, childExport)
	}
	return out
}

func widgetTreeRole(bpIdx, genClassIdx, outerIdx int, outerClassName string) string {
	switch {
	case outerIdx == bpIdx:
		return "designer"
	case outerIdx == genClassIdx:
		return "generated"
	case strings.EqualFold(outerClassName, "WidgetBlueprint"):
		return "designer"
	case strings.EqualFold(outerClassName, "WidgetBlueprintGeneratedClass"):
		return "generated"
	default:
		return "unknown"
	}
}

func appendUniqueInt(items []int, value int) []int {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func appendUniqueString(items []string, value string) []string {
	for _, item := range items {
		if strings.EqualFold(item, value) {
			return items
		}
	}
	return append(items, value)
}

// ---------------------------------------------------------------------------
// Property decode helper
// ---------------------------------------------------------------------------

// decodeAllProperties decodes all properties in a slice and returns a map
// keyed by property name. Only the first occurrence of each name is kept.
func decodeAllProperties(asset *uasset.Asset, props []uasset.PropertyTag) map[string]any {
	out := make(map[string]any, len(props))
	for _, p := range props {
		name := p.Name.Display(asset.Names)
		if _, exists := out[name]; exists {
			continue
		}
		if v, ok := asset.DecodePropertyValue(p); ok {
			enrichDecodedTextValue(v)
			out[name] = v
		}
	}
	return out
}
