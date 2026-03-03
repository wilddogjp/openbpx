package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/wilddogjp/openbpx/pkg/uasset"
	"github.com/wilddogjp/openbpx/pkg/validate"
	"gopkg.in/yaml.v3"
)

const (
	structuredOutputFormatJSON = "json"
	structuredOutputFormatTOML = "toml"
	defaultToolVersion         = "0.1.5"
)

var currentStructuredOutputFormat = structuredOutputFormatJSON
var toolVersion = defaultToolVersion

type outputFormatFlag struct {
	value   string
	allowed map[string]struct{}
}

func newOutputFormatFlag(defaultValue string, allowed ...string) *outputFormatFlag {
	f := &outputFormatFlag{
		value:   strings.ToLower(strings.TrimSpace(defaultValue)),
		allowed: make(map[string]struct{}, len(allowed)),
	}
	for _, candidate := range allowed {
		normalized := strings.ToLower(strings.TrimSpace(candidate))
		if normalized == "" {
			continue
		}
		f.allowed[normalized] = struct{}{}
	}
	if f.value == "" {
		f.value = structuredOutputFormatJSON
	}
	if _, ok := f.allowed[f.value]; !ok {
		f.allowed[f.value] = struct{}{}
	}
	return f
}

func (f *outputFormatFlag) String() string {
	return f.value
}

func (f *outputFormatFlag) Set(raw string) error {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return fmt.Errorf("format must not be empty")
	}
	if _, ok := f.allowed[normalized]; !ok {
		allowed := make([]string, 0, len(f.allowed))
		for candidate := range f.allowed {
			allowed = append(allowed, candidate)
		}
		sort.Strings(allowed)
		return fmt.Errorf("unsupported format: %s (allowed: %s)", normalized, strings.Join(allowed, ", "))
	}
	f.value = normalized
	return nil
}

func (f *outputFormatFlag) allow(values ...string) {
	for _, candidate := range values {
		normalized := strings.ToLower(strings.TrimSpace(candidate))
		if normalized == "" {
			continue
		}
		f.allowed[normalized] = struct{}{}
	}
}

func setStructuredOutputFormat(raw string) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case structuredOutputFormatTOML:
		currentStructuredOutputFormat = structuredOutputFormatTOML
	default:
		currentStructuredOutputFormat = structuredOutputFormatJSON
	}
}

func formatFlagFor(fs *flag.FlagSet) *outputFormatFlag {
	if fs == nil {
		return nil
	}
	f := fs.Lookup("format")
	if f == nil {
		return nil
	}
	v, ok := f.Value.(*outputFormatFlag)
	if !ok {
		return nil
	}
	return v
}

func outputFormatFromFlagSet(fs *flag.FlagSet) string {
	formatFlag := formatFlagFor(fs)
	if formatFlag == nil {
		return structuredOutputFormatJSON
	}
	return formatFlag.value
}

func allowOutputFormats(fs *flag.FlagSet, usage string, formats ...string) {
	formatFlag := formatFlagFor(fs)
	if formatFlag == nil {
		return
	}
	formatFlag.allow(formats...)
	if usage != "" {
		if f := fs.Lookup("format"); f != nil {
			f.Usage = usage
		}
	}
}

