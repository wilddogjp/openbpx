package cli

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

func TestFindWidgetBlueprintExportsCaseInsensitive(t *testing.T) {
	asset := &uasset.Asset{
		Names: []uasset.NameEntry{
			{Value: "widgetblueprint"},
			{Value: "Blueprint"},
		},
		Imports: []uasset.ImportEntry{
			{ObjectName: uasset.NameRef{Index: 0, Number: 0}},
			{ObjectName: uasset.NameRef{Index: 1, Number: 0}},
		},
		Exports: []uasset.ExportEntry{
			{ClassIndex: uasset.PackageIndex(-1)},
			{ClassIndex: uasset.PackageIndex(-2)},
		},
	}

	got := findWidgetBlueprintExports(asset)
	want := []int{0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findWidgetBlueprintExports: got %v want %v", got, want)
	}
}

func TestNormalizeWidgetExportListPreservesOrderAndAddsRoot(t *testing.T) {
	got := normalizeWidgetExportList(9, []int{11, 11, 13, 0, 13})
	want := []int{9, 11, 13}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeWidgetExportList: got %v want %v", got, want)
	}
}

func TestOrderChildExportsForParentPrefersParentSlotOrder(t *testing.T) {
	slotByExport := map[int]*widgetSlotData{
		21: {exportIndex: 21, contentExport: 13},
		20: {exportIndex: 20, contentExport: 11},
	}

	got := orderChildExportsForParent([]int{21, 20}, slotByExport, []int{11, 13})
	want := []int{13, 11}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("orderChildExportsForParent: got %v want %v", got, want)
	}
}

func TestWidgetTreeRole(t *testing.T) {
	if got := widgetTreeRole(7, 8, 7, "WidgetBlueprint"); got != "designer" {
		t.Fatalf("designer role: got %q want designer", got)
	}
	if got := widgetTreeRole(7, 8, 8, "WidgetBlueprintGeneratedClass"); got != "generated" {
		t.Fatalf("generated role: got %q want generated", got)
	}
	if got := widgetTreeRole(7, 8, 99, "Other"); got != "unknown" {
		t.Fatalf("unknown role: got %q want unknown", got)
	}
}

func TestBuildWidgetBlueprintEntryIncludesWidgetReadTextAndBrushSummaries(t *testing.T) {
	textAsset, err := uasset.ParseFile(findExistingGoldenParseFixturePath(t, "WBP_TextBlock.uasset"), uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse text asset: %v", err)
	}
	textBlueprint := mustFindSingleWidgetBlueprintExport(t, textAsset)
	textEntry, err := buildWidgetBlueprintEntry(textAsset, textBlueprint)
	if err != nil {
		t.Fatalf("buildWidgetBlueprintEntry text: %v", err)
	}
	textLogical := requireWidgetReadLogicalWidgetByPath(t, textEntry, "TextBlock_72")
	textSummary, ok := textLogical["text"].(map[string]any)
	if !ok {
		t.Fatalf("logical text summary type: got %#v", textLogical["text"])
	}
	if got, want := textSummary["sourceString"], "Text Block"; got != want {
		t.Fatalf("logical text sourceString: got %#v want %q", got, want)
	}

	brushAsset, err := uasset.ParseFile(findExistingGoldenOperationFixturePath(t, "widget_write_brush_image", "after.uasset"), uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse brush asset: %v", err)
	}
	brushBlueprint := mustFindSingleWidgetBlueprintExport(t, brushAsset)
	brushEntry, err := buildWidgetBlueprintEntry(brushAsset, brushBlueprint)
	if err != nil {
		t.Fatalf("buildWidgetBlueprintEntry brush: %v", err)
	}
	brushLogical := requireWidgetReadLogicalWidgetByPath(t, brushEntry, "Image_22")
	brushSummary, ok := brushLogical["brush"].(map[string]any)
	if !ok {
		t.Fatalf("logical brush summary type: got %#v", brushLogical["brush"])
	}
	if got, want := brushSummary["resourceObjectPath"], "/Game/Effects/Textures/Decals/chippedcracks"; got != want {
		t.Fatalf("logical brush resourceObjectPath: got %#v want %q", got, want)
	}
}

