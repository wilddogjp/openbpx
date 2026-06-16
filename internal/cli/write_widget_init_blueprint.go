package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/wilddogjp/openbpx/pkg/edit"
	"github.com/wilddogjp/openbpx/pkg/uasset"
)

type widgetInitTemplateSpec struct {
	Key             string
	Engine          string
	FixturePath     string
	TemplatePackage string
	TemplateAsset   string
	TemplateBytes   []byte
}

var (
	widgetInitAssetNamePattern   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	widgetInitPackagePathPattern = regexp.MustCompile(`^/[A-Za-z0-9_]+(?:/[A-Za-z0-9_]+)*$`)
	widgetInitTemplateSpecs      = []widgetInitTemplateSpec{
		{
			Key:             "minimum",
			Engine:          "ue5.6",
			FixturePath:     filepath.Join("testdata", "golden", "ue5.6", "parse", "WBP_Minimum.uasset"),
			TemplatePackage: "/Game/BPXFixtures/Parse/WBP_Minimum",
			TemplateAsset:   "WBP_Minimum",
			TemplateBytes:   widgetInitMinimumUE56TemplateBytes,
		},
	}
)

func runBlueprintWidgetInit(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("blueprint widget-init", stderr)
	opts := registerCommonFlags(fs)
	templateKey := fs.String("template", "", "template key (supported: minimum)")
	engine := fs.String("engine", "auto", "template engine: auto or explicit engine id like ue5.6")
	assetNameRaw := fs.String("asset-name", "", "optional Unreal asset/object name; defaults to output filename stem")
	packagePathRaw := fs.String("package-path", "", "optional Unreal package directory like /Game/UI")
	parentClassRaw := fs.String("parent-class", "", "optional compiled parent class like /Script/CommonUI.CommonActivatableWidget or /Script/LyraGame.LyraActivatableWidget")
	force := fs.Bool("force", false, "overwrite destination file if it already exists")
	dryRun := fs.Bool("dry-run", false, "do not write output")
	backup := fs.Bool("backup", false, "create <out>.backup before overwrite when destination exists")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || strings.TrimSpace(*templateKey) == "" {
		fmt.Fprintln(stderr, "usage: bpx blueprint widget-init <out.uasset> --template <minimum> [--engine <auto|ue5.6>] [--asset-name <Name>] [--package-path </Game/...>] [--parent-class </Script/Module.ClassName>] [--force] [--dry-run] [--backup]")
		return 1
	}

	outFile := fs.Arg(0)
	identity, err := resolveWidgetInitIdentity(outFile, *assetNameRaw, *packagePathRaw)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	templateSpec, err := resolveWidgetInitTemplate(strings.TrimSpace(*templateKey), strings.TrimSpace(*engine))
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if identity.LongPackageName == templateSpec.TemplatePackage {
		fmt.Fprintf(stderr, "error: destination package %q matches the template package; choose a different --package-path or --asset-name\n", identity.LongPackageName)
		return 1
	}

	templateBytes, err := readWidgetInitTemplateBytes(templateSpec)
	if err != nil {
		fmt.Fprintf(stderr, "error: read widget-init template: %v\n", err)
		return 1
	}
	templateAsset, err := uasset.ParseBytes(templateBytes, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: parse widget-init template: %v\n", err)
		return 1
	}
	if err := validateWidgetInitTemplateShape(templateAsset, templateSpec.TemplateAsset); err != nil {
		fmt.Fprintf(stderr, "error: validate widget-init template: %v\n", err)
		return 1
	}

	workingBytes := append([]byte(nil), templateBytes...)
	workingAsset := templateAsset
	updates := make([]map[string]any, 0, 4)
	addedNames := make([]string, 0, 8)
	addedImports := make([]map[string]any, 0, 4)

	if templateSpec.TemplatePackage != identity.LongPackageName {
		workingBytes, _, _, _, err = rewriteReferencesAsset(workingAsset, *opts, templateSpec.TemplatePackage, identity.LongPackageName)
		if err != nil {
			fmt.Fprintf(stderr, "error: rewrite template package identity: %v\n", err)
			return 1
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, *opts)
		if err != nil {
			fmt.Fprintf(stderr, "error: reparse after template package rewrite: %v\n", err)
			return 1
		}
		if strings.TrimSpace(workingAsset.Summary.PackageName) != identity.LongPackageName {
			workingBytes, err = rewriteWidgetInitSummaryPackageName(workingAsset, identity.LongPackageName)
			if err != nil {
				fmt.Fprintf(stderr, "error: rewrite summary package identity: %v\n", err)
				return 1
			}
			workingAsset, err = uasset.ParseBytes(workingBytes, *opts)
			if err != nil {
				fmt.Fprintf(stderr, "error: reparse after summary package rewrite: %v\n", err)
				return 1
			}
		}
		workingBytes, _, err = rewriteWidgetInitAssetRegistryStringReplace(workingAsset, templateSpec.TemplatePackage, identity.LongPackageName)
		if err != nil {
			fmt.Fprintf(stderr, "error: rewrite asset registry package identity: %v\n", err)
			return 1
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, *opts)
		if err != nil {
			fmt.Fprintf(stderr, "error: reparse after asset registry package rewrite: %v\n", err)
			return 1
		}
		updates = append(updates, map[string]any{
			"path":     "package.name",
			"oldValue": templateSpec.TemplatePackage,
			"newValue": identity.LongPackageName,
		})
	}

	if templateSpec.TemplateAsset != identity.AssetName {
		workingBytes, _, _, _, err = rewriteReferencesAsset(workingAsset, *opts, templateSpec.TemplateAsset, identity.AssetName)
		if err != nil {
			fmt.Fprintf(stderr, "error: rewrite template asset identity: %v\n", err)
			return 1
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, *opts)
		if err != nil {
			fmt.Fprintf(stderr, "error: reparse after template asset rewrite: %v\n", err)
			return 1
		}
		workingBytes, _, err = rewriteWidgetInitAssetRegistryStringReplace(workingAsset, templateSpec.TemplateAsset, identity.AssetName)
		if err != nil {
			fmt.Fprintf(stderr, "error: rewrite asset registry asset identity: %v\n", err)
			return 1
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, *opts)
		if err != nil {
			fmt.Fprintf(stderr, "error: reparse after asset registry asset rewrite: %v\n", err)
			return 1
		}
		updates = append(updates,
			map[string]any{
				"path":     "export.WidgetBlueprint.ObjectName",
				"oldValue": templateSpec.TemplateAsset,
				"newValue": identity.AssetName,
			},
			map[string]any{
				"path":     "export.GeneratedClass.ObjectName",
				"oldValue": templateSpec.TemplateAsset + "_C",
				"newValue": identity.AssetName + "_C",
			},
			map[string]any{
				"path":     "export.CDO.ObjectName",
				"oldValue": "Default__" + templateSpec.TemplateAsset + "_C",
				"newValue": "Default__" + identity.AssetName + "_C",
			},
		)
	}

	if strings.TrimSpace(*parentClassRaw) != "" {
		parentClassSpec, err := parseWidgetParentClassSpec(*parentClassRaw)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		ctx, err := resolveWidgetAddRootContext(workingAsset, 0, "CanvasPanel")
		if err != nil {
			fmt.Fprintf(stderr, "error: resolve rootless widget-init context: %v\n", err)
			return 1
		}
		var (
			oldParentClass             string
			addedNamesForParentClass   []string
			addedImportsForParentClass []map[string]any
		)
		workingBytes, workingAsset, addedNamesForParentClass, addedImportsForParentClass, oldParentClass, err = rewriteWidgetBlueprintParentClass(workingAsset, *opts, ctx.Target.BlueprintExport, ctx.GeneratedClassExport, parentClassSpec)
		if err != nil {
			fmt.Fprintf(stderr, "error: rewrite widget-init parent class: %v\n", err)
			return 1
		}
		addedNames = append(addedNames, addedNamesForParentClass...)
		addedImports = append(addedImports, addedImportsForParentClass...)
		updates = append(updates, map[string]any{
			"path":     "export.WidgetBlueprint.ParentClass",
			"oldValue": oldParentClass,
			"newValue": parentClassSpec.Raw,
		})
	}

	if err := validateWidgetInitResult(workingAsset, templateSpec, identity); err != nil {
		fmt.Fprintf(stderr, "error: validate widget-init result: %v\n", err)
		return 1
	}

	resp := map[string]any{
		"file":            outFile,
		"template":        templateSpec.Key,
		"engine":          templateSpec.Engine,
		"assetName":       identity.AssetName,
		"packagePath":     identity.PackagePath,
		"longPackageName": identity.LongPackageName,
		"templateSource":  templateSpec.FixturePath,
		"dryRun":          *dryRun,
		"changed":         true,
		"outputBytes":     len(workingBytes),
		"updates":         updates,
	}
	if len(addedNames) > 0 {
		resp["addedNames"] = addedNames
	}
	if len(addedImports) > 0 {
		resp["addedImports"] = addedImports
	}
	if strings.TrimSpace(*parentClassRaw) != "" {
		resp["parentClass"] = strings.TrimSpace(*parentClassRaw)
	}
	if *dryRun {
		return printJSON(stdout, resp)
	}

	info, statErr := os.Stat(outFile)
	if statErr == nil {
		if info.IsDir() {
			fmt.Fprintf(stderr, "error: destination is a directory: %s\n", outFile)
			return 1
		}
		if !*force {
			fmt.Fprintf(stderr, "error: destination file already exists; pass --force to overwrite: %s\n", outFile)
			return 1
		}
		if *backup {
			if err := createBackupIfExists(outFile); err != nil {
				fmt.Fprintf(stderr, "error: backup destination file: %v\n", err)
				return 1
			}
		}
	} else if !os.IsNotExist(statErr) {
		fmt.Fprintf(stderr, "error: stat destination file: %v\n", statErr)
		return 1
	}

	if err := os.MkdirAll(filepath.Dir(outFile), 0o755); err != nil {
		fmt.Fprintf(stderr, "error: create destination directory: %v\n", err)
		return 1
	}
	if err := writeFileAtomically(outFile, workingBytes, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write file: %v\n", err)
		return 1
	}
	return printJSON(stdout, resp)
}

