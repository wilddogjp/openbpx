package cli

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"github.com/wilddogjp/openbpx/pkg/edit"
	"github.com/wilddogjp/openbpx/pkg/uasset"
)

type widgetParentClassSpec struct {
	Raw         string
	PackagePath string
	ClassName   string
}

func runBlueprintWidgetParentClass(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("blueprint widget-parent-class", stderr)
	opts := registerCommonFlags(fs)
	parentClassRaw := fs.String("class", "", "compiled parent class path like /Script/CommonUI.CommonActivatableWidget or /Script/LyraGame.LyraActivatableWidget")
	exportIndex := fs.Int("export", 0, "optional 1-based WidgetBlueprint export index")
	dryRun := fs.Bool("dry-run", false, "do not write output")
	backup := fs.Bool("backup", false, "create <file>.backup before overwrite")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || strings.TrimSpace(*parentClassRaw) == "" {
		fmt.Fprintln(stderr, "usage: bpx blueprint widget-parent-class <file.uasset> --class </Script/Module.ClassName> [--export <n>] [--dry-run] [--backup]")
		return 1
	}

	spec, err := parseWidgetParentClassSpec(*parentClassRaw)
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

	ctx, err := resolveWidgetAddRootContext(asset, *exportIndex, "CanvasPanel")
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	outBytes, updatedAsset, addedNames, addedImports, oldParentClass, err := rewriteWidgetBlueprintParentClass(asset, *opts, ctx.Target.BlueprintExport, ctx.GeneratedClassExport, spec)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	changed := !bytesEqualLocal(asset.Raw.Bytes, outBytes)

	if _, err := resolveWidgetAddRootContext(updatedAsset, ctx.Target.BlueprintExport+1, "CanvasPanel"); err != nil {
		fmt.Fprintf(stderr, "error: validate rootless widget parent-class result: %v\n", err)
		return 1
	}
	if err := validateWidgetBlueprintParentClassResult(updatedAsset, ctx.Target.BlueprintExport, ctx.GeneratedClassExport, spec); err != nil {
		fmt.Fprintf(stderr, "error: validate widget parent-class result: %v\n", err)
		return 1
	}

	resp := map[string]any{
		"file":         file,
		"export":       ctx.Target.BlueprintExport + 1,
		"class":        spec.Raw,
		"dryRun":       *dryRun,
		"changed":      changed,
		"outputBytes":  len(outBytes),
		"addedNames":   addedNames,
		"addedImports": addedImports,
		"updates": []map[string]any{
			{
				"path":     "export.WidgetBlueprint.ParentClass",
				"oldValue": oldParentClass,
				"newValue": spec.Raw,
			},
			{
				"path":     "export.GeneratedClass.SuperIndex",
				"oldValue": oldParentClass,
				"newValue": spec.Raw,
			},
		},
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

func parseWidgetParentClassSpec(raw string) (widgetParentClassSpec, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return widgetParentClassSpec{}, fmt.Errorf("parent class path is required")
	}
	if !strings.HasPrefix(trimmed, "/Script/") {
		return widgetParentClassSpec{}, fmt.Errorf("parent class must be a compiled /Script class path like /Script/CommonUI.CommonActivatableWidget or /Script/LyraGame.LyraActivatableWidget")
	}
	slash := strings.LastIndexByte(trimmed, '/')
	dot := strings.LastIndexByte(trimmed, '.')
	if dot <= slash || dot == len(trimmed)-1 {
		return widgetParentClassSpec{}, fmt.Errorf("parent class must be a compiled /Script class path like /Script/CommonUI.CommonActivatableWidget or /Script/LyraGame.LyraActivatableWidget")
	}
	packagePath := trimmed[:dot]
	className := trimmed[dot+1:]
	if packagePath == "" || className == "" {
		return widgetParentClassSpec{}, fmt.Errorf("parent class must be a compiled /Script class path like /Script/CommonUI.CommonActivatableWidget or /Script/LyraGame.LyraActivatableWidget")
	}
	if strings.Contains(className, "/") || strings.HasSuffix(className, "_C") {
		return widgetParentClassSpec{}, fmt.Errorf("parent class must be a compiled /Script class path like /Script/CommonUI.CommonActivatableWidget or /Script/LyraGame.LyraActivatableWidget")
	}
	return widgetParentClassSpec{
		Raw:         trimmed,
		PackagePath: packagePath,
		ClassName:   className,
	}, nil
}

func rewriteWidgetBlueprintParentClass(asset *uasset.Asset, opts uasset.ParseOptions, blueprintExport, generatedClassExport int, spec widgetParentClassSpec) ([]byte, *uasset.Asset, []string, []map[string]any, string, error) {
	if asset == nil {
		return nil, nil, nil, nil, "", fmt.Errorf("asset is nil")
	}
	currentParentClass, err := readWidgetBlueprintParentClassPath(asset, blueprintExport)
	if err != nil {
		return nil, nil, nil, nil, "", err
	}

	currentSpec, err := parseWidgetParentClassSpec(currentParentClass)
	if err != nil {
		return nil, nil, nil, nil, "", fmt.Errorf("parse current parent class: %w", err)
	}

	workingAsset := asset
	addedNames := make([]string, 0, 8)
	addedImports := make([]map[string]any, 0, 4)

	_, workingAsset, renamedDefaultObject, err := rewriteWidgetParentClassDefaultObjectName(workingAsset, opts, "Default__"+currentSpec.ClassName, "Default__"+spec.ClassName)
	if err != nil {
		return nil, nil, nil, nil, "", fmt.Errorf("rewrite parent default object name: %w", err)
	}
	if renamedDefaultObject {
		addedNames = append(addedNames, "Default__"+spec.ClassName)
	}

	prefixNames := []string{
		"PaletteCategory",
		"TextProperty",
	}
	suffixNames := []string{
		spec.PackagePath,
		spec.ClassName,
	}
	if !renamedDefaultObject {
		suffixNames = append(suffixNames, "Default__"+spec.ClassName)
	}
	_, workingAsset, ensuredNames, err := insertBrushImageNames(workingAsset, opts, prefixNames, suffixNames)
	if err != nil {
		return nil, nil, nil, nil, "", fmt.Errorf("ensure widget parent-class names: %w", err)
	}
	addedNames = append(addedNames, ensuredNames...)

	_, workingAsset, importResults, err := ensureWidgetParentClassImports(workingAsset, opts, currentSpec, spec)
	if err != nil {
		return nil, nil, nil, nil, "", fmt.Errorf("ensure widget parent-class import: %w", err)
	}
	addedImports = append(addedImports, importResults...)
	classImportIdx, found := findNativeClassImport(workingAsset, spec.PackagePath, spec.ClassName)
	if !found || classImportIdx <= 0 {
		return nil, nil, nil, nil, "", fmt.Errorf("native class import %q not found after insertion", spec.Raw)
	}
	oldClassImportIdx, _ := findNativeClassImport(workingAsset, currentSpec.PackagePath, currentSpec.ClassName)

	_, workingAsset, err = applyWidgetAddPropertyWrite(workingAsset, blueprintExport, "ParentClass", "ObjectProperty", map[string]any{"index": int32(-classImportIdx)}, "GeneratedClass")
	if err != nil {
		return nil, nil, nil, nil, "", fmt.Errorf("write WidgetBlueprint ParentClass: %w", err)
	}
	_, workingAsset, err = rewriteExportSuperIndex(workingAsset, generatedClassExport, uasset.PackageIndex(-classImportIdx))
	if err != nil {
		return nil, nil, nil, nil, "", fmt.Errorf("rewrite generated class super index: %w", err)
	}
	if oldClassImportIdx > 0 && oldClassImportIdx != classImportIdx {
		_, workingAsset, err = rewriteWidgetParentClassGeneratedClassSerializedImportIndex(workingAsset, opts, generatedClassExport, uasset.PackageIndex(-oldClassImportIdx), uasset.PackageIndex(-classImportIdx))
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("rewrite generated class serialized parent import index: %w", err)
		}
	}
	_, workingAsset, err = ensureWidgetParentClassCDOPaletteCategory(workingAsset, opts)
	if err != nil {
		return nil, nil, nil, nil, "", fmt.Errorf("ensure widget parent-class CDO palette category: %w", err)
	}
	if (workingAsset.Summary.PackageFlags & packageFlagRequiresLoc) == 0 {
		workingBytes, _, err := rewritePackageFlags(workingAsset, workingAsset.Summary.PackageFlags|packageFlagRequiresLoc)
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("set package localization flag: %w", err)
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, opts)
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("reparse after package flag rewrite: %w", err)
		}
	}
	oldRegistryClassPath := widgetParentClassAssetRegistryValue(currentSpec)
	newRegistryClassPath := widgetParentClassAssetRegistryValue(spec)
	workingBytes, _, err := rewriteWidgetParentClassAssetRegistryTags(workingAsset, oldRegistryClassPath, newRegistryClassPath)
	if err != nil {
		return nil, nil, nil, nil, "", fmt.Errorf("rewrite widget parent-class asset registry: %w", err)
	}
	workingAsset, err = uasset.ParseBytes(workingBytes, opts)
	if err != nil {
		return nil, nil, nil, nil, "", fmt.Errorf("reparse after widget parent-class asset registry rewrite: %w", err)
	}
	if oldClassImportIdx > 0 && oldClassImportIdx != classImportIdx {
		_, workingAsset, err = rewriteWidgetAddAssetRegistryImportedClassBits(workingAsset, nil, []int{oldClassImportIdx})
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("rewrite widget parent-class imported class bits: %w", err)
		}
	}
	workingBytes, workingAsset, err = finalizeWidgetBlueprintMutation(asset, workingAsset, opts, asset.Exports[blueprintExport].ObjectName.Display(asset.Names))
	if err != nil {
		return nil, nil, nil, nil, "", fmt.Errorf("finalize widget parent-class mutation: %w", err)
	}
	return workingBytes, workingAsset, addedNames, addedImports, currentParentClass, nil
}

