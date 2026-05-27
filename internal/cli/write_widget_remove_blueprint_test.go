package cli

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

func TestRunBlueprintWidgetRemoveRichTextBlockUnderCanvasPanel(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_CanvasPanel_RichTextBlock.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-remove", work,
		"--widget", "CanvasPanel_22/RichTextBlock_31",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("decode stdout json: %v", err)
	}
	if got := int(resp["removedExportCount"].(float64)); got != 4 {
		t.Fatalf("removedExportCount: got %d want 4", got)
	}
	if got := int(resp["removedImportCount"].(float64)); got != 2 {
		t.Fatalf("removedImportCount: got %d want 2", got)
	}
	if got, ok := resp["compactedExportEntries"].(bool); !ok || !got {
		t.Fatalf("compactedExportEntries: got %v want true", resp["compactedExportEntries"])
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	if got := len(asset.Exports); got != 11 {
		t.Fatalf("export count: got %d want 11", got)
	}
	if got := len(asset.Imports); got != 19 {
		t.Fatalf("import count: got %d want 19", got)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("logical widget count: got %d want 1", len(targets))
	}
	root, err := selectWidgetWriteTarget(targets, "CanvasPanel_22")
	if err != nil {
		t.Fatalf("select root widget: %v", err)
	}
	if root.ClassName != "CanvasPanel" {
		t.Fatalf("root class: got %q want %q", root.ClassName, "CanvasPanel")
	}
	if len(root.SlotExports) != 0 {
		t.Fatalf("root slot exports: got %d want 0 (%v)", len(root.SlotExports), root.SlotExports)
	}
	if _, err := selectWidgetWriteTarget(targets, "CanvasPanel_22/RichTextBlock_31"); err == nil {
		t.Fatalf("expected removed child widget to be absent from logical targets")
	}
	for _, exportIdx := range root.Exports {
		requireWidgetRemovePropertyMissing(t, asset, exportIdx, "Slots")
	}

	guidMap, err := decodeExportRootPropertyValue(asset, root.BlueprintExport, "WidgetVariableNameToGuidMap")
	if err != nil {
		t.Fatalf("decode WidgetVariableNameToGuidMap: %v", err)
	}
	guidMapValue, ok := guidMap.(map[string]any)
	if !ok {
		t.Fatalf("WidgetVariableNameToGuidMap type: got %T want map", guidMap)
	}
	guidItems, err := anySliceLocal(guidMapValue["value"])
	if err != nil {
		t.Fatalf("WidgetVariableNameToGuidMap items: %v", err)
	}
	if len(guidItems) != 1 {
		t.Fatalf("WidgetVariableNameToGuidMap count: got %d want 1", len(guidItems))
	}
	entry, ok := guidItems[0].(map[string]any)
	if !ok {
		t.Fatalf("guid map entry type: got %T want map", guidItems[0])
	}
	keyValue, ok := entry["key"].(map[string]any)
	if !ok {
		t.Fatalf("guid map key type: got %T want map", entry["key"])
	}
	keyName, ok := extractWrappedValueLocal(keyValue["value"]).(map[string]any)
	if !ok || !strings.EqualFold(strings.TrimSpace(widgetRemoveTestAnyToString(keyName["name"])), "CanvasPanel_22") {
		t.Fatalf("guid map remaining key: got %v want CanvasPanel_22", keyValue["value"])
	}
	requireWidgetRemovePropertyMissing(t, asset, root.BlueprintExport, "GeneratedVariables")

	categorySorting, err := decodeExportRootPropertyValue(asset, root.BlueprintExport, "CategorySorting")
	if err != nil {
		t.Fatalf("decode CategorySorting: %v", err)
	}
	categoryMap, ok := categorySorting.(map[string]any)
	if !ok {
		t.Fatalf("CategorySorting type: got %T want map", categorySorting)
	}
	categoryItems, err := anySliceLocal(categoryMap["value"])
	if err != nil {
		t.Fatalf("CategorySorting items: %v", err)
	}
	if len(categoryItems) != 2 {
		t.Fatalf("CategorySorting count: got %d want 2", len(categoryItems))
	}
	wantCategoryNames := []string{"Event Graph", "WBP Canvas Panel Rich Text Block"}
	for i, want := range wantCategoryNames {
		categoryWrapped, ok := categoryItems[i].(map[string]any)
		if !ok {
			t.Fatalf("CategorySorting entry %d type: got %T want map", i, categoryItems[i])
		}
		categoryValue, ok := extractWrappedValueLocal(categoryWrapped["value"]).(map[string]any)
		if !ok || !strings.EqualFold(strings.TrimSpace(widgetRemoveTestAnyToString(categoryValue["name"])), want) {
			t.Fatalf("CategorySorting entry %d: got %v want %s", i, categoryItems[i], want)
		}
	}

	ctx, err := resolveWidgetAddContext(asset, *root)
	if err != nil {
		t.Fatalf("resolveWidgetAddContext: %v", err)
	}
	requireWidgetRemovePropertyMissing(t, asset, ctx.GeneratedClassExport, "PropertyGuids")

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"validate", work}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("validate exit code: got %d want 0 stderr=%s", code, stderr.String())
	}
}

