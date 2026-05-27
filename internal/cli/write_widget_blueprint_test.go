package cli

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

func TestRunBlueprintWidgetWriteRejectsNegativeExport(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-write", "/tmp/nonexistent.uasset",
		"--widget", "TextBlock_0",
		"--property", "text",
		"--value", "Updated",
		"--export", "-1",
		"--dry-run",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: bpx blueprint widget-write") {
		t.Fatalf("expected usage error for negative export, got: %s", stderr.String())
	}
}

func TestNormalizeWidgetExportListDeduplicatesAndPrependsRoot(t *testing.T) {
	got := normalizeWidgetExportList(5, []int{9, 9, 13})
	want := []int{5, 9, 13}
	if len(got) != len(want) {
		t.Fatalf("len: got %d want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: got %d want %d (%v)", i, got[i], want[i], got)
		}
	}
}

func TestNormalizeWidgetWriteRequestVisibility(t *testing.T) {
	property, path, valueJSON, requiredNames, err := normalizeWidgetWriteRequest("visibility", "Collapsed")
	if err != nil {
		t.Fatalf("normalizeWidgetWriteRequest: %v", err)
	}
	if property != "visibility" {
		t.Fatalf("property: got %q want %q", property, "visibility")
	}
	if path != "Visibility" {
		t.Fatalf("path: got %q want %q", path, "Visibility")
	}
	if !strings.Contains(valueJSON, "ESlateVisibility::Collapsed") {
		t.Fatalf("valueJSON missing collapsed enum: %s", valueJSON)
	}
	if len(requiredNames) != 6 {
		t.Fatalf("requiredNames len: got %d want 6", len(requiredNames))
	}
}

