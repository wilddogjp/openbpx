package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/wilddogjp/openbpx/pkg/edit"
	"github.com/wilddogjp/openbpx/pkg/uasset"
)

func runWrite(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("write", stderr)
	opts := registerCommonFlags(fs)
	outPath := fs.String("out", "", "output .uasset path")
	dryRun := fs.Bool("dry-run", false, "do not write output")
	backup := fs.Bool("backup", false, "create <out>.backup when destination exists")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || *outPath == "" {
		fmt.Fprintln(stderr, "usage: bpx write <file.uasset> --out <new.uasset> [--dry-run] [--backup]")
		return 1
	}

	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	outBytes, err := edit.RewriteAsset(asset, nil)
	if err != nil {
		fmt.Fprintf(stderr, "error: rewrite asset: %v\n", err)
		return 1
	}
	changed := !bytes.Equal(asset.Raw.Bytes, outBytes)

	if *dryRun {
		return printJSON(stdout, map[string]any{
			"file":    file,
			"out":     *outPath,
			"dryRun":  true,
			"changed": changed,
			"bytes":   len(outBytes),
		})
	}

	if *backup {
		if err := createBackupIfExists(*outPath); err != nil {
			fmt.Fprintf(stderr, "error: backup output file: %v\n", err)
			return 1
		}
	}
	if err := os.WriteFile(*outPath, outBytes, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write output: %v\n", err)
		return 1
	}
	return printJSON(stdout, map[string]any{
		"file":    file,
		"out":     *outPath,
		"dryRun":  false,
		"changed": changed,
		"bytes":   len(outBytes),
	})
}

