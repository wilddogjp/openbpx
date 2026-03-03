package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

func runPackageCustomVersions(args []string, stdout, stderr io.Writer) int {
	file, asset, ok := parseSingleAssetCommand(args, "package custom-versions", "usage: bpx package custom-versions <file.uasset>", stderr)
	if !ok {
		return 1
	}
	items := make([]map[string]any, 0, len(asset.Summary.CustomVersions))
	for _, cv := range asset.Summary.CustomVersions {
		items = append(items, map[string]any{
			"guid":    cv.Key.String(),
			"version": cv.Version,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i]["guid"].(string) < items[j]["guid"].(string)
	})
	return printJSON(stdout, map[string]any{
		"file":           file,
		"customVersions": items,
	})
}

func runPackageDepends(args []string, stdout, stderr io.Writer) int {
	file, asset, ok := parseSingleAssetCommand(args, "package depends", "usage: bpx package depends <file.uasset>", stderr)
	if !ok {
		return 1
	}
	items, warnings := parseDependsMap(asset)
	return printJSON(stdout, map[string]any{
		"file":       file,
		"dependsMap": items,
		"warnings":   warnings,
	})
}

func runPackageResolveIndex(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("package resolve-index", stderr)
	opts := registerCommonFlags(fs)
	index := fs.Int("index", 0, "package index (signed int32)")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: bpx package resolve-index <file.uasset> --index <i>")
		return 1
	}
	if *index < math.MinInt32 || *index > math.MaxInt32 {
		fmt.Fprintf(stderr, "error: index out of int32 range: %d\n", *index)
		return 1
	}
	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	pkgIdx := uasset.PackageIndex(int32(*index))
	return printJSON(stdout, map[string]any{
		"file":     file,
		"index":    int32(*index),
		"kind":     pkgIdx.Kind(),
		"resolved": asset.ParseIndex(pkgIdx),
	})
}

func runPackageSection(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("package section", stderr)
	opts := registerCommonFlags(fs)
	section := fs.String("name", "", "section name")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || strings.TrimSpace(*section) == "" {
		fmt.Fprintln(stderr, "usage: bpx package section <file.uasset> --name <soft-object-paths|gatherable-text|thumbnails|searchable-names|world-tile|asset-registry|bulk-data>")
		return 1
	}
	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	resp, err := buildPackageSectionResponse(file, asset, *section)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	return printJSON(stdout, resp)
}

func buildPackageSectionResponse(file string, asset *uasset.Asset, section string) (map[string]any, error) {
	normalized, ok := normalizePackageSectionName(section)
	if !ok {
		return nil, fmt.Errorf("unsupported section name: %s", section)
	}

	start := int64(0)
	responseName := ""
	switch normalized {
	case "soft-object-paths":
		start = int64(asset.Summary.SoftObjectPathsOffset)
		responseName = "softObjectPaths"
	case "gatherable-text":
		start = int64(asset.Summary.GatherableTextDataOffset)
		responseName = "gatherableTextData"
	case "thumbnails":
		start = int64(asset.Summary.ThumbnailTableOffset)
		responseName = "thumbnailTable"
	case "searchable-names":
		start = int64(asset.Summary.SearchableNamesOffset)
		responseName = "searchableNames"
	case "world-tile":
		start = int64(asset.Summary.WorldTileInfoDataOffset)
		responseName = "worldTileInfo"
	case "asset-registry":
		start = int64(asset.Summary.AssetRegistryDataOffset)
		responseName = "assetRegistryData"
	case "bulk-data":
		start = asset.Summary.BulkDataStartOffset
		responseName = "bulkData"
	default:
		return nil, fmt.Errorf("unsupported section name: %s", section)
	}

	data, begin, end, present := sectionByOffset(asset, start)
	resp := baseSectionResponse(file, responseName, begin, end, data, present)
	resp["sectionName"] = normalized
	switch normalized {
	case "soft-object-paths":
		entries, warnings := parseSoftObjectPathEntries(asset, data, asset.Summary.SoftObjectPathsCount)
		resp["count"] = asset.Summary.SoftObjectPathsCount
		resp["entries"] = entries
		resp["warnings"] = warnings
	case "gatherable-text":
		resp["count"] = asset.Summary.GatherableTextDataCount
	case "bulk-data":
		resp["bulkDataStartOffset"] = asset.Summary.BulkDataStartOffset
		resp["dataResourceOffset"] = asset.Summary.DataResourceOffset
	}
	return resp, nil
}