type widgetInitIdentity struct {
	AssetName       string
	PackagePath     string
	LongPackageName string
}

func resolveWidgetInitIdentity(outFile, assetNameRaw, packagePathRaw string) (widgetInitIdentity, error) {
	var out widgetInitIdentity

	cleanOut := filepath.Clean(strings.TrimSpace(outFile))
	if cleanOut == "." || cleanOut == "" {
		return out, fmt.Errorf("output path is required")
	}
	if !strings.EqualFold(filepath.Ext(cleanOut), ".uasset") {
		return out, fmt.Errorf("output path must end with .uasset")
	}

	filenameStem := strings.TrimSpace(strings.TrimSuffix(filepath.Base(cleanOut), filepath.Ext(cleanOut)))
	derivedPackagePath, derivedPackagePathOK := deriveWidgetInitPackagePath(cleanOut)

	assetName := strings.TrimSpace(assetNameRaw)
	if assetName == "" {
		assetName = filenameStem
	} else if derivedPackagePathOK && assetName != filenameStem {
		return out, fmt.Errorf("asset name %q must match output filename stem %q when output path is inside UE Content or plugin Content", assetName, filenameStem)
	}
	if !widgetInitAssetNamePattern.MatchString(assetName) {
		return out, fmt.Errorf("asset name must match %s, got %q", widgetInitAssetNamePattern.String(), assetName)
	}

	packagePath := normalizeWidgetInitPackagePath(packagePathRaw)
	if packagePath == "" {
		if !derivedPackagePathOK {
			return out, fmt.Errorf("package path is required unless output path is inside a UE Content directory or plugin Content directory")
		}
		packagePath = derivedPackagePath
	} else if derivedPackagePathOK && packagePath != derivedPackagePath {
		return out, fmt.Errorf("package path %q must match derived output package path %q when output path is inside UE Content or plugin Content", packagePath, derivedPackagePath)
	}
	if !widgetInitPackagePathPattern.MatchString(packagePath) {
		return out, fmt.Errorf("package path must match %s, got %q", widgetInitPackagePathPattern.String(), packagePath)
	}
	if packagePath == "/Script" {
		return out, fmt.Errorf("package path must target game or plugin content, got %q", packagePath)
	}
	if strings.EqualFold(pathBaseLocal(packagePath), assetName) {
		return out, fmt.Errorf("package path must be a directory like /Game/UI, not a full asset path ending in %q", assetName)
	}

	out.AssetName = assetName
	out.PackagePath = packagePath
	out.LongPackageName = packagePath + "/" + assetName
	return out, nil
}