func ensureWidgetParentClassImports(asset *uasset.Asset, opts uasset.ParseOptions, currentSpec, targetSpec widgetParentClassSpec) ([]byte, *uasset.Asset, []map[string]any, error) {
	workingAsset := asset
	workingBytes := append([]byte(nil), asset.Raw.Bytes...)
	results := make([]map[string]any, 0, 4)

	if removeIdx, found := findParentDefaultObjectImport(workingAsset, currentSpec, ""); found {
		var err error
		workingBytes, err = edit.RemoveImportEntries(workingAsset, []int{removeIdx - 1})
		if err != nil {
			return nil, nil, nil, fmt.Errorf("remove old parent default object import: %w", err)
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, opts)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("reparse after removing old parent default object import: %w", err)
		}
	}

	packageImportIndex, packageExists := findPackageImportByObjectPath(workingAsset, targetSpec.PackagePath)
	if !packageExists {
		entry, err := buildImportAddEntry(workingAsset, "/Script/CoreUObject", "Package", 0, targetSpec.PackagePath)
		if err != nil {
			return nil, nil, nil, err
		}
		insertAt := preferredSortedPackageImportInsertPos(workingAsset, targetSpec.PackagePath)
		workingBytes, err = edit.InsertImportEntries(workingAsset, insertAt, []uasset.ImportEntry{entry})
		if err != nil {
			return nil, nil, nil, fmt.Errorf("insert script package import %s: %w", targetSpec.PackagePath, err)
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, opts)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("reparse after script package import %s: %w", targetSpec.PackagePath, err)
		}
		results = append(results, importAddResult(workingAsset, insertAt, true))
		packageImportIndex = insertAt + 1
	}

	if idx, found := findNativeClassImport(workingAsset, targetSpec.PackagePath, targetSpec.ClassName); found {
		results = append(results, importAddResult(workingAsset, idx-1, false))
	} else {
		insertAt := preferredScriptClassImportInsertPos(workingAsset, targetSpec.PackagePath, targetSpec.ClassName)
		entry, err := buildImportAddEntry(workingAsset, "/Script/CoreUObject", "Class", uasset.PackageIndex(-packageImportIndex), targetSpec.ClassName)
		if err != nil {
			return nil, nil, nil, err
		}
		workingBytes, err = edit.InsertImportEntries(workingAsset, insertAt, []uasset.ImportEntry{entry})
		if err != nil {
			return nil, nil, nil, fmt.Errorf("insert native class import %s: %w", targetSpec.ClassName, err)
		}
		workingAsset, err = uasset.ParseBytes(workingBytes, opts)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("reparse after native class import %s: %w", targetSpec.ClassName, err)
		}
		results = append(results, importAddResult(workingAsset, insertAt, true))
	}

	defaultObjectName := "Default__" + targetSpec.ClassName
	if idx, found := findParentDefaultObjectImport(workingAsset, targetSpec, "Default__"+targetSpec.ClassName); found {
		results = append(results, importAddResult(workingAsset, idx-1, false))
		return workingBytes, workingAsset, results, nil
	}

	packageImportIndex, packageExists = findPackageImportByObjectPath(workingAsset, targetSpec.PackagePath)
	if !packageExists || packageImportIndex <= 0 {
		return nil, nil, nil, fmt.Errorf("script package import %q not found before default object insert", targetSpec.PackagePath)
	}
	entry, err := buildImportAddEntry(workingAsset, targetSpec.PackagePath, targetSpec.ClassName, uasset.PackageIndex(-packageImportIndex), defaultObjectName)
	if err != nil {
		return nil, nil, nil, err
	}
	insertAt := preferredLeadingObjectImportInsertPos(workingAsset)
	workingBytes, err = edit.InsertImportEntries(workingAsset, insertAt, []uasset.ImportEntry{entry})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("insert parent default object import %s: %w", defaultObjectName, err)
	}
	workingAsset, err = uasset.ParseBytes(workingBytes, opts)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("reparse after parent default object import %s: %w", defaultObjectName, err)
	}
	results = append(results, importAddResult(workingAsset, insertAt, true))
	return workingBytes, workingAsset, results, nil
}

