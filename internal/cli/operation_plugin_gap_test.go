package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestOperationFixturesMissingFromUEPluginAreExplicit(t *testing.T) {
	t.Parallel()

	pluginSpecs, err := uePluginGeneratedOperationNames()
	if err != nil {
		t.Fatalf("read UE plugin operation specs: %v", err)
	}
	if len(pluginSpecs) == 0 {
		t.Fatalf("no UE plugin operation specs found")
	}

	for _, root := range goldenFixtureRoots(t, "operations") {
		root := root
		t.Run(filepath.Base(root), func(t *testing.T) {
			entries, err := os.ReadDir(filepath.Join(root, "operations"))
			if err != nil {
				t.Fatalf("read operations dir: %v", err)
			}

			goldenOps := make([]string, 0, len(entries))
			for _, opDir := range listOperationSpecDirs(entries, filepath.Join(root, "operations")) {
				goldenOps = append(goldenOps, filepath.Base(opDir))
			}
			slices.Sort(goldenOps)

			missing := make([]string, 0, 32)
			for _, name := range goldenOps {
				if !slices.Contains(pluginSpecs, name) {
					missing = append(missing, name)
				}
			}

			expectedMissing := []string{}
			if !slices.Equal(missing, expectedMissing) {
				t.Fatalf("unexpected UE plugin coverage gap\nactual=%v\nexpected=%v", missing, expectedMissing)
			}
		})
	}
}

func TestUEPluginGeneratedOperationNamesIncludeButtonBrushDetailOps(t *testing.T) {
	t.Parallel()

	pluginSpecs, err := uePluginGeneratedOperationNames()
	if err != nil {
		t.Fatalf("read UE plugin operation specs: %v", err)
	}

	required := []string{
		"widget_write_button_brush_normal",
		"widget_write_button_brush_tint",
		"widget_write_button_brush_image_size",
		"widget_write_button_brush_draw_as",
	}
	for _, name := range required {
		if !slices.Contains(pluginSpecs, name) {
			t.Fatalf("UE plugin operation spec missing %q", name)
		}
	}
}

func TestUEPluginBasicWidgetAddOperationsAreNoLongerDeferred(t *testing.T) {
	t.Parallel()

	notYetGenerated, err := uePluginNotYetGeneratedOperationNames()
	if err != nil {
		t.Fatalf("read UE plugin deferred operation specs: %v", err)
	}

	required := []string{
		"widget_add_progressbar_canvaspanel",
		"widget_add_slider_canvaspanel",
		"widget_add_spacer_canvaspanel",
		"widget_add_scrollbar_canvaspanel",
	}
	for _, name := range required {
		if slices.Contains(notYetGenerated, name) {
			t.Fatalf("UE plugin basic widget add operation should no longer be deferred: %q", name)
		}
	}
}

func TestUEPluginBasicWidgetWriteOperationsAreNoLongerDeferred(t *testing.T) {
	t.Parallel()

	notYetGenerated, err := uePluginNotYetGeneratedOperationNames()
	if err != nil {
		t.Fatalf("read UE plugin deferred operation specs: %v", err)
	}

	required := []string{
		"widget_write_progressbar_percent",
		"widget_write_progressbar_fill_color",
		"widget_write_slider_value",
		"widget_write_slider_orientation",
		"widget_write_spacer_size",
		"widget_write_scrollbar_thickness",
		"widget_write_scrollbar_orientation",
	}
	for _, name := range required {
		if slices.Contains(notYetGenerated, name) {
			t.Fatalf("UE plugin basic widget write operation should no longer be deferred: %q", name)
		}
	}
}

