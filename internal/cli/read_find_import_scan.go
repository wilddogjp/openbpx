package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

func runFind(args []string, stdout, stderr io.Writer) int {
	return dispatchSubcommand(
		args,
		stdout,
		stderr,
		"usage: bpx find <assets|summary> ...",
		"unknown find command: %s\n",
		subcommandSpec{Name: "assets", Run: runFindAssets},
		subcommandSpec{Name: "summary", Run: runFindSummary},
	)
}

func runFindAssets(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("find assets", stderr)
	pattern := fs.String("pattern", "*.uasset", "glob pattern")
	recursive := fs.Bool("recursive", true, "scan recursively")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: bpx find assets <directory> [--pattern \"*.uasset\"] [--recursive]")
		return 1
	}
	root := fs.Arg(0)

	assets, err := uasset.CollectAssetFiles(root, *pattern, *recursive)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	resp := map[string]any{
		"directory": root,
		"pattern":   *pattern,
		"recursive": *recursive,
		"count":     len(assets),
		"assets":    assets,
	}
	return printJSON(stdout, resp)
}

func runImport(args []string, stdout, stderr io.Writer) int {
	return dispatchSubcommand(
		args,
		stdout,
		stderr,
		"usage: bpx import <list|search|graph> ...",
		"unknown import command: %s\n",
		subcommandSpec{Name: "list", Run: runImportList},
		subcommandSpec{Name: "search", Run: runImportSearch},
		subcommandSpec{Name: "graph", Run: runImportGraph},
	)
}

func runImportList(args []string, stdout, stderr io.Writer) int {
	file, asset, ok := parseSingleAssetCommand(args, "import list", "usage: bpx import list <file.uasset>", stderr)
	if !ok {
		return 1
	}

	items := make([]map[string]any, 0, len(asset.Imports))
	for i, imp := range asset.Imports {
		items = append(items, map[string]any{
			"index":          i + 1,
			"classPackage":   imp.ClassPackage.Display(asset.Names),
			"className":      imp.ClassName.Display(asset.Names),
			"outerIndex":     int32(imp.OuterIndex),
			"outerResolved":  asset.ParseIndex(imp.OuterIndex),
			"objectName":     imp.ObjectName.Display(asset.Names),
			"packageName":    imp.PackageName.Display(asset.Names),
			"importOptional": imp.ImportOptional,
		})
	}

	return printJSON(stdout, map[string]any{
		"file":    file,
		"imports": items,
	})
}

func runImportSearch(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("import search", stderr)
	opts := registerCommonFlags(fs)
	object := fs.String("object", "", "object name search token")
	classPackage := fs.String("class-package", "", "class package search token")
	className := fs.String("class-name", "", "class name search token")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || (*object == "" && *classPackage == "" && *className == "") {
		fmt.Fprintln(stderr, "usage: bpx import search <file.uasset> [--object <name>] [--class-package <pkg>] [--class-name <cls>]")
		return 1
	}
	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	matches := make([]map[string]any, 0, 16)
	for i, imp := range asset.Imports {
		obj := imp.ObjectName.Display(asset.Names)
		cp := imp.ClassPackage.Display(asset.Names)
		cn := imp.ClassName.Display(asset.Names)
		if *object != "" && !containsFold(obj, *object) {
			continue
		}
		if *classPackage != "" && !containsFold(cp, *classPackage) {
			continue
		}
		if *className != "" && !containsFold(cn, *className) {
			continue
		}
		matches = append(matches, map[string]any{
			"index":          i + 1,
			"classPackage":   cp,
			"className":      cn,
			"outerIndex":     int32(imp.OuterIndex),
			"outerResolved":  asset.ParseIndex(imp.OuterIndex),
			"objectName":     obj,
			"packageName":    imp.PackageName.Display(asset.Names),
			"importOptional": imp.ImportOptional,
		})
	}

	return printJSON(stdout, map[string]any{
		"file":        file,
		"query":       map[string]any{"object": *object, "classPackage": *classPackage, "className": *className},
		"matchCount":  len(matches),
		"matchImport": matches,
	})
}

