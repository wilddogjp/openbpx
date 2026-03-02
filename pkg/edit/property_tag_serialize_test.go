package edit

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

func TestSerializePropertyTagLegacyObjectProperty(t *testing.T) {
	asset := &uasset.Asset{
		Names: []uasset.NameEntry{
			{Value: "None"},
			{Value: "ObjectProperty"},
			{Value: "MyProp"},
		},
		Summary: uasset.PackageSummary{
			FileVersionUE4: ue4VersionUE56,
			FileVersionUE5: 1006,
		},
	}

	tag := uasset.PropertyTag{
		Name: uasset.NameRef{Index: 2, Number: 0},
		TypeNodes: []uasset.PropertyTypeNode{
			{Name: uasset.NameRef{Index: 1, Number: 0}, InnerCount: 0},
		},
	}
	valueBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(valueBytes, 0)

	got, size, err := serializePropertyTag(asset, tag, valueBytes, nil, binary.LittleEndian)
	if err != nil {
		t.Fatalf("serialize legacy object property: %v", err)
	}
	if got, want := size, int32(4); got != want {
		t.Fatalf("size: got %d want %d", got, want)
	}

	var want bytes.Buffer
	writeNameRef := func(index, number int32) {
		_ = binary.Write(&want, binary.LittleEndian, index)
		_ = binary.Write(&want, binary.LittleEndian, number)
	}
	writeNameRef(2, 0) // property name
	writeNameRef(1, 0) // legacy type
	_ = binary.Write(&want, binary.LittleEndian, int32(4))
	_ = binary.Write(&want, binary.LittleEndian, int32(0)) // array index
	_ = binary.Write(&want, binary.LittleEndian, uint8(0)) // has property guid
	_ = binary.Write(&want, binary.LittleEndian, int32(0)) // value

	if !bytes.Equal(got, want.Bytes()) {
		t.Fatalf("legacy object property tag bytes mismatch")
	}
}

func TestSerializePropertyTagLegacyBoolProperty(t *testing.T) {
	asset := &uasset.Asset{
		Names: []uasset.NameEntry{
			{Value: "None"},
			{Value: "BoolProperty"},
			{Value: "bEnabled"},
		},
		Summary: uasset.PackageSummary{
			FileVersionUE4: ue4VersionUE56,
			FileVersionUE5: 1006,
		},
	}

	tag := uasset.PropertyTag{
		Name: uasset.NameRef{Index: 2, Number: 0},
		TypeNodes: []uasset.PropertyTypeNode{
			{Name: uasset.NameRef{Index: 1, Number: 0}, InnerCount: 0},
		},
	}
	boolValue := true
	got, size, err := serializePropertyTag(asset, tag, nil, &boolValue, binary.LittleEndian)
	if err != nil {
		t.Fatalf("serialize legacy bool property: %v", err)
	}
	if got, want := size, int32(0); got != want {
		t.Fatalf("size: got %d want %d", got, want)
	}

	var want bytes.Buffer
	writeNameRef := func(index, number int32) {
		_ = binary.Write(&want, binary.LittleEndian, index)
		_ = binary.Write(&want, binary.LittleEndian, number)
	}
	writeNameRef(2, 0) // property name
	writeNameRef(1, 0) // legacy type
	_ = binary.Write(&want, binary.LittleEndian, int32(0))
	_ = binary.Write(&want, binary.LittleEndian, int32(0)) // array index
	_ = binary.Write(&want, binary.LittleEndian, uint8(1)) // bool value
	_ = binary.Write(&want, binary.LittleEndian, uint8(0)) // has property guid

	if !bytes.Equal(got, want.Bytes()) {
		t.Fatalf("legacy bool property tag bytes mismatch")
	}
}

func TestSerializePropertyTagLegacyWritesPropertyExtensionsAt1011(t *testing.T) {
	asset := &uasset.Asset{
		Names: []uasset.NameEntry{
			{Value: "None"},
			{Value: "ObjectProperty"},
			{Value: "MyProp"},
		},
		Summary: uasset.PackageSummary{
			FileVersionUE4: ue4VersionUE56,
			FileVersionUE5: 1011,
		},
	}

	tag := uasset.PropertyTag{
		Name:                         uasset.NameRef{Index: 2, Number: 0},
		TypeNodes:                    []uasset.PropertyTypeNode{{Name: uasset.NameRef{Index: 1, Number: 0}, InnerCount: 0}},
		PropertyExtensions:           propertyExtensionOverridableInfo,
		OverridableOperation:         3,
		ExperimentalOverridableLogic: true,
	}
	valueBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(valueBytes, 0)

	got, size, err := serializePropertyTag(asset, tag, valueBytes, nil, binary.LittleEndian)
	if err != nil {
		t.Fatalf("serialize legacy property tag with extensions: %v", err)
	}
	if got, want := size, int32(4); got != want {
		t.Fatalf("size: got %d want %d", got, want)
	}

	var want bytes.Buffer
	writeNameRef := func(index, number int32) {
		_ = binary.Write(&want, binary.LittleEndian, index)
		_ = binary.Write(&want, binary.LittleEndian, number)
	}
	writeNameRef(2, 0) // property name
	writeNameRef(1, 0) // legacy type
	_ = binary.Write(&want, binary.LittleEndian, int32(4))
	_ = binary.Write(&want, binary.LittleEndian, int32(0)) // array index
	_ = binary.Write(&want, binary.LittleEndian, uint8(0)) // has property guid
	_ = binary.Write(&want, binary.LittleEndian, uint8(propertyExtensionOverridableInfo))
	_ = binary.Write(&want, binary.LittleEndian, uint8(3))
	_ = binary.Write(&want, binary.LittleEndian, uint32(1))
	_ = binary.Write(&want, binary.LittleEndian, int32(0)) // value

	if !bytes.Equal(got, want.Bytes()) {
		t.Fatalf("legacy property tag extension bytes mismatch")
	}
}
