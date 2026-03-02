package uasset

import (
	"fmt"
	"strings"
)

const (
	propertyFlagHasArrayIndex         = uint8(0x01)
	propertyFlagHasPropertyGUID       = uint8(0x02)
	propertyFlagHasPropertyExtensions = uint8(0x04)
	propertyFlagHasBinaryOrNative     = uint8(0x08)
	propertyFlagBoolTrue              = uint8(0x10)
	propertyFlagSkippedSerialize      = uint8(0x20)
	propertyExtensionOverridableInfo  = uint8(0x02)

	classSerializationControlOverridableInfo = uint8(0x02)

	maxPropertyTypeNodes = 8192
)

// PropertyTypeNode is one node in UE::FPropertyTypeName serialized stream.
type PropertyTypeNode struct {
	Name       NameRef `json:"name"`
	InnerCount int32   `json:"innerCount"`
}

// PropertyTag represents one top-level property tag in tagged property serialization.
type PropertyTag struct {
	Name                         NameRef            `json:"name"`
	TypeNodes                    []PropertyTypeNode `json:"typeNodes"`
	Size                         int32              `json:"size"`
	Flags                        uint8              `json:"flags"`
	ArrayIndex                   int32              `json:"arrayIndex"`
	StructGUID                   *GUID              `json:"structGuid,omitempty"`
	PropertyGUID                 *GUID              `json:"propertyGuid,omitempty"`
	PropertyExtensions           uint8              `json:"propertyExtensions,omitempty"`
	OverridableOperation         uint8              `json:"overridableOperation,omitempty"`
	ExperimentalOverridableLogic bool               `json:"experimentalOverridableLogic,omitempty"`
	Offset                       int                `json:"offset"`
	ValueOffset                  int                `json:"valueOffset"`
}

// TypeString renders the nested type name in a stable textual format.
func (p PropertyTag) TypeString(names []NameEntry) string {
	if len(p.TypeNodes) == 0 {
		return ""
	}
	i := 0
	var build func() string
	build = func() string {
		if i >= len(p.TypeNodes) {
			return ""
		}
		n := p.TypeNodes[i]
		i++
		name := n.Name.Display(names)
		if n.InnerCount <= 0 {
			return name
		}
		parts := make([]string, 0, n.InnerCount)
		for j := int32(0); j < n.InnerCount && i <= len(p.TypeNodes); j++ {
			parts = append(parts, build())
		}
		return fmt.Sprintf("%s(%s)", name, strings.Join(parts, ","))
	}
	return build()
}

// PropertyListResult wraps parsed properties and parse warnings.
type PropertyListResult struct {
	Properties []PropertyTag `json:"properties"`
	Warnings   []string      `json:"warnings"`
	EndOffset  int           `json:"endOffset"`
}

type parsedPropertyTagHeader struct {
	TypeNodes                    []PropertyTypeNode
	Size                         int32
	Flags                        uint8
	ArrayIndex                   int32
	StructGUID                   *GUID
	PropertyGUID                 *GUID
	PropertyExtensions           uint8
	OverridableOperation         uint8
	ExperimentalOverridableLogic bool
}

// ParseExportProperties parses top-level tagged properties for one export.
func (a *Asset) ParseExportProperties(exportIndex int) PropertyListResult {
	result := PropertyListResult{}
	if exportIndex < 0 || exportIndex >= len(a.Exports) {
		result.Warnings = append(result.Warnings, "export index out of range")
		return result
	}
	exp := a.Exports[exportIndex]
	if exp.SerialOffset < 0 || exp.SerialSize < 0 {
		result.Warnings = append(result.Warnings, "invalid export serial range")
		return result
	}
	start := exp.SerialOffset
	end := exp.SerialOffset + exp.SerialSize
	if start < 0 || end < start || end > int64(len(a.Raw.Bytes)) {
		result.Warnings = append(result.Warnings, "export serial range out of file bounds")
		return result
	}

	propertyStart := int(start)
	propertyEnd := int(end)
	if exp.ScriptSerializationEndOffset > exp.ScriptSerializationStartOffset &&
		exp.ScriptSerializationStartOffset >= 0 &&
		exp.ScriptSerializationEndOffset <= exp.SerialSize {
		propertyStart = int(start + exp.ScriptSerializationStartOffset)
		propertyEnd = int(start + exp.ScriptSerializationEndOffset)
	}

	needsClassControl := !a.Summary.UsesUnversionedPropertySerialization() && a.Summary.FileVersionUE5 >= ue5PropertyTagExtension
	return a.parseTaggedPropertiesRange(propertyStart, propertyEnd, needsClassControl)
}

