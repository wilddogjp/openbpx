package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wilddogjp/openbpx/pkg/edit"
	"github.com/wilddogjp/openbpx/pkg/uasset"
)

func TestEnsureNameEntriesPresentSortedKeepsWidgetBlueprintGeneratedClassParseable(t *testing.T) {
	t.Parallel()

	work := filepath.Join(t.TempDir(), "WBP_NameRemap.uasset")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-init", work,
		"--template", "minimum",
		"--asset-name", "WBP_NameRemap",
		"--package-path", "/Game/WBP",
		"--force",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("widget-init exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "root",
		"--type", "canvaspanel",
		"--name", "CanvasPanel_1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("root widget-add exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "CanvasPanel_1",
		"--type", "image",
		"--name", "Image_1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("image widget-add exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	opts := uasset.DefaultParseOptions()
	asset, err := uasset.ParseFile(work, opts)
	if err != nil {
		t.Fatalf("parse asset before ensure: %v", err)
	}

	layout := widgetAnchorLayoutData{
		Left:   64,
		Top:    48,
		Right:  256,
		Bottom: 256,
	}
	requiredNames := widgetLayoutRequiredNamesForData(layout)
	boundary := int(asset.Summary.NamesReferencedFromExportDataCount)
	if boundary < 0 {
		boundary = 0
	}
	if boundary > len(asset.Names) {
		boundary = len(asset.Names)
	}
	oldNames := append([]uasset.NameEntry(nil), asset.Names...)
	prefix := append([]uasset.NameEntry(nil), asset.Names[:boundary]...)
	suffix := append([]uasset.NameEntry(nil), asset.Names[boundary:]...)
	for _, rawName := range requiredNames {
		nameValue := rawName
		if nameValue == "" {
			continue
		}
		if findNameIndexByValue(prefix, nameValue) >= 0 {
			continue
		}
		if idx := findNameIndexByValue(suffix, nameValue); idx >= 0 {
			entry := suffix[idx]
			suffix = append(suffix[:idx], suffix[idx+1:]...)
			pos := lowerBoundNameEntry(prefix, nameValue)
			prefix = append(prefix[:pos], append([]uasset.NameEntry{entry}, prefix[pos:]...)...)
			continue
		}
		nonCase, casePreserving := edit.ComputeNameEntryHashesUE56(nameValue)
		entry := uasset.NameEntry{
			Value:              nameValue,
			NonCaseHash:        nonCase,
			CasePreservingHash: casePreserving,
		}
		pos := lowerBoundNameEntry(prefix, nameValue)
		prefix = append(prefix[:pos], append([]uasset.NameEntry{entry}, prefix[pos:]...)...)
	}
	newNames := append(prefix, suffix...)
	indexRemap, err := edit.BuildNameIndexRemapAllowInsertedNewEntries(oldNames, newNames)
	if err != nil {
		t.Fatalf("build name remap: %v", err)
	}
	workingBytes, err := edit.RewriteImportExportNameRefs(asset, indexRemap)
	if err != nil {
		t.Fatalf("RewriteImportExportNameRefs: %v", err)
	}
	var workingAsset *uasset.Asset
	remapped := *asset
	remapped.Raw.Bytes = workingBytes
	workingBytes, err = edit.RewriteNameMap(&remapped, newNames)
	if err != nil {
		t.Fatalf("RewriteNameMap: %v", err)
	}
	workingAsset, err = uasset.ParseBytes(workingBytes, opts)
	if err != nil {
		t.Fatalf("parse after name map rewrite: %v", err)
	}

	exportMutations, err := edit.BuildExportNameRemapMutations(asset, workingAsset, indexRemap, "", "")
	if err != nil {
		t.Fatalf("BuildExportNameRemapMutations: %v", err)
	}
	workingBytes, err = edit.RewriteAsset(workingAsset, exportMutations)
	if err != nil {
		t.Fatalf("RewriteAsset export mutations: %v", err)
	}
	workingAsset, err = uasset.ParseBytes(workingBytes, opts)
	if err != nil {
		t.Fatalf("parse after export remap: %v", err)
	}
	requireGeneratedClassParseWarningsEmpty(t, workingAsset, "after export remap")

	workingBytes, updatedAsset, err := edit.PatchInstancedTailNameRefs(asset, workingAsset, indexRemap, opts)
	if err != nil {
		t.Fatalf("PatchInstancedTailNameRefs: %v", err)
	}
	if len(workingBytes) == 0 {
		t.Fatalf("PatchInstancedTailNameRefs returned empty bytes")
	}
	requireGeneratedClassParseWarningsEmpty(t, updatedAsset, "after instanced tail patch")

	_, ensuredAsset, _, err := ensureNameEntriesPresentSorted(asset, opts, requiredNames)
	if err != nil {
		t.Fatalf("ensureNameEntriesPresentSorted: %v", err)
	}
	requireGeneratedClassParseWarningsEmpty(t, ensuredAsset, "after ensureNameEntriesPresentSorted")
}

func requireGeneratedClassParseWarningsEmpty(t *testing.T, asset *uasset.Asset, stage string) {
	t.Helper()

	generatedClassExport := -1
	for i, exp := range asset.Exports {
		if asset.ResolveClassName(exp) == "WidgetBlueprintGeneratedClass" {
			if generatedClassExport >= 0 {
				t.Fatalf("%s: multiple WidgetBlueprintGeneratedClass exports found", stage)
			}
			generatedClassExport = i
		}
	}
	if generatedClassExport < 0 {
		t.Fatalf("%s: WidgetBlueprintGeneratedClass export not found", stage)
	}
	start, end, withClassControl := edit.ExportPropertyBounds(asset, asset.Exports[generatedClassExport])
	parsed := asset.ParseTaggedPropertiesRange(start, end, withClassControl)
	if len(parsed.Warnings) > 0 {
		t.Fatalf("%s: generated class parse warnings: %v", stage, parsed.Warnings)
	}
}

func lowerBoundNameEntry(entries []uasset.NameEntry, nameValue string) int {
	needle := strings.ToLower(nameValue)
	lo, hi := 0, len(entries)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if strings.ToLower(entries[mid].Value) >= needle {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo
}
