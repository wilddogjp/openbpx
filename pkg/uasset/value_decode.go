package uasset

import (
	"encoding/base64"
	"encoding/binary"
	"math"
	"strings"
)

const maxTextPropertyDecodeDepth = 8

// DecodePropertyValue decodes one property value for common scalar/reference types.
// It returns (value, true) when decoded; otherwise (nil, false).
func (a *Asset) DecodePropertyValue(tag PropertyTag) (any, bool) {
	if tag.ValueOffset < 0 || tag.Size < 0 {
		return nil, false
	}
	start := tag.ValueOffset
	end := start + int(tag.Size)
	if end < start || end > len(a.Raw.Bytes) {
		return nil, false
	}
	raw := a.Raw.Bytes[start:end]
	root, ok := buildDecodeTypeTree(tag.TypeNodes, a.Names)
	if !ok {
		return nil, false
	}
	if tag.Flags&propertyFlagSkippedSerialize != 0 && root.Name != "BoolProperty" {
		// SkippedSerialize values are not present in the serialized payload for most types.
		// Decoding from raw bytes would read unrelated data outside the property parse range.
		return nil, false
	}

	switch root.Name {
	case "BoolProperty":
		return (tag.Flags & propertyFlagBoolTrue) != 0, true
	case "StructProperty":
		return a.decodeTopLevelStructProperty(tag, raw, root)
	case "ArrayProperty":
		return a.decodeTopLevelArrayProperty(raw, root)
	case "SetProperty":
		return a.decodeTopLevelSetProperty(raw, root)
	case "MapProperty":
		return a.decodeTopLevelMapProperty(tag, raw, root)
	default:
		r := a.newAssetReader(raw)
		v, ok := a.decodeValueFromReader(r, root)
		if !ok || r.remaining() != 0 {
			return nil, false
		}
		return v, true
	}
}

func (a *Asset) newAssetReader(data []byte) *byteReader {
	return NewByteReaderWithByteSwapping(data, a.Summary.UsesByteSwappedSerialization())
}

type decodeTypeNode struct {
	Name     string
	Children []*decodeTypeNode
}

func buildDecodeTypeTree(nodes []PropertyTypeNode, names []NameEntry) (*decodeTypeNode, bool) {
	if len(nodes) == 0 {
		return nil, false
	}
	idx := 0
	root, ok := parseDecodeTypeNode(nodes, &idx, names)
	if !ok || idx != len(nodes) {
		return nil, false
	}
	return root, true
}

func parseDecodeTypeNode(nodes []PropertyTypeNode, idx *int, names []NameEntry) (*decodeTypeNode, bool) {
	if *idx >= len(nodes) {
		return nil, false
	}
	cur := nodes[*idx]
	*idx++
	n := &decodeTypeNode{Name: cur.Name.Display(names)}
	for i := int32(0); i < cur.InnerCount; i++ {
		child, ok := parseDecodeTypeNode(nodes, idx, names)
		if !ok {
			return nil, false
		}
		n.Children = append(n.Children, child)
	}
	return n, true
}

func (a *Asset) decodeValueFromReader(r *byteReader, node *decodeTypeNode) (any, bool) {
	switch node.Name {
	case "ByteProperty":
		return a.decodeBytePropertyFromReader(r, node)
	case "BoolProperty":
		v, err := r.readUint8()
		if err != nil {
			return nil, false
		}
		return v != 0, true
	case "IntProperty":
		v, err := r.readInt32()
		return v, err == nil
	case "UInt32Property":
		v, err := r.readUint32()
		return v, err == nil
	case "Int64Property":
		v, err := r.readInt64()
		return v, err == nil
	case "UInt64Property":
		b, err := r.readBytes(8)
		if err != nil {
			return nil, false
		}
		return r.byteOrder().Uint64(b), true
	case "Int8Property":
		v, err := r.readUint8()
		if err != nil {
			return nil, false
		}
		return int8(v), true
	case "Int16Property":
		b, err := r.readBytes(2)
		if err != nil {
			return nil, false
		}
		return int16(r.byteOrder().Uint16(b)), true
	case "UInt16Property":
		v, err := r.readUint16()
		return v, err == nil
	case "FloatProperty":
		v, err := r.readUint32()
		if err != nil {
			return nil, false
		}
		return math.Float32frombits(v), true
	case "DoubleProperty":
		b, err := r.readBytes(8)
		if err != nil {
			return nil, false
		}
		return math.Float64frombits(r.byteOrder().Uint64(b)), true
	case "NameProperty":
		return a.decodeNameRefFromReader(r)
	case "ObjectProperty", "ClassProperty":
		return a.decodePackageIndexFromReader(r)
	case "WeakObjectProperty":
		return a.decodeWeakObjectPropertyFromReader(r)
	case "LazyObjectProperty":
		return a.decodeLazyObjectPropertyFromReader(r)
	case "InterfaceProperty":
		return a.decodeInterfacePropertyFromReader(r)
	case "StrProperty":
		v, err := r.readFString()
		return v, err == nil
	case "EnumProperty":
		return a.decodeEnumPropertyFromReader(r, node)
	case "OptionalProperty":
		return a.decodeOptionalPropertyFromReader(r, node)
	case "TextProperty":
		return a.decodeTextPropertyFromReader(r)
	case "SoftObjectProperty", "SoftObjectPathProperty", "SoftClassPathProperty":
		return a.decodeSoftObjectPathFromReader(r)
	case "StructProperty":
		structType := decodeStructTypeName(node)
		return a.decodeKnownStructFromReader(r, structType)
	case "DelegateProperty":
		return a.decodeDelegateFromReader(r)
	case "MulticastDelegateProperty", "MulticastInlineDelegateProperty", "MulticastSparseDelegateProperty":
		return a.decodeMulticastDelegateFromReader(r)
	case "FieldPathProperty":
		return a.decodeFieldPathPropertyFromReader(r)
	default:
		return nil, false
	}
}

func (a *Asset) decodeBytePropertyFromReader(r *byteReader, node *decodeTypeNode) (any, bool) {
	enumType := ""
	if len(node.Children) > 0 {
		enumType = node.Children[0].Name
	}

	start := r.offset()
	if enumType != "" {
		ref, err := r.readNameRef(len(a.Names))
		if err == nil && r.remaining() == 0 {
			value := ref.Display(a.Names)
			if value != "" && !strings.Contains(value, "::") {
				value = enumType + "::" + value
			}
			return map[string]any{
				"enumType": enumType,
				"value":    value,
			}, true
		}
		_ = r.seek(start)
	}

	v, err := r.readUint8()
	if err != nil {
		return nil, false
	}
	return v, true
}

func (a *Asset) decodeOptionalPropertyFromReader(r *byteReader, node *decodeTypeNode) (any, bool) {
	isSet, err := r.readUBool()
	if err != nil {
		return nil, false
	}

	inner := &decodeTypeNode{Name: "Unknown"}
	if len(node.Children) > 0 {
		inner = node.Children[0]
	}

	out := map[string]any{
		"optionalType": inner.Name,
		"isSet":        isSet,
	}
	if !isSet {
		return out, true
	}

	if len(node.Children) == 0 {
		if r.remaining() > 0 {
			raw, err := r.readBytes(r.remaining())
			if err != nil {
				return nil, false
			}
			out["rawBase64"] = base64.StdEncoding.EncodeToString(raw)
		}
		return out, true
	}

	start := r.offset()
	val, ok := a.decodeValueFromReader(r, inner)
	if ok {
		out["value"] = map[string]any{
			"type":  inner.Name,
			"value": val,
		}
		return out, true
	}
	_ = r.seek(start)
	if r.remaining() > 0 {
		raw, err := r.readBytes(r.remaining())
		if err != nil {
			return nil, false
		}
		out["rawBase64"] = base64.StdEncoding.EncodeToString(raw)
	}
	return out, true
}

func (a *Asset) decodeTopLevelStructProperty(tag PropertyTag, raw []byte, node *decodeTypeNode) (any, bool) {
	structType := decodeStructTypeName(node)
	if v, ok := a.decodeKnownStructFromBytes(structType, raw); ok {
		return map[string]any{
			"structType": structType,
			"value":      v,
		}, true
	}

	decoded, props, ok := a.decodeTaggedStructFromBytes(raw, structType)
	if !ok {
		return map[string]any{
			"structType": structType,
			"rawBase64":  base64.StdEncoding.EncodeToString(raw),
		}, true
	}
	if len(props.Properties) == 0 && !isKnownTaggedStructDecodeCandidate(strings.ToLower(structType)) {
		return map[string]any{
			"structType": structType,
			"rawBase64":  base64.StdEncoding.EncodeToString(raw),
		}, true
	}
	out, _ := decoded.(map[string]any)
	if len(props.Warnings) > 0 {
		out["warnings"] = props.Warnings
	}
	if props.EndOffset < len(raw) {
		out["trailingBytes"] = len(raw) - props.EndOffset
	}
	return out, true
}

func (a *Asset) decodeTopLevelArrayProperty(raw []byte, node *decodeTypeNode) (any, bool) {
	elem := &decodeTypeNode{Name: "Unknown"}
	if len(node.Children) > 0 {
		elem = node.Children[0]
	}

	r := a.newAssetReader(raw)
	count, err := r.readInt32()
	if err != nil {
		return nil, false
	}
	if count == -1 {
		return a.decodeTopLevelOverridableArrayProperty(raw, elem)
	}
	if count < 0 || count > maxTableEntries {
		return nil, false
	}

	values := make([]any, 0, count)
	for i := int32(0); i < count; i++ {
		val, ok := a.decodeValueFromReader(r, elem)
		if !ok {
			return map[string]any{
				"arrayType": elem.Name,
				"rawBase64": base64.StdEncoding.EncodeToString(raw),
			}, true
		}
		values = append(values, map[string]any{
			"type":  elem.Name,
			"value": val,
		})
	}
	out := map[string]any{
		"arrayType": elem.Name,
		"value":     values,
	}
	if r.remaining() > 0 {
		out["trailingBytes"] = r.remaining()
	}
	return out, true
}