func TestUEPluginScrollBoxSizeBoxAndTextStyleOperationsAreNoLongerDeferred(t *testing.T) {
	t.Parallel()

	notYetGenerated, err := uePluginNotYetGeneratedOperationNames()
	if err != nil {
		t.Fatalf("read UE plugin deferred operation specs: %v", err)
	}

	required := []string{
		"widget_write_text_color_root_textblock",
		"widget_write_text_font_size_root_textblock",
		"widget_write_text_justification_root_textblock",
		"widget_write_text_font_root_textblock",
		"widget_write_text_typeface_root_textblock",
		"widget_write_text_auto_wrap_text_root_textblock",
		"widget_write_text_wrap_text_at_root_textblock",
		"widget_write_text_line_height_percentage_root_textblock",
		"widget_write_text_shadow_offset_root_textblock",
		"widget_write_text_shadow_color_root_textblock",
		"widget_write_text_outline_size_root_textblock",
		"widget_write_text_outline_color_root_textblock",
		"widget_write_checkbox_is_checked_canvaspanel",
		"widget_write_checkbox_checked_state_canvaspanel",
		"widget_write_sizebox_width_canvaspanel",
		"widget_write_sizebox_height_canvaspanel",
		"widget_write_sizebox_min_desired_width_canvaspanel",
		"widget_write_sizebox_min_desired_height_canvaspanel",
		"widget_write_sizebox_max_desired_width_canvaspanel",
		"widget_write_sizebox_max_desired_height_canvaspanel",
		"widget_write_sizebox_min_aspect_ratio_canvaspanel",
		"widget_write_sizebox_max_aspect_ratio_canvaspanel",
		"widget_write_scrollbox_orientation_canvaspanel",
		"widget_write_scrollbox_scrollbar_visibility_canvaspanel",
		"widget_write_scrollbox_consume_mouse_wheel_canvaspanel",
		"widget_write_richtext_auto_wrap_text",
		"widget_write_richtext_wrap_text_at",
		"widget_write_richtext_line_height_percentage",
	}
	for _, name := range required {
		if slices.Contains(notYetGenerated, name) {
			t.Fatalf("UE plugin text/layout widget write operation should no longer be deferred: %q", name)
		}
	}
}

func TestUEPluginScaleBoxWrapBoxAndWidgetSwitcherWriteOperationsAreNoLongerDeferred(t *testing.T) {
	t.Parallel()

	notYetGenerated, err := uePluginNotYetGeneratedOperationNames()
	if err != nil {
		t.Fatalf("read UE plugin deferred operation specs: %v", err)
	}

	required := []string{
		"widget_write_scalebox_stretch_canvaspanel",
		"widget_write_scalebox_stretch_direction_canvaspanel",
		"widget_write_scalebox_user_specified_scale_canvaspanel",
		"widget_write_scalebox_ignore_inherited_scale_canvaspanel",
		"widget_write_wrapbox_wrap_size_canvaspanel",
		"widget_write_wrapbox_explicit_wrap_size_canvaspanel",
		"widget_write_wrapbox_inner_slot_padding_canvaspanel",
		"widget_write_wrapbox_orientation_canvaspanel",
		"widget_write_widgetswitcher_active_widget_index_canvaspanel",
	}
	for _, name := range required {
		if slices.Contains(notYetGenerated, name) {
			t.Fatalf("UE plugin ScaleBox/WrapBox/WidgetSwitcher operation should no longer be deferred: %q", name)
		}
	}
}

func TestUEPluginWrapperAndUniformGridWriteOperationsAreNoLongerDeferred(t *testing.T) {
	t.Parallel()

	notYetGenerated, err := uePluginNotYetGeneratedOperationNames()
	if err != nil {
		t.Fatalf("read UE plugin deferred operation specs: %v", err)
	}

	required := []string{
		"widget_write_retainerbox_retain_render_canvaspanel",
		"widget_write_retainerbox_render_on_invalidation_canvaspanel",
		"widget_write_retainerbox_render_on_phase_canvaspanel",
		"widget_write_retainerbox_phase_canvaspanel",
		"widget_write_retainerbox_phase_count_canvaspanel",
		"widget_write_backgroundblur_strength_canvaspanel",
		"widget_write_backgroundblur_apply_alpha_to_blur_canvaspanel",
		"widget_write_safezone_pad_left_canvaspanel",
		"widget_write_safezone_pad_right_canvaspanel",
		"widget_write_safezone_pad_top_canvaspanel",
		"widget_write_safezone_pad_bottom_canvaspanel",
		"widget_write_invalidationbox_can_cache_canvaspanel",
		"widget_write_uniformgridpanel_min_desired_slot_width_canvaspanel",
		"widget_write_uniformgridpanel_min_desired_slot_height_canvaspanel",
		"widget_write_uniformgridpanel_slot_padding_canvaspanel",
	}
	for _, name := range required {
		if slices.Contains(notYetGenerated, name) {
			t.Fatalf("UE plugin wrapper/UniformGridPanel operation should no longer be deferred: %q", name)
		}
	}
}