func normalizePackageSectionName(section string) (string, bool) {
	token := strings.ToLower(strings.TrimSpace(section))

	switch token {
	case "soft-object-paths":
		return "soft-object-paths", true
	case "gatherable-text":
		return "gatherable-text", true
	case "thumbnails":
		return "thumbnails", true
	case "searchable-names":
		return "searchable-names", true
	case "world-tile":
		return "world-tile", true
	case "asset-registry":
		return "asset-registry", true
	case "bulk-data":
		return "bulk-data", true
	default:
		return "", false
	}
}

func runDataTable(args []string, stdout, stderr io.Writer) int {
	return dispatchSubcommand(
		args,
		stdout,
		stderr,
		"usage: bpx datatable <read|update-row|add-row|remove-row> ...",
		"unknown datatable command: %s\n",
		subcommandSpec{Name: "read", Run: runDataTableRead},
		subcommandSpec{Name: "update-row", Run: runDataTableUpdateRow},
		subcommandSpec{Name: "add-row", Run: runDataTableAddRow},
		subcommandSpec{Name: "remove-row", Run: runDataTableRemoveRow},
	)
}

func runDataTableRead(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("datatable read", stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("export", 0, "optional 1-based export index")
	allowOutputFormats(fs, "output format: json, toml, csv, or tsv", structuredOutputFormatJSON, structuredOutputFormatTOML, "csv", "tsv")
	outPath := fs.String("out", "", "write output to path instead of stdout")
	var rowFilters dataTableRowFilterFlag
	fs.Var(&rowFilters, "row", "optional row name filter (repeatable or comma-separated)")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: bpx datatable read <file.uasset> [--export <n>] [--row <name>] [--format json|toml|csv|tsv] [--out path]")
		return 1
	}
	format := outputFormatFromFlagSet(fs)
	file := fs.Arg(0)
	if *outPath != "" {
		if err := ensureOutputPathDistinctFromInput(file, *outPath); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	}
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	resp, err := buildDataTableReadResponse(file, asset, *exportIndex, rowFilters.Normalized())
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	return emitDataTableReadOutput(stdout, stderr, file, resp, format, *outPath)
}

func emitDataTableReadOutput(stdout, stderr io.Writer, file string, payload map[string]any, format, outPath string) int {
	switch strings.ToLower(format) {
	case "json", "toml":
		if outPath == "" {
			return printJSON(stdout, payload)
		}
		body, err := marshalStructuredPayload(payload, format)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		if err := writeFileAtomically(outPath, body, 0o644); err != nil {
			fmt.Fprintf(stderr, "error: write output: %v\n", err)
			return 1
		}
	case "csv", "tsv":
		delimiter := ','
		if strings.ToLower(format) == "tsv" {
			delimiter = '\t'
		}
		body, err := marshalDataTableFlatRows(payload, delimiter)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		if outPath == "" {
			if _, err := io.WriteString(stdout, body); err != nil {
				fmt.Fprintf(stderr, "error: write output: %v\n", err)
				return 1
			}
			return 0
		}
		if err := writeFileAtomically(outPath, []byte(body), 0o644); err != nil {
			fmt.Fprintf(stderr, "error: write output: %v\n", err)
			return 1
		}
	default:
		fmt.Fprintf(stderr, "error: unsupported read format: %s\n", format)
		return 1
	}

	return printJSON(stdout, map[string]any{
		"file":   file,
		"format": strings.ToLower(format),
		"out":    outPath,
	})
}

