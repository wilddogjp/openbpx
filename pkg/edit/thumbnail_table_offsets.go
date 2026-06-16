package edit

import (
	"encoding/binary"
	"fmt"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

func patchThumbnailTableFileOffsets(raw, out []byte, asset *uasset.Asset, translate func(int64) int64) error {
	if asset == nil || len(raw) == 0 {
		return nil
	}

	tableStart := int64(asset.Summary.ThumbnailTableOffset)
	if tableStart <= 0 {
		return nil
	}
	if tableStart >= int64(len(raw)) {
		return fmt.Errorf("thumbnail table offset out of bounds: %d", tableStart)
	}

	tableEnd := findThumbnailTableEndOffset(asset, raw)
	if tableEnd <= tableStart {
		return nil
	}
	if tableEnd > int64(len(raw)) {
		return fmt.Errorf("thumbnail table end out of bounds: %d", tableEnd)
	}

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}

	r := newByteCodec(raw[tableStart:tableEnd], order)
	count, err := r.readInt32()
	if err != nil {
		return fmt.Errorf("read thumbnail table count: %w", err)
	}
	if count < 0 {
		return fmt.Errorf("invalid thumbnail table count: %d", count)
	}

	for i := int32(0); i < count; i++ {
		if _, err := r.readFString(); err != nil {
			return fmt.Errorf("thumbnail table entry[%d] class name: %w", i, err)
		}
		if _, err := r.readFString(); err != nil {
			return fmt.Errorf("thumbnail table entry[%d] object path: %w", i, err)
		}
		valuePos := r.off
		oldOffset, err := r.readInt32()
		if err != nil {
			return fmt.Errorf("thumbnail table entry[%d] file offset: %w", i, err)
		}
		if oldOffset <= 0 {
			continue
		}

		writePos := translate(tableStart + int64(valuePos))
		if writePos < 0 || writePos+4 > int64(len(out)) {
			return fmt.Errorf("thumbnail table entry[%d] write position out of bounds: %d", i, writePos)
		}
		newOffset := translate(int64(oldOffset))
		if newOffset > int64(^uint32(0)>>1) {
			return fmt.Errorf("thumbnail table entry[%d] overflow after translation: %d", i, newOffset)
		}
		if err := writeInt32At(out, int(writePos), int32(newOffset), order); err != nil {
			return fmt.Errorf("patch thumbnail table entry[%d] file offset: %w", i, err)
		}
	}

	return nil
}

func findThumbnailTableEndOffset(asset *uasset.Asset, raw []byte) int64 {
	start := int64(asset.Summary.ThumbnailTableOffset)
	if start <= 0 {
		return 0
	}

	end := int64(len(raw))
	add := func(off int64) {
		if off <= start || off > int64(len(raw)) {
			return
		}
		if off < end {
			end = off
		}
	}

	s := asset.Summary
	add(int64(s.ImportTypeHierarchiesOffset))
	add(int64(s.AssetRegistryDataOffset))
	add(int64(s.WorldTileInfoDataOffset))
	add(int64(s.PreloadDependencyOffset))
	add(s.PayloadTOCOffset)
	add(int64(s.DataResourceOffset))
	add(s.BulkDataStartOffset)
	for _, exp := range asset.Exports {
		add(exp.SerialOffset)
		add(exp.SerialOffset + exp.SerialSize)
	}
	return end
}