func (a *Asset) decodeTopLevelOverridableArrayProperty(raw []byte, elem *decodeTypeNode) (any, bool) {
	r := a.newAssetReader(raw)
	numReplaced, err := r.readInt32()
	if err != nil || numReplaced != -1 {
		return nil, false
	}

	readList := func() ([]any, bool) {
		count, err := r.readInt32()
		if err != nil || count < 0 || count > maxTableEntries {
			return nil, false
		}
		items := make([]any, 0, count)
		for i := int32(0); i < count; i++ {
			val, ok := a.decodeValueFromReader(r, elem)
			if !ok {
				return nil, false
			}
			items = append(items, map[string]any{
				"type":  elem.Name,
				"value": val,
			})
		}
		return items, true
	}

	removed, ok := readList()
	if !ok {
		return map[string]any{
			"arrayType": elem.Name,
			"rawBase64": base64.StdEncoding.EncodeToString(raw),
		}, true
	}
	modified, ok := readList()
	if !ok {
		return map[string]any{
			"arrayType": elem.Name,
			"rawBase64": base64.StdEncoding.EncodeToString(raw),
		}, true
	}

	out := map[string]any{
		"arrayType":   elem.Name,
		"overridable": true,
		"replaceMode": false,
		"removed":     removed,
		"modified":    modified,
	}

	if a.Summary.FileVersionUE5 >= ue5OSSubObjectShadowSerialization {
		shadowed, ok := readList()
		if !ok {
			return map[string]any{
				"arrayType": elem.Name,
				"rawBase64": base64.StdEncoding.EncodeToString(raw),
			}, true
		}
		out["shadowed"] = shadowed
	}

	added, ok := readList()
	if !ok {
		return map[string]any{
			"arrayType": elem.Name,
			"rawBase64": base64.StdEncoding.EncodeToString(raw),
		}, true
	}
	out["added"] = added
	if r.remaining() > 0 {
		out["trailingBytes"] = r.remaining()
	}
	return out, true
}

func (a *Asset) decodeTopLevelSetProperty(raw []byte, node *decodeTypeNode) (any, bool) {
	elem := &decodeTypeNode{Name: "Unknown"}
	if len(node.Children) > 0 {
		elem = node.Children[0]
	}
	r := a.newAssetReader(raw)
	removeCount, err := r.readInt32()
	if err != nil || removeCount < 0 || removeCount > maxTableEntries {
		return nil, false
	}
	for i := int32(0); i < removeCount; i++ {
		if _, ok := a.decodeValueFromReader(r, elem); !ok {
			return map[string]any{
				"setType":   elem.Name,
				"rawBase64": base64.StdEncoding.EncodeToString(raw),
			}, true
		}
	}
	addCount, err := r.readInt32()
	if err != nil || addCount < 0 || addCount > maxTableEntries {
		return nil, false
	}
	values := make([]any, 0, addCount)
	for i := int32(0); i < addCount; i++ {
		val, ok := a.decodeValueFromReader(r, elem)
		if !ok {
			return map[string]any{
				"setType":   elem.Name,
				"rawBase64": base64.StdEncoding.EncodeToString(raw),
			}, true
		}
		values = append(values, map[string]any{
			"type":  elem.Name,
			"value": val,
		})
	}
	out := map[string]any{
		"setType": elem.Name,
		"value":   values,
	}
	if r.remaining() > 0 {
		out["trailingBytes"] = r.remaining()
	}
	return out, true
}

func (a *Asset) decodeTopLevelMapProperty(tag PropertyTag, raw []byte, node *decodeTypeNode) (any, bool) {
	keyNode := &decodeTypeNode{Name: "Unknown"}
	valueNode := &decodeTypeNode{Name: "Unknown"}
	if len(node.Children) >= 1 {
		keyNode = node.Children[0]
	}
	if len(node.Children) >= 2 {
		valueNode = node.Children[1]
	}

	if tag.ExperimentalOverridableLogic {
		if out, ok := a.decodeTopLevelOverridableMapProperty(raw, keyNode, valueNode); ok {
			return out, true
		}
	}
	return a.decodeTopLevelStandardMapProperty(raw, keyNode, valueNode)
}

func (a *Asset) decodeTopLevelStandardMapProperty(raw []byte, keyNode, valueNode *decodeTypeNode) (any, bool) {
	r := a.newAssetReader(raw)
	removeCount, err := r.readInt32()
	if err != nil {
		return nil, false
	}
	replaceMap := false
	removed := make([]any, 0)
	switch {
	case removeCount == -1:
		replaceMap = true
	case removeCount >= 0 && removeCount <= maxTableEntries:
		for i := int32(0); i < removeCount; i++ {
			keyVal, ok := a.decodeValueFromReader(r, keyNode)
			if !ok {
				return map[string]any{
					"keyType":   keyNode.Name,
					"valueType": valueNode.Name,
					"rawBase64": base64.StdEncoding.EncodeToString(raw),
				}, true
			}
			removed = append(removed, map[string]any{
				"type":  keyNode.Name,
				"value": keyVal,
			})
		}
	default:
		return nil, false
	}
	addCount, err := r.readInt32()
	if err != nil || addCount < 0 || addCount > maxTableEntries {
		return nil, false
	}
	values := make([]map[string]any, 0, addCount)
	for i := int32(0); i < addCount; i++ {
		keyVal, ok := a.decodeValueFromReader(r, keyNode)
		if !ok {
			return map[string]any{
				"keyType":   keyNode.Name,
				"valueType": valueNode.Name,
				"rawBase64": base64.StdEncoding.EncodeToString(raw),
			}, true
		}
		valVal, ok := a.decodeValueFromReader(r, valueNode)
		if !ok {
			return map[string]any{
				"keyType":   keyNode.Name,
				"valueType": valueNode.Name,
				"rawBase64": base64.StdEncoding.EncodeToString(raw),
			}, true
		}
		values = append(values, map[string]any{
			"key": map[string]any{
				"type":  keyNode.Name,
				"value": keyVal,
			},
			"value": map[string]any{
				"type":  valueNode.Name,
				"value": valVal,
			},
		})
	}
	out := map[string]any{
		"keyType":    keyNode.Name,
		"valueType":  valueNode.Name,
		"replaceMap": replaceMap,
		"value":      values,
	}
	if len(removed) > 0 {
		out["removed"] = removed
	}
	if r.remaining() > 0 {
		out["trailingBytes"] = r.remaining()
	}
	return out, true
}

func (a *Asset) decodeTopLevelOverridableMapProperty(raw []byte, keyNode, valueNode *decodeTypeNode) (any, bool) {
	r := a.newAssetReader(raw)
	numReplaced, err := r.readInt32()
	if err != nil {
		return nil, false
	}

	decodeEntry := func() (map[string]any, bool) {
		keyVal, ok := a.decodeValueFromReader(r, keyNode)
		if !ok {
			return nil, false
		}
		valVal, ok := a.decodeValueFromReader(r, valueNode)
		if !ok {
			return nil, false
		}
		return map[string]any{
			"key": map[string]any{
				"type":  keyNode.Name,
				"value": keyVal,
			},
			"value": map[string]any{
				"type":  valueNode.Name,
				"value": valVal,
			},
		}, true
	}

	decodeEntries := func(count int32) ([]map[string]any, bool) {
		if count < 0 || count > maxTableEntries {
			return nil, false
		}
		entries := make([]map[string]any, 0, count)
		for i := int32(0); i < count; i++ {
			entry, ok := decodeEntry()
			if !ok {
				return nil, false
			}
			entries = append(entries, entry)
		}
		return entries, true
	}

	if numReplaced != -1 {
		replaced, ok := decodeEntries(numReplaced)
		if !ok {
			return map[string]any{
				"keyType":     keyNode.Name,
				"valueType":   valueNode.Name,
				"overridable": true,
				"rawBase64":   base64.StdEncoding.EncodeToString(raw),
			}, true
		}
		out := map[string]any{
			"keyType":     keyNode.Name,
			"valueType":   valueNode.Name,
			"overridable": true,
			"replaceMap":  true,
			"value":       replaced,
		}
		if r.remaining() > 0 {
			out["trailingBytes"] = r.remaining()
		}
		return out, true
	}

	removedCount, err := r.readInt32()
	if err != nil || removedCount < 0 || removedCount > maxTableEntries {
		return nil, false
	}
	removed := make([]any, 0, removedCount)
	for i := int32(0); i < removedCount; i++ {
		keyVal, ok := a.decodeValueFromReader(r, keyNode)
		if !ok {
			return map[string]any{
				"keyType":     keyNode.Name,
				"valueType":   valueNode.Name,
				"overridable": true,
				"rawBase64":   base64.StdEncoding.EncodeToString(raw),
			}, true
		}
		removed = append(removed, map[string]any{
			"type":  keyNode.Name,
			"value": keyVal,
		})
	}

	modifiedCount, err := r.readInt32()
	if err != nil {
		return nil, false
	}
	modified, ok := decodeEntries(modifiedCount)
	if !ok {
		return map[string]any{
			"keyType":     keyNode.Name,
			"valueType":   valueNode.Name,
			"overridable": true,
			"rawBase64":   base64.StdEncoding.EncodeToString(raw),
		}, true
	}

	shadowed := []map[string]any(nil)
	if a.Summary.FileVersionUE5 >= ue5OSSubObjectShadowSerialization {
		shadowedCount, err := r.readInt32()
		if err != nil {
			return nil, false
		}
		shadowed, ok = decodeEntries(shadowedCount)
		if !ok {
			return map[string]any{
				"keyType":     keyNode.Name,
				"valueType":   valueNode.Name,
				"overridable": true,
				"rawBase64":   base64.StdEncoding.EncodeToString(raw),
			}, true
		}
	}

	addedCount, err := r.readInt32()
	if err != nil {
		return nil, false
	}
	added, ok := decodeEntries(addedCount)
	if !ok {
		return map[string]any{
			"keyType":     keyNode.Name,
			"valueType":   valueNode.Name,
			"overridable": true,
			"rawBase64":   base64.StdEncoding.EncodeToString(raw),
		}, true
	}

	combined := make([]map[string]any, 0, len(modified)+len(added))
	combined = append(combined, modified...)
	combined = append(combined, added...)

	out := map[string]any{
		"keyType":     keyNode.Name,
		"valueType":   valueNode.Name,
		"overridable": true,
		"replaceMap":  false,
		"value":       combined,
		"removed":     removed,
		"modified":    modified,
		"added":       added,
	}
	if shadowed != nil {
		out["shadowed"] = shadowed
	}
	if r.remaining() > 0 {
		out["trailingBytes"] = r.remaining()
	}
	return out, true
}