func normalizeWidgetInitPackagePath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	cleaned := filepath.ToSlash(filepath.Clean(trimmed))
	if cleaned == "." {
		return ""
	}
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}
	return strings.TrimRight(cleaned, "/")
}

func deriveWidgetInitPackagePath(outFile string) (string, bool) {
	abs, err := filepath.Abs(outFile)
	if err != nil {
		return "", false
	}
	cleaned := filepath.Clean(abs)
	segments := splitPathSegments(cleaned)
	if len(segments) == 0 {
		return "", false
	}
	for i := 0; i < len(segments); i++ {
		if !strings.EqualFold(segments[i], "Content") {
			continue
		}
		pkgSegments := make([]string, 0, len(segments[i+1:]))
		for _, seg := range segments[i+1 : len(segments)-1] {
			if strings.TrimSpace(seg) != "" {
				pkgSegments = append(pkgSegments, seg)
			}
		}
		if i >= 2 && strings.EqualFold(segments[i-2], "Plugins") {
			root := "/" + segments[i-1]
			if len(pkgSegments) == 0 {
				return root, true
			}
			return root + "/" + strings.Join(pkgSegments, "/"), true
		}
		root := "/Game"
		if len(pkgSegments) == 0 {
			return root, true
		}
		return root + "/" + strings.Join(pkgSegments, "/"), true
	}
	return "", false
}