func TestUEPluginInputWidgetAddOperationsAreNoLongerDeferred(t *testing.T) {
	t.Parallel()

	notYetGenerated, err := uePluginNotYetGeneratedOperationNames()
	if err != nil {
		t.Fatalf("read UE plugin deferred operation specs: %v", err)
	}

	required := []string{
		"widget_add_editabletext_canvaspanel",
		"widget_add_multilineeditabletextbox_canvaspanel",
		"widget_add_spinbox_canvaspanel",
		"widget_add_comboboxstring_canvaspanel",
	}
	for _, name := range required {
		if slices.Contains(notYetGenerated, name) {
			t.Fatalf("UE plugin input widget add operation should no longer be deferred: %q", name)
		}
	}
}

func TestUEPluginListViewWidgetAddOperationsAreNoLongerDeferred(t *testing.T) {
	t.Parallel()

	notYetGenerated, err := uePluginNotYetGeneratedOperationNames()
	if err != nil {
		t.Fatalf("read UE plugin deferred operation specs: %v", err)
	}

	required := []string{
		"widget_add_listview_canvaspanel",
		"widget_add_tileview_canvaspanel",
		"widget_add_treeview_canvaspanel",
	}
	for _, name := range required {
		if slices.Contains(notYetGenerated, name) {
			t.Fatalf("UE plugin list/tile/tree view add operation should no longer be deferred: %q", name)
		}
	}
}

func TestUEPluginInputWidgetWriteOperationsAreNoLongerDeferred(t *testing.T) {
	t.Parallel()

	notYetGenerated, err := uePluginNotYetGeneratedOperationNames()
	if err != nil {
		t.Fatalf("read UE plugin deferred operation specs: %v", err)
	}

	required := []string{
		"widget_write_text_canvaspanel_child_editabletext",
		"widget_write_editabletext_hint_text_canvaspanel",
		"widget_write_editabletext_is_read_only_canvaspanel",
		"widget_write_editabletext_is_password_canvaspanel",
		"widget_write_editabletext_minimum_desired_width_canvaspanel",
		"widget_write_editabletext_justification_canvaspanel",
		"widget_write_text_canvaspanel_child_multilineeditabletextbox",
		"widget_write_multilineeditabletextbox_hint_text_canvaspanel",
		"widget_write_multilineeditabletextbox_is_read_only_canvaspanel",
		"widget_write_multilineeditabletextbox_justification_canvaspanel",
		"widget_write_spinbox_value_canvaspanel",
		"widget_write_spinbox_min_value_canvaspanel",
		"widget_write_spinbox_max_value_canvaspanel",
		"widget_write_spinbox_delta_canvaspanel",
		"widget_write_comboboxstring_options_canvaspanel",
		"widget_write_comboboxstring_selected_option_canvaspanel",
	}
	for _, name := range required {
		if slices.Contains(notYetGenerated, name) {
			t.Fatalf("UE plugin input widget write operation should no longer be deferred: %q", name)
		}
	}
}

