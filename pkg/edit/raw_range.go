package edit

import (
	"encoding/binary"
	"fmt"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

type rawRangePatch struct {
	oldStart int64
	oldEnd   int64
	newStart int64
	newLen   int64
}

// RewriteRawRange replaces one non-export byte range and updates summary/export offsets.
// The range must not overlap export serial payloads.
func RewriteRawRange(asset *uasset.Asset, start, end int64, replacement []byte) ([]byte, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	raw := asset.Raw.Bytes
	if len(raw) == 0 {
		return nil, fmt.Errorf("asset has no raw bytes")
	}
	if start < 0 || end < start || end > int64(len(raw)) {
		return nil, fmt.Errorf("rewrite range out of bounds: %d..%d (size=%d)", start, end, len(raw))
	}
	if replacement == nil {
		replacement = []byte{}
	}

	for i, exp := range asset.Exports {
		expStart := exp.SerialOffset
		expEnd := exp.SerialOffset + exp.SerialSize
		if expStart < 0 || expEnd < expStart || expEnd > int64(len(raw)) {
			return nil, fmt.Errorf("export[%d] serial range out of bounds (%d..%d)", i+1, expStart, expEnd)
		}
		if start < expEnd && end > expStart {
			return nil, fmt.Errorf("rewrite range overlaps export[%d] payload (%d..%d)", i+1, expStart, expEnd)
		}
	}

	patch := rawRangePatch{
		oldStart: start,
		oldEnd:   end,
		newStart: start,
		newLen:   int64(len(replacement)),
	}

	delta := patch.newLen - (patch.oldEnd - patch.oldStart)
	outCap := int64(len(raw)) + delta
	if outCap < 0 {
		return nil, fmt.Errorf("invalid rewritten size")
	}

	out := make([]byte, 0, outCap)
	out = append(out, raw[:start]...)
	out = append(out, replacement...)
	out = append(out, raw[end:]...)

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}

	summaryFields, err := scanSummaryOffsetFields(raw)
	if err != nil {
		return nil, fmt.Errorf("scan summary offsets: %w", err)
	}
	for _, field := range summaryFields {
		writePos := translateRawRangeOffset(int64(field.pos), patch)
		if writePos < 0 || writePos+int64(field.size) > int64(len(out)) {
			return nil, fmt.Errorf("summary %s write position out of bounds: %d", field.name, writePos)
		}
		switch field.size {
		case 4:
			oldV, err := readInt32At(raw, field.pos, order)
			if err != nil {
				return nil, fmt.Errorf("read summary %s: %w", field.name, err)
			}
			if oldV < 0 {
				continue
			}
			mapped := translateRawRangeOffset(int64(oldV), patch)
			if mapped > int64(^uint32(0)>>1) {
				return nil, fmt.Errorf("summary %s overflow after translation: %d", field.name, mapped)
			}
			if err := writeInt32At(out, int(writePos), int32(mapped), order); err != nil {
				return nil, fmt.Errorf("patch summary %s: %w", field.name, err)
			}
		case 8:
			oldV, err := readInt64At(raw, field.pos, order)
			if err != nil {
				return nil, fmt.Errorf("read summary %s: %w", field.name, err)
			}
			if oldV < 0 {
				continue
			}
			mapped := translateRawRangeOffset(oldV, patch)
			if err := writeInt64At(out, int(writePos), mapped, order); err != nil {
				return nil, fmt.Errorf("patch summary %s: %w", field.name, err)
			}
		default:
			return nil, fmt.Errorf("unsupported summary field size for %s: %d", field.name, field.size)
		}
	}

	exportFields, err := scanExportFieldPositions(raw, asset)
	if err != nil {
		return nil, fmt.Errorf("scan export fields: %w", err)
	}
	if len(exportFields) != len(asset.Exports) {
		return nil, fmt.Errorf("export field scan mismatch: got %d want %d", len(exportFields), len(asset.Exports))
	}
	for i, expField := range exportFields {
		serialOffsetPos := translateRawRangeOffset(int64(expField.serialOffsetPos), patch)
		if serialOffsetPos < 0 || serialOffsetPos+8 > int64(len(out)) {
			return nil, fmt.Errorf("export[%d] serial offset field out of bounds: %d", i+1, serialOffsetPos)
		}
		oldSerialOffset := asset.Exports[i].SerialOffset
		if oldSerialOffset < 0 {
			continue
		}
		newSerialOffset := translateRawRangeOffset(oldSerialOffset, patch)
		if err := writeInt64At(out, int(serialOffsetPos), newSerialOffset, order); err != nil {
			return nil, fmt.Errorf("patch export[%d] serial offset: %w", i+1, err)
		}
	}

	return out, nil
}

func translateRawRangeOffset(oldPos int64, patch rawRangePatch) int64 {
	if oldPos < patch.oldStart {
		return oldPos
	}
	if oldPos >= patch.oldEnd {
		return oldPos + (patch.newLen - (patch.oldEnd - patch.oldStart))
	}
	rel := oldPos - patch.oldStart
	if rel < 0 {
		rel = 0
	}
	if rel > patch.newLen {
		rel = patch.newLen
	}
	return patch.newStart + rel
}