func TestNormalizeWidgetVisibilityValueRejectsUnsupported(t *testing.T) {
	_, err := normalizeWidgetVisibilityValue("Gone")
	if err == nil {
		t.Fatalf("expected unsupported visibility to fail")
	}
	if !strings.Contains(err.Error(), "unsupported visibility") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIsDefaultWidgetTextLocalizationKey(t *testing.T) {
	tests := map[string]bool{
		"TextBlockDefaultValue":    true,
		"RichTextBlockDefaultText": true,
		"OtherDefaultText":         false,
		"":                         false,
	}
	for key, want := range tests {
		if got := isDefaultWidgetTextLocalizationKey(key); got != want {
			t.Fatalf("key %q: got %v want %v", key, got, want)
		}
	}
}

func TestValidateWidgetWriteTargetRequiresDesignerTreeExport(t *testing.T) {
	err := validateWidgetWriteTarget(widgetWriteTarget{
		ObjectName: "TextBlock_31",
		Path:       "CanvasPanel_22/TextBlock_31",
		Exports:    nil,
	})
	if err == nil {
		t.Fatalf("expected validateWidgetWriteTarget to fail")
	}
	if !strings.Contains(err.Error(), "writable export") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateWidgetWriteTargetAcceptsDesignerTreeExport(t *testing.T) {
	err := validateWidgetWriteTarget(widgetWriteTarget{
		ObjectName: "TextBlock_31",
		Path:       "CanvasPanel_22/TextBlock_31",
		Exports:    []int{8, 9},
	})
	if err != nil {
		t.Fatalf("validateWidgetWriteTarget: %v", err)
	}
}

func TestValidateWidgetWriteTargetRejectsMoreThanPair(t *testing.T) {
	err := validateWidgetWriteTarget(widgetWriteTarget{
		ObjectName: "TextBlock_31",
		Path:       "CanvasPanel_22/TextBlock_31",
		Exports:    []int{8, 9, 10},
	})
	if err == nil {
		t.Fatalf("expected validateWidgetWriteTarget to fail")
	}
	if !strings.Contains(err.Error(), "designer/generated pair") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateWidgetBlueprintShapeStable(t *testing.T) {
	before := widgetBlueprintShape{
		"designer":  []string{"CanvasPanel_22", "CanvasPanel_22/TextBlock_31"},
		"generated": []string{"CanvasPanel_22", "CanvasPanel_22/TextBlock_31"},
	}
	after := widgetBlueprintShape{
		"designer":  []string{"CanvasPanel_22", "CanvasPanel_22/TextBlock_31"},
		"generated": []string{"CanvasPanel_22", "CanvasPanel_22/TextBlock_31"},
	}
	if err := validateWidgetBlueprintShapeStable(before, after); err != nil {
		t.Fatalf("validateWidgetBlueprintShapeStable: %v", err)
	}
}

func TestValidateWidgetBlueprintShapeStableRejectsPathDrift(t *testing.T) {
	before := widgetBlueprintShape{
		"designer":  []string{"CanvasPanel_22", "CanvasPanel_22/TextBlock_31"},
		"generated": []string{"CanvasPanel_22", "CanvasPanel_22/TextBlock_31"},
	}
	after := widgetBlueprintShape{
		"designer":  []string{"CanvasPanel_22", "CanvasPanel_22/TextBlock_31"},
		"generated": []string{"CanvasPanel_22"},
	}
	err := validateWidgetBlueprintShapeStable(before, after)
	if err == nil {
		t.Fatalf("expected validateWidgetBlueprintShapeStable to fail")
	}
	if !strings.Contains(err.Error(), "widget paths changed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBrushImageRequiredPrefixNamesCoversStructHelpers(t *testing.T) {
	names := brushImageRequiredPrefixNames()
	required := []string{
		"/Script/SlateCore",
		"ByteProperty",
		"DeprecateSlateVector2D",
		"ObjectProperty",
		"SlateBrush",
		"StructProperty",
	}
	for _, want := range required {
		if !slices.Contains(names, want) {
			t.Fatalf("brushImageRequiredPrefixNames missing %q: %v", want, names)
		}
	}
}

func TestCollectWidgetWriteTargetsInfersSlotExportsForWidgetAddChild(t *testing.T) {
	path := findExistingGoldenOperationFixturePath(t, "widget_add_image_canvaspanel", "after.uasset")
	asset, err := uasset.ParseFile(path, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	child, err := selectWidgetWriteTarget(targets, "CanvasPanel_22/Image_23")
	if err != nil {
		t.Fatalf("select child widget: %v", err)
	}
	if len(child.Exports) != 1 {
		t.Fatalf("child exports: got %d want 1", len(child.Exports))
	}
	if len(child.SlotExports) != 2 {
		t.Fatalf("slot exports: got %d want 2 (%v)", len(child.SlotExports), child.SlotExports)
	}
}

func TestRunBlueprintWidgetWriteLayoutAndPadding(t *testing.T) {
	path := findExistingGoldenOperationFixturePath(t, "widget_add_image_canvaspanel", "after.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-write", work,
		"--widget", "CanvasPanel_22/Image_23",
		"--property", "layout-position",
		"--value", "12,34",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("layout-position exit code: got %d want 0 stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{
		"blueprint", "widget-write", work,
		"--widget", "CanvasPanel_22/Image_23",
		"--property", "layout-size",
		"--value", "56,78",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("layout-size exit code: got %d want 0 stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{
		"blueprint", "widget-write", work,
		"--widget", "CanvasPanel_22/Image_23",
		"--property", "slot-padding",
		"--value", "1,2,3,4",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("slot-padding exit code: got %d want 0 stderr=%s", code, stderr.String())
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
	if len(child.SlotExports) != 2 {
		t.Fatalf("slot exports: got %d want 2 (%v)", len(child.SlotExports), child.SlotExports)
	}
	for _, slotExport := range child.SlotExports {
		layout, err := readWidgetAnchorLayoutData(asset, slotExport, "LayoutData")
		if err != nil {
			t.Fatalf("read layout data on slot %d: %v", slotExport+1, err)
		}
		if layout.Left != 12 || layout.Top != 34 || layout.Right != 56 || layout.Bottom != 78 {
			t.Fatalf("slot %d layout: got %+v", slotExport+1, layout)
		}
		if _, err := decodeExportRootPropertyValue(asset, slotExport, "Padding"); err != nil {
			t.Fatalf("slot %d padding missing: %v", slotExport+1, err)
		}
	}
}

func TestRunBlueprintWidgetWriteLayoutOnUserWidgetChild(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_CanvasPanel.uasset")
	work := copyFixtureToTemp(t, path)

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
		t.Fatalf("widget-add userwidget exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{
		"blueprint", "widget-write", work,
		"--widget", "CanvasPanel_22/WBP_TextBlock_1",
		"--property", "layout-data",
		"--value", "{\"position\":[32,48],\"size\":[280,64],\"anchors\":[0,0,0,0],\"alignment\":[0,0]}",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("layout-data exit code: got %d want 0 stderr=%s", code, stderr.String())
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
	for _, slotExport := range child.SlotExports {
		layout, err := readWidgetAnchorLayoutData(asset, slotExport, "LayoutData")
		if err != nil {
			t.Fatalf("read layout data on slot %d: %v", slotExport+1, err)
		}
		if layout.Left != 32 || layout.Top != 48 || layout.Right != 280 || layout.Bottom != 64 {
			t.Fatalf("slot %d layout: got %+v", slotExport+1, layout)
		}
		if layout.MinX != 0 || layout.MinY != 0 || layout.MaxX != 0 || layout.MaxY != 0 {
			t.Fatalf("slot %d anchors: got %+v", slotExport+1, layout)
		}
		if layout.AlignX != 0 || layout.AlignY != 0 {
			t.Fatalf("slot %d alignment: got %+v", slotExport+1, layout)
		}
		raw, err := decodeExportRootPropertyValue(asset, slotExport, "LayoutData")
		if err != nil {
			t.Fatalf("decode layout data on slot %d: %v", slotExport+1, err)
		}
		root, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("slot %d layout raw type: got %T", slotExport+1, raw)
		}
		fields, _ := root["value"].(map[string]any)
		if _, ok := fields["Anchors"]; !ok {
			t.Fatalf("slot %d layout missing Anchors field: %#v", slotExport+1, fields)
		}
		if _, ok := fields["Alignment"]; !ok {
			t.Fatalf("slot %d layout missing Alignment field: %#v", slotExport+1, fields)
		}
	}
}

func TestRunBlueprintWidgetWritePaddingAndAlignmentAcrossSlotClasses(t *testing.T) {
	tests := []struct {
		name          string
		expectedClass string
		prepare       func(t *testing.T) (string, string)
	}{
		{
			name:          "OverlaySlot",
			expectedClass: "OverlaySlot",
			prepare: func(t *testing.T) (string, string) {
				return copyFixtureToTemp(t, findExistingGoldenOperationFixturePath(t, "widget_add_two_image_overlay", "after.uasset")), "Overlay_21/Image_1"
			},
		},
		{
			name:          "HorizontalBoxSlot",
			expectedClass: "HorizontalBoxSlot",
			prepare: func(t *testing.T) (string, string) {
				work := copyFixtureToTemp(t, findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset"))
				var stdout bytes.Buffer
				var stderr bytes.Buffer
				for _, argv := range [][]string{
					{
						"blueprint", "widget-add", work,
						"--parent", "root",
						"--type", "horizontalbox",
						"--name", "HorizontalBox_1",
					},
					{
						"blueprint", "widget-add", work,
						"--parent", "HorizontalBox_1",
						"--type", "image",
						"--name", "Image_1",
					},
				} {
					stdout.Reset()
					stderr.Reset()
					if code := Run(argv, &stdout, &stderr); code != 0 {
						t.Fatalf("setup exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
					}
				}
				return work, "HorizontalBox_1/Image_1"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			work, widgetPath := tt.prepare(t)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			for _, argv := range [][]string{
				{
					"blueprint", "widget-write", work,
					"--widget", widgetPath,
					"--property", "slot-padding",
					"--value", "5,6,7,8",
				},
				{
					"blueprint", "widget-write", work,
					"--widget", widgetPath,
					"--property", "slot-horizontal-alignment",
					"--value", "Right",
				},
				{
					"blueprint", "widget-write", work,
					"--widget", widgetPath,
					"--property", "slot-vertical-alignment",
					"--value", "Bottom",
				},
			} {
				stdout.Reset()
				stderr.Reset()
				if code := Run(argv, &stdout, &stderr); code != 0 {
					t.Fatalf("widget-write exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
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
			child, err := selectWidgetWriteTarget(targets, widgetPath)
			if err != nil {
				t.Fatalf("select child widget: %v", err)
			}
			slotClassName, err := widgetWriteSlotClassName(asset, child.SlotExports)
			if err != nil {
				t.Fatalf("widgetWriteSlotClassName: %v", err)
			}
			if slotClassName != tt.expectedClass {
				t.Fatalf("slot class: got %q want %q", slotClassName, tt.expectedClass)
			}
			for _, slotExport := range child.SlotExports {
				paddingValue, err := decodeExportRootPropertyValue(asset, slotExport, "Padding")
				if err != nil {
					t.Fatalf("slot %d padding missing: %v", slotExport+1, err)
				}
				paddingSummary, ok := widgetReadMarginSummary(paddingValue)
				if !ok {
					t.Fatalf("slot %d padding summary missing: %#v", slotExport+1, paddingValue)
				}
				if got, want := paddingSummary["left"], float32(5); got != want {
					t.Fatalf("slot %d padding.left: got %#v want %v", slotExport+1, got, want)
				}
				if got, want := paddingSummary["top"], float32(6); got != want {
					t.Fatalf("slot %d padding.top: got %#v want %v", slotExport+1, got, want)
				}
				if got, want := paddingSummary["right"], float32(7); got != want {
					t.Fatalf("slot %d padding.right: got %#v want %v", slotExport+1, got, want)
				}
				if got, want := paddingSummary["bottom"], float32(8); got != want {
					t.Fatalf("slot %d padding.bottom: got %#v want %v", slotExport+1, got, want)
				}

				hAlignValue, err := decodeExportRootPropertyValue(asset, slotExport, "HorizontalAlignment")
				if err != nil {
					t.Fatalf("slot %d HorizontalAlignment missing: %v", slotExport+1, err)
				}
				if got, ok := widgetReadEnumValue(hAlignValue); !ok || got != "EHorizontalAlignment::HAlign_Right" {
					t.Fatalf("slot %d HorizontalAlignment: got %#v ok=%v", slotExport+1, got, ok)
				}

				vAlignValue, err := decodeExportRootPropertyValue(asset, slotExport, "VerticalAlignment")
				if err != nil {
					t.Fatalf("slot %d VerticalAlignment missing: %v", slotExport+1, err)
				}
				if got, ok := widgetReadEnumValue(vAlignValue); !ok || got != "EVerticalAlignment::VAlign_Bottom" {
					t.Fatalf("slot %d VerticalAlignment: got %#v ok=%v", slotExport+1, got, ok)
				}
			}
		})
	}
}

func TestRunBlueprintWidgetWriteSlotSizeOnUserWidgetChildrenInHorizontalBox(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	steps := [][]string{
		{
			"blueprint", "widget-add", work,
			"--parent", "root",
			"--type", "horizontalbox",
			"--name", "HorizontalBox_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "HorizontalBox_1",
			"--type", "userwidget",
			"--class", "/Game/BPXFixtures/Parse/WBP_TextBlock",
			"--name", "WBP_TextBlock_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "HorizontalBox_1",
			"--type", "userwidget",
			"--class", "/Game/BPXFixtures/Parse/WBP_TextBlock",
			"--name", "WBP_TextBlock_2",
		},
	}
	for _, argv := range steps {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
			t.Fatalf("%v exit code: got %d want 0 stderr=%s", argv, code, stderr.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	code := Run([]string{
		"blueprint", "widget-write", work,
		"--widget", "HorizontalBox_1/WBP_TextBlock_1",
		"--property", "slot-size",
		"--value", "fill:2",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("slot-size exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{
		"blueprint", "widget-write", work,
		"--widget", "HorizontalBox_1/WBP_TextBlock_1",
		"--property", "slot-horizontal-alignment",
		"--value", "center",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("slot-horizontal-alignment exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	child, err := selectWidgetWriteTarget(targets, "HorizontalBox_1/WBP_TextBlock_1")
	if err != nil {
		t.Fatalf("select first child widget: %v", err)
	}
	if got, want := child.ClassName, "WBP_TextBlock_C"; got != want {
		t.Fatalf("child class: got %q want %q", got, want)
	}
	for _, slotExport := range child.SlotExports {
		sizeValue, err := decodeExportRootPropertyValue(asset, slotExport, "Size")
		if err != nil {
			t.Fatalf("slot %d size missing: %v", slotExport+1, err)
		}
		sizeSummary, ok := widgetReadSlateChildSizeSummary(sizeValue)
		if !ok {
			t.Fatalf("slot %d size summary missing: %#v", slotExport+1, sizeValue)
		}
		if got, want := sizeSummary["rule"], "ESlateSizeRule::Fill"; got != want {
			t.Fatalf("slot %d size.rule: got %#v want %v", slotExport+1, got, want)
		}
		if got, want := sizeSummary["value"], float32(2); got != want {
			t.Fatalf("slot %d size.value: got %#v want %v", slotExport+1, got, want)
		}

		hAlignValue, err := decodeExportRootPropertyValue(asset, slotExport, "HorizontalAlignment")
		if err != nil {
			t.Fatalf("slot %d HorizontalAlignment missing: %v", slotExport+1, err)
		}
		if got, ok := widgetReadEnumValue(hAlignValue); !ok || got != "EHorizontalAlignment::HAlign_Center" {
			t.Fatalf("slot %d HorizontalAlignment: got %#v ok=%v", slotExport+1, got, ok)
		}
	}

	secondChild, err := selectWidgetWriteTarget(targets, "HorizontalBox_1/WBP_TextBlock_2")
	if err != nil {
		t.Fatalf("select second child widget: %v", err)
	}
	if got, want := secondChild.ClassName, "WBP_TextBlock_C"; got != want {
		t.Fatalf("second child class: got %q want %q", got, want)
	}
	if len(secondChild.SlotExports) != 2 {
		t.Fatalf("second child slot exports: got %d want 2 (%v)", len(secondChild.SlotExports), secondChild.SlotExports)
	}
}

func TestRunBlueprintWidgetWriteBoxSlotSize(t *testing.T) {
	tests := []struct {
		name          string
		rootType      string
		rootName      string
		expectedClass string
		value         string
		wantRule      string
		wantSize      float32
	}{
		{
			name:          "HorizontalBox fill",
			rootType:      "horizontalbox",
			rootName:      "HorizontalBox_1",
			expectedClass: "HorizontalBoxSlot",
			value:         "fill:2.5",
			wantRule:      "ESlateSizeRule::Fill",
			wantSize:      2.5,
		},
		{
			name:          "VerticalBox auto",
			rootType:      "verticalbox",
			rootName:      "VerticalBox_1",
			expectedClass: "VerticalBoxSlot",
			value:         "auto",
			wantRule:      "ESlateSizeRule::Automatic",
			wantSize:      1,
		},
		{
			name:          "StackBox fill",
			rootType:      "stackbox",
			rootName:      "StackBox_1",
			expectedClass: "StackBoxSlot",
			value:         "fill:3",
			wantRule:      "ESlateSizeRule::Fill",
			wantSize:      3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			work := copyFixtureToTemp(t, findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset"))

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
					"blueprint", "widget-write", work,
					"--widget", tt.rootName + "/Image_1",
					"--property", "slot-size",
					"--value", tt.value,
				},
			} {
				stdout.Reset()
				stderr.Reset()
				if code := Run(argv, &stdout, &stderr); code != 0 {
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
			child, err := selectWidgetWriteTarget(targets, tt.rootName+"/Image_1")
			if err != nil {
				t.Fatalf("select child widget: %v", err)
			}
			slotClassName, err := widgetWriteSlotClassName(asset, child.SlotExports)
			if err != nil {
				t.Fatalf("widgetWriteSlotClassName: %v", err)
			}
			if slotClassName != tt.expectedClass {
				t.Fatalf("slot class: got %q want %q", slotClassName, tt.expectedClass)
			}

			for _, slotExport := range child.SlotExports {
				sizeValue, err := decodeExportRootPropertyValue(asset, slotExport, "Size")
				if err != nil {
					t.Fatalf("slot %d size missing: %v", slotExport+1, err)
				}
				sizeSummary, ok := widgetReadSlateChildSizeSummary(sizeValue)
				if !ok {
					t.Fatalf("slot %d size summary missing: %#v", slotExport+1, sizeValue)
				}
				if got := sizeSummary["rule"]; got != tt.wantRule {
					t.Fatalf("slot %d size rule: got %#v want %q", slotExport+1, got, tt.wantRule)
				}
				if got := sizeSummary["value"]; got != tt.wantSize {
					t.Fatalf("slot %d size value: got %#v want %v", slotExport+1, got, tt.wantSize)
				}
			}
		})
	}
}

func TestRunBlueprintWidgetWriteGridPanelFill(t *testing.T) {
	work := copyFixtureToTemp(t, findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-add", work,
			"--parent", "root",
			"--type", "gridpanel",
			"--name", "GridPanel_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "GridPanel_1",
			"--type", "image",
			"--name", "Image_1",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", "GridPanel_1",
			"--property", "grid-column-fill",
			"--value", "1,3,1",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", "GridPanel_1",
			"--property", "grid-row-fill",
			"--value", "2,1",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
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
	grid, err := selectWidgetWriteTarget(targets, "GridPanel_1")
	if err != nil {
		t.Fatalf("select grid widget: %v", err)
	}
	if len(grid.Exports) != 2 {
		t.Fatalf("grid exports: got %d want 2 (%v)", len(grid.Exports), grid.Exports)
	}

	for _, exportIdx := range grid.Exports {
		columnFill, err := decodeExportRootPropertyValue(asset, exportIdx, "ColumnFill")
		if err != nil {
			t.Fatalf("export %d ColumnFill missing: %v", exportIdx+1, err)
		}
		columnValues, ok := widgetReadFloatArraySummary(columnFill)
		if !ok {
			t.Fatalf("export %d ColumnFill summary missing: %#v", exportIdx+1, columnFill)
		}
		requireWidgetReadFloatSlice(t, columnValues, []float32{1, 3, 1})

		rowFill, err := decodeExportRootPropertyValue(asset, exportIdx, "RowFill")
		if err != nil {
			t.Fatalf("export %d RowFill missing: %v", exportIdx+1, err)
		}
		rowValues, ok := widgetReadFloatArraySummary(rowFill)
		if !ok {
			t.Fatalf("export %d RowFill summary missing: %#v", exportIdx+1, rowFill)
		}
		requireWidgetReadFloatSlice(t, rowValues, []float32{2, 1})
	}
}

func TestRunBlueprintWidgetWriteButtonAndBorderVisualProperties(t *testing.T) {
	tests := []struct {
		name         string
		prepare      [][]string
		targetPath   string
		updates      [][]string
		verifyExport func(t *testing.T, asset *uasset.Asset, exportIdx int)
	}{
		{
			name: "Button",
			prepare: [][]string{
				{"blueprint", "widget-add", "--parent", "root", "--type", "button", "--name", "Button_1"},
				{"blueprint", "widget-add", "--parent", "Button_1", "--type", "image", "--name", "Image_1"},
			},
			targetPath: "Button_1",
			updates: [][]string{
				{"blueprint", "widget-write", "--widget", "Button_1", "--property", "button-background-color", "--value", "0.15,0.35,0.85,1"},
				{"blueprint", "widget-write", "--widget", "Button_1", "--property", "button-color-and-opacity", "--value", "1,0.8,0.6,0.9"},
				{"blueprint", "widget-write", "--widget", "Button_1", "--property", "is-focusable", "--value", "false"},
			},
			verifyExport: func(t *testing.T, asset *uasset.Asset, exportIdx int) {
				t.Helper()
				backgroundValue, err := decodeExportRootPropertyValue(asset, exportIdx, "BackgroundColor")
				if err != nil {
					t.Fatalf("decode BackgroundColor: %v", err)
				}
				requireWidgetReadLinearColor(t, backgroundValue, widgetLinearColor{R: 0.15, G: 0.35, B: 0.85, A: 1})

				contentValue, err := decodeExportRootPropertyValue(asset, exportIdx, "ColorAndOpacity")
				if err != nil {
					t.Fatalf("decode ColorAndOpacity: %v", err)
				}
				requireWidgetReadLinearColor(t, contentValue, widgetLinearColor{R: 1, G: 0.8, B: 0.6, A: 0.9})

				isFocusable, err := decodeExportRootPropertyValue(asset, exportIdx, "IsFocusable")
				if err != nil {
					t.Fatalf("decode IsFocusable: %v", err)
				}
				if got, ok := widgetReadBoolValue(isFocusable); !ok || got {
					t.Fatalf("IsFocusable: got %#v want false", isFocusable)
				}
			},
		},
		{
			name: "Border",
			prepare: [][]string{
				{"blueprint", "widget-add", "--parent", "root", "--type", "border", "--name", "Border_1"},
				{"blueprint", "widget-add", "--parent", "Border_1", "--type", "image", "--name", "Image_1"},
			},
			targetPath: "Border_1",
			updates: [][]string{
				{"blueprint", "widget-write", "--widget", "Border_1", "--property", "border-padding", "--value", "18,24,30,36"},
				{"blueprint", "widget-write", "--widget", "Border_1", "--property", "border-brush-color", "--value", "0.2,0.25,0.3,0.95"},
				{"blueprint", "widget-write", "--widget", "Border_1", "--property", "border-content-color-and-opacity", "--value", "0.9,0.8,0.6,1"},
				{"blueprint", "widget-write", "--widget", "Border_1", "--property", "border-horizontal-alignment", "--value", "Center"},
				{"blueprint", "widget-write", "--widget", "Border_1", "--property", "border-vertical-alignment", "--value", "Bottom"},
			},
			verifyExport: func(t *testing.T, asset *uasset.Asset, exportIdx int) {
				t.Helper()
				paddingValue, err := decodeExportRootPropertyValue(asset, exportIdx, "Padding")
				if err != nil {
					t.Fatalf("decode Padding: %v", err)
				}
				paddingSummary, ok := widgetReadMarginSummary(paddingValue)
				if !ok {
					t.Fatalf("Padding summary missing: %#v", paddingValue)
				}
				if got, want := paddingSummary["left"], float32(18); got != want {
					t.Fatalf("Padding.left: got %#v want %v", got, want)
				}
				if got, want := paddingSummary["bottom"], float32(36); got != want {
					t.Fatalf("Padding.bottom: got %#v want %v", got, want)
				}

				brushColorValue, err := decodeExportRootPropertyValue(asset, exportIdx, "BrushColor")
				if err != nil {
					t.Fatalf("decode BrushColor: %v", err)
				}
				requireWidgetReadLinearColor(t, brushColorValue, widgetLinearColor{R: 0.2, G: 0.25, B: 0.3, A: 0.95})

				contentColorValue, err := decodeExportRootPropertyValue(asset, exportIdx, "ContentColorAndOpacity")
				if err != nil {
					t.Fatalf("decode ContentColorAndOpacity: %v", err)
				}
				requireWidgetReadLinearColor(t, contentColorValue, widgetLinearColor{R: 0.9, G: 0.8, B: 0.6, A: 1})

				hAlignValue, err := decodeExportRootPropertyValue(asset, exportIdx, "HorizontalAlignment")
				if err != nil {
					t.Fatalf("decode HorizontalAlignment: %v", err)
				}
				if got, ok := widgetReadEnumValue(hAlignValue); !ok || got != "EHorizontalAlignment::HAlign_Center" {
					t.Fatalf("HorizontalAlignment: got %#v want EHorizontalAlignment::HAlign_Center", hAlignValue)
				}

				vAlignValue, err := decodeExportRootPropertyValue(asset, exportIdx, "VerticalAlignment")
				if err != nil {
					t.Fatalf("decode VerticalAlignment: %v", err)
				}
				if got, ok := widgetReadEnumValue(vAlignValue); !ok || got != "EVerticalAlignment::VAlign_Bottom" {
					t.Fatalf("VerticalAlignment: got %#v want EVerticalAlignment::VAlign_Bottom", vAlignValue)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			work := copyFixtureToTemp(t, findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset"))

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			for _, rawArgv := range append(append([][]string{}, tt.prepare...), tt.updates...) {
				argv := append([]string{}, rawArgv[:2]...)
				argv = append(argv, work)
				argv = append(argv, rawArgv[2:]...)
				stdout.Reset()
				stderr.Reset()
				if code := Run(argv, &stdout, &stderr); code != 0 {
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
			target, err := selectWidgetWriteTarget(targets, tt.targetPath)
			if err != nil {
				t.Fatalf("select widget target: %v", err)
			}
			for _, exportIdx := range target.Exports {
				tt.verifyExport(t, asset, exportIdx)
			}
		})
	}
}

func TestRunBlueprintWidgetWriteButtonStateBrushImages(t *testing.T) {
	work := copyFixtureToTemp(t, findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-add", work,
			"--parent", "root",
			"--type", "button",
			"--name", "Button_1",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", "Button_1",
			"--property", "button-normal-image",
			"--value", "/Game/UI/Menu/Art/T_UI_Icon_SimpleArrow",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", "Button_1",
			"--property", "button-hovered-image",
			"--value", "/Game/UI/Menu/Art/T_UI_Icon_SimpleDiamond",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", "Button_1",
			"--property", "button-pressed-image",
			"--value", "/Game/UI/Menu/Art/T_UI_Icon_SimpleArrow",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", "Button_1",
			"--property", "button-disabled-image",
			"--value", "/Game/UI/Menu/Art/T_UI_Icon_SimpleDiamond",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
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
	target, err := selectWidgetWriteTarget(targets, "Button_1")
	if err != nil {
		t.Fatalf("select button widget: %v", err)
	}
	for _, exportIdx := range target.Exports {
		widgetStyleValue, err := decodeExportRootPropertyValue(asset, exportIdx, "WidgetStyle")
		if err != nil {
			t.Fatalf("decode WidgetStyle: %v", err)
		}
		brushes := widgetReadButtonBrushesSummary(asset, widgetStyleValue)
		if len(brushes) == 0 {
			t.Fatalf("button brushes missing: %#v", widgetStyleValue)
		}
		requireWidgetReadBrushPath(t, brushes["normal"], "/Game/UI/Menu/Art/T_UI_Icon_SimpleArrow")
		requireWidgetReadBrushPath(t, brushes["hovered"], "/Game/UI/Menu/Art/T_UI_Icon_SimpleDiamond")
		requireWidgetReadBrushPath(t, brushes["pressed"], "/Game/UI/Menu/Art/T_UI_Icon_SimpleArrow")
		requireWidgetReadBrushPath(t, brushes["disabled"], "/Game/UI/Menu/Art/T_UI_Icon_SimpleDiamond")
	}
}

func TestRunBlueprintWidgetWriteButtonStateBrushDetails(t *testing.T) {
	work := copyFixtureToTemp(t, findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-add", work,
			"--parent", "root",
			"--type", "button",
			"--name", "Button_1",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", "Button_1",
			"--property", "button-normal-image",
			"--value", "/Game/UI/Menu/Art/T_UI_Icon_SimpleArrow",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", "Button_1",
			"--property", "button-normal-tint",
			"--value", "0.2,0.35,0.9,0.85",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", "Button_1",
			"--property", "button-hovered-tint",
			"--value", "1,0.6,0.25,1",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", "Button_1",
			"--property", "button-normal-image-size",
			"--value", "96,48",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", "Button_1",
			"--property", "button-hovered-image-size",
			"--value", "64,64",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", "Button_1",
			"--property", "button-normal-draw-as",
			"--value", "RoundedBox",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", "Button_1",
			"--property", "button-disabled-draw-as",
			"--value", "Border",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
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
	target, err := selectWidgetWriteTarget(targets, "Button_1")
	if err != nil {
		t.Fatalf("select button widget: %v", err)
	}
	for _, exportIdx := range target.Exports {
		widgetStyleValue, err := decodeExportRootPropertyValue(asset, exportIdx, "WidgetStyle")
		if err != nil {
			t.Fatalf("decode WidgetStyle: %v", err)
		}
		brushes := widgetReadButtonBrushesSummary(asset, widgetStyleValue)
		if len(brushes) == 0 {
			t.Fatalf("button brushes missing: %#v", widgetStyleValue)
		}
		normal, ok := brushes["normal"].(map[string]any)
		if !ok {
			t.Fatalf("normal brush type: got %#v", brushes["normal"])
		}
		requireWidgetReadBrushPath(t, normal, "/Game/UI/Menu/Art/T_UI_Icon_SimpleArrow")
		requireWidgetReadLinearColor(t, normal["tintColor"], widgetLinearColor{R: 0.2, G: 0.35, B: 0.9, A: 0.85})
		requireWidgetReadVector2(t, normal["imageSize"], 96, 48)
		if got, want := normal["drawAs"], "ESlateBrushDrawType::RoundedBox"; got != want {
			t.Fatalf("normal drawAs: got %#v want %q", got, want)
		}

		hovered, ok := brushes["hovered"].(map[string]any)
		if !ok {
			t.Fatalf("hovered brush type: got %#v", brushes["hovered"])
		}
		requireWidgetReadLinearColor(t, hovered["tintColor"], widgetLinearColor{R: 1, G: 0.6, B: 0.25, A: 1})
		requireWidgetReadVector2(t, hovered["imageSize"], 64, 64)

		disabled, ok := brushes["disabled"].(map[string]any)
		if !ok {
			t.Fatalf("disabled brush type: got %#v", brushes["disabled"])
		}
		if got, want := disabled["drawAs"], "ESlateBrushDrawType::Border"; got != want {
			t.Fatalf("disabled drawAs: got %#v want %q", got, want)
		}
	}
}

func TestRunBlueprintWidgetWriteButtonAndBorderVisualPropertiesRejectWrongWidgetClass(t *testing.T) {
	work := copyFixtureToTemp(t, findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-add", work,
			"--parent", "root",
			"--type", "button",
			"--name", "Button_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "Button_1",
			"--type", "image",
			"--name", "Image_1",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
			t.Fatalf("setup exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	cases := []struct {
		property string
		value    string
		wantErr  string
	}{
		{
			property: "border-padding",
			value:    "4",
			wantErr:  "border-padding requires a Border widget",
		},
		{
			property: "button-background-color",
			value:    "1,1,1,1",
			wantErr:  "button-background-color requires a Button widget",
		},
	}

	for _, tc := range cases {
		t.Run(tc.property, func(t *testing.T) {
			stdout.Reset()
			stderr.Reset()
			widget := "Button_1"
			if strings.HasPrefix(tc.property, "button-") {
				widget = "Button_1/Image_1"
			}
			code := Run([]string{
				"blueprint", "widget-write", work,
				"--widget", widget,
				"--property", tc.property,
				"--value", tc.value,
			}, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("exit code: got %d want 1 stderr=%s", code, stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.wantErr) {
				t.Fatalf("unexpected stderr: %s", stderr.String())
			}
		})
	}
}

func TestRunBlueprintWidgetWriteGridSlotRowAndColumn(t *testing.T) {
	tests := []struct {
		rootType string
		rootName string
	}{
		{rootType: "gridpanel", rootName: "GridPanel_1"},
		{rootType: "uniformgridpanel", rootName: "UniformGridPanel_1"},
	}

	for _, tt := range tests {
		t.Run(tt.rootName, func(t *testing.T) {
			work := copyFixtureToTemp(t, findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset"))

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
					"blueprint", "widget-write", work,
					"--widget", tt.rootName + "/Image_1",
					"--property", "slot-row",
					"--value", "2",
				},
				{
					"blueprint", "widget-write", work,
					"--widget", tt.rootName + "/Image_1",
					"--property", "slot-column",
					"--value", "3",
				},
				{
					"blueprint", "widget-write", work,
					"--widget", tt.rootName + "/Image_1",
					"--property", "slot-row-span",
					"--value", "2",
				},
				{
					"blueprint", "widget-write", work,
					"--widget", tt.rootName + "/Image_1",
					"--property", "slot-column-span",
					"--value", "4",
				},
				{
					"blueprint", "widget-write", work,
					"--widget", tt.rootName + "/Image_1",
					"--property", "slot-layer",
					"--value", "5",
				},
				{
					"blueprint", "widget-write", work,
					"--widget", tt.rootName + "/Image_1",
					"--property", "slot-nudge",
					"--value", "10,20",
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
			child, err := selectWidgetWriteTarget(targets, tt.rootName+"/Image_1")
			if err != nil {
				t.Fatalf("select child widget: %v", err)
			}
			for _, slotExport := range child.SlotExports {
				rowValue, err := decodeExportRootPropertyValue(asset, slotExport, "Row")
				if err != nil {
					t.Fatalf("decode slot Row: %v", err)
				}
				row, ok := extractIntLike(rowValue)
				if !ok || row != 2 {
					t.Fatalf("slot Row: got %#v want 2", rowValue)
				}
				columnValue, err := decodeExportRootPropertyValue(asset, slotExport, "Column")
				if err != nil {
					t.Fatalf("decode slot Column: %v", err)
				}
				column, ok := extractIntLike(columnValue)
				if !ok || column != 3 {
					t.Fatalf("slot Column: got %#v want 3", columnValue)
				}
				rowSpanValue, err := decodeExportRootPropertyValue(asset, slotExport, "RowSpan")
				if err != nil {
					t.Fatalf("decode slot RowSpan: %v", err)
				}
				rowSpan, ok := extractIntLike(rowSpanValue)
				if !ok || rowSpan != 2 {
					t.Fatalf("slot RowSpan: got %#v want 2", rowSpanValue)
				}
				columnSpanValue, err := decodeExportRootPropertyValue(asset, slotExport, "ColumnSpan")
				if err != nil {
					t.Fatalf("decode slot ColumnSpan: %v", err)
				}
				columnSpan, ok := extractIntLike(columnSpanValue)
				if !ok || columnSpan != 4 {
					t.Fatalf("slot ColumnSpan: got %#v want 4", columnSpanValue)
				}
				layerValue, err := decodeExportRootPropertyValue(asset, slotExport, "Layer")
				if err != nil {
					t.Fatalf("decode slot Layer: %v", err)
				}
				layer, ok := extractIntLike(layerValue)
				if !ok || layer != 5 {
					t.Fatalf("slot Layer: got %#v want 5", layerValue)
				}
				nudgeValue, err := decodeExportRootPropertyValue(asset, slotExport, "Nudge")
				if err != nil {
					t.Fatalf("decode slot Nudge: %v", err)
				}
				nudge, ok := widgetReadVector2Summary(nudgeValue)
				if !ok {
					t.Fatalf("slot Nudge missing or invalid: %#v", nudgeValue)
				}
				if got, want := nudge["x"], float32(10); got != want {
					t.Fatalf("slot Nudge.x: got %#v want %v", got, want)
				}
				if got, want := nudge["y"], float32(20); got != want {
					t.Fatalf("slot Nudge.y: got %#v want %v", got, want)
				}
			}
		})
	}
}

func TestRunBlueprintWidgetWriteMenuAnchorPlacement(t *testing.T) {
	work := copyFixtureToTemp(t, findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-add", work,
			"--parent", "root",
			"--type", "menuanchor",
			"--name", "MenuAnchor_1",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", "MenuAnchor_1",
			"--property", "menu-anchor-placement",
			"--value", "BelowRightAnchor",
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
	target, err := selectWidgetWriteTarget(targets, "MenuAnchor_1")
	if err != nil {
		t.Fatalf("select MenuAnchor widget: %v", err)
	}
	for _, exportIdx := range target.Exports {
		value, err := decodeExportRootPropertyValue(asset, exportIdx, "Placement")
		if err != nil {
			t.Fatalf("decode Placement: %v", err)
		}
		enumValue, ok := widgetReadEnumValue(value)
		if !ok || enumValue != "EMenuPlacement::MenuPlacement_BelowRightAnchor" {
			t.Fatalf("Placement: got %#v want EMenuPlacement::MenuPlacement_BelowRightAnchor", value)
		}
	}
}

func TestRunBlueprintWidgetWriteLayoutRejectsNonCanvasPanelSlot(t *testing.T) {
	work := copyFixtureToTemp(t, findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-add", work,
			"--parent", "root",
			"--type", "verticalbox",
			"--name", "VerticalBox_1",
		},
		{
			"blueprint", "widget-add", work,
			"--parent", "VerticalBox_1",
			"--type", "image",
			"--name", "Image_1",
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
		"blueprint", "widget-write", work,
		"--widget", "VerticalBox_1/Image_1",
		"--property", "layout-position",
		"--value", "10,20",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1 stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "requires CanvasPanelSlot") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRunBlueprintWidgetWriteTextAddsMissingTextProperty(t *testing.T) {
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

	stdout.Reset()
	stderr.Reset()
	code := Run([]string{
		"blueprint", "widget-write", outPath,
		"--widget", "CanvasPanel_21/TextBlock_1",
		"--property", "text",
		"--value", "Updated text",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(outPath, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse final asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets final: %v", err)
	}
	target, err := selectWidgetWriteTarget(targets, "CanvasPanel_21/TextBlock_1")
	if err != nil {
		t.Fatalf("select final widget: %v", err)
	}
	for i, exportIdx := range target.Exports {
		current, err := decodeExportRootPropertyValue(asset, exportIdx, "Text")
		if err != nil {
			t.Fatalf("decode export %d Text: %v", i, err)
		}
		textValue, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("export %d Text type: got %T want map", i, current)
		}
		if got, want := textValue["sourceString"], "Updated text"; got != want {
			t.Fatalf("export %d Text sourceString: got %#v want %q", i, got, want)
		}
	}
}

func TestRunBlueprintWidgetWriteTextAddsMissingTextPropertyForRichTextBlock(t *testing.T) {
	outPath, childPath := buildRichTextWidgetTestAsset(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-write", outPath,
		"--widget", childPath,
		"--property", "text",
		"--value", "Updated rich text",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(outPath, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse final asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets final: %v", err)
	}
	target, err := selectWidgetWriteTarget(targets, childPath)
	if err != nil {
		t.Fatalf("select final widget: %v", err)
	}
	if got, want := target.ClassName, "RichTextBlock"; got != want {
		t.Fatalf("className: got %q want %q", got, want)
	}
	for i, exportIdx := range target.Exports {
		current, err := decodeExportRootPropertyValue(asset, exportIdx, "Text")
		if err != nil {
			t.Fatalf("decode export %d Text: %v", i, err)
		}
		textValue, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("export %d Text type: got %T want map", i, current)
		}
		if got, want := textValue["sourceString"], "Updated rich text"; got != want {
			t.Fatalf("export %d Text sourceString: got %#v want %q", i, got, want)
		}
	}
}

func TestRunBlueprintWidgetWriteVisibilityAndOpacityForRichTextBlock(t *testing.T) {
	work, childPath := buildRichTextWidgetTestAsset(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-write", work,
			"--widget", childPath,
			"--property", "richtext-default-font",
			"--value", "/Game/UI/Fonts/NotoSans",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", childPath,
			"--property", "richtext-default-typeface",
			"--value", "Regular",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", childPath,
			"--property", "visibility",
			"--value", "Collapsed",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", childPath,
			"--property", "render-opacity",
			"--value", "0.5",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
			t.Fatalf("widget-write exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse final asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets final: %v", err)
	}
	target, err := selectWidgetWriteTarget(targets, childPath)
	if err != nil {
		t.Fatalf("select final widget: %v", err)
	}
	for _, exportIdx := range target.Exports {
		visibility, err := decodeExportRootPropertyValue(asset, exportIdx, "Visibility")
		if err != nil {
			t.Fatalf("decode export %d Visibility: %v", exportIdx+1, err)
		}
		if got, ok := widgetReadEnumValue(visibility); !ok || got != "ESlateVisibility::Collapsed" {
			t.Fatalf("export %d Visibility enum: got %#v want %q", exportIdx+1, got, "ESlateVisibility::Collapsed")
		}

		renderOpacity, err := decodeExportRootPropertyValue(asset, exportIdx, "RenderOpacity")
		if err != nil {
			t.Fatalf("decode export %d RenderOpacity: %v", exportIdx+1, err)
		}
		switch got := renderOpacity.(type) {
		case float32:
			if got != 0.5 {
				t.Fatalf("export %d RenderOpacity: got %v want 0.5", exportIdx+1, got)
			}
		case float64:
			if got != 0.5 {
				t.Fatalf("export %d RenderOpacity: got %v want 0.5", exportIdx+1, got)
			}
		default:
			t.Fatalf("export %d RenderOpacity type: got %T want float", exportIdx+1, renderOpacity)
		}
	}
}

func TestRunBlueprintWidgetWriteRichTextStyleProperties(t *testing.T) {
	binaryPath := buildBPXBinaryForTest(t)
	work, childPath := buildRichTextWidgetTestAssetWithBinary(t, binaryPath)

	for _, argv := range [][]string{
		{
			"blueprint", "widget-write", work,
			"--widget", childPath,
			"--property", "richtext-default-font",
			"--value", "/Game/UI/Fonts/NotoSans",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", childPath,
			"--property", "richtext-default-typeface",
			"--value", "Regular",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", childPath,
			"--property", "richtext-default-font-size",
			"--value", "28",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", childPath,
			"--property", "richtext-default-color-and-opacity",
			"--value", "1,0.9,0.25,1",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", childPath,
			"--property", "richtext-default-shadow-offset",
			"--value", "3,4",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", childPath,
			"--property", "richtext-default-shadow-color-and-opacity",
			"--value", "0.05,0.1,0.15,0.8",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", childPath,
			"--property", "richtext-default-outline-size",
			"--value", "2",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", childPath,
			"--property", "richtext-default-outline-color",
			"--value", "0.2,0.3,0.9,1",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", childPath,
			"--property", "richtext-auto-wrap-text",
			"--value", "true",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", childPath,
			"--property", "richtext-wrap-text-at",
			"--value", "320",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", childPath,
			"--property", "richtext-line-height-percentage",
			"--value", "1.25",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", childPath,
			"--property", "richtext-justification",
			"--value", "Center",
		},
	} {
		runBPXBinaryCommand(t, binaryPath, argv...)
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse final asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets final: %v", err)
	}
	target, err := selectWidgetWriteTarget(targets, childPath)
	if err != nil {
		t.Fatalf("select final widget: %v", err)
	}
	for _, exportIdx := range target.Exports {
		override, err := decodeExportRootPropertyValue(asset, exportIdx, "bOverrideDefaultStyle")
		if err != nil {
			t.Fatalf("decode export %d bOverrideDefaultStyle: %v", exportIdx+1, err)
		}
		if got, ok := widgetReadBoolValue(override); !ok || !got {
			t.Fatalf("export %d bOverrideDefaultStyle: got %#v want true", exportIdx+1, override)
		}

		style, err := decodeExportRootPropertyValue(asset, exportIdx, "DefaultTextStyleOverride")
		if err != nil {
			t.Fatalf("decode export %d DefaultTextStyleOverride: %v", exportIdx+1, err)
		}
		styleFields := widgetDecodedStructValue(style)
		if styleFields == nil {
			t.Fatalf("export %d DefaultTextStyleOverride fields missing", exportIdx+1)
		}

		font, ok := widgetReadFontSummary(asset, styleFields["Font"])
		if !ok {
			t.Fatalf("export %d Font summary missing", exportIdx+1)
		}
		if got, want := font["size"], 28; got != want {
			t.Fatalf("export %d Font.Size: got %#v want %d", exportIdx+1, got, want)
		}
		if got, want := font["typefaceFontName"], "Regular"; got != want {
			t.Fatalf("export %d TypefaceFontName: got %#v want %q", exportIdx+1, got, want)
		}
		if got, want := font["fontObjectPath"], "/Game/UI/Fonts/NotoSans"; got != want {
			t.Fatalf("export %d FontObjectPath: got %#v want %q", exportIdx+1, got, want)
		}

		color, ok := widgetReadSlateColorSummary(styleFields["ColorAndOpacity"])
		if !ok {
			t.Fatalf("export %d ColorAndOpacity summary missing", exportIdx+1)
		}
		requireWidgetReadLinearColor(t, color, widgetLinearColor{R: 1, G: 0.9, B: 0.25, A: 1})
		if _, ok := color["colorUseRule"]; ok {
			t.Fatalf("export %d ColorUseRule unexpectedly present: %#v", exportIdx+1, color["colorUseRule"])
		}

		justification, err := decodeExportRootPropertyValue(asset, exportIdx, "Justification")
		if err != nil {
			t.Fatalf("decode export %d Justification: %v", exportIdx+1, err)
		}
		if got, ok := widgetReadEnumValue(justification); !ok || got != "ETextJustify::Center" {
			t.Fatalf("export %d Justification: got %#v want %q", exportIdx+1, got, "ETextJustify::Center")
		}

		autoWrapText, err := decodeExportRootPropertyValue(asset, exportIdx, "AutoWrapText")
		if err != nil {
			t.Fatalf("decode export %d AutoWrapText: %v", exportIdx+1, err)
		}
		if got, ok := widgetReadBoolValue(autoWrapText); !ok || !got {
			t.Fatalf("export %d AutoWrapText: got %#v want true", exportIdx+1, autoWrapText)
		}

		wrapTextAt, err := decodeExportRootPropertyValue(asset, exportIdx, "WrapTextAt")
		if err != nil {
			t.Fatalf("decode export %d WrapTextAt: %v", exportIdx+1, err)
		}
		if got, want := widgetDecodedScalarFloat(wrapTextAt), float32(320); got != want {
			t.Fatalf("export %d WrapTextAt: got %v want %v", exportIdx+1, got, want)
		}

		lineHeightPercentage, err := decodeExportRootPropertyValue(asset, exportIdx, "LineHeightPercentage")
		if err != nil {
			t.Fatalf("decode export %d LineHeightPercentage: %v", exportIdx+1, err)
		}
		if got, want := widgetDecodedScalarFloat(lineHeightPercentage), float32(1.25); got != want {
			t.Fatalf("export %d LineHeightPercentage: got %v want %v", exportIdx+1, got, want)
		}

		shadowOffset, ok := widgetReadVector2Summary(styleFields["ShadowOffset"])
		if !ok {
			t.Fatalf("export %d ShadowOffset summary missing", exportIdx+1)
		}
		requireWidgetReadFloatMap(t, shadowOffset, map[string]float32{"x": 3, "y": 4})

		shadowColor, ok := widgetReadLinearColorSummary(styleFields["ShadowColorAndOpacity"])
		if !ok {
			t.Fatalf("export %d ShadowColorAndOpacity summary missing", exportIdx+1)
		}
		requireWidgetReadLinearColor(t, shadowColor, widgetLinearColor{R: 0.05, G: 0.1, B: 0.15, A: 0.8})

		outline, ok := font["outlineSettings"].(map[string]any)
		if !ok {
			t.Fatalf("export %d Font.OutlineSettings summary missing", exportIdx+1)
		}
		if got, want := outline["outlineSize"], 2; got != want {
			t.Fatalf("export %d OutlineSize: got %#v want %d", exportIdx+1, got, want)
		}
		outlineColor, ok := outline["outlineColor"].(map[string]any)
		if !ok {
			t.Fatalf("export %d OutlineColor summary missing", exportIdx+1)
		}
		requireWidgetReadLinearColor(t, outlineColor, widgetLinearColor{R: 0.2, G: 0.3, B: 0.9, A: 1})
	}

	if importIdx, found := findFontImportByPath(asset, "/Game/UI/Fonts/NotoSans"); !found || importIdx <= 0 {
		t.Fatalf("font import not found for %s", "/Game/UI/Fonts/NotoSans")
	}
}

func TestRunBlueprintWidgetWriteProgressBarProperties(t *testing.T) {
	work, childPath := buildWidgetTestAsset(t, "canvaspanel", "CanvasPanel_21", "progressbar", "ProgressBar_1")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-write", work,
			"--widget", childPath,
			"--property", "progressbar-percent",
			"--value", "0.75",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", childPath,
			"--property", "progressbar-fill-color",
			"--value", "0.2,0.4,0.6,0.8",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
			t.Fatalf("widget-write exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse final asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets final: %v", err)
	}
	target, err := selectWidgetWriteTarget(targets, childPath)
	if err != nil {
		t.Fatalf("select final widget: %v", err)
	}
	for _, exportIdx := range target.Exports {
		percent, err := decodeExportRootPropertyValue(asset, exportIdx, "Percent")
		if err != nil {
			t.Fatalf("decode export %d Percent: %v", exportIdx+1, err)
		}
		if got := widgetDecodedScalarFloat(percent); got != 0.75 {
			t.Fatalf("export %d Percent: got %v want %v", exportIdx+1, got, float32(0.75))
		}

		fillColor, err := decodeExportRootPropertyValue(asset, exportIdx, "FillColorAndOpacity")
		if err != nil {
			t.Fatalf("decode export %d FillColorAndOpacity: %v", exportIdx+1, err)
		}
		colorSummary, ok := widgetReadLinearColorSummary(fillColor)
		if !ok {
			t.Fatalf("export %d FillColorAndOpacity summary missing", exportIdx+1)
		}
		requireWidgetReadLinearColor(t, colorSummary, widgetLinearColor{R: 0.2, G: 0.4, B: 0.6, A: 0.8})
	}

	blueprints := findWidgetBlueprintExports(asset)
	if len(blueprints) != 1 {
		t.Fatalf("widget blueprint exports: got %d want 1", len(blueprints))
	}
	entry, err := buildWidgetBlueprintEntry(asset, blueprints[0])
	if err != nil {
		t.Fatalf("buildWidgetBlueprintEntry: %v", err)
	}
	logical := requireWidgetReadLogicalWidgetByPath(t, entry, childPath)
	progressSummary, ok := logical["progressBar"].(map[string]any)
	if !ok {
		t.Fatalf("progressBar summary type: got %#v", logical["progressBar"])
	}
	if got, want := progressSummary["percent"], float32(0.75); got != want {
		t.Fatalf("progressBar.percent: got %#v want %v", got, want)
	}
}

func TestRunBlueprintWidgetWriteSliderProperties(t *testing.T) {
	work, childPath := buildWidgetTestAsset(t, "canvaspanel", "CanvasPanel_21", "slider", "Slider_1")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "slider-value", "--value", "0.5"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "slider-min-value", "--value", "0.1"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "slider-max-value", "--value", "0.9"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "slider-step-size", "--value", "0.05"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "slider-orientation", "--value", "Vertical"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "slider-is-focusable", "--value", "false"},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
			t.Fatalf("widget-write exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse final asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets final: %v", err)
	}
	target, err := selectWidgetWriteTarget(targets, childPath)
	if err != nil {
		t.Fatalf("select final widget: %v", err)
	}
	for _, exportIdx := range target.Exports {
		for _, tc := range []struct {
			property string
			want     float32
		}{
			{property: "Value", want: 0.5},
			{property: "MinValue", want: 0.1},
			{property: "MaxValue", want: 0.9},
			{property: "StepSize", want: 0.05},
		} {
			value, err := decodeExportRootPropertyValue(asset, exportIdx, tc.property)
			if err != nil {
				t.Fatalf("decode export %d %s: %v", exportIdx+1, tc.property, err)
			}
			if got := widgetDecodedScalarFloat(value); got != tc.want {
				t.Fatalf("export %d %s: got %v want %v", exportIdx+1, tc.property, got, tc.want)
			}
		}

		orientation, err := decodeExportRootPropertyValue(asset, exportIdx, "Orientation")
		if err != nil {
			t.Fatalf("decode export %d Orientation: %v", exportIdx+1, err)
		}
		if got, ok := widgetReadEnumValue(orientation); !ok || got != "EOrientation::Orient_Vertical" {
			t.Fatalf("export %d Orientation: got %#v want %q", exportIdx+1, got, "EOrientation::Orient_Vertical")
		}

		isFocusable, err := decodeExportRootPropertyValue(asset, exportIdx, "IsFocusable")
		if err != nil {
			t.Fatalf("decode export %d IsFocusable: %v", exportIdx+1, err)
		}
		if got, ok := widgetReadBoolValue(isFocusable); !ok || got {
			t.Fatalf("export %d IsFocusable: got %#v want false", exportIdx+1, got)
		}
	}
}

func TestRunBlueprintWidgetWriteSpacerSeparatorAndScrollBarProperties(t *testing.T) {
	tests := []struct {
		name      string
		childType string
		childName string
		writes    [][]string
		check     func(t *testing.T, asset *uasset.Asset, exportIdx int)
	}{
		{
			name:      "Spacer",
			childType: "spacer",
			childName: "Spacer_1",
			writes: [][]string{
				{"blueprint", "widget-write", "--property", "spacer-size", "--value", "24,48"},
			},
			check: func(t *testing.T, asset *uasset.Asset, exportIdx int) {
				t.Helper()
				size, err := decodeExportRootPropertyValue(asset, exportIdx, "Size")
				if err != nil {
					t.Fatalf("decode Size: %v", err)
				}
				summary, ok := widgetReadVector2Summary(size)
				if !ok {
					t.Fatalf("Size summary missing")
				}
				requireWidgetReadFloatMap(t, summary, map[string]float32{"x": 24, "y": 48})
			},
		},
		{
			name:      "ScrollBar",
			childType: "scrollbar",
			childName: "ScrollBar_1",
			writes: [][]string{
				{"blueprint", "widget-write", "--property", "scrollbar-thickness", "--value", "5,12"},
				{"blueprint", "widget-write", "--property", "scrollbar-orientation", "--value", "Horizontal"},
			},
			check: func(t *testing.T, asset *uasset.Asset, exportIdx int) {
				t.Helper()
				thickness, err := decodeExportRootPropertyValue(asset, exportIdx, "Thickness")
				if err != nil {
					t.Fatalf("decode Thickness: %v", err)
				}
				summary, ok := widgetReadVector2Summary(thickness)
				if !ok {
					t.Fatalf("Thickness summary missing")
				}
				requireWidgetReadFloatMap(t, summary, map[string]float32{"x": 5, "y": 12})
				orientation, err := decodeExportRootPropertyValue(asset, exportIdx, "Orientation")
				if err != nil {
					t.Fatalf("decode Orientation: %v", err)
				}
				if got, ok := widgetReadEnumValue(orientation); !ok || got != "EOrientation::Orient_Horizontal" {
					t.Fatalf("Orientation: got %#v want %q", got, "EOrientation::Orient_Horizontal")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			work, childPath := buildWidgetTestAsset(t, "canvaspanel", "CanvasPanel_21", tc.childType, tc.childName)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			for _, argv := range tc.writes {
				fullArgv := append([]string(nil), argv...)
				fullArgv = append(fullArgv[:2], append([]string{work, "--widget", childPath}, fullArgv[2:]...)...)
				stdout.Reset()
				stderr.Reset()
				if code := Run(fullArgv, &stdout, &stderr); code != 0 {
					t.Fatalf("widget-write exit code: got %d want 0 argv=%v stderr=%s", code, fullArgv, stderr.String())
				}
			}

			asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("parse final asset: %v", err)
			}
			targets, err := collectWidgetWriteTargets(asset, 0)
			if err != nil {
				t.Fatalf("collectWidgetWriteTargets final: %v", err)
			}
			target, err := selectWidgetWriteTarget(targets, childPath)
			if err != nil {
				t.Fatalf("select final widget: %v", err)
			}
			for _, exportIdx := range target.Exports {
				tc.check(t, asset, exportIdx)
			}
		})
	}
}

func TestRunBlueprintWidgetWriteTextBlockStyleProperties(t *testing.T) {
	work, childPath := buildWidgetTestAsset(t, "canvaspanel", "CanvasPanel_21", "textblock", "TextBlock_1")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "text-color", "--value", "1,0.9,0.25,1"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "text-font-family", "--value", "/Game/UI/Foundation/Fonts/NotoSans"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "text-typeface", "--value", "Bold"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "text-font-size", "--value", "28"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "text-justification", "--value", "Center"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "text-auto-wrap-text", "--value", "true"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "text-wrap-text-at", "--value", "320"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "text-line-height-percentage", "--value", "1.25"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "text-shadow-offset", "--value", "3,4"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "text-shadow-color-and-opacity", "--value", "0.1,0.2,0.3,0.8"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "text-outline-size", "--value", "2"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "text-outline-color", "--value", "0.8,0.7,0.6,1"},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
			t.Fatalf("widget-write exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse final asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets final: %v", err)
	}
	target, err := selectWidgetWriteTarget(targets, childPath)
	if err != nil {
		t.Fatalf("select final widget: %v", err)
	}
	for _, exportIdx := range target.Exports {
		colorValue, err := decodeExportRootPropertyValue(asset, exportIdx, "ColorAndOpacity")
		if err != nil {
			t.Fatalf("decode export %d ColorAndOpacity: %v", exportIdx+1, err)
		}
		colorSummary, ok := widgetReadSlateColorSummary(colorValue)
		if !ok {
			t.Fatalf("export %d ColorAndOpacity summary missing", exportIdx+1)
		}
		requireWidgetReadLinearColor(t, colorSummary, widgetLinearColor{R: 1, G: 0.9, B: 0.25, A: 1})

		fontValue, err := decodeExportRootPropertyValue(asset, exportIdx, "Font")
		if err != nil {
			t.Fatalf("decode export %d Font: %v", exportIdx+1, err)
		}
		fontSummary, ok := widgetReadFontSummary(asset, fontValue)
		if !ok {
			t.Fatalf("export %d Font summary missing", exportIdx+1)
		}
		if got, want := fontSummary["size"], 28; got != want {
			t.Fatalf("export %d Font.Size: got %#v want %d", exportIdx+1, got, want)
		}
		if got, want := fontSummary["fontObjectPath"], "/Game/UI/Foundation/Fonts/NotoSans"; got != want {
			t.Fatalf("export %d Font.FontObjectPath: got %#v want %q", exportIdx+1, got, want)
		}
		if got, want := fontSummary["typefaceFontName"], "Bold"; got != want {
			t.Fatalf("export %d Font.TypefaceFontName: got %#v want %q", exportIdx+1, got, want)
		}
		outlineSettings, ok := fontSummary["outlineSettings"].(map[string]any)
		if !ok {
			t.Fatalf("export %d Font.OutlineSettings summary missing", exportIdx+1)
		}
		if got, want := outlineSettings["outlineSize"], 2; got != want {
			t.Fatalf("export %d Font.OutlineSettings.OutlineSize: got %#v want %d", exportIdx+1, got, want)
		}
		requireWidgetReadLinearColor(t, outlineSettings["outlineColor"], widgetLinearColor{R: 0.8, G: 0.7, B: 0.6, A: 1})

		justification, err := decodeExportRootPropertyValue(asset, exportIdx, "Justification")
		if err != nil {
			t.Fatalf("decode export %d Justification: %v", exportIdx+1, err)
		}
		if got, ok := widgetReadEnumValue(justification); !ok || got != "ETextJustify::Center" {
			t.Fatalf("export %d Justification: got %#v want %q", exportIdx+1, got, "ETextJustify::Center")
		}

		autoWrapText, err := decodeExportRootPropertyValue(asset, exportIdx, "AutoWrapText")
		if err != nil {
			t.Fatalf("decode export %d AutoWrapText: %v", exportIdx+1, err)
		}
		if got, ok := widgetReadBoolValue(autoWrapText); !ok || !got {
			t.Fatalf("export %d AutoWrapText: got %#v want true", exportIdx+1, got)
		}

		wrapTextAt, err := decodeExportRootPropertyValue(asset, exportIdx, "WrapTextAt")
		if err != nil {
			t.Fatalf("decode export %d WrapTextAt: %v", exportIdx+1, err)
		}
		if got, want := widgetDecodedScalarFloat(wrapTextAt), float32(320); got != want {
			t.Fatalf("export %d WrapTextAt: got %v want %v", exportIdx+1, got, want)
		}

		lineHeightPercentage, err := decodeExportRootPropertyValue(asset, exportIdx, "LineHeightPercentage")
		if err != nil {
			t.Fatalf("decode export %d LineHeightPercentage: %v", exportIdx+1, err)
		}
		if got, want := widgetDecodedScalarFloat(lineHeightPercentage), float32(1.25); got != want {
			t.Fatalf("export %d LineHeightPercentage: got %v want %v", exportIdx+1, got, want)
		}

		shadowOffset, err := decodeExportRootPropertyValue(asset, exportIdx, "ShadowOffset")
		if err != nil {
			t.Fatalf("decode export %d ShadowOffset: %v", exportIdx+1, err)
		}
		shadowOffsetSummary, ok := widgetReadVector2Summary(shadowOffset)
		if !ok {
			t.Fatalf("export %d ShadowOffset summary missing", exportIdx+1)
		}
		requireWidgetReadFloatMap(t, shadowOffsetSummary, map[string]float32{"x": 3, "y": 4})

		shadowColor, err := decodeExportRootPropertyValue(asset, exportIdx, "ShadowColorAndOpacity")
		if err != nil {
			t.Fatalf("decode export %d ShadowColorAndOpacity: %v", exportIdx+1, err)
		}
		requireWidgetReadLinearColor(t, shadowColor, widgetLinearColor{R: 0.1, G: 0.2, B: 0.3, A: 0.8})
	}
}

func TestRunBlueprintWidgetWriteSizeBoxAndScrollBoxProperties(t *testing.T) {
	tests := []struct {
		name      string
		childType string
		childName string
		writes    [][]string
		check     func(t *testing.T, asset *uasset.Asset, exportIdx int)
	}{
		{
			name:      "SizeBox",
			childType: "sizebox",
			childName: "SizeBox_1",
			writes: [][]string{
				{"blueprint", "widget-write", "--property", "sizebox-width", "--value", "320"},
				{"blueprint", "widget-write", "--property", "sizebox-height", "--value", "72"},
				{"blueprint", "widget-write", "--property", "sizebox-min-desired-width", "--value", "120"},
				{"blueprint", "widget-write", "--property", "sizebox-max-desired-height", "--value", "240"},
				{"blueprint", "widget-write", "--property", "sizebox-min-aspect-ratio", "--value", "1.2"},
				{"blueprint", "widget-write", "--property", "sizebox-max-aspect-ratio", "--value", "1.8"},
			},
			check: func(t *testing.T, asset *uasset.Asset, exportIdx int) {
				t.Helper()
				width, err := decodeExportRootPropertyValue(asset, exportIdx, "WidthOverride")
				if err != nil {
					t.Fatalf("decode WidthOverride: %v", err)
				}
				if got, want := widgetDecodedScalarFloat(width), float32(320); got != want {
					t.Fatalf("WidthOverride: got %v want %v", got, want)
				}
				height, err := decodeExportRootPropertyValue(asset, exportIdx, "HeightOverride")
				if err != nil {
					t.Fatalf("decode HeightOverride: %v", err)
				}
				if got, want := widgetDecodedScalarFloat(height), float32(72); got != want {
					t.Fatalf("HeightOverride: got %v want %v", got, want)
				}
				minDesiredWidth, err := decodeExportRootPropertyValue(asset, exportIdx, "MinDesiredWidth")
				if err != nil {
					t.Fatalf("decode MinDesiredWidth: %v", err)
				}
				if got, want := widgetDecodedScalarFloat(minDesiredWidth), float32(120); got != want {
					t.Fatalf("MinDesiredWidth: got %v want %v", got, want)
				}
				maxDesiredHeight, err := decodeExportRootPropertyValue(asset, exportIdx, "MaxDesiredHeight")
				if err != nil {
					t.Fatalf("decode MaxDesiredHeight: %v", err)
				}
				if got, want := widgetDecodedScalarFloat(maxDesiredHeight), float32(240); got != want {
					t.Fatalf("MaxDesiredHeight: got %v want %v", got, want)
				}
				minAspectRatio, err := decodeExportRootPropertyValue(asset, exportIdx, "MinAspectRatio")
				if err != nil {
					t.Fatalf("decode MinAspectRatio: %v", err)
				}
				if got, want := widgetDecodedScalarFloat(minAspectRatio), float32(1.2); got != want {
					t.Fatalf("MinAspectRatio: got %v want %v", got, want)
				}
				maxAspectRatio, err := decodeExportRootPropertyValue(asset, exportIdx, "MaxAspectRatio")
				if err != nil {
					t.Fatalf("decode MaxAspectRatio: %v", err)
				}
				if got, want := widgetDecodedScalarFloat(maxAspectRatio), float32(1.8); got != want {
					t.Fatalf("MaxAspectRatio: got %v want %v", got, want)
				}
			},
		},
		{
			name:      "ScrollBox",
			childType: "scrollbox",
			childName: "ScrollBox_1",
			writes: [][]string{
				{"blueprint", "widget-write", "--property", "scrollbox-orientation", "--value", "Horizontal"},
				{"blueprint", "widget-write", "--property", "scrollbox-scrollbar-visibility", "--value", "Collapsed"},
				{"blueprint", "widget-write", "--property", "scrollbox-consume-mouse-wheel", "--value", "Always"},
				{"blueprint", "widget-write", "--property", "scrollbox-is-focusable", "--value", "true"},
			},
			check: func(t *testing.T, asset *uasset.Asset, exportIdx int) {
				t.Helper()
				orientation, err := decodeExportRootPropertyValue(asset, exportIdx, "Orientation")
				if err != nil {
					t.Fatalf("decode Orientation: %v", err)
				}
				if got, ok := widgetReadEnumValue(orientation); !ok || got != "EOrientation::Orient_Horizontal" {
					t.Fatalf("Orientation: got %#v want %q", got, "EOrientation::Orient_Horizontal")
				}
				scrollBarVisibility, err := decodeExportRootPropertyValue(asset, exportIdx, "ScrollBarVisibility")
				if err != nil {
					t.Fatalf("decode ScrollBarVisibility: %v", err)
				}
				if got, ok := widgetReadEnumValue(scrollBarVisibility); !ok || got != "ESlateVisibility::Collapsed" {
					t.Fatalf("ScrollBarVisibility: got %#v want %q", got, "ESlateVisibility::Collapsed")
				}
				consumeMouseWheel, err := decodeExportRootPropertyValue(asset, exportIdx, "ConsumeMouseWheel")
				if err != nil {
					t.Fatalf("decode ConsumeMouseWheel: %v", err)
				}
				if got, ok := widgetReadEnumValue(consumeMouseWheel); !ok || got != "EConsumeMouseWheel::Always" {
					t.Fatalf("ConsumeMouseWheel: got %#v want %q", got, "EConsumeMouseWheel::Always")
				}
				isFocusable, err := decodeExportRootPropertyValue(asset, exportIdx, "bIsFocusable")
				if err != nil {
					t.Fatalf("decode bIsFocusable: %v", err)
				}
				if got, ok := widgetReadBoolValue(isFocusable); !ok || !got {
					t.Fatalf("bIsFocusable: got %#v want true", isFocusable)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			work, childPath := buildWidgetTestAsset(t, "canvaspanel", "CanvasPanel_21", tc.childType, tc.childName)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			for _, argv := range tc.writes {
				fullArgv := append([]string(nil), argv...)
				fullArgv = append(fullArgv[:2], append([]string{work, "--widget", childPath}, fullArgv[2:]...)...)
				stdout.Reset()
				stderr.Reset()
				if code := Run(fullArgv, &stdout, &stderr); code != 0 {
					t.Fatalf("widget-write exit code: got %d want 0 argv=%v stderr=%s", code, fullArgv, stderr.String())
				}
			}

			asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("parse final asset: %v", err)
			}
			targets, err := collectWidgetWriteTargets(asset, 0)
			if err != nil {
				t.Fatalf("collectWidgetWriteTargets final: %v", err)
			}
			target, err := selectWidgetWriteTarget(targets, childPath)
			if err != nil {
				t.Fatalf("select final widget: %v", err)
			}
			for _, exportIdx := range target.Exports {
				tc.check(t, asset, exportIdx)
			}
		})
	}
}

func TestRunBlueprintWidgetWriteCheckBoxProperties(t *testing.T) {
	work, childPath := buildWidgetTestAsset(t, "canvaspanel", "CanvasPanel_21", "checkbox", "CheckBox_1")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "checkbox-is-checked", "--value", "true"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "checkbox-checked-state", "--value", "Undetermined"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "checkbox-is-focusable", "--value", "false"},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
			t.Fatalf("widget-write exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse final asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets final: %v", err)
	}
	target, err := selectWidgetWriteTarget(targets, childPath)
	if err != nil {
		t.Fatalf("select final widget: %v", err)
	}
	for _, exportIdx := range target.Exports {
		checkedState, err := decodeExportRootPropertyValue(asset, exportIdx, "CheckedState")
		if err != nil {
			t.Fatalf("decode export %d CheckedState: %v", exportIdx+1, err)
		}
		if got, ok := widgetReadEnumValue(checkedState); !ok || got != "ECheckBoxState::Undetermined" {
			t.Fatalf("export %d CheckedState: got %#v want %q", exportIdx+1, got, "ECheckBoxState::Undetermined")
		}
		isFocusable, err := decodeExportRootPropertyValue(asset, exportIdx, "IsFocusable")
		if err != nil {
			t.Fatalf("decode export %d IsFocusable: %v", exportIdx+1, err)
		}
		if got, ok := widgetReadBoolValue(isFocusable); !ok || got {
			t.Fatalf("export %d IsFocusable: got %#v want false", exportIdx+1, got)
		}
	}
}

func TestRunBlueprintWidgetWriteEditableTextBoxProperties(t *testing.T) {
	work, childPath := buildWidgetTestAsset(t, "canvaspanel", "CanvasPanel_21", "editabletextbox", "EditableTextBox_1")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "text", "--value", "Player Name"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "editabletextbox-hint-text", "--value", "Enter name"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "editabletextbox-is-read-only", "--value", "true"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "editabletextbox-is-password", "--value", "true"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "editabletextbox-minimum-desired-width", "--value", "240"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "editabletextbox-justification", "--value", "Center"},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
			t.Fatalf("widget-write exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse final asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets final: %v", err)
	}
	target, err := selectWidgetWriteTarget(targets, childPath)
	if err != nil {
		t.Fatalf("select final widget: %v", err)
	}
	for _, exportIdx := range target.Exports {
		textValue, err := decodeExportRootPropertyValue(asset, exportIdx, "Text")
		if err != nil {
			t.Fatalf("decode export %d Text: %v", exportIdx+1, err)
		}
		textSummary := widgetReadTextSummary(textValue)
		if got, want := textSummary["sourceString"], "Player Name"; got != want {
			t.Fatalf("export %d Text.sourceString: got %#v want %q", exportIdx+1, got, want)
		}

		hintText, err := decodeExportRootPropertyValue(asset, exportIdx, "HintText")
		if err != nil {
			t.Fatalf("decode export %d HintText: %v", exportIdx+1, err)
		}
		hintSummary := widgetReadTextSummary(hintText)
		if got, want := hintSummary["sourceString"], "Enter name"; got != want {
			t.Fatalf("export %d HintText.sourceString: got %#v want %q", exportIdx+1, got, want)
		}

		isReadOnly, err := decodeExportRootPropertyValue(asset, exportIdx, "IsReadOnly")
		if err != nil {
			t.Fatalf("decode export %d IsReadOnly: %v", exportIdx+1, err)
		}
		if got, ok := widgetReadBoolValue(isReadOnly); !ok || !got {
			t.Fatalf("export %d IsReadOnly: got %#v want true", exportIdx+1, isReadOnly)
		}

		isPassword, err := decodeExportRootPropertyValue(asset, exportIdx, "IsPassword")
		if err != nil {
			t.Fatalf("decode export %d IsPassword: %v", exportIdx+1, err)
		}
		if got, ok := widgetReadBoolValue(isPassword); !ok || !got {
			t.Fatalf("export %d IsPassword: got %#v want true", exportIdx+1, isPassword)
		}

		minDesiredWidth, err := decodeExportRootPropertyValue(asset, exportIdx, "MinimumDesiredWidth")
		if err != nil {
			t.Fatalf("decode export %d MinimumDesiredWidth: %v", exportIdx+1, err)
		}
		if got, want := widgetDecodedScalarFloat(minDesiredWidth), float32(240); got != want {
			t.Fatalf("export %d MinimumDesiredWidth: got %v want %v", exportIdx+1, got, want)
		}

		justification, err := decodeExportRootPropertyValue(asset, exportIdx, "Justification")
		if err != nil {
			t.Fatalf("decode export %d Justification: %v", exportIdx+1, err)
		}
		if got, ok := widgetReadEnumValue(justification); !ok || got != "ETextJustify::Center" {
			t.Fatalf("export %d Justification: got %#v want %q", exportIdx+1, got, "ETextJustify::Center")
		}
	}

	blueprints := findWidgetBlueprintExports(asset)
	if len(blueprints) != 1 {
		t.Fatalf("widget blueprint exports: got %d want 1", len(blueprints))
	}
	entry, err := buildWidgetBlueprintEntry(asset, blueprints[0])
	if err != nil {
		t.Fatalf("buildWidgetBlueprintEntry: %v", err)
	}
	logical := requireWidgetReadLogicalWidgetByPath(t, entry, childPath)
	summary, ok := logical["editableTextBox"].(map[string]any)
	if !ok {
		t.Fatalf("editableTextBox summary type: got %#v", logical["editableTextBox"])
	}
	if got, want := summary["minimumDesiredWidth"], float32(240); got != want {
		t.Fatalf("editableTextBox.minimumDesiredWidth: got %#v want %v", got, want)
	}
	if got, want := summary["justification"], "ETextJustify::Center"; got != want {
		t.Fatalf("editableTextBox.justification: got %#v want %q", got, want)
	}
}

func TestRunBlueprintWidgetWriteEditableTextProperties(t *testing.T) {
	work, childPath := buildWidgetTestAsset(t, "canvaspanel", "CanvasPanel_21", "editabletext", "EditableText_1")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "text", "--value", "Display Name"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "editabletext-hint-text", "--value", "Enter display name"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "editabletext-is-read-only", "--value", "true"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "editabletext-is-password", "--value", "true"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "editabletext-minimum-desired-width", "--value", "260"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "editabletext-justification", "--value", "Right"},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
			t.Fatalf("widget-write exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse final asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets final: %v", err)
	}
	target, err := selectWidgetWriteTarget(targets, childPath)
	if err != nil {
		t.Fatalf("select final widget: %v", err)
	}
	for _, exportIdx := range target.Exports {
		textValue, err := decodeExportRootPropertyValue(asset, exportIdx, "Text")
		if err != nil {
			t.Fatalf("decode export %d Text: %v", exportIdx+1, err)
		}
		if got, want := widgetReadTextSummary(textValue)["sourceString"], "Display Name"; got != want {
			t.Fatalf("export %d Text.sourceString: got %#v want %q", exportIdx+1, got, want)
		}
		hintText, err := decodeExportRootPropertyValue(asset, exportIdx, "HintText")
		if err != nil {
			t.Fatalf("decode export %d HintText: %v", exportIdx+1, err)
		}
		if got, want := widgetReadTextSummary(hintText)["sourceString"], "Enter display name"; got != want {
			t.Fatalf("export %d HintText.sourceString: got %#v want %q", exportIdx+1, got, want)
		}
		isReadOnly, err := decodeExportRootPropertyValue(asset, exportIdx, "IsReadOnly")
		if err != nil {
			t.Fatalf("decode export %d IsReadOnly: %v", exportIdx+1, err)
		}
		if got, ok := widgetReadBoolValue(isReadOnly); !ok || !got {
			t.Fatalf("export %d IsReadOnly: got %#v want true", exportIdx+1, isReadOnly)
		}
		isPassword, err := decodeExportRootPropertyValue(asset, exportIdx, "IsPassword")
		if err != nil {
			t.Fatalf("decode export %d IsPassword: %v", exportIdx+1, err)
		}
		if got, ok := widgetReadBoolValue(isPassword); !ok || !got {
			t.Fatalf("export %d IsPassword: got %#v want true", exportIdx+1, isPassword)
		}
		minDesiredWidth, err := decodeExportRootPropertyValue(asset, exportIdx, "MinimumDesiredWidth")
		if err != nil {
			t.Fatalf("decode export %d MinimumDesiredWidth: %v", exportIdx+1, err)
		}
		if got, want := widgetDecodedScalarFloat(minDesiredWidth), float32(260); got != want {
			t.Fatalf("export %d MinimumDesiredWidth: got %v want %v", exportIdx+1, got, want)
		}
		justification, err := decodeExportRootPropertyValue(asset, exportIdx, "Justification")
		if err != nil {
			t.Fatalf("decode export %d Justification: %v", exportIdx+1, err)
		}
		if got, ok := widgetReadEnumValue(justification); !ok || got != "ETextJustify::Right" {
			t.Fatalf("export %d Justification: got %#v want %q", exportIdx+1, got, "ETextJustify::Right")
		}
	}

	blueprints := findWidgetBlueprintExports(asset)
	if len(blueprints) != 1 {
		t.Fatalf("widget blueprint exports: got %d want 1", len(blueprints))
	}
	entry, err := buildWidgetBlueprintEntry(asset, blueprints[0])
	if err != nil {
		t.Fatalf("buildWidgetBlueprintEntry: %v", err)
	}
	logical := requireWidgetReadLogicalWidgetByPath(t, entry, childPath)
	summary, ok := logical["editableText"].(map[string]any)
	if !ok {
		t.Fatalf("editableText summary type: got %#v", logical["editableText"])
	}
	if got, want := summary["minimumDesiredWidth"], float32(260); got != want {
		t.Fatalf("editableText.minimumDesiredWidth: got %#v want %v", got, want)
	}
	if got, want := summary["justification"], "ETextJustify::Right"; got != want {
		t.Fatalf("editableText.justification: got %#v want %q", got, want)
	}
}

func TestRunBlueprintWidgetWriteMultiLineEditableTextBoxProperties(t *testing.T) {
	work, childPath := buildWidgetTestAsset(t, "canvaspanel", "CanvasPanel_21", "multilineeditabletextbox", "MultiLineEditableTextBox_1")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "text", "--value", "Line 1\nLine 2"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "multilineeditabletextbox-hint-text", "--value", "Enter description"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "multilineeditabletextbox-is-read-only", "--value", "true"},
		{"blueprint", "widget-write", work, "--widget", childPath, "--property", "multilineeditabletextbox-justification", "--value", "Center"},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
			t.Fatalf("widget-write exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse final asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets final: %v", err)
	}
	target, err := selectWidgetWriteTarget(targets, childPath)
	if err != nil {
		t.Fatalf("select final widget: %v", err)
	}
	for _, exportIdx := range target.Exports {
		textValue, err := decodeExportRootPropertyValue(asset, exportIdx, "Text")
		if err != nil {
			t.Fatalf("decode export %d Text: %v", exportIdx+1, err)
		}
		if got, want := widgetReadTextSummary(textValue)["sourceString"], "Line 1\nLine 2"; got != want {
			t.Fatalf("export %d Text.sourceString: got %#v want %q", exportIdx+1, got, want)
		}
		hintText, err := decodeExportRootPropertyValue(asset, exportIdx, "HintText")
		if err != nil {
			t.Fatalf("decode export %d HintText: %v", exportIdx+1, err)
		}
		if got, want := widgetReadTextSummary(hintText)["sourceString"], "Enter description"; got != want {
			t.Fatalf("export %d HintText.sourceString: got %#v want %q", exportIdx+1, got, want)
		}
		isReadOnly, err := decodeExportRootPropertyValue(asset, exportIdx, "bIsReadOnly")
		if err != nil {
			t.Fatalf("decode export %d bIsReadOnly: %v", exportIdx+1, err)
		}
		if got, ok := widgetReadBoolValue(isReadOnly); !ok || !got {
			t.Fatalf("export %d bIsReadOnly: got %#v want true", exportIdx+1, isReadOnly)
		}
		justification, err := decodeExportRootPropertyValue(asset, exportIdx, "Justification")
		if err != nil {
			t.Fatalf("decode export %d Justification: %v", exportIdx+1, err)
		}
		if got, ok := widgetReadEnumValue(justification); !ok || got != "ETextJustify::Center" {
			t.Fatalf("export %d Justification: got %#v want %q", exportIdx+1, got, "ETextJustify::Center")
		}
	}

	blueprints := findWidgetBlueprintExports(asset)
	if len(blueprints) != 1 {
		t.Fatalf("widget blueprint exports: got %d want 1", len(blueprints))
	}
	entry, err := buildWidgetBlueprintEntry(asset, blueprints[0])
	if err != nil {
		t.Fatalf("buildWidgetBlueprintEntry: %v", err)
	}
	logical := requireWidgetReadLogicalWidgetByPath(t, entry, childPath)
	summary, ok := logical["multiLineEditableTextBox"].(map[string]any)
	if !ok {
		t.Fatalf("multiLineEditableTextBox summary type: got %#v", logical["multiLineEditableTextBox"])
	}
	if got, want := summary["justification"], "ETextJustify::Center"; got != want {
		t.Fatalf("multiLineEditableTextBox.justification: got %#v want %q", got, want)
	}
}

func TestRunBlueprintWidgetWriteSpinBoxAndComboBoxStringProperties(t *testing.T) {
	tests := []struct {
		name       string
		childType  string
		childName  string
		writes     [][]string
		check      func(t *testing.T, asset *uasset.Asset, exportIdx int)
		summaryKey string
	}{
		{
			name:      "SpinBox",
			childType: "spinbox",
			childName: "SpinBox_1",
			writes: [][]string{
				{"blueprint", "widget-write", "--property", "spinbox-value", "--value", "42"},
				{"blueprint", "widget-write", "--property", "spinbox-min-value", "--value", "10"},
				{"blueprint", "widget-write", "--property", "spinbox-max-value", "--value", "100"},
				{"blueprint", "widget-write", "--property", "spinbox-delta", "--value", "5"},
			},
			check: func(t *testing.T, asset *uasset.Asset, exportIdx int) {
				t.Helper()
				value, err := decodeExportRootPropertyValue(asset, exportIdx, "Value")
				if err != nil {
					t.Fatalf("decode Value: %v", err)
				}
				if got, want := widgetDecodedScalarFloat(value), float32(42); got != want {
					t.Fatalf("Value: got %v want %v", got, want)
				}
				minValue, err := decodeExportRootPropertyValue(asset, exportIdx, "MinValue")
				if err != nil {
					t.Fatalf("decode MinValue: %v", err)
				}
				if got, want := widgetDecodedScalarFloat(minValue), float32(10); got != want {
					t.Fatalf("MinValue: got %v want %v", got, want)
				}
				overrideMin, err := decodeExportRootPropertyValue(asset, exportIdx, "bOverride_MinValue")
				if err != nil {
					t.Fatalf("decode bOverride_MinValue: %v", err)
				}
				if got, ok := widgetReadBoolValue(overrideMin); !ok || !got {
					t.Fatalf("bOverride_MinValue: got %#v want true", overrideMin)
				}
				maxValue, err := decodeExportRootPropertyValue(asset, exportIdx, "MaxValue")
				if err != nil {
					t.Fatalf("decode MaxValue: %v", err)
				}
				if got, want := widgetDecodedScalarFloat(maxValue), float32(100); got != want {
					t.Fatalf("MaxValue: got %v want %v", got, want)
				}
				overrideMax, err := decodeExportRootPropertyValue(asset, exportIdx, "bOverride_MaxValue")
				if err != nil {
					t.Fatalf("decode bOverride_MaxValue: %v", err)
				}
				if got, ok := widgetReadBoolValue(overrideMax); !ok || !got {
					t.Fatalf("bOverride_MaxValue: got %#v want true", overrideMax)
				}
				delta, err := decodeExportRootPropertyValue(asset, exportIdx, "Delta")
				if err != nil {
					t.Fatalf("decode Delta: %v", err)
				}
				if got, want := widgetDecodedScalarFloat(delta), float32(5); got != want {
					t.Fatalf("Delta: got %v want %v", got, want)
				}
			},
			summaryKey: "spinBox",
		},
		{
			name:      "ComboBoxString",
			childType: "comboboxstring",
			childName: "ComboBoxString_1",
			writes: [][]string{
				{"blueprint", "widget-write", "--property", "comboboxstring-options", "--value", "[\"Easy\",\"Normal\",\"Hard\"]"},
				{"blueprint", "widget-write", "--property", "comboboxstring-selected-option", "--value", "Normal"},
				{"blueprint", "widget-write", "--property", "comboboxstring-is-focusable", "--value", "false"},
			},
			check: func(t *testing.T, asset *uasset.Asset, exportIdx int) {
				t.Helper()
				options, err := decodeExportRootPropertyValue(asset, exportIdx, "DefaultOptions")
				if err != nil {
					t.Fatalf("decode DefaultOptions: %v", err)
				}
				values, ok := widgetReadStringArraySummary(options)
				if !ok || len(values) != 3 || values[0] != "Easy" || values[1] != "Normal" || values[2] != "Hard" {
					t.Fatalf("DefaultOptions: got %#v", options)
				}
				selected, err := decodeExportRootPropertyValue(asset, exportIdx, "SelectedOption")
				if err != nil {
					t.Fatalf("decode SelectedOption: %v", err)
				}
				if got, ok := widgetReadStringValue(selected); !ok || got != "Normal" {
					t.Fatalf("SelectedOption: got %#v want %q", selected, "Normal")
				}
				isFocusable, err := decodeExportRootPropertyValue(asset, exportIdx, "bIsFocusable")
				if err != nil {
					t.Fatalf("decode bIsFocusable: %v", err)
				}
				if got, ok := widgetReadBoolValue(isFocusable); !ok || got {
					t.Fatalf("bIsFocusable: got %#v want false", isFocusable)
				}
			},
			summaryKey: "comboBoxString",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			work, childPath := buildWidgetTestAsset(t, "canvaspanel", "CanvasPanel_21", tt.childType, tt.childName)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			for _, argv := range tt.writes {
				fullArgv := append([]string(nil), argv...)
				fullArgv = append(fullArgv[:2], append([]string{work, "--widget", childPath}, fullArgv[2:]...)...)
				stdout.Reset()
				stderr.Reset()
				if code := Run(fullArgv, &stdout, &stderr); code != 0 {
					t.Fatalf("widget-write exit code: got %d want 0 argv=%v stderr=%s", code, fullArgv, stderr.String())
				}
			}

			asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("parse final asset: %v", err)
			}
			targets, err := collectWidgetWriteTargets(asset, 0)
			if err != nil {
				t.Fatalf("collectWidgetWriteTargets final: %v", err)
			}
			target, err := selectWidgetWriteTarget(targets, childPath)
			if err != nil {
				t.Fatalf("select final widget: %v", err)
			}
			for _, exportIdx := range target.Exports {
				tt.check(t, asset, exportIdx)
			}

			blueprints := findWidgetBlueprintExports(asset)
			if len(blueprints) != 1 {
				t.Fatalf("widget blueprint exports: got %d want 1", len(blueprints))
			}
			entry, err := buildWidgetBlueprintEntry(asset, blueprints[0])
			if err != nil {
				t.Fatalf("buildWidgetBlueprintEntry: %v", err)
			}
			logical := requireWidgetReadLogicalWidgetByPath(t, entry, childPath)
			if _, ok := logical[tt.summaryKey].(map[string]any); !ok {
				t.Fatalf("%s summary type: got %#v", tt.summaryKey, logical[tt.summaryKey])
			}
		})
	}
}

func TestRunBlueprintWidgetWriteListViewAndTileViewProperties(t *testing.T) {
	tests := []struct {
		name       string
		childType  string
		childName  string
		writes     [][]string
		check      func(t *testing.T, asset *uasset.Asset, exportIdx int)
		summaryKey string
	}{
		{
			name:      "ListView",
			childType: "listview",
			childName: "ListView_1",
			writes: [][]string{
				{"blueprint", "widget-write", "--property", "listview-entry-widget-class", "--value", "/Game/WBP/WBP_ListEntry_Text"},
				{"blueprint", "widget-write", "--property", "listview-orientation", "--value", "Horizontal"},
				{"blueprint", "widget-write", "--property", "listview-selection-mode", "--value", "Multi"},
				{"blueprint", "widget-write", "--property", "listview-consume-mouse-wheel", "--value", "Never"},
				{"blueprint", "widget-write", "--property", "listview-is-focusable", "--value", "false"},
				{"blueprint", "widget-write", "--property", "listview-return-focus-to-selection", "--value", "true"},
				{"blueprint", "widget-write", "--property", "listview-clear-scroll-velocity-on-selection", "--value", "false"},
				{"blueprint", "widget-write", "--property", "listview-scroll-into-view-alignment", "--value", "BottomOrRight"},
				{"blueprint", "widget-write", "--property", "listview-wheel-scroll-multiplier", "--value", "2.5"},
				{"blueprint", "widget-write", "--property", "listview-enable-scroll-animation", "--value", "true"},
				{"blueprint", "widget-write", "--property", "listview-allow-overscroll", "--value", "false"},
				{"blueprint", "widget-write", "--property", "listview-enable-right-click-scrolling", "--value", "false"},
				{"blueprint", "widget-write", "--property", "listview-enable-touch-scrolling", "--value", "false"},
				{"blueprint", "widget-write", "--property", "listview-is-pointer-scrolling-enabled", "--value", "false"},
				{"blueprint", "widget-write", "--property", "listview-is-gamepad-scrolling-enabled", "--value", "false"},
				{"blueprint", "widget-write", "--property", "listview-horizontal-entry-spacing", "--value", "12"},
				{"blueprint", "widget-write", "--property", "listview-vertical-entry-spacing", "--value", "6"},
				{"blueprint", "widget-write", "--property", "listview-scrollbar-padding", "--value", "1,2,3,4"},
			},
			check: func(t *testing.T, asset *uasset.Asset, exportIdx int) {
				t.Helper()
				entryWidgetClass, err := decodeExportRootPropertyValue(asset, exportIdx, "EntryWidgetClass")
				if err != nil {
					t.Fatalf("decode EntryWidgetClass: %v", err)
				}
				entrySummary, ok := widgetReadObjectRefSummary(asset, entryWidgetClass)
				if !ok {
					t.Fatalf("EntryWidgetClass summary missing")
				}
				if got, want := entrySummary["path"], "/Game/WBP/WBP_ListEntry_Text"; got != want {
					t.Fatalf("EntryWidgetClass path: got %#v want %q", got, want)
				}
				orientation, err := decodeExportRootPropertyValue(asset, exportIdx, "Orientation")
				if err != nil {
					t.Fatalf("decode Orientation: %v", err)
				}
				if got, ok := widgetReadEnumValue(orientation); !ok || got != "EOrientation::Orient_Horizontal" {
					t.Fatalf("Orientation: got %#v want %q", got, "EOrientation::Orient_Horizontal")
				}
				selectionMode, err := decodeExportRootPropertyValue(asset, exportIdx, "SelectionMode")
				if err != nil {
					t.Fatalf("decode SelectionMode: %v", err)
				}
				if got, ok := widgetReadEnumValue(selectionMode); !ok || got != "ESelectionMode::Multi" {
					t.Fatalf("SelectionMode: got %#v want %q", got, "ESelectionMode::Multi")
				}
				consumeMouseWheel, err := decodeExportRootPropertyValue(asset, exportIdx, "ConsumeMouseWheel")
				if err != nil {
					t.Fatalf("decode ConsumeMouseWheel: %v", err)
				}
				if got, ok := widgetReadEnumValue(consumeMouseWheel); !ok || got != "EConsumeMouseWheel::Never" {
					t.Fatalf("ConsumeMouseWheel: got %#v want %q", got, "EConsumeMouseWheel::Never")
				}
				isFocusable, err := decodeExportRootPropertyValue(asset, exportIdx, "bIsFocusable")
				if err != nil {
					t.Fatalf("decode bIsFocusable: %v", err)
				}
				if got, ok := widgetReadBoolValue(isFocusable); !ok || got {
					t.Fatalf("bIsFocusable: got %#v want false", isFocusable)
				}
				returnFocusToSelection, err := decodeExportRootPropertyValue(asset, exportIdx, "bReturnFocusToSelection")
				if err != nil {
					t.Fatalf("decode bReturnFocusToSelection: %v", err)
				}
				if got, ok := widgetReadBoolValue(returnFocusToSelection); !ok || !got {
					t.Fatalf("bReturnFocusToSelection: got %#v want true", returnFocusToSelection)
				}
				clearScrollVelocityOnSelection, err := decodeExportRootPropertyValue(asset, exportIdx, "bClearScrollVelocityOnSelection")
				if err != nil {
					t.Fatalf("decode bClearScrollVelocityOnSelection: %v", err)
				}
				if got, ok := widgetReadBoolValue(clearScrollVelocityOnSelection); !ok || got {
					t.Fatalf("bClearScrollVelocityOnSelection: got %#v want false", clearScrollVelocityOnSelection)
				}
				scrollIntoViewAlignment, err := decodeExportRootPropertyValue(asset, exportIdx, "ScrollIntoViewAlignment")
				if err != nil {
					t.Fatalf("decode ScrollIntoViewAlignment: %v", err)
				}
				if got, ok := widgetReadEnumValue(scrollIntoViewAlignment); !ok || got != "EScrollIntoViewAlignment::BottomOrRight" {
					t.Fatalf("ScrollIntoViewAlignment: got %#v want %q", got, "EScrollIntoViewAlignment::BottomOrRight")
				}
				wheelScrollMultiplier, err := decodeExportRootPropertyValue(asset, exportIdx, "WheelScrollMultiplier")
				if err != nil {
					t.Fatalf("decode WheelScrollMultiplier: %v", err)
				}
				if got, want := widgetDecodedScalarFloat(wheelScrollMultiplier), float32(2.5); got != want {
					t.Fatalf("WheelScrollMultiplier: got %v want %v", got, want)
				}
				enableScrollAnimation, err := decodeExportRootPropertyValue(asset, exportIdx, "bEnableScrollAnimation")
				if err != nil {
					t.Fatalf("decode bEnableScrollAnimation: %v", err)
				}
				if got, ok := widgetReadBoolValue(enableScrollAnimation); !ok || !got {
					t.Fatalf("bEnableScrollAnimation: got %#v want true", enableScrollAnimation)
				}
				allowOverscroll, err := decodeExportRootPropertyValue(asset, exportIdx, "AllowOverscroll")
				if err != nil {
					t.Fatalf("decode AllowOverscroll: %v", err)
				}
				if got, ok := widgetReadBoolValue(allowOverscroll); !ok || got {
					t.Fatalf("AllowOverscroll: got %#v want false", allowOverscroll)
				}
				enableRightClickScrolling, err := decodeExportRootPropertyValue(asset, exportIdx, "bEnableRightClickScrolling")
				if err != nil {
					t.Fatalf("decode bEnableRightClickScrolling: %v", err)
				}
				if got, ok := widgetReadBoolValue(enableRightClickScrolling); !ok || got {
					t.Fatalf("bEnableRightClickScrolling: got %#v want false", enableRightClickScrolling)
				}
				enableTouchScrolling, err := decodeExportRootPropertyValue(asset, exportIdx, "bEnableTouchScrolling")
				if err != nil {
					t.Fatalf("decode bEnableTouchScrolling: %v", err)
				}
				if got, ok := widgetReadBoolValue(enableTouchScrolling); !ok || got {
					t.Fatalf("bEnableTouchScrolling: got %#v want false", enableTouchScrolling)
				}
				isPointerScrollingEnabled, err := decodeExportRootPropertyValue(asset, exportIdx, "bIsPointerScrollingEnabled")
				if err != nil {
					t.Fatalf("decode bIsPointerScrollingEnabled: %v", err)
				}
				if got, ok := widgetReadBoolValue(isPointerScrollingEnabled); !ok || got {
					t.Fatalf("bIsPointerScrollingEnabled: got %#v want false", isPointerScrollingEnabled)
				}
				isGamepadScrollingEnabled, err := decodeExportRootPropertyValue(asset, exportIdx, "bIsGamepadScrollingEnabled")
				if err != nil {
					t.Fatalf("decode bIsGamepadScrollingEnabled: %v", err)
				}
				if got, ok := widgetReadBoolValue(isGamepadScrollingEnabled); !ok || got {
					t.Fatalf("bIsGamepadScrollingEnabled: got %#v want false", isGamepadScrollingEnabled)
				}
				horizontalEntrySpacing, err := decodeExportRootPropertyValue(asset, exportIdx, "HorizontalEntrySpacing")
				if err != nil {
					t.Fatalf("decode HorizontalEntrySpacing: %v", err)
				}
				if got, want := widgetDecodedScalarFloat(horizontalEntrySpacing), float32(12); got != want {
					t.Fatalf("HorizontalEntrySpacing: got %v want %v", got, want)
				}
				verticalEntrySpacing, err := decodeExportRootPropertyValue(asset, exportIdx, "VerticalEntrySpacing")
				if err != nil {
					t.Fatalf("decode VerticalEntrySpacing: %v", err)
				}
				if got, want := widgetDecodedScalarFloat(verticalEntrySpacing), float32(6); got != want {
					t.Fatalf("VerticalEntrySpacing: got %v want %v", got, want)
				}
				scrollBarPadding, err := decodeExportRootPropertyValue(asset, exportIdx, "ScrollBarPadding")
				if err != nil {
					t.Fatalf("decode ScrollBarPadding: %v", err)
				}
				paddingSummary, ok := widgetReadMarginSummary(scrollBarPadding)
				if !ok {
					t.Fatalf("ScrollBarPadding summary missing")
				}
				requireWidgetReadFloatMap(t, paddingSummary, map[string]float32{"left": 1, "top": 2, "right": 3, "bottom": 4})
			},
			summaryKey: "listView",
		},
		{
			name:      "TileView",
			childType: "tileview",
			childName: "TileView_1",
			writes: [][]string{
				{"blueprint", "widget-write", "--property", "listview-entry-widget-class", "--value", "/Game/WBP/WBP_ListEntry_Text"},
				{"blueprint", "widget-write", "--property", "listview-selection-mode", "--value", "SingleToggle"},
				{"blueprint", "widget-write", "--property", "tileview-entry-width", "--value", "180"},
				{"blueprint", "widget-write", "--property", "tileview-entry-height", "--value", "96"},
				{"blueprint", "widget-write", "--property", "tileview-scrollbar-disabled-visibility", "--value", "Hidden"},
				{"blueprint", "widget-write", "--property", "tileview-entry-size-includes-entry-spacing", "--value", "false"},
			},
			check: func(t *testing.T, asset *uasset.Asset, exportIdx int) {
				t.Helper()
				entryWidgetClass, err := decodeExportRootPropertyValue(asset, exportIdx, "EntryWidgetClass")
				if err != nil {
					t.Fatalf("decode EntryWidgetClass: %v", err)
				}
				entrySummary, ok := widgetReadObjectRefSummary(asset, entryWidgetClass)
				if !ok {
					t.Fatalf("EntryWidgetClass summary missing")
				}
				if got, want := entrySummary["path"], "/Game/WBP/WBP_ListEntry_Text"; got != want {
					t.Fatalf("EntryWidgetClass path: got %#v want %q", got, want)
				}
				selectionMode, err := decodeExportRootPropertyValue(asset, exportIdx, "SelectionMode")
				if err != nil {
					t.Fatalf("decode SelectionMode: %v", err)
				}
				if got, ok := widgetReadEnumValue(selectionMode); !ok || got != "ESelectionMode::SingleToggle" {
					t.Fatalf("SelectionMode: got %#v want %q", got, "ESelectionMode::SingleToggle")
				}
				entryWidth, err := decodeExportRootPropertyValue(asset, exportIdx, "EntryWidth")
				if err != nil {
					t.Fatalf("decode EntryWidth: %v", err)
				}
				if got, want := widgetDecodedScalarFloat(entryWidth), float32(180); got != want {
					t.Fatalf("EntryWidth: got %v want %v", got, want)
				}
				entryHeight, err := decodeExportRootPropertyValue(asset, exportIdx, "EntryHeight")
				if err != nil {
					t.Fatalf("decode EntryHeight: %v", err)
				}
				if got, want := widgetDecodedScalarFloat(entryHeight), float32(96); got != want {
					t.Fatalf("EntryHeight: got %v want %v", got, want)
				}
				scrollbarDisabledVisibility, err := decodeExportRootPropertyValue(asset, exportIdx, "ScrollbarDisabledVisibility")
				if err != nil {
					t.Fatalf("decode ScrollbarDisabledVisibility: %v", err)
				}
				if got, ok := widgetReadEnumValue(scrollbarDisabledVisibility); !ok || got != "ESlateVisibility::Hidden" {
					t.Fatalf("ScrollbarDisabledVisibility: got %#v want %q", got, "ESlateVisibility::Hidden")
				}
				entrySizeIncludesEntrySpacing, err := decodeExportRootPropertyValue(asset, exportIdx, "bEntrySizeIncludesEntrySpacing")
				if err != nil {
					t.Fatalf("decode bEntrySizeIncludesEntrySpacing: %v", err)
				}
				if got, ok := widgetReadBoolValue(entrySizeIncludesEntrySpacing); !ok || got {
					t.Fatalf("bEntrySizeIncludesEntrySpacing: got %#v want false", entrySizeIncludesEntrySpacing)
				}
			},
			summaryKey: "tileView",
		},
		{
			name:      "TreeView",
			childType: "treeview",
			childName: "TreeView_1",
			writes: [][]string{
				{"blueprint", "widget-write", "--property", "listview-entry-widget-class", "--value", "/Game/WBP/WBP_ListEntry_Text"},
				{"blueprint", "widget-write", "--property", "listview-selection-mode", "--value", "Multi"},
				{"blueprint", "widget-write", "--property", "listview-consume-mouse-wheel", "--value", "Always"},
			},
			check: func(t *testing.T, asset *uasset.Asset, exportIdx int) {
				t.Helper()
				entryWidgetClass, err := decodeExportRootPropertyValue(asset, exportIdx, "EntryWidgetClass")
				if err != nil {
					t.Fatalf("decode EntryWidgetClass: %v", err)
				}
				entrySummary, ok := widgetReadObjectRefSummary(asset, entryWidgetClass)
				if !ok {
					t.Fatalf("EntryWidgetClass summary missing")
				}
				if got, want := entrySummary["path"], "/Game/WBP/WBP_ListEntry_Text"; got != want {
					t.Fatalf("EntryWidgetClass path: got %#v want %q", got, want)
				}
				selectionMode, err := decodeExportRootPropertyValue(asset, exportIdx, "SelectionMode")
				if err != nil {
					t.Fatalf("decode SelectionMode: %v", err)
				}
				if got, ok := widgetReadEnumValue(selectionMode); !ok || got != "ESelectionMode::Multi" {
					t.Fatalf("SelectionMode: got %#v want %q", got, "ESelectionMode::Multi")
				}
				consumeMouseWheel, err := decodeExportRootPropertyValue(asset, exportIdx, "ConsumeMouseWheel")
				if err != nil {
					t.Fatalf("decode ConsumeMouseWheel: %v", err)
				}
				if got, ok := widgetReadEnumValue(consumeMouseWheel); !ok || got != "EConsumeMouseWheel::Always" {
					t.Fatalf("ConsumeMouseWheel: got %#v want %q", got, "EConsumeMouseWheel::Always")
				}
			},
			summaryKey: "listView",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			work, childPath := buildWidgetTestAsset(t, "canvaspanel", "CanvasPanel_21", tt.childType, tt.childName)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			for _, argv := range tt.writes {
				fullArgv := append([]string(nil), argv...)
				fullArgv = append(fullArgv[:2], append([]string{work, "--widget", childPath}, fullArgv[2:]...)...)
				stdout.Reset()
				stderr.Reset()
				if code := Run(fullArgv, &stdout, &stderr); code != 0 {
					t.Fatalf("widget-write exit code: got %d want 0 argv=%v stderr=%s", code, fullArgv, stderr.String())
				}
			}

			asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("parse final asset: %v", err)
			}
			targets, err := collectWidgetWriteTargets(asset, 0)
			if err != nil {
				t.Fatalf("collectWidgetWriteTargets final: %v", err)
			}
			target, err := selectWidgetWriteTarget(targets, childPath)
			if err != nil {
				t.Fatalf("select final widget: %v", err)
			}
			for _, exportIdx := range target.Exports {
				tt.check(t, asset, exportIdx)
			}

			blueprints := findWidgetBlueprintExports(asset)
			if len(blueprints) != 1 {
				t.Fatalf("widget blueprint exports: got %d want 1", len(blueprints))
			}
			entry, err := buildWidgetBlueprintEntry(asset, blueprints[0])
			if err != nil {
				t.Fatalf("buildWidgetBlueprintEntry: %v", err)
			}
			logical := requireWidgetReadLogicalWidgetByPath(t, entry, childPath)
			if _, ok := logical[tt.summaryKey].(map[string]any); !ok {
				t.Fatalf("%s summary type: got %#v", tt.summaryKey, logical[tt.summaryKey])
			}
			listViewSummary, ok := logical["listView"].(map[string]any)
			if !ok {
				t.Fatalf("listView summary type: got %#v", logical["listView"])
			}
			if got, want := listViewSummary["entryWidgetClassPath"], "/Game/WBP/WBP_ListEntry_Text"; got != want {
				t.Fatalf("listView entryWidgetClassPath: got %#v want %q", got, want)
			}
			switch tt.name {
			case "ListView":
				if got, want := listViewSummary["scrollIntoViewAlignment"], "EScrollIntoViewAlignment::BottomOrRight"; got != want {
					t.Fatalf("listView scrollIntoViewAlignment: got %#v want %q", got, want)
				}
				if got, want := listViewSummary["allowOverscroll"], false; got != want {
					t.Fatalf("listView allowOverscroll: got %#v want %v", got, want)
				}
			case "TileView":
				tileViewSummary, ok := logical["tileView"].(map[string]any)
				if !ok {
					t.Fatalf("tileView summary type: got %#v", logical["tileView"])
				}
				if got, want := tileViewSummary["scrollbarDisabledVisibility"], "ESlateVisibility::Hidden"; got != want {
					t.Fatalf("tileView scrollbarDisabledVisibility: got %#v want %q", got, want)
				}
				if got, want := tileViewSummary["entrySizeIncludesEntrySpacing"], false; got != want {
					t.Fatalf("tileView entrySizeIncludesEntrySpacing: got %#v want %v", got, want)
				}
			}
		})
	}
}

func TestRunBlueprintWidgetWriteListViewEntryWidgetClassRejectsInvalidPath(t *testing.T) {
	work, childPath := buildWidgetTestAsset(t, "canvaspanel", "CanvasPanel_21", "listview", "ListView_1")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-write", work,
		"--widget", childPath,
		"--property", "listview-entry-widget-class",
		"--value", "WBP_ListEntry_Text",
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("widget-write exit code: got 0 want non-zero")
	}
	if !strings.Contains(stderr.String(), "requires a WidgetBlueprint asset path") {
		t.Fatalf("stderr: got %q want invalid asset path guidance", stderr.String())
	}
}

func TestRunBlueprintWidgetWriteScaleBoxWrapBoxAndWidgetSwitcherProperties(t *testing.T) {
	tests := []struct {
		name      string
		childType string
		childName string
		writes    [][]string
		check     func(t *testing.T, asset *uasset.Asset, exportIdx int)
	}{
		{
			name:      "ScaleBox",
			childType: "scalebox",
			childName: "ScaleBox_1",
			writes: [][]string{
				{"blueprint", "widget-write", "--property", "scalebox-stretch", "--value", "ScaleToFit"},
				{"blueprint", "widget-write", "--property", "scalebox-stretch-direction", "--value", "DownOnly"},
				{"blueprint", "widget-write", "--property", "scalebox-user-specified-scale", "--value", "1.25"},
				{"blueprint", "widget-write", "--property", "scalebox-ignore-inherited-scale", "--value", "true"},
			},
			check: func(t *testing.T, asset *uasset.Asset, exportIdx int) {
				t.Helper()
				if _, found, err := decodeExportRootPropertyValueIfPresent(asset, exportIdx, "Stretch"); err != nil {
					t.Fatalf("decode Stretch presence: %v", err)
				} else if found {
					t.Fatalf("Stretch should remain implicit default for ScaleToFit")
				}
				stretchDirection, err := decodeExportRootPropertyValue(asset, exportIdx, "StretchDirection")
				if err != nil {
					t.Fatalf("decode StretchDirection: %v", err)
				}
				if got, ok := widgetReadEnumValue(stretchDirection); !ok || got != "EStretchDirection::DownOnly" {
					t.Fatalf("StretchDirection: got %#v want %q", got, "EStretchDirection::DownOnly")
				}
				userSpecifiedScale, err := decodeExportRootPropertyValue(asset, exportIdx, "UserSpecifiedScale")
				if err != nil {
					t.Fatalf("decode UserSpecifiedScale: %v", err)
				}
				if got, want := widgetDecodedScalarFloat(userSpecifiedScale), float32(1.25); got != want {
					t.Fatalf("UserSpecifiedScale: got %v want %v", got, want)
				}
				ignoreInheritedScale, err := decodeExportRootPropertyValue(asset, exportIdx, "IgnoreInheritedScale")
				if err != nil {
					t.Fatalf("decode IgnoreInheritedScale: %v", err)
				}
				if got, ok := widgetReadBoolValue(ignoreInheritedScale); !ok || !got {
					t.Fatalf("IgnoreInheritedScale: got %#v want true", ignoreInheritedScale)
				}
			},
		},
		{
			name:      "WrapBox",
			childType: "wrapbox",
			childName: "WrapBox_1",
			writes: [][]string{
				{"blueprint", "widget-write", "--property", "wrapbox-wrap-size", "--value", "480"},
				{"blueprint", "widget-write", "--property", "wrapbox-explicit-wrap-size", "--value", "true"},
				{"blueprint", "widget-write", "--property", "wrapbox-inner-slot-padding", "--value", "8,12"},
				{"blueprint", "widget-write", "--property", "wrapbox-orientation", "--value", "Vertical"},
			},
			check: func(t *testing.T, asset *uasset.Asset, exportIdx int) {
				t.Helper()
				wrapSize, err := decodeExportRootPropertyValue(asset, exportIdx, "WrapSize")
				if err != nil {
					t.Fatalf("decode WrapSize: %v", err)
				}
				if got, want := widgetDecodedScalarFloat(wrapSize), float32(480); got != want {
					t.Fatalf("WrapSize: got %v want %v", got, want)
				}
				explicitWrapSize, err := decodeExportRootPropertyValue(asset, exportIdx, "bExplicitWrapSize")
				if err != nil {
					t.Fatalf("decode bExplicitWrapSize: %v", err)
				}
				if got, ok := widgetReadBoolValue(explicitWrapSize); !ok || !got {
					t.Fatalf("bExplicitWrapSize: got %#v want true", explicitWrapSize)
				}
				innerSlotPadding, err := decodeExportRootPropertyValue(asset, exportIdx, "InnerSlotPadding")
				if err != nil {
					t.Fatalf("decode InnerSlotPadding: %v", err)
				}
				paddingSummary, ok := widgetReadVector2Summary(innerSlotPadding)
				if !ok {
					t.Fatalf("InnerSlotPadding summary missing")
				}
				requireWidgetReadFloatMap(t, paddingSummary, map[string]float32{"x": 8, "y": 12})
				orientation, err := decodeExportRootPropertyValue(asset, exportIdx, "Orientation")
				if err != nil {
					t.Fatalf("decode Orientation: %v", err)
				}
				if got, ok := widgetReadEnumValue(orientation); !ok || got != "EOrientation::Orient_Vertical" {
					t.Fatalf("Orientation: got %#v want %q", got, "EOrientation::Orient_Vertical")
				}
			},
		},
		{
			name:      "WidgetSwitcher",
			childType: "widgetswitcher",
			childName: "WidgetSwitcher_1",
			writes: [][]string{
				{"blueprint", "widget-write", "--property", "widgetswitcher-active-widget-index", "--value", "2"},
			},
			check: func(t *testing.T, asset *uasset.Asset, exportIdx int) {
				t.Helper()
				activeWidgetIndex, err := decodeExportRootPropertyValue(asset, exportIdx, "ActiveWidgetIndex")
				if err != nil {
					t.Fatalf("decode ActiveWidgetIndex: %v", err)
				}
				if got, ok := widgetReadIntValue(activeWidgetIndex); !ok || got != 2 {
					t.Fatalf("ActiveWidgetIndex: got %#v want %v", activeWidgetIndex, 2)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			work, childPath := buildWidgetTestAsset(t, "canvaspanel", "CanvasPanel_21", tc.childType, tc.childName)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			for _, argv := range tc.writes {
				fullArgv := append([]string(nil), argv...)
				fullArgv = append(fullArgv[:2], append([]string{work, "--widget", childPath}, fullArgv[2:]...)...)
				stdout.Reset()
				stderr.Reset()
				if code := Run(fullArgv, &stdout, &stderr); code != 0 {
					t.Fatalf("widget-write exit code: got %d want 0 argv=%v stderr=%s", code, fullArgv, stderr.String())
				}
			}

			asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("parse final asset: %v", err)
			}
			targets, err := collectWidgetWriteTargets(asset, 0)
			if err != nil {
				t.Fatalf("collectWidgetWriteTargets final: %v", err)
			}
			target, err := selectWidgetWriteTarget(targets, childPath)
			if err != nil {
				t.Fatalf("select final widget: %v", err)
			}
			for _, exportIdx := range target.Exports {
				tc.check(t, asset, exportIdx)
			}

			blueprints := findWidgetBlueprintExports(asset)
			if len(blueprints) != 1 {
				t.Fatalf("widget blueprint exports: got %d want 1", len(blueprints))
			}
			entry, err := buildWidgetBlueprintEntry(asset, blueprints[0])
			if err != nil {
				t.Fatalf("buildWidgetBlueprintEntry: %v", err)
			}
			logical := requireWidgetReadLogicalWidgetByPath(t, entry, childPath)
			switch tc.name {
			case "ScaleBox":
				summary, ok := logical["scaleBox"].(map[string]any)
				if !ok {
					t.Fatalf("scaleBox summary type: got %#v", logical["scaleBox"])
				}
				if _, ok := summary["stretch"]; ok {
					t.Fatalf("scaleBox.stretch should remain implicit default")
				}
				if got, want := summary["stretchDirection"], "EStretchDirection::DownOnly"; got != want {
					t.Fatalf("scaleBox.stretchDirection: got %#v want %q", got, want)
				}
			case "WrapBox":
				summary, ok := logical["wrapBox"].(map[string]any)
				if !ok {
					t.Fatalf("wrapBox summary type: got %#v", logical["wrapBox"])
				}
				if got, want := summary["wrapSize"], float32(480); got != want {
					t.Fatalf("wrapBox.wrapSize: got %#v want %v", got, want)
				}
			case "WidgetSwitcher":
				summary, ok := logical["widgetSwitcher"].(map[string]any)
				if !ok {
					t.Fatalf("widgetSwitcher summary type: got %#v", logical["widgetSwitcher"])
				}
				if got, want := summary["activeWidgetIndex"], 2; got != want {
					t.Fatalf("widgetSwitcher.activeWidgetIndex: got %#v want %v", got, want)
				}
			}
		})
	}
}

func TestRunBlueprintWidgetWriteRetainerBlurSafeZoneInvalidationAndUniformGridProperties(t *testing.T) {
	tests := []struct {
		name      string
		childType string
		childName string
		writes    [][]string
		check     func(t *testing.T, asset *uasset.Asset, exportIdx int)
		logical   func(t *testing.T, logical map[string]any)
	}{
		{
			name:      "RetainerBox",
			childType: "retainerbox",
			childName: "RetainerBox_1",
			writes: [][]string{
				{"blueprint", "widget-write", "--property", "retainerbox-retain-render", "--value", "false"},
				{"blueprint", "widget-write", "--property", "retainerbox-render-on-invalidation", "--value", "true"},
				{"blueprint", "widget-write", "--property", "retainerbox-render-on-phase", "--value", "false"},
				{"blueprint", "widget-write", "--property", "retainerbox-phase", "--value", "1"},
				{"blueprint", "widget-write", "--property", "retainerbox-phase-count", "--value", "3"},
			},
			check: func(t *testing.T, asset *uasset.Asset, exportIdx int) {
				t.Helper()
				for _, tt := range []struct {
					path string
					want any
				}{
					{"bRetainRender", false},
					{"RenderOnInvalidation", true},
					{"RenderOnPhase", false},
					{"Phase", 1},
					{"PhaseCount", 3},
				} {
					value, err := decodeExportRootPropertyValue(asset, exportIdx, tt.path)
					if err != nil {
						t.Fatalf("decode %s: %v", tt.path, err)
					}
					switch want := tt.want.(type) {
					case bool:
						if got, ok := widgetReadBoolValue(value); !ok || got != want {
							t.Fatalf("%s: got %#v want %v", tt.path, value, want)
						}
					case int:
						if got, ok := widgetReadIntValue(value); !ok || got != want {
							t.Fatalf("%s: got %#v want %v", tt.path, value, want)
						}
					}
				}
			},
			logical: func(t *testing.T, logical map[string]any) {
				summary, ok := logical["retainerBox"].(map[string]any)
				if !ok {
					t.Fatalf("retainerBox summary type: got %#v", logical["retainerBox"])
				}
				if got, want := summary["phaseCount"], 3; got != want {
					t.Fatalf("retainerBox.phaseCount: got %#v want %v", got, want)
				}
			},
		},
		{
			name:      "BackgroundBlur",
			childType: "backgroundblur",
			childName: "BackgroundBlur_1",
			writes: [][]string{
				{"blueprint", "widget-write", "--property", "backgroundblur-strength", "--value", "16"},
				{"blueprint", "widget-write", "--property", "backgroundblur-apply-alpha-to-blur", "--value", "false"},
			},
			check: func(t *testing.T, asset *uasset.Asset, exportIdx int) {
				t.Helper()
				blurStrength, err := decodeExportRootPropertyValue(asset, exportIdx, "BlurStrength")
				if err != nil {
					t.Fatalf("decode BlurStrength: %v", err)
				}
				if got, want := widgetDecodedScalarFloat(blurStrength), float32(16); got != want {
					t.Fatalf("BlurStrength: got %v want %v", got, want)
				}
				applyAlpha, err := decodeExportRootPropertyValue(asset, exportIdx, "bApplyAlphaToBlur")
				if err != nil {
					t.Fatalf("decode bApplyAlphaToBlur: %v", err)
				}
				if got, ok := widgetReadBoolValue(applyAlpha); !ok || got {
					t.Fatalf("bApplyAlphaToBlur: got %#v want false", applyAlpha)
				}
			},
			logical: func(t *testing.T, logical map[string]any) {
				summary, ok := logical["backgroundBlur"].(map[string]any)
				if !ok {
					t.Fatalf("backgroundBlur summary type: got %#v", logical["backgroundBlur"])
				}
				if got, want := summary["blurStrength"], float32(16); got != want {
					t.Fatalf("backgroundBlur.blurStrength: got %#v want %v", got, want)
				}
			},
		},
		{
			name:      "SafeZone",
			childType: "safezone",
			childName: "SafeZone_1",
			writes: [][]string{
				{"blueprint", "widget-write", "--property", "safezone-pad-left", "--value", "false"},
				{"blueprint", "widget-write", "--property", "safezone-pad-right", "--value", "false"},
				{"blueprint", "widget-write", "--property", "safezone-pad-top", "--value", "false"},
				{"blueprint", "widget-write", "--property", "safezone-pad-bottom", "--value", "false"},
			},
			check: func(t *testing.T, asset *uasset.Asset, exportIdx int) {
				t.Helper()
				for _, tt := range []struct {
					path string
					want bool
				}{
					{"PadLeft", false},
					{"PadRight", false},
					{"PadTop", false},
					{"PadBottom", false},
				} {
					value, err := decodeExportRootPropertyValue(asset, exportIdx, tt.path)
					if err != nil {
						t.Fatalf("decode %s: %v", tt.path, err)
					}
					if got, ok := widgetReadBoolValue(value); !ok || got != tt.want {
						t.Fatalf("%s: got %#v want %v", tt.path, value, tt.want)
					}
				}
			},
			logical: func(t *testing.T, logical map[string]any) {
				summary, ok := logical["safeZone"].(map[string]any)
				if !ok {
					t.Fatalf("safeZone summary type: got %#v", logical["safeZone"])
				}
				if got, want := summary["padLeft"], false; got != want {
					t.Fatalf("safeZone.padLeft: got %#v want %v", got, want)
				}
			},
		},
		{
			name:      "InvalidationBox",
			childType: "invalidationbox",
			childName: "InvalidationBox_1",
			writes: [][]string{
				{"blueprint", "widget-write", "--property", "invalidationbox-can-cache", "--value", "false"},
			},
			check: func(t *testing.T, asset *uasset.Asset, exportIdx int) {
				t.Helper()
				value, err := decodeExportRootPropertyValue(asset, exportIdx, "bCanCache")
				if err != nil {
					t.Fatalf("decode bCanCache: %v", err)
				}
				if got, ok := widgetReadBoolValue(value); !ok || got {
					t.Fatalf("bCanCache: got %#v want false", value)
				}
			},
			logical: func(t *testing.T, logical map[string]any) {
				summary, ok := logical["invalidationBox"].(map[string]any)
				if !ok {
					t.Fatalf("invalidationBox summary type: got %#v", logical["invalidationBox"])
				}
				if got, want := summary["canCache"], false; got != want {
					t.Fatalf("invalidationBox.canCache: got %#v want %v", got, want)
				}
			},
		},
		{
			name:      "UniformGridPanel",
			childType: "uniformgridpanel",
			childName: "UniformGridPanel_1",
			writes: [][]string{
				{"blueprint", "widget-write", "--property", "uniformgridpanel-min-desired-slot-width", "--value", "160"},
				{"blueprint", "widget-write", "--property", "uniformgridpanel-min-desired-slot-height", "--value", "48"},
				{"blueprint", "widget-write", "--property", "uniformgridpanel-slot-padding", "--value", "4,6,8,10"},
			},
			check: func(t *testing.T, asset *uasset.Asset, exportIdx int) {
				t.Helper()
				width, err := decodeExportRootPropertyValue(asset, exportIdx, "MinDesiredSlotWidth")
				if err != nil {
					t.Fatalf("decode MinDesiredSlotWidth: %v", err)
				}
				if got, want := widgetDecodedScalarFloat(width), float32(160); got != want {
					t.Fatalf("MinDesiredSlotWidth: got %v want %v", got, want)
				}
				height, err := decodeExportRootPropertyValue(asset, exportIdx, "MinDesiredSlotHeight")
				if err != nil {
					t.Fatalf("decode MinDesiredSlotHeight: %v", err)
				}
				if got, want := widgetDecodedScalarFloat(height), float32(48); got != want {
					t.Fatalf("MinDesiredSlotHeight: got %v want %v", got, want)
				}
				padding, err := decodeExportRootPropertyValue(asset, exportIdx, "SlotPadding")
				if err != nil {
					t.Fatalf("decode SlotPadding: %v", err)
				}
				summary, ok := widgetReadMarginSummary(padding)
				if !ok {
					t.Fatalf("SlotPadding summary missing")
				}
				requireWidgetReadFloatMap(t, summary, map[string]float32{"left": 4, "top": 6, "right": 8, "bottom": 10})
			},
			logical: func(t *testing.T, logical map[string]any) {
				summary, ok := logical["uniformGridPanel"].(map[string]any)
				if !ok {
					t.Fatalf("uniformGridPanel summary type: got %#v", logical["uniformGridPanel"])
				}
				if got, want := summary["minDesiredSlotWidth"], float32(160); got != want {
					t.Fatalf("uniformGridPanel.minDesiredSlotWidth: got %#v want %v", got, want)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			work, childPath := buildWidgetTestAsset(t, "canvaspanel", "CanvasPanel_21", tc.childType, tc.childName)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			for _, argv := range tc.writes {
				fullArgv := append([]string(nil), argv...)
				fullArgv = append(fullArgv[:2], append([]string{work, "--widget", childPath}, fullArgv[2:]...)...)
				stdout.Reset()
				stderr.Reset()
				if code := Run(fullArgv, &stdout, &stderr); code != 0 {
					t.Fatalf("widget-write exit code: got %d want 0 argv=%v stderr=%s", code, fullArgv, stderr.String())
				}
			}

			asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("parse final asset: %v", err)
			}
			targets, err := collectWidgetWriteTargets(asset, 0)
			if err != nil {
				t.Fatalf("collectWidgetWriteTargets final: %v", err)
			}
			target, err := selectWidgetWriteTarget(targets, childPath)
			if err != nil {
				t.Fatalf("select final widget: %v", err)
			}
			for _, exportIdx := range target.Exports {
				tc.check(t, asset, exportIdx)
			}

			blueprints := findWidgetBlueprintExports(asset)
			if len(blueprints) != 1 {
				t.Fatalf("widget blueprint exports: got %d want 1", len(blueprints))
			}
			entry, err := buildWidgetBlueprintEntry(asset, blueprints[0])
			if err != nil {
				t.Fatalf("buildWidgetBlueprintEntry: %v", err)
			}
			logical := requireWidgetReadLogicalWidgetByPath(t, entry, childPath)
			tc.logical(t, logical)
		})
	}
}

func TestRunBlueprintWidgetWriteScrollBarVerticalOrientationNoop(t *testing.T) {
	work, childPath := buildWidgetTestAsset(t, "canvaspanel", "CanvasPanel_21", "scrollbar", "ScrollBar_1")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := Run([]string{
		"blueprint", "widget-write", work,
		"--widget", childPath,
		"--property", "scrollbar-orientation",
		"--value", "Vertical",
	}, &stdout, &stderr); code != 0 {
		t.Fatalf("widget-write exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	afterBytes, err := os.ReadFile(work)
	if err != nil {
		t.Fatalf("read after asset: %v", err)
	}
	asset, err := uasset.ParseBytes(afterBytes, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse final asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets final: %v", err)
	}
	target, err := selectWidgetWriteTarget(targets, childPath)
	if err != nil {
		t.Fatalf("select final widget: %v", err)
	}
	for _, exportIdx := range target.Exports {
		if _, found, err := decodeExportRootPropertyValueIfPresent(asset, exportIdx, "Orientation"); err != nil {
			t.Fatalf("decode export %d Orientation: %v", exportIdx+1, err)
		} else if found {
			t.Fatalf("export %d Orientation: expected no serialized override for default vertical orientation", exportIdx+1)
		}
	}
}

func TestRunBlueprintWidgetWriteRichTextStyleSetProperty(t *testing.T) {
	binaryPath := buildBPXBinaryForTest(t)
	work, childPath := buildRichTextWidgetTestAssetWithBinary(t, binaryPath)

	runBPXBinaryCommand(t, binaryPath,
		"blueprint", "widget-write", work,
		"--widget", childPath,
		"--property", "richtext-style-set",
		"--value", "/Game/UI/Settings/SettingsDescriptionStyles",
	)

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse final asset: %v", err)
	}
	if _, found := findDataTableImportByPath(asset, "/Game/UI/Settings/SettingsDescriptionStyles"); !found {
		t.Fatalf("findDataTableImportByPath: expected SettingsDescriptionStyles import")
	}

	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets final: %v", err)
	}
	target, err := selectWidgetWriteTarget(targets, childPath)
	if err != nil {
		t.Fatalf("select final widget: %v", err)
	}
	designerExport := selectBrushDependsExport(asset, *target)
	if designerExport < 0 {
		t.Fatalf("designer export: got %d want >= 0", designerExport)
	}

	value, err := decodeExportRootPropertyValue(asset, designerExport, "TextStyleSet")
	if err != nil {
		t.Fatalf("decode designer TextStyleSet: %v", err)
	}
	summary, ok := widgetReadObjectRefSummary(asset, value)
	if !ok {
		t.Fatalf("widgetReadObjectRefSummary: got false want true")
	}
	if got, want := summary["path"], "/Game/UI/Settings/SettingsDescriptionStyles"; got != want {
		t.Fatalf("designer TextStyleSet path: got %#v want %q", got, want)
	}

	for _, exportIdx := range target.Exports {
		if exportIdx == designerExport {
			continue
		}
		if _, err := decodeExportRootPropertyValue(asset, exportIdx, "TextStyleSet"); err == nil {
			t.Fatalf("generated export %d unexpectedly has TextStyleSet", exportIdx+1)
		}
	}
}

func TestRunBlueprintWidgetWriteRichTextDecoratorClassesProperty(t *testing.T) {
	binaryPath := buildBPXBinaryForTest(t)
	work, childPath := buildRichTextWidgetTestAssetWithBinary(t, binaryPath)

	runBPXBinaryCommand(t, binaryPath,
		"blueprint", "widget-write", work,
		"--widget", childPath,
		"--property", "richtext-decorator-classes",
		"--value", "/Game/UI/Settings/NewRichTextBlockDecorator",
	)

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse final asset: %v", err)
	}
	if _, found := findBlueprintGeneratedClassImportByPath(asset, "/Game/UI/Settings/NewRichTextBlockDecorator"); !found {
		t.Fatalf("findBlueprintGeneratedClassImportByPath: expected NewRichTextBlockDecorator import")
	}

	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets final: %v", err)
	}
	target, err := selectWidgetWriteTarget(targets, childPath)
	if err != nil {
		t.Fatalf("select final widget: %v", err)
	}
	designerExport := selectBrushDependsExport(asset, *target)
	if designerExport < 0 {
		t.Fatalf("designer export: got %d want >= 0", designerExport)
	}

	value, err := decodeExportRootPropertyValue(asset, designerExport, "DecoratorClasses")
	if err != nil {
		t.Fatalf("decode designer DecoratorClasses: %v", err)
	}
	summaries, ok := widgetReadObjectRefArraySummary(asset, value)
	if !ok || len(summaries) != 1 {
		t.Fatalf("widgetReadObjectRefArraySummary: got %#v want 1 entry", value)
	}
	if got, want := summaries[0]["path"], "/Game/UI/Settings/NewRichTextBlockDecorator"; got != want {
		t.Fatalf("designer DecoratorClasses[0].path: got %#v want %q", got, want)
	}

	for _, exportIdx := range target.Exports {
		if exportIdx == designerExport {
			continue
		}
		if _, err := decodeExportRootPropertyValue(asset, exportIdx, "DecoratorClasses"); err == nil {
			t.Fatalf("generated export %d unexpectedly has DecoratorClasses", exportIdx+1)
		}
	}
}

func TestRunBlueprintWidgetWriteRichTextDecoratorClassesPropertyMultiple(t *testing.T) {
	binaryPath := buildBPXBinaryForTest(t)
	work, childPath := buildRichTextWidgetTestAssetWithBinary(t, binaryPath)

	runBPXBinaryCommand(t, binaryPath,
		"blueprint", "widget-write", work,
		"--widget", childPath,
		"--property", "richtext-decorator-classes",
		"--value", "[\"/Game/UI/Settings/NewRichTextBlockDecorator\",\"/Game/UI/Settings/NewRichTextBlockDecorator1\"]",
	)

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse final asset: %v", err)
	}

	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets final: %v", err)
	}
	target, err := selectWidgetWriteTarget(targets, childPath)
	if err != nil {
		t.Fatalf("select final widget: %v", err)
	}
	designerExport := selectBrushDependsExport(asset, *target)
	if designerExport < 0 {
		t.Fatalf("designer export: got %d want >= 0", designerExport)
	}

	value, err := decodeExportRootPropertyValue(asset, designerExport, "DecoratorClasses")
	if err != nil {
		t.Fatalf("decode designer DecoratorClasses: %v", err)
	}
	summaries, ok := widgetReadObjectRefArraySummary(asset, value)
	if !ok || len(summaries) != 2 {
		t.Fatalf("widgetReadObjectRefArraySummary: got %#v want 2 entries", value)
	}
	gotPaths := []any{summaries[0]["path"], summaries[1]["path"]}
	wantPaths := []any{"/Game/UI/Settings/NewRichTextBlockDecorator", "/Game/UI/Settings/NewRichTextBlockDecorator1"}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("designer DecoratorClasses paths: got %#v want %#v", gotPaths, wantPaths)
	}
}

func TestRunBlueprintWidgetWriteBrushImageAfterRootOverlayAdd(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset")
	work := copyFixtureToTemp(t, path)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "root",
		"--type", "overlay",
		"--name", "Overlay_21",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("root overlay add exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{
		"blueprint", "widget-add", work,
		"--parent", "Overlay_21",
		"--type", "image",
		"--name", "Image_1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("image add exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	const texturePath = "/Game/Characters/Heroes/Mannequin/Textures/Shared/T_UE_Logo_V2"
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{
		"blueprint", "widget-write", work,
		"--widget", "Overlay_21/Image_1",
		"--property", "brush-image",
		"--value", texturePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("brush-image exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	targets, err := collectWidgetWriteTargets(asset, 0)
	if err != nil {
		t.Fatalf("collectWidgetWriteTargets: %v", err)
	}
	child, err := selectWidgetWriteTarget(targets, "Overlay_21/Image_1")
	if err != nil {
		t.Fatalf("select child widget: %v", err)
	}
	if child.ClassName != "Image" {
		t.Fatalf("child class: got %q want %q", child.ClassName, "Image")
	}
	if len(child.Exports) != 2 {
		t.Fatalf("child exports: got %d want 2", len(child.Exports))
	}

	textureImportIdx, found := findTextureImportByPath(asset, texturePath)
	if !found {
		t.Fatalf("texture import not found for %s", texturePath)
	}
	wantResourceIndex := int32(-textureImportIdx)
	for _, exportIdx := range child.Exports {
		if got := brushResourceObjectIndexForTest(t, asset, exportIdx); got != wantResourceIndex {
			t.Fatalf("export %d Brush.ResourceObject index: got %d want %d", exportIdx+1, got, wantResourceIndex)
		}
	}
}

func TestRunBlueprintWidgetWriteBrushImageKeepsGeneratedClassFieldTypesStableAfterSecondOverlayImage(t *testing.T) {
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
		{
			"blueprint", "widget-write", work,
			"--widget", "Overlay_21/Image_1",
			"--property", "brush-image",
			"--value", "/Game/Characters/Heroes/Mannequin/Textures/Shared/T_UE_Logo_V2",
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
	generatedClassExport, ok := findExportIndexByObjectName(asset, "WBP_Minimum_C")
	if !ok {
		t.Fatalf("generated class export not found")
	}
	fieldTypes, err := generatedClassFieldTypeNamesForTest(asset, generatedClassExport)
	if err != nil {
		t.Fatalf("generatedClassFieldTypeNamesForTest: %v", err)
	}
	if len(fieldTypes) < 2 {
		t.Fatalf("generated class field types: got %v want at least 2 entries", fieldTypes)
	}
	if got, want := fieldTypes[:2], []string{"ObjectProperty", "ObjectProperty"}; !slices.Equal(got, want) {
		t.Fatalf("generated class field types: got %v want %v", got, want)
	}
}

func TestRunBlueprintWidgetMinimumOverlayThreeImagesBrushImageEndToEnd(t *testing.T) {
	path := findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset")
	work := copyFixtureToTemp(t, path)

	const texturePath = "/Game/Characters/Heroes/Mannequin/Textures/Shared/T_UE_Logo_V2"

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
		{
			"blueprint", "widget-add", work,
			"--parent", "Overlay_21",
			"--type", "image",
			"--name", "Image_3",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", "Overlay_21/Image_1",
			"--property", "brush-image",
			"--value", texturePath,
		},
		{
			"blueprint", "widget-write", work,
			"--widget", "Overlay_21/Image_2",
			"--property", "brush-image",
			"--value", texturePath,
		},
		{
			"blueprint", "widget-write", work,
			"--widget", "Overlay_21/Image_3",
			"--property", "brush-image",
			"--value", texturePath,
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
	if got, want := len(targets), 4; got != want {
		t.Fatalf("logical widget count: got %d want %d", got, want)
	}

	parent, err := selectWidgetWriteTarget(targets, "Overlay_21")
	if err != nil {
		t.Fatalf("select parent widget: %v", err)
	}
	shape, err := captureWidgetBlueprintShape(asset, parent.BlueprintExport)
	if err != nil {
		t.Fatalf("captureWidgetBlueprintShape: %v", err)
	}
	for _, wantPath := range []string{
		"Overlay_21/Image_1",
		"Overlay_21/Image_2",
		"Overlay_21/Image_3",
	} {
		if !containsStringFold(shape["designer"], wantPath) {
			t.Fatalf("designer shape missing child path %q: %v", wantPath, shape["designer"])
		}
	}

	textureImportIdx, found := findTextureImportByPath(asset, texturePath)
	if !found {
		t.Fatalf("texture import not found for %s", texturePath)
	}
	wantResourceIndex := int32(-textureImportIdx)

	textureImportCount := 0
	for _, imp := range asset.Imports {
		if !textureImportIsSupported(asset, imp) {
			continue
		}
		if strings.EqualFold(resolveImportTargetPath(asset, imp), texturePath) {
			textureImportCount++
		}
	}
	if got, want := textureImportCount, 1; got != want {
		t.Fatalf("texture import count: got %d want %d", got, want)
	}

	for _, widgetPath := range []string{
		"Overlay_21/Image_1",
		"Overlay_21/Image_2",
		"Overlay_21/Image_3",
	} {
		target, err := selectWidgetWriteTarget(targets, widgetPath)
		if err != nil {
			t.Fatalf("select child widget %q: %v", widgetPath, err)
		}
		if target.ClassName != "Image" {
			t.Fatalf("child class for %q: got %q want %q", widgetPath, target.ClassName, "Image")
		}
		if len(target.Exports) == 0 {
			t.Fatalf("child exports missing for %q", widgetPath)
		}
		for _, exportIdx := range target.Exports {
			if got := brushResourceObjectIndexForTest(t, asset, exportIdx); got != wantResourceIndex {
				t.Fatalf("%s export %d Brush.ResourceObject index: got %d want %d", widgetPath, exportIdx+1, got, wantResourceIndex)
			}
		}
	}

	state := widgetBlueprintFixtureStateSignature(t, asset, mustFindSingleWidgetBlueprintExport(t, asset))
	generatedVarNames, ok := state["generatedVarNames"].([]string)
	if !ok {
		t.Fatalf("generatedVarNames missing or invalid: %#v", state["generatedVarNames"])
	}
	if got, want := len(generatedVarNames), 3; got != want {
		t.Fatalf("generated variable name count: got %d want %d (%v)", got, want, generatedVarNames)
	}
	for _, wantName := range []string{"Image_1", "Image_2", "Image_3"} {
		if !slices.Contains(generatedVarNames, wantName) {
			t.Fatalf("generated variable names missing %q: %v", wantName, generatedVarNames)
		}
	}
}

func TestRunBlueprintWidgetWriteBrushImageMatchesFixtureThumbnailPrimaryOffsetAndAssetRegistryBody(t *testing.T) {
	before := findExistingGoldenOperationFixturePath(t, "widget_write_brush_image", "before.uasset")
	after := findExistingGoldenOperationFixturePath(t, "widget_write_brush_image", "after.uasset")
	work := copyFixtureToTemp(t, before)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-write", work,
		"--widget", "Image_22",
		"--property", "brush-image",
		"--value", "/Game/Effects/Textures/Decals/chippedcracks",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("brush-image exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	actualAsset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	expectedAsset, err := uasset.ParseFile(after, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse expected asset: %v", err)
	}

	actualTable, _, _, actualPresent := sectionByOffset(actualAsset, int64(actualAsset.Summary.ThumbnailTableOffset))
	expectedTable, _, _, expectedPresent := sectionByOffset(expectedAsset, int64(expectedAsset.Summary.ThumbnailTableOffset))
	if !actualPresent || !expectedPresent {
		t.Fatalf("thumbnail table missing: actual=%v expected=%v", actualPresent, expectedPresent)
	}
	actualOffsets := thumbnailTableFileOffsetsForTest(t, actualAsset)
	expectedOffsets := thumbnailTableFileOffsetsForTest(t, expectedAsset)
	if len(actualOffsets) == 0 || len(expectedOffsets) == 0 {
		t.Fatalf("thumbnail table offsets missing: actual=%v expected=%v", actualOffsets, expectedOffsets)
	}
	if actualOffsets[0] != expectedOffsets[0] {
		t.Fatalf("thumbnail primary offset mismatch: got %d want %d", actualOffsets[0], expectedOffsets[0])
	}
	if len(actualTable) != len(expectedTable) {
		t.Fatalf("thumbnail table size mismatch: got %d want %d", len(actualTable), len(expectedTable))
	}
	if !bytes.Equal(assetRegistryBodyForTest(t, actualAsset), assetRegistryBodyForTest(t, expectedAsset)) {
		t.Fatalf("asset registry body mismatch after dependency offset field")
	}
}

func TestRunBlueprintWidgetWriteLayoutCanvasPanelSlotMatchesFixtureThumbnailPrimaryOffsetAndAssetRegistryBody(t *testing.T) {
	before := findExistingGoldenOperationFixturePath(t, "widget_write_layout_canvaspanelslot", "before.uasset")
	after := findExistingGoldenOperationFixturePath(t, "widget_write_layout_canvaspanelslot", "after.uasset")
	work := copyFixtureToTemp(t, before)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"blueprint", "widget-write", work,
		"--widget", "CanvasPanel_22/Image_29",
		"--property", "layout-data",
		"--value", "{\"position\":[0,0],\"size\":[200,60],\"anchors\":[0.5,0.5,0.5,0.5],\"alignment\":[0.5,0.5]}",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("layout-data exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	actualAsset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten asset: %v", err)
	}
	expectedAsset, err := uasset.ParseFile(after, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse expected asset: %v", err)
	}

	actualTable, _, _, actualPresent := sectionByOffset(actualAsset, int64(actualAsset.Summary.ThumbnailTableOffset))
	expectedTable, _, _, expectedPresent := sectionByOffset(expectedAsset, int64(expectedAsset.Summary.ThumbnailTableOffset))
	if !actualPresent || !expectedPresent {
		t.Fatalf("thumbnail table missing: actual=%v expected=%v", actualPresent, expectedPresent)
	}
	actualOffsets := thumbnailTableFileOffsetsForTest(t, actualAsset)
	expectedOffsets := thumbnailTableFileOffsetsForTest(t, expectedAsset)
	if len(actualOffsets) == 0 || len(expectedOffsets) == 0 {
		t.Fatalf("thumbnail table offsets missing: actual=%v expected=%v", actualOffsets, expectedOffsets)
	}
	if actualOffsets[0] != expectedOffsets[0] {
		t.Fatalf("thumbnail primary offset mismatch: got %d want %d", actualOffsets[0], expectedOffsets[0])
	}
	if len(actualTable) != len(expectedTable) {
		t.Fatalf("thumbnail table size mismatch: got %d want %d", len(actualTable), len(expectedTable))
	}
	if !bytes.Equal(assetRegistryBodyForTest(t, actualAsset), assetRegistryBodyForTest(t, expectedAsset)) {
		t.Fatalf("asset registry body mismatch after dependency offset field")
	}
}

func TestSyncBlueprintEditorSearchTailOffsetsOperationFixtures(t *testing.T) {
	cases := []struct {
		name string
		run  []string
	}{
		{
			name: "widget_write_brush_image",
			run: []string{
				"blueprint", "widget-write",
				"--widget", "Image_22",
				"--property", "brush-image",
				"--value", "/Game/Effects/Textures/Decals/chippedcracks",
			},
		},
		{
			name: "widget_write_layout_canvaspanelslot",
			run: []string{
				"blueprint", "widget-write",
				"--widget", "CanvasPanel_22/Image_29",
				"--property", "layout-data",
				"--value", "{\"position\":[0,0],\"size\":[200,60],\"anchors\":[0.5,0.5,0.5,0.5],\"alignment\":[0.5,0.5]}",
			},
		},
		{
			name: "widget_write_button_brush_image_size",
			run: []string{
				"blueprint", "widget-write",
				"--widget", "Button_1",
				"--property", "button-normal-image-size",
				"--value", "96,48",
			},
		},
		{
			name: "widget_write_button_brush_tint",
			run: []string{
				"blueprint", "widget-write",
				"--widget", "Button_1",
				"--property", "button-normal-tint",
				"--value", "0.25,0.4,0.9,0.8",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := findExistingGoldenOperationFixturePath(t, tc.name, "before.uasset")
			after := findExistingGoldenOperationFixturePath(t, tc.name, "after.uasset")
			work := copyFixtureToTemp(t, before)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			argv := append([]string{}, tc.run...)
			argv = append(argv[:2], append([]string{work}, argv[2:]...)...)
			code := Run(argv, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("command exit code: got %d want 0 stderr=%s", code, stderr.String())
			}

			actualAsset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("parse rewritten asset: %v", err)
			}
			syncedBytes, syncedAsset, err := syncBlueprintEditorSearchTailOffsets(actualAsset, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("syncBlueprintEditorSearchTailOffsets: %v", err)
			}
			expectedAsset, err := uasset.ParseFile(after, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("parse expected asset: %v", err)
			}

			actualTail := blueprintSearchTailBytesForTest(t, syncedAsset)
			expectedTail := blueprintSearchTailBytesForTest(t, expectedAsset)
			if !bytes.Equal(actualTail, expectedTail) {
				t.Fatalf("tail mismatch after sync: len(got)=%d len(want)=%d", len(actualTail), len(expectedTail))
			}
			if len(syncedBytes) != len(syncedAsset.Raw.Bytes) {
				t.Fatalf("synced bytes len mismatch: got %d want %d", len(syncedBytes), len(syncedAsset.Raw.Bytes))
			}
		})
	}
}

func TestParseBlueprintSearchTailVerboseRecordOperationFixtures(t *testing.T) {
	cases := []struct {
		name      string
		run       []string
		recordPos int
		field1    int32
		field2    int32
	}{
		{
			name: "widget_write_brush_image",
			run: []string{
				"blueprint", "widget-write",
				"--widget", "Image_22",
				"--property", "brush-image",
				"--value", "/Game/Effects/Textures/Decals/chippedcracks",
			},
			recordPos: 0x618e,
			field1:    0x38,
			field2:    0x4c,
		},
		{
			name: "widget_write_layout_canvaspanelslot",
			run: []string{
				"blueprint", "widget-write",
				"--widget", "CanvasPanel_22/Image_29",
				"--property", "layout-data",
				"--value", "{\"position\":[0,0],\"size\":[200,60],\"anchors\":[0.5,0.5,0.5,0.5],\"alignment\":[0.5,0.5]}",
			},
			recordPos: 0x6808,
			field1:    0x3e,
			field2:    0x55,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := findExistingGoldenOperationFixturePath(t, tc.name, "before.uasset")
			work := copyFixtureToTemp(t, before)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			argv := append([]string{}, tc.run...)
			argv = append(argv[:2], append([]string{work}, argv[2:]...)...)
			code := Run(argv, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("command exit code: got %d want 0 stderr=%s", code, stderr.String())
			}

			actualAsset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("parse rewritten asset: %v", err)
			}
			if tc.recordPos < 0 || tc.recordPos >= len(actualAsset.Raw.Bytes) {
				t.Fatalf("record position out of range: 0x%x", tc.recordPos)
			}
			record, ok := parseBlueprintSearchTailVerboseRecord(
				actualAsset.Raw.Bytes[tc.recordPos:],
				actualAsset.Summary.UsesByteSwappedSerialization(),
			)
			if !ok {
				t.Fatalf("parseBlueprintSearchTailVerboseRecord failed at 0x%x", tc.recordPos)
			}
			if record.field1 != tc.field1 || record.field2 != tc.field2 {
				t.Fatalf(
					"parsed fields mismatch: got field1=0x%x field2=0x%x want field1=0x%x field2=0x%x",
					record.field1,
					record.field2,
					tc.field1,
					tc.field2,
				)
			}
		})
	}
}

func generatedClassFieldTypeNamesForTest(asset *uasset.Asset, exportIndex int) ([]string, error) {
	if exportIndex < 0 || exportIndex >= len(asset.Exports) {
		return nil, nil
	}
	exp := asset.Exports[exportIndex]
	start := int(exp.SerialOffset)
	end := int(exp.SerialOffset + exp.SerialSize)
	scriptEnd := int(exp.ScriptSerializationEndOffset)
	if start < 0 || end < start || end > len(asset.Raw.Bytes) || scriptEnd < 0 || scriptEnd > end-start {
		return nil, nil
	}

	payload := asset.Raw.Bytes[start:end]
	tail := payload[scriptEnd:]
	if len(tail) < 16 {
		return nil, nil
	}
	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}

	fieldCount := int(order.Uint32(tail[12:16]))
	offset := 16
	names := make([]string, 0, fieldCount)
	for i := 0; i < fieldCount; i++ {
		if len(tail[offset:]) < 8 {
			return nil, nil
		}
		nameRef := uasset.NameRef{
			Index:  int32(order.Uint32(tail[offset : offset+4])),
			Number: int32(order.Uint32(tail[offset+4 : offset+8])),
		}
		names = append(names, nameRef.Display(asset.Names))
		recordLen, err := generatedClassWidgetVariableFieldRecordLength(tail[offset:], order)
		if err != nil {
			return nil, err
		}
		offset += recordLen
	}
	return names, nil
}

func thumbnailTableFileOffsetsForTest(t *testing.T, asset *uasset.Asset) []int32 {
	t.Helper()
	table, _, _, present := sectionByOffset(asset, int64(asset.Summary.ThumbnailTableOffset))
	if !present {
		return nil
	}
	r := uasset.NewByteReaderWithByteSwapping(table, asset.Summary.UsesByteSwappedSerialization())
	count, err := r.ReadInt32()
	if err != nil {
		t.Fatalf("read thumbnail table count: %v", err)
	}
	if count < 0 {
		t.Fatalf("invalid thumbnail table count: %d", count)
	}

	offsets := make([]int32, 0, count)
	for i := int32(0); i < count; i++ {
		if _, err := r.ReadFString(); err != nil {
			t.Fatalf("read thumbnail class name %d: %v", i, err)
		}
		if _, err := r.ReadFString(); err != nil {
			t.Fatalf("read thumbnail object path %d: %v", i, err)
		}
		value, err := r.ReadInt32()
		if err != nil {
			t.Fatalf("read thumbnail file offset %d: %v", i, err)
		}
		offsets = append(offsets, value)
	}
	return offsets
}

func assetRegistryBodyForTest(t *testing.T, asset *uasset.Asset) []byte {
	t.Helper()
	start := int(asset.Summary.AssetRegistryDataOffset)
	end := int(asset.Summary.PreloadDependencyOffset)
	if start <= 0 || end <= start || end > len(asset.Raw.Bytes) {
		t.Fatalf("asset registry range out of bounds: %d..%d (size=%d)", start, end, len(asset.Raw.Bytes))
	}
	if start+8 > end {
		t.Fatalf("asset registry dependency offset field out of bounds: %d..%d", start, end)
	}
	return append([]byte(nil), asset.Raw.Bytes[start+8:end]...)
}

func blueprintSearchTailBytesForTest(t *testing.T, asset *uasset.Asset) []byte {
	t.Helper()
	start := int(asset.Summary.PreloadDependencyOffset)
	end := int(asset.Summary.BulkDataStartOffset)
	if start <= 0 || end <= start || end > len(asset.Raw.Bytes) {
		t.Fatalf("blueprint search tail range out of bounds: %d..%d (size=%d)", start, end, len(asset.Raw.Bytes))
	}
	return append([]byte(nil), asset.Raw.Bytes[start:end]...)
}
