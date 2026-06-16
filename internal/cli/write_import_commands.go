package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/wilddogjp/openbpx/pkg/edit"
	"github.com/wilddogjp/openbpx/pkg/uasset"
)

func runImportAdd(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("import add", stderr)
	opts := registerCommonFlags(fs)
	texturePath := fs.String("texture", "", "Texture2D package path like /Game/UI/T_MyTexture")
	dryRun := fs.Bool("dry-run", false, "do not write output")
	backup := fs.Bool("backup", false, "create <file>.backup before overwrite")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || strings.TrimSpace(*texturePath) == "" {
		fmt.Fprintln(stderr, "usage: bpx import add <file.uasset> --texture </Game/Path/TextureName> [--dry-run] [--backup]")
		return 1
	}

	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	outBytes, updatedAsset, addedNames, addedImports, changed, err := appendTextureImport(asset, *opts, *texturePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	resp := map[string]any{
		"file":         file,
		"texture":      strings.TrimSpace(*texturePath),
		"addedNames":   addedNames,
		"addedImports": addedImports,
		"changed":      changed,
		"dryRun":       *dryRun,
		"importCount":  len(updatedAsset.Imports),
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
	if err := writeFileAtomically(file, outBytes, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write file: %v\n", err)
		return 1
	}
	return printJSON(stdout, resp)
}

func appendTextureImport(asset *uasset.Asset, opts uasset.ParseOptions, texturePath string) ([]byte, *uasset.Asset, []string, []map[string]any, bool, error) {
	if asset == nil {
		return nil, nil, nil, nil, false, fmt.Errorf("asset is nil")
	}
	packagePath, objectName, ok := parseAssetPackagePath(texturePath)
	if !ok {
		return nil, nil, nil, nil, false, fmt.Errorf("texture path must be a full package path like /Game/UI/T_MyTexture")
	}
	return appendObjectImport(asset, opts, packagePath, objectName, "/Script/Engine", "Texture2D")
}

func buildImportAddEntry(asset *uasset.Asset, classPackage, className string, outerIndex uasset.PackageIndex, objectName string) (uasset.ImportEntry, error) {
	classPackageIdx := findNameIndexByValue(asset.Names, classPackage)
	classNameIdx := findNameIndexByValue(asset.Names, className)
	objectNameIdx := findNameIndexByValue(asset.Names, objectName)
	noneIdx := findNameIndexByValue(asset.Names, "None")
	if classPackageIdx < 0 || classNameIdx < 0 || objectNameIdx < 0 || noneIdx < 0 {
		return uasset.ImportEntry{}, fmt.Errorf("required import-add names are missing from NameMap")
	}
	return uasset.ImportEntry{
		ClassPackage: uasset.NameRef{Index: int32(classPackageIdx)},
		ClassName:    uasset.NameRef{Index: int32(classNameIdx)},
		OuterIndex:   outerIndex,
		ObjectName:   uasset.NameRef{Index: int32(objectNameIdx)},
		PackageName:  uasset.NameRef{Index: int32(noneIdx)},
	}, nil
}

func findPackageImportByObjectPath(asset *uasset.Asset, packagePath string) (int, bool) {
	for i, imp := range asset.Imports {
		if !strings.EqualFold(imp.ClassName.Display(asset.Names), "Package") {
			continue
		}
		if strings.EqualFold(imp.ObjectName.Display(asset.Names), packagePath) {
			return i + 1, true
		}
	}
	return 0, false
}

func findTextureImportByPath(asset *uasset.Asset, packagePath string) (int, bool) {
	return findObjectImportByPath(asset, "/Script/Engine", "Texture2D", packagePath)
}

func findFontImportByPath(asset *uasset.Asset, packagePath string) (int, bool) {
	return findObjectImportByPath(asset, "/Script/Engine", "Font", packagePath)
}

func findDataTableImportByPath(asset *uasset.Asset, packagePath string) (int, bool) {
	return findObjectImportByPath(asset, "/Script/Engine", "DataTable", packagePath)
}

func findBlueprintGeneratedClassImportByPath(asset *uasset.Asset, packagePath string) (int, bool) {
	return findObjectImportByPath(asset, "/Script/Engine", "BlueprintGeneratedClass", packagePath)
}

func findObjectImportByPath(asset *uasset.Asset, classPackage, className, packagePath string) (int, bool) {
	for i, imp := range asset.Imports {
		if !objectImportMatchesClass(asset, imp, classPackage, className) {
			continue
		}
		if importTargetsPackagePath(asset, imp, packagePath) {
			return i + 1, true
		}
	}
	return 0, false
}

func findObjectImportByObjectName(asset *uasset.Asset, classPackage, className, objectName string) (int, bool) {
	for i, imp := range asset.Imports {
		if !objectImportMatchesClass(asset, imp, classPackage, className) {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(imp.ObjectName.Display(asset.Names)), strings.TrimSpace(objectName)) {
			return i + 1, true
		}
	}
	return 0, false
}

func importTargetsPackagePath(asset *uasset.Asset, imp uasset.ImportEntry, packagePath string) bool {
	if strings.EqualFold(resolveImportTargetPath(asset, imp), packagePath) {
		return true
	}
	if asset == nil || imp.OuterIndex >= 0 {
		return false
	}
	outerImportIndex := int(-imp.OuterIndex)
	if outerImportIndex <= 0 || outerImportIndex > len(asset.Imports) {
		return false
	}
	outer := asset.Imports[outerImportIndex-1]
	if !strings.EqualFold(strings.TrimSpace(outer.ClassName.Display(asset.Names)), "Package") {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(outer.ObjectName.Display(asset.Names)), strings.TrimSpace(packagePath))
}

func importAddResult(asset *uasset.Asset, zeroIndex int, created bool) map[string]any {
	imp := asset.Imports[zeroIndex]
	return map[string]any{
		"index":         zeroIndex + 1,
		"created":       created,
		"classPackage":  imp.ClassPackage.Display(asset.Names),
		"className":     imp.ClassName.Display(asset.Names),
		"outerIndex":    int32(imp.OuterIndex),
		"outerResolved": asset.ParseIndex(imp.OuterIndex),
		"objectName":    imp.ObjectName.Display(asset.Names),
		"packageName":   imp.PackageName.Display(asset.Names),
		"targetPath":    resolveImportTargetPath(asset, imp),
	}
}

func preferredPackageImportInsertPos(asset *uasset.Asset) int {
	for i, imp := range asset.Imports {
		if strings.EqualFold(imp.ClassName.Display(asset.Names), "Package") {
			return i
		}
	}
	return len(asset.Imports)
}

func parseTexturePackagePath(selector string) (string, string, bool) {
	return parseAssetPackagePath(selector)
}

func parseAssetPackagePath(selector string) (string, string, bool) {
	trimmed := strings.TrimSpace(selector)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") {
		return "", "", false
	}
	packagePath := trimmed
	assetName := pathBaseName(trimmed)
	if dot := strings.LastIndexByte(trimmed, '.'); dot > strings.LastIndexByte(trimmed, '/') {
		packagePath = trimmed[:dot]
		assetName = trimmed[dot+1:]
	}
	if packagePath == "" || assetName == "" {
		return "", "", false
	}
	return packagePath, assetName, true
}

func pathBaseName(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if idx := strings.LastIndexByte(path, '/'); idx >= 0 && idx < len(path)-1 {
		return path[idx+1:]
	}
	return path
}

func textureImportIsSupported(asset *uasset.Asset, imp uasset.ImportEntry) bool {
	return objectImportMatchesClass(asset, imp, "/Script/Engine", "Texture2D")
}

func objectImportMatchesClass(asset *uasset.Asset, imp uasset.ImportEntry, classPackage, className string) bool {
	if asset == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(imp.ClassPackage.Display(asset.Names)), strings.TrimSpace(classPackage)) &&
		strings.EqualFold(strings.TrimSpace(imp.ClassName.Display(asset.Names)), strings.TrimSpace(className))
}