func TestWidgetReadBasicWidgetSummaries(t *testing.T) {
	progressBar := widgetReadWidgetSummary(nil, "ProgressBar", map[string]any{
		"Percent": map[string]any{"value": float32(0.75)},
		"FillColorAndOpacity": map[string]any{
			"value": map[string]any{"r": 0.2, "g": 0.4, "b": 0.6, "a": 0.8},
		},
	})
	progressSummary, ok := progressBar["progressBar"].(map[string]any)
	if !ok {
		t.Fatalf("progressBar summary type: got %#v", progressBar["progressBar"])
	}
	if got, want := progressSummary["percent"], float32(0.75); got != want {
		t.Fatalf("progressBar.percent: got %#v want %v", got, want)
	}
	requireWidgetReadLinearColor(t, progressSummary["fillColorAndOpacity"], widgetLinearColor{R: 0.2, G: 0.4, B: 0.6, A: 0.8})

	slider := widgetReadWidgetSummary(nil, "Slider", map[string]any{
		"Value":       map[string]any{"value": float32(0.5)},
		"MinValue":    map[string]any{"value": float32(0.1)},
		"MaxValue":    map[string]any{"value": float32(0.9)},
		"StepSize":    map[string]any{"value": float32(0.05)},
		"Orientation": map[string]any{"value": map[string]any{"value": "EOrientation::Orient_Vertical"}},
	})
	sliderSummary, ok := slider["slider"].(map[string]any)
	if !ok {
		t.Fatalf("slider summary type: got %#v", slider["slider"])
	}
	if got, want := sliderSummary["value"], float32(0.5); got != want {
		t.Fatalf("slider.value: got %#v want %v", got, want)
	}
	if got, want := sliderSummary["orientation"], "EOrientation::Orient_Vertical"; got != want {
		t.Fatalf("slider.orientation: got %#v want %q", got, want)
	}

	spacer := widgetReadWidgetSummary(nil, "Spacer", map[string]any{
		"Size": map[string]any{"value": map[string]any{"X": 24.0, "Y": 48.0}},
	})
	spacerSummary, ok := spacer["spacer"].(map[string]any)
	if !ok {
		t.Fatalf("spacer summary type: got %#v", spacer["spacer"])
	}
	requireWidgetReadFloatMap(t, spacerSummary["size"], map[string]float32{"x": 24, "y": 48})

	scrollBar := widgetReadWidgetSummary(nil, "ScrollBar", map[string]any{
		"Thickness":   map[string]any{"value": map[string]any{"X": 5.0, "Y": 12.0}},
		"Orientation": map[string]any{"value": map[string]any{"value": "EOrientation::Orient_Vertical"}},
	})
	scrollBarSummary, ok := scrollBar["scrollBar"].(map[string]any)
	if !ok {
		t.Fatalf("scrollBar summary type: got %#v", scrollBar["scrollBar"])
	}
	requireWidgetReadFloatMap(t, scrollBarSummary["thickness"], map[string]float32{"x": 5, "y": 12})
	if got, want := scrollBarSummary["orientation"], "EOrientation::Orient_Vertical"; got != want {
		t.Fatalf("scrollBar.orientation: got %#v want %q", got, want)
	}

	checkBox := widgetReadWidgetSummary(nil, "CheckBox", map[string]any{
		"CheckedState": map[string]any{"value": map[string]any{"value": "ECheckBoxState::Checked"}},
	})
	checkBoxSummary, ok := checkBox["checkBox"].(map[string]any)
	if !ok {
		t.Fatalf("checkBox summary type: got %#v", checkBox["checkBox"])
	}
	if got, want := checkBoxSummary["checkedState"], "ECheckBoxState::Checked"; got != want {
		t.Fatalf("checkBox.checkedState: got %#v want %q", got, want)
	}

	scrollBox := widgetReadWidgetSummary(nil, "ScrollBox", map[string]any{
		"Orientation":         map[string]any{"value": map[string]any{"value": "EOrientation::Orient_Horizontal"}},
		"ScrollBarVisibility": map[string]any{"value": map[string]any{"value": "ESlateVisibility::Collapsed"}},
		"ConsumeMouseWheel":   map[string]any{"value": map[string]any{"value": "EConsumeMouseWheel::Always"}},
	})
	scrollBoxSummary, ok := scrollBox["scrollBox"].(map[string]any)
	if !ok {
		t.Fatalf("scrollBox summary type: got %#v", scrollBox["scrollBox"])
	}
	if got, want := scrollBoxSummary["orientation"], "EOrientation::Orient_Horizontal"; got != want {
		t.Fatalf("scrollBox.orientation: got %#v want %q", got, want)
	}
	if got, want := scrollBoxSummary["scrollBarVisibility"], "ESlateVisibility::Collapsed"; got != want {
		t.Fatalf("scrollBox.scrollBarVisibility: got %#v want %q", got, want)
	}
	if got, want := scrollBoxSummary["consumeMouseWheel"], "EConsumeMouseWheel::Always"; got != want {
		t.Fatalf("scrollBox.consumeMouseWheel: got %#v want %q", got, want)
	}

	sizeBox := widgetReadWidgetSummary(nil, "SizeBox", map[string]any{
		"WidthOverride":    map[string]any{"value": float32(320)},
		"HeightOverride":   map[string]any{"value": float32(72)},
		"MinDesiredWidth":  map[string]any{"value": float32(120)},
		"MaxDesiredHeight": map[string]any{"value": float32(240)},
		"MinAspectRatio":   map[string]any{"value": float32(1.2)},
		"MaxAspectRatio":   map[string]any{"value": float32(1.8)},
	})
	sizeBoxSummary, ok := sizeBox["sizeBox"].(map[string]any)
	if !ok {
		t.Fatalf("sizeBox summary type: got %#v", sizeBox["sizeBox"])
	}
	if got, want := sizeBoxSummary["widthOverride"], float32(320); got != want {
		t.Fatalf("sizeBox.widthOverride: got %#v want %v", got, want)
	}
	if got, want := sizeBoxSummary["heightOverride"], float32(72); got != want {
		t.Fatalf("sizeBox.heightOverride: got %#v want %v", got, want)
	}
	if got, want := sizeBoxSummary["minDesiredWidth"], float32(120); got != want {
		t.Fatalf("sizeBox.minDesiredWidth: got %#v want %v", got, want)
	}
	if got, want := sizeBoxSummary["maxDesiredHeight"], float32(240); got != want {
		t.Fatalf("sizeBox.maxDesiredHeight: got %#v want %v", got, want)
	}
	if got, want := sizeBoxSummary["minAspectRatio"], float32(1.2); got != want {
		t.Fatalf("sizeBox.minAspectRatio: got %#v want %v", got, want)
	}
	if got, want := sizeBoxSummary["maxAspectRatio"], float32(1.8); got != want {
		t.Fatalf("sizeBox.maxAspectRatio: got %#v want %v", got, want)
	}
}

