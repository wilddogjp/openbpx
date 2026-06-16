package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wilddogjp/openbpx/pkg/edit"
	"github.com/wilddogjp/openbpx/pkg/uasset"
)

func TestRunBlueprintWidgetInitRejectsMissingTemplate(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-init", filepath.Join(t.TempDir(), "WBP_Login.uasset"),
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: bpx blueprint widget-init") {
		t.Fatalf("expected usage error, got: %s", stderr.String())
	}
}

func TestRunBlueprintWidgetInitRequiresPackagePathWhenNotDerivable(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-init", filepath.Join(t.TempDir(), "WBP_Login.uasset"),
		"--template", "minimum",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1 stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "package path is required") {
		t.Fatalf("expected package-path error, got: %s", stderr.String())
	}
}

func TestRunBlueprintWidgetInitDryRun(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "WBP_Login.uasset")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-init", outPath,
		"--template", "minimum",
		"--package-path", "/Game/UI",
		"--dry-run",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not create output file: err=%v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got, want := resp["assetName"], "WBP_Login"; got != want {
		t.Fatalf("assetName: got %#v want %q", got, want)
	}
	if got, want := resp["packagePath"], "/Game/UI"; got != want {
		t.Fatalf("packagePath: got %#v want %q", got, want)
	}
	if got, want := resp["longPackageName"], "/Game/UI/WBP_Login"; got != want {
		t.Fatalf("longPackageName: got %#v want %q", got, want)
	}
	if got, ok := resp["dryRun"].(bool); !ok || !got {
		t.Fatalf("dryRun: got %#v want true", resp["dryRun"])
	}
	if got, want := resp["templateSource"], filepath.Join("testdata", "golden", "ue5.6", "parse", "WBP_Minimum.uasset"); got != want {
		t.Fatalf("templateSource: got %#v want %q", got, want)
	}
}

func TestWidgetInitEmbeddedTemplateMatchesFixture(t *testing.T) {
	fixturePath := findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset")
	fixtureBytes, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if !bytes.Equal(widgetInitMinimumUE56TemplateBytes, fixtureBytes) {
		t.Fatalf("embedded template bytes differ from fixture %s", fixturePath)
	}
}

func TestRunBlueprintWidgetInitUsesEmbeddedTemplateOutsideRepo(t *testing.T) {
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	outDir := filepath.Join(t.TempDir(), "isolated")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir isolated dir: %v", err)
	}
	if err := os.Chdir(outDir); err != nil {
		t.Fatalf("chdir isolated dir: %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(origWD); chdirErr != nil {
			t.Fatalf("restore cwd: %v", chdirErr)
		}
	})

	outPath := filepath.Join(outDir, "WBP_Embedded.uasset")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-init", outPath,
		"--template", "minimum",
		"--package-path", "/Game/UI",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(outPath, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse embedded output asset: %v", err)
	}
	if got, want := asset.Summary.PackageName, "/Game/UI/WBP_Embedded"; got != want {
		t.Fatalf("summary package name: got %q want %q", got, want)
	}
}