func runImportGraph(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("import graph", stderr)
	opts := registerCommonFlags(fs)
	pattern := fs.String("pattern", "*.uasset", "glob pattern")
	recursive := fs.Bool("recursive", true, "scan recursively")
	groupBy := fs.String("group-by", "root", "grouping target: root or object")
	filter := fs.String("filter", "", "optional substring filter for import target")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: bpx import graph <directory> [--pattern \"*.uasset\"] [--recursive] [--group-by root|object] [--filter <token>]")
		return 1
	}
	if *groupBy != "root" && *groupBy != "object" {
		fmt.Fprintln(stderr, "error: --group-by must be root or object")
		return 1
	}

	rootDir := fs.Arg(0)
	files, err := uasset.CollectAssetFiles(rootDir, *pattern, *recursive)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	rootCounts := map[string]int{}
	objectCounts := map[string]int{}
	edgeCounts := map[string]int{}
	parseFailures := make([]map[string]string, 0, 8)

	for _, file := range files {
		asset, err := uasset.ParseFile(file, *opts)
		if err != nil {
			parseFailures = append(parseFailures, map[string]string{"file": file, "error": err.Error()})
			continue
		}
		from := asset.Summary.PackageName
		if from == "" {
			from = file
		}
		for _, imp := range asset.Imports {
			packageName := resolveImportTargetPath(asset, imp)
			objectName := imp.ObjectName.Display(asset.Names)
			targetObject := packageName
			if objectName != "" && objectName != "None" {
				if targetObject == "" || targetObject == "None" {
					targetObject = objectName
				} else {
					targetObject = targetObject + ":" + objectName
				}
			}
			targetRoot := packageRoot(packageName)
			if *filter != "" && !containsFold(targetRoot, *filter) && !containsFold(targetObject, *filter) {
				continue
			}

			rootCounts[targetRoot]++
			objectCounts[targetObject]++

			to := targetRoot
			if *groupBy == "object" {
				to = targetObject
			}
			edgeCounts[from+"->"+to]++
		}
	}

	type edge struct {
		From  string `json:"from"`
		To    string `json:"to"`
		Count int    `json:"count"`
	}
	edges := make([]edge, 0, len(edgeCounts))
	for k, count := range edgeCounts {
		parts := strings.SplitN(k, "->", 2)
		if len(parts) != 2 {
			continue
		}
		edges = append(edges, edge{From: parts[0], To: parts[1], Count: count})
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Count != edges[j].Count {
			return edges[i].Count > edges[j].Count
		}
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})

	return printJSON(stdout, map[string]any{
		"directory":      rootDir,
		"pattern":        *pattern,
		"recursive":      *recursive,
		"groupBy":        *groupBy,
		"filter":         *filter,
		"fileCount":      len(files),
		"parseFailCount": len(parseFailures),
		"parseFailures":  parseFailures,
		"rootCounts":     rootCounts,
		"objectCounts":   objectCounts,
		"edges":          edges,
	})
}
func runFindSummary(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("find summary", stderr)
	opts := registerCommonFlags(fs)
	pattern := fs.String("pattern", "*.uasset", "glob pattern")
	recursive := fs.Bool("recursive", true, "scan recursively")
	allowOutputFormats(fs, "output format: json or toml", structuredOutputFormatJSON, structuredOutputFormatTOML)
	outPath := fs.String("out", "", "write output to path instead of stdout")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: bpx find summary <directory> [--pattern \"*.uasset\"] [--recursive] [--format json|toml] [--out <path>]")
		return 1
	}
	format := outputFormatFromFlagSet(fs)
	rootDir := fs.Arg(0)
	files, err := uasset.CollectAssetFiles(rootDir, *pattern, *recursive)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	assetKindCounts := map[string]int{}
	topDirCounts := map[string]int{}
	datatableFiles := make([]string, 0, 64)
	blueprintFiles := make([]string, 0, 64)
	parseFailures := make([]map[string]string, 0, 8)

	for _, file := range files {
		asset, err := uasset.ParseFile(file, *opts)
		if err != nil {
			parseFailures = append(parseFailures, map[string]string{"file": file, "error": err.Error()})
			continue
		}
		kind := asset.GuessAssetKind()
		assetKindCounts[kind]++
		rel, relErr := filepath.Rel(rootDir, file)
		dirKey := filepath.Dir(file)
		if relErr == nil {
			dirKey = topDirFromRelative(rel)
		}
		topDirCounts[dirKey]++
		if assetHasAnyClass(asset, []string{"datatable", "curvetable", "compositedatatable"}) {
			datatableFiles = append(datatableFiles, file)
		}
		if assetHasAnyClass(asset, []string{"blueprint"}) {
			blueprintFiles = append(blueprintFiles, file)
		}
	}
	sort.Strings(datatableFiles)
	sort.Strings(blueprintFiles)

	resp := map[string]any{
		"directory":       rootDir,
		"pattern":         *pattern,
		"recursive":       *recursive,
		"fileCount":       len(files),
		"parsedCount":     len(files) - len(parseFailures),
		"parseFailCount":  len(parseFailures),
		"parseFailures":   parseFailures,
		"assetKindCounts": assetKindCounts,
		"topDirCounts":    topDirCounts,
		"datatableFiles":  datatableFiles,
		"blueprintFiles":  blueprintFiles,
	}

	if *outPath == "" {
		return printJSON(stdout, resp)
	}
	body, err := marshalStructuredPayload(resp, format)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := writeFileAtomically(*outPath, body, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write output: %v\n", err)
		return 1
	}
	return printJSON(stdout, map[string]any{
		"directory": rootDir,
		"format":    format,
		"out":       *outPath,
	})
}
func topDirFromRelative(rel string) string {
	clean := filepath.ToSlash(filepath.Clean(rel))
	if clean == "." || clean == "" {
		return "."
	}
	parts := strings.Split(clean, "/")
	if len(parts) <= 1 {
		return "."
	}
	if parts[0] == "." && len(parts) > 2 {
		return parts[1]
	}
	if parts[0] == "" {
		return "/"
	}
	return parts[0]
}