func splitPathSegments(path string) []string {
	normalized := filepath.ToSlash(filepath.Clean(path))
	if normalized == "." || normalized == "/" {
		return nil
	}
	return strings.Split(strings.Trim(normalized, "/"), "/")
}

func pathBaseLocal(path string) string {
	trimmed := strings.TrimRight(filepath.ToSlash(strings.TrimSpace(path)), "/")
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	return parts[len(parts)-1]
}

func resolveWidgetInitTemplate(templateKey, engineRaw string) (widgetInitTemplateSpec, error) {
	key := strings.ToLower(strings.TrimSpace(templateKey))
	engine := strings.ToLower(strings.TrimSpace(engineRaw))
	if engine == "" {
		engine = "auto"
	}

	candidates := make([]widgetInitTemplateSpec, 0, len(widgetInitTemplateSpecs))
	for _, spec := range widgetInitTemplateSpecs {
		if spec.Key == key {
			candidates = append(candidates, spec)
		}
	}
	if len(candidates) == 0 {
		available := make([]string, 0, len(widgetInitTemplateSpecs))
		for _, spec := range widgetInitTemplateSpecs {
			available = append(available, spec.Key)
		}
		slices.Sort(available)
		available = slices.Compact(available)
		return widgetInitTemplateSpec{}, fmt.Errorf("unknown widget-init template %q (supported: %s)", templateKey, strings.Join(available, ", "))
	}

	if hasEmbeddedWidgetInitTemplate(candidates) {
		return resolveWidgetInitEmbeddedTemplate(candidates, templateKey, engineRaw, engine)
	}

	if engine != "auto" {
		for _, spec := range candidates {
			if spec.Engine == engine {
				resolvedPath, err := resolveWidgetInitTemplateFixturePath(spec.FixturePath)
				if err != nil {
					return widgetInitTemplateSpec{}, err
				}
				spec.FixturePath = resolvedPath
				return spec, nil
			}
		}
		engines := make([]string, 0, len(candidates))
		for _, spec := range candidates {
			engines = append(engines, spec.Engine)
		}
		slices.Sort(engines)
		return widgetInitTemplateSpec{}, fmt.Errorf("template %q does not support engine %q (available: %s)", templateKey, engineRaw, strings.Join(engines, ", "))
	}

	found := make([]widgetInitTemplateSpec, 0, len(candidates))
	for _, spec := range candidates {
		resolvedPath, err := resolveWidgetInitTemplateFixturePath(spec.FixturePath)
		if err == nil {
			spec.FixturePath = resolvedPath
			found = append(found, spec)
		}
	}
	if len(found) == 1 {
		return found[0], nil
	}
	if len(found) > 1 {
		engines := make([]string, 0, len(found))
		for _, spec := range found {
			engines = append(engines, spec.Engine)
		}
		slices.Sort(engines)
		return widgetInitTemplateSpec{}, fmt.Errorf("template %q is available for multiple engines (%s); pass --engine", templateKey, strings.Join(engines, ", "))
	}
	return widgetInitTemplateSpec{}, fmt.Errorf("could not locate template fixture for %q; expected %s somewhere above the current working directory", templateKey, candidates[0].FixturePath)
}