// Run executes bpx CLI.
func Run(args []string, stdout, stderr io.Writer) int {
	prevFormat := currentStructuredOutputFormat
	setStructuredOutputFormat(structuredOutputFormatJSON)
	defer setStructuredOutputFormat(prevFormat)

	if len(args) == 0 {
		printRootUsage(stderr)
		return 1
	}
	if len(args) == 2 && isHelpToken(args[1]) {
		return runHelp(args[:1], stdout, stderr)
	}

	switch args[0] {
	case "version", "--version", "-v":
		return runVersion(args[1:], stdout, stderr)
	case "find":
		return runFind(args[1:], stdout, stderr)
	case "info":
		return runInfo(args[1:], stdout, stderr)
	case "dump":
		return runDump(args[1:], stdout, stderr)
	case "export":
		return runExport(args[1:], stdout, stderr)
	case "import":
		return runImport(args[1:], stdout, stderr)
	case "prop":
		return runProp(args[1:], stdout, stderr)
	case "write":
		return runWrite(args[1:], stdout, stderr)
	case "var":
		return runVar(args[1:], stdout, stderr)
	case "ref":
		return runRef(args[1:], stdout, stderr)
	case "package":
		return runPackage(args[1:], stdout, stderr)
	case "localization":
		return runLocalization(args[1:], stdout, stderr)
	case "datatable":
		return runDataTable(args[1:], stdout, stderr)
	case "blueprint":
		return runBlueprint(args[1:], stdout, stderr)
	case "enum":
		return runEnum(args[1:], stdout, stderr)
	case "name":
		return runName(args[1:], stdout, stderr)
	case "struct":
		return runStruct(args[1:], stdout, stderr)
	case "stringtable":
		return runStringTable(args[1:], stdout, stderr)
	case "class":
		return runClass(args[1:], stdout, stderr)
	case "level":
		return runLevel(args[1:], stdout, stderr)
	case "raw":
		return runRaw(args[1:], stdout, stderr)
	case "metadata":
		return runMetadata(args[1:], stdout, stderr)
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		return runHelp(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		printRootUsage(stderr)
		return 1
	}
}

func runHelp(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printRootUsage(stdout)
		return 0
	}
	if isHelpToken(args[0]) {
		printRootUsage(stdout)
		return 0
	}
	topic := strings.TrimSpace(strings.ToLower(args[0]))
	if topic == "" {
		printRootUsage(stdout)
		return 0
	}
	if !printTopicUsage(stdout, topic) {
		fmt.Fprintf(stderr, "unknown help topic: %s\n\n", args[0])
		printRootUsage(stderr)
		return 1
	}
	return 0
}

func runVersion(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: bpx version")
		return 1
	}
	version := strings.TrimSpace(toolVersion)
	if version == "" {
		version = defaultToolVersion
	}
	fmt.Fprintln(stdout, version)
	return 0
}

func isHelpToken(token string) bool {
	switch token {
	case "help", "-h", "--help":
		return true
	default:
		return false
	}
}

func runInfo(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("info", stderr)
	opts := registerCommonFlags(fs)
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: bpx info <file.uasset>")
		return 1
	}
	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	resp := map[string]any{
		"file":               file,
		"engineVersion":      asset.Summary.SavedByEngineVersion,
		"fileVersionUE4":     asset.Summary.FileVersionUE4,
		"fileVersionUE5":     asset.Summary.FileVersionUE5,
		"packageName":        asset.Summary.PackageName,
		"packageFlags":       asset.Summary.PackageFlags,
		"nameCount":          asset.Summary.NameCount,
		"importCount":        asset.Summary.ImportCount,
		"exportCount":        asset.Summary.ExportCount,
		"totalHeaderSize":    asset.Summary.TotalHeaderSize,
		"summarySize":        asset.Summary.SummarySize,
		"customVersionCount": len(asset.Summary.CustomVersions),
		"assetKind":          asset.GuessAssetKind(),
	}
	return printJSON(stdout, resp)
}

func runExport(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: bpx export <list|info|set-header> ...")
		return 1
	}
	switch args[0] {
	case "list":
		return runExportList(args[1:], stdout, stderr)
	case "info":
		return runExportInfo(args[1:], stdout, stderr)
	case "set-header":
		return runExportSetHeader(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown export command: %s\n", args[0])
		return 1
	}
}

