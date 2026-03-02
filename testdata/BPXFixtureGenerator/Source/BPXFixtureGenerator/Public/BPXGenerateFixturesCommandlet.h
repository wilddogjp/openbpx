#pragma once

#include "Commandlets/Commandlet.h"

#include "BPXGenerateFixturesCommandlet.generated.h"

UCLASS()
class UBPXGenerateFixturesCommandlet : public UCommandlet
{
    GENERATED_BODY()

public:
    UBPXGenerateFixturesCommandlet();

    virtual int32 Main(const FString& Params) override;

private:
    bool ValidateWindowsOrUncPath(const FString& InPath) const;
};