func hasEmbeddedWidgetInitTemplate(candidates []widgetInitTemplateSpec) bool {
	for _, spec := range candidates {
		if len(spec.TemplateBytes) > 0 {
			return true
		}
	}
	return false
}

func resolveWidgetInitEmbeddedTemplate(candidates []widgetInitTemplateSpec, templateKey, engineRaw, engine string) (widgetInitTemplateSpec, error) {
	if engine != "auto" {
		for _, spec := range candidates {
			if spec.Engine == engine && len(spec.TemplateBytes) > 0 {
				return spec, nil
			}
		}
		engines := make([]string, 0, len(candidates))
		for _, spec := range candidates {
			if len(spec.TemplateBytes) > 0 {
				engines = append(engines, spec.Engine)
			}
		}
		slices.Sort(engines)
		if len(engines) == 0 {
			return widgetInitTemplateSpec{}, fmt.Errorf("template %q does not have an embedded runtime template for engine %q", templateKey, engineRaw)
		}
		return widgetInitTemplateSpec{}, fmt.Errorf("template %q does not support engine %q (embedded available: %s)", templateKey, engineRaw, strings.Join(engines, ", "))
	}

	found := make([]widgetInitTemplateSpec, 0, len(candidates))
	for _, spec := range candidates {
		if len(spec.TemplateBytes) > 0 {
			found = append(found, spec)
		}
	}
	if len(found) == 1 {
		return found[0], nil
	}
	if len(found) > 1 {
		engines := make([]string, 0, len(found))
		for _, spec := range found {
			engines = append(engines, spec.Engine)
		}
		slices.Sort(engines)
		return widgetInitTemplateSpec{}, fmt.Errorf("template %q is embedded for multiple engines (%s); pass --engine", templateKey, strings.Join(engines, ", "))
	}
	return widgetInitTemplateSpec{}, fmt.Errorf("template %q does not have an embedded runtime template", templateKey)
}

func readWidgetInitTemplateBytes(spec widgetInitTemplateSpec) ([]byte, error) {
	if len(spec.TemplateBytes) > 0 {
		return append([]byte(nil), spec.TemplateBytes...), nil
	}
	return os.ReadFile(spec.FixturePath)
}

func resolveWidgetInitTemplateFixturePath(relPath string) (string, error) {
	trimmed := filepath.Clean(strings.TrimSpace(relPath))
	if trimmed == "" {
		return "", fmt.Errorf("template fixture path is empty")
	}
	if filepath.IsAbs(trimmed) {
		if _, err := os.Stat(trimmed); err != nil {
			return "", fmt.Errorf("template fixture not found: %w", err)
		}
		return trimmed, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, trimmed)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return "", fmt.Errorf("template fixture not found: %s", relPath)
}

func validateWidgetInitTemplateShape(asset *uasset.Asset, templateAssetName string) error {
	if asset == nil {
		return fmt.Errorf("asset is nil")
	}
	if err := validateWidgetInitBlueprintShape(asset); err != nil {
		return err
	}
	if bpIdx, ok := findExportIndexByObjectName(asset, templateAssetName); !ok || bpIdx < 0 {
		return fmt.Errorf("template WidgetBlueprint export %q not found", templateAssetName)
	}
	return nil
}

