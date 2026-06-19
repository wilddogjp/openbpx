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
	defaultToolVersion         = "0.2.1"
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
		printRootUsage(stdout)
		return 0
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
	case "generate-skills":
		return runGenerateSkills(args[1:], stdout, stderr)
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
	case "material":
		return runMaterial(args[1:], stdout, stderr)
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
	return dispatchSubcommand(
		args,
		stdout,
		stderr,
		"usage: bpx export <list|info|set-header> ...",
		"unknown export command: %s\n",
		subcommandSpec{Name: "list", Run: runExportList},
		subcommandSpec{Name: "info", Run: runExportInfo},
		subcommandSpec{Name: "set-header", Run: runExportSetHeader},
	)
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
	return dispatchSubcommand(
		args,
		stdout,
		stderr,
		"usage: bpx prop <list|set|add|remove> ...",
		"unknown prop command: %s\n",
		subcommandSpec{Name: "list", Run: runPropList},
		subcommandSpec{Name: "set", Run: runPropSet},
		subcommandSpec{Name: "add", Run: runPropAdd},
		subcommandSpec{Name: "remove", Run: runPropRemove},
	)
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
	return dispatchSubcommand(
		args,
		stdout,
		stderr,
		"usage: bpx package <meta|custom-versions|depends|resolve-index|section|set-flags> ...",
		"unknown package command: %s\n",
		subcommandSpec{Name: "meta", Run: runPackageMeta},
		subcommandSpec{Name: "custom-versions", Run: runPackageCustomVersions},
		subcommandSpec{Name: "depends", Run: runPackageDepends},
		subcommandSpec{Name: "resolve-index", Run: runPackageResolveIndex},
		subcommandSpec{Name: "section", Run: runPackageSection},
		subcommandSpec{Name: "set-flags", Run: runPackageSetFlags},
	)
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
		"dependsOffset":               asset.Summary.DependsOffset,
		"thumbnailTableOffset":        asset.Summary.ThumbnailTableOffset,
		"importTypeHierarchiesCount":  asset.Summary.ImportTypeHierarchiesCount,
		"importTypeHierarchiesOffset": asset.Summary.ImportTypeHierarchiesOffset,
		"assetRegistryDataOffset":     asset.Summary.AssetRegistryDataOffset,
		"bulkDataStartOffset":         asset.Summary.BulkDataStartOffset,
		"preloadDependencyCount":      asset.Summary.PreloadDependencyCount,
		"preloadDependencyOffset":     asset.Summary.PreloadDependencyOffset,
		"dataResourceOffset":          asset.Summary.DataResourceOffset,
		"generations":                 asset.Summary.Generations,
		"customVersions":              customVersions,
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
	fmt.Fprintln(w, "Help tips:")
	fmt.Fprintln(w, "  `bpx help <command>` shows usage plus command behavior details.")
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
	if behaviorLines := helpTopicBehaviorLines(topic); len(behaviorLines) > 0 {
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "Behavior:")
		for _, line := range behaviorLines {
			fmt.Fprintf(w, "  %s\n", line)
		}
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
				"bpx generate-skills [--output-dir <dir>] [--filter <token>]",
				"bpx find assets <directory> [--pattern \"*.uasset\"] [--recursive]",
				"bpx find summary <directory> [--pattern \"*.uasset\"] [--recursive] [--format json|toml] [--out <path>]",
				"bpx info <file.uasset>",
				"bpx dump <file.uasset> [--format json|toml|yaml] [--out path]",
				"bpx export list <file.uasset> [--class <token>]",
				"bpx export info <file.uasset> --export <n>",
				"bpx import list <file.uasset>",
				"bpx import search <file.uasset> [--object <name>] [--class-package <pkg>] [--class-name <cls>]",
				"bpx import graph <directory> [--pattern \"*.uasset\"] [--recursive] [--group-by root|object] [--filter <token>]",
				"bpx import add <file.uasset> --texture </Game/Path/TextureName> [--dry-run] [--backup]",
				"bpx prop list <file.uasset> --export <n>",
				"bpx var list <file.uasset>",
				"bpx name list <file.uasset>",
				"bpx package meta <file.uasset>",
				"bpx package custom-versions <file.uasset>",
				"bpx package depends <file.uasset> [--reverse]",
				"bpx package resolve-index <file.uasset> --index <i>",
				"bpx package section <file.uasset> --name <section>",
				"bpx localization read <file.uasset> [--export <n>] [--include-history] [--format json|toml|csv]",
				"bpx localization query <file.uasset> [--export <n>] [--namespace <ns>] [--key <key>] [--text <token>] [--history-type <type>] [--limit <n>]",
				"bpx localization resolve <file.uasset> [--export <n>] --culture <culture> [--locres <path>] [--missing-only]",
				"bpx datatable read <file.uasset> [--export <n>] [--row <name>] [--format json|toml|csv|tsv] [--out path]",
				"bpx blueprint info <file.uasset> [--export <n>]",
				"bpx blueprint widget-read <file.uasset> [--export <n>]",
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
				"bpx level actor-search <file.umap> [--name <token>] [--actor-label <token>] [--actor-class <token>] [--limit <n>]",
				"bpx level var-list <file.umap> --actor <name|PersistentLevel.Name|export-index>",
				"bpx material read <file.uasset> [--export <n>] [--include-hlsl] [--children-root <directory>] [--parent <token>] [--pattern \"*.uasset\"] [--recursive] [--limit <n>]",
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
				"bpx blueprint widget-init <out.uasset> --template <minimum> [--engine <auto|ue5.6>] [--asset-name <Name>] [--package-path </Game/...>] [--parent-class </Script/Module.ClassName>] [--force] [--dry-run] [--backup]",
				"bpx blueprint widget-parent-class <file.uasset> --class </Script/Module.ClassName> [--export <n>] [--dry-run] [--backup]",
				"bpx blueprint widget-add <file.uasset> --parent <path|name|root> --type <image|textblock|richtextblock|progressbar|slider|spacer|scrollbar|editabletext|editabletextbox|multilineeditabletextbox|spinbox|comboboxstring|checkbox|userwidget|button|border|retainerbox|invalidationbox|menuanchor|namedslot|sizebox|scalebox|backgroundblur|safezone|windowtitlebararea|canvaspanel|overlay|verticalbox|horizontalbox|stackbox|scrollbox|wrapbox|gridpanel|uniformgridpanel|widgetswitcher|listview|tileview|treeview> --name <Widget_N> [--class </Game/...> when --type userwidget] [--export <n>] [--dry-run] [--backup]",
				"bpx blueprint widget-remove <file.uasset> --widget <path|name> [--export <n>] [--dry-run] [--backup]",
				"bpx blueprint widget-write <file.uasset> --widget <path|name> --property <text|visibility|render-opacity|brush-image|progressbar-percent|progressbar-fill-color|slider-value|slider-min-value|slider-max-value|slider-step-size|slider-orientation|spacer-size|scrollbar-thickness|scrollbar-orientation|checkbox-checked-state|checkbox-is-checked|editabletext-hint-text|editabletext-is-read-only|editabletext-is-password|editabletext-minimum-desired-width|editabletext-justification|editabletextbox-hint-text|editabletextbox-is-read-only|editabletextbox-is-password|editabletextbox-minimum-desired-width|editabletextbox-justification|multilineeditabletextbox-hint-text|multilineeditabletextbox-is-read-only|multilineeditabletextbox-justification|spinbox-value|spinbox-min-value|spinbox-max-value|spinbox-delta|comboboxstring-selected-option|comboboxstring-options|is-focusable|button-is-focusable|checkbox-is-focusable|slider-is-focusable|scrollbox-is-focusable|comboboxstring-is-focusable|listview-entry-widget-class|listview-orientation|listview-selection-mode|listview-consume-mouse-wheel|listview-is-focusable|listview-return-focus-to-selection|listview-clear-scroll-velocity-on-selection|listview-scroll-into-view-alignment|listview-wheel-scroll-multiplier|listview-enable-scroll-animation|listview-allow-overscroll|listview-enable-right-click-scrolling|listview-enable-touch-scrolling|listview-is-pointer-scrolling-enabled|listview-is-gamepad-scrolling-enabled|listview-horizontal-entry-spacing|listview-vertical-entry-spacing|listview-scrollbar-padding|tileview-entry-width|tileview-entry-height|tileview-scrollbar-disabled-visibility|tileview-entry-size-includes-entry-spacing|scrollbox-orientation|scrollbox-scrollbar-visibility|scrollbox-consume-mouse-wheel|sizebox-width-override|sizebox-width|sizebox-height-override|sizebox-height|sizebox-min-desired-width|sizebox-min-desired-height|sizebox-max-desired-width|sizebox-max-desired-height|sizebox-min-aspect-ratio|sizebox-max-aspect-ratio|scalebox-stretch|scalebox-stretch-direction|scalebox-user-specified-scale|scalebox-ignore-inherited-scale|wrapbox-wrap-size|wrapbox-explicit-wrap-size|wrapbox-inner-slot-padding|wrapbox-orientation|widgetswitcher-active-widget-index|retainerbox-retain-render|retainerbox-render-on-invalidation|retainerbox-render-on-phase|retainerbox-phase|retainerbox-phase-count|backgroundblur-strength|backgroundblur-apply-alpha-to-blur|safezone-pad-left|safezone-pad-right|safezone-pad-top|safezone-pad-bottom|invalidationbox-can-cache|uniformgridpanel-min-desired-slot-width|uniformgridpanel-min-desired-slot-height|uniformgridpanel-slot-padding|text-color-and-opacity|text-color|text-font|text-font-family|text-typeface|text-font-size|text-justification|text-auto-wrap-text|text-wrap-text-at|text-line-height-percentage|text-shadow-offset|text-shadow-color-and-opacity|text-outline-size|text-outline-color|button-normal-image|button-hovered-image|button-pressed-image|button-disabled-image|button-normal-tint|button-hovered-tint|button-pressed-tint|button-disabled-tint|button-normal-image-size|button-hovered-image-size|button-pressed-image-size|button-disabled-image-size|button-normal-draw-as|button-hovered-draw-as|button-pressed-draw-as|button-disabled-draw-as|menu-anchor-placement|button-background-color|button-color-and-opacity|border-padding|border-brush-color|border-content-color-and-opacity|border-horizontal-alignment|border-vertical-alignment|grid-row-fill|grid-column-fill|richtext-style-set|richtext-decorator-classes|richtext-override-default-style|richtext-default-font|richtext-default-font-family|richtext-default-typeface|richtext-default-font-size|richtext-default-color-and-opacity|richtext-default-shadow-offset|richtext-default-shadow-color-and-opacity|richtext-default-outline-size|richtext-default-outline-color|richtext-auto-wrap-text|richtext-wrap-text-at|richtext-line-height-percentage|richtext-justification|slot-padding|slot-size|slot-horizontal-alignment|slot-vertical-alignment|slot-row|slot-column|slot-row-span|slot-column-span|slot-layer|slot-nudge|layout-position|layout-size|layout-anchors|layout-alignment|layout-data> --value <value> [--export <n>] [--dry-run] [--backup]",
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
		return "Print the BPX CLI semantic version."
	case "find":
		return "Scan directories for assets and summarize parse outcomes."
	case "generate-skills":
		return "Generate BPX SKILL.md templates from built-in command help."
	case "info":
		return "Read one package summary (engine version, table counts, asset kind)."
	case "dump":
		return "Dump package summary/name/import/export tables in structured formats."
	case "validate":
		return "Run package integrity checks (exit code 2 when result is not OK)."
	case "export":
		return "Inspect export headers or update selected header fields."
	case "import":
		return "Inspect ImportMap entries and aggregate import dependency graphs."
	case "prop":
		return "Read decoded properties or mutate properties in one export."
	case "write":
		return "Rewrite a parsed package to a separate output path."
	case "var":
		return "Inspect variable defaults/declarations and update defaults or names."
	case "ref":
		return "Bulk-rewrite reference strings in NameMap and decoded properties."
	case "name":
		return "Inspect and edit NameMap entries with UE5-compatible hashes."
	case "package":
		return "Inspect package metadata/sections or update package flags."
	case "localization":
		return "Read/query/resolve localization data and edit existing text identities."
	case "datatable":
		return "Read DataTable-family rows and update DataTable rows."
	case "blueprint":
		return "Inspect and analyze blueprint exports, bytecode, and graph data."
	case "enum":
		return "List enum exports or update existing enum values."
	case "struct":
		return "List struct exports or inspect one struct export."
	case "stringtable":
		return "Read and edit StringTable entries and namespace."
	case "class":
		return "Inspect one class export payload/header by index."
	case "level":
		return "Inspect level exports and read/write actor properties in .umap."
	case "material":
		return "Inspect materials, scan child instances, and extract custom HLSL."
	case "raw":
		return "Read one export serial payload as base64."
	case "metadata":
		return "Read metadata exports or update root/object metadata key-values."
	default:
		return ""
	}
}