func TestUEPluginListViewWidgetWriteOperationsAreNoLongerDeferred(t *testing.T) {
	t.Parallel()

	notYetGenerated, err := uePluginNotYetGeneratedOperationNames()
	if err != nil {
		t.Fatalf("read UE plugin deferred operation specs: %v", err)
	}

	required := []string{
		"widget_write_listview_entry_widget_class_canvaspanel",
		"widget_write_listview_orientation_canvaspanel",
		"widget_write_listview_selection_mode_canvaspanel",
		"widget_write_listview_consume_mouse_wheel_canvaspanel",
		"widget_write_listview_is_focusable_canvaspanel",
		"widget_write_listview_return_focus_to_selection_canvaspanel",
		"widget_write_listview_clear_scroll_velocity_on_selection_canvaspanel",
		"widget_write_listview_scroll_into_view_alignment_canvaspanel",
		"widget_write_listview_wheel_scroll_multiplier_canvaspanel",
		"widget_write_listview_enable_scroll_animation_canvaspanel",
		"widget_write_listview_allow_overscroll_canvaspanel",
		"widget_write_listview_enable_right_click_scrolling_canvaspanel",
		"widget_write_listview_enable_touch_scrolling_canvaspanel",
		"widget_write_listview_is_pointer_scrolling_enabled_canvaspanel",
		"widget_write_listview_is_gamepad_scrolling_enabled_canvaspanel",
		"widget_write_listview_horizontal_entry_spacing_canvaspanel",
		"widget_write_listview_vertical_entry_spacing_canvaspanel",
		"widget_write_listview_scrollbar_padding_canvaspanel",
		"widget_write_tileview_entry_widget_class_canvaspanel",
		"widget_write_tileview_entry_width_canvaspanel",
		"widget_write_tileview_entry_height_canvaspanel",
		"widget_write_tileview_scrollbar_disabled_visibility_canvaspanel",
		"widget_write_tileview_entry_size_includes_entry_spacing_canvaspanel",
		"widget_write_treeview_entry_widget_class_canvaspanel",
		"widget_write_treeview_selection_mode_canvaspanel",
	}
	for _, name := range required {
		if slices.Contains(notYetGenerated, name) {
			t.Fatalf("UE plugin list/tile/tree view widget write operation should no longer be deferred: %q", name)
		}
	}
}

func TestUEPluginBasicWidgetDeferredOperationsHaveAfterStateImplementations(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("../../testdata/BPXFixtureGenerator/Source/BPXFixtureGenerator/Private/BPXGenerateFixturesCommandlet.cpp")
	if err != nil {
		t.Fatalf("read UE plugin source: %v", err)
	}
	text := string(body)

	required := []string{
		"AddBareCanvasPanelLeafWidget<UProgressBar>(",
		"AddBareCanvasPanelLeafWidget<USlider>(",
		"AddBareCanvasPanelLeafWidget<USpacer>(",
		"AddBareCanvasPanelLeafWidget<UScrollBar>(",
		"ProgressBar->Percent = 0.75f;",
		"ProgressBar->FillColorAndOpacity = FLinearColor(0.2f, 0.4f, 0.6f, 0.8f);",
		"Slider->Value = 0.5f;",
		"Slider->Orientation = EOrientation::Orient_Vertical;",
		"Spacer->Size = FVector2D(24.0f, 48.0f);",
		"ScrollBar->Thickness = FVector2D(5.0f, 12.0f);",
		"ScrollBar->Orientation = EOrientation::Orient_Vertical;",
	}
	for _, needle := range required {
		if !strings.Contains(text, needle) {
			t.Fatalf("UE plugin basic widget after-state implementation missing %q", needle)
		}
	}
}

