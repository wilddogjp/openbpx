package edit

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

const (
	propertyFlagHasArrayIndex         = uint8(0x01)
	propertyFlagHasPropertyGUID       = uint8(0x02)
	propertyFlagHasPropertyExtensions = uint8(0x04)
	propertyFlagHasBinaryOrNative     = uint8(0x08)
	propertyFlagBoolTrue              = uint8(0x10)
	propertyFlagSkippedSerialize      = uint8(0x20)
	propertyExtensionOverridableInfo  = uint8(0x02)
)

// PropertySetResult is the computed mutation for one prop set operation.
type PropertySetResult struct {
	Mutation     ExportMutation
	ExportIndex  int
	PropertyName string
	Path         string
	OldValue     any
	NewValue     any
	OldSize      int32
	NewSize      int32
	ByteDelta    int
}

type typeTreeNode struct {
	Name     string
	Children []*typeTreeNode
}

// BuildPropertySetMutation builds one export mutation for `bpx prop set`.
func BuildPropertySetMutation(asset *uasset.Asset, exportIndex int, path string, valueJSON string) (*PropertySetResult, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	if exportIndex < 0 || exportIndex >= len(asset.Exports) {
		return nil, fmt.Errorf("export index out of range: %d", exportIndex+1)
	}

	ops, err := parsePath(path)
	if err != nil {
		return nil, fmt.Errorf("parse path: %w", err)
	}
	if len(ops) == 0 || ops[0].Kind != pathOpField {
		return nil, fmt.Errorf("path must start with a property field")
	}

	var userValue any
	dec := json.NewDecoder(strings.NewReader(valueJSON))
	dec.UseNumber()
	if err := dec.Decode(&userValue); err != nil {
		return nil, fmt.Errorf("parse --value JSON: %w", err)
	}
	if dec.More() {
		return nil, fmt.Errorf("parse --value JSON: trailing tokens are not allowed")
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

	rootName := ops[0].FieldName
	rootIdx := -1
	for i, p := range parsed.Properties {
		if p.Name.Display(asset.Names) == rootName {
			rootIdx = i
			break
		}
	}
	if rootIdx < 0 {
		return nil, fmt.Errorf("property not found: %s", rootName)
	}

	rootTag := parsed.Properties[rootIdx]
	decoded, ok := asset.DecodePropertyValue(rootTag)
	if !ok {
		return nil, fmt.Errorf("property value is not editable for path %q", path)
	}

	typeTree, err := buildTypeTree(rootTag.TypeNodes, asset.Names)
	if err != nil {
		return nil, fmt.Errorf("build type tree for property %s: %w", rootName, err)
	}

	oldLeaf := decoded
	newLeaf := decoded
	updatedDecoded := decoded
	if len(ops) == 1 {
		updatedDecoded, err = coerceForType(asset, typeTree.Name, decoded, userValue)
		if err != nil {
			return nil, fmt.Errorf("coerce value for %s: %w", rootName, err)
		}
		newLeaf = updatedDecoded
	} else {
		updatedDecoded, oldLeaf, newLeaf, err = applyPathMutation(asset, typeTree.Name, decoded, ops[1:], userValue)
		if err != nil {
			return nil, err
		}
	}

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}
	valueBytes, boolValue, err := encodePropertyValue(asset, typeTree, updatedDecoded, order)
	if err != nil {
		return nil, fmt.Errorf("encode property %s: %w", rootName, err)
	}

	if rootTag.Flags&propertyFlagSkippedSerialize != 0 && boolValue == nil {
		return nil, fmt.Errorf("property %s uses skipped serialization and cannot be edited", rootName)
	}
	serializedTag, newSize, err := serializePropertyTag(asset, rootTag, valueBytes, boolValue, order)
	if err != nil {
		return nil, fmt.Errorf("serialize property %s: %w", rootName, err)
	}

	prefixEnd := noneStart
	if len(parsed.Properties) > 0 {
		prefixEnd = parsed.Properties[0].Offset
	}
	if prefixEnd < propertyStart || noneStart < propertyStart {
		return nil, fmt.Errorf("invalid property bounds while rewriting")
	}

	var tagBlob []byte
	tagBlob = append(tagBlob, asset.Raw.Bytes[propertyStart:prefixEnd]...)
	for i, p := range parsed.Properties {
		tagStart := p.Offset
		tagEnd := noneStart
		if i+1 < len(parsed.Properties) {
			tagEnd = parsed.Properties[i+1].Offset
		}
		if tagStart < propertyStart || tagEnd < tagStart || tagEnd > noneStart {
			return nil, fmt.Errorf("invalid tag boundaries for property %s", p.Name.Display(asset.Names))
		}
		if i == rootIdx {
			tagBlob = append(tagBlob, serializedTag...)
		} else {
			tagBlob = append(tagBlob, asset.Raw.Bytes[tagStart:tagEnd]...)
		}
	}

	noneBytes := asset.Raw.Bytes[noneStart:parsed.EndOffset]
	tagBlob = append(tagBlob, noneBytes...)
	trailing := asset.Raw.Bytes[parsed.EndOffset:propertyEnd]
	newPropertyRegion := make([]byte, 0, len(tagBlob)+len(trailing))
	newPropertyRegion = append(newPropertyRegion, tagBlob...)
	newPropertyRegion = append(newPropertyRegion, trailing...)

	relStart := propertyStart - serialStart
	relEnd := propertyEnd - serialStart
	oldPayload := asset.Raw.Bytes[serialStart:serialEnd]
	newPayload := make([]byte, 0, len(oldPayload)+(len(newPropertyRegion)-(propertyEnd-propertyStart)))
	newPayload = append(newPayload, oldPayload[:relStart]...)
	newPayload = append(newPayload, newPropertyRegion...)
	newPayload = append(newPayload, oldPayload[relEnd:]...)

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

	return &PropertySetResult{
		Mutation:     mutation,
		ExportIndex:  exportIndex,
		PropertyName: rootName,
		Path:         path,
		OldValue:     oldLeaf,
		NewValue:     newLeaf,
		OldSize:      rootTag.Size,
		NewSize:      newSize,
		ByteDelta:    len(newPayload) - len(oldPayload),
	}, nil
}

func exportPropertyBounds(asset *uasset.Asset, exp uasset.ExportEntry) (start int, end int, withClassControl bool) {
	start = int(exp.SerialOffset)
	end = int(exp.SerialOffset + exp.SerialSize)
	withClassControl = !asset.Summary.UsesUnversionedPropertySerialization() && asset.Summary.FileVersionUE5 >= ue5PropertyTagExtension
	if exp.ScriptSerializationEndOffset > exp.ScriptSerializationStartOffset &&
		exp.ScriptSerializationStartOffset >= 0 &&
		exp.ScriptSerializationEndOffset <= exp.SerialSize {
		start = int(exp.SerialOffset + exp.ScriptSerializationStartOffset)
		end = int(exp.SerialOffset + exp.ScriptSerializationEndOffset)
	}
	return
}

func buildTypeTree(nodes []uasset.PropertyTypeNode, names []uasset.NameEntry) (*typeTreeNode, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("empty property type nodes")
	}
	idx := 0
	root, err := readTypeTreeNode(nodes, names, &idx)
	if err != nil {
		return nil, err
	}
	if idx != len(nodes) {
		return nil, fmt.Errorf("type nodes trailing items (%d/%d)", idx, len(nodes))
	}
	return root, nil
}

func readTypeTreeNode(nodes []uasset.PropertyTypeNode, names []uasset.NameEntry, idx *int) (*typeTreeNode, error) {
	if *idx >= len(nodes) {
		return nil, fmt.Errorf("type node overflow")
	}
	cur := nodes[*idx]
	*idx = *idx + 1
	n := &typeTreeNode{Name: cur.Name.Display(names)}
	for i := int32(0); i < cur.InnerCount; i++ {
		child, err := readTypeTreeNode(nodes, names, idx)
		if err != nil {
			return nil, err
		}
		n.Children = append(n.Children, child)
	}
	return n, nil
}

func applyPathMutation(asset *uasset.Asset, nodeType string, nodeValue any, ops []pathOp, userValue any) (updated any, oldLeaf any, newLeaf any, err error) {
	if len(ops) == 0 {
		v, err := coerceForType(asset, nodeType, nodeValue, userValue)
		if err != nil {
			return nil, nil, nil, err
		}
		return v, nodeValue, v, nil
	}

	op := ops[0]
	baseType := normalizeTypeName(nodeType)
	switch op.Kind {
	case pathOpField:
		if baseType != "StructProperty" {
			return nil, nil, nil, fmt.Errorf("dot access is only supported on StructProperty, got %s", baseType)
		}
		m, ok := nodeValue.(map[string]any)
		if !ok {
			return nil, nil, nil, fmt.Errorf("invalid struct value representation")
		}
		fieldsRaw, ok := m["value"]
		if !ok {
			return nil, nil, nil, fmt.Errorf("struct has no value payload")
		}
		fields, ok := fieldsRaw.(map[string]any)
		if !ok {
			return nil, nil, nil, fmt.Errorf("struct value payload is not object")
		}
		fieldKey, fieldRaw, exists := findStructField(fields, op.FieldName)
		if !exists {
			return nil, nil, nil, fmt.Errorf("field not found: %s", op.FieldName)
		}

		if wrapper, ok := fieldRaw.(map[string]any); ok {
			childType, _ := wrapper["type"].(string)
			if childType == "" {
				return nil, nil, nil, fmt.Errorf("field %s has no type metadata", op.FieldName)
			}
			childValue := wrapper["value"]
			newChild, oldLeaf, newLeaf, err := applyPathMutation(asset, childType, childValue, ops[1:], userValue)
			if err != nil {
				return nil, nil, nil, err
			}
			wrapper["value"] = newChild
			fields[fieldKey] = wrapper
			m["value"] = fields
			return m, oldLeaf, newLeaf, nil
		}

		structType, _ := m["structType"].(string)
		fieldType := knownStructFieldType(structType, op.FieldName)
		if fieldType == "" {
			return nil, nil, nil, fmt.Errorf("field %s on struct %s is not editable", op.FieldName, structType)
		}
		newChild, oldLeaf, newLeaf, err := applyPathMutation(asset, fieldType, fieldRaw, ops[1:], userValue)
		if err != nil {
			return nil, nil, nil, err
		}
		fields[fieldKey] = newChild
		m["value"] = fields
		return m, oldLeaf, newLeaf, nil

	case pathOpSubscript:
		switch baseType {
		case "ArrayProperty":
			idx, ok := op.Subscript.(int)
			if !ok {
				return nil, nil, nil, fmt.Errorf("array index must be integer")
			}
			m, ok := nodeValue.(map[string]any)
			if !ok {
				return nil, nil, nil, fmt.Errorf("invalid array value representation")
			}
			items, err := toAnySlice(m["value"])
			if err != nil {
				return nil, nil, nil, err
			}
			if idx < 0 || idx >= len(items) {
				return nil, nil, nil, fmt.Errorf("array index out of range: %d (len=%d)", idx, len(items))
			}
			elemType, _ := m["arrayType"].(string)
			itemRaw := items[idx]
			if wrapper, ok := itemRaw.(map[string]any); ok {
				if t, ok := wrapper["type"].(string); ok && t != "" {
					elemType = t
				}
				newChild, oldLeaf, newLeaf, err := applyPathMutation(asset, elemType, wrapper["value"], ops[1:], userValue)
				if err != nil {
					return nil, nil, nil, err
				}
				wrapper["value"] = newChild
				items[idx] = wrapper
				m["value"] = items
				return m, oldLeaf, newLeaf, nil
			}
			newChild, oldLeaf, newLeaf, err := applyPathMutation(asset, elemType, itemRaw, ops[1:], userValue)
			if err != nil {
				return nil, nil, nil, err
			}
			items[idx] = newChild
			m["value"] = items
			return m, oldLeaf, newLeaf, nil

		case "MapProperty":
			m, ok := nodeValue.(map[string]any)
			if !ok {
				return nil, nil, nil, fmt.Errorf("invalid map value representation")
			}
			keyType, _ := m["keyType"].(string)
			if keyType == "" {
				return nil, nil, nil, fmt.Errorf("map has no key type metadata")
			}
			lookupKey, err := coerceForType(asset, keyType, nil, op.Subscript)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("map key coercion failed: %w", err)
			}
			entries, err := toMapEntrySlice(m["value"])
			if err != nil {
				return nil, nil, nil, err
			}
			found := -1
			for i, entry := range entries {
				keyWrapper, ok := entry["key"].(map[string]any)
				if !ok {
					continue
				}
				keyVal, err := coerceForType(asset, keyType, nil, keyWrapper["value"])
				if err != nil {
					continue
				}
				if reflect.DeepEqual(keyVal, lookupKey) {
					found = i
					break
				}
			}
			if found < 0 {
				return nil, nil, nil, fmt.Errorf("map key not found: %v", op.Subscript)
			}

			entry := entries[found]
			valueWrapper, ok := entry["value"].(map[string]any)
			if !ok {
				return nil, nil, nil, fmt.Errorf("map value entry has invalid shape")
			}
			valueType, _ := valueWrapper["type"].(string)
			if valueType == "" {
				return nil, nil, nil, fmt.Errorf("map value entry has no type metadata")
			}
			newChild, oldLeaf, newLeaf, err := applyPathMutation(asset, valueType, valueWrapper["value"], ops[1:], userValue)
			if err != nil {
				return nil, nil, nil, err
			}
			valueWrapper["value"] = newChild
			entry["value"] = valueWrapper
			entries[found] = entry
			m["value"] = entries
			return m, oldLeaf, newLeaf, nil

		default:
			return nil, nil, nil, fmt.Errorf("subscript access is not supported on %s", baseType)
		}
	}
	return nil, nil, nil, fmt.Errorf("unsupported path operation")
}