func validateWidgetInitResult(asset *uasset.Asset, spec widgetInitTemplateSpec, identity widgetInitIdentity) error {
	if asset == nil {
		return fmt.Errorf("asset is nil")
	}
	if err := validateWidgetInitBlueprintShape(asset); err != nil {
		return err
	}
	if _, ok := findExportIndexByObjectName(asset, identity.AssetName); !ok {
		return fmt.Errorf("WidgetBlueprint export %q not found after rewrite", identity.AssetName)
	}
	if _, ok := findExportIndexByObjectName(asset, identity.AssetName+"_C"); !ok {
		return fmt.Errorf("generated class export %q not found after rewrite", identity.AssetName+"_C")
	}
	if _, ok := findExportIndexByObjectName(asset, "Default__"+identity.AssetName+"_C"); !ok {
		return fmt.Errorf("CDO export %q not found after rewrite", "Default__"+identity.AssetName+"_C")
	}
	if residuals := findWidgetInitResidualTemplateRefs(asset, spec, identity); len(residuals) > 0 {
		return fmt.Errorf("template identity references remain after rewrite: %s", strings.Join(residuals, ", "))
	}
	return nil
}

func validateWidgetInitBlueprintShape(asset *uasset.Asset) error {
	bpExports := findWidgetBlueprintExports(asset)
	if len(bpExports) != 1 {
		return fmt.Errorf("expected exactly one WidgetBlueprint export, got %d", len(bpExports))
	}
	if _, err := resolveWidgetAddRootContext(asset, 0, "CanvasPanel"); err != nil {
		return fmt.Errorf("expected an empty-root WidgetBlueprint template shape: %w", err)
	}
	return nil
}

func rewriteWidgetInitSummaryPackageName(asset *uasset.Asset, packageName string) ([]byte, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	raw := asset.Raw.Bytes
	reader := uasset.NewByteReaderWithByteSwapping(raw, asset.Summary.UsesByteSwappedSerialization())
	legacyVersion, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read summary legacy version: %w", err)
	}
	if legacyVersion != -8 && legacyVersion != -7 {
		if _, err := reader.ReadInt32(); err != nil {
			return nil, fmt.Errorf("read summary legacy UE3 version: %w", err)
		}
	}
	if legacyVersion != -4 {
		if _, err := reader.ReadInt32(); err != nil {
			return nil, fmt.Errorf("read summary file version UE4 tag: %w", err)
		}
	}
	fileUE4, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read summary file version UE4: %w", err)
	}
	fileUE5, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read summary file version UE5: %w", err)
	}
	fileLicensee, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read summary file version licensee: %w", err)
	}
	if fileUE4 == 0 && fileUE5 == 0 && fileLicensee == 0 {
		fileUE5 = asset.Summary.FileVersionUE5
	}
	if fileUE5 >= ue5PackageSavedHash {
		if err := reader.Skip(20); err != nil {
			return nil, fmt.Errorf("skip summary saved hash: %w", err)
		}
		if _, err := reader.ReadInt32(); err != nil {
			return nil, fmt.Errorf("read summary total header size: %w", err)
		}
	}
	if legacyVersion <= -2 {
		if err := skipSummaryCustomVersionsForRewrite(reader, legacyVersion); err != nil {
			return nil, fmt.Errorf("skip summary custom versions: %w", err)
		}
	}
	if fileUE5 < ue5PackageSavedHash {
		if _, err := reader.ReadInt32(); err != nil {
			return nil, fmt.Errorf("read summary total header size: %w", err)
		}
	}

	start := reader.Offset()
	if _, err := reader.ReadFString(); err != nil {
		return nil, fmt.Errorf("read summary package name: %w", err)
	}
	end := reader.Offset()
	replacement := appendFStringOrdered(nil, packageName, packageByteOrder(asset))
	outBytes, err := edit.RewriteRawRange(asset, int64(start), int64(end), replacement)
	if err != nil {
		return nil, fmt.Errorf("rewrite summary package name: %w", err)
	}
	return outBytes, nil
}