func runExportList(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("export list", stderr)
	opts := registerCommonFlags(fs)
	classFilter := fs.String("class", "", "optional class-name filter token")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: bpx export list <file.uasset> [--class <token>]")
		return 1
	}
	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	type item struct {
		Index        int    `json:"index"`
		ObjectName   string `json:"objectName"`
		ClassName    string `json:"className"`
		ClassIndex   int32  `json:"classIndex"`
		SuperIndex   int32  `json:"superIndex"`
		OuterIndex   int32  `json:"outerIndex"`
		SerialSize   int64  `json:"serialSize"`
		SerialOffset int64  `json:"serialOffset"`
	}

	items := make([]item, 0, len(asset.Exports))
	for i, exp := range asset.Exports {
		className := asset.ResolveClassName(exp)
		if *classFilter != "" && !containsFold(className, *classFilter) {
			continue
		}
		items = append(items, item{
			Index:        i + 1,
			ObjectName:   exp.ObjectName.Display(asset.Names),
			ClassName:    className,
			ClassIndex:   int32(exp.ClassIndex),
			SuperIndex:   int32(exp.SuperIndex),
			OuterIndex:   int32(exp.OuterIndex),
			SerialSize:   exp.SerialSize,
			SerialOffset: exp.SerialOffset,
		})
	}
	resp := map[string]any{
		"file":        file,
		"classFilter": *classFilter,
		"count":       len(items),
		"exports":     items,
	}
	return printJSON(stdout, resp)
}