func rewriteWidgetParentClassDefaultObjectName(asset *uasset.Asset, opts uasset.ParseOptions, oldValue, newValue string) ([]byte, *uasset.Asset, bool, error) {
	if asset == nil {
		return nil, nil, false, fmt.Errorf("asset is nil")
	}
	oldIdx := findNameIndexByValue(asset.Names, oldValue)
	if oldIdx < 0 || findNameIndexByValue(asset.Names, newValue) >= 0 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, false, nil
	}
	updatedNames := append([]uasset.NameEntry(nil), asset.Names...)
	nonCase, casePreserving := edit.ComputeNameEntryHashesUE56(newValue)
	updatedNames[oldIdx] = uasset.NameEntry{
		Value:              newValue,
		NonCaseHash:        nonCase,
		CasePreservingHash: casePreserving,
	}
	outBytes, err := edit.RewriteNameMap(asset, updatedNames)
	if err != nil {
		return nil, nil, false, err
	}
	updatedAsset, err := uasset.ParseBytes(outBytes, opts)
	if err != nil {
		return nil, nil, false, err
	}
	return outBytes, updatedAsset, true, nil
}

func preferredSortedPackageImportInsertPos(asset *uasset.Asset, packagePath string) int {
	target := strings.ToLower(strings.TrimSpace(packagePath))
	lastPackage := -1
	for i, imp := range asset.Imports {
		if !strings.EqualFold(imp.ClassName.Display(asset.Names), "Package") {
			continue
		}
		current := strings.ToLower(strings.TrimSpace(imp.ObjectName.Display(asset.Names)))
		if current > target {
			return i
		}
		lastPackage = i
	}
	if lastPackage >= 0 {
		return lastPackage + 1
	}
	return len(asset.Imports)
}