func TestWidgetReadTextStyleSummaryNormalizesFontAndBrushDetails(t *testing.T) {
	textStyle := widgetReadTextStyleSummary(nil, "TextBlock", map[string]any{
		"Font": map[string]any{
			"value": map[string]any{
				"FontObject": map[string]any{
					"value": map[string]any{
						"index":    -7,
						"resolved": "import:7:Roboto",
						"path":     "/Game/UI/Fonts/Roboto",
					},
				},
				"TypefaceFontName": map[string]any{
					"value": map[string]any{
						"name": "Bold",
					},
				},
				"Size":          map[string]any{"value": 24},
				"LetterSpacing": map[string]any{"value": 120},
				"OutlineSettings": map[string]any{
					"value": map[string]any{
						"OutlineSize": map[string]any{"value": 2},
						"OutlineColor": map[string]any{
							"value": map[string]any{
								"r": 0.1,
								"g": 0.2,
								"b": 0.3,
								"a": 0.9,
							},
						},
					},
				},
			},
		},
		"ColorAndOpacity": map[string]any{
			"value": map[string]any{
				"SpecifiedColor": map[string]any{
					"value": map[string]any{
						"r": 1.0,
						"g": 0.5,
						"b": 0.25,
						"a": 0.75,
					},
				},
				"ColorUseRule": map[string]any{
					"value": map[string]any{
						"value": "ESlateColorStylingMode::UseColor_Specified",
					},
				},
			},
		},
		"Justification": map[string]any{
			"value": map[string]any{
				"value": "ETextJustify::Center",
			},
		},
		"ShadowOffset": map[string]any{
			"value": map[string]any{
				"X": 3.0,
				"Y": 4.0,
			},
		},
		"ShadowColorAndOpacity": map[string]any{
			"value": map[string]any{
				"r": 0.05,
				"g": 0.1,
				"b": 0.15,
				"a": 0.8,
			},
		},
		"StrikeBrush": map[string]any{
			"value": map[string]any{
				"DrawAs": map[string]any{
					"value": map[string]any{
						"value": "ESlateBrushDrawType::Image",
					},
				},
				"ImageSize": map[string]any{
					"value": map[string]any{
						"X": 9.0,
						"Y": 2.0,
					},
				},
				"Margin": map[string]any{
					"value": map[string]any{
						"Left":   1.0,
						"Top":    2.0,
						"Right":  3.0,
						"Bottom": 4.0,
					},
				},
			},
		},
	})
	font, ok := textStyle["font"].(map[string]any)
	if !ok {
		t.Fatalf("font type: got %#v", textStyle["font"])
	}
	if got, want := font["fontObjectPath"], "/Game/UI/Fonts/Roboto"; got != want {
		t.Fatalf("font.fontObjectPath: got %#v want %q", got, want)
	}
	if got, want := font["typefaceFontName"], "Bold"; got != want {
		t.Fatalf("font.typefaceFontName: got %#v want %q", got, want)
	}
	if got, want := font["size"], 24; got != want {
		t.Fatalf("font.size: got %#v want %d", got, want)
	}
	if got, want := font["letterSpacing"], 120; got != want {
		t.Fatalf("font.letterSpacing: got %#v want %d", got, want)
	}
	outline, ok := font["outlineSettings"].(map[string]any)
	if !ok {
		t.Fatalf("font.outlineSettings type: got %#v", font["outlineSettings"])
	}
	if got, want := outline["outlineSize"], 2; got != want {
		t.Fatalf("font.outlineSettings.outlineSize: got %#v want %d", got, want)
	}

	color, ok := textStyle["colorAndOpacity"].(map[string]any)
	if !ok {
		t.Fatalf("colorAndOpacity type: got %#v", textStyle["colorAndOpacity"])
	}
	requireWidgetReadLinearColor(t, color, widgetLinearColor{R: 1, G: 0.5, B: 0.25, A: 0.75})
	if got, want := color["colorUseRule"], "ESlateColorStylingMode::UseColor_Specified"; got != want {
		t.Fatalf("colorAndOpacity.colorUseRule: got %#v want %q", got, want)
	}
	if got, want := textStyle["justification"], "ETextJustify::Center"; got != want {
		t.Fatalf("justification: got %#v want %q", got, want)
	}
	shadowOffset, ok := textStyle["shadowOffset"].(map[string]any)
	if !ok {
		t.Fatalf("shadowOffset type: got %#v", textStyle["shadowOffset"])
	}
	if got, want := shadowOffset["x"], float32(3); got != want {
		t.Fatalf("shadowOffset.x: got %#v want %v", got, want)
	}
	requireWidgetReadLinearColor(t, textStyle["shadowColorAndOpacity"], widgetLinearColor{R: 0.05, G: 0.1, B: 0.15, A: 0.8})

	strikeBrush, ok := textStyle["strikeBrush"].(map[string]any)
	if !ok {
		t.Fatalf("strikeBrush type: got %#v", textStyle["strikeBrush"])
	}
	if got, want := strikeBrush["drawAs"], "ESlateBrushDrawType::Image"; got != want {
		t.Fatalf("strikeBrush.drawAs: got %#v want %q", got, want)
	}
	margin, ok := strikeBrush["margin"].(map[string]any)
	if !ok {
		t.Fatalf("strikeBrush.margin type: got %#v", strikeBrush["margin"])
	}
	if got, want := margin["bottom"], float32(4); got != want {
		t.Fatalf("strikeBrush.margin.bottom: got %#v want %v", got, want)
	}
}

func TestWidgetReadTextStyleSummarySkipsRichTextBlock(t *testing.T) {
	textStyle := widgetReadTextStyleSummary(nil, "RichTextBlock", map[string]any{
		"Font": map[string]any{"value": map[string]any{"Size": map[string]any{"value": 24}}},
	})
	if textStyle != nil {
		t.Fatalf("textStyle: got %#v want nil", textStyle)
	}
}