func (a *Asset) decodeNameRefFromReader(r *byteReader) (any, bool) {
	ref, err := r.readNameRef(len(a.Names))
	if err != nil {
		return nil, false
	}
	return map[string]any{
		"index":  ref.Index,
		"number": ref.Number,
		"name":   ref.Display(a.Names),
	}, true
}

func (a *Asset) decodePackageIndexFromReader(r *byteReader) (any, bool) {
	rawIdx, err := r.readInt32()
	if err != nil {
		return nil, false
	}
	return map[string]any{
		"index":    rawIdx,
		"resolved": a.ParseIndex(PackageIndex(rawIdx)),
	}, true
}

func (a *Asset) decodeWeakObjectPropertyFromReader(r *byteReader) (any, bool) {
	return a.decodeArchiveObjectReferenceFromReader(r)
}

func (a *Asset) decodeLazyObjectPropertyFromReader(r *byteReader) (any, bool) {
	return a.decodeArchiveObjectReferenceFromReader(r)
}

func (a *Asset) decodeInterfacePropertyFromReader(r *byteReader) (any, bool) {
	return a.decodeArchiveObjectReferenceFromReader(r)
}

func (a *Asset) decodeArchiveObjectReferenceFromReader(r *byteReader) (any, bool) {
	start := r.offset()
	if obj, ok := a.decodePackageIndexFromReader(r); ok && r.remaining() == 0 {
		return obj, true
	}
	_ = r.seek(start)

	if r.remaining() == 8 {
		objectIndex, err := r.readInt32()
		if err != nil {
			return nil, false
		}
		serialNumber, err := r.readInt32()
		if err != nil {
			return nil, false
		}
		return map[string]any{
			"objectIndex":  objectIndex,
			"serialNumber": serialNumber,
		}, true
	}

	if r.remaining() == 16 {
		guid, err := r.readGUID()
		if err != nil {
			return nil, false
		}
		return map[string]any{
			"guid": guid.String(),
		}, true
	}
	return nil, false
}

func (a *Asset) decodeEnumPropertyFromReader(r *byteReader, node *decodeTypeNode) (any, bool) {
	enumType := ""
	if len(node.Children) > 0 {
		enumType = node.Children[0].Name
	}
	start := r.offset()
	ref, err := r.readNameRef(len(a.Names))
	if err == nil {
		value := ref.Display(a.Names)
		if enumType != "" && value != "" && !strings.Contains(value, "::") {
			value = enumType + "::" + value
		}
		return map[string]any{
			"enumType": enumType,
			"value":    value,
		}, true
	}
	_ = r.seek(start)

	switch r.remaining() {
	case 1:
		v, e := r.readUint8()
		if e != nil {
			return nil, false
		}
		return map[string]any{"enumType": enumType, "value": int32(v)}, true
	case 2:
		b, e := r.readBytes(2)
		if e != nil {
			return nil, false
		}
		return map[string]any{"enumType": enumType, "value": int32(r.byteOrder().Uint16(b))}, true
	case 4:
		v, e := r.readInt32()
		if e != nil {
			return nil, false
		}
		return map[string]any{"enumType": enumType, "value": v}, true
	default:
		return nil, false
	}
}

func (a *Asset) decodeTextPropertyFromReader(r *byteReader) (any, bool) {
	return a.decodeTextPropertyFromReaderWithDepth(r, 0)
}

func (a *Asset) decodeTextPropertyFromReaderWithDepth(r *byteReader, depth int) (any, bool) {
	if depth > maxTextPropertyDecodeDepth {
		return nil, false
	}

	start := r.offset()
	flags, err := r.readInt32()
	if err != nil {
		return nil, false
	}
	historyType, err := r.readUint8()
	if err != nil {
		_ = r.seek(start)
		return nil, false
	}

	result := map[string]any{
		"flags":           flags,
		"historyType":     textHistoryTypeName(historyType),
		"historyTypeCode": historyType,
	}

	switch historyType {
	case 255: // None
		hasCultureInvariantString, err := r.readUBool()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		result["hasCultureInvariantString"] = hasCultureInvariantString
		if hasCultureInvariantString {
			value, err := r.readFString()
			if err != nil {
				_ = r.seek(start)
				return nil, false
			}
			result["cultureInvariantString"] = value
			result["value"] = value
		}
	case 0: // Base
		namespace, err := r.readFString()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		key, err := r.readFString()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		source, err := r.readFString()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		result["namespace"] = namespace
		result["key"] = key
		result["sourceString"] = source
		result["value"] = source
		result["cultureInvariantString"] = source
	case 1: // NamedFormat
		formatText, ok := a.decodeTextPropertyFromReaderWithDepth(r, depth+1)
		if !ok {
			_ = r.seek(start)
			return nil, false
		}
		arguments, ok := a.decodeTextNamedArgumentsFromReader(r, depth+1)
		if !ok {
			_ = r.seek(start)
			return nil, false
		}
		result["formatText"] = formatText
		result["arguments"] = arguments
	case 2: // OrderedFormat
		formatText, ok := a.decodeTextPropertyFromReaderWithDepth(r, depth+1)
		if !ok {
			_ = r.seek(start)
			return nil, false
		}
		arguments, ok := a.decodeTextOrderedArgumentsFromReader(r, depth+1)
		if !ok {
			_ = r.seek(start)
			return nil, false
		}
		result["formatText"] = formatText
		result["arguments"] = arguments
	case 3: // ArgumentFormat
		formatText, ok := a.decodeTextPropertyFromReaderWithDepth(r, depth+1)
		if !ok {
			_ = r.seek(start)
			return nil, false
		}
		arguments, ok := a.decodeTextArgumentDataArrayFromReader(r, depth+1)
		if !ok {
			_ = r.seek(start)
			return nil, false
		}
		result["formatText"] = formatText
		result["arguments"] = arguments
	case 4, 5: // AsNumber / AsPercent
		sourceValue, ok := a.decodeTextFormatArgumentValueFromReader(r, depth+1)
		if !ok {
			_ = r.seek(start)
			return nil, false
		}
		result["sourceValue"] = sourceValue
		hasFormatOptions, err := r.readUBool()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		result["hasFormatOptions"] = hasFormatOptions
		if hasFormatOptions {
			formatOptions, ok := a.decodeTextNumberFormattingOptionsFromReader(r)
			if !ok {
				_ = r.seek(start)
				return nil, false
			}
			result["formatOptions"] = formatOptions
		}
		targetCulture, err := r.readFString()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		result["targetCulture"] = targetCulture
	case 6: // AsCurrency
		currencyCode, err := r.readFString()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		result["currencyCode"] = currencyCode
		sourceValue, ok := a.decodeTextFormatArgumentValueFromReader(r, depth+1)
		if !ok {
			_ = r.seek(start)
			return nil, false
		}
		result["sourceValue"] = sourceValue
		hasFormatOptions, err := r.readUBool()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		result["hasFormatOptions"] = hasFormatOptions
		if hasFormatOptions {
			formatOptions, ok := a.decodeTextNumberFormattingOptionsFromReader(r)
			if !ok {
				_ = r.seek(start)
				return nil, false
			}
			result["formatOptions"] = formatOptions
		}
		targetCulture, err := r.readFString()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		result["targetCulture"] = targetCulture
	case 7: // AsDate
		sourceDateTimeTicks, err := r.readInt64()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		dateStyleCode, err := r.readUint8()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		timeZone, err := r.readFString()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		targetCulture, err := r.readFString()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		dateStyleCodeSigned := int8(dateStyleCode)
		result["sourceDateTimeTicks"] = sourceDateTimeTicks
		result["dateStyleCode"] = int32(dateStyleCodeSigned)
		result["dateStyle"] = textDateTimeStyleName(dateStyleCodeSigned)
		result["timeZone"] = timeZone
		result["targetCulture"] = targetCulture
	case 8: // AsTime
		sourceDateTimeTicks, err := r.readInt64()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		timeStyleCode, err := r.readUint8()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		timeZone, err := r.readFString()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		targetCulture, err := r.readFString()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		timeStyleCodeSigned := int8(timeStyleCode)
		result["sourceDateTimeTicks"] = sourceDateTimeTicks
		result["timeStyleCode"] = int32(timeStyleCodeSigned)
		result["timeStyle"] = textDateTimeStyleName(timeStyleCodeSigned)
		result["timeZone"] = timeZone
		result["targetCulture"] = targetCulture
	case 9: // AsDateTime
		sourceDateTimeTicks, err := r.readInt64()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		dateStyleCode, err := r.readUint8()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		timeStyleCode, err := r.readUint8()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		dateStyleCodeSigned := int8(dateStyleCode)
		timeStyleCodeSigned := int8(timeStyleCode)
		result["sourceDateTimeTicks"] = sourceDateTimeTicks
		result["dateStyleCode"] = int32(dateStyleCodeSigned)
		result["dateStyle"] = textDateTimeStyleName(dateStyleCodeSigned)
		result["timeStyleCode"] = int32(timeStyleCodeSigned)
		result["timeStyle"] = textDateTimeStyleName(timeStyleCodeSigned)
		if dateStyleCodeSigned == 5 { // EDateTimeStyle::Custom
			customPattern, err := r.readFString()
			if err != nil {
				_ = r.seek(start)
				return nil, false
			}
			result["customPattern"] = customPattern
		}
		timeZone, err := r.readFString()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		targetCulture, err := r.readFString()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		result["timeZone"] = timeZone
		result["targetCulture"] = targetCulture
	case 10: // Transform
		sourceText, ok := a.decodeTextPropertyFromReaderWithDepth(r, depth+1)
		if !ok {
			_ = r.seek(start)
			return nil, false
		}
		transformTypeCode, err := r.readUint8()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		result["sourceText"] = sourceText
		result["transformTypeCode"] = int32(transformTypeCode)
		result["transformType"] = textTransformTypeName(transformTypeCode)
	case 11: // StringTableEntry
		tableID, ok := a.decodeNameRefFromReader(r)
		if !ok {
			_ = r.seek(start)
			return nil, false
		}
		key, err := r.readFString()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		result["tableId"] = tableID
		if tableIDMap, ok := tableID.(map[string]any); ok {
			result["tableIdName"] = tableIDMap["name"]
		}
		result["key"] = key
	case 12: // TextGenerator
		generatorType, ok := a.decodeNameRefFromReader(r)
		if !ok {
			_ = r.seek(start)
			return nil, false
		}
		result["generatorType"] = generatorType
		generatorTypeName := ""
		if generatorTypeMap, ok := generatorType.(map[string]any); ok {
			generatorTypeName, _ = generatorTypeMap["name"].(string)
			result["generatorTypeName"] = generatorTypeName
		}

		if !strings.EqualFold(generatorTypeName, "None") {
			count, err := r.readInt32()
			if err != nil || count < 0 || count > maxTableEntries || int(count) > r.remaining() {
				_ = r.seek(start)
				return nil, false
			}
			payload, err := r.readBytes(int(count))
			if err != nil {
				_ = r.seek(start)
				return nil, false
			}
			result["generatorContentsBase64"] = base64.StdEncoding.EncodeToString(payload)
		}
	default:
		if r.remaining() > 0 {
			bytes, err := r.readBytes(r.remaining())
			if err != nil {
				_ = r.seek(start)
				return nil, false
			}
			result["rawBase64"] = base64.StdEncoding.EncodeToString(bytes)
		}
	}
	return result, true
}