func coerceForType(asset *uasset.Asset, nodeType string, current any, input any) (any, error) {
	switch normalizeTypeName(nodeType) {
	case "BoolProperty":
		return asBool(input)
	case "Int8Property":
		v, err := asInt64(input)
		if err != nil {
			return nil, err
		}
		if v < math.MinInt8 || v > math.MaxInt8 {
			return nil, fmt.Errorf("int8 overflow: %d", v)
		}
		return int8(v), nil
	case "Int16Property":
		v, err := asInt64(input)
		if err != nil {
			return nil, err
		}
		if v < math.MinInt16 || v > math.MaxInt16 {
			return nil, fmt.Errorf("int16 overflow: %d", v)
		}
		return int16(v), nil
	case "IntProperty":
		v, err := asInt64(input)
		if err != nil {
			return nil, err
		}
		if v < math.MinInt32 || v > math.MaxInt32 {
			return nil, fmt.Errorf("int32 overflow: %d", v)
		}
		return int32(v), nil
	case "Int64Property":
		v, err := asInt64(input)
		if err != nil {
			return nil, err
		}
		return v, nil
	case "UInt16Property":
		v, err := asUint64(input)
		if err != nil {
			return nil, err
		}
		if v > math.MaxUint16 {
			return nil, fmt.Errorf("uint16 overflow: %d", v)
		}
		return uint16(v), nil
	case "UInt32Property":
		v, err := asUint64(input)
		if err != nil {
			return nil, err
		}
		if v > math.MaxUint32 {
			return nil, fmt.Errorf("uint32 overflow: %d", v)
		}
		return uint32(v), nil
	case "UInt64Property":
		v, err := asUint64(input)
		if err != nil {
			return nil, err
		}
		return v, nil
	case "FloatProperty":
		v, err := asFloat64(input)
		if err != nil {
			return nil, err
		}
		return float32(v), nil
	case "DoubleProperty":
		v, err := asFloat64(input)
		if err != nil {
			return nil, err
		}
		return v, nil
	case "StrProperty":
		s, ok := input.(string)
		if !ok {
			return nil, fmt.Errorf("expected string")
		}
		return s, nil
	case "TextProperty":
		return coerceTextProperty(asset, current, input)
	case "NameProperty":
		return coerceNameProperty(asset, input)
	case "EnumProperty":
		currentMap, _ := current.(map[string]any)
		enumType := ""
		if currentMap != nil {
			enumType, _ = currentMap["enumType"].(string)
		}
		if m, ok := input.(map[string]any); ok {
			if rawType, ok := m["enumType"].(string); ok && strings.TrimSpace(rawType) != "" {
				enumType = rawType
			}
			if v, ok := m["value"]; ok {
				input = v
			}
		}
		switch v := input.(type) {
		case string:
			name := strings.TrimSpace(v)
			if name == "" {
				return nil, fmt.Errorf("enum value must not be empty")
			}
			if enumType != "" && !strings.Contains(name, "::") {
				name = enumType + "::" + name
			}
			if resolveEnumNameIndex(asset.Names, enumType, name) < 0 {
				return nil, fmt.Errorf("enum name %q not present in NameMap", name)
			}
			return map[string]any{
				"enumType": enumType,
				"value":    name,
			}, nil
		default:
			ordinal, err := asInt64(v)
			if err != nil {
				return nil, fmt.Errorf("expected enum string/object/integer")
			}
			if ordinal < math.MinInt32 || ordinal > math.MaxInt32 {
				return nil, fmt.Errorf("enum ordinal overflow: %d", ordinal)
			}
			if enumType != "" {
				candidates := enumValueCandidates(asset.Names, enumType)
				if int(ordinal) >= 0 && int(ordinal) < len(candidates) {
					return map[string]any{
						"enumType": enumType,
						"value":    candidates[ordinal],
					}, nil
				}
			}
			return map[string]any{
				"enumType": enumType,
				"value":    int32(ordinal),
			}, nil
		}
	case "StructProperty":
		currentMap, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid current struct shape")
		}
		structType, _ := currentMap["structType"].(string)
		inMap, ok := input.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected object for struct %s", structType)
		}

		switch strings.ToLower(structType) {
		case "vector":
			x, err := structFieldAsFloat64(inMap, "X", "x")
			if err != nil {
				return nil, err
			}
			y, err := structFieldAsFloat64(inMap, "Y", "y")
			if err != nil {
				return nil, err
			}
			z, err := structFieldAsFloat64(inMap, "Z", "z")
			if err != nil {
				return nil, err
			}
			out := cloneAnyMap(currentMap)
			out["value"] = map[string]any{"x": x, "y": y, "z": z}
			return out, nil
		case "rotator":
			pitch, err := structFieldAsFloat64(inMap, "Pitch", "pitch")
			if err != nil {
				return nil, err
			}
			yaw, err := structFieldAsFloat64(inMap, "Yaw", "yaw")
			if err != nil {
				return nil, err
			}
			roll, err := structFieldAsFloat64(inMap, "Roll", "roll")
			if err != nil {
				return nil, err
			}
			out := cloneAnyMap(currentMap)
			out["value"] = map[string]any{"pitch": pitch, "yaw": yaw, "roll": roll}
			return out, nil
		default:
			out := cloneAnyMap(currentMap)
			if raw, ok := inMap["rawBase64"].(string); ok && strings.TrimSpace(raw) != "" {
				out["rawBase64"] = raw
				delete(out, "value")
				return out, nil
			}
			if payload, exists := inMap["value"]; exists {
				out["value"] = payload
				delete(out, "rawBase64")
				return out, nil
			}
			out["value"] = inMap
			delete(out, "rawBase64")
			return out, nil
		}
	case "ObjectProperty", "ClassProperty", "WeakObjectProperty":
		idx, err := asInt64(input)
		if err != nil {
			if m, ok := input.(map[string]any); ok {
				idx, err = asInt64(m["index"])
			}
		}
		if err != nil {
			return nil, fmt.Errorf("expected object index")
		}
		out := map[string]any{"index": int32(idx), "resolved": asset.ParseIndex(uasset.PackageIndex(int32(idx)))}
		return out, nil
	case "ByteProperty":
		if currentMap, ok := current.(map[string]any); ok {
			if enumType, ok := currentMap["enumType"].(string); ok && enumType != "" {
				switch v := input.(type) {
				case string:
					name := v
					if !strings.Contains(name, "::") {
						name = enumType + "::" + name
					}
					return map[string]any{
						"enumType": enumType,
						"value":    name,
					}, nil
				case json.Number, float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
					n, err := asUint64(v)
					if err != nil {
						return nil, err
					}
					if n > math.MaxUint8 {
						return nil, fmt.Errorf("enum byte overflow: %d", n)
					}
					return map[string]any{
						"enumType": enumType,
						"value":    int32(n),
					}, nil
				}
			}
		}
		if m, ok := input.(map[string]any); ok {
			if _, hasEnumType := m["enumType"]; hasEnumType {
				return m, nil
			}
			if v, ok := m["value"]; ok {
				return coerceForType(asset, "ByteProperty", current, v)
			}
		}
		v, err := asUint64(input)
		if err != nil {
			return nil, err
		}
		if v > math.MaxUint8 {
			return nil, fmt.Errorf("byte overflow: %d", v)
		}
		return uint8(v), nil
	case "ArrayProperty":
		items, err := toAnySlice(input)
		if err != nil {
			return nil, fmt.Errorf("array replacement requires JSON array")
		}
		currentMap, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid current array shape")
		}
		elemType, _ := currentMap["arrayType"].(string)
		if elemType == "" {
			return nil, fmt.Errorf("array has no element type metadata")
		}
		wrapped := make([]any, 0, len(items))
		for _, item := range items {
			coerced, err := coerceForType(asset, elemType, nil, item)
			if err != nil {
				return nil, err
			}
			wrapped = append(wrapped, map[string]any{"type": elemType, "value": coerced})
		}
		out := map[string]any{}
		for k, v := range currentMap {
			out[k] = v
		}
		out["value"] = wrapped
		return out, nil
	default:
		return nil, fmt.Errorf("type %s is not editable in current update scope", nodeType)
	}
}

func coerceNameProperty(asset *uasset.Asset, input any) (map[string]any, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	switch v := input.(type) {
	case string:
		idx := findNameIndex(asset.Names, v)
		if idx < 0 {
			return nil, fmt.Errorf("name %q is not present in NameMap (current update scope does not add names)", v)
		}
		return map[string]any{"index": int32(idx), "number": int32(0), "name": v}, nil
	case map[string]any:
		if nameRaw, ok := v["name"]; ok {
			if name, ok := nameRaw.(string); ok && name != "" {
				idx := findNameIndex(asset.Names, name)
				if idx < 0 {
					return nil, fmt.Errorf("name %q is not present in NameMap (current update scope does not add names)", name)
				}
				num := int64(0)
				if numberRaw, ok := v["number"]; ok {
					parsed, err := asInt64(numberRaw)
					if err != nil {
						return nil, fmt.Errorf("invalid NameProperty.number: %w", err)
					}
					num = parsed
				}
				return map[string]any{"index": int32(idx), "number": int32(num), "name": name}, nil
			}
		}
		idx, err := asInt64(v["index"])
		if err != nil {
			return nil, fmt.Errorf("invalid NameProperty.index: %w", err)
		}
		num := int64(0)
		if numberRaw, ok := v["number"]; ok {
			parsed, err := asInt64(numberRaw)
			if err != nil {
				return nil, fmt.Errorf("invalid NameProperty.number: %w", err)
			}
			num = parsed
		}
		if idx < 0 || int(idx) >= len(asset.Names) {
			return nil, fmt.Errorf("name index out of range: %d", idx)
		}
		return map[string]any{
			"index":  int32(idx),
			"number": int32(num),
			"name":   asset.Names[idx].Value,
		}, nil
	default:
		return nil, fmt.Errorf("expected string or object for NameProperty")
	}
}

