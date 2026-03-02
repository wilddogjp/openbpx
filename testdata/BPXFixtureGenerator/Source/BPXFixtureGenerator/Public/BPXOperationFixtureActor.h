#pragma once

#include "CoreMinimal.h"
#include "Engine/DataTable.h"
#include "GameFramework/Actor.h"
#include "BPXOperationFixtureActor.generated.h"

UENUM(BlueprintType)
enum EBPXFixtureEnum : uint8
{
	BPXEnum_ValueA UMETA(DisplayName="ValueA"),
	BPXEnum_ValueB UMETA(DisplayName="ValueB"),
	BPXEnum_ValueC UMETA(DisplayName="ValueC")
};

USTRUCT(BlueprintType)
struct FBPXFixtureCustomStruct
{
	GENERATED_BODY()

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category="BPX")
	int32 IntVal;

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category="BPX")
	TEnumAsByte<EBPXFixtureEnum> EnumVal;

	FBPXFixtureCustomStruct()
		: IntVal(0)
		, EnumVal(BPXEnum_ValueA)
	{
	}
};

USTRUCT(BlueprintType)
struct FBPXOperationTableRow : public FTableRowBase
{
	GENERATED_BODY()

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category="BPX")
	int32 Score;

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category="BPX")
	float Rate;

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category="BPX")
	FString Label;

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category="BPX")
	TEnumAsByte<EBPXFixtureEnum> Mode;

	FBPXOperationTableRow()
		: Score(0)
		, Rate(0.0f)
		, Label(TEXT(""))
		, Mode(BPXEnum_ValueA)
	{
	}
};

UCLASS(BlueprintType)
class BPXFIXTUREGENERATOR_API ABPXOperationFixtureActor : public AActor
{
	GENERATED_BODY()

public:
	ABPXOperationFixtureActor();

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category="BPX")
	int32 FixtureInt;

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category="BPX")
	int64 FixtureInt64;

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category="BPX")
	float FixtureFloat;

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category="BPX")
	double FixtureDouble;

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category="BPX")
	bool FixtureBool;

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category="BPX")
	FString MyStr;

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category="BPX")
	FName FixtureName;

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category="BPX")
	TEnumAsByte<EBPXFixtureEnum> FixtureEnum;

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category="BPX")
	TEnumAsByte<EBPXFixtureEnum> FixtureEnumAnchor;

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category="BPX")
	TEnumAsByte<EBPXFixtureEnum> FixtureEnumAnchorAlt;

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category="BPX")
	FVector FixtureVector;

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category="BPX")
	FRotator FixtureRotator;

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category="BPX")
	TArray<int32> MyArray;

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category="BPX")
	TMap<FString, int32> MyMap;

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category="BPX")
	FBPXFixtureCustomStruct FixtureCustom;
};