func buildDataTableReadResponse(file string, asset *uasset.Asset, exportIndex int, rowFilters []string) (map[string]any, error) {
	filterSet := make(map[string]struct{}, len(rowFilters))
	for _, row := range rowFilters {
		filterSet[row] = struct{}{}
	}
	matchedRows := make(map[string]bool, len(filterSet))
	targets := make([]int, 0, 4)
	if exportIndex > 0 {
		idx, err := asset.ResolveExportIndex(exportIndex)
		if err != nil {
			return nil, fmt.Errorf("resolve export index: %w", err)
		}
		targets = append(targets, idx)
	} else {
		for i, exp := range asset.Exports {
			className := asset.ResolveClassName(exp)
			if className == "DataTable" || className == "CurveTable" || className == "CompositeDataTable" {
				targets = append(targets, i)
			}
		}
	}

	tables := make([]map[string]any, 0, len(targets))
	for _, idx := range targets {
		exp := asset.Exports[idx]
		props := asset.ParseExportProperties(idx)
		rows, rowWarnings := parseDataTableRows(asset, idx, props)
		filteredRows := rows
		if len(filterSet) > 0 {
			filteredRows = filterDataTableRows(rows, filterSet, matchedRows)
		}
		className := asset.ResolveClassName(exp)
		table := map[string]any{
			"index":            idx + 1,
			"objectName":       exp.ObjectName.Display(asset.Names),
			"className":        className,
			"headerProperties": toPropertyOutputs(asset, props.Properties, true),
			"headerWarnings":   props.Warnings,
			"rowCount":         len(filteredRows),
			"rows":             filteredRows,
			"rowWarnings":      rowWarnings,
		}
		if className == "CompositeDataTable" {
			parents, parentWarnings := buildCompositeParentRefs(asset, props.Properties)
			table["compositeParentCount"] = len(parents)
			table["compositeParents"] = parents
			if len(parentWarnings) > 0 {
				table["compositeParentWarnings"] = parentWarnings
			}
		}
		tables = append(tables, table)
	}

	if len(filterSet) > 0 {
		missing := make([]string, 0, len(filterSet))
		for row := range filterSet {
			if !matchedRows[row] {
				missing = append(missing, row)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			return nil, fmt.Errorf("datatable rows not found: %s", strings.Join(missing, ", "))
		}
	}

	return map[string]any{
		"file":       file,
		"tableCount": len(tables),
		"tables":     tables,
	}, nil
}

func filterDataTableRows(rows []map[string]any, filterSet map[string]struct{}, matched map[string]bool) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		rowName, _ := row["rowName"].(string)
		if _, ok := filterSet[rowName]; !ok {
			continue
		}
		matched[rowName] = true
		out = append(out, row)
	}
	return out
}

type dataTableRowFilterFlag []string

func (f *dataTableRowFilterFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *dataTableRowFilterFlag) Set(value string) error {
	for _, token := range strings.Split(value, ",") {
		row := strings.TrimSpace(token)
		if row == "" {
			continue
		}
		*f = append(*f, row)
	}
	return nil
}

func (f dataTableRowFilterFlag) Normalized() []string {
	if len(f) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(f))
	out := make([]string, 0, len(f))
	for _, row := range f {
		if row == "" {
			continue
		}
		if _, ok := seen[row]; ok {
			continue
		}
		seen[row] = struct{}{}
		out = append(out, row)
	}
	sort.Strings(out)
	return out
}