func TestWidgetReadRichTextStyleSummaryIncludesOverrideAndDefaultStyle(t *testing.T) {
	richTextStyle := widgetReadRichTextStyleSummary(nil, "RichTextBlock", map[string]any{
		"TextStyleSet": map[string]any{
			"value": map[string]any{
				"index":    -12,
				"resolved": "import:12:SettingsDescriptionStyles",
			},
		},
		"DecoratorClasses": map[string]any{
			"value": []any{
				map[string]any{
					"value": map[string]any{
						"index":    -1,
						"resolved": "import:1:NewRichTextBlockDecorator_C",
					},
				},
			},
		},
		"bOverrideDefaultStyle": true,
		"Justification": map[string]any{
			"value": map[string]any{
				"value": "ETextJustify::Center",
			},
		},
		"AutoWrapText":         true,
		"WrapTextAt":           float32(320),
		"LineHeightPercentage": float32(1.25),
		"DefaultTextStyleOverride": map[string]any{
			"structType": "TextBlockStyle",
			"value": map[string]any{
				"Font": map[string]any{
					"value": map[string]any{
						"Size": map[string]any{"value": 28},
						"OutlineSettings": map[string]any{
							"value": map[string]any{
								"OutlineSize": map[string]any{"value": 2},
								"OutlineColor": map[string]any{
									"value": map[string]any{
										"r": 0.2,
										"g": 0.3,
										"b": 0.9,
										"a": 1.0,
									},
								},
							},
						},
					},
				},
				"ColorAndOpacity": map[string]any{
					"value": map[string]any{
						"SpecifiedColor": map[string]any{
							"value": map[string]any{
								"r": 1.0,
								"g": 0.9,
								"b": 0.25,
								"a": 1.0,
							},
						},
					},
				},
				"ShadowOffset": map[string]any{
					"value": map[string]any{
						"X": 3.0,
						"Y": 4.0,
					},
				},
				"ShadowColorAndOpacity": map[string]any{
					"value": map[string]any{
						"r": 0.05,
						"g": 0.1,
						"b": 0.15,
						"a": 0.8,
					},
				},
			},
		},
	})
	if richTextStyle == nil {
		t.Fatalf("richTextStyle: got nil want summary")
	}
	if got, want := richTextStyle["overrideDefaultStyle"], true; got != want {
		t.Fatalf("overrideDefaultStyle: got %#v want %v", got, want)
	}
	textStyleSet, ok := richTextStyle["textStyleSet"].(map[string]any)
	if !ok {
		t.Fatalf("textStyleSet type: got %#v", richTextStyle["textStyleSet"])
	}
	if got, want := textStyleSet["resolved"], "import:12:SettingsDescriptionStyles"; got != want {
		t.Fatalf("textStyleSet.resolved: got %#v want %q", got, want)
	}
	decoratorClasses, ok := richTextStyle["decoratorClasses"].([]map[string]any)
	if !ok || len(decoratorClasses) != 1 {
		t.Fatalf("decoratorClasses: got %#v want 1 entry", richTextStyle["decoratorClasses"])
	}
	if got, want := decoratorClasses[0]["resolved"], "import:1:NewRichTextBlockDecorator_C"; got != want {
		t.Fatalf("decoratorClasses[0].resolved: got %#v want %q", got, want)
	}
	if got, want := richTextStyle["justification"], "ETextJustify::Center"; got != want {
		t.Fatalf("justification: got %#v want %q", got, want)
	}
	if got, want := richTextStyle["autoWrapText"], true; got != want {
		t.Fatalf("autoWrapText: got %#v want %v", got, want)
	}
	if got, want := richTextStyle["wrapTextAt"], float32(320); got != want {
		t.Fatalf("wrapTextAt: got %#v want %v", got, want)
	}
	if got, want := richTextStyle["lineHeightPercentage"], float32(1.25); got != want {
		t.Fatalf("lineHeightPercentage: got %#v want %v", got, want)
	}
	defaultStyle, ok := richTextStyle["defaultTextStyleOverride"].(map[string]any)
	if !ok {
		t.Fatalf("defaultTextStyleOverride type: got %#v", richTextStyle["defaultTextStyleOverride"])
	}
	font, ok := defaultStyle["font"].(map[string]any)
	if !ok {
		t.Fatalf("font type: got %#v", defaultStyle["font"])
	}
	if got, want := font["size"], 28; got != want {
		t.Fatalf("font.size: got %#v want %d", got, want)
	}
	outline, ok := font["outlineSettings"].(map[string]any)
	if !ok {
		t.Fatalf("font.outlineSettings type: got %#v", font["outlineSettings"])
	}
	if got, want := outline["outlineSize"], 2; got != want {
		t.Fatalf("font.outlineSettings.outlineSize: got %#v want %d", got, want)
	}
	requireWidgetReadLinearColor(t, outline["outlineColor"], widgetLinearColor{R: 0.2, G: 0.3, B: 0.9, A: 1})
	requireWidgetReadLinearColor(t, defaultStyle["colorAndOpacity"], widgetLinearColor{R: 1, G: 0.9, B: 0.25, A: 1})
	requireWidgetReadFloatMap(t, defaultStyle["shadowOffset"], map[string]float32{"x": 3, "y": 4})
	requireWidgetReadLinearColor(t, defaultStyle["shadowColorAndOpacity"], widgetLinearColor{R: 0.05, G: 0.1, B: 0.15, A: 0.8})
}

func TestBuildWidgetBlueprintEntryIncludesRichTextBlockLogicalSummary(t *testing.T) {
	work, childPath := buildRichTextWidgetTestAsset(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := Run([]string{
		"blueprint", "widget-write", work,
		"--widget", childPath,
		"--property", "text",
		"--value", "Rich text body",
	}, &stdout, &stderr); code != 0 {
		t.Fatalf("widget-write exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rich text asset: %v", err)
	}
	blueprint := mustFindSingleWidgetBlueprintExport(t, asset)
	entry, err := buildWidgetBlueprintEntry(asset, blueprint)
	if err != nil {
		t.Fatalf("buildWidgetBlueprintEntry rich text: %v", err)
	}
	logical := requireWidgetReadLogicalWidgetByPath(t, entry, childPath)
	if got, want := logical["className"], "RichTextBlock"; got != want {
		t.Fatalf("className: got %#v want %q", got, want)
	}
	textSummary, ok := logical["text"].(map[string]any)
	if !ok {
		t.Fatalf("logical text summary type: got %#v", logical["text"])
	}
	if got, want := textSummary["sourceString"], "Rich text body"; got != want {
		t.Fatalf("logical text sourceString: got %#v want %q", got, want)
	}
	if _, ok := logical["textStyle"]; ok {
		t.Fatalf("RichTextBlock logical summary unexpectedly included textStyle: %#v", logical["textStyle"])
	}
	if _, ok := logical["richTextStyle"]; ok {
		t.Fatalf("RichTextBlock logical summary unexpectedly included richTextStyle: %#v", logical["richTextStyle"])
	}
}

func TestBuildWidgetBlueprintEntryIncludesRichTextBlockStyleSetSummary(t *testing.T) {
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
		t.Fatalf("parse rich text asset: %v", err)
	}
	blueprint := mustFindSingleWidgetBlueprintExport(t, asset)
	entry, err := buildWidgetBlueprintEntry(asset, blueprint)
	if err != nil {
		t.Fatalf("buildWidgetBlueprintEntry rich text style set: %v", err)
	}
	logical := requireWidgetReadLogicalWidgetByPath(t, entry, childPath)
	richTextStyle, ok := logical["richTextStyle"].(map[string]any)
	if !ok {
		t.Fatalf("logical richTextStyle type: got %#v", logical["richTextStyle"])
	}
	if got, want := richTextStyle["textStyleSetPath"], "/Game/UI/Settings/SettingsDescriptionStyles"; got != want {
		t.Fatalf("logical richTextStyle.textStyleSetPath: got %#v want %q", got, want)
	}
}

func TestBuildWidgetBlueprintEntryIncludesRichTextBlockDecoratorSummary(t *testing.T) {
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
		t.Fatalf("parse rich text asset: %v", err)
	}
	blueprint := mustFindSingleWidgetBlueprintExport(t, asset)
	entry, err := buildWidgetBlueprintEntry(asset, blueprint)
	if err != nil {
		t.Fatalf("buildWidgetBlueprintEntry rich text decorator: %v", err)
	}
	logical := requireWidgetReadLogicalWidgetByPath(t, entry, childPath)
	richTextStyle, ok := logical["richTextStyle"].(map[string]any)
	if !ok {
		t.Fatalf("logical richTextStyle type: got %#v", logical["richTextStyle"])
	}
	if got, want := richTextStyle["decoratorClassPaths"], []string{"/Game/UI/Settings/NewRichTextBlockDecorator"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("logical richTextStyle.decoratorClassPaths: got %#v want %#v", got, want)
	}
}