func preferredLeadingObjectImportInsertPos(asset *uasset.Asset) int {
	for i, imp := range asset.Imports {
		if strings.EqualFold(imp.ClassName.Display(asset.Names), "Package") {
			return i
		}
	}
	return len(asset.Imports)
}

func findParentDefaultObjectImport(asset *uasset.Asset, spec widgetParentClassSpec, objectName string) (int, bool) {
	for i, imp := range asset.Imports {
		if !strings.EqualFold(strings.TrimSpace(imp.ClassPackage.Display(asset.Names)), strings.TrimSpace(spec.PackagePath)) {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(imp.ClassName.Display(asset.Names)), strings.TrimSpace(spec.ClassName)) {
			continue
		}
		if strings.TrimSpace(objectName) != "" && !strings.EqualFold(strings.TrimSpace(imp.ObjectName.Display(asset.Names)), strings.TrimSpace(objectName)) {
			continue
		}
		if imp.OuterIndex >= 0 {
			continue
		}
		outerIdx := imp.OuterIndex.ResolveIndex()
		if outerIdx < 0 || outerIdx >= len(asset.Imports) {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(asset.Imports[outerIdx].ObjectName.Display(asset.Names)), strings.TrimSpace(spec.PackagePath)) {
			continue
		}
		return i + 1, true
	}
	return 0, false
}

func ensureWidgetParentClassCDOPaletteCategory(asset *uasset.Asset, opts uasset.ParseOptions) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	cdoExport, err := findCDOExportIndex(asset)
	if err != nil {
		return nil, nil, err
	}
	value := map[string]any{
		"flags":                  int32(8),
		"historyType":            "Base",
		"historyTypeCode":        uint8(0),
		"namespace":              "UMG",
		"key":                    "UserCreated",
		"sourceString":           "User Created",
		"value":                  "User Created",
		"cultureInvariantString": "User Created",
		"displayString":          "User Created",
	}
	return applyWidgetAddPropertyWrite(asset, cdoExport, "PaletteCategory", "TextProperty", value, "bHasScriptImplementedTick")
}