func TestRunBlueprintWidgetRemoveTextLeavesNoLocalizationResidue(t *testing.T) {
	cases := []struct {
		name        string
		fixture     string
		widget      string
		rootPath    string
		rootClass   string
		wantExports int
		wantImports int
	}{
		{
			name:        "canvaspanel_textblock",
			fixture:     "WBP_CanvasPanel_TextBlock.uasset",
			widget:      "CanvasPanel_22/TextBlock_31",
			rootPath:    "CanvasPanel_22",
			rootClass:   "CanvasPanel",
			wantExports: 11,
			wantImports: 19,
		},
		{
			name:        "overlay_textblock",
			fixture:     "WBP_Overlay_TextBlock.uasset",
			widget:      "Overlay_116/TextBlock_36",
			rootPath:    "Overlay_116",
			rootClass:   "Overlay",
			wantExports: 11,
			wantImports: 19,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := findExistingGoldenParseFixturePath(t, tc.fixture)
			work := copyFixtureToTemp(t, path)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run([]string{
				"blueprint", "widget-remove", work,
				"--widget", tc.widget,
			}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
			}

			asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("parse rewritten asset: %v", err)
			}
			if got := len(asset.Exports); got != tc.wantExports {
				t.Fatalf("export count: got %d want %d", got, tc.wantExports)
			}
			if got := len(asset.Imports); got != tc.wantImports {
				t.Fatalf("import count: got %d want %d", got, tc.wantImports)
			}
			if asset.Summary.GatherableTextDataCount != 0 {
				t.Fatalf("gatherable text data count: got %d want 0", asset.Summary.GatherableTextDataCount)
			}
			if asset.Summary.PackageFlags != 0 {
				t.Fatalf("package flags: got 0x%08x want 0", asset.Summary.PackageFlags)
			}
			if findNameIndex(asset.Names, "Text") >= 0 {
				t.Fatalf("expected Text name entry to be compacted")
			}

			targets, err := collectWidgetWriteTargets(asset, 0)
			if err != nil {
				t.Fatalf("collectWidgetWriteTargets: %v", err)
			}
			if len(targets) != 1 {
				t.Fatalf("logical widget count: got %d want 1", len(targets))
			}
			root, err := selectWidgetWriteTarget(targets, tc.rootPath)
			if err != nil {
				t.Fatalf("select root widget: %v", err)
			}
			if root.ClassName != tc.rootClass {
				t.Fatalf("root class: got %q want %q", root.ClassName, tc.rootClass)
			}
		})
	}
}