func TestBuildWidgetBlueprintEntryIncludesSlotSummaries(t *testing.T) {
	layoutAsset, err := uasset.ParseFile(findExistingGoldenOperationFixturePath(t, "widget_write_layout_canvaspanelslot", "after.uasset"), uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse layout asset: %v", err)
	}
	layoutBlueprint := mustFindSingleWidgetBlueprintExport(t, layoutAsset)
	layoutEntry, err := buildWidgetBlueprintEntry(layoutAsset, layoutBlueprint)
	if err != nil {
		t.Fatalf("buildWidgetBlueprintEntry layout: %v", err)
	}
	layoutLogical := requireWidgetReadLogicalWidgetByPath(t, layoutEntry, "CanvasPanel_22/Image_29")
	slotLayout, ok := layoutLogical["slotLayout"].(map[string]any)
	if !ok {
		t.Fatalf("slotLayout type: got %#v", layoutLogical["slotLayout"])
	}
	requireWidgetReadFloatSlice(t, slotLayout["position"], []float32{0, 0})
	requireWidgetReadFloatSlice(t, slotLayout["size"], []float32{200, 60})
	requireWidgetReadFloatSlice(t, slotLayout["anchors"], []float32{0.5, 0.5, 0.5, 0.5})
	requireWidgetReadFloatSlice(t, slotLayout["alignment"], []float32{0.5, 0.5})

	work := copyFixtureToTemp(t, findExistingGoldenOperationFixturePath(t, "widget_add_image_canvaspanel", "after.uasset"))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-write", work,
			"--widget", "CanvasPanel_22/Image_23",
			"--property", "slot-padding",
			"--value", "1,2,3,4",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", "CanvasPanel_22/Image_23",
			"--property", "slot-horizontal-alignment",
			"--value", "Center",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", "CanvasPanel_22/Image_23",
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

	updatedAsset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse updated asset: %v", err)
	}
	updatedBlueprint := mustFindSingleWidgetBlueprintExport(t, updatedAsset)
	updatedEntry, err := buildWidgetBlueprintEntry(updatedAsset, updatedBlueprint)
	if err != nil {
		t.Fatalf("buildWidgetBlueprintEntry updated: %v", err)
	}
	updatedLogical := requireWidgetReadLogicalWidgetByPath(t, updatedEntry, "CanvasPanel_22/Image_23")
	slotPadding, ok := updatedLogical["slotPadding"].(map[string]any)
	if !ok {
		t.Fatalf("slotPadding type: got %#v", updatedLogical["slotPadding"])
	}
	if got, want := slotPadding["left"], float32(1); got != want {
		t.Fatalf("slotPadding.left: got %#v want %v", got, want)
	}
	if got, want := updatedLogical["slotHorizontalAlignment"], "EHorizontalAlignment::HAlign_Center"; got != want {
		t.Fatalf("slotHorizontalAlignment: got %#v want %q", got, want)
	}
	if got, want := updatedLogical["slotVerticalAlignment"], "EVerticalAlignment::VAlign_Bottom"; got != want {
		t.Fatalf("slotVerticalAlignment: got %#v want %q", got, want)
	}
}

func TestBuildWidgetBlueprintEntryIncludesPaddingAndAlignmentSummariesAcrossSlotClasses(t *testing.T) {
	tests := []struct {
		name     string
		prepare  func(t *testing.T) (string, string)
		expected string
	}{
		{
			name: "OverlaySlot",
			prepare: func(t *testing.T) (string, string) {
				return copyFixtureToTemp(t, findExistingGoldenOperationFixturePath(t, "widget_add_two_image_overlay", "after.uasset")), "Overlay_21/Image_1"
			},
			expected: "Overlay_21/Image_1",
		},
		{
			name: "HorizontalBoxSlot",
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
			expected: "HorizontalBox_1/Image_1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			work, logicalPath := tt.prepare(t)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			for _, argv := range [][]string{
				{
					"blueprint", "widget-write", work,
					"--widget", logicalPath,
					"--property", "slot-padding",
					"--value", "9,10,11,12",
				},
				{
					"blueprint", "widget-write", work,
					"--widget", logicalPath,
					"--property", "slot-horizontal-alignment",
					"--value", "Center",
				},
				{
					"blueprint", "widget-write", work,
					"--widget", logicalPath,
					"--property", "slot-vertical-alignment",
					"--value", "Top",
				},
			} {
				stdout.Reset()
				stderr.Reset()
				if code := Run(argv, &stdout, &stderr); code != 0 {
					t.Fatalf("widget-write exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
				}
			}

			updatedAsset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("parse updated asset: %v", err)
			}
			updatedBlueprint := mustFindSingleWidgetBlueprintExport(t, updatedAsset)
			updatedEntry, err := buildWidgetBlueprintEntry(updatedAsset, updatedBlueprint)
			if err != nil {
				t.Fatalf("buildWidgetBlueprintEntry updated: %v", err)
			}
			updatedLogical := requireWidgetReadLogicalWidgetByPath(t, updatedEntry, logicalPath)
			slotPadding, ok := updatedLogical["slotPadding"].(map[string]any)
			if !ok {
				t.Fatalf("slotPadding type: got %#v", updatedLogical["slotPadding"])
			}
			if got, want := slotPadding["left"], float32(9); got != want {
				t.Fatalf("slotPadding.left: got %#v want %v", got, want)
			}
			if got, want := slotPadding["bottom"], float32(12); got != want {
				t.Fatalf("slotPadding.bottom: got %#v want %v", got, want)
			}
			if got, want := updatedLogical["slotHorizontalAlignment"], "EHorizontalAlignment::HAlign_Center"; got != want {
				t.Fatalf("slotHorizontalAlignment: got %#v want %q", got, want)
			}
			if got, want := updatedLogical["slotVerticalAlignment"], "EVerticalAlignment::VAlign_Top"; got != want {
				t.Fatalf("slotVerticalAlignment: got %#v want %q", got, want)
			}
		})
	}
}