func widgetParentClassAssetRegistryValue(spec widgetParentClassSpec) string {
	return fmt.Sprintf("/Script/CoreUObject.Class'%s.%s'", spec.PackagePath, spec.ClassName)
}

func rewriteWidgetParentClassGeneratedClassSerializedImportIndex(asset *uasset.Asset, opts uasset.ParseOptions, exportIndex int, oldIndex, newIndex uasset.PackageIndex) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	if exportIndex < 0 || exportIndex >= len(asset.Exports) {
		return nil, nil, fmt.Errorf("export index out of range: %d", exportIndex+1)
	}
	exp := asset.Exports[exportIndex]
	start := int(exp.SerialOffset)
	end := start + int(exp.SerialSize)
	if start < 0 || end > len(asset.Raw.Bytes) || start > end {
		return nil, nil, fmt.Errorf("generated class serial range out of bounds: %d..%d", start, end)
	}

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}

	out := append([]byte(nil), asset.Raw.Bytes...)
	oldRaw := uint32(int32(oldIndex))
	newRaw := uint32(int32(newIndex))
	changed := false
	for pos := start; pos+4 <= end; pos++ {
		if order.Uint32(out[pos:pos+4]) != oldRaw {
			continue
		}
		order.PutUint32(out[pos:pos+4], newRaw)
		changed = true
	}
	if !changed {
		return out, asset, nil
	}
	if err := edit.FinalizePackageBytes(out, asset.Summary.FileVersionUE5); err != nil {
		return nil, nil, fmt.Errorf("finalize package bytes: %w", err)
	}
	updatedAsset, err := uasset.ParseBytes(out, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("reparse rewritten asset: %w", err)
	}
	return out, updatedAsset, nil
}

