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
#include "Internationalization/StringTable.h"
#include "Engine/World.h"
#include "GameFramework/Actor.h"
#include "HAL/FileManager.h"
#include "HAL/PlatformFileManager.h"
#include "Internationalization/StringTableCore.h"
#include "Kismet2/BlueprintEditorUtils.h"
#include "Kismet2/EnumEditorUtils.h"
#include "Kismet2/KismetEditorUtilities.h"
#include "Kismet2/StructureEditorUtils.h"
#include "Materials/Material.h"
#include "Materials/MaterialInstanceConstant.h"
#include "Misc/CommandLine.h"
#include "Misc/FileHelper.h"
#include "Misc/PackageName.h"
#include "Misc/Paths.h"
#include "Serialization/JsonSerializer.h"
#include "Serialization/JsonWriter.h"
#include "UObject/Package.h"
#include "UObject/MetaData.h"
#include "UObject/SavePackage.h"
#include "UObject/UnrealType.h"
#include "StructUtils/UserDefinedStruct.h"
#include "Factories/WorldFactory.h"

DEFINE_LOG_CATEGORY_STATIC(LogBPXFixtureGenerator, Log, All);

namespace
{
enum class EParseFixtureKind
{
    Blueprint,
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
    FString Notes;
};

struct FOperationBlueprintDefaults
{
    FString BeforeFixtureValue;
    FString AfterFixtureValue;
};

void AddBlueprintMemberVariable(UBlueprint* Blueprint, const FName& VariableName, const FEdGraphPinType& PinType, const FString& DefaultValue);

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
        const TCHAR* Notes
    ) {
        return FOperationFixtureSpec{
            FString(Name),
            FString(Command),
            FString(ArgsJson),
            FString(UEProcedure),
            FString(Expect),
            FString(Notes)
        };
    };

    return {
        MakeOperation(TEXT("prop_add"), TEXT("prop add"), TEXT("{\"export\":5,\"spec\":\"{\\\"name\\\":\\\"bCanBeDamaged\\\",\\\"type\\\":\\\"BoolProperty\\\",\\\"value\\\":false}\"}"), TEXT("Add bCanBeDamaged override tag by changing default true -> false"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_add_fixture_int"), TEXT("prop add"), TEXT("{\"export\":5,\"spec\":\"{\\\"name\\\":\\\"FixtureInt\\\",\\\"type\\\":\\\"IntProperty\\\",\\\"value\\\":42}\"}"), TEXT("Add FixtureInt override tag by changing default 0 -> 42"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_remove"), TEXT("prop remove"), TEXT("{\"export\":5,\"path\":\"bCanBeDamaged\"}"), TEXT("Remove bCanBeDamaged override tag by changing default false -> true"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_remove_fixture_int"), TEXT("prop remove"), TEXT("{\"export\":5,\"path\":\"FixtureInt\"}"), TEXT("Remove FixtureInt override tag by changing default 42 -> 0"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_bool"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureBool\",\"value\":\"true\"}"), TEXT("Toggle native bool property"), TEXT("unsupported"), TEXT("Native bool default-elision behavior is not emulated yet.")),
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
        MakeOperation(TEXT("prop_set_text"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureText\",\"value\":\"\\\"Changed\\\"\"}"), TEXT("Set Text variable"), TEXT("unsupported"), TEXT("Fixture migration pending for text operation.")),
        MakeOperation(TEXT("prop_set_enum"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureEnum\",\"value\":\"\\\"BPXEnum_ValueA\\\"\"}"), TEXT("Set enum variable"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_enum_numeric"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureEnum\",\"value\":\"1\"}"), TEXT("Set enum variable by numeric literal"), TEXT("unsupported"), TEXT("Enum numeric literal coercion to FName is not implemented yet.")),
        MakeOperation(TEXT("prop_set_enum_anchor"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureEnumAnchor\",\"value\":\"\\\"BPXEnum_ValueA\\\"\"}"), TEXT("Set secondary enum variable"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_vector"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureVector\",\"value\":\"{\\\"X\\\":1.5,\\\"Y\\\":-2.3,\\\"Z\\\":100.0}\"}"), TEXT("Set Vector variable"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_vector_axis_x"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureVector.X\",\"value\":\"-123.456\"}"), TEXT("Set Vector.X field"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_rotator"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureRotator\",\"value\":\"{\\\"Pitch\\\":45,\\\"Yaw\\\":90,\\\"Roll\\\":180}\"}"), TEXT("Set Rotator variable"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_rotator_axis_roll"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureRotator.Roll\",\"value\":\"-45.5\"}"), TEXT("Set Rotator.Roll field"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_color"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureColor\",\"value\":\"{\\\"R\\\":1,\\\"G\\\":0,\\\"B\\\":0,\\\"A\\\":1}\"}"), TEXT("Set color variable"), TEXT("unsupported"), TEXT("Fixture migration pending for color operation.")),
        MakeOperation(TEXT("prop_set_transform"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureTransform\",\"value\":\"{\\\"Translation\\\":{\\\"X\\\":1,\\\"Y\\\":2,\\\"Z\\\":3}}\"}"), TEXT("Set transform variable"), TEXT("unsupported"), TEXT("Fixture migration pending for transform operation.")),
        MakeOperation(TEXT("prop_set_soft_object"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureSoft\",\"value\":\"{\\\"packageName\\\":\\\"/Game/New\\\",\\\"assetName\\\":\\\"Asset\\\"}\"}"), TEXT("Set soft object path variable"), TEXT("unsupported"), TEXT("Fixture migration pending for soft-object operation.")),
        MakeOperation(TEXT("prop_set_array_element"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"MyArray[1]\",\"value\":\"99\"}"), TEXT("Set array element"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_array_replace_longer"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"MyArray\",\"value\":\"[1,2,3,4,5,6,7,8]\"}"), TEXT("Replace array with longer payload"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_array_replace_empty"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"MyArray\",\"value\":\"[4]\"}"), TEXT("Replace array with shorter payload"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_map_value"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"MyMap[\\\"key\\\"]\",\"value\":\"99\"}"), TEXT("Set map value"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("prop_set_custom_struct_int"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureCustom.IntVal\",\"value\":\"42\"}"), TEXT("Set custom struct int field"), TEXT("unsupported"), TEXT("Custom StructProperty tagged re-encoding is not implemented yet.")),
        MakeOperation(TEXT("prop_set_custom_struct_enum"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"FixtureCustom.EnumVal\",\"value\":\"\\\"BPXEnum_ValueB\\\"\"}"), TEXT("Set custom struct enum field"), TEXT("unsupported"), TEXT("Custom StructProperty tagged re-encoding is not implemented yet.")),
        MakeOperation(TEXT("prop_set_nested_struct"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"Inner.IntVal\",\"value\":\"42\"}"), TEXT("Set nested struct value"), TEXT("unsupported"), TEXT("Fixture migration pending for nested-struct operation.")),
        MakeOperation(TEXT("prop_set_nested_array_struct"), TEXT("prop set"), TEXT("{\"export\":5,\"path\":\"InnerArray[0].StrVal\",\"value\":\"\\\"new\\\"\"}"), TEXT("Set nested array struct value"), TEXT("unsupported"), TEXT("Fixture migration pending for nested-array operation.")),
        MakeOperation(TEXT("dt_update_int"), TEXT("datatable update-row"), TEXT("{\"row\":\"Row_A\",\"values\":\"{\\\"Score\\\":999}\"}"), TEXT("Update DataTable int column"), TEXT("unsupported"), TEXT("datatable update-row is not implemented yet.")),
        MakeOperation(TEXT("dt_update_float"), TEXT("datatable update-row"), TEXT("{\"row\":\"Row_B\",\"values\":\"{\\\"Rate\\\":0.5}\"}"), TEXT("Update DataTable float column"), TEXT("unsupported"), TEXT("datatable update-row is not implemented yet.")),
        MakeOperation(TEXT("dt_update_string"), TEXT("datatable update-row"), TEXT("{\"row\":\"Row_A\",\"values\":\"{\\\"Name\\\":\\\"NewName\\\"}\"}"), TEXT("Update DataTable string column"), TEXT("unsupported"), TEXT("datatable update-row is not implemented yet.")),
        MakeOperation(TEXT("dt_update_multi_field"), TEXT("datatable update-row"), TEXT("{\"row\":\"Row_A\",\"values\":\"{\\\"Score\\\":50,\\\"Rate\\\":0.1}\"}"), TEXT("Update DataTable multiple columns"), TEXT("unsupported"), TEXT("datatable update-row is not implemented yet.")),
        MakeOperation(TEXT("dt_update_complex"), TEXT("datatable update-row"), TEXT("{\"row\":\"Row_A\",\"values\":\"{\\\"Tags\\\":[\\\"TagA\\\",\\\"TagB\\\"]}\"}"), TEXT("Update DataTable complex column"), TEXT("unsupported"), TEXT("datatable update-row is not implemented yet.")),
        MakeOperation(TEXT("dt_add_row"), TEXT("datatable add-row"), TEXT("{\"row\":\"Row_A_1\"}"), TEXT("Add one DataTable row"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("dt_add_row_values_scalar"), TEXT("datatable add-row"), TEXT("{\"row\":\"Row_A_1\",\"values\":\"{\\\"Score\\\":123}\"}"), TEXT("Add one DataTable row with scalar field update"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("dt_add_row_values_mixed"), TEXT("datatable add-row"), TEXT("{\"row\":\"Row_B_1\",\"values\":\"{\\\"Score\\\":7,\\\"Rate\\\":0.25,\\\"Label\\\":\\\"Row_B_added\\\",\\\"Mode\\\":\\\"BPXEnum_ValueB\\\"}\"}"), TEXT("Add one DataTable row with mixed-type field updates"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("dt_remove_row"), TEXT("datatable remove-row"), TEXT("{\"row\":\"Row_A_1\"}"), TEXT("Remove one DataTable row"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("dt_remove_row_base"), TEXT("datatable remove-row"), TEXT("{\"row\":\"Row_B\"}"), TEXT("Remove one base DataTable row"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("dt_add_row_composite_reject"), TEXT("datatable add-row"), TEXT("{\"row\":\"Row_A_1\"}"), TEXT("Reject add-row against CompositeDataTable"), TEXT("unsupported"), TEXT("UE5.6 UCompositeDataTable::AddRow is no-op; bpx rejects writes to composite tables.")),
        MakeOperation(TEXT("dt_remove_row_composite_reject"), TEXT("datatable remove-row"), TEXT("{\"row\":\"Row_A\"}"), TEXT("Reject remove-row against CompositeDataTable"), TEXT("unsupported"), TEXT("UE5.6 UCompositeDataTable::RemoveRow is no-op; bpx rejects writes to composite tables.")),
        MakeOperation(TEXT("dt_update_row_composite_reject"), TEXT("datatable update-row"), TEXT("{\"row\":\"Row_A\",\"values\":\"{\\\"Score\\\":999}\"}"), TEXT("Reject update-row against CompositeDataTable"), TEXT("unsupported"), TEXT("UE5.6 composite tables rebuild rows from parents; bpx rejects writes to composite tables.")),
        MakeOperation(TEXT("metadata_set_tooltip"), TEXT("metadata set-root"), TEXT("{\"export\":1,\"key\":\"ToolTip\",\"value\":\"Updated\"}"), TEXT("Set root metadata tooltip"), TEXT("unsupported"), TEXT("metadata set-root is not implemented yet.")),
        MakeOperation(TEXT("metadata_set_category"), TEXT("metadata set-root"), TEXT("{\"export\":1,\"key\":\"Category\",\"value\":\"Gameplay\"}"), TEXT("Set root metadata category"), TEXT("unsupported"), TEXT("metadata set-root is not implemented yet.")),
        MakeOperation(TEXT("export_set_header"), TEXT("export set-header"), TEXT("{\"index\":1,\"fields\":\"{\\\"objectFlags\\\":1}\"}"), TEXT("Set export header fields"), TEXT("unsupported"), TEXT("export set-header is not implemented yet.")),
        MakeOperation(TEXT("package_set_flags"), TEXT("package set-flags"), TEXT("{\"flags\":\"PKG_FilterEditorOnly\"}"), TEXT("Set package flags"), TEXT("unsupported"), TEXT("package set-flags is not implemented yet.")),
        MakeOperation(TEXT("var_set_default_int"), TEXT("var set-default"), TEXT("{\"name\":\"MyStr\",\"value\":\"\\\"changed\\\"\"}"), TEXT("Set variable default value"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("var_set_default_empty"), TEXT("var set-default"), TEXT("{\"name\":\"MyStr\",\"value\":\"\\\"\\\"\"}"), TEXT("Set variable default value to empty string"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("var_set_default_unicode"), TEXT("var set-default"), TEXT("{\"name\":\"MyStr\",\"value\":\"\\\"テスト\\\"\"}"), TEXT("Set variable default value to unicode string"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("var_set_default_long"), TEXT("var set-default"), TEXT("{\"name\":\"MyStr\",\"value\":\"\\\"Lorem ipsum dolor sit amet var-default\\\"\"}"), TEXT("Set variable default value to long string"), TEXT("byte_equal"), TEXT("Validated by operation-equivalence test.")),
        MakeOperation(TEXT("var_set_default_string"), TEXT("var set-default"), TEXT("{\"name\":\"FixtureValue\",\"value\":\"\\\"NewTitle\\\"\"}"), TEXT("Set variable string default value"), TEXT("unsupported"), TEXT("Fixture migration pending for var set-default string.")),
        MakeOperation(TEXT("var_set_default_vector"), TEXT("var set-default"), TEXT("{\"name\":\"FixtureValue\",\"value\":\"{\\\"X\\\":1,\\\"Y\\\":2,\\\"Z\\\":3}\"}"), TEXT("Set variable vector default value"), TEXT("unsupported"), TEXT("Fixture migration pending for var set-default vector.")),
        MakeOperation(TEXT("var_rename_simple"), TEXT("var rename"), TEXT("{\"from\":\"OldVar\",\"to\":\"NewVar\"}"), TEXT("Rename simple variable"), TEXT("unsupported"), TEXT("var rename is not implemented yet.")),
        MakeOperation(TEXT("var_rename_with_refs"), TEXT("var rename"), TEXT("{\"from\":\"UsedVar\",\"to\":\"RenamedVar\"}"), TEXT("Rename referenced variable"), TEXT("unsupported"), TEXT("var rename is not implemented yet.")),
        MakeOperation(TEXT("var_rename_unicode"), TEXT("var rename"), TEXT("{\"from\":\"体力\",\"to\":\"HP\"}"), TEXT("Rename unicode variable"), TEXT("unsupported"), TEXT("var rename is not implemented yet.")),
        MakeOperation(TEXT("ref_rewrite_single"), TEXT("ref rewrite"), TEXT("{\"from\":\"/Game/Old/Mesh\",\"to\":\"/Game/New/Mesh\"}"), TEXT("Rewrite one soft reference"), TEXT("unsupported"), TEXT("ref rewrite is not implemented yet.")),
        MakeOperation(TEXT("ref_rewrite_multi"), TEXT("ref rewrite"), TEXT("{\"from\":\"/Game/OldDir\",\"to\":\"/Game/NewDir\"}"), TEXT("Rewrite references under directory"), TEXT("unsupported"), TEXT("ref rewrite is not implemented yet."))
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

    return Spec.Name == TEXT("prop_set_bool")
        || Spec.Name == TEXT("prop_set_enum")
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
        || Spec.Name == TEXT("var_set_default_int")
        || Spec.Name == TEXT("var_set_default_empty")
        || Spec.Name == TEXT("var_set_default_unicode")
        || Spec.Name == TEXT("var_set_default_long");
}

bool UsesNativeOperationFixtureParent(const FOperationFixtureSpec& Spec)
{
    return Spec.Name == TEXT("prop_set_int")
        || Spec.Name == TEXT("prop_set_bool")
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
        || Spec.Name == TEXT("var_set_default_int")
        || Spec.Name == TEXT("var_set_default_empty")
        || Spec.Name == TEXT("var_set_default_unicode")
        || Spec.Name == TEXT("var_set_default_long")
        || Spec.Name == TEXT("prop_add_fixture_int")
        || Spec.Name == TEXT("prop_remove_fixture_int");
}

bool IsDataTableOperation(const FOperationFixtureSpec& Spec)
{
    return Spec.Name == TEXT("dt_add_row")
        || Spec.Name == TEXT("dt_add_row_values_scalar")
        || Spec.Name == TEXT("dt_add_row_values_mixed")
        || Spec.Name == TEXT("dt_remove_row")
        || Spec.Name == TEXT("dt_remove_row_base");
}

bool IsCompositeDataTableRejectOperation(const FOperationFixtureSpec& Spec)
{
    return Spec.Name == TEXT("dt_add_row_composite_reject")
        || Spec.Name == TEXT("dt_remove_row_composite_reject")
        || Spec.Name == TEXT("dt_update_row_composite_reject");
}

bool ShouldIgnoreSavedHash(const FOperationFixtureSpec& Spec)
{
    return IsSinglePackageOperation(Spec)
        || IsDataTableOperation(Spec)
        || IsCompositeDataTableRejectOperation(Spec);
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
    else if (Spec.Name == TEXT("prop_set_enum"))
    {
        FByteProperty* Prop = FindFProperty<FByteProperty>(Blueprint->GeneratedClass, TEXT("FixtureEnum"));
        FByteProperty* AnchorProp = FindFProperty<FByteProperty>(Blueprint->GeneratedClass, TEXT("FixtureEnumAnchor"));
        FByteProperty* AnchorAltProp = FindFProperty<FByteProperty>(Blueprint->GeneratedClass, TEXT("FixtureEnumAnchorAlt"));
        if (!Prop || !AnchorProp || !AnchorAltProp)
        {
            OutError = TEXT("FixtureEnum enum byte property not found.");
            return false;
        }
        const uint8 Value = static_cast<uint8>(bBefore ? BPXEnum_ValueB : BPXEnum_ValueA);
        Prop->SetPropertyValue_InContainer(CDO, Value);
        AnchorProp->SetPropertyValue_InContainer(CDO, static_cast<uint8>(BPXEnum_ValueB));
        AnchorAltProp->SetPropertyValue_InContainer(CDO, static_cast<uint8>(BPXEnum_ValueA));
    }
    else if (Spec.Name == TEXT("prop_set_enum_numeric"))
    {
        FByteProperty* Prop = FindFProperty<FByteProperty>(Blueprint->GeneratedClass, TEXT("FixtureEnum"));
        if (!Prop)
        {
            OutError = TEXT("FixtureEnum enum byte property not found.");
            return false;
        }
        Prop->SetPropertyValue_InContainer(CDO, static_cast<uint8>(bBefore ? BPXEnum_ValueA : BPXEnum_ValueB));
    }
    else if (Spec.Name == TEXT("prop_set_enum_anchor"))
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
        Prop->SetPropertyValue_InContainer(CDO, static_cast<uint8>(bBefore ? BPXEnum_ValueB : BPXEnum_ValueA));
        AnchorAltProp->SetPropertyValue_InContainer(CDO, static_cast<uint8>(BPXEnum_ValueB));
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
        FByteProperty* EnumValProp = FindFProperty<FByteProperty>(Prop->Struct, TEXT("EnumVal"));
        if (!EnumValProp)
        {
            OutError = TEXT("FixtureCustom.EnumVal field not found.");
            return false;
        }
        EnumValProp->SetPropertyValue_InContainer(StructPtr, static_cast<uint8>(bBefore ? BPXEnum_ValueC : BPXEnum_ValueB));
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
    else
    {
        OutError = FString::Printf(TEXT("Unsupported single-package operation: %s"), *Spec.Name);
        return false;
    }

    CDO->MarkPackageDirty();
    Blueprint->MarkPackageDirty();
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

bool SavePackageToDisk(UPackage* Package, UObject* TopLevelObject, const FString& PackageFilename, FString& OutError)
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
    SaveArgs.TopLevelFlags = RF_Public | RF_Standalone;
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
}

bool WriteOperationSidecars(const FString& OperationDir, const FOperationFixtureSpec& Spec, bool bForce, bool bIncludeSavedHashIgnore, FString& OutError)
{
    IPlatformFile& PlatformFile = FPlatformFileManager::Get().GetPlatformFile();
    PlatformFile.CreateDirectoryTree(*OperationDir);

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
    RootObject->SetStringField(TEXT("notes"), Spec.Notes);

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
        TEXT("# %s\n\nThis operation fixture pair was generated by `BPXGenerateFixtures` commandlet.\n\n- command: `%s`\n- expect: `%s`\n- notes: %s\n- output: `before.uasset`, `after.uasset`, `operation.json`\n"),
        *Spec.Name,
        *Spec.Command,
        *Spec.Expect,
        *Spec.Notes
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

    const FString ParseDir = FPaths::Combine(*BpxRepoRoot, TEXT("testdata"), TEXT("golden"), TEXT("parse"));
    const FString OpsDir = FPaths::Combine(*BpxRepoRoot, TEXT("testdata"), TEXT("golden"), TEXT("operations"));

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
            if (IsIncluded(Spec.Name, IncludeSet))
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
        CheckCollisionIfAny(FPaths::Combine(OpDir, TEXT("after.uasset")), bForce, Conflicts);
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
        const bool bUseDataTableMutation = IsDataTableOperation(Spec);
        const bool bUseCompositeDataTableRejectMutation = IsCompositeDataTableRejectOperation(Spec);
        const FString BeforePackageName = FString::Printf(TEXT("/Game/BPXFixtures/Operations/%s/Before"), *Spec.Name);
        const FString AfterPackageName = FString::Printf(TEXT("/Game/BPXFixtures/Operations/%s/After"), *Spec.Name);
        const FString BeforeSource = FPackageName::LongPackageNameToFilename(BeforePackageName, FPackageName::GetAssetPackageExtension());
        const FString AfterSource = FPackageName::LongPackageNameToFilename(AfterPackageName, FPackageName::GetAssetPackageExtension());
        const FString FixturePackageName = FString::Printf(TEXT("/Game/BPXFixtures/Operations/%s/Fixture"), *Spec.Name);
        const FString FixtureSource = FPackageName::LongPackageNameToFilename(FixturePackageName, FPackageName::GetAssetPackageExtension());
        const FString CompositeParentPackageName = FString::Printf(TEXT("/Game/BPXFixtures/Operations/%s/FixtureParent"), *Spec.Name);
        const FString CompositeParentSource = FPackageName::LongPackageNameToFilename(CompositeParentPackageName, FPackageName::GetAssetPackageExtension());
        const FString OperationDir = FPaths::Combine(OpsDir, Spec.Name);
        const FString BeforeOutput = FPaths::Combine(OperationDir, TEXT("before.uasset"));
        const FString AfterOutput = FPaths::Combine(OperationDir, TEXT("after.uasset"));
        FString ErrorText;

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

                if (!ApplyOperationBlueprintState(FixtureBlueprint, Spec, false, ErrorText))
                {
                    UE_LOG(LogBPXFixtureGenerator, Error, TEXT("Failed to set after-state for %s: %s"), *Spec.Name, *ErrorText);
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