func TestBuildWidgetBlueprintEntryIncludesBoxSlotSizeSummaries(t *testing.T) {
	tests := []struct {
		name     string
		rootType string
		rootName string
		value    string
		wantRule string
		wantSize float32
	}{
		{
			name:     "HorizontalBox",
			rootType: "horizontalbox",
			rootName: "HorizontalBox_1",
			value:    "fill:2",
			wantRule: "ESlateSizeRule::Fill",
			wantSize: 2,
		},
		{
			name:     "VerticalBox",
			rootType: "verticalbox",
			rootName: "VerticalBox_1",
			value:    "auto",
			wantRule: "ESlateSizeRule::Automatic",
			wantSize: 1,
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
					t.Fatalf("widget-write exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
				}
			}

			updatedAsset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("parse updated asset: %v", err)
			}
			updatedBlueprint := mustFindSingleWidgetBlueprintExport(t, updatedAsset)
			updatedEntry, err := buildWidgetBlueprintEntry(updatedAsset, updatedBlueprint)
			if err != nil {
				t.Fatalf("buildWidgetBlueprintEntry updated: %v", err)
			}
			updatedLogical := requireWidgetReadLogicalWidgetByPath(t, updatedEntry, tt.rootName+"/Image_1")
			slotSize, ok := updatedLogical["slotSize"].(map[string]any)
			if !ok {
				t.Fatalf("slotSize type: got %#v", updatedLogical["slotSize"])
			}
			if got := slotSize["rule"]; got != tt.wantRule {
				t.Fatalf("slotSize.rule: got %#v want %q", got, tt.wantRule)
			}
			if got := slotSize["value"]; got != tt.wantSize {
				t.Fatalf("slotSize.value: got %#v want %v", got, tt.wantSize)
			}
		})
	}
}

func TestBuildWidgetBlueprintEntryIncludesGridFillSummaries(t *testing.T) {
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
			"--value", "1,2,1",
		},
		{
			"blueprint", "widget-write", work,
			"--widget", "GridPanel_1",
			"--property", "grid-row-fill",
			"--value", "3,1",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
			t.Fatalf("widget-write exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	updatedAsset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse updated asset: %v", err)
	}
	updatedBlueprint := mustFindSingleWidgetBlueprintExport(t, updatedAsset)
	updatedEntry, err := buildWidgetBlueprintEntry(updatedAsset, updatedBlueprint)
	if err != nil {
		t.Fatalf("buildWidgetBlueprintEntry updated: %v", err)
	}
	gridLogical := requireWidgetReadLogicalWidgetByPath(t, updatedEntry, "GridPanel_1")
	requireWidgetReadFloatSlice(t, gridLogical["gridColumnFill"], []float32{1, 2, 1})
	requireWidgetReadFloatSlice(t, gridLogical["gridRowFill"], []float32{3, 1})
}

func TestBuildWidgetBlueprintEntryIncludesButtonAndBorderStyleSummaries(t *testing.T) {
	tests := []struct {
		name       string
		prepare    [][]string
		updates    [][]string
		targetPath string
		verify     func(t *testing.T, logical map[string]any)
	}{
		{
			name: "Button",
			prepare: [][]string{
				{"blueprint", "widget-add", "--parent", "root", "--type", "button", "--name", "Button_1"},
				{"blueprint", "widget-add", "--parent", "Button_1", "--type", "image", "--name", "Image_1"},
			},
			updates: [][]string{
				{"blueprint", "widget-write", "--widget", "Button_1", "--property", "button-background-color", "--value", "0.1,0.2,0.3,1"},
				{"blueprint", "widget-write", "--widget", "Button_1", "--property", "button-color-and-opacity", "--value", "0.9,0.85,0.4,0.75"},
				{"blueprint", "widget-write", "--widget", "Button_1", "--property", "button-normal-image", "--value", "/Game/UI/Menu/Art/T_UI_Icon_SimpleArrow"},
				{"blueprint", "widget-write", "--widget", "Button_1", "--property", "button-hovered-image", "--value", "/Game/UI/Menu/Art/T_UI_Icon_SimpleDiamond"},
				{"blueprint", "widget-write", "--widget", "Button_1", "--property", "button-normal-tint", "--value", "0.25,0.4,0.9,0.8"},
				{"blueprint", "widget-write", "--widget", "Button_1", "--property", "button-normal-image-size", "--value", "80,56"},
				{"blueprint", "widget-write", "--widget", "Button_1", "--property", "button-normal-draw-as", "--value", "RoundedBox"},
			},
			targetPath: "Button_1",
			verify: func(t *testing.T, logical map[string]any) {
				t.Helper()
				style, ok := logical["buttonStyle"].(map[string]any)
				if !ok {
					t.Fatalf("buttonStyle type: got %#v", logical["buttonStyle"])
				}
				requireWidgetReadLinearColor(t, style["backgroundColor"], widgetLinearColor{R: 0.1, G: 0.2, B: 0.3, A: 1})
				requireWidgetReadLinearColor(t, style["colorAndOpacity"], widgetLinearColor{R: 0.9, G: 0.85, B: 0.4, A: 0.75})
				brushes, ok := style["brushes"].(map[string]any)
				if !ok {
					t.Fatalf("buttonStyle.brushes type: got %#v", style["brushes"])
				}
				normal, ok := brushes["normal"].(map[string]any)
				if !ok {
					t.Fatalf("buttonStyle.brushes.normal type: got %#v", brushes["normal"])
				}
				requireWidgetReadBrushPath(t, normal, "/Game/UI/Menu/Art/T_UI_Icon_SimpleArrow")
				requireWidgetReadLinearColor(t, normal["tintColor"], widgetLinearColor{R: 0.25, G: 0.4, B: 0.9, A: 0.8})
				requireWidgetReadVector2(t, normal["imageSize"], 80, 56)
				if got, want := normal["drawAs"], "ESlateBrushDrawType::RoundedBox"; got != want {
					t.Fatalf("buttonStyle.brushes.normal.drawAs: got %#v want %q", got, want)
				}
				requireWidgetReadBrushPath(t, brushes["hovered"], "/Game/UI/Menu/Art/T_UI_Icon_SimpleDiamond")
			},
		},
		{
			name: "Border",
			prepare: [][]string{
				{"blueprint", "widget-add", "--parent", "root", "--type", "border", "--name", "Border_1"},
				{"blueprint", "widget-add", "--parent", "Border_1", "--type", "image", "--name", "Image_1"},
			},
			updates: [][]string{
				{"blueprint", "widget-write", "--widget", "Border_1", "--property", "border-padding", "--value", "6,8,10,12"},
				{"blueprint", "widget-write", "--widget", "Border_1", "--property", "border-brush-color", "--value", "0.2,0.3,0.4,0.95"},
				{"blueprint", "widget-write", "--widget", "Border_1", "--property", "border-content-color-and-opacity", "--value", "1,0.6,0.25,0.8"},
				{"blueprint", "widget-write", "--widget", "Border_1", "--property", "border-horizontal-alignment", "--value", "Right"},
				{"blueprint", "widget-write", "--widget", "Border_1", "--property", "border-vertical-alignment", "--value", "Center"},
			},
			targetPath: "Border_1",
			verify: func(t *testing.T, logical map[string]any) {
				t.Helper()
				style, ok := logical["borderStyle"].(map[string]any)
				if !ok {
					t.Fatalf("borderStyle type: got %#v", logical["borderStyle"])
				}
				padding, ok := style["padding"].(map[string]any)
				if !ok {
					t.Fatalf("borderStyle.padding type: got %#v", style["padding"])
				}
				if got, want := padding["left"], float32(6); got != want {
					t.Fatalf("borderStyle.padding.left: got %#v want %v", got, want)
				}
				if got, want := padding["bottom"], float32(12); got != want {
					t.Fatalf("borderStyle.padding.bottom: got %#v want %v", got, want)
				}
				requireWidgetReadLinearColor(t, style["brushColor"], widgetLinearColor{R: 0.2, G: 0.3, B: 0.4, A: 0.95})
				requireWidgetReadLinearColor(t, style["contentColorAndOpacity"], widgetLinearColor{R: 1, G: 0.6, B: 0.25, A: 0.8})
				if got, want := style["horizontalAlignment"], "EHorizontalAlignment::HAlign_Right"; got != want {
					t.Fatalf("borderStyle.horizontalAlignment: got %#v want %q", got, want)
				}
				if got, want := style["verticalAlignment"], "EVerticalAlignment::VAlign_Center"; got != want {
					t.Fatalf("borderStyle.verticalAlignment: got %#v want %q", got, want)
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
					t.Fatalf("widget-write exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
				}
			}

			updatedAsset, err := uasset.ParseFile(work, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("parse updated asset: %v", err)
			}
			updatedBlueprint := mustFindSingleWidgetBlueprintExport(t, updatedAsset)
			updatedEntry, err := buildWidgetBlueprintEntry(updatedAsset, updatedBlueprint)
			if err != nil {
				t.Fatalf("buildWidgetBlueprintEntry updated: %v", err)
			}
			tt.verify(t, requireWidgetReadLogicalWidgetByPath(t, updatedEntry, tt.targetPath))
		})
	}
}

