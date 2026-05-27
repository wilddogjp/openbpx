package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

func TestRunBlueprintWidgetAddRejectsMissingName(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-add", "/tmp/nonexistent.uasset",
		"--parent", "CanvasPanel_22",
		"--type", "image",
		"--dry-run",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: bpx blueprint widget-add") {
		t.Fatalf("expected usage error, got: %s", stderr.String())
	}
}

func TestRunBlueprintWidgetAddRejectsUnsupportedType(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-add", "/tmp/nonexistent.uasset",
		"--parent", "CanvasPanel_22",
		"--type", "commontextblock",
		"--name", "Image_23",
		"--dry-run",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if !strings.Contains(stderr.String(), "unsupported widget-add type") {
		t.Fatalf("expected unsupported type error, got: %s", stderr.String())
	}
}

func TestParseWidgetAddNameInstancedDisplay(t *testing.T) {
	name, err := parseWidgetAddName("Image_23")
	if err != nil {
		t.Fatalf("parseWidgetAddName: %v", err)
	}
	if name.Base != "Image" {
		t.Fatalf("base: got %q want %q", name.Base, "Image")
	}
	if name.Number != 24 {
		t.Fatalf("number: got %d want 24", name.Number)
	}
}

func TestSelectWidgetWriteTargetRejectsAmbiguousForWidgetAdd(t *testing.T) {
	_, err := selectWidgetWriteTarget([]widgetWriteTarget{
		{BlueprintExport: 7, ObjectName: "Image_23", Path: "CanvasPanel_22/Image_23", Exports: []int{8, 9}},
		{BlueprintExport: 7, ObjectName: "Image_23", Path: "Overlay_116/Image_23", Exports: []int{10, 11}},
	}, "Image_23")
	if err == nil {
		t.Fatalf("expected ambiguous selector to fail")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunBlueprintWidgetAddRejectsUnsupportedParentClass(t *testing.T) {
	path := findExistingGoldenOperationFixturePath(t, "widget_write_brush_image", "before.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "Image_22",
		"--type", "image",
		"--name", "Image_23",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1 stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "does not accept child widgets") {
		t.Fatalf("expected leaf parent class error, got: %s", stderr.String())
	}
}

func TestWidgetChildPolicyForClass(t *testing.T) {
	tests := []struct {
		className string
		want      widgetChildPolicy
		ok        bool
	}{
		{className: "Image", want: widgetChildPolicyNone, ok: true},
		{className: "TextBlock", want: widgetChildPolicyNone, ok: true},
		{className: "RichTextBlock", want: widgetChildPolicyNone, ok: true},
		{className: "ProgressBar", want: widgetChildPolicyNone, ok: true},
		{className: "Slider", want: widgetChildPolicyNone, ok: true},
		{className: "Spacer", want: widgetChildPolicyNone, ok: true},
		{className: "ScrollBar", want: widgetChildPolicyNone, ok: true},
		{className: "EditableText", want: widgetChildPolicyNone, ok: true},
		{className: "EditableTextBox", want: widgetChildPolicyNone, ok: true},
		{className: "MultiLineEditableTextBox", want: widgetChildPolicyNone, ok: true},
		{className: "SpinBox", want: widgetChildPolicyNone, ok: true},
		{className: "ComboBoxString", want: widgetChildPolicyNone, ok: true},
		{className: "ListView", want: widgetChildPolicyNone, ok: true},
		{className: "TileView", want: widgetChildPolicyNone, ok: true},
		{className: "TreeView", want: widgetChildPolicyNone, ok: true},
		{className: "CheckBox", want: widgetChildPolicySingle, ok: true},
		{className: "Button", want: widgetChildPolicySingle, ok: true},
		{className: "NamedSlot", want: widgetChildPolicySingle, ok: true},
		{className: "RetainerBox", want: widgetChildPolicySingle, ok: true},
		{className: "InvalidationBox", want: widgetChildPolicySingle, ok: true},
		{className: "MenuAnchor", want: widgetChildPolicySingle, ok: true},
		{className: "SizeBox", want: widgetChildPolicySingle, ok: true},
		{className: "ScaleBox", want: widgetChildPolicySingle, ok: true},
		{className: "BackgroundBlur", want: widgetChildPolicySingle, ok: true},
		{className: "SafeZone", want: widgetChildPolicySingle, ok: true},
		{className: "WindowTitleBarArea", want: widgetChildPolicySingle, ok: true},
		{className: "Border", want: widgetChildPolicySingle, ok: true},
		{className: "CanvasPanel", want: widgetChildPolicyMulti, ok: true},
		{className: "ScrollBox", want: widgetChildPolicyMulti, ok: true},
		{className: "WrapBox", want: widgetChildPolicyMulti, ok: true},
		{className: "GridPanel", want: widgetChildPolicyMulti, ok: true},
		{className: "UniformGridPanel", want: widgetChildPolicyMulti, ok: true},
		{className: "WidgetSwitcher", want: widgetChildPolicyMulti, ok: true},
		{className: "StackBox", want: widgetChildPolicyMulti, ok: true},
		{className: "MysteryWidget", want: widgetChildPolicyUnknown, ok: false},
	}

	for _, tt := range tests {
		got, ok := widgetChildPolicyForClass(tt.className)
		if got != tt.want || ok != tt.ok {
			t.Fatalf("%s: got (%v, %v) want (%v, %v)", tt.className, got, ok, tt.want, tt.ok)
		}
	}
}

func TestRunBlueprintWidgetAddBasicLeafWidgetsUnderCanvasPanel(t *testing.T) {
	tests := []struct {
		widgetType            string
		className             string
		objectName            string
		wantGeneratedVariable bool
	}{
		{widgetType: "progressbar", className: "ProgressBar", objectName: "ProgressBar_1", wantGeneratedVariable: true},
		{widgetType: "slider", className: "Slider", objectName: "Slider_1", wantGeneratedVariable: true},
		{widgetType: "spacer", className: "Spacer", objectName: "Spacer_1", wantGeneratedVariable: false},
		{widgetType: "scrollbar", className: "ScrollBar", objectName: "ScrollBar_1", wantGeneratedVariable: false},
		{widgetType: "editabletext", className: "EditableText", objectName: "EditableText_1", wantGeneratedVariable: true},
		{widgetType: "editabletextbox", className: "EditableTextBox", objectName: "EditableTextBox_1", wantGeneratedVariable: true},
		{widgetType: "multilineeditabletextbox", className: "MultiLineEditableTextBox", objectName: "MultiLineEditableTextBox_1", wantGeneratedVariable: true},
		{widgetType: "spinbox", className: "SpinBox", objectName: "SpinBox_1", wantGeneratedVariable: true},
		{widgetType: "comboboxstring", className: "ComboBoxString", objectName: "ComboBoxString_1", wantGeneratedVariable: true},
		{widgetType: "listview", className: "ListView", objectName: "ListView_1", wantGeneratedVariable: true},
		{widgetType: "tileview", className: "TileView", objectName: "TileView_1", wantGeneratedVariable: true},
		{widgetType: "treeview", className: "TreeView", objectName: "TreeView_1", wantGeneratedVariable: true},
	}

	for _, tt := range tests {
		t.Run(tt.className, func(t *testing.T) {
			path := findExistingGoldenParseFixturePath(t, "WBP_CanvasPanel.uasset")
			work := copyFixtureToTemp(t, path)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run([]string{
				"blueprint", "widget-add", work,
				"--parent", "CanvasPanel_22",
				"--type", tt.widgetType,
				"--name", tt.objectName,
			}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
			}

			asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("parse rewritten asset: %v", err)
			}
			targets, err := collectWidgetWriteTargets(asset, 0)
			if err != nil {
				t.Fatalf("collectWidgetWriteTargets: %v", err)
			}
			child, err := selectWidgetWriteTarget(targets, "CanvasPanel_22/"+tt.objectName)
			if err != nil {
				t.Fatalf("select child widget: %v", err)
			}
			if got, want := child.ClassName, tt.className; got != want {
				t.Fatalf("child class: got %q want %q", got, want)
			}
			if len(child.Exports) != 2 {
				t.Fatalf("widget exports: got %d want 2 (%v)", len(child.Exports), child.Exports)
			}
			if len(child.SlotExports) != 2 {
				t.Fatalf("slot exports: got %d want 2 (%v)", len(child.SlotExports), child.SlotExports)
			}
			if _, found := findUMGClassImport(asset, tt.className); !found {
				t.Fatalf("%s class import not found after widget-add", tt.className)
			}
			rawBase64 := strings.TrimSpace(widgetAddGeneratedVariableVarTypeRawBase64ByObjectName(asset, child.BlueprintExport, tt.objectName))
			if tt.wantGeneratedVariable && rawBase64 == "" {
				t.Fatalf("%s generated variable VarType rawBase64 is empty", tt.className)
			}
			if !tt.wantGeneratedVariable && rawBase64 != "" {
				t.Fatalf("%s generated variable VarType rawBase64: got %q want empty", tt.className, rawBase64)
			}
		})
	}
}

func TestRunBlueprintWidgetAddNamedSlotUnderCanvasPanelAndAcceptsOneChild(t *testing.T) {
	work := copyFixtureToTemp(t, findExistingGoldenParseFixturePath(t, "WBP_CanvasPanel.uasset"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-add", work,
			"--parent", "CanvasPanel_22",
			"--type", "namedslot",
			"--name", "NamedSlot_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "CanvasPanel_22/NamedSlot_1",
			"--type", "textblock",
			"--name", "TextBlock_1",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
			t.Fatalf("widget-add exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	namedSlot, err := selectWidgetWriteTarget(targets, "CanvasPanel_22/NamedSlot_1")
	if err != nil {
		t.Fatalf("select NamedSlot: %v", err)
	}
	if got, want := namedSlot.ClassName, "NamedSlot"; got != want {
		t.Fatalf("namedslot class: got %q want %q", got, want)
	}
	child, err := selectWidgetWriteTarget(targets, "CanvasPanel_22/NamedSlot_1/TextBlock_1")
	if err != nil {
		t.Fatalf("select NamedSlot child: %v", err)
	}
	if got, want := child.ClassName, "TextBlock"; got != want {
		t.Fatalf("namedslot child class: got %q want %q", got, want)
	}
	if _, found := findUMGClassImport(asset, "NamedSlot"); !found {
		t.Fatalf("NamedSlot class import not found after widget-add")
	}
}

func TestRunBlueprintWidgetAddNamedSlotRejectsSecondDirectChild(t *testing.T) {
	work := copyFixtureToTemp(t, findExistingGoldenParseFixturePath(t, "WBP_CanvasPanel.uasset"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-add", work,
			"--parent", "CanvasPanel_22",
			"--type", "namedslot",
			"--name", "NamedSlot_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "CanvasPanel_22/NamedSlot_1",
			"--type", "textblock",
			"--name", "TextBlock_1",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
			t.Fatalf("widget-add exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	code := Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "CanvasPanel_22/NamedSlot_1",
		"--type", "image",
		"--name", "Image_1",
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected second NamedSlot child add to fail")
	}
	if !strings.Contains(stderr.String(), "accepts only one direct child") {
		t.Fatalf("expected single-child parent error, got: %s", stderr.String())
	}
}

func TestRunBlueprintWidgetAddTopLevelCustomNamedTextBlockUnderCanvasPanelOmitsGeneratedWidgetCompanion(t *testing.T) {
	work := copyFixtureToTemp(t, findExistingGoldenParseFixturePath(t, "WBP_CanvasPanel.uasset"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "CanvasPanel_22",
		"--type", "textblock",
		"--name", "Text_Header",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	child, err := selectWidgetWriteTarget(targets, "CanvasPanel_22/Text_Header")
	if err != nil {
		t.Fatalf("select child widget: %v", err)
	}
	if got, want := child.ClassName, "TextBlock"; got != want {
		t.Fatalf("child class: got %q want %q", got, want)
	}
	if got, want := len(child.Exports), 1; got != want {
		t.Fatalf("widget exports: got %d want %d (%v)", got, want, child.Exports)
	}
	if got, want := len(child.SlotExports), 2; got != want {
		t.Fatalf("slot exports: got %d want %d (%v)", got, want, child.SlotExports)
	}

	blueprintExport := mustFindSingleWidgetBlueprintExport(t, asset)
	shape, err := captureWidgetBlueprintShape(asset, blueprintExport)
	if err != nil {
		t.Fatalf("captureWidgetBlueprintShape: %v", err)
	}
	if !containsStringFold(shape["designer"], "CanvasPanel_22/Text_Header") {
		t.Fatalf("designer shape missing custom text child: %v", shape["designer"])
	}
	if containsStringFold(shape["generated"], "CanvasPanel_22/Text_Header") {
		t.Fatalf("generated shape unexpectedly contains custom text child: %v", shape["generated"])
	}
	if got, want := generatedClassWidgetVariableFieldNames(t, asset), []string{"Text_Header"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("generated class field names: got %v want %v", got, want)
	}
	if got, want := generatedClassPropertyGuidNames(t, asset), []string{"Text_Header"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("generated class PropertyGuids: got %v want %v", got, want)
	}
	if got, want := widgetBlueprintGeneratedVariableNames(t, asset), []string{"Text_Header"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("WidgetBlueprint GeneratedVariables: got %v want %v", got, want)
	}
	if got, want := generatedTreeAllWidgetNamesWithNulls(t, asset), []string{"CanvasPanel_22", ""}; !reflect.DeepEqual(got, want) {
		t.Fatalf("generated WidgetTree AllWidgets: got %v want %v", got, want)
	}
	if !widgetAddHasDisplayLabel(asset, child.Exports[0], "Text_Header") {
		t.Fatalf("designer custom text missing DisplayLabel")
	}
	depends, warnings := parseDependsMap(asset)
	if len(warnings) > 0 {
		t.Fatalf("parseDependsMap warnings: %v", warnings)
	}
	generatedSlotExport := child.SlotExports[len(child.SlotExports)-1]
	generatedSlotDeps := anyMapSlice(depends[generatedSlotExport]["dependencies"])
	if len(generatedSlotDeps) != 0 {
		t.Fatalf("generated slot depends count: got %d want 0 (%v)", len(generatedSlotDeps), generatedSlotDeps)
	}

}

func TestRunBlueprintWidgetAddNestedCanvasPanelLateChildRewritesAllWidgetsInPreorder(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "WBP_PreorderNested.uasset")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{"blueprint", "widget-init", outPath, "--template", "minimum", "--package-path", "/Game/UI"},
		{"blueprint", "widget-add", outPath, "--parent", "root", "--type", "canvaspanel", "--name", "Canvas_Root"},
		{"blueprint", "widget-add", outPath, "--parent", "Canvas_Root", "--type", "border", "--name", "Border_Body"},
		{"blueprint", "widget-add", outPath, "--parent", "Canvas_Root/Border_Body", "--type", "canvaspanel", "--name", "Canvas_Cards"},
		{"blueprint", "widget-add", outPath, "--parent", "Canvas_Root/Border_Body/Canvas_Cards", "--type", "border", "--name", "Border_Card01"},
		{"blueprint", "widget-add", outPath, "--parent", "Canvas_Root/Border_Body/Canvas_Cards/Border_Card01", "--type", "canvaspanel", "--name", "Canvas_Card01"},
		{"blueprint", "widget-add", outPath, "--parent", "Canvas_Root/Border_Body/Canvas_Cards/Border_Card01/Canvas_Card01", "--type", "textblock", "--name", "Text_Card01_Main"},
		{"blueprint", "widget-add", outPath, "--parent", "Canvas_Root/Border_Body/Canvas_Cards/Border_Card01/Canvas_Card01", "--type", "border", "--name", "Border_Card01_Footer"},
		{"blueprint", "widget-add", outPath, "--parent", "Canvas_Root/Border_Body/Canvas_Cards/Border_Card01/Canvas_Card01/Border_Card01_Footer", "--type", "textblock", "--name", "Text_Card01_Footer"},
		{"blueprint", "widget-add", outPath, "--parent", "Canvas_Root/Border_Body/Canvas_Cards", "--type", "border", "--name", "Border_Card02"},
		{"blueprint", "widget-add", outPath, "--parent", "Canvas_Root/Border_Body/Canvas_Cards/Border_Card02", "--type", "canvaspanel", "--name", "Canvas_Card02"},
		{"blueprint", "widget-add", outPath, "--parent", "Canvas_Root/Border_Body/Canvas_Cards/Border_Card02/Canvas_Card02", "--type", "textblock", "--name", "Text_Card02_Main"},
		{"blueprint", "widget-add", outPath, "--parent", "Canvas_Root/Border_Body/Canvas_Cards/Border_Card01/Canvas_Card01", "--type", "textblock", "--name", "Text_Card01_Extra01"},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
			t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(outPath, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	designerNames := widgetTreeAllWidgetNamesWithNullsForRole(t, asset, "designer")
	generatedNames := widgetTreeAllWidgetNamesWithNullsForRole(t, asset, "generated")

	designerExtra := indexOfStringLocal(designerNames, "Text_Card01_Extra01")
	designerCard02 := indexOfStringLocal(designerNames, "Border_Card02")
	if designerExtra < 0 || designerCard02 < 0 {
		t.Fatalf("designer AllWidgets missing expected names: %v", designerNames)
	}
	if designerExtra >= designerCard02 {
		t.Fatalf("designer AllWidgets order: extra=%d card02=%d names=%v", designerExtra, designerCard02, designerNames)
	}

	generatedCard02 := indexOfStringLocal(generatedNames, "Border_Card02")
	if generatedCard02 < 0 {
		t.Fatalf("generated AllWidgets missing Border_Card02: %v", generatedNames)
	}
	if designerExtra >= len(generatedNames) {
		t.Fatalf("generated AllWidgets shorter than designer order: %v", generatedNames)
	}
	if got := generatedNames[designerExtra]; got != "Text_Card01_Extra01" {
		t.Fatalf("generated AllWidgets expected mirrored extra child at index %d, got %q (%v)", designerExtra, got, generatedNames)
	}
	if designerExtra >= generatedCard02 {
		t.Fatalf("generated AllWidgets order: extra=%d card02=%d names=%v", designerExtra, generatedCard02, generatedNames)
	}

	generatedVarNames := widgetBlueprintGeneratedVariableNames(t, asset)
	propGuidNames := generatedClassPropertyGuidNames(t, asset)
	fieldNames := generatedClassWidgetVariableFieldNames(t, asset)
	for label, names := range map[string][]string{
		"GeneratedVariables": generatedVarNames,
		"PropertyGuids":      propGuidNames,
		"FieldNames":         fieldNames,
	} {
		extra := indexOfStringLocal(names, "Text_Card01_Extra01")
		card02 := indexOfStringLocal(names, "Border_Card02")
		if extra < 0 || card02 < 0 {
			t.Fatalf("%s missing expected names: %v", label, names)
		}
		if extra >= card02 {
			t.Fatalf("%s order: extra=%d card02=%d names=%v", label, extra, card02, names)
		}
	}
}

func TestRunBlueprintWidgetAddFourCardsPreservesGeneratedClassTailSuffixRefs(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "WBP_FourCardsTailRefs.uasset")

	argvs := [][]string{
		{"blueprint", "widget-init", outPath, "--template", "minimum", "--package-path", "/Game/WBP"},
		{"blueprint", "widget-add", outPath, "--parent", "root", "--type", "canvaspanel", "--name", "Canvas_Root"},
		{"blueprint", "widget-add", outPath, "--parent", "Canvas_Root", "--type", "border", "--name", "Border_Body"},
		{"blueprint", "widget-add", outPath, "--parent", "Canvas_Root/Border_Body", "--type", "canvaspanel", "--name", "Canvas_Cards"},
	}
	for _, suffix := range []string{"03", "04", "08", "11"} {
		cardName := "Border_Card" + suffix
		cardPath := "Canvas_Root/Border_Body/Canvas_Cards/" + cardName
		canvasName := "Canvas_Card" + suffix
		canvasPath := cardPath + "/" + canvasName
		argvs = append(argvs,
			[]string{"blueprint", "widget-add", outPath, "--parent", "Canvas_Root/Border_Body/Canvas_Cards", "--type", "border", "--name", cardName},
			[]string{"blueprint", "widget-add", outPath, "--parent", cardPath, "--type", "canvaspanel", "--name", canvasName},
			[]string{"blueprint", "widget-add", outPath, "--parent", canvasPath, "--type", "border", "--name", cardName + "_Footer"},
			[]string{"blueprint", "widget-add", outPath, "--parent", canvasPath + "/" + cardName + "_Footer", "--type", "textblock", "--name", "Text_Card" + suffix + "_Footer"},
			[]string{"blueprint", "widget-add", outPath, "--parent", canvasPath, "--type", "border", "--name", cardName + "_Header"},
			[]string{"blueprint", "widget-add", outPath, "--parent", canvasPath + "/" + cardName + "_Header", "--type", "textblock", "--name", "Text_Card" + suffix + "_Header"},
			[]string{"blueprint", "widget-add", outPath, "--parent", canvasPath, "--type", "border", "--name", cardName + "_Row01"},
			[]string{"blueprint", "widget-add", outPath, "--parent", canvasPath + "/" + cardName + "_Row01", "--type", "textblock", "--name", "Text_Card" + suffix + "_Row01"},
			[]string{"blueprint", "widget-add", outPath, "--parent", canvasPath, "--type", "textblock", "--name", "Text_Card" + suffix + "_Main"},
			[]string{"blueprint", "widget-add", outPath, "--parent", canvasPath, "--type", "textblock", "--name", "Text_Card" + suffix + "_Other01"},
		)
	}
	runWidgetAddArgsSequence(t, argvs)

	asset, err := uasset.ParseFile(outPath, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	blueprintExport := mustFindSingleWidgetBlueprintExport(t, asset)
	props := asset.ParseExportProperties(blueprintExport)
	decoded := decodeAllProperties(asset, props.Properties)
	generatedClassExport := widgetExportIndexFromDecoded(decoded["GeneratedClass"]) - 1
	if generatedClassExport < 0 || generatedClassExport >= len(asset.Exports) {
		t.Fatalf("generated class export out of range: %d", generatedClassExport+1)
	}
	generatedClassName := asset.Exports[generatedClassExport].ObjectName.Display(asset.Names)
	cdoExport, found := findExportIndexByObjectName(asset, "Default__"+generatedClassName)
	if !found {
		t.Fatalf("CDO export not found for %q", generatedClassName)
	}
	noneIndex := findNameIndex(asset.Names, "None")
	if noneIndex < 0 {
		t.Fatalf("None name not found")
	}

	exp := asset.Exports[generatedClassExport]
	payload := asset.Raw.Bytes[int(exp.SerialOffset):int(exp.SerialOffset+exp.SerialSize)]
	if int(exp.ScriptSerializationEndOffset) > len(payload) {
		t.Fatalf("generated class script end offset out of bounds: %d > %d", exp.ScriptSerializationEndOffset, len(payload))
	}
	tail := payload[int(exp.ScriptSerializationEndOffset):]
	if len(tail) < 28 {
		t.Fatalf("generated class tail too short: %d", len(tail))
	}
	order := packageByteOrder(asset)
	if got, want := int32(order.Uint32(tail[len(tail)-28:len(tail)-24])), int32(blueprintExport+1); got != want {
		t.Fatalf("tail blueprint ref: got %d want %d", got, want)
	}
	if got := order.Uint32(tail[len(tail)-24 : len(tail)-20]); got != 0 {
		t.Fatalf("tail blueprint upper dword: got %d want 0", got)
	}
	if got, want := int32(order.Uint32(tail[len(tail)-16:len(tail)-12])), noneIndex; got != want {
		t.Fatalf("tail None name index: got %d want %d", got, want)
	}
	if got, want := int32(order.Uint32(tail[len(tail)-4:])), int32(cdoExport+1); got != want {
		t.Fatalf("tail CDO ref: got %d want %d", got, want)
	}
}

func TestRunBlueprintWidgetAddUserWidgetRequiresClass(t *testing.T) {
	work := copyFixtureToTemp(t, findExistingGoldenParseFixturePath(t, "WBP_CanvasPanel.uasset"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "CanvasPanel_22",
		"--type", "userwidget",
		"--name", "WBP_TextBlock_1",
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit when --class is missing")
	}
	if !strings.Contains(stderr.String(), "widget-add --type userwidget requires --class") {
		t.Fatalf("expected missing --class error, got: %s", stderr.String())
	}
}

func TestRunBlueprintWidgetAddUserWidgetUnderCanvasPanel(t *testing.T) {
	work := copyFixtureToTemp(t, findExistingGoldenParseFixturePath(t, "WBP_CanvasPanel.uasset"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "CanvasPanel_22",
		"--type", "userwidget",
		"--class", "/Game/BPXFixtures/Parse/WBP_TextBlock",
		"--name", "WBP_TextBlock_1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	child, err := selectWidgetWriteTarget(targets, "CanvasPanel_22/WBP_TextBlock_1")
	if err != nil {
		t.Fatalf("select child widget: %v", err)
	}
	if got, want := child.ClassName, "WBP_TextBlock_C"; got != want {
		t.Fatalf("child class: got %q want %q", got, want)
	}
	if len(child.Exports) != 2 {
		t.Fatalf("widget exports: got %d want 2 (%v)", len(child.Exports), child.Exports)
	}
	if len(child.SlotExports) != 2 {
		t.Fatalf("slot exports: got %d want 2 (%v)", len(child.SlotExports), child.SlotExports)
	}
	if idx, found := findObjectImportByPath(asset, "/Script/UMG", "WidgetBlueprintGeneratedClass", "/Game/BPXFixtures/Parse/WBP_TextBlock"); !found || idx <= 0 {
		t.Fatalf("WidgetBlueprintGeneratedClass import for WBP_TextBlock not found after widget-add")
	}
	rawBase64 := strings.TrimSpace(widgetAddGeneratedVariableVarTypeRawBase64ByObjectName(asset, child.BlueprintExport, "WBP_TextBlock_1"))
	if got, want := rawBase64, "QgAAAAAAAABBAAAAAAAAAOv///8AAAAAAAAAAAAAAAAAQQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"; got != want {
		t.Fatalf("generated variable VarType rawBase64: got %q want %q", got, want)
	}
	depends, warnings := parseDependsMap(asset)
	if len(warnings) > 0 {
		t.Fatalf("parseDependsMap warnings: %v", warnings)
	}
	if len(depends) < 11 {
		t.Fatalf("depends map entries: got %d want at least 11", len(depends))
	}
	deps := anyMapSlice(depends[9]["dependencies"])
	if len(deps) != 2 {
		t.Fatalf("designer child depends count: got %d want 2", len(deps))
	}
	if got, ok := anyInt(deps[0]["index"]); !ok || got != -21 {
		t.Fatalf("designer child first dependency: got %v want -21", deps[0]["index"])
	}
}

func TestRunBlueprintWidgetAddRichTextBlockUnderCanvasPanel(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_CanvasPanel.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "CanvasPanel_22",
		"--type", "richtextblock",
		"--name", "RichTextBlock_23",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	child, err := selectWidgetWriteTarget(targets, "CanvasPanel_22/RichTextBlock_23")
	if err != nil {
		t.Fatalf("select child widget: %v", err)
	}
	if child.ClassName != "RichTextBlock" {
		t.Fatalf("child class: got %q want %q", child.ClassName, "RichTextBlock")
	}
	if len(child.SlotExports) != 2 {
		t.Fatalf("slot exports: got %d want 2 (%v)", len(child.SlotExports), child.SlotExports)
	}
	if _, found := findUMGClassImport(asset, "RichTextBlock"); !found {
		t.Fatalf("RichTextBlock class import not found after widget-add")
	}
	if rawBase64 := widgetAddGeneratedVariableVarTypeRawBase64ByObjectName(asset, child.BlueprintExport, "RichTextBlock_23"); strings.TrimSpace(rawBase64) == "" {
		t.Fatalf("RichTextBlock generated variable VarType rawBase64 is empty")
	}
}

func TestRunBlueprintWidgetAddRejectsRichTextBlockAsParent(t *testing.T) {
	work, childPath := buildRichTextWidgetTestAsset(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-add", work,
		"--parent", childPath,
		"--type", "image",
		"--name", "Image_1",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1 stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "does not accept child widgets") {
		t.Fatalf("expected leaf parent class error, got: %s", stderr.String())
	}
}

func TestWidgetAddSlotClassName(t *testing.T) {
	tests := []struct {
		panelClass string
		want       string
	}{
		{panelClass: "CanvasPanel", want: "CanvasPanelSlot"},
		{panelClass: "GridPanel", want: "GridSlot"},
		{panelClass: "RetainerBox", want: "PanelSlot"},
		{panelClass: "InvalidationBox", want: "PanelSlot"},
		{panelClass: "MenuAnchor", want: "PanelSlot"},
		{panelClass: "UniformGridPanel", want: "UniformGridSlot"},
		{panelClass: "StackBox", want: "StackBoxSlot"},
		{panelClass: "WindowTitleBarArea", want: "WindowTitleBarAreaSlot"},
	}

	for _, tt := range tests {
		got, err := widgetAddSlotClassName(tt.panelClass)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tt.panelClass, err)
		}
		if got != tt.want {
			t.Fatalf("%s: got %q want %q", tt.panelClass, got, tt.want)
		}
	}
}

func TestWidgetAddIsDirectChildPath(t *testing.T) {
	tests := []struct {
		parent    string
		candidate string
		want      bool
	}{
		{parent: "Button_1", candidate: "Button_1", want: false},
		{parent: "Button_1", candidate: "CanvasPanel_1/Button_1", want: false},
		{parent: "CanvasPanel_1/Button_1", candidate: "CanvasPanel_1/Button_1/TextBlock_1", want: true},
		{parent: "CanvasPanel_1/Button_1", candidate: "CanvasPanel_1/Button_1/Overlay_1/TextBlock_1", want: false},
	}
	for _, tt := range tests {
		if got := widgetAddIsDirectChildPath(tt.parent, tt.candidate); got != tt.want {
			t.Fatalf("widgetAddIsDirectChildPath(%q, %q): got %v want %v", tt.parent, tt.candidate, got, tt.want)
		}
	}
}

func TestRunBlueprintWidgetAddRejectsLeafParentClass(t *testing.T) {
	path := findExistingGoldenOperationFixturePath(t, "widget_write_brush_image", "before.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "Image_22",
		"--type", "image",
		"--name", "Image_23",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1 stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "does not accept child widgets") {
		t.Fatalf("expected leaf parent class error, got: %s", stderr.String())
	}
}

func TestValidateWidgetAddTargetRejectsUnknownParentClass(t *testing.T) {
	err := validateWidgetAddTarget(&uasset.Asset{}, []widgetWriteTarget{
		{BlueprintExport: 1, ObjectName: "Child_1", Path: "Mystery_1/Child_1"},
	}, widgetWriteTarget{
		BlueprintExport: 1,
		ClassName:       "MysteryPanel",
		ObjectName:      "Mystery_1",
		Path:            "Mystery_1",
	}, "Child_2")
	if err == nil {
		t.Fatalf("expected unknown parent class to fail")
	}
	if !strings.Contains(err.Error(), "widget-add currently supports only") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunBlueprintWidgetAddSupportsParentWithExistingChildren(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_CanvasPanel_TextBlock.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "CanvasPanel_22",
		"--type", "image",
		"--name", "Image_23",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	child, err := selectWidgetWriteTarget(targets, "CanvasPanel_22/Image_23")
	if err != nil {
		t.Fatalf("select child widget: %v", err)
	}
	if child.ClassName != "Image" {
		t.Fatalf("child class: got %q want %q", child.ClassName, "Image")
	}
	if len(child.SlotExports) != 2 {
		t.Fatalf("slot exports: got %d want 2 (%v)", len(child.SlotExports), child.SlotExports)
	}
}

func TestRunBlueprintWidgetAddTextBlockUnderCanvasPanel(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_CanvasPanel.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "CanvasPanel_22",
		"--type", "textblock",
		"--name", "TextBlock_23",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	child, err := selectWidgetWriteTarget(targets, "CanvasPanel_22/TextBlock_23")
	if err != nil {
		t.Fatalf("select child widget: %v", err)
	}
	parent, err := selectWidgetWriteTarget(targets, "CanvasPanel_22")
	if err != nil {
		t.Fatalf("select parent widget: %v", err)
	}
	if child.ClassName != "TextBlock" {
		t.Fatalf("child class: got %q want %q", child.ClassName, "TextBlock")
	}
	if len(child.SlotExports) != 2 {
		t.Fatalf("slot exports: got %d want 2 (%v)", len(child.SlotExports), child.SlotExports)
	}
	if _, found := findUMGClassImport(asset, "TextBlock"); !found {
		t.Fatalf("TextBlock class import not found after widget-add")
	}
	depends, warnings := parseDependsMap(asset)
	if len(warnings) > 0 {
		t.Fatalf("parseDependsMap warnings: %v", warnings)
	}
	generatedSlotExport := child.SlotExports[len(child.SlotExports)-1]
	generatedParentExport := parent.Exports[len(parent.Exports)-1]
	generatedSlotDeps := anyMapSlice(depends[generatedSlotExport]["dependencies"])
	wantGeneratedSlotDeps := 0
	if len(child.Exports) > 1 {
		wantGeneratedSlotDeps = 2
	}
	if len(generatedSlotDeps) != wantGeneratedSlotDeps {
		t.Fatalf("generated slot depends count: got %d want %d (%v)", len(generatedSlotDeps), wantGeneratedSlotDeps, generatedSlotDeps)
	}
	if len(child.Exports) > 1 {
		if got, ok := anyInt(generatedSlotDeps[0]["index"]); !ok || got != generatedParentExport+1 {
			t.Fatalf("generated slot parent dependency: got %v want %d", generatedSlotDeps[0]["index"], generatedParentExport+1)
		}
		generatedChildExport := child.Exports[len(child.Exports)-1]
		if got, ok := anyInt(generatedSlotDeps[1]["index"]); !ok || got != generatedChildExport+1 {
			t.Fatalf("generated slot child dependency: got %v want %d", generatedSlotDeps[1]["index"], generatedChildExport+1)
		}
	}

}

func TestRunBlueprintWidgetAddNestedCanvasPanelCustomTextUpdatesGeneratedTreeDepends(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "WBP_NestedCanvasDepends.uasset")

	runWidgetAddArgsSequence(t, [][]string{
		{"blueprint", "widget-init", outPath, "--template", "minimum", "--package-path", "/Game/UI"},
		{"blueprint", "widget-add", outPath, "--parent", "root", "--type", "canvaspanel", "--name", "Canvas_Root"},
		{"blueprint", "widget-add", outPath, "--parent", "Canvas_Root", "--type", "border", "--name", "Border_Body"},
		{"blueprint", "widget-add", outPath, "--parent", "Canvas_Root/Border_Body", "--type", "canvaspanel", "--name", "Canvas_Cards"},
		{"blueprint", "widget-add", outPath, "--parent", "Canvas_Root/Border_Body/Canvas_Cards", "--type", "textblock", "--name", "Text_Item01"},
	})

	asset, err := uasset.ParseFile(outPath, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	child, err := selectWidgetWriteTarget(targets, "Canvas_Root/Border_Body/Canvas_Cards/Text_Item01")
	if err != nil {
		t.Fatalf("select child widget: %v", err)
	}
	parent, err := selectWidgetWriteTarget(targets, "Canvas_Root/Border_Body/Canvas_Cards")
	if err != nil {
		t.Fatalf("select parent widget: %v", err)
	}
	if got, want := len(child.Exports), 2; got != want {
		t.Fatalf("widget exports: got %d want %d (%v)", got, want, child.Exports)
	}
	if got, want := len(child.SlotExports), 2; got != want {
		t.Fatalf("slot exports: got %d want %d (%v)", got, want, child.SlotExports)
	}

	depends, warnings := parseDependsMap(asset)
	if len(warnings) > 0 {
		t.Fatalf("parseDependsMap warnings: %v", warnings)
	}
	generatedSlotExport := child.SlotExports[len(child.SlotExports)-1]
	generatedParentExport := parent.Exports[len(parent.Exports)-1]
	generatedChildExport := child.Exports[len(child.Exports)-1]
	generatedSlotDeps := anyMapSlice(depends[generatedSlotExport]["dependencies"])
	if len(generatedSlotDeps) != 2 {
		t.Fatalf("generated slot depends count: got %d want 2 (%v)", len(generatedSlotDeps), generatedSlotDeps)
	}
	if got, ok := anyInt(generatedSlotDeps[0]["index"]); !ok || got != generatedParentExport+1 {
		t.Fatalf("generated slot parent dependency: got %v want %d", generatedSlotDeps[0]["index"], generatedParentExport+1)
	}
	if got, ok := anyInt(generatedSlotDeps[1]["index"]); !ok || got != generatedChildExport+1 {
		t.Fatalf("generated slot child dependency: got %v want %d", generatedSlotDeps[1]["index"], generatedChildExport+1)
	}
	if !widgetAddHasDisplayLabel(asset, child.Exports[0], "Text_Item01") {
		t.Fatalf("designer nested custom text missing DisplayLabel")
	}
	if !widgetAddHasDisplayLabel(asset, generatedChildExport, "Text_Item01") {
		t.Fatalf("generated nested custom text missing DisplayLabel")
	}

	blueprintExport := mustFindSingleWidgetBlueprintExport(t, asset)
	shape, err := captureWidgetBlueprintShape(asset, blueprintExport)
	if err != nil {
		t.Fatalf("captureWidgetBlueprintShape: %v", err)
	}
	if !containsStringFold(shape["generated"], "Canvas_Root/Border_Body/Canvas_Cards/Text_Item01") {
		t.Fatalf("generated shape missing nested custom text child: %v", shape["generated"])
	}
	props := asset.ParseExportProperties(blueprintExport)
	decoded := decodeAllProperties(asset, props.Properties)
	generatedClassExport := widgetExportIndexFromDecoded(decoded["GeneratedClass"]) - 1
	treeExports := findWidgetTreeExports(asset, blueprintExport, generatedClassExport)
	if len(treeExports) != 2 {
		t.Fatalf("widget tree exports: got %d want 2 (%v)", len(treeExports), treeExports)
	}
	generatedTreeDeps := anyMapSlice(depends[treeExports[1]]["dependencies"])
	if len(generatedTreeDeps) != 2 {
		t.Fatalf("generated tree depends count: got %d want 2 (%v)", len(generatedTreeDeps), generatedTreeDeps)
	}
	if got, ok := anyInt(generatedTreeDeps[0]["index"]); !ok || got != generatedParentExport+1 {
		t.Fatalf("generated tree dependency: got %v want %d", generatedTreeDeps[0]["index"], generatedParentExport+1)
	}
	if got, ok := anyInt(generatedTreeDeps[1]["index"]); !ok || got != generatedChildExport+1 {
		t.Fatalf("generated tree child dependency: got %v want %d", generatedTreeDeps[1]["index"], generatedChildExport+1)
	}
}

func TestRunBlueprintWidgetAddRejectsDuplicateWidgetName(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_CanvasPanel.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "CanvasPanel_22",
		"--type", "image",
		"--name", "CanvasPanel_22",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1 stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Fatalf("expected duplicate widget name error, got: %s", stderr.String())
	}
}

func TestRunBlueprintWidgetAddCanvasPanel(t *testing.T) {
	path := findExistingGoldenOperationFixturePath(t, "widget_add_image_canvaspanel", "before.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "CanvasPanel_22",
		"--type", "image",
		"--name", "Image_23",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	child, err := selectWidgetWriteTarget(targets, "CanvasPanel_22/Image_23")
	if err != nil {
		t.Fatalf("select child widget: %v", err)
	}
	if child.ClassName != "Image" {
		t.Fatalf("child class: got %q want %q", child.ClassName, "Image")
	}
	if len(child.Exports) != 1 {
		t.Fatalf("child exports: got %d want 1", len(child.Exports))
	}
	if len(child.SlotExports) != 2 {
		t.Fatalf("child slot exports: got %d want 2 (%v)", len(child.SlotExports), child.SlotExports)
	}
	if _, found := findUMGClassImport(asset, "Image"); !found {
		t.Fatalf("Image class import not found after widget-add")
	}
	if _, found := findUMGClassImport(asset, "CanvasPanelSlot"); !found {
		t.Fatalf("CanvasPanelSlot class import not found after widget-add")
	}

	slotCount := 0
	for _, exp := range asset.Exports {
		if exp.ObjectName.Display(asset.Names) == "CanvasPanelSlot_1" {
			slotCount++
		}
	}
	if slotCount != 2 {
		t.Fatalf("slot export count: got %d want 2", slotCount)
	}

	parent, err := selectWidgetWriteTarget(targets, "CanvasPanel_22")
	if err != nil {
		t.Fatalf("select parent widget: %v", err)
	}
	shape, err := captureWidgetBlueprintShape(asset, parent.BlueprintExport)
	if err != nil {
		t.Fatalf("captureWidgetBlueprintShape: %v", err)
	}
	if !containsStringFold(shape["designer"], "CanvasPanel_22/Image_23") {
		t.Fatalf("shape[designer] missing child path: %v", shape["designer"])
	}
	if containsStringFold(shape["generated"], "CanvasPanel_22/Image_23") {
		t.Fatalf("shape[generated] unexpectedly contains child path: %v", shape["generated"])
	}
}

func TestRunBlueprintWidgetAddAfterRootAddEnsuresExpandedInDesigner(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "root",
		"--type", "canvaspanel",
		"--name", "CanvasPanel_21",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("root add exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "CanvasPanel_21",
		"--type", "image",
		"--name", "Image_1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("image add exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	parent, err := selectWidgetWriteTarget(targets, "CanvasPanel_21")
	if err != nil {
		t.Fatalf("select parent widget: %v", err)
	}
	for _, exportIdx := range parent.Exports {
		value, err := decodeExportRootPropertyValue(asset, exportIdx, "bExpandedInDesigner")
		if err != nil {
			t.Fatalf("export %d missing bExpandedInDesigner: %v", exportIdx+1, err)
		}
		boolValue, ok := value.(bool)
		if !ok {
			t.Fatalf("export %d bExpandedInDesigner type: got %T want bool", exportIdx+1, value)
		}
		if !boolValue {
			t.Fatalf("export %d bExpandedInDesigner: got false want true", exportIdx+1)
		}
	}
}

func TestRunBlueprintWidgetAddAfterRootAddPreservesGeneratedClassDisplayNameFString(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "root",
		"--type", "canvaspanel",
		"--name", "CanvasPanel_21",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("root add exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "CanvasPanel_21",
		"--type", "image",
		"--name", "Image_1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("image add exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	generatedClassExport, ok := findExportIndexByObjectName(asset, "WBP_Minimum_C")
	if !ok {
		t.Fatalf("generated class export not found")
	}
	exp := asset.Exports[generatedClassExport]
	payload := asset.Raw.Bytes[int(exp.SerialOffset):int(exp.SerialOffset+exp.SerialSize)]
	tail := payload[int(exp.ScriptSerializationEndOffset):]

	valid := append([]byte{0x08, 0x00, 0x00, 0x00}, []byte("Image_1\x00")...)
	if !bytes.Contains(tail, valid) {
		t.Fatalf("generated class tail missing valid Image_1 FString prefix")
	}
	corrupt := append([]byte{0x0b, 0x00, 0x00, 0x00}, []byte("Image_1\x00")...)
	if bytes.Contains(tail, corrupt) {
		t.Fatalf("generated class tail still contains corrupted Image_1 FString prefix")
	}
}

func TestRunBlueprintWidgetAddOverlay(t *testing.T) {
	path := findExistingGoldenOperationFixturePath(t, "widget_add_image_overlay", "before.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "Overlay_116",
		"--type", "image",
		"--name", "Image_23",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	child, err := selectWidgetWriteTarget(targets, "Overlay_116/Image_23")
	if err != nil {
		t.Fatalf("select child widget: %v", err)
	}
	if child.ClassName != "Image" {
		t.Fatalf("child class: got %q want %q", child.ClassName, "Image")
	}
	if len(child.Exports) != 1 {
		t.Fatalf("child exports: got %d want 1", len(child.Exports))
	}
	if len(child.SlotExports) != 2 {
		t.Fatalf("child slot exports: got %d want 2 (%v)", len(child.SlotExports), child.SlotExports)
	}
	if _, found := findUMGClassImport(asset, "OverlaySlot"); !found {
		t.Fatalf("OverlaySlot class import not found after widget-add")
	}

	slotCount := 0
	for _, exp := range asset.Exports {
		if exp.ObjectName.Display(asset.Names) == "OverlaySlot_1" {
			slotCount++
		}
	}
	if slotCount != 2 {
		t.Fatalf("slot export count: got %d want 2", slotCount)
	}
}

func TestRunBlueprintWidgetAddOverlaySecondImage(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-add", work,
			"--parent", "root",
			"--type", "overlay",
			"--name", "Overlay_21",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "Overlay_21",
			"--type", "image",
			"--name", "Image_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "Overlay_21",
			"--type", "image",
			"--name", "Image_2",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		code := Run(argv, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	firstChild, err := selectWidgetWriteTarget(targets, "Overlay_21/Image_1")
	if err != nil {
		t.Fatalf("select first child widget: %v", err)
	}
	secondChild, err := selectWidgetWriteTarget(targets, "Overlay_21/Image_2")
	if err != nil {
		t.Fatalf("select second child widget: %v", err)
	}
	if firstChild.ClassName != "Image" || secondChild.ClassName != "Image" {
		t.Fatalf("child classes: got %q and %q", firstChild.ClassName, secondChild.ClassName)
	}
	parent, err := selectWidgetWriteTarget(targets, "Overlay_21")
	if err != nil {
		t.Fatalf("select parent widget: %v", err)
	}
	shape, err := captureWidgetBlueprintShape(asset, parent.BlueprintExport)
	if err != nil {
		t.Fatalf("captureWidgetBlueprintShape: %v", err)
	}
	if !containsStringFold(shape["designer"], "Overlay_21/Image_1") || !containsStringFold(shape["designer"], "Overlay_21/Image_2") {
		t.Fatalf("designer shape missing overlay children: %v", shape["designer"])
	}
	blueprintProps := asset.ParseExportProperties(parent.BlueprintExport)
	decoded := decodeAllProperties(asset, blueprintProps.Properties)
	generatedVars, ok := decoded["GeneratedVariables"].(map[string]any)
	if !ok {
		t.Fatalf("GeneratedVariables missing or invalid")
	}
	items, err := anySliceLocal(generatedVars["value"])
	if err != nil {
		t.Fatalf("GeneratedVariables items: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("GeneratedVariables count: got %d want 2", len(items))
	}
	requireNameAbsent(t, asset, "WBP Image 1")
	if got, want := generatedClassWidgetVariableFieldNames(t, asset), []string{"Image_2", "Image_1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("generated class field names: got %v want %v", got, want)
	}
	requireGeneratedClassWidgetVariableFieldCategoryLengths(t, asset, "WBP_Minimum", []string{"Image_2", "Image_1"})
}

func TestRunBlueprintWidgetAddOverlayRootChainFirstImage(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-add", work,
			"--parent", "root",
			"--type", "overlay",
			"--name", "Overlay_21",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "Overlay_21",
			"--type", "image",
			"--name", "Image_1",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		code := Run(argv, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	parent, err := selectWidgetWriteTarget(targets, "Overlay_21")
	if err != nil {
		t.Fatalf("select parent widget: %v", err)
	}
	shape, err := captureWidgetBlueprintShape(asset, parent.BlueprintExport)
	if err != nil {
		t.Fatalf("captureWidgetBlueprintShape: %v", err)
	}
	if !containsStringFold(shape["designer"], "Overlay_21/Image_1") {
		t.Fatalf("designer shape missing child path: %v", shape["designer"])
	}
	if !containsStringFold(shape["generated"], "Overlay_21/Image_1") {
		t.Fatalf("generated shape missing child path: %v", shape["generated"])
	}
	requireNameAbsent(t, asset, "WBP Image 1")
	if got, want := generatedClassWidgetVariableFieldNames(t, asset), []string{"Image_1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("generated class field names: got %v want %v", got, want)
	}
	requireGeneratedClassWidgetVariableFieldCategoryLengths(t, asset, "WBP_Minimum", []string{"Image_1"})
}

func TestRunBlueprintWidgetAddOverlayRootChainFirstTextBlock(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-add", work,
			"--parent", "root",
			"--type", "overlay",
			"--name", "Overlay_21",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "Overlay_21",
			"--type", "textblock",
			"--name", "TextBlock_1",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		code := Run(argv, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	parent, err := selectWidgetWriteTarget(targets, "Overlay_21")
	if err != nil {
		t.Fatalf("select parent widget: %v", err)
	}
	shape, err := captureWidgetBlueprintShape(asset, parent.BlueprintExport)
	if err != nil {
		t.Fatalf("captureWidgetBlueprintShape: %v", err)
	}
	if !containsStringFold(shape["designer"], "Overlay_21/TextBlock_1") {
		t.Fatalf("designer shape missing child path: %v", shape["designer"])
	}
	if !containsStringFold(shape["generated"], "Overlay_21/TextBlock_1") {
		t.Fatalf("generated shape missing child path: %v", shape["generated"])
	}
	requireNameAbsent(t, asset, "WBP TextBlock 1")
	if got, want := generatedClassWidgetVariableFieldNames(t, asset), []string{"TextBlock_1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("generated class field names: got %v want %v", got, want)
	}
	requireGeneratedClassWidgetVariableFieldCategoryLengths(t, asset, "WBP_Minimum", []string{"TextBlock_1"})
}

func TestRunBlueprintWidgetAddOverlayRootChainSecondTextBlock(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-add", work,
			"--parent", "root",
			"--type", "overlay",
			"--name", "Overlay_21",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "Overlay_21",
			"--type", "textblock",
			"--name", "TextBlock_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "Overlay_21",
			"--type", "textblock",
			"--name", "TextBlock_2",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		code := Run(argv, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	parent, err := selectWidgetWriteTarget(targets, "Overlay_21")
	if err != nil {
		t.Fatalf("select parent widget: %v", err)
	}
	shape, err := captureWidgetBlueprintShape(asset, parent.BlueprintExport)
	if err != nil {
		t.Fatalf("captureWidgetBlueprintShape: %v", err)
	}
	if !containsStringFold(shape["designer"], "Overlay_21/TextBlock_1") || !containsStringFold(shape["designer"], "Overlay_21/TextBlock_2") {
		t.Fatalf("designer shape missing overlay children: %v", shape["designer"])
	}
	if got, want := generatedClassWidgetVariableFieldNames(t, asset), []string{"TextBlock_2", "TextBlock_1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("generated class field names: got %v want %v", got, want)
	}
	requireGeneratedClassWidgetVariableFieldCategoryLengths(t, asset, "WBP_Minimum", []string{"TextBlock_2", "TextBlock_1"})
}

func TestRunBlueprintWidgetAddOverlayRootChainSecondImageMatchesFixture(t *testing.T) {
	fixtureDir := findExistingGoldenOperationFixtureDir(t, "widget_add_two_image_overlay")
	work := copyFixtureToTemp(t, filepath.Join(fixtureDir, "before.uasset"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-add", work,
			"--parent", "root",
			"--type", "overlay",
			"--name", "Overlay_21",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "Overlay_21",
			"--type", "image",
			"--name", "Image_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "Overlay_21",
			"--type", "image",
			"--name", "Image_2",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		code := Run(argv, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	requireWidgetAddFixtureGraphMatch(t, work, filepath.Join(fixtureDir, "after.uasset"), []string{
		"Overlay_21",
		"Overlay_21/Image_1",
		"Overlay_21/Image_2",
	})

	actualAsset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	expectedAsset, err := uasset.ParseFile(filepath.Join(fixtureDir, "after.uasset"), uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse expected asset: %v", err)
	}
	requireNameAbsent(t, actualAsset, "WBP Image 1")
	if got, want := len(actualAsset.Names), len(expectedAsset.Names); got != want {
		t.Fatalf("name count: got %d want %d", got, want)
	}
	if got, want := generatedClassWidgetVariableFieldNames(t, actualAsset), generatedClassWidgetVariableFieldNames(t, expectedAsset); !reflect.DeepEqual(got, want) {
		t.Fatalf("generated class field names: got %v want %v", got, want)
	}
	requireGeneratedClassWidgetVariableFieldCategoryLengths(t, actualAsset, "WBP_Minimum", []string{"Image_2", "Image_1"})
}

func TestRunBlueprintWidgetAddNestedOverlayImage(t *testing.T) {
	beforePath := findExistingGoldenOperationFixturePath(t, "widget_add_image_nested_overlay", "before.uasset")
	work := copyFixtureToTemp(t, beforePath)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "CanvasPanel_1/Overlay_1",
		"--type", "image",
		"--name", "Image_3",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	actualAsset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	actualBlueprintExport := mustFindSingleWidgetBlueprintExport(t, actualAsset)
	actualShape, err := captureWidgetBlueprintShape(actualAsset, actualBlueprintExport)
	if err != nil {
		t.Fatalf("capture actual widget shape: %v", err)
	}
	if !containsStringFold(actualShape["designer"], "CanvasPanel_1/Overlay_1/Image_3") {
		t.Fatalf("designer shape missing nested child: %v", actualShape["designer"])
	}
	if !widgetBlueprintShapeContainsPathOrSuffix(actualShape["generated"], "CanvasPanel_1/Overlay_1/Image_3") {
		t.Fatalf("generated shape missing nested child: %v", actualShape["generated"])
	}

	targets, err := collectWidgetWriteTargets(actualAsset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	parent, err := selectWidgetWriteTarget(targets, "CanvasPanel_1/Overlay_1")
	if err != nil {
		t.Fatalf("select parent widget: %v", err)
	}
	if got, want := len(parent.Exports), 2; got != want {
		t.Fatalf("parent export count: got %d want %d (%v)", got, want, parent.Exports)
	}
	child, err := selectWidgetWriteTarget(targets, "CanvasPanel_1/Overlay_1/Image_3")
	if err != nil {
		t.Fatalf("select nested child widget: %v", err)
	}
	if got, want := len(child.Exports), 2; got != want {
		t.Fatalf("child export count: got %d want %d (%v)", got, want, child.Exports)
	}

	expectedAsset, err := uasset.ParseFile(findExistingGoldenOperationFixturePath(t, "widget_add_image_nested_overlay", "after.uasset"), uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse expected asset: %v", err)
	}
	gotNames := generatedClassWidgetVariableFieldNames(t, actualAsset)
	wantNames := generatedClassWidgetVariableFieldNames(t, expectedAsset)
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("generated class field names: got %v want %v", gotNames, wantNames)
	}
	requireGeneratedClassWidgetVariableFieldCategoryLengths(t, actualAsset, "WBP_MultiLevelSmoke", wantNames)

	actualBytes, err := os.ReadFile(work)
	if err != nil {
		t.Fatalf("read rewritten asset bytes: %v", err)
	}
	expectedBytes, err := os.ReadFile(findExistingGoldenOperationFixturePath(t, "widget_add_image_nested_overlay", "after.uasset"))
	if err != nil {
		t.Fatalf("read expected asset bytes: %v", err)
	}
	diffExports := make([]int, 0, 4)
	for i := range actualAsset.Exports {
		actualExport := actualAsset.Exports[i]
		expectedExport := expectedAsset.Exports[i]
		actualPayload := actualBytes[actualExport.SerialOffset : actualExport.SerialOffset+actualExport.SerialSize]
		expectedPayload := expectedBytes[expectedExport.SerialOffset : expectedExport.SerialOffset+expectedExport.SerialSize]
		if !bytes.Equal(actualPayload, expectedPayload) {
			diffExports = append(diffExports, i+1)
		}
	}
	if got, want := diffExports, []int{20, 21}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected payload diff exports: got %v want %v", got, want)
	}
}

func TestRunBlueprintWidgetAddCanvasPanelTwoImagesAndNestedOverlayImageEndToEnd(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset")
	work := copyFixtureToTemp(t, path)

	runWidgetAddArgsSequence(t, nestedCanvasOverlayWidgetAddArgs(work))

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	blueprintExport := mustFindSingleWidgetBlueprintExport(t, asset)
	shape, err := captureWidgetBlueprintShape(asset, blueprintExport)
	if err != nil {
		t.Fatalf("captureWidgetBlueprintShape: %v", err)
	}
	for _, wantPath := range []string{
		"CanvasPanel_1/Image_1",
		"CanvasPanel_1/Image_2",
		"CanvasPanel_1/Overlay_1",
		"CanvasPanel_1/Overlay_1/Image_3",
	} {
		if !containsStringFold(shape["designer"], wantPath) {
			t.Fatalf("designer shape missing child path %q: %v", wantPath, shape["designer"])
		}
	}
	requireWidgetTargetMinimums(t, asset, []widgetTargetMinimumExpectation{
		{path: "CanvasPanel_1/Image_1", className: "Image", wantExportsMin: 1, wantSlotsMin: 1},
		{path: "CanvasPanel_1/Image_2", className: "Image", wantExportsMin: 1, wantSlotsMin: 1},
		{path: "CanvasPanel_1/Overlay_1", className: "Overlay", wantExportsMin: 2, wantSlotsMin: 1},
		{path: "CanvasPanel_1/Overlay_1/Image_3", className: "Image", wantExportsMin: 2, wantSlotsMin: 2},
	})

	summaries := generatedClassWidgetVariableFieldSummaries(t, asset)
	fieldOrder := make(map[string]int, len(summaries))
	for i, summary := range summaries {
		fieldOrder[summary.Name] = i
		if summary.DisplayNameLen != int32(len(summary.Name)+1) {
			t.Fatalf("generated class field %q display-name len: got %d want %d", summary.Name, summary.DisplayNameLen, len(summary.Name)+1)
		}
		if summary.CategoryLen != int32(len("WBP_Minimum")+1) {
			t.Fatalf("generated class field %q category len: got %d want %d", summary.Name, summary.CategoryLen, len("WBP_Minimum")+1)
		}
	}
	for _, name := range []string{"Image_1", "Image_2", "Image_3"} {
		if _, ok := fieldOrder[name]; !ok {
			t.Fatalf("generated class field %q missing from %v", name, generatedClassWidgetVariableFieldNames(t, asset))
		}
	}
	if _, ok := fieldOrder["Overlay_1"]; ok {
		t.Fatalf("generated class field %q should be absent from %v", "Overlay_1", generatedClassWidgetVariableFieldNames(t, asset))
	}
}

func TestRunBlueprintWidgetAddWidgetInitCanvasPanelTwoImagesAndNestedOverlayImageEndToEnd(t *testing.T) {
	work := filepath.Join(t.TempDir(), "WBP_FreshNested.uasset")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-init", work,
		"--template", "minimum",
		"--asset-name", "WBP_FreshNested",
		"--package-path", "/Game/WBP",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("widget-init exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	runWidgetAddArgsSequence(t, nestedCanvasOverlayWidgetAddArgs(work))

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	blueprintExport := mustFindSingleWidgetBlueprintExport(t, asset)
	shape, err := captureWidgetBlueprintShape(asset, blueprintExport)
	if err != nil {
		t.Fatalf("captureWidgetBlueprintShape: %v", err)
	}
	for _, wantPath := range []string{
		"CanvasPanel_1",
		"CanvasPanel_1/Image_1",
		"CanvasPanel_1/Image_2",
		"CanvasPanel_1/Overlay_1",
		"CanvasPanel_1/Overlay_1/Image_3",
	} {
		if !containsStringFold(shape["designer"], wantPath) {
			t.Fatalf("designer shape missing child path %q: %v", wantPath, shape["designer"])
		}
		if wantPath == "CanvasPanel_1/Image_1" || wantPath == "CanvasPanel_1/Image_2" {
			continue
		}
		if !containsStringFold(shape["generated"], wantPath) {
			t.Fatalf("generated shape missing child path %q: %v", wantPath, shape["generated"])
		}
	}

	requireWidgetTargetMinimums(t, asset, []widgetTargetMinimumExpectation{
		{path: "CanvasPanel_1", className: "CanvasPanel", wantExportsMin: 2, wantSlotsMin: 0},
		{path: "CanvasPanel_1/Image_1", className: "Image", wantExportsMin: 1, wantSlotsMin: 1},
		{path: "CanvasPanel_1/Image_2", className: "Image", wantExportsMin: 1, wantSlotsMin: 1},
		{path: "CanvasPanel_1/Overlay_1", className: "Overlay", wantExportsMin: 2, wantSlotsMin: 2},
		{path: "CanvasPanel_1/Overlay_1/Image_3", className: "Image", wantExportsMin: 2, wantSlotsMin: 2},
	})
	if got, want := generatedClassWidgetVariableFieldNames(t, asset), []string{"Image_1", "Image_2", "Image_3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("generated class field names: got %v want %v", got, want)
	}
	if got, want := generatedClassPropertyGuidNames(t, asset), []string{"Image_1", "Image_2", "Image_3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("generated class PropertyGuids: got %v want %v", got, want)
	}
	if got, want := widgetBlueprintGeneratedVariableNames(t, asset), []string{"Image_1", "Image_2", "Image_3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("WidgetBlueprint GeneratedVariables: got %v want %v", got, want)
	}
	if got, want := generatedTreeAllWidgetNamesWithNulls(t, asset), []string{"CanvasPanel_1", "", "", "Overlay_1", "Image_3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("generated WidgetTree AllWidgets: got %v want %v", got, want)
	}
}

func TestIgnoredWidgetVariableGuidMapValueRangesWidgetAdd(t *testing.T) {
	path := findExistingGoldenOperationFixturePath(t, "widget_add_image_canvaspanel", "before.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "CanvasPanel_22",
		"--type", "image",
		"--name", "Image_23",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	body, err := os.ReadFile(work)
	if err != nil {
		t.Fatalf("read rewritten asset: %v", err)
	}
	ranges, err := ignoredWidgetVariableGuidMapValueRanges(body, operationSpec{
		Command: "blueprint widget-add",
		Args: map[string]any{
			"parent": "CanvasPanel_22",
			"type":   "image",
			"name":   "Image_23",
		},
	})
	if err != nil {
		t.Fatalf("ignoredWidgetVariableGuidMapValueRanges: %v", err)
	}
	if len(ranges) != 3 {
		t.Fatalf("guid ignore range count: got %d want 3", len(ranges))
	}
	for i, item := range ranges {
		if got := item.end - item.start; got != 16 {
			t.Fatalf("guid ignore length[%d]: got %d want 16", i, got)
		}
	}
}

func TestRunBlueprintWidgetAddRootCanvasPanel(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "root",
		"--type", "canvaspanel",
		"--name", "CanvasPanel_22",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	root, err := selectWidgetWriteTarget(targets, "CanvasPanel_22")
	if err != nil {
		t.Fatalf("select root widget: %v", err)
	}
	if root.ClassName != "CanvasPanel" {
		t.Fatalf("root class: got %q want %q", root.ClassName, "CanvasPanel")
	}
	if len(root.Exports) != 2 {
		t.Fatalf("root exports: got %d want 2 (%v)", len(root.Exports), root.Exports)
	}
}

func TestRunBlueprintWidgetAddRootVerticalBoxAndChildTextBlock(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-add", work,
			"--parent", "root",
			"--type", "verticalbox",
			"--name", "VerticalBox_21",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "VerticalBox_21",
			"--type", "textblock",
			"--name", "TextBlock_1",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		code := Run(argv, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	root, err := selectWidgetWriteTarget(targets, "VerticalBox_21")
	if err != nil {
		t.Fatalf("select root widget: %v", err)
	}
	if root.ClassName != "VerticalBox" {
		t.Fatalf("root class: got %q want %q", root.ClassName, "VerticalBox")
	}
	child, err := selectWidgetWriteTarget(targets, "VerticalBox_21/TextBlock_1")
	if err != nil {
		t.Fatalf("select child widget: %v", err)
	}
	if child.ClassName != "TextBlock" {
		t.Fatalf("child class: got %q want %q", child.ClassName, "TextBlock")
	}
	if len(child.Exports) != 2 {
		t.Fatalf("child exports: got %d want 2 (%v)", len(child.Exports), child.Exports)
	}
	if len(child.SlotExports) != 2 {
		t.Fatalf("child slot exports: got %d want 2 (%v)", len(child.SlotExports), child.SlotExports)
	}
	if _, found := findUMGClassImport(asset, "VerticalBox"); !found {
		t.Fatalf("VerticalBox class import not found after widget-add")
	}
	if _, found := findUMGClassImport(asset, "VerticalBoxSlot"); !found {
		t.Fatalf("VerticalBoxSlot class import not found after widget-add")
	}
}

func TestRunBlueprintWidgetAddButtonUnderCanvasPanelAndTextChild(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-add", work,
			"--parent", "root",
			"--type", "canvaspanel",
			"--name", "CanvasPanel_21",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "CanvasPanel_21",
			"--type", "button",
			"--name", "Button_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "Button_1",
			"--type", "textblock",
			"--name", "TextBlock_1",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		code := Run(argv, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	button, err := selectWidgetWriteTarget(targets, "CanvasPanel_21/Button_1")
	if err != nil {
		t.Fatalf("select button widget: %v", err)
	}
	if button.ClassName != "Button" {
		t.Fatalf("button class: got %q want %q", button.ClassName, "Button")
	}
	if len(button.Exports) != 2 {
		t.Fatalf("button exports: got %d want 2 (%v)", len(button.Exports), button.Exports)
	}
	if len(button.SlotExports) != 2 {
		t.Fatalf("button slot exports: got %d want 2 (%v)", len(button.SlotExports), button.SlotExports)
	}
	textChild, err := selectWidgetWriteTarget(targets, "CanvasPanel_21/Button_1/TextBlock_1")
	if err != nil {
		t.Fatalf("select text child widget: %v", err)
	}
	if textChild.ClassName != "TextBlock" {
		t.Fatalf("text child class: got %q want %q", textChild.ClassName, "TextBlock")
	}
	if len(textChild.Exports) != 2 {
		t.Fatalf("text child exports: got %d want 2 (%v)", len(textChild.Exports), textChild.Exports)
	}
	if len(textChild.SlotExports) != 2 {
		t.Fatalf("text child slot exports: got %d want 2 (%v)", len(textChild.SlotExports), textChild.SlotExports)
	}
	if _, found := findUMGClassImport(asset, "Button"); !found {
		t.Fatalf("Button class import not found after widget-add")
	}
	if _, found := findUMGClassImport(asset, "ButtonSlot"); !found {
		t.Fatalf("ButtonSlot class import not found after widget-add")
	}
}

func TestRunBlueprintWidgetAddBorderUnderCanvasPanelAndImageChild(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-add", work,
			"--parent", "root",
			"--type", "canvaspanel",
			"--name", "CanvasPanel_21",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "CanvasPanel_21",
			"--type", "border",
			"--name", "Border_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "Border_1",
			"--type", "image",
			"--name", "Image_1",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		code := Run(argv, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	border, err := selectWidgetWriteTarget(targets, "CanvasPanel_21/Border_1")
	if err != nil {
		t.Fatalf("select border widget: %v", err)
	}
	if border.ClassName != "Border" {
		t.Fatalf("border class: got %q want %q", border.ClassName, "Border")
	}
	if got, want := len(border.Exports), 2; got != want {
		t.Fatalf("border exports: got %d want %d (%v)", got, want, border.Exports)
	}
	if got, want := len(border.SlotExports), 2; got != want {
		t.Fatalf("border slot exports: got %d want %d (%v)", got, want, border.SlotExports)
	}
	image, err := selectWidgetWriteTarget(targets, "CanvasPanel_21/Border_1/Image_1")
	if err != nil {
		t.Fatalf("select image child widget: %v", err)
	}
	if image.ClassName != "Image" {
		t.Fatalf("image child class: got %q want %q", image.ClassName, "Image")
	}
	if got, want := len(image.Exports), 2; got != want {
		t.Fatalf("image child exports: got %d want %d (%v)", got, want, image.Exports)
	}
	if got, want := len(image.SlotExports), 2; got != want {
		t.Fatalf("image child slot exports: got %d want %d (%v)", got, want, image.SlotExports)
	}
	if _, found := findUMGClassImport(asset, "Border"); !found {
		t.Fatalf("Border class import not found after widget-add")
	}
	if _, found := findUMGClassImport(asset, "BorderSlot"); !found {
		t.Fatalf("BorderSlot class import not found after widget-add")
	}
}

func TestRunBlueprintWidgetAddSingleChildParentsUnderCanvasPanel(t *testing.T) {
	tests := []struct {
		parentType  string
		parentClass string
		parentName  string
	}{
		{parentType: "sizebox", parentClass: "SizeBox", parentName: "SizeBox_1"},
		{parentType: "scalebox", parentClass: "ScaleBox", parentName: "ScaleBox_1"},
		{parentType: "backgroundblur", parentClass: "BackgroundBlur", parentName: "BackgroundBlur_1"},
		{parentType: "safezone", parentClass: "SafeZone", parentName: "SafeZone_1"},
		{parentType: "retainerbox", parentClass: "RetainerBox", parentName: "RetainerBox_1"},
		{parentType: "invalidationbox", parentClass: "InvalidationBox", parentName: "InvalidationBox_1"},
		{parentType: "menuanchor", parentClass: "MenuAnchor", parentName: "MenuAnchor_1"},
		{parentType: "checkbox", parentClass: "CheckBox", parentName: "CheckBox_1"},
		{parentType: "windowtitlebararea", parentClass: "WindowTitleBarArea", parentName: "WindowTitleBarArea_1"},
	}

	for _, tt := range tests {
		t.Run(tt.parentClass, func(t *testing.T) {
			path := findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset")
			work := copyFixtureToTemp(t, path)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			for _, argv := range [][]string{
				{
					"blueprint", "widget-add", work,
					"--parent", "root",
					"--type", "canvaspanel",
					"--name", "CanvasPanel_21",
				},
				{
					"blueprint", "widget-add", work,
					"--parent", "CanvasPanel_21",
					"--type", tt.parentType,
					"--name", tt.parentName,
				},
				{
					"blueprint", "widget-add", work,
					"--parent", tt.parentName,
					"--type", "image",
					"--name", "Image_1",
				},
			} {
				stdout.Reset()
				stderr.Reset()
				code := Run(argv, &stdout, &stderr)
				if code != 0 {
					t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
				}
			}

			asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("parse rewritten asset: %v", err)
			}
			targets, err := collectWidgetWriteTargets(asset, 0)
			if err != nil {
				t.Fatalf("collectWidgetWriteTargets: %v", err)
			}
			parent, err := selectWidgetWriteTarget(targets, "CanvasPanel_21/"+tt.parentName)
			if err != nil {
				t.Fatalf("select parent widget: %v", err)
			}
			if parent.ClassName != tt.parentClass {
				t.Fatalf("parent class: got %q want %q", parent.ClassName, tt.parentClass)
			}
			child, err := selectWidgetWriteTarget(targets, "CanvasPanel_21/"+tt.parentName+"/Image_1")
			if err != nil {
				t.Fatalf("select child widget: %v", err)
			}
			if child.ClassName != "Image" {
				t.Fatalf("child class: got %q want %q", child.ClassName, "Image")
			}
			if len(child.Exports) != 2 {
				t.Fatalf("child exports: got %d want 2 (%v)", len(child.Exports), child.Exports)
			}
			if len(child.SlotExports) != 2 {
				t.Fatalf("child slot exports: got %d want 2 (%v)", len(child.SlotExports), child.SlotExports)
			}
			if _, found := findUMGClassImport(asset, tt.parentClass); !found {
				t.Fatalf("%s class import not found after widget-add", tt.parentClass)
			}
			slotClass, err := widgetAddSlotClassName(tt.parentClass)
			if err != nil {
				t.Fatalf("widgetAddSlotClassName(%q): %v", tt.parentClass, err)
			}
			if _, found := findUMGClassImport(asset, slotClass); !found {
				t.Fatalf("%s class import not found after widget-add", slotClass)
			}
		})
	}
}

func TestRunBlueprintWidgetAddSingleChildRootWidgetsAcceptOneImage(t *testing.T) {
	tests := []struct {
		rootType  string
		rootClass string
		rootName  string
	}{
		{rootType: "button", rootClass: "Button", rootName: "Button_1"},
		{rootType: "checkbox", rootClass: "CheckBox", rootName: "CheckBox_1"},
		{rootType: "border", rootClass: "Border", rootName: "Border_1"},
		{rootType: "retainerbox", rootClass: "RetainerBox", rootName: "RetainerBox_1"},
		{rootType: "invalidationbox", rootClass: "InvalidationBox", rootName: "InvalidationBox_1"},
		{rootType: "menuanchor", rootClass: "MenuAnchor", rootName: "MenuAnchor_1"},
		{rootType: "sizebox", rootClass: "SizeBox", rootName: "SizeBox_1"},
		{rootType: "scalebox", rootClass: "ScaleBox", rootName: "ScaleBox_1"},
		{rootType: "backgroundblur", rootClass: "BackgroundBlur", rootName: "BackgroundBlur_1"},
		{rootType: "safezone", rootClass: "SafeZone", rootName: "SafeZone_1"},
		{rootType: "windowtitlebararea", rootClass: "WindowTitleBarArea", rootName: "WindowTitleBarArea_1"},
	}

	for _, tt := range tests {
		t.Run(tt.rootClass, func(t *testing.T) {
			path := findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset")
			work := copyFixtureToTemp(t, path)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			for _, argv := range [][]string{
				{
					"blueprint", "widget-add", work,
					"--parent", "root",
					"--type", tt.rootType,
					"--name", tt.rootName,
				},
				{
					"blueprint", "widget-add", work,
					"--parent", tt.rootName,
					"--type", "image",
					"--name", "Image_1",
				},
			} {
				stdout.Reset()
				stderr.Reset()
				code := Run(argv, &stdout, &stderr)
				if code != 0 {
					t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
				}
			}

			asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("parse rewritten asset: %v", err)
			}
			targets, err := collectWidgetWriteTargets(asset, 0)
			if err != nil {
				t.Fatalf("collectWidgetWriteTargets: %v", err)
			}
			root, err := selectWidgetWriteTarget(targets, tt.rootName)
			if err != nil {
				t.Fatalf("select root widget: %v", err)
			}
			if root.ClassName != tt.rootClass {
				t.Fatalf("root class: got %q want %q", root.ClassName, tt.rootClass)
			}
			child, err := selectWidgetWriteTarget(targets, tt.rootName+"/Image_1")
			if err != nil {
				t.Fatalf("select child widget: %v", err)
			}
			if child.ClassName != "Image" {
				t.Fatalf("child class: got %q want %q", child.ClassName, "Image")
			}
			if len(child.Exports) != 2 {
				t.Fatalf("child exports: got %d want 2 (%v)", len(child.Exports), child.Exports)
			}
			if len(child.SlotExports) != 2 {
				t.Fatalf("child slot exports: got %d want 2 (%v)", len(child.SlotExports), child.SlotExports)
			}
			if _, found := findUMGClassImport(asset, tt.rootClass); !found {
				t.Fatalf("%s class import not found after widget-add", tt.rootClass)
			}
			slotClass, err := widgetAddSlotClassName(tt.rootClass)
			if err != nil {
				t.Fatalf("widgetAddSlotClassName(%q): %v", tt.rootClass, err)
			}
			if _, found := findUMGClassImport(asset, slotClass); !found {
				t.Fatalf("%s class import not found after widget-add", slotClass)
			}
		})
	}
}

func TestRunBlueprintWidgetAddMultiChildRootPanelsAcceptMultipleImages(t *testing.T) {
	tests := []struct {
		rootType  string
		rootClass string
		rootName  string
	}{
		{rootType: "scrollbox", rootClass: "ScrollBox", rootName: "ScrollBox_1"},
		{rootType: "wrapbox", rootClass: "WrapBox", rootName: "WrapBox_1"},
		{rootType: "stackbox", rootClass: "StackBox", rootName: "StackBox_1"},
		{rootType: "gridpanel", rootClass: "GridPanel", rootName: "GridPanel_1"},
		{rootType: "uniformgridpanel", rootClass: "UniformGridPanel", rootName: "UniformGridPanel_1"},
		{rootType: "widgetswitcher", rootClass: "WidgetSwitcher", rootName: "WidgetSwitcher_1"},
	}

	for _, tt := range tests {
		t.Run(tt.rootClass, func(t *testing.T) {
			path := findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset")
			work := copyFixtureToTemp(t, path)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			for _, argv := range [][]string{
				{
					"blueprint", "widget-add", work,
					"--parent", "root",
					"--type", tt.rootType,
					"--name", tt.rootName,
				},
				{
					"blueprint", "widget-add", work,
					"--parent", tt.rootName,
					"--type", "image",
					"--name", "Image_1",
				},
				{
					"blueprint", "widget-add", work,
					"--parent", tt.rootName,
					"--type", "image",
					"--name", "Image_2",
				},
			} {
				stdout.Reset()
				stderr.Reset()
				code := Run(argv, &stdout, &stderr)
				if code != 0 {
					t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
				}
			}

			asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("parse rewritten asset: %v", err)
			}
			targets, err := collectWidgetWriteTargets(asset, 0)
			if err != nil {
				t.Fatalf("collectWidgetWriteTargets: %v", err)
			}
			root, err := selectWidgetWriteTarget(targets, tt.rootName)
			if err != nil {
				t.Fatalf("select root widget: %v", err)
			}
			if root.ClassName != tt.rootClass {
				t.Fatalf("root class: got %q want %q", root.ClassName, tt.rootClass)
			}
			for _, childName := range []string{"Image_1", "Image_2"} {
				child, err := selectWidgetWriteTarget(targets, tt.rootName+"/"+childName)
				if err != nil {
					t.Fatalf("select child widget %s: %v", childName, err)
				}
				if child.ClassName != "Image" {
					t.Fatalf("%s class: got %q want %q", childName, child.ClassName, "Image")
				}
				if len(child.Exports) != 2 {
					t.Fatalf("%s exports: got %d want 2 (%v)", childName, len(child.Exports), child.Exports)
				}
				if len(child.SlotExports) != 2 {
					t.Fatalf("%s slot exports: got %d want 2 (%v)", childName, len(child.SlotExports), child.SlotExports)
				}
			}
			if _, found := findUMGClassImport(asset, tt.rootClass); !found {
				t.Fatalf("%s class import not found after widget-add", tt.rootClass)
			}
			slotClass, err := widgetAddSlotClassName(tt.rootClass)
			if err != nil {
				t.Fatalf("widgetAddSlotClassName(%q): %v", tt.rootClass, err)
			}
			if _, found := findUMGClassImport(asset, slotClass); !found {
				t.Fatalf("%s class import not found after widget-add", slotClass)
			}
		})
	}
}

func TestRunBlueprintWidgetAddStackBoxUnderCanvasPanelAndTwoImages(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-add", work,
			"--parent", "root",
			"--type", "canvaspanel",
			"--name", "CanvasPanel_21",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "CanvasPanel_21",
			"--type", "stackbox",
			"--name", "StackBox_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "StackBox_1",
			"--type", "image",
			"--name", "Image_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "StackBox_1",
			"--type", "image",
			"--name", "Image_2",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		code := Run(argv, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	stackBox, err := selectWidgetWriteTarget(targets, "CanvasPanel_21/StackBox_1")
	if err != nil {
		t.Fatalf("select stack box widget: %v", err)
	}
	if stackBox.ClassName != "StackBox" {
		t.Fatalf("stack box class: got %q want %q", stackBox.ClassName, "StackBox")
	}
	for _, childName := range []string{"Image_1", "Image_2"} {
		child, err := selectWidgetWriteTarget(targets, "CanvasPanel_21/StackBox_1/"+childName)
		if err != nil {
			t.Fatalf("select child widget %s: %v", childName, err)
		}
		if child.ClassName != "Image" {
			t.Fatalf("%s class: got %q want %q", childName, child.ClassName, "Image")
		}
		if len(child.Exports) != 2 {
			t.Fatalf("%s exports: got %d want 2 (%v)", childName, len(child.Exports), child.Exports)
		}
		if len(child.SlotExports) != 2 {
			t.Fatalf("%s slot exports: got %d want 2 (%v)", childName, len(child.SlotExports), child.SlotExports)
		}
	}
	if _, found := findUMGClassImport(asset, "StackBox"); !found {
		t.Fatalf("StackBox class import not found after widget-add")
	}
	if _, found := findUMGClassImport(asset, "StackBoxSlot"); !found {
		t.Fatalf("StackBoxSlot class import not found after widget-add")
	}
}

func TestRunBlueprintWidgetAddScaleBoxOverlayVerticalBoxAndTwoImages(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-add", work,
			"--parent", "root",
			"--type", "scalebox",
			"--name", "ScaleBox_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "ScaleBox_1",
			"--type", "overlay",
			"--name", "Overlay_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "Overlay_1",
			"--type", "verticalbox",
			"--name", "VerticalBox_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "VerticalBox_1",
			"--type", "image",
			"--name", "Image_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "VerticalBox_1",
			"--type", "image",
			"--name", "Image_2",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		code := Run(argv, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	for _, tc := range []struct {
		path      string
		className string
	}{
		{path: "ScaleBox_1", className: "ScaleBox"},
		{path: "ScaleBox_1/Overlay_1", className: "Overlay"},
		{path: "ScaleBox_1/Overlay_1/VerticalBox_1", className: "VerticalBox"},
		{path: "ScaleBox_1/Overlay_1/VerticalBox_1/Image_1", className: "Image"},
		{path: "ScaleBox_1/Overlay_1/VerticalBox_1/Image_2", className: "Image"},
	} {
		target, err := selectWidgetWriteTarget(targets, tc.path)
		if err != nil {
			t.Fatalf("select widget %s: %v", tc.path, err)
		}
		if target.ClassName != tc.className {
			t.Fatalf("%s class: got %q want %q", tc.path, target.ClassName, tc.className)
		}
	}
}

func TestRunBlueprintWidgetAddRejectsSecondChildUnderButton(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-add", work,
			"--parent", "root",
			"--type", "canvaspanel",
			"--name", "CanvasPanel_21",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "CanvasPanel_21",
			"--type", "button",
			"--name", "Button_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "Button_1",
			"--type", "textblock",
			"--name", "TextBlock_1",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		code := Run(argv, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("setup exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	code := Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "Button_1",
		"--type", "image",
		"--name", "Image_1",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1 stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "accepts only one direct child") {
		t.Fatalf("expected single-child parent error, got: %s", stderr.String())
	}
}

func TestRunBlueprintWidgetAddRootHorizontalBoxAndTwoImages(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-add", work,
			"--parent", "root",
			"--type", "horizontalbox",
			"--name", "HorizontalBox_21",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "HorizontalBox_21",
			"--type", "image",
			"--name", "Image_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "HorizontalBox_21",
			"--type", "image",
			"--name", "Image_2",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		code := Run(argv, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	for _, path := range []string{"HorizontalBox_21/Image_1", "HorizontalBox_21/Image_2"} {
		child, err := selectWidgetWriteTarget(targets, path)
		if err != nil {
			t.Fatalf("select child widget %s: %v", path, err)
		}
		if child.ClassName != "Image" {
			t.Fatalf("%s class: got %q want %q", path, child.ClassName, "Image")
		}
		if len(child.Exports) != 2 {
			t.Fatalf("%s exports: got %d want 2 (%v)", path, len(child.Exports), child.Exports)
		}
		if len(child.SlotExports) != 2 {
			t.Fatalf("%s slot exports: got %d want 2 (%v)", path, len(child.SlotExports), child.SlotExports)
		}
	}
}

func TestRunBlueprintWidgetAddMatchesFixtureForNewOperationCases(t *testing.T) {
	cases := []struct {
		name      string
		opName    string
		argv      []string
		selectors []string
	}{
		{
			name:   "root_horizontalbox",
			opName: "widget_add_root_horizontalbox",
			argv: []string{
				"blueprint", "widget-add",
				"--parent", "root",
				"--type", "horizontalbox",
				"--name", "HorizontalBox_21",
			},
			selectors: []string{"HorizontalBox_21"},
		},
		{
			name:   "image_under_border",
			opName: "widget_add_image_border_canvaspanel",
			argv: []string{
				"blueprint", "widget-add",
				"--parent", "CanvasPanel_1/Border_1",
				"--type", "image",
				"--name", "Image_1",
			},
			selectors: []string{
				"CanvasPanel_1/Border_1",
				"CanvasPanel_1/Border_1/Image_1",
			},
		},
		{
			name:   "image_under_sizebox",
			opName: "widget_add_image_sizebox_canvaspanel",
			argv: []string{
				"blueprint", "widget-add",
				"--parent", "CanvasPanel_1/SizeBox_1",
				"--type", "image",
				"--name", "Image_1",
			},
			selectors: []string{
				"CanvasPanel_1/SizeBox_1",
				"CanvasPanel_1/SizeBox_1/Image_1",
			},
		},
		{
			name:   "image_under_horizontalbox",
			opName: "widget_add_image_horizontalbox_canvaspanel",
			argv: []string{
				"blueprint", "widget-add",
				"--parent", "CanvasPanel_1/HorizontalBox_1",
				"--type", "image",
				"--name", "Image_1",
			},
			selectors: []string{
				"CanvasPanel_1/HorizontalBox_1",
				"CanvasPanel_1/HorizontalBox_1/Image_1",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixtureDir := findExistingGoldenOperationFixtureDir(t, tc.opName)
			work := copyFixtureToTemp(t, filepath.Join(fixtureDir, "before.uasset"))

			argv := append([]string(nil), tc.argv...)
			argv = append(argv[:2], append([]string{work}, argv[2:]...)...)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(argv, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
			}

			requireWidgetAddFixtureGraphMatch(t, work, filepath.Join(fixtureDir, "after.uasset"), tc.selectors)
		})
	}
}

func copyFixtureToTemp(t *testing.T, source string) string {
	t.Helper()
	body, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), filepath.Base(source))
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write temp fixture: %v", err)
	}
	return path
}

func findExistingGoldenParseFixturePath(t *testing.T, name string) string {
	t.Helper()
	roots := goldenFixtureRoots(t, "parse")
	for i := len(roots) - 1; i >= 0; i-- {
		path := filepath.Join(roots[i], "parse", name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	t.Fatalf("parse fixture not found: %s", name)
	return ""
}

func findExistingGoldenOperationFixturePath(t *testing.T, opName, name string) string {
	t.Helper()
	roots := goldenFixtureRoots(t, "operations")
	for i := len(roots) - 1; i >= 0; i-- {
		path := filepath.Join(roots[i], "operations", opName, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	t.Fatalf("operation fixture not found: %s/%s", opName, name)
	return ""
}

func findExistingGoldenOperationFixtureDir(t *testing.T, opName string) string {
	t.Helper()
	roots := goldenFixtureRoots(t, "operations")
	for i := len(roots) - 1; i >= 0; i-- {
		path := filepath.Join(roots[i], "operations", opName)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}
	t.Fatalf("operation fixture dir not found: %s", opName)
	return ""
}

type widgetTargetFixtureSignature struct {
	ClassName   string
	ExportNames []string
	SlotNames   []string
}

type exportSequenceSignature struct {
	ClassName string
	Object    string
	Outer     string
}

type generatedClassWidgetVariableFieldSummary struct {
	Name           string
	DisplayNameLen int32
	CategoryLen    int32
}

func requireWidgetAddFixtureGraphMatch(t *testing.T, actualPath, expectedPath string, selectors []string) {
	t.Helper()

	actualAsset, err := uasset.ParseFile(actualPath, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse actual asset: %v", err)
	}
	expectedAsset, err := uasset.ParseFile(expectedPath, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse expected asset: %v", err)
	}

	if len(actualAsset.Exports) != len(expectedAsset.Exports) {
		t.Fatalf("export count mismatch: got %d want %d", len(actualAsset.Exports), len(expectedAsset.Exports))
	}
	if !reflect.DeepEqual(exportSequenceSignatureFromAsset(actualAsset), exportSequenceSignatureFromAsset(expectedAsset)) {
		t.Fatalf("export sequence mismatch:\nactual=%v\nexpected=%v", exportSequenceSignatureFromAsset(actualAsset), exportSequenceSignatureFromAsset(expectedAsset))
	}

	actualBlueprintExport := mustFindSingleWidgetBlueprintExport(t, actualAsset)
	expectedBlueprintExport := mustFindSingleWidgetBlueprintExport(t, expectedAsset)

	actualShape, err := captureWidgetBlueprintShape(actualAsset, actualBlueprintExport)
	if err != nil {
		t.Fatalf("capture actual widget shape: %v", err)
	}
	expectedShape, err := captureWidgetBlueprintShape(expectedAsset, expectedBlueprintExport)
	if err != nil {
		t.Fatalf("capture expected widget shape: %v", err)
	}
	if !reflect.DeepEqual(actualShape, expectedShape) {
		t.Fatalf("widget shape mismatch:\nactual=%v\nexpected=%v", actualShape, expectedShape)
	}
	if !reflect.DeepEqual(widgetBlueprintFixtureStateSignature(t, actualAsset, actualBlueprintExport), widgetBlueprintFixtureStateSignature(t, expectedAsset, expectedBlueprintExport)) {
		t.Fatalf("widget blueprint state mismatch:\nactual=%v\nexpected=%v", widgetBlueprintFixtureStateSignature(t, actualAsset, actualBlueprintExport), widgetBlueprintFixtureStateSignature(t, expectedAsset, expectedBlueprintExport))
	}

	actualTargets, err := collectWidgetWriteTargets(actualAsset, 0)
	if err != nil {
		t.Fatalf("collect actual targets: %v", err)
	}
	expectedTargets, err := collectWidgetWriteTargets(expectedAsset, 0)
	if err != nil {
		t.Fatalf("collect expected targets: %v", err)
	}

	for _, selector := range selectors {
		actualTarget, err := selectWidgetWriteTarget(actualTargets, selector)
		if err != nil {
			t.Fatalf("select actual target %q: %v", selector, err)
		}
		expectedTarget, err := selectWidgetWriteTarget(expectedTargets, selector)
		if err != nil {
			t.Fatalf("select expected target %q: %v", selector, err)
		}
		actualSig := widgetTargetFixtureSignatureFromAsset(actualAsset, *actualTarget)
		expectedSig := widgetTargetFixtureSignatureFromAsset(expectedAsset, *expectedTarget)
		if !reflect.DeepEqual(actualSig, expectedSig) {
			t.Fatalf("target signature mismatch for %q:\nactual=%+v\nexpected=%+v", selector, actualSig, expectedSig)
		}
	}
}

func mustFindSingleWidgetBlueprintExport(t *testing.T, asset *uasset.Asset) int {
	t.Helper()
	exports := findWidgetBlueprintExports(asset)
	if len(exports) != 1 {
		t.Fatalf("widget blueprint export count: got %d want 1", len(exports))
	}
	return exports[0]
}

func widgetTargetFixtureSignatureFromAsset(asset *uasset.Asset, target widgetWriteTarget) widgetTargetFixtureSignature {
	out := widgetTargetFixtureSignature{
		ClassName:   target.ClassName,
		ExportNames: make([]string, 0, len(target.Exports)),
		SlotNames:   make([]string, 0, len(target.SlotExports)),
	}
	for _, exportIdx := range target.Exports {
		out.ExportNames = append(out.ExportNames, asset.Exports[exportIdx].ObjectName.Display(asset.Names))
	}
	for _, exportIdx := range target.SlotExports {
		out.SlotNames = append(out.SlotNames, asset.Exports[exportIdx].ObjectName.Display(asset.Names))
	}
	return out
}

func exportSequenceSignatureFromAsset(asset *uasset.Asset) []exportSequenceSignature {
	out := make([]exportSequenceSignature, 0, len(asset.Exports))
	for _, exp := range asset.Exports {
		outer := ""
		outerIdx := resolveOuterExportIndex(asset, exp)
		if outerIdx >= 0 && outerIdx < len(asset.Exports) {
			outer = asset.Exports[outerIdx].ObjectName.Display(asset.Names)
		}
		out = append(out, exportSequenceSignature{
			ClassName: asset.ResolveClassName(exp),
			Object:    exp.ObjectName.Display(asset.Names),
			Outer:     outer,
		})
	}
	return out
}

func requireNameAbsent(t *testing.T, asset *uasset.Asset, name string) {
	t.Helper()
	for _, entry := range asset.Names {
		if entry.Value == name {
			t.Fatalf("unexpected name entry found: %q", name)
		}
	}
}

type widgetTargetMinimumExpectation struct {
	path           string
	className      string
	wantExportsMin int
	wantSlotsMin   int
}

func nestedCanvasOverlayWidgetAddArgs(work string) [][]string {
	return [][]string{
		{
			"blueprint", "widget-add", work,
			"--parent", "root",
			"--type", "canvaspanel",
			"--name", "CanvasPanel_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "CanvasPanel_1",
			"--type", "image",
			"--name", "Image_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "CanvasPanel_1",
			"--type", "image",
			"--name", "Image_2",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "CanvasPanel_1",
			"--type", "overlay",
			"--name", "Overlay_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "CanvasPanel_1/Overlay_1",
			"--type", "image",
			"--name", "Image_3",
		},
	}
}

func runWidgetAddArgsSequence(t *testing.T, argvs [][]string) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range argvs {
		stdout.Reset()
		stderr.Reset()
		code := Run(argv, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}
}

func requireWidgetTargetMinimums(t *testing.T, asset *uasset.Asset, expectations []widgetTargetMinimumExpectation) {
	t.Helper()
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	for _, tc := range expectations {
		target, err := selectWidgetWriteTarget(targets, tc.path)
		if err != nil {
			t.Fatalf("select widget %s: %v", tc.path, err)
		}
		if target.ClassName != tc.className {
			t.Fatalf("%s class: got %q want %q", tc.path, target.ClassName, tc.className)
		}
		if got := len(target.Exports); got < tc.wantExportsMin {
			t.Fatalf("%s exports: got %d want at least %d (%v)", tc.path, got, tc.wantExportsMin, target.Exports)
		}
		if got := len(target.SlotExports); got < tc.wantSlotsMin {
			t.Fatalf("%s slot exports: got %d want at least %d (%v)", tc.path, got, tc.wantSlotsMin, target.SlotExports)
		}
	}
}

func generatedClassWidgetVariableFieldNames(t *testing.T, asset *uasset.Asset) []string {
	t.Helper()
	summaries := generatedClassWidgetVariableFieldSummaries(t, asset)
	names := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		names = append(names, summary.Name)
	}
	return names
}

func generatedClassPropertyGuidNames(t *testing.T, asset *uasset.Asset) []string {
	t.Helper()
	blueprintExport := mustFindSingleWidgetBlueprintExport(t, asset)
	props := asset.ParseExportProperties(blueprintExport)
	decoded := decodeAllProperties(asset, props.Properties)
	generatedClassExport := widgetExportIndexFromDecoded(decoded["GeneratedClass"]) - 1
	if generatedClassExport < 0 || generatedClassExport >= len(asset.Exports) {
		t.Fatalf("generated class export out of range: %d", generatedClassExport+1)
	}
	return propertyGuidNamesForExport(t, asset, generatedClassExport)
}

func widgetBlueprintGeneratedVariableNames(t *testing.T, asset *uasset.Asset) []string {
	t.Helper()
	blueprintExport := mustFindSingleWidgetBlueprintExport(t, asset)
	return generatedVariableNamesForExport(t, asset, blueprintExport)
}

func generatedTreeAllWidgetNamesWithNulls(t *testing.T, asset *uasset.Asset) []string {
	t.Helper()
	return widgetTreeAllWidgetNamesWithNullsForRole(t, asset, "generated")
}

func widgetTreeAllWidgetNamesWithNullsForRole(t *testing.T, asset *uasset.Asset, role string) []string {
	t.Helper()
	blueprintExport := mustFindSingleWidgetBlueprintExport(t, asset)
	props := asset.ParseExportProperties(blueprintExport)
	decoded := decodeAllProperties(asset, props.Properties)
	generatedClassExport := widgetExportIndexFromDecoded(decoded["GeneratedClass"]) - 1
	if generatedClassExport < 0 || generatedClassExport >= len(asset.Exports) {
		t.Fatalf("generated class export out of range: %d", generatedClassExport+1)
	}
	treeExports := findWidgetTreeExports(asset, blueprintExport, generatedClassExport)
	for _, treeExport := range treeExports {
		treeExp := asset.Exports[treeExport]
		outerIdx := resolveOuterExportIndex(asset, treeExp)
		outerClassName := ""
		if outerIdx >= 0 && outerIdx < len(asset.Exports) {
			outerClassName = asset.ResolveClassName(asset.Exports[outerIdx])
		}
		if widgetTreeRole(blueprintExport, generatedClassExport, outerIdx, outerClassName) != role {
			continue
		}
		values, err := readWidgetAddObjectRefArrayValue(asset, treeExport, "AllWidgets")
		if err != nil {
			t.Fatalf("read %s WidgetTree AllWidgets: %v", role, err)
		}
		names := make([]string, 0, len(values))
		for _, item := range values {
			exportIndex, ok := widgetAddDecodedObjectRefExportIndex(item)
			if !ok || exportIndex < 0 {
				names = append(names, "")
				continue
			}
			names = append(names, asset.Exports[exportIndex].ObjectName.Display(asset.Names))
		}
		return names
	}
	t.Fatalf("%s WidgetTree not found", role)
	return nil
}

func indexOfStringLocal(items []string, want string) int {
	for i, item := range items {
		if item == want {
			return i
		}
	}
	return -1
}

func generatedClassWidgetVariableFieldSummaries(t *testing.T, asset *uasset.Asset) []generatedClassWidgetVariableFieldSummary {
	t.Helper()
	blueprintExport := mustFindSingleWidgetBlueprintExport(t, asset)
	props := asset.ParseExportProperties(blueprintExport)
	decoded := decodeAllProperties(asset, props.Properties)
	generatedClassExport := widgetExportIndexFromDecoded(decoded["GeneratedClass"]) - 1
	if generatedClassExport < 0 || generatedClassExport >= len(asset.Exports) {
		t.Fatalf("generated class export out of range: %d", generatedClassExport+1)
	}
	exp := asset.Exports[generatedClassExport]
	payload := asset.Raw.Bytes[int(exp.SerialOffset):int(exp.SerialOffset+exp.SerialSize)]
	tail := payload[int(exp.ScriptSerializationEndOffset):]
	if len(tail) < 16 {
		t.Fatalf("generated class tail too short: %d", len(tail))
	}
	order := packageByteOrder(asset)
	fieldCount := int(order.Uint32(tail[12:16]))
	offset := 16
	summaries := make([]generatedClassWidgetVariableFieldSummary, 0, fieldCount)
	for i := 0; i < fieldCount; i++ {
		if len(tail[offset:]) < 28 {
			t.Fatalf("generated class field %d header out of bounds", i)
		}
		summary := generatedClassWidgetVariableFieldSummary{
			Name: uasset.NameRef{
				Index:  int32(order.Uint32(tail[offset+8 : offset+12])),
				Number: int32(order.Uint32(tail[offset+12 : offset+16])),
			}.Display(asset.Names),
		}
		metaCount := int(order.Uint32(tail[offset+24 : offset+28]))
		cursor := offset + 28
		for j := 0; j < metaCount; j++ {
			if len(tail[cursor:]) < 8 {
				t.Fatalf("generated class field %d metadata %d key out of bounds", i, j)
			}
			keyRef := uasset.NameRef{
				Index:  int32(order.Uint32(tail[cursor : cursor+4])),
				Number: int32(order.Uint32(tail[cursor+4 : cursor+8])),
			}
			cursor += 8
			strLen, consumed, err := generatedClassFieldFStringLength(tail[cursor:], order)
			if err != nil {
				t.Fatalf("generated class field %d metadata %d string length: %v", i, j, err)
			}
			switch keyRef.Display(asset.Names) {
			case "DisplayName":
				summary.DisplayNameLen = strLen
			case "Category":
				summary.CategoryLen = strLen
			}
			cursor += consumed
		}
		recordLen, err := generatedClassWidgetVariableFieldRecordLength(tail[offset:], order)
		if err != nil {
			t.Fatalf("generated class field %d length: %v", i, err)
		}
		summaries = append(summaries, summary)
		offset += recordLen
	}
	return summaries
}

func requireGeneratedClassWidgetVariableFieldCategoryLengths(t *testing.T, asset *uasset.Asset, blueprintObjectName string, expectedNames []string) {
	t.Helper()
	summaries := generatedClassWidgetVariableFieldSummaries(t, asset)
	if got, want := len(summaries), len(expectedNames); got != want {
		t.Fatalf("generated class field count: got %d want %d", got, want)
	}
	wantCategoryLen := int32(len(blueprintObjectName) + 1)
	for i, summary := range summaries {
		if summary.Name != expectedNames[i] {
			t.Fatalf("generated class field %d name: got %q want %q", i, summary.Name, expectedNames[i])
		}
		if summary.DisplayNameLen != int32(len(summary.Name)+1) {
			t.Fatalf("generated class field %q display-name len: got %d want %d", summary.Name, summary.DisplayNameLen, len(summary.Name)+1)
		}
		if summary.CategoryLen != wantCategoryLen {
			t.Fatalf("generated class field %q category len: got %d want %d", summary.Name, summary.CategoryLen, wantCategoryLen)
		}
	}
}

func widgetBlueprintFixtureStateSignature(t *testing.T, asset *uasset.Asset, blueprintExport int) map[string]any {
	t.Helper()
	props := asset.ParseExportProperties(blueprintExport)
	decoded := decodeAllProperties(asset, props.Properties)

	categoryNames := []string{}
	if categorySorting, ok := decoded["CategorySorting"].(map[string]any); ok {
		if items, err := anySliceLocal(categorySorting["value"]); err == nil {
			for _, item := range items {
				if wrapped, ok := item.(map[string]any); ok {
					if value, ok := wrapped["value"].(map[string]any); ok {
						if name, ok := value["name"].(string); ok {
							categoryNames = append(categoryNames, strings.TrimSpace(name))
						}
					}
				}
			}
		}
	}

	return map[string]any{
		"categorySorting":   categoryNames,
		"generatedVarNames": generatedVariableNamesForExport(t, asset, blueprintExport),
	}
}

func generatedVariableNamesForExport(t *testing.T, asset *uasset.Asset, exportIndex int) []string {
	t.Helper()
	props := asset.ParseExportProperties(exportIndex)
	decoded := decodeAllProperties(asset, props.Properties)
	generatedVariableNames := []string{}
	generatedVars, ok := decoded["GeneratedVariables"].(map[string]any)
	if !ok {
		return generatedVariableNames
	}
	items, err := anySliceLocal(generatedVars["value"])
	if err != nil {
		t.Fatalf("GeneratedVariables items: %v", err)
	}
	for _, item := range items {
		wrapped, ok := item.(map[string]any)
		if !ok {
			continue
		}
		value, ok := wrapped["value"].(map[string]any)
		if !ok {
			continue
		}
		fields, ok := value["value"].(map[string]any)
		if !ok {
			continue
		}
		varName, ok := fields["VarName"].(map[string]any)
		if !ok {
			continue
		}
		nameValue, ok := varName["value"].(map[string]any)
		if !ok {
			continue
		}
		if name, ok := nameValue["name"].(string); ok {
			generatedVariableNames = append(generatedVariableNames, name)
		}
	}
	return generatedVariableNames
}

func propertyGuidNamesForExport(t *testing.T, asset *uasset.Asset, exportIndex int) []string {
	t.Helper()
	props := asset.ParseExportProperties(exportIndex)
	decoded := decodeAllProperties(asset, props.Properties)
	propertyGuidNames := []string{}
	propertyGuids, ok := decoded["PropertyGuids"].(map[string]any)
	if !ok {
		return propertyGuidNames
	}
	items, err := anySliceLocal(propertyGuids["value"])
	if err != nil {
		t.Fatalf("PropertyGuids items: %v", err)
	}
	for _, item := range items {
		wrapped, ok := item.(map[string]any)
		if !ok {
			continue
		}
		key, ok := wrapped["key"].(map[string]any)
		if !ok {
			continue
		}
		value, ok := key["value"].(map[string]any)
		if !ok {
			continue
		}
		if name, ok := value["name"].(string); ok {
			propertyGuidNames = append(propertyGuidNames, name)
		}
	}
	return propertyGuidNames
}
