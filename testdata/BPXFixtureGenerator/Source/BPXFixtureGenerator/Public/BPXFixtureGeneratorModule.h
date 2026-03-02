#pragma once

#include "Delegates/Delegate.h"
#include "Modules/ModuleInterface.h"

class FBPXFixtureGeneratorModule : public IModuleInterface
{
public:
    virtual void StartupModule() override;
    virtual void ShutdownModule() override;

private:
    FDelegateHandle ToolMenusHandle;
};