func preferredObjectImportInsertPos(asset *uasset.Asset) int {
	pos := len(asset.Imports)
	for i, imp := range asset.Imports {
		className := strings.TrimSpace(imp.ClassName.Display(asset.Names))
		switch {
		case strings.EqualFold(className, "Class"):
			continue
		case strings.EqualFold(className, "Package"), strings.EqualFold(className, "ScriptStruct"):
			pos = i + 1
		default:
			if pos < len(asset.Imports) {
				return pos
			}
			return i
		}
	}
	return pos
}

func appendObjectImport(asset *uasset.Asset, opts uasset.ParseOptions, packagePath, objectName, classPackage, className string) ([]byte, *uasset.Asset, []string, []map[string]any, bool, error) {
	if asset == nil {
		return nil, nil, nil, nil, false, fmt.Errorf("asset is nil")
	}

	requiredNames := []string{
		"None",
		"/Script/CoreUObject",
		"Package",
		classPackage,
		className,
		packagePath,
		objectName,
	}
	workingBytes, workingAsset, addedNames, err := ensureNameEntriesPresent(asset, opts, requiredNames)
	if err != nil {
		return nil, nil, nil, nil, false, fmt.Errorf("ensure import-add names: %w", err)
	}

	if idx, found := findObjectImportByPath(workingAsset, classPackage, className, packagePath); found {
		return workingBytes, workingAsset, addedNames, []map[string]any{
			importAddResult(workingAsset, idx-1, false),
		}, len(addedNames) > 0, nil
	}

	packageImportIndex, packageExists := findPackageImportByObjectPath(workingAsset, packagePath)
	updatedAsset := workingAsset
	results := make([]map[string]any, 0, 2)
	var outBytes []byte

	if !packageExists {
		entry, err := buildImportAddEntry(updatedAsset, "/Script/CoreUObject", "Package", 0, packagePath)
		if err != nil {
			return nil, nil, nil, nil, false, err
		}
		insertAt := preferredPackageImportInsertPos(updatedAsset)
		outBytes, err = edit.InsertImportEntries(updatedAsset, insertAt, []uasset.ImportEntry{entry})
		if err != nil {
			return nil, nil, nil, nil, false, fmt.Errorf("insert package import: %w", err)
		}
		updatedAsset, err = uasset.ParseBytes(outBytes, opts)
		if err != nil {
			return nil, nil, nil, nil, false, fmt.Errorf("reparse rewritten asset: %w", err)
		}
		packageImportIndex = insertAt + 1
		results = append(results, importAddResult(updatedAsset, insertAt, true))
	}

	objectOuter := uasset.PackageIndex(-packageImportIndex)
	objectEntry, err := buildImportAddEntry(updatedAsset, classPackage, className, objectOuter, objectName)
	if err != nil {
		return nil, nil, nil, nil, false, err
	}
	objectInsertAt := preferredObjectImportInsertPos(updatedAsset)
	outBytes, err = edit.InsertImportEntries(updatedAsset, objectInsertAt, []uasset.ImportEntry{objectEntry})
	if err != nil {
		return nil, nil, nil, nil, false, fmt.Errorf("insert %s import: %w", className, err)
	}
	updatedAsset, err = uasset.ParseBytes(outBytes, opts)
	if err != nil {
		return nil, nil, nil, nil, false, fmt.Errorf("reparse rewritten asset: %w", err)
	}
	results = append(results, importAddResult(updatedAsset, objectInsertAt, true))
	return outBytes, updatedAsset, addedNames, results, true, nil
}
