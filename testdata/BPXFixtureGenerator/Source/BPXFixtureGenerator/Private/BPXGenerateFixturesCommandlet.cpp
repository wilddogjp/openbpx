#include "BPXGenerateFixturesCommandlet.h"
#include "BPXOperationFixtureActor.h"

#include "AssetRegistry/AssetRegistryModule.h"
#include "Dom/JsonObject.h"
#include "EdGraph/EdGraphPin.h"
#include "EdGraphSchema_K2.h"
#include "Editor.h"
#include "Engine/Blueprint.h"
#include "Engine/CompositeDataTable.h"
#include "Engine/DataTable.h"
#include "Engine/SCS_Node.h"
#include "Engine/SimpleConstructionScript.h"
#include "Blueprint/WidgetTree.h"
#include "Internationalization/StringTable.h"
#include "Engine/World.h"
#include "GameFramework/Actor.h"
#include "HAL/FileManager.h"
#include "HAL/PlatformFileManager.h"
#include "Internationalization/StringTableCore.h"
#include "IO/IoHash.h"
#include "Kismet2/BlueprintEditorUtils.h"
#include "Kismet2/EnumEditorUtils.h"
#include "Kismet2/KismetEditorUtilities.h"
#include "Kismet2/StructureEditorUtils.h"
#include "Materials/Material.h"
#include "Materials/MaterialInstanceConstant.h"
#include "Misc/CommandLine.h"
#include "Misc/EngineVersion.h"
#include "Misc/FileHelper.h"
#include "Misc/PackageName.h"
#include "Misc/Paths.h"
#include "Serialization/JsonSerializer.h"
#include "Serialization/JsonWriter.h"
#include "UObject/Package.h"
#include "UObject/MetaData.h"
#include "UObject/SavePackage.h"
#include "UObject/UnrealType.h"
#include "WidgetBlueprint.h"
#include "WidgetBlueprintEditorUtils.h"
#include "Components/Border.h"
#include "Components/BackgroundBlur.h"
#include "Components/Button.h"
#include "Components/CanvasPanel.h"
#include "Components/CanvasPanelSlot.h"
#include "Components/CheckBox.h"
#include "Components/ComboBoxString.h"
#include "Components/EditableText.h"
#include "Components/EditableTextBox.h"
#include "Components/HorizontalBox.h"
#include "Components/InvalidationBox.h"
#include "Components/ContentWidget.h"
#include "Components/Image.h"
#include "Components/ListView.h"
#include "Components/NamedSlot.h"
#include "Components/Overlay.h"
#include "Components/PanelWidget.h"
#include "Components/ProgressBar.h"
#include "Components/RetainerBox.h"
#include "Components/RichTextBlock.h"
#include "Components/SafeZone.h"
#include "Components/ScrollBar.h"
#include "Components/ScrollBox.h"
#include "Components/SizeBox.h"
#include "Components/ScaleBox.h"
#include "Components/Slider.h"
#include "Components/SpinBox.h"
#include "Components/Spacer.h"
#include "Components/TextBlock.h"
#include "Components/TileView.h"
#include "Components/TreeView.h"
#include "Components/MultiLineEditableTextBox.h"
#include "Components/UniformGridPanel.h"
#include "Components/VerticalBox.h"
#include "Components/WidgetSwitcher.h"
#include "Components/WrapBox.h"
#include "Blueprint/UserWidget.h"
#include "Engine/Texture2D.h"
#include "StructUtils/UserDefinedStruct.h"
#include "Factories/WorldFactory.h"

DEFINE_LOG_CATEGORY_STATIC(LogBPXFixtureGenerator, Log, All);

namespace
{
enum class EParseFixtureKind
{
    Blueprint,
    WidgetBlueprint,
    DataTable,
    UserEnum,
    UserStruct,
    StringTable,
    MaterialInstance,
    Level
};

struct FParseFixtureSpec
{
    FString Key;
    FString FileName;
    EParseFixtureKind Kind;
    FString ParentKey;
};

struct FOperationFixtureSpec
{
    FString Name;
    FString Command;
    FString ArgsJson;
    FString UEProcedure;
    FString Expect;
    FString ErrorContains;
    FString Notes;
    FString IgnorePackageSectionsJson;
};

struct FOperationBlueprintDefaults
{
    FString BeforeFixtureValue;
    FString AfterFixtureValue;
};

struct FNameFixtureEntry
{
    FString Value;
    uint16 NonCaseHash = 0;
    uint16 CasePreservingHash = 0;
};

struct FNameFixtureSummaryOffsetField
{
    FString Name;
    int64 Pos = 0;
    int32 Size = 0;
};

struct FNameFixtureExportFieldPatch
{
    int64 SerialSizePos = INDEX_NONE;
    int64 SerialOffsetPos = INDEX_NONE;
    int64 ScriptStartPos = INDEX_NONE;
    int64 ScriptEndPos = INDEX_NONE;
};

struct FNameFixtureSummaryInfo
{
    int32 LegacyVersion = 0;
    int32 FileVersionUE4 = 0;
    int32 FileVersionUE5 = 0;
    int32 FileVersionLicenseeUE = 0;
    int64 SavedHashPos = INDEX_NONE;
    int64 NameCountPos = INDEX_NONE;
    int64 NameOffsetPos = INDEX_NONE;
    int64 NamesReferencedFromExportDataCountPos = INDEX_NONE;
    int32 NameCount = 0;
    int32 NameOffset = 0;
    int32 ExportCount = 0;
    int32 ExportOffset = 0;
    int32 PackageFlags = 0;
    int32 SoftObjectPathsOffset = 0;
    int32 GatherableTextDataOffset = 0;
    int32 MetaDataOffset = 0;
    int32 ImportOffset = 0;
    int32 ExportMapOffset = 0;
    int32 CellImportOffset = 0;
    int32 CellExportOffset = 0;
    int32 DependsOffset = 0;
    int32 SoftPackageReferencesOffset = 0;
    int32 SearchableNamesOffset = 0;
    int32 ThumbnailTableOffset = 0;
    int32 ImportTypeHierarchiesOffset = 0;
    int32 AssetRegistryDataOffset = 0;
    int32 PreloadDependencyOffset = 0;
    int32 DataResourceOffset = 0;
    int64 BulkDataStartOffset = 0;
    int64 PayloadTOCOffset = 0;
    int32 TotalHeaderSize = 0;
    TArray<int64> GenerationNameCountPos;
    TArray<FNameFixtureSummaryOffsetField> OffsetFields;
};

void AddBlueprintMemberVariable(UBlueprint* Blueprint, const FName& VariableName, const FEdGraphPinType& PinType, const FString& DefaultValue);

constexpr uint32 BPXPackageFileTag = 0x9E2A83C1u;
constexpr int32 BPXUE5NamesFromExportData = 1001;
constexpr int32 BPXUE5PayloadTOC = 1002;
constexpr int32 BPXUE5OptionalResources = 1003;
constexpr int32 BPXUE5RemoveObjectExportPkgGUID = 1005;
constexpr int32 BPXUE5TrackObjectExportInherited = 1006;
constexpr int32 BPXUE5AddSoftObjectPathList = 1008;
constexpr int32 BPXUE5DataResources = 1009;
constexpr int32 BPXUE5ScriptSerializationOffset = 1010;
constexpr int32 BPXUE5MetadataSerializationOff = 1014;
constexpr int32 BPXUE5VerseCells = 1015;
constexpr int32 BPXUE5PackageSavedHash = 1016;
constexpr int32 BPXUE5ImportTypeHierarchies = 1018;
constexpr int32 BPXUE4VersionUE56 = 522;
constexpr uint32 BPXPkgFlagFilterEditorOnly = 0x80000000u;

FString NormalizeToken(const FString& InToken)
{
    FString Token = InToken;
    Token.TrimStartAndEndInline();
    while (Token.EndsWith(TEXT("/")) || Token.EndsWith(TEXT("\\")))
    {
        Token.LeftChopInline(1, EAllowShrinking::No);
    }

    if (Token.EndsWith(TEXT(".uasset"), ESearchCase::IgnoreCase))
    {
        Token.LeftChopInline(7, EAllowShrinking::No);
    }
    else if (Token.EndsWith(TEXT(".umap"), ESearchCase::IgnoreCase))
    {
        Token.LeftChopInline(5, EAllowShrinking::No);
    }

    Token.ToLowerInline();
    return Token;
}

TSet<FString> ParseCsvSet(const FString& Csv)
{
    TSet<FString> Result;
    FString NormalizedCsv = Csv;
    NormalizedCsv.ReplaceInline(TEXT(";"), TEXT(","));

    TArray<FString> Tokens;
    NormalizedCsv.ParseIntoArray(Tokens, TEXT(","), true);
    for (const FString& Token : Tokens)
    {
        const FString Normalized = NormalizeToken(Token);
        if (!Normalized.IsEmpty())
        {
            Result.Add(Normalized);
        }
    }

    return Result;
}

TArray<FParseFixtureSpec> BuildParseSpecs()
{
    return {
        {TEXT("BP_Empty"), TEXT("BP_Empty.uasset"), EParseFixtureKind::Blueprint, TEXT("")},
        {TEXT("BP_SimpleVars"), TEXT("BP_SimpleVars.uasset"), EParseFixtureKind::Blueprint, TEXT("")},
        {TEXT("BP_AllScalarTypes"), TEXT("BP_AllScalarTypes.uasset"), EParseFixtureKind::Blueprint, TEXT("")},
        {TEXT("BP_MathTypes"), TEXT("BP_MathTypes.uasset"), EParseFixtureKind::Blueprint, TEXT("")},
        {TEXT("BP_RefTypes"), TEXT("BP_RefTypes.uasset"), EParseFixtureKind::Blueprint, TEXT("")},
        {TEXT("BP_Containers"), TEXT("BP_Containers.uasset"), EParseFixtureKind::Blueprint, TEXT("")},
        {TEXT("BP_Nested"), TEXT("BP_Nested.uasset"), EParseFixtureKind::Blueprint, TEXT("")},
        {TEXT("BP_GameplayTags"), TEXT("BP_GameplayTags.uasset"), EParseFixtureKind::Blueprint, TEXT("")},
        {TEXT("BP_WithFunctions"), TEXT("BP_WithFunctions.uasset"), EParseFixtureKind::Blueprint, TEXT("")},
        {TEXT("BP_Base"), TEXT("BP_Base.uasset"), EParseFixtureKind::Blueprint, TEXT("")},
        {TEXT("BP_Mid"), TEXT("BP_Mid.uasset"), EParseFixtureKind::Blueprint, TEXT("BP_Base")},
        {TEXT("BP_Child"), TEXT("BP_Child.uasset"), EParseFixtureKind::Blueprint, TEXT("BP_Mid")},
        {TEXT("BP_SoftRefs"), TEXT("BP_SoftRefs.uasset"), EParseFixtureKind::Blueprint, TEXT("")},
        {TEXT("BP_ManyImports"), TEXT("BP_ManyImports.uasset"), EParseFixtureKind::Blueprint, TEXT("")},
        {TEXT("BP_WithMetadata"), TEXT("BP_WithMetadata.uasset"), EParseFixtureKind::Blueprint, TEXT("")},
        {TEXT("BP_Unicode"), TEXT("BP_Unicode.uasset"), EParseFixtureKind::Blueprint, TEXT("")},
        {TEXT("BP_LargeArray"), TEXT("BP_LargeArray.uasset"), EParseFixtureKind::Blueprint, TEXT("")},
        {TEXT("BP_Empty_StringTableRef"), TEXT("BP_Empty_StringTableRef.uasset"), EParseFixtureKind::Blueprint, TEXT("")},
        {TEXT("WBP_Minimum"), TEXT("WBP_Minimum.uasset"), EParseFixtureKind::WidgetBlueprint, TEXT("")},
        {TEXT("WBP_TextBlock"), TEXT("WBP_TextBlock.uasset"), EParseFixtureKind::WidgetBlueprint, TEXT("")},
        {TEXT("WBP_RichTextBlock"), TEXT("WBP_RichTextBlock.uasset"), EParseFixtureKind::WidgetBlueprint, TEXT("")},
        {TEXT("WBP_CanvasPanel"), TEXT("WBP_CanvasPanel.uasset"), EParseFixtureKind::WidgetBlueprint, TEXT("")},
        {TEXT("WBP_CanvasPanel_TextBlock"), TEXT("WBP_CanvasPanel_TextBlock.uasset"), EParseFixtureKind::WidgetBlueprint, TEXT("")},
        {TEXT("WBP_CanvasPanel_RichTextBlock"), TEXT("WBP_CanvasPanel_RichTextBlock.uasset"), EParseFixtureKind::WidgetBlueprint, TEXT("")},
        {TEXT("WBP_Overlay"), TEXT("WBP_Overlay.uasset"), EParseFixtureKind::WidgetBlueprint, TEXT("")},
        {TEXT("WBP_Overlay_TextBlock"), TEXT("WBP_Overlay_TextBlock.uasset"), EParseFixtureKind::WidgetBlueprint, TEXT("")},
        {TEXT("DT_Simple"), TEXT("DT_Simple.uasset"), EParseFixtureKind::DataTable, TEXT("")},
        {TEXT("DT_Complex"), TEXT("DT_Complex.uasset"), EParseFixtureKind::DataTable, TEXT("")},
        {TEXT("E_Direction"), TEXT("E_Direction.uasset"), EParseFixtureKind::UserEnum, TEXT("")},
        {TEXT("S_PlayerData"), TEXT("S_PlayerData.uasset"), EParseFixtureKind::UserStruct, TEXT("")},
        {TEXT("ST_UI"), TEXT("ST_UI.uasset"), EParseFixtureKind::StringTable, TEXT("")},
        {TEXT("L_Minimal"), TEXT("L_Minimal.umap"), EParseFixtureKind::Level, TEXT("")},
        {TEXT("MI_Chrome"), TEXT("MI_Chrome.uasset"), EParseFixtureKind::MaterialInstance, TEXT("")},
        {TEXT("BP_CustomVersions"), TEXT("BP_CustomVersions.uasset"), EParseFixtureKind::Blueprint, TEXT("")},
        {TEXT("BP_WithThumbnail"), TEXT("BP_WithThumbnail.uasset"), EParseFixtureKind::Blueprint, TEXT("")},
        {TEXT("BP_DependsMap"), TEXT("BP_DependsMap.uasset"), EParseFixtureKind::Blueprint, TEXT("")}
    };
}

TArray<FOperationFixtureSpec> BuildOperationSpecs()
{
    auto MakeOperation = [](
        const TCHAR* Name,
        const TCHAR* Command,
        const TCHAR* ArgsJson,
        const TCHAR* UEProcedure,
        const TCHAR* Expect,
        const TCHAR* Notes,
        const TCHAR* IgnorePackageSectionsJson = TEXT("")
    ) {
        return FOperationFixtureSpec{
            FString(Name),
            FString(Command),
            FString(ArgsJson),
            FString(UEProcedure),
            FString(Expect),
            FString(),
            FString(Notes),
            FString(IgnorePackageSectionsJson)
        };
    };

    auto MakeOperationWithErrorContains = [](
        const TCHAR* Name,
        const TCHAR* Command,
        const TCHAR* ArgsJson,
        const TCHAR* UEProcedure,
        const TCHAR* Expect,
        const TCHAR* ErrorContains,
        const TCHAR* Notes
    ) {
        return FOperationFixtureSpec{
            FString(Name),
            FString(Command),
            FString(ArgsJson),
            FString(UEProcedure),
            FString(Expect),
            FString(ErrorContains),
            FString(Notes)
        };
    };

    return {
        MakeOperation(TEXT("prop_add"), TEXT("prop add"), TEXT("{\"export\":5,\"spec\":\"{\\\"name\\\":\\\"bCanBeDamaged\\\",\\\"type\\\":\\\"BoolProperty\\\",\\\"value\\\":false}\"}"), TEXT("Add bCanBeDamaged override tag by changing default true -> false"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_add_fixture_int"), TEXT("prop add"), TEXT("{\"export\":5,\"spec\":\"{\\\"name\\\":\\\"FixtureInt\\\",\\\"type\\\":\\\"IntProperty\\\",\\\"value\\\":42}\"}"), TEXT("Add FixtureInt override tag by changing default 0 -> 42"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_remove"), TEXT("prop remove"), TEXT("{\"export\":5,\"path\":\"bCanBeDamaged\"}"), TEXT("Remove bCanBeDamaged override tag by changing default false -> true"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_remove_fixture_int"), TEXT("prop remove"), TEXT("{\"export\":5,\"path\":\"FixtureInt\"}"), TEXT("Remove FixtureInt override tag by changing default 42 -> 0"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_bool"), TEXT("prop set"), TEXT("{\"export\":3,\"path\":\"VBool\",\"value\":\"false\"}"), TEXT("Toggle BoolProperty default on scalar fixture"), TEXT("byte_equal"), TEXT("Validated on ue5.6/ue5.7 scalar fixture roots.")),
        MakeOperation(TEXT("prop_set_int"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"MyStr\",\"value\":\"\\\"changed\\\"\"}"), TEXT("Update CDO string property with prop set"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_int_negative"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureInt\",\"value\":\"-1\"}"), TEXT("Set int variable to negative value"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_int_max"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureInt\",\"value\":\"2147483647\"}"), TEXT("Set int variable to int32 max"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_int_min"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureInt\",\"value\":\"-2147483648\"}"), TEXT("Set int variable to int32 min"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_int64"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureInt64\",\"value\":\"9223372036854775807\"}"), TEXT("Set int64 variable"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_float"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureFloat\",\"value\":\"3.14\"}"), TEXT("Set float variable"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_float_special"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureFloat\",\"value\":\"1e-38\"}"), TEXT("Set float variable to special near-subnormal value"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_double"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureDouble\",\"value\":\"2.718281828\"}"), TEXT("Set double variable"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_string_same_len"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"MyStr\",\"value\":\"\\\"World\\\"\"}"), TEXT("Set string variable to same length value"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_string_diff_len"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"MyStr\",\"value\":\"\\\"Hello World\\\"\"}"), TEXT("Set string variable to different length value"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_string_empty"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"MyStr\",\"value\":\"\\\"\\\"\"}"), TEXT("Set string variable to empty"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_string_unicode"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"MyStr\",\"value\":\"\\\"テスト\\\"\"}"), TEXT("Set string variable to unicode"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_string_long_expand"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"MyStr\",\"value\":\"\\\"Lorem ipsum dolor sit amet, consectetur adipiscing elit 0123456789\\\"\"}"), TEXT("Set string variable to significantly longer ASCII text"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_string_shrink"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"MyStr\",\"value\":\"\\\"x\\\"\"}"), TEXT("Set string variable to shorter ASCII text"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_name"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureName\",\"value\":\"\\\"BoolProperty\\\"\"}"), TEXT("Set Name variable"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_text"), TEXT("prop set"), TEXT("{\"export\":11,\"path\":\"CategoryName\",\"value\":\"\\\"Gameplay\\\"\"}"), TEXT("Update TextProperty CategoryName on SCS node"), TEXT("byte_equal"), TEXT("Validated on ue5.6/ue5.7 scalar fixture roots.")),
        MakeOperation(TEXT("widget_write_text_root_textblock"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"TextBlock_72\",\"property\":\"text\",\"value\":\"Updated root text\"}"), TEXT("Update the root TextBlock text on WBP_TextBlock"), TEXT("byte_equal"), TEXT("Covers root TextBlock text rewrite across designer/generated WidgetTree exports."), TEXT("[\"editor-thumbnails\",\"widget-text-localization-keys\"]")),
        MakeOperation(TEXT("widget_write_text_color_root_textblock"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"TextBlock_72\",\"property\":\"text-color\",\"value\":\"0.15,0.45,0.75,0.9\"}"), TEXT("Set ColorAndOpacity on the root TextBlock on WBP_TextBlock"), TEXT("byte_equal"), TEXT("Covers root TextBlock SlateColor rewrite across designer/generated WidgetTree exports."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_text_font_size_root_textblock"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"TextBlock_72\",\"property\":\"text-font-size\",\"value\":\"28\"}"), TEXT("Set Font.Size on the root TextBlock on WBP_TextBlock"), TEXT("byte_equal"), TEXT("Covers root TextBlock font-size rewrite inside SlateFontInfo."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_text_justification_root_textblock"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"TextBlock_72\",\"property\":\"text-justification\",\"value\":\"Center\"}"), TEXT("Set Justification to Center on the root TextBlock on WBP_TextBlock"), TEXT("byte_equal"), TEXT("Covers root TextBlock justification rewrite across designer/generated WidgetTree exports."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_text_font_root_textblock"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"TextBlock_72\",\"property\":\"text-font\",\"value\":\"/Game/UI/Foundation/Fonts/NotoSans\"}"), TEXT("Set Font.FontObject on the root TextBlock on WBP_TextBlock"), TEXT("byte_equal"), TEXT("Covers root TextBlock font-object rewrite inside SlateFontInfo."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"soft-package-references\",\"soft-package-searchable-summary-offsets\"]")),
        MakeOperation(TEXT("widget_write_text_typeface_root_textblock"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"TextBlock_72\",\"property\":\"text-typeface\",\"value\":\"Bold\"}"), TEXT("Set Font.TypefaceFontName to Bold on the root TextBlock on WBP_TextBlock"), TEXT("byte_equal"), TEXT("Covers root TextBlock typeface rewrite inside SlateFontInfo."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_text_auto_wrap_text_root_textblock"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"TextBlock_72\",\"property\":\"text-auto-wrap-text\",\"value\":\"true\"}"), TEXT("Enable AutoWrapText on the root TextBlock on WBP_TextBlock"), TEXT("byte_equal"), TEXT("Covers root TextBlock AutoWrapText bool insertion across designer/generated WidgetTree exports."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_text_wrap_text_at_root_textblock"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"TextBlock_72\",\"property\":\"text-wrap-text-at\",\"value\":\"320\"}"), TEXT("Set WrapTextAt to 320 on the root TextBlock on WBP_TextBlock"), TEXT("byte_equal"), TEXT("Covers root TextBlock WrapTextAt float insertion across designer/generated WidgetTree exports."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_text_line_height_percentage_root_textblock"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"TextBlock_72\",\"property\":\"text-line-height-percentage\",\"value\":\"1.25\"}"), TEXT("Set LineHeightPercentage to 1.25 on the root TextBlock on WBP_TextBlock"), TEXT("byte_equal"), TEXT("Covers root TextBlock LineHeightPercentage float insertion across designer/generated WidgetTree exports."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_text_shadow_offset_root_textblock"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"TextBlock_72\",\"property\":\"text-shadow-offset\",\"value\":\"3,4\"}"), TEXT("Set ShadowOffset to 3x4 on the root TextBlock on WBP_TextBlock"), TEXT("byte_equal"), TEXT("Covers root TextBlock shadow-offset rewrite inside the text style payload."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_text_shadow_color_root_textblock"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"TextBlock_72\",\"property\":\"text-shadow-color-and-opacity\",\"value\":\"0.05,0.1,0.15,0.8\"}"), TEXT("Set ShadowColorAndOpacity on the root TextBlock on WBP_TextBlock"), TEXT("byte_equal"), TEXT("Covers root TextBlock shadow-color rewrite inside the text style payload."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_text_outline_size_root_textblock"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"TextBlock_72\",\"property\":\"text-outline-size\",\"value\":\"2\"}"), TEXT("Set Font.OutlineSettings.OutlineSize on the root TextBlock on WBP_TextBlock"), TEXT("byte_equal"), TEXT("Covers root TextBlock outline-size rewrite inside FontOutlineSettings."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_text_outline_color_root_textblock"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"TextBlock_72\",\"property\":\"text-outline-color\",\"value\":\"0.2,0.3,0.9,1\"}"), TEXT("Set Font.OutlineSettings.OutlineColor on the root TextBlock on WBP_TextBlock"), TEXT("byte_equal"), TEXT("Covers root TextBlock outline-color rewrite inside FontOutlineSettings."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_text_canvaspanel_child"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_22/TextBlock_31\",\"property\":\"text\",\"value\":\"Updated canvas child text\"}"), TEXT("Update the child TextBlock text under the root CanvasPanel on WBP_CanvasPanel_TextBlock"), TEXT("byte_equal"), TEXT("Covers child TextBlock text rewrite under CanvasPanel across designer/generated WidgetTree exports."), TEXT("[\"editor-thumbnails\",\"widget-text-localization-keys\"]")),
        MakeOperation(TEXT("widget_write_text_overlay_child"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"Overlay_116/TextBlock_36\",\"property\":\"text\",\"value\":\"Updated overlay child text\"}"), TEXT("Update the child TextBlock text under the root Overlay on WBP_Overlay_TextBlock"), TEXT("byte_equal"), TEXT("Covers child TextBlock text rewrite under Overlay across designer/generated WidgetTree exports."), TEXT("[\"editor-thumbnails\",\"widget-text-localization-keys\"]")),
        MakeOperation(TEXT("widget_write_text_root_richtextblock"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"RichTextBlock_72\",\"property\":\"text\",\"value\":\"Updated root rich text\"}"), TEXT("Update the root RichTextBlock text on WBP_RichTextBlock"), TEXT("byte_equal"), TEXT("Covers root RichTextBlock text rewrite across designer/generated WidgetTree exports."), TEXT("[\"editor-thumbnails\",\"widget-text-localization-keys\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_text_canvaspanel_child_richtextblock"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_22/RichTextBlock_31\",\"property\":\"text\",\"value\":\"Updated rich canvas child text\"}"), TEXT("Update the child RichTextBlock text under the root CanvasPanel on WBP_CanvasPanel_RichTextBlock"), TEXT("byte_equal"), TEXT("Covers child RichTextBlock text rewrite under CanvasPanel across designer/generated WidgetTree exports."), TEXT("[\"editor-thumbnails\",\"widget-text-localization-keys\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_add_image_canvaspanel"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_22\",\"type\":\"image\",\"name\":\"Image_23\"}"), TEXT("Add one bare Image child under the root CanvasPanel on WBP_CanvasPanel"), TEXT("byte_equal"), TEXT("Covers widget-add v1 for an empty root CanvasPanel: inserts designer/generated Image + CanvasPanelSlot exports and updates WidgetTree/widget-variable references."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\"]")),
        MakeOperation(TEXT("widget_add_richtextblock_canvaspanel"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_22\",\"type\":\"richtextblock\",\"name\":\"RichTextBlock_23\"}"), TEXT("Add one bare RichTextBlock child under the root CanvasPanel on WBP_CanvasPanel"), TEXT("byte_equal"), TEXT("Covers widget-add for a bare RichTextBlock child under CanvasPanel, including generated-pair insertion for follow-up text writes."), TEXT("[\"editor-thumbnails\",\"widget-text-key-only\",\"widget-variable-guid-map-values\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_add_progressbar_canvaspanel"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_22\",\"type\":\"progressbar\",\"name\":\"ProgressBar_23\"}"), TEXT("Add one bare ProgressBar child under the root CanvasPanel on WBP_CanvasPanel"), TEXT("byte_equal"), TEXT("Validated on ue5.7 ProgressBar widget-add fixture pair."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_add_slider_canvaspanel"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_22\",\"type\":\"slider\",\"name\":\"Slider_23\"}"), TEXT("Add one bare Slider child under the root CanvasPanel on WBP_CanvasPanel"), TEXT("byte_equal"), TEXT("Validated on ue5.7 Slider widget-add fixture pair."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_add_spacer_canvaspanel"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_22\",\"type\":\"spacer\",\"name\":\"Spacer_23\"}"), TEXT("Add one bare Spacer child under the root CanvasPanel on WBP_CanvasPanel"), TEXT("byte_equal"), TEXT("Validated on ue5.7 Spacer widget-add fixture pair."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_add_scrollbar_canvaspanel"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_22\",\"type\":\"scrollbar\",\"name\":\"ScrollBar_23\"}"), TEXT("Add one bare ScrollBar child under the root CanvasPanel on WBP_CanvasPanel"), TEXT("byte_equal"), TEXT("Validated on ue5.7 ScrollBar widget-add fixture pair."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_add_editabletext_canvaspanel"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_22\",\"type\":\"editabletext\",\"name\":\"EditableText_23\"}"), TEXT("Add one bare EditableText child under the root CanvasPanel on WBP_CanvasPanel"), TEXT("byte_equal"), TEXT("Covers widget-add for EditableText under CanvasPanel."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_add_editabletextbox_canvaspanel"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_22\",\"type\":\"editabletextbox\",\"name\":\"EditableTextBox_23\"}"), TEXT("Add one bare EditableTextBox child under the root CanvasPanel on WBP_CanvasPanel"), TEXT("byte_equal"), TEXT("Covers widget-add for EditableTextBox under CanvasPanel."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_add_multilineeditabletextbox_canvaspanel"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_22\",\"type\":\"multilineeditabletextbox\",\"name\":\"MultiLineEditableTextBox_23\"}"), TEXT("Add one bare MultiLineEditableTextBox child under the root CanvasPanel on WBP_CanvasPanel"), TEXT("byte_equal"), TEXT("Covers widget-add for MultiLineEditableTextBox under CanvasPanel."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_add_spinbox_canvaspanel"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_22\",\"type\":\"spinbox\",\"name\":\"SpinBox_23\"}"), TEXT("Add one bare SpinBox child under the root CanvasPanel on WBP_CanvasPanel"), TEXT("byte_equal"), TEXT("Covers widget-add for SpinBox under CanvasPanel."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_add_comboboxstring_canvaspanel"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_22\",\"type\":\"comboboxstring\",\"name\":\"ComboBoxString_23\"}"), TEXT("Add one bare ComboBoxString child under the root CanvasPanel on WBP_CanvasPanel"), TEXT("byte_equal"), TEXT("Covers widget-add for ComboBoxString under CanvasPanel."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_add_listview_canvaspanel"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_22\",\"type\":\"listview\",\"name\":\"ListView_23\"}"), TEXT("Add one bare ListView child under the root CanvasPanel on WBP_CanvasPanel"), TEXT("byte_equal"), TEXT("Covers widget-add for ListView under CanvasPanel."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_add_tileview_canvaspanel"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_22\",\"type\":\"tileview\",\"name\":\"TileView_23\"}"), TEXT("Add one bare TileView child under the root CanvasPanel on WBP_CanvasPanel"), TEXT("byte_equal"), TEXT("Covers widget-add for TileView under CanvasPanel."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_add_treeview_canvaspanel"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_22\",\"type\":\"treeview\",\"name\":\"TreeView_23\"}"), TEXT("Add one bare TreeView child under the root CanvasPanel on WBP_CanvasPanel"), TEXT("byte_equal"), TEXT("Covers widget-add for TreeView under CanvasPanel."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_add_namedslot_canvaspanel"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_22\",\"type\":\"namedslot\",\"name\":\"NamedSlot_23\"}"), TEXT("Add one bare NamedSlot child under the root CanvasPanel on WBP_CanvasPanel"), TEXT("byte_equal"), TEXT("Covers widget-add for NamedSlot under CanvasPanel."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_add_textblock_namedslot_canvaspanel"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_22/NamedSlot_1\",\"type\":\"textblock\",\"name\":\"TextBlock_1\"}"), TEXT("Add one TextBlock child under the NamedSlot child on WBP_CanvasPanel_NamedSlot"), TEXT("byte_equal"), TEXT("Covers widget-add for a single content child under NamedSlot."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_add_userwidget_canvaspanel"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_22\",\"type\":\"userwidget\",\"class\":\"/Game/BPXFixtures/Parse/WBP_TextBlock\",\"name\":\"WBP_TextBlock_1\"}"), TEXT("Add one WBP_TextBlock child widget under the root CanvasPanel on WBP_CanvasPanel"), TEXT("byte_equal"), TEXT("Covers widget-add for a child WidgetBlueprint instance under CanvasPanel, including BlueprintGeneratedClass import wiring."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_parent_class_commonactivatablewidget_rootless"), TEXT("blueprint widget-parent-class"), TEXT("{\"class\":\"/Script/CommonUI.CommonActivatableWidget\"}"), TEXT("Replace the rootless WBP_Minimum parent class with CommonActivatableWidget"), TEXT("byte_equal"), TEXT("Covers native WidgetBlueprint parent-class rewrites while the blueprint is still rootless."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_remove_textblock_canvaspanel"), TEXT("blueprint widget-remove"), TEXT("{\"widget\":\"CanvasPanel_22/TextBlock_31\"}"), TEXT("Remove the bare TextBlock child under the root CanvasPanel on WBP_CanvasPanel_TextBlock"), TEXT("byte_equal"), TEXT("Covers widget-remove for a non-root TextBlock leaf under CanvasPanel, including orphan export/import compaction."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_remove_richtextblock_canvaspanel"), TEXT("blueprint widget-remove"), TEXT("{\"widget\":\"CanvasPanel_22/RichTextBlock_31\"}"), TEXT("Remove the bare RichTextBlock child under the root CanvasPanel on WBP_CanvasPanel_RichTextBlock"), TEXT("byte_equal"), TEXT("Covers widget-remove for a non-root RichTextBlock leaf under CanvasPanel, including orphan export/import compaction."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_remove_textblock_overlay"), TEXT("blueprint widget-remove"), TEXT("{\"widget\":\"Overlay_116/TextBlock_36\"}"), TEXT("Remove the bare TextBlock child under the root Overlay on WBP_Overlay_TextBlock"), TEXT("byte_equal"), TEXT("Covers widget-remove for a non-root TextBlock leaf under Overlay, including orphan export/import compaction."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_remove_image_canvaspanel"), TEXT("blueprint widget-remove"), TEXT("{\"widget\":\"CanvasPanel_1/Image_1\"}"), TEXT("Remove the bare Image child under the root CanvasPanel on WBP_CanvasPanel_Image"), TEXT("byte_equal"), TEXT("Covers widget-remove for a non-root Image leaf under CanvasPanel, including the designer-only Image export plus orphan slot compaction."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_remove_image_overlay"), TEXT("blueprint widget-remove"), TEXT("{\"widget\":\"Overlay_1/Image_1\"}"), TEXT("Remove the bare Image child under the root Overlay on WBP_Overlay_Image"), TEXT("byte_equal"), TEXT("Covers widget-remove for a non-root Image leaf under Overlay, including orphan slot and variable cleanup."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_remove_image_verticalbox"), TEXT("blueprint widget-remove"), TEXT("{\"widget\":\"VerticalBox_1/Image_1\"}"), TEXT("Remove the bare Image child under the root VerticalBox on WBP_VerticalBox_Image"), TEXT("byte_equal"), TEXT("Covers widget-remove for a non-root Image leaf under VerticalBox, including slot-array cleanup and variable compaction."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_remove_image_horizontalbox"), TEXT("blueprint widget-remove"), TEXT("{\"widget\":\"HorizontalBox_1/Image_1\"}"), TEXT("Remove the bare Image child under the root HorizontalBox on WBP_HorizontalBox_Image"), TEXT("byte_equal"), TEXT("Covers widget-remove for a non-root Image leaf under HorizontalBox, including slot-array cleanup and variable compaction."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_remove_button_canvaspanel"), TEXT("blueprint widget-remove"), TEXT("{\"widget\":\"CanvasPanel_1/Button_1\"}"), TEXT("Remove the bare Button child under the root CanvasPanel on WBP_CanvasPanel_Button"), TEXT("byte_equal"), TEXT("Covers widget-remove for a non-root Button leaf under CanvasPanel, including CanvasPanelSlot cleanup and variable compaction."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_remove_border_canvaspanel"), TEXT("blueprint widget-remove"), TEXT("{\"widget\":\"CanvasPanel_1/Border_1\"}"), TEXT("Remove the bare Border child under the root CanvasPanel on WBP_CanvasPanel_Border"), TEXT("byte_equal"), TEXT("Covers widget-remove for a non-root Border leaf under CanvasPanel, including CanvasPanelSlot cleanup and variable compaction."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_remove_sizebox_canvaspanel"), TEXT("blueprint widget-remove"), TEXT("{\"widget\":\"CanvasPanel_1/SizeBox_1\"}"), TEXT("Remove the bare SizeBox child under the root CanvasPanel on WBP_CanvasPanel_SizeBox"), TEXT("byte_equal"), TEXT("Covers widget-remove for a non-root SizeBox leaf under CanvasPanel, including CanvasPanelSlot cleanup and variable compaction."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_remove_gridpanel_canvaspanel"), TEXT("blueprint widget-remove"), TEXT("{\"widget\":\"CanvasPanel_1/GridPanel_1\"}"), TEXT("Remove the bare GridPanel child under the root CanvasPanel on WBP_CanvasPanel_GridPanel"), TEXT("byte_equal"), TEXT("Covers widget-remove for a non-root GridPanel leaf under CanvasPanel, including CanvasPanelSlot cleanup and variable compaction."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_remove_backgroundblur_canvaspanel"), TEXT("blueprint widget-remove"), TEXT("{\"widget\":\"CanvasPanel_1/BackgroundBlur_1\"}"), TEXT("Remove the bare BackgroundBlur child under the root CanvasPanel on WBP_CanvasPanel_BackgroundBlur"), TEXT("byte_equal"), TEXT("Covers widget-remove for a non-root BackgroundBlur leaf under CanvasPanel, including CanvasPanelSlot cleanup and variable compaction."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_remove_invalidationbox_canvaspanel"), TEXT("blueprint widget-remove"), TEXT("{\"widget\":\"CanvasPanel_1/InvalidationBox_1\"}"), TEXT("Remove the bare InvalidationBox child under the root CanvasPanel on WBP_CanvasPanel_InvalidationBox"), TEXT("byte_equal"), TEXT("Covers widget-remove for a non-root InvalidationBox leaf under CanvasPanel, including CanvasPanelSlot cleanup and variable compaction."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_remove_retainerbox_canvaspanel"), TEXT("blueprint widget-remove"), TEXT("{\"widget\":\"CanvasPanel_1/RetainerBox_1\"}"), TEXT("Remove the bare RetainerBox child under the root CanvasPanel on WBP_CanvasPanel_RetainerBox"), TEXT("byte_equal"), TEXT("Covers widget-remove for a non-root RetainerBox leaf under CanvasPanel, including CanvasPanelSlot cleanup and variable compaction."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_remove_safezone_canvaspanel"), TEXT("blueprint widget-remove"), TEXT("{\"widget\":\"CanvasPanel_1/SafeZone_1\"}"), TEXT("Remove the bare SafeZone child under the root CanvasPanel on WBP_CanvasPanel_SafeZone"), TEXT("byte_equal"), TEXT("Covers widget-remove for a non-root SafeZone leaf under CanvasPanel, including CanvasPanelSlot cleanup and variable compaction."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_remove_scalebox_canvaspanel"), TEXT("blueprint widget-remove"), TEXT("{\"widget\":\"CanvasPanel_1/ScaleBox_1\"}"), TEXT("Remove the bare ScaleBox child under the root CanvasPanel on WBP_CanvasPanel_ScaleBox"), TEXT("byte_equal"), TEXT("Covers widget-remove for a non-root ScaleBox leaf under CanvasPanel, including CanvasPanelSlot cleanup and variable compaction."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_remove_scrollbox_canvaspanel"), TEXT("blueprint widget-remove"), TEXT("{\"widget\":\"CanvasPanel_1/ScrollBox_1\"}"), TEXT("Remove the bare ScrollBox child under the root CanvasPanel on WBP_CanvasPanel_ScrollBox"), TEXT("byte_equal"), TEXT("Covers widget-remove for a non-root ScrollBox leaf under CanvasPanel, including CanvasPanelSlot cleanup and variable compaction."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_remove_widgetswitcher_canvaspanel"), TEXT("blueprint widget-remove"), TEXT("{\"widget\":\"CanvasPanel_1/WidgetSwitcher_1\"}"), TEXT("Remove the bare WidgetSwitcher child under the root CanvasPanel on WBP_CanvasPanel_WidgetSwitcher"), TEXT("byte_equal"), TEXT("Covers widget-remove for a non-root WidgetSwitcher leaf under CanvasPanel, including CanvasPanelSlot cleanup and variable compaction."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_remove_wrapbox_canvaspanel"), TEXT("blueprint widget-remove"), TEXT("{\"widget\":\"CanvasPanel_1/WrapBox_1\"}"), TEXT("Remove the bare WrapBox child under the root CanvasPanel on WBP_CanvasPanel_WrapBox"), TEXT("byte_equal"), TEXT("Covers widget-remove for a non-root WrapBox leaf under CanvasPanel, including CanvasPanelSlot cleanup and variable compaction."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_add_image_canvaspanel_nonempty"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_22\",\"type\":\"image\",\"name\":\"Image_59\"}"), TEXT("Add one bare Image child under the non-empty root CanvasPanel on WBP_CanvasPanel_TextBlock"), TEXT("byte_equal"), TEXT("Covers widget-add on a non-empty root CanvasPanel while preserving the existing TextBlock child and slot ordering."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\"]")),
        MakeOperation(TEXT("widget_add_image_overlay"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"Overlay_116\",\"type\":\"image\",\"name\":\"Image_23\"}"), TEXT("Add one bare Image child under the root Overlay on WBP_Overlay"), TEXT("byte_equal"), TEXT("Covers widget-add v1 for an empty root Overlay: inserts designer/generated Image + OverlaySlot exports and updates WidgetTree/widget-variable references."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\"]")),
        MakeOperation(TEXT("widget_add_image_overlay_nonempty"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"Overlay_116\",\"type\":\"image\",\"name\":\"Image_71\"}"), TEXT("Add one bare Image child under the non-empty root Overlay on WBP_Overlay_TextBlock"), TEXT("byte_equal"), TEXT("Covers widget-add on a non-empty root Overlay while preserving the existing TextBlock child and slot ordering."), TEXT("[\"editor-thumbnails\",\"widget-text-key-only\",\"widget-variable-guid-map-values\"]")),
        MakeOperation(TEXT("widget_add_image_nested_overlay"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_1/Overlay_1\",\"type\":\"image\",\"name\":\"Image_3\"}"), TEXT("Add one bare Image child under the nested Overlay_1 on WBP_MultiLevelSmoke"), TEXT("byte_equal"), TEXT("Covers widget-add on a nested top-level Overlay child under a root CanvasPanel, including generated-tree rootless companion rewrites for the nested branch."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\"]")),
        MakeOperation(TEXT("widget_add_root_canvaspanel"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"root\",\"type\":\"canvaspanel\",\"name\":\"CanvasPanel_21\"}"), TEXT("Create a root CanvasPanel on WBP_Minimum"), TEXT("byte_equal"), TEXT("Covers widget-add root creation on an empty WidgetBlueprint by wiring WidgetTree RootWidget/AllWidgets for a new CanvasPanel."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\"]")),
        MakeOperation(TEXT("widget_add_root_verticalbox"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"root\",\"type\":\"verticalbox\",\"name\":\"VerticalBox_21\"}"), TEXT("Create a root VerticalBox on WBP_Minimum"), TEXT("byte_equal"), TEXT("Covers widget-add root creation for VerticalBox on an empty WidgetBlueprint."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\"]")),
        MakeOperation(TEXT("widget_add_root_horizontalbox"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"root\",\"type\":\"horizontalbox\",\"name\":\"HorizontalBox_21\"}"), TEXT("Create a root HorizontalBox on WBP_Minimum"), TEXT("byte_equal"), TEXT("Covers widget-add root creation for HorizontalBox on an empty WidgetBlueprint."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_add_button_canvaspanel"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_22\",\"type\":\"button\",\"name\":\"Button_1\"}"), TEXT("Add one bare Button child under the root CanvasPanel on WBP_CanvasPanel"), TEXT("byte_equal"), TEXT("Covers widget-add for a container child under CanvasPanel, including generated-pair insertion for follow-up child edits."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\"]")),
        MakeOperation(TEXT("widget_add_image_border_canvaspanel"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_1/Border_1\",\"type\":\"image\",\"name\":\"Image_1\"}"), TEXT("Add one bare Image child under the existing Border_1 child on WBP_CanvasPanel_Border"), TEXT("byte_equal"), TEXT("Covers widget-add follow-up under a single-child Border wrapper nested below a root CanvasPanel."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_add_image_sizebox_canvaspanel"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_1/SizeBox_1\",\"type\":\"image\",\"name\":\"Image_1\"}"), TEXT("Add one bare Image child under the existing SizeBox_1 child on WBP_CanvasPanel_SizeBox"), TEXT("byte_equal"), TEXT("Covers widget-add follow-up under a single-child SizeBox wrapper nested below a root CanvasPanel."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_add_image_horizontalbox_canvaspanel"), TEXT("blueprint widget-add"), TEXT("{\"parent\":\"CanvasPanel_1/HorizontalBox_1\",\"type\":\"image\",\"name\":\"Image_1\"}"), TEXT("Add one bare Image child under the existing HorizontalBox_1 child on WBP_CanvasPanel_HorizontalBox"), TEXT("byte_equal"), TEXT("Covers widget-add follow-up under a nested HorizontalBox panel below a root CanvasPanel."), TEXT("[\"editor-thumbnails\",\"widget-variable-guid-map-values\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_opacity_root_textblock"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"TextBlock_72\",\"property\":\"render-opacity\",\"value\":\"0.8\"}"), TEXT("Set RenderOpacity to 0.8 on the root TextBlock on WBP_TextBlock_Opacity"), TEXT("byte_equal"), TEXT("Covers root widget RenderOpacity write across designer/generated WidgetTree exports. Source WBP has RenderOpacity=0.5 pre-set.")),
        MakeOperation(TEXT("widget_write_opacity_root_richtextblock"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"RichTextBlock_72\",\"property\":\"render-opacity\",\"value\":\"0.8\"}"), TEXT("Set RenderOpacity to 0.8 on the root RichTextBlock on WBP_RichTextBlock"), TEXT("byte_equal"), TEXT("Covers root RichTextBlock RenderOpacity write across designer/generated WidgetTree exports.")),
        MakeOperation(TEXT("widget_write_opacity_canvaspanel_child"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_22/TextBlock_31\",\"property\":\"render-opacity\",\"value\":\"0.5\"}"), TEXT("Set RenderOpacity to 0.5 on the child TextBlock under the root CanvasPanel on WBP_CanvasPanel_TextBlock"), TEXT("byte_equal"), TEXT("Covers child widget RenderOpacity write under CanvasPanel across designer/generated WidgetTree exports.")),
        MakeOperation(TEXT("widget_write_visibility_root_textblock"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"TextBlock_72\",\"property\":\"visibility\",\"value\":\"Collapsed\"}"), TEXT("Set Visibility to Collapsed on the root TextBlock on WBP_TextBlock"), TEXT("byte_equal"), TEXT("Covers root widget Visibility write across designer/generated WidgetTree exports.")),
        MakeOperation(TEXT("widget_write_visibility_root_richtextblock"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"RichTextBlock_72\",\"property\":\"visibility\",\"value\":\"Collapsed\"}"), TEXT("Set Visibility to Collapsed on the root RichTextBlock on WBP_RichTextBlock"), TEXT("byte_equal"), TEXT("Covers root RichTextBlock Visibility write across designer/generated WidgetTree exports.")),
        MakeOperation(TEXT("widget_write_richtext_default_shadow_offset"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"RichTextBlock_72\",\"property\":\"richtext-default-shadow-offset\",\"value\":\"3,4\"}"), TEXT("Set DefaultTextStyleOverride.ShadowOffset on the root RichTextBlock on WBP_RichTextBlock"), TEXT("byte_equal"), TEXT("Covers RichTextBlock default-style shadow offset insertion with bOverrideDefaultStyle enablement."), TEXT("[\"editor-thumbnails\"]")),
        MakeOperation(TEXT("widget_write_richtext_default_shadow_color"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"RichTextBlock_72\",\"property\":\"richtext-default-shadow-color-and-opacity\",\"value\":\"0.05,0.1,0.15,0.8\"}"), TEXT("Set DefaultTextStyleOverride.ShadowColorAndOpacity on the root RichTextBlock on WBP_RichTextBlock"), TEXT("byte_equal"), TEXT("Covers RichTextBlock default-style shadow color insertion with bOverrideDefaultStyle enablement."), TEXT("[\"editor-thumbnails\"]")),
        MakeOperation(TEXT("widget_write_richtext_default_outline_size"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"RichTextBlock_72\",\"property\":\"richtext-default-outline-size\",\"value\":\"2\"}"), TEXT("Set DefaultTextStyleOverride.Font.OutlineSettings.OutlineSize on the root RichTextBlock on WBP_RichTextBlock"), TEXT("byte_equal"), TEXT("Covers RichTextBlock outline-size insertion inside FontOutlineSettings."), TEXT("[\"editor-thumbnails\"]")),
        MakeOperation(TEXT("widget_write_richtext_default_outline_color"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"RichTextBlock_72\",\"property\":\"richtext-default-outline-color\",\"value\":\"0.2,0.3,0.9,1\"}"), TEXT("Set DefaultTextStyleOverride.Font.OutlineSettings.OutlineColor on the root RichTextBlock on WBP_RichTextBlock"), TEXT("byte_equal"), TEXT("Covers RichTextBlock outline-color insertion inside FontOutlineSettings."), TEXT("[\"editor-thumbnails\"]")),
        MakeOperation(TEXT("widget_write_richtext_auto_wrap_text"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"RichTextBlock_72\",\"property\":\"richtext-auto-wrap-text\",\"value\":\"true\"}"), TEXT("Enable AutoWrapText on the root RichTextBlock on WBP_RichTextBlock"), TEXT("byte_equal"), TEXT("Covers root RichTextBlock AutoWrapText bool insertion across designer/generated WidgetTree exports."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_richtext_wrap_text_at"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"RichTextBlock_72\",\"property\":\"richtext-wrap-text-at\",\"value\":\"320\"}"), TEXT("Set WrapTextAt to 320 on the root RichTextBlock on WBP_RichTextBlock"), TEXT("byte_equal"), TEXT("Covers root RichTextBlock WrapTextAt float insertion across designer/generated WidgetTree exports."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_richtext_line_height_percentage"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"RichTextBlock_72\",\"property\":\"richtext-line-height-percentage\",\"value\":\"1.25\"}"), TEXT("Set LineHeightPercentage to 1.25 on the root RichTextBlock on WBP_RichTextBlock"), TEXT("byte_equal"), TEXT("Covers root RichTextBlock LineHeightPercentage float insertion across designer/generated WidgetTree exports."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_progressbar_percent"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ProgressBar_1\",\"property\":\"progressbar-percent\",\"value\":\"0.75\"}"), TEXT("Set Percent to 0.75 on the ProgressBar child under the root CanvasPanel on WBP_CanvasPanel_ProgressBar"), TEXT("byte_equal"), TEXT("Validated on ue5.7 ProgressBar widget-write fixture pair."), TEXT("[\"editor-thumbnails\"]")),
        MakeOperation(TEXT("widget_write_progressbar_fill_color"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ProgressBar_1\",\"property\":\"progressbar-fill-color\",\"value\":\"0.2,0.4,0.6,0.8\"}"), TEXT("Set FillColorAndOpacity on the ProgressBar child under the root CanvasPanel on WBP_CanvasPanel_ProgressBar"), TEXT("byte_equal"), TEXT("Validated on ue5.7 ProgressBar widget-write fixture pair."), TEXT("[\"editor-thumbnails\"]")),
        MakeOperation(TEXT("widget_write_slider_value"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/Slider_1\",\"property\":\"slider-value\",\"value\":\"0.5\"}"), TEXT("Set Value to 0.5 on the Slider child under the root CanvasPanel on WBP_CanvasPanel_Slider"), TEXT("byte_equal"), TEXT("Validated on ue5.7 Slider widget-write fixture pair."), TEXT("[\"editor-thumbnails\"]")),
        MakeOperation(TEXT("widget_write_slider_orientation"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/Slider_1\",\"property\":\"slider-orientation\",\"value\":\"Vertical\"}"), TEXT("Set Orientation to Vertical on the Slider child under the root CanvasPanel on WBP_CanvasPanel_Slider"), TEXT("byte_equal"), TEXT("Validated on ue5.7 Slider widget-write fixture pair."), TEXT("[\"editor-thumbnails\"]")),
        MakeOperation(TEXT("widget_write_slider_is_focusable"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/Slider_1\",\"property\":\"slider-is-focusable\",\"value\":\"false\"}"), TEXT("Disable focus on the Slider child under the root CanvasPanel on WBP_CanvasPanel_Slider"), TEXT("byte_equal"), TEXT("Covers Slider IsFocusable bool rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_spacer_size"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/Spacer_1\",\"property\":\"spacer-size\",\"value\":\"24,48\"}"), TEXT("Set Size to 24x48 on the Spacer child under the root CanvasPanel on WBP_CanvasPanel_Spacer"), TEXT("byte_equal"), TEXT("Validated on ue5.7 Spacer widget-write fixture pair."), TEXT("[\"editor-thumbnails\"]")),
        MakeOperation(TEXT("widget_write_scrollbar_thickness"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ScrollBar_1\",\"property\":\"scrollbar-thickness\",\"value\":\"5,12\"}"), TEXT("Set Thickness to 5x12 on the ScrollBar child under the root CanvasPanel on WBP_CanvasPanel_ScrollBar"), TEXT("byte_equal"), TEXT("Validated on ue5.7 ScrollBar widget-write fixture pair."), TEXT("[\"editor-thumbnails\"]")),
        MakeOperation(TEXT("widget_write_scrollbar_orientation"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ScrollBar_1\",\"property\":\"scrollbar-orientation\",\"value\":\"Vertical\"}"), TEXT("Set Orientation to Vertical on the ScrollBar child under the root CanvasPanel on WBP_CanvasPanel_ScrollBar"), TEXT("byte_equal"), TEXT("Validated on ue5.7 ScrollBar widget-write fixture pair."), TEXT("[\"editor-thumbnails\"]")),
        MakeOperation(TEXT("widget_write_checkbox_is_checked_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/CheckBox_1\",\"property\":\"checkbox-is-checked\",\"value\":\"true\"}"), TEXT("Set CheckedState to Checked on the CheckBox child under the root CanvasPanel on WBP_CanvasPanel_CheckBox"), TEXT("byte_equal"), TEXT("Covers CheckBox bool-to-state rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_checkbox_checked_state_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/CheckBox_1\",\"property\":\"checkbox-checked-state\",\"value\":\"Undetermined\"}"), TEXT("Set CheckedState to Undetermined on the CheckBox child under the root CanvasPanel on WBP_CanvasPanel_CheckBox"), TEXT("byte_equal"), TEXT("Covers explicit CheckBox CheckedState enum rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_checkbox_is_focusable_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/CheckBox_1\",\"property\":\"checkbox-is-focusable\",\"value\":\"false\"}"), TEXT("Disable focus on the CheckBox child under the root CanvasPanel on WBP_CanvasPanel_CheckBox"), TEXT("byte_equal"), TEXT("Covers CheckBox IsFocusable bool rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_text_canvaspanel_child_editabletextbox"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/EditableTextBox_1\",\"property\":\"text\",\"value\":\"Player Name\"}"), TEXT("Set Text to Player Name on the EditableTextBox child under the root CanvasPanel on WBP_CanvasPanel_EditableTextBox"), TEXT("byte_equal"), TEXT("Covers EditableTextBox text rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"widget-text-localization-keys\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_text_canvaspanel_child_editabletext"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/EditableText_1\",\"property\":\"text\",\"value\":\"Display Name\"}"), TEXT("Set Text to Display Name on the EditableText child under the root CanvasPanel on WBP_CanvasPanel_EditableText"), TEXT("byte_equal"), TEXT("Covers EditableText text rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"widget-text-localization-keys\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_editabletext_hint_text_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/EditableText_1\",\"property\":\"editabletext-hint-text\",\"value\":\"Enter display name\"}"), TEXT("Set HintText to Enter display name on the EditableText child under the root CanvasPanel on WBP_CanvasPanel_EditableText"), TEXT("byte_equal"), TEXT("Covers EditableText hint-text rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"widget-text-localization-keys\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_editabletext_is_read_only_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/EditableText_1\",\"property\":\"editabletext-is-read-only\",\"value\":\"true\"}"), TEXT("Enable IsReadOnly on the EditableText child under the root CanvasPanel on WBP_CanvasPanel_EditableText"), TEXT("byte_equal"), TEXT("Covers EditableText is-read-only bool rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_editabletext_is_password_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/EditableText_1\",\"property\":\"editabletext-is-password\",\"value\":\"true\"}"), TEXT("Enable IsPassword on the EditableText child under the root CanvasPanel on WBP_CanvasPanel_EditableText"), TEXT("byte_equal"), TEXT("Covers EditableText is-password bool rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_editabletext_minimum_desired_width_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/EditableText_1\",\"property\":\"editabletext-minimum-desired-width\",\"value\":\"260\"}"), TEXT("Set MinimumDesiredWidth to 260 on the EditableText child under the root CanvasPanel on WBP_CanvasPanel_EditableText"), TEXT("byte_equal"), TEXT("Covers EditableText minimum-desired-width rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_editabletext_justification_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/EditableText_1\",\"property\":\"editabletext-justification\",\"value\":\"Right\"}"), TEXT("Set Justification to Right on the EditableText child under the root CanvasPanel on WBP_CanvasPanel_EditableText"), TEXT("byte_equal"), TEXT("Covers EditableText justification enum rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_editabletextbox_hint_text_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/EditableTextBox_1\",\"property\":\"editabletextbox-hint-text\",\"value\":\"Enter name\"}"), TEXT("Set HintText to Enter name on the EditableTextBox child under the root CanvasPanel on WBP_CanvasPanel_EditableTextBox"), TEXT("byte_equal"), TEXT("Covers EditableTextBox hint-text rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"widget-text-localization-keys\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_editabletextbox_is_read_only_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/EditableTextBox_1\",\"property\":\"editabletextbox-is-read-only\",\"value\":\"true\"}"), TEXT("Enable IsReadOnly on the EditableTextBox child under the root CanvasPanel on WBP_CanvasPanel_EditableTextBox"), TEXT("byte_equal"), TEXT("Covers EditableTextBox is-read-only bool rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_editabletextbox_is_password_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/EditableTextBox_1\",\"property\":\"editabletextbox-is-password\",\"value\":\"true\"}"), TEXT("Enable IsPassword on the EditableTextBox child under the root CanvasPanel on WBP_CanvasPanel_EditableTextBox"), TEXT("byte_equal"), TEXT("Covers EditableTextBox is-password bool rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_editabletextbox_minimum_desired_width_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/EditableTextBox_1\",\"property\":\"editabletextbox-minimum-desired-width\",\"value\":\"240\"}"), TEXT("Set MinimumDesiredWidth to 240 on the EditableTextBox child under the root CanvasPanel on WBP_CanvasPanel_EditableTextBox"), TEXT("byte_equal"), TEXT("Covers EditableTextBox minimum-desired-width rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_editabletextbox_justification_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/EditableTextBox_1\",\"property\":\"editabletextbox-justification\",\"value\":\"Center\"}"), TEXT("Set Justification to Center on the EditableTextBox child under the root CanvasPanel on WBP_CanvasPanel_EditableTextBox"), TEXT("byte_equal"), TEXT("Covers EditableTextBox justification enum rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_text_canvaspanel_child_multilineeditabletextbox"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/MultiLineEditableTextBox_1\",\"property\":\"text\",\"value\":\"Line 1\\nLine 2\"}"), TEXT("Set Text to two lines on the MultiLineEditableTextBox child under the root CanvasPanel on WBP_CanvasPanel_MultiLineEditableTextBox"), TEXT("byte_equal"), TEXT("Covers MultiLineEditableTextBox text rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"widget-text-localization-keys\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_multilineeditabletextbox_hint_text_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/MultiLineEditableTextBox_1\",\"property\":\"multilineeditabletextbox-hint-text\",\"value\":\"Enter description\"}"), TEXT("Set HintText to Enter description on the MultiLineEditableTextBox child under the root CanvasPanel on WBP_CanvasPanel_MultiLineEditableTextBox"), TEXT("byte_equal"), TEXT("Covers MultiLineEditableTextBox hint-text rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"widget-text-localization-keys\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_multilineeditabletextbox_is_read_only_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/MultiLineEditableTextBox_1\",\"property\":\"multilineeditabletextbox-is-read-only\",\"value\":\"true\"}"), TEXT("Enable IsReadOnly on the MultiLineEditableTextBox child under the root CanvasPanel on WBP_CanvasPanel_MultiLineEditableTextBox"), TEXT("byte_equal"), TEXT("Covers MultiLineEditableTextBox is-read-only bool rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_multilineeditabletextbox_justification_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/MultiLineEditableTextBox_1\",\"property\":\"multilineeditabletextbox-justification\",\"value\":\"Center\"}"), TEXT("Set Justification to Center on the MultiLineEditableTextBox child under the root CanvasPanel on WBP_CanvasPanel_MultiLineEditableTextBox"), TEXT("byte_equal"), TEXT("Covers MultiLineEditableTextBox justification enum rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_spinbox_value_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/SpinBox_1\",\"property\":\"spinbox-value\",\"value\":\"42\"}"), TEXT("Set Value to 42 on the SpinBox child under the root CanvasPanel on WBP_CanvasPanel_SpinBox"), TEXT("byte_equal"), TEXT("Covers SpinBox value rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_spinbox_min_value_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/SpinBox_1\",\"property\":\"spinbox-min-value\",\"value\":\"10\"}"), TEXT("Set MinValue to 10 on the SpinBox child under the root CanvasPanel on WBP_CanvasPanel_SpinBox"), TEXT("byte_equal"), TEXT("Covers SpinBox minimum value rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_spinbox_max_value_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/SpinBox_1\",\"property\":\"spinbox-max-value\",\"value\":\"100\"}"), TEXT("Set MaxValue to 100 on the SpinBox child under the root CanvasPanel on WBP_CanvasPanel_SpinBox"), TEXT("byte_equal"), TEXT("Covers SpinBox maximum value rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_spinbox_delta_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/SpinBox_1\",\"property\":\"spinbox-delta\",\"value\":\"5\"}"), TEXT("Set Delta to 5 on the SpinBox child under the root CanvasPanel on WBP_CanvasPanel_SpinBox"), TEXT("byte_equal"), TEXT("Covers SpinBox delta rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_comboboxstring_options_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ComboBoxString_1\",\"property\":\"comboboxstring-options\",\"value\":\"[\\\"Easy\\\",\\\"Normal\\\",\\\"Hard\\\"]\"}"), TEXT("Set DefaultOptions to Easy, Normal, Hard on the ComboBoxString child under the root CanvasPanel on WBP_CanvasPanel_ComboBoxString"), TEXT("byte_equal"), TEXT("Covers ComboBoxString default-options rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_comboboxstring_selected_option_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ComboBoxString_1\",\"property\":\"comboboxstring-selected-option\",\"value\":\"Normal\"}"), TEXT("Set SelectedOption to Normal on the ComboBoxString child under the root CanvasPanel on WBP_CanvasPanel_ComboBoxString"), TEXT("byte_equal"), TEXT("Covers ComboBoxString selected-option rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_comboboxstring_is_focusable_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ComboBoxString_1\",\"property\":\"comboboxstring-is-focusable\",\"value\":\"false\"}"), TEXT("Disable focus on the ComboBoxString child under the root CanvasPanel on WBP_CanvasPanel_ComboBoxString"), TEXT("byte_equal"), TEXT("Covers ComboBoxString bIsFocusable bool rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_listview_entry_widget_class_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ListView_1\",\"property\":\"listview-entry-widget-class\",\"value\":\"/Game/WBP/WBP_ListEntry_Text\"}"), TEXT("Set EntryWidgetClass to WBP_ListEntry_Text on the ListView child under the root CanvasPanel on WBP_CanvasPanel_ListView_NoEntry"), TEXT("byte_equal"), TEXT("Covers ListView EntryWidgetClass rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_listview_orientation_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ListView_1\",\"property\":\"listview-orientation\",\"value\":\"Horizontal\"}"), TEXT("Set Orientation to Horizontal on the ListView child under the root CanvasPanel on WBP_CanvasPanel_ListView"), TEXT("byte_equal"), TEXT("Covers ListView orientation rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_listview_selection_mode_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ListView_1\",\"property\":\"listview-selection-mode\",\"value\":\"Multi\"}"), TEXT("Set SelectionMode to Multi on the ListView child under the root CanvasPanel on WBP_CanvasPanel_ListView"), TEXT("byte_equal"), TEXT("Covers ListView selection-mode rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_listview_consume_mouse_wheel_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ListView_1\",\"property\":\"listview-consume-mouse-wheel\",\"value\":\"Never\"}"), TEXT("Set ConsumeMouseWheel to Never on the ListView child under the root CanvasPanel on WBP_CanvasPanel_ListView"), TEXT("byte_equal"), TEXT("Covers ListView consume-mouse-wheel rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_listview_is_focusable_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ListView_1\",\"property\":\"listview-is-focusable\",\"value\":\"false\"}"), TEXT("Disable focus on the ListView child under the root CanvasPanel on WBP_CanvasPanel_ListView"), TEXT("byte_equal"), TEXT("Covers ListView bIsFocusable rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_listview_return_focus_to_selection_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ListView_1\",\"property\":\"listview-return-focus-to-selection\",\"value\":\"true\"}"), TEXT("Enable ReturnFocusToSelection on the ListView child under the root CanvasPanel on WBP_CanvasPanel_ListView"), TEXT("byte_equal"), TEXT("Covers ListView bReturnFocusToSelection rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_listview_clear_scroll_velocity_on_selection_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ListView_1\",\"property\":\"listview-clear-scroll-velocity-on-selection\",\"value\":\"false\"}"), TEXT("Disable ClearScrollVelocityOnSelection on the ListView child under the root CanvasPanel on WBP_CanvasPanel_ListView"), TEXT("byte_equal"), TEXT("Covers ListView bClearScrollVelocityOnSelection rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_listview_scroll_into_view_alignment_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ListView_1\",\"property\":\"listview-scroll-into-view-alignment\",\"value\":\"BottomOrRight\"}"), TEXT("Set ScrollIntoViewAlignment to BottomOrRight on the ListView child under the root CanvasPanel on WBP_CanvasPanel_ListView"), TEXT("byte_equal"), TEXT("Covers ListView ScrollIntoViewAlignment rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_listview_wheel_scroll_multiplier_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ListView_1\",\"property\":\"listview-wheel-scroll-multiplier\",\"value\":\"2.5\"}"), TEXT("Set WheelScrollMultiplier to 2.5 on the ListView child under the root CanvasPanel on WBP_CanvasPanel_ListView"), TEXT("byte_equal"), TEXT("Covers ListView wheel-scroll-multiplier rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_listview_enable_scroll_animation_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ListView_1\",\"property\":\"listview-enable-scroll-animation\",\"value\":\"true\"}"), TEXT("Enable ScrollAnimation on the ListView child under the root CanvasPanel on WBP_CanvasPanel_ListView"), TEXT("byte_equal"), TEXT("Covers ListView enable-scroll-animation rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_listview_allow_overscroll_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ListView_1\",\"property\":\"listview-allow-overscroll\",\"value\":\"false\"}"), TEXT("Disable AllowOverscroll on the ListView child under the root CanvasPanel on WBP_CanvasPanel_ListView"), TEXT("byte_equal"), TEXT("Covers ListView AllowOverscroll rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_listview_enable_right_click_scrolling_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ListView_1\",\"property\":\"listview-enable-right-click-scrolling\",\"value\":\"false\"}"), TEXT("Disable bEnableRightClickScrolling on the ListView child under the root CanvasPanel on WBP_CanvasPanel_ListView"), TEXT("byte_equal"), TEXT("Covers ListView right-click scrolling rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_listview_enable_touch_scrolling_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ListView_1\",\"property\":\"listview-enable-touch-scrolling\",\"value\":\"false\"}"), TEXT("Disable bEnableTouchScrolling on the ListView child under the root CanvasPanel on WBP_CanvasPanel_ListView"), TEXT("byte_equal"), TEXT("Covers ListView touch scrolling rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_listview_is_pointer_scrolling_enabled_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ListView_1\",\"property\":\"listview-is-pointer-scrolling-enabled\",\"value\":\"false\"}"), TEXT("Disable pointer scrolling on the ListView child under the root CanvasPanel on WBP_CanvasPanel_ListView"), TEXT("byte_equal"), TEXT("Covers ListView pointer scrolling rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_listview_is_gamepad_scrolling_enabled_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ListView_1\",\"property\":\"listview-is-gamepad-scrolling-enabled\",\"value\":\"false\"}"), TEXT("Disable gamepad scrolling on the ListView child under the root CanvasPanel on WBP_CanvasPanel_ListView"), TEXT("byte_equal"), TEXT("Covers ListView gamepad scrolling rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_listview_horizontal_entry_spacing_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ListView_1\",\"property\":\"listview-horizontal-entry-spacing\",\"value\":\"12\"}"), TEXT("Set HorizontalEntrySpacing to 12 on the ListView child under the root CanvasPanel on WBP_CanvasPanel_ListView"), TEXT("byte_equal"), TEXT("Covers ListView horizontal-entry-spacing rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_listview_vertical_entry_spacing_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ListView_1\",\"property\":\"listview-vertical-entry-spacing\",\"value\":\"6\"}"), TEXT("Set VerticalEntrySpacing to 6 on the ListView child under the root CanvasPanel on WBP_CanvasPanel_ListView"), TEXT("byte_equal"), TEXT("Covers ListView vertical-entry-spacing rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_listview_scrollbar_padding_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ListView_1\",\"property\":\"listview-scrollbar-padding\",\"value\":\"1,2,3,4\"}"), TEXT("Set ScrollBarPadding to 1,2,3,4 on the ListView child under the root CanvasPanel on WBP_CanvasPanel_ListView"), TEXT("byte_equal"), TEXT("Covers ListView scrollbar-padding rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_tileview_entry_widget_class_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/TileView_1\",\"property\":\"listview-entry-widget-class\",\"value\":\"/Game/WBP/WBP_ListEntry_Text\"}"), TEXT("Set EntryWidgetClass to WBP_ListEntry_Text on the TileView child under the root CanvasPanel on WBP_CanvasPanel_TileView_NoEntry"), TEXT("byte_equal"), TEXT("Covers TileView EntryWidgetClass rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_tileview_entry_width_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/TileView_1\",\"property\":\"tileview-entry-width\",\"value\":\"180\"}"), TEXT("Set EntryWidth to 180 on the TileView child under the root CanvasPanel on WBP_CanvasPanel_TileView"), TEXT("byte_equal"), TEXT("Covers TileView entry-width rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_tileview_entry_height_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/TileView_1\",\"property\":\"tileview-entry-height\",\"value\":\"96\"}"), TEXT("Set EntryHeight to 96 on the TileView child under the root CanvasPanel on WBP_CanvasPanel_TileView"), TEXT("byte_equal"), TEXT("Covers TileView entry-height rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_tileview_scrollbar_disabled_visibility_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/TileView_1\",\"property\":\"tileview-scrollbar-disabled-visibility\",\"value\":\"Hidden\"}"), TEXT("Set ScrollbarDisabledVisibility to Hidden on the TileView child under the root CanvasPanel on WBP_CanvasPanel_TileView"), TEXT("byte_equal"), TEXT("Covers TileView scrollbar-disabled-visibility rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_tileview_entry_size_includes_entry_spacing_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/TileView_1\",\"property\":\"tileview-entry-size-includes-entry-spacing\",\"value\":\"false\"}"), TEXT("Disable bEntrySizeIncludesEntrySpacing on the TileView child under the root CanvasPanel on WBP_CanvasPanel_TileView"), TEXT("byte_equal"), TEXT("Covers TileView entry-size-includes-entry-spacing rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_treeview_entry_widget_class_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/TreeView_1\",\"property\":\"listview-entry-widget-class\",\"value\":\"/Game/WBP/WBP_ListEntry_Text\"}"), TEXT("Set EntryWidgetClass to WBP_ListEntry_Text on the TreeView child under the root CanvasPanel on WBP_CanvasPanel_TreeView_NoEntry"), TEXT("byte_equal"), TEXT("Covers TreeView EntryWidgetClass rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_treeview_selection_mode_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/TreeView_1\",\"property\":\"listview-selection-mode\",\"value\":\"Multi\"}"), TEXT("Set SelectionMode to Multi on the TreeView child under the root CanvasPanel on WBP_CanvasPanel_TreeView"), TEXT("byte_equal"), TEXT("Covers TreeView selection-mode rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_sizebox_width_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/SizeBox_1\",\"property\":\"sizebox-width-override\",\"value\":\"320\"}"), TEXT("Set WidthOverride to 320 on the SizeBox child under the root CanvasPanel on WBP_CanvasPanel_SizeBox"), TEXT("byte_equal"), TEXT("Covers SizeBox width-override rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_sizebox_height_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/SizeBox_1\",\"property\":\"sizebox-height-override\",\"value\":\"72\"}"), TEXT("Set HeightOverride to 72 on the SizeBox child under the root CanvasPanel on WBP_CanvasPanel_SizeBox"), TEXT("byte_equal"), TEXT("Covers SizeBox height-override rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_sizebox_min_desired_width_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/SizeBox_1\",\"property\":\"sizebox-min-desired-width\",\"value\":\"160\"}"), TEXT("Set MinDesiredWidth to 160 on the SizeBox child under the root CanvasPanel on WBP_CanvasPanel_SizeBox"), TEXT("byte_equal"), TEXT("Covers SizeBox min-desired-width rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_sizebox_min_desired_height_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/SizeBox_1\",\"property\":\"sizebox-min-desired-height\",\"value\":\"48\"}"), TEXT("Set MinDesiredHeight to 48 on the SizeBox child under the root CanvasPanel on WBP_CanvasPanel_SizeBox"), TEXT("byte_equal"), TEXT("Covers SizeBox min-desired-height rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_sizebox_max_desired_width_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/SizeBox_1\",\"property\":\"sizebox-max-desired-width\",\"value\":\"640\"}"), TEXT("Set MaxDesiredWidth to 640 on the SizeBox child under the root CanvasPanel on WBP_CanvasPanel_SizeBox"), TEXT("byte_equal"), TEXT("Covers SizeBox max-desired-width rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_sizebox_max_desired_height_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/SizeBox_1\",\"property\":\"sizebox-max-desired-height\",\"value\":\"240\"}"), TEXT("Set MaxDesiredHeight to 240 on the SizeBox child under the root CanvasPanel on WBP_CanvasPanel_SizeBox"), TEXT("byte_equal"), TEXT("Covers SizeBox max-desired-height rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_sizebox_min_aspect_ratio_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/SizeBox_1\",\"property\":\"sizebox-min-aspect-ratio\",\"value\":\"1.25\"}"), TEXT("Set MinAspectRatio to 1.25 on the SizeBox child under the root CanvasPanel on WBP_CanvasPanel_SizeBox"), TEXT("byte_equal"), TEXT("Covers SizeBox min-aspect-ratio rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_sizebox_max_aspect_ratio_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/SizeBox_1\",\"property\":\"sizebox-max-aspect-ratio\",\"value\":\"2.0\"}"), TEXT("Set MaxAspectRatio to 2.0 on the SizeBox child under the root CanvasPanel on WBP_CanvasPanel_SizeBox"), TEXT("byte_equal"), TEXT("Covers SizeBox max-aspect-ratio rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_scrollbox_orientation_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ScrollBox_1\",\"property\":\"scrollbox-orientation\",\"value\":\"Horizontal\"}"), TEXT("Set Orientation to Horizontal on the ScrollBox child under the root CanvasPanel on WBP_CanvasPanel_ScrollBox"), TEXT("byte_equal"), TEXT("Covers ScrollBox orientation rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_scrollbox_scrollbar_visibility_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ScrollBox_1\",\"property\":\"scrollbox-scrollbar-visibility\",\"value\":\"Collapsed\"}"), TEXT("Set ScrollBarVisibility to Collapsed on the ScrollBox child under the root CanvasPanel on WBP_CanvasPanel_ScrollBox"), TEXT("byte_equal"), TEXT("Covers ScrollBox scrollbar-visibility rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_scrollbox_consume_mouse_wheel_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ScrollBox_1\",\"property\":\"scrollbox-consume-mouse-wheel\",\"value\":\"Always\"}"), TEXT("Set ConsumeMouseWheel to Always on the ScrollBox child under the root CanvasPanel on WBP_CanvasPanel_ScrollBox"), TEXT("byte_equal"), TEXT("Covers ScrollBox consume-mouse-wheel enum rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_scrollbox_is_focusable_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ScrollBox_1\",\"property\":\"scrollbox-is-focusable\",\"value\":\"true\"}"), TEXT("Enable focus on the ScrollBox child under the root CanvasPanel on WBP_CanvasPanel_ScrollBox"), TEXT("byte_equal"), TEXT("Covers ScrollBox bIsFocusable bool rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_scalebox_stretch_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ScaleBox_1\",\"property\":\"scalebox-stretch\",\"value\":\"ScaleToFit\"}"), TEXT("Set Stretch to ScaleToFit on the ScaleBox child under the root CanvasPanel on WBP_CanvasPanel_ScaleBox"), TEXT("byte_equal"), TEXT("Covers ScaleBox stretch enum rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_scalebox_stretch_direction_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ScaleBox_1\",\"property\":\"scalebox-stretch-direction\",\"value\":\"DownOnly\"}"), TEXT("Set StretchDirection to DownOnly on the ScaleBox child under the root CanvasPanel on WBP_CanvasPanel_ScaleBox"), TEXT("byte_equal"), TEXT("Covers ScaleBox stretch-direction enum rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_scalebox_user_specified_scale_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ScaleBox_1\",\"property\":\"scalebox-user-specified-scale\",\"value\":\"1.25\"}"), TEXT("Set UserSpecifiedScale to 1.25 on the ScaleBox child under the root CanvasPanel on WBP_CanvasPanel_ScaleBox"), TEXT("byte_equal"), TEXT("Covers ScaleBox user-specified scale rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_scalebox_ignore_inherited_scale_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/ScaleBox_1\",\"property\":\"scalebox-ignore-inherited-scale\",\"value\":\"true\"}"), TEXT("Enable IgnoreInheritedScale on the ScaleBox child under the root CanvasPanel on WBP_CanvasPanel_ScaleBox"), TEXT("byte_equal"), TEXT("Covers ScaleBox ignore-inherited-scale bool rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_wrapbox_wrap_size_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/WrapBox_1\",\"property\":\"wrapbox-wrap-size\",\"value\":\"480\"}"), TEXT("Set WrapSize to 480 on the WrapBox child under the root CanvasPanel on WBP_CanvasPanel_WrapBox"), TEXT("byte_equal"), TEXT("Covers WrapBox wrap-size rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_wrapbox_explicit_wrap_size_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/WrapBox_1\",\"property\":\"wrapbox-explicit-wrap-size\",\"value\":\"true\"}"), TEXT("Enable ExplicitWrapSize on the WrapBox child under the root CanvasPanel on WBP_CanvasPanel_WrapBox"), TEXT("byte_equal"), TEXT("Covers WrapBox explicit-wrap-size bool rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_wrapbox_inner_slot_padding_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/WrapBox_1\",\"property\":\"wrapbox-inner-slot-padding\",\"value\":\"8,12\"}"), TEXT("Set InnerSlotPadding to 8x12 on the WrapBox child under the root CanvasPanel on WBP_CanvasPanel_WrapBox"), TEXT("byte_equal"), TEXT("Covers WrapBox inner-slot-padding rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_wrapbox_orientation_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/WrapBox_1\",\"property\":\"wrapbox-orientation\",\"value\":\"Vertical\"}"), TEXT("Set Orientation to Vertical on the WrapBox child under the root CanvasPanel on WBP_CanvasPanel_WrapBox"), TEXT("byte_equal"), TEXT("Covers WrapBox orientation rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_widgetswitcher_active_widget_index_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/WidgetSwitcher_1\",\"property\":\"widgetswitcher-active-widget-index\",\"value\":\"2\"}"), TEXT("Set ActiveWidgetIndex to 2 on the WidgetSwitcher child under the root CanvasPanel on WBP_CanvasPanel_WidgetSwitcherChildren"), TEXT("byte_equal"), TEXT("Covers WidgetSwitcher active-widget-index rewrite with multiple child slots present."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_retainerbox_retain_render_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/RetainerBox_1\",\"property\":\"retainerbox-retain-render\",\"value\":\"true\"}"), TEXT("Enable RetainRender on the RetainerBox child under the root CanvasPanel on WBP_CanvasPanel_RetainerBox"), TEXT("byte_equal"), TEXT("Covers RetainerBox retain-render bool rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_retainerbox_render_on_invalidation_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/RetainerBox_1\",\"property\":\"retainerbox-render-on-invalidation\",\"value\":\"false\"}"), TEXT("Disable RenderOnInvalidation on the RetainerBox child under the root CanvasPanel on WBP_CanvasPanel_RetainerBox"), TEXT("byte_equal"), TEXT("Covers RetainerBox render-on-invalidation bool rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_retainerbox_render_on_phase_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/RetainerBox_1\",\"property\":\"retainerbox-render-on-phase\",\"value\":\"true\"}"), TEXT("Enable RenderOnPhase on the RetainerBox child under the root CanvasPanel on WBP_CanvasPanel_RetainerBox"), TEXT("byte_equal"), TEXT("Covers RetainerBox render-on-phase bool rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_retainerbox_phase_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/RetainerBox_1\",\"property\":\"retainerbox-phase\",\"value\":\"1\"}"), TEXT("Set Phase to 1 on the RetainerBox child under the root CanvasPanel on WBP_CanvasPanel_RetainerBox"), TEXT("byte_equal"), TEXT("Covers RetainerBox phase int rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_retainerbox_phase_count_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/RetainerBox_1\",\"property\":\"retainerbox-phase-count\",\"value\":\"3\"}"), TEXT("Set PhaseCount to 3 on the RetainerBox child under the root CanvasPanel on WBP_CanvasPanel_RetainerBox"), TEXT("byte_equal"), TEXT("Covers RetainerBox phase-count int rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_backgroundblur_strength_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/BackgroundBlur_1\",\"property\":\"backgroundblur-strength\",\"value\":\"16\"}"), TEXT("Set BlurStrength to 16 on the BackgroundBlur child under the root CanvasPanel on WBP_CanvasPanel_BackgroundBlur"), TEXT("byte_equal"), TEXT("Covers BackgroundBlur blur-strength rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_backgroundblur_apply_alpha_to_blur_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/BackgroundBlur_1\",\"property\":\"backgroundblur-apply-alpha-to-blur\",\"value\":\"true\"}"), TEXT("Enable ApplyAlphaToBlur on the BackgroundBlur child under the root CanvasPanel on WBP_CanvasPanel_BackgroundBlur"), TEXT("byte_equal"), TEXT("Covers BackgroundBlur apply-alpha-to-blur bool rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_safezone_pad_left_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/SafeZone_1\",\"property\":\"safezone-pad-left\",\"value\":\"false\"}"), TEXT("Disable PadLeft on the SafeZone child under the root CanvasPanel on WBP_CanvasPanel_SafeZone"), TEXT("byte_equal"), TEXT("Covers SafeZone pad-left bool rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_safezone_pad_right_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/SafeZone_1\",\"property\":\"safezone-pad-right\",\"value\":\"true\"}"), TEXT("Enable PadRight on the SafeZone child under the root CanvasPanel on WBP_CanvasPanel_SafeZone"), TEXT("byte_equal"), TEXT("Covers SafeZone pad-right bool rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_safezone_pad_top_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/SafeZone_1\",\"property\":\"safezone-pad-top\",\"value\":\"false\"}"), TEXT("Disable PadTop on the SafeZone child under the root CanvasPanel on WBP_CanvasPanel_SafeZone"), TEXT("byte_equal"), TEXT("Covers SafeZone pad-top bool rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_safezone_pad_bottom_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/SafeZone_1\",\"property\":\"safezone-pad-bottom\",\"value\":\"true\"}"), TEXT("Enable PadBottom on the SafeZone child under the root CanvasPanel on WBP_CanvasPanel_SafeZone"), TEXT("byte_equal"), TEXT("Covers SafeZone pad-bottom bool rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_invalidationbox_can_cache_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/InvalidationBox_1\",\"property\":\"invalidationbox-can-cache\",\"value\":\"true\"}"), TEXT("Enable CanCache on the InvalidationBox child under the root CanvasPanel on WBP_CanvasPanel_InvalidationBox"), TEXT("byte_equal"), TEXT("Covers InvalidationBox can-cache bool rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("widget_write_uniformgridpanel_min_desired_slot_width_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/UniformGridPanel_1\",\"property\":\"uniformgridpanel-min-desired-slot-width\",\"value\":\"160\"}"), TEXT("Set MinDesiredSlotWidth to 160 on the UniformGridPanel child under the root CanvasPanel on WBP_CanvasPanel_UniformGridPanel"), TEXT("byte_equal"), TEXT("Covers UniformGridPanel min-desired-slot-width rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_uniformgridpanel_min_desired_slot_height_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/UniformGridPanel_1\",\"property\":\"uniformgridpanel-min-desired-slot-height\",\"value\":\"48\"}"), TEXT("Set MinDesiredSlotHeight to 48 on the UniformGridPanel child under the root CanvasPanel on WBP_CanvasPanel_UniformGridPanel"), TEXT("byte_equal"), TEXT("Covers UniformGridPanel min-desired-slot-height rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_uniformgridpanel_slot_padding_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/UniformGridPanel_1\",\"property\":\"uniformgridpanel-slot-padding\",\"value\":\"4,6,8,10\"}"), TEXT("Set SlotPadding to 4,6,8,10 on the UniformGridPanel child under the root CanvasPanel on WBP_CanvasPanel_UniformGridPanel"), TEXT("byte_equal"), TEXT("Covers UniformGridPanel slot-padding rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\",\"blueprint-search-tail\"]")),
        MakeOperation(TEXT("widget_write_visibility_overlay_child"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"Overlay_116/TextBlock_36\",\"property\":\"visibility\",\"value\":\"Collapsed\"}"), TEXT("Set Visibility to Collapsed on the child TextBlock under the root Overlay on WBP_Overlay_TextBlock"), TEXT("byte_equal"), TEXT("Covers child widget Visibility write under Overlay across designer/generated WidgetTree exports.")),
        MakeOperation(TEXT("widget_write_layout_canvaspanelslot"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_22/Image_29\",\"property\":\"layout-data\",\"value\":\"{\\\"position\\\":[0,0],\\\"size\\\":[200,60],\\\"anchors\\\":[0.5,0.5,0.5,0.5],\\\"alignment\\\":[0.5,0.5]}\"}"), TEXT("Set CanvasPanelSlot layout data on Image_29 under WBP_CanvasPanel_Image"), TEXT("byte_equal"), TEXT("Covers full CanvasPanelSlot LayoutData rewrite including position, size, anchors, and alignment."), TEXT("[\"editor-thumbnails\"]")),
        MakeOperation(TEXT("widget_write_brush_image"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"Image_22\",\"property\":\"brush-image\",\"value\":\"/Game/Effects/Textures/Decals/chippedcracks\"}"), TEXT("Set the Brush ResourceObject on root Image widget to Texture2D chippedcracks via UE Editor property panel"), TEXT("byte_equal"), TEXT("Covers brush-image assignment on root Image widget: adds Texture2D import, inserts NameMap entries in alphabetical order, sets SlateBrush struct with ImageType/ImageSize/ResourceObject, and syncs DependsMap."), TEXT("[\"editor-thumbnails\"]")),
        MakeOperation(TEXT("widget_write_button_brush_normal"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"Button_1\",\"property\":\"button-normal-image\",\"value\":\"/Game/UI/Menu/Art/T_UI_Icon_SimpleArrow\"}"), TEXT("Assign /Game/UI/Menu/Art/T_UI_Icon_SimpleArrow to Button_1 normal state brush on WBP_ButtonVisualSmoke"), TEXT("byte_equal"), TEXT("Covers Button.WidgetStyle.Normal brush ResourceObject write via the UMG details panel, including Texture2D import/reference persistence."), TEXT("[\"editor-thumbnails\"]")),
        MakeOperation(TEXT("widget_write_button_brush_tint"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"Button_1\",\"property\":\"button-normal-tint\",\"value\":\"0.25,0.4,0.9,0.8\"}"), TEXT("Set Button_1 normal state brush tint on WBP_ButtonVisualSmoke"), TEXT("byte_equal"), TEXT("Covers Button.WidgetStyle.Normal.TintColor write using a specified SlateColor value."), TEXT("[\"editor-thumbnails\"]")),
        MakeOperation(TEXT("widget_write_button_brush_image_size"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"Button_1\",\"property\":\"button-normal-image-size\",\"value\":\"96,48\"}"), TEXT("Set Button_1 normal state brush image size on WBP_ButtonVisualSmoke"), TEXT("byte_equal"), TEXT("Covers Button.WidgetStyle.Normal.ImageSize write using DeprecateSlateVector2D payload semantics."), TEXT("[\"editor-thumbnails\"]")),
        MakeOperation(TEXT("widget_write_button_brush_draw_as"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"Button_1\",\"property\":\"button-normal-draw-as\",\"value\":\"RoundedBox\"}"), TEXT("Set Button_1 normal state brush draw type on WBP_ButtonVisualSmoke"), TEXT("byte_equal"), TEXT("Covers Button.WidgetStyle.Normal.DrawAs enum write on a state brush."), TEXT("[\"editor-thumbnails\"]")),
        MakeOperation(TEXT("widget_write_button_is_focusable_canvaspanel"), TEXT("blueprint widget-write"), TEXT("{\"widget\":\"CanvasPanel_1/Button_1\",\"property\":\"button-is-focusable\",\"value\":\"false\"}"), TEXT("Disable focus on the Button child under the root CanvasPanel on WBP_CanvasPanel_Button"), TEXT("byte_equal"), TEXT("Covers Button IsFocusable bool rewrite on a bare CanvasPanel child."), TEXT("[\"editor-thumbnails\",\"saved-hash\"]")),
        MakeOperation(TEXT("prop_set_enum"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureEnum\",\"value\":\"\\\"BPXEnum_ValueA\\\"\"}"), TEXT("Set enum variable"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_enum_numeric"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureEnum\",\"value\":\"1\"}"), TEXT("Set enum variable by numeric literal"), TEXT("byte_equal"), TEXT("Enum numeric literal coercion to underlying enum value is implemented.")),
        MakeOperation(TEXT("prop_set_enum_anchor"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureEnumAnchor\",\"value\":\"\\\"BPXEnum_ValueA\\\"\"}"), TEXT("Set secondary enum variable"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_vector"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureVector\",\"value\":\"{\\\"X\\\":1.5,\\\"Y\\\":-2.3,\\\"Z\\\":100.0}\"}"), TEXT("Set Vector variable"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_vector_axis_x"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureVector.X\",\"value\":\"-123.456\"}"), TEXT("Set Vector.X field"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_rotator"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureRotator\",\"value\":\"{\\\"Pitch\\\":45,\\\"Yaw\\\":90,\\\"Roll\\\":180}\"}"), TEXT("Set Rotator variable"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_rotator_axis_roll"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureRotator.Roll\",\"value\":\"-45.5\"}"), TEXT("Set Rotator.Roll field"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_color"), TEXT("prop set"), TEXT("{\"export\":3,\"path\":\"VColor\",\"value\":\"{\\\"structType\\\":\\\"LinearColor\\\",\\\"value\\\":{\\\"r\\\":0.25,\\\"g\\\":0.5,\\\"b\\\":0.75,\\\"a\\\":1}}\"}"), TEXT("Update LinearColor value on math fixture"), TEXT("byte_equal"), TEXT("Validated on ue5.6/ue5.7 math fixture roots.")),
        MakeOperation(TEXT("prop_set_transform"), TEXT("prop set"), TEXT("{\"export\":3,\"path\":\"VTransform\",\"value\":\"{\\\"structType\\\":\\\"Transform\\\",\\\"value\\\":{\\\"Translation\\\":{\\\"type\\\":\\\"StructProperty(Vector(/Script/CoreUObject))\\\",\\\"value\\\":{\\\"structType\\\":\\\"Vector\\\",\\\"value\\\":{\\\"x\\\":1,\\\"y\\\":2,\\\"z\\\":3}}},\\\"Rotation\\\":{\\\"type\\\":\\\"StructProperty(Quat(/Script/CoreUObject))\\\",\\\"value\\\":{\\\"structType\\\":\\\"Quat\\\",\\\"value\\\":{\\\"x\\\":0,\\\"y\\\":0,\\\"z\\\":0,\\\"w\\\":1}}},\\\"Scale3D\\\":{\\\"type\\\":\\\"StructProperty(Vector(/Script/CoreUObject))\\\",\\\"value\\\":{\\\"structType\\\":\\\"Vector\\\",\\\"value\\\":{\\\"x\\\":1,\\\"y\\\":1,\\\"z\\\":1}}}}}\"}"), TEXT("Update Transform value on math fixture"), TEXT("byte_equal"), TEXT("Validated on ue5.6/ue5.7 math fixture roots.")),
        MakeOperationWithErrorContains(TEXT("prop_set_soft_object"), TEXT("prop set"), TEXT("{\"export\":1,\"path\":\"LastEditedDocuments[0].EditedObjectPath\",\"value\":\"{\\\"packageName\\\":\\\"/Game/BPXFixtures/Parse/BP_WithMetadata\\\",\\\"assetName\\\":\\\"BP_WithMetadata_C\\\"}\"}"), TEXT("Attempt soft object path update inside EditedDocumentInfo array"), TEXT("error_equal"), TEXT("EditedDocumentInfo is not editable"), TEXT("Fixture now asserts explicit rejection with byte-identical output.")),
        MakeOperation(TEXT("prop_set_array_element"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"MyArray[1]\",\"value\":\"99\"}"), TEXT("Set array element"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_array_replace_longer"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"MyArray\",\"value\":\"[1,2,3,4,5,6,7,8]\"}"), TEXT("Replace array with longer payload"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_array_replace_empty"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"MyArray\",\"value\":\"[4]\"}"), TEXT("Replace array with shorter payload"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_map_value"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"MyMap[\\\"key\\\"]\",\"value\":\"99\"}"), TEXT("Set map value"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_custom_struct_int"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureCustom.IntVal\",\"value\":\"42\"}"), TEXT("Set custom struct int field"), TEXT("byte_equal"), TEXT("Custom StructProperty tagged re-encoding for int fields is implemented.")),
        MakeOperation(TEXT("prop_set_custom_struct_enum"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureCustom.EnumVal\",\"value\":\"1\"}"), TEXT("Set custom struct enum field via numeric literal"), TEXT("byte_equal"), TEXT("Covers custom struct enum re-encoding without requiring NameMap growth.")),
        MakeOperation(TEXT("prop_set_nested_struct"), TEXT("prop set"), TEXT("{\"export\":3,\"path\":\"VTransform.Translation\",\"value\":\"{\\\"x\\\":1,\\\"y\\\":2,\\\"z\\\":3}\"}"), TEXT("Update nested Transform.Translation struct"), TEXT("byte_equal"), TEXT("Validated on ue5.6/ue5.7 math fixture roots.")),
        MakeOperationWithErrorContains(TEXT("prop_set_nested_array_struct"), TEXT("prop set"), TEXT("{\"export\":1,\"path\":\"LastEditedDocuments[0].SavedZoomAmount\",\"value\":\"-2.5\"}"), TEXT("Attempt nested array-of-struct leaf update on blueprint metadata export"), TEXT("error_equal"), TEXT("EditedDocumentInfo is not editable"), TEXT("Fixture now asserts explicit rejection with byte-identical output.")),
        MakeOperation(TEXT("dt_update_int"), TEXT("datatable update-row"), TEXT("{\"row\":\"Row_A\",\"values\":\"{\\\"Score\\\":999}\"}"), TEXT("Update DataTable int column"), TEXT("byte_equal"), TEXT("Validated on ue5.6/ue5.7 DataTable fixtures.")),
        MakeOperation(TEXT("dt_update_float"), TEXT("datatable update-row"), TEXT("{\"row\":\"Row_B\",\"values\":\"{\\\"Rate\\\":1.25}\"}"), TEXT("Update DataTable float column"), TEXT("byte_equal"), TEXT("Validated on ue5.6/ue5.7 DataTable fixtures.")),
        MakeOperation(TEXT("dt_update_string"), TEXT("datatable update-row"), TEXT("{\"row\":\"Row_A\",\"values\":\"{\\\"Label\\\":\\\"NewName\\\"}\"}"), TEXT("Update DataTable string column"), TEXT("byte_equal"), TEXT("Validated on ue5.6/ue5.7 DataTable fixtures.")),
        MakeOperation(TEXT("dt_update_multi_field"), TEXT("datatable update-row"), TEXT("{\"row\":\"Row_A\",\"values\":\"{\\\"Score\\\":50,\\\"Rate\\\":0.1}\"}"), TEXT("Update DataTable multiple columns"), TEXT("byte_equal"), TEXT("Validated on ue5.6/ue5.7 DataTable fixtures.")),
        MakeOperationWithErrorContains(TEXT("dt_update_complex"), TEXT("datatable update-row"), TEXT("{\"row\":\"Row_A\",\"values\":\"{\\\"Tags\\\":[\\\"TagA\\\",\\\"TagB\\\"]}\"}"), TEXT("Attempt complex DataTable row update against scalar row schema"), TEXT("error_equal"), TEXT("property not found: Tags"), TEXT("Fixture now asserts explicit rejection with byte-identical output.")),
        MakeOperation(TEXT("dt_add_row"), TEXT("datatable add-row"), TEXT("{\"row\":\"Row_A_1\"}"), TEXT("Add one DataTable row"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("dt_add_row_values_scalar"), TEXT("datatable add-row"), TEXT("{\"row\":\"Row_A_1\",\"values\":\"{\\\"Score\\\":123}\"}"), TEXT("Add one DataTable row with scalar field update"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("dt_add_row_values_mixed"), TEXT("datatable add-row"), TEXT("{\"row\":\"Row_B_1\",\"values\":\"{\\\"Score\\\":7,\\\"Rate\\\":0.25,\\\"Label\\\":\\\"Row_B_added\\\",\\\"Mode\\\":\\\"BPXEnum_ValueB\\\"}\"}"), TEXT("Add one DataTable row with mixed-type field updates"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("dt_remove_row"), TEXT("datatable remove-row"), TEXT("{\"row\":\"Row_A_1\"}"), TEXT("Remove one DataTable row"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("dt_remove_row_base"), TEXT("datatable remove-row"), TEXT("{\"row\":\"Row_B\"}"), TEXT("Remove one base DataTable row"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperationWithErrorContains(TEXT("dt_add_row_composite_reject"), TEXT("datatable add-row"), TEXT("{\"row\":\"Row_A_1\"}"), TEXT("Reject add-row against CompositeDataTable"), TEXT("error_equal"), TEXT("composite"), TEXT("Fixture now asserts explicit rejection with byte-identical output.")),
        MakeOperationWithErrorContains(TEXT("dt_remove_row_composite_reject"), TEXT("datatable remove-row"), TEXT("{\"row\":\"Row_A\"}"), TEXT("Reject remove-row against CompositeDataTable"), TEXT("error_equal"), TEXT("composite"), TEXT("Fixture now asserts explicit rejection with byte-identical output.")),
        MakeOperationWithErrorContains(TEXT("dt_update_row_composite_reject"), TEXT("datatable update-row"), TEXT("{\"row\":\"Row_A\",\"values\":\"{\\\"Score\\\":999}\"}"), TEXT("Reject update-row against CompositeDataTable"), TEXT("error_equal"), TEXT("composite"), TEXT("Fixture now asserts explicit rejection with byte-identical output.")),
        MakeOperationWithErrorContains(TEXT("metadata_set_tooltip"), TEXT("metadata set-root"), TEXT("{\"export\":1,\"key\":\"ToolTip\",\"value\":\"Updated\"}"), TEXT("Attempt root tooltip metadata update"), TEXT("error_equal"), TEXT("no editable path matched"), TEXT("Fixture now asserts explicit rejection with byte-identical output.")),
        MakeOperation(TEXT("metadata_set_category"), TEXT("metadata set-root"), TEXT("{\"export\":11,\"key\":\"CategoryName\",\"value\":\"Gameplay\"}"), TEXT("Update root category text property via metadata set-root path fallback"), TEXT("byte_equal"), TEXT("Validated on ue5.6/ue5.7 scalar fixture roots.")),
        MakeOperation(TEXT("export_set_header"), TEXT("export set-header"), TEXT("{\"index\":1,\"fields\":\"{\\\"objectFlags\\\":1}\"}"), TEXT("Set export header fields"), TEXT("byte_equal"), TEXT("export set-header updates selected export header fields deterministically.")),
        MakeOperation(TEXT("package_set_flags"), TEXT("package set-flags"), TEXT("{\"flags\":\"PKG_REQUIRESLOCALIZATIONGATHER|PKG_RUNTIMEGENERATED\"}"), TEXT("Set package flags without touching shape-sensitive bits"), TEXT("byte_equal"), TEXT("Validated on ue5.6/ue5.7 parse fixtures.")),
        MakeOperation(TEXT("var_set_default_int"), TEXT("var set-default"), TEXT("{\"name\":\"MyStr\",\"value\":\"\\\"changed\\\"\"}"), TEXT("Set variable default value"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("var_set_default_empty"), TEXT("var set-default"), TEXT("{\"name\":\"MyStr\",\"value\":\"\\\"\\\"\"}"), TEXT("Set variable default value to empty string"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("var_set_default_unicode"), TEXT("var set-default"), TEXT("{\"name\":\"MyStr\",\"value\":\"\\\"テスト\\\"\"}"), TEXT("Set variable default value to unicode string"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("var_set_default_long"), TEXT("var set-default"), TEXT("{\"name\":\"MyStr\",\"value\":\"\\\"Lorem ipsum dolor sit amet var-default\\\"\"}"), TEXT("Set variable default value to long string"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("var_set_default_string"), TEXT("var set-default"), TEXT("{\"name\":\"VString\",\"value\":\"\\\"golden\\\"\"}"), TEXT("Update string variable default value"), TEXT("byte_equal"), TEXT("Validated on ue5.6/ue5.7 scalar fixture roots.")),
        MakeOperation(TEXT("var_set_default_vector"), TEXT("var set-default"), TEXT("{\"name\":\"VVector\",\"value\":\"{\\\"x\\\":1,\\\"y\\\":2,\\\"z\\\":3}\"}"), TEXT("Update vector variable default value"), TEXT("byte_equal"), TEXT("Validated on ue5.6/ue5.7 math fixture roots.")),
        MakeOperation(TEXT("var_rename_simple"), TEXT("var rename"), TEXT("{\"from\":\"OldVar\",\"to\":\"NewVar\"}"), TEXT("Rename simple variable"), TEXT("byte_equal"), TEXT("var rename supports deterministic declaration/name-map rewrites.")),
        MakeOperation(TEXT("var_rename_with_refs"), TEXT("var rename"), TEXT("{\"from\":\"UsedVar\",\"to\":\"RenamedVar\"}"), TEXT("Rename referenced variable"), TEXT("byte_equal"), TEXT("var rename applies declaration/name-map rewrites with reference updates.")),
        MakeOperation(TEXT("var_rename_unicode"), TEXT("var rename"), TEXT("{\"from\":\"体力\",\"to\":\"HP\"}"), TEXT("Rename unicode variable"), TEXT("byte_equal"), TEXT("var rename supports unicode-aware target names.")),
        MakeOperation(TEXT("ref_rewrite_single"), TEXT("ref rewrite"), TEXT("{\"from\":\"/Game/Old/Mesh\",\"to\":\"/Game/New/Mesh\"}"), TEXT("Rewrite one soft reference"), TEXT("byte_equal"), TEXT("ref rewrite supports single-path replacements.")),
        MakeOperation(TEXT("ref_rewrite_multi"), TEXT("ref rewrite"), TEXT("{\"from\":\"/Game/OldDir\",\"to\":\"/Game/NewDir\"}"), TEXT("Rewrite references under directory"), TEXT("byte_equal"), TEXT("ref rewrite supports directory-wide replacements.")),
        MakeOperation(TEXT("enum_write_value"), TEXT("enum write-value"), TEXT("{\"export\":5,\"name\":\"FixtureEnum\",\"value\":\"BPXEnum_ValueA\"}"), TEXT("Update existing enum property value"), TEXT("byte_equal"), TEXT("Covers enum write-value through an existing enum-backed property fixture.")),
        MakeOperationWithErrorContains(TEXT("enum_write_value_missing"), TEXT("enum write-value"), TEXT("{\"export\":\"1\",\"name\":\"NewEnumerator0\",\"value\":\"Up\"}"), TEXT("Reject enum write-value when editable property path is absent"), TEXT("error_equal"), TEXT("no editable path matched"), TEXT("Enum write-value currently targets existing enum-backed properties only.")),
        MakeOperation(TEXT("enum_write_value_numeric"), TEXT("enum write-value"), TEXT("{\"export\":\"5\",\"name\":\"FixtureEnum\",\"value\":\"1\"}"), TEXT("Update existing enum property value by numeric literal"), TEXT("byte_equal"), TEXT("Covers enum write-value numeric variant.")),
        MakeOperationWithErrorContains(TEXT("level_var_set"), TEXT("level var-set"), TEXT("{\"actor\":\"LyraWorldSettings\",\"path\":\"NavigationSystemConfig\",\"value\":\"0\"}"), TEXT("Reject placed-actor NavigationSystemConfig rewrite because UE save also compacts dependent graph state"), TEXT("error_equal"), TEXT("not supported by level var-set"), TEXT("Level var-set currently rejects NavigationSystemConfig because UE save also compacts related import/export/name state.")),
        MakeOperationWithErrorContains(TEXT("level_var_set_export_selector"), TEXT("level var-set"), TEXT("{\"actor\":\"4\",\"path\":\"NavigationSystemConfig\",\"value\":\"0\"}"), TEXT("Reject placed-actor NavigationSystemConfig rewrite when selected by export index"), TEXT("error_equal"), TEXT("not supported by level var-set"), TEXT("Level var-set currently rejects NavigationSystemConfig even when selected by export index.")),
        MakeOperationWithErrorContains(TEXT("level_var_set_path_selector"), TEXT("level var-set"), TEXT("{\"actor\":\"PersistentLevel.LyraWorldSettings\",\"path\":\"NavigationSystemConfig\",\"value\":\"0\"}"), TEXT("Reject placed-actor NavigationSystemConfig rewrite when selected by full object path"), TEXT("error_equal"), TEXT("not supported by level var-set"), TEXT("Level var-set currently rejects NavigationSystemConfig even when selected by full object path.")),
        MakeOperation(TEXT("localization_rekey"), TEXT("localization rekey"), TEXT("{\"from-key\":\"Default\",\"namespace\":\"SCS\",\"to-key\":\"MainMenu\"}"), TEXT("rekey localization IDs within namespace"), TEXT("byte_equal"), TEXT("Updates TextProperty and GatherableTextData keys")),
        MakeOperation(TEXT("localization_rewrite_namespace"), TEXT("localization rewrite-namespace"), TEXT("{\"from\":\"SCS\",\"to\":\"UI\"}"), TEXT("rewrite localization namespace across text sources"), TEXT("byte_equal"), TEXT("Updates TextProperty and GatherableTextData entries")),
        MakeOperation(TEXT("localization_set_id"), TEXT("localization set-id"), TEXT("{\"export\":11,\"path\":\"CategoryName\",\"namespace\":\"UI\",\"key\":\"BTN_OK\"}"), TEXT("Update existing TextProperty localization id"), TEXT("byte_equal"), TEXT("Existing TextProperty/StringTable-reference id rewrite.")),
        MakeOperation(TEXT("localization_set_id_base_text"), TEXT("localization set-id"), TEXT("{\"export\":\"11\",\"path\":\"CategoryName\",\"namespace\":\"UI\",\"key\":\"BTN_OK\"}"), TEXT("Update Base TextProperty localization id"), TEXT("byte_equal"), TEXT("Existing Base TextProperty namespace/key rewrite.")),
        MakeOperation(TEXT("localization_set_source"), TEXT("localization set-source"), TEXT("{\"export\":11,\"path\":\"CategoryName\",\"value\":\"Gameplay\"}"), TEXT("Update existing TextProperty source string"), TEXT("byte_equal"), TEXT("Existing TextProperty source-string rewrite.")),
        MakeOperation(TEXT("localization_set_source_unicode"), TEXT("localization set-source"), TEXT("{\"export\":\"11\",\"path\":\"CategoryName\",\"value\":\"ゲームプレイ\"}"), TEXT("Update existing TextProperty source string to unicode text"), TEXT("byte_equal"), TEXT("Existing TextProperty source-string unicode rewrite.")),
        MakeOperation(TEXT("localization_set_stringtable_ref"), TEXT("localization set-stringtable-ref"), TEXT("{\"export\":11,\"path\":\"CategoryName\",\"table\":\"SimpleConstructionScript\",\"key\":\"BTN_OK\"}"), TEXT("Convert existing TextProperty to StringTable reference"), TEXT("byte_equal"), TEXT("Existing TextProperty conversion to StringTableEntry form.")),
        MakeOperationWithErrorContains(TEXT("localization_set_stringtable_ref_missing_table"), TEXT("localization set-stringtable-ref"), TEXT("{\"export\":\"11\",\"path\":\"CategoryName\",\"table\":\"UI.Common\",\"key\":\"BTN_OK\"}"), TEXT("Reject StringTable ref conversion when table name is missing from NameMap"), TEXT("error_equal"), TEXT("is not present in NameMap"), TEXT("StringTable ref update must fail when NameMap growth would be required.")),
        MakeOperation(TEXT("metadata_set_object"), TEXT("metadata set-object"), TEXT("{\"export\":11,\"import\":10,\"key\":\"CategoryName\",\"value\":\"Gameplay\"}"), TEXT("Update object metadata fallback path on SCS node metadata fixture"), TEXT("byte_equal"), TEXT("Covers metadata set-object command path.")),
        MakeOperation(TEXT("metadata_set_object_unicode"), TEXT("metadata set-object"), TEXT("{\"export\":\"11\",\"import\":\"10\",\"key\":\"CategoryName\",\"value\":\"ゲームプレイ\"}"), TEXT("Update object metadata fallback path with unicode text"), TEXT("byte_equal"), TEXT("Covers metadata set-object unicode text rewrite.")),
        MakeOperation(TEXT("name_add"), TEXT("name add"), TEXT("{\"value\":\"BPX_OpName\"}"), TEXT("Append one NameMap entry"), TEXT("byte_equal"), TEXT("Low-level NameMap append fixture.")),
        MakeOperation(TEXT("name_add_hash_override"), TEXT("name add"), TEXT("{\"value\":\"BPX_OpNameHash\",\"non-case-hash\":\"1234\",\"case-preserving-hash\":\"5678\"}"), TEXT("Append one NameMap entry with explicit stored hashes"), TEXT("byte_equal"), TEXT("Low-level NameMap append fixture with hash override.")),
        MakeOperation(TEXT("name_remove"), TEXT("name remove"), TEXT("{\"index\":103}"), TEXT("Remove one safe tail NameMap entry"), TEXT("byte_equal"), TEXT("Low-level NameMap tail removal fixture.")),
        MakeOperationWithErrorContains(TEXT("name_remove_non_tail_reject"), TEXT("name remove"), TEXT("{\"index\":\"1\"}"), TEXT("Reject non-tail NameMap removal"), TEXT("error_equal"), TEXT("tail entry only"), TEXT("NameMap remove must reject non-tail indices.")),
        MakeOperationWithErrorContains(TEXT("name_remove_referenced_reject"), TEXT("name remove"), TEXT("{\"index\":\"102\"}"), TEXT("Reject referenced tail NameMap removal"), TEXT("error_equal"), TEXT("still referenced"), TEXT("NameMap remove must reject referenced tail entries.")),
        MakeOperation(TEXT("name_set"), TEXT("name set"), TEXT("{\"index\":1,\"value\":\"BP_Empty_Renamed\"}"), TEXT("Update one NameMap entry in place"), TEXT("byte_equal"), TEXT("Low-level NameMap set fixture.")),
        MakeOperation(TEXT("name_set_hash_override"), TEXT("name set"), TEXT("{\"index\":\"1\",\"value\":\"BP_Empty_ManualHash\",\"non-case-hash\":\"2345\",\"case-preserving-hash\":\"6789\"}"), TEXT("Update one NameMap entry with explicit stored hashes"), TEXT("byte_equal"), TEXT("Low-level NameMap set fixture with hash override.")),
        MakeOperation(TEXT("package_set_flags_raw"), TEXT("package set-flags"), TEXT("{\"flags\":\"537133056\"}"), TEXT("Set package flags using raw numeric value without touching shape-sensitive bits"), TEXT("byte_equal"), TEXT("Raw numeric package flag update fixture.")),
        MakeOperation(TEXT("stringtable_remove_entry"), TEXT("stringtable remove-entry"), TEXT("{\"export\":1,\"key\":\"BTN_CANCEL\"}"), TEXT("remove one key from string table"), TEXT("byte_equal"), TEXT("Serialized FStringTable payload rewrite")),
        MakeOperation(TEXT("stringtable_set_namespace"), TEXT("stringtable set-namespace"), TEXT("{\"export\":1,\"namespace\":\"UI.Common\"}"), TEXT("set string table namespace"), TEXT("byte_equal"), TEXT("Serialized FStringTable namespace rewrite")),
        MakeOperation(TEXT("stringtable_write_entry"), TEXT("stringtable write-entry"), TEXT("{\"export\":1,\"key\":\"BTN_OK\",\"value\":\"Confirm\"}"), TEXT("Update one StringTable entry value"), TEXT("byte_equal"), TEXT("Serialized StringTable entry rewrite.")),
        MakeOperation(TEXT("stringtable_write_entry_unicode"), TEXT("stringtable write-entry"), TEXT("{\"export\":\"1\",\"key\":\"BTN_CANCEL\",\"value\":\"キャンセル\"}"), TEXT("Update one StringTable entry value to unicode text"), TEXT("byte_equal"), TEXT("Serialized StringTable entry unicode rewrite.")),
        MakeOperation(TEXT("write_roundtrip"), TEXT("write"), TEXT("{\"out\":\"{TARGET}.out.uasset\"}"), TEXT("Write package bytes to a new output file without edits"), TEXT("byte_equal"), TEXT("Round-trip write output should stay byte-identical.")),
        MakeOperation(TEXT("write_roundtrip_umap"), TEXT("write"), TEXT("{\"out\":\"{TARGET}.out.umap\"}"), TEXT("Write map package bytes to a new output file without edits"), TEXT("byte_equal"), TEXT("Round-trip write output for .umap should stay byte-identical.")),
        MakeOperation(TEXT("write_roundtrip_bp_withmetadata"), TEXT("write"), TEXT("{\"out\":\"{TARGET}.out.uasset\"}"), TEXT("Write blueprint package bytes to a new output file without edits"), TEXT("byte_equal"), TEXT("Round-trip write output for BP_WithMetadata should stay byte-identical.")),
        MakeOperation(TEXT("write_roundtrip_dt_simple"), TEXT("write"), TEXT("{\"out\":\"{TARGET}.out.uasset\"}"), TEXT("Write DataTable package bytes to a new output file without edits"), TEXT("byte_equal"), TEXT("Round-trip write output for DT_Simple should stay byte-identical.")),
        MakeOperation(TEXT("write_roundtrip_st_ui"), TEXT("write"), TEXT("{\"out\":\"{TARGET}.out.uasset\"}"), TEXT("Write StringTable package bytes to a new output file without edits"), TEXT("byte_equal"), TEXT("Round-trip write output for ST_UI should stay byte-identical.")),
        MakeOperation(TEXT("write_roundtrip_bp_withfunctions"), TEXT("write"), TEXT("{\"out\":\"{TARGET}.out.uasset\"}"), TEXT("Write blueprint-with-functions package bytes to a new output file without edits"), TEXT("byte_equal"), TEXT("Round-trip write output for BP_WithFunctions should stay byte-identical.")),
        MakeOperation(TEXT("write_roundtrip_bp_empty_stringtable_ref"), TEXT("write"), TEXT("{\"out\":\"{TARGET}.out.uasset\"}"), TEXT("Write stringtable-reference blueprint package bytes to a new output file without edits"), TEXT("byte_equal"), TEXT("Round-trip write output for BP_Empty_StringTableRef should stay byte-identical.")),
        MakeOperation(TEXT("package_set_flags_runtimegenerated"), TEXT("package set-flags"), TEXT("{\"flags\":\"PKG_RUNTIMEGENERATED\"}"), TEXT("Set package flags to runtime-generated only"), TEXT("byte_equal"), TEXT("Symbolic single-flag package update fixture.")),
        MakeOperation(TEXT("package_set_flags_clear_zero"), TEXT("package set-flags"), TEXT("{\"flags\":\"0\"}"), TEXT("Clear non-shape-sensitive package flags using raw zero"), TEXT("byte_equal"), TEXT("Raw zero package flag update fixture.")),
        MakeOperationWithErrorContains(TEXT("package_set_flags_filtereditoronly_reject"), TEXT("package set-flags"), TEXT("{\"flags\":\"PKG_FILTEREDITORONLY\"}"), TEXT("Reject shape-sensitive PKG_FilterEditorOnly update"), TEXT("error_equal"), TEXT("not supported"), TEXT("Package flag update must reject PKG_FilterEditorOnly.")),
        MakeOperationWithErrorContains(TEXT("package_set_flags_unversionedprops_reject"), TEXT("package set-flags"), TEXT("{\"flags\":\"PKG_UNVERSIONEDPROPERTIES\"}"), TEXT("Reject shape-sensitive PKG_UnversionedProperties update"), TEXT("error_equal"), TEXT("not supported"), TEXT("Package flag update must reject PKG_UnversionedProperties.")),
        MakeOperation(TEXT("name_add_unicode"), TEXT("name add"), TEXT("{\"value\":\"名前追加\"}"), TEXT("Append one unicode NameMap entry"), TEXT("byte_equal"), TEXT("Low-level unicode NameMap append fixture.")),
        MakeOperation(TEXT("name_add_long_ascii"), TEXT("name add"), TEXT("{\"value\":\"BPX_Operation_Name_Long\"}"), TEXT("Append one long ASCII NameMap entry"), TEXT("byte_equal"), TEXT("Low-level long ASCII NameMap append fixture.")),
        MakeOperationWithErrorContains(TEXT("name_add_duplicate_reject"), TEXT("name add"), TEXT("{\"value\":\"None\"}"), TEXT("Reject duplicate NameMap append"), TEXT("error_equal"), TEXT("already exists"), TEXT("NameMap add must reject duplicate values.")),
        MakeOperation(TEXT("name_add_hash_override_alt"), TEXT("name add"), TEXT("{\"value\":\"BPX_ManualHashAlt\",\"non-case-hash\":\"4321\",\"case-preserving-hash\":\"8765\"}"), TEXT("Append one NameMap entry with alternate explicit stored hashes"), TEXT("byte_equal"), TEXT("Low-level NameMap append fixture with alternate hash override.")),
        MakeOperation(TEXT("name_set_unicode"), TEXT("name set"), TEXT("{\"index\":\"1\",\"value\":\"名前変更\"}"), TEXT("Update one NameMap entry to unicode text"), TEXT("byte_equal"), TEXT("Low-level unicode NameMap set fixture.")),
        MakeOperation(TEXT("name_set_hash_only"), TEXT("name set"), TEXT("{\"index\":\"1\",\"value\":\"/Script/CoreUObject\",\"non-case-hash\":\"1111\",\"case-preserving-hash\":\"2222\"}"), TEXT("Update one NameMap entry hash fields without changing stored text"), TEXT("byte_equal"), TEXT("Low-level NameMap hash-only rewrite fixture.")),
        MakeOperation(TEXT("name_set_hash_override_alt"), TEXT("name set"), TEXT("{\"index\":\"2\",\"value\":\"BPX_Name_Alt\",\"non-case-hash\":\"3333\",\"case-preserving-hash\":\"4444\"}"), TEXT("Update one NameMap entry with alternate explicit hashes"), TEXT("byte_equal"), TEXT("Low-level NameMap set fixture with alternate hash override.")),
        MakeOperation(TEXT("name_set_case_variant"), TEXT("name set"), TEXT("{\"index\":\"3\",\"value\":\"core_redirects_case\"}"), TEXT("Update one NameMap entry to a new ASCII case variant"), TEXT("byte_equal"), TEXT("Low-level NameMap case-variant rewrite fixture.")),
        MakeOperation(TEXT("name_set_ascii_alt"), TEXT("name set"), TEXT("{\"index\":\"4\",\"value\":\"BPX_Name_Ascii\"}"), TEXT("Update one NameMap entry to alternate ASCII text"), TEXT("byte_equal"), TEXT("Low-level ASCII NameMap set fixture.")),
        MakeOperation(TEXT("metadata_set_category_unicode"), TEXT("metadata set-root"), TEXT("{\"export\":\"11\",\"key\":\"CategoryName\",\"value\":\"ゲームプレイ\"}"), TEXT("Update root category metadata to unicode text"), TEXT("byte_equal"), TEXT("Unicode root metadata rewrite fixture.")),
        MakeOperation(TEXT("metadata_set_category_ascii_alt"), TEXT("metadata set-root"), TEXT("{\"export\":\"11\",\"key\":\"CategoryName\",\"value\":\"UI\"}"), TEXT("Update root category metadata to alternate ASCII text"), TEXT("byte_equal"), TEXT("Alternate ASCII root metadata rewrite fixture.")),
        MakeOperation(TEXT("metadata_set_object_empty"), TEXT("metadata set-object"), TEXT("{\"export\":\"11\",\"import\":\"10\",\"key\":\"CategoryName\",\"value\":\"\"}"), TEXT("Update object metadata fallback path to empty text"), TEXT("byte_equal"), TEXT("Object metadata fallback rewrite to empty text.")),
        MakeOperationWithErrorContains(TEXT("metadata_set_object_tooltip_reject"), TEXT("metadata set-object"), TEXT("{\"export\":\"1\",\"import\":\"10\",\"key\":\"ToolTip\",\"value\":\"Updated\"}"), TEXT("Reject object metadata update when editable path is absent"), TEXT("error_equal"), TEXT("no editable path matched"), TEXT("Object metadata update must fail when no fallback path exists.")),
        MakeOperation(TEXT("enum_write_value_anchor"), TEXT("enum write-value"), TEXT("{\"export\":\"5\",\"name\":\"FixtureEnumAnchor\",\"value\":\"BPXEnum_ValueA\"}"), TEXT("Update existing secondary enum property value"), TEXT("byte_equal"), TEXT("Enum write-value fixture on FixtureEnumAnchor.")),
        MakeOperation(TEXT("enum_write_value_anchor_alt"), TEXT("enum write-value"), TEXT("{\"export\":\"5\",\"name\":\"FixtureEnumAnchorAlt\",\"value\":\"BPXEnum_ValueB\"}"), TEXT("Update existing alternate enum property value"), TEXT("byte_equal"), TEXT("Enum write-value fixture on FixtureEnumAnchorAlt.")),
        MakeOperation(TEXT("enum_write_value_numeric_zero"), TEXT("enum write-value"), TEXT("{\"export\":\"5\",\"name\":\"FixtureEnum\",\"value\":\"0\"}"), TEXT("Update existing enum property value by numeric zero literal"), TEXT("byte_equal"), TEXT("Enum write-value numeric zero variant.")),
        MakeOperation(TEXT("enum_write_value_numeric_two"), TEXT("enum write-value"), TEXT("{\"export\":\"5\",\"name\":\"FixtureEnum\",\"value\":\"2\"}"), TEXT("Update existing enum property value by numeric two literal"), TEXT("byte_equal"), TEXT("Enum write-value numeric two variant.")),
        MakeOperation(TEXT("stringtable_write_entry_btn_start"), TEXT("stringtable write-entry"), TEXT("{\"export\":\"1\",\"key\":\"BTN_START\",\"value\":\"Begin\"}"), TEXT("Update BTN_START StringTable entry value"), TEXT("byte_equal"), TEXT("Serialized StringTable BTN_START rewrite.")),
        MakeOperation(TEXT("stringtable_write_entry_title_unicode"), TEXT("stringtable write-entry"), TEXT("{\"export\":\"1\",\"key\":\"LBL_TITLE\",\"value\":\"テスト題名\"}"), TEXT("Update title StringTable entry to unicode text"), TEXT("byte_equal"), TEXT("Serialized StringTable title unicode rewrite.")),
        MakeOperation(TEXT("stringtable_remove_entry_btn_start"), TEXT("stringtable remove-entry"), TEXT("{\"export\":\"1\",\"key\":\"BTN_START\"}"), TEXT("Remove BTN_START key from string table"), TEXT("byte_equal"), TEXT("Serialized StringTable BTN_START removal.")),
        MakeOperationWithErrorContains(TEXT("stringtable_remove_entry_missing_reject"), TEXT("stringtable remove-entry"), TEXT("{\"export\":\"1\",\"key\":\"BTN_MISSING\"}"), TEXT("Reject string table entry removal when key is absent"), TEXT("error_equal"), TEXT("string table key not found"), TEXT("StringTable remove-entry must fail for missing keys.")),
        MakeOperation(TEXT("stringtable_set_namespace_alt"), TEXT("stringtable set-namespace"), TEXT("{\"export\":\"1\",\"namespace\":\"UI.Menu\"}"), TEXT("Set string table namespace to alternate ASCII value"), TEXT("byte_equal"), TEXT("Serialized StringTable alternate namespace rewrite.")),
        MakeOperation(TEXT("stringtable_set_namespace_unicode"), TEXT("stringtable set-namespace"), TEXT("{\"export\":\"1\",\"namespace\":\"UI.共通\"}"), TEXT("Set string table namespace to unicode value"), TEXT("byte_equal"), TEXT("Serialized StringTable unicode namespace rewrite.")),
        MakeOperation(TEXT("localization_set_source_alt_ascii"), TEXT("localization set-source"), TEXT("{\"export\":\"11\",\"path\":\"CategoryName\",\"value\":\"HUD\"}"), TEXT("Update existing TextProperty source string to alternate ASCII text"), TEXT("byte_equal"), TEXT("Alternate ASCII TextProperty source-string rewrite.")),
        MakeOperation(TEXT("localization_set_source_empty"), TEXT("localization set-source"), TEXT("{\"export\":\"11\",\"path\":\"CategoryName\",\"value\":\"\"}"), TEXT("Update existing TextProperty source string to empty text"), TEXT("byte_equal"), TEXT("Empty TextProperty source-string rewrite.")),
        MakeOperation(TEXT("localization_set_id_alt_key"), TEXT("localization set-id"), TEXT("{\"export\":\"11\",\"path\":\"CategoryName\",\"namespace\":\"UI\",\"key\":\"BTN_CANCEL\"}"), TEXT("Update existing StringTable-reference TextProperty key"), TEXT("byte_equal"), TEXT("StringTable-reference localization id rewrite to BTN_CANCEL.")),
        MakeOperation(TEXT("localization_set_id_base_text_alt"), TEXT("localization set-id"), TEXT("{\"export\":\"11\",\"path\":\"CategoryName\",\"namespace\":\"UI\",\"key\":\"HUD_TITLE\"}"), TEXT("Update Base TextProperty localization id to alternate key"), TEXT("byte_equal"), TEXT("Base TextProperty localization id rewrite to HUD_TITLE.")),
        MakeOperation(TEXT("localization_set_stringtable_ref_alt_key"), TEXT("localization set-stringtable-ref"), TEXT("{\"export\":\"11\",\"path\":\"CategoryName\",\"table\":\"SimpleConstructionScript\",\"key\":\"BTN_CANCEL\"}"), TEXT("Convert existing TextProperty to StringTable reference with alternate key"), TEXT("byte_equal"), TEXT("TextProperty conversion to StringTableEntry using BTN_CANCEL.")),
        MakeOperationWithErrorContains(TEXT("localization_set_stringtable_ref_missing_table_alt"), TEXT("localization set-stringtable-ref"), TEXT("{\"export\":\"11\",\"path\":\"CategoryName\",\"table\":\"UI.Menu\",\"key\":\"BTN_OK\"}"), TEXT("Reject StringTable ref conversion when alternate table name is missing from NameMap"), TEXT("error_equal"), TEXT("is not present in NameMap"), TEXT("StringTable ref update must fail when alternate table NameMap growth would be required.")),
        MakeOperation(TEXT("localization_rewrite_namespace_alt"), TEXT("localization rewrite-namespace"), TEXT("{\"from\":\"SCS\",\"to\":\"HUD\"}"), TEXT("Rewrite localization namespace across text sources to HUD"), TEXT("byte_equal"), TEXT("Namespace rewrite variant from SCS to HUD.")),
        MakeOperation(TEXT("localization_rekey_alt"), TEXT("localization rekey"), TEXT("{\"namespace\":\"SCS\",\"from-key\":\"Default\",\"to-key\":\"HUDTitle\"}"), TEXT("Rekey localization IDs within namespace to HUDTitle"), TEXT("byte_equal"), TEXT("Localization rekey variant to HUDTitle.")),
        MakeOperationWithErrorContains(TEXT("level_var_set_missing_actor_reject"), TEXT("level var-set"), TEXT("{\"actor\":\"MissingActor\",\"path\":\"NavigationSystemConfig\",\"value\":\"0\"}"), TEXT("Reject level var-set when actor selector is absent"), TEXT("error_equal"), TEXT("actor not found"), TEXT("Level var-set must fail for missing actor selectors.")),
        MakeOperationWithErrorContains(TEXT("level_var_set_bad_path_reject"), TEXT("level var-set"), TEXT("{\"actor\":\"LyraWorldSettings\",\"path\":\"MissingProp\",\"value\":\"0\"}"), TEXT("Reject level var-set when property path is absent"), TEXT("error_equal"), TEXT("property not found"), TEXT("Level var-set must fail for missing property paths.")),
        MakeOperationWithErrorContains(TEXT("level_var_set_export_selector_reject"), TEXT("level var-set"), TEXT("{\"actor\":\"999\",\"path\":\"NavigationSystemConfig\",\"value\":\"0\"}"), TEXT("Reject level var-set when export-index selector is out of range"), TEXT("error_equal"), TEXT("export index out of range"), TEXT("Level var-set must fail for invalid export selectors.")),
        MakeOperationWithErrorContains(TEXT("level_var_set_path_selector_reject"), TEXT("level var-set"), TEXT("{\"actor\":\"PersistentLevel.Missing\",\"path\":\"NavigationSystemConfig\",\"value\":\"0\"}"), TEXT("Reject level var-set when full object-path selector is absent"), TEXT("error_equal"), TEXT("actor not found"), TEXT("Level var-set must fail for missing full-path selectors.")),
        MakeOperation(TEXT("name_remove_unicode"), TEXT("name remove"), TEXT("{\"index\":\"103\"}"), TEXT("Remove one safe unicode tail NameMap entry"), TEXT("byte_equal"), TEXT("Low-level unicode NameMap tail removal fixture.")),
        MakeOperation(TEXT("name_remove_hash"), TEXT("name remove"), TEXT("{\"index\":\"103\"}"), TEXT("Remove one safe ASCII tail NameMap entry after add"), TEXT("byte_equal"), TEXT("Low-level NameMap tail removal fixture for freshly added ASCII tail.")),
        MakeOperationWithErrorContains(TEXT("name_remove_export_region_reject"), TEXT("name remove"), TEXT("{\"index\":\"10\"}"), TEXT("Reject NameMap removal inside export-data reserved region"), TEXT("error_equal"), TEXT("NamesReferencedFromExportData"), TEXT("NameMap remove must reject indices inside NamesReferencedFromExportDataCount.")),
        MakeOperationWithErrorContains(TEXT("name_remove_non_tail_reject_alt"), TEXT("name remove"), TEXT("{\"index\":\"5\"}"), TEXT("Reject alternate non-tail NameMap removal"), TEXT("error_equal"), TEXT("tail entry only"), TEXT("NameMap remove must reject alternate non-tail indices.")),
        MakeOperationWithErrorContains(TEXT("name_remove_referenced_reject_alt"), TEXT("name remove"), TEXT("{\"index\":\"101\"}"), TEXT("Reject alternate referenced NameMap removal"), TEXT("error_equal"), TEXT("still referenced"), TEXT("NameMap remove must reject alternate referenced tail entries.")),
        MakeOperationWithErrorContains(TEXT("enum_write_value_to_c_reject"), TEXT("enum write-value"), TEXT("{\"export\":\"5\",\"name\":\"FixtureEnum\",\"value\":\"BPXEnum_ValueC\"}"), TEXT("Reject enum write-value when target enum name is missing from NameMap"), TEXT("error_equal"), TEXT("not present in NameMap"), TEXT("Enum write-value must fail when NameMap growth would be required."))
    };
}

bool IsSinglePackageOperation(const FOperationFixtureSpec& Spec)
{
    if (Spec.Name == TEXT("prop_add")
        || Spec.Name == TEXT("prop_add_fixture_int")
        || Spec.Name == TEXT("prop_remove")
        || Spec.Name == TEXT("prop_remove_fixture_int"))
    {
        return true;
    }

    return Spec.Name == TEXT("prop_set_enum")
        || Spec.Name == TEXT("prop_set_enum_numeric")
        || Spec.Name == TEXT("prop_set_enum_anchor")
        || Spec.Name == TEXT("prop_set_int")
        || Spec.Name == TEXT("prop_set_int_negative")
        || Spec.Name == TEXT("prop_set_int_max")
        || Spec.Name == TEXT("prop_set_int_min")
        || Spec.Name == TEXT("prop_set_int64")
        || Spec.Name == TEXT("prop_set_float")
        || Spec.Name == TEXT("prop_set_float_special")
        || Spec.Name == TEXT("prop_set_double")
        || Spec.Name == TEXT("prop_set_string_same_len")
        || Spec.Name == TEXT("prop_set_string_diff_len")
        || Spec.Name == TEXT("prop_set_string_empty")
        || Spec.Name == TEXT("prop_set_string_unicode")
        || Spec.Name == TEXT("prop_set_string_long_expand")
        || Spec.Name == TEXT("prop_set_string_shrink")
        || Spec.Name == TEXT("prop_set_name")
        || Spec.Name == TEXT("prop_set_vector")
        || Spec.Name == TEXT("prop_set_vector_axis_x")
        || Spec.Name == TEXT("prop_set_rotator")
        || Spec.Name == TEXT("prop_set_rotator_axis_roll")
        || Spec.Name == TEXT("prop_set_array_element")
        || Spec.Name == TEXT("prop_set_array_replace_longer")
        || Spec.Name == TEXT("prop_set_array_replace_empty")
        || Spec.Name == TEXT("prop_set_map_value")
        || Spec.Name == TEXT("prop_set_custom_struct_int")
        || Spec.Name == TEXT("prop_set_custom_struct_enum")
        || Spec.Name == TEXT("enum_write_value")
        || Spec.Name == TEXT("enum_write_value_missing")
        || Spec.Name == TEXT("enum_write_value_numeric")
        || Spec.Name == TEXT("enum_write_value_anchor")
        || Spec.Name == TEXT("enum_write_value_anchor_alt")
        || Spec.Name == TEXT("enum_write_value_numeric_zero")
        || Spec.Name == TEXT("enum_write_value_numeric_two")
        || Spec.Name == TEXT("enum_write_value_to_c_reject")
        || Spec.Name == TEXT("export_set_header")
        || Spec.Name == TEXT("var_set_default_int")
        || Spec.Name == TEXT("var_set_default_empty")
        || Spec.Name == TEXT("var_set_default_unicode")
        || Spec.Name == TEXT("var_set_default_long")
        || Spec.Name == TEXT("var_rename_simple")
        || Spec.Name == TEXT("var_rename_with_refs")
        || Spec.Name == TEXT("var_rename_unicode")
        || Spec.Name == TEXT("ref_rewrite_single")
        || Spec.Name == TEXT("ref_rewrite_multi");
}

bool UsesNativeOperationFixtureParent(const FOperationFixtureSpec& Spec)
{
    return Spec.Name == TEXT("prop_set_int")
        || Spec.Name == TEXT("prop_set_enum")
        || Spec.Name == TEXT("prop_set_enum_numeric")
        || Spec.Name == TEXT("prop_set_enum_anchor")
        || Spec.Name == TEXT("prop_set_int_negative")
        || Spec.Name == TEXT("prop_set_int_max")
        || Spec.Name == TEXT("prop_set_int_min")
        || Spec.Name == TEXT("prop_set_int64")
        || Spec.Name == TEXT("prop_set_float")
        || Spec.Name == TEXT("prop_set_float_special")
        || Spec.Name == TEXT("prop_set_double")
        || Spec.Name == TEXT("prop_set_string_same_len")
        || Spec.Name == TEXT("prop_set_string_diff_len")
        || Spec.Name == TEXT("prop_set_string_empty")
        || Spec.Name == TEXT("prop_set_string_unicode")
        || Spec.Name == TEXT("prop_set_string_long_expand")
        || Spec.Name == TEXT("prop_set_string_shrink")
        || Spec.Name == TEXT("prop_set_name")
        || Spec.Name == TEXT("prop_set_vector")
        || Spec.Name == TEXT("prop_set_vector_axis_x")
        || Spec.Name == TEXT("prop_set_rotator")
        || Spec.Name == TEXT("prop_set_rotator_axis_roll")
        || Spec.Name == TEXT("prop_set_array_element")
        || Spec.Name == TEXT("prop_set_array_replace_longer")
        || Spec.Name == TEXT("prop_set_array_replace_empty")
        || Spec.Name == TEXT("prop_set_map_value")
        || Spec.Name == TEXT("prop_set_custom_struct_int")
        || Spec.Name == TEXT("prop_set_custom_struct_enum")
        || Spec.Name == TEXT("enum_write_value")
        || Spec.Name == TEXT("enum_write_value_missing")
        || Spec.Name == TEXT("enum_write_value_numeric")
        || Spec.Name == TEXT("enum_write_value_anchor")
        || Spec.Name == TEXT("enum_write_value_anchor_alt")
        || Spec.Name == TEXT("enum_write_value_numeric_zero")
        || Spec.Name == TEXT("enum_write_value_numeric_two")
        || Spec.Name == TEXT("enum_write_value_to_c_reject")
        || Spec.Name == TEXT("var_set_default_int")
        || Spec.Name == TEXT("var_set_default_empty")
        || Spec.Name == TEXT("var_set_default_unicode")
        || Spec.Name == TEXT("var_set_default_long")
        || Spec.Name == TEXT("prop_add_fixture_int")
        || Spec.Name == TEXT("prop_remove_fixture_int");
}

FString ParseBlueprintFixtureKeyForOperation(const FOperationFixtureSpec& Spec)
{
    if (Spec.Name == TEXT("prop_set_bool")
        || Spec.Name == TEXT("prop_set_text")
        || Spec.Name == TEXT("metadata_set_category")
        || Spec.Name == TEXT("metadata_set_category_unicode")
        || Spec.Name == TEXT("metadata_set_category_ascii_alt")
        || Spec.Name == TEXT("var_set_default_string"))
    {
        return TEXT("BP_AllScalarTypes");
    }

    if (Spec.Name == TEXT("prop_set_color")
        || Spec.Name == TEXT("prop_set_transform")
        || Spec.Name == TEXT("prop_set_nested_struct")
        || Spec.Name == TEXT("var_set_default_vector"))
    {
        return TEXT("BP_MathTypes");
    }

    if (Spec.Name == TEXT("prop_set_soft_object")
        || Spec.Name == TEXT("prop_set_nested_array_struct")
        || Spec.Name == TEXT("metadata_set_tooltip")
        || Spec.Name == TEXT("metadata_set_object")
        || Spec.Name == TEXT("metadata_set_object_unicode")
        || Spec.Name == TEXT("metadata_set_object_empty")
        || Spec.Name == TEXT("metadata_set_object_tooltip_reject")
        || Spec.Name == TEXT("localization_set_source")
        || Spec.Name == TEXT("localization_set_source_unicode")
        || Spec.Name == TEXT("localization_set_source_alt_ascii")
        || Spec.Name == TEXT("localization_set_source_empty")
        || Spec.Name == TEXT("localization_set_id_base_text")
        || Spec.Name == TEXT("localization_set_id_base_text_alt")
        || Spec.Name == TEXT("localization_set_stringtable_ref")
        || Spec.Name == TEXT("localization_set_stringtable_ref_alt_key")
        || Spec.Name == TEXT("localization_set_stringtable_ref_missing_table")
        || Spec.Name == TEXT("localization_set_stringtable_ref_missing_table_alt")
        || Spec.Name == TEXT("localization_rekey")
        || Spec.Name == TEXT("localization_rekey_alt")
        || Spec.Name == TEXT("localization_rewrite_namespace")
        || Spec.Name == TEXT("localization_rewrite_namespace_alt"))
    {
        return TEXT("BP_WithMetadata");
    }

    if (Spec.Name == TEXT("localization_set_id")
        || Spec.Name == TEXT("localization_set_id_alt_key"))
    {
        return TEXT("BP_Empty_StringTableRef");
    }

    if (Spec.Name == TEXT("package_set_flags")
        || Spec.Name == TEXT("package_set_flags_raw")
        || Spec.Name == TEXT("package_set_flags_runtimegenerated")
        || Spec.Name == TEXT("package_set_flags_clear_zero")
        || Spec.Name == TEXT("package_set_flags_filtereditoronly_reject")
        || Spec.Name == TEXT("package_set_flags_unversionedprops_reject"))
    {
        return TEXT("BP_Empty");
    }

    return FString();
}

FString ParseWidgetFixtureKeyForOperation(const FOperationFixtureSpec& Spec)
{
    if (Spec.Name == TEXT("widget_write_text_root_textblock"))
    {
        return TEXT("WBP_TextBlock");
    }
    if (Spec.Name == TEXT("widget_parent_class_commonactivatablewidget_rootless"))
    {
        return TEXT("WBP_RootlessParentClass");
    }
    if (Spec.Name == TEXT("widget_write_text_color_root_textblock")
        || Spec.Name == TEXT("widget_write_text_font_size_root_textblock")
        || Spec.Name == TEXT("widget_write_text_justification_root_textblock")
        || Spec.Name == TEXT("widget_write_text_font_root_textblock")
        || Spec.Name == TEXT("widget_write_text_typeface_root_textblock")
        || Spec.Name == TEXT("widget_write_text_auto_wrap_text_root_textblock")
        || Spec.Name == TEXT("widget_write_text_wrap_text_at_root_textblock")
        || Spec.Name == TEXT("widget_write_text_line_height_percentage_root_textblock")
        || Spec.Name == TEXT("widget_write_text_shadow_offset_root_textblock")
        || Spec.Name == TEXT("widget_write_text_shadow_color_root_textblock")
        || Spec.Name == TEXT("widget_write_text_outline_size_root_textblock")
        || Spec.Name == TEXT("widget_write_text_outline_color_root_textblock"))
    {
        return TEXT("WBP_TextBlock");
    }
    if (Spec.Name == TEXT("widget_write_text_root_richtextblock")
        || Spec.Name == TEXT("widget_write_opacity_root_richtextblock")
        || Spec.Name == TEXT("widget_write_visibility_root_richtextblock")
        || Spec.Name == TEXT("widget_write_richtext_default_shadow_offset")
        || Spec.Name == TEXT("widget_write_richtext_default_shadow_color")
        || Spec.Name == TEXT("widget_write_richtext_default_outline_size")
        || Spec.Name == TEXT("widget_write_richtext_default_outline_color")
        || Spec.Name == TEXT("widget_write_richtext_auto_wrap_text")
        || Spec.Name == TEXT("widget_write_richtext_wrap_text_at")
        || Spec.Name == TEXT("widget_write_richtext_line_height_percentage"))
    {
        return TEXT("WBP_RichTextBlock");
    }
    if (Spec.Name == TEXT("widget_write_text_canvaspanel_child"))
    {
        return TEXT("WBP_CanvasPanel_TextBlock");
    }
    if (Spec.Name == TEXT("widget_write_progressbar_percent")
        || Spec.Name == TEXT("widget_write_progressbar_fill_color"))
    {
        return TEXT("WBP_CanvasPanel_ProgressBar");
    }
    if (Spec.Name == TEXT("widget_write_slider_value")
        || Spec.Name == TEXT("widget_write_slider_orientation")
        || Spec.Name == TEXT("widget_write_slider_is_focusable"))
    {
        return TEXT("WBP_CanvasPanel_Slider");
    }
    if (Spec.Name == TEXT("widget_write_spacer_size"))
    {
        return TEXT("WBP_CanvasPanel_Spacer");
    }
    if (Spec.Name == TEXT("widget_write_scrollbar_thickness")
        || Spec.Name == TEXT("widget_write_scrollbar_orientation"))
    {
        return TEXT("WBP_CanvasPanel_ScrollBar");
    }
    if (Spec.Name == TEXT("widget_write_checkbox_is_checked_canvaspanel")
        || Spec.Name == TEXT("widget_write_checkbox_checked_state_canvaspanel")
        || Spec.Name == TEXT("widget_write_checkbox_is_focusable_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_CheckBox");
    }
    if (Spec.Name == TEXT("widget_write_text_canvaspanel_child_editabletextbox")
        || Spec.Name == TEXT("widget_write_editabletextbox_hint_text_canvaspanel")
        || Spec.Name == TEXT("widget_write_editabletextbox_is_read_only_canvaspanel")
        || Spec.Name == TEXT("widget_write_editabletextbox_is_password_canvaspanel")
        || Spec.Name == TEXT("widget_write_editabletextbox_minimum_desired_width_canvaspanel")
        || Spec.Name == TEXT("widget_write_editabletextbox_justification_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_EditableTextBox");
    }
    if (Spec.Name == TEXT("widget_write_text_canvaspanel_child_editabletext")
        || Spec.Name == TEXT("widget_write_editabletext_hint_text_canvaspanel")
        || Spec.Name == TEXT("widget_write_editabletext_is_read_only_canvaspanel")
        || Spec.Name == TEXT("widget_write_editabletext_is_password_canvaspanel")
        || Spec.Name == TEXT("widget_write_editabletext_minimum_desired_width_canvaspanel")
        || Spec.Name == TEXT("widget_write_editabletext_justification_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_EditableText");
    }
    if (Spec.Name == TEXT("widget_write_text_canvaspanel_child_multilineeditabletextbox")
        || Spec.Name == TEXT("widget_write_multilineeditabletextbox_hint_text_canvaspanel")
        || Spec.Name == TEXT("widget_write_multilineeditabletextbox_is_read_only_canvaspanel")
        || Spec.Name == TEXT("widget_write_multilineeditabletextbox_justification_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_MultiLineEditableTextBox");
    }
    if (Spec.Name == TEXT("widget_write_spinbox_value_canvaspanel")
        || Spec.Name == TEXT("widget_write_spinbox_min_value_canvaspanel")
        || Spec.Name == TEXT("widget_write_spinbox_max_value_canvaspanel")
        || Spec.Name == TEXT("widget_write_spinbox_delta_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_SpinBox");
    }
    if (Spec.Name == TEXT("widget_write_comboboxstring_options_canvaspanel")
        || Spec.Name == TEXT("widget_write_comboboxstring_selected_option_canvaspanel")
        || Spec.Name == TEXT("widget_write_comboboxstring_is_focusable_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_ComboBoxString");
    }
    if (Spec.Name == TEXT("widget_write_sizebox_width_canvaspanel")
        || Spec.Name == TEXT("widget_write_sizebox_height_canvaspanel")
        || Spec.Name == TEXT("widget_write_sizebox_min_desired_width_canvaspanel")
        || Spec.Name == TEXT("widget_write_sizebox_min_desired_height_canvaspanel")
        || Spec.Name == TEXT("widget_write_sizebox_max_desired_width_canvaspanel")
        || Spec.Name == TEXT("widget_write_sizebox_max_desired_height_canvaspanel")
        || Spec.Name == TEXT("widget_write_sizebox_min_aspect_ratio_canvaspanel")
        || Spec.Name == TEXT("widget_write_sizebox_max_aspect_ratio_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_SizeBox");
    }
    if (Spec.Name == TEXT("widget_write_scrollbox_orientation_canvaspanel")
        || Spec.Name == TEXT("widget_write_scrollbox_scrollbar_visibility_canvaspanel")
        || Spec.Name == TEXT("widget_write_scrollbox_consume_mouse_wheel_canvaspanel")
        || Spec.Name == TEXT("widget_write_scrollbox_is_focusable_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_ScrollBox");
    }
    if (Spec.Name == TEXT("widget_write_button_is_focusable_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_Button");
    }
    if (Spec.Name == TEXT("widget_write_text_canvaspanel_child_richtextblock"))
    {
        return TEXT("WBP_CanvasPanel_RichTextBlock");
    }
    if (Spec.Name == TEXT("widget_remove_textblock_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_TextBlock");
    }
    if (Spec.Name == TEXT("widget_remove_richtextblock_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_RichTextBlock");
    }
    if (Spec.Name == TEXT("widget_remove_textblock_overlay"))
    {
        return TEXT("WBP_Overlay_TextBlock");
    }
    if (Spec.Name == TEXT("widget_remove_image_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_Image");
    }
    if (Spec.Name == TEXT("widget_remove_image_overlay"))
    {
        return TEXT("WBP_Overlay_Image");
    }
    if (Spec.Name == TEXT("widget_remove_image_verticalbox"))
    {
        return TEXT("WBP_VerticalBox_Image");
    }
    if (Spec.Name == TEXT("widget_remove_image_horizontalbox"))
    {
        return TEXT("WBP_HorizontalBox_Image");
    }
    if (Spec.Name == TEXT("widget_remove_button_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_Button");
    }
    if (Spec.Name == TEXT("widget_remove_border_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_Border");
    }
    if (Spec.Name == TEXT("widget_remove_sizebox_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_SizeBox");
    }
    if (Spec.Name == TEXT("widget_remove_gridpanel_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_GridPanel");
    }
    if (Spec.Name == TEXT("widget_remove_backgroundblur_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_BackgroundBlur");
    }
    if (Spec.Name == TEXT("widget_remove_invalidationbox_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_InvalidationBox");
    }
    if (Spec.Name == TEXT("widget_remove_retainerbox_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_RetainerBox");
    }
    if (Spec.Name == TEXT("widget_remove_safezone_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_SafeZone");
    }
    if (Spec.Name == TEXT("widget_remove_scalebox_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_ScaleBox");
    }
    if (Spec.Name == TEXT("widget_remove_scrollbox_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_ScrollBox");
    }
    if (Spec.Name == TEXT("widget_remove_widgetswitcher_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_WidgetSwitcher");
    }
    if (Spec.Name == TEXT("widget_remove_wrapbox_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_WrapBox");
    }
    if (Spec.Name == TEXT("widget_write_text_overlay_child"))
    {
        return TEXT("WBP_Overlay_TextBlock");
    }
    if (Spec.Name == TEXT("widget_add_image_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel");
    }
    if (Spec.Name == TEXT("widget_add_richtextblock_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel");
    }
    if (Spec.Name == TEXT("widget_add_progressbar_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel");
    }
    if (Spec.Name == TEXT("widget_add_slider_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel");
    }
    if (Spec.Name == TEXT("widget_add_spacer_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel");
    }
    if (Spec.Name == TEXT("widget_add_scrollbar_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel");
    }
    if (Spec.Name == TEXT("widget_add_editabletext_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel");
    }
    if (Spec.Name == TEXT("widget_add_editabletextbox_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel");
    }
    if (Spec.Name == TEXT("widget_add_multilineeditabletextbox_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel");
    }
    if (Spec.Name == TEXT("widget_add_spinbox_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel");
    }
    if (Spec.Name == TEXT("widget_add_comboboxstring_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel");
    }
    if (Spec.Name == TEXT("widget_add_listview_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel");
    }
    if (Spec.Name == TEXT("widget_add_tileview_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel");
    }
    if (Spec.Name == TEXT("widget_add_treeview_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel");
    }
    if (Spec.Name == TEXT("widget_add_namedslot_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel");
    }
    if (Spec.Name == TEXT("widget_add_textblock_namedslot_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_NamedSlot");
    }
    if (Spec.Name == TEXT("widget_add_userwidget_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel");
    }
    if (Spec.Name == TEXT("widget_add_image_overlay"))
    {
        return TEXT("WBP_Overlay");
    }
    if (Spec.Name == TEXT("widget_add_image_nested_overlay"))
    {
        return TEXT("WBP_MultiLevelSmoke");
    }
    if (Spec.Name == TEXT("widget_add_root_verticalbox"))
    {
        return TEXT("WBP_Minimum");
    }
    if (Spec.Name == TEXT("widget_add_root_horizontalbox"))
    {
        return TEXT("WBP_Minimum");
    }
    if (Spec.Name == TEXT("widget_add_button_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel");
    }
    if (Spec.Name == TEXT("widget_add_image_border_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_Border");
    }
    if (Spec.Name == TEXT("widget_add_image_sizebox_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_SizeBox");
    }
    if (Spec.Name == TEXT("widget_add_image_horizontalbox_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_HorizontalBox");
    }
    if (Spec.Name == TEXT("widget_write_opacity_root_textblock"))
    {
        return TEXT("WBP_TextBlock_Opacity");
    }
    if (Spec.Name == TEXT("widget_write_opacity_canvaspanel_child"))
    {
        return TEXT("WBP_CanvasPanel_TextBlock");
    }
    if (Spec.Name == TEXT("widget_write_visibility_root_textblock"))
    {
        return TEXT("WBP_TextBlock");
    }
    if (Spec.Name == TEXT("widget_write_visibility_overlay_child"))
    {
        return TEXT("WBP_Overlay_TextBlock");
    }
    if (Spec.Name == TEXT("widget_write_brush_image"))
    {
        return TEXT("WBP_Image");
    }
    if (Spec.Name == TEXT("widget_write_button_brush_normal")
        || Spec.Name == TEXT("widget_write_button_brush_tint")
        || Spec.Name == TEXT("widget_write_button_brush_image_size")
        || Spec.Name == TEXT("widget_write_button_brush_draw_as"))
    {
        return TEXT("WBP_ButtonVisualSmoke");
    }
    if (Spec.Name == TEXT("widget_write_scalebox_stretch_canvaspanel")
        || Spec.Name == TEXT("widget_write_scalebox_stretch_direction_canvaspanel")
        || Spec.Name == TEXT("widget_write_scalebox_user_specified_scale_canvaspanel")
        || Spec.Name == TEXT("widget_write_scalebox_ignore_inherited_scale_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_ScaleBox");
    }
    if (Spec.Name == TEXT("widget_write_wrapbox_wrap_size_canvaspanel")
        || Spec.Name == TEXT("widget_write_wrapbox_explicit_wrap_size_canvaspanel")
        || Spec.Name == TEXT("widget_write_wrapbox_inner_slot_padding_canvaspanel")
        || Spec.Name == TEXT("widget_write_wrapbox_orientation_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_WrapBox");
    }
    if (Spec.Name == TEXT("widget_write_widgetswitcher_active_widget_index_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_WidgetSwitcherChildren");
    }
    if (Spec.Name == TEXT("widget_write_listview_entry_widget_class_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_ListView_NoEntry");
    }
    if (Spec.Name == TEXT("widget_write_listview_orientation_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_selection_mode_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_consume_mouse_wheel_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_is_focusable_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_return_focus_to_selection_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_clear_scroll_velocity_on_selection_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_scroll_into_view_alignment_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_wheel_scroll_multiplier_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_enable_scroll_animation_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_allow_overscroll_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_enable_right_click_scrolling_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_enable_touch_scrolling_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_is_pointer_scrolling_enabled_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_is_gamepad_scrolling_enabled_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_horizontal_entry_spacing_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_vertical_entry_spacing_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_scrollbar_padding_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_ListView");
    }
    if (Spec.Name == TEXT("widget_write_tileview_entry_widget_class_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_TileView_NoEntry");
    }
    if (Spec.Name == TEXT("widget_write_tileview_entry_width_canvaspanel")
        || Spec.Name == TEXT("widget_write_tileview_entry_height_canvaspanel")
        || Spec.Name == TEXT("widget_write_tileview_scrollbar_disabled_visibility_canvaspanel")
        || Spec.Name == TEXT("widget_write_tileview_entry_size_includes_entry_spacing_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_TileView");
    }
    if (Spec.Name == TEXT("widget_write_treeview_entry_widget_class_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_TreeView_NoEntry");
    }
    if (Spec.Name == TEXT("widget_write_treeview_selection_mode_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_TreeView");
    }
    if (Spec.Name == TEXT("widget_write_retainerbox_retain_render_canvaspanel")
        || Spec.Name == TEXT("widget_write_retainerbox_render_on_invalidation_canvaspanel")
        || Spec.Name == TEXT("widget_write_retainerbox_render_on_phase_canvaspanel")
        || Spec.Name == TEXT("widget_write_retainerbox_phase_canvaspanel")
        || Spec.Name == TEXT("widget_write_retainerbox_phase_count_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_RetainerBox");
    }
    if (Spec.Name == TEXT("widget_write_backgroundblur_strength_canvaspanel")
        || Spec.Name == TEXT("widget_write_backgroundblur_apply_alpha_to_blur_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_BackgroundBlur");
    }
    if (Spec.Name == TEXT("widget_write_safezone_pad_left_canvaspanel")
        || Spec.Name == TEXT("widget_write_safezone_pad_right_canvaspanel")
        || Spec.Name == TEXT("widget_write_safezone_pad_top_canvaspanel")
        || Spec.Name == TEXT("widget_write_safezone_pad_bottom_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_SafeZone");
    }
    if (Spec.Name == TEXT("widget_write_invalidationbox_can_cache_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_InvalidationBox");
    }
    if (Spec.Name == TEXT("widget_write_uniformgridpanel_min_desired_slot_width_canvaspanel")
        || Spec.Name == TEXT("widget_write_uniformgridpanel_min_desired_slot_height_canvaspanel")
        || Spec.Name == TEXT("widget_write_uniformgridpanel_slot_padding_canvaspanel"))
    {
        return TEXT("WBP_CanvasPanel_UniformGridPanel");
    }
    return FString();
}

bool IsParseBlueprintOperation(const FOperationFixtureSpec& Spec)
{
    return !ParseBlueprintFixtureKeyForOperation(Spec).IsEmpty();
}

bool IsWidgetBlueprintOperation(const FOperationFixtureSpec& Spec)
{
    return !ParseWidgetFixtureKeyForOperation(Spec).IsEmpty();
}

bool IsDataTableOperation(const FOperationFixtureSpec& Spec)
{
    return Spec.Name == TEXT("dt_add_row")
        || Spec.Name == TEXT("dt_add_row_values_scalar")
        || Spec.Name == TEXT("dt_add_row_values_mixed")
        || Spec.Name == TEXT("dt_remove_row")
        || Spec.Name == TEXT("dt_remove_row_base");
}

bool IsDataTableUpdateOperation(const FOperationFixtureSpec& Spec)
{
    return Spec.Name == TEXT("dt_update_int")
        || Spec.Name == TEXT("dt_update_float")
        || Spec.Name == TEXT("dt_update_string")
        || Spec.Name == TEXT("dt_update_multi_field")
        || Spec.Name == TEXT("dt_update_complex");
}

bool IsCompositeDataTableRejectOperation(const FOperationFixtureSpec& Spec)
{
    return Spec.Name == TEXT("dt_add_row_composite_reject")
        || Spec.Name == TEXT("dt_remove_row_composite_reject")
        || Spec.Name == TEXT("dt_update_row_composite_reject");
}

bool IsStringTableOperation(const FOperationFixtureSpec& Spec)
{
    return Spec.Name == TEXT("stringtable_remove_entry")
        || Spec.Name == TEXT("stringtable_set_namespace")
        || Spec.Name == TEXT("stringtable_write_entry")
        || Spec.Name == TEXT("stringtable_write_entry_unicode")
        || Spec.Name == TEXT("stringtable_write_entry_btn_start")
        || Spec.Name == TEXT("stringtable_write_entry_title_unicode")
        || Spec.Name == TEXT("stringtable_remove_entry_btn_start")
        || Spec.Name == TEXT("stringtable_remove_entry_missing_reject")
        || Spec.Name == TEXT("stringtable_set_namespace_alt")
        || Spec.Name == TEXT("stringtable_set_namespace_unicode");
}

bool IsLevelOperation(const FOperationFixtureSpec& Spec)
{
    return Spec.Name == TEXT("level_var_set")
        || Spec.Name == TEXT("level_var_set_export_selector")
        || Spec.Name == TEXT("level_var_set_path_selector")
        || Spec.Name == TEXT("level_var_set_missing_actor_reject")
        || Spec.Name == TEXT("level_var_set_bad_path_reject")
        || Spec.Name == TEXT("level_var_set_export_selector_reject")
        || Spec.Name == TEXT("level_var_set_path_selector_reject");
}

bool IsNameOperation(const FOperationFixtureSpec& Spec)
{
    return Spec.Name == TEXT("name_add")
        || Spec.Name == TEXT("name_add_hash_override")
        || Spec.Name == TEXT("name_remove")
        || Spec.Name == TEXT("name_remove_non_tail_reject")
        || Spec.Name == TEXT("name_remove_referenced_reject")
        || Spec.Name == TEXT("name_set")
        || Spec.Name == TEXT("name_set_hash_override")
        || Spec.Name == TEXT("name_add_unicode")
        || Spec.Name == TEXT("name_add_long_ascii")
        || Spec.Name == TEXT("name_add_duplicate_reject")
        || Spec.Name == TEXT("name_add_hash_override_alt")
        || Spec.Name == TEXT("name_set_unicode")
        || Spec.Name == TEXT("name_set_hash_only")
        || Spec.Name == TEXT("name_set_hash_override_alt")
        || Spec.Name == TEXT("name_set_case_variant")
        || Spec.Name == TEXT("name_set_ascii_alt")
        || Spec.Name == TEXT("name_remove_unicode")
        || Spec.Name == TEXT("name_remove_hash")
        || Spec.Name == TEXT("name_remove_export_region_reject")
        || Spec.Name == TEXT("name_remove_non_tail_reject_alt")
        || Spec.Name == TEXT("name_remove_referenced_reject_alt");
}

FString ParseFixtureKeyForWriteRoundtripOperation(const FOperationFixtureSpec& Spec)
{
    if (Spec.Name == TEXT("write_roundtrip"))
    {
        return TEXT("BP_Empty");
    }
    if (Spec.Name == TEXT("write_roundtrip_umap"))
    {
        return TEXT("L_Minimal");
    }
    if (Spec.Name == TEXT("write_roundtrip_bp_withmetadata"))
    {
        return TEXT("BP_WithMetadata");
    }
    if (Spec.Name == TEXT("write_roundtrip_dt_simple"))
    {
        return TEXT("DT_Simple");
    }
    if (Spec.Name == TEXT("write_roundtrip_st_ui"))
    {
        return TEXT("ST_UI");
    }
    if (Spec.Name == TEXT("write_roundtrip_bp_withfunctions"))
    {
        return TEXT("BP_WithFunctions");
    }
    if (Spec.Name == TEXT("write_roundtrip_bp_empty_stringtable_ref"))
    {
        return TEXT("BP_Empty_StringTableRef");
    }
    return FString();
}

bool FindParseFixtureSpecByKey(const FString& Key, FParseFixtureSpec& OutSpec)
{
    for (const FParseFixtureSpec& Spec : BuildParseSpecs())
    {
        if (Spec.Key == Key)
        {
            OutSpec = Spec;
            return true;
        }
    }
    return false;
}

bool IsNotYetGeneratedOperation(const FOperationFixtureSpec& Spec)
{
    return false;
}

struct FNameFixtureCursor
{
    const TArray64<uint8>& Data;
    int64 Offset = 0;

    explicit FNameFixtureCursor(const TArray64<uint8>& InData)
        : Data(InData)
    {
    }

    bool Skip(int64 Count, FString& OutError)
    {
        if (Count < 0 || Offset + Count > Data.Num())
        {
            OutError = FString::Printf(TEXT("Unexpected EOF while skipping %lld bytes at %lld."), Count, Offset);
            return false;
        }
        Offset += Count;
        return true;
    }

    bool ReadUInt16(uint16& OutValue, FString& OutError)
    {
        if (Offset + 2 > Data.Num())
        {
            OutError = FString::Printf(TEXT("Unexpected EOF while reading uint16 at %lld."), Offset);
            return false;
        }
        OutValue = uint16(Data[Offset]) | (uint16(Data[Offset + 1]) << 8);
        Offset += 2;
        return true;
    }

    bool ReadUInt32(uint32& OutValue, FString& OutError)
    {
        if (Offset + 4 > Data.Num())
        {
            OutError = FString::Printf(TEXT("Unexpected EOF while reading uint32 at %lld."), Offset);
            return false;
        }
        OutValue = uint32(Data[Offset])
            | (uint32(Data[Offset + 1]) << 8)
            | (uint32(Data[Offset + 2]) << 16)
            | (uint32(Data[Offset + 3]) << 24);
        Offset += 4;
        return true;
    }

    bool ReadInt32(int32& OutValue, FString& OutError)
    {
        uint32 RawValue = 0;
        if (!ReadUInt32(RawValue, OutError))
        {
            return false;
        }
        OutValue = static_cast<int32>(RawValue);
        return true;
    }

    bool ReadInt64(int64& OutValue, FString& OutError)
    {
        if (Offset + 8 > Data.Num())
        {
            OutError = FString::Printf(TEXT("Unexpected EOF while reading int64 at %lld."), Offset);
            return false;
        }
        uint64 RawValue = uint64(Data[Offset])
            | (uint64(Data[Offset + 1]) << 8)
            | (uint64(Data[Offset + 2]) << 16)
            | (uint64(Data[Offset + 3]) << 24)
            | (uint64(Data[Offset + 4]) << 32)
            | (uint64(Data[Offset + 5]) << 40)
            | (uint64(Data[Offset + 6]) << 48)
            | (uint64(Data[Offset + 7]) << 56);
        Offset += 8;
        OutValue = static_cast<int64>(RawValue);
        return true;
    }

    bool ReadFString(FString& OutValue, FString& OutError)
    {
        int32 Length = 0;
        if (!ReadInt32(Length, OutError))
        {
            return false;
        }
        if (Length == 0)
        {
            OutValue.Reset();
            return true;
        }

        if (Length > 0)
        {
            const int64 ByteCount = Length;
            if (Offset + ByteCount > Data.Num())
            {
                OutError = FString::Printf(TEXT("Unexpected EOF while reading narrow FString (%d) at %lld."), Length, Offset);
                return false;
            }
            TArray<ANSICHAR> Buffer;
            Buffer.SetNumUninitialized(Length);
            FMemory::Memcpy(Buffer.GetData(), Data.GetData() + Offset, ByteCount);
            Offset += ByteCount;
            if (Buffer.Num() > 0 && Buffer.Last() == '\0')
            {
                Buffer.Pop(EAllowShrinking::No);
            }
            Buffer.Add('\0');
            OutValue = FString(UTF8_TO_TCHAR(Buffer.GetData()));
            return true;
        }

        const int32 WideCount = -Length;
        if (WideCount <= 0)
        {
            OutError = FString::Printf(TEXT("Invalid wide FString length %d."), Length);
            return false;
        }
        const int64 ByteCount = int64(WideCount) * 2;
        if (Offset + ByteCount > Data.Num())
        {
            OutError = FString::Printf(TEXT("Unexpected EOF while reading wide FString (%d) at %lld."), WideCount, Offset);
            return false;
        }

        TArray<UTF16CHAR> Buffer;
        Buffer.SetNumUninitialized(WideCount);
        for (int32 Index = 0; Index < WideCount; ++Index)
        {
            Buffer[Index] = UTF16CHAR(uint16(Data[Offset + Index * 2]) | (uint16(Data[Offset + Index * 2 + 1]) << 8));
        }
        Offset += ByteCount;
        if (Buffer.Num() > 0 && Buffer.Last() == 0)
        {
            Buffer.Pop(EAllowShrinking::No);
        }
        OutValue = FString(StringCast<TCHAR>(Buffer.GetData(), Buffer.Num()).Get());
        return true;
    }
};

bool IsASCIIName(const FString& Value)
{
    for (TCHAR Ch : Value)
    {
        if (Ch > 0x7f)
        {
            return false;
        }
    }
    return true;
}

const uint32* IEEECRC32Table()
{
    static bool bInitialized = false;
    static uint32 Table[256] = {};
    if (!bInitialized)
    {
        constexpr uint32 Polynomial = 0xEDB88320;
        for (uint32 Index = 0; Index < 256; ++Index)
        {
            uint32 CRC = Index;
            for (int32 Bit = 0; Bit < 8; ++Bit)
            {
                CRC = (CRC & 1u) != 0 ? ((CRC >> 1) ^ Polynomial) : (CRC >> 1);
            }
            Table[Index] = CRC;
        }
        bInitialized = true;
    }
    return Table;
}

uint32 UENameHashCasePreservingASCII(const FString& Value)
{
    uint32 CRC = ~uint32(0);
    FTCHARToUTF8 UTF8Value(*Value);
    const uint8* Bytes = reinterpret_cast<const uint8*>(UTF8Value.Get());
    const uint32* IEEETable = IEEECRC32Table();
    for (int32 Index = 0; Index < UTF8Value.Length(); ++Index)
    {
        uint32 Codepoint = Bytes[Index];
        for (int32 Shift = 0; Shift < 4; ++Shift)
        {
            CRC = (CRC >> 8) ^ IEEETable[(CRC ^ Codepoint) & 0xff];
            Codepoint >>= 8;
        }
    }
    return ~CRC;
}

uint32 UENameHashCasePreservingUTF16(const FString& Value)
{
    uint32 CRC = ~uint32(0);
    FTCHARToUTF16 UTF16Value(*Value);
    const UTF16CHAR* Units = UTF16Value.Get();
    const uint32* IEEETable = IEEECRC32Table();
    for (int32 Index = 0; Index < UTF16Value.Length(); ++Index)
    {
        uint32 Codepoint = Units[Index];
        for (int32 Shift = 0; Shift < 4; ++Shift)
        {
            CRC = (CRC >> 8) ^ IEEETable[(CRC ^ Codepoint) & 0xff];
            Codepoint >>= 8;
        }
    }
    return ~CRC;
}

const uint32* DeprecatedNameCRCTable()
{
    static bool bInitialized = false;
    static uint32 Table[256] = {};
    if (!bInitialized)
    {
        constexpr uint32 Polynomial = 0x04C11DB7;
        for (uint32 Index = 0; Index < 256; ++Index)
        {
            uint32 CRC = Index << 24;
            for (int32 Bit = 0; Bit < 8; ++Bit)
            {
                CRC = (CRC & 0x80000000u) != 0 ? ((CRC << 1) ^ Polynomial) : (CRC << 1);
            }
            Table[Index] = CRC;
        }
        bInitialized = true;
    }
    return Table;
}

uint32 UENameHashDeprecatedASCII(const FString& Value)
{
    uint32 Hash = 0;
    FTCHARToUTF8 UTF8Value(*Value);
    const uint8* Bytes = reinterpret_cast<const uint8*>(UTF8Value.Get());
    const uint32* Table = DeprecatedNameCRCTable();
    for (int32 Index = 0; Index < UTF8Value.Length(); ++Index)
    {
        const uint8 Upper = static_cast<uint8>(FChar::ToUpper(TCHAR(Bytes[Index])));
        Hash = ((Hash >> 8) & 0x00ffffff) ^ Table[(Hash ^ Upper) & 0x000000ff];
    }
    return Hash;
}

uint32 UENameHashDeprecatedUTF16(const FString& Value)
{
    uint32 Hash = 0;
    FTCHARToUTF16 UTF16Value(*Value);
    const UTF16CHAR* Units = UTF16Value.Get();
    const uint32* Table = DeprecatedNameCRCTable();
    for (int32 Index = 0; Index < UTF16Value.Length(); ++Index)
    {
        const uint16 Upper = uint16(FChar::ToUpper(TCHAR(Units[Index])));
        const uint32 Low = Upper & 0x00ff;
        Hash = ((Hash >> 8) & 0x00ffffff) ^ Table[(Hash ^ Low) & 0x000000ff];
        const uint32 High = (Upper >> 8) & 0x00ff;
        Hash = ((Hash >> 8) & 0x00ffffff) ^ Table[(Hash ^ High) & 0x000000ff];
    }
    return Hash;
}

void ComputeNameEntryHashes(const FString& Value, uint16& OutNonCaseHash, uint16& OutCasePreservingHash)
{
    if (IsASCIIName(Value))
    {
        OutNonCaseHash = uint16(UENameHashDeprecatedASCII(Value) & 0xffff);
        OutCasePreservingHash = uint16(UENameHashCasePreservingASCII(Value) & 0xffff);
        return;
    }

    OutNonCaseHash = uint16(UENameHashDeprecatedUTF16(Value) & 0xffff);
    OutCasePreservingHash = uint16(UENameHashCasePreservingUTF16(Value) & 0xffff);
}

void AddSummaryOffsetField(FNameFixtureSummaryInfo& Info, const TCHAR* Name, int64 Pos, int32 Size)
{
    FNameFixtureSummaryOffsetField& Field = Info.OffsetFields.AddDefaulted_GetRef();
    Field.Name = Name;
    Field.Pos = Pos;
    Field.Size = Size;
}

bool SkipSummaryCustomVersions(FNameFixtureCursor& Cursor, int32 LegacyVersion, FString& OutError)
{
    int32 Count = 0;
    if (!Cursor.ReadInt32(Count, OutError))
    {
        return false;
    }
    if (Count < 0)
    {
        OutError = FString::Printf(TEXT("Invalid custom version count: %d."), Count);
        return false;
    }

    for (int32 Index = 0; Index < Count; ++Index)
    {
        if (LegacyVersion == -2)
        {
            if (!Cursor.Skip(8, OutError))
            {
                return false;
            }
            continue;
        }

        if (!Cursor.Skip(16, OutError))
        {
            return false;
        }
        int32 Version = 0;
        if (!Cursor.ReadInt32(Version, OutError))
        {
            return false;
        }
        if (LegacyVersion >= -5)
        {
            FString FriendlyName;
            if (!Cursor.ReadFString(FriendlyName, OutError))
            {
                return false;
            }
        }
    }

    return true;
}

bool SkipEngineVersion(FNameFixtureCursor& Cursor, FString& OutError)
{
    if (!Cursor.Skip(2, OutError)
        || !Cursor.Skip(2, OutError)
        || !Cursor.Skip(2, OutError)
        || !Cursor.Skip(4, OutError))
    {
        return false;
    }
    FString Branch;
    return Cursor.ReadFString(Branch, OutError);
}

bool ReadSummaryInfo(const TArray64<uint8>& Bytes, FNameFixtureSummaryInfo& OutInfo, FString& OutError)
{
    if (Bytes.Num() < 4)
    {
        OutError = TEXT("Package is too small.");
        return false;
    }

    FNameFixtureCursor Cursor(Bytes);
    int32 Tag = 0;
    if (!Cursor.ReadInt32(Tag, OutError))
    {
        return false;
    }
    if (uint32(Tag) != BPXPackageFileTag)
    {
        OutError = FString::Printf(TEXT("Unsupported package tag: 0x%08x."), uint32(Tag));
        return false;
    }

    if (!Cursor.ReadInt32(OutInfo.LegacyVersion, OutError))
    {
        return false;
    }
    if (OutInfo.LegacyVersion != -4)
    {
        int32 LegacyUE3Version = 0;
        if (!Cursor.ReadInt32(LegacyUE3Version, OutError))
        {
            return false;
        }
    }
    if (!Cursor.ReadInt32(OutInfo.FileVersionUE4, OutError)
        || !Cursor.ReadInt32(OutInfo.FileVersionUE5, OutError)
        || !Cursor.ReadInt32(OutInfo.FileVersionLicenseeUE, OutError))
    {
        return false;
    }
    if (OutInfo.FileVersionUE4 == 0 && OutInfo.FileVersionUE5 == 0 && OutInfo.FileVersionLicenseeUE == 0)
    {
        OutInfo.FileVersionUE4 = BPXUE4VersionUE56;
        OutInfo.FileVersionUE5 = BPXUE5ImportTypeHierarchies;
    }
    if (OutInfo.FileVersionUE5 >= BPXUE5PackageSavedHash)
    {
        OutInfo.SavedHashPos = Cursor.Offset;
        if (!Cursor.Skip(20, OutError))
        {
            return false;
        }
        const int64 TotalHeaderSizePos = Cursor.Offset;
        if (!Cursor.ReadInt32(OutInfo.TotalHeaderSize, OutError))
        {
            return false;
        }
        AddSummaryOffsetField(OutInfo, TEXT("TotalHeaderSize"), TotalHeaderSizePos, 4);
    }
    if (OutInfo.LegacyVersion <= -2 && !SkipSummaryCustomVersions(Cursor, OutInfo.LegacyVersion, OutError))
    {
        return false;
    }
    if (OutInfo.FileVersionUE5 < BPXUE5PackageSavedHash)
    {
        const int64 TotalHeaderSizePos = Cursor.Offset;
        if (!Cursor.ReadInt32(OutInfo.TotalHeaderSize, OutError))
        {
            return false;
        }
        AddSummaryOffsetField(OutInfo, TEXT("TotalHeaderSize"), TotalHeaderSizePos, 4);
    }

    FString PackageName;
    if (!Cursor.ReadFString(PackageName, OutError))
    {
        return false;
    }
    if (!Cursor.ReadInt32(OutInfo.PackageFlags, OutError))
    {
        return false;
    }

    OutInfo.NameCountPos = Cursor.Offset;
    if (!Cursor.ReadInt32(OutInfo.NameCount, OutError))
    {
        return false;
    }
    OutInfo.NameOffsetPos = Cursor.Offset;
    if (!Cursor.ReadInt32(OutInfo.NameOffset, OutError))
    {
        return false;
    }
    AddSummaryOffsetField(OutInfo, TEXT("NameOffset"), OutInfo.NameOffsetPos, 4);

    if (OutInfo.FileVersionUE5 >= BPXUE5AddSoftObjectPathList)
    {
        int32 SoftObjectPathsCount = 0;
        if (!Cursor.ReadInt32(SoftObjectPathsCount, OutError))
        {
            return false;
        }
        const int64 FieldPos = Cursor.Offset;
        if (!Cursor.ReadInt32(OutInfo.SoftObjectPathsOffset, OutError))
        {
            return false;
        }
        AddSummaryOffsetField(OutInfo, TEXT("SoftObjectPathsOffset"), FieldPos, 4);
    }
    if ((uint32(OutInfo.PackageFlags) & BPXPkgFlagFilterEditorOnly) == 0)
    {
        FString LocalizationID;
        if (!Cursor.ReadFString(LocalizationID, OutError))
        {
            return false;
        }
    }

    int32 GatherableTextDataCount = 0;
    if (!Cursor.ReadInt32(GatherableTextDataCount, OutError))
    {
        return false;
    }
    {
        const int64 FieldPos = Cursor.Offset;
        if (!Cursor.ReadInt32(OutInfo.GatherableTextDataOffset, OutError))
        {
            return false;
        }
        AddSummaryOffsetField(OutInfo, TEXT("GatherableTextDataOffset"), FieldPos, 4);
    }

    if (!Cursor.ReadInt32(OutInfo.ExportCount, OutError))
    {
        return false;
    }
    {
        const int64 FieldPos = Cursor.Offset;
        if (!Cursor.ReadInt32(OutInfo.ExportMapOffset, OutError))
        {
            return false;
        }
        OutInfo.ExportOffset = OutInfo.ExportMapOffset;
        AddSummaryOffsetField(OutInfo, TEXT("ExportOffset"), FieldPos, 4);
    }

    int32 ImportCount = 0;
    if (!Cursor.ReadInt32(ImportCount, OutError))
    {
        return false;
    }
    {
        const int64 FieldPos = Cursor.Offset;
        if (!Cursor.ReadInt32(OutInfo.ImportOffset, OutError))
        {
            return false;
        }
        AddSummaryOffsetField(OutInfo, TEXT("ImportOffset"), FieldPos, 4);
    }

    if (OutInfo.FileVersionUE5 >= BPXUE5VerseCells)
    {
        int32 CountValue = 0;
        if (!Cursor.ReadInt32(CountValue, OutError))
        {
            return false;
        }
        int64 FieldPos = Cursor.Offset;
        if (!Cursor.ReadInt32(OutInfo.CellExportOffset, OutError))
        {
            return false;
        }
        AddSummaryOffsetField(OutInfo, TEXT("CellExportOffset"), FieldPos, 4);
        if (!Cursor.ReadInt32(CountValue, OutError))
        {
            return false;
        }
        FieldPos = Cursor.Offset;
        if (!Cursor.ReadInt32(OutInfo.CellImportOffset, OutError))
        {
            return false;
        }
        AddSummaryOffsetField(OutInfo, TEXT("CellImportOffset"), FieldPos, 4);
    }

    if (OutInfo.FileVersionUE5 >= BPXUE5MetadataSerializationOff)
    {
        const int64 FieldPos = Cursor.Offset;
        if (!Cursor.ReadInt32(OutInfo.MetaDataOffset, OutError))
        {
            return false;
        }
        AddSummaryOffsetField(OutInfo, TEXT("MetaDataOffset"), FieldPos, 4);
    }

    {
        const int64 FieldPos = Cursor.Offset;
        if (!Cursor.ReadInt32(OutInfo.DependsOffset, OutError))
        {
            return false;
        }
        AddSummaryOffsetField(OutInfo, TEXT("DependsOffset"), FieldPos, 4);
    }

    int32 CountValue = 0;
    if (!Cursor.ReadInt32(CountValue, OutError))
    {
        return false;
    }
    {
        const int64 FieldPos = Cursor.Offset;
        if (!Cursor.ReadInt32(OutInfo.SoftPackageReferencesOffset, OutError))
        {
            return false;
        }
        AddSummaryOffsetField(OutInfo, TEXT("SoftPackageReferencesOffset"), FieldPos, 4);
    }
    {
        const int64 FieldPos = Cursor.Offset;
        if (!Cursor.ReadInt32(OutInfo.SearchableNamesOffset, OutError))
        {
            return false;
        }
        AddSummaryOffsetField(OutInfo, TEXT("SearchableNamesOffset"), FieldPos, 4);
    }
    {
        const int64 FieldPos = Cursor.Offset;
        if (!Cursor.ReadInt32(OutInfo.ThumbnailTableOffset, OutError))
        {
            return false;
        }
        AddSummaryOffsetField(OutInfo, TEXT("ThumbnailTableOffset"), FieldPos, 4);
    }

    if (OutInfo.FileVersionUE5 >= BPXUE5ImportTypeHierarchies)
    {
        if (!Cursor.ReadInt32(CountValue, OutError))
        {
            return false;
        }
        const int64 FieldPos = Cursor.Offset;
        if (!Cursor.ReadInt32(OutInfo.ImportTypeHierarchiesOffset, OutError))
        {
            return false;
        }
        AddSummaryOffsetField(OutInfo, TEXT("ImportTypeHierarchiesOffset"), FieldPos, 4);
    }

    if (OutInfo.FileVersionUE5 < BPXUE5PackageSavedHash && !Cursor.Skip(16, OutError))
    {
        return false;
    }
    if ((uint32(OutInfo.PackageFlags) & BPXPkgFlagFilterEditorOnly) == 0 && !Cursor.Skip(16, OutError))
    {
        return false;
    }

    int32 GenerationCount = 0;
    if (!Cursor.ReadInt32(GenerationCount, OutError))
    {
        return false;
    }
    if (GenerationCount < 0)
    {
        OutError = FString::Printf(TEXT("Invalid generation count: %d."), GenerationCount);
        return false;
    }
    for (int32 Index = 0; Index < GenerationCount; ++Index)
    {
        int32 ExportCountValue = 0;
        if (!Cursor.ReadInt32(ExportCountValue, OutError))
        {
            return false;
        }
        OutInfo.GenerationNameCountPos.Add(Cursor.Offset);
        int32 GenerationNameCount = 0;
        if (!Cursor.ReadInt32(GenerationNameCount, OutError))
        {
            return false;
        }
    }

    if (!SkipEngineVersion(Cursor, OutError) || !SkipEngineVersion(Cursor, OutError))
    {
        return false;
    }
    uint32 CompressionFlags = 0;
    if (!Cursor.ReadUInt32(CompressionFlags, OutError))
    {
        return false;
    }
    int32 ChunkCount = 0;
    if (!Cursor.ReadInt32(ChunkCount, OutError))
    {
        return false;
    }
    if (ChunkCount < 0 || !Cursor.Skip(int64(ChunkCount) * 16, OutError))
    {
        return false;
    }
    uint32 PackageSource = 0;
    if (!Cursor.ReadUInt32(PackageSource, OutError))
    {
        return false;
    }
    int32 AdditionalPackagesCount = 0;
    if (!Cursor.ReadInt32(AdditionalPackagesCount, OutError))
    {
        return false;
    }
    if (AdditionalPackagesCount < 0)
    {
        OutError = FString::Printf(TEXT("Invalid additional packages count: %d."), AdditionalPackagesCount);
        return false;
    }
    for (int32 Index = 0; Index < AdditionalPackagesCount; ++Index)
    {
        FString AdditionalPackage;
        if (!Cursor.ReadFString(AdditionalPackage, OutError))
        {
            return false;
        }
    }
    if (OutInfo.LegacyVersion > -7)
    {
        int32 TextureAllocations = 0;
        if (!Cursor.ReadInt32(TextureAllocations, OutError))
        {
            return false;
        }
    }

    {
        const int64 FieldPos = Cursor.Offset;
        if (!Cursor.ReadInt32(OutInfo.AssetRegistryDataOffset, OutError))
        {
            return false;
        }
        AddSummaryOffsetField(OutInfo, TEXT("AssetRegistryDataOffset"), FieldPos, 4);
    }
    {
        const int64 FieldPos = Cursor.Offset;
        if (!Cursor.ReadInt64(OutInfo.BulkDataStartOffset, OutError))
        {
            return false;
        }
        AddSummaryOffsetField(OutInfo, TEXT("BulkDataStartOffset"), FieldPos, 8);
    }
    {
        const int64 FieldPos = Cursor.Offset;
        int32 WorldTileInfoDataOffset = 0;
        if (!Cursor.ReadInt32(WorldTileInfoDataOffset, OutError))
        {
            return false;
        }
        AddSummaryOffsetField(OutInfo, TEXT("WorldTileInfoDataOffset"), FieldPos, 4);
    }

    int32 ChunkIDsCount = 0;
    if (!Cursor.ReadInt32(ChunkIDsCount, OutError))
    {
        return false;
    }
    if (ChunkIDsCount < 0 || !Cursor.Skip(int64(ChunkIDsCount) * 4, OutError))
    {
        return false;
    }
    int32 PreloadDependencyCount = 0;
    if (!Cursor.ReadInt32(PreloadDependencyCount, OutError))
    {
        return false;
    }
    {
        const int64 FieldPos = Cursor.Offset;
        if (!Cursor.ReadInt32(OutInfo.PreloadDependencyOffset, OutError))
        {
            return false;
        }
        AddSummaryOffsetField(OutInfo, TEXT("PreloadDependencyOffset"), FieldPos, 4);
    }
    if (OutInfo.FileVersionUE5 >= BPXUE5NamesFromExportData)
    {
        OutInfo.NamesReferencedFromExportDataCountPos = Cursor.Offset;
        int32 NamesReferencedFromExportDataCount = 0;
        if (!Cursor.ReadInt32(NamesReferencedFromExportDataCount, OutError))
        {
            return false;
        }
    }
    if (OutInfo.FileVersionUE5 >= BPXUE5PayloadTOC)
    {
        const int64 FieldPos = Cursor.Offset;
        if (!Cursor.ReadInt64(OutInfo.PayloadTOCOffset, OutError))
        {
            return false;
        }
        AddSummaryOffsetField(OutInfo, TEXT("PayloadTOCOffset"), FieldPos, 8);
    }
    if (OutInfo.FileVersionUE5 >= BPXUE5DataResources)
    {
        const int64 FieldPos = Cursor.Offset;
        if (!Cursor.ReadInt32(OutInfo.DataResourceOffset, OutError))
        {
            return false;
        }
        AddSummaryOffsetField(OutInfo, TEXT("DataResourceOffset"), FieldPos, 4);
    }

    return true;
}

bool ReadNameEntries(const TArray64<uint8>& Bytes, const FNameFixtureSummaryInfo& Summary, TArray<FNameFixtureEntry>& OutEntries, FString& OutError)
{
    if (Summary.NameOffset < 0 || Summary.NameOffset > Bytes.Num())
    {
        OutError = FString::Printf(TEXT("NameOffset out of range: %d."), Summary.NameOffset);
        return false;
    }

    FNameFixtureCursor Cursor(Bytes);
    Cursor.Offset = Summary.NameOffset;
    OutEntries.Reset();
    OutEntries.Reserve(Summary.NameCount);
    for (int32 Index = 0; Index < Summary.NameCount; ++Index)
    {
        FNameFixtureEntry Entry;
        if (!Cursor.ReadFString(Entry.Value, OutError)
            || !Cursor.ReadUInt16(Entry.NonCaseHash, OutError)
            || !Cursor.ReadUInt16(Entry.CasePreservingHash, OutError))
        {
            return false;
        }
        OutEntries.Add(Entry);
    }
    return true;
}

int64 FindNameMapEndOffset(const FNameFixtureSummaryInfo& Summary, const TArray64<uint8>& Bytes)
{
    const TArray<int64> Candidates = {
        Summary.SoftObjectPathsOffset,
        Summary.GatherableTextDataOffset,
        Summary.MetaDataOffset,
        Summary.ImportOffset,
        Summary.ExportMapOffset,
        Summary.CellImportOffset,
        Summary.CellExportOffset,
        Summary.DependsOffset,
        Summary.SoftPackageReferencesOffset,
        Summary.SearchableNamesOffset,
        Summary.ThumbnailTableOffset,
        Summary.ImportTypeHierarchiesOffset,
        Summary.AssetRegistryDataOffset,
        Summary.PreloadDependencyOffset,
        Summary.DataResourceOffset,
        Summary.PayloadTOCOffset,
        Summary.BulkDataStartOffset,
        Summary.TotalHeaderSize
    };

    int64 EndOffset = Bytes.Num();
    for (int64 Candidate : Candidates)
    {
        if (Candidate > Summary.NameOffset && Candidate <= Bytes.Num() && Candidate < EndOffset)
        {
            EndOffset = Candidate;
        }
    }
    return EndOffset;
}

bool IsNameMutationNoop(const TArray<FNameFixtureEntry>& OldEntries, const TArray<FNameFixtureEntry>& NewEntries)
{
    if (OldEntries.Num() != NewEntries.Num())
    {
        return false;
    }
    for (int32 Index = 0; Index < OldEntries.Num(); ++Index)
    {
        const FNameFixtureEntry& OldEntry = OldEntries[Index];
        const FNameFixtureEntry& NewEntry = NewEntries[Index];
        if (OldEntry.Value != NewEntry.Value
            || OldEntry.NonCaseHash != NewEntry.NonCaseHash
            || OldEntry.CasePreservingHash != NewEntry.CasePreservingHash)
        {
            return false;
        }
    }
    return true;
}

int32 CountRemovedPrefixNameEntries(const TArray<FNameFixtureEntry>& OldEntries, const TArray<FNameFixtureEntry>& NewEntries, int32 PrefixLen)
{
    PrefixLen = FMath::Clamp(PrefixLen, 0, OldEntries.Num());
    if (PrefixLen == 0)
    {
        return 0;
    }

    TMap<FString, int32> RemainingCounts;
    auto MakeKey = [](const FNameFixtureEntry& Entry) {
        return FString::Printf(TEXT("%s|%u|%u"), *Entry.Value, Entry.NonCaseHash, Entry.CasePreservingHash);
    };

    for (const FNameFixtureEntry& Entry : NewEntries)
    {
        RemainingCounts.FindOrAdd(MakeKey(Entry))++;
    }

    int32 Removed = 0;
    for (int32 Index = 0; Index < PrefixLen; ++Index)
    {
        const FString Key = MakeKey(OldEntries[Index]);
        int32* Count = RemainingCounts.Find(Key);
        if (Count && *Count > 0)
        {
            --(*Count);
            continue;
        }
        ++Removed;
    }
    return Removed;
}

void WriteInt32LE(TArray64<uint8>& Bytes, int64 Pos, int32 Value)
{
    Bytes[Pos + 0] = uint8(Value & 0xff);
    Bytes[Pos + 1] = uint8((Value >> 8) & 0xff);
    Bytes[Pos + 2] = uint8((Value >> 16) & 0xff);
    Bytes[Pos + 3] = uint8((uint32(Value) >> 24) & 0xff);
}

void WriteInt64LE(TArray64<uint8>& Bytes, int64 Pos, int64 Value)
{
    const uint64 RawValue = static_cast<uint64>(Value);
    for (int32 Shift = 0; Shift < 8; ++Shift)
    {
        Bytes[Pos + Shift] = uint8((RawValue >> (Shift * 8)) & 0xff);
    }
}

int32 ReadInt32LEAt(const TArray64<uint8>& Bytes, int64 Pos)
{
    return int32(uint32(Bytes[Pos + 0])
        | (uint32(Bytes[Pos + 1]) << 8)
        | (uint32(Bytes[Pos + 2]) << 16)
        | (uint32(Bytes[Pos + 3]) << 24));
}

int64 ReadInt64LEAt(const TArray64<uint8>& Bytes, int64 Pos)
{
    uint64 RawValue = 0;
    for (int32 Shift = 0; Shift < 8; ++Shift)
    {
        RawValue |= uint64(Bytes[Pos + Shift]) << (Shift * 8);
    }
    return static_cast<int64>(RawValue);
}

TArray64<uint8> EncodeNameMapEntries(const TArray<FNameFixtureEntry>& Entries)
{
    TArray64<uint8> Encoded;
    for (const FNameFixtureEntry& Entry : Entries)
    {
        if (Entry.Value.IsEmpty())
        {
            const int32 Zero = 0;
            Encoded.Append(reinterpret_cast<const uint8*>(&Zero), sizeof(int32));
        }
        else if (IsASCIIName(Entry.Value))
        {
            FTCHARToUTF8 UTF8Value(*Entry.Value);
            const int32 Length = UTF8Value.Length() + 1;
            const int32 Offset = Encoded.Num();
            Encoded.SetNumUninitialized(Offset + 4 + Length);
            WriteInt32LE(Encoded, Offset, Length);
            FMemory::Memcpy(Encoded.GetData() + Offset + 4, UTF8Value.Get(), UTF8Value.Length());
            Encoded[Offset + 4 + UTF8Value.Length()] = 0;
        }
        else
        {
            FTCHARToUTF16 UTF16Value(*Entry.Value);
            const int32 Length = -(UTF16Value.Length() + 1);
            const int32 Offset = Encoded.Num();
            Encoded.SetNumUninitialized(Offset + 4 + (UTF16Value.Length() + 1) * 2);
            WriteInt32LE(Encoded, Offset, Length);
            for (int32 Index = 0; Index < UTF16Value.Length(); ++Index)
            {
                const uint16 CodeUnit = UTF16Value.Get()[Index];
                Encoded[Offset + 4 + Index * 2] = uint8(CodeUnit & 0xff);
                Encoded[Offset + 4 + Index * 2 + 1] = uint8((CodeUnit >> 8) & 0xff);
            }
            Encoded[Offset + 4 + UTF16Value.Length() * 2] = 0;
            Encoded[Offset + 4 + UTF16Value.Length() * 2 + 1] = 0;
        }

        const int64 HashPos = Encoded.Num();
        Encoded.SetNumUninitialized(HashPos + 4);
        Encoded[HashPos + 0] = uint8(Entry.NonCaseHash & 0xff);
        Encoded[HashPos + 1] = uint8((Entry.NonCaseHash >> 8) & 0xff);
        Encoded[HashPos + 2] = uint8(Entry.CasePreservingHash & 0xff);
        Encoded[HashPos + 3] = uint8((Entry.CasePreservingHash >> 8) & 0xff);
    }
    return Encoded;
}

int64 TranslateNameMapOffset(int64 OldPos, int64 NameStart, int64 NameEnd, int64 NewNameLen)
{
    const int64 OldLen = NameEnd - NameStart;
    const int64 Delta = NewNameLen - OldLen;
    if (OldPos < NameStart)
    {
        return OldPos;
    }
    if (OldPos >= NameEnd)
    {
        return OldPos + Delta;
    }
    const int64 RelativePos = FMath::Clamp<int64>(OldPos - NameStart, 0, NewNameLen);
    return NameStart + RelativePos;
}

bool ScanExportFieldPositions(const TArray64<uint8>& Bytes, const FNameFixtureSummaryInfo& Summary, TArray<FNameFixtureExportFieldPatch>& OutFields, FString& OutError)
{
    if (Summary.ExportOffset < 0 || Summary.ExportOffset > Bytes.Num())
    {
        OutError = FString::Printf(TEXT("ExportOffset out of range: %d."), Summary.ExportOffset);
        return false;
    }

    FNameFixtureCursor Cursor(Bytes);
    Cursor.Offset = Summary.ExportOffset;
    OutFields.Reset();
    OutFields.Reserve(Summary.ExportCount);
    for (int32 Index = 0; Index < Summary.ExportCount; ++Index)
    {
        if (!Cursor.Skip(4 * 4, OutError)
            || !Cursor.Skip(8, OutError)
            || !Cursor.Skip(4, OutError))
        {
            return false;
        }

        FNameFixtureExportFieldPatch Patch;
        Patch.SerialSizePos = Cursor.Offset;
        int64 SerialSize = 0;
        if (!Cursor.ReadInt64(SerialSize, OutError))
        {
            return false;
        }
        Patch.SerialOffsetPos = Cursor.Offset;
        int64 SerialOffset = 0;
        if (!Cursor.ReadInt64(SerialOffset, OutError))
        {
            return false;
        }
        if (!Cursor.Skip(4 * 3, OutError))
        {
            return false;
        }
        if (Summary.FileVersionUE5 < BPXUE5RemoveObjectExportPkgGUID && !Cursor.Skip(16, OutError))
        {
            return false;
        }
        if (Summary.FileVersionUE5 >= BPXUE5TrackObjectExportInherited && !Cursor.Skip(4, OutError))
        {
            return false;
        }
        if (!Cursor.Skip(4, OutError)
            || !Cursor.Skip(4 * 2, OutError))
        {
            return false;
        }
        if (Summary.FileVersionUE5 >= BPXUE5OptionalResources && !Cursor.Skip(4, OutError))
        {
            return false;
        }
        if (!Cursor.Skip(4 * 5, OutError))
        {
            return false;
        }
        if (Summary.FileVersionUE5 >= BPXUE5ScriptSerializationOffset)
        {
            Patch.ScriptStartPos = Cursor.Offset;
            int64 ScriptStart = 0;
            if (!Cursor.ReadInt64(ScriptStart, OutError))
            {
                return false;
            }
            Patch.ScriptEndPos = Cursor.Offset;
            int64 ScriptEnd = 0;
            if (!Cursor.ReadInt64(ScriptEnd, OutError))
            {
                return false;
            }
        }
        OutFields.Add(Patch);
    }
    return true;
}

void UpdateSavedHashIfPresent(TArray64<uint8>& Bytes, const FNameFixtureSummaryInfo& Summary)
{
    if (Summary.SavedHashPos == INDEX_NONE || Summary.SavedHashPos + 20 > Bytes.Num())
    {
        return;
    }
    const int64 HashEnd = Summary.PayloadTOCOffset > 0 && Summary.PayloadTOCOffset <= Bytes.Num()
        ? Summary.PayloadTOCOffset
        : Bytes.Num();
    FMemory::Memzero(Bytes.GetData() + Summary.SavedHashPos, 20);
    const FIoHash Hash = FIoHashBuilder::HashBuffer(Bytes.GetData(), HashEnd);
    FMemory::Memcpy(Bytes.GetData() + Summary.SavedHashPos, Hash.GetBytes(), 20);
}

bool RewriteNameMapPackageBytes(const TArray64<uint8>& InputBytes, const TArray<FNameFixtureEntry>& NewEntries, TArray64<uint8>& OutBytes, FString& OutError)
{
    FNameFixtureSummaryInfo Summary;
    if (!ReadSummaryInfo(InputBytes, Summary, OutError))
    {
        return false;
    }

    TArray<FNameFixtureEntry> OldEntries;
    if (!ReadNameEntries(InputBytes, Summary, OldEntries, OutError))
    {
        return false;
    }
    if (NewEntries.Num() == 0)
    {
        OutError = TEXT("NameMap must not be empty.");
        return false;
    }
    if (IsNameMutationNoop(OldEntries, NewEntries))
    {
        OutBytes = InputBytes;
        return true;
    }

    const int64 NameStart = Summary.NameOffset;
    const int64 NameEnd = FindNameMapEndOffset(Summary, InputBytes);
    if (NameStart < 0 || NameStart > NameEnd || NameEnd > InputBytes.Num())
    {
        OutError = FString::Printf(TEXT("Invalid NameMap range: %lld..%lld."), NameStart, NameEnd);
        return false;
    }

    const TArray64<uint8> EncodedNameMap = EncodeNameMapEntries(NewEntries);
    OutBytes.Reset();
    OutBytes.Reserve(InputBytes.Num() + EncodedNameMap.Num() - (NameEnd - NameStart));
    OutBytes.Append(InputBytes.GetData(), NameStart);
    OutBytes.Append(EncodedNameMap);
    OutBytes.Append(InputBytes.GetData() + NameEnd, InputBytes.Num() - NameEnd);

    for (const FNameFixtureSummaryOffsetField& Field : Summary.OffsetFields)
    {
        const int64 WritePos = TranslateNameMapOffset(Field.Pos, NameStart, NameEnd, EncodedNameMap.Num());
        if (WritePos < 0 || WritePos + Field.Size > OutBytes.Num())
        {
            OutError = FString::Printf(TEXT("Summary field %s write out of range at %lld."), *Field.Name, WritePos);
            return false;
        }
        if (Field.Size == 4)
        {
            const int32 OldValue = ReadInt32LEAt(InputBytes, Field.Pos);
            if (OldValue >= 0)
            {
                WriteInt32LE(OutBytes, WritePos, int32(TranslateNameMapOffset(OldValue, NameStart, NameEnd, EncodedNameMap.Num())));
            }
        }
        else if (Field.Size == 8)
        {
            const int64 OldValue = ReadInt64LEAt(InputBytes, Field.Pos);
            if (OldValue >= 0)
            {
                WriteInt64LE(OutBytes, WritePos, TranslateNameMapOffset(OldValue, NameStart, NameEnd, EncodedNameMap.Num()));
            }
        }
    }

    WriteInt32LE(OutBytes, Summary.NameCountPos, NewEntries.Num());
    for (int64 GenerationPos : Summary.GenerationNameCountPos)
    {
        const int64 WritePos = TranslateNameMapOffset(GenerationPos, NameStart, NameEnd, EncodedNameMap.Num());
        if (WritePos >= 0 && WritePos + 4 <= OutBytes.Num())
        {
            WriteInt32LE(OutBytes, WritePos, NewEntries.Num());
        }
    }
    if (Summary.NamesReferencedFromExportDataCountPos != INDEX_NONE)
    {
        int32 CurrentValue = ReadInt32LEAt(InputBytes, Summary.NamesReferencedFromExportDataCountPos);
        int32 NextValue = CurrentValue;
        if (NewEntries.Num() < OldEntries.Num() && CurrentValue > 0)
        {
            NextValue -= CountRemovedPrefixNameEntries(OldEntries, NewEntries, CurrentValue);
        }
        NextValue = FMath::Clamp(NextValue, 0, NewEntries.Num());
        const int64 WritePos = TranslateNameMapOffset(Summary.NamesReferencedFromExportDataCountPos, NameStart, NameEnd, EncodedNameMap.Num());
        if (WritePos >= 0 && WritePos + 4 <= OutBytes.Num())
        {
            WriteInt32LE(OutBytes, WritePos, NextValue);
        }
    }

    TArray<FNameFixtureExportFieldPatch> ExportFields;
    if (!ScanExportFieldPositions(InputBytes, Summary, ExportFields, OutError))
    {
        return false;
    }
    for (const FNameFixtureExportFieldPatch& Patch : ExportFields)
    {
        const int64 OldSerialOffset = ReadInt64LEAt(InputBytes, Patch.SerialOffsetPos);
        const int64 WritePos = TranslateNameMapOffset(Patch.SerialOffsetPos, NameStart, NameEnd, EncodedNameMap.Num());
        if (WritePos < 0 || WritePos + 8 > OutBytes.Num())
        {
            OutError = FString::Printf(TEXT("Export serial offset field write out of range at %lld."), WritePos);
            return false;
        }
        WriteInt64LE(OutBytes, WritePos, TranslateNameMapOffset(OldSerialOffset, NameStart, NameEnd, EncodedNameMap.Num()));
    }

    if (Summary.AssetRegistryDataOffset > 0 && Summary.AssetRegistryDataOffset + 8 <= InputBytes.Num())
    {
        const int64 OldDependencyOffset = ReadInt64LEAt(InputBytes, Summary.AssetRegistryDataOffset);
        if (OldDependencyOffset > 0)
        {
            const int64 WritePos = TranslateNameMapOffset(Summary.AssetRegistryDataOffset, NameStart, NameEnd, EncodedNameMap.Num());
            if (WritePos >= 0 && WritePos + 8 <= OutBytes.Num())
            {
                WriteInt64LE(OutBytes, WritePos, TranslateNameMapOffset(OldDependencyOffset, NameStart, NameEnd, EncodedNameMap.Num()));
            }
        }
    }

    UpdateSavedHashIfPresent(OutBytes, Summary);
    return true;
}

void AppendNameEntry(TArray<FNameFixtureEntry>& Entries, const FString& Value, TOptional<uint16> NonCaseHash = TOptional<uint16>(), TOptional<uint16> CasePreservingHash = TOptional<uint16>())
{
    FNameFixtureEntry& Entry = Entries.AddDefaulted_GetRef();
    Entry.Value = Value;
    ComputeNameEntryHashes(Value, Entry.NonCaseHash, Entry.CasePreservingHash);
    if (NonCaseHash.IsSet())
    {
        Entry.NonCaseHash = NonCaseHash.GetValue();
    }
    if (CasePreservingHash.IsSet())
    {
        Entry.CasePreservingHash = CasePreservingHash.GetValue();
    }
}

bool BuildNameOperationEntries(const TArray<FNameFixtureEntry>& BaseEntries, const FOperationFixtureSpec& Spec, bool bBefore, TArray<FNameFixtureEntry>& OutEntries, FString& OutError)
{
    OutEntries = BaseEntries;

    auto EnsureIndex = [&OutEntries, &OutError](int32 Index) -> bool {
        if (!OutEntries.IsValidIndex(Index))
        {
            OutError = FString::Printf(TEXT("Name index out of range for fixture build: %d (count=%d)."), Index, OutEntries.Num());
            return false;
        }
        return true;
    };

    if (bBefore)
    {
        if (Spec.Name == TEXT("name_remove"))
        {
            AppendNameEntry(OutEntries, TEXT("BPX_RemoveTail"));
        }
        else if (Spec.Name == TEXT("name_remove_unicode"))
        {
            AppendNameEntry(OutEntries, TEXT("Tail削除"));
        }
        else if (Spec.Name == TEXT("name_remove_hash"))
        {
            AppendNameEntry(OutEntries, TEXT("TailHash"));
        }
        return true;
    }

    if (Spec.Expect == TEXT("error_equal"))
    {
        return true;
    }

    if (Spec.Name == TEXT("name_add"))
    {
        AppendNameEntry(OutEntries, TEXT("BPX_OpName"));
    }
    else if (Spec.Name == TEXT("name_add_hash_override"))
    {
        AppendNameEntry(OutEntries, TEXT("BPX_OpNameHash"), uint16(1234), uint16(5678));
    }
    else if (Spec.Name == TEXT("name_add_unicode"))
    {
        AppendNameEntry(OutEntries, TEXT("名前追加"));
    }
    else if (Spec.Name == TEXT("name_add_long_ascii"))
    {
        AppendNameEntry(OutEntries, TEXT("BPX_Operation_Name_Long"));
    }
    else if (Spec.Name == TEXT("name_add_hash_override_alt"))
    {
        AppendNameEntry(OutEntries, TEXT("BPX_ManualHashAlt"), uint16(4321), uint16(8765));
    }
    else if (Spec.Name == TEXT("name_set"))
    {
        if (!EnsureIndex(1))
        {
            return false;
        }
        ComputeNameEntryHashes(TEXT("BP_Empty_Renamed"), OutEntries[1].NonCaseHash, OutEntries[1].CasePreservingHash);
        OutEntries[1].Value = TEXT("BP_Empty_Renamed");
    }
    else if (Spec.Name == TEXT("name_set_hash_override"))
    {
        if (!EnsureIndex(1))
        {
            return false;
        }
        OutEntries[1].Value = TEXT("BP_Empty_ManualHash");
        OutEntries[1].NonCaseHash = 2345;
        OutEntries[1].CasePreservingHash = 6789;
    }
    else if (Spec.Name == TEXT("name_set_unicode"))
    {
        if (!EnsureIndex(1))
        {
            return false;
        }
        ComputeNameEntryHashes(TEXT("名前変更"), OutEntries[1].NonCaseHash, OutEntries[1].CasePreservingHash);
        OutEntries[1].Value = TEXT("名前変更");
    }
    else if (Spec.Name == TEXT("name_set_hash_only"))
    {
        if (!EnsureIndex(1))
        {
            return false;
        }
        OutEntries[1].Value = TEXT("/Script/CoreUObject");
        OutEntries[1].NonCaseHash = 1111;
        OutEntries[1].CasePreservingHash = 2222;
    }
    else if (Spec.Name == TEXT("name_set_hash_override_alt"))
    {
        if (!EnsureIndex(2))
        {
            return false;
        }
        OutEntries[2].Value = TEXT("BPX_Name_Alt");
        OutEntries[2].NonCaseHash = 3333;
        OutEntries[2].CasePreservingHash = 4444;
    }
    else if (Spec.Name == TEXT("name_set_case_variant"))
    {
        if (!EnsureIndex(3))
        {
            return false;
        }
        ComputeNameEntryHashes(TEXT("core_redirects_case"), OutEntries[3].NonCaseHash, OutEntries[3].CasePreservingHash);
        OutEntries[3].Value = TEXT("core_redirects_case");
    }
    else if (Spec.Name == TEXT("name_set_ascii_alt"))
    {
        if (!EnsureIndex(4))
        {
            return false;
        }
        ComputeNameEntryHashes(TEXT("BPX_Name_Ascii"), OutEntries[4].NonCaseHash, OutEntries[4].CasePreservingHash);
        OutEntries[4].Value = TEXT("BPX_Name_Ascii");
    }
    else if (Spec.Name == TEXT("name_remove")
        || Spec.Name == TEXT("name_remove_unicode")
        || Spec.Name == TEXT("name_remove_hash"))
    {
        if (OutEntries.Num() == 0)
        {
            OutError = TEXT("No tail NameMap entry available for removal.");
            return false;
        }
        OutEntries.RemoveAt(OutEntries.Num() - 1, 1, EAllowShrinking::No);
    }
    else
    {
        OutError = FString::Printf(TEXT("Unsupported name fixture operation: %s"), *Spec.Name);
        return false;
    }

    return true;
}

bool WriteBytesToOutput(const TArray64<uint8>& Bytes, const FString& OutputPath, bool bForce, FString& OutError)
{
    IPlatformFile& PlatformFile = FPlatformFileManager::Get().GetPlatformFile();
    const FString OutputDir = FPaths::GetPath(OutputPath);
    if (!OutputDir.IsEmpty())
    {
        PlatformFile.CreateDirectoryTree(*OutputDir);
    }
    if (PlatformFile.FileExists(*OutputPath))
    {
        if (!bForce)
        {
            OutError = FString::Printf(TEXT("Destination already exists: %s"), *OutputPath);
            return false;
        }
        if (!PlatformFile.DeleteFile(*OutputPath))
        {
            OutError = FString::Printf(TEXT("Failed to delete existing destination: %s"), *OutputPath);
            return false;
        }
    }
    if (!FFileHelper::SaveArrayToFile(Bytes, *OutputPath))
    {
        OutError = FString::Printf(TEXT("Failed to write output bytes: %s"), *OutputPath);
        return false;
    }
    return true;
}

void PopulateStringTableFixture(UStringTable* StringTable)
{
    if (!StringTable)
    {
        return;
    }

    FStringTableRef StringTableRef = StringTable->GetMutableStringTable();
    StringTableRef->SetNamespace(TEXT("UI"));
    StringTableRef->SetSourceString(TEXT("BTN_OK"), TEXT("OK"));
    StringTableRef->SetSourceString(TEXT("BTN_CANCEL"), TEXT("Cancel"));
    StringTableRef->SetSourceString(TEXT("BTN_START"), TEXT("Start"));
    StringTableRef->SetSourceString(TEXT("BTN_QUIT"), TEXT("Quit"));
    StringTableRef->SetSourceString(TEXT("LBL_TITLE"), TEXT("Test Fixture"));
    StringTableRef->SetSourceString(TEXT("LBL_HP"), TEXT("HP"));
    StringTableRef->SetSourceString(TEXT("LBL_ATTACK"), TEXT("ATK"));
    StringTableRef->SetSourceString(TEXT("LBL_DEFENSE"), TEXT("DEF"));
    StringTableRef->SetSourceString(TEXT("BTN_CONTINUE"), TEXT("Continue"));
    StringTableRef->SetSourceString(TEXT("BTN_CANCEL_JP"), TEXT("キャンセル"));
}

bool ApplyStringTableOperationAfterState(UStringTable* StringTable, const FOperationFixtureSpec& Spec, FString& OutError)
{
    if (!StringTable)
    {
        OutError = TEXT("StringTable is null.");
        return false;
    }
    if (Spec.Expect == TEXT("error_equal"))
    {
        return true;
    }

    FStringTableRef StringTableRef = StringTable->GetMutableStringTable();
    if (Spec.Name == TEXT("stringtable_remove_entry"))
    {
        StringTableRef->RemoveSourceString(TEXT("BTN_CANCEL"));
    }
    else if (Spec.Name == TEXT("stringtable_set_namespace"))
    {
        StringTableRef->SetNamespace(TEXT("UI.Common"));
    }
    else if (Spec.Name == TEXT("stringtable_write_entry"))
    {
        StringTableRef->SetSourceString(TEXT("BTN_OK"), TEXT("Confirm"));
    }
    else if (Spec.Name == TEXT("stringtable_write_entry_unicode"))
    {
        StringTableRef->SetSourceString(TEXT("BTN_CANCEL"), TEXT("キャンセル"));
    }
    else if (Spec.Name == TEXT("stringtable_write_entry_btn_start"))
    {
        StringTableRef->SetSourceString(TEXT("BTN_START"), TEXT("Begin"));
    }
    else if (Spec.Name == TEXT("stringtable_write_entry_title_unicode"))
    {
        StringTableRef->SetSourceString(TEXT("LBL_TITLE"), TEXT("テスト題名"));
    }
    else if (Spec.Name == TEXT("stringtable_remove_entry_btn_start"))
    {
        StringTableRef->RemoveSourceString(TEXT("BTN_START"));
    }
    else if (Spec.Name == TEXT("stringtable_set_namespace_alt"))
    {
        StringTableRef->SetNamespace(TEXT("UI.Menu"));
    }
    else if (Spec.Name == TEXT("stringtable_set_namespace_unicode"))
    {
        StringTableRef->SetNamespace(TEXT("UI.共通"));
    }
    else
    {
        OutError = FString::Printf(TEXT("Unsupported stringtable operation: %s"), *Spec.Name);
        return false;
    }

    StringTable->MarkPackageDirty();
    return true;
}

UWorld* CreateLevelFixtureWorld(const FString& PackageName, const FString& AssetName)
{
    UPackage* Package = CreatePackage(*PackageName);
    if (!Package)
    {
        return nullptr;
    }

    UWorldFactory* WorldFactory = NewObject<UWorldFactory>();
    if (!WorldFactory)
    {
        return nullptr;
    }
    WorldFactory->WorldType = EWorldType::Editor;

    UObject* WorldObject = WorldFactory->FactoryCreateNew(
        UWorld::StaticClass(),
        Package,
        FName(*AssetName),
        RF_Public | RF_Standalone,
        nullptr,
        GWarn
    );
    UWorld* World = Cast<UWorld>(WorldObject);
    if (World)
    {
        FAssetRegistryModule::AssetCreated(World);
        World->MarkPackageDirty();
    }
    return World;
}

bool ApplyLevelOperationAfterState(UWorld* World, const FOperationFixtureSpec& Spec, FString& OutError)
{
    if (!World)
    {
        OutError = TEXT("World is null.");
        return false;
    }
    if (Spec.Expect == TEXT("error_equal"))
    {
        return true;
    }

    AWorldSettings* WorldSettings = World->GetWorldSettings();
    if (!WorldSettings)
    {
        OutError = TEXT("WorldSettings is null.");
        return false;
    }

    if (Spec.Name == TEXT("level_var_set")
        || Spec.Name == TEXT("level_var_set_export_selector")
        || Spec.Name == TEXT("level_var_set_path_selector"))
    {
        if (FObjectPropertyBase* Prop = FindFProperty<FObjectPropertyBase>(AWorldSettings::StaticClass(), TEXT("NavigationSystemConfig")))
        {
            Prop->SetObjectPropertyValue_InContainer(WorldSettings, nullptr);
        }
        else
        {
            OutError = TEXT("NavigationSystemConfig property not found on WorldSettings.");
            return false;
        }
        WorldSettings->MarkPackageDirty();
        World->MarkPackageDirty();
        return true;
    }

    OutError = FString::Printf(TEXT("Unsupported level operation: %s"), *Spec.Name);
    return false;
}

bool ShouldIgnoreSavedHash(const FOperationFixtureSpec& Spec)
{
    if (Spec.Expect == TEXT("error_equal")
        || IsParseBlueprintOperation(Spec)
        || IsDataTableUpdateOperation(Spec)
        || Spec.Name == TEXT("prop_set_custom_struct_enum"))
    {
        return false;
    }

    return IsSinglePackageOperation(Spec)
        || IsDataTableOperation(Spec)
        || IsCompositeDataTableRejectOperation(Spec)
        || Spec.Name == TEXT("localization_rekey")
        || Spec.Name == TEXT("localization_rewrite_namespace");
}

bool ConfigureOperationBlueprintVariables(UBlueprint* Blueprint, const FOperationFixtureSpec& Spec, FString& OutError)
{
    if (!Blueprint)
    {
        OutError = TEXT("Blueprint is null.");
        return false;
    }

    if (UsesNativeOperationFixtureParent(Spec))
    {
        if (!Blueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null for %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    auto AddVariable = [Blueprint](const FName& VariableName, const FEdGraphPinType& PinType, const FString& DefaultValue) {
        AddBlueprintMemberVariable(Blueprint, VariableName, PinType, DefaultValue);
    };

    if (Spec.Name == TEXT("prop_set_bool"))
    {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_Boolean;
        AddVariable(TEXT("FixtureValue"), PinType, TEXT("false"));
    }
    else if (Spec.Name == TEXT("prop_set_int_negative") || Spec.Name == TEXT("prop_set_int_max") || Spec.Name == TEXT("prop_set_int_min"))
    {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_Int;
        AddVariable(TEXT("FixtureValue"), PinType, TEXT("1"));
    }
    else if (Spec.Name == TEXT("prop_set_int64"))
    {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_Int64;
        AddVariable(TEXT("FixtureValue"), PinType, TEXT("1"));
    }
    else if (Spec.Name == TEXT("prop_set_float") || Spec.Name == TEXT("prop_set_float_special"))
    {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_Real;
        PinType.PinSubCategory = UEdGraphSchema_K2::PC_Float;
        AddVariable(TEXT("FixtureValue"), PinType, TEXT("1.0"));
    }
    else if (Spec.Name == TEXT("prop_set_double"))
    {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_Real;
        PinType.PinSubCategory = UEdGraphSchema_K2::PC_Double;
        AddVariable(TEXT("FixtureValue"), PinType, TEXT("1.0"));
    }
    else if (Spec.Name == TEXT("prop_set_string_same_len") || Spec.Name == TEXT("prop_set_string_diff_len") || Spec.Name == TEXT("prop_set_string_empty") || Spec.Name == TEXT("prop_set_string_unicode"))
    {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_String;
        AddVariable(TEXT("FixtureValue"), PinType, TEXT("Hello"));
    }
    else if (Spec.Name == TEXT("prop_set_name"))
    {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_Name;
        AddVariable(TEXT("FixtureName"), PinType, TEXT("Actor"));
    }
    else if (Spec.Name == TEXT("prop_set_vector"))
    {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_Struct;
        PinType.PinSubCategoryObject = TBaseStructure<FVector>::Get();
        AddVariable(TEXT("FixtureVector"), PinType, TEXT("(X=0.0,Y=0.0,Z=0.0)"));
    }
    else if (Spec.Name == TEXT("prop_set_rotator"))
    {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_Struct;
        PinType.PinSubCategoryObject = TBaseStructure<FRotator>::Get();
        AddVariable(TEXT("FixtureRotator"), PinType, TEXT("(Pitch=0.0,Yaw=0.0,Roll=0.0)"));
    }
    else if (Spec.Name == TEXT("prop_set_array_element"))
    {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_Int;
        PinType.ContainerType = EPinContainerType::Array;
        AddVariable(TEXT("MyArray"), PinType, TEXT(""));
    }
    else if (Spec.Name == TEXT("prop_set_map_value"))
    {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_String;
        PinType.ContainerType = EPinContainerType::Map;
        PinType.PinValueType.TerminalCategory = UEdGraphSchema_K2::PC_Int;
        AddVariable(TEXT("MyMap"), PinType, TEXT(""));
    }
    else if (Spec.Name == TEXT("prop_set_int") || Spec.Name == TEXT("var_set_default_int"))
    {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_String;
        AddVariable(TEXT("MyStr"), PinType, TEXT("hello"));
    }
    else if (Spec.Name == TEXT("var_rename_simple"))
    {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_Int;
        AddVariable(TEXT("OldVar"), PinType, TEXT("1"));
    }
    else if (Spec.Name == TEXT("var_rename_with_refs"))
    {
        FEdGraphPinType IntType;
        IntType.PinCategory = UEdGraphSchema_K2::PC_Int;
        AddVariable(TEXT("UsedVar"), IntType, TEXT("1"));

        FEdGraphPinType StringType;
        StringType.PinCategory = UEdGraphSchema_K2::PC_String;
        AddVariable(TEXT("ConsumerRef"), StringType, TEXT("UsedVar"));
    }
    else if (Spec.Name == TEXT("var_rename_unicode"))
    {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_Int;
        AddVariable(TEXT("体力"), PinType, TEXT("100"));
    }
    else if (Spec.Name == TEXT("ref_rewrite_single"))
    {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_String;
        AddVariable(TEXT("PrimaryRef"), PinType, TEXT("/Game/Old/Mesh"));
        AddVariable(TEXT("SecondaryRef"), PinType, TEXT("/Game/Old/Mesh"));
    }
    else if (Spec.Name == TEXT("ref_rewrite_multi"))
    {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_String;
        AddVariable(TEXT("DirRefA"), PinType, TEXT("/Game/OldDir/MeshA"));
        AddVariable(TEXT("DirRefB"), PinType, TEXT("/Game/OldDir/Sub/MeshB"));
        AddVariable(TEXT("DirRefKeep"), PinType, TEXT("/Game/KeepDir/MeshC"));
    }

    Blueprint->MarkPackageDirty();
    FKismetEditorUtilities::CompileBlueprint(Blueprint);
    if (!Blueprint->GeneratedClass)
    {
        OutError = FString::Printf(TEXT("GeneratedClass is null after compile for %s"), *Spec.Name);
        return false;
    }

    return true;
}

bool ApplyOperationBlueprintState(UBlueprint* Blueprint, const FOperationFixtureSpec& Spec, bool bBefore, FString& OutError)
{
    if (!Blueprint || !Blueprint->GeneratedClass)
    {
        OutError = TEXT("Blueprint or GeneratedClass is null.");
        return false;
    }

    UObject* CDO = Blueprint->GeneratedClass->GetDefaultObject();
    if (!CDO)
    {
        OutError = TEXT("Failed to resolve CDO.");
        return false;
    }

    if (Spec.Name == TEXT("prop_set_bool"))
    {
        FBoolProperty* Prop = FindFProperty<FBoolProperty>(Blueprint->GeneratedClass, TEXT("FixtureBool"));
        if (!Prop)
        {
            OutError = TEXT("FixtureBool BoolProperty not found.");
            return false;
        }
        Prop->SetPropertyValue_InContainer(CDO, bBefore ? false : true);
    }
    else if (Spec.Name == TEXT("prop_set_enum")
        || Spec.Name == TEXT("enum_write_value")
        || Spec.Name == TEXT("enum_write_value_missing")
        || Spec.Name == TEXT("enum_write_value_to_c_reject"))
    {
        FByteProperty* Prop = FindFProperty<FByteProperty>(Blueprint->GeneratedClass, TEXT("FixtureEnum"));
        FByteProperty* AnchorProp = FindFProperty<FByteProperty>(Blueprint->GeneratedClass, TEXT("FixtureEnumAnchor"));
        FByteProperty* AnchorAltProp = FindFProperty<FByteProperty>(Blueprint->GeneratedClass, TEXT("FixtureEnumAnchorAlt"));
        if (!Prop || !AnchorProp || !AnchorAltProp)
        {
            OutError = TEXT("FixtureEnum enum byte property not found.");
            return false;
        }
        uint8 Value = static_cast<uint8>(bBefore ? BPXEnum_ValueB : BPXEnum_ValueA);
        if (!bBefore && Spec.Name == TEXT("enum_write_value_to_c_reject"))
        {
            Value = static_cast<uint8>(BPXEnum_ValueC);
        }
        Prop->SetPropertyValue_InContainer(CDO, Value);
        AnchorProp->SetPropertyValue_InContainer(CDO, static_cast<uint8>(BPXEnum_ValueB));
        AnchorAltProp->SetPropertyValue_InContainer(CDO, static_cast<uint8>(BPXEnum_ValueA));
    }
    else if (Spec.Name == TEXT("prop_set_enum_numeric")
        || Spec.Name == TEXT("enum_write_value_numeric")
        || Spec.Name == TEXT("enum_write_value_numeric_zero")
        || Spec.Name == TEXT("enum_write_value_numeric_two"))
    {
        FByteProperty* Prop = FindFProperty<FByteProperty>(Blueprint->GeneratedClass, TEXT("FixtureEnum"));
        FByteProperty* AnchorProp = FindFProperty<FByteProperty>(Blueprint->GeneratedClass, TEXT("FixtureEnumAnchor"));
        FByteProperty* AnchorAltProp = FindFProperty<FByteProperty>(Blueprint->GeneratedClass, TEXT("FixtureEnumAnchorAlt"));
        if (!Prop || !AnchorProp || !AnchorAltProp)
        {
            OutError = TEXT("FixtureEnum enum byte property not found.");
            return false;
        }
        uint8 Value = static_cast<uint8>(bBefore ? BPXEnum_ValueA : BPXEnum_ValueB);
        if (!bBefore && Spec.Name == TEXT("enum_write_value_numeric_zero"))
        {
            Value = static_cast<uint8>(BPXEnum_ValueA);
        }
        else if (!bBefore && Spec.Name == TEXT("enum_write_value_numeric_two"))
        {
            Value = static_cast<uint8>(BPXEnum_ValueC);
        }
        Prop->SetPropertyValue_InContainer(CDO, Value);
        AnchorProp->SetPropertyValue_InContainer(CDO, static_cast<uint8>(BPXEnum_ValueB));
        AnchorAltProp->SetPropertyValue_InContainer(CDO, static_cast<uint8>(BPXEnum_ValueA));
    }
    else if (Spec.Name == TEXT("prop_set_enum_anchor")
        || Spec.Name == TEXT("enum_write_value_anchor")
        || Spec.Name == TEXT("enum_write_value_anchor_alt"))
    {
        FByteProperty* Prop = FindFProperty<FByteProperty>(Blueprint->GeneratedClass, TEXT("FixtureEnumAnchor"));
        FByteProperty* EnumProp = FindFProperty<FByteProperty>(Blueprint->GeneratedClass, TEXT("FixtureEnum"));
        FByteProperty* AnchorAltProp = FindFProperty<FByteProperty>(Blueprint->GeneratedClass, TEXT("FixtureEnumAnchorAlt"));
        if (!Prop || !EnumProp || !AnchorAltProp)
        {
            OutError = TEXT("FixtureEnumAnchor enum byte property not found.");
            return false;
        }
        EnumProp->SetPropertyValue_InContainer(CDO, static_cast<uint8>(BPXEnum_ValueA));
        uint8 AnchorValue = static_cast<uint8>(bBefore ? BPXEnum_ValueB : BPXEnum_ValueA);
        uint8 AnchorAltValue = static_cast<uint8>(BPXEnum_ValueB);
        if (!bBefore && Spec.Name == TEXT("enum_write_value_anchor_alt"))
        {
            AnchorValue = static_cast<uint8>(BPXEnum_ValueB);
            AnchorAltValue = static_cast<uint8>(BPXEnum_ValueB);
        }
        Prop->SetPropertyValue_InContainer(CDO, AnchorValue);
        AnchorAltProp->SetPropertyValue_InContainer(CDO, AnchorAltValue);
    }
    else if (Spec.Name == TEXT("prop_set_int_negative"))
    {
        FIntProperty* Prop = FindFProperty<FIntProperty>(Blueprint->GeneratedClass, TEXT("FixtureInt"));
        if (!Prop)
        {
            OutError = TEXT("FixtureInt IntProperty not found.");
            return false;
        }
        Prop->SetPropertyValue_InContainer(CDO, bBefore ? 1 : -1);
    }
    else if (Spec.Name == TEXT("prop_set_int_max"))
    {
        FIntProperty* Prop = FindFProperty<FIntProperty>(Blueprint->GeneratedClass, TEXT("FixtureInt"));
        if (!Prop)
        {
            OutError = TEXT("FixtureInt IntProperty not found.");
            return false;
        }
        Prop->SetPropertyValue_InContainer(CDO, bBefore ? 1 : MAX_int32);
    }
    else if (Spec.Name == TEXT("prop_set_int_min"))
    {
        FIntProperty* Prop = FindFProperty<FIntProperty>(Blueprint->GeneratedClass, TEXT("FixtureInt"));
        if (!Prop)
        {
            OutError = TEXT("FixtureInt IntProperty not found.");
            return false;
        }
        Prop->SetPropertyValue_InContainer(CDO, bBefore ? 1 : MIN_int32);
    }
    else if (Spec.Name == TEXT("prop_set_int64"))
    {
        FInt64Property* Prop = FindFProperty<FInt64Property>(Blueprint->GeneratedClass, TEXT("FixtureInt64"));
        if (!Prop)
        {
            OutError = TEXT("FixtureInt64 Int64Property not found.");
            return false;
        }
        Prop->SetPropertyValue_InContainer(CDO, bBefore ? 1 : MAX_int64);
    }
    else if (Spec.Name == TEXT("prop_set_float"))
    {
        FFloatProperty* Prop = FindFProperty<FFloatProperty>(Blueprint->GeneratedClass, TEXT("FixtureFloat"));
        if (!Prop)
        {
            OutError = TEXT("FixtureFloat FloatProperty not found.");
            return false;
        }
        Prop->SetPropertyValue_InContainer(CDO, bBefore ? 1.0f : 3.14f);
    }
    else if (Spec.Name == TEXT("prop_set_float_special"))
    {
        FFloatProperty* Prop = FindFProperty<FFloatProperty>(Blueprint->GeneratedClass, TEXT("FixtureFloat"));
        if (!Prop)
        {
            OutError = TEXT("FixtureFloat FloatProperty not found.");
            return false;
        }
        Prop->SetPropertyValue_InContainer(CDO, bBefore ? 1.0f : 1e-38f);
    }
    else if (Spec.Name == TEXT("prop_set_double"))
    {
        FDoubleProperty* Prop = FindFProperty<FDoubleProperty>(Blueprint->GeneratedClass, TEXT("FixtureDouble"));
        if (!Prop)
        {
            OutError = TEXT("FixtureDouble DoubleProperty not found.");
            return false;
        }
        Prop->SetPropertyValue_InContainer(CDO, bBefore ? 1.0 : 2.718281828);
    }
    else if (Spec.Name == TEXT("prop_set_string_same_len"))
    {
        FStrProperty* Prop = FindFProperty<FStrProperty>(Blueprint->GeneratedClass, TEXT("MyStr"));
        if (!Prop)
        {
            OutError = TEXT("MyStr StrProperty not found.");
            return false;
        }
        Prop->SetPropertyValue_InContainer(CDO, bBefore ? TEXT("Hello") : TEXT("World"));
    }
    else if (Spec.Name == TEXT("prop_set_string_diff_len"))
    {
        FStrProperty* Prop = FindFProperty<FStrProperty>(Blueprint->GeneratedClass, TEXT("MyStr"));
        if (!Prop)
        {
            OutError = TEXT("MyStr StrProperty not found.");
            return false;
        }
        Prop->SetPropertyValue_InContainer(CDO, bBefore ? TEXT("Hi") : TEXT("Hello World"));
    }
    else if (Spec.Name == TEXT("prop_set_string_empty"))
    {
        FStrProperty* Prop = FindFProperty<FStrProperty>(Blueprint->GeneratedClass, TEXT("MyStr"));
        if (!Prop)
        {
            OutError = TEXT("MyStr StrProperty not found.");
            return false;
        }
        Prop->SetPropertyValue_InContainer(CDO, bBefore ? TEXT("Hello") : TEXT(""));
    }
    else if (Spec.Name == TEXT("prop_set_string_unicode"))
    {
        FStrProperty* Prop = FindFProperty<FStrProperty>(Blueprint->GeneratedClass, TEXT("MyStr"));
        if (!Prop)
        {
            OutError = TEXT("MyStr StrProperty not found.");
            return false;
        }
        Prop->SetPropertyValue_InContainer(CDO, bBefore ? TEXT("test") : TEXT("テスト"));
    }
    else if (Spec.Name == TEXT("prop_set_string_long_expand"))
    {
        FStrProperty* Prop = FindFProperty<FStrProperty>(Blueprint->GeneratedClass, TEXT("MyStr"));
        if (!Prop)
        {
            OutError = TEXT("MyStr StrProperty not found.");
            return false;
        }
        Prop->SetPropertyValue_InContainer(CDO, bBefore ? TEXT("tiny") : TEXT("Lorem ipsum dolor sit amet, consectetur adipiscing elit 0123456789"));
    }
    else if (Spec.Name == TEXT("prop_set_string_shrink"))
    {
        FStrProperty* Prop = FindFProperty<FStrProperty>(Blueprint->GeneratedClass, TEXT("MyStr"));
        if (!Prop)
        {
            OutError = TEXT("MyStr StrProperty not found.");
            return false;
        }
        Prop->SetPropertyValue_InContainer(CDO, bBefore ? TEXT("This text is intentionally long for shrink testing.") : TEXT("x"));
    }
    else if (Spec.Name == TEXT("prop_set_name"))
    {
        FNameProperty* Prop = FindFProperty<FNameProperty>(Blueprint->GeneratedClass, TEXT("FixtureName"));
        if (!Prop)
        {
            OutError = TEXT("FixtureName NameProperty not found.");
            return false;
        }
        Prop->SetPropertyValue_InContainer(CDO, bBefore ? FName(TEXT("BlueprintType")) : FName(TEXT("BoolProperty")));
    }
    else if (Spec.Name == TEXT("prop_set_vector"))
    {
        FStructProperty* Prop = FindFProperty<FStructProperty>(Blueprint->GeneratedClass, TEXT("FixtureVector"));
        if (!Prop || Prop->Struct != TBaseStructure<FVector>::Get())
        {
            OutError = TEXT("FixtureVector FVector property not found.");
            return false;
        }
        FVector* Ptr = Prop->ContainerPtrToValuePtr<FVector>(CDO);
        if (!Ptr)
        {
            OutError = TEXT("Failed to access FixtureVector storage.");
            return false;
        }
        *Ptr = bBefore ? FVector(0.25, -0.5, 0.75) : FVector(1.5, -2.3, 100.0);
    }
    else if (Spec.Name == TEXT("prop_set_vector_axis_x"))
    {
        FStructProperty* Prop = FindFProperty<FStructProperty>(Blueprint->GeneratedClass, TEXT("FixtureVector"));
        if (!Prop || Prop->Struct != TBaseStructure<FVector>::Get())
        {
            OutError = TEXT("FixtureVector FVector property not found.");
            return false;
        }
        FVector* Ptr = Prop->ContainerPtrToValuePtr<FVector>(CDO);
        if (!Ptr)
        {
            OutError = TEXT("Failed to access FixtureVector storage.");
            return false;
        }
        *Ptr = bBefore ? FVector(10.0, 2.0, 3.0) : FVector(-123.456, 2.0, 3.0);
    }
    else if (Spec.Name == TEXT("prop_set_rotator"))
    {
        FStructProperty* Prop = FindFProperty<FStructProperty>(Blueprint->GeneratedClass, TEXT("FixtureRotator"));
        if (!Prop || Prop->Struct != TBaseStructure<FRotator>::Get())
        {
            OutError = TEXT("FixtureRotator FRotator property not found.");
            return false;
        }
        FRotator* Ptr = Prop->ContainerPtrToValuePtr<FRotator>(CDO);
        if (!Ptr)
        {
            OutError = TEXT("Failed to access FixtureRotator storage.");
            return false;
        }
        *Ptr = bBefore ? FRotator(1.0, 2.0, 3.0) : FRotator(45.0, 90.0, 180.0);
    }
    else if (Spec.Name == TEXT("prop_set_rotator_axis_roll"))
    {
        FStructProperty* Prop = FindFProperty<FStructProperty>(Blueprint->GeneratedClass, TEXT("FixtureRotator"));
        if (!Prop || Prop->Struct != TBaseStructure<FRotator>::Get())
        {
            OutError = TEXT("FixtureRotator FRotator property not found.");
            return false;
        }
        FRotator* Ptr = Prop->ContainerPtrToValuePtr<FRotator>(CDO);
        if (!Ptr)
        {
            OutError = TEXT("Failed to access FixtureRotator storage.");
            return false;
        }
        *Ptr = bBefore ? FRotator(10.0, 20.0, 30.0) : FRotator(10.0, 20.0, -45.5);
    }
    else if (Spec.Name == TEXT("prop_set_array_element"))
    {
        FArrayProperty* Prop = FindFProperty<FArrayProperty>(Blueprint->GeneratedClass, TEXT("MyArray"));
        FIntProperty* Inner = Prop ? CastField<FIntProperty>(Prop->Inner) : nullptr;
        if (!Prop || !Inner)
        {
            OutError = TEXT("MyArray int array property not found.");
            return false;
        }
        void* ArrayPtr = Prop->ContainerPtrToValuePtr<void>(CDO);
        FScriptArrayHelper Helper(Prop, ArrayPtr);
        Helper.EmptyValues();
        const TArray<int32> Values = bBefore ? TArray<int32>{1, 10, 3} : TArray<int32>{1, 99, 3};
        for (const int32 Value : Values)
        {
            const int32 Index = Helper.AddValue();
            Inner->SetPropertyValue(Helper.GetRawPtr(Index), Value);
        }
    }
    else if (Spec.Name == TEXT("prop_set_array_replace_longer"))
    {
        FArrayProperty* Prop = FindFProperty<FArrayProperty>(Blueprint->GeneratedClass, TEXT("MyArray"));
        FIntProperty* Inner = Prop ? CastField<FIntProperty>(Prop->Inner) : nullptr;
        if (!Prop || !Inner)
        {
            OutError = TEXT("MyArray int array property not found.");
            return false;
        }
        void* ArrayPtr = Prop->ContainerPtrToValuePtr<void>(CDO);
        FScriptArrayHelper Helper(Prop, ArrayPtr);
        Helper.EmptyValues();
        const TArray<int32> Values = bBefore ? TArray<int32>{1, 2} : TArray<int32>{1, 2, 3, 4, 5, 6, 7, 8};
        for (const int32 Value : Values)
        {
            const int32 Index = Helper.AddValue();
            Inner->SetPropertyValue(Helper.GetRawPtr(Index), Value);
        }
    }
    else if (Spec.Name == TEXT("prop_set_array_replace_empty"))
    {
        FArrayProperty* Prop = FindFProperty<FArrayProperty>(Blueprint->GeneratedClass, TEXT("MyArray"));
        FIntProperty* Inner = Prop ? CastField<FIntProperty>(Prop->Inner) : nullptr;
        if (!Prop || !Inner)
        {
            OutError = TEXT("MyArray int array property not found.");
            return false;
        }
        void* ArrayPtr = Prop->ContainerPtrToValuePtr<void>(CDO);
        FScriptArrayHelper Helper(Prop, ArrayPtr);
        Helper.EmptyValues();
        const TArray<int32> Values = bBefore ? TArray<int32>{4, 5, 6} : TArray<int32>{4};
        for (const int32 Value : Values)
        {
            const int32 Index = Helper.AddValue();
            Inner->SetPropertyValue(Helper.GetRawPtr(Index), Value);
        }
    }
    else if (Spec.Name == TEXT("prop_set_map_value"))
    {
        FMapProperty* Prop = FindFProperty<FMapProperty>(Blueprint->GeneratedClass, TEXT("MyMap"));
        FStrProperty* KeyProp = Prop ? CastField<FStrProperty>(Prop->KeyProp) : nullptr;
        FIntProperty* ValueProp = Prop ? CastField<FIntProperty>(Prop->ValueProp) : nullptr;
        if (!Prop || !KeyProp || !ValueProp)
        {
            OutError = TEXT("MyMap map<string,int> property not found.");
            return false;
        }

        void* MapPtr = Prop->ContainerPtrToValuePtr<void>(CDO);
        FScriptMapHelper Helper(Prop, MapPtr);
        Helper.EmptyValues();
        const int32 Index = Helper.AddDefaultValue_Invalid_NeedsRehash();
        KeyProp->SetPropertyValue(Helper.GetKeyPtr(Index), TEXT("key"));
        ValueProp->SetPropertyValue(Helper.GetValuePtr(Index), bBefore ? 10 : 99);
        Helper.Rehash();
    }
    else if (Spec.Name == TEXT("prop_set_custom_struct_int"))
    {
        FStructProperty* Prop = FindFProperty<FStructProperty>(Blueprint->GeneratedClass, TEXT("FixtureCustom"));
        if (!Prop || Prop->Struct == nullptr)
        {
            OutError = TEXT("FixtureCustom StructProperty not found.");
            return false;
        }
        void* StructPtr = Prop->ContainerPtrToValuePtr<void>(CDO);
        if (!StructPtr)
        {
            OutError = TEXT("Failed to access FixtureCustom storage.");
            return false;
        }
        FIntProperty* IntValProp = FindFProperty<FIntProperty>(Prop->Struct, TEXT("IntVal"));
        if (!IntValProp)
        {
            OutError = TEXT("FixtureCustom.IntVal field not found.");
            return false;
        }
        IntValProp->SetPropertyValue_InContainer(StructPtr, bBefore ? 1 : 42);
    }
    else if (Spec.Name == TEXT("prop_set_custom_struct_enum"))
    {
        FStructProperty* Prop = FindFProperty<FStructProperty>(Blueprint->GeneratedClass, TEXT("FixtureCustom"));
        FByteProperty* AnchorProp = FindFProperty<FByteProperty>(Blueprint->GeneratedClass, TEXT("FixtureEnumAnchor"));
        FByteProperty* AnchorAltProp = FindFProperty<FByteProperty>(Blueprint->GeneratedClass, TEXT("FixtureEnumAnchorAlt"));
        if (!Prop || Prop->Struct == nullptr)
        {
            OutError = TEXT("FixtureCustom StructProperty not found.");
            return false;
        }
        if (!AnchorProp || !AnchorAltProp)
        {
            OutError = TEXT("Fixture enum anchor byte properties not found.");
            return false;
        }
        void* StructPtr = Prop->ContainerPtrToValuePtr<void>(CDO);
        if (!StructPtr)
        {
            OutError = TEXT("Failed to access FixtureCustom storage.");
            return false;
        }
        FByteProperty* EnumValProp = FindFProperty<FByteProperty>(Prop->Struct, TEXT("EnumVal"));
        if (!EnumValProp)
        {
            OutError = TEXT("FixtureCustom.EnumVal field not found.");
            return false;
        }
        EnumValProp->SetPropertyValue_InContainer(StructPtr, static_cast<uint8>(bBefore ? BPXEnum_ValueC : BPXEnum_ValueB));
        AnchorProp->SetPropertyValue_InContainer(CDO, static_cast<uint8>(BPXEnum_ValueB));
        AnchorAltProp->SetPropertyValue_InContainer(CDO, static_cast<uint8>(BPXEnum_ValueA));
    }
    else if (Spec.Name == TEXT("prop_set_int")
        || Spec.Name == TEXT("var_set_default_int")
        || Spec.Name == TEXT("var_set_default_empty")
        || Spec.Name == TEXT("var_set_default_unicode")
        || Spec.Name == TEXT("var_set_default_long"))
    {
        FStrProperty* Prop = FindFProperty<FStrProperty>(Blueprint->GeneratedClass, TEXT("MyStr"));
        if (!Prop)
        {
            OutError = TEXT("MyStr StrProperty not found.");
            return false;
        }
        FString BeforeValue = TEXT("hello");
        FString AfterValue = TEXT("changed");
        if (Spec.Name == TEXT("var_set_default_empty"))
        {
            BeforeValue = TEXT("hello");
            AfterValue = TEXT("");
        }
        else if (Spec.Name == TEXT("var_set_default_unicode"))
        {
            BeforeValue = TEXT("test");
            AfterValue = TEXT("テスト");
        }
        else if (Spec.Name == TEXT("var_set_default_long"))
        {
            BeforeValue = TEXT("tiny");
            AfterValue = TEXT("Lorem ipsum dolor sit amet var-default");
        }
        Prop->SetPropertyValue_InContainer(CDO, bBefore ? BeforeValue : AfterValue);
    }
    else if (Spec.Name == TEXT("var_rename_simple")
        || Spec.Name == TEXT("var_rename_with_refs")
        || Spec.Name == TEXT("var_rename_unicode"))
    {
        const FName OldName =
            Spec.Name == TEXT("var_rename_simple") ? FName(TEXT("OldVar")) :
            Spec.Name == TEXT("var_rename_with_refs") ? FName(TEXT("UsedVar")) :
            FName(TEXT("体力"));
        const FName NewName =
            Spec.Name == TEXT("var_rename_simple") ? FName(TEXT("NewVar")) :
            Spec.Name == TEXT("var_rename_with_refs") ? FName(TEXT("RenamedVar")) :
            FName(TEXT("HP"));

        if (bBefore)
        {
            if (FBlueprintEditorUtils::FindNewVariableIndex(Blueprint, OldName) == INDEX_NONE)
            {
                OutError = FString::Printf(TEXT("Expected source variable %s was not found."), *OldName.ToString());
                return false;
            }
            Blueprint->MarkPackageDirty();
            return true;
        }

        if (FBlueprintEditorUtils::FindNewVariableIndex(Blueprint, OldName) == INDEX_NONE)
        {
            OutError = FString::Printf(TEXT("Expected source variable %s was not found before rename."), *OldName.ToString());
            return false;
        }
        FBlueprintEditorUtils::RenameMemberVariable(Blueprint, OldName, NewName);
        FKismetEditorUtilities::CompileBlueprint(Blueprint);
        if (FBlueprintEditorUtils::FindNewVariableIndex(Blueprint, NewName) == INDEX_NONE)
        {
            OutError = FString::Printf(TEXT("Expected renamed variable %s was not found."), *NewName.ToString());
            return false;
        }
        if (!Blueprint->GeneratedClass || FindFProperty<FProperty>(Blueprint->GeneratedClass, NewName) == nullptr)
        {
            OutError = FString::Printf(TEXT("Expected generated-class property %s was not found after compile."), *NewName.ToString());
            return false;
        }
        Blueprint->MarkPackageDirty();
        return true;
    }
    else if (Spec.Name == TEXT("ref_rewrite_single"))
    {
        FStrProperty* PrimaryProp = FindFProperty<FStrProperty>(Blueprint->GeneratedClass, TEXT("PrimaryRef"));
        FStrProperty* SecondaryProp = FindFProperty<FStrProperty>(Blueprint->GeneratedClass, TEXT("SecondaryRef"));
        if (!PrimaryProp || !SecondaryProp)
        {
            OutError = TEXT("ref_rewrite_single string properties not found.");
            return false;
        }
        const FString Value = bBefore ? TEXT("/Game/Old/Mesh") : TEXT("/Game/New/Mesh");
        PrimaryProp->SetPropertyValue_InContainer(CDO, Value);
        SecondaryProp->SetPropertyValue_InContainer(CDO, Value);
    }
    else if (Spec.Name == TEXT("ref_rewrite_multi"))
    {
        FStrProperty* RefAProp = FindFProperty<FStrProperty>(Blueprint->GeneratedClass, TEXT("DirRefA"));
        FStrProperty* RefBProp = FindFProperty<FStrProperty>(Blueprint->GeneratedClass, TEXT("DirRefB"));
        FStrProperty* RefKeepProp = FindFProperty<FStrProperty>(Blueprint->GeneratedClass, TEXT("DirRefKeep"));
        if (!RefAProp || !RefBProp || !RefKeepProp)
        {
            OutError = TEXT("ref_rewrite_multi string properties not found.");
            return false;
        }
        RefAProp->SetPropertyValue_InContainer(CDO, bBefore ? TEXT("/Game/OldDir/MeshA") : TEXT("/Game/NewDir/MeshA"));
        RefBProp->SetPropertyValue_InContainer(CDO, bBefore ? TEXT("/Game/OldDir/Sub/MeshB") : TEXT("/Game/NewDir/Sub/MeshB"));
        RefKeepProp->SetPropertyValue_InContainer(CDO, TEXT("/Game/KeepDir/MeshC"));
    }
    else if (Spec.Name == TEXT("export_set_header"))
    {
        Blueprint->MarkPackageDirty();
        return true;
    }
    else
    {
        OutError = FString::Printf(TEXT("Unsupported single-package operation: %s"), *Spec.Name);
        return false;
    }

    CDO->MarkPackageDirty();
    Blueprint->MarkPackageDirty();
    return true;
}

bool ApplyParseBlueprintOperationAfterState(UBlueprint* Blueprint, const FOperationFixtureSpec& Spec, FString& OutError)
{
    if (!Blueprint || !Blueprint->GeneratedClass)
    {
        OutError = TEXT("Blueprint or GeneratedClass is null.");
        return false;
    }

    if (Spec.Name == TEXT("metadata_set_tooltip")
        || Spec.Name == TEXT("prop_set_soft_object")
        || Spec.Name == TEXT("prop_set_nested_array_struct"))
    {
        return true;
    }

    if (Spec.Name == TEXT("package_set_flags")
        || Spec.Name == TEXT("package_set_flags_raw")
        || Spec.Name == TEXT("package_set_flags_runtimegenerated")
        || Spec.Name == TEXT("package_set_flags_clear_zero")
        || Spec.Name == TEXT("package_set_flags_filtereditoronly_reject")
        || Spec.Name == TEXT("package_set_flags_unversionedprops_reject"))
    {
        UPackage* Package = Blueprint->GetOutermost();
        if (!Package)
        {
            OutError = TEXT("Blueprint package is null.");
            return false;
        }
        if (Spec.Name == TEXT("package_set_flags_clear_zero"))
        {
            Package->ClearPackageFlags(Package->GetPackageFlags());
        }
        else if (Spec.Name == TEXT("package_set_flags_runtimegenerated"))
        {
            Package->ClearPackageFlags(Package->GetPackageFlags());
            Package->SetPackageFlags(PKG_RuntimeGenerated);
        }
        else if (Spec.Name == TEXT("package_set_flags_raw"))
        {
            Package->ClearPackageFlags(Package->GetPackageFlags());
            Package->SetPackageFlags(EPackageFlags(PKG_RequiresLocalizationGather | PKG_RuntimeGenerated));
        }
        else if (Spec.Name == TEXT("package_set_flags_filtereditoronly_reject"))
        {
            Package->SetPackageFlags(PKG_FilterEditorOnly);
        }
        else if (Spec.Name == TEXT("package_set_flags_unversionedprops_reject"))
        {
            Package->SetPackageFlags(PKG_UnversionedProperties);
        }
        else
        {
            Package->SetPackageFlags(PKG_RequiresLocalizationGather | PKG_RuntimeGenerated);
        }
        Package->MarkPackageDirty();
        Blueprint->MarkPackageDirty();
        return true;
    }

    if (Spec.Name == TEXT("prop_set_text")
        || Spec.Name == TEXT("metadata_set_category")
        || Spec.Name == TEXT("metadata_set_category_unicode")
        || Spec.Name == TEXT("metadata_set_category_ascii_alt")
        || Spec.Name == TEXT("metadata_set_object")
        || Spec.Name == TEXT("metadata_set_object_unicode")
        || Spec.Name == TEXT("metadata_set_object_empty")
        || Spec.Name == TEXT("localization_set_source")
        || Spec.Name == TEXT("localization_set_source_unicode")
        || Spec.Name == TEXT("localization_set_source_alt_ascii")
        || Spec.Name == TEXT("localization_set_source_empty")
        || Spec.Name == TEXT("localization_set_id_base_text")
        || Spec.Name == TEXT("localization_set_id_base_text_alt")
        || Spec.Name == TEXT("localization_set_stringtable_ref")
        || Spec.Name == TEXT("localization_set_stringtable_ref_alt_key")
        || Spec.Name == TEXT("localization_rekey")
        || Spec.Name == TEXT("localization_rekey_alt")
        || Spec.Name == TEXT("localization_rewrite_namespace")
        || Spec.Name == TEXT("localization_rewrite_namespace_alt"))
    {
        USimpleConstructionScript* SCS = Blueprint->SimpleConstructionScript;
        if (!SCS)
        {
            OutError = TEXT("SimpleConstructionScript is null.");
            return false;
        }

        const TArray<USCS_Node*>& RootNodes = SCS->GetRootNodes();
        if (RootNodes.Num() == 0 || !RootNodes[0])
        {
            OutError = TEXT("SimpleConstructionScript root node not found.");
            return false;
        }

        FTextProperty* CategoryProp = FindFProperty<FTextProperty>(USCS_Node::StaticClass(), TEXT("CategoryName"));
        if (!CategoryProp)
        {
            OutError = TEXT("USCS_Node.CategoryName TextProperty not found.");
            return false;
        }

        FText* CategoryValue = CategoryProp->ContainerPtrToValuePtr<FText>(RootNodes[0]);
        if (!CategoryValue)
        {
            OutError = TEXT("Failed to access SCS node CategoryName storage.");
            return false;
        }

        if (Spec.Name == TEXT("prop_set_text")
            || Spec.Name == TEXT("metadata_set_category")
            || Spec.Name == TEXT("metadata_set_object")
            || Spec.Name == TEXT("localization_set_source"))
        {
            *CategoryValue = FText::ChangeKey(TEXT("SCS"), TEXT("Default"), FText::FromString(TEXT("Gameplay")));
        }
        else if (Spec.Name == TEXT("metadata_set_category_unicode")
            || Spec.Name == TEXT("metadata_set_object_unicode")
            || Spec.Name == TEXT("localization_set_source_unicode"))
        {
            *CategoryValue = FText::ChangeKey(TEXT("SCS"), TEXT("Default"), FText::FromString(TEXT("ゲームプレイ")));
        }
        else if (Spec.Name == TEXT("metadata_set_category_ascii_alt"))
        {
            *CategoryValue = FText::ChangeKey(TEXT("SCS"), TEXT("Default"), FText::FromString(TEXT("UI")));
        }
        else if (Spec.Name == TEXT("localization_set_source_alt_ascii"))
        {
            *CategoryValue = FText::ChangeKey(TEXT("SCS"), TEXT("Default"), FText::FromString(TEXT("HUD")));
        }
        else if (Spec.Name == TEXT("metadata_set_object_empty")
            || Spec.Name == TEXT("localization_set_source_empty"))
        {
            *CategoryValue = FText::ChangeKey(TEXT("SCS"), TEXT("Default"), FText::FromString(TEXT("")));
        }
        else if (Spec.Name == TEXT("localization_set_id_base_text"))
        {
            *CategoryValue = FText::ChangeKey(TEXT("UI"), TEXT("BTN_OK"), FText::FromString(TEXT("Default")));
        }
        else if (Spec.Name == TEXT("localization_set_id_base_text_alt"))
        {
            *CategoryValue = FText::ChangeKey(TEXT("UI"), TEXT("HUD_TITLE"), FText::FromString(TEXT("Default")));
        }
        else if (Spec.Name == TEXT("localization_set_stringtable_ref"))
        {
            *CategoryValue = FText::FromStringTable(FName(TEXT("SimpleConstructionScript")), TEXT("BTN_OK"));
        }
        else if (Spec.Name == TEXT("localization_set_stringtable_ref_alt_key"))
        {
            *CategoryValue = FText::FromStringTable(FName(TEXT("SimpleConstructionScript")), TEXT("BTN_CANCEL"));
        }
        else if (Spec.Name == TEXT("localization_rekey"))
        {
            *CategoryValue = FText::ChangeKey(TEXT("SCS"), TEXT("MainMenu"), FText::FromString(TEXT("Default")));
        }
        else if (Spec.Name == TEXT("localization_rekey_alt"))
        {
            *CategoryValue = FText::ChangeKey(TEXT("SCS"), TEXT("HUDTitle"), FText::FromString(TEXT("Default")));
        }
        else if (Spec.Name == TEXT("localization_rewrite_namespace"))
        {
            *CategoryValue = FText::ChangeKey(TEXT("UI"), TEXT("Default"), FText::FromString(TEXT("Default")));
        }
        else if (Spec.Name == TEXT("localization_rewrite_namespace_alt"))
        {
            *CategoryValue = FText::ChangeKey(TEXT("HUD"), TEXT("Default"), FText::FromString(TEXT("Default")));
        }
        else
        {
            OutError = FString::Printf(TEXT("Unsupported parse-blueprint text operation: %s"), *Spec.Name);
            return false;
        }
        RootNodes[0]->MarkPackageDirty();
        Blueprint->MarkPackageDirty();
        return true;
    }

    if (Spec.Name == TEXT("localization_set_id")
        || Spec.Name == TEXT("localization_set_id_alt_key"))
    {
        USimpleConstructionScript* SCS = Blueprint->SimpleConstructionScript;
        if (!SCS)
        {
            OutError = TEXT("SimpleConstructionScript is null.");
            return false;
        }
        const TArray<USCS_Node*>& RootNodes = SCS->GetRootNodes();
        if (RootNodes.Num() == 0 || !RootNodes[0])
        {
            OutError = TEXT("SimpleConstructionScript root node not found.");
            return false;
        }
        FTextProperty* CategoryProp = FindFProperty<FTextProperty>(USCS_Node::StaticClass(), TEXT("CategoryName"));
        if (!CategoryProp)
        {
            OutError = TEXT("USCS_Node.CategoryName TextProperty not found.");
            return false;
        }
        FText* CategoryValue = CategoryProp->ContainerPtrToValuePtr<FText>(RootNodes[0]);
        if (!CategoryValue)
        {
            OutError = TEXT("Failed to access SCS node CategoryName storage.");
            return false;
        }
        *CategoryValue = FText::FromStringTable(
            FName(TEXT("SimpleConstructionScript")),
            Spec.Name == TEXT("localization_set_id") ? TEXT("BTN_OK") : TEXT("BTN_CANCEL")
        );
        RootNodes[0]->MarkPackageDirty();
        Blueprint->MarkPackageDirty();
        return true;
    }

    UObject* CDO = Blueprint->GeneratedClass->GetDefaultObject();
    if (!CDO)
    {
        OutError = TEXT("Failed to resolve CDO.");
        return false;
    }

    if (Spec.Name == TEXT("prop_set_bool"))
    {
        FBoolProperty* Prop = FindFProperty<FBoolProperty>(Blueprint->GeneratedClass, TEXT("VBool"));
        if (!Prop)
        {
            OutError = TEXT("VBool BoolProperty not found.");
            return false;
        }
        Prop->SetPropertyValue_InContainer(CDO, false);
    }
    else if (Spec.Name == TEXT("var_set_default_string"))
    {
        FStrProperty* Prop = FindFProperty<FStrProperty>(Blueprint->GeneratedClass, TEXT("VString"));
        if (!Prop)
        {
            OutError = TEXT("VString StrProperty not found.");
            return false;
        }
        Prop->SetPropertyValue_InContainer(CDO, TEXT("golden"));
    }
    else if (Spec.Name == TEXT("var_set_default_vector"))
    {
        FStructProperty* Prop = FindFProperty<FStructProperty>(Blueprint->GeneratedClass, TEXT("VVector"));
        if (!Prop || Prop->Struct != TBaseStructure<FVector>::Get())
        {
            OutError = TEXT("VVector FVector property not found.");
            return false;
        }
        FVector* Value = Prop->ContainerPtrToValuePtr<FVector>(CDO);
        if (!Value)
        {
            OutError = TEXT("Failed to access VVector storage.");
            return false;
        }
        *Value = FVector(1.0, 2.0, 3.0);
    }
    else if (Spec.Name == TEXT("prop_set_color"))
    {
        FStructProperty* Prop = FindFProperty<FStructProperty>(Blueprint->GeneratedClass, TEXT("VColor"));
        if (!Prop || Prop->Struct != TBaseStructure<FLinearColor>::Get())
        {
            OutError = TEXT("VColor LinearColor property not found.");
            return false;
        }
        FLinearColor* Value = Prop->ContainerPtrToValuePtr<FLinearColor>(CDO);
        if (!Value)
        {
            OutError = TEXT("Failed to access VColor storage.");
            return false;
        }
        *Value = FLinearColor(0.25f, 0.5f, 0.75f, 1.0f);
    }
    else if (Spec.Name == TEXT("prop_set_transform") || Spec.Name == TEXT("prop_set_nested_struct"))
    {
        FStructProperty* Prop = FindFProperty<FStructProperty>(Blueprint->GeneratedClass, TEXT("VTransform"));
        if (!Prop || Prop->Struct != TBaseStructure<FTransform>::Get())
        {
            OutError = TEXT("VTransform Transform property not found.");
            return false;
        }
        FTransform* Value = Prop->ContainerPtrToValuePtr<FTransform>(CDO);
        if (!Value)
        {
            OutError = TEXT("Failed to access VTransform storage.");
            return false;
        }

        if (Spec.Name == TEXT("prop_set_transform"))
        {
            *Value = FTransform(FQuat::Identity, FVector(1.0, 2.0, 3.0), FVector(1.0, 1.0, 1.0));
        }
        else
        {
            Value->SetTranslation(FVector(1.0, 2.0, 3.0));
        }
    }
    else
    {
        OutError = FString::Printf(TEXT("Unsupported parse-blueprint operation: %s"), *Spec.Name);
        return false;
    }

    CDO->MarkPackageDirty();
    Blueprint->MarkPackageDirty();
    return true;
}

bool ApplyDataTableUpdateAfterState(UDataTable* FixtureDataTable, const FOperationFixtureSpec& Spec, FString& OutError)
{
    if (!FixtureDataTable)
    {
        OutError = TEXT("DataTable is null.");
        return false;
    }

    if (Spec.Expect == TEXT("error_equal"))
    {
        return true;
    }

    FBPXOperationTableRow* RowA = FixtureDataTable->FindRow<FBPXOperationTableRow>(TEXT("Row_A"), TEXT("ApplyDataTableUpdateAfterState"));
    FBPXOperationTableRow* RowB = FixtureDataTable->FindRow<FBPXOperationTableRow>(TEXT("Row_B"), TEXT("ApplyDataTableUpdateAfterState"));
    if (!RowA || !RowB)
    {
        OutError = TEXT("Expected DataTable rows Row_A/Row_B not found.");
        return false;
    }

    if (Spec.Name == TEXT("dt_update_int"))
    {
        RowA->Score = 999;
    }
    else if (Spec.Name == TEXT("dt_update_float"))
    {
        RowB->Rate = 1.25f;
    }
    else if (Spec.Name == TEXT("dt_update_string"))
    {
        RowA->Label = TEXT("NewName");
    }
    else if (Spec.Name == TEXT("dt_update_multi_field"))
    {
        RowA->Score = 50;
        RowA->Rate = 0.1f;
    }
    else
    {
        OutError = FString::Printf(TEXT("Unsupported datatable update operation: %s"), *Spec.Name);
        return false;
    }

    FixtureDataTable->MarkPackageDirty();
    return true;
}

FOperationBlueprintDefaults ResolveOperationBlueprintDefaults(const FOperationFixtureSpec& Spec)
{
    if (Spec.Name == TEXT("prop_add"))
    {
        return {TEXT(""), TEXT("1")};
    }
    if (Spec.Name == TEXT("prop_remove"))
    {
        return {TEXT("1"), TEXT("")};
    }

    // Default operation fixtures compare one scalar default update (0 -> 1).
    return {TEXT("0"), TEXT("1")};
}

bool SavePackageToDisk(UPackage* Package, UObject* TopLevelObject, const FString& PackageFilename, FString& OutError,
    EObjectFlags TopLevelFlags = EObjectFlags(RF_Public | RF_Standalone))
{
    if (!Package || !TopLevelObject)
    {
        OutError = TEXT("SavePackageToDisk received null Package or TopLevelObject.");
        return false;
    }

    const FString ResolvedFilename = FPaths::ConvertRelativePathToFull(PackageFilename);
    const FString Directory = FPaths::GetPath(ResolvedFilename);
    IPlatformFile& PlatformFile = FPlatformFileManager::Get().GetPlatformFile();
    if (!Directory.IsEmpty() && !PlatformFile.CreateDirectoryTree(*Directory))
    {
        OutError = FString::Printf(TEXT("Failed to create directory for package save: %s"), *Directory);
        return false;
    }

    Package->MarkPackageDirty();

    FSavePackageArgs SaveArgs;
    SaveArgs.TopLevelFlags = TopLevelFlags;
    SaveArgs.Error = GWarn;
    SaveArgs.SaveFlags = SAVE_None;

    if (!UPackage::SavePackage(Package, TopLevelObject, *ResolvedFilename, SaveArgs))
    {
        OutError = FString::Printf(TEXT("Failed to save package: %s (resolved from: %s)"), *ResolvedFilename, *PackageFilename);
        return false;
    }

    if (!PlatformFile.FileExists(*ResolvedFilename))
    {
        OutError = FString::Printf(TEXT("SavePackage reported success but file does not exist: %s"), *ResolvedFilename);
        return false;
    }

    return true;
}

bool CopyFileToOutput(const FString& SourceFile, const FString& DestinationFile, bool bForce, FString& OutError)
{
    IPlatformFile& PlatformFile = FPlatformFileManager::Get().GetPlatformFile();

    if (!PlatformFile.FileExists(*SourceFile))
    {
        OutError = FString::Printf(TEXT("Source file not found: %s"), *SourceFile);
        return false;
    }

    const FString DestinationDir = FPaths::GetPath(DestinationFile);
    if (!DestinationDir.IsEmpty())
    {
        PlatformFile.CreateDirectoryTree(*DestinationDir);
    }

    if (PlatformFile.FileExists(*DestinationFile))
    {
        if (!bForce)
        {
            OutError = FString::Printf(TEXT("Destination already exists: %s"), *DestinationFile);
            return false;
        }

        if (!PlatformFile.DeleteFile(*DestinationFile))
        {
            OutError = FString::Printf(TEXT("Failed to delete existing destination file: %s"), *DestinationFile);
            return false;
        }
    }

    if (!PlatformFile.CopyFile(*DestinationFile, *SourceFile))
    {
        OutError = FString::Printf(TEXT("Failed to copy file: %s -> %s"), *SourceFile, *DestinationFile);
        return false;
    }

    return true;
}

FString WidgetBlueprintSourceObjectPath(const FString& FixtureKey)
{
    FParseFixtureSpec ParseSpec;
    if (FindParseFixtureSpecByKey(FixtureKey, ParseSpec) && ParseSpec.Kind == EParseFixtureKind::WidgetBlueprint)
    {
        return FString::Printf(TEXT("/Game/BPXFixtures/Parse/%s.%s"), *FixtureKey, *FixtureKey);
    }
    if (FixtureKey.StartsWith(TEXT("WBP_CanvasPanel_")))
    {
        return FString::Printf(TEXT("/Game/WBP/Panel/CanvasPanel/%s.%s"), *FixtureKey, *FixtureKey);
    }
    if (FixtureKey.StartsWith(TEXT("WBP_Overlay_")))
    {
        return FString::Printf(TEXT("/Game/WBP/Panel/Overlay/%s.%s"), *FixtureKey, *FixtureKey);
    }
    if (FixtureKey.StartsWith(TEXT("WBP_VerticalBox_")))
    {
        return FString::Printf(TEXT("/Game/WBP/Panel/VerticalBox/%s.%s"), *FixtureKey, *FixtureKey);
    }
    if (FixtureKey.StartsWith(TEXT("WBP_HorizontalBox_")))
    {
        return FString::Printf(TEXT("/Game/WBP/Panel/HorizontalBox/%s.%s"), *FixtureKey, *FixtureKey);
    }
    return FString::Printf(TEXT("/Game/WBP/%s.%s"), *FixtureKey, *FixtureKey);
}

UWidgetBlueprint* DuplicateWidgetBlueprintAsset(const FString& SourceFixtureKey, const FString& DestinationPackageName, const FString& DestinationAssetName, FString& OutError)
{
    const FString PrimarySourceObjectPath = WidgetBlueprintSourceObjectPath(SourceFixtureKey);
    const FString TopLevelWBPObjectPath = FString::Printf(TEXT("/Game/WBP/%s.%s"), *SourceFixtureKey, *SourceFixtureKey);

    TArray<FString> CandidateSourceObjectPaths;
    CandidateSourceObjectPaths.Add(PrimarySourceObjectPath);
    if (SourceFixtureKey.StartsWith(TEXT("WBP_")) && PrimarySourceObjectPath != TopLevelWBPObjectPath)
    {
        CandidateSourceObjectPaths.Add(TopLevelWBPObjectPath);
    }

    UWidgetBlueprint* SourceBlueprint = nullptr;
    for (const FString& CandidatePath : CandidateSourceObjectPaths)
    {
        SourceBlueprint = LoadObject<UWidgetBlueprint>(nullptr, *CandidatePath);
        if (SourceBlueprint)
        {
            break;
        }
    }
    if (!SourceBlueprint)
    {
        OutError = FString::Printf(TEXT("Failed to load source WidgetBlueprint from any candidate path: %s"), *FString::Join(CandidateSourceObjectPaths, TEXT(", ")));
        return nullptr;
    }

    UPackage* DestinationPackage = CreatePackage(*DestinationPackageName);
    if (!DestinationPackage)
    {
        OutError = FString::Printf(TEXT("Failed to create destination package for WidgetBlueprint: %s"), *DestinationPackageName);
        return nullptr;
    }

    UWidgetBlueprint* DuplicatedBlueprint = Cast<UWidgetBlueprint>(
        StaticDuplicateObject(SourceBlueprint, DestinationPackage, FName(*DestinationAssetName))
    );
    if (!DuplicatedBlueprint)
    {
        OutError = FString::Printf(TEXT("Failed to duplicate WidgetBlueprint %s into %s"), *SourceFixtureKey, *DestinationPackageName);
        return nullptr;
    }

    FAssetRegistryModule::AssetCreated(DuplicatedBlueprint);
    DuplicatedBlueprint->MarkPackageDirty();
    FKismetEditorUtilities::CompileBlueprint(DuplicatedBlueprint);
    return DuplicatedBlueprint;
}

UWidget* FindChildWidgetByName(UWidget* ParentWidget, const FString& ChildName)
{
    if (!ParentWidget)
    {
        return nullptr;
    }

    if (UPanelWidget* PanelWidget = Cast<UPanelWidget>(ParentWidget))
    {
        for (int32 Index = 0; Index < PanelWidget->GetChildrenCount(); ++Index)
        {
            UWidget* Child = PanelWidget->GetChildAt(Index);
            if (Child && Child->GetName() == ChildName)
            {
                return Child;
            }
        }
    }

    if (UContentWidget* ContentWidget = Cast<UContentWidget>(ParentWidget))
    {
        UWidget* Child = ContentWidget->GetContent();
        if (Child && Child->GetName() == ChildName)
        {
            return Child;
        }
    }

    return nullptr;
}

int32 GetDirectWidgetChildCount(UWidget* Widget)
{
    if (!Widget)
    {
        return 0;
    }

    if (UPanelWidget* PanelWidget = Cast<UPanelWidget>(Widget))
    {
        return PanelWidget->GetChildrenCount();
    }

    if (UContentWidget* ContentWidget = Cast<UContentWidget>(Widget))
    {
        return ContentWidget->GetContent() ? 1 : 0;
    }

    return 0;
}

UWidget* ResolveWidgetByPath(UWidgetTree* WidgetTree, const FString& WidgetPath, FString& OutError)
{
    if (!WidgetTree)
    {
        OutError = TEXT("WidgetTree is null.");
        return nullptr;
    }
    if (!WidgetTree->RootWidget)
    {
        OutError = TEXT("WidgetTree root widget is null.");
        return nullptr;
    }

    TArray<FString> Segments;
    WidgetPath.ParseIntoArray(Segments, TEXT("/"), true);
    if (Segments.Num() == 0)
    {
        OutError = TEXT("Widget path is empty.");
        return nullptr;
    }

    UWidget* Current = WidgetTree->RootWidget;
    if (Current->GetName() != Segments[0])
    {
        Current = WidgetTree->FindWidget(FName(*Segments[0]));
        if (!Current)
        {
            OutError = FString::Printf(TEXT("Widget path root not found: %s"), *Segments[0]);
            return nullptr;
        }
    }

    for (int32 SegmentIndex = 1; SegmentIndex < Segments.Num(); ++SegmentIndex)
    {
        Current = FindChildWidgetByName(Current, Segments[SegmentIndex]);
        if (!Current)
        {
            OutError = FString::Printf(TEXT("Widget path segment not found: %s"), *Segments[SegmentIndex]);
            return nullptr;
        }
    }

    return Current;
}

bool RemoveWidgetBlueprintLeaf(UWidgetBlueprint* WidgetBlueprint, const FString& WidgetPath, FString& OutError)
{
    if (!WidgetBlueprint)
    {
        OutError = TEXT("WidgetBlueprint is null.");
        return false;
    }
    if (!WidgetBlueprint->WidgetTree)
    {
        OutError = TEXT("WidgetTree is null.");
        return false;
    }

    UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, WidgetPath, OutError);
    if (!TargetWidget)
    {
        return false;
    }
    if (WidgetBlueprint->WidgetTree->RootWidget == TargetWidget)
    {
        OutError = FString::Printf(TEXT("Refusing to remove root widget: %s"), *WidgetPath);
        return false;
    }
    if (GetDirectWidgetChildCount(TargetWidget) != 0)
    {
        OutError = FString::Printf(TEXT("Refusing to remove non-leaf widget: %s"), *WidgetPath);
        return false;
    }

    int32 ChildIndex = INDEX_NONE;
    UPanelWidget* ParentPanel = UWidgetTree::FindWidgetParent(TargetWidget, ChildIndex);
    if (!ParentPanel)
    {
        OutError = FString::Printf(TEXT("Widget parent not found for %s"), *WidgetPath);
        return false;
    }
    const FName TargetName = TargetWidget->GetFName();

    FWidgetBlueprintEditorUtils::DeleteWidgets(
        WidgetBlueprint,
        TSet<UWidget*>{TargetWidget},
        FWidgetBlueprintEditorUtils::EDeleteWidgetWarningType::DeleteSilently
    );
    if (WidgetBlueprint->WidgetTree->FindWidget(TargetName))
    {
        OutError = FString::Printf(TEXT("WidgetTree still resolves removed widget: %s"), *WidgetPath);
        return false;
    }
    if (WidgetBlueprint->GetAllSourceWidgets().ContainsByPredicate([TargetName](const UWidget* Widget)
    {
        return Widget && Widget->GetFName() == TargetName;
    }))
    {
        OutError = FString::Printf(TEXT("Removed widget still present in source widget list: %s"), *WidgetPath);
        return false;
    }

    ParentPanel->MarkPackageDirty();
    WidgetBlueprint->WidgetTree->MarkPackageDirty();
    WidgetBlueprint->MarkPackageDirty();
    FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
    if (!WidgetBlueprint->GeneratedClass)
    {
        OutError = FString::Printf(TEXT("GeneratedClass is null after compiling remove op for %s"), *WidgetPath);
        return false;
    }

    return true;
}

template <typename TWidget>
bool AddBareCanvasPanelLeafWidget(
    UWidgetBlueprint* WidgetBlueprint,
    const TCHAR* ParentPath,
    const TCHAR* WidgetName,
    const TCHAR* SpecName,
    FString& OutError)
{
    if (!WidgetBlueprint || !WidgetBlueprint->WidgetTree)
    {
        OutError = FString::Printf(TEXT("WidgetTree is null for %s"), SpecName);
        return false;
    }

    UWidget* ParentWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, ParentPath, OutError);
    UCanvasPanel* CanvasPanel = Cast<UCanvasPanel>(ParentWidget);
    if (!CanvasPanel)
    {
        OutError = FString::Printf(TEXT("Resolved widget is not a CanvasPanel for %s"), ParentPath);
        return false;
    }

    WidgetBlueprint->Modify();
    CanvasPanel->Modify();
    TWidget* ChildWidget = WidgetBlueprint->WidgetTree->ConstructWidget<TWidget>(TWidget::StaticClass(), WidgetName);
    if (!ChildWidget)
    {
        OutError = FString::Printf(TEXT("ConstructWidget returned null for %s"), SpecName);
        return false;
    }
    if (!CanvasPanel->AddChildToCanvas(ChildWidget))
    {
        OutError = FString::Printf(TEXT("AddChildToCanvas returned null for %s"), SpecName);
        return false;
    }
    if (!WidgetBlueprint->WidgetVariableNameToGuidMap.Contains(ChildWidget->GetFName()))
    {
        WidgetBlueprint->OnVariableAdded(ChildWidget->GetFName());
    }
    ChildWidget->Modify();
    ChildWidget->MarkPackageDirty();
    CanvasPanel->MarkPackageDirty();
    WidgetBlueprint->MarkPackageDirty();
    FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
    if (!WidgetBlueprint->GeneratedClass)
    {
        OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), SpecName);
        return false;
    }
    return true;
}

bool ApplyWidgetBlueprintOperationAfterState(UWidgetBlueprint* WidgetBlueprint, const FOperationFixtureSpec& Spec, FString& OutError)
{
    if (!WidgetBlueprint)
    {
        OutError = TEXT("WidgetBlueprint is null.");
        return false;
    }

    // --- Text write operations ---
    if (Spec.Name == TEXT("widget_parent_class_commonactivatablewidget_rootless"))
    {
        if (!WidgetBlueprint->WidgetTree)
        {
            OutError = TEXT("WidgetTree is null for widget_parent_class_commonactivatablewidget_rootless");
            return false;
        }
        if (WidgetBlueprint->WidgetTree->RootWidget != nullptr)
        {
            OutError = TEXT("RootWidget must be null for widget_parent_class_commonactivatablewidget_rootless");
            return false;
        }

        UClass* ParentClass = LoadClass<UUserWidget>(nullptr, TEXT("/Script/CommonUI.CommonActivatableWidget"));
        if (!ParentClass)
        {
            OutError = TEXT("LoadClass failed for /Script/CommonUI.CommonActivatableWidget");
            return false;
        }

        WidgetBlueprint->Modify();
        WidgetBlueprint->ParentClass = ParentClass;
        FBlueprintEditorUtils::MarkBlueprintAsStructurallyModified(WidgetBlueprint);
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    // --- Text write operations ---
    if (Spec.Name == TEXT("widget_write_text_root_textblock")
        || Spec.Name == TEXT("widget_write_text_canvaspanel_child")
        || Spec.Name == TEXT("widget_write_text_overlay_child")
        || Spec.Name == TEXT("widget_write_text_root_richtextblock")
        || Spec.Name == TEXT("widget_write_text_canvaspanel_child_richtextblock"))
    {
        FString WidgetPath;
        FString NewTextValue;
        const bool bRichTextBlock =
            Spec.Name == TEXT("widget_write_text_root_richtextblock")
            || Spec.Name == TEXT("widget_write_text_canvaspanel_child_richtextblock");
        if (Spec.Name == TEXT("widget_write_text_root_textblock"))
        {
            WidgetPath = TEXT("TextBlock_72");
            NewTextValue = TEXT("Updated root text");
        }
        else if (Spec.Name == TEXT("widget_write_text_root_richtextblock"))
        {
            WidgetPath = TEXT("RichTextBlock_72");
            NewTextValue = TEXT("Updated root rich text");
        }
        else if (Spec.Name == TEXT("widget_write_text_canvaspanel_child"))
        {
            WidgetPath = TEXT("CanvasPanel_22/TextBlock_31");
            NewTextValue = TEXT("Updated canvas child text");
        }
        else if (Spec.Name == TEXT("widget_write_text_canvaspanel_child_richtextblock"))
        {
            WidgetPath = TEXT("CanvasPanel_22/RichTextBlock_31");
            NewTextValue = TEXT("Updated rich canvas child text");
        }
        else
        {
            WidgetPath = TEXT("Overlay_116/TextBlock_36");
            NewTextValue = TEXT("Updated overlay child text");
        }

        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, WidgetPath, OutError);
        if (bRichTextBlock)
        {
            URichTextBlock* RichTextBlock = Cast<URichTextBlock>(TargetWidget);
            if (!RichTextBlock)
            {
                OutError = FString::Printf(TEXT("Resolved widget is not a RichTextBlock for %s"), *WidgetPath);
                return false;
            }

            RichTextBlock->Modify();
            RichTextBlock->SetText(FText::FromString(NewTextValue));
            RichTextBlock->MarkPackageDirty();
        }
        else
        {
            UTextBlock* TextBlock = Cast<UTextBlock>(TargetWidget);
            if (!TextBlock)
            {
                OutError = FString::Printf(TEXT("Resolved widget is not a TextBlock for %s"), *WidgetPath);
                return false;
            }

            TextBlock->Modify();
            TextBlock->SetText(FText::FromString(NewTextValue));
            TextBlock->MarkPackageDirty();
        }
        WidgetBlueprint->Modify();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    // --- RenderOpacity write operations ---
    if (Spec.Name == TEXT("widget_write_opacity_root_textblock"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("TextBlock_72"), OutError);
        if (!TargetWidget)
        {
            return false;
        }

        TargetWidget->Modify();
        TargetWidget->SetRenderOpacity(0.8f);
        TargetWidget->MarkPackageDirty();
        WidgetBlueprint->Modify();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_text_color_root_textblock")
        || Spec.Name == TEXT("widget_write_text_font_size_root_textblock")
        || Spec.Name == TEXT("widget_write_text_justification_root_textblock")
        || Spec.Name == TEXT("widget_write_text_font_root_textblock")
        || Spec.Name == TEXT("widget_write_text_typeface_root_textblock")
        || Spec.Name == TEXT("widget_write_text_auto_wrap_text_root_textblock")
        || Spec.Name == TEXT("widget_write_text_wrap_text_at_root_textblock")
        || Spec.Name == TEXT("widget_write_text_line_height_percentage_root_textblock")
        || Spec.Name == TEXT("widget_write_text_shadow_offset_root_textblock")
        || Spec.Name == TEXT("widget_write_text_shadow_color_root_textblock")
        || Spec.Name == TEXT("widget_write_text_outline_size_root_textblock")
        || Spec.Name == TEXT("widget_write_text_outline_color_root_textblock"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("TextBlock_72"), OutError);
        UTextBlock* TextBlock = Cast<UTextBlock>(TargetWidget);
        if (!TextBlock)
        {
            OutError = TEXT("Resolved widget is not a TextBlock for root TextBlock style operation");
            return false;
        }

        WidgetBlueprint->Modify();
        TextBlock->Modify();
        if (Spec.Name == TEXT("widget_write_text_color_root_textblock"))
        {
            TextBlock->SetColorAndOpacity(FSlateColor(FLinearColor(0.15f, 0.45f, 0.75f, 0.9f)));
        }
        else if (Spec.Name == TEXT("widget_write_text_font_size_root_textblock"))
        {
            FSlateFontInfo FontInfo = TextBlock->GetFont();
            FontInfo.Size = 28;
            TextBlock->SetFont(FontInfo);
        }
        else if (Spec.Name == TEXT("widget_write_text_font_root_textblock"))
        {
            UObject* FontObject = LoadObject<UObject>(nullptr, TEXT("/Game/UI/Foundation/Fonts/NotoSans.NotoSans"));
            if (!FontObject)
            {
                OutError = TEXT("LoadObject failed for /Game/UI/Foundation/Fonts/NotoSans.NotoSans");
                return false;
            }
            FSlateFontInfo FontInfo = TextBlock->GetFont();
            FontInfo.FontObject = FontObject;
            TextBlock->SetFont(FontInfo);
        }
        else if (Spec.Name == TEXT("widget_write_text_typeface_root_textblock"))
        {
            FSlateFontInfo FontInfo = TextBlock->GetFont();
            FontInfo.TypefaceFontName = TEXT("Bold");
            TextBlock->SetFont(FontInfo);
        }
        else if (Spec.Name == TEXT("widget_write_text_auto_wrap_text_root_textblock"))
        {
            TextBlock->SetAutoWrapText(true);
        }
        else if (Spec.Name == TEXT("widget_write_text_wrap_text_at_root_textblock"))
        {
            TextBlock->SetWrapTextAt(320.0f);
        }
        else if (Spec.Name == TEXT("widget_write_text_line_height_percentage_root_textblock"))
        {
            TextBlock->SetLineHeightPercentage(1.25f);
        }
        else if (Spec.Name == TEXT("widget_write_text_shadow_offset_root_textblock"))
        {
            TextBlock->SetShadowOffset(FVector2D(3.0f, 4.0f));
        }
        else if (Spec.Name == TEXT("widget_write_text_shadow_color_root_textblock"))
        {
            TextBlock->SetShadowColorAndOpacity(FLinearColor(0.05f, 0.1f, 0.15f, 0.8f));
        }
        else if (Spec.Name == TEXT("widget_write_text_outline_size_root_textblock"))
        {
            FSlateFontInfo FontInfo = TextBlock->GetFont();
            FontInfo.OutlineSettings.OutlineSize = 2;
            TextBlock->SetFont(FontInfo);
        }
        else if (Spec.Name == TEXT("widget_write_text_outline_color_root_textblock"))
        {
            FSlateFontInfo FontInfo = TextBlock->GetFont();
            FontInfo.OutlineSettings.OutlineColor = FLinearColor(0.2f, 0.3f, 0.9f, 1.0f);
            TextBlock->SetFont(FontInfo);
        }
        else
        {
            TextBlock->SetJustification(ETextJustify::Center);
        }
        TextBlock->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_opacity_root_richtextblock"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("RichTextBlock_72"), OutError);
        if (!TargetWidget)
        {
            return false;
        }

        TargetWidget->Modify();
        TargetWidget->SetRenderOpacity(0.8f);
        TargetWidget->MarkPackageDirty();
        WidgetBlueprint->Modify();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_opacity_canvaspanel_child"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_22/TextBlock_31"), OutError);
        if (!TargetWidget)
        {
            return false;
        }

        TargetWidget->Modify();
        TargetWidget->SetRenderOpacity(0.5f);
        TargetWidget->MarkPackageDirty();
        WidgetBlueprint->Modify();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_visibility_root_textblock"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("TextBlock_72"), OutError);
        if (!TargetWidget)
        {
            return false;
        }

        TargetWidget->Modify();
        TargetWidget->SetVisibility(ESlateVisibility::Collapsed);
        TargetWidget->MarkPackageDirty();
        WidgetBlueprint->Modify();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_visibility_root_richtextblock"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("RichTextBlock_72"), OutError);
        if (!TargetWidget)
        {
            return false;
        }

        TargetWidget->Modify();
        TargetWidget->SetVisibility(ESlateVisibility::Collapsed);
        TargetWidget->MarkPackageDirty();
        WidgetBlueprint->Modify();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_richtext_default_shadow_offset")
        || Spec.Name == TEXT("widget_write_richtext_default_shadow_color")
        || Spec.Name == TEXT("widget_write_richtext_default_outline_size")
        || Spec.Name == TEXT("widget_write_richtext_default_outline_color")
        || Spec.Name == TEXT("widget_write_richtext_auto_wrap_text")
        || Spec.Name == TEXT("widget_write_richtext_wrap_text_at")
        || Spec.Name == TEXT("widget_write_richtext_line_height_percentage"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("RichTextBlock_72"), OutError);
        URichTextBlock* RichTextBlock = Cast<URichTextBlock>(TargetWidget);
        if (!RichTextBlock)
        {
            OutError = TEXT("Resolved widget is not a RichTextBlock for default-style RichText operation");
            return false;
        }

        RichTextBlock->Modify();
        if (Spec.Name == TEXT("widget_write_richtext_auto_wrap_text"))
        {
            RichTextBlock->SetAutoWrapText(true);
        }
        else if (Spec.Name == TEXT("widget_write_richtext_wrap_text_at"))
        {
            RichTextBlock->SetWrapTextAt(320.0f);
        }
        else if (Spec.Name == TEXT("widget_write_richtext_line_height_percentage"))
        {
            RichTextBlock->SetLineHeightPercentage(1.25f);
        }
        else if (Spec.Name == TEXT("widget_write_richtext_default_shadow_offset"))
        {
            RichTextBlock->SetDefaultShadowOffset(FVector2D(3.0f, 4.0f));
        }
        else if (Spec.Name == TEXT("widget_write_richtext_default_shadow_color"))
        {
            RichTextBlock->SetDefaultShadowColorAndOpacity(FLinearColor(0.05f, 0.1f, 0.15f, 0.8f));
        }
        else
        {
            FTextBlockStyle Style = RichTextBlock->GetCurrentDefaultTextStyle();
            if (Spec.Name == TEXT("widget_write_richtext_default_outline_size"))
            {
                Style.Font.OutlineSettings.OutlineSize = 2;
            }
            else
            {
                Style.Font.OutlineSettings.OutlineColor = FLinearColor(0.2f, 0.3f, 0.9f, 1.0f);
            }
            RichTextBlock->SetDefaultTextStyle(Style);
        }
        RichTextBlock->MarkPackageDirty();
        WidgetBlueprint->Modify();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_progressbar_percent")
        || Spec.Name == TEXT("widget_write_progressbar_fill_color"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/ProgressBar_1"), OutError);
        UProgressBar* ProgressBar = Cast<UProgressBar>(TargetWidget);
        if (!ProgressBar)
        {
            OutError = TEXT("Resolved widget is not a ProgressBar for CanvasPanel_1/ProgressBar_1");
            return false;
        }

        WidgetBlueprint->Modify();
        ProgressBar->Modify();
        if (Spec.Name == TEXT("widget_write_progressbar_percent"))
        {
            ProgressBar->Percent = 0.75f;
        }
        else
        {
            ProgressBar->FillColorAndOpacity = FLinearColor(0.2f, 0.4f, 0.6f, 0.8f);
        }
        ProgressBar->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_slider_value")
        || Spec.Name == TEXT("widget_write_slider_orientation")
        || Spec.Name == TEXT("widget_write_slider_is_focusable"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/Slider_1"), OutError);
        USlider* Slider = Cast<USlider>(TargetWidget);
        if (!Slider)
        {
            OutError = TEXT("Resolved widget is not a Slider for CanvasPanel_1/Slider_1");
            return false;
        }

        WidgetBlueprint->Modify();
        Slider->Modify();
        if (Spec.Name == TEXT("widget_write_slider_value"))
        {
            Slider->Value = 0.5f;
        }
        else if (Spec.Name == TEXT("widget_write_slider_orientation"))
        {
            Slider->Orientation = EOrientation::Orient_Vertical;
        }
        else
        {
            FBoolProperty* IsFocusableProperty = FindFProperty<FBoolProperty>(USlider::StaticClass(), TEXT("IsFocusable"));
            if (!IsFocusableProperty)
            {
                OutError = TEXT("IsFocusable property not found on USlider");
                return false;
            }
            IsFocusableProperty->SetPropertyValue_InContainer(Slider, false);
        }
        Slider->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_spacer_size"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/Spacer_1"), OutError);
        USpacer* Spacer = Cast<USpacer>(TargetWidget);
        if (!Spacer)
        {
            OutError = TEXT("Resolved widget is not a Spacer for CanvasPanel_1/Spacer_1");
            return false;
        }

        WidgetBlueprint->Modify();
        Spacer->Modify();
        Spacer->Size = FVector2D(24.0f, 48.0f);
        Spacer->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_scrollbar_thickness")
        || Spec.Name == TEXT("widget_write_scrollbar_orientation"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/ScrollBar_1"), OutError);
        UScrollBar* ScrollBar = Cast<UScrollBar>(TargetWidget);
        if (!ScrollBar)
        {
            OutError = TEXT("Resolved widget is not a ScrollBar for CanvasPanel_1/ScrollBar_1");
            return false;
        }

        WidgetBlueprint->Modify();
        ScrollBar->Modify();
        if (Spec.Name == TEXT("widget_write_scrollbar_thickness"))
        {
            ScrollBar->Thickness = FVector2D(5.0f, 12.0f);
        }
        else
        {
            ScrollBar->Orientation = EOrientation::Orient_Vertical;
        }
        ScrollBar->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_checkbox_is_checked_canvaspanel")
        || Spec.Name == TEXT("widget_write_checkbox_checked_state_canvaspanel")
        || Spec.Name == TEXT("widget_write_checkbox_is_focusable_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/CheckBox_1"), OutError);
        UCheckBox* CheckBox = Cast<UCheckBox>(TargetWidget);
        if (!CheckBox)
        {
            OutError = TEXT("Resolved widget is not a CheckBox for CanvasPanel_1/CheckBox_1");
            return false;
        }

        WidgetBlueprint->Modify();
        CheckBox->Modify();
        if (Spec.Name == TEXT("widget_write_checkbox_is_checked_canvaspanel"))
        {
            CheckBox->SetIsChecked(true);
        }
        else if (Spec.Name == TEXT("widget_write_checkbox_checked_state_canvaspanel"))
        {
            CheckBox->SetCheckedState(ECheckBoxState::Undetermined);
        }
        else
        {
            FBoolProperty* IsFocusableProperty = FindFProperty<FBoolProperty>(UCheckBox::StaticClass(), TEXT("IsFocusable"));
            if (!IsFocusableProperty)
            {
                OutError = TEXT("IsFocusable property not found on UCheckBox");
                return false;
            }
            IsFocusableProperty->SetPropertyValue_InContainer(CheckBox, false);
        }
        CheckBox->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_text_canvaspanel_child_editabletextbox")
        || Spec.Name == TEXT("widget_write_editabletextbox_hint_text_canvaspanel")
        || Spec.Name == TEXT("widget_write_editabletextbox_is_read_only_canvaspanel")
        || Spec.Name == TEXT("widget_write_editabletextbox_is_password_canvaspanel")
        || Spec.Name == TEXT("widget_write_editabletextbox_minimum_desired_width_canvaspanel")
        || Spec.Name == TEXT("widget_write_editabletextbox_justification_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/EditableTextBox_1"), OutError);
        UEditableTextBox* EditableTextBox = Cast<UEditableTextBox>(TargetWidget);
        if (!EditableTextBox)
        {
            OutError = TEXT("Resolved widget is not an EditableTextBox for CanvasPanel_1/EditableTextBox_1");
            return false;
        }

        WidgetBlueprint->Modify();
        EditableTextBox->Modify();
        if (Spec.Name == TEXT("widget_write_text_canvaspanel_child_editabletextbox"))
        {
            EditableTextBox->SetText(FText::FromString(TEXT("Player Name")));
        }
        else if (Spec.Name == TEXT("widget_write_editabletextbox_hint_text_canvaspanel"))
        {
            EditableTextBox->SetHintText(FText::FromString(TEXT("Enter name")));
        }
        else if (Spec.Name == TEXT("widget_write_editabletextbox_is_read_only_canvaspanel"))
        {
            EditableTextBox->SetIsReadOnly(true);
        }
        else if (Spec.Name == TEXT("widget_write_editabletextbox_is_password_canvaspanel"))
        {
            EditableTextBox->SetIsPassword(true);
        }
        else if (Spec.Name == TEXT("widget_write_editabletextbox_minimum_desired_width_canvaspanel"))
        {
            EditableTextBox->MinimumDesiredWidth = 240.0f;
        }
        else
        {
            EditableTextBox->SetJustification(ETextJustify::Center);
        }
        EditableTextBox->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_text_canvaspanel_child_editabletext")
        || Spec.Name == TEXT("widget_write_editabletext_hint_text_canvaspanel")
        || Spec.Name == TEXT("widget_write_editabletext_is_read_only_canvaspanel")
        || Spec.Name == TEXT("widget_write_editabletext_is_password_canvaspanel")
        || Spec.Name == TEXT("widget_write_editabletext_minimum_desired_width_canvaspanel")
        || Spec.Name == TEXT("widget_write_editabletext_justification_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/EditableText_1"), OutError);
        UEditableText* EditableText = Cast<UEditableText>(TargetWidget);
        if (!EditableText)
        {
            OutError = TEXT("Resolved widget is not an EditableText for CanvasPanel_1/EditableText_1");
            return false;
        }

        WidgetBlueprint->Modify();
        EditableText->Modify();
        if (Spec.Name == TEXT("widget_write_text_canvaspanel_child_editabletext"))
        {
            EditableText->SetText(FText::FromString(TEXT("Display Name")));
        }
        else if (Spec.Name == TEXT("widget_write_editabletext_hint_text_canvaspanel"))
        {
            EditableText->SetHintText(FText::FromString(TEXT("Enter display name")));
        }
        else if (Spec.Name == TEXT("widget_write_editabletext_is_read_only_canvaspanel"))
        {
            EditableText->SetIsReadOnly(true);
        }
        else if (Spec.Name == TEXT("widget_write_editabletext_is_password_canvaspanel"))
        {
            EditableText->SetIsPassword(true);
        }
        else if (Spec.Name == TEXT("widget_write_editabletext_minimum_desired_width_canvaspanel"))
        {
            EditableText->SetMinimumDesiredWidth(260.0f);
        }
        else
        {
            EditableText->SetJustification(ETextJustify::Right);
        }
        EditableText->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_text_canvaspanel_child_multilineeditabletextbox")
        || Spec.Name == TEXT("widget_write_multilineeditabletextbox_hint_text_canvaspanel")
        || Spec.Name == TEXT("widget_write_multilineeditabletextbox_is_read_only_canvaspanel")
        || Spec.Name == TEXT("widget_write_multilineeditabletextbox_justification_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/MultiLineEditableTextBox_1"), OutError);
        UMultiLineEditableTextBox* MultiLineEditableTextBox = Cast<UMultiLineEditableTextBox>(TargetWidget);
        if (!MultiLineEditableTextBox)
        {
            OutError = TEXT("Resolved widget is not a MultiLineEditableTextBox for CanvasPanel_1/MultiLineEditableTextBox_1");
            return false;
        }

        WidgetBlueprint->Modify();
        MultiLineEditableTextBox->Modify();
        if (Spec.Name == TEXT("widget_write_text_canvaspanel_child_multilineeditabletextbox"))
        {
            MultiLineEditableTextBox->SetText(FText::FromString(TEXT("Line 1\nLine 2")));
        }
        else if (Spec.Name == TEXT("widget_write_multilineeditabletextbox_hint_text_canvaspanel"))
        {
            MultiLineEditableTextBox->SetHintText(FText::FromString(TEXT("Enter description")));
        }
        else if (Spec.Name == TEXT("widget_write_multilineeditabletextbox_is_read_only_canvaspanel"))
        {
            MultiLineEditableTextBox->SetIsReadOnly(true);
        }
        else
        {
            MultiLineEditableTextBox->SetJustification(ETextJustify::Center);
        }
        MultiLineEditableTextBox->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_spinbox_value_canvaspanel")
        || Spec.Name == TEXT("widget_write_spinbox_min_value_canvaspanel")
        || Spec.Name == TEXT("widget_write_spinbox_max_value_canvaspanel")
        || Spec.Name == TEXT("widget_write_spinbox_delta_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/SpinBox_1"), OutError);
        USpinBox* SpinBox = Cast<USpinBox>(TargetWidget);
        if (!SpinBox)
        {
            OutError = TEXT("Resolved widget is not a SpinBox for CanvasPanel_1/SpinBox_1");
            return false;
        }

        WidgetBlueprint->Modify();
        SpinBox->Modify();
        if (Spec.Name == TEXT("widget_write_spinbox_value_canvaspanel"))
        {
            SpinBox->SetValue(42.0f);
        }
        else if (Spec.Name == TEXT("widget_write_spinbox_min_value_canvaspanel"))
        {
            SpinBox->SetMinValue(10.0f);
        }
        else if (Spec.Name == TEXT("widget_write_spinbox_max_value_canvaspanel"))
        {
            SpinBox->SetMaxValue(100.0f);
        }
        else
        {
            SpinBox->SetDelta(5.0f);
        }
        SpinBox->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_comboboxstring_options_canvaspanel")
        || Spec.Name == TEXT("widget_write_comboboxstring_selected_option_canvaspanel")
        || Spec.Name == TEXT("widget_write_comboboxstring_is_focusable_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/ComboBoxString_1"), OutError);
        UComboBoxString* ComboBoxString = Cast<UComboBoxString>(TargetWidget);
        if (!ComboBoxString)
        {
            OutError = TEXT("Resolved widget is not a ComboBoxString for CanvasPanel_1/ComboBoxString_1");
            return false;
        }

        WidgetBlueprint->Modify();
        ComboBoxString->Modify();
        if (Spec.Name == TEXT("widget_write_comboboxstring_options_canvaspanel"))
        {
            ComboBoxString->ClearOptions();
            ComboBoxString->AddOption(TEXT("Easy"));
            ComboBoxString->AddOption(TEXT("Normal"));
            ComboBoxString->AddOption(TEXT("Hard"));
        }
        else if (Spec.Name == TEXT("widget_write_comboboxstring_selected_option_canvaspanel"))
        {
            FStrProperty* SelectedOptionProperty = FindFProperty<FStrProperty>(UComboBoxString::StaticClass(), TEXT("SelectedOption"));
            if (!SelectedOptionProperty)
            {
                OutError = TEXT("SelectedOption property not found on UComboBoxString");
                return false;
            }
            SelectedOptionProperty->SetPropertyValue_InContainer(ComboBoxString, TEXT("Normal"));
        }
        else
        {
            FBoolProperty* IsFocusableProperty = FindFProperty<FBoolProperty>(UComboBoxString::StaticClass(), TEXT("bIsFocusable"));
            if (!IsFocusableProperty)
            {
                OutError = TEXT("bIsFocusable property not found on UComboBoxString");
                return false;
            }
            IsFocusableProperty->SetPropertyValue_InContainer(ComboBoxString, false);
        }
        ComboBoxString->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_sizebox_width_canvaspanel")
        || Spec.Name == TEXT("widget_write_sizebox_height_canvaspanel")
        || Spec.Name == TEXT("widget_write_sizebox_min_desired_width_canvaspanel")
        || Spec.Name == TEXT("widget_write_sizebox_min_desired_height_canvaspanel")
        || Spec.Name == TEXT("widget_write_sizebox_max_desired_width_canvaspanel")
        || Spec.Name == TEXT("widget_write_sizebox_max_desired_height_canvaspanel")
        || Spec.Name == TEXT("widget_write_sizebox_min_aspect_ratio_canvaspanel")
        || Spec.Name == TEXT("widget_write_sizebox_max_aspect_ratio_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/SizeBox_1"), OutError);
        USizeBox* SizeBox = Cast<USizeBox>(TargetWidget);
        if (!SizeBox)
        {
            OutError = TEXT("Resolved widget is not a SizeBox for CanvasPanel_1/SizeBox_1");
            return false;
        }

        WidgetBlueprint->Modify();
        SizeBox->Modify();
        if (Spec.Name == TEXT("widget_write_sizebox_width_canvaspanel"))
        {
            SizeBox->SetWidthOverride(320.0f);
        }
        else if (Spec.Name == TEXT("widget_write_sizebox_height_canvaspanel"))
        {
            SizeBox->SetHeightOverride(72.0f);
        }
        else if (Spec.Name == TEXT("widget_write_sizebox_min_desired_width_canvaspanel"))
        {
            SizeBox->SetMinDesiredWidth(160.0f);
        }
        else if (Spec.Name == TEXT("widget_write_sizebox_min_desired_height_canvaspanel"))
        {
            SizeBox->SetMinDesiredHeight(48.0f);
        }
        else if (Spec.Name == TEXT("widget_write_sizebox_max_desired_width_canvaspanel"))
        {
            SizeBox->SetMaxDesiredWidth(640.0f);
        }
        else if (Spec.Name == TEXT("widget_write_sizebox_max_desired_height_canvaspanel"))
        {
            SizeBox->SetMaxDesiredHeight(240.0f);
        }
        else if (Spec.Name == TEXT("widget_write_sizebox_min_aspect_ratio_canvaspanel"))
        {
            SizeBox->SetMinAspectRatio(1.25f);
        }
        else
        {
            SizeBox->SetMaxAspectRatio(2.0f);
        }
        SizeBox->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_scrollbox_orientation_canvaspanel")
        || Spec.Name == TEXT("widget_write_scrollbox_scrollbar_visibility_canvaspanel")
        || Spec.Name == TEXT("widget_write_scrollbox_consume_mouse_wheel_canvaspanel")
        || Spec.Name == TEXT("widget_write_scrollbox_is_focusable_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/ScrollBox_1"), OutError);
        UScrollBox* ScrollBox = Cast<UScrollBox>(TargetWidget);
        if (!ScrollBox)
        {
            OutError = TEXT("Resolved widget is not a ScrollBox for CanvasPanel_1/ScrollBox_1");
            return false;
        }

        WidgetBlueprint->Modify();
        ScrollBox->Modify();
        if (Spec.Name == TEXT("widget_write_scrollbox_orientation_canvaspanel"))
        {
            ScrollBox->SetOrientation(EOrientation::Orient_Horizontal);
        }
        else if (Spec.Name == TEXT("widget_write_scrollbox_scrollbar_visibility_canvaspanel"))
        {
            ScrollBox->SetScrollBarVisibility(ESlateVisibility::Collapsed);
        }
        else if (Spec.Name == TEXT("widget_write_scrollbox_consume_mouse_wheel_canvaspanel"))
        {
            ScrollBox->SetConsumeMouseWheel(EConsumeMouseWheel::Always);
        }
        else
        {
            ScrollBox->SetIsFocusable(true);
        }
        ScrollBox->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_scalebox_stretch_canvaspanel")
        || Spec.Name == TEXT("widget_write_scalebox_stretch_direction_canvaspanel")
        || Spec.Name == TEXT("widget_write_scalebox_user_specified_scale_canvaspanel")
        || Spec.Name == TEXT("widget_write_scalebox_ignore_inherited_scale_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/ScaleBox_1"), OutError);
        UScaleBox* ScaleBox = Cast<UScaleBox>(TargetWidget);
        if (!ScaleBox)
        {
            OutError = TEXT("Resolved widget is not a ScaleBox for CanvasPanel_1/ScaleBox_1");
            return false;
        }

        WidgetBlueprint->Modify();
        ScaleBox->Modify();
        if (Spec.Name == TEXT("widget_write_scalebox_stretch_canvaspanel"))
        {
            ScaleBox->SetStretch(EStretch::ScaleToFit);
        }
        else if (Spec.Name == TEXT("widget_write_scalebox_stretch_direction_canvaspanel"))
        {
            ScaleBox->SetStretchDirection(EStretchDirection::DownOnly);
        }
        else if (Spec.Name == TEXT("widget_write_scalebox_user_specified_scale_canvaspanel"))
        {
            ScaleBox->SetUserSpecifiedScale(1.25f);
        }
        else
        {
            ScaleBox->SetIgnoreInheritedScale(true);
        }
        ScaleBox->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_wrapbox_wrap_size_canvaspanel")
        || Spec.Name == TEXT("widget_write_wrapbox_explicit_wrap_size_canvaspanel")
        || Spec.Name == TEXT("widget_write_wrapbox_inner_slot_padding_canvaspanel")
        || Spec.Name == TEXT("widget_write_wrapbox_orientation_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/WrapBox_1"), OutError);
        UWrapBox* WrapBox = Cast<UWrapBox>(TargetWidget);
        if (!WrapBox)
        {
            OutError = TEXT("Resolved widget is not a WrapBox for CanvasPanel_1/WrapBox_1");
            return false;
        }

        WidgetBlueprint->Modify();
        WrapBox->Modify();
        if (Spec.Name == TEXT("widget_write_wrapbox_wrap_size_canvaspanel"))
        {
            WrapBox->SetWrapSize(480.0f);
        }
        else if (Spec.Name == TEXT("widget_write_wrapbox_explicit_wrap_size_canvaspanel"))
        {
            WrapBox->SetExplicitWrapSize(true);
        }
        else if (Spec.Name == TEXT("widget_write_wrapbox_inner_slot_padding_canvaspanel"))
        {
            WrapBox->SetInnerSlotPadding(FVector2D(8.0f, 12.0f));
        }
        else
        {
            WrapBox->SetOrientation(EOrientation::Orient_Vertical);
        }
        WrapBox->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_widgetswitcher_active_widget_index_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/WidgetSwitcher_1"), OutError);
        UWidgetSwitcher* WidgetSwitcher = Cast<UWidgetSwitcher>(TargetWidget);
        if (!WidgetSwitcher)
        {
            OutError = TEXT("Resolved widget is not a WidgetSwitcher for CanvasPanel_1/WidgetSwitcher_1");
            return false;
        }

        WidgetBlueprint->Modify();
        WidgetSwitcher->Modify();
        WidgetSwitcher->SetActiveWidgetIndex(2);
        WidgetSwitcher->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_listview_entry_widget_class_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/ListView_1"), OutError);
        UListView* ListView = Cast<UListView>(TargetWidget);
        if (!ListView)
        {
            OutError = TEXT("Resolved widget is not a ListView for CanvasPanel_1/ListView_1");
            return false;
        }

        UClass* EntryWidgetClass = LoadClass<UUserWidget>(nullptr, TEXT("/Game/WBP/WBP_ListEntry_Text.WBP_ListEntry_Text_C"));
        if (!EntryWidgetClass)
        {
            OutError = TEXT("Failed to load /Game/WBP/WBP_ListEntry_Text.WBP_ListEntry_Text_C");
            return false;
        }
        FClassProperty* EntryWidgetClassProp = FindFProperty<FClassProperty>(UListViewBase::StaticClass(), TEXT("EntryWidgetClass"));
        if (!EntryWidgetClassProp)
        {
            OutError = TEXT("EntryWidgetClass property not found on UListViewBase");
            return false;
        }

        WidgetBlueprint->Modify();
        ListView->Modify();
        EntryWidgetClassProp->SetPropertyValue_InContainer(ListView, EntryWidgetClass);
        ListView->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_listview_orientation_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_selection_mode_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_consume_mouse_wheel_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_is_focusable_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_return_focus_to_selection_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_clear_scroll_velocity_on_selection_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_scroll_into_view_alignment_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_wheel_scroll_multiplier_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_enable_scroll_animation_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_allow_overscroll_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_enable_right_click_scrolling_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_enable_touch_scrolling_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_is_pointer_scrolling_enabled_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_is_gamepad_scrolling_enabled_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_horizontal_entry_spacing_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_vertical_entry_spacing_canvaspanel")
        || Spec.Name == TEXT("widget_write_listview_scrollbar_padding_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/ListView_1"), OutError);
        UListView* ListView = Cast<UListView>(TargetWidget);
        if (!ListView)
        {
            OutError = TEXT("Resolved widget is not a ListView for CanvasPanel_1/ListView_1");
            return false;
        }

        WidgetBlueprint->Modify();
        ListView->Modify();
        if (Spec.Name == TEXT("widget_write_listview_orientation_canvaspanel"))
        {
            FByteProperty* OrientationProp = FindFProperty<FByteProperty>(UListView::StaticClass(), TEXT("Orientation"));
            if (!OrientationProp)
            {
                OutError = TEXT("Orientation property not found on UListView");
                return false;
            }
            uint8* OrientationPtr = OrientationProp->ContainerPtrToValuePtr<uint8>(ListView);
            *OrientationPtr = static_cast<uint8>(EOrientation::Orient_Horizontal);
        }
        else if (Spec.Name == TEXT("widget_write_listview_selection_mode_canvaspanel"))
        {
            ListView->SetSelectionMode(ESelectionMode::Multi);
        }
        else if (Spec.Name == TEXT("widget_write_listview_consume_mouse_wheel_canvaspanel"))
        {
            FProperty* ConsumeProp = FindFProperty<FProperty>(UListView::StaticClass(), TEXT("ConsumeMouseWheel"));
            if (FEnumProperty* EnumProp = CastField<FEnumProperty>(ConsumeProp))
            {
                EnumProp->GetUnderlyingProperty()->SetIntPropertyValue(EnumProp->ContainerPtrToValuePtr<void>(ListView), static_cast<int64>(EConsumeMouseWheel::Never));
            }
            else if (FByteProperty* ByteProp = CastField<FByteProperty>(ConsumeProp))
            {
                ByteProp->SetPropertyValue(ByteProp->ContainerPtrToValuePtr<uint8>(ListView), static_cast<uint8>(EConsumeMouseWheel::Never));
            }
            else
            {
                OutError = TEXT("ConsumeMouseWheel property not found on UListView");
                return false;
            }
        }
        else if (Spec.Name == TEXT("widget_write_listview_is_focusable_canvaspanel"))
        {
            FBoolProperty* BoolProp = FindFProperty<FBoolProperty>(UListView::StaticClass(), TEXT("bIsFocusable"));
            if (!BoolProp)
            {
                OutError = TEXT("bIsFocusable property not found on UListView");
                return false;
            }
            BoolProp->SetPropertyValue(BoolProp->ContainerPtrToValuePtr<bool>(ListView), false);
        }
        else if (Spec.Name == TEXT("widget_write_listview_return_focus_to_selection_canvaspanel"))
        {
            FBoolProperty* BoolProp = FindFProperty<FBoolProperty>(UListView::StaticClass(), TEXT("bReturnFocusToSelection"));
            if (!BoolProp)
            {
                OutError = TEXT("bReturnFocusToSelection property not found on UListView");
                return false;
            }
            BoolProp->SetPropertyValue(BoolProp->ContainerPtrToValuePtr<bool>(ListView), true);
        }
        else if (Spec.Name == TEXT("widget_write_listview_clear_scroll_velocity_on_selection_canvaspanel"))
        {
            FBoolProperty* BoolProp = FindFProperty<FBoolProperty>(UListView::StaticClass(), TEXT("bClearScrollVelocityOnSelection"));
            if (!BoolProp)
            {
                OutError = TEXT("bClearScrollVelocityOnSelection property not found on UListView");
                return false;
            }
            BoolProp->SetPropertyValue(BoolProp->ContainerPtrToValuePtr<bool>(ListView), false);
        }
        else if (Spec.Name == TEXT("widget_write_listview_scroll_into_view_alignment_canvaspanel"))
        {
            FProperty* AlignmentProp = FindFProperty<FProperty>(UListView::StaticClass(), TEXT("ScrollIntoViewAlignment"));
            if (FEnumProperty* EnumProp = CastField<FEnumProperty>(AlignmentProp))
            {
                EnumProp->GetUnderlyingProperty()->SetIntPropertyValue(EnumProp->ContainerPtrToValuePtr<void>(ListView), static_cast<int64>(EScrollIntoViewAlignment::BottomOrRight));
            }
            else if (FByteProperty* ByteProp = CastField<FByteProperty>(AlignmentProp))
            {
                ByteProp->SetPropertyValue(ByteProp->ContainerPtrToValuePtr<uint8>(ListView), static_cast<uint8>(EScrollIntoViewAlignment::BottomOrRight));
            }
            else
            {
                OutError = TEXT("ScrollIntoViewAlignment property not found on UListView");
                return false;
            }
        }
        else if (Spec.Name == TEXT("widget_write_listview_wheel_scroll_multiplier_canvaspanel"))
        {
            ListView->SetWheelScrollMultiplier(2.5f);
        }
        else if (Spec.Name == TEXT("widget_write_listview_enable_scroll_animation_canvaspanel"))
        {
            FBoolProperty* ScrollAnimationProp = FindFProperty<FBoolProperty>(UListViewBase::StaticClass(), TEXT("bEnableScrollAnimation"));
            if (!ScrollAnimationProp)
            {
                OutError = TEXT("bEnableScrollAnimation property not found on UListViewBase");
                return false;
            }
            ScrollAnimationProp->SetPropertyValue(ScrollAnimationProp->ContainerPtrToValuePtr<bool>(ListView), true);
        }
        else if (Spec.Name == TEXT("widget_write_listview_allow_overscroll_canvaspanel"))
        {
            FBoolProperty* BoolProp = FindFProperty<FBoolProperty>(UListViewBase::StaticClass(), TEXT("AllowOverscroll"));
            if (!BoolProp)
            {
                OutError = TEXT("AllowOverscroll property not found on UListViewBase");
                return false;
            }
            BoolProp->SetPropertyValue(BoolProp->ContainerPtrToValuePtr<bool>(ListView), false);
        }
        else if (Spec.Name == TEXT("widget_write_listview_enable_right_click_scrolling_canvaspanel"))
        {
            FBoolProperty* BoolProp = FindFProperty<FBoolProperty>(UListViewBase::StaticClass(), TEXT("bEnableRightClickScrolling"));
            if (!BoolProp)
            {
                OutError = TEXT("bEnableRightClickScrolling property not found on UListViewBase");
                return false;
            }
            BoolProp->SetPropertyValue(BoolProp->ContainerPtrToValuePtr<bool>(ListView), false);
        }
        else if (Spec.Name == TEXT("widget_write_listview_enable_touch_scrolling_canvaspanel"))
        {
            FBoolProperty* BoolProp = FindFProperty<FBoolProperty>(UListViewBase::StaticClass(), TEXT("bEnableTouchScrolling"));
            if (!BoolProp)
            {
                OutError = TEXT("bEnableTouchScrolling property not found on UListViewBase");
                return false;
            }
            BoolProp->SetPropertyValue(BoolProp->ContainerPtrToValuePtr<bool>(ListView), false);
        }
        else if (Spec.Name == TEXT("widget_write_listview_is_pointer_scrolling_enabled_canvaspanel"))
        {
            ListView->SetIsPointerScrollingEnabled(false);
        }
        else if (Spec.Name == TEXT("widget_write_listview_is_gamepad_scrolling_enabled_canvaspanel"))
        {
            ListView->SetIsGamepadScrollingEnabled(false);
        }
        else if (Spec.Name == TEXT("widget_write_listview_horizontal_entry_spacing_canvaspanel"))
        {
            ListView->SetHorizontalEntrySpacing(12.0f);
        }
        else if (Spec.Name == TEXT("widget_write_listview_vertical_entry_spacing_canvaspanel"))
        {
            ListView->SetVerticalEntrySpacing(6.0f);
        }
        else
        {
            ListView->SetScrollBarPadding(FMargin(1.0f, 2.0f, 3.0f, 4.0f));
        }
        ListView->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_tileview_entry_widget_class_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/TileView_1"), OutError);
        UTileView* TileView = Cast<UTileView>(TargetWidget);
        if (!TileView)
        {
            OutError = TEXT("Resolved widget is not a TileView for CanvasPanel_1/TileView_1");
            return false;
        }

        UClass* EntryWidgetClass = LoadClass<UUserWidget>(nullptr, TEXT("/Game/WBP/WBP_ListEntry_Text.WBP_ListEntry_Text_C"));
        if (!EntryWidgetClass)
        {
            OutError = TEXT("Failed to load /Game/WBP/WBP_ListEntry_Text.WBP_ListEntry_Text_C");
            return false;
        }
        FClassProperty* EntryWidgetClassProp = FindFProperty<FClassProperty>(UListViewBase::StaticClass(), TEXT("EntryWidgetClass"));
        if (!EntryWidgetClassProp)
        {
            OutError = TEXT("EntryWidgetClass property not found on UListViewBase");
            return false;
        }

        WidgetBlueprint->Modify();
        TileView->Modify();
        EntryWidgetClassProp->SetPropertyValue_InContainer(TileView, EntryWidgetClass);
        TileView->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_tileview_entry_width_canvaspanel")
        || Spec.Name == TEXT("widget_write_tileview_entry_height_canvaspanel")
        || Spec.Name == TEXT("widget_write_tileview_scrollbar_disabled_visibility_canvaspanel")
        || Spec.Name == TEXT("widget_write_tileview_entry_size_includes_entry_spacing_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/TileView_1"), OutError);
        UTileView* TileView = Cast<UTileView>(TargetWidget);
        if (!TileView)
        {
            OutError = TEXT("Resolved widget is not a TileView for CanvasPanel_1/TileView_1");
            return false;
        }

        WidgetBlueprint->Modify();
        TileView->Modify();
        if (Spec.Name == TEXT("widget_write_tileview_entry_width_canvaspanel"))
        {
            TileView->SetEntryWidth(180.0f);
        }
        else if (Spec.Name == TEXT("widget_write_tileview_entry_height_canvaspanel"))
        {
            TileView->SetEntryHeight(96.0f);
        }
        else if (Spec.Name == TEXT("widget_write_tileview_scrollbar_disabled_visibility_canvaspanel"))
        {
            FProperty* VisibilityProp = FindFProperty<FProperty>(UTileView::StaticClass(), TEXT("ScrollbarDisabledVisibility"));
            if (FEnumProperty* EnumProp = CastField<FEnumProperty>(VisibilityProp))
            {
                EnumProp->GetUnderlyingProperty()->SetIntPropertyValue(EnumProp->ContainerPtrToValuePtr<void>(TileView), static_cast<int64>(ESlateVisibility::Hidden));
            }
            else if (FByteProperty* ByteProp = CastField<FByteProperty>(VisibilityProp))
            {
                ByteProp->SetPropertyValue(ByteProp->ContainerPtrToValuePtr<uint8>(TileView), static_cast<uint8>(ESlateVisibility::Hidden));
            }
            else
            {
                OutError = TEXT("ScrollbarDisabledVisibility property not found on UTileView");
                return false;
            }
        }
        else
        {
            FBoolProperty* BoolProp = FindFProperty<FBoolProperty>(UTileView::StaticClass(), TEXT("bEntrySizeIncludesEntrySpacing"));
            if (!BoolProp)
            {
                OutError = TEXT("bEntrySizeIncludesEntrySpacing property not found on UTileView");
                return false;
            }
            BoolProp->SetPropertyValue(BoolProp->ContainerPtrToValuePtr<bool>(TileView), false);
        }
        TileView->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_treeview_entry_widget_class_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/TreeView_1"), OutError);
        UTreeView* TreeView = Cast<UTreeView>(TargetWidget);
        if (!TreeView)
        {
            OutError = TEXT("Resolved widget is not a TreeView for CanvasPanel_1/TreeView_1");
            return false;
        }

        UClass* EntryWidgetClass = LoadClass<UUserWidget>(nullptr, TEXT("/Game/WBP/WBP_ListEntry_Text.WBP_ListEntry_Text_C"));
        if (!EntryWidgetClass)
        {
            OutError = TEXT("Failed to load /Game/WBP/WBP_ListEntry_Text.WBP_ListEntry_Text_C");
            return false;
        }
        FClassProperty* EntryWidgetClassProp = FindFProperty<FClassProperty>(UListViewBase::StaticClass(), TEXT("EntryWidgetClass"));
        if (!EntryWidgetClassProp)
        {
            OutError = TEXT("EntryWidgetClass property not found on UListViewBase");
            return false;
        }

        WidgetBlueprint->Modify();
        TreeView->Modify();
        EntryWidgetClassProp->SetPropertyValue_InContainer(TreeView, EntryWidgetClass);
        TreeView->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_treeview_selection_mode_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/TreeView_1"), OutError);
        UTreeView* TreeView = Cast<UTreeView>(TargetWidget);
        if (!TreeView)
        {
            OutError = TEXT("Resolved widget is not a TreeView for CanvasPanel_1/TreeView_1");
            return false;
        }

        WidgetBlueprint->Modify();
        TreeView->Modify();
        TreeView->SetSelectionMode(ESelectionMode::Multi);
        TreeView->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_retainerbox_retain_render_canvaspanel")
        || Spec.Name == TEXT("widget_write_retainerbox_render_on_invalidation_canvaspanel")
        || Spec.Name == TEXT("widget_write_retainerbox_render_on_phase_canvaspanel")
        || Spec.Name == TEXT("widget_write_retainerbox_phase_canvaspanel")
        || Spec.Name == TEXT("widget_write_retainerbox_phase_count_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/RetainerBox_1"), OutError);
        URetainerBox* RetainerBox = Cast<URetainerBox>(TargetWidget);
        if (!RetainerBox)
        {
            OutError = TEXT("Resolved widget is not a RetainerBox for CanvasPanel_1/RetainerBox_1");
            return false;
        }

        WidgetBlueprint->Modify();
        RetainerBox->Modify();
        if (Spec.Name == TEXT("widget_write_retainerbox_retain_render_canvaspanel"))
        {
            RetainerBox->SetRetainRendering(true);
        }
        else if (Spec.Name == TEXT("widget_write_retainerbox_render_on_invalidation_canvaspanel"))
        {
            RetainerBox->RenderOnInvalidation = false;
        }
        else if (Spec.Name == TEXT("widget_write_retainerbox_render_on_phase_canvaspanel"))
        {
            RetainerBox->RenderOnPhase = true;
        }
        else if (Spec.Name == TEXT("widget_write_retainerbox_phase_canvaspanel"))
        {
            RetainerBox->Phase = 1;
        }
        else
        {
            RetainerBox->PhaseCount = 3;
        }
        RetainerBox->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_backgroundblur_strength_canvaspanel")
        || Spec.Name == TEXT("widget_write_backgroundblur_apply_alpha_to_blur_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/BackgroundBlur_1"), OutError);
        UBackgroundBlur* BackgroundBlur = Cast<UBackgroundBlur>(TargetWidget);
        if (!BackgroundBlur)
        {
            OutError = TEXT("Resolved widget is not a BackgroundBlur for CanvasPanel_1/BackgroundBlur_1");
            return false;
        }

        WidgetBlueprint->Modify();
        BackgroundBlur->Modify();
        if (Spec.Name == TEXT("widget_write_backgroundblur_strength_canvaspanel"))
        {
            BackgroundBlur->SetBlurStrength(16.0f);
        }
        else
        {
            BackgroundBlur->SetApplyAlphaToBlur(true);
        }
        BackgroundBlur->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_safezone_pad_left_canvaspanel")
        || Spec.Name == TEXT("widget_write_safezone_pad_right_canvaspanel")
        || Spec.Name == TEXT("widget_write_safezone_pad_top_canvaspanel")
        || Spec.Name == TEXT("widget_write_safezone_pad_bottom_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/SafeZone_1"), OutError);
        USafeZone* SafeZone = Cast<USafeZone>(TargetWidget);
        if (!SafeZone)
        {
            OutError = TEXT("Resolved widget is not a SafeZone for CanvasPanel_1/SafeZone_1");
            return false;
        }

        WidgetBlueprint->Modify();
        SafeZone->Modify();
        if (Spec.Name == TEXT("widget_write_safezone_pad_left_canvaspanel"))
        {
            SafeZone->PadLeft = false;
        }
        else if (Spec.Name == TEXT("widget_write_safezone_pad_right_canvaspanel"))
        {
            SafeZone->PadRight = true;
        }
        else if (Spec.Name == TEXT("widget_write_safezone_pad_top_canvaspanel"))
        {
            SafeZone->PadTop = false;
        }
        else
        {
            SafeZone->PadBottom = true;
        }
        SafeZone->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_invalidationbox_can_cache_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/InvalidationBox_1"), OutError);
        UInvalidationBox* InvalidationBox = Cast<UInvalidationBox>(TargetWidget);
        if (!InvalidationBox)
        {
            OutError = TEXT("Resolved widget is not an InvalidationBox for CanvasPanel_1/InvalidationBox_1");
            return false;
        }

        WidgetBlueprint->Modify();
        InvalidationBox->Modify();
        InvalidationBox->SetCanCache(true);
        InvalidationBox->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_uniformgridpanel_min_desired_slot_width_canvaspanel")
        || Spec.Name == TEXT("widget_write_uniformgridpanel_min_desired_slot_height_canvaspanel")
        || Spec.Name == TEXT("widget_write_uniformgridpanel_slot_padding_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/UniformGridPanel_1"), OutError);
        UUniformGridPanel* UniformGridPanel = Cast<UUniformGridPanel>(TargetWidget);
        if (!UniformGridPanel)
        {
            OutError = TEXT("Resolved widget is not a UniformGridPanel for CanvasPanel_1/UniformGridPanel_1");
            return false;
        }

        WidgetBlueprint->Modify();
        UniformGridPanel->Modify();
        if (Spec.Name == TEXT("widget_write_uniformgridpanel_min_desired_slot_width_canvaspanel"))
        {
            UniformGridPanel->SetMinDesiredSlotWidth(160.0f);
        }
        else if (Spec.Name == TEXT("widget_write_uniformgridpanel_min_desired_slot_height_canvaspanel"))
        {
            UniformGridPanel->SetMinDesiredSlotHeight(48.0f);
        }
        else
        {
            UniformGridPanel->SetSlotPadding(FMargin(4.0f, 6.0f, 8.0f, 10.0f));
        }
        UniformGridPanel->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_visibility_overlay_child"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("Overlay_116/TextBlock_36"), OutError);
        if (!TargetWidget)
        {
            return false;
        }

        TargetWidget->Modify();
        TargetWidget->SetVisibility(ESlateVisibility::Collapsed);
        TargetWidget->MarkPackageDirty();
        WidgetBlueprint->Modify();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_brush_image"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("Image_22"), OutError);
        UImage* ImageWidget = Cast<UImage>(TargetWidget);
        if (!ImageWidget)
        {
            OutError = TEXT("Resolved widget is not an Image for Image_22");
            return false;
        }

        UTexture2D* Texture = LoadObject<UTexture2D>(nullptr, TEXT("/Game/Effects/Textures/Decals/chippedcracks.chippedcracks"));
        if (!Texture)
        {
            OutError = TEXT("Failed to load texture /Game/Effects/Textures/Decals/chippedcracks.chippedcracks");
            return false;
        }

        ImageWidget->Modify();
        ImageWidget->SetBrushFromTexture(Texture, true);
        if (ImageWidget->Brush.GetResourceObject() != Texture)
        {
            FSlateBrush Brush = ImageWidget->Brush;
            Brush.SetResourceObject(Texture);
            Brush.ImageType = ESlateBrushImageType::FullColor;
            Brush.ImageSize = FVector2D(64.0f, 64.0f);
            ImageWidget->SetBrush(Brush);
        }
        ImageWidget->MarkPackageDirty();
        WidgetBlueprint->Modify();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_button_is_focusable_canvaspanel"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/Button_1"), OutError);
        UButton* ButtonWidget = Cast<UButton>(TargetWidget);
        if (!ButtonWidget)
        {
            OutError = TEXT("Resolved widget is not a Button for CanvasPanel_1/Button_1");
            return false;
        }

        WidgetBlueprint->Modify();
        ButtonWidget->Modify();

        FBoolProperty* IsFocusableProperty = FindFProperty<FBoolProperty>(UButton::StaticClass(), TEXT("IsFocusable"));
        if (!IsFocusableProperty)
        {
            OutError = TEXT("IsFocusable property not found on UButton");
            return false;
        }
        IsFocusableProperty->SetPropertyValue_InContainer(ButtonWidget, false);

        ButtonWidget->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_write_button_brush_normal")
        || Spec.Name == TEXT("widget_write_button_brush_tint")
        || Spec.Name == TEXT("widget_write_button_brush_image_size")
        || Spec.Name == TEXT("widget_write_button_brush_draw_as"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("Button_1"), OutError);
        UButton* ButtonWidget = Cast<UButton>(TargetWidget);
        if (!ButtonWidget)
        {
            OutError = TEXT("Resolved widget is not a Button for Button_1");
            return false;
        }

        WidgetBlueprint->Modify();
        ButtonWidget->Modify();

        FButtonStyle Style = ButtonWidget->WidgetStyle;
        FSlateBrush* NormalBrush = &Style.Normal;
        if (!NormalBrush)
        {
            OutError = TEXT("Button WidgetStyle.Normal brush is unavailable.");
            return false;
        }

        if (Spec.Name == TEXT("widget_write_button_brush_normal"))
        {
            UTexture2D* Texture = LoadObject<UTexture2D>(nullptr, TEXT("/Game/UI/Menu/Art/T_UI_Icon_SimpleArrow.T_UI_Icon_SimpleArrow"));
            if (!Texture)
            {
                OutError = TEXT("Failed to load texture /Game/UI/Menu/Art/T_UI_Icon_SimpleArrow.T_UI_Icon_SimpleArrow");
                return false;
            }
            NormalBrush->SetResourceObject(Texture);
            NormalBrush->ImageType = ESlateBrushImageType::FullColor;
            NormalBrush->ImageSize = FVector2D(512.0f, 512.0f);
        }
        else if (Spec.Name == TEXT("widget_write_button_brush_tint"))
        {
            NormalBrush->TintColor = FSlateColor(FLinearColor(0.25f, 0.4f, 0.9f, 0.8f));
        }
        else if (Spec.Name == TEXT("widget_write_button_brush_image_size"))
        {
            NormalBrush->ImageSize = FVector2D(96.0f, 48.0f);
        }
        else if (Spec.Name == TEXT("widget_write_button_brush_draw_as"))
        {
            NormalBrush->DrawAs = ESlateBrushDrawType::RoundedBox;
        }

        ButtonWidget->WidgetStyle = Style;
        ButtonWidget->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_add_image_canvaspanel"))
    {
        UWidget* ParentWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_22"), OutError);
        UCanvasPanel* CanvasPanel = Cast<UCanvasPanel>(ParentWidget);
        if (!CanvasPanel)
        {
            OutError = TEXT("Resolved widget is not a CanvasPanel for CanvasPanel_22");
            return false;
        }

        WidgetBlueprint->Modify();
        CanvasPanel->Modify();
        UImage* ImageWidget = WidgetBlueprint->WidgetTree->ConstructWidget<UImage>(UImage::StaticClass(), TEXT("Image_23"));
        if (!ImageWidget)
        {
            OutError = TEXT("ConstructWidget<UImage> returned null for widget_add_image_canvaspanel");
            return false;
        }
        if (!CanvasPanel->AddChildToCanvas(ImageWidget))
        {
            OutError = TEXT("AddChildToCanvas returned null for widget_add_image_canvaspanel");
            return false;
        }
        ImageWidget->Modify();
        ImageWidget->MarkPackageDirty();
        CanvasPanel->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_add_image_canvaspanel_nonempty"))
    {
        UWidget* ParentWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_22"), OutError);
        UCanvasPanel* CanvasPanel = Cast<UCanvasPanel>(ParentWidget);
        if (!CanvasPanel)
        {
            OutError = TEXT("Resolved widget is not a CanvasPanel for CanvasPanel_22");
            return false;
        }

        WidgetBlueprint->Modify();
        CanvasPanel->Modify();
        UImage* ImageWidget = WidgetBlueprint->WidgetTree->ConstructWidget<UImage>(UImage::StaticClass(), TEXT("Image_59"));
        if (!ImageWidget)
        {
            OutError = TEXT("ConstructWidget<UImage> returned null for widget_add_image_canvaspanel_nonempty");
            return false;
        }
        if (!CanvasPanel->AddChildToCanvas(ImageWidget))
        {
            OutError = TEXT("AddChildToCanvas returned null for widget_add_image_canvaspanel_nonempty");
            return false;
        }
        ImageWidget->Modify();
        ImageWidget->MarkPackageDirty();
        CanvasPanel->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_add_image_overlay"))
    {
        UWidget* ParentWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("Overlay_116"), OutError);
        UOverlay* Overlay = Cast<UOverlay>(ParentWidget);
        if (!Overlay)
        {
            OutError = TEXT("Resolved widget is not an Overlay for Overlay_116");
            return false;
        }

        WidgetBlueprint->Modify();
        Overlay->Modify();
        UImage* ImageWidget = WidgetBlueprint->WidgetTree->ConstructWidget<UImage>(UImage::StaticClass(), TEXT("Image_23"));
        if (!ImageWidget)
        {
            OutError = TEXT("ConstructWidget<UImage> returned null for widget_add_image_overlay");
            return false;
        }
        if (!Overlay->AddChildToOverlay(ImageWidget))
        {
            OutError = TEXT("AddChildToOverlay returned null for widget_add_image_overlay");
            return false;
        }
        ImageWidget->Modify();
        ImageWidget->MarkPackageDirty();
        Overlay->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_add_image_overlay_nonempty"))
    {
        UWidget* ParentWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("Overlay_116"), OutError);
        UOverlay* Overlay = Cast<UOverlay>(ParentWidget);
        if (!Overlay)
        {
            OutError = TEXT("Resolved widget is not an Overlay for Overlay_116");
            return false;
        }

        WidgetBlueprint->Modify();
        Overlay->Modify();
        UImage* ImageWidget = WidgetBlueprint->WidgetTree->ConstructWidget<UImage>(UImage::StaticClass(), TEXT("Image_71"));
        if (!ImageWidget)
        {
            OutError = TEXT("ConstructWidget<UImage> returned null for widget_add_image_overlay_nonempty");
            return false;
        }
        if (!Overlay->AddChildToOverlay(ImageWidget))
        {
            OutError = TEXT("AddChildToOverlay returned null for widget_add_image_overlay_nonempty");
            return false;
        }
        ImageWidget->Modify();
        ImageWidget->MarkPackageDirty();
        Overlay->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_add_image_nested_overlay"))
    {
        UWidget* ParentWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/Overlay_1"), OutError);
        UOverlay* Overlay = Cast<UOverlay>(ParentWidget);
        if (!Overlay)
        {
            OutError = TEXT("Resolved widget is not an Overlay for CanvasPanel_1/Overlay_1");
            return false;
        }

        WidgetBlueprint->Modify();
        Overlay->Modify();
        UImage* ImageWidget = WidgetBlueprint->WidgetTree->ConstructWidget<UImage>(UImage::StaticClass(), TEXT("Image_3"));
        if (!ImageWidget)
        {
            OutError = TEXT("ConstructWidget<UImage> returned null for widget_add_image_nested_overlay");
            return false;
        }
        if (!Overlay->AddChildToOverlay(ImageWidget))
        {
            OutError = TEXT("AddChildToOverlay returned null for widget_add_image_nested_overlay");
            return false;
        }
        ImageWidget->Modify();
        ImageWidget->MarkPackageDirty();
        Overlay->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_add_root_canvaspanel"))
    {
        if (!WidgetBlueprint->WidgetTree)
        {
            OutError = TEXT("WidgetTree is null for widget_add_root_canvaspanel");
            return false;
        }
        if (WidgetBlueprint->WidgetTree->RootWidget)
        {
            OutError = TEXT("WidgetTree already has a root widget for widget_add_root_canvaspanel");
            return false;
        }

        WidgetBlueprint->Modify();
        WidgetBlueprint->WidgetTree->Modify();
        UCanvasPanel* RootCanvasPanel = WidgetBlueprint->WidgetTree->ConstructWidget<UCanvasPanel>(UCanvasPanel::StaticClass(), TEXT("CanvasPanel_21"));
        if (!RootCanvasPanel)
        {
            OutError = TEXT("ConstructWidget<UCanvasPanel> returned null for widget_add_root_canvaspanel");
            return false;
        }
        WidgetBlueprint->WidgetTree->RootWidget = RootCanvasPanel;
        RootCanvasPanel->Modify();
        RootCanvasPanel->MarkPackageDirty();
        WidgetBlueprint->WidgetTree->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_add_root_verticalbox"))
    {
        if (!WidgetBlueprint->WidgetTree)
        {
            OutError = TEXT("WidgetTree is null for widget_add_root_verticalbox");
            return false;
        }
        if (WidgetBlueprint->WidgetTree->RootWidget)
        {
            OutError = TEXT("WidgetTree already has a root widget for widget_add_root_verticalbox");
            return false;
        }

        WidgetBlueprint->Modify();
        WidgetBlueprint->WidgetTree->Modify();
        UVerticalBox* RootVerticalBox = WidgetBlueprint->WidgetTree->ConstructWidget<UVerticalBox>(UVerticalBox::StaticClass(), TEXT("VerticalBox_21"));
        if (!RootVerticalBox)
        {
            OutError = TEXT("ConstructWidget<UVerticalBox> returned null for widget_add_root_verticalbox");
            return false;
        }
        WidgetBlueprint->WidgetTree->RootWidget = RootVerticalBox;
        RootVerticalBox->Modify();
        RootVerticalBox->MarkPackageDirty();
        WidgetBlueprint->WidgetTree->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_add_root_horizontalbox"))
    {
        if (!WidgetBlueprint->WidgetTree)
        {
            OutError = TEXT("WidgetTree is null for widget_add_root_horizontalbox");
            return false;
        }
        if (WidgetBlueprint->WidgetTree->RootWidget)
        {
            OutError = TEXT("WidgetTree already has a root widget for widget_add_root_horizontalbox");
            return false;
        }

        WidgetBlueprint->Modify();
        WidgetBlueprint->WidgetTree->Modify();
        UHorizontalBox* RootHorizontalBox = WidgetBlueprint->WidgetTree->ConstructWidget<UHorizontalBox>(UHorizontalBox::StaticClass(), TEXT("HorizontalBox_21"));
        if (!RootHorizontalBox)
        {
            OutError = TEXT("ConstructWidget<UHorizontalBox> returned null for widget_add_root_horizontalbox");
            return false;
        }
        WidgetBlueprint->WidgetTree->RootWidget = RootHorizontalBox;
        RootHorizontalBox->Modify();
        RootHorizontalBox->MarkPackageDirty();
        WidgetBlueprint->WidgetTree->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_add_button_canvaspanel"))
    {
        UWidget* ParentWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_22"), OutError);
        UCanvasPanel* CanvasPanel = Cast<UCanvasPanel>(ParentWidget);
        if (!CanvasPanel)
        {
            OutError = TEXT("Resolved widget is not a CanvasPanel for CanvasPanel_22");
            return false;
        }

        WidgetBlueprint->Modify();
        CanvasPanel->Modify();
        UButton* ButtonWidget = WidgetBlueprint->WidgetTree->ConstructWidget<UButton>(UButton::StaticClass(), TEXT("Button_1"));
        if (!ButtonWidget)
        {
            OutError = TEXT("ConstructWidget<UButton> returned null for widget_add_button_canvaspanel");
            return false;
        }
        if (!CanvasPanel->AddChildToCanvas(ButtonWidget))
        {
            OutError = TEXT("AddChildToCanvas returned null for widget_add_button_canvaspanel");
            return false;
        }
        if (!WidgetBlueprint->WidgetVariableNameToGuidMap.Contains(ButtonWidget->GetFName()))
        {
            WidgetBlueprint->OnVariableAdded(ButtonWidget->GetFName());
        }
        ButtonWidget->Modify();
        ButtonWidget->MarkPackageDirty();
        CanvasPanel->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_add_richtextblock_canvaspanel"))
    {
        UWidget* ParentWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_22"), OutError);
        UCanvasPanel* CanvasPanel = Cast<UCanvasPanel>(ParentWidget);
        if (!CanvasPanel)
        {
            OutError = TEXT("Resolved widget is not a CanvasPanel for CanvasPanel_22");
            return false;
        }

        WidgetBlueprint->Modify();
        CanvasPanel->Modify();
        URichTextBlock* RichTextBlockWidget = WidgetBlueprint->WidgetTree->ConstructWidget<URichTextBlock>(URichTextBlock::StaticClass(), TEXT("RichTextBlock_23"));
        if (!RichTextBlockWidget)
        {
            OutError = TEXT("ConstructWidget<URichTextBlock> returned null for widget_add_richtextblock_canvaspanel");
            return false;
        }
        if (!CanvasPanel->AddChildToCanvas(RichTextBlockWidget))
        {
            OutError = TEXT("AddChildToCanvas returned null for widget_add_richtextblock_canvaspanel");
            return false;
        }
        if (!WidgetBlueprint->WidgetVariableNameToGuidMap.Contains(RichTextBlockWidget->GetFName()))
        {
            WidgetBlueprint->OnVariableAdded(RichTextBlockWidget->GetFName());
        }
        RichTextBlockWidget->Modify();
        RichTextBlockWidget->MarkPackageDirty();
        CanvasPanel->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_add_progressbar_canvaspanel"))
    {
        return AddBareCanvasPanelLeafWidget<UProgressBar>(
            WidgetBlueprint,
            TEXT("CanvasPanel_22"),
            TEXT("ProgressBar_23"),
            TEXT("widget_add_progressbar_canvaspanel"),
            OutError);
    }

    if (Spec.Name == TEXT("widget_add_slider_canvaspanel"))
    {
        return AddBareCanvasPanelLeafWidget<USlider>(
            WidgetBlueprint,
            TEXT("CanvasPanel_22"),
            TEXT("Slider_23"),
            TEXT("widget_add_slider_canvaspanel"),
            OutError);
    }

    if (Spec.Name == TEXT("widget_add_spacer_canvaspanel"))
    {
        return AddBareCanvasPanelLeafWidget<USpacer>(
            WidgetBlueprint,
            TEXT("CanvasPanel_22"),
            TEXT("Spacer_23"),
            TEXT("widget_add_spacer_canvaspanel"),
            OutError);
    }

    if (Spec.Name == TEXT("widget_add_scrollbar_canvaspanel"))
    {
        return AddBareCanvasPanelLeafWidget<UScrollBar>(
            WidgetBlueprint,
            TEXT("CanvasPanel_22"),
            TEXT("ScrollBar_23"),
            TEXT("widget_add_scrollbar_canvaspanel"),
            OutError);
    }

    if (Spec.Name == TEXT("widget_add_editabletext_canvaspanel"))
    {
        return AddBareCanvasPanelLeafWidget<UEditableText>(
            WidgetBlueprint,
            TEXT("CanvasPanel_22"),
            TEXT("EditableText_23"),
            TEXT("widget_add_editabletext_canvaspanel"),
            OutError);
    }

    if (Spec.Name == TEXT("widget_add_editabletextbox_canvaspanel"))
    {
        return AddBareCanvasPanelLeafWidget<UEditableTextBox>(
            WidgetBlueprint,
            TEXT("CanvasPanel_22"),
            TEXT("EditableTextBox_23"),
            TEXT("widget_add_editabletextbox_canvaspanel"),
            OutError);
    }

    if (Spec.Name == TEXT("widget_add_multilineeditabletextbox_canvaspanel"))
    {
        return AddBareCanvasPanelLeafWidget<UMultiLineEditableTextBox>(
            WidgetBlueprint,
            TEXT("CanvasPanel_22"),
            TEXT("MultiLineEditableTextBox_23"),
            TEXT("widget_add_multilineeditabletextbox_canvaspanel"),
            OutError);
    }

    if (Spec.Name == TEXT("widget_add_spinbox_canvaspanel"))
    {
        return AddBareCanvasPanelLeafWidget<USpinBox>(
            WidgetBlueprint,
            TEXT("CanvasPanel_22"),
            TEXT("SpinBox_23"),
            TEXT("widget_add_spinbox_canvaspanel"),
            OutError);
    }

    if (Spec.Name == TEXT("widget_add_comboboxstring_canvaspanel"))
    {
        return AddBareCanvasPanelLeafWidget<UComboBoxString>(
            WidgetBlueprint,
            TEXT("CanvasPanel_22"),
            TEXT("ComboBoxString_23"),
            TEXT("widget_add_comboboxstring_canvaspanel"),
            OutError);
    }

    if (Spec.Name == TEXT("widget_add_listview_canvaspanel"))
    {
        return AddBareCanvasPanelLeafWidget<UListView>(
            WidgetBlueprint,
            TEXT("CanvasPanel_22"),
            TEXT("ListView_23"),
            TEXT("widget_add_listview_canvaspanel"),
            OutError);
    }

    if (Spec.Name == TEXT("widget_add_tileview_canvaspanel"))
    {
        return AddBareCanvasPanelLeafWidget<UTileView>(
            WidgetBlueprint,
            TEXT("CanvasPanel_22"),
            TEXT("TileView_23"),
            TEXT("widget_add_tileview_canvaspanel"),
            OutError);
    }

    if (Spec.Name == TEXT("widget_add_treeview_canvaspanel"))
    {
        return AddBareCanvasPanelLeafWidget<UTreeView>(
            WidgetBlueprint,
            TEXT("CanvasPanel_22"),
            TEXT("TreeView_23"),
            TEXT("widget_add_treeview_canvaspanel"),
            OutError);
    }

    if (Spec.Name == TEXT("widget_add_namedslot_canvaspanel"))
    {
        return AddBareCanvasPanelLeafWidget<UNamedSlot>(
            WidgetBlueprint,
            TEXT("CanvasPanel_22"),
            TEXT("NamedSlot_23"),
            TEXT("widget_add_namedslot_canvaspanel"),
            OutError);
    }

    if (Spec.Name == TEXT("widget_add_textblock_namedslot_canvaspanel"))
    {
        UWidget* ParentWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_22/NamedSlot_1"), OutError);
        UNamedSlot* NamedSlot = Cast<UNamedSlot>(ParentWidget);
        if (!NamedSlot)
        {
            OutError = TEXT("Resolved widget is not a NamedSlot for CanvasPanel_22/NamedSlot_1");
            return false;
        }

        WidgetBlueprint->Modify();
        NamedSlot->Modify();
        UTextBlock* ChildWidget = WidgetBlueprint->WidgetTree->ConstructWidget<UTextBlock>(UTextBlock::StaticClass(), TEXT("TextBlock_1"));
        if (!ChildWidget)
        {
            OutError = TEXT("ConstructWidget<UTextBlock> returned null for widget_add_textblock_namedslot_canvaspanel");
            return false;
        }
        if (!NamedSlot->SetContent(ChildWidget))
        {
            OutError = TEXT("NamedSlot->SetContent returned false for widget_add_textblock_namedslot_canvaspanel");
            return false;
        }
        if (!WidgetBlueprint->WidgetVariableNameToGuidMap.Contains(ChildWidget->GetFName()))
        {
            WidgetBlueprint->OnVariableAdded(ChildWidget->GetFName());
        }
        ChildWidget->Modify();
        ChildWidget->MarkPackageDirty();
        NamedSlot->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = TEXT("GeneratedClass is null after compiling widget_add_textblock_namedslot_canvaspanel");
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_add_userwidget_canvaspanel"))
    {
        UCanvasPanel* CanvasPanel = Cast<UCanvasPanel>(ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_22"), OutError));
        if (!CanvasPanel)
        {
            OutError = TEXT("CanvasPanel_22 is not a CanvasPanel for widget_add_userwidget_canvaspanel");
            return false;
        }

        UClass* ChildClass = LoadClass<UUserWidget>(nullptr, TEXT("/Game/BPXFixtures/Parse/WBP_TextBlock.WBP_TextBlock_C"));
        if (!ChildClass)
        {
            OutError = TEXT("LoadClass failed for /Game/BPXFixtures/Parse/WBP_TextBlock.WBP_TextBlock_C");
            return false;
        }

        WidgetBlueprint->Modify();
        CanvasPanel->Modify();
        UUserWidget* ChildWidget = WidgetBlueprint->WidgetTree->ConstructWidget<UUserWidget>(ChildClass, TEXT("WBP_TextBlock_1"));
        if (!ChildWidget)
        {
            OutError = TEXT("ConstructWidget<UUserWidget> returned null for widget_add_userwidget_canvaspanel");
            return false;
        }
        if (!CanvasPanel->AddChildToCanvas(ChildWidget))
        {
            OutError = TEXT("AddChildToCanvas returned null for widget_add_userwidget_canvaspanel");
            return false;
        }
        if (!WidgetBlueprint->WidgetVariableNameToGuidMap.Contains(ChildWidget->GetFName()))
        {
            WidgetBlueprint->OnVariableAdded(ChildWidget->GetFName());
        }
        ChildWidget->Modify();
        ChildWidget->MarkPackageDirty();
        CanvasPanel->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = TEXT("GeneratedClass is null after compiling widget_add_userwidget_canvaspanel");
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_add_image_border_canvaspanel"))
    {
        UWidget* ParentWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/Border_1"), OutError);
        UBorder* Border = Cast<UBorder>(ParentWidget);
        if (!Border)
        {
            OutError = TEXT("Resolved widget is not a Border for CanvasPanel_1/Border_1");
            return false;
        }

        WidgetBlueprint->Modify();
        Border->Modify();
        UImage* ImageWidget = WidgetBlueprint->WidgetTree->ConstructWidget<UImage>(UImage::StaticClass(), TEXT("Image_1"));
        if (!ImageWidget)
        {
            OutError = TEXT("ConstructWidget<UImage> returned null for widget_add_image_border_canvaspanel");
            return false;
        }
        if (!Border->SetContent(ImageWidget))
        {
            OutError = TEXT("SetContent returned false for widget_add_image_border_canvaspanel");
            return false;
        }
        if (!WidgetBlueprint->WidgetVariableNameToGuidMap.Contains(ImageWidget->GetFName()))
        {
            WidgetBlueprint->OnVariableAdded(ImageWidget->GetFName());
        }
        ImageWidget->Modify();
        ImageWidget->MarkPackageDirty();
        Border->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_add_image_sizebox_canvaspanel"))
    {
        UWidget* ParentWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/SizeBox_1"), OutError);
        USizeBox* SizeBox = Cast<USizeBox>(ParentWidget);
        if (!SizeBox)
        {
            OutError = TEXT("Resolved widget is not a SizeBox for CanvasPanel_1/SizeBox_1");
            return false;
        }

        WidgetBlueprint->Modify();
        SizeBox->Modify();
        UImage* ImageWidget = WidgetBlueprint->WidgetTree->ConstructWidget<UImage>(UImage::StaticClass(), TEXT("Image_1"));
        if (!ImageWidget)
        {
            OutError = TEXT("ConstructWidget<UImage> returned null for widget_add_image_sizebox_canvaspanel");
            return false;
        }
        if (!SizeBox->SetContent(ImageWidget))
        {
            OutError = TEXT("SetContent returned false for widget_add_image_sizebox_canvaspanel");
            return false;
        }
        if (!WidgetBlueprint->WidgetVariableNameToGuidMap.Contains(ImageWidget->GetFName()))
        {
            WidgetBlueprint->OnVariableAdded(ImageWidget->GetFName());
        }
        ImageWidget->Modify();
        ImageWidget->MarkPackageDirty();
        SizeBox->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_add_image_horizontalbox_canvaspanel"))
    {
        UWidget* ParentWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_1/HorizontalBox_1"), OutError);
        UHorizontalBox* HorizontalBox = Cast<UHorizontalBox>(ParentWidget);
        if (!HorizontalBox)
        {
            OutError = TEXT("Resolved widget is not a HorizontalBox for CanvasPanel_1/HorizontalBox_1");
            return false;
        }

        WidgetBlueprint->Modify();
        HorizontalBox->Modify();
        UImage* ImageWidget = WidgetBlueprint->WidgetTree->ConstructWidget<UImage>(UImage::StaticClass(), TEXT("Image_1"));
        if (!ImageWidget)
        {
            OutError = TEXT("ConstructWidget<UImage> returned null for widget_add_image_horizontalbox_canvaspanel");
            return false;
        }
        if (!HorizontalBox->AddChildToHorizontalBox(ImageWidget))
        {
            OutError = TEXT("AddChildToHorizontalBox returned null for widget_add_image_horizontalbox_canvaspanel");
            return false;
        }
        if (!WidgetBlueprint->WidgetVariableNameToGuidMap.Contains(ImageWidget->GetFName()))
        {
            WidgetBlueprint->OnVariableAdded(ImageWidget->GetFName());
        }
        ImageWidget->Modify();
        ImageWidget->MarkPackageDirty();
        HorizontalBox->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    if (Spec.Name == TEXT("widget_remove_textblock_canvaspanel"))
    {
        return RemoveWidgetBlueprintLeaf(WidgetBlueprint, TEXT("CanvasPanel_22/TextBlock_31"), OutError);
    }

    if (Spec.Name == TEXT("widget_remove_richtextblock_canvaspanel"))
    {
        return RemoveWidgetBlueprintLeaf(WidgetBlueprint, TEXT("CanvasPanel_22/RichTextBlock_31"), OutError);
    }

    if (Spec.Name == TEXT("widget_remove_textblock_overlay"))
    {
        return RemoveWidgetBlueprintLeaf(WidgetBlueprint, TEXT("Overlay_116/TextBlock_36"), OutError);
    }

    if (Spec.Name == TEXT("widget_remove_image_canvaspanel"))
    {
        return RemoveWidgetBlueprintLeaf(WidgetBlueprint, TEXT("CanvasPanel_1/Image_1"), OutError);
    }
    if (Spec.Name == TEXT("widget_remove_image_overlay"))
    {
        return RemoveWidgetBlueprintLeaf(WidgetBlueprint, TEXT("Overlay_1/Image_1"), OutError);
    }
    if (Spec.Name == TEXT("widget_remove_image_verticalbox"))
    {
        return RemoveWidgetBlueprintLeaf(WidgetBlueprint, TEXT("VerticalBox_1/Image_1"), OutError);
    }
    if (Spec.Name == TEXT("widget_remove_image_horizontalbox"))
    {
        return RemoveWidgetBlueprintLeaf(WidgetBlueprint, TEXT("HorizontalBox_1/Image_1"), OutError);
    }
    if (Spec.Name == TEXT("widget_remove_button_canvaspanel"))
    {
        return RemoveWidgetBlueprintLeaf(WidgetBlueprint, TEXT("CanvasPanel_1/Button_1"), OutError);
    }
    if (Spec.Name == TEXT("widget_remove_border_canvaspanel"))
    {
        return RemoveWidgetBlueprintLeaf(WidgetBlueprint, TEXT("CanvasPanel_1/Border_1"), OutError);
    }
    if (Spec.Name == TEXT("widget_remove_sizebox_canvaspanel"))
    {
        return RemoveWidgetBlueprintLeaf(WidgetBlueprint, TEXT("CanvasPanel_1/SizeBox_1"), OutError);
    }
    if (Spec.Name == TEXT("widget_remove_gridpanel_canvaspanel"))
    {
        return RemoveWidgetBlueprintLeaf(WidgetBlueprint, TEXT("CanvasPanel_1/GridPanel_1"), OutError);
    }
    if (Spec.Name == TEXT("widget_remove_backgroundblur_canvaspanel"))
    {
        return RemoveWidgetBlueprintLeaf(WidgetBlueprint, TEXT("CanvasPanel_1/BackgroundBlur_1"), OutError);
    }
    if (Spec.Name == TEXT("widget_remove_invalidationbox_canvaspanel"))
    {
        return RemoveWidgetBlueprintLeaf(WidgetBlueprint, TEXT("CanvasPanel_1/InvalidationBox_1"), OutError);
    }
    if (Spec.Name == TEXT("widget_remove_retainerbox_canvaspanel"))
    {
        return RemoveWidgetBlueprintLeaf(WidgetBlueprint, TEXT("CanvasPanel_1/RetainerBox_1"), OutError);
    }
    if (Spec.Name == TEXT("widget_remove_safezone_canvaspanel"))
    {
        return RemoveWidgetBlueprintLeaf(WidgetBlueprint, TEXT("CanvasPanel_1/SafeZone_1"), OutError);
    }
    if (Spec.Name == TEXT("widget_remove_scalebox_canvaspanel"))
    {
        return RemoveWidgetBlueprintLeaf(WidgetBlueprint, TEXT("CanvasPanel_1/ScaleBox_1"), OutError);
    }
    if (Spec.Name == TEXT("widget_remove_scrollbox_canvaspanel"))
    {
        return RemoveWidgetBlueprintLeaf(WidgetBlueprint, TEXT("CanvasPanel_1/ScrollBox_1"), OutError);
    }
    if (Spec.Name == TEXT("widget_remove_widgetswitcher_canvaspanel"))
    {
        return RemoveWidgetBlueprintLeaf(WidgetBlueprint, TEXT("CanvasPanel_1/WidgetSwitcher_1"), OutError);
    }
    if (Spec.Name == TEXT("widget_remove_wrapbox_canvaspanel"))
    {
        return RemoveWidgetBlueprintLeaf(WidgetBlueprint, TEXT("CanvasPanel_1/WrapBox_1"), OutError);
    }

    if (Spec.Name == TEXT("widget_write_layout_canvaspanelslot"))
    {
        UWidget* TargetWidget = ResolveWidgetByPath(WidgetBlueprint->WidgetTree, TEXT("CanvasPanel_22/Image_29"), OutError);
        if (!TargetWidget)
        {
            return false;
        }

        UCanvasPanelSlot* CanvasSlot = Cast<UCanvasPanelSlot>(TargetWidget->Slot);
        if (!CanvasSlot)
        {
            OutError = TEXT("Resolved widget slot is not a CanvasPanelSlot for CanvasPanel_22/Image_29");
            return false;
        }

        WidgetBlueprint->Modify();
        TargetWidget->Modify();
        CanvasSlot->Modify();
        CanvasSlot->SetAnchors(FAnchors(0.5f, 0.5f, 0.5f, 0.5f));
        CanvasSlot->SetAlignment(FVector2D(0.5f, 0.5f));
        CanvasSlot->SetPosition(FVector2D(0.0f, 0.0f));
        CanvasSlot->SetSize(FVector2D(200.0f, 60.0f));
        CanvasSlot->MarkPackageDirty();
        TargetWidget->MarkPackageDirty();
        WidgetBlueprint->MarkPackageDirty();
        FKismetEditorUtilities::CompileBlueprint(WidgetBlueprint);
        if (!WidgetBlueprint->GeneratedClass)
        {
            OutError = FString::Printf(TEXT("GeneratedClass is null after compiling %s"), *Spec.Name);
            return false;
        }
        return true;
    }

    OutError = FString::Printf(TEXT("Unsupported widget blueprint operation: %s"), *Spec.Name);
    return false;
}

UBlueprint* CreateActorBlueprintAsset(const FString& PackageName, const FString& AssetName, UClass* ParentClass, const FString& FixtureValueDefault)
{
    UPackage* Package = CreatePackage(*PackageName);
    if (!Package)
    {
        return nullptr;
    }

    UBlueprint* Blueprint = FKismetEditorUtilities::CreateBlueprint(
        ParentClass ? ParentClass : AActor::StaticClass(),
        Package,
        FName(*AssetName),
        BPTYPE_Normal,
        UBlueprint::StaticClass(),
        UBlueprintGeneratedClass::StaticClass(),
        NAME_None
    );

    if (!Blueprint)
    {
        return nullptr;
    }

    if (!FixtureValueDefault.IsEmpty())
    {
        FEdGraphPinType IntType;
        IntType.PinCategory = UEdGraphSchema_K2::PC_Int;
        FBlueprintEditorUtils::AddMemberVariable(Blueprint, TEXT("FixtureValue"), IntType, FixtureValueDefault);
    }

    FAssetRegistryModule::AssetCreated(Blueprint);
    Blueprint->MarkPackageDirty();
    FKismetEditorUtilities::CompileBlueprint(Blueprint);
    return Blueprint;
}

void AddBlueprintMemberVariable(UBlueprint* Blueprint, const FName& VariableName, const FEdGraphPinType& PinType, const FString& DefaultValue)
{
    if (!Blueprint)
    {
        return;
    }
    FBlueprintEditorUtils::AddMemberVariable(Blueprint, VariableName, PinType, DefaultValue);
}

void PopulateBlueprintParseFixture(UBlueprint* Blueprint, const FString& FixtureKey)
{
    if (!Blueprint)
    {
        return;
    }

    auto AddBool = [Blueprint](const TCHAR* Name, const TCHAR* DefaultValue) {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_Boolean;
        AddBlueprintMemberVariable(Blueprint, FName(Name), PinType, FString(DefaultValue));
    };
    auto AddInt = [Blueprint](const TCHAR* Name, const TCHAR* DefaultValue) {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_Int;
        AddBlueprintMemberVariable(Blueprint, FName(Name), PinType, FString(DefaultValue));
    };
    auto AddInt64 = [Blueprint](const TCHAR* Name, const TCHAR* DefaultValue) {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_Int64;
        AddBlueprintMemberVariable(Blueprint, FName(Name), PinType, FString(DefaultValue));
    };
    auto AddFloat = [Blueprint](const TCHAR* Name, const TCHAR* DefaultValue) {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_Real;
        PinType.PinSubCategory = UEdGraphSchema_K2::PC_Float;
        AddBlueprintMemberVariable(Blueprint, FName(Name), PinType, FString(DefaultValue));
    };
    auto AddDouble = [Blueprint](const TCHAR* Name, const TCHAR* DefaultValue) {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_Real;
        PinType.PinSubCategory = UEdGraphSchema_K2::PC_Double;
        AddBlueprintMemberVariable(Blueprint, FName(Name), PinType, FString(DefaultValue));
    };
    auto AddString = [Blueprint](const TCHAR* Name, const TCHAR* DefaultValue) {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_String;
        AddBlueprintMemberVariable(Blueprint, FName(Name), PinType, FString(DefaultValue));
    };
    auto AddName = [Blueprint](const TCHAR* Name, const TCHAR* DefaultValue) {
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_Name;
        AddBlueprintMemberVariable(Blueprint, FName(Name), PinType, FString(DefaultValue));
    };
    auto AddStruct = [Blueprint](const TCHAR* Name, UScriptStruct* StructType, const TCHAR* DefaultValue) {
        if (!StructType)
        {
            return;
        }
        FEdGraphPinType PinType;
        PinType.PinCategory = UEdGraphSchema_K2::PC_Struct;
        PinType.PinSubCategoryObject = StructType;
        AddBlueprintMemberVariable(Blueprint, FName(Name), PinType, FString(DefaultValue));
    };

    if (FixtureKey == TEXT("BP_SimpleVars"))
    {
        AddBool(TEXT("MyBool"), TEXT("true"));
        AddInt(TEXT("MyInt"), TEXT("42"));
        AddFloat(TEXT("MyFloat"), TEXT("3.14"));
        AddString(TEXT("MyString"), TEXT("Hello"));
        AddName(TEXT("MyName"), TEXT("OldName"));
        AddStruct(TEXT("MyVector"), TBaseStructure<FVector>::Get(), TEXT("(X=1.0,Y=2.0,Z=3.0)"));
        AddStruct(TEXT("MyRotator"), TBaseStructure<FRotator>::Get(), TEXT("(Pitch=45.0,Yaw=90.0,Roll=180.0)"));
    }
    else if (FixtureKey == TEXT("BP_AllScalarTypes"))
    {
        AddBool(TEXT("VBool"), TEXT("true"));
        AddInt(TEXT("VInt"), TEXT("7"));
        AddInt64(TEXT("VInt64"), TEXT("9223372036854775807"));
        AddFloat(TEXT("VFloat"), TEXT("1.25"));
        AddDouble(TEXT("VDouble"), TEXT("2.718281828"));
        AddString(TEXT("VString"), TEXT("scalar"));
        AddName(TEXT("VName"), TEXT("ScalarName"));
    }
    else if (FixtureKey == TEXT("BP_MathTypes"))
    {
        AddStruct(TEXT("VVector"), TBaseStructure<FVector>::Get(), TEXT("(X=10.0,Y=20.0,Z=30.0)"));
        AddStruct(TEXT("VRotator"), TBaseStructure<FRotator>::Get(), TEXT("(Pitch=10.0,Yaw=20.0,Roll=30.0)"));
        AddStruct(TEXT("VColor"), TBaseStructure<FLinearColor>::Get(), TEXT("(R=1.0,G=0.0,B=0.0,A=1.0)"));
        AddStruct(TEXT("VTransform"), TBaseStructure<FTransform>::Get(), TEXT("(Rotation=(X=0.0,Y=0.0,Z=0.0,W=1.0),Translation=(X=1.0,Y=2.0,Z=3.0),Scale3D=(X=1.0,Y=1.0,Z=1.0))"));
    }
    else if (FixtureKey == TEXT("BP_Unicode"))
    {
        AddInt(TEXT("体力"), TEXT("100"));
        AddInt(TEXT("攻撃力"), TEXT("25"));
        AddString(TEXT("説明"), TEXT("テストデータ"));
    }
    else if (FixtureKey == TEXT("BP_LargeArray"))
    {
        AddInt(TEXT("BigArraySeed"), TEXT("1000"));
    }

    Blueprint->MarkPackageDirty();
    FKismetEditorUtilities::CompileBlueprint(Blueprint);

    if (FixtureKey == TEXT("BP_Empty_StringTableRef") && Blueprint->SimpleConstructionScript)
    {
        const TArray<USCS_Node*>& RootNodes = Blueprint->SimpleConstructionScript->GetRootNodes();
        if (RootNodes.Num() > 0 && RootNodes[0])
        {
            if (FTextProperty* CategoryProp = FindFProperty<FTextProperty>(USCS_Node::StaticClass(), TEXT("CategoryName")))
            {
                if (FText* CategoryValue = CategoryProp->ContainerPtrToValuePtr<FText>(RootNodes[0]))
                {
                    *CategoryValue = FText::FromStringTable(FName(TEXT("SimpleConstructionScript")), TEXT("ST_ENTRY_KEY_000001"));
                    RootNodes[0]->MarkPackageDirty();
                }
            }
            Blueprint->MarkPackageDirty();
        }
    }
}

bool WriteOperationSidecars(const FString& OperationDir, const FOperationFixtureSpec& Spec, bool bForce, bool bIncludeSavedHashIgnore, FString& OutError)
{
    IPlatformFile& PlatformFile = FPlatformFileManager::Get().GetPlatformFile();
    PlatformFile.CreateDirectoryTree(*OperationDir);
    const FString BeforeOutputName = IsLevelOperation(Spec) ? TEXT("before.umap") : TEXT("before.uasset");
    const FString AfterOutputName = IsLevelOperation(Spec) ? TEXT("after.umap") : TEXT("after.uasset");

    const FString JsonPath = FPaths::Combine(OperationDir, TEXT("operation.json"));
    const FString ReadmePath = FPaths::Combine(OperationDir, TEXT("README.md"));

    if (!bForce)
    {
        if (PlatformFile.FileExists(*JsonPath) || PlatformFile.FileExists(*ReadmePath))
        {
            OutError = FString::Printf(TEXT("Operation sidecar already exists for %s"), *Spec.Name);
            return false;
        }
    }

    TSharedPtr<FJsonObject> ArgsObject;
    const TSharedRef<TJsonReader<TCHAR>> ArgsReader = TJsonReaderFactory<TCHAR>::Create(Spec.ArgsJson);
    if (!FJsonSerializer::Deserialize(ArgsReader, ArgsObject) || !ArgsObject.IsValid())
    {
        OutError = FString::Printf(TEXT("Failed to parse operation args JSON for %s: %s"), *Spec.Name, *Spec.ArgsJson);
        return false;
    }

    const TSharedRef<FJsonObject> RootObject = MakeShared<FJsonObject>();
    RootObject->SetStringField(TEXT("command"), Spec.Command);
    RootObject->SetObjectField(TEXT("args"), ArgsObject);
    RootObject->SetStringField(TEXT("ue_procedure"), Spec.UEProcedure);
    RootObject->SetStringField(TEXT("expect"), Spec.Expect);
    if (!Spec.ErrorContains.IsEmpty())
    {
        RootObject->SetStringField(TEXT("error_contains"), Spec.ErrorContains);
    }
    RootObject->SetStringField(TEXT("notes"), Spec.Notes);
    if (!Spec.IgnorePackageSectionsJson.IsEmpty())
    {
        TArray<TSharedPtr<FJsonValue>> IgnorePackageSections;
        const TSharedRef<TJsonReader<TCHAR>> IgnorePackageSectionsReader = TJsonReaderFactory<TCHAR>::Create(Spec.IgnorePackageSectionsJson);
        if (!FJsonSerializer::Deserialize(IgnorePackageSectionsReader, IgnorePackageSections))
        {
            OutError = FString::Printf(TEXT("Failed to parse ignore package sections JSON for %s: %s"), *Spec.Name, *Spec.IgnorePackageSectionsJson);
            return false;
        }
        RootObject->SetArrayField(TEXT("ignore_package_sections"), IgnorePackageSections);
    }

    if (bIncludeSavedHashIgnore)
    {
        TArray<TSharedPtr<FJsonValue>> IgnoreOffsets;
        const TSharedRef<FJsonObject> IgnoreObject = MakeShared<FJsonObject>();
        IgnoreObject->SetNumberField(TEXT("offset"), 24);
        IgnoreObject->SetNumberField(TEXT("length"), 20);
        IgnoreObject->SetStringField(TEXT("reason"), TEXT("ue-save-nondeterministic"));
        IgnoreOffsets.Add(MakeShared<FJsonValueObject>(IgnoreObject));
        RootObject->SetArrayField(TEXT("ignore_offsets"), IgnoreOffsets);
    }

    FString JsonText;
    const TSharedRef<TJsonWriter<TCHAR, TPrettyJsonPrintPolicy<TCHAR>>> JsonWriter = TJsonWriterFactory<TCHAR, TPrettyJsonPrintPolicy<TCHAR>>::Create(&JsonText);
    if (!FJsonSerializer::Serialize(RootObject, JsonWriter))
    {
        OutError = FString::Printf(TEXT("Failed to serialize operation.json payload for %s"), *Spec.Name);
        return false;
    }
    JsonText += TEXT("\n");

    const FString ReadmeText = FString::Printf(
        TEXT("# %s\n\nThis operation fixture pair was generated by `BPXGenerateFixtures` commandlet.\n\n- command: `%s`\n- expect: `%s`\n- notes: %s\n- output: `%s`, `%s`, `operation.json`\n"),
        *Spec.Name,
        *Spec.Command,
        *Spec.Expect,
        *Spec.Notes,
        *BeforeOutputName,
        *AfterOutputName
    );

    if (!FFileHelper::SaveStringToFile(JsonText, *JsonPath, FFileHelper::EEncodingOptions::ForceUTF8WithoutBOM))
    {
        OutError = FString::Printf(TEXT("Failed to write operation.json: %s"), *JsonPath);
        return false;
    }

    if (!FFileHelper::SaveStringToFile(ReadmeText, *ReadmePath, FFileHelper::EEncodingOptions::ForceUTF8WithoutBOM))
    {
        OutError = FString::Printf(TEXT("Failed to write README.md: %s"), *ReadmePath);
        return false;
    }

    return true;
}

bool ShouldRunScope1(const TSet<FString>& ScopeSet)
{
    return ScopeSet.Num() == 0
        || ScopeSet.Contains(TEXT("1"))
        || ScopeSet.Contains(TEXT("all"));
}

bool ShouldRunScope2(const TSet<FString>& ScopeSet)
{
    return ScopeSet.Num() == 0
        || ScopeSet.Contains(TEXT("2"))
        || ScopeSet.Contains(TEXT("all"));
}

bool IsIncluded(const FString& Name, const TSet<FString>& IncludeSet)
{
    if (IncludeSet.Num() == 0)
    {
        return true;
    }

    const FString Normalized = NormalizeToken(Name);
    return IncludeSet.Contains(Normalized);
}

bool CheckCollisionIfAny(const FString& FilePath, bool bForce, TArray<FString>& OutConflicts)
{
    if (bForce)
    {
        return true;
    }

    if (FPaths::FileExists(FilePath))
    {
        OutConflicts.Add(FilePath);
        return false;
    }

    return true;
}

bool DeleteFileIfExists(const FString& FilePath, FString& OutError)
{
    IPlatformFile& PlatformFile = FPlatformFileManager::Get().GetPlatformFile();
    if (!PlatformFile.FileExists(*FilePath))
    {
        return true;
    }

    if (!PlatformFile.DeleteFile(*FilePath))
    {
        OutError = FString::Printf(TEXT("Failed to delete existing generated source file: %s"), *FilePath);
        return false;
    }

    return true;
}

}

UBPXGenerateFixturesCommandlet::UBPXGenerateFixturesCommandlet()
{
    IsClient = false;
    IsEditor = true;
    IsServer = false;
    LogToConsole = true;
    UseCommandletResultAsExitCode = true;
}

int32 UBPXGenerateFixturesCommandlet::Main(const FString& Params)
{
    TArray<FString> Tokens;
    TArray<FString> Switches;
    TMap<FString, FString> NamedParams;
    ParseCommandLine(*Params, Tokens, Switches, NamedParams);

    const FString* BpxRepoRoot = NamedParams.Find(TEXT("BpxRepoRoot"));
    if (!BpxRepoRoot || BpxRepoRoot->IsEmpty())
    {
        UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Missing required parameter: -BpxRepoRoot=<path>"));
        return 2;
    }

    if (!ValidateWindowsOrUncPath(*BpxRepoRoot))
    {
        UE_LOG(LogBPXFixtureGenerator, Error, TEXT("BpxRepoRoot must be a Windows path (e.g. G:\\...) or UNC path."));
        return 2;
    }

    const FString ScopeCsv = NamedParams.Contains(TEXT("Scope")) ? NamedParams[TEXT("Scope")] : TEXT("1,2");
    const FString IncludeCsv = NamedParams.Contains(TEXT("Include")) ? NamedParams[TEXT("Include")] : TEXT("");

    const TSet<FString> ScopeSet = ParseCsvSet(ScopeCsv);
    const TSet<FString> IncludeSet = ParseCsvSet(IncludeCsv);
    const bool bForce = Switches.Contains(TEXT("Force"));

    const bool bRunScope1 = ShouldRunScope1(ScopeSet);
    const bool bRunScope2 = ShouldRunScope2(ScopeSet);

    if (!bRunScope1 && !bRunScope2)
    {
        UE_LOG(LogBPXFixtureGenerator, Error, TEXT("No valid scope selected. Scope must include 1 or 2."));
        return 2;
    }

    const FEngineVersion CurrentVersion = FEngineVersion::Current();
    const FString EngineTag = FString::Printf(TEXT("ue%d.%d"), CurrentVersion.GetMajor(), CurrentVersion.GetMinor());
    const FString CanonicalGoldenRoot = FPaths::Combine(*BpxRepoRoot, TEXT("testdata"), TEXT("golden"), EngineTag);

    FString GoldenRoot;
    const FString* GoldenRootParam = NamedParams.Find(TEXT("GoldenRoot"));
    if (GoldenRootParam && !GoldenRootParam->IsEmpty())
    {
        if (!ValidateWindowsOrUncPath(*GoldenRootParam))
        {
            UE_LOG(LogBPXFixtureGenerator, Error, TEXT("GoldenRoot must be a Windows path (e.g. G:\\...) or UNC path."));
            return 2;
        }
        GoldenRoot = *GoldenRootParam;
    }
    else
    {
        GoldenRoot = CanonicalGoldenRoot;
    }

    const FString ParseDir = FPaths::Combine(GoldenRoot, TEXT("parse"));
    const FString OpsDir = FPaths::Combine(GoldenRoot, TEXT("operations"));

    IPlatformFile& PlatformFile = FPlatformFileManager::Get().GetPlatformFile();
    PlatformFile.CreateDirectoryTree(*ParseDir);
    PlatformFile.CreateDirectoryTree(*OpsDir);

    TArray<FParseFixtureSpec> ParseSpecs;
    TArray<FOperationFixtureSpec> OperationSpecs;

    if (bRunScope1)
    {
        for (const FParseFixtureSpec& Spec : BuildParseSpecs())
        {
            if (IsIncluded(Spec.Key, IncludeSet) || IsIncluded(Spec.FileName, IncludeSet))
            {
                ParseSpecs.Add(Spec);
            }
        }
    }

    if (bRunScope2)
    {
        for (const FOperationFixtureSpec& Spec : BuildOperationSpecs())
        {
            if (IsIncluded(Spec.Name, IncludeSet) && !IsNotYetGeneratedOperation(Spec))
            {
                OperationSpecs.Add(Spec);
            }
        }
    }

    if (ParseSpecs.Num() == 0 && OperationSpecs.Num() == 0)
    {
        UE_LOG(LogBPXFixtureGenerator, Warning, TEXT("No fixtures selected after scope/include filtering."));
        return 0;
    }

    TArray<FString> Conflicts;
    for (const FParseFixtureSpec& Spec : ParseSpecs)
    {
        const FString OutputPath = FPaths::Combine(ParseDir, Spec.FileName);
        CheckCollisionIfAny(OutputPath, bForce, Conflicts);
    }

    for (const FOperationFixtureSpec& Spec : OperationSpecs)
    {
        const FString OpDir = FPaths::Combine(OpsDir, Spec.Name);
        CheckCollisionIfAny(FPaths::Combine(OpDir, TEXT("before.uasset")), bForce, Conflicts);
        CheckCollisionIfAny(FPaths::Combine(OpDir, TEXT("before.umap")), bForce, Conflicts);
        CheckCollisionIfAny(FPaths::Combine(OpDir, TEXT("after.uasset")), bForce, Conflicts);
        CheckCollisionIfAny(FPaths::Combine(OpDir, TEXT("after.umap")), bForce, Conflicts);
        CheckCollisionIfAny(FPaths::Combine(OpDir, TEXT("operation.json")), bForce, Conflicts);
        CheckCollisionIfAny(FPaths::Combine(OpDir, TEXT("README.md")), bForce, Conflicts);
    }

    if (Conflicts.Num() > 0)
    {
        UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Detected existing output files. Re-run with -Force to overwrite."));
        for (const FString& Conflict : Conflicts)
        {
            UE_LOG(LogBPXFixtureGenerator, Error, TEXT("  %s"), *Conflict);
        }
        return 4;
    }

    TMap<FString, UClass*> BlueprintClasses;
    TMap<FString, FString> GeneratedParseSourceByKey;
    int32 ParseGeneratedCount = 0;
    int32 OperationGeneratedCount = 0;

    for (const FParseFixtureSpec& Spec : ParseSpecs)
    {
        const bool bIsMap = Spec.Kind == EParseFixtureKind::Level;
        const FString PackageName = FString::Printf(TEXT("/Game/BPXFixtures/Parse/%s"), *Spec.Key);
        const FString AssetName = Spec.Key;
        const FString SourceFilename = FPackageName::LongPackageNameToFilename(
            PackageName,
            bIsMap ? FPackageName::GetMapPackageExtension() : FPackageName::GetAssetPackageExtension()
        );

        FString ErrorText;
        if (!DeleteFileIfExists(SourceFilename, ErrorText))
        {
            UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
            return 5;
        }

        UPackage* Package = CreatePackage(*PackageName);
        if (!Package)
        {
            UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create package: %s"), *PackageName);
            return 5;
        }

        UObject* AssetObject = nullptr;

        switch (Spec.Kind)
        {
        case EParseFixtureKind::Blueprint:
        {
            UClass* ParentClass = AActor::StaticClass();
            if (!Spec.ParentKey.IsEmpty())
            {
                const FString ParentLookup = NormalizeToken(Spec.ParentKey);
                UClass* const* FoundParent = BlueprintClasses.Find(ParentLookup);
                if (FoundParent && *FoundParent)
                {
                    ParentClass = *FoundParent;
                }
                else
                {
                    UE_LOG(LogBPXFixtureGenerator, Warning, TEXT("Missing parent class for %s; fallback to AActor"), *Spec.Key);
                }
            }

            UBlueprint* Blueprint = CreateActorBlueprintAsset(PackageName, AssetName, ParentClass, TEXT(""));
            if (!Blueprint)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create blueprint fixture: %s"), *Spec.Key);
                return 5;
            }

            PopulateBlueprintParseFixture(Blueprint, Spec.Key);
            AssetObject = Blueprint;
            if (Blueprint->GeneratedClass)
            {
                BlueprintClasses.Add(NormalizeToken(Spec.Key), Blueprint->GeneratedClass);
            }
            break;
        }
        case EParseFixtureKind::WidgetBlueprint:
        {
            UWidgetBlueprint* WidgetBlueprint = DuplicateWidgetBlueprintAsset(Spec.Key, PackageName, AssetName, ErrorText);
            if (!WidgetBlueprint)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 5;
            }

            AssetObject = WidgetBlueprint;
            break;
        }
        case EParseFixtureKind::DataTable:
        {
            UDataTable* DataTable = NewObject<UDataTable>(Package, FName(*AssetName), RF_Public | RF_Standalone);
            if (!DataTable)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create data table fixture: %s"), *Spec.Key);
                return 5;
            }
            DataTable->RowStruct = FTableRowBase::StaticStruct();
            const FTableRowBase EmptyRow;
            DataTable->AddRow(TEXT("Row_A"), EmptyRow);
            DataTable->AddRow(TEXT("Row_B"), EmptyRow);
            DataTable->AddRow(TEXT("Row_C"), EmptyRow);
            FAssetRegistryModule::AssetCreated(DataTable);
            DataTable->MarkPackageDirty();
            AssetObject = DataTable;
            break;
        }
        case EParseFixtureKind::UserEnum:
        {
            UEnum* EnumAsset = FEnumEditorUtils::CreateUserDefinedEnum(Package, FName(*AssetName), RF_Public | RF_Standalone);
            if (!EnumAsset)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create enum fixture: %s"), *Spec.Key);
                return 5;
            }
            FAssetRegistryModule::AssetCreated(EnumAsset);
            EnumAsset->MarkPackageDirty();
            AssetObject = EnumAsset;
            break;
        }
        case EParseFixtureKind::UserStruct:
        {
            UUserDefinedStruct* StructAsset = FStructureEditorUtils::CreateUserDefinedStruct(Package, FName(*AssetName), RF_Public | RF_Standalone);
            if (!StructAsset)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create struct fixture: %s"), *Spec.Key);
                return 5;
            }
            FAssetRegistryModule::AssetCreated(StructAsset);
            StructAsset->MarkPackageDirty();
            AssetObject = StructAsset;
            break;
        }
        case EParseFixtureKind::StringTable:
        {
            UStringTable* StringTable = NewObject<UStringTable>(Package, FName(*AssetName), RF_Public | RF_Standalone);
            if (!StringTable)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create string table fixture: %s"), *Spec.Key);
                return 5;
            }
            PopulateStringTableFixture(StringTable);

            FAssetRegistryModule::AssetCreated(StringTable);
            StringTable->MarkPackageDirty();
            AssetObject = StringTable;
            break;
        }
        case EParseFixtureKind::MaterialInstance:
        {
            UMaterialInstanceConstant* MaterialInstance = NewObject<UMaterialInstanceConstant>(Package, FName(*AssetName), RF_Public | RF_Standalone);
            if (!MaterialInstance)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create material instance fixture: %s"), *Spec.Key);
                return 5;
            }

            MaterialInstance->SetParentEditorOnly(UMaterial::GetDefaultMaterial(MD_Surface));
            FAssetRegistryModule::AssetCreated(MaterialInstance);
            MaterialInstance->MarkPackageDirty();
            AssetObject = MaterialInstance;
            break;
        }
        case EParseFixtureKind::Level:
        {
            UWorldFactory* WorldFactory = NewObject<UWorldFactory>();
            if (!WorldFactory)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create world factory for fixture: %s"), *Spec.Key);
                return 5;
            }

            WorldFactory->WorldType = EWorldType::Editor;
            UObject* WorldObject = WorldFactory->FactoryCreateNew(
                UWorld::StaticClass(),
                Package,
                FName(*AssetName),
                RF_Public | RF_Standalone,
                nullptr,
                GWarn
            );
            if (!WorldObject)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create level fixture: %s"), *Spec.Key);
                return 5;
            }

            UWorld* World = Cast<UWorld>(WorldObject);
            if (!World)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Created level object is not UWorld: %s"), *Spec.Key);
                return 5;
            }

            FAssetRegistryModule::AssetCreated(World);
            World->MarkPackageDirty();
            AssetObject = World;
            break;
        }
        }

        if (!SavePackageToDisk(Package, AssetObject, SourceFilename, ErrorText))
        {
            UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
            return 5;
        }
        GeneratedParseSourceByKey.Add(NormalizeToken(Spec.Key), SourceFilename);

        const FString OutputFilename = FPaths::Combine(ParseDir, Spec.FileName);
        if (!CopyFileToOutput(SourceFilename, OutputFilename, bForce, ErrorText))
        {
            UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
            return 6;
        }

        if (Spec.Kind == EParseFixtureKind::Level)
        {
            if (UWorld* GeneratedWorld = Cast<UWorld>(AssetObject))
            {
                if (GEngine)
                {
                    GEngine->DestroyWorldContext(GeneratedWorld);
                }
                GeneratedWorld->DestroyWorld(false);
            }
        }

        ++ParseGeneratedCount;
    }

    for (const FOperationFixtureSpec& Spec : OperationSpecs)
    {
        const bool bUseSinglePackageMutation = IsSinglePackageOperation(Spec);
        const bool bUseParseBlueprintMutation = IsParseBlueprintOperation(Spec);
        const bool bUseWidgetBlueprintMutation = IsWidgetBlueprintOperation(Spec);
        const bool bUseDataTableMutation = IsDataTableOperation(Spec);
        const bool bUseDataTableUpdateMutation = IsDataTableUpdateOperation(Spec);
        const bool bUseCompositeDataTableRejectMutation = IsCompositeDataTableRejectOperation(Spec);
        const bool bUseStringTableMutation = IsStringTableOperation(Spec);
        const bool bUseLevelMutation = IsLevelOperation(Spec);
        const bool bUseNameMutation = IsNameOperation(Spec);
        const FString OperationPackageExtension = bUseLevelMutation ? FPackageName::GetMapPackageExtension() : FPackageName::GetAssetPackageExtension();
        const FString BeforeOutputName = bUseLevelMutation ? TEXT("before.umap") : TEXT("before.uasset");
        const FString AfterOutputName = bUseLevelMutation ? TEXT("after.umap") : TEXT("after.uasset");
        const FString BeforePackageName = FString::Printf(TEXT("/Game/BPXFixtures/Operations/%s/Before"), *Spec.Name);
        const FString AfterPackageName = FString::Printf(TEXT("/Game/BPXFixtures/Operations/%s/After"), *Spec.Name);
        const FString BeforeSource = FPackageName::LongPackageNameToFilename(BeforePackageName, OperationPackageExtension);
        const FString AfterSource = FPackageName::LongPackageNameToFilename(AfterPackageName, OperationPackageExtension);
        const FString FixturePackageName = FString::Printf(TEXT("/Game/BPXFixtures/Operations/%s/Fixture"), *Spec.Name);
        const FString FixtureSource = FPackageName::LongPackageNameToFilename(FixturePackageName, OperationPackageExtension);
        const FString CompositeParentPackageName = FString::Printf(TEXT("/Game/BPXFixtures/Operations/%s/FixtureParent"), *Spec.Name);
        const FString CompositeParentSource = FPackageName::LongPackageNameToFilename(CompositeParentPackageName, FPackageName::GetAssetPackageExtension());
        const FString OperationDir = FPaths::Combine(OpsDir, Spec.Name);
        const FString BeforeOutput = FPaths::Combine(OperationDir, BeforeOutputName);
        const FString AfterOutput = FPaths::Combine(OperationDir, AfterOutputName);
        FString ErrorText;

        const FString WriteRoundtripParseKey = ParseFixtureKeyForWriteRoundtripOperation(Spec);
        if (!WriteRoundtripParseKey.IsEmpty())
        {
            const FString* GeneratedSource = GeneratedParseSourceByKey.Find(NormalizeToken(WriteRoundtripParseKey));
            FString GeneratedSourcePath;
            if (!GeneratedSource)
            {
                FParseFixtureSpec ParseSpec;
                if (!FindParseFixtureSpecByKey(WriteRoundtripParseKey, ParseSpec))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Missing parse fixture spec for write roundtrip %s (%s)"), *Spec.Name, *WriteRoundtripParseKey);
                    return 7;
                }

                const bool bRoundtripMap = ParseSpec.Kind == EParseFixtureKind::Level;
                const FString RoundtripPackageName = FString::Printf(TEXT("/Game/BPXFixtures/Operations/%s/RoundtripFixture"), *Spec.Name);
                const FString RoundtripAssetName = ParseSpec.Key;
                GeneratedSourcePath = FPackageName::LongPackageNameToFilename(
                    RoundtripPackageName,
                    bRoundtripMap ? FPackageName::GetMapPackageExtension() : FPackageName::GetAssetPackageExtension()
                );

                UObject* AssetObject = nullptr;
                switch (ParseSpec.Kind)
                {
                case EParseFixtureKind::Blueprint:
                {
                    UBlueprint* Blueprint = CreateActorBlueprintAsset(RoundtripPackageName, RoundtripAssetName, AActor::StaticClass(), TEXT(""));
                    if (!Blueprint)
                    {
                        UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create write roundtrip blueprint fixture: %s"), *Spec.Name);
                        return 7;
                    }
                    PopulateBlueprintParseFixture(Blueprint, ParseSpec.Key);
                    AssetObject = Blueprint;
                    break;
                }
                case EParseFixtureKind::DataTable:
                {
                    UPackage* Package = CreatePackage(*RoundtripPackageName);
                    UDataTable* DataTable = Package ? NewObject<UDataTable>(Package, FName(*RoundtripAssetName), RF_Public | RF_Standalone) : nullptr;
                    if (!DataTable)
                    {
                        UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create write roundtrip datatable fixture: %s"), *Spec.Name);
                        return 7;
                    }
                    DataTable->RowStruct = FTableRowBase::StaticStruct();
                    const FTableRowBase EmptyRow;
                    DataTable->AddRow(TEXT("Row_A"), EmptyRow);
                    DataTable->AddRow(TEXT("Row_B"), EmptyRow);
                    DataTable->AddRow(TEXT("Row_C"), EmptyRow);
                    FAssetRegistryModule::AssetCreated(DataTable);
                    DataTable->MarkPackageDirty();
                    AssetObject = DataTable;
                    break;
                }
                case EParseFixtureKind::StringTable:
                {
                    UPackage* Package = CreatePackage(*RoundtripPackageName);
                    UStringTable* StringTable = Package ? NewObject<UStringTable>(Package, FName(*RoundtripAssetName), RF_Public | RF_Standalone) : nullptr;
                    if (!StringTable)
                    {
                        UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create write roundtrip stringtable fixture: %s"), *Spec.Name);
                        return 7;
                    }
                    PopulateStringTableFixture(StringTable);
                    FAssetRegistryModule::AssetCreated(StringTable);
                    StringTable->MarkPackageDirty();
                    AssetObject = StringTable;
                    break;
                }
                case EParseFixtureKind::Level:
                {
                    UWorld* World = CreateLevelFixtureWorld(RoundtripPackageName, RoundtripAssetName);
                    if (!World)
                    {
                        UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create write roundtrip level fixture: %s"), *Spec.Name);
                        return 7;
                    }
                    AssetObject = World;
                    break;
                }
                default:
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Unsupported write roundtrip parse fixture kind for %s"), *Spec.Name);
                    return 7;
                }

                if (!SavePackageToDisk(AssetObject->GetOutermost(), AssetObject, GeneratedSourcePath, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                    return 7;
                }
                GeneratedParseSourceByKey.Add(NormalizeToken(WriteRoundtripParseKey), GeneratedSourcePath);
                GeneratedSource = &GeneratedSourcePath;
            }
            if (!GeneratedSource)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Missing generated parse fixture for write roundtrip %s (%s)"), *Spec.Name, *WriteRoundtripParseKey);
                return 7;
            }
            if (!CopyFileToOutput(*GeneratedSource, BeforeOutput, bForce, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 8;
            }
            if (!CopyFileToOutput(*GeneratedSource, AfterOutput, bForce, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 8;
            }
            ++OperationGeneratedCount;
            continue;
        }

        if (!DeleteFileIfExists(BeforeSource, ErrorText))
        {
            UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
            return 7;
        }
        if (!DeleteFileIfExists(AfterSource, ErrorText))
        {
            UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
            return 7;
        }

        if (!DeleteFileIfExists(FixtureSource, ErrorText))
        {
            UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
            return 7;
        }
        if (!DeleteFileIfExists(CompositeParentSource, ErrorText))
        {
            UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
            return 7;
        }

        if (bUseCompositeDataTableRejectMutation)
        {
            UPackage* ParentPackage = CreatePackage(*CompositeParentPackageName);
            if (!ParentPackage)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create composite parent datatable package: %s"), *Spec.Name);
                return 7;
            }

            UDataTable* ParentDataTable = NewObject<UDataTable>(ParentPackage, TEXT("FixtureParent"), RF_Public | RF_Standalone);
            if (!ParentDataTable)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create composite parent datatable object: %s"), *Spec.Name);
                return 7;
            }
            ParentDataTable->RowStruct = FBPXOperationTableRow::StaticStruct();
            ParentDataTable->AddRow(TEXT("Row_A"), FBPXOperationTableRow());
            FAssetRegistryModule::AssetCreated(ParentDataTable);
            ParentDataTable->MarkPackageDirty();
            if (!SavePackageToDisk(ParentPackage, ParentDataTable, CompositeParentSource, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 7;
            }

            UPackage* FixturePackage = CreatePackage(*FixturePackageName);
            if (!FixturePackage)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create composite datatable fixture package: %s"), *Spec.Name);
                return 7;
            }

            UCompositeDataTable* FixtureDataTable = NewObject<UCompositeDataTable>(FixturePackage, TEXT("Fixture"), RF_Public | RF_Standalone);
            if (!FixtureDataTable)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create composite datatable fixture object: %s"), *Spec.Name);
                return 7;
            }

            FixtureDataTable->RowStruct = FBPXOperationTableRow::StaticStruct();
            FixtureDataTable->AddParentTable(ParentDataTable);
            FAssetRegistryModule::AssetCreated(FixtureDataTable);
            FixtureDataTable->MarkPackageDirty();

            if (!SavePackageToDisk(FixturePackage, FixtureDataTable, FixtureSource, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 7;
            }
            if (!CopyFileToOutput(FixtureSource, BeforeOutput, bForce, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 8;
            }
            if (!CopyFileToOutput(FixtureSource, AfterOutput, bForce, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 8;
            }
        }
        else if (bUseStringTableMutation)
        {
            UPackage* FixturePackage = CreatePackage(*FixturePackageName);
            if (!FixturePackage)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create stringtable fixture package: %s"), *Spec.Name);
                return 7;
            }

            UStringTable* FixtureStringTable = NewObject<UStringTable>(FixturePackage, TEXT("Fixture"), RF_Public | RF_Standalone);
            if (!FixtureStringTable)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create stringtable fixture object: %s"), *Spec.Name);
                return 7;
            }

            PopulateStringTableFixture(FixtureStringTable);
            FAssetRegistryModule::AssetCreated(FixtureStringTable);
            FixtureStringTable->MarkPackageDirty();

            if (!SavePackageToDisk(FixturePackage, FixtureStringTable, FixtureSource, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 7;
            }
            if (!CopyFileToOutput(FixtureSource, BeforeOutput, bForce, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 8;
            }

            if (Spec.Expect == TEXT("error_equal"))
            {
                if (!CopyFileToOutput(FixtureSource, AfterOutput, bForce, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                    return 8;
                }
            }
            else
            {
                if (!ApplyStringTableOperationAfterState(FixtureStringTable, Spec, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to mutate stringtable fixture %s: %s"), *Spec.Name, *ErrorText);
                    return 7;
                }
                if (!SavePackageToDisk(FixturePackage, FixtureStringTable, FixtureSource, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                    return 7;
                }
                if (!CopyFileToOutput(FixtureSource, AfterOutput, bForce, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                    return 8;
                }
            }
        }
        else if (bUseDataTableMutation)
        {
            UPackage* FixturePackage = CreatePackage(*FixturePackageName);
            if (!FixturePackage)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create datatable fixture package: %s"), *Spec.Name);
                return 7;
            }

            UDataTable* FixtureDataTable = NewObject<UDataTable>(FixturePackage, TEXT("Fixture"), RF_Public | RF_Standalone);
            if (!FixtureDataTable)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create datatable fixture object: %s"), *Spec.Name);
                return 7;
            }

            FixtureDataTable->RowStruct = FBPXOperationTableRow::StaticStruct();
            auto MakeRow = [](int32 Score, float Rate, const TCHAR* Label, EBPXFixtureEnum Mode) {
                FBPXOperationTableRow Row;
                Row.Score = Score;
                Row.Rate = Rate;
                Row.Label = Label;
                Row.Mode = Mode;
                return Row;
            };

            const FBPXOperationTableRow RowA = MakeRow(0, 0.0f, TEXT(""), BPXEnum_ValueA);
            const FBPXOperationTableRow RowB = MakeRow(20, 0.5f, TEXT("Row_B_seed"), BPXEnum_ValueB);
            const FBPXOperationTableRow RowC = MakeRow(30, 0.75f, TEXT("Row_C_seed"), BPXEnum_ValueC);
            const FBPXOperationTableRow DefaultRow = MakeRow(0, 0.0f, TEXT(""), BPXEnum_ValueA);
            const FBPXOperationTableRow AddRowScalar = MakeRow(123, 0.0f, TEXT(""), BPXEnum_ValueA);
            const FBPXOperationTableRow AddRowMixed = MakeRow(7, 0.25f, TEXT("Row_B_added"), BPXEnum_ValueB);

            FixtureDataTable->AddRow(TEXT("Row_A"), RowA);
            FixtureDataTable->AddRow(TEXT("Row_B"), RowB);
            FixtureDataTable->AddRow(TEXT("Row_C"), RowC);
            if (Spec.Name == TEXT("dt_remove_row"))
            {
                FixtureDataTable->AddRow(TEXT("Row_A_1"), DefaultRow);
            }
            else if (Spec.Name == TEXT("dt_remove_row_base"))
            {
                // Keep Row_B base name referenced after removing "Row_B" so NameMap stays stable.
                FixtureDataTable->AddRow(TEXT("Row_B_1"), AddRowMixed);
            }
            FAssetRegistryModule::AssetCreated(FixtureDataTable);
            FixtureDataTable->MarkPackageDirty();

            if (!SavePackageToDisk(FixturePackage, FixtureDataTable, FixtureSource, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 7;
            }
            if (!CopyFileToOutput(FixtureSource, BeforeOutput, bForce, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 8;
            }

            if (Spec.Name == TEXT("dt_add_row"))
            {
                FixtureDataTable->AddRow(TEXT("Row_A_1"), DefaultRow);
            }
            else if (Spec.Name == TEXT("dt_add_row_values_scalar"))
            {
                FixtureDataTable->AddRow(TEXT("Row_A_1"), AddRowScalar);
            }
            else if (Spec.Name == TEXT("dt_add_row_values_mixed"))
            {
                FixtureDataTable->AddRow(TEXT("Row_B_1"), AddRowMixed);
            }
            else if (Spec.Name == TEXT("dt_remove_row"))
            {
                // UDataTable::RemoveRow emits a single-row change event with the removed key and
                // UE5.6 core path dereferences RowMap[ChangedRowName] after removal.
                // Rebuild rows explicitly to generate a stable after-state without triggering that path.
                FixtureDataTable->EmptyTable();
                FixtureDataTable->AddRow(TEXT("Row_A"), RowA);
                FixtureDataTable->AddRow(TEXT("Row_B"), RowB);
                FixtureDataTable->AddRow(TEXT("Row_C"), RowC);
            }
            else if (Spec.Name == TEXT("dt_remove_row_base"))
            {
                FixtureDataTable->EmptyTable();
                FixtureDataTable->AddRow(TEXT("Row_A"), RowA);
                FixtureDataTable->AddRow(TEXT("Row_C"), RowC);
                FixtureDataTable->AddRow(TEXT("Row_B_1"), AddRowMixed);
            }
            else
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Unsupported datatable operation fixture: %s"), *Spec.Name);
                return 7;
            }
            FixtureDataTable->MarkPackageDirty();

            if (!SavePackageToDisk(FixturePackage, FixtureDataTable, FixtureSource, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 7;
            }
            if (!CopyFileToOutput(FixtureSource, AfterOutput, bForce, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 8;
            }
        }
        else if (bUseDataTableUpdateMutation)
        {
            UPackage* FixturePackage = CreatePackage(*FixturePackageName);
            if (!FixturePackage)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create datatable update fixture package: %s"), *Spec.Name);
                return 7;
            }

            UDataTable* FixtureDataTable = NewObject<UDataTable>(FixturePackage, TEXT("Fixture"), RF_Public | RF_Standalone);
            if (!FixtureDataTable)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create datatable update fixture object: %s"), *Spec.Name);
                return 7;
            }

            FixtureDataTable->RowStruct = FBPXOperationTableRow::StaticStruct();
            auto MakeRow = [](int32 Score, float Rate, const TCHAR* Label, EBPXFixtureEnum Mode) {
                FBPXOperationTableRow Row;
                Row.Score = Score;
                Row.Rate = Rate;
                Row.Label = Label;
                Row.Mode = Mode;
                return Row;
            };

            FixtureDataTable->AddRow(TEXT("Row_A"), MakeRow(0, 0.0f, TEXT(""), BPXEnum_ValueA));
            FixtureDataTable->AddRow(TEXT("Row_B"), MakeRow(20, 0.5f, TEXT("Row_B_seed"), BPXEnum_ValueB));
            FixtureDataTable->AddRow(TEXT("Row_C"), MakeRow(30, 0.75f, TEXT("Row_C_seed"), BPXEnum_ValueC));
            FAssetRegistryModule::AssetCreated(FixtureDataTable);
            FixtureDataTable->MarkPackageDirty();

            if (!SavePackageToDisk(FixturePackage, FixtureDataTable, FixtureSource, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 7;
            }
            if (!CopyFileToOutput(FixtureSource, BeforeOutput, bForce, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 8;
            }

            if (Spec.Expect == TEXT("error_equal"))
            {
                if (!CopyFileToOutput(FixtureSource, AfterOutput, bForce, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                    return 8;
                }
            }
            else
            {
                if (!ApplyDataTableUpdateAfterState(FixtureDataTable, Spec, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to mutate datatable update fixture %s: %s"), *Spec.Name, *ErrorText);
                    return 7;
                }
                if (!SavePackageToDisk(FixturePackage, FixtureDataTable, FixtureSource, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                    return 7;
                }
                if (!CopyFileToOutput(FixtureSource, AfterOutput, bForce, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                    return 8;
                }
            }
        }
        else if (bUseParseBlueprintMutation)
        {
            const FString FixtureKey = ParseBlueprintFixtureKeyForOperation(Spec);
            UBlueprint* FixtureBlueprint = CreateActorBlueprintAsset(FixturePackageName, FixtureKey, AActor::StaticClass(), TEXT(""));
            if (!FixtureBlueprint)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create parse-backed operation fixture package: %s"), *Spec.Name);
                return 7;
            }

            PopulateBlueprintParseFixture(FixtureBlueprint, FixtureKey);

            if (!SavePackageToDisk(FixtureBlueprint->GetOutermost(), FixtureBlueprint, FixtureSource, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 7;
            }
            if (!CopyFileToOutput(FixtureSource, BeforeOutput, bForce, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 8;
            }

            if (Spec.Expect == TEXT("error_equal"))
            {
                if (!CopyFileToOutput(FixtureSource, AfterOutput, bForce, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                    return 8;
                }
            }
            else
            {
                if (!ApplyParseBlueprintOperationAfterState(FixtureBlueprint, Spec, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to mutate parse-backed operation fixture %s: %s"), *Spec.Name, *ErrorText);
                    return 7;
                }
                if (!SavePackageToDisk(FixtureBlueprint->GetOutermost(), FixtureBlueprint, FixtureSource, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                    return 7;
                }
                if (!CopyFileToOutput(FixtureSource, AfterOutput, bForce, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                    return 8;
                }
            }
        }
        else if (bUseWidgetBlueprintMutation)
        {
            const FString FixtureKey = ParseWidgetFixtureKeyForOperation(Spec);
            UWidgetBlueprint* FixtureBlueprint = DuplicateWidgetBlueprintAsset(FixtureKey, FixturePackageName, FixtureKey, ErrorText);
            if (!FixtureBlueprint)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 7;
            }

            if (!SavePackageToDisk(FixtureBlueprint->GetOutermost(), FixtureBlueprint, FixtureSource, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 7;
            }
            if (!CopyFileToOutput(FixtureSource, BeforeOutput, bForce, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 8;
            }

            if (Spec.Expect == TEXT("error_equal"))
            {
                if (!CopyFileToOutput(FixtureSource, AfterOutput, bForce, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                    return 8;
                }
            }
            else
            {
                if (!ApplyWidgetBlueprintOperationAfterState(FixtureBlueprint, Spec, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to mutate widget blueprint fixture %s: %s"), *Spec.Name, *ErrorText);
                    return 7;
                }
                if (!SavePackageToDisk(FixtureBlueprint->GetOutermost(), FixtureBlueprint, FixtureSource, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                    return 7;
                }
                if (!CopyFileToOutput(FixtureSource, AfterOutput, bForce, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                    return 8;
                }
            }
        }
        else if (bUseLevelMutation)
        {
            UWorld* FixtureWorld = CreateLevelFixtureWorld(FixturePackageName, TEXT("Fixture"));
            if (!FixtureWorld)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create level fixture world: %s"), *Spec.Name);
                return 7;
            }

            if (!SavePackageToDisk(FixtureWorld->GetOutermost(), FixtureWorld, FixtureSource, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 7;
            }
            if (!CopyFileToOutput(FixtureSource, BeforeOutput, bForce, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 8;
            }

            if (Spec.Expect == TEXT("error_equal"))
            {
                if (!CopyFileToOutput(FixtureSource, AfterOutput, bForce, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                    return 8;
                }
                if (GEngine)
                {
                    GEngine->DestroyWorldContext(FixtureWorld);
                }
                FixtureWorld->DestroyWorld(false);
                ++OperationGeneratedCount;
                continue;
            }

            if (!ApplyLevelOperationAfterState(FixtureWorld, Spec, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to mutate level fixture %s: %s"), *Spec.Name, *ErrorText);
                return 7;
            }

            if (!SavePackageToDisk(FixtureWorld->GetOutermost(), FixtureWorld, FixtureSource, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 7;
            }
            if (!CopyFileToOutput(FixtureSource, AfterOutput, bForce, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 8;
            }

            if (GEngine)
            {
                GEngine->DestroyWorldContext(FixtureWorld);
            }
            FixtureWorld->DestroyWorld(false);
        }
        else if (bUseNameMutation)
        {
            const FString* BaseFixtureSource = GeneratedParseSourceByKey.Find(NormalizeToken(TEXT("BP_Empty")));
            if (!BaseFixtureSource)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Missing generated BP_Empty parse fixture for name operation: %s"), *Spec.Name);
                return 7;
            }

            TArray64<uint8> BaseBytes;
            if (!FFileHelper::LoadFileToArray(BaseBytes, **BaseFixtureSource))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to load base name fixture bytes: %s"), **BaseFixtureSource);
                return 7;
            }

            FNameFixtureSummaryInfo BaseSummary;
            if (!ReadSummaryInfo(BaseBytes, BaseSummary, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to parse base name fixture summary %s: %s"), *Spec.Name, *ErrorText);
                return 7;
            }

            TArray<FNameFixtureEntry> BaseEntries;
            if (!ReadNameEntries(BaseBytes, BaseSummary, BaseEntries, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to read base NameMap %s: %s"), *Spec.Name, *ErrorText);
                return 7;
            }

            TArray<FNameFixtureEntry> BeforeEntries;
            if (!BuildNameOperationEntries(BaseEntries, Spec, true, BeforeEntries, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to build before NameMap entries for %s: %s"), *Spec.Name, *ErrorText);
                return 7;
            }

            TArray64<uint8> BeforeBytes;
            if (!RewriteNameMapPackageBytes(BaseBytes, BeforeEntries, BeforeBytes, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to build before bytes for %s: %s"), *Spec.Name, *ErrorText);
                return 7;
            }
            if (!WriteBytesToOutput(BeforeBytes, BeforeOutput, bForce, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 8;
            }

            TArray<FNameFixtureEntry> AfterEntries;
            if (!BuildNameOperationEntries(BeforeEntries, Spec, false, AfterEntries, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to build after NameMap entries for %s: %s"), *Spec.Name, *ErrorText);
                return 7;
            }

            TArray64<uint8> AfterBytes;
            if (!RewriteNameMapPackageBytes(BeforeBytes, AfterEntries, AfterBytes, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to build after bytes for %s: %s"), *Spec.Name, *ErrorText);
                return 7;
            }
            if (!WriteBytesToOutput(AfterBytes, AfterOutput, bForce, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 8;
            }
        }
        else if (bUseSinglePackageMutation)
        {
            const FString InitialDefault = TEXT("");
            UClass* FixtureParentClass = UsesNativeOperationFixtureParent(Spec) ? ABPXOperationFixtureActor::StaticClass() : AActor::StaticClass();
            UBlueprint* FixtureBlueprint = CreateActorBlueprintAsset(FixturePackageName, TEXT("Fixture"), FixtureParentClass, InitialDefault);
            if (!FixtureBlueprint)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create operation fixture package: %s"), *Spec.Name);
                return 7;
            }

            if (!FixtureBlueprint->GeneratedClass)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("GeneratedClass is null for operation fixture: %s"), *Spec.Name);
                return 7;
            }

            if (Spec.Name == TEXT("prop_add")
                || Spec.Name == TEXT("prop_add_fixture_int")
                || Spec.Name == TEXT("prop_remove")
                || Spec.Name == TEXT("prop_remove_fixture_int"))
            {
                const bool bBoolOperation = Spec.Name == TEXT("prop_add") || Spec.Name == TEXT("prop_remove");
                const bool bIntOperation = Spec.Name == TEXT("prop_add_fixture_int") || Spec.Name == TEXT("prop_remove_fixture_int");
                const FName TargetPropertyName = bBoolOperation ? FName(TEXT("bCanBeDamaged")) : FName(TEXT("FixtureInt"));
                const FName TargetTypeName = bBoolOperation ? FName(TEXT("BoolProperty")) : FName(TEXT("IntProperty"));
                if (!bBoolOperation && !bIntOperation)
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Unsupported prop add/remove fixture operation: %s"), *Spec.Name);
                    return 7;
                }

                // Keep property/type names in NameMap across before/after so fixture diff focuses on
                // tagged property presence and value bytes, not NameMap structural edits.
                if (UPackage* FixturePackage = FixtureBlueprint->GetOutermost())
                {
                    FMetaData& MetaData = FixturePackage->GetMetaData();
                    MetaData.SetValue(FixtureBlueprint, TargetPropertyName, TEXT("FixtureSeed"));
                    MetaData.SetValue(FixtureBlueprint, TargetTypeName, TEXT("TypeSeed"));
                }

                UObject* CDO = nullptr;
                FBoolProperty* BoolProperty = nullptr;
                FIntProperty* IntProperty = nullptr;
                auto ResolveTargetProperty = [&](UBlueprint* Blueprint) -> bool
                {
                    CDO = nullptr;
                    BoolProperty = nullptr;
                    IntProperty = nullptr;
                    if (!Blueprint || !Blueprint->GeneratedClass)
                    {
                        return false;
                    }
                    CDO = Blueprint->GeneratedClass->GetDefaultObject();
                    if (!CDO)
                    {
                        return false;
                    }
                    if (bBoolOperation)
                    {
                        BoolProperty = FindFProperty<FBoolProperty>(Blueprint->GeneratedClass, TEXT("bCanBeDamaged"));
                        return BoolProperty != nullptr;
                    }
                    IntProperty = FindFProperty<FIntProperty>(Blueprint->GeneratedClass, TEXT("FixtureInt"));
                    return IntProperty != nullptr;
                };

                if (!ResolveTargetProperty(FixtureBlueprint))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("target property not found for operation fixture: %s"), *Spec.Name);
                    return 7;
                }

                if (bBoolOperation)
                {
                    const bool bBeforeValue = Spec.Name == TEXT("prop_add");
                    const bool bAfterValue = !bBeforeValue;
                    BoolProperty->SetPropertyValue_InContainer(CDO, bBeforeValue);
                    CDO->MarkPackageDirty();

                    if (!SavePackageToDisk(FixtureBlueprint->GetOutermost(), FixtureBlueprint, FixtureSource, ErrorText))
                    {
                        UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                        return 7;
                    }
                    if (!CopyFileToOutput(FixtureSource, BeforeOutput, bForce, ErrorText))
                    {
                        UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                        return 8;
                    }

                    if (!ResolveTargetProperty(FixtureBlueprint))
                    {
                        UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to resolve refreshed bool target property for operation fixture: %s"), *Spec.Name);
                        return 7;
                    }

                    BoolProperty->SetPropertyValue_InContainer(CDO, bAfterValue);
                }
                else
                {
                    const int32 BeforeValue = Spec.Name == TEXT("prop_add_fixture_int") ? 0 : 42;
                    const int32 AfterValue = Spec.Name == TEXT("prop_add_fixture_int") ? 42 : 0;
                    IntProperty->SetPropertyValue_InContainer(CDO, BeforeValue);
                    CDO->MarkPackageDirty();

                    if (!SavePackageToDisk(FixtureBlueprint->GetOutermost(), FixtureBlueprint, FixtureSource, ErrorText))
                    {
                        UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                        return 7;
                    }
                    if (!CopyFileToOutput(FixtureSource, BeforeOutput, bForce, ErrorText))
                    {
                        UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                        return 8;
                    }

                    if (!ResolveTargetProperty(FixtureBlueprint))
                    {
                        UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to resolve refreshed int target property for operation fixture: %s"), *Spec.Name);
                        return 7;
                    }

                    IntProperty->SetPropertyValue_InContainer(CDO, AfterValue);
                }
                CDO->MarkPackageDirty();

                if (!SavePackageToDisk(FixtureBlueprint->GetOutermost(), FixtureBlueprint, FixtureSource, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                    return 7;
                }
                if (!CopyFileToOutput(FixtureSource, AfterOutput, bForce, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                    return 8;
                }
            }
            else
            {
                if (!ConfigureOperationBlueprintVariables(FixtureBlueprint, Spec, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to configure operation blueprint %s: %s"), *Spec.Name, *ErrorText);
                    return 7;
                }

                if (!ApplyOperationBlueprintState(FixtureBlueprint, Spec, true, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to set before-state for %s: %s"), *Spec.Name, *ErrorText);
                    return 7;
                }
                if (!SavePackageToDisk(FixtureBlueprint->GetOutermost(), FixtureBlueprint, FixtureSource, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                    return 7;
                }
                if (!CopyFileToOutput(FixtureSource, BeforeOutput, bForce, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                    return 8;
                }

                if (Spec.Expect == TEXT("error_equal"))
                {
                    if (!CopyFileToOutput(FixtureSource, AfterOutput, bForce, ErrorText))
                    {
                        UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                        return 8;
                    }
                }
                else
                {
                    if (!ApplyOperationBlueprintState(FixtureBlueprint, Spec, false, ErrorText))
                    {
                        UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to set after-state for %s: %s"), *Spec.Name, *ErrorText);
                        return 7;
                    }
                    const EObjectFlags AfterTopLevelFlags = Spec.Name == TEXT("export_set_header")
                        ? EObjectFlags(RF_Public)
                        : EObjectFlags(RF_Public | RF_Standalone);
                    if (!SavePackageToDisk(FixtureBlueprint->GetOutermost(), FixtureBlueprint, FixtureSource, ErrorText, AfterTopLevelFlags))
                    {
                        UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                        return 7;
                    }
                    if (!CopyFileToOutput(FixtureSource, AfterOutput, bForce, ErrorText))
                    {
                        UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                        return 8;
                    }
                }
            }
        }
        else
        {
            const FOperationBlueprintDefaults Defaults = ResolveOperationBlueprintDefaults(Spec);
            UBlueprint* BeforeBlueprint = CreateActorBlueprintAsset(BeforePackageName, TEXT("Before"), AActor::StaticClass(), Defaults.BeforeFixtureValue);
            UBlueprint* AfterBlueprint = CreateActorBlueprintAsset(AfterPackageName, TEXT("After"), AActor::StaticClass(), Defaults.AfterFixtureValue);
            if (!BeforeBlueprint || !AfterBlueprint)
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to create operation fixture pair: %s"), *Spec.Name);
                return 7;
            }
            if (!SavePackageToDisk(BeforeBlueprint->GetOutermost(), BeforeBlueprint, BeforeSource, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 7;
            }
            if (!SavePackageToDisk(AfterBlueprint->GetOutermost(), AfterBlueprint, AfterSource, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 7;
            }

            if (!CopyFileToOutput(BeforeSource, BeforeOutput, bForce, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 8;
            }
            if (!CopyFileToOutput(AfterSource, AfterOutput, bForce, ErrorText))
            {
                UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
                return 8;
            }
        }

        if (!WriteOperationSidecars(OperationDir, Spec, bForce, ShouldIgnoreSavedHash(Spec), ErrorText))
        {
            UE_LOG(LogBPXFixtureGenerator, Error, TEXT("%s"), *ErrorText);
            return 8;
        }

        ++OperationGeneratedCount;
    }

    UE_LOG(LogBPXFixtureGenerator, Display, TEXT("BPX fixture generation complete."));
    UE_LOG(LogBPXFixtureGenerator, Display, TEXT("  parse fixtures: %d"), ParseGeneratedCount);
    UE_LOG(LogBPXFixtureGenerator, Display, TEXT("  operation pairs: %d"), OperationGeneratedCount);
    UE_LOG(LogBPXFixtureGenerator, Display, TEXT("  output parse dir: %s"), *ParseDir);
    UE_LOG(LogBPXFixtureGenerator, Display, TEXT("  output operations dir: %s"), *OpsDir);

    return 0;
}

bool UBPXGenerateFixturesCommandlet::ValidateWindowsOrUncPath(const FString& InPath) const
{
    if (InPath.Len() >= 3)
    {
        const bool bDrivePrefix = FChar::IsAlpha(InPath[0]) && InPath[1] == TEXT(':') && (InPath[2] == TEXT('\\') || InPath[2] == TEXT('/'));
        if (bDrivePrefix)
        {
            return true;
        }
    }

    return InPath.StartsWith(TEXT("\\\\"));
}