func coerceTextProperty(asset *uasset.Asset, current any, input any) (map[string]any, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	currentMap, ok := current.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid current TextProperty shape")
	}
	out := cloneAnyMap(currentMap)

	historyType, err := textHistoryTypeCodeFromMap(out)
	if err != nil {
		return nil, err
	}
	out["historyTypeCode"] = historyType
	out["historyType"] = textHistoryTypeNameByCode(historyType)

	switch v := input.(type) {
	case string:
		switch historyType {
		case 0: // Base
			out["sourceString"] = v
			out["value"] = v
			out["cultureInvariantString"] = v
		case 255: // None
			out["hasCultureInvariantString"] = true
			out["cultureInvariantString"] = v
			out["value"] = v
		default:
			return nil, fmt.Errorf("TextProperty historyType %s cannot be updated from plain string", textHistoryTypeNameByCode(historyType))
		}
		return out, nil
	case map[string]any:
		if flagsRaw, hasFlags := v["flags"]; hasFlags {
			flags, err := asInt64(flagsRaw)
			if err != nil {
				return nil, fmt.Errorf("invalid TextProperty.flags: %w", err)
			}
			if flags < math.MinInt32 || flags > math.MaxInt32 {
				return nil, fmt.Errorf("TextProperty.flags out of range: %d", flags)
			}
			out["flags"] = int32(flags)
		}
		if historyRaw, hasHistory := v["historyType"]; hasHistory {
			nextType, err := textHistoryTypeCodeFromValue(historyRaw)
			if err != nil {
				return nil, err
			}
			historyType = nextType
		}
		if historyRaw, hasHistory := v["historyTypeCode"]; hasHistory {
			nextType, err := textHistoryTypeCodeFromValue(historyRaw)
			if err != nil {
				return nil, err
			}
			historyType = nextType
		}
		out["historyTypeCode"] = historyType
		out["historyType"] = textHistoryTypeNameByCode(historyType)

		switch historyType {
		case 0: // Base
			if nsRaw, ok := v["namespace"]; ok {
				ns, ok := nsRaw.(string)
				if !ok {
					return nil, fmt.Errorf("TextProperty.namespace must be string")
				}
				out["namespace"] = ns
			}
			if keyRaw, ok := v["key"]; ok {
				key, ok := keyRaw.(string)
				if !ok {
					return nil, fmt.Errorf("TextProperty.key must be string")
				}
				out["key"] = key
			}
			source := ""
			if sourceRaw, ok := v["sourceString"]; ok {
				s, ok := sourceRaw.(string)
				if !ok {
					return nil, fmt.Errorf("TextProperty.sourceString must be string")
				}
				source = s
			} else if valueRaw, ok := v["value"]; ok {
				s, ok := valueRaw.(string)
				if !ok {
					return nil, fmt.Errorf("TextProperty.value must be string")
				}
				source = s
			} else if invariantRaw, ok := v["cultureInvariantString"]; ok {
				s, ok := invariantRaw.(string)
				if !ok {
					return nil, fmt.Errorf("TextProperty.cultureInvariantString must be string")
				}
				source = s
			}
			if source != "" || v["sourceString"] != nil || v["value"] != nil || v["cultureInvariantString"] != nil {
				out["sourceString"] = source
				out["value"] = source
				out["cultureInvariantString"] = source
			}
			delete(out, "tableId")
			delete(out, "tableIdName")
		case 11: // StringTableEntry
			tableIDName, _ := out["tableIdName"].(string)
			if tableRaw, ok := v["tableIdName"]; ok {
				s, ok := tableRaw.(string)
				if !ok {
					return nil, fmt.Errorf("TextProperty.tableIdName must be string")
				}
				tableIDName = s
			}
			if tableRaw, ok := v["table"]; ok {
				s, ok := tableRaw.(string)
				if !ok {
					return nil, fmt.Errorf("TextProperty.table must be string")
				}
				tableIDName = s
			}
			if tableIDName == "" {
				return nil, fmt.Errorf("TextProperty StringTableEntry requires tableIdName")
			}
			tableRef, err := coerceNameProperty(asset, map[string]any{"name": tableIDName})
			if err != nil {
				return nil, err
			}
			key, _ := out["key"].(string)
			if keyRaw, ok := v["key"]; ok {
				s, ok := keyRaw.(string)
				if !ok {
					return nil, fmt.Errorf("TextProperty.key must be string")
				}
				key = s
			}
			out["tableIdName"] = tableIDName
			out["tableId"] = tableRef
			out["key"] = key
			delete(out, "namespace")
			delete(out, "sourceString")
			delete(out, "cultureInvariantString")
			delete(out, "value")
		case 255: // None
			hasInvariant := false
			if raw, ok := v["hasCultureInvariantString"]; ok {
				b, err := asBool(raw)
				if err != nil {
					return nil, fmt.Errorf("TextProperty.hasCultureInvariantString: %w", err)
				}
				hasInvariant = b
			}
			invariant := ""
			if raw, ok := v["cultureInvariantString"]; ok {
				s, ok := raw.(string)
				if !ok {
					return nil, fmt.Errorf("TextProperty.cultureInvariantString must be string")
				}
				invariant = s
				hasInvariant = true
			}
			out["hasCultureInvariantString"] = hasInvariant
			if hasInvariant {
				out["cultureInvariantString"] = invariant
				out["value"] = invariant
			} else {
				delete(out, "cultureInvariantString")
				delete(out, "value")
			}
			delete(out, "namespace")
			delete(out, "key")
			delete(out, "tableId")
			delete(out, "tableIdName")
			delete(out, "sourceString")
		default:
			return nil, fmt.Errorf("TextProperty historyType %s is not editable in current update scope", textHistoryTypeNameByCode(historyType))
		}
		return out, nil
	default:
		return nil, fmt.Errorf("expected string or object for TextProperty")
	}
}

func textHistoryTypeCodeFromMap(m map[string]any) (int32, error) {
	if m == nil {
		return 0, fmt.Errorf("TextProperty value is nil")
	}
	if raw, ok := m["historyTypeCode"]; ok {
		return textHistoryTypeCodeFromValue(raw)
	}
	if raw, ok := m["historyType"]; ok {
		return textHistoryTypeCodeFromValue(raw)
	}
	return 0, nil
}

func textHistoryTypeCodeFromValue(raw any) (int32, error) {
	switch v := raw.(type) {
	case string:
		if code, ok := textHistoryTypeCodeByName(v); ok {
			return code, nil
		}
		return 0, fmt.Errorf("unsupported TextProperty historyType: %s", v)
	default:
		n, err := asInt64(v)
		if err != nil {
			return 0, fmt.Errorf("invalid TextProperty historyType: %w", err)
		}
		if n < 0 {
			if n == -1 {
				return 255, nil
			}
			return 0, fmt.Errorf("invalid TextProperty historyType code: %d", n)
		}
		if n > math.MaxUint8 {
			return 0, fmt.Errorf("invalid TextProperty historyType code: %d", n)
		}
		return int32(n), nil
	}
}

func textHistoryTypeCodeByName(name string) (int32, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "none":
		return 255, true
	case "base":
		return 0, true
	case "namedformat":
		return 1, true
	case "orderedformat":
		return 2, true
	case "argumentformat":
		return 3, true
	case "asnumber":
		return 4, true
	case "aspercent":
		return 5, true
	case "ascurrency":
		return 6, true
	case "asdate":
		return 7, true
	case "astime":
		return 8, true
	case "asdatetime":
		return 9, true
	case "transform":
		return 10, true
	case "stringtableentry":
		return 11, true
	case "textgenerator":
		return 12, true
	default:
		return 0, false
	}
}

func textHistoryTypeNameByCode(code int32) string {
	switch code {
	case 255:
		return "None"
	case 0:
		return "Base"
	case 1:
		return "NamedFormat"
	case 2:
		return "OrderedFormat"
	case 3:
		return "ArgumentFormat"
	case 4:
		return "AsNumber"
	case 5:
		return "AsPercent"
	case 6:
		return "AsCurrency"
	case 7:
		return "AsDate"
	case 8:
		return "AsTime"
	case 9:
		return "AsDateTime"
	case 10:
		return "Transform"
	case 11:
		return "StringTableEntry"
	case 12:
		return "TextGenerator"
	default:
		return fmt.Sprintf("Unknown(%d)", code)
	}
}

func findNameIndex(names []uasset.NameEntry, needle string) int {
	for i, n := range names {
		if n.Value == needle {
			return i
		}
	}
	return -1
}

func resolveEnumNameIndex(names []uasset.NameEntry, enumType string, rawName string) int {
	rawName = strings.TrimSpace(rawName)
	if rawName == "" {
		return -1
	}
	candidates := make([]string, 0, 3)
	candidates = append(candidates, rawName)
	if strings.Contains(rawName, "::") {
		parts := strings.SplitN(rawName, "::", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
			candidates = append(candidates, strings.TrimSpace(parts[1]))
		}
	} else if trimmedEnumType := strings.TrimSpace(enumType); trimmedEnumType != "" {
		candidates = append(candidates, trimmedEnumType+"::"+rawName)
	}
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if idx := findNameIndex(names, candidate); idx >= 0 {
			return idx
		}
	}
	return -1
}

