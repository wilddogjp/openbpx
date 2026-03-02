#include "BPXFixtureGeneratorModule.h"

#include "Misc/MessageDialog.h"
#include "Styling/AppStyle.h"
#include "ToolMenus.h"

#define LOCTEXT_NAMESPACE "FBPXFixtureGeneratorModule"

static void RegisterBPXFixtureMenu()
{
    UToolMenu* ToolsMenu = UToolMenus::Get()->ExtendMenu("LevelEditor.MainMenu.Tools");
    if (!ToolsMenu)
    {
        return;
    }

    FToolMenuSection& Section = ToolsMenu->FindOrAddSection("BPX");
    Section.AddMenuEntry(
        "BPXGenerateFixtures",
        LOCTEXT("BPXGenerateFixturesLabel", "Generate BPX Fixtures"),
        LOCTEXT("BPXGenerateFixturesTooltip", "Run the BPX fixture generator commandlet from command line."),
        FSlateIcon(FAppStyle::GetAppStyleSetName(), "Icons.Play"),
        FUIAction(FExecuteAction::CreateLambda([]()
        {
            FMessageDialog::Open(
                EAppMsgType::Ok,
                LOCTEXT(
                    "BPXGenerateFixturesMessage",
                    "Run UnrealEditor-Cmd with -run=BPXGenerateFixtures to generate fixtures.\n"
                    "The plugin source of truth is wilddog-bpx/testdata/BPXFixtureGenerator."
                )
            );
        }))
    );
}

void FBPXFixtureGeneratorModule::StartupModule()
{
    if (UToolMenus::IsToolMenuUIEnabled())
    {
        ToolMenusHandle = UToolMenus::RegisterStartupCallback(FSimpleMulticastDelegate::FDelegate::CreateStatic(&RegisterBPXFixtureMenu));
    }
}

void FBPXFixtureGeneratorModule::ShutdownModule()
{
    if (UToolMenus::IsToolMenuUIEnabled())
    {
        if (ToolMenusHandle.IsValid())
        {
            UToolMenus::UnRegisterStartupCallback(ToolMenusHandle);
            ToolMenusHandle.Reset();
        }
        UToolMenus::UnregisterOwner(this);
    }
}

#undef LOCTEXT_NAMESPACE

IMPLEMENT_MODULE(FBPXFixtureGeneratorModule, BPXFixtureGenerator)