func TestUEPluginTextAndLayoutWidgetWriteOperationsHaveAfterStateImplementations(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("../../testdata/BPXFixtureGenerator/Source/BPXFixtureGenerator/Private/BPXGenerateFixturesCommandlet.cpp")
	if err != nil {
		t.Fatalf("read UE plugin source: %v", err)
	}
	text := string(body)

	required := []string{
		"TextBlock->SetColorAndOpacity(FSlateColor(FLinearColor(0.15f, 0.45f, 0.75f, 0.9f)));",
		"FontInfo.Size = 28;",
		"FontInfo.FontObject = FontObject;",
		"FontInfo.TypefaceFontName = TEXT(\"Bold\");",
		"TextBlock->SetJustification(ETextJustify::Center);",
		"TextBlock->SetAutoWrapText(true);",
		"TextBlock->SetWrapTextAt(320.0f);",
		"TextBlock->SetLineHeightPercentage(1.25f);",
		"TextBlock->SetShadowOffset(FVector2D(3.0f, 4.0f));",
		"TextBlock->SetShadowColorAndOpacity(FLinearColor(0.05f, 0.1f, 0.15f, 0.8f));",
		"FontInfo.OutlineSettings.OutlineSize = 2;",
		"FontInfo.OutlineSettings.OutlineColor = FLinearColor(0.2f, 0.3f, 0.9f, 1.0f);",
		"CheckBox->SetIsChecked(true);",
		"CheckBox->SetCheckedState(ECheckBoxState::Undetermined);",
		"SizeBox->SetWidthOverride(320.0f);",
		"SizeBox->SetHeightOverride(72.0f);",
		"SizeBox->SetMinDesiredWidth(160.0f);",
		"SizeBox->SetMinDesiredHeight(48.0f);",
		"SizeBox->SetMaxDesiredWidth(640.0f);",
		"SizeBox->SetMaxDesiredHeight(240.0f);",
		"SizeBox->SetMinAspectRatio(1.25f);",
		"SizeBox->SetMaxAspectRatio(2.0f);",
		"ScrollBox->SetOrientation(EOrientation::Orient_Horizontal);",
		"ScrollBox->SetScrollBarVisibility(ESlateVisibility::Collapsed);",
		"ScrollBox->SetConsumeMouseWheel(EConsumeMouseWheel::Always);",
		"RichTextBlock->SetAutoWrapText(true);",
		"RichTextBlock->SetWrapTextAt(320.0f);",
		"RichTextBlock->SetLineHeightPercentage(1.25f);",
	}
	for _, needle := range required {
		if !strings.Contains(text, needle) {
			t.Fatalf("UE plugin text/layout widget write after-state implementation missing %q", needle)
		}
	}
}

func TestUEPluginInputWidgetWriteOperationsHaveAfterStateImplementations(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("../../testdata/BPXFixtureGenerator/Source/BPXFixtureGenerator/Private/BPXGenerateFixturesCommandlet.cpp")
	if err != nil {
		t.Fatalf("read UE plugin source: %v", err)
	}
	text := string(body)

	required := []string{
		"EditableText->SetText(FText::FromString(TEXT(\"Display Name\")));",
		"EditableText->SetHintText(FText::FromString(TEXT(\"Enter display name\")));",
		"EditableText->SetIsReadOnly(true);",
		"EditableText->SetIsPassword(true);",
		"EditableText->SetMinimumDesiredWidth(260.0f);",
		"EditableText->SetJustification(ETextJustify::Right);",
		"MultiLineEditableTextBox->SetText(FText::FromString(TEXT(\"Line 1\\nLine 2\")));",
		"MultiLineEditableTextBox->SetHintText(FText::FromString(TEXT(\"Enter description\")));",
		"MultiLineEditableTextBox->SetIsReadOnly(true);",
		"MultiLineEditableTextBox->SetJustification(ETextJustify::Center);",
		"SpinBox->SetValue(42.0f);",
		"SpinBox->SetMinValue(10.0f);",
		"SpinBox->SetMaxValue(100.0f);",
		"SpinBox->SetDelta(5.0f);",
		"ComboBoxString->ClearOptions();",
		"ComboBoxString->AddOption(TEXT(\"Easy\"));",
		"SelectedOptionProperty->SetPropertyValue_InContainer(ComboBoxString, TEXT(\"Normal\"));",
	}
	for _, needle := range required {
		if !strings.Contains(text, needle) {
			t.Fatalf("UE plugin input widget after-state implementation missing %q", needle)
		}
	}
}