func enumValueCandidates(names []uasset.NameEntry, enumType string) []string {
	enumType = strings.TrimSpace(enumType)
	if enumType == "" {
		return nil
	}
	prefix := enumType + "::"
	out := make([]string, 0, 16)
	seen := map[string]struct{}{}
	for _, n := range names {
		value := strings.TrimSpace(n.Value)
		if !strings.HasPrefix(value, prefix) {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizeTypeName(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return typeName
	}
	if idx := strings.IndexByte(typeName, '('); idx >= 0 {
		return typeName[:idx]
	}
	return typeName
}

func findStructField(fields map[string]any, field string) (string, any, bool) {
	if fields == nil {
		return "", nil, false
	}
	if v, ok := fields[field]; ok {
		return field, v, true
	}

	needle := strings.ToLower(strings.TrimSpace(field))
	for k, v := range fields {
		if strings.ToLower(strings.TrimSpace(k)) == needle {
			return k, v, true
		}
	}
	return "", nil, false
}

func structFieldAsFloat64(fields map[string]any, keys ...string) (float64, error) {
	for _, key := range keys {
		if _, raw, ok := findStructField(fields, key); ok {
			return asFloat64(raw)
		}
	}
	return 0, fmt.Errorf("missing struct field (%s)", strings.Join(keys, "/"))
}

func structFieldAsInt32(fields map[string]any, keys ...string) (int32, error) {
	for _, key := range keys {
		if _, raw, ok := findStructField(fields, key); ok {
			v, err := asInt64(raw)
			if err != nil {
				return 0, err
			}
			if v < math.MinInt32 || v > math.MaxInt32 {
				return 0, fmt.Errorf("int32 overflow: %d", v)
			}
			return int32(v), nil
		}
	}
	return 0, fmt.Errorf("missing struct field (%s)", strings.Join(keys, "/"))
}

func cloneAnyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func knownStructFieldType(structType, field string) string {
	switch strings.ToLower(strings.TrimSpace(structType)) {
	case "vector":
		switch strings.ToLower(strings.TrimSpace(field)) {
		case "x", "y", "z":
			return "DoubleProperty"
		}
	case "vector3d":
		switch strings.ToLower(strings.TrimSpace(field)) {
		case "x", "y", "z":
			return "DoubleProperty"
		}
	case "rotator":
		switch strings.ToLower(strings.TrimSpace(field)) {
		case "pitch", "yaw", "roll", "x", "y", "z":
			return "DoubleProperty"
		}
	case "rotator3d":
		switch strings.ToLower(strings.TrimSpace(field)) {
		case "pitch", "yaw", "roll", "x", "y", "z":
			return "DoubleProperty"
		}
	case "levelviewportinfo":
		switch strings.ToLower(strings.TrimSpace(field)) {
		case "camposition":
			return "StructProperty(Vector)"
		case "camrotation":
			return "StructProperty(Rotator)"
		case "camorthozoom":
			return "FloatProperty"
		case "camupdated":
			return "BoolProperty"
		}
	case "vector2d":
		switch strings.ToLower(strings.TrimSpace(field)) {
		case "x", "y":
			return "DoubleProperty"
		}
	case "vector4", "quat", "plane", "vector4d", "quat4d", "plane4d":
		switch strings.ToLower(strings.TrimSpace(field)) {
		case "x", "y", "z", "w":
			return "DoubleProperty"
		}
	case "vector4f", "quat4f", "plane4f":
		switch strings.ToLower(strings.TrimSpace(field)) {
		case "x", "y", "z", "w":
			return "FloatProperty"
		}
	case "framenumber":
		if strings.EqualFold(strings.TrimSpace(field), "value") {
			return "IntProperty"
		}
	}
	return ""
}

func encodePropertyValue(asset *uasset.Asset, root *typeTreeNode, decoded any, order binary.ByteOrder) ([]byte, *bool, error) {
	if root == nil {
		return nil, nil, fmt.Errorf("root type is nil")
	}
	if normalizeTypeName(root.Name) == "BoolProperty" {
		v, err := asBool(decoded)
		if err != nil {
			return nil, nil, err
		}
		return nil, &v, nil
	}
	b, err := encodeValueByType(asset, root, decoded, order)
	if err != nil {
		return nil, nil, err
	}
	return b, nil, nil
}

func encodeValueByType(asset *uasset.Asset, node *typeTreeNode, value any, order binary.ByteOrder) ([]byte, error) {
	w := newByteWriter(order, 64)
	if err := writeValueByType(asset, w, node, value); err != nil {
		return nil, err
	}
	return w.bytes(), nil
}

func writeValueByType(asset *uasset.Asset, w *byteWriter, node *typeTreeNode, value any) error {
	switch normalizeTypeName(node.Name) {
	case "BoolProperty":
		v, err := asBool(value)
		if err != nil {
			return err
		}
		if v {
			w.writeUint8(1)
		} else {
			w.writeUint8(0)
		}
		return nil
	case "Int8Property":
		v, err := asInt64(value)
		if err != nil {
			return err
		}
		if v < math.MinInt8 || v > math.MaxInt8 {
			return fmt.Errorf("int8 overflow: %d", v)
		}
		w.writeUint8(uint8(int8(v)))
		return nil
	case "Int16Property":
		v, err := asInt64(value)
		if err != nil {
			return err
		}
		if v < math.MinInt16 || v > math.MaxInt16 {
			return fmt.Errorf("int16 overflow: %d", v)
		}
		var b [2]byte
		w.order.PutUint16(b[:], uint16(int16(v)))
		w.writeBytes(b[:])
		return nil
	case "UInt16Property":
		v, err := asUint64(value)
		if err != nil {
			return err
		}
		if v > math.MaxUint16 {
			return fmt.Errorf("uint16 overflow: %d", v)
		}
		var b [2]byte
		w.order.PutUint16(b[:], uint16(v))
		w.writeBytes(b[:])
		return nil
	case "IntProperty":
		v, err := asInt64(value)
		if err != nil {
			return err
		}
		if v < math.MinInt32 || v > math.MaxInt32 {
			return fmt.Errorf("int32 overflow: %d", v)
		}
		w.writeInt32(int32(v))
		return nil
	case "UInt32Property":
		v, err := asUint64(value)
		if err != nil {
			return err
		}
		if v > math.MaxUint32 {
			return fmt.Errorf("uint32 overflow: %d", v)
		}
		w.writeUint32(uint32(v))
		return nil
	case "Int64Property":
		v, err := asInt64(value)
		if err != nil {
			return err
		}
		w.writeInt64(v)
		return nil
	case "UInt64Property":
		v, err := asUint64(value)
		if err != nil {
			return err
		}
		w.writeInt64(int64(v))
		return nil
	case "FloatProperty":
		v, err := asFloat64(value)
		if err != nil {
			return err
		}
		bits := math.Float32bits(float32(v))
		w.writeUint32(bits)
		return nil
	case "DoubleProperty":
		v, err := asFloat64(value)
		if err != nil {
			return err
		}
		w.writeInt64(int64(math.Float64bits(v)))
		return nil
	case "StrProperty":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string")
		}
		w.writeFString(s)
		return nil
	case "TextProperty":
		return writeTextProperty(asset, w, value)
	case "NameProperty":
		m, err := coerceNameProperty(asset, value)
		if err != nil {
			return err
		}
		idx, _ := m["index"].(int32)
		num, _ := m["number"].(int32)
		w.writeNameRef(idx, num)
		return nil
	case "EnumProperty":
		coerced, err := coerceForType(asset, "EnumProperty", value, value)
		if err != nil {
			return err
		}
		m, ok := coerced.(map[string]any)
		if !ok {
			return fmt.Errorf("enum value must be object")
		}
		enumType, _ := m["enumType"].(string)
		switch raw := m["value"].(type) {
		case string:
			idx := resolveEnumNameIndex(asset.Names, enumType, raw)
			if idx < 0 {
				return fmt.Errorf("enum name %q not present in NameMap", raw)
			}
			w.writeNameRef(int32(idx), 0)
			return nil
		default:
			n, err := asInt64(raw)
			if err != nil {
				return fmt.Errorf("enum value must be string or integer")
			}
			w.writeInt32(int32(n))
			return nil
		}
	case "ObjectProperty", "ClassProperty", "WeakObjectProperty":
		obj, err := coerceForType(asset, node.Name, nil, value)
		if err != nil {
			return err
		}
		m, _ := obj.(map[string]any)
		idx, _ := m["index"].(int32)
		w.writeInt32(idx)
		return nil
	case "ByteProperty":
		if m, ok := value.(map[string]any); ok {
			enumType, _ := m["enumType"].(string)
			if raw, ok := m["value"]; ok {
				if s, ok := raw.(string); ok {
					idx := resolveEnumNameIndex(asset.Names, enumType, s)
					if idx < 0 {
						return fmt.Errorf("enum name %q not present in NameMap", s)
					}
					w.writeNameRef(int32(idx), 0)
					return nil
				}
				value = raw
			}
		}
		v, err := asUint64(value)
		if err != nil {
			return err
		}
		if v > math.MaxUint8 {
			return fmt.Errorf("byte overflow: %d", v)
		}
		w.writeUint8(uint8(v))
		return nil
	case "ArrayProperty":
		if len(node.Children) < 1 {
			return fmt.Errorf("array type has no child type")
		}
		m, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("array value must be object with value[]")
		}
		items, err := toAnySlice(m["value"])
		if err != nil {
			return err
		}
		if len(items) > math.MaxInt32 {
			return fmt.Errorf("array too large: %d", len(items))
		}
		w.writeInt32(int32(len(items)))
		for _, item := range items {
			itemVal := item
			if wrapper, ok := item.(map[string]any); ok {
				if v, ok := wrapper["value"]; ok {
					itemVal = v
				}
			}
			if err := writeValueByType(asset, w, node.Children[0], itemVal); err != nil {
				return err
			}
		}
		return nil
	case "MapProperty":
		if len(node.Children) < 2 {
			return fmt.Errorf("map type has insufficient child types")
		}
		m, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("map value must be object")
		}
		overridable, _ := m["overridable"].(bool)
		if overridable {
			// UE5.6 overridable map delta format.
			w.writeInt32(-1)
			removed, _ := toAnySlice(m["removed"])
			w.writeInt32(int32(len(removed)))
			for _, item := range removed {
				itemVal := extractWrappedValue(item)
				if err := writeValueByType(asset, w, node.Children[0], itemVal); err != nil {
					return err
				}
			}
			if err := writeMapEntryList(asset, w, node.Children[0], node.Children[1], m["modified"]); err != nil {
				return err
			}
			if asset.Summary.FileVersionUE5 >= ue5OSSubObjectShadowSerialization {
				if err := writeMapEntryList(asset, w, node.Children[0], node.Children[1], m["shadowed"]); err != nil {
					return err
				}
			}
			if err := writeMapEntryList(asset, w, node.Children[0], node.Children[1], m["added"]); err != nil {
				return err
			}
			return nil
		}

		replaceMap, _ := m["replaceMap"].(bool)
		if replaceMap {
			w.writeInt32(-1)
		} else {
			removed, _ := toAnySlice(m["removed"])
			w.writeInt32(int32(len(removed)))
			for _, item := range removed {
				if err := writeValueByType(asset, w, node.Children[0], extractWrappedValue(item)); err != nil {
					return err
				}
			}
		}

		entries, err := toMapEntrySlice(m["value"])
		if err != nil {
			return err
		}
		w.writeInt32(int32(len(entries)))
		for _, entry := range entries {
			keyVal := extractWrappedValue(entry["key"])
			valVal := extractWrappedValue(entry["value"])
			if err := writeValueByType(asset, w, node.Children[0], keyVal); err != nil {
				return err
			}
			if err := writeValueByType(asset, w, node.Children[1], valVal); err != nil {
				return err
			}
		}
		return nil
	case "StructProperty":
		m, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("struct value must be object")
		}
		if rawB64, ok := m["rawBase64"].(string); ok && rawB64 != "" {
			raw, err := base64.StdEncoding.DecodeString(rawB64)
			if err != nil {
				return fmt.Errorf("decode rawBase64: %w", err)
			}
			w.writeBytes(raw)
			return nil
		}
		structType, _ := m["structType"].(string)
		fields, ok := m["value"].(map[string]any)
		if !ok {
			return fmt.Errorf("struct value payload missing")
		}
		switch strings.ToLower(strings.TrimSpace(structType)) {
		case "vector", "vector3d":
			x, err := structFieldAsFloat64(fields, "X", "x")
			if err != nil {
				return err
			}
			y, err := structFieldAsFloat64(fields, "Y", "y")
			if err != nil {
				return err
			}
			z, err := structFieldAsFloat64(fields, "Z", "z")
			if err != nil {
				return err
			}
			w.writeInt64(int64(math.Float64bits(x)))
			w.writeInt64(int64(math.Float64bits(y)))
			w.writeInt64(int64(math.Float64bits(z)))
			return nil
		case "rotator", "rotator3d":
			pitch, err := structFieldAsFloat64(fields, "Pitch", "pitch", "X", "x")
			if err != nil {
				return err
			}
			yaw, err := structFieldAsFloat64(fields, "Yaw", "yaw", "Y", "y")
			if err != nil {
				return err
			}
			roll, err := structFieldAsFloat64(fields, "Roll", "roll", "Z", "z")
			if err != nil {
				return err
			}
			w.writeInt64(int64(math.Float64bits(pitch)))
			w.writeInt64(int64(math.Float64bits(yaw)))
			w.writeInt64(int64(math.Float64bits(roll)))
			return nil
		case "vector2d":
			x, err := structFieldAsFloat64(fields, "X", "x")
			if err != nil {
				return err
			}
			y, err := structFieldAsFloat64(fields, "Y", "y")
			if err != nil {
				return err
			}
			w.writeInt64(int64(math.Float64bits(x)))
			w.writeInt64(int64(math.Float64bits(y)))
			return nil
		case "vector4", "quat", "plane", "vector4d", "quat4d", "plane4d":
			x, err := structFieldAsFloat64(fields, "X", "x")
			if err != nil {
				return err
			}
			y, err := structFieldAsFloat64(fields, "Y", "y")
			if err != nil {
				return err
			}
			z, err := structFieldAsFloat64(fields, "Z", "z")
			if err != nil {
				return err
			}
			wv, err := structFieldAsFloat64(fields, "W", "w")
			if err != nil {
				return err
			}
			w.writeInt64(int64(math.Float64bits(x)))
			w.writeInt64(int64(math.Float64bits(y)))
			w.writeInt64(int64(math.Float64bits(z)))
			w.writeInt64(int64(math.Float64bits(wv)))
			return nil
		case "vector2f":
			x, err := structFieldAsFloat64(fields, "X", "x")
			if err != nil {
				return err
			}
			y, err := structFieldAsFloat64(fields, "Y", "y")
			if err != nil {
				return err
			}
			w.writeUint32(math.Float32bits(float32(x)))
			w.writeUint32(math.Float32bits(float32(y)))
			return nil
		case "vector3f":
			x, err := structFieldAsFloat64(fields, "X", "x")
			if err != nil {
				return err
			}
			y, err := structFieldAsFloat64(fields, "Y", "y")
			if err != nil {
				return err
			}
			z, err := structFieldAsFloat64(fields, "Z", "z")
			if err != nil {
				return err
			}
			w.writeUint32(math.Float32bits(float32(x)))
			w.writeUint32(math.Float32bits(float32(y)))
			w.writeUint32(math.Float32bits(float32(z)))
			return nil
		case "vector4f", "quat4f", "plane4f":
			x, err := structFieldAsFloat64(fields, "X", "x")
			if err != nil {
				return err
			}
			y, err := structFieldAsFloat64(fields, "Y", "y")
			if err != nil {
				return err
			}
			z, err := structFieldAsFloat64(fields, "Z", "z")
			if err != nil {
				return err
			}
			wv, err := structFieldAsFloat64(fields, "W", "w")
			if err != nil {
				return err
			}
			w.writeUint32(math.Float32bits(float32(x)))
			w.writeUint32(math.Float32bits(float32(y)))
			w.writeUint32(math.Float32bits(float32(z)))
			w.writeUint32(math.Float32bits(float32(wv)))
			return nil
		case "framenumber":
			v, err := structFieldAsInt32(fields, "Value", "value")
			if err != nil {
				return err
			}
			w.writeInt32(v)
			return nil
		case "framerate":
			numerator, err := structFieldAsInt32(fields, "Numerator", "numerator")
			if err != nil {
				return err
			}
			denominator, err := structFieldAsInt32(fields, "Denominator", "denominator")
			if err != nil {
				return err
			}
			w.writeInt32(numerator)
			w.writeInt32(denominator)
			return nil
		case "softobjectpath", "softclasspath":
			return writeSoftObjectPathStruct(asset, w, fields)
		case "perqualitylevelint":
			return writePerQualityLevelStruct(asset, w, fields, false)
		case "perqualitylevelfloat":
			return writePerQualityLevelStruct(asset, w, fields, true)
		case "perplatformint":
			return writePerPlatformStruct(asset, w, fields, perPlatformValueInt)
		case "perplatformfloat":
			return writePerPlatformStruct(asset, w, fields, perPlatformValueFloat)
		case "perplatformframerate":
			return writePerPlatformStruct(asset, w, fields, perPlatformValueFrameRate)
		case "uniquenetidrepl":
			return writeUniqueNetIdReplStruct(asset, w, fields)
		case "remoteobjectreference":
			return writeRemoteObjectReferenceStruct(w, fields)
		case "animationattributeidentifier":
			return writeAnimationAttributeIdentifierStruct(asset, w, fields)
		case "levelviewportinfo":
			return writeLevelViewportInfoStruct(asset, w, fields)
		default:
			if err := writeTaggedStructFromFields(asset, w, fields); err == nil {
				return nil
			} else {
				return fmt.Errorf("struct type %s is not editable in current update scope: %w", structType, err)
			}
		}
	default:
		return fmt.Errorf("type %s is not editable in current update scope", node.Name)
	}
}

