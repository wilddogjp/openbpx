package cli

import (
	"fmt"
	"strings"

	"github.com/wilddogjp/bpx/pkg/edit"
	"github.com/wilddogjp/bpx/pkg/uasset"
)

// buildPropertySetStructLeafFallbackMutation applies a best-effort fallback for
// custom StructProperty leaf edits when pkg/edit cannot re-encode the full struct.
// Current fallback scope is fixed-size in-place payload updates only.
func buildPropertySetStructLeafFallbackMutation(asset *uasset.Asset, exportIndex int, path string, valueJSON string) (*edit.PropertySetResult, error) {
	path = strings.TrimSpace(path)
	dot := strings.Index(path, ".")
	if dot <= 0 || dot >= len(path)-1 {
		return nil, fmt.Errorf("path is not a struct leaf path")
	}
	rootName := strings.TrimSpace(path[:dot])
	innerPath := strings.TrimSpace(path[dot+1:])
	if rootName == "" || innerPath == "" {
		return nil, fmt.Errorf("path is not a struct leaf path")
	}
	if strings.Contains(rootName, "[") {
		return nil, fmt.Errorf("fallback does not support indexed root path")
	}

	exp := asset.Exports[exportIndex]
	serialStart := int(exp.SerialOffset)
	serialEnd := int(exp.SerialOffset + exp.SerialSize)
	if serialStart < 0 || serialEnd < serialStart || serialEnd > len(asset.Raw.Bytes) {
		return nil, fmt.Errorf("export serial range out of bounds")
	}

	props := asset.ParseExportProperties(exportIndex)
	if len(props.Warnings) > 0 {
		return nil, fmt.Errorf("cannot safely edit export properties: %s", strings.Join(props.Warnings, "; "))
	}

	var rootTag *uasset.PropertyTag
	for i := range props.Properties {
		p := &props.Properties[i]
		if p.Name.Display(asset.Names) != rootName || p.ArrayIndex != 0 {
			continue
		}
		rootTag = p
		break
	}
	if rootTag == nil {
		return nil, fmt.Errorf("property not found: %s", rootName)
	}
	if len(rootTag.TypeNodes) == 0 || rootTag.TypeNodes[0].Name.Display(asset.Names) != "StructProperty" {
		return nil, fmt.Errorf("property %s is not StructProperty", rootName)
	}

	valueStart := rootTag.ValueOffset
	valueEnd := valueStart + int(rootTag.Size)
	if valueStart < serialStart || valueEnd < valueStart || valueEnd > serialEnd {
		return nil, fmt.Errorf("struct payload range out of bounds")
	}
	rootPayload := append([]byte(nil), asset.Raw.Bytes[valueStart:valueEnd]...)

	updatedPayload, innerResult, err := mutateTaggedPayloadProperty(asset, rootPayload, innerPath, valueJSON)
	if err != nil {
		return nil, err
	}
	if len(updatedPayload) != len(rootPayload) {
		return nil, fmt.Errorf("struct payload size changed (%d -> %d), fallback supports fixed-size edits only", len(rootPayload), len(updatedPayload))
	}

	relStart := valueStart - serialStart
	relEnd := valueEnd - serialStart
	oldExportPayload := asset.Raw.Bytes[serialStart:serialEnd]
	newExportPayload := make([]byte, 0, len(oldExportPayload))
	newExportPayload = append(newExportPayload, oldExportPayload[:relStart]...)
	newExportPayload = append(newExportPayload, updatedPayload...)
	newExportPayload = append(newExportPayload, oldExportPayload[relEnd:]...)

	return &edit.PropertySetResult{
		Mutation: edit.ExportMutation{
			ExportIndex: exportIndex,
			Payload:     newExportPayload,
		},
		ExportIndex:  exportIndex,
		PropertyName: rootName,
		Path:         path,
		OldValue:     innerResult.OldValue,
		NewValue:     innerResult.NewValue,
		OldSize:      rootTag.Size,
		NewSize:      rootTag.Size,
		ByteDelta:    len(newExportPayload) - len(oldExportPayload),
	}, nil
}