func TestUEPluginListViewWidgetWriteOperationsHaveAfterStateImplementations(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("../../testdata/BPXFixtureGenerator/Source/BPXFixtureGenerator/Private/BPXGenerateFixturesCommandlet.cpp")
	if err != nil {
		t.Fatalf("read UE plugin source: %v", err)
	}
	text := string(body)

	required := []string{
		"*OrientationPtr = static_cast<uint8>(EOrientation::Orient_Horizontal);",
		"ListView->SetSelectionMode(ESelectionMode::Multi);",
		"ConsumeProp = FindFProperty<FProperty>(UListView::StaticClass(), TEXT(\"ConsumeMouseWheel\"));",
		"static_cast<uint8>(EConsumeMouseWheel::Never)",
		"ListView->SetWheelScrollMultiplier(2.5f);",
		"ScrollAnimationProp->SetPropertyValue(ScrollAnimationProp->ContainerPtrToValuePtr<bool>(ListView), true);",
		"ListView->SetHorizontalEntrySpacing(12.0f);",
		"ListView->SetVerticalEntrySpacing(6.0f);",
		"ListView->SetScrollBarPadding(FMargin(1.0f, 2.0f, 3.0f, 4.0f));",
		"TileView->SetEntryWidth(180.0f);",
		"TileView->SetEntryHeight(96.0f);",
		"TreeView->SetSelectionMode(ESelectionMode::Multi);",
	}
	for _, needle := range required {
		if !strings.Contains(text, needle) {
			t.Fatalf("UE plugin list/tile/tree view write after-state implementation missing %q", needle)
		}
	}
}

func TestUEPluginScaleBoxWrapBoxAndWidgetSwitcherWriteOperationsHaveAfterStateImplementations(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("../../testdata/BPXFixtureGenerator/Source/BPXFixtureGenerator/Private/BPXGenerateFixturesCommandlet.cpp")
	if err != nil {
		t.Fatalf("read UE plugin source: %v", err)
	}
	text := string(body)

	required := []string{
		"ScaleBox->SetStretch(EStretch::ScaleToFit);",
		"ScaleBox->SetStretchDirection(EStretchDirection::DownOnly);",
		"ScaleBox->SetUserSpecifiedScale(1.25f);",
		"ScaleBox->SetIgnoreInheritedScale(true);",
		"WrapBox->SetWrapSize(480.0f);",
		"WrapBox->SetExplicitWrapSize(true);",
		"WrapBox->SetInnerSlotPadding(FVector2D(8.0f, 12.0f));",
		"WrapBox->SetOrientation(EOrientation::Orient_Vertical);",
		"WidgetSwitcher->SetActiveWidgetIndex(2);",
		"return TEXT(\"WBP_CanvasPanel_WidgetSwitcherChildren\");",
	}
	for _, needle := range required {
		if !strings.Contains(text, needle) {
			t.Fatalf("UE plugin ScaleBox/WrapBox/WidgetSwitcher after-state implementation missing %q", needle)
		}
	}
}

func TestUEPluginWrapperAndUniformGridWriteOperationsHaveAfterStateImplementations(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("../../testdata/BPXFixtureGenerator/Source/BPXFixtureGenerator/Private/BPXGenerateFixturesCommandlet.cpp")
	if err != nil {
		t.Fatalf("read UE plugin source: %v", err)
	}
	text := string(body)

	required := []string{
		"RetainerBox->SetRetainRendering(true);",
		"RetainerBox->RenderOnInvalidation = false;",
		"RetainerBox->RenderOnPhase = true;",
		"RetainerBox->Phase = 1;",
		"RetainerBox->PhaseCount = 3;",
		"BackgroundBlur->SetBlurStrength(16.0f);",
		"BackgroundBlur->SetApplyAlphaToBlur(true);",
		"SafeZone->PadLeft = false;",
		"SafeZone->PadRight = true;",
		"SafeZone->PadTop = false;",
		"SafeZone->PadBottom = true;",
		"InvalidationBox->SetCanCache(true);",
		"UniformGridPanel->SetMinDesiredSlotWidth(160.0f);",
		"UniformGridPanel->SetMinDesiredSlotHeight(48.0f);",
		"UniformGridPanel->SetSlotPadding(FMargin(4.0f, 6.0f, 8.0f, 10.0f));",
		"return TEXT(\"WBP_CanvasPanel_RetainerBox\");",
		"return TEXT(\"WBP_CanvasPanel_BackgroundBlur\");",
		"return TEXT(\"WBP_CanvasPanel_SafeZone\");",
		"return TEXT(\"WBP_CanvasPanel_InvalidationBox\");",
		"return TEXT(\"WBP_CanvasPanel_UniformGridPanel\");",
	}
	for _, needle := range required {
		if !strings.Contains(text, needle) {
			t.Fatalf("UE plugin wrapper/UniformGridPanel after-state implementation missing %q", needle)
		}
	}
}