func buildCompositeParentRefs(asset *uasset.Asset, props []uasset.PropertyTag) ([]map[string]any, []string) {
	parentTag := (*uasset.PropertyTag)(nil)
	for i := range props {
		p := &props[i]
		if p.ArrayIndex != 0 {
			continue
		}
		if p.Name.Display(asset.Names) != "ParentTables" {
			continue
		}
		parentTag = p
		break
	}
	if parentTag == nil {
		return []map[string]any{}, nil
	}

	decoded, ok := asset.DecodePropertyValue(*parentTag)
	if !ok {
		return []map[string]any{}, []string{"decode ParentTables value failed"}
	}
	parentMap, ok := decoded.(map[string]any)
	if !ok {
		return []map[string]any{}, []string{"ParentTables decoded value shape is invalid"}
	}
	valueRaw, ok := parentMap["value"]
	if !ok {
		return []map[string]any{}, []string{"ParentTables decoded value has no entries"}
	}

	var entries []any
	switch list := valueRaw.(type) {
	case []any:
		entries = list
	case []map[string]any:
		entries = make([]any, 0, len(list))
		for _, item := range list {
			entries = append(entries, item)
		}
	default:
		return []map[string]any{}, []string{"ParentTables decoded entries type is unsupported"}
	}

	refs := make([]map[string]any, 0, len(entries))
	warnings := make([]string, 0, 4)
	for i, entry := range entries {
		rawIndex, ok := decodeObjectIndex(entry)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("ParentTables[%d] decode failed", i))
			continue
		}

		idx := uasset.PackageIndex(rawIndex)
		ref := map[string]any{
			"slot":     i,
			"index":    rawIndex,
			"kind":     idx.Kind(),
			"resolved": asset.ParseIndex(idx),
		}

		switch idx.Kind() {
		case "import":
			importIdx := idx.ResolveIndex()
			if importIdx >= 0 && importIdx < len(asset.Imports) {
				imp := asset.Imports[importIdx]
				packageName := imp.PackageName.Display(asset.Names)
				objectName := imp.ObjectName.Display(asset.Names)
				targetPath := resolveImportTargetPath(asset, imp)
				targetObjectPath := targetPath
				if strings.HasPrefix(targetPath, "/") &&
					objectName != "" &&
					!strings.EqualFold(objectName, "None") &&
					!strings.Contains(targetPath, ".") {
					targetObjectPath = targetPath + "." + objectName
				}
				ref["importIndex"] = importIdx + 1
				ref["classPackage"] = imp.ClassPackage.Display(asset.Names)
				ref["className"] = imp.ClassName.Display(asset.Names)
				ref["packageName"] = packageName
				ref["objectName"] = objectName
				ref["targetPath"] = targetPath
				ref["targetObjectPath"] = targetObjectPath
			}
		case "export":
			exportIdx := idx.ResolveIndex()
			if exportIdx >= 0 && exportIdx < len(asset.Exports) {
				exp := asset.Exports[exportIdx]
				ref["exportIndex"] = exportIdx + 1
				ref["objectName"] = exp.ObjectName.Display(asset.Names)
				ref["className"] = asset.ResolveClassName(exp)
			}
		}
		refs = append(refs, ref)
	}

	return refs, warnings
}

func decodeObjectIndex(v any) (int32, bool) {
	switch t := v.(type) {
	case int32:
		return t, true
	case int:
		return int32(t), true
	case int64:
		return int32(t), true
	case float64:
		return int32(t), true
	case map[string]any:
		if value, ok := t["value"]; ok {
			if idx, ok := decodeObjectIndex(value); ok {
				return idx, true
			}
		}
		if index, ok := t["index"]; ok {
			if idx, ok := decodeObjectIndex(index); ok {
				return idx, true
			}
		}
	}
	return 0, false
}

func marshalDataTableFlatRows(payload map[string]any, delimiter rune) (string, error) {
	tablesAny, ok := payload["tables"]
	if !ok {
		return "", fmt.Errorf("payload has no tables")
	}
	tables, ok := tablesAny.([]map[string]any)
	if !ok {
		return "", fmt.Errorf("tables payload type mismatch")
	}

	var b strings.Builder
	w := csv.NewWriter(&b)
	w.Comma = delimiter
	if err := w.Write([]string{
		"file", "tableIndex", "tableName", "rowIndex", "rowName", "propertyName", "propertyType", "value",
	}); err != nil {
		return "", fmt.Errorf("write header: %w", err)
	}
	for _, t := range tables {
		tableIndex := fmt.Sprint(t["index"])
		tableName := fmt.Sprint(t["objectName"])
		rowsAny, _ := t["rows"].([]map[string]any)
		for _, row := range rowsAny {
			rowIndex := fmt.Sprint(row["rowIndex"])
			rowName := fmt.Sprint(row["rowName"])
			propsAny, _ := row["properties"].([]map[string]any)
			for _, p := range propsAny {
				value := ""
				if v, ok := p["value"]; ok {
					value = renderAnyValue(v)
				}
				if err := w.Write([]string{
					fmt.Sprint(payload["file"]),
					tableIndex,
					tableName,
					rowIndex,
					rowName,
					fmt.Sprint(p["name"]),
					fmt.Sprint(p["type"]),
					value,
				}); err != nil {
					return "", fmt.Errorf("write row: %w", err)
				}
			}
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", fmt.Errorf("flush writer: %w", err)
	}
	return b.String(), nil
}

func renderAnyValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprint(t)
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprint(t)
		}
		return string(b)
	}
}

