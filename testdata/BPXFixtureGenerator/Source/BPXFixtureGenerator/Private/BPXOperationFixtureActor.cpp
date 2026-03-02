#include "BPXOperationFixtureActor.h"

ABPXOperationFixtureActor::ABPXOperationFixtureActor()
{
	FixtureInt = 0;
	FixtureInt64 = 0;
	FixtureFloat = 0.0f;
	FixtureDouble = 0.0;
	FixtureBool = false;
	MyStr = TEXT("CtorDefault");
	FixtureName = FName(TEXT("FixtureNameDefault"));
	FixtureEnum = BPXEnum_ValueC;
	FixtureEnumAnchor = BPXEnum_ValueC;
	FixtureEnumAnchorAlt = BPXEnum_ValueC;
	FixtureVector = FVector::ZeroVector;
	FixtureRotator = FRotator::ZeroRotator;
	FixtureCustom.IntVal = 0;
	FixtureCustom.EnumVal = BPXEnum_ValueA;
}