func writeTextProperty(asset *uasset.Asset, w *byteWriter, value any) error {
	if asset == nil {
		return fmt.Errorf("asset is nil")
	}
	m, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("TextProperty value must be object")
	}

	flags := int64(0)
	if raw, ok := m["flags"]; ok {
		v, err := asInt64(raw)
		if err != nil {
			return fmt.Errorf("invalid TextProperty.flags: %w", err)
		}
		if v < math.MinInt32 || v > math.MaxInt32 {
			return fmt.Errorf("TextProperty.flags out of range: %d", v)
		}
		flags = v
	}
	w.writeInt32(int32(flags))

	historyType, err := textHistoryTypeCodeFromMap(m)
	if err != nil {
		return err
	}
	if historyType < 0 || historyType > math.MaxUint8 {
		return fmt.Errorf("invalid TextProperty historyType code: %d", historyType)
	}
	w.writeUint8(uint8(historyType))

	switch historyType {
	case 255: // None
		hasInvariant := false
		if raw, ok := m["hasCultureInvariantString"]; ok {
			b, err := asBool(raw)
			if err != nil {
				return fmt.Errorf("TextProperty.hasCultureInvariantString: %w", err)
			}
			hasInvariant = b
		}
		invariant := ""
		if raw, ok := m["cultureInvariantString"]; ok {
			s, ok := raw.(string)
			if !ok {
				return fmt.Errorf("TextProperty.cultureInvariantString must be string")
			}
			invariant = s
			hasInvariant = true
		}
		w.writeUBool(hasInvariant)
		if hasInvariant {
			w.writeFString(invariant)
		}
		return nil
	case 0: // Base
		namespace, _ := m["namespace"].(string)
		key, _ := m["key"].(string)
		source, _ := m["sourceString"].(string)
		if source == "" {
			if raw, ok := m["value"].(string); ok {
				source = raw
			}
		}
		if source == "" {
			if raw, ok := m["cultureInvariantString"].(string); ok {
				source = raw
			}
		}
		w.writeFString(namespace)
		w.writeFString(key)
		w.writeFString(source)
		return nil
	case 11: // StringTableEntry
		tableIDName, _ := m["tableIdName"].(string)
		if tableIDName == "" {
			if table, ok := m["tableId"].(map[string]any); ok {
				if s, ok := table["name"].(string); ok {
					tableIDName = s
				}
			}
		}
		if tableIDName == "" {
			return fmt.Errorf("TextProperty StringTableEntry requires tableIdName")
		}
		tableRef, err := coerceNameProperty(asset, map[string]any{"name": tableIDName})
		if err != nil {
			return err
		}
		key, _ := m["key"].(string)
		idx, _ := tableRef["index"].(int32)
		num, _ := tableRef["number"].(int32)
		w.writeNameRef(idx, num)
		w.writeFString(key)
		return nil
	default:
		return fmt.Errorf("TextProperty historyType %s is not editable in current update scope", textHistoryTypeNameByCode(historyType))
	}
}

func writeMapEntryList(asset *uasset.Asset, w *byteWriter, keyNode, valueNode *typeTreeNode, raw any) error {
	entries, err := toMapEntrySlice(raw)
	if err != nil {
		return err
	}
	w.writeInt32(int32(len(entries)))
	for _, entry := range entries {
		if err := writeValueByType(asset, w, keyNode, extractWrappedValue(entry["key"])); err != nil {
			return err
		}
		if err := writeValueByType(asset, w, valueNode, extractWrappedValue(entry["value"])); err != nil {
			return err
		}
	}
	return nil
}

func writeLevelViewportInfoStruct(asset *uasset.Asset, w *byteWriter, fields map[string]any) error {
	if asset == nil {
		return fmt.Errorf("asset is nil")
	}
	if fields == nil {
		return fmt.Errorf("LevelViewportInfo fields are missing")
	}

	fieldRef := func(name string) (uasset.NameRef, error) {
		idx := findNameIndex(asset.Names, name)
		if idx < 0 {
			return uasset.NameRef{}, fmt.Errorf("name %q is not present in NameMap (current update scope does not add names)", name)
		}
		return uasset.NameRef{Index: int32(idx), Number: 0}, nil
	}
	valueFor := func(name string) (any, error) {
		_, raw, ok := findStructField(fields, name)
		if !ok {
			return nil, fmt.Errorf("missing struct field (%s)", name)
		}
		if wrapper, ok := raw.(map[string]any); ok {
			if inner, exists := wrapper["value"]; exists {
				return inner, nil
			}
		}
		return raw, nil
	}

	structRef, err := fieldRef("StructProperty")
	if err != nil {
		return err
	}
	vectorRef, err := fieldRef("Vector")
	if err != nil {
		return err
	}
	rotatorRef, err := fieldRef("Rotator")
	if err != nil {
		return err
	}
	floatRef, err := fieldRef("FloatProperty")
	if err != nil {
		return err
	}
	boolRef, err := fieldRef("BoolProperty")
	if err != nil {
		return err
	}

	writeField := func(
		fieldName string,
		typeNodes []uasset.PropertyTypeNode,
		tree *typeTreeNode,
		flags uint8,
	) error {
		nameRef, err := fieldRef(fieldName)
		if err != nil {
			return err
		}
		fieldValue, err := valueFor(fieldName)
		if err != nil {
			return err
		}
		valueBytes, boolValue, err := encodePropertyValue(asset, tree, fieldValue, w.order)
		if err != nil {
			return err
		}
		tagBytes, _, err := serializePropertyTag(asset, uasset.PropertyTag{
			Name:      nameRef,
			TypeNodes: typeNodes,
			Flags:     flags,
		}, valueBytes, boolValue, w.order)
		if err != nil {
			return err
		}
		w.writeBytes(tagBytes)
		return nil
	}

	if err := writeField(
		"CamPosition",
		[]uasset.PropertyTypeNode{
			{Name: structRef, InnerCount: 1},
			{Name: vectorRef, InnerCount: 0},
		},
		&typeTreeNode{
			Name: "StructProperty",
			Children: []*typeTreeNode{
				{Name: "Vector"},
			},
		},
		0,
	); err != nil {
		return err
	}

	if err := writeField(
		"CamRotation",
		[]uasset.PropertyTypeNode{
			{Name: structRef, InnerCount: 1},
			{Name: rotatorRef, InnerCount: 0},
		},
		&typeTreeNode{
			Name: "StructProperty",
			Children: []*typeTreeNode{
				{Name: "Rotator"},
			},
		},
		0,
	); err != nil {
		return err
	}

	if err := writeField(
		"CamOrthoZoom",
		[]uasset.PropertyTypeNode{
			{Name: floatRef, InnerCount: 0},
		},
		&typeTreeNode{Name: "FloatProperty"},
		0,
	); err != nil {
		return err
	}

	if err := writeField(
		"CamUpdated",
		[]uasset.PropertyTypeNode{
			{Name: boolRef, InnerCount: 0},
		},
		&typeTreeNode{Name: "BoolProperty"},
		propertyFlagSkippedSerialize,
	); err != nil {
		return err
	}

	noneRef, err := fieldRef("None")
	if err != nil {
		return err
	}
	w.writeNameRef(noneRef.Index, 0)
	return nil
}