func TestBuildWidgetBlueprintEntryIncludesGridSlotAndMenuAnchorSummaries(t *testing.T) {
	gridWork := copyFixtureToTemp(t, findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-add", gridWork,
			"--parent", "root",
			"--type", "gridpanel",
			"--name", "GridPanel_1",
		},
		{
			"blueprint", "widget-add", gridWork,
			"--parent", "GridPanel_1",
			"--type", "image",
			"--name", "Image_1",
		},
		{
			"blueprint", "widget-write", gridWork,
			"--widget", "GridPanel_1/Image_1",
			"--property", "slot-row",
			"--value", "4",
		},
		{
			"blueprint", "widget-write", gridWork,
			"--widget", "GridPanel_1/Image_1",
			"--property", "slot-column",
			"--value", "2",
		},
		{
			"blueprint", "widget-write", gridWork,
			"--widget", "GridPanel_1/Image_1",
			"--property", "slot-row-span",
			"--value", "3",
		},
		{
			"blueprint", "widget-write", gridWork,
			"--widget", "GridPanel_1/Image_1",
			"--property", "slot-column-span",
			"--value", "4",
		},
		{
			"blueprint", "widget-write", gridWork,
			"--widget", "GridPanel_1/Image_1",
			"--property", "slot-layer",
			"--value", "7",
		},
		{
			"blueprint", "widget-write", gridWork,
			"--widget", "GridPanel_1/Image_1",
			"--property", "slot-nudge",
			"--value", "3,6",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
			t.Fatalf("grid widget-write exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	gridAsset, err := uasset.ParseFile(gridWork, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse grid asset: %v", err)
	}
	gridBlueprint := mustFindSingleWidgetBlueprintExport(t, gridAsset)
	gridEntry, err := buildWidgetBlueprintEntry(gridAsset, gridBlueprint)
	if err != nil {
		t.Fatalf("buildWidgetBlueprintEntry grid: %v", err)
	}
	gridLogical := requireWidgetReadLogicalWidgetByPath(t, gridEntry, "GridPanel_1/Image_1")
	if got, want := gridLogical["slotRow"], 4; got != want {
		t.Fatalf("slotRow: got %#v want %d", got, want)
	}
	if got, want := gridLogical["slotColumn"], 2; got != want {
		t.Fatalf("slotColumn: got %#v want %d", got, want)
	}
	if got, want := gridLogical["slotRowSpan"], 3; got != want {
		t.Fatalf("slotRowSpan: got %#v want %d", got, want)
	}
	if got, want := gridLogical["slotColumnSpan"], 4; got != want {
		t.Fatalf("slotColumnSpan: got %#v want %d", got, want)
	}
	if got, want := gridLogical["slotLayer"], 7; got != want {
		t.Fatalf("slotLayer: got %#v want %d", got, want)
	}
	nudge, ok := gridLogical["slotNudge"].(map[string]any)
	if !ok {
		t.Fatalf("slotNudge type: got %#v", gridLogical["slotNudge"])
	}
	if got, want := nudge["x"], float32(3); got != want {
		t.Fatalf("slotNudge.x: got %#v want %v", got, want)
	}
	if got, want := nudge["y"], float32(6); got != want {
		t.Fatalf("slotNudge.y: got %#v want %v", got, want)
	}

	menuWork := copyFixtureToTemp(t, findExistingGoldenParseFixturePath(t, "WBP_Minimum.uasset"))
	for _, argv := range [][]string{
		{
			"blueprint", "widget-add", menuWork,
			"--parent", "root",
			"--type", "menuanchor",
			"--name", "MenuAnchor_1",
		},
		{
			"blueprint", "widget-write", menuWork,
			"--widget", "MenuAnchor_1",
			"--property", "menu-anchor-placement",
			"--value", "MenuRight",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
			t.Fatalf("menu widget-write exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	menuAsset, err := uasset.ParseFile(menuWork, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse menu asset: %v", err)
	}
	menuBlueprint := mustFindSingleWidgetBlueprintExport(t, menuAsset)
	menuEntry, err := buildWidgetBlueprintEntry(menuAsset, menuBlueprint)
	if err != nil {
		t.Fatalf("buildWidgetBlueprintEntry menu: %v", err)
	}
	menuLogical := requireWidgetReadLogicalWidgetByPath(t, menuEntry, "MenuAnchor_1")
	if got, want := menuLogical["menuAnchorPlacement"], "EMenuPlacement::MenuPlacement_MenuRight"; got != want {
		t.Fatalf("menuAnchorPlacement: got %#v want %q", got, want)
	}
}

func TestBuildWidgetBlueprintEntryMergesGeneratedRootlessWidgetsBySuffix(t *testing.T) {
	asset, err := uasset.ParseFile(findExistingGoldenOperationFixturePath(t, "widget_add_image_nested_overlay", "after.uasset"), uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse nested overlay asset: %v", err)
	}
	blueprintExport := mustFindSingleWidgetBlueprintExport(t, asset)
	entry, err := buildWidgetBlueprintEntry(asset, blueprintExport)
	if err != nil {
		t.Fatalf("buildWidgetBlueprintEntry nested overlay: %v", err)
	}
	if got, want := entry["logicalWidgetCount"], 5; got != want {
		t.Fatalf("logicalWidgetCount: got %#v want %d", got, want)
	}

	overlay := requireWidgetReadLogicalWidgetByPath(t, entry, "CanvasPanel_1/Overlay_1")
	if got, want := len(widgetWriteZeroBasedIndices(overlay["widgetExports"])), 2; got != want {
		t.Fatalf("overlay widget export count: got %d want %d", got, want)
	}
	requireWidgetReadStringSet(t, overlay["presentIn"], []string{"designer", "generated"})

	child := requireWidgetReadLogicalWidgetByPath(t, entry, "CanvasPanel_1/Overlay_1/Image_3")
	if got, want := len(widgetWriteZeroBasedIndices(child["widgetExports"])), 2; got != want {
		t.Fatalf("nested child widget export count: got %d want %d", got, want)
	}
	requireWidgetReadStringSet(t, child["presentIn"], []string{"designer", "generated"})
}

func requireWidgetReadLogicalWidgetByPath(t *testing.T, entry map[string]any, path string) map[string]any {
	t.Helper()
	items, ok := entry["logicalWidgets"].([]map[string]any)
	if ok {
		for _, item := range items {
			if item["path"] == path {
				return item
			}
		}
	}
	rawItems, ok := entry["logicalWidgets"].([]any)
	if !ok {
		t.Fatalf("logicalWidgets type: got %#v", entry["logicalWidgets"])
	}
	for _, raw := range rawItems {
		item, ok := raw.(map[string]any)
		if ok && item["path"] == path {
			return item
		}
	}
	t.Fatalf("logical widget %q not found", path)
	return nil
}

func requireWidgetReadFloatSlice(t *testing.T, raw any, want []float32) {
	t.Helper()
	items, ok := raw.([]float32)
	if ok {
		if !reflect.DeepEqual(items, want) {
			t.Fatalf("float slice: got %v want %v", items, want)
		}
		return
	}
	generic, ok := raw.([]any)
	if !ok {
		t.Fatalf("float slice type: got %#v", raw)
	}
	got := make([]float32, 0, len(generic))
	for _, item := range generic {
		switch value := item.(type) {
		case float32:
			got = append(got, value)
		case float64:
			got = append(got, float32(value))
		default:
			t.Fatalf("float slice item type: got %#v", item)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("float slice: got %v want %v", got, want)
	}
}

func requireWidgetReadStringSet(t *testing.T, raw any, want []string) {
	t.Helper()
	got := make([]string, 0, len(want))
	switch items := raw.(type) {
	case []string:
		got = append(got, items...)
	case []any:
		for _, item := range items {
			text, ok := item.(string)
			if !ok {
				t.Fatalf("string slice item type: got %#v", item)
			}
			got = append(got, text)
		}
	default:
		t.Fatalf("string slice type: got %#v", raw)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("string slice: got %v want %v", got, want)
	}
}

func requireWidgetReadLinearColor(t *testing.T, raw any, want widgetLinearColor) {
	t.Helper()
	summary, ok := widgetReadLinearColorSummary(raw)
	if !ok {
		m, mapOK := raw.(map[string]any)
		if !mapOK {
			t.Fatalf("linear color type: got %#v", raw)
		}
		summary = m
	}
	if got := summary["r"]; got != want.R {
		t.Fatalf("linear color r: got %#v want %v", got, want.R)
	}
	if got := summary["g"]; got != want.G {
		t.Fatalf("linear color g: got %#v want %v", got, want.G)
	}
	if got := summary["b"]; got != want.B {
		t.Fatalf("linear color b: got %#v want %v", got, want.B)
	}
	if got := summary["a"]; got != want.A {
		t.Fatalf("linear color a: got %#v want %v", got, want.A)
	}
}

func requireWidgetReadFloatMap(t *testing.T, raw any, want map[string]float32) {
	t.Helper()
	got, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("float map type: got %#v", raw)
	}
	for key, wantValue := range want {
		value, ok := got[key]
		if !ok {
			t.Fatalf("float map missing key %q in %#v", key, got)
		}
		switch typed := value.(type) {
		case float32:
			if typed != wantValue {
				t.Fatalf("float map %s: got %v want %v", key, typed, wantValue)
			}
		case float64:
			if float32(typed) != wantValue {
				t.Fatalf("float map %s: got %v want %v", key, typed, wantValue)
			}
		default:
			t.Fatalf("float map %s type: got %#v", key, value)
		}
	}
}

func requireWidgetReadBrushPath(t *testing.T, raw any, want string) {
	t.Helper()
	brush, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("brush type: got %#v", raw)
	}
	if got := brush["resourceObjectPath"]; got != want {
		t.Fatalf("brush resourceObjectPath: got %#v want %q", got, want)
	}
}

func requireWidgetReadVector2(t *testing.T, raw any, wantX, wantY float32) {
	t.Helper()
	value, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("vector2 type: got %#v", raw)
	}
	if got := value["x"]; got != wantX {
		t.Fatalf("vector2 x: got %#v want %v", got, wantX)
	}
	if got := value["y"]; got != wantY {
		t.Fatalf("vector2 y: got %#v want %v", got, wantY)
	}
}
