using UnrealBuildTool;

public class BPXFixtureGenerator : ModuleRules
{
    public BPXFixtureGenerator(ReadOnlyTargetRules Target) : base(Target)
    {
        PCHUsage = PCHUsageMode.UseExplicitOrSharedPCHs;

        PublicDependencyModuleNames.AddRange(
            new string[]
            {
                "Core",
                "CoreUObject",
                "Engine",
                "ToolMenus"
            }
        );

        PrivateDependencyModuleNames.AddRange(
            new string[]
            {
                "AssetRegistry",
                "AssetTools",
                "BlueprintGraph",
                "EditorFramework",
                "Json",
                "JsonUtilities",
                "Kismet",
                "KismetCompiler",
                "Projects",
                "Slate",
                "SlateCore",
                "UnrealEd"
            }
        );
    }
}