func parseDependsMap(asset *uasset.Asset) ([]map[string]any, []string) {
	data, _, _, present := sectionByOffset(asset, int64(asset.Summary.DependsOffset))
	if !present {
		return []map[string]any{}, []string{"depends map section not present"}
	}
	r := uasset.NewByteReaderWithByteSwapping(data, asset.Summary.UsesByteSwappedSerialization())
	items := make([]map[string]any, 0, len(asset.Exports))
	warnings := make([]string, 0, 4)

	for i := range asset.Exports {
		count, err := r.ReadInt32()
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("depends parse stopped at export %d: %v", i+1, err))
			break
		}
		if count < 0 || count > 1_000_000 {
			warnings = append(warnings, fmt.Sprintf("depends parse stopped at export %d: invalid dependency count=%d", i+1, count))
			break
		}
		deps := make([]map[string]any, 0, count)
		for j := int32(0); j < count; j++ {
			rawIdx, err := r.ReadInt32()
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("depends parse stopped at export %d dependency %d: %v", i+1, j, err))
				break
			}
			pkgIdx := uasset.PackageIndex(rawIdx)
			deps = append(deps, map[string]any{
				"index":    rawIdx,
				"resolved": asset.ParseIndex(pkgIdx),
			})
		}
		items = append(items, map[string]any{
			"export":          i + 1,
			"exportName":      asset.Exports[i].ObjectName.Display(asset.Names),
			"dependencyCount": len(deps),
			"dependencies":    deps,
		})
	}
	if r.Remaining() > 0 {
		warnings = append(warnings, fmt.Sprintf("depends map trailing bytes: %d", r.Remaining()))
	}
	return items, warnings
}

func parseSoftObjectPathEntries(asset *uasset.Asset, data []byte, count int32) ([]map[string]any, []string) {
	if count <= 0 || len(data) == 0 {
		return []map[string]any{}, nil
	}
	if !asset.Summary.SupportsSoftObjectPathListInSummary() {
		return []map[string]any{}, []string{
			fmt.Sprintf(
				"soft object path list is unavailable for fileVersionUE5=%d (requires >= %d)",
				asset.Summary.FileVersionUE5,
				1008,
			),
		}
	}
	r := uasset.NewByteReaderWithByteSwapping(data, asset.Summary.UsesByteSwappedSerialization())
	entries := make([]map[string]any, 0, count)
	warnings := make([]string, 0, 4)

	for i := int32(0); i < count; i++ {
		pkg, err := r.ReadNameRef(len(asset.Names))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("soft object path parse stopped at %d: %v", i, err))
			break
		}
		assetName, err := r.ReadNameRef(len(asset.Names))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("soft object path parse stopped at %d asset name: %v", i, err))
			break
		}
		subPath, err := r.ReadSoftObjectSubPath()
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("soft object path parse stopped at %d sub path: %v", i, err))
			break
		}
		entries = append(entries, map[string]any{
			"packageName": pkg.Display(asset.Names),
			"assetName":   assetName.Display(asset.Names),
			"subPath":     subPath,
		})
	}
	if r.Remaining() > 0 {
		warnings = append(warnings, fmt.Sprintf("soft object path trailing bytes: %d", r.Remaining()))
	}
	return entries, warnings
}

type dataTableRowCandidate struct {
	rowCount    int32
	startOffset int
	label       string
}

type dataTableRowParseResult struct {
	rows     []map[string]any
	warnings []string
	trailing int
	complete bool
}

