package edit

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

// PropertyAddResult is the computed mutation for one prop add operation.
type PropertyAddResult struct {
	Mutation     ExportMutation
	ExportIndex  int
	PropertyName string
	PropertyType string
	ArrayIndex   int32
	NewValue     any
	NewSize      int32
	ByteDelta    int
}

// PropertyRemoveResult is the computed mutation for one prop remove operation.
type PropertyRemoveResult struct {
	Mutation     ExportMutation
	ExportIndex  int
	PropertyName string
	ArrayIndex   int32
	OldValue     any
	OldSize      int32
	ByteDelta    int
}

type propertyAddSpec struct {
	Name       string
	Type       string
	ArrayIndex int32
	Value      any
}

// BuildPropertyAddMutation builds one export mutation for `bpx prop add`.
func BuildPropertyAddMutation(asset *uasset.Asset, exportIndex int, specJSON string) (*PropertyAddResult, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	if exportIndex < 0 || exportIndex >= len(asset.Exports) {
		return nil, fmt.Errorf("export index out of range: %d", exportIndex+1)
	}

	spec, err := parsePropertyAddSpec(specJSON)
	if err != nil {
		return nil, err
	}

	exp := asset.Exports[exportIndex]
	serialStart := int(exp.SerialOffset)
	serialEnd := int(exp.SerialOffset + exp.SerialSize)
	if serialStart < 0 || serialEnd < serialStart || serialEnd > len(asset.Raw.Bytes) {
		return nil, fmt.Errorf("export serial range out of bounds")
	}
	propertyStart, propertyEnd, withClassControl := exportPropertyBounds(asset, exp)
	if propertyStart < serialStart || propertyEnd > serialEnd || propertyStart > propertyEnd {
		return nil, fmt.Errorf("property range out of bounds")
	}

	parsed := asset.ParseTaggedPropertiesRange(propertyStart, propertyEnd, withClassControl)
	if len(parsed.Warnings) > 0 {
		return nil, fmt.Errorf("cannot safely edit export properties: %s", strings.Join(parsed.Warnings, "; "))
	}
	if parsed.EndOffset < propertyStart+8 {
		return nil, fmt.Errorf("property terminator not found")
	}
	noneStart := parsed.EndOffset - 8

	for _, p := range parsed.Properties {
		if p.Name.Display(asset.Names) == spec.Name && p.ArrayIndex == spec.ArrayIndex {
			return nil, fmt.Errorf("property already exists: %s", formatPropertySelector(spec.Name, spec.ArrayIndex))
		}
	}

	propertyNameRef, err := resolveNameRef(asset, spec.Name)
	if err != nil {
		return nil, err
	}
	rootTypeNode, typeNodes, err := parsePropertyTypeNodesForAdd(asset, spec.Type)
	if err != nil {
		return nil, err
	}

	coerced, err := coerceForType(asset, rootTypeNode.Name, nil, spec.Value)
	if err != nil {
		return nil, fmt.Errorf("coerce value for %s: %w", spec.Name, err)
	}
	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}
	valueBytes, boolValue, err := encodePropertyValue(asset, rootTypeNode, coerced, order)
	if err != nil {
		return nil, fmt.Errorf("encode property %s: %w", spec.Name, err)
	}

	flags := uint8(0)
	if spec.ArrayIndex != 0 {
		flags |= propertyFlagHasArrayIndex
	}
	newTag := uasset.PropertyTag{
		Name:       propertyNameRef,
		TypeNodes:  typeNodes,
		Flags:      flags,
		ArrayIndex: spec.ArrayIndex,
	}
	serializedTag, newSize, err := serializePropertyTag(asset, newTag, valueBytes, boolValue, order)
	if err != nil {
		return nil, fmt.Errorf("serialize property %s: %w", spec.Name, err)
	}

	prefixEnd := noneStart
	if len(parsed.Properties) > 0 {
		prefixEnd = parsed.Properties[0].Offset
	}
	if prefixEnd < propertyStart || noneStart < propertyStart {
		return nil, fmt.Errorf("invalid property bounds while rewriting")
	}

	tagBlob := append([]byte{}, asset.Raw.Bytes[propertyStart:prefixEnd]...)
	for i, p := range parsed.Properties {
		tagStart := p.Offset
		tagEnd := noneStart
		if i+1 < len(parsed.Properties) {
			tagEnd = parsed.Properties[i+1].Offset
		}
		if tagStart < propertyStart || tagEnd < tagStart || tagEnd > noneStart {
			return nil, fmt.Errorf("invalid tag boundaries for property %s", p.Name.Display(asset.Names))
		}
		tagBlob = append(tagBlob, asset.Raw.Bytes[tagStart:tagEnd]...)
	}
	tagBlob = append(tagBlob, serializedTag...)

	noneBytes := asset.Raw.Bytes[noneStart:parsed.EndOffset]
	tagBlob = append(tagBlob, noneBytes...)
	trailing := asset.Raw.Bytes[parsed.EndOffset:propertyEnd]
	newPropertyRegion := make([]byte, 0, len(tagBlob)+len(trailing))
	newPropertyRegion = append(newPropertyRegion, tagBlob...)
	newPropertyRegion = append(newPropertyRegion, trailing...)

	newPayload, err := rewriteExportPayload(asset.Raw.Bytes, serialStart, serialEnd, propertyStart, propertyEnd, newPropertyRegion)
	if err != nil {
		return nil, err
	}

	mutation := ExportMutation{
		ExportIndex: exportIndex,
		Payload:     newPayload,
	}
	if !asset.Summary.UsesUnversionedPropertySerialization() && asset.Summary.FileVersionUE5 >= ue5ScriptSerializationOffset {
		oldStartRel := exp.ScriptSerializationStartOffset
		oldEndRel := exp.ScriptSerializationEndOffset
		if oldEndRel >= oldStartRel {
			rangeStartRel := int64(propertyStart - serialStart)
			rangeEndRel := int64(propertyEnd - serialStart)
			if oldStartRel == rangeStartRel && oldEndRel == rangeEndRel {
				delta := int64(len(newPropertyRegion) - (propertyEnd - propertyStart))
				mutation.UpdateScript = true
				mutation.ScriptStartRel = oldStartRel
				mutation.ScriptEndRel = oldEndRel + delta
			}
		}
	}

	return &PropertyAddResult{
		Mutation:     mutation,
		ExportIndex:  exportIndex,
		PropertyName: spec.Name,
		PropertyType: spec.Type,
		ArrayIndex:   spec.ArrayIndex,
		NewValue:     coerced,
		NewSize:      newSize,
		ByteDelta:    len(newPayload) - (serialEnd - serialStart),
	}, nil
}