func (a *Asset) decodeTextNamedArgumentsFromReader(r *byteReader, depth int) ([]map[string]any, bool) {
	count, err := r.readInt32()
	if err != nil || count < 0 || count > maxTableEntries {
		return nil, false
	}
	args := make([]map[string]any, 0, count)
	for i := int32(0); i < count; i++ {
		name, err := r.readFString()
		if err != nil {
			return nil, false
		}
		value, ok := a.decodeTextFormatArgumentValueFromReader(r, depth)
		if !ok {
			return nil, false
		}
		args = append(args, map[string]any{
			"name":  name,
			"value": value,
		})
	}
	return args, true
}

func (a *Asset) decodeTextOrderedArgumentsFromReader(r *byteReader, depth int) ([]any, bool) {
	count, err := r.readInt32()
	if err != nil || count < 0 || count > maxTableEntries {
		return nil, false
	}
	args := make([]any, 0, count)
	for i := int32(0); i < count; i++ {
		value, ok := a.decodeTextFormatArgumentValueFromReader(r, depth)
		if !ok {
			return nil, false
		}
		args = append(args, value)
	}
	return args, true
}

func (a *Asset) decodeTextArgumentDataArrayFromReader(r *byteReader, depth int) ([]map[string]any, bool) {
	count, err := r.readInt32()
	if err != nil || count < 0 || count > maxTableEntries {
		return nil, false
	}
	args := make([]map[string]any, 0, count)
	for i := int32(0); i < count; i++ {
		arg, ok := a.decodeTextArgumentDataFromReader(r, depth)
		if !ok {
			return nil, false
		}
		args = append(args, arg)
	}
	return args, true
}

func (a *Asset) decodeTextFormatArgumentValueFromReader(r *byteReader, depth int) (map[string]any, bool) {
	typeCode, err := r.readUint8()
	if err != nil {
		return nil, false
	}

	out := map[string]any{
		"typeCode": int32(typeCode),
		"type":     textFormatArgumentTypeName(typeCode),
	}

	switch typeCode {
	case 0: // Int
		v, err := r.readInt64()
		if err != nil {
			return nil, false
		}
		out["value"] = v
	case 1: // UInt
		b, err := r.readBytes(8)
		if err != nil {
			return nil, false
		}
		out["value"] = r.byteOrder().Uint64(b)
	case 2: // Float
		bits, err := r.readUint32()
		if err != nil {
			return nil, false
		}
		out["value"] = math.Float32frombits(bits)
	case 3: // Double
		b, err := r.readBytes(8)
		if err != nil {
			return nil, false
		}
		out["value"] = math.Float64frombits(r.byteOrder().Uint64(b))
	case 4: // Text
		textValue, ok := a.decodeTextPropertyFromReaderWithDepth(r, depth+1)
		if !ok {
			return nil, false
		}
		out["value"] = textValue
	case 5: // Gender (stored as UInt)
		b, err := r.readBytes(8)
		if err != nil {
			return nil, false
		}
		genderCode := r.byteOrder().Uint64(b)
		out["value"] = genderCode
		out["genderCode"] = genderCode
		out["gender"] = textGenderName(genderCode)
	default:
		return nil, false
	}

	return out, true
}

func (a *Asset) decodeTextArgumentDataFromReader(r *byteReader, depth int) (map[string]any, bool) {
	name, err := r.readFString()
	if err != nil {
		return nil, false
	}
	typeCode, err := r.readUint8()
	if err != nil {
		return nil, false
	}

	out := map[string]any{
		"name":          name,
		"valueTypeCode": int32(typeCode),
		"valueType":     textFormatArgumentTypeName(typeCode),
	}

	switch typeCode {
	case 0: // Int
		v, err := r.readInt64()
		if err != nil {
			return nil, false
		}
		out["value"] = v
	case 2: // Float
		bits, err := r.readUint32()
		if err != nil {
			return nil, false
		}
		out["value"] = math.Float32frombits(bits)
	case 3: // Double
		b, err := r.readBytes(8)
		if err != nil {
			return nil, false
		}
		out["value"] = math.Float64frombits(r.byteOrder().Uint64(b))
	case 4: // Text
		textValue, ok := a.decodeTextPropertyFromReaderWithDepth(r, depth+1)
		if !ok {
			return nil, false
		}
		out["value"] = textValue
	case 5: // Gender
		genderCode, err := r.readUint8()
		if err != nil {
			return nil, false
		}
		out["value"] = int32(genderCode)
		out["genderCode"] = int32(genderCode)
		out["gender"] = textGenderName(uint64(genderCode))
	default:
		return nil, false
	}

	return out, true
}

func (a *Asset) decodeTextNumberFormattingOptionsFromReader(r *byteReader) (map[string]any, bool) {
	alwaysSign, err := r.readUBool()
	if err != nil {
		return nil, false
	}
	useGrouping, err := r.readUBool()
	if err != nil {
		return nil, false
	}
	roundingModeRaw, err := r.readUint8()
	if err != nil {
		return nil, false
	}
	minIntegralDigits, err := r.readInt32()
	if err != nil {
		return nil, false
	}
	maxIntegralDigits, err := r.readInt32()
	if err != nil {
		return nil, false
	}
	minFractionalDigits, err := r.readInt32()
	if err != nil {
		return nil, false
	}
	maxFractionalDigits, err := r.readInt32()
	if err != nil {
		return nil, false
	}

	roundingMode := int8(roundingModeRaw)
	return map[string]any{
		"alwaysSign":              alwaysSign,
		"useGrouping":             useGrouping,
		"roundingModeCode":        int32(roundingMode),
		"roundingMode":            textRoundingModeName(roundingMode),
		"minimumIntegralDigits":   minIntegralDigits,
		"maximumIntegralDigits":   maxIntegralDigits,
		"minimumFractionalDigits": minFractionalDigits,
		"maximumFractionalDigits": maxFractionalDigits,
	}, true
}