func rewriteWidgetParentClassAssetRegistryTags(asset *uasset.Asset, oldValue, newValue string) ([]byte, int, error) {
	if asset == nil {
		return nil, 0, fmt.Errorf("asset is nil")
	}
	oldValue = strings.TrimSpace(oldValue)
	newValue = strings.TrimSpace(newValue)
	if oldValue == "" || oldValue == newValue {
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
		for tagIdx := range obj.Tags {
			tag := &obj.Tags[tagIdx]
			if !strings.EqualFold(tag.Key, "NativeParentClass") && !strings.EqualFold(tag.Key, "ParentClass") {
				continue
			}
			if strings.TrimSpace(tag.Value) != oldValue {
				continue
			}
			tag.Value = newValue
			changeCount++
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

func findNativeClassImport(asset *uasset.Asset, packagePath, className string) (int, bool) {
	for i, imp := range asset.Imports {
		if !strings.EqualFold(imp.ClassPackage.Display(asset.Names), "/Script/CoreUObject") {
			continue
		}
		if !strings.EqualFold(imp.ClassName.Display(asset.Names), "Class") {
			continue
		}
		if !strings.EqualFold(imp.ObjectName.Display(asset.Names), className) {
			continue
		}
		if imp.OuterIndex >= 0 {
			continue
		}
		outerIdx := imp.OuterIndex.ResolveIndex()
		if outerIdx < 0 || outerIdx >= len(asset.Imports) {
			continue
		}
		if !strings.EqualFold(asset.Imports[outerIdx].ObjectName.Display(asset.Names), packagePath) {
			continue
		}
		return i + 1, true
	}
	return 0, false
}

func preferredScriptClassImportInsertPos(asset *uasset.Asset, packagePath, className string) int {
	targetPackage := strings.ToLower(strings.TrimSpace(packagePath))
	targetClass := strings.ToLower(strings.TrimSpace(className))
	lastScriptClass := -1
	for i, imp := range asset.Imports {
		if !strings.EqualFold(imp.ClassPackage.Display(asset.Names), "/Script/CoreUObject") {
			continue
		}
		if !strings.EqualFold(imp.ClassName.Display(asset.Names), "Class") {
			continue
		}
		if imp.OuterIndex >= 0 {
			continue
		}
		outerIdx := imp.OuterIndex.ResolveIndex()
		if outerIdx < 0 || outerIdx >= len(asset.Imports) {
			continue
		}
		currentPackage := strings.ToLower(strings.TrimSpace(asset.Imports[outerIdx].ObjectName.Display(asset.Names)))
		currentClass := strings.ToLower(strings.TrimSpace(imp.ObjectName.Display(asset.Names)))
		if currentPackage > targetPackage || (currentPackage == targetPackage && currentClass > targetClass) {
			return i
		}
		lastScriptClass = i
	}
	if lastScriptClass >= 0 {
		return lastScriptClass + 1
	}
	return len(asset.Imports)
}

func readWidgetBlueprintParentClassPath(asset *uasset.Asset, blueprintExport int) (string, error) {
	decoded, err := decodeExportRootPropertyValue(asset, blueprintExport, "ParentClass")
	if err != nil {
		return "", fmt.Errorf("read WidgetBlueprint ParentClass: %w", err)
	}
	ref, ok := decoded.(map[string]any)
	if !ok {
		return "", fmt.Errorf("WidgetBlueprint ParentClass is not an object reference")
	}
	indexValue, ok := extractIntLike(ref["index"])
	if !ok || indexValue >= 0 {
		return "", fmt.Errorf("WidgetBlueprint ParentClass is not an import reference")
	}
	importIdx := -indexValue
	return nativeClassPathFromImportIndex(asset, importIdx)
}

func nativeClassPathFromImportIndex(asset *uasset.Asset, importIndex int) (string, error) {
	if asset == nil {
		return "", fmt.Errorf("asset is nil")
	}
	zeroIdx := importIndex - 1
	if zeroIdx < 0 || zeroIdx >= len(asset.Imports) {
		return "", fmt.Errorf("import index out of range: %d", importIndex)
	}
	imp := asset.Imports[zeroIdx]
	if imp.OuterIndex >= 0 {
		return "", fmt.Errorf("import %d is not a top-level class import", importIndex)
	}
	outerIdx := imp.OuterIndex.ResolveIndex()
	if outerIdx < 0 || outerIdx >= len(asset.Imports) {
		return "", fmt.Errorf("import %d outer index is out of range", importIndex)
	}
	return asset.Imports[outerIdx].ObjectName.Display(asset.Names) + "." + imp.ObjectName.Display(asset.Names), nil
}

func rewriteExportSuperIndex(asset *uasset.Asset, exportIndex int, superIndex uasset.PackageIndex) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	if exportIndex < 0 || exportIndex >= len(asset.Exports) {
		return nil, nil, fmt.Errorf("export index out of range: %d", exportIndex+1)
	}
	positions, err := scanExportHeaderPositions(asset)
	if err != nil {
		return nil, nil, err
	}
	if len(positions) != len(asset.Exports) {
		return nil, nil, fmt.Errorf("export header position mismatch")
	}
	pos := positions[exportIndex]
	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}
	out := append([]byte(nil), asset.Raw.Bytes...)
	order.PutUint32(out[pos.superIndex:pos.superIndex+4], uint32(int32(superIndex)))
	if err := edit.FinalizePackageBytes(out, asset.Summary.FileVersionUE5); err != nil {
		return nil, nil, fmt.Errorf("finalize package bytes: %w", err)
	}
	updatedAsset, err := uasset.ParseBytes(out, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		return nil, nil, fmt.Errorf("reparse rewritten asset: %w", err)
	}
	return out, updatedAsset, nil
}

func validateWidgetBlueprintParentClassResult(asset *uasset.Asset, blueprintExport, generatedClassExport int, spec widgetParentClassSpec) error {
	if asset == nil {
		return fmt.Errorf("asset is nil")
	}
	parentClassPath, err := readWidgetBlueprintParentClassPath(asset, blueprintExport)
	if err != nil {
		return err
	}
	if parentClassPath != spec.Raw {
		return fmt.Errorf("WidgetBlueprint ParentClass mismatch: got %q want %q", parentClassPath, spec.Raw)
	}
	if generatedClassExport < 0 || generatedClassExport >= len(asset.Exports) {
		return fmt.Errorf("generated class export out of range: %d", generatedClassExport+1)
	}
	classImportIndex, found := findNativeClassImport(asset, spec.PackagePath, spec.ClassName)
	if !found || classImportIndex <= 0 {
		return fmt.Errorf("native class import %q not found after rewrite", spec.Raw)
	}
	if got := asset.Exports[generatedClassExport].SuperIndex; got != uasset.PackageIndex(-classImportIndex) {
		return fmt.Errorf("generated class super index mismatch: got %s want import %s", asset.ParseIndex(got), spec.Raw)
	}
	return nil
}

func bytesEqualLocal(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