func TestRunBlueprintWidgetRemoveImageUnderCanvasPanel(t *testing.T) {
	work := copyFixtureToTemp(t, findExistingGoldenOperationFixturePath(t, "widget_write_layout_canvaspanelslot", "before.uasset"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-remove", work,
		"--widget", "CanvasPanel_22/Image_29",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("decode stdout json: %v", err)
	}
	if got := int(resp["removedExportCount"].(float64)); got != 4 {
		t.Fatalf("removedExportCount: got %d want 4", got)
	}
	if got := int(resp["removedImportCount"].(float64)); got != 2 {
		t.Fatalf("removedImportCount: got %d want 2", got)
	}
	if got := int(resp["removedNameCount"].(float64)); got < 2 {
		t.Fatalf("removedNameCount: got %d want at least 2", got)
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	if got := len(asset.Exports); got != 11 {
		t.Fatalf("export count: got %d want 11", got)
	}
	if got := len(asset.Imports); got != 21 {
		t.Fatalf("import count: got %d want 21", got)
	}

	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("logical widget count: got %d want 1", len(targets))
	}
	root, err := selectWidgetWriteTarget(targets, "CanvasPanel_22")
	if err != nil {
		t.Fatalf("select root widget: %v", err)
	}
	if root.ClassName != "CanvasPanel" {
		t.Fatalf("root class: got %q want %q", root.ClassName, "CanvasPanel")
	}
	if _, err := selectWidgetWriteTarget(targets, "CanvasPanel_22/Image_29"); err == nil {
		t.Fatalf("expected removed child widget to be absent from logical targets")
	}
	for _, exportIdx := range root.Exports {
		requireWidgetRemovePropertyMissing(t, asset, exportIdx, "Slots")
	}
	if findNameIndex(asset.Names, "Image_29") >= 0 {
		t.Fatalf("expected Image_29 name entry to be compacted")
	}
	if findNameIndex(asset.Names, "DisplayLabel") >= 0 {
		t.Fatalf("expected DisplayLabel name entry to be compacted")
	}
	requireWidgetRemovePropertyMissing(t, asset, root.BlueprintExport, "GeneratedVariables")

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"validate", work}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("validate exit code: got %d want 0 stderr=%s", code, stderr.String())
	}
}

func TestRunBlueprintWidgetRemoveBackgroundBlurKeepsGeneratedClassFooterRefsStable(t *testing.T) {
	work := copyFixtureToTemp(t, findExistingGoldenOperationFixturePath(t, "widget_remove_backgroundblur_canvaspanel", "before.uasset"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-remove", work,
		"--widget", "CanvasPanel_1/BackgroundBlur_1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}

	got := widgetGeneratedClassTailExportRefNames(t, asset)
	want := []string{
		"WBP_CanvasPanel_BackgroundBlur",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("generated class tail export refs: got %v want %v", got, want)
	}
}

func TestRunBlueprintWidgetRemoveRejectsRootWidget(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_CanvasPanel.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-remove", work,
		"--widget", "CanvasPanel_22",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1 stderr=%s", code, stderr.String())
	}
	if !strings.Contains(strings.ToLower(stderr.String()), "non-root child") {
		t.Fatalf("expected non-root child error, got: %s", stderr.String())
	}
}

func TestRunBlueprintWidgetRemoveRejectsNonLeafWidget(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_CanvasPanel.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "CanvasPanel_22",
		"--type", "button",
		"--name", "Button_1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("add Button_1 exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "CanvasPanel_22/Button_1",
		"--type", "image",
		"--name", "Image_1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("add Image_1 exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{
		"blueprint", "widget-remove", work,
		"--widget", "CanvasPanel_22/Button_1",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1 stderr=%s", code, stderr.String())
	}
	if !strings.Contains(strings.ToLower(stderr.String()), "not a leaf") {
		t.Fatalf("expected non-leaf error, got: %s", stderr.String())
	}
}

func TestValidateWidgetRemoveTargetRejectsUnsafeTargets(t *testing.T) {
	tests := []struct {
		name    string
		targets []widgetWriteTarget
		target  widgetWriteTarget
		want    string
	}{
		{
			name: "root_widget",
			targets: []widgetWriteTarget{
				{Path: "CanvasPanel_22", ObjectName: "CanvasPanel_22", SlotExports: nil},
			},
			target: widgetWriteTarget{
				Path:       "CanvasPanel_22",
				ObjectName: "CanvasPanel_22",
			},
			want: "non-root child",
		},
		{
			name: "non_leaf_widget",
			targets: []widgetWriteTarget{
				{Path: "CanvasPanel_22", ObjectName: "CanvasPanel_22"},
				{Path: "CanvasPanel_22/Button_1", ObjectName: "Button_1", SlotExports: []int{3, 4}},
				{Path: "CanvasPanel_22/Button_1/Image_1", ObjectName: "Image_1", SlotExports: []int{5, 6}},
			},
			target: widgetWriteTarget{
				Path:        "CanvasPanel_22/Button_1",
				ObjectName:  "Button_1",
				SlotExports: []int{3, 4},
			},
			want: "not a leaf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWidgetRemoveTarget(tt.targets, tt.target)
			if err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.want)) {
				t.Fatalf("error: got %q want substring %q", err, tt.want)
			}
		})
	}
}