func textHistoryTypeName(v uint8) string {
	switch v {
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
	case 255:
		return "None"
	default:
		return "Unknown"
	}
}

func textTransformTypeName(v uint8) string {
	switch v {
	case 0:
		return "ToLower"
	case 1:
		return "ToUpper"
	default:
		return "Unknown"
	}
}

func textDateTimeStyleName(v int8) string {
	switch v {
	case 0:
		return "Default"
	case 1:
		return "Short"
	case 2:
		return "Medium"
	case 3:
		return "Long"
	case 4:
		return "Full"
	case 5:
		return "Custom"
	default:
		return "Unknown"
	}
}

func textFormatArgumentTypeName(v uint8) string {
	switch v {
	case 0:
		return "Int"
	case 1:
		return "UInt"
	case 2:
		return "Float"
	case 3:
		return "Double"
	case 4:
		return "Text"
	case 5:
		return "Gender"
	default:
		return "Unknown"
	}
}

func textGenderName(v uint64) string {
	switch v {
	case 0:
		return "Masculine"
	case 1:
		return "Feminine"
	case 2:
		return "Neuter"
	default:
		return "Unknown"
	}
}

func textRoundingModeName(v int8) string {
	switch v {
	case 0:
		return "HalfToEven"
	case 1:
		return "HalfFromZero"
	case 2:
		return "HalfToZero"
	case 3:
		return "FromZero"
	case 4:
		return "ToZero"
	case 5:
		return "ToNegativeInfinity"
	case 6:
		return "ToPositiveInfinity"
	default:
		return "Unknown"
	}
}

func splitLegacySoftObjectAssetPath(raw string) (packageName string, assetName string) {
	path := strings.TrimSpace(raw)
	if path == "" || strings.EqualFold(path, "None") {
		return "", ""
	}
	dot := strings.LastIndexByte(path, '.')
	if dot <= 0 || dot+1 >= len(path) {
		return path, ""
	}
	return path[:dot], path[dot+1:]
}

func (a *Asset) decodeSoftObjectPathFromReader(r *byteReader) (any, bool) {
	start := r.offset()

	knownVersion := a.Summary.FileVersionUE5 > 0
	tryModern := !knownVersion || a.Summary.SupportsTopLevelAssetPathSoftObjectPath()
	tryLegacy := !knownVersion || !a.Summary.SupportsTopLevelAssetPathSoftObjectPath()

	if tryModern {
		if r.remaining() == 4 && a.Summary.SupportsSoftObjectPathListInSummary() && a.Summary.SoftObjectPathsCount > 0 {
			index, err := r.readInt32()
			if err != nil {
				return nil, false
			}
			return a.decodeSoftObjectPathIndex(index), true
		}

		packageName, err := r.readNameRef(len(a.Names))
		if err == nil {
			assetName, err := r.readNameRef(len(a.Names))
			if err == nil {
				subPath, err := r.readSoftObjectSubPath()
				if err == nil {
					return map[string]any{
						"packageName": packageName.Display(a.Names),
						"assetName":   assetName.Display(a.Names),
						"subPath":     subPath,
					}, true
				}
			}
		}
		_ = r.seek(start)
	}

	if tryLegacy {
		assetPathNameRef, err := r.readNameRef(len(a.Names))
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		subPath, err := r.readSoftObjectSubPath()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}

		assetPathName := assetPathNameRef.Display(a.Names)
		packageName, assetName := splitLegacySoftObjectAssetPath(assetPathName)
		return map[string]any{
			"assetPathName": assetPathName,
			"packageName":   packageName,
			"assetName":     assetName,
			"subPath":       subPath,
		}, true
	}

	_ = r.seek(start)
	return nil, false
}

func (a *Asset) decodeKnownStructFromReader(r *byteReader, structType string) (any, bool) {
	full := strings.TrimSpace(structType)
	if full == "" {
		full = "UnknownStruct"
	}
	normalized := normalizeStructTypeName(full)
	low := strings.ToLower(normalized)
	switch low {
	case "gameplaytagcontainer":
		start := r.offset()
		count, err := r.readInt32()
		if err != nil || count < 0 || count > maxTableEntries {
			_ = r.seek(start)
			return nil, false
		}
		items := make([]string, 0, count)
		for i := int32(0); i < count; i++ {
			ref, err := r.readNameRef(len(a.Names))
			if err != nil {
				_ = r.seek(start)
				return nil, false
			}
			items = append(items, ref.Display(a.Names))
		}
		return items, true
	case "framenumber":
		start := r.offset()
		v, err := r.readInt32()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		return map[string]any{
			"structType": normalized,
			"value": map[string]any{
				"value": v,
			},
		}, true
	case "perqualitylevelint":
		return a.decodePerQualityLevelStructFromReader(r, normalized, false)
	case "perqualitylevelfloat":
		return a.decodePerQualityLevelStructFromReader(r, normalized, true)
	case "perplatformint":
		return a.decodePerPlatformStructFromReader(r, normalized, perPlatformValueInt)
	case "perplatformfloat":
		return a.decodePerPlatformStructFromReader(r, normalized, perPlatformValueFloat)
	case "perplatformframerate":
		return a.decodePerPlatformStructFromReader(r, normalized, perPlatformValueFrameRate)
	case "uniquenetidrepl":
		return a.decodeUniqueNetIdReplFromReader(r, normalized)
	case "remoteobjectreference":
		return a.decodeRemoteObjectReferenceFromReader(r, normalized)
	case "animationattributeidentifier":
		return a.decodeAnimationAttributeIdentifierFromReader(r, normalized)
	case "levelviewportinfo":
		return a.decodeTaggedStructFromReader(r, normalized)
	case "niagaravariablebase":
		return a.decodeNiagaraVariableBaseFromReader(r, normalized)
	case "niagaratypedefinition":
		return a.decodeTaggedStructFromReader(r, "NiagaraTypeDefinition")
	case "softobjectpath", "softclasspath":
		return a.decodeSoftObjectPathFromReader(r)
	case "gameplaytag":
		v, ok := a.decodeNameRefFromReader(r)
		if !ok {
			return nil, false
		}
		name, _ := v.(map[string]any)["name"].(string)
		return name, true
	default:
		size := knownStructFixedSize(low)
		if size <= 0 {
			if isKnownTaggedStructDecodeCandidate(low) || isLikelyTaggedAssetStructType(full) {
				return a.decodeTaggedStructFromReader(r, full)
			}
			return nil, false
		}
		start := r.offset()
		raw, err := r.readBytes(size)
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		return a.decodeKnownStructFromBytes(normalized, raw)
	}
}

func isLikelyTaggedAssetStructType(structType string) bool {
	low := strings.ToLower(structType)
	return strings.Contains(low, "(/game/") || strings.Contains(low, "(/engine/")
}

func isKnownTaggedStructDecodeCandidate(structTypeLower string) bool {
	switch structTypeLower {
	case "levelviewportinfo",
		"animationattributeidentifier",
		"animnotifyevent",
		"animsyncmarker",
		"animcurvebase",
		"attributecurve",
		"floatcurve",
		"transformcurve",
		"rawanimsequencetrack",
		"interpcurvefloat",
		"interpcurvevector2d",
		"interpcurvevector",
		"interpcurvequat",
		"interpcurvetwovectors",
		"interpcurvelinearcolor",
		"interpcurvepointfloat",
		"interpcurvepointvector2d",
		"interpcurvepointvector",
		"interpcurvepointquat",
		"interpcurvepointtwovectors",
		"interpcurvepointlinearcolor",
		"editeddocumentinfo",
		"builderpoly",
		"bodyinstance",
		"collisionresponse",
		"responsechannel",
		"kaggregategeom",
		"ksphereelem",
		"kboxelem",
		"ksphylelem",
		"kconvexelem",
		"ktaperedcapsuleelem",
		"timeline",
		"timelinefloattrack",
		"timelinevectortrack",
		"timelinelinearcolortrack",
		"timelineevententry",
		"splinecurves",
		"runtimefloatcurve",
		"richcurve",
		"richcurvekey",
		"materialspriteelement",
		"scalarparametervalue",
		"vectorparametervalue",
		"textureparametervalue",
		"postprocesssettings",
		"edgraphpintype",
		"moviesceneeventparameters",
		"moviescenenumericvariant",
		"moviescenetimewarpvariant",
		"weightedblendables",
		"weightedblendable",
		"foliagedensityfalloff",
		"vehiclesteeringconfig",
		"niagarauserredirectionparameterstore",
		"niagaravariablebase",
		"niagaravariant",
		"remoteobjectreference":
		return true
	default:
		return false
	}
}

func (a *Asset) decodeTaggedStructFromReader(r *byteReader, structType string) (any, bool) {
	start := r.offset()
	remaining := r.remaining()
	if remaining < 8 {
		return nil, false
	}
	raw, err := r.readBytes(remaining)
	if err != nil {
		_ = r.seek(start)
		return nil, false
	}
	_ = r.seek(start)

	decoded, props, ok := a.decodeTaggedStructFromBytes(raw, structType)
	if !ok {
		_ = r.seek(start)
		return nil, false
	}
	if err := r.seek(start + props.EndOffset); err != nil {
		_ = r.seek(start)
		return nil, false
	}
	if len(props.Properties) == 0 && !isKnownTaggedStructDecodeCandidate(strings.ToLower(structType)) {
		_ = r.seek(start)
		return nil, false
	}

	return decoded, true
}