// BuildPropertyRemoveMutation builds one export mutation for `bpx prop remove`.
func BuildPropertyRemoveMutation(asset *uasset.Asset, exportIndex int, path string) (*PropertyRemoveResult, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	if exportIndex < 0 || exportIndex >= len(asset.Exports) {
		return nil, fmt.Errorf("export index out of range: %d", exportIndex+1)
	}

	propertyName, arrayIndex, err := parseTopLevelPropertySelector(path)
	if err != nil {
		return nil, err
	}

	exp := asset.Exports[exportIndex]
	serialStart := int(exp.SerialOffset)
	serialEnd := int(exp.SerialOffset + exp.SerialSize)
	if serialStart < 0 || serialEnd < serialStart || serialEnd > len(asset.Raw.Bytes) {
		return nil, fmt.Errorf("export serial range out of bounds")
	}
	propertyStart, propertyEnd, withClassControl := exportPropertyBounds(asset, exp)
	if propertyStart < serialStart || propertyEnd > serialEnd || propertyStart > propertyEnd {
		return nil, fmt.Errorf("property range out of bounds")
	}

	parsed := asset.ParseTaggedPropertiesRange(propertyStart, propertyEnd, withClassControl)
	if len(parsed.Warnings) > 0 {
		return nil, fmt.Errorf("cannot safely edit export properties: %s", strings.Join(parsed.Warnings, "; "))
	}
	if parsed.EndOffset < propertyStart+8 {
		return nil, fmt.Errorf("property terminator not found")
	}
	noneStart := parsed.EndOffset - 8

	targetIdx := -1
	for i, p := range parsed.Properties {
		if p.Name.Display(asset.Names) == propertyName && p.ArrayIndex == arrayIndex {
			if targetIdx >= 0 {
				return nil, fmt.Errorf("path matches multiple properties: %s", formatPropertySelector(propertyName, arrayIndex))
			}
			targetIdx = i
		}
	}
	if targetIdx < 0 {
		return nil, fmt.Errorf("property not found: %s", formatPropertySelector(propertyName, arrayIndex))
	}

	targetTag := parsed.Properties[targetIdx]
	var oldValue any
	if decoded, ok := asset.DecodePropertyValue(targetTag); ok {
		oldValue = decoded
	}

	prefixEnd := noneStart
	if len(parsed.Properties) > 0 {
		prefixEnd = parsed.Properties[0].Offset
	}
	if prefixEnd < propertyStart || noneStart < propertyStart {
		return nil, fmt.Errorf("invalid property bounds while rewriting")
	}

	tagBlob := append([]byte{}, asset.Raw.Bytes[propertyStart:prefixEnd]...)
	for i, p := range parsed.Properties {
		tagStart := p.Offset
		tagEnd := noneStart
		if i+1 < len(parsed.Properties) {
			tagEnd = parsed.Properties[i+1].Offset
		}
		if tagStart < propertyStart || tagEnd < tagStart || tagEnd > noneStart {
			return nil, fmt.Errorf("invalid tag boundaries for property %s", p.Name.Display(asset.Names))
		}
		if i == targetIdx {
			continue
		}
		tagBlob = append(tagBlob, asset.Raw.Bytes[tagStart:tagEnd]...)
	}

	noneBytes := asset.Raw.Bytes[noneStart:parsed.EndOffset]
	tagBlob = append(tagBlob, noneBytes...)
	trailing := asset.Raw.Bytes[parsed.EndOffset:propertyEnd]
	newPropertyRegion := make([]byte, 0, len(tagBlob)+len(trailing))
	newPropertyRegion = append(newPropertyRegion, tagBlob...)
	newPropertyRegion = append(newPropertyRegion, trailing...)

	newPayload, err := rewriteExportPayload(asset.Raw.Bytes, serialStart, serialEnd, propertyStart, propertyEnd, newPropertyRegion)
	if err != nil {
		return nil, err
	}

	mutation := ExportMutation{
		ExportIndex: exportIndex,
		Payload:     newPayload,
	}
	if !asset.Summary.UsesUnversionedPropertySerialization() && asset.Summary.FileVersionUE5 >= ue5ScriptSerializationOffset {
		oldStartRel := exp.ScriptSerializationStartOffset
		oldEndRel := exp.ScriptSerializationEndOffset
		if oldEndRel >= oldStartRel {
			rangeStartRel := int64(propertyStart - serialStart)
			rangeEndRel := int64(propertyEnd - serialStart)
			if oldStartRel == rangeStartRel && oldEndRel == rangeEndRel {
				delta := int64(len(newPropertyRegion) - (propertyEnd - propertyStart))
				mutation.UpdateScript = true
				mutation.ScriptStartRel = oldStartRel
				mutation.ScriptEndRel = oldEndRel + delta
			}
		}
	}

	return &PropertyRemoveResult{
		Mutation:     mutation,
		ExportIndex:  exportIndex,
		PropertyName: propertyName,
		ArrayIndex:   arrayIndex,
		OldValue:     oldValue,
		OldSize:      targetTag.Size,
		ByteDelta:    len(newPayload) - (serialEnd - serialStart),
	}, nil
}