func TestRunBlueprintWidgetInitCreatesAsset(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "WBP_Login.uasset")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-init", outPath,
		"--template", "minimum",
		"--package-path", "/Game/UI",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(outPath, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse output asset: %v", err)
	}
	if got, want := asset.Summary.PackageName, "/Game/UI/WBP_Login"; got != want {
		t.Fatalf("summary package name: got %q want %q", got, want)
	}
	if _, ok := findExportIndexByObjectName(asset, "WBP_Login"); !ok {
		t.Fatalf("WidgetBlueprint export WBP_Login not found")
	}
	if _, ok := findExportIndexByObjectName(asset, "WBP_Login_C"); !ok {
		t.Fatalf("generated class export WBP_Login_C not found")
	}
	if _, ok := findExportIndexByObjectName(asset, "Default__WBP_Login_C"); !ok {
		t.Fatalf("CDO export Default__WBP_Login_C not found")
	}
	if err := validateWidgetInitBlueprintShape(asset); err != nil {
		t.Fatalf("validateWidgetInitBlueprintShape: %v", err)
	}
	for _, entry := range asset.Names {
		if strings.Contains(entry.Value, "WBP_Minimum") {
			t.Fatalf("template name leaked after init: %q", entry.Value)
		}
	}
	section, _, _, err := parseAssetRegistrySection(asset)
	if err != nil {
		t.Fatalf("parse asset registry: %v", err)
	}
	if section == nil {
		t.Fatalf("asset registry section missing")
	}
	for _, obj := range section.Objects {
		if strings.Contains(obj.ObjectPath, "WBP_Minimum") {
			t.Fatalf("template object path leaked after init: %q", obj.ObjectPath)
		}
		if strings.Contains(obj.ObjectClass, "WBP_Minimum") {
			t.Fatalf("template object class leaked after init: %q", obj.ObjectClass)
		}
		for _, tag := range obj.Tags {
			if strings.Contains(tag.Value, "WBP_Minimum") {
				t.Fatalf("template asset registry tag leaked after init: %s=%q", tag.Key, tag.Value)
			}
			if strings.Contains(tag.Value, "/Game/BPXFixtures/Parse/WBP_Minimum") {
				t.Fatalf("template package leaked in asset registry tag after init: %s=%q", tag.Key, tag.Value)
			}
		}
	}
}

func TestRunBlueprintWidgetInitDerivesPackagePathFromContentOutput(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "MyProject", "Content", "UI", "WBP_Login.uasset")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-init", outPath,
		"--template", "minimum",
		"--dry-run",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got, want := resp["packagePath"], "/Game/UI"; got != want {
		t.Fatalf("packagePath: got %#v want %q", got, want)
	}
}

func TestRunBlueprintWidgetInitRejectsMismatchedAssetNameForContentOutput(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "MyProject", "Content", "UI", "WBP_Login.uasset")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-init", outPath,
		"--template", "minimum",
		"--asset-name", "WBP_Other",
		"--dry-run",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1 stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "must match output filename stem") {
		t.Fatalf("expected asset-name mismatch error, got: %s", stderr.String())
	}
}

func TestRunBlueprintWidgetInitRejectsMismatchedPackagePathForContentOutput(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "MyProject", "Content", "UI", "WBP_Login.uasset")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-init", outPath,
		"--template", "minimum",
		"--package-path", "/Game/Other",
		"--dry-run",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1 stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "must match derived output package path") {
		t.Fatalf("expected package-path mismatch error, got: %s", stderr.String())
	}
}

func TestRunBlueprintWidgetInitAllowsMatchingExplicitIdentityForContentOutput(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "MyProject", "Content", "UI", "WBP_Login.uasset")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-init", outPath,
		"--template", "minimum",
		"--asset-name", "WBP_Login",
		"--package-path", "/Game/UI",
		"--dry-run",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}
}

