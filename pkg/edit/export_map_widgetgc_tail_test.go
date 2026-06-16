package edit

import (
	"encoding/binary"
	"testing"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

func TestRemapWidgetBlueprintGeneratedClassTailRefsOnlyTouchesSuffixExportRefs(t *testing.T) {
	asset := widgetGCTailTestAsset()
	tail := widgetGCTailTestBytes()

	changed := remapWidgetBlueprintGeneratedClassTailRefs(tail, 0, asset, map[int]int{
		4: 104, // Default__Foo_C
		5: 105, // BP_Foo
	}, binary.LittleEndian)
	if !changed {
		t.Fatalf("expected tail remap to change blueprint/CDO refs")
	}

	if got := binary.LittleEndian.Uint32(tail[len(tail)-28 : len(tail)-24]); got != 106 {
		t.Fatalf("blueprint suffix ref: got %d want %d", got, 106)
	}
	if got := binary.LittleEndian.Uint32(tail[len(tail)-4:]); got != 105 {
		t.Fatalf("CDO suffix ref: got %d want %d", got, 105)
	}

	// These neighboring {index,0} pairs are NameRefs for Engine / None and must stay untouched.
	if got := binary.LittleEndian.Uint32(tail[len(tail)-36 : len(tail)-32]); got != 5 {
		t.Fatalf("engine NameRef index was unexpectedly remapped: got %d want %d", got, 5)
	}
	if got := binary.LittleEndian.Uint32(tail[len(tail)-16 : len(tail)-12]); got != 6 {
		t.Fatalf("none NameRef index was unexpectedly remapped: got %d want %d", got, 6)
	}
}

func TestRemapWidgetBlueprintGeneratedClassTailRefsForDeletionOnlyChecksSuffixExportRefs(t *testing.T) {
	asset := widgetGCTailTestAsset()
	tail := widgetGCTailTestBytes()

	changed, err := remapWidgetBlueprintGeneratedClassTailRefsForDeletion(tail, asset, map[int]int{
		4: 104, // Default__Foo_C
		5: 105, // BP_Foo
	}, map[int]bool{
		4: true,
		5: true,
	}, binary.LittleEndian)
	if err != nil {
		t.Fatalf("unexpected deletion remap error: %v", err)
	}
	if !changed {
		t.Fatalf("expected deletion remap to change blueprint/CDO refs")
	}

	if got := binary.LittleEndian.Uint32(tail[len(tail)-28 : len(tail)-24]); got != 106 {
		t.Fatalf("blueprint suffix ref: got %d want %d", got, 106)
	}
	if got := binary.LittleEndian.Uint32(tail[len(tail)-4:]); got != 105 {
		t.Fatalf("CDO suffix ref: got %d want %d", got, 105)
	}
	if got := binary.LittleEndian.Uint32(tail[len(tail)-36 : len(tail)-32]); got != 5 {
		t.Fatalf("engine NameRef index was unexpectedly remapped: got %d want %d", got, 5)
	}
	if got := binary.LittleEndian.Uint32(tail[len(tail)-16 : len(tail)-12]); got != 6 {
		t.Fatalf("none NameRef index was unexpectedly remapped: got %d want %d", got, 6)
	}
}

func widgetGCTailTestAsset() *uasset.Asset {
	names := []uasset.NameEntry{
		{Value: "Dummy"},
		{Value: "Default__Foo_C"},
		{Value: "BP_Foo"},
		{Value: "WidgetBlueprint"},
		{Value: "WidgetBlueprintGeneratedClass"},
		{Value: "Engine"},
		{Value: "None"},
	}
	return &uasset.Asset{
		Names: names,
		Imports: []uasset.ImportEntry{
			{
				ObjectName: uasset.NameRef{Index: 3},
			},
		},
		Exports: []uasset.ExportEntry{
			{},
			{},
			{},
			{},
			{
				ObjectName: uasset.NameRef{Index: 1},
			},
			{
				ClassIndex: uasset.PackageIndex(-1),
				ObjectName: uasset.NameRef{Index: 2},
			},
		},
	}
}

func widgetGCTailTestBytes() []byte {
	tail := make([]byte, 72)
	// fieldCount == 0 at tail[12:16]
	// Suffix layout taken from observed WidgetBlueprintGeneratedClass tails:
	//   len-36: NameRef("Engine")
	//   len-28: int64-like {BlueprintExportIndex,0}
	//   len-16: NameRef("None")
	//   len-4:  CDO export index
	binary.LittleEndian.PutUint32(tail[len(tail)-36:len(tail)-32], 5) // NameMap index "Engine"
	binary.LittleEndian.PutUint32(tail[len(tail)-28:len(tail)-24], 6) // old BP_Foo export index (serialized)
	binary.LittleEndian.PutUint32(tail[len(tail)-16:len(tail)-12], 6) // NameMap index "None"
	binary.LittleEndian.PutUint32(tail[len(tail)-4:], 5)              // old Default__Foo_C export index (serialized)
	return tail
}