func (a *Asset) decodeNiagaraVariableBaseFromReader(r *byteReader, structType string) (any, bool) {
	start := r.offset()
	nameRef, ok := a.decodeNameRefFromReader(r)
	if !ok {
		_ = r.seek(start)
		return nil, false
	}

	typeDefHandle, ok := a.decodeTaggedStructFromReader(r, "NiagaraTypeDefinition")
	if !ok {
		_ = r.seek(start)
		return nil, false
	}

	name := ""
	if m, ok := nameRef.(map[string]any); ok {
		name, _ = m["name"].(string)
	}

	return map[string]any{
		"structType": structType,
		"value": map[string]any{
			"name":          name,
			"nameRef":       nameRef,
			"typeDefHandle": typeDefHandle,
		},
	}, true
}

func wrapDecodedStructFieldValue(typeNodes []PropertyTypeNode, names []NameEntry, decoded any) any {
	root, ok := buildDecodeTypeTree(typeNodes, names)
	if !ok || root == nil || root.Name != "StructProperty" {
		return decoded
	}
	if m, ok := decoded.(map[string]any); ok {
		if _, hasStructType := m["structType"]; hasStructType {
			if _, hasValue := m["value"]; hasValue {
				return decoded
			}
		}
	}

	structType := decodeStructTypeName(root)
	return map[string]any{
		"structType": structType,
		"value":      decoded,
	}
}

func decodeStructTypeName(node *decodeTypeNode) string {
	if node == nil || node.Name != "StructProperty" || len(node.Children) == 0 {
		return "UnknownStruct"
	}
	structNode := node.Children[0]
	base := strings.TrimSpace(structNode.Name)
	if base == "" {
		return "UnknownStruct"
	}
	if len(structNode.Children) == 0 {
		return base
	}
	firstArg := strings.ToLower(strings.TrimSpace(structNode.Children[0].Name))
	if strings.HasPrefix(firstArg, "/game/") || strings.HasPrefix(firstArg, "/engine/") {
		full := strings.TrimSpace(renderDecodeTypeNode(structNode))
		if full != "" {
			return full
		}
	}
	return base
}

func renderDecodeTypeNode(node *decodeTypeNode) string {
	if node == nil {
		return ""
	}
	name := strings.TrimSpace(node.Name)
	if len(node.Children) == 0 {
		return name
	}
	parts := make([]string, 0, len(node.Children))
	for _, child := range node.Children {
		parts = append(parts, renderDecodeTypeNode(child))
	}
	return name + "(" + strings.Join(parts, ",") + ")"
}

func normalizeStructTypeName(structType string) string {
	name := strings.TrimSpace(structType)
	if idx := strings.IndexByte(name, '('); idx >= 0 {
		name = strings.TrimSpace(name[:idx])
	}
	if name == "" {
		return "UnknownStruct"
	}
	return name
}

func knownStructFixedSize(structTypeLower string) int {
	switch structTypeLower {
	case "vector", "rotator", "vector_netquantize":
		return 24
	case "vector3d", "rotator3d":
		return 24
	case "quat", "vector4", "plane":
		return 32
	case "quat4d", "vector4d", "plane4d":
		return 32
	case "vector2d":
		return 16
	case "vector2f":
		return 8
	case "vector3f":
		return 12
	case "vector4f":
		return 16
	case "quat4f", "plane4f":
		return 16
	case "linearcolor":
		return 16
	case "color":
		return 4
	case "intpoint", "intvector2":
		return 8
	case "int32point", "int32vector2":
		return 8
	case "intvector":
		return 12
	case "int32vector":
		return 12
	case "framerate":
		return 8
	case "box":
		return 49
	case "box3f":
		return 25
	case "matrix":
		return 128
	case "twovectors":
		return 48
	case "guid":
		return 16
	case "datetime", "timespan":
		return 8
	case "framenumber":
		return 4
	case "gameplaytag":
		return 8
	case "softobjectpath", "softclasspath":
		return -1 // variable size
	case "floatrange":
		return 8
	default:
		return 0
	}
}

func (a *Asset) decodeKnownStructFromBytes(structType string, raw []byte) (any, bool) {
	if a.Summary.UsesByteSwappedSerialization() {
		// Fixed-size struct decoders below assume little-endian field layout.
		return nil, false
	}
	normalized := normalizeStructTypeName(structType)
	switch strings.ToLower(normalized) {
	case "vector", "vector_netquantize", "vector3d":
		return decodeVector3Doubles(raw)
	case "rotator", "rotator3d":
		return decodeRotatorDoubles(raw)
	case "quat", "vector4", "plane", "quat4d", "vector4d", "plane4d":
		return decodeVector4Doubles(raw)
	case "vector2d":
		return decodeVector2Doubles(raw)
	case "vector2f":
		return decodeVector2Floats(raw)
	case "vector3f":
		return decodeVector3Floats(raw)
	case "vector4f", "quat4f", "plane4f":
		return decodeVector4Floats(raw)
	case "linearcolor":
		return decodeLinearColor(raw)
	case "color":
		return decodeColor(raw)
	case "intpoint", "intvector2", "int32point", "int32vector2":
		return decodeIntVector2(raw)
	case "intvector", "int32vector":
		return decodeIntVector3(raw)
	case "box":
		return decodeBox(raw)
	case "box3f":
		return decodeBox3f(raw)
	case "matrix":
		return decodeMatrix(raw)
	case "twovectors":
		return decodeTwoVectors(raw)
	case "guid":
		return decodeGuid(raw)
	case "datetime", "timespan":
		return decodeTicks(raw)
	case "framenumber":
		return decodeFrameNumber(raw)
	case "framerate":
		return decodeFrameRate(raw)
	case "softobjectpath", "softclasspath":
		r := a.newAssetReader(raw)
		return a.decodeSoftObjectPathFromReader(r)
	case "gameplaytag":
		r := a.newAssetReader(raw)
		v, ok := a.decodeNameRefFromReader(r)
		if !ok {
			return nil, false
		}
		name, _ := v.(map[string]any)["name"].(string)
		return name, true
	case "gameplaytagcontainer":
		r := a.newAssetReader(raw)
		return a.decodeKnownStructFromReader(r, "gameplaytagcontainer")
	case "floatrange":
		return decodeFloatRange(raw)
	case "perqualitylevelint":
		r := a.newAssetReader(raw)
		v, ok := a.decodePerQualityLevelStructFromReader(r, normalized, false)
		if !ok {
			return nil, false
		}
		if m, ok := v.(map[string]any); ok {
			if inner, exists := m["value"]; exists {
				return inner, true
			}
		}
		return v, true
	case "perqualitylevelfloat":
		r := a.newAssetReader(raw)
		v, ok := a.decodePerQualityLevelStructFromReader(r, normalized, true)
		if !ok {
			return nil, false
		}
		if m, ok := v.(map[string]any); ok {
			if inner, exists := m["value"]; exists {
				return inner, true
			}
		}
		return v, true
	case "perplatformint":
		r := a.newAssetReader(raw)
		v, ok := a.decodePerPlatformStructFromReader(r, normalized, perPlatformValueInt)
		if !ok || r.remaining() != 0 {
			return nil, false
		}
		if m, ok := v.(map[string]any); ok {
			if inner, exists := m["value"]; exists {
				return inner, true
			}
		}
		return v, true
	case "perplatformfloat":
		r := a.newAssetReader(raw)
		v, ok := a.decodePerPlatformStructFromReader(r, normalized, perPlatformValueFloat)
		if !ok || r.remaining() != 0 {
			return nil, false
		}
		if m, ok := v.(map[string]any); ok {
			if inner, exists := m["value"]; exists {
				return inner, true
			}
		}
		return v, true
	case "perplatformframerate":
		r := a.newAssetReader(raw)
		v, ok := a.decodePerPlatformStructFromReader(r, normalized, perPlatformValueFrameRate)
		if !ok || r.remaining() != 0 {
			return nil, false
		}
		if m, ok := v.(map[string]any); ok {
			if inner, exists := m["value"]; exists {
				return inner, true
			}
		}
		return v, true
	case "uniquenetidrepl":
		r := a.newAssetReader(raw)
		v, ok := a.decodeUniqueNetIdReplFromReader(r, normalized)
		if !ok {
			return nil, false
		}
		if m, ok := v.(map[string]any); ok {
			if inner, exists := m["value"]; exists {
				return inner, true
			}
		}
		return v, true
	case "remoteobjectreference":
		r := a.newAssetReader(raw)
		v, ok := a.decodeRemoteObjectReferenceFromReader(r, normalized)
		if !ok || r.remaining() != 0 {
			return nil, false
		}
		if m, ok := v.(map[string]any); ok {
			if inner, exists := m["value"]; exists {
				return inner, true
			}
		}
		return v, true
	case "animationattributeidentifier":
		r := a.newAssetReader(raw)
		v, ok := a.decodeAnimationAttributeIdentifierFromReader(r, normalized)
		if !ok || r.remaining() != 0 {
			return nil, false
		}
		if m, ok := v.(map[string]any); ok {
			if inner, exists := m["value"]; exists {
				return inner, true
			}
		}
		return v, true
	case "niagaravariablebase":
		r := a.newAssetReader(raw)
		v, ok := a.decodeNiagaraVariableBaseFromReader(r, normalized)
		if !ok || r.remaining() != 0 {
			return nil, false
		}
		if m, ok := v.(map[string]any); ok {
			if inner, exists := m["value"]; exists {
				return inner, true
			}
		}
		return v, true
	default:
		return nil, false
	}
}

type perPlatformValueKind int