func TestRunBlueprintWidgetInitFollowupWidgetCommands(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "WBP_Login.uasset")

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
			"--name", "CanvasPanel_21",
		},
		{
			"blueprint", "widget-add", outPath,
			"--parent", "CanvasPanel_21",
			"--type", "image",
			"--name", "Image_1",
		},
		{
			"blueprint", "widget-write", outPath,
			"--widget", "CanvasPanel_21/Image_1",
			"--property", "layout-data",
			"--value", `{"position":[0,0],"size":[200,60],"anchors":[0,0,0,0],"alignment":[0,0]}`,
		},
		{
			"blueprint", "widget-write", outPath,
			"--widget", "CanvasPanel_21/Image_1",
			"--property", "brush-image",
			"--value", "/Game/Effects/Textures/Decals/chippedcracks",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		code := Run(argv, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(outPath, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse final asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	child, err := selectWidgetWriteTarget(targets, "CanvasPanel_21/Image_1")
	if err != nil {
		t.Fatalf("select child widget: %v", err)
	}
	if child.ClassName != "Image" {
		t.Fatalf("child class: got %q want Image", child.ClassName)
	}
	if len(child.SlotExports) != 2 {
		t.Fatalf("child slot exports: got %d want 2", len(child.SlotExports))
	}
}

func TestRunBlueprintWidgetInitFollowupWidgetCommandsWithCustomNames(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "WBP_Login.uasset")

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
			"--name", "Canvas_Root",
		},
		{
			"blueprint", "widget-add", outPath,
			"--parent", "Canvas_Root",
			"--type", "image",
			"--name", "Image_Circle",
		},
		{
			"blueprint", "widget-add", outPath,
			"--parent", "Canvas_Root",
			"--type", "image",
			"--name", "Image_Square",
		},
		{
			"blueprint", "widget-add", outPath,
			"--parent", "Canvas_Root",
			"--type", "textblock",
			"--name", "Text_Label",
		},
		{
			"blueprint", "widget-write", outPath,
			"--widget", "Canvas_Root/Text_Label",
			"--property", "text",
			"--value", "Hello World",
		},
		{
			"blueprint", "widget-write", outPath,
			"--widget", "Canvas_Root/Image_Circle",
			"--property", "brush-image",
			"--value", "/Game/Effects/Textures/Decals/chippedcracks",
		},
		{
			"blueprint", "widget-write", outPath,
			"--widget", "Canvas_Root/Image_Square",
			"--property", "brush-image",
			"--value", "/Game/Effects/Textures/Decals/chippedcracks",
		},
		{
			"validate", outPath,
		},
	} {
		stdout.Reset()
		stderr.Reset()
		code := Run(argv, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(outPath, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse final asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	canvas, err := selectWidgetWriteTarget(targets, "Canvas_Root")
	if err != nil {
		t.Fatalf("select canvas widget: %v", err)
	}
	imageCircle, err := selectWidgetWriteTarget(targets, "Canvas_Root/Image_Circle")
	if err != nil {
		t.Fatalf("select Image_Circle: %v", err)
	}
	imageSquare, err := selectWidgetWriteTarget(targets, "Canvas_Root/Image_Square")
	if err != nil {
		t.Fatalf("select Image_Square: %v", err)
	}
	textLabel, err := selectWidgetWriteTarget(targets, "Canvas_Root/Text_Label")
	if err != nil {
		t.Fatalf("select Text_Label: %v", err)
	}
	if canvas.ClassName != "CanvasPanel" {
		t.Fatalf("canvas class: got %q want CanvasPanel", canvas.ClassName)
	}
	if imageCircle.ClassName != "Image" || imageSquare.ClassName != "Image" {
		t.Fatalf("image classes: got %q and %q want Image", imageCircle.ClassName, imageSquare.ClassName)
	}
	if textLabel.ClassName != "TextBlock" {
		t.Fatalf("text class: got %q want TextBlock", textLabel.ClassName)
	}
	if len(imageCircle.SlotExports) != 2 || len(imageSquare.SlotExports) != 2 || len(textLabel.SlotExports) != 2 {
		t.Fatalf("slot exports: circle=%d square=%d text=%d", len(imageCircle.SlotExports), len(imageSquare.SlotExports), len(textLabel.SlotExports))
	}
	requireMirroredGeneratedWidgetTarget(t, asset, *imageCircle)
	requireMirroredGeneratedWidgetTarget(t, asset, *imageSquare)
	if len(textLabel.Exports) != 1 {
		t.Fatalf("%s exports: got %d want 1 (%v)", textLabel.Path, len(textLabel.Exports), textLabel.Exports)
	}
	requireGeneratedVariableVarTypeRawBase64(t, asset, canvas.BlueprintExport, "Image_Circle", "Image_Square", "Text_Label")
}

func TestRunBlueprintWidgetInitFollowupOverlayWidgetCommandsWithCustomNames(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "WBP_OverlayLogin.uasset")

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
			"--type", "overlay",
			"--name", "Overlay_Root",
		},
		{
			"blueprint", "widget-add", outPath,
			"--parent", "Overlay_Root",
			"--type", "image",
			"--name", "Image_Circle",
		},
		{
			"blueprint", "widget-add", outPath,
			"--parent", "Overlay_Root",
			"--type", "image",
			"--name", "Image_Square",
		},
		{
			"blueprint", "widget-add", outPath,
			"--parent", "Overlay_Root",
			"--type", "textblock",
			"--name", "Text_Label",
		},
		{
			"blueprint", "widget-write", outPath,
			"--widget", "Overlay_Root/Text_Label",
			"--property", "text",
			"--value", "Hello Overlay",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		code := Run(argv, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(outPath, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse final asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	overlay, err := selectWidgetWriteTarget(targets, "Overlay_Root")
	if err != nil {
		t.Fatalf("select overlay widget: %v", err)
	}
	imageCircle, err := selectWidgetWriteTarget(targets, "Overlay_Root/Image_Circle")
	if err != nil {
		t.Fatalf("select Image_Circle: %v", err)
	}
	imageSquare, err := selectWidgetWriteTarget(targets, "Overlay_Root/Image_Square")
	if err != nil {
		t.Fatalf("select Image_Square: %v", err)
	}
	textLabel, err := selectWidgetWriteTarget(targets, "Overlay_Root/Text_Label")
	if err != nil {
		t.Fatalf("select Text_Label: %v", err)
	}
	if overlay.ClassName != "Overlay" {
		t.Fatalf("overlay class: got %q want Overlay", overlay.ClassName)
	}
	if imageCircle.ClassName != "Image" || imageSquare.ClassName != "Image" {
		t.Fatalf("image classes: got %q and %q want Image", imageCircle.ClassName, imageSquare.ClassName)
	}
	if textLabel.ClassName != "TextBlock" {
		t.Fatalf("text class: got %q want TextBlock", textLabel.ClassName)
	}
	requireMirroredGeneratedWidgetTarget(t, asset, *imageCircle)
	requireMirroredGeneratedWidgetTarget(t, asset, *imageSquare)
	requireMirroredGeneratedWidgetTarget(t, asset, *textLabel)
	requireGeneratedVariableVarTypeRawBase64(t, asset, overlay.BlueprintExport, "Image_Circle", "Image_Square", "Text_Label")
}

func requireMirroredGeneratedWidgetTarget(t *testing.T, asset *uasset.Asset, target widgetWriteTarget) {
	t.Helper()
	if len(target.Exports) != 2 {
		t.Fatalf("%s exports: got %d want 2 (%v)", target.Path, len(target.Exports), target.Exports)
	}
	if len(target.SlotExports) != 2 {
		t.Fatalf("%s slot exports: got %d want 2 (%v)", target.Path, len(target.SlotExports), target.SlotExports)
	}
	generatedSlotValue, err := decodeExportRootPropertyValue(asset, target.SlotExports[1], "Content")
	if err != nil {
		t.Fatalf("%s generated slot Content: %v", target.Path, err)
	}
	ref, ok := generatedSlotValue.(map[string]any)
	if !ok {
		t.Fatalf("%s generated slot Content type: got %T want map", target.Path, generatedSlotValue)
	}
	index, ok := widgetAddInt64(ref["index"])
	if !ok || index <= 0 {
		t.Fatalf("%s generated slot Content index: got %#v want positive export ref", target.Path, ref["index"])
	}
}

func requireGeneratedVariableVarTypeRawBase64(t *testing.T, asset *uasset.Asset, blueprintExport int, objectNames ...string) {
	t.Helper()
	current, err := decodeExportRootPropertyValue(asset, blueprintExport, "GeneratedVariables")
	if err != nil {
		t.Fatalf("GeneratedVariables decode: %v", err)
	}
	decodedArray, ok := current.(map[string]any)
	if !ok {
		t.Fatalf("GeneratedVariables type: got %T want map", current)
	}
	items, err := anySliceLocal(decodedArray["value"])
	if err != nil {
		t.Fatalf("GeneratedVariables items: %v", err)
	}
	entries := map[string]map[string]any{}
	for _, item := range items {
		wrapped, ok := item.(map[string]any)
		if !ok {
			continue
		}
		entryValue := wrapped
		if inner, exists := wrapped["value"]; exists {
			innerMap, ok := inner.(map[string]any)
			if !ok {
				continue
			}
			entryValue = innerMap
		}
		fields, ok := entryValue["value"].(map[string]any)
		if !ok {
			continue
		}
		varNameField, ok := fields["VarName"].(map[string]any)
		if !ok {
			continue
		}
		varNameValue, ok := varNameField["value"].(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(varNameValue["name"]))
		if name != "" {
			entries[name] = fields
		}
	}
	for _, objectName := range objectNames {
		fields, ok := entries[objectName]
		if !ok {
			t.Fatalf("GeneratedVariables missing entry for %s", objectName)
		}
		varTypeField, ok := fields["VarType"].(map[string]any)
		if !ok {
			t.Fatalf("%s VarType field type: got %T want map", objectName, fields["VarType"])
		}
		varTypeStruct, ok := varTypeField["value"].(map[string]any)
		if !ok {
			t.Fatalf("%s VarType struct type: got %T want map", objectName, varTypeField["value"])
		}
		if trailing := varTypeStruct["trailingBytes"]; trailing != nil {
			t.Fatalf("%s VarType trailingBytes: got %#v want nil", objectName, trailing)
		}
		innerValue, ok := varTypeStruct["value"].(map[string]any)
		if !ok {
			t.Fatalf("%s VarType inner type: got %T want map", objectName, varTypeStruct["value"])
		}
		rawBase64 := strings.TrimSpace(fmt.Sprint(innerValue["rawBase64"]))
		if rawBase64 == "" {
			t.Fatalf("%s VarType rawBase64 is empty", objectName)
		}
	}
}

func TestEnsureWidgetAddPrerequisitesKeepsOverlayBlueprintPropertiesParseable(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "WBP_OverlayLogin.uasset")

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
			"--type", "overlay",
			"--name", "Overlay_Root",
		},
		{
			"blueprint", "widget-add", outPath,
			"--parent", "Overlay_Root",
			"--type", "image",
			"--name", "Image_Circle",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		code := Run(argv, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(outPath, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	parent, err := selectWidgetWriteTarget(targets, "Overlay_Root")
	if err != nil {
		t.Fatalf("select Overlay_Root: %v", err)
	}
	if err := validateWidgetWriteTarget(*parent); err != nil {
		t.Fatalf("validateWidgetWriteTarget: %v", err)
	}
	if err := validateWidgetAddTarget(asset, targets, *parent, "Image_Square"); err != nil {
		t.Fatalf("validateWidgetAddTarget: %v", err)
	}
	ctx, err := resolveWidgetAddContext(asset, *parent)
	if err != nil {
		t.Fatalf("resolveWidgetAddContext: %v", err)
	}
	parentSlots, err := readWidgetAddObjectRefArrayValue(asset, ctx.DesignerParentExport, "Slots")
	if err != nil {
		t.Fatalf("readWidgetAddObjectRefArrayValue: %v", err)
	}
	parentHadChildren := len(parentSlots) > 0
	childName, err := parseWidgetAddName("Image_Square")
	if err != nil {
		t.Fatalf("parseWidgetAddName: %v", err)
	}
	_, workingAsset, _, _, err := ensureWidgetAddPrerequisites(
		asset,
		uasset.DefaultParseOptions(),
		ctx.PanelClass,
		widgetAddChildClassSpec{ResolvedClassName: "Image"},
		childName,
		ctx.BlueprintObjectName,
		!parentHadChildren,
		true,
		true,
	)
	if err != nil {
		t.Fatalf("ensureWidgetAddPrerequisites: %v", err)
	}
	ctx, err = refreshWidgetAddContext(workingAsset, 0, "Overlay_Root")
	if err != nil {
		t.Fatalf("refreshWidgetAddContext: %v", err)
	}
	if _, err := captureGeneratedClassWidgetVariableFieldLayout(workingAsset, ctx.GeneratedClassExport); err != nil {
		t.Fatalf("captureGeneratedClassWidgetVariableFieldLayout: %v", err)
	}

	exp := workingAsset.Exports[11]
	start, end, withClassControl := edit.ExportPropertyBounds(workingAsset, exp)
	parsed := workingAsset.ParseTaggedPropertiesRange(start, end, withClassControl)
	if len(parsed.Warnings) > 0 {
		t.Fatalf("widget blueprint property warnings after prerequisites: %v", parsed.Warnings)
	}
}

func TestInsertExportEntriesAllowsSecondOverlayImageDesignerSlotAfterCustomFirstImage(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "WBP_OverlayLogin.uasset")

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
			"--type", "overlay",
			"--name", "Overlay_Root",
		},
		{
			"blueprint", "widget-add", outPath,
			"--parent", "Overlay_Root",
			"--type", "image",
			"--name", "Image_Circle",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		code := Run(argv, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(outPath, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	parent, err := selectWidgetWriteTarget(targets, "Overlay_Root")
	if err != nil {
		t.Fatalf("select Overlay_Root: %v", err)
	}
	ctx, err := resolveWidgetAddContext(asset, *parent)
	if err != nil {
		t.Fatalf("resolveWidgetAddContext: %v", err)
	}
	childName, err := parseWidgetAddName("Image_Square")
	if err != nil {
		t.Fatalf("parseWidgetAddName: %v", err)
	}
	_, workingAsset, _, _, err := ensureWidgetAddPrerequisites(
		asset,
		uasset.DefaultParseOptions(),
		ctx.PanelClass,
		widgetAddChildClassSpec{ResolvedClassName: "Image"},
		childName,
		ctx.BlueprintObjectName,
		false,
		true,
		true,
	)
	if err != nil {
		t.Fatalf("ensureWidgetAddPrerequisites: %v", err)
	}
	ctx, err = refreshWidgetAddContext(workingAsset, 0, "Overlay_Root")
	if err != nil {
		t.Fatalf("refreshWidgetAddContext: %v", err)
	}
	overlayChainMode := isWidgetAddOverlayRootChainMode(workingAsset, ctx)
	slotName, err := nextWidgetAddSlotName(workingAsset, ctx.PanelClass, widgetAddSlotDefaultSuffix(ctx.PanelClass, overlayChainMode))
	if err != nil {
		t.Fatalf("nextWidgetAddSlotName: %v", err)
	}
	slotEntry, err := buildWidgetAddSlotEntry(workingAsset, ctx.PanelClass, ctx.DesignerParentExport, slotName)
	if err != nil {
		t.Fatalf("buildWidgetAddSlotEntry: %v", err)
	}
	insertAt, err := findWidgetAddChildInsertPos(workingAsset, ctx.DesignerParentExport, maxIntLocal(ctx.DesignerParentExport, ctx.GeneratedParentExport)+1)
	if err != nil {
		t.Fatalf("findWidgetAddChildInsertPos: %v", err)
	}

	exp := workingAsset.Exports[11]
	start, end, withClassControl := edit.ExportPropertyBounds(workingAsset, exp)
	parsed := workingAsset.ParseTaggedPropertiesRange(start, end, withClassControl)
	if len(parsed.Warnings) > 0 {
		t.Fatalf("widget blueprint property warnings before insert: %v", parsed.Warnings)
	}

	if _, err := edit.InsertExportEntries(workingAsset, insertAt, []edit.ExportInsertEntry{slotEntry}); err != nil {
		t.Fatalf("InsertExportEntries: %v", err)
	}
}

func TestRunBlueprintWidgetInitWidgetReadSeesAddedRootCanvasPanel(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "WBP_Login.uasset")

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
			"--name", "CanvasPanel_21",
		},
		{
			"blueprint", "widget-read", outPath,
		},
	} {
		stdout.Reset()
		stderr.Reset()
		code := Run(argv, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	var resp map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("decode widget-read response: %v", err)
	}
	blueprints, ok := resp["widgetBlueprints"].([]any)
	if !ok || len(blueprints) != 1 {
		t.Fatalf("widgetBlueprints: got %#v want single entry", resp["widgetBlueprints"])
	}
	bp, ok := blueprints[0].(map[string]any)
	if !ok {
		t.Fatalf("widgetBlueprints[0]: got %#v want map", blueprints[0])
	}
	if got, want := int(bp["logicalWidgetCount"].(float64)), 1; got != want {
		t.Fatalf("logicalWidgetCount: got %d want %d", got, want)
	}
	logicalWidgets, ok := bp["logicalWidgets"].([]any)
	if !ok || len(logicalWidgets) != 1 {
		t.Fatalf("logicalWidgets: got %#v want single entry", bp["logicalWidgets"])
	}
	logicalWidget, ok := logicalWidgets[0].(map[string]any)
	if !ok {
		t.Fatalf("logicalWidgets[0]: got %#v want map", logicalWidgets[0])
	}
	if got, want := logicalWidget["objectName"], "CanvasPanel_21"; got != want {
		t.Fatalf("logical widget objectName: got %#v want %q", got, want)
	}
	trees, ok := bp["widgetTrees"].([]any)
	if !ok || len(trees) != 2 {
		t.Fatalf("widgetTrees: got %#v want two entries", bp["widgetTrees"])
	}
	for i, treeRaw := range trees {
		tree, ok := treeRaw.(map[string]any)
		if !ok {
			t.Fatalf("widgetTrees[%d]: got %#v want map", i, treeRaw)
		}
		if got, want := int(tree["widgetCount"].(float64)), 1; got != want {
			t.Fatalf("widgetTrees[%d].widgetCount: got %d want %d", i, got, want)
		}
		widgets, ok := tree["widgets"].([]any)
		if !ok || len(widgets) != 1 {
			t.Fatalf("widgetTrees[%d].widgets: got %#v want single entry", i, tree["widgets"])
		}
		widget, ok := widgets[0].(map[string]any)
		if !ok {
			t.Fatalf("widgetTrees[%d].widgets[0]: got %#v want map", i, widgets[0])
		}
		if got, want := widget["objectName"], "CanvasPanel_21"; got != want {
			t.Fatalf("widgetTrees[%d].widgets[0].objectName: got %#v want %q", i, got, want)
		}
	}
}

func TestRunBlueprintWidgetInitRejectsExistingFileWithoutForce(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "WBP_Login.uasset")
	if err := os.WriteFile(outPath, []byte("existing"), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-init", outPath,
		"--template", "minimum",
		"--package-path", "/Game/UI",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1 stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "destination file already exists") {
		t.Fatalf("expected existing file error, got: %s", stderr.String())
	}
}

func TestRunBlueprintWidgetInitForceWithBackup(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "WBP_Login.uasset")
	if err := os.WriteFile(outPath, []byte("existing"), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-init", outPath,
		"--template", "minimum",
		"--package-path", "/Game/UI",
		"--force",
		"--backup",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	backupBytes, err := os.ReadFile(outPath + ".backup")
	if err != nil {
		t.Fatalf("read backup file: %v", err)
	}
	if string(backupBytes) != "existing" {
		t.Fatalf("backup content: got %q want %q", string(backupBytes), "existing")
	}
}