func rewriteWidgetInitAssetRegistryStringReplace(asset *uasset.Asset, from, to string) ([]byte, int, error) {
	if asset == nil {
		return nil, 0, fmt.Errorf("asset is nil")
	}
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == "" || from == to {
		return append([]byte(nil), asset.Raw.Bytes...), 0, nil
	}

	section, sectionStart, sectionEnd, err := parseAssetRegistrySection(asset)
	if err != nil {
		return nil, 0, err
	}
	if section == nil {
		return append([]byte(nil), asset.Raw.Bytes...), 0, nil
	}

	changed := false
	changeCount := 0
	for objIdx := range section.Objects {
		obj := &section.Objects[objIdx]
		if replaced, count := replaceAllWithCount(obj.ObjectPath, from, to); count > 0 {
			obj.ObjectPath = replaced
			changeCount += count
			changed = true
		}
		if replaced, count := replaceAllWithCount(obj.ObjectClass, from, to); count > 0 {
			obj.ObjectClass = replaced
			changeCount += count
			changed = true
		}
		for tagIdx := range obj.Tags {
			tag := &obj.Tags[tagIdx]
			var nextValue string
			var count int
			switch {
			case strings.EqualFold(tag.Key, assetRegistryTagFindInBlueprintsData), strings.EqualFold(tag.Key, assetRegistryTagUnversionedFindInBlueprintsData):
				nextValue, count, err = rewriteFiBTagValue(asset, tag.Value, func(history map[string]any) int {
					return replaceHistoryStrings(history, from, to)
				})
				if err != nil {
					return nil, 0, fmt.Errorf("rewrite asset registry tag %s on %s: %w", tag.Key, obj.ObjectPath, err)
				}
			default:
				nextValue, count = rewritePlainAssetRegistryTagValue(tag.Value, func(history map[string]any) int {
					return replaceHistoryStrings(history, from, to)
				})
			}
			if count == 0 || nextValue == tag.Value {
				continue
			}
			tag.Value = nextValue
			changeCount += count
			changed = true
		}
	}
	if !changed {
		return append([]byte(nil), asset.Raw.Bytes...), 0, nil
	}

	newSection, err := encodeAssetRegistrySection(asset, section, sectionStart, 0, false)
	if err != nil {
		return nil, 0, err
	}
	outBytes, err := edit.RewriteRawRange(asset, sectionStart, sectionEnd, newSection)
	if err != nil {
		return nil, 0, fmt.Errorf("rewrite asset registry section: %w", err)
	}
	return outBytes, changeCount, nil
}

func findWidgetInitResidualTemplateRefs(asset *uasset.Asset, spec widgetInitTemplateSpec, identity widgetInitIdentity) []string {
	if asset == nil {
		return []string{"asset=nil"}
	}
	residuals := make([]string, 0, 4)
	section, _, _, _ := parseAssetRegistrySection(asset)
	checks := []struct {
		needle string
		active bool
	}{
		{needle: spec.TemplatePackage, active: spec.TemplatePackage != identity.LongPackageName},
		{needle: spec.TemplateAsset + "_C", active: spec.TemplateAsset != identity.AssetName},
		{needle: "Default__" + spec.TemplateAsset + "_C", active: spec.TemplateAsset != identity.AssetName},
		{needle: "SKEL_" + spec.TemplateAsset + "_C", active: spec.TemplateAsset != identity.AssetName},
		{needle: spec.TemplateAsset, active: spec.TemplateAsset != identity.AssetName},
	}
	seen := make(map[string]struct{}, len(checks))
	for _, check := range checks {
		if !check.active || strings.TrimSpace(check.needle) == "" {
			continue
		}
		if strings.Contains(asset.Summary.PackageName, check.needle) {
			if _, ok := seen[check.needle]; !ok {
				residuals = append(residuals, check.needle)
				seen[check.needle] = struct{}{}
			}
			continue
		}
		for _, entry := range asset.Names {
			if !strings.Contains(entry.Value, check.needle) {
				continue
			}
			if _, ok := seen[check.needle]; ok {
				break
			}
			residuals = append(residuals, check.needle)
			seen[check.needle] = struct{}{}
			break
		}
		if _, ok := seen[check.needle]; ok || section == nil {
			continue
		}
		for _, obj := range section.Objects {
			if strings.Contains(obj.ObjectPath, check.needle) || strings.Contains(obj.ObjectClass, check.needle) {
				residuals = append(residuals, check.needle)
				seen[check.needle] = struct{}{}
				break
			}
			found := false
			for _, tag := range obj.Tags {
				if !strings.Contains(tag.Value, check.needle) {
					continue
				}
				residuals = append(residuals, check.needle)
				seen[check.needle] = struct{}{}
				found = true
				break
			}
			if found {
				break
			}
		}
	}
	slices.Sort(residuals)
	return residuals
}