func runExportInfo(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("export info", stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("export", 0, "1-based export index")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || *exportIndex <= 0 {
		fmt.Fprintln(stderr, "usage: bpx export info <file.uasset> --export <n>")
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
	resp := map[string]any{
		"file":       file,
		"index":      *exportIndex,
		"objectName": exp.ObjectName.Display(asset.Names),
		"class": map[string]any{
			"index":    int32(exp.ClassIndex),
			"resolved": asset.ParseIndex(exp.ClassIndex),
			"name":     asset.ResolveClassName(exp),
		},
		"super": map[string]any{
			"index":    int32(exp.SuperIndex),
			"resolved": asset.ParseIndex(exp.SuperIndex),
		},
		"template": map[string]any{
			"index":    int32(exp.TemplateIndex),
			"resolved": asset.ParseIndex(exp.TemplateIndex),
		},
		"outer": map[string]any{
			"index":    int32(exp.OuterIndex),
			"resolved": asset.ParseIndex(exp.OuterIndex),
		},
		"objectFlags": exp.ObjectFlags,
		"serial": map[string]any{
			"size":                     exp.SerialSize,
			"offset":                   exp.SerialOffset,
			"scriptSerializationStart": exp.ScriptSerializationStartOffset,
			"scriptSerializationEnd":   exp.ScriptSerializationEndOffset,
		},
		"flags": map[string]any{
			"forcedExport":             exp.ForcedExport,
			"notForClient":             exp.NotForClient,
			"notForServer":             exp.NotForServer,
			"notAlwaysLoadedForEditor": exp.NotAlwaysLoadedForEditor,
			"isAsset":                  exp.IsAsset,
			"isInheritedInstance":      exp.IsInheritedInstance,
			"generatePublicHash":       exp.GeneratePublicHash,
		},
		"dependencyHeader": map[string]any{
			"firstExportDependency":                 exp.FirstExportDependency,
			"serializationBeforeSerializationDeps":  exp.SerializationBeforeSerializationDeps,
			"createBeforeSerializationDeps":         exp.CreateBeforeSerializationDeps,
			"serializationBeforeCreateDependencies": exp.SerializationBeforeCreateDependencies,
			"createBeforeCreateDependencies":        exp.CreateBeforeCreateDependencies,
		},
	}
	return printJSON(stdout, resp)
}

func runProp(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: bpx prop <list|set|add|remove> ...")
		return 1
	}
	switch args[0] {
	case "list":
		return runPropList(args[1:], stdout, stderr)
	case "set":
		return runPropSet(args[1:], stdout, stderr)
	case "add":
		return runPropAdd(args[1:], stdout, stderr)
	case "remove":
		return runPropRemove(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown prop command: %s\n", args[0])
		return 1
	}
}

func runPropList(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("prop list", stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("export", 0, "1-based export index")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || *exportIndex <= 0 {
		fmt.Fprintln(stderr, "usage: bpx prop list <file.uasset> --export <n>")
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

	parsed := asset.ParseExportProperties(idx)
	items := toPropertyOutputs(asset, parsed.Properties, true)
	resp := map[string]any{
		"file":       file,
		"export":     *exportIndex,
		"objectName": asset.Exports[idx].ObjectName.Display(asset.Names),
		"className":  asset.ResolveClassName(asset.Exports[idx]),
		"decoded":    true,
		"properties": items,
		"warnings":   parsed.Warnings,
	}
	return printJSON(stdout, resp)
}

func runPackage(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: bpx package <meta|custom-versions|depends|resolve-index|section|set-flags> ...")
		return 1
	}
	switch args[0] {
	case "meta":
		return runPackageMeta(args[1:], stdout, stderr)
	case "custom-versions":
		return runPackageCustomVersions(args[1:], stdout, stderr)
	case "depends":
		return runPackageDepends(args[1:], stdout, stderr)
	case "resolve-index":
		return runPackageResolveIndex(args[1:], stdout, stderr)
	case "section":
		return runPackageSection(args[1:], stdout, stderr)
	case "set-flags":
		return runPackageSetFlags(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown package command: %s\n", args[0])
		return 1
	}
}

func runPackageMeta(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("package meta", stderr)
	opts := registerCommonFlags(fs)
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: bpx package meta <file.uasset>")
		return 1
	}
	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	customVersions := make([]map[string]any, 0, len(asset.Summary.CustomVersions))
	for _, cv := range asset.Summary.CustomVersions {
		customVersions = append(customVersions, map[string]any{"guid": cv.Key.String(), "version": cv.Version})
	}
	sort.Slice(customVersions, func(i, j int) bool {
		return customVersions[i]["guid"].(string) < customVersions[j]["guid"].(string)
	})

	resp := map[string]any{
		"file":                file,
		"packageName":         asset.Summary.PackageName,
		"packageGuid":         asset.Summary.PersistentGUID.String(),
		"persistentGuid":      asset.Summary.PersistentGUID.String(),
		"savedHash":           asset.Summary.SavedHashHex(),
		"packageFlags":        fmt.Sprintf("0x%08x", asset.Summary.PackageFlags),
		"packageFlagsRaw":     asset.Summary.PackageFlags,
		"isUnversioned":       asset.Summary.Unversioned,
		"savedByEngine":       asset.Summary.SavedByEngineVersion,
		"compatibleEngine":    asset.Summary.CompatibleEngineVersion,
		"fileVersionUE4":      asset.Summary.FileVersionUE4,
		"fileVersionUE5":      asset.Summary.FileVersionUE5,
		"fileVersionLicensee": asset.Summary.FileVersionLicenseeUE4,
		"name": map[string]any{
			"count":  asset.Summary.NameCount,
			"offset": asset.Summary.NameOffset,
		},
		"imports": map[string]any{
			"count":  asset.Summary.ImportCount,
			"offset": asset.Summary.ImportOffset,
		},
		"exports": map[string]any{
			"count":  asset.Summary.ExportCount,
			"offset": asset.Summary.ExportOffset,
		},
		"dependsOffset":           asset.Summary.DependsOffset,
		"thumbnailTableOffset":    asset.Summary.ThumbnailTableOffset,
		"assetRegistryDataOffset": asset.Summary.AssetRegistryDataOffset,
		"bulkDataStartOffset":     asset.Summary.BulkDataStartOffset,
		"preloadDependencyCount":  asset.Summary.PreloadDependencyCount,
		"preloadDependencyOffset": asset.Summary.PreloadDependencyOffset,
		"dataResourceOffset":      asset.Summary.DataResourceOffset,
		"generations":             asset.Summary.Generations,
		"customVersions":          customVersions,
	}
	return printJSON(stdout, resp)
}

func runDump(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("dump", stderr)
	opts := registerCommonFlags(fs)
	allowOutputFormats(fs, "output format: json, toml, or yaml", structuredOutputFormatJSON, structuredOutputFormatTOML, "yaml", "yml")
	outPath := fs.String("out", "", "write output to path instead of stdout")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: bpx dump <file.uasset> [--format json|toml|yaml] [--out path]")
		return 1
	}

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

	payload := map[string]any{
		"file":    file,
		"summary": asset.Summary,
		"names":   asset.Names,
		"imports": asset.Imports,
		"exports": asset.Exports,
	}
	body, normalizedFormat, err := marshalDumpPayload(payload, outputFormatFromFlagSet(fs))
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if *outPath != "" {
		if err := writeFileAtomically(*outPath, body, 0o644); err != nil {
			fmt.Fprintf(stderr, "error: write dump output: %v\n", err)
			return 1
		}
		return printJSON(stdout, map[string]any{
			"file":   file,
			"format": normalizedFormat,
			"out":    *outPath,
		})
	}

	if _, err := stdout.Write(body); err != nil {
		fmt.Fprintf(stderr, "error: write dump output: %v\n", err)
		return 1
	}
	return 0
}

func runValidate(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("validate", stderr)
	opts := registerCommonFlags(fs)
	binaryEquality := fs.Bool("binary-equality", false, "validate no-op byte equality and reparse consistency")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: bpx validate <file.uasset> [--binary-equality]")
		return 1
	}
	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	report := validate.Run(asset, *binaryEquality)
	resp := map[string]any{"file": file, "result": report}
	code := printJSON(stdout, resp)
	if code != 0 {
		return code
	}
	if !report.OK {
		return 2
	}
	return 0
}

func registerCommonFlags(fs *flag.FlagSet) *uasset.ParseOptions {
	opts := uasset.DefaultParseOptions()
	return &opts
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Var(newOutputFormatFlag(structuredOutputFormatJSON, structuredOutputFormatJSON, structuredOutputFormatTOML), "format", "output format: json or toml")
	return fs
}

func parseFlagSet(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(normalizeFlagArgs(fs, args)); err != nil {
		return err
	}
	setStructuredOutputFormat(outputFormatFromFlagSet(fs))
	return nil
}

func normalizeFlagArgs(fs *flag.FlagSet, args []string) []string {
	flags := make([]string, 0, len(args))
	positionals := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		token := args[i]
		if token == "--" {
			positionals = append(positionals, args[i:]...)
			break
		}

		name, inlineValue, isFlagToken := splitFlagToken(token)
		if !isFlagToken {
			positionals = append(positionals, token)
			continue
		}

		if fs.Lookup(name) == nil {
			// Keep unknown flag-like tokens in the flag stream so flag.Parse can report them.
			flags = append(flags, token)
			continue
		}

		flags = append(flags, token)
		if inlineValue {
			continue
		}

		if isBoolFlag(fs, name) {
			if i+1 < len(args) && isBoolLiteral(args[i+1]) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}

		if i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}

	return append(flags, positionals...)
}