// ParseTaggedPropertiesRange parses tagged properties in one byte range.
func (a *Asset) ParseTaggedPropertiesRange(startOffset, endOffset int, withClassSerializationControl bool) PropertyListResult {
	return a.parseTaggedPropertiesRange(startOffset, endOffset, withClassSerializationControl)
}

func (a *Asset) parseTaggedPropertiesRange(startOffset, endOffset int, withClassSerializationControl bool) PropertyListResult {
	result := PropertyListResult{EndOffset: startOffset}
	if startOffset < 0 || endOffset < startOffset || endOffset > len(a.Raw.Bytes) {
		result.Warnings = append(result.Warnings, "property parse range out of bounds")
		return result
	}

	r := NewByteReaderWithByteSwapping(a.Raw.Bytes[startOffset:endOffset], a.Summary.UsesByteSwappedSerialization())
	if withClassSerializationControl {
		control, err := r.readUint8()
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("property parse stopped: read serialization control extension: %v", err))
			result.EndOffset = startOffset + r.offset()
			return result
		}
		// UE5.6 class-level serialization control may include bits without payload
		// (e.g. ReserveForFutureUse). Match engine behavior and ignore unknown bits.
		if control&classSerializationControlOverridableInfo != 0 {
			// UE5.6 UStruct::SerializeVersionedTaggedProperties writes only OverridableOperation (uint8)
			// for class-level SerializationControlExtensions.
			if _, err := r.readUint8(); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("property parse stopped: read overridable operation: %v", err))
				result.EndOffset = startOffset + r.offset()
				return result
			}
		}
	}

	for !r.eof() {
		tagStart := r.offset()
		name, err := r.readNameRef(len(a.Names))
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("property parse stopped: %v", err))
			break
		}
		if name.IsNone(a.Names) {
			break
		}

		tagHeader, err := a.parsePropertyTagHeader(r)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("property tag parse stopped: %v", err))
			break
		}

		valueOffset := r.offset()
		if tagHeader.Size < 0 {
			result.Warnings = append(result.Warnings, "negative property size")
			break
		}
		if tagHeader.Flags&propertyFlagSkippedSerialize == 0 {
			if err := r.skip(int(tagHeader.Size)); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("property value skip stopped: %v", err))
				break
			}
		}

		result.Properties = append(result.Properties, PropertyTag{
			Name:                         name,
			TypeNodes:                    tagHeader.TypeNodes,
			Size:                         tagHeader.Size,
			Flags:                        tagHeader.Flags,
			ArrayIndex:                   tagHeader.ArrayIndex,
			StructGUID:                   tagHeader.StructGUID,
			PropertyGUID:                 tagHeader.PropertyGUID,
			PropertyExtensions:           tagHeader.PropertyExtensions,
			OverridableOperation:         tagHeader.OverridableOperation,
			ExperimentalOverridableLogic: tagHeader.ExperimentalOverridableLogic,
			Offset:                       startOffset + tagStart,
			ValueOffset:                  startOffset + valueOffset,
		})
	}
	result.EndOffset = startOffset + r.offset()
	return result
}

func (a *Asset) parsePropertyTagHeader(r *byteReader) (parsedPropertyTagHeader, error) {
	if a.Summary.SupportsPropertyTagCompleteTypeName() {
		return a.parsePropertyTagHeaderComplete(r)
	}
	return a.parsePropertyTagHeaderLegacy(r)
}