func packageRoot(packageName string) string {
	name := strings.TrimSpace(packageName)
	if name == "" || strings.EqualFold(name, "none") {
		return "<none>"
	}
	if idx := strings.IndexByte(name, ':'); idx >= 0 {
		name = name[:idx]
	}
	if idx := strings.IndexByte(name, '.'); idx > 0 {
		name = name[:idx]
	}
	if !strings.HasPrefix(name, "/") {
		return name
	}
	parts := strings.Split(strings.Trim(name, "/"), "/")
	if len(parts) == 0 {
		return "/"
	}
	if len(parts) == 1 {
		return "/" + parts[0]
	}
	return "/" + parts[0] + "/" + parts[1]
}

func resolveImportTargetPath(asset *uasset.Asset, imp uasset.ImportEntry) string {
	packageName := imp.PackageName.Display(asset.Names)
	if strings.HasPrefix(packageName, "/") {
		return packageName
	}
	outerResolved := asset.ParseIndex(imp.OuterIndex)
	if outerPath := extractResolvedObjectPath(outerResolved); strings.HasPrefix(outerPath, "/") {
		return outerPath
	}
	objectName := imp.ObjectName.Display(asset.Names)
	if strings.HasPrefix(objectName, "/") {
		return objectName
	}
	return packageName
}

func extractResolvedObjectPath(resolved string) string {
	parts := strings.SplitN(resolved, ":", 3)
	if len(parts) != 3 {
		return ""
	}
	return parts[2]
}

func assetHasAnyClass(asset *uasset.Asset, classTokens []string) bool {
	if len(classTokens) == 0 {
		return false
	}
	tokens := make([]string, 0, len(classTokens))
	for _, token := range classTokens {
		token = strings.ToLower(strings.TrimSpace(token))
		if token != "" {
			tokens = append(tokens, token)
		}
	}
	if len(tokens) == 0 {
		return false
	}
	for _, exp := range asset.Exports {
		className := strings.ToLower(asset.ResolveClassName(exp))
		for _, token := range tokens {
			if strings.Contains(className, token) {
				return true
			}
		}
	}
	return false
}

func containsFold(base, token string) bool {
	return strings.Contains(strings.ToLower(base), strings.ToLower(token))
}