const (
	perPlatformValueInt perPlatformValueKind = iota
	perPlatformValueFloat
	perPlatformValueFrameRate
)

func (a *Asset) decodePerPlatformStructFromReader(r *byteReader, structType string, kind perPlatformValueKind) (any, bool) {
	start := r.offset()
	bCooked, err := r.readUBool()
	if err != nil {
		_ = r.seek(start)
		return nil, false
	}

	defaultEntry, ok := a.decodePerPlatformValueFromReader(r, kind)
	if !ok {
		_ = r.seek(start)
		return nil, false
	}

	entries := make([]map[string]any, 0)
	if !bCooked {
		count, err := r.readInt32()
		if err != nil || count < 0 || count > maxTableEntries {
			_ = r.seek(start)
			return nil, false
		}
		entries = make([]map[string]any, 0, count)
		for i := int32(0); i < count; i++ {
			keyRef, err := r.readNameRef(len(a.Names))
			if err != nil {
				_ = r.seek(start)
				return nil, false
			}
			valueEntry, ok := a.decodePerPlatformValueFromReader(r, kind)
			if !ok {
				_ = r.seek(start)
				return nil, false
			}
			entries = append(entries, map[string]any{
				"key": map[string]any{
					"type": "NameProperty",
					"value": map[string]any{
						"index":  keyRef.Index,
						"number": keyRef.Number,
						"name":   keyRef.Display(a.Names),
					},
				},
				"value": valueEntry,
			})
		}
	}

	valueType := "IntProperty"
	switch kind {
	case perPlatformValueFloat:
		valueType = "FloatProperty"
	case perPlatformValueFrameRate:
		valueType = "StructProperty(FrameRate)"
	}

	return map[string]any{
		"structType": structType,
		"value": map[string]any{
			"bCooked": map[string]any{
				"type":  "BoolProperty",
				"value": bCooked,
			},
			"Default": defaultEntry,
			"PerPlatform": map[string]any{
				"type": "MapProperty(NameProperty," + valueType + ")",
				"value": map[string]any{
					"keyType":   "NameProperty",
					"valueType": valueType,
					"value":     entries,
				},
			},
		},
	}, true
}

func (a *Asset) decodePerPlatformValueFromReader(r *byteReader, kind perPlatformValueKind) (map[string]any, bool) {
	switch kind {
	case perPlatformValueInt:
		v, err := r.readInt32()
		if err != nil {
			return nil, false
		}
		return map[string]any{
			"type":  "IntProperty",
			"value": v,
		}, true
	case perPlatformValueFloat:
		bits, err := r.readUint32()
		if err != nil {
			return nil, false
		}
		return map[string]any{
			"type":  "FloatProperty",
			"value": math.Float32frombits(bits),
		}, true
	case perPlatformValueFrameRate:
		v, ok := decodeFrameRateFromReader(r)
		if !ok {
			return nil, false
		}
		return map[string]any{
			"type": "StructProperty(FrameRate)",
			"value": map[string]any{
				"structType": "FrameRate",
				"value":      v,
			},
		}, true
	default:
		return nil, false
	}
}

func (a *Asset) decodeRemoteObjectReferenceFromReader(r *byteReader, structType string) (any, bool) {
	start := r.offset()
	rawID, err := r.readBytes(8)
	if err != nil {
		_ = r.seek(start)
		return nil, false
	}
	objectID := r.byteOrder().Uint64(rawID)
	serverID, err := r.readUint32()
	if err != nil {
		_ = r.seek(start)
		return nil, false
	}
	return map[string]any{
		"structType": structType,
		"value": map[string]any{
			"ObjectId": map[string]any{
				"type":  "UInt64Property",
				"value": objectID,
			},
			"ServerId": map[string]any{
				"type":  "UInt32Property",
				"value": serverID,
			},
		},
	}, true
}

func (a *Asset) decodeAnimationAttributeIdentifierFromReader(r *byteReader, structType string) (any, bool) {
	start := r.offset()
	nameRef, ok := a.decodeNameRefFromReader(r)
	if !ok {
		_ = r.seek(start)
		return nil, false
	}
	boneNameRef, ok := a.decodeNameRefFromReader(r)
	if !ok {
		_ = r.seek(start)
		return nil, false
	}
	boneIndex, err := r.readInt32()
	if err != nil {
		_ = r.seek(start)
		return nil, false
	}
	scriptStructPath, ok := a.decodeSoftObjectPathFromReader(r)
	if !ok {
		_ = r.seek(start)
		return nil, false
	}
	return map[string]any{
		"structType": structType,
		"value": map[string]any{
			"Name": map[string]any{
				"type":  "NameProperty",
				"value": nameRef,
			},
			"BoneName": map[string]any{
				"type":  "NameProperty",
				"value": boneNameRef,
			},
			"BoneIndex": map[string]any{
				"type":  "IntProperty",
				"value": boneIndex,
			},
			"ScriptStructPath": map[string]any{
				"type":  "StructProperty(SoftObjectPath)",
				"value": scriptStructPath,
			},
		},
	}, true
}

func (a *Asset) decodeTaggedStructFromBytes(raw []byte, structType string) (any, PropertyListResult, bool) {
	tmp := &Asset{
		Raw:     RawAsset{Bytes: raw},
		Summary: a.Summary,
		Names:   a.Names,
	}
	props := tmp.ParseTaggedPropertiesRange(0, len(raw), false)
	if len(props.Warnings) > 0 || props.EndOffset <= 0 || props.EndOffset > len(raw) {
		return nil, props, false
	}
	fields := tmp.decodeTaggedStructFields(props.Properties)
	return map[string]any{
		"structType": structType,
		"value":      fields,
	}, props, true
}

func (a *Asset) decodeTaggedStructFields(props []PropertyTag) map[string]any {
	fields := map[string]any{}
	for _, p := range props {
		entry := map[string]any{"type": p.TypeString(a.Names)}
		if val, ok := a.DecodePropertyValue(p); ok {
			entry["value"] = wrapDecodedStructFieldValue(p.TypeNodes, a.Names, val)
		}
		fields[p.Name.Display(a.Names)] = entry
	}
	return fields
}

func (a *Asset) decodePerQualityLevelStructFromReader(r *byteReader, structType string, valueIsFloat bool) (any, bool) {
	start := r.offset()
	bCooked, err := r.readUBool()
	if err != nil {
		_ = r.seek(start)
		return nil, false
	}
	defaultEntry := map[string]any{"type": "IntProperty"}
	if valueIsFloat {
		defaultEntry["type"] = "FloatProperty"
		v, err := r.readUint32()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		defaultEntry["value"] = math.Float32frombits(v)
	} else {
		v, err := r.readInt32()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		defaultEntry["value"] = v
	}
	count, err := r.readInt32()
	if err != nil || count < 0 || count > maxTableEntries {
		_ = r.seek(start)
		return nil, false
	}
	entries := make([]map[string]any, 0, count)
	for i := int32(0); i < count; i++ {
		key, err := r.readInt32()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		valWrapper := map[string]any{"type": "IntProperty"}
		if valueIsFloat {
			bits, err := r.readUint32()
			if err != nil {
				_ = r.seek(start)
				return nil, false
			}
			valWrapper["type"] = "FloatProperty"
			valWrapper["value"] = math.Float32frombits(bits)
		} else {
			v, err := r.readInt32()
			if err != nil {
				_ = r.seek(start)
				return nil, false
			}
			valWrapper["value"] = v
		}
		entries = append(entries, map[string]any{
			"key": map[string]any{
				"type":  "IntProperty",
				"value": key,
			},
			"value": valWrapper,
		})
	}
	return map[string]any{
		"structType": structType,
		"value": map[string]any{
			"bCooked": map[string]any{
				"type":  "BoolProperty",
				"value": bCooked,
			},
			"Default": defaultEntry,
			"PerQuality": map[string]any{
				"type": "MapProperty(IntProperty," + defaultEntry["type"].(string) + ")",
				"value": map[string]any{
					"keyType":   "IntProperty",
					"valueType": defaultEntry["type"],
					"value":     entries,
				},
			},
		},
	}, true
}

func (a *Asset) decodeUniqueNetIdReplFromReader(r *byteReader, structType string) (any, bool) {
	start := r.offset()
	size, err := r.readInt32()
	if err != nil || size < 0 {
		_ = r.seek(start)
		return nil, false
	}

	fields := map[string]any{
		"Size": map[string]any{
			"type":  "IntProperty",
			"value": size,
		},
	}
	if size > 0 {
		typeRef, err := r.readNameRef(len(a.Names))
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		contents, err := r.readFString()
		if err != nil {
			_ = r.seek(start)
			return nil, false
		}
		fields["Type"] = map[string]any{
			"type": "NameProperty",
			"value": map[string]any{
				"index":  typeRef.Index,
				"number": typeRef.Number,
				"name":   typeRef.Display(a.Names),
			},
		}
		fields["Contents"] = map[string]any{
			"type":  "StrProperty",
			"value": contents,
		}
	}
	return map[string]any{
		"structType": structType,
		"value":      fields,
	}, true
}

func (a *Asset) decodeDelegateFromReader(r *byteReader) (any, bool) {
	obj, ok := a.decodePackageIndexFromReader(r)
	if !ok {
		return nil, false
	}
	name, ok := a.decodeNameRefFromReader(r)
	if !ok {
		return nil, false
	}
	nameMap, _ := name.(map[string]any)
	objMap, _ := obj.(map[string]any)
	return map[string]any{
		"object":   objMap["index"],
		"resolved": objMap["resolved"],
		"delegate": nameMap["name"],
	}, true
}