func (a *Asset) parsePropertyTagHeaderComplete(r *byteReader) (parsedPropertyTagHeader, error) {
	var out parsedPropertyTagHeader
	typeNodes, err := parsePropertyTypeNameNodes(r, len(a.Names))
	if err != nil {
		return out, fmt.Errorf("property type parse: %w", err)
	}
	out.TypeNodes = typeNodes

	size, err := r.readInt32()
	if err != nil {
		return out, fmt.Errorf("property size parse: %w", err)
	}
	out.Size = size

	flags, err := r.readUint8()
	if err != nil {
		return out, fmt.Errorf("property flags parse: %w", err)
	}
	out.Flags = flags

	if flags&propertyFlagHasArrayIndex != 0 {
		arrayIndex, err := r.readInt32()
		if err != nil {
			return out, fmt.Errorf("property array index parse: %w", err)
		}
		out.ArrayIndex = arrayIndex
	}

	if flags&propertyFlagHasPropertyGUID != 0 {
		guid, err := r.readGUID()
		if err != nil {
			return out, fmt.Errorf("property guid parse: %w", err)
		}
		out.PropertyGUID = &guid
	}

	if flags&propertyFlagHasPropertyExtensions != 0 {
		extensions, err := r.readUint8()
		if err != nil {
			return out, fmt.Errorf("property extension flags parse: %w", err)
		}
		out.PropertyExtensions = extensions
		if extensions&propertyExtensionOverridableInfo != 0 {
			overridableOperation, err := r.readUint8()
			if err != nil {
				return out, fmt.Errorf("property override op parse: %w", err)
			}
			experimentalOverridableLogic, err := r.readUBool()
			if err != nil {
				return out, fmt.Errorf("property override bool parse: %w", err)
			}
			out.OverridableOperation = overridableOperation
			out.ExperimentalOverridableLogic = experimentalOverridableLogic
		}
	}

	return out, nil
}