func parsePropertyAddSpec(specJSON string) (*propertyAddSpec, error) {
	dec := json.NewDecoder(strings.NewReader(specJSON))
	dec.UseNumber()

	raw := map[string]any{}
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse --spec JSON: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("parse --spec JSON: trailing tokens are not allowed")
		}
		return nil, fmt.Errorf("parse --spec JSON: %w", err)
	}

	name, _ := raw["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("spec.name is required")
	}
	typeName, _ := raw["type"].(string)
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return nil, fmt.Errorf("spec.type is required")
	}

	value, ok := raw["value"]
	if !ok {
		return nil, fmt.Errorf("spec.value is required")
	}

	arrayIndex := int32(0)
	if v, exists := raw["arrayIndex"]; exists {
		i64, err := asInt64(v)
		if err != nil {
			return nil, fmt.Errorf("invalid spec.arrayIndex: %w", err)
		}
		if i64 < 0 || i64 > int64(^uint32(0)>>1) {
			return nil, fmt.Errorf("spec.arrayIndex out of range: %d", i64)
		}
		arrayIndex = int32(i64)
	}

	return &propertyAddSpec{
		Name:       name,
		Type:       typeName,
		ArrayIndex: arrayIndex,
		Value:      value,
	}, nil
}

func parsePropertyTypeNodesForAdd(asset *uasset.Asset, typeName string) (*typeTreeNode, []uasset.PropertyTypeNode, error) {
	normalized := normalizeTypeName(typeName)
	if normalized == "" {
		return nil, nil, fmt.Errorf("spec.type is required")
	}
	if strings.ContainsAny(typeName, "(),") || normalized != typeName {
		return nil, nil, fmt.Errorf("spec.type currently supports only non-nested types, got %q", typeName)
	}
	typeRef, err := resolveNameRef(asset, normalized)
	if err != nil {
		return nil, nil, err
	}
	return &typeTreeNode{Name: normalized}, []uasset.PropertyTypeNode{
		{Name: typeRef, InnerCount: 0},
	}, nil
}