func (a *Asset) decodeMulticastDelegateFromReader(r *byteReader) (any, bool) {
	count, err := r.readInt32()
	if err != nil || count < 0 || count > maxTableEntries {
		return nil, false
	}
	items := make([]any, 0, count)
	for i := int32(0); i < count; i++ {
		v, ok := a.decodeDelegateFromReader(r)
		if !ok {
			return nil, false
		}
		items = append(items, v)
	}
	return items, true
}

func (a *Asset) decodeFieldPathPropertyFromReader(r *byteReader) (any, bool) {
	count, err := r.readInt32()
	if err != nil || count < 0 || count > maxTableEntries {
		return nil, false
	}
	path := make([]string, 0, count)
	for i := int32(0); i < count; i++ {
		v, ok := a.decodeNameRefFromReader(r)
		if !ok {
			return nil, false
		}
		m, _ := v.(map[string]any)
		path = append(path, m["name"].(string))
	}
	owner, ok := a.decodePackageIndexFromReader(r)
	if !ok {
		return nil, false
	}
	own, _ := owner.(map[string]any)
	return map[string]any{
		"path":          path,
		"owner":         own["index"],
		"resolvedOwner": own["resolved"],
	}, true
}

func decodeVector2Doubles(raw []byte) (any, bool) {
	if len(raw) != 16 {
		return nil, false
	}
	return map[string]any{
		"x": math.Float64frombits(binary.LittleEndian.Uint64(raw[0:8])),
		"y": math.Float64frombits(binary.LittleEndian.Uint64(raw[8:16])),
	}, true
}

func decodeVector3Doubles(raw []byte) (any, bool) {
	if len(raw) != 24 {
		return nil, false
	}
	return map[string]any{
		"x": math.Float64frombits(binary.LittleEndian.Uint64(raw[0:8])),
		"y": math.Float64frombits(binary.LittleEndian.Uint64(raw[8:16])),
		"z": math.Float64frombits(binary.LittleEndian.Uint64(raw[16:24])),
	}, true
}

func decodeRotatorDoubles(raw []byte) (any, bool) {
	if len(raw) != 24 {
		return nil, false
	}
	return map[string]any{
		"pitch": math.Float64frombits(binary.LittleEndian.Uint64(raw[0:8])),
		"yaw":   math.Float64frombits(binary.LittleEndian.Uint64(raw[8:16])),
		"roll":  math.Float64frombits(binary.LittleEndian.Uint64(raw[16:24])),
	}, true
}

func decodeVector4Doubles(raw []byte) (any, bool) {
	if len(raw) != 32 {
		return nil, false
	}
	return map[string]any{
		"x": math.Float64frombits(binary.LittleEndian.Uint64(raw[0:8])),
		"y": math.Float64frombits(binary.LittleEndian.Uint64(raw[8:16])),
		"z": math.Float64frombits(binary.LittleEndian.Uint64(raw[16:24])),
		"w": math.Float64frombits(binary.LittleEndian.Uint64(raw[24:32])),
	}, true
}

func decodeVector2Floats(raw []byte) (any, bool) {
	if len(raw) != 8 {
		return nil, false
	}
	return map[string]any{
		"x": math.Float32frombits(binary.LittleEndian.Uint32(raw[0:4])),
		"y": math.Float32frombits(binary.LittleEndian.Uint32(raw[4:8])),
	}, true
}

func decodeVector3Floats(raw []byte) (any, bool) {
	if len(raw) != 12 {
		return nil, false
	}
	return map[string]any{
		"x": math.Float32frombits(binary.LittleEndian.Uint32(raw[0:4])),
		"y": math.Float32frombits(binary.LittleEndian.Uint32(raw[4:8])),
		"z": math.Float32frombits(binary.LittleEndian.Uint32(raw[8:12])),
	}, true
}

func decodeVector4Floats(raw []byte) (any, bool) {
	if len(raw) != 16 {
		return nil, false
	}
	return map[string]any{
		"x": math.Float32frombits(binary.LittleEndian.Uint32(raw[0:4])),
		"y": math.Float32frombits(binary.LittleEndian.Uint32(raw[4:8])),
		"z": math.Float32frombits(binary.LittleEndian.Uint32(raw[8:12])),
		"w": math.Float32frombits(binary.LittleEndian.Uint32(raw[12:16])),
	}, true
}

func decodeLinearColor(raw []byte) (any, bool) {
	if len(raw) != 16 {
		return nil, false
	}
	return map[string]any{
		"r": math.Float32frombits(binary.LittleEndian.Uint32(raw[0:4])),
		"g": math.Float32frombits(binary.LittleEndian.Uint32(raw[4:8])),
		"b": math.Float32frombits(binary.LittleEndian.Uint32(raw[8:12])),
		"a": math.Float32frombits(binary.LittleEndian.Uint32(raw[12:16])),
	}, true
}

func decodeColor(raw []byte) (any, bool) {
	if len(raw) != 4 {
		return nil, false
	}
	return map[string]any{
		"r": raw[2],
		"g": raw[1],
		"b": raw[0],
		"a": raw[3],
	}, true
}

func decodeIntVector2(raw []byte) (any, bool) {
	if len(raw) != 8 {
		return nil, false
	}
	return map[string]any{
		"x": int32(binary.LittleEndian.Uint32(raw[0:4])),
		"y": int32(binary.LittleEndian.Uint32(raw[4:8])),
	}, true
}

func decodeIntVector3(raw []byte) (any, bool) {
	if len(raw) != 12 {
		return nil, false
	}
	return map[string]any{
		"x": int32(binary.LittleEndian.Uint32(raw[0:4])),
		"y": int32(binary.LittleEndian.Uint32(raw[4:8])),
		"z": int32(binary.LittleEndian.Uint32(raw[8:12])),
	}, true
}

func decodeBox(raw []byte) (any, bool) {
	if len(raw) != 49 {
		return nil, false
	}
	min, ok := decodeVector3Doubles(raw[0:24])
	if !ok {
		return nil, false
	}
	max, ok := decodeVector3Doubles(raw[24:48])
	if !ok {
		return nil, false
	}
	return map[string]any{
		"min":     min,
		"max":     max,
		"isValid": raw[48],
	}, true
}

func decodeBox3f(raw []byte) (any, bool) {
	if len(raw) != 25 {
		return nil, false
	}
	min, ok := decodeVector3Floats(raw[0:12])
	if !ok {
		return nil, false
	}
	max, ok := decodeVector3Floats(raw[12:24])
	if !ok {
		return nil, false
	}
	return map[string]any{
		"min":     min,
		"max":     max,
		"isValid": raw[24],
	}, true
}

func decodeMatrix(raw []byte) (any, bool) {
	if len(raw) != 128 {
		return nil, false
	}
	plane := func(offset int) map[string]any {
		return map[string]any{
			"x": math.Float64frombits(binary.LittleEndian.Uint64(raw[offset : offset+8])),
			"y": math.Float64frombits(binary.LittleEndian.Uint64(raw[offset+8 : offset+16])),
			"z": math.Float64frombits(binary.LittleEndian.Uint64(raw[offset+16 : offset+24])),
			"w": math.Float64frombits(binary.LittleEndian.Uint64(raw[offset+24 : offset+32])),
		}
	}
	return map[string]any{
		"xPlane": plane(0),
		"yPlane": plane(32),
		"zPlane": plane(64),
		"wPlane": plane(96),
	}, true
}

func decodeTwoVectors(raw []byte) (any, bool) {
	if len(raw) != 48 {
		return nil, false
	}
	v1, ok := decodeVector3Doubles(raw[0:24])
	if !ok {
		return nil, false
	}
	v2, ok := decodeVector3Doubles(raw[24:48])
	if !ok {
		return nil, false
	}
	return map[string]any{
		"v1": v1,
		"v2": v2,
	}, true
}

func decodeGuid(raw []byte) (any, bool) {
	if len(raw) != 16 {
		return nil, false
	}
	var g GUID
	copy(g[:], raw)
	return g.String(), true
}

func decodeTicks(raw []byte) (any, bool) {
	if len(raw) != 8 {
		return nil, false
	}
	return map[string]any{
		"ticks": int64(binary.LittleEndian.Uint64(raw)),
	}, true
}

func decodeFrameNumber(raw []byte) (any, bool) {
	if len(raw) != 4 {
		return nil, false
	}
	return map[string]any{
		"value": int32(binary.LittleEndian.Uint32(raw)),
	}, true
}

func decodeFrameRateFromReader(r *byteReader) (any, bool) {
	numerator, err := r.readInt32()
	if err != nil {
		return nil, false
	}
	denominator, err := r.readInt32()
	if err != nil {
		return nil, false
	}
	return map[string]any{
		"Numerator":   numerator,
		"Denominator": denominator,
	}, true
}

func decodeFrameRate(raw []byte) (any, bool) {
	if len(raw) != 8 {
		return nil, false
	}
	return map[string]any{
		"Numerator":   int32(binary.LittleEndian.Uint32(raw[0:4])),
		"Denominator": int32(binary.LittleEndian.Uint32(raw[4:8])),
	}, true
}

func decodeFloatRange(raw []byte) (any, bool) {
	if len(raw) != 8 {
		return nil, false
	}
	return map[string]any{
		"lowerBound": math.Float32frombits(binary.LittleEndian.Uint32(raw[0:4])),
		"upperBound": math.Float32frombits(binary.LittleEndian.Uint32(raw[4:8])),
	}, true
}