func parseDataTableRows(asset *uasset.Asset, exportIndex int, headerProps uasset.PropertyListResult) ([]map[string]any, []string) {
	exp := asset.Exports[exportIndex]
	rowEnd := int(exp.SerialOffset + exp.SerialSize)
	rowStart := headerProps.EndOffset
	if rowStart < int(exp.SerialOffset) || rowStart > rowEnd {
		return []map[string]any{}, []string{"row section start out of range"}
	}
	r := uasset.NewByteReaderWithByteSwapping(asset.Raw.Bytes[rowStart:rowEnd], asset.Summary.UsesByteSwappedSerialization())
	if r.Remaining() == 0 {
		return []map[string]any{}, nil
	}
	if r.Remaining() < 4 {
		return []map[string]any{}, []string{"read row count: unexpected EOF"}
	}
	firstCount, err := r.ReadInt32()
	if err != nil {
		return []map[string]any{}, []string{fmt.Sprintf("read row count: %v", err)}
	}

	candidates := []dataTableRowCandidate{
		{rowCount: firstCount, startOffset: 4, label: "count@+0"},
	}
	// Some cooked DataTable assets include a leading int32 prefix before NumRows.
	// Keep a secondary candidate and validate by actual row parse completeness.
	if (firstCount == 0 || firstCount == 1) && r.Remaining() >= 4 {
		secondCount, secondErr := r.ReadInt32()
		if secondErr == nil {
			candidates = append(candidates, dataTableRowCandidate{
				rowCount:    secondCount,
				startOffset: 8,
				label:       "count@+4",
			})
		}
	}

	bestIdx := -1
	results := make([]dataTableRowParseResult, 0, len(candidates))
	for _, c := range candidates {
		if c.rowCount < 0 || c.rowCount > 1_000_000 {
			results = append(results, dataTableRowParseResult{
				rows:     []map[string]any{},
				warnings: []string{fmt.Sprintf("invalid row count candidate (%s): %d", c.label, c.rowCount)},
				trailing: rowEnd - rowStart,
				complete: false,
			})
			continue
		}
		result := parseDataTableRowsWithCount(asset, rowStart, rowEnd, c.startOffset, c.rowCount)
		results = append(results, result)
		if bestIdx == -1 {
			bestIdx = len(results) - 1
			continue
		}
		best := results[bestIdx]
		cur := result
		if cur.complete != best.complete {
			if cur.complete {
				bestIdx = len(results) - 1
			}
			continue
		}
		if len(cur.rows) != len(best.rows) {
			if len(cur.rows) > len(best.rows) {
				bestIdx = len(results) - 1
			}
			continue
		}
		if cur.trailing != best.trailing {
			if cur.trailing < best.trailing {
				bestIdx = len(results) - 1
			}
			continue
		}
		if len(cur.warnings) < len(best.warnings) {
			bestIdx = len(results) - 1
		}
	}

	if bestIdx == -1 {
		return []map[string]any{}, []string{"no valid datatable row count candidate"}
	}
	best := results[bestIdx]
	if len(candidates) > 1 && bestIdx == 1 {
		best.warnings = append([]string{
			fmt.Sprintf("datatable row count selected from secondary header (%s=%d)", candidates[bestIdx].label, candidates[bestIdx].rowCount),
		}, best.warnings...)
	}
	return best.rows, best.warnings
}

func parseDataTableRowsWithCount(asset *uasset.Asset, rowStart, rowEnd, startOffset int, rowCount int32) dataTableRowParseResult {
	if startOffset < 0 || rowStart+startOffset > rowEnd {
		return dataTableRowParseResult{
			rows:     []map[string]any{},
			warnings: []string{"datatable row section start offset out of bounds"},
			trailing: rowEnd - rowStart,
			complete: false,
		}
	}
	r := uasset.NewByteReaderWithByteSwapping(asset.Raw.Bytes[rowStart+startOffset:rowEnd], asset.Summary.UsesByteSwappedSerialization())
	rows := make([]map[string]any, 0, rowCount)
	warnings := make([]string, 0, 8)
	for i := int32(0); i < rowCount; i++ {
		rowName, err := r.ReadNameRef(len(asset.Names))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("read row name at row %d: %v", i, err))
			break
		}
		rowDataStart := rowStart + startOffset + r.Offset()
		rowProps := asset.ParseTaggedPropertiesRange(rowDataStart, rowEnd, false)
		if rowProps.EndOffset <= rowDataStart {
			for _, w := range rowProps.Warnings {
				warnings = append(warnings, fmt.Sprintf("row %d property parse warning: %s", i, w))
			}
			warnings = append(warnings, fmt.Sprintf("row %d parse made no progress", i))
			break
		}
		if err := r.Seek(rowProps.EndOffset - (rowStart + startOffset)); err != nil {
			warnings = append(warnings, fmt.Sprintf("row %d seek failed: %v", i, err))
			break
		}
		rows = append(rows, map[string]any{
			"rowIndex":      i,
			"rowName":       rowName.Display(asset.Names),
			"propertyCount": len(rowProps.Properties),
			"properties":    toPropertyOutputs(asset, rowProps.Properties, true),
			"warnings":      rowProps.Warnings,
		})
	}
	trailing := r.Remaining()
	if r.Remaining() > 0 {
		warnings = append(warnings, fmt.Sprintf("datatable trailing bytes: %d", trailing))
	}
	return dataTableRowParseResult{
		rows:     rows,
		warnings: warnings,
		trailing: trailing,
		complete: int32(len(rows)) == rowCount,
	}
}