func resolveNameRef(asset *uasset.Asset, value string) (uasset.NameRef, error) {
	if asset == nil {
		return uasset.NameRef{}, fmt.Errorf("asset is nil")
	}
	idx := findNameIndex(asset.Names, value)
	if idx < 0 {
		return uasset.NameRef{}, fmt.Errorf("name %q is not present in NameMap (prop add does not add names)", value)
	}
	return uasset.NameRef{Index: int32(idx), Number: 0}, nil
}

func parseTopLevelPropertySelector(path string) (string, int32, error) {
	ops, err := parsePath(path)
	if err != nil {
		return "", 0, fmt.Errorf("parse path: %w", err)
	}
	if len(ops) == 0 || ops[0].Kind != pathOpField {
		return "", 0, fmt.Errorf("path must start with a property field")
	}

	name := ops[0].FieldName
	if len(ops) == 1 {
		return name, 0, nil
	}
	if len(ops) == 2 && ops[1].Kind == pathOpSubscript {
		switch v := ops[1].Subscript.(type) {
		case int:
			if v < 0 {
				return "", 0, fmt.Errorf("array index must be non-negative: %d", v)
			}
			return name, int32(v), nil
		default:
			return "", 0, fmt.Errorf("array index must be integer, got %T", v)
		}
	}
	return "", 0, fmt.Errorf("path must reference a top-level property (optional [arrayIndex] only)")
}

func formatPropertySelector(name string, arrayIndex int32) string {
	if arrayIndex <= 0 {
		return name
	}
	return name + "[" + strconv.FormatInt(int64(arrayIndex), 10) + "]"
}

func rewriteExportPayload(raw []byte, serialStart, serialEnd, propertyStart, propertyEnd int, newPropertyRegion []byte) ([]byte, error) {
	if serialStart < 0 || serialEnd < serialStart || serialEnd > len(raw) {
		return nil, fmt.Errorf("export serial range out of bounds")
	}
	if propertyStart < serialStart || propertyEnd < propertyStart || propertyEnd > serialEnd {
		return nil, fmt.Errorf("property range out of bounds")
	}

	relStart := propertyStart - serialStart
	relEnd := propertyEnd - serialStart
	oldPayload := raw[serialStart:serialEnd]
	newPayload := make([]byte, 0, len(oldPayload)+(len(newPropertyRegion)-(propertyEnd-propertyStart)))
	newPayload = append(newPayload, oldPayload[:relStart]...)
	newPayload = append(newPayload, newPropertyRegion...)
	newPayload = append(newPayload, oldPayload[relEnd:]...)
	return newPayload, nil
}