func runPropSet(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("prop set", stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("export", 0, "1-based export index")
	path := fs.String("path", "", "property path")
	valueJSON := fs.String("value", "", "JSON literal value")
	dryRun := fs.Bool("dry-run", false, "do not write output")
	backup := fs.Bool("backup", false, "create <file>.backup before overwrite")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || *exportIndex <= 0 || strings.TrimSpace(*path) == "" || strings.TrimSpace(*valueJSON) == "" {
		fmt.Fprintln(stderr, "usage: bpx prop set <file.uasset> --export <n> --path <dot.path> --value '<json>' [--dry-run] [--backup]")
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

	editResult, err := edit.BuildPropertySetMutation(asset, idx, *path, *valueJSON)
	if err != nil {
		fallbackResult, fallbackErr := buildPropertySetStructLeafFallbackMutation(asset, idx, *path, *valueJSON)
		if fallbackErr != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		editResult = fallbackResult
	}

	outBytes, err := edit.RewriteAsset(asset, []edit.ExportMutation{editResult.Mutation})
	if err != nil {
		fmt.Fprintf(stderr, "error: rewrite asset: %v\n", err)
		return 1
	}
	changed := !bytes.Equal(asset.Raw.Bytes, outBytes)

	resp := map[string]any{
		"file":        file,
		"export":      *exportIndex,
		"path":        *path,
		"property":    editResult.PropertyName,
		"oldValue":    editResult.OldValue,
		"newValue":    editResult.NewValue,
		"oldSize":     editResult.OldSize,
		"newSize":     editResult.NewSize,
		"byteDelta":   editResult.ByteDelta,
		"dryRun":      *dryRun,
		"changed":     changed,
		"outputBytes": len(outBytes),
	}

	if *dryRun {
		return printJSON(stdout, resp)
	}
	if *backup {
		if err := createBackupFile(file); err != nil {
			fmt.Fprintf(stderr, "error: backup source file: %v\n", err)
			return 1
		}
	}
	if err := os.WriteFile(file, outBytes, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write file: %v\n", err)
		return 1
	}
	return printJSON(stdout, resp)
}

func runPropAdd(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("prop add", stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("export", 0, "1-based export index")
	specJSON := fs.String("spec", "", "property add spec JSON")
	dryRun := fs.Bool("dry-run", false, "do not write output")
	backup := fs.Bool("backup", false, "create <file>.backup before overwrite")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || *exportIndex <= 0 || strings.TrimSpace(*specJSON) == "" {
		fmt.Fprintln(stderr, "usage: bpx prop add <file.uasset> --export <n> --spec '<json>' [--dry-run] [--backup]")
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

	editResult, err := edit.BuildPropertyAddMutation(asset, idx, *specJSON)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	outBytes, err := edit.RewriteAsset(asset, []edit.ExportMutation{editResult.Mutation})
	if err != nil {
		fmt.Fprintf(stderr, "error: rewrite asset: %v\n", err)
		return 1
	}
	changed := !bytes.Equal(asset.Raw.Bytes, outBytes)

	resp := map[string]any{
		"file":        file,
		"export":      *exportIndex,
		"property":    editResult.PropertyName,
		"type":        editResult.PropertyType,
		"arrayIndex":  editResult.ArrayIndex,
		"newValue":    editResult.NewValue,
		"newSize":     editResult.NewSize,
		"byteDelta":   editResult.ByteDelta,
		"dryRun":      *dryRun,
		"changed":     changed,
		"outputBytes": len(outBytes),
	}

	if *dryRun {
		return printJSON(stdout, resp)
	}
	if *backup {
		if err := createBackupFile(file); err != nil {
			fmt.Fprintf(stderr, "error: backup source file: %v\n", err)
			return 1
		}
	}
	if err := os.WriteFile(file, outBytes, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write file: %v\n", err)
		return 1
	}
	return printJSON(stdout, resp)
}

func runPropRemove(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("prop remove", stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("export", 0, "1-based export index")
	path := fs.String("path", "", "property path")
	dryRun := fs.Bool("dry-run", false, "do not write output")
	backup := fs.Bool("backup", false, "create <file>.backup before overwrite")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || *exportIndex <= 0 || strings.TrimSpace(*path) == "" {
		fmt.Fprintln(stderr, "usage: bpx prop remove <file.uasset> --export <n> --path <dot.path> [--dry-run] [--backup]")
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

	editResult, err := edit.BuildPropertyRemoveMutation(asset, idx, *path)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	outBytes, err := edit.RewriteAsset(asset, []edit.ExportMutation{editResult.Mutation})
	if err != nil {
		fmt.Fprintf(stderr, "error: rewrite asset: %v\n", err)
		return 1
	}
	changed := !bytes.Equal(asset.Raw.Bytes, outBytes)

	resp := map[string]any{
		"file":        file,
		"export":      *exportIndex,
		"path":        *path,
		"property":    editResult.PropertyName,
		"arrayIndex":  editResult.ArrayIndex,
		"oldValue":    editResult.OldValue,
		"oldSize":     editResult.OldSize,
		"byteDelta":   editResult.ByteDelta,
		"dryRun":      *dryRun,
		"changed":     changed,
		"outputBytes": len(outBytes),
	}

	if *dryRun {
		return printJSON(stdout, resp)
	}
	if *backup {
		if err := createBackupFile(file); err != nil {
			fmt.Fprintf(stderr, "error: backup source file: %v\n", err)
			return 1
		}
	}
	if err := os.WriteFile(file, outBytes, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write file: %v\n", err)
		return 1
	}
	return printJSON(stdout, resp)
}

func runVar(args []string, stdout, stderr io.Writer) int {
	return dispatchSubcommand(
		args,
		stdout,
		stderr,
		"usage: bpx var <list|set-default|rename> ...",
		"unknown var command: %s\n",
		subcommandSpec{Name: "list", Run: runVarList},
		subcommandSpec{Name: "set-default", Run: runVarSetDefault},
		subcommandSpec{Name: "rename", Run: runVarRename},
	)
}

func runVarList(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("var list", stderr)
	opts := registerCommonFlags(fs)
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: bpx var list <file.uasset>")
		return 1
	}
	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	cdoIdx, err := findCDOExportIndex(asset)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	cdoProps := asset.ParseExportProperties(cdoIdx)
	if len(cdoProps.Warnings) > 0 {
		fmt.Fprintf(stderr, "error: cannot parse CDO properties: %s\n", strings.Join(cdoProps.Warnings, "; "))
		return 1
	}

	type varRecord struct {
		Name     string `json:"name"`
		Type     string `json:"type,omitempty"`
		Default  any    `json:"default,omitempty"`
		Source   string `json:"source"`
		Mismatch bool   `json:"mismatch"`
	}

	records := map[string]varRecord{}
	for _, p := range cdoProps.Properties {
		name := p.Name.Display(asset.Names)
		if name == "" || name == "None" {
			continue
		}
		typeName := p.TypeString(asset.Names)
		var decoded any
		if v, ok := asset.DecodePropertyValue(p); ok {
			decoded = v
		}
		records[name] = varRecord{
			Name:     name,
			Type:     typeName,
			Default:  decoded,
			Source:   "cdo",
			Mismatch: false,
		}
	}

	declared, declWarnings := collectDeclaredVariables(asset)
	for name, declType := range declared {
		rec, exists := records[name]
		if !exists {
			records[name] = varRecord{
				Name:     name,
				Type:     declType,
				Source:   "declaration",
				Mismatch: false,
			}
			continue
		}
		rec.Source = "merged"
		if declType != "" && rec.Type != "" && declType != rec.Type {
			rec.Mismatch = true
		}
		if rec.Type == "" {
			rec.Type = declType
		}
		records[name] = rec
	}

	keys := make([]string, 0, len(records))
	for k := range records {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]varRecord, 0, len(keys))
	for _, k := range keys {
		out = append(out, records[k])
	}

	return printJSON(stdout, map[string]any{
		"file":      file,
		"cdoExport": cdoIdx + 1,
		"count":     len(out),
		"variables": out,
		"warnings":  declWarnings,
	})
}

func runVarSetDefault(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("var set-default", stderr)
	opts := registerCommonFlags(fs)
	name := fs.String("name", "", "variable name")
	valueJSON := fs.String("value", "", "JSON literal value")
	dryRun := fs.Bool("dry-run", false, "do not write output")
	backup := fs.Bool("backup", false, "create <file>.backup before overwrite")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || strings.TrimSpace(*name) == "" || strings.TrimSpace(*valueJSON) == "" {
		fmt.Fprintln(stderr, "usage: bpx var set-default <file.uasset> --name <var> --value '<json>' [--dry-run] [--backup]")
		return 1
	}

	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	cdoIdx, err := findCDOExportIndex(asset)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	editResult, err := edit.BuildPropertySetMutation(asset, cdoIdx, *name, *valueJSON)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	outBytes, err := edit.RewriteAsset(asset, []edit.ExportMutation{editResult.Mutation})
	if err != nil {
		fmt.Fprintf(stderr, "error: rewrite asset: %v\n", err)
		return 1
	}
	changed := !bytes.Equal(asset.Raw.Bytes, outBytes)

	resp := map[string]any{
		"file":        file,
		"name":        *name,
		"export":      cdoIdx + 1,
		"oldValue":    editResult.OldValue,
		"newValue":    editResult.NewValue,
		"oldSize":     editResult.OldSize,
		"newSize":     editResult.NewSize,
		"byteDelta":   editResult.ByteDelta,
		"dryRun":      *dryRun,
		"changed":     changed,
		"outputBytes": len(outBytes),
	}

	if *dryRun {
		return printJSON(stdout, resp)
	}
	if *backup {
		if err := createBackupFile(file); err != nil {
			fmt.Fprintf(stderr, "error: backup source file: %v\n", err)
			return 1
		}
	}
	if err := os.WriteFile(file, outBytes, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write file: %v\n", err)
		return 1
	}
	return printJSON(stdout, resp)
}

func runLevelVarList(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("level var-list", stderr)
	opts := registerCommonFlags(fs)
	actorSelector := fs.String("actor", "", "actor selector: object name, PersistentLevel.<name>, or export index")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || strings.TrimSpace(*actorSelector) == "" {
		fmt.Fprintln(stderr, "usage: bpx level var-list <file.umap> --actor <name|PersistentLevel.Name|export-index>")
		return 1
	}

	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	targetExport, matchedBy, err := resolveLevelActorExportIndex(asset, *actorSelector)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	props := asset.ParseExportProperties(targetExport)
	return printJSON(stdout, map[string]any{
		"file":       file,
		"selector":   *actorSelector,
		"actor":      levelActorInfo(asset, targetExport, matchedBy),
		"count":      len(props.Properties),
		"properties": toPropertyOutputs(asset, props.Properties, true),
		"warnings":   props.Warnings,
	})
}

func runLevelVarSet(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("level var-set", stderr)
	opts := registerCommonFlags(fs)
	actorSelector := fs.String("actor", "", "actor selector: object name, PersistentLevel.<name>, or export index")
	path := fs.String("path", "", "property path")
	valueJSON := fs.String("value", "", "JSON literal value")
	dryRun := fs.Bool("dry-run", false, "do not write output")
	backup := fs.Bool("backup", false, "create <file>.backup before overwrite")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || strings.TrimSpace(*actorSelector) == "" || strings.TrimSpace(*path) == "" || strings.TrimSpace(*valueJSON) == "" {
		fmt.Fprintln(stderr, "usage: bpx level var-set <file.umap> --actor <name|PersistentLevel.Name|export-index> --path <dot.path> --value '<json>' [--dry-run] [--backup]")
		return 1
	}

	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	targetExport, matchedBy, err := resolveLevelActorExportIndex(asset, *actorSelector)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	editResult, err := edit.BuildPropertySetMutation(asset, targetExport, *path, *valueJSON)
	if err != nil {
		fallbackResult, fallbackErr := buildPropertySetStructLeafFallbackMutation(asset, targetExport, *path, *valueJSON)
		if fallbackErr != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		editResult = fallbackResult
	}

	outBytes, err := edit.RewriteAsset(asset, []edit.ExportMutation{editResult.Mutation})
	if err != nil {
		fmt.Fprintf(stderr, "error: rewrite asset: %v\n", err)
		return 1
	}
	changed := !bytes.Equal(asset.Raw.Bytes, outBytes)

	resp := map[string]any{
		"file":        file,
		"selector":    *actorSelector,
		"actor":       levelActorInfo(asset, targetExport, matchedBy),
		"path":        *path,
		"property":    editResult.PropertyName,
		"oldValue":    editResult.OldValue,
		"newValue":    editResult.NewValue,
		"oldSize":     editResult.OldSize,
		"newSize":     editResult.NewSize,
		"byteDelta":   editResult.ByteDelta,
		"dryRun":      *dryRun,
		"changed":     changed,
		"outputBytes": len(outBytes),
	}

	if *dryRun {
		return printJSON(stdout, resp)
	}
	if *backup {
		if err := createBackupFile(file); err != nil {
			fmt.Fprintf(stderr, "error: backup source file: %v\n", err)
			return 1
		}
	}
	if err := os.WriteFile(file, outBytes, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write file: %v\n", err)
		return 1
	}
	return printJSON(stdout, resp)
}

func resolveLevelActorExportIndex(asset *uasset.Asset, selector string) (int, string, error) {
	if asset == nil {
		return 0, "", fmt.Errorf("asset is nil")
	}
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return 0, "", fmt.Errorf("empty actor selector")
	}

	persistentLevelIndex, candidates, err := collectPersistentLevelChildren(asset)
	if err != nil {
		return 0, "", err
	}

	if rawIndex, convErr := strconv.Atoi(selector); convErr == nil {
		idx, err := asset.ResolveExportIndex(rawIndex)
		if err != nil {
			return 0, "", err
		}
		if !isDirectChildOfPersistentLevel(asset, idx, persistentLevelIndex) {
			return 0, "", fmt.Errorf("export %d is not a direct child of PersistentLevel", rawIndex)
		}
		return idx, "export-index", nil
	}

	normalized := selector
	const persistentPrefix = "PersistentLevel."
	if strings.HasPrefix(strings.ToLower(normalized), strings.ToLower(persistentPrefix)) {
		normalized = normalized[len(persistentPrefix):]
	}

	matches := make([]int, 0, 2)
	for _, idx := range candidates {
		name := asset.Exports[idx].ObjectName.Display(asset.Names)
		if strings.EqualFold(name, normalized) {
			matches = append(matches, idx)
		}
	}
	if len(matches) == 1 {
		return matches[0], "name", nil
	}
	if len(matches) > 1 {
		return 0, "", fmt.Errorf("actor selector %q is ambiguous: %s", selector, formatLevelActorCandidates(asset, matches))
	}

	contains := make([]int, 0, 4)
	needle := strings.ToLower(normalized)
	for _, idx := range candidates {
		name := asset.Exports[idx].ObjectName.Display(asset.Names)
		if strings.Contains(strings.ToLower(name), needle) {
			contains = append(contains, idx)
		}
	}
	if len(contains) == 1 {
		return contains[0], "contains", nil
	}
	if len(contains) > 1 {
		return 0, "", fmt.Errorf("actor selector %q is ambiguous: %s", selector, formatLevelActorCandidates(asset, contains))
	}
	return 0, "", fmt.Errorf("actor %q not found under PersistentLevel (available: %s)", selector, formatLevelActorCandidates(asset, candidates))
}

func collectPersistentLevelChildren(asset *uasset.Asset) (int, []int, error) {
	if asset == nil {
		return 0, nil, fmt.Errorf("asset is nil")
	}
	persistentLevelIndex := -1
	for i, exp := range asset.Exports {
		if strings.EqualFold(exp.ObjectName.Display(asset.Names), "PersistentLevel") {
			persistentLevelIndex = i
			break
		}
	}
	if persistentLevelIndex < 0 {
		return 0, nil, fmt.Errorf("PersistentLevel export not found")
	}
	out := make([]int, 0, 32)
	for i := range asset.Exports {
		if i == persistentLevelIndex {
			continue
		}
		if isDirectChildOfPersistentLevel(asset, i, persistentLevelIndex) {
			out = append(out, i)
		}
	}
	if len(out) == 0 {
		return 0, nil, fmt.Errorf("no direct child exports under PersistentLevel")
	}
	return persistentLevelIndex, out, nil
}

func isDirectChildOfPersistentLevel(asset *uasset.Asset, exportIndex, persistentLevelIndex int) bool {
	if asset == nil || exportIndex < 0 || exportIndex >= len(asset.Exports) {
		return false
	}
	outer := asset.Exports[exportIndex].OuterIndex
	if outer <= 0 {
		return false
	}
	return outer.ResolveIndex() == persistentLevelIndex
}

func formatLevelActorCandidates(asset *uasset.Asset, indexes []int) string {
	if asset == nil || len(indexes) == 0 {
		return "(none)"
	}
	items := make([]string, 0, len(indexes))
	for _, idx := range indexes {
		if idx < 0 || idx >= len(asset.Exports) {
			continue
		}
		exp := asset.Exports[idx]
		items = append(items, fmt.Sprintf("%d:%s(%s)", idx+1, exp.ObjectName.Display(asset.Names), asset.ResolveClassName(exp)))
	}
	if len(items) == 0 {
		return "(none)"
	}
	return strings.Join(items, ", ")
}

func levelActorInfo(asset *uasset.Asset, exportIndex int, matchedBy string) map[string]any {
	exp := asset.Exports[exportIndex]
	return map[string]any{
		"matchedBy":  matchedBy,
		"export":     exportIndex + 1,
		"objectName": exp.ObjectName.Display(asset.Names),
		"objectPath": "PersistentLevel." + exp.ObjectName.Display(asset.Names),
		"className":  asset.ResolveClassName(exp),
		"outer": map[string]any{
			"index":    int32(exp.OuterIndex),
			"resolved": asset.ParseIndex(exp.OuterIndex),
		},
	}
}

func findCDOExportIndex(asset *uasset.Asset) (int, error) {
	if asset == nil || len(asset.Exports) == 0 {
		return 0, fmt.Errorf("asset has no exports")
	}
	for i, exp := range asset.Exports {
		if strings.HasPrefix(exp.ObjectName.Display(asset.Names), "Default__") {
			return i, nil
		}
	}
	if len(asset.Exports) == 1 {
		return 0, nil
	}
	return 0, fmt.Errorf("CDO export (Default__*) not found")
}

func collectDeclaredVariables(asset *uasset.Asset) (map[string]string, []string) {
	out := map[string]string{}
	warnings := make([]string, 0, 4)
	if asset == nil {
		return out, warnings
	}
	for i, exp := range asset.Exports {
		className := strings.ToLower(asset.ResolveClassName(exp))
		if !strings.Contains(className, "blueprint") {
			continue
		}
		props := asset.ParseExportProperties(i)
		if len(props.Warnings) > 0 {
			warnings = append(warnings, fmt.Sprintf("blueprint export %d: %s", i+1, strings.Join(props.Warnings, "; ")))
			continue
		}
		for _, p := range props.Properties {
			if p.Name.Display(asset.Names) != "NewVariables" {
				continue
			}
			decoded, ok := asset.DecodePropertyValue(p)
			if ok {
				declared := collectDeclaredVariablesFromDecoded(decoded)
				if len(declared) > 0 {
					for name, typeName := range declared {
						out[name] = typeName
					}
					continue
				}
			}

			declared, err := collectDeclaredVariablesFromRaw(asset, p)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("blueprint export %d NewVariables fallback parse failed: %v", i+1, err))
				continue
			}
			for name, typeName := range declared {
				out[name] = typeName
			}
			if len(declared) == 0 {
				warnings = append(warnings, fmt.Sprintf("blueprint export %d NewVariables had no extractable declarations", i+1))
			}
		}
	}
	return out, warnings
}

func collectDeclaredVariablesFromDecoded(decoded any) map[string]string {
	out := map[string]string{}
	m, ok := decoded.(map[string]any)
	if !ok {
		return out
	}
	items, ok := m["value"]
	if !ok {
		return out
	}
	arr, ok := items.([]any)
	if !ok {
		if wrapped, ok := items.([]map[string]any); ok {
			arr = make([]any, 0, len(wrapped))
			for _, item := range wrapped {
				arr = append(arr, item)
			}
		} else {
			return out
		}
	}
	for _, item := range arr {
		wrapper, ok := item.(map[string]any)
		if !ok {
			continue
		}
		inner, ok := wrapper["value"].(map[string]any)
		if !ok {
			continue
		}
		fields, ok := inner["value"].(map[string]any)
		if !ok {
			continue
		}
		name := ""
		typeName := ""
		if varNameRaw, ok := fields["VarName"].(map[string]any); ok {
			if nameValue, ok := varNameRaw["value"].(map[string]any); ok {
				if s, ok := nameValue["name"].(string); ok {
					name = s
				}
			}
		}
		if varTypeRaw, ok := fields["VarType"].(map[string]any); ok {
			if t, ok := varTypeRaw["type"].(string); ok {
				typeName = t
			}
		}
		if name != "" {
			out[name] = typeName
		}
	}
	return out
}

func collectDeclaredVariablesFromRaw(asset *uasset.Asset, tag uasset.PropertyTag) (map[string]string, error) {
	out := map[string]string{}
	if tag.Size < 4 {
		return out, fmt.Errorf("tag size too small: %d", tag.Size)
	}
	start := tag.ValueOffset
	end := tag.ValueOffset + int(tag.Size)
	if start < 0 || end < start || end > len(asset.Raw.Bytes) {
		return out, fmt.Errorf("tag value range out of bounds (%d..%d)", start, end)
	}
	r := uasset.NewByteReaderWithByteSwapping(asset.Raw.Bytes[start:end], asset.Summary.UsesByteSwappedSerialization())
	count, err := r.ReadInt32()
	if err != nil {
		return out, fmt.Errorf("read array count: %w", err)
	}
	if count < 0 || count > 100000 {
		return out, fmt.Errorf("invalid array count: %d", count)
	}
	for i := int32(0); i < count; i++ {
		itemStart := start + r.Offset()
		if itemStart >= end {
			return out, fmt.Errorf("array element %d starts out of range", i)
		}
		parsed := asset.ParseTaggedPropertiesRange(itemStart, end, false)
		if len(parsed.Warnings) > 0 {
			return out, fmt.Errorf("array element %d parse warnings: %s", i, strings.Join(parsed.Warnings, "; "))
		}
		if parsed.EndOffset <= itemStart {
			return out, fmt.Errorf("array element %d parser made no progress", i)
		}
		name := extractVariableNameFromTaggedStruct(asset, parsed.Properties)
		if name != "" {
			out[name] = ""
		}
		if err := r.Seek(parsed.EndOffset - start); err != nil {
			return out, fmt.Errorf("seek array element %d end: %w", i, err)
		}
	}
	return out, nil
}

func extractVariableNameFromTaggedStruct(asset *uasset.Asset, props []uasset.PropertyTag) string {
	for _, p := range props {
		if p.Name.Display(asset.Names) != "VarName" {
			continue
		}
		v, ok := asset.DecodePropertyValue(p)
		if !ok {
			continue
		}
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := m["name"].(string); ok && name != "" {
			return name
		}
	}
	return ""
}

func createBackupFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read source file: %w", err)
	}
	backupPath := path + ".backup"
	if err := os.WriteFile(backupPath, data, 0o644); err != nil {
		return fmt.Errorf("write backup file: %w", err)
	}
	return nil
}

func createBackupIfExists(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.WriteFile(path+".backup", data, 0o644); err != nil {
		return err
	}
	return nil
}
