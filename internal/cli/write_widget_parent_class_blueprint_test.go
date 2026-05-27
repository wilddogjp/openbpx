package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

func TestRunBlueprintWidgetInitWithParentClass(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "WBP_Activatable.uasset")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-init", outPath,
		"--template", "minimum",
		"--package-path", "/Game/UI",
		"--parent-class", "/Script/CommonUI.CommonActivatableWidget",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("widget-init exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(outPath, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse output asset: %v", err)
	}
	assertWidgetBlueprintParentClass(t, asset, "WBP_Activatable", "/Script/CommonUI.CommonActivatableWidget")
}

func TestRunBlueprintWidgetInitWithProjectModuleParentClass(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "WBP_LyraActivatable.uasset")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-init", outPath,
		"--template", "minimum",
		"--package-path", "/Game/UI",
		"--parent-class", "/Script/LyraGame.LyraActivatableWidget",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("widget-init exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(outPath, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse output asset: %v", err)
	}
	assertWidgetBlueprintParentClass(t, asset, "WBP_LyraActivatable", "/Script/LyraGame.LyraActivatableWidget")
}

func TestRunBlueprintWidgetInitRejectsNonScriptParentClass(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "WBP_Activatable.uasset")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-init", outPath,
		"--template", "minimum",
		"--package-path", "/Game/UI",
		"--parent-class", "/Game/UI/WBP_Base",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("widget-init exit code: got %d want 1 stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "compiled /Script class path like /Script/CommonUI.CommonActivatableWidget or /Script/LyraGame.LyraActivatableWidget") {
		t.Fatalf("expected compiled /Script parent-class error, got: %s", stderr.String())
	}
}

func TestRunBlueprintWidgetParentClassRewritesRootlessWidget(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "WBP_Rootless.uasset")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-init", outPath,
			"--template", "minimum",
			"--package-path", "/Game/UI",
		},
		{
			"blueprint", "widget-parent-class", outPath,
			"--class", "/Script/CommonUI.CommonActivatableWidget",
		},
		{
			"blueprint", "widget-add", outPath,
			"--parent", "root",
			"--type", "overlay",
			"--name", "Overlay_1",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
			t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(outPath, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse output asset: %v", err)
	}
	assertWidgetBlueprintParentClass(t, asset, "WBP_Rootless", "/Script/CommonUI.CommonActivatableWidget")
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	target, err := selectWidgetWriteTarget(targets, "Overlay_1")
	if err != nil {
		t.Fatalf("select root widget: %v", err)
	}
	if got, want := target.ClassName, "Overlay"; got != want {
		t.Fatalf("root widget class: got %q want %q", got, want)
	}
}

func TestRunBlueprintWidgetParentClassAcceptsProjectModuleClass(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "WBP_RootlessProjectParent.uasset")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-init", outPath,
			"--template", "minimum",
			"--package-path", "/Game/UI",
		},
		{
			"blueprint", "widget-parent-class", outPath,
			"--class", "/Script/LyraGame.LyraActivatableWidget",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
			t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(outPath, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse output asset: %v", err)
	}
	assertWidgetBlueprintParentClass(t, asset, "WBP_RootlessProjectParent", "/Script/LyraGame.LyraActivatableWidget")
}

func TestRunBlueprintWidgetParentClassRejectsNonRootlessWidget(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "WBP_NonRootless.uasset")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-init", outPath,
			"--template", "minimum",
			"--package-path", "/Game/UI",
		},
		{
			"blueprint", "widget-add", outPath,
			"--parent", "root",
			"--type", "canvaspanel",
			"--name", "CanvasPanel_1",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
			t.Fatalf("setup exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	code := Run([]string{
		"blueprint", "widget-parent-class", outPath,
		"--class", "/Script/CommonUI.CommonActivatableWidget",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("widget-parent-class exit code: got %d want 1 stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "already has a root widget") {
		t.Fatalf("expected rootless safety error, got: %s", stderr.String())
	}
}

func assertWidgetBlueprintParentClass(t *testing.T, asset *uasset.Asset, blueprintObjectName, want string) {
	t.Helper()
	blueprintExport, ok := findExportIndexByObjectName(asset, blueprintObjectName)
	if !ok {
		t.Fatalf("WidgetBlueprint export %q not found", blueprintObjectName)
	}
	bpProps := asset.ParseExportProperties(blueprintExport)
	decoded := decodeAllProperties(asset, bpProps.Properties)
	parentRaw, ok := decoded["ParentClass"]
	if !ok {
		t.Fatalf("ParentClass property missing")
	}
	parentMap, ok := parentRaw.(map[string]any)
	if !ok {
		t.Fatalf("ParentClass decoded value type: %T", parentRaw)
	}
	parentIndex, ok := extractIntLike(parentMap["index"])
	if !ok || parentIndex >= 0 {
		t.Fatalf("ParentClass index: got %#v want negative import index", parentMap["index"])
	}
	got, err := nativeClassPathFromImportIndex(asset, -parentIndex)
	if err != nil {
		t.Fatalf("nativeClassPathFromImportIndex: %v", err)
	}
	if got != want {
		t.Fatalf("ParentClass: got %q want %q", got, want)
	}
	generatedExport, ok := findExportIndexByObjectName(asset, blueprintObjectName+"_C")
	if !ok {
		t.Fatalf("generated class export %q not found", blueprintObjectName+"_C")
	}
	if got, wantIdx := asset.Exports[generatedExport].SuperIndex, uasset.PackageIndex(parentIndex); got != wantIdx {
		t.Fatalf("generated class super index: got %s want %s", asset.ParseIndex(got), asset.ParseIndex(wantIdx))
	}
	spec, err := parseWidgetParentClassSpec(want)
	if err != nil {
		t.Fatalf("parseWidgetParentClassSpec(%q): %v", want, err)
	}
	if _, found := findNativeClassImport(asset, spec.PackagePath, spec.ClassName); !found {
		t.Fatalf("%s import not found after parent-class rewrite", spec.ClassName)
	}
	if _, found := findParentDefaultObjectImport(asset, spec, "Default__"+spec.ClassName); !found {
		t.Fatalf("%s default object import not found after parent-class rewrite", spec.ClassName)
	}
	if want == "/Script/CommonUI.CommonActivatableWidget" || want == "/Script/LyraGame.LyraActivatableWidget" {
		if got := asset.Summary.PackageFlags & packageFlagRequiresLoc; got == 0 {
			t.Fatalf("package flags missing PKG_RequiresLocalizationGather: raw=0x%08x", asset.Summary.PackageFlags)
		}

		cdoExport, err := findCDOExportIndex(asset)
		if err != nil {
			t.Fatalf("findCDOExportIndex: %v", err)
		}
		props := asset.ParseExportProperties(cdoExport)
		decoded := decodeAllProperties(asset, props.Properties)
		paletteRaw, ok := decoded["PaletteCategory"]
		if !ok {
			t.Fatalf("PaletteCategory property missing on CDO after parent-class rewrite")
		}
		palette, ok := paletteRaw.(map[string]any)
		if !ok {
			t.Fatalf("PaletteCategory decoded value type: %T", paletteRaw)
		}
		if got, _ := palette["namespace"].(string); got != "UMG" {
			t.Fatalf("PaletteCategory namespace: got %q want %q", got, "UMG")
		}
		if got, _ := palette["key"].(string); got != "UserCreated" {
			t.Fatalf("PaletteCategory key: got %q want %q", got, "UserCreated")
		}
		if got, _ := palette["value"].(string); got != "User Created" {
			t.Fatalf("PaletteCategory value: got %q want %q", got, "User Created")
		}
	}
}