func helpTopicBehaviorLines(topic string) []string {
	switch topic {
	case "version":
		return []string{
			"Prints the CLI semantic version string and exits with code 0.",
			"No package file is parsed.",
		}
	case "find":
		return []string{
			"`assets`: collects files matching --pattern under a directory (default `*.uasset`, recursive).",
			"`summary`: parses each match and reports parsed counts, asset kind counts, and parse failures.",
			"`summary` continues when per-file parse fails and reports `parseFailures`.",
			"For map-only scans, pass `--pattern \"*.umap\"`.",
		}
	case "generate-skills":
		return []string{
			"Generates `SKILL.md` files from built-in BPX help metadata under `--output-dir` (default: `skills`).",
			"`--filter` limits generation by substring match on skill name/description.",
			"Generated output applies built-in command-profile supplements baked into the binary.",
		}
	case "info":
		return []string{
			"Parses one package and prints engine version, table counts, and guessed asset kind.",
			"Read-only command; no files are written.",
		}
	case "dump":
		return []string{
			"Emits Summary/NameMap/ImportMap/ExportMap payload for one package.",
			"Supports --format json|toml|yaml and optional --out file write.",
			"When `--out` is used, stdout returns an acknowledgement object (`file`, `format`, `out`).",
		}
	case "validate":
		return []string{
			"Runs parse and consistency checks for one package.",
			"`--binary-equality` also checks no-op rewrite byte equality.",
			"Returns exit code 2 when validation result is not OK.",
			"Validation details are emitted in `result` payload.",
		}
	case "export":
		return []string{
			"`list`: lists export headers with class/object/serial info.",
			"`info`: inspects one export header by --export index.",
			"`set-header`: updates selected export header fields (write command).",
			"`set-header` requires non-empty `--fields` JSON and reports old/new field values.",
		}
	case "import":
		return []string{
			"`list`: lists ImportMap entries for one package.",
			"`search`: filters imports by object/class tokens (requires at least one filter).",
			"`graph`: aggregates import dependency edges across a directory.",
			"`add`: append-only Texture2D import reference insertion for an existing `/Game/...` asset path.",
			"Use `blueprint widget-write --property brush-image` as the normal image-texture workflow; use `import add` when you need manual import management before lower-level edits.",
			"`graph` reports per-file parse failures without aborting the whole scan.",
		}
	case "prop":
		return []string{
			"`list`: decodes properties for one export and includes warnings.",
			"`set`: updates an existing property value at --path.",
			"`add`: appends a new top-level property from --spec JSON.",
			"`remove`: removes a property at --path.",
			"Write subcommands report old/new values, size deltas, and changed-byte status.",
		}
	case "write":
		return []string{
			"Rewrites parsed package bytes to --out using current in-memory structure.",
			"Never modifies the source file; output target is required.",
			"`--dry-run` reports changed/bytes without writing files.",
			"`--backup` creates `<out>.backup` when destination already exists.",
		}
	case "var":
		return []string{
			"`list`: merges CDO defaults with declaration metadata.",
			"`set-default`: writes a variable default on CDO properties.",
			"`rename`: rewrites matching NameMap entries from --from to --to.",
			"`rename` fails when destination variable is already declared; may return declaration warnings.",
		}
	case "ref":
		return []string{
			"`rewrite`: replaces reference tokens across NameMap and decodable properties.",
			"Requires different `--from` and `--to` values.",
			"Response includes NameMap/property rewrite counts and warnings.",
		}
	case "name":
		return []string{
			"`list`: lists NameMap entries and hashes.",
			"`add`: appends a new NameMap entry.",
			"`set`: rewrites one NameMap entry by index.",
			"`remove`: removes tail NameMap entry only when safety checks pass.",
			"`add`/`set` auto-compute UE5 hashes when hash flags are omitted.",
		}
	case "package":
		return []string{
			"`meta`: shows package GUID/flags/version/offset summary.",
			"`custom-versions`: lists custom version GUID/version pairs.",
			"`depends`: decodes DependsMap entries.",
			"`depends --reverse`: adds reverse dependency view (who references each export).",
			"`resolve-index`: classifies and resolves signed FPackageIndex.",
			"`section`: reads one raw package section by --name.",
			"`set-flags`: rewrites package flags within supported safe scope.",
			"`set-flags` blocks `PKG_FilterEditorOnly` and `PKG_UnversionedProperties` toggles.",
		}
	case "localization":
		return []string{
			"`read`: enumerates TextProperty + GatherableTextData entries.",
			"`query`: filters entries by namespace/key/text/history type.",
			"`resolve`: previews localized strings for --culture (optional .locres).",
			"`set-source`/`set-id`/`set-stringtable-ref`: updates existing text data.",
			"`rewrite-namespace`/`rekey`: bulk-rewrites namespace or key values.",
			"`resolve --missing-only` returns unresolved entries only.",
		}
	case "datatable":
		return []string{
			"`read`: decodes DataTable/CurveTable/CompositeDataTable rows.",
			"`update-row`: patches fields in an existing row (DataTable exports only).",
			"`add-row`: appends a new row (DataTable only; row name must resolve in NameMap).",
			"`remove-row`: removes a row by name (DataTable only).",
			"`read --format csv|tsv` flattens rows for spreadsheet-style output.",
		}
	case "blueprint":
		return []string{
			"`info`: summarizes blueprint/function exports.",
			"`widget-read`: reads WidgetBlueprint / WidgetTree hierarchy as normalized JSON, plus logical widget aggregation and high-level widget/slot summaries.",
			"`widget-init`: clones a validated empty WidgetBlueprint template into a new output asset and rewrites package/object identity.",
			"`widget-parent-class`: rewrites the WidgetBlueprint parent class on an otherwise rootless WidgetBlueprint.",
			"`widget-add`: creates a root container/content widget or inserts a bare child widget under supported panel/content parents.",
			"`widget-remove`: removes one non-root leaf widget from the logical WidgetTree plus related WidgetBlueprint metadata.",
			"`widget-write`: updates one logical widget across designer/generated trees.",
			"`bytecode`: extracts selected bytecode range as base64.",
			"`disasm`: disassembles bytecode (json|toml|text, optional analysis).",
			"`trace`: traces an execution path between nodes.",
			"`call-args`: inspects call-node argument pins/defaults.",
			"`refs`: reverse-searches soft-path usage on node pins.",
			"`search`: token-searches nodes/pins in one blueprint package.",
			"`scan-functions`: aggregates function names across a directory.",
			"`infer-pack`: emits CFG/callsite/def-use inference artifacts.",
			"`widget-init` currently supports the `minimum` template and rewrites identity only within validated template layouts.",
			"`widget-init` expects `--package-path` to be a directory like `/Game/UI`; BPX appends the asset name automatically. `--parent-class` currently accepts compiled `/Script/...` classes, including project/plugin module classes such as `/Script/LyraGame.LyraActivatableWidget`.",
			"`widget-parent-class` currently supports only rootless WidgetBlueprints and compiled `/Script/...` parent classes, including project/plugin module classes.",
			"`widget-add` supports non-empty `CanvasPanel` / `Overlay` / `VerticalBox` / `HorizontalBox` / `StackBox` / `ScrollBox` / `WrapBox` / `GridPanel` / `UniformGridPanel` / `WidgetSwitcher` parents plus single-child `Button` / `CheckBox` / `Border` / `RetainerBox` / `InvalidationBox` / `MenuAnchor` / `NamedSlot` / `SizeBox` / `ScaleBox` / `BackgroundBlur` / `SafeZone` / `WindowTitleBarArea` parents; `--parent root` supports the same container/content set except leaf widgets such as `Image` / `TextBlock` / `RichTextBlock` / `ProgressBar` / `Slider` / `Spacer` / `ScrollBar` / `EditableText` / `EditableTextBox` / `MultiLineEditableTextBox` / `SpinBox` / `ComboBoxString` / `UserWidget`. `--type userwidget` requires `--class </Game/...>` and instantiates the referenced WidgetBlueprintGeneratedClass as a child.",
			"`widget-remove` currently supports non-root leaf widgets only and rewrites WidgetTree/Blueprint metadata plus removable orphan export/import/name entries when the remaining package references validate cleanly.",
			"Widget-building commands (`widget-init`, `widget-parent-class`, `widget-add`, `widget-remove`, `widget-write`) are order-sensitive and must be run sequentially against the same asset.",
			"Do not parallelize repeated widget mutations on one asset; later steps depend on the exact bytes/layout produced by earlier steps.",
			"`widget-write` supports `text`, `visibility`, `render-opacity`, `brush-image`, basic widget helpers such as `progressbar-percent`, `progressbar-fill-color`, `slider-value`, `slider-min-value`, `slider-max-value`, `slider-step-size`, `slider-orientation`, `spacer-size`, `scrollbar-thickness`, `scrollbar-orientation`, `checkbox-checked-state`, `checkbox-is-checked`, `editabletext-hint-text`, `editabletext-is-read-only`, `editabletext-is-password`, `editabletext-minimum-desired-width`, `editabletext-justification`, `editabletextbox-hint-text`, `editabletextbox-is-read-only`, `editabletextbox-is-password`, `editabletextbox-minimum-desired-width`, `editabletextbox-justification`, `multilineeditabletextbox-hint-text`, `multilineeditabletextbox-is-read-only`, `multilineeditabletextbox-justification`, `spinbox-value`, `spinbox-min-value`, `spinbox-max-value`, `spinbox-delta`, `comboboxstring-selected-option`, `comboboxstring-options`, focus helpers such as `is-focusable`, `button-is-focusable`, `checkbox-is-focusable`, `slider-is-focusable`, `scrollbox-is-focusable`, `comboboxstring-is-focusable`, `scrollbox-orientation`, `scrollbox-scrollbar-visibility`, `scrollbox-consume-mouse-wheel`, `sizebox-width-override`, `sizebox-height-override`, `sizebox-min/max-desired-*`, `sizebox-min/max-aspect-ratio`, `scalebox-stretch`, `scalebox-stretch-direction`, `scalebox-user-specified-scale`, `scalebox-ignore-inherited-scale`, `wrapbox-wrap-size`, `wrapbox-explicit-wrap-size`, `wrapbox-inner-slot-padding`, `wrapbox-orientation`, `widgetswitcher-active-widget-index`, `retainerbox-retain-render`, `retainerbox-render-on-invalidation`, `retainerbox-render-on-phase`, `retainerbox-phase`, `retainerbox-phase-count`, `backgroundblur-strength`, `backgroundblur-apply-alpha-to-blur`, `safezone-pad-left/right/top/bottom`, `invalidationbox-can-cache`, and `uniformgridpanel-min-desired-slot-width` / `uniformgridpanel-min-desired-slot-height` / `uniformgridpanel-slot-padding`, `TextBlock` style helpers such as `text-color`, `text-font`, `text-typeface`, `text-font-size`, `text-justification`, `text-auto-wrap-text`, `text-wrap-text-at`, `text-line-height-percentage`, `text-shadow-offset`, `text-shadow-color-and-opacity`, and `text-outline-size` / `text-outline-color`, button state-brush helpers such as `button-normal-image`, `button-normal-tint`, `button-normal-image-size`, and `button-normal-draw-as`, `menu-anchor-placement`, button/border appearance helpers such as `button-background-color`, `button-color-and-opacity`, `border-padding`, and `border-brush-color`, `RichTextBlock` helpers such as `richtext-style-set`, `richtext-decorator-classes`, `richtext-override-default-style`, `richtext-default-font`, `richtext-default-typeface`, `richtext-default-font-size`, `richtext-default-color-and-opacity`, `richtext-default-shadow-offset`, `richtext-default-shadow-color-and-opacity`, `richtext-default-outline-size`, `richtext-default-outline-color`, `richtext-auto-wrap-text`, `richtext-wrap-text-at`, `richtext-line-height-percentage`, and `richtext-justification`, grid fill helpers such as `grid-row-fill` / `grid-column-fill`, and slot/layout helpers such as `slot-padding`, `slot-size`, `slot-row`, `slot-column`, `slot-row-span`, `slot-column-span`, `slot-layer`, `slot-nudge`, `layout-position`, and `layout-data`.",
			"`widget-read` summaries currently cover widget-level text/brush/button/border/grid/basic-widget data and slot-level layout/grid helpers for the supported classes.",
			"`widget-move` / `widget-clone`, broader RichTextBlock styling such as transform policy, strike brushes, and material-backed font overrides, and CommonUI-specific writes are not implemented yet.",
			"For image widgets, prefer `widget-write --property brush-image`; it adds missing texture imports automatically.",
			"`widget-write --property brush-image` expects a full Unreal texture path like `/Game/UI/T_Icon`, not a filesystem path.",
			"If your shell rewrites `/Game/...` arguments (for example Git Bash/MSYS path conversion), disable that rewriting before running widget commands.",
			"`bytecode`/`disasm` support range selection (`auto|export-map|ustruct-script|serial-full`).",
		}
	case "enum":
		return []string{
			"`list`: enumerates enum exports.",
			"`write-value`: updates an existing enum entry value.",
			"`write-value` edits existing data only (no enum entry insertion/removal).",
		}
	case "struct":
		return []string{
			"`definition`: lists struct-like exports.",
			"`details`: inspects one struct export by --export index.",
		}
	case "stringtable":
		return []string{
			"`read`: lists string table exports.",
			"`write-entry`: updates an existing key value.",
			"`remove-entry`: removes an existing key.",
			"`set-namespace`: rewrites string table namespace.",
			"Write commands operate on existing string table exports and report changed-byte status.",
		}
	case "class":
		return []string{
			"Inspects one class export payload/header by --export index.",
			"Output follows generic export info shape (`file`, `export`).",
		}
	case "level":
		return []string{
			"`info`: inspects one level export.",
			"`actor-search`: filters PersistentLevel child exports by name/ActorLabel/ActorClass tokens.",
			"`var-list`: decodes actor properties selected by --actor.",
			"`var-set`: updates one actor property at --path.",
			"`--actor` accepts object name, `PersistentLevel.<Name>`, or export index.",
		}
	case "material":
		return []string{
			"`read`: unified read entry for material inputs/references/parent and optional child scan/HLSL summary.",
			"`inspect`: summarizes material inputs, asset references, and direct parent material.",
			"`children`: scans a directory for material instances matching --parent token.",
			"`hlsl`: shows custom-node HLSL snippets (`UMaterialExpressionCustom::Code`) and explains full-translation limits.",
		}
	case "raw":
		return []string{
			"Reads raw serial payload bytes for one export.",
			"`--full` includes the complete base64 payload.",
			"Default output keeps payload compact via abbreviated base64 fields.",
		}
	case "metadata":
		return []string{
			"Default form reads one metadata export by --export index.",
			"`set-root`: updates root metadata key/value.",
			"`set-object`: updates metadata for one import key/value.",
			"Set commands report the resolved property path that was mutated.",
		}
	default:
		return nil
	}
}

func topicHasWriteCommands(topic string) bool {
	switch topic {
	case "prop", "write", "var", "ref", "name", "export", "package", "datatable", "metadata", "enum", "stringtable", "localization", "level", "blueprint":
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