func TestSelectWidgetWriteTargetRejectsAmbiguousForWidgetRemove(t *testing.T) {
	_, err := selectWidgetWriteTarget([]widgetWriteTarget{
		{BlueprintExport: 7, ObjectName: "Image_1", Path: "CanvasPanel_22/Image_1", Exports: []int{8, 9}},
		{BlueprintExport: 7, ObjectName: "Image_1", Path: "Overlay_116/Image_1", Exports: []int{10, 11}},
	}, "Image_1")
	if err == nil {
		t.Fatalf("expected ambiguous selector to fail")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "CanvasPanel_22/Image_1") || !strings.Contains(err.Error(), "Overlay_116/Image_1") {
		t.Fatalf("expected full widget paths in ambiguity error, got: %v", err)
	}
}

func requireWidgetRemovePropertyMissing(t *testing.T, asset *uasset.Asset, exportIndex int, propertyName string) {
	t.Helper()
	if _, err := decodeExportRootPropertyValue(asset, exportIndex, propertyName); err == nil {
		t.Fatalf("expected property %s to be absent on export %d", propertyName, exportIndex+1)
	}
}

func widgetGeneratedClassTailExportRefNames(t *testing.T, asset *uasset.Asset) []string {
	t.Helper()

	generatedClassExport := -1
	for i, exp := range asset.Exports {
		if asset.ResolveClassName(exp) == "WidgetBlueprintGeneratedClass" {
			generatedClassExport = i
			break
		}
	}
	if generatedClassExport < 0 {
		t.Fatalf("WidgetBlueprintGeneratedClass export not found")
	}

	layout, err := captureGeneratedClassWidgetVariableFieldLayout(asset, generatedClassExport)
	if err != nil {
		t.Fatalf("captureGeneratedClassWidgetVariableFieldLayout: %v", err)
	}
	exp := asset.Exports[generatedClassExport]
	payload := asset.Raw.Bytes[int(exp.SerialOffset):int(exp.SerialOffset+exp.SerialSize)]
	tail := payload[layout.ScriptEnd:]
	if layout.RecordsEnd > len(tail) {
		t.Fatalf("generated class records end out of bounds: %d > %d", layout.RecordsEnd, len(tail))
	}

	order := packageByteOrder(asset)
	names := make([]string, 0, 4)
	seen := map[int32]bool{}
	for off := layout.RecordsEnd; off+8 <= len(tail); off++ {
		idx := int32(order.Uint32(tail[off : off+4]))
		if idx <= 0 || idx > int32(len(asset.Exports)) {
			continue
		}
		if order.Uint32(tail[off+4:off+8]) != 0 {
			continue
		}
		if seen[idx] {
			continue
		}
		seen[idx] = true
		names = append(names, asset.Exports[idx-1].ObjectName.Display(asset.Names))
		off += 7
	}
	slices.Sort(names)
	return names
}

func widgetRemoveTestAnyToString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