type perPlatformValueKind int

const (
	perPlatformValueInt perPlatformValueKind = iota
	perPlatformValueFloat
	perPlatformValueFrameRate
)

func writeSoftObjectPathStruct(asset *uasset.Asset, w *byteWriter, fields map[string]any) error {
	if asset == nil {
		return fmt.Errorf("asset is nil")
	}
	if fields == nil {
		return fmt.Errorf("soft object path fields are missing")
	}

	if idxRaw, exists := fields["softObjectPathIndex"]; exists {
		idx, err := asInt64(idxRaw)
		if err != nil {
			return fmt.Errorf("softObjectPathIndex must be integer: %w", err)
		}
		if idx < math.MinInt32 || idx > math.MaxInt32 {
			return fmt.Errorf("softObjectPathIndex out of range: %d", idx)
		}
		w.writeInt32(int32(idx))
		return nil
	}

	packageName, ok := fields["packageName"].(string)
	if !ok {
		return fmt.Errorf("SoftObjectPath.packageName must be string")
	}
	assetName, ok := fields["assetName"].(string)
	if !ok {
		return fmt.Errorf("SoftObjectPath.assetName must be string")
	}
	subPath := ""
	if raw, exists := fields["subPath"]; exists {
		if s, ok := raw.(string); ok {
			subPath = s
		} else {
			return fmt.Errorf("SoftObjectPath.subPath must be string")
		}
	}

	if asset.Summary.FileVersionUE5 > 0 && !asset.Summary.SupportsTopLevelAssetPathSoftObjectPath() {
		assetPathName := strings.TrimSpace(packageName)
		if trimmedAsset := strings.TrimSpace(assetName); trimmedAsset != "" {
			if assetPathName == "" {
				assetPathName = trimmedAsset
			} else {
				assetPathName += "." + trimmedAsset
			}
		}
		assetPathRef, err := coerceNameProperty(asset, map[string]any{"name": assetPathName})
		if err != nil {
			return err
		}
		assetPathIdx, _ := assetPathRef["index"].(int32)
		assetPathNum, _ := assetPathRef["number"].(int32)
		w.writeNameRef(assetPathIdx, assetPathNum)
		w.writeFString(subPath)
		return nil
	}

	pkgRef, err := coerceNameProperty(asset, map[string]any{"name": packageName})
	if err != nil {
		return err
	}
	assetRef, err := coerceNameProperty(asset, map[string]any{"name": assetName})
	if err != nil {
		return err
	}
	pkgIdx, _ := pkgRef["index"].(int32)
	pkgNum, _ := pkgRef["number"].(int32)
	assetIdx, _ := assetRef["index"].(int32)
	assetNum, _ := assetRef["number"].(int32)

	w.writeNameRef(pkgIdx, pkgNum)
	w.writeNameRef(assetIdx, assetNum)
	w.writeFString(subPath)
	return nil
}

func writePerPlatformStruct(asset *uasset.Asset, w *byteWriter, fields map[string]any, kind perPlatformValueKind) error {
	if asset == nil {
		return fmt.Errorf("asset is nil")
	}
	if fields == nil {
		return fmt.Errorf("per-platform struct fields are missing")
	}
	bCookedRaw, err := structFieldValue(fields, "bCooked")
	if err != nil {
		return err
	}
	bCooked, err := asBool(bCookedRaw)
	if err != nil {
		return err
	}
	defaultRaw, err := structFieldValue(fields, "Default")
	if err != nil {
		return err
	}

	w.writeUBool(bCooked)
	if err := writePerPlatformValue(w, defaultRaw, kind); err != nil {
		return err
	}

	if bCooked {
		return nil
	}

	perPlatformRaw, err := structFieldValue(fields, "PerPlatform")
	if err != nil {
		return err
	}
	perPlatformMap, ok := perPlatformRaw.(map[string]any)
	if !ok {
		return fmt.Errorf("PerPlatform field must be object")
	}
	entries, err := toMapEntrySlice(perPlatformMap["value"])
	if err != nil {
		return err
	}
	w.writeInt32(int32(len(entries)))
	for _, entry := range entries {
		keyValue := extractWrappedValue(entry["key"])
		keyName, err := coerceNameProperty(asset, keyValue)
		if err != nil {
			return err
		}
		keyIndex, _ := keyName["index"].(int32)
		keyNumber, _ := keyName["number"].(int32)
		w.writeNameRef(keyIndex, keyNumber)
		if err := writePerPlatformValue(w, extractWrappedValue(entry["value"]), kind); err != nil {
			return err
		}
	}
	return nil
}

func writePerPlatformValue(w *byteWriter, raw any, kind perPlatformValueKind) error {
	switch kind {
	case perPlatformValueInt:
		v, err := asInt64(raw)
		if err != nil {
			return err
		}
		if v < math.MinInt32 || v > math.MaxInt32 {
			return fmt.Errorf("int32 overflow: %d", v)
		}
		w.writeInt32(int32(v))
		return nil
	case perPlatformValueFloat:
		v, err := asFloat64(raw)
		if err != nil {
			return err
		}
		w.writeUint32(math.Float32bits(float32(v)))
		return nil
	case perPlatformValueFrameRate:
		m, ok := raw.(map[string]any)
		if ok {
			if v, exists := m["value"]; exists {
				if inner, ok := v.(map[string]any); ok {
					m = inner
				}
			}
		}
		if !ok {
			return fmt.Errorf("FrameRate value must be object")
		}
		numerator, err := structFieldAsInt32(m, "Numerator", "numerator")
		if err != nil {
			return err
		}
		denominator, err := structFieldAsInt32(m, "Denominator", "denominator")
		if err != nil {
			return err
		}
		w.writeInt32(numerator)
		w.writeInt32(denominator)
		return nil
	default:
		return fmt.Errorf("unsupported per-platform value kind")
	}
}

func writeRemoteObjectReferenceStruct(w *byteWriter, fields map[string]any) error {
	if fields == nil {
		return fmt.Errorf("RemoteObjectReference fields are missing")
	}
	objectIDRaw, err := structFieldValue(fields, "ObjectId")
	if err != nil {
		return err
	}
	objectID, err := asUint64(objectIDRaw)
	if err != nil {
		return err
	}
	serverIDRaw, err := structFieldValue(fields, "ServerId")
	if err != nil {
		return err
	}
	serverID, err := asUint64(serverIDRaw)
	if err != nil {
		return err
	}
	if serverID > math.MaxUint32 {
		return fmt.Errorf("uint32 overflow: %d", serverID)
	}
	w.writeInt64(int64(objectID))
	w.writeUint32(uint32(serverID))
	return nil
}

func writeAnimationAttributeIdentifierStruct(asset *uasset.Asset, w *byteWriter, fields map[string]any) error {
	if asset == nil {
		return fmt.Errorf("asset is nil")
	}
	if fields == nil {
		return fmt.Errorf("AnimationAttributeIdentifier fields are missing")
	}
	nameRaw, err := structFieldValue(fields, "Name")
	if err != nil {
		return err
	}
	nameRef, err := coerceNameProperty(asset, nameRaw)
	if err != nil {
		return err
	}
	boneNameRaw, err := structFieldValue(fields, "BoneName")
	if err != nil {
		return err
	}
	boneNameRef, err := coerceNameProperty(asset, boneNameRaw)
	if err != nil {
		return err
	}
	boneIndexRaw, err := structFieldValue(fields, "BoneIndex")
	if err != nil {
		return err
	}
	boneIndex, err := asInt64(boneIndexRaw)
	if err != nil {
		return err
	}
	if boneIndex < math.MinInt32 || boneIndex > math.MaxInt32 {
		return fmt.Errorf("int32 overflow: %d", boneIndex)
	}
	scriptStructPathRaw, err := structFieldValue(fields, "ScriptStructPath")
	if err != nil {
		return err
	}
	scriptStructPath, ok := scriptStructPathRaw.(map[string]any)
	if !ok {
		return fmt.Errorf("ScriptStructPath must be object")
	}

	nameIndex, _ := nameRef["index"].(int32)
	nameNumber, _ := nameRef["number"].(int32)
	boneNameIndex, _ := boneNameRef["index"].(int32)
	boneNameNumber, _ := boneNameRef["number"].(int32)
	w.writeNameRef(nameIndex, nameNumber)
	w.writeNameRef(boneNameIndex, boneNameNumber)
	w.writeInt32(int32(boneIndex))
	return writeSoftObjectPathStruct(asset, w, scriptStructPath)
}

func writePerQualityLevelStruct(asset *uasset.Asset, w *byteWriter, fields map[string]any, valueIsFloat bool) error {
	if fields == nil {
		return fmt.Errorf("per-quality struct fields are missing")
	}
	bCookedRaw, err := structFieldValue(fields, "bCooked")
	if err != nil {
		return err
	}
	bCooked, err := asBool(bCookedRaw)
	if err != nil {
		return err
	}
	defaultRaw, err := structFieldValue(fields, "Default")
	if err != nil {
		return err
	}
	perQualityRaw, err := structFieldValue(fields, "PerQuality")
	if err != nil {
		return err
	}
	perQuality, ok := perQualityRaw.(map[string]any)
	if !ok {
		return fmt.Errorf("PerQuality field must be object")
	}

	w.writeUBool(bCooked)
	if valueIsFloat {
		v, err := asFloat64(defaultRaw)
		if err != nil {
			return err
		}
		w.writeUint32(math.Float32bits(float32(v)))
	} else {
		v, err := asInt64(defaultRaw)
		if err != nil {
			return err
		}
		if v < math.MinInt32 || v > math.MaxInt32 {
			return fmt.Errorf("int32 overflow: %d", v)
		}
		w.writeInt32(int32(v))
	}

	entries, err := toMapEntrySlice(perQuality["value"])
	if err != nil {
		return err
	}
	w.writeInt32(int32(len(entries)))
	for _, entry := range entries {
		key, err := asInt64(extractWrappedValue(entry["key"]))
		if err != nil {
			return err
		}
		if key < math.MinInt32 || key > math.MaxInt32 {
			return fmt.Errorf("int32 overflow: %d", key)
		}
		w.writeInt32(int32(key))
		if valueIsFloat {
			v, err := asFloat64(extractWrappedValue(entry["value"]))
			if err != nil {
				return err
			}
			w.writeUint32(math.Float32bits(float32(v)))
		} else {
			v, err := asInt64(extractWrappedValue(entry["value"]))
			if err != nil {
				return err
			}
			if v < math.MinInt32 || v > math.MaxInt32 {
				return fmt.Errorf("int32 overflow: %d", v)
			}
			w.writeInt32(int32(v))
		}
	}
	return nil
}