func splitFlagToken(token string) (name string, inlineValue bool, isFlag bool) {
	if len(token) < 2 || !strings.HasPrefix(token, "-") {
		return "", false, false
	}
	if token == "--" || token == "-" {
		return "", false, false
	}
	trimmed := strings.TrimLeft(token, "-")
	if trimmed == "" {
		return "", false, false
	}
	if eq := strings.IndexByte(trimmed, '='); eq >= 0 {
		if eq == 0 {
			return "", false, false
		}
		return trimmed[:eq], true, true
	}
	return trimmed, false, true
}

func isBoolFlag(fs *flag.FlagSet, name string) bool {
	f := fs.Lookup(name)
	if f == nil {
		return false
	}
	type boolFlag interface {
		IsBoolFlag() bool
	}
	v, ok := f.Value.(boolFlag)
	return ok && v.IsBoolFlag()
}

func isBoolLiteral(v string) bool {
	switch strings.ToLower(v) {
	case "true", "false", "1", "0":
		return true
	default:
		return false
	}
}

func printJSON(w io.Writer, payload any) int {
	body, err := marshalStructuredPayload(payload, currentStructuredOutputFormat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: encode %s output: %v\n", currentStructuredOutputFormat, err)
		return 1
	}
	if _, err := w.Write(body); err != nil {
		fmt.Fprintf(os.Stderr, "error: encode %s output: %v\n", currentStructuredOutputFormat, err)
		return 1
	}
	return 0
}