func TestUEPluginWidgetOperationSpecsMissingGoldenFixturesAreExplicit(t *testing.T) {
	t.Parallel()

	pluginSpecs, err := uePluginGeneratedOperationNames()
	if err != nil {
		t.Fatalf("read UE plugin operation specs: %v", err)
	}

	goldenSpecs := map[string]struct{}{}
	for _, root := range goldenFixtureRoots(t, "operations") {
		entries, err := os.ReadDir(filepath.Join(root, "operations"))
		if err != nil {
			t.Fatalf("read operations dir: %v", err)
		}
		for _, opDir := range listOperationSpecDirs(entries, filepath.Join(root, "operations")) {
			goldenSpecs[filepath.Base(opDir)] = struct{}{}
		}
	}

	missing := make([]string, 0, 8)
	for _, name := range pluginSpecs {
		if !strings.HasPrefix(name, "widget_") {
			continue
		}
		if _, ok := goldenSpecs[name]; ok {
			continue
		}
		missing = append(missing, name)
	}

	expectedMissing := []string{}
	if !slices.Equal(missing, expectedMissing) {
		t.Fatalf("unexpected missing widget golden fixtures\nactual=%v\nexpected=%v", missing, expectedMissing)
	}
}

func uePluginGeneratedOperationNames() ([]string, error) {
	body, err := os.ReadFile("../../testdata/BPXFixtureGenerator/Source/BPXFixtureGenerator/Private/BPXGenerateFixturesCommandlet.cpp")
	if err != nil {
		return nil, err
	}
	text := string(body)
	specRe := regexp.MustCompile(`MakeOperation(?:WithErrorContains)?\(TEXT\("([^"]+)"\)`)
	matches := specRe.FindAllStringSubmatch(text, -1)
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			names = append(names, match[1])
		}
	}

	notYetGenerated, err := uePluginNotYetGeneratedOperationNames()
	if err != nil {
		return nil, err
	}
	notGenerated := make(map[string]struct{}, len(notYetGenerated))
	for _, name := range notYetGenerated {
		notGenerated[name] = struct{}{}
	}

	filtered := make([]string, 0, len(names))
	for _, name := range names {
		if _, blocked := notGenerated[name]; blocked {
			continue
		}
		filtered = append(filtered, name)
	}
	slices.Sort(filtered)
	return slices.Compact(filtered), nil
}

func uePluginNotYetGeneratedOperationNames() ([]string, error) {
	body, err := os.ReadFile("../../testdata/BPXFixtureGenerator/Source/BPXFixtureGenerator/Private/BPXGenerateFixturesCommandlet.cpp")
	if err != nil {
		return nil, err
	}
	text := string(body)

	names := make([]string, 0, 16)
	start := regexp.MustCompile(`bool IsNotYetGeneratedOperation\(const FOperationFixtureSpec& Spec\)\s*\{`).FindStringIndex(text)
	if start == nil {
		return names, nil
	}
	rest := text[start[1]:]
	end := regexp.MustCompile(`\n\}`).FindStringIndex(rest)
	if end == nil {
		return names, nil
	}
	fnBody := rest[:end[0]]
	nameRe := regexp.MustCompile(`Spec\.Name == TEXT\("([^"]+)"\)`)
	for _, match := range nameRe.FindAllStringSubmatch(fnBody, -1) {
		if len(match) > 1 {
			names = append(names, match[1])
		}
	}

	slices.Sort(names)
	return slices.Compact(names), nil
}