func writeUniqueNetIdReplStruct(asset *uasset.Asset, w *byteWriter, fields map[string]any) error {
	if asset == nil {
		return fmt.Errorf("asset is nil")
	}
	if fields == nil {
		return fmt.Errorf("UniqueNetIdRepl fields are missing")
	}
	sizeRaw, err := structFieldValue(fields, "Size")
	if err != nil {
		return err
	}
	size, err := asInt64(sizeRaw)
	if err != nil {
		return err
	}
	if size < 0 || size > math.MaxInt32 {
		return fmt.Errorf("invalid UniqueNetIdRepl size: %d", size)
	}

	_, typeRaw, hasType := findStructField(fields, "Type")
	_, contentsRaw, hasContents := findStructField(fields, "Contents")
	if size > 0 && (!hasType || !hasContents) {
		return fmt.Errorf("UniqueNetIdRepl size>0 requires Type and Contents fields")
	}
	if size == 0 {
		w.writeInt32(0)
		return nil
	}

	typeValue := extractWrappedValue(typeRaw)
	typeName, err := coerceNameProperty(asset, typeValue)
	if err != nil {
		return err
	}
	typeIndex, _ := typeName["index"].(int32)
	typeNumber, _ := typeName["number"].(int32)
	contents, ok := extractWrappedValue(contentsRaw).(string)
	if !ok {
		return fmt.Errorf("UniqueNetIdRepl.Contents must be string")
	}

	w.writeInt32(int32(size))
	w.writeNameRef(typeIndex, typeNumber)
	w.writeFString(contents)
	return nil
}

func writeTaggedStructFromFields(asset *uasset.Asset, w *byteWriter, fields map[string]any) error {
	if asset == nil {
		return fmt.Errorf("asset is nil")
	}
	if fields == nil {
		return fmt.Errorf("struct fields are missing")
	}

	type orderedField struct {
		name   string
		raw    map[string]any
		offset int
		hasOff bool
	}
	ordered := make([]orderedField, 0, len(fields))
	for name, raw := range fields {
		m, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("field %s has invalid value shape", name)
		}
		entry := orderedField{name: name, raw: m}
		if off, err := asInt64(m["offset"]); err == nil {
			entry.offset = int(off)
			entry.hasOff = true
		}
		ordered = append(ordered, entry)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].hasOff && ordered[j].hasOff {
			if ordered[i].offset != ordered[j].offset {
				return ordered[i].offset < ordered[j].offset
			}
		} else if ordered[i].hasOff != ordered[j].hasOff {
			return ordered[i].hasOff
		}
		return ordered[i].name < ordered[j].name
	})

	for _, entry := range ordered {
		fieldNameRef, err := resolveNameRef(asset, entry.name)
		if err != nil {
			return err
		}
		typeNodes, err := parseTypeNodesFromWrapper(asset, entry.raw)
		if err != nil {
			return fmt.Errorf("field %s parse type nodes: %w", entry.name, err)
		}
		tree, err := buildTypeTree(typeNodes, asset.Names)
		if err != nil {
			return fmt.Errorf("field %s build type tree: %w", entry.name, err)
		}
		valueBytes, boolValue, err := encodePropertyValue(asset, tree, entry.raw["value"], w.order)
		if err != nil {
			return fmt.Errorf("field %s encode value: %w", entry.name, err)
		}

		flags := uint8(0)
		if rawFlags, ok := entry.raw["flags"]; ok {
			v, err := asInt64(rawFlags)
			if err != nil {
				return fmt.Errorf("field %s invalid flags: %w", entry.name, err)
			}
			if v < 0 || v > math.MaxUint8 {
				return fmt.Errorf("field %s flags out of range: %d", entry.name, v)
			}
			flags = uint8(v)
		}
		tag := uasset.PropertyTag{
			Name:      fieldNameRef,
			TypeNodes: typeNodes,
			Flags:     flags,
		}
		if tag.Flags&propertyFlagHasArrayIndex != 0 {
			if rawArray, ok := entry.raw["arrayIndex"]; ok {
				v, err := asInt64(rawArray)
				if err != nil {
					return fmt.Errorf("field %s invalid arrayIndex: %w", entry.name, err)
				}
				if v < math.MinInt32 || v > math.MaxInt32 {
					return fmt.Errorf("field %s arrayIndex out of range: %d", entry.name, v)
				}
				tag.ArrayIndex = int32(v)
			}
		}
		if tag.Flags&propertyFlagHasPropertyGUID != 0 {
			guidRaw, _ := entry.raw["propertyGuid"].(string)
			guid, err := parseGUIDString(guidRaw)
			if err != nil {
				return fmt.Errorf("field %s invalid propertyGuid: %w", entry.name, err)
			}
			tag.PropertyGUID = &guid
		}
		if tag.Flags&propertyFlagHasPropertyExtensions != 0 {
			if rawExt, ok := entry.raw["propertyExtensions"]; ok {
				v, err := asInt64(rawExt)
				if err != nil {
					return fmt.Errorf("field %s invalid propertyExtensions: %w", entry.name, err)
				}
				if v < 0 || v > math.MaxUint8 {
					return fmt.Errorf("field %s propertyExtensions out of range: %d", entry.name, v)
				}
				tag.PropertyExtensions = uint8(v)
			}
			if tag.PropertyExtensions&propertyExtensionOverridableInfo != 0 {
				if rawOp, ok := entry.raw["overridableOperation"]; ok {
					v, err := asInt64(rawOp)
					if err != nil {
						return fmt.Errorf("field %s invalid overridableOperation: %w", entry.name, err)
					}
					if v < 0 || v > math.MaxUint8 {
						return fmt.Errorf("field %s overridableOperation out of range: %d", entry.name, v)
					}
					tag.OverridableOperation = uint8(v)
				}
				if rawBool, ok := entry.raw["experimentalOverridableLogic"]; ok {
					b, err := asBool(rawBool)
					if err != nil {
						return fmt.Errorf("field %s invalid experimentalOverridableLogic: %w", entry.name, err)
					}
					tag.ExperimentalOverridableLogic = b
				}
			}
		}

		tagBytes, _, err := serializePropertyTag(asset, tag, valueBytes, boolValue, w.order)
		if err != nil {
			return fmt.Errorf("field %s serialize tag: %w", entry.name, err)
		}
		w.writeBytes(tagBytes)
	}

	noneRef, err := resolveNameRef(asset, "None")
	if err != nil {
		return err
	}
	w.writeNameRef(noneRef.Index, 0)
	return nil
}

func parseTypeNodesFromWrapper(asset *uasset.Asset, wrapper map[string]any) ([]uasset.PropertyTypeNode, error) {
	if raw, ok := wrapper["typeNodes"]; ok {
		nodesRaw := make([]any, 0)
		switch v := raw.(type) {
		case []any:
			nodesRaw = v
		case []map[string]any:
			for _, item := range v {
				nodesRaw = append(nodesRaw, item)
			}
		default:
			return nil, fmt.Errorf("typeNodes must be array")
		}
		nodes := make([]uasset.PropertyTypeNode, 0, len(nodesRaw))
		for _, item := range nodesRaw {
			nodeMap, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("typeNodes element must be object")
			}
			name, _ := nodeMap["name"].(string)
			name = strings.TrimSpace(name)
			if name == "" {
				return nil, fmt.Errorf("type node name is required")
			}
			ref, err := resolveNameRef(asset, name)
			if err != nil {
				return nil, err
			}
			innerCount, err := asInt64(nodeMap["innerCount"])
			if err != nil {
				return nil, fmt.Errorf("type node innerCount: %w", err)
			}
			if innerCount < 0 || innerCount > math.MaxInt32 {
				return nil, fmt.Errorf("type node innerCount out of range: %d", innerCount)
			}
			nodes = append(nodes, uasset.PropertyTypeNode{
				Name:       ref,
				InnerCount: int32(innerCount),
			})
		}
		return nodes, nil
	}

	typeStr, _ := wrapper["type"].(string)
	typeStr = strings.TrimSpace(typeStr)
	if typeStr == "" {
		return nil, fmt.Errorf("type or typeNodes is required")
	}
	tree, err := parseTypeExpression(typeStr)
	if err != nil {
		return nil, err
	}
	nodes := make([]uasset.PropertyTypeNode, 0, 8)
	var walk func(n *typeTreeNode) error
	walk = func(n *typeTreeNode) error {
		ref, err := resolveNameRef(asset, n.Name)
		if err != nil {
			return err
		}
		nodes = append(nodes, uasset.PropertyTypeNode{
			Name:       ref,
			InnerCount: int32(len(n.Children)),
		})
		for _, child := range n.Children {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(tree); err != nil {
		return nil, err
	}
	return nodes, nil
}

func parseTypeExpression(raw string) (*typeTreeNode, error) {
	type parser struct {
		s string
		i int
	}
	var (
		readNode func(p *parser) (*typeTreeNode, error)
		skipWS   = func(p *parser) {
			for p.i < len(p.s) {
				switch p.s[p.i] {
				case ' ', '\t', '\r', '\n':
					p.i++
				default:
					return
				}
			}
		}
	)
	readNode = func(p *parser) (*typeTreeNode, error) {
		skipWS(p)
		start := p.i
		for p.i < len(p.s) {
			ch := p.s[p.i]
			if ch == '(' || ch == ')' || ch == ',' || ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' {
				break
			}
			p.i++
		}
		if start == p.i {
			return nil, fmt.Errorf("expected type name at %d", p.i)
		}
		node := &typeTreeNode{Name: p.s[start:p.i]}
		skipWS(p)
		if p.i >= len(p.s) || p.s[p.i] != '(' {
			return node, nil
		}
		p.i++
		for {
			child, err := readNode(p)
			if err != nil {
				return nil, err
			}
			node.Children = append(node.Children, child)
			skipWS(p)
			if p.i >= len(p.s) {
				return nil, fmt.Errorf("missing closing ')' for %s", node.Name)
			}
			if p.s[p.i] == ')' {
				p.i++
				break
			}
			if p.s[p.i] != ',' {
				return nil, fmt.Errorf("expected ',' or ')' at %d", p.i)
			}
			p.i++
		}
		return node, nil
	}

	p := &parser{s: raw}
	node, err := readNode(p)
	if err != nil {
		return nil, err
	}
	skipWS(p)
	if p.i != len(p.s) {
		return nil, fmt.Errorf("unexpected trailing type expression at %d", p.i)
	}
	return node, nil
}

func parseGUIDString(raw string) (uasset.GUID, error) {
	var out uasset.GUID
	raw = strings.TrimSpace(raw)
	raw = strings.ReplaceAll(raw, "-", "")
	if len(raw) != 32 {
		return out, fmt.Errorf("guid length mismatch")
	}
	decoded, err := hex.DecodeString(raw)
	if err != nil {
		return out, err
	}
	if len(decoded) != len(out) {
		return out, fmt.Errorf("guid length mismatch")
	}
	copy(out[:], decoded)
	return out, nil
}

func structFieldValue(fields map[string]any, keys ...string) (any, error) {
	for _, key := range keys {
		_, raw, ok := findStructField(fields, key)
		if !ok {
			continue
		}
		return extractWrappedValue(raw), nil
	}
	return nil, fmt.Errorf("missing struct field (%s)", strings.Join(keys, "/"))
}

func extractWrappedValue(v any) any {
	if m, ok := v.(map[string]any); ok {
		if inner, ok := m["value"]; ok {
			return inner
		}
	}
	return v
}