type helpCategory struct {
	Title string
	Lines []string
}

func printRootUsage(w io.Writer) {
	fmt.Fprintln(w, "BPX (bpx) - Blueprint Toolkit for Unreal Engine")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  bpx <command> [subcommand] [args]")
	fmt.Fprintln(w, "  bpx version")
	fmt.Fprintln(w, "  bpx help [command]")
	fmt.Fprintln(w, "  bpx <command> help")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Parse behavior:")
	fmt.Fprintln(w, "  Unknown-byte preservation is always enabled (fixed).")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Common output flags:")
	fmt.Fprintln(w, "  --format json|toml (default json): structured output format")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Write safety flags (when supported):")
	fmt.Fprintln(w, "  --dry-run: preview changes without writing files")
	fmt.Fprintln(w, "  --backup: create <target>.backup before overwrite")

	for _, category := range helpCatalog() {
		fmt.Fprintln(w, "")
		fmt.Fprintf(w, "%s:\n", category.Title)
		for _, line := range category.Lines {
			fmt.Fprintf(w, "  %s\n", line)
		}
	}
}

func printTopicUsage(w io.Writer, topic string) bool {
	lines := usageLinesForTopic(topic)
	if len(lines) == 0 {
		return false
	}

	fmt.Fprintf(w, "BPX help: %s\n", topic)
	if summary := helpTopicSummary(topic); summary != "" {
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, summary)
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	for _, line := range lines {
		fmt.Fprintf(w, "  %s\n", line)
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Parse behavior:")
	fmt.Fprintln(w, "  Unknown-byte preservation is always enabled (fixed).")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Common output flags:")
	fmt.Fprintln(w, "  --format json|toml (default json)")
	if topicHasWriteCommands(topic) {
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "Write safety flags:")
		fmt.Fprintln(w, "  --dry-run --backup")
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Run `bpx help` for all commands.")
	return true
}

func helpCatalog() []helpCategory {
	return []helpCategory{
		{
			Title: "Read/Analysis Commands",
			Lines: []string{
				"bpx version",
				"bpx find assets <directory> [--pattern \"*.uasset\"] [--recursive]",
				"bpx find summary <directory> [--pattern \"*.uasset\"] [--recursive] [--format json|toml] [--out <path>]",
				"bpx info <file.uasset>",
				"bpx dump <file.uasset> [--format json|toml|yaml] [--out path]",
				"bpx export list <file.uasset> [--class <token>]",
				"bpx export info <file.uasset> --export <n>",
				"bpx import list <file.uasset>",
				"bpx import search <file.uasset> [--object <name>] [--class-package <pkg>] [--class-name <cls>]",
				"bpx import graph <directory> [--pattern \"*.uasset\"] [--recursive] [--group-by root|object] [--filter <token>]",
				"bpx prop list <file.uasset> --export <n>",
				"bpx var list <file.uasset>",
				"bpx name list <file.uasset>",
				"bpx package meta <file.uasset>",
				"bpx package custom-versions <file.uasset>",
				"bpx package depends <file.uasset>",
				"bpx package resolve-index <file.uasset> --index <i>",
				"bpx package section <file.uasset> --name <section>",
				"bpx localization read <file.uasset> [--export <n>] [--include-history] [--format json|toml|csv]",
				"bpx localization query <file.uasset> [--export <n>] [--namespace <ns>] [--key <key>] [--text <token>] [--history-type <type>] [--limit <n>]",
				"bpx localization resolve <file.uasset> [--export <n>] --culture <culture> [--locres <path>] [--missing-only]",
				"bpx datatable read <file.uasset> [--export <n>] [--row <name>] [--format json|toml|csv|tsv] [--out path]",
				"bpx blueprint info <file.uasset> [--export <n>]",
				"bpx blueprint bytecode <file.uasset> --export <n> [--range-source auto|export-map|ustruct-script|serial-full] [--strict-range] [--diagnostics]",
				"bpx blueprint disasm <file.uasset> --export <n> [--format json|toml|text] [--analysis] [--entrypoint <vm>] [--max-steps <n>] [--range-source auto|export-map|ustruct-script|serial-full] [--strict-range] [--diagnostics]",
				"bpx blueprint trace <file.uasset> --from <Node|Node.Pin> [--to-node <token>] [--to-function <token>] [--max-depth <n>]",
				"bpx blueprint call-args <file.uasset> --member <token> [--class <token>] [--all-pins] [--include-exec]",
				"bpx blueprint refs <file.uasset> --soft-path <path> [--class <token>] [--include-routes] [--max-routes <n>] [--max-depth <n>]",
				"bpx blueprint search <file.uasset> [--class <token>] [--member <token>] [--name <token>] [--show <fields>] [--limit <n>]",
				"bpx blueprint infer-pack <file.uasset> --export <n> [--entrypoint <vm>] [--max-steps <n>] [--out <dir>] [--range-source auto|export-map|ustruct-script|serial-full] [--strict-range] [--diagnostics]",
				"bpx blueprint scan-functions <directory> --recursive [--name-like <regex>] [--aggregate]",
				"bpx enum list <file.uasset>",
				"bpx struct definition <file.uasset>",
				"bpx struct details <file.uasset> --export <n>",
				"bpx stringtable read <file.uasset>",
				"bpx class <file.uasset> --export <n>",
				"bpx level info <file.umap> --export <n>",
				"bpx level var-list <file.umap> --actor <name|PersistentLevel.Name|export-index>",
				"bpx raw <file.uasset> --export <n>",
				"bpx metadata <file.uasset> --export <n>",
				"bpx validate <file.uasset> [--binary-equality]",
			},
		},
		{
			Title: "Write/Update Commands",
			Lines: []string{
				"bpx prop set <file.uasset> --export <n> --path <dot.path> --value '<json>' [--dry-run] [--backup]",
				"bpx prop add <file.uasset> --export <n> --spec '<json>' [--dry-run] [--backup]",
				"bpx prop remove <file.uasset> --export <n> --path <dot.path> [--dry-run] [--backup]",
				"bpx write <file.uasset> --out <new.uasset> [--dry-run] [--backup]",
				"bpx var set-default <file.uasset> --name <var> --value '<json>' [--dry-run] [--backup]",
				"bpx var rename <file.uasset> --from <old> --to <new> [--dry-run] [--backup]",
				"bpx ref rewrite <file.uasset> --from <old> --to <new> [--dry-run] [--backup]",
				"bpx name add <file.uasset> --value <name> [--non-case-hash <u16>] [--case-preserving-hash <u16>] [--dry-run] [--backup]",
				"bpx name set <file.uasset> --index <n> --value <name> [--non-case-hash <u16>] [--case-preserving-hash <u16>] [--dry-run] [--backup]",
				"bpx name remove <file.uasset> --index <n> [--dry-run] [--backup]",
				"bpx export set-header <file.uasset> --index <n> --fields '<json>' [--dry-run] [--backup]",
				"bpx package set-flags <file.uasset> --flags <enum-or-raw> [--dry-run] [--backup]",
				"bpx datatable update-row <file.uasset> --row <name> --values '<json>' [--export <n>] [--dry-run] [--backup]",
				"bpx datatable add-row <file.uasset> --row <name> [--values '<json>'] [--export <n>] [--dry-run] [--backup]",
				"bpx datatable remove-row <file.uasset> --row <name> [--export <n>] [--dry-run] [--backup]",
				"bpx metadata set-root <file.uasset> --export <n> --key <k> --value <v> [--dry-run] [--backup]",
				"bpx metadata set-object <file.uasset> --export <n> --import <i> --key <k> --value <v> [--dry-run] [--backup]",
				"bpx enum write-value <file.uasset> --export <n> --name <k> --value <v> [--dry-run] [--backup]",
				"bpx stringtable write-entry <file.uasset> --export <n> --key <k> --value <v> [--dry-run] [--backup]",
				"bpx stringtable remove-entry <file.uasset> --export <n> --key <k> [--dry-run] [--backup]",
				"bpx stringtable set-namespace <file.uasset> --export <n> --namespace <ns> [--dry-run] [--backup]",
				"bpx localization set-source <file.uasset> --export <n> --path <dot.path> --value <text> [--dry-run] [--backup]",
				"bpx localization set-id <file.uasset> --export <n> --path <dot.path> --namespace <ns> --key <key> [--dry-run] [--backup]",
				"bpx localization set-stringtable-ref <file.uasset> --export <n> --path <dot.path> --table <table-id> --key <key> [--dry-run] [--backup]",
				"bpx localization rewrite-namespace <file.uasset> --from <ns-old> --to <ns-new> [--dry-run] [--backup]",
				"bpx localization rekey <file.uasset> --namespace <ns> --from-key <k-old> --to-key <k-new> [--dry-run] [--backup]",
				"bpx level var-set <file.umap> --actor <name|PersistentLevel.Name|export-index> --path <dot.path> --value '<json>' [--dry-run] [--backup]",
			},
		},
	}
}

func usageLinesForTopic(topic string) []string {
	lines := make([]string, 0, 8)
	for _, category := range helpCatalog() {
		for _, line := range category.Lines {
			if usageTopicFromLine(line) == topic {
				lines = append(lines, line)
			}
		}
	}
	return lines
}

func usageTopicFromLine(line string) string {
	if !strings.HasPrefix(line, "bpx ") {
		return ""
	}
	rest := strings.TrimPrefix(line, "bpx ")
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func helpTopicSummary(topic string) string {
	switch topic {
	case "version":
		return "Show the BPX CLI semantic version."
	case "find":
		return "Search directories for assets and summarize parseability."
	case "info", "dump", "validate":
		return "Inspect one package and optionally validate round-trip constraints."
	case "export", "import", "prop", "name", "package", "datatable", "blueprint", "localization", "metadata", "stringtable", "level", "var", "ref":
		return "Use subcommands below for read and write operations."
	case "enum", "struct", "class", "raw":
		return "Inspect specific UE data structures in one package export."
	case "write":
		return "Rewrite package bytes to a target file while preserving structure."
	default:
		return ""
	}
}

func topicHasWriteCommands(topic string) bool {
	switch topic {
	case "prop", "write", "var", "ref", "name", "export", "package", "datatable", "metadata", "enum", "stringtable", "localization", "level":
		return true
	default:
		return false
	}
}

func marshalDumpPayload(payload any, format string) ([]byte, string, error) {
	switch strings.ToLower(format) {
	case "json", "toml":
		b, err := marshalStructuredPayload(payload, format)
		if err != nil {
			return nil, "", err
		}
		return b, strings.ToLower(format), nil
	case "yaml", "yml":
		b, err := yaml.Marshal(payload)
		if err != nil {
			return nil, "", fmt.Errorf("marshal yaml: %w", err)
		}
		return b, "yaml", nil
	default:
		return nil, "", fmt.Errorf("unsupported dump format: %s", format)
	}
}

func marshalStructuredPayload(payload any, format string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case structuredOutputFormatJSON:
		b, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal json: %w", err)
		}
		return append(b, '\n'), nil
	case structuredOutputFormatTOML:
		b, err := toml.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal toml: %w", err)
		}
		if len(b) == 0 || b[len(b)-1] != '\n' {
			b = append(b, '\n')
		}
		return b, nil
	default:
		return nil, fmt.Errorf("unsupported output format: %s", format)
	}
}