func (a *Asset) parsePropertyTagHeaderLegacy(r *byteReader) (parsedPropertyTagHeader, error) {
	var out parsedPropertyTagHeader
	typeName, err := r.readNameRef(len(a.Names))
	if err != nil {
		return out, fmt.Errorf("legacy property type parse: %w", err)
	}
	typeNameText := typeName.Display(a.Names)
	out.TypeNodes = []PropertyTypeNode{
		{Name: typeName, InnerCount: 0},
	}

	size, err := r.readInt32()
	if err != nil {
		return out, fmt.Errorf("legacy property size parse: %w", err)
	}
	out.Size = size

	arrayIndex, err := r.readInt32()
	if err != nil {
		return out, fmt.Errorf("legacy property array index parse: %w", err)
	}
	out.ArrayIndex = arrayIndex
	if arrayIndex != 0 {
		out.Flags |= propertyFlagHasArrayIndex
	}

	appendChild := func(ref NameRef) {
		out.TypeNodes[0].InnerCount++
		out.TypeNodes = append(out.TypeNodes, PropertyTypeNode{Name: ref, InnerCount: 0})
	}

	switch typeNameText {
	case "StructProperty":
		structName, err := r.readNameRef(len(a.Names))
		if err != nil {
			return out, fmt.Errorf("legacy StructProperty structName parse: %w", err)
		}
		appendChild(structName)
		structGUID, err := r.readGUID()
		if err != nil {
			return out, fmt.Errorf("legacy StructProperty structGuid parse: %w", err)
		}
		if !guidIsZero(structGUID) {
			g := structGUID
			out.StructGUID = &g
		}
	case "BoolProperty":
		boolVal, err := r.readUint8()
		if err != nil {
			return out, fmt.Errorf("legacy BoolProperty bool parse: %w", err)
		}
		if boolVal != 0 {
			out.Flags |= propertyFlagBoolTrue
		}
	case "ByteProperty":
		enumName, err := r.readNameRef(len(a.Names))
		if err != nil {
			return out, fmt.Errorf("legacy ByteProperty enum parse: %w", err)
		}
		if !enumName.IsNone(a.Names) {
			appendChild(enumName)
		}
	case "EnumProperty":
		enumName, err := r.readNameRef(len(a.Names))
		if err != nil {
			return out, fmt.Errorf("legacy EnumProperty enum parse: %w", err)
		}
		appendChild(enumName)
		if byteTypeRef, ok := findNameRefByValue(a.Names, "ByteProperty"); ok {
			appendChild(byteTypeRef)
		}
	case "ArrayProperty", "OptionalProperty", "SetProperty":
		innerType, err := r.readNameRef(len(a.Names))
		if err != nil {
			return out, fmt.Errorf("legacy %s inner type parse: %w", typeNameText, err)
		}
		appendChild(innerType)
	case "MapProperty":
		innerType, err := r.readNameRef(len(a.Names))
		if err != nil {
			return out, fmt.Errorf("legacy MapProperty key type parse: %w", err)
		}
		valueType, err := r.readNameRef(len(a.Names))
		if err != nil {
			return out, fmt.Errorf("legacy MapProperty value type parse: %w", err)
		}
		appendChild(innerType)
		appendChild(valueType)
	}

	hasPropertyGUID, err := r.readUint8()
	if err != nil {
		return out, fmt.Errorf("legacy property guid flag parse: %w", err)
	}
	if hasPropertyGUID != 0 {
		guid, err := r.readGUID()
		if err != nil {
			return out, fmt.Errorf("legacy property guid parse: %w", err)
		}
		out.PropertyGUID = &guid
		out.Flags |= propertyFlagHasPropertyGUID
	}

	if a.Summary.FileVersionUE5 >= ue5PropertyTagExtension {
		extensions, err := r.readUint8()
		if err != nil {
			return out, fmt.Errorf("legacy property extension flags parse: %w", err)
		}
		out.PropertyExtensions = extensions
		if extensions != 0 {
			out.Flags |= propertyFlagHasPropertyExtensions
		}
		if extensions&propertyExtensionOverridableInfo != 0 {
			overridableOperation, err := r.readUint8()
			if err != nil {
				return out, fmt.Errorf("legacy property override op parse: %w", err)
			}
			experimentalOverridableLogic, err := r.readUBool()
			if err != nil {
				return out, fmt.Errorf("legacy property override bool parse: %w", err)
			}
			out.OverridableOperation = overridableOperation
			out.ExperimentalOverridableLogic = experimentalOverridableLogic
		}
	}

	return out, nil
}

func findNameRefByValue(names []NameEntry, value string) (NameRef, bool) {
	for i := range names {
		if names[i].Value == value {
			return NameRef{Index: int32(i), Number: 0}, true
		}
	}
	return NameRef{}, false
}

func guidIsZero(g GUID) bool {
	for _, b := range g {
		if b != 0 {
			return false
		}
	}
	return true
}

func parsePropertyTypeNameNodes(r *byteReader, nameCount int) ([]PropertyTypeNode, error) {
	nodes := make([]PropertyTypeNode, 0, 4)
	remaining := 1
	for remaining > 0 {
		if len(nodes) >= maxPropertyTypeNodes {
			return nil, fmt.Errorf("property type graph exceeds limit (%d)", maxPropertyTypeNodes)
		}
		name, err := r.readNameRef(nameCount)
		if err != nil {
			return nil, fmt.Errorf("read type node name: %w", err)
		}
		innerCount, err := r.readInt32()
		if err != nil {
			return nil, fmt.Errorf("read type node inner count: %w", err)
		}
		if innerCount < 0 {
			return nil, fmt.Errorf("invalid negative inner count: %d", innerCount)
		}
		nodes = append(nodes, PropertyTypeNode{Name: name, InnerCount: innerCount})
		remaining += int(innerCount) - 1
		if remaining > 100000 {
			return nil, fmt.Errorf("unreasonable property type graph size")
		}
	}
	return nodes, nil
}