func serializePropertyTag(asset *uasset.Asset, tag uasset.PropertyTag, valueBytes []byte, boolValue *bool, order binary.ByteOrder) ([]byte, int32, error) {
	if asset != nil &&
		asset.Summary.FileVersionUE5 > 0 &&
		asset.Summary.FileVersionUE5 < ue5PropertyTagCompleteType {
		return serializePropertyTagLegacy(asset, tag, valueBytes, boolValue, order)
	}

	flags := tag.Flags
	size := int32(len(valueBytes))
	if boolValue != nil {
		size = 0
		if *boolValue {
			flags |= propertyFlagBoolTrue
		} else {
			flags &^= propertyFlagBoolTrue
		}
	}
	if size < 0 {
		return nil, 0, fmt.Errorf("negative property size")
	}

	w := newByteWriter(order, int(size)+64)
	w.writeNameRef(tag.Name.Index, tag.Name.Number)
	for _, tn := range tag.TypeNodes {
		w.writeNameRef(tn.Name.Index, tn.Name.Number)
		w.writeInt32(tn.InnerCount)
	}
	w.writeInt32(size)
	w.writeUint8(flags)
	if flags&propertyFlagHasArrayIndex != 0 {
		w.writeInt32(tag.ArrayIndex)
	}
	if flags&propertyFlagHasPropertyGUID != 0 {
		if tag.PropertyGUID == nil {
			return nil, 0, fmt.Errorf("property has GUID flag but GUID is nil")
		}
		writeGUID(w, *tag.PropertyGUID)
	}
	if flags&propertyFlagHasPropertyExtensions != 0 {
		w.writeUint8(tag.PropertyExtensions)
		if tag.PropertyExtensions&propertyExtensionOverridableInfo != 0 {
			w.writeUint8(tag.OverridableOperation)
			w.writeUBool(tag.ExperimentalOverridableLogic)
		}
	}
	if flags&propertyFlagSkippedSerialize == 0 {
		w.writeBytes(valueBytes)
	}
	return w.bytes(), size, nil
}

func serializePropertyTagLegacy(asset *uasset.Asset, tag uasset.PropertyTag, valueBytes []byte, boolValue *bool, order binary.ByteOrder) ([]byte, int32, error) {
	if asset == nil {
		return nil, 0, fmt.Errorf("asset is nil")
	}
	if len(tag.TypeNodes) == 0 {
		return nil, 0, fmt.Errorf("legacy property tag typeNodes are missing")
	}
	if tag.Flags&propertyFlagSkippedSerialize != 0 {
		return nil, 0, fmt.Errorf("legacy property tag does not support SkippedSerialize flag")
	}
	if tag.Flags&propertyFlagHasBinaryOrNative != 0 {
		return nil, 0, fmt.Errorf("legacy property tag does not support BinaryOrNative flag")
	}

	typeTree, err := buildTypeTree(tag.TypeNodes, asset.Names)
	if err != nil {
		return nil, 0, fmt.Errorf("legacy property type tree: %w", err)
	}
	rootType := strings.TrimSpace(typeTree.Name)
	if rootType == "" {
		return nil, 0, fmt.Errorf("legacy property root type is empty")
	}

	size := int32(len(valueBytes))
	if boolValue != nil {
		size = 0
	}
	if size < 0 {
		return nil, 0, fmt.Errorf("negative property size")
	}

	noneRef, err := resolveNameRef(asset, "None")
	if err != nil {
		return nil, 0, err
	}
	legacyNameRef := func(name string) (uasset.NameRef, error) {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" || strings.EqualFold(trimmed, "None") {
			return noneRef, nil
		}
		return resolveNameRef(asset, trimmed)
	}

	w := newByteWriter(order, int(size)+64)
	w.writeNameRef(tag.Name.Index, tag.Name.Number)
	w.writeNameRef(tag.TypeNodes[0].Name.Index, tag.TypeNodes[0].Name.Number)
	w.writeInt32(size)
	w.writeInt32(tag.ArrayIndex)

	switch rootType {
	case "StructProperty":
		if len(typeTree.Children) == 0 {
			return nil, 0, fmt.Errorf("legacy StructProperty requires struct type parameter")
		}
		structNameRef, err := legacyNameRef(typeTree.Children[0].Name)
		if err != nil {
			return nil, 0, err
		}
		w.writeNameRef(structNameRef.Index, structNameRef.Number)
		structGUID := uasset.GUID{}
		if tag.StructGUID != nil {
			structGUID = *tag.StructGUID
		}
		writeGUID(w, structGUID)
	case "BoolProperty":
		boolVal := (tag.Flags & propertyFlagBoolTrue) != 0
		if boolValue != nil {
			boolVal = *boolValue
		}
		if boolVal {
			w.writeUint8(1)
		} else {
			w.writeUint8(0)
		}
	case "ByteProperty":
		enumRef := noneRef
		if len(typeTree.Children) > 0 {
			enumRef, err = legacyNameRef(typeTree.Children[0].Name)
			if err != nil {
				return nil, 0, err
			}
		}
		w.writeNameRef(enumRef.Index, enumRef.Number)
	case "EnumProperty":
		enumRef := noneRef
		if len(typeTree.Children) > 0 {
			enumRef, err = legacyNameRef(typeTree.Children[0].Name)
			if err != nil {
				return nil, 0, err
			}
		}
		w.writeNameRef(enumRef.Index, enumRef.Number)
	case "ArrayProperty", "OptionalProperty", "SetProperty":
		if len(typeTree.Children) == 0 {
			return nil, 0, fmt.Errorf("legacy %s requires inner type parameter", rootType)
		}
		innerRef, err := legacyNameRef(typeTree.Children[0].Name)
		if err != nil {
			return nil, 0, err
		}
		w.writeNameRef(innerRef.Index, innerRef.Number)
	case "MapProperty":
		if len(typeTree.Children) < 2 {
			return nil, 0, fmt.Errorf("legacy MapProperty requires key/value type parameters")
		}
		keyRef, err := legacyNameRef(typeTree.Children[0].Name)
		if err != nil {
			return nil, 0, err
		}
		valueRef, err := legacyNameRef(typeTree.Children[1].Name)
		if err != nil {
			return nil, 0, err
		}
		w.writeNameRef(keyRef.Index, keyRef.Number)
		w.writeNameRef(valueRef.Index, valueRef.Number)
	}

	hasPropertyGUID := tag.Flags&propertyFlagHasPropertyGUID != 0
	if hasPropertyGUID {
		if tag.PropertyGUID == nil {
			return nil, 0, fmt.Errorf("property has GUID flag but GUID is nil")
		}
		w.writeUint8(1)
		writeGUID(w, *tag.PropertyGUID)
	} else {
		w.writeUint8(0)
	}

	if asset.Summary.FileVersionUE5 >= ue5PropertyTagExtension {
		w.writeUint8(tag.PropertyExtensions)
		if tag.PropertyExtensions&propertyExtensionOverridableInfo != 0 {
			w.writeUint8(tag.OverridableOperation)
			w.writeUBool(tag.ExperimentalOverridableLogic)
		}
	}

	w.writeBytes(valueBytes)
	return w.bytes(), size, nil
}

func writeGUID(w *byteWriter, guid uasset.GUID) {
	a := binary.LittleEndian.Uint32(guid[0:4])
	b := binary.LittleEndian.Uint32(guid[4:8])
	c := binary.LittleEndian.Uint32(guid[8:12])
	d := binary.LittleEndian.Uint32(guid[12:16])
	w.writeUint32(a)
	w.writeUint32(b)
	w.writeUint32(c)
	w.writeUint32(d)
}

func toAnySlice(v any) ([]any, error) {
	switch t := v.(type) {
	case nil:
		return []any{}, nil
	case []any:
		return t, nil
	case []map[string]any:
		out := make([]any, 0, len(t))
		for _, it := range t {
			out = append(out, it)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("expected array, got %T", v)
	}
}

func toMapEntrySlice(v any) ([]map[string]any, error) {
	switch t := v.(type) {
	case nil:
		return []map[string]any{}, nil
	case []map[string]any:
		return t, nil
	case []any:
		out := make([]map[string]any, 0, len(t))
		for _, raw := range t {
			m, ok := raw.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("map entry has invalid shape: %T", raw)
			}
			out = append(out, m)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("expected map entry list, got %T", v)
	}
}

func asBool(v any) (bool, error) {
	switch t := v.(type) {
	case bool:
		return t, nil
	case json.Number:
		i, err := t.Int64()
		if err != nil {
			return false, fmt.Errorf("expected bool (0/1), got %q", t.String())
		}
		return i != 0, nil
	case float64:
		if t == 0 {
			return false, nil
		}
		if t == 1 {
			return true, nil
		}
	}
	return false, fmt.Errorf("expected bool")
}

func asInt64(v any) (int64, error) {
	switch t := v.(type) {
	case int:
		return int64(t), nil
	case int8:
		return int64(t), nil
	case int16:
		return int64(t), nil
	case int32:
		return int64(t), nil
	case int64:
		return t, nil
	case uint8:
		return int64(t), nil
	case uint16:
		return int64(t), nil
	case uint32:
		return int64(t), nil
	case uint64:
		if t > math.MaxInt64 {
			return 0, fmt.Errorf("integer overflow: %d", t)
		}
		return int64(t), nil
	case float64:
		if math.Trunc(t) != t {
			return 0, fmt.Errorf("expected integer, got %v", t)
		}
		return int64(t), nil
	case float32:
		if math.Trunc(float64(t)) != float64(t) {
			return 0, fmt.Errorf("expected integer, got %v", t)
		}
		return int64(t), nil
	case json.Number:
		i, err := t.Int64()
		if err == nil {
			return i, nil
		}
		f, ferr := t.Float64()
		if ferr != nil || math.Trunc(f) != f {
			return 0, fmt.Errorf("expected integer, got %q", t.String())
		}
		return int64(f), nil
	case string:
		i, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("expected integer string, got %q", t)
		}
		return i, nil
	default:
		return 0, fmt.Errorf("expected integer, got %T", v)
	}
}

func asUint64(v any) (uint64, error) {
	switch t := v.(type) {
	case uint8:
		return uint64(t), nil
	case uint16:
		return uint64(t), nil
	case uint32:
		return uint64(t), nil
	case uint64:
		return t, nil
	case int:
		if t < 0 {
			return 0, fmt.Errorf("expected unsigned integer, got %d", t)
		}
		return uint64(t), nil
	case int8:
		if t < 0 {
			return 0, fmt.Errorf("expected unsigned integer, got %d", t)
		}
		return uint64(t), nil
	case int16:
		if t < 0 {
			return 0, fmt.Errorf("expected unsigned integer, got %d", t)
		}
		return uint64(t), nil
	case int32:
		if t < 0 {
			return 0, fmt.Errorf("expected unsigned integer, got %d", t)
		}
		return uint64(t), nil
	case int64:
		if t < 0 {
			return 0, fmt.Errorf("expected unsigned integer, got %d", t)
		}
		return uint64(t), nil
	case float64:
		if t < 0 || math.Trunc(t) != t {
			return 0, fmt.Errorf("expected unsigned integer, got %v", t)
		}
		return uint64(t), nil
	case json.Number:
		i, err := t.Int64()
		if err == nil {
			if i < 0 {
				return 0, fmt.Errorf("expected unsigned integer, got %d", i)
			}
			return uint64(i), nil
		}
		f, ferr := t.Float64()
		if ferr != nil || f < 0 || math.Trunc(f) != f {
			return 0, fmt.Errorf("expected unsigned integer, got %q", t.String())
		}
		return uint64(f), nil
	default:
		return 0, fmt.Errorf("expected unsigned integer, got %T", v)
	}
}

func asFloat64(v any) (float64, error) {
	switch t := v.(type) {
	case float64:
		return t, nil
	case float32:
		return float64(t), nil
	case int:
		return float64(t), nil
	case int8:
		return float64(t), nil
	case int16:
		return float64(t), nil
	case int32:
		return float64(t), nil
	case int64:
		return float64(t), nil
	case uint8:
		return float64(t), nil
	case uint16:
		return float64(t), nil
	case uint32:
		return float64(t), nil
	case uint64:
		return float64(t), nil
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			return 0, fmt.Errorf("expected number, got %q", t.String())
		}
		return f, nil
	default:
		return 0, fmt.Errorf("expected number, got %T", v)
	}
}
