package cli

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/wilddogjp/bpx/pkg/edit"
	"github.com/wilddogjp/bpx/pkg/uasset"
)

type dataTableRowLocation struct {
	Index     int
	Name      string
	NameStart int
	NameEnd   int
	Start     int
	End       int
}

type dataTableRowLayout struct {
	RowStart    int
	RowEnd      int
	RowsEnd     int
	StartOffset int
	CountOffset int
	RowCount    int32
	Rows        []dataTableRowLocation
	Trailing    int
}

type dataTableLayoutCandidate struct {
	label       string
	startOffset int
	rowCount    int32
}

func runDataTableUpdateRow(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("datatable update-row", stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("export", 0, "optional 1-based export index")
	rowName := fs.String("row", "", "row name")
	valuesJSON := fs.String("values", "", "JSON object of field updates")
	dryRun := fs.Bool("dry-run", false, "do not write output")
	backup := fs.Bool("backup", false, "create <file>.backup before overwrite")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || strings.TrimSpace(*rowName) == "" || strings.TrimSpace(*valuesJSON) == "" {
		fmt.Fprintln(stderr, "usage: bpx datatable update-row <file.uasset> --row <name> --values '<json>' [--export <n>] [--dry-run] [--backup]")
		return 1
	}

	updates, err := parseJSONMap(*valuesJSON)
	if err != nil {
		fmt.Fprintf(stderr, "error: parse --values JSON: %v\n", err)
		return 1
	}
	if len(updates) == 0 {
		fmt.Fprintln(stderr, "error: --values must not be empty")
		return 1
	}

	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	targetIdx, err := resolveDataTableExportIndexForUpdate(asset, *exportIndex)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	rowLoc, err := locateDataTableRow(asset, targetIdx, *rowName)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	rowPayload := make([]byte, rowLoc.End-rowLoc.Start)
	copy(rowPayload, asset.Raw.Bytes[rowLoc.Start:rowLoc.End])

	keys := make([]string, 0, len(updates))
	for k := range updates {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	changeSet := make([]map[string]any, 0, len(keys))
	for _, field := range keys {
		valueJSON, err := marshalJSONValue(updates[field])
		if err != nil {
			fmt.Fprintf(stderr, "error: encode value for %s: %v\n", field, err)
			return 1
		}
		var result *edit.PropertySetResult
		rowPayload, result, err = mutateTaggedPayloadProperty(asset, rowPayload, field, valueJSON)
		if err != nil {
			fmt.Fprintf(stderr, "error: update row field %s: %v\n", field, err)
			return 1
		}
		changeSet = append(changeSet, map[string]any{
			"field":    field,
			"oldValue": result.OldValue,
			"newValue": result.NewValue,
			"oldSize":  result.OldSize,
			"newSize":  result.NewSize,
		})
	}

	newPayload, err := rewriteDataTableRowPayload(asset, targetIdx, rowLoc, rowPayload)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	outBytes, err := edit.RewriteAsset(asset, []edit.ExportMutation{{ExportIndex: targetIdx, Payload: newPayload}})
	if err != nil {
		fmt.Fprintf(stderr, "error: rewrite asset: %v\n", err)
		return 1
	}
	changed := !bytes.Equal(asset.Raw.Bytes, outBytes)

	resp := map[string]any{
		"file":        file,
		"export":      targetIdx + 1,
		"row":         rowLoc.Name,
		"updated":     changeSet,
		"dryRun":      *dryRun,
		"changed":     changed,
		"outputBytes": len(outBytes),
	}
	if *dryRun {
		return printJSON(stdout, resp)
	}
	if *backup {
		if err := createBackupFile(file); err != nil {
			fmt.Fprintf(stderr, "error: backup source file: %v\n", err)
			return 1
		}
	}
	if err := os.WriteFile(file, outBytes, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write file: %v\n", err)
		return 1
	}
	return printJSON(stdout, resp)
}

func runDataTableAddRow(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("datatable add-row", stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("export", 0, "optional 1-based export index")
	rowName := fs.String("row", "", "row name")
	valuesJSON := fs.String("values", "{}", "JSON object of field values (optional)")
	dryRun := fs.Bool("dry-run", false, "do not write output")
	backup := fs.Bool("backup", false, "create <file>.backup before overwrite")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || strings.TrimSpace(*rowName) == "" {
		fmt.Fprintln(stderr, "usage: bpx datatable add-row <file.uasset> --row <name> [--values '<json>'] [--export <n>] [--dry-run] [--backup]")
		return 1
	}

	updatesRaw := strings.TrimSpace(*valuesJSON)
	if updatesRaw == "" {
		updatesRaw = "{}"
	}
	updates, err := parseJSONMap(updatesRaw)
	if err != nil {
		fmt.Fprintf(stderr, "error: parse --values JSON: %v\n", err)
		return 1
	}

	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	targetIdx, err := resolveDataTableExportIndexForUpdate(asset, *exportIndex)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	layout, err := detectDataTableRowLayout(asset, targetIdx)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if _, exists := findDataTableRowByDisplay(layout.Rows, strings.TrimSpace(*rowName)); exists {
		fmt.Fprintf(stderr, "error: row already exists: %s\n", strings.TrimSpace(*rowName))
		return 1
	}

	nameRef, err := resolveDisplayNameRef(asset.Names, strings.TrimSpace(*rowName))
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	rowNameBytes := encodeNameRef(nameRef, packageByteOrder(asset))

	var rowPayload []byte
	if len(layout.Rows) > 0 {
		template := layout.Rows[0]
		rowPayload = append([]byte(nil), asset.Raw.Bytes[template.Start:template.End]...)
	} else {
		rowPayload, err = buildTaggedNonePayload(asset)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	}

	keys := make([]string, 0, len(updates))
	for k := range updates {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	changeSet := make([]map[string]any, 0, len(keys))
	for _, field := range keys {
		valueJSON, err := marshalJSONValue(updates[field])
		if err != nil {
			fmt.Fprintf(stderr, "error: encode value for %s: %v\n", field, err)
			return 1
		}
		var result *edit.PropertySetResult
		rowPayload, result, err = mutateTaggedPayloadProperty(asset, rowPayload, field, valueJSON)
		if err != nil {
			fmt.Fprintf(stderr, "error: set row field %s: %v\n", field, err)
			return 1
		}
		changeSet = append(changeSet, map[string]any{
			"field":    field,
			"oldValue": result.OldValue,
			"newValue": result.NewValue,
			"oldSize":  result.OldSize,
			"newSize":  result.NewSize,
		})
	}

	rowEntry := make([]byte, 0, len(rowNameBytes)+len(rowPayload))
	rowEntry = append(rowEntry, rowNameBytes...)
	rowEntry = append(rowEntry, rowPayload...)

	newPayload, err := insertDataTableRowPayload(asset, targetIdx, layout, rowEntry, layout.RowCount+1)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	outBytes, err := edit.RewriteAsset(asset, []edit.ExportMutation{{ExportIndex: targetIdx, Payload: newPayload}})
	if err != nil {
		fmt.Fprintf(stderr, "error: rewrite asset: %v\n", err)
		return 1
	}
	changed := !bytes.Equal(asset.Raw.Bytes, outBytes)

	resp := map[string]any{
		"file":           file,
		"export":         targetIdx + 1,
		"row":            nameRef.Display(asset.Names),
		"rowCountBefore": layout.RowCount,
		"rowCountAfter":  layout.RowCount + 1,
		"updated":        changeSet,
		"byteDelta":      len(outBytes) - len(asset.Raw.Bytes),
		"dryRun":         *dryRun,
		"changed":        changed,
		"outputBytes":    len(outBytes),
	}
	if *dryRun {
		return printJSON(stdout, resp)
	}
	if *backup {
		if err := createBackupFile(file); err != nil {
			fmt.Fprintf(stderr, "error: backup source file: %v\n", err)
			return 1
		}
	}
	if err := os.WriteFile(file, outBytes, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write file: %v\n", err)
		return 1
	}
	return printJSON(stdout, resp)
}

func runDataTableRemoveRow(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("datatable remove-row", stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("export", 0, "optional 1-based export index")
	rowName := fs.String("row", "", "row name")
	dryRun := fs.Bool("dry-run", false, "do not write output")
	backup := fs.Bool("backup", false, "create <file>.backup before overwrite")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || strings.TrimSpace(*rowName) == "" {
		fmt.Fprintln(stderr, "usage: bpx datatable remove-row <file.uasset> --row <name> [--export <n>] [--dry-run] [--backup]")
		return 1
	}

	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	targetIdx, err := resolveDataTableExportIndexForUpdate(asset, *exportIndex)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	layout, err := detectDataTableRowLayout(asset, targetIdx)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	rowLoc, ok := findDataTableRowByDisplay(layout.Rows, strings.TrimSpace(*rowName))
	if !ok {
		fmt.Fprintf(stderr, "error: row not found: %s\n", strings.TrimSpace(*rowName))
		return 1
	}

	newPayload, err := removeDataTableRowPayload(asset, targetIdx, layout, rowLoc, layout.RowCount-1)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	outBytes, err := edit.RewriteAsset(asset, []edit.ExportMutation{{ExportIndex: targetIdx, Payload: newPayload}})
	if err != nil {
		fmt.Fprintf(stderr, "error: rewrite asset: %v\n", err)
		return 1
	}
	changed := !bytes.Equal(asset.Raw.Bytes, outBytes)

	resp := map[string]any{
		"file":           file,
		"export":         targetIdx + 1,
		"row":            rowLoc.Name,
		"removedIndex":   rowLoc.Index,
		"rowCountBefore": layout.RowCount,
		"rowCountAfter":  layout.RowCount - 1,
		"byteDelta":      len(outBytes) - len(asset.Raw.Bytes),
		"dryRun":         *dryRun,
		"changed":        changed,
		"outputBytes":    len(outBytes),
	}
	if *dryRun {
		return printJSON(stdout, resp)
	}
	if *backup {
		if err := createBackupFile(file); err != nil {
			fmt.Fprintf(stderr, "error: backup source file: %v\n", err)
			return 1
		}
	}
	if err := os.WriteFile(file, outBytes, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write file: %v\n", err)
		return 1
	}
	return printJSON(stdout, resp)
}

func resolveDataTableExportIndexForUpdate(asset *uasset.Asset, explicitIndex int) (int, error) {
	if explicitIndex > 0 {
		idx, err := asset.ResolveExportIndex(explicitIndex)
		if err != nil {
			return 0, err
		}
		className := asset.ResolveClassName(asset.Exports[idx])
		if className == "DataTable" {
			return idx, nil
		}
		if className == "CompositeDataTable" {
			return 0, fmt.Errorf("export %d is CompositeDataTable; datatable write commands support DataTable only", explicitIndex)
		}
		if className == "CurveTable" || className == "CompositeCurveTable" {
			return 0, fmt.Errorf("export %d is %s; datatable write commands support DataTable only", explicitIndex, className)
		}
		return 0, fmt.Errorf("export %d is not a DataTable export (class=%s)", explicitIndex, className)
	}
	foundComposite := false
	foundCurve := false
	for i, exp := range asset.Exports {
		className := asset.ResolveClassName(exp)
		if className == "DataTable" {
			return i, nil
		}
		if className == "CompositeDataTable" {
			foundComposite = true
		}
		if className == "CurveTable" || className == "CompositeCurveTable" {
			foundCurve = true
		}
	}
	if foundComposite || foundCurve {
		return 0, fmt.Errorf("writable DataTable export not found (datatable write commands support DataTable only)")
	}
	return 0, fmt.Errorf("datatable export not found")
}

func locateDataTableRow(asset *uasset.Asset, exportIndex int, rowName string) (*dataTableRowLocation, error) {
	layout, err := detectDataTableRowLayout(asset, exportIndex)
	if err != nil {
		return nil, err
	}
	row, ok := findDataTableRowByDisplay(layout.Rows, strings.TrimSpace(rowName))
	if !ok {
		return nil, fmt.Errorf("row not found: %s", strings.TrimSpace(rowName))
	}
	return row, nil
}

func detectDataTableRowLayout(asset *uasset.Asset, exportIndex int) (*dataTableRowLayout, error) {
	exp := asset.Exports[exportIndex]
	header := asset.ParseExportProperties(exportIndex)
	if len(header.Warnings) > 0 {
		return nil, fmt.Errorf("datatable header parse warnings: %s", strings.Join(header.Warnings, "; "))
	}
	rowStart := header.EndOffset
	rowEnd := int(exp.SerialOffset + exp.SerialSize)
	if rowStart < int(exp.SerialOffset) || rowStart > rowEnd {
		return nil, fmt.Errorf("datatable row range is out of bounds")
	}
	if rowEnd-rowStart < 4 {
		return nil, fmt.Errorf("datatable row section is too small")
	}

	r := uasset.NewByteReaderWithByteSwapping(asset.Raw.Bytes[rowStart:rowEnd], asset.Summary.UsesByteSwappedSerialization())
	firstCount, err := r.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read row count: %w", err)
	}
	candidates := []dataTableLayoutCandidate{
		{label: "count@+0", rowCount: firstCount, startOffset: 4},
	}
	if (firstCount == 0 || firstCount == 1) && r.Remaining() >= 4 {
		secondCount, secondErr := r.ReadInt32()
		if secondErr == nil {
			candidates = append(candidates, dataTableLayoutCandidate{
				label:       "count@+4",
				rowCount:    secondCount,
				startOffset: 8,
			})
		}
	}

	var best *dataTableRowLayout
	for _, c := range candidates {
		if c.rowCount < 0 || c.rowCount > 1_000_000 {
			continue
		}
		layout, ok := detectDataTableRowLayoutFromCandidate(asset, rowStart, rowEnd, c.startOffset, c.rowCount)
		if !ok {
			continue
		}
		if best == nil {
			best = layout
			continue
		}
		if len(layout.Rows) > len(best.Rows) {
			best = layout
			continue
		}
		if len(layout.Rows) < len(best.Rows) {
			continue
		}
		if layout.Trailing < best.Trailing {
			best = layout
			continue
		}
		if layout.Trailing > best.Trailing {
			continue
		}
		if layout.StartOffset > best.StartOffset {
			best = layout
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no valid datatable row layout candidate")
	}
	return best, nil
}

func detectDataTableRowLayoutFromCandidate(asset *uasset.Asset, rowStart, rowEnd, startOffset int, rowCount int32) (*dataTableRowLayout, bool) {
	if startOffset < 0 || rowStart+startOffset > rowEnd {
		return nil, false
	}
	r := uasset.NewByteReaderWithByteSwapping(asset.Raw.Bytes[rowStart+startOffset:rowEnd], asset.Summary.UsesByteSwappedSerialization())
	rows := make([]dataTableRowLocation, 0, rowCount)
	for i := int32(0); i < rowCount; i++ {
		nameStart := rowStart + startOffset + r.Offset()
		nameRef, err := r.ReadNameRef(len(asset.Names))
		if err != nil {
			return nil, false
		}
		nameEnd := rowStart + startOffset + r.Offset()
		entryStart := nameEnd
		props := asset.ParseTaggedPropertiesRange(entryStart, rowEnd, false)
		if props.EndOffset <= entryStart {
			return nil, false
		}
		entryEnd := props.EndOffset
		if err := r.Seek(entryEnd - (rowStart + startOffset)); err != nil {
			return nil, false
		}
		rows = append(rows, dataTableRowLocation{
			Index:     int(i),
			Name:      nameRef.Display(asset.Names),
			NameStart: nameStart,
			NameEnd:   nameEnd,
			Start:     entryStart,
			End:       entryEnd,
		})
	}
	rowsEnd := rowStart + startOffset + r.Offset()
	if rowsEnd < rowStart+startOffset || rowsEnd > rowEnd {
		return nil, false
	}
	countOffset := rowStart + startOffset - 4
	if countOffset < rowStart || countOffset+4 > rowEnd {
		return nil, false
	}
	return &dataTableRowLayout{
		RowStart:    rowStart,
		RowEnd:      rowEnd,
		RowsEnd:     rowsEnd,
		StartOffset: startOffset,
		CountOffset: countOffset,
		RowCount:    rowCount,
		Rows:        rows,
		Trailing:    rowEnd - rowsEnd,
	}, true
}

func findDataTableRowByDisplay(rows []dataTableRowLocation, rowName string) (*dataTableRowLocation, bool) {
	for i := range rows {
		if rows[i].Name == rowName {
			return &rows[i], true
		}
	}
	return nil, false
}

func rewriteDataTableRowPayload(asset *uasset.Asset, exportIndex int, rowLoc *dataTableRowLocation, rowPayload []byte) ([]byte, error) {
	serialStart, serialEnd, err := dataTableSerialRange(asset, exportIndex)
	if err != nil {
		return nil, err
	}
	relStart := rowLoc.Start - serialStart
	relEnd := rowLoc.End - serialStart
	if relStart < 0 || relEnd < relStart || relEnd > (serialEnd-serialStart) {
		return nil, fmt.Errorf("datatable row payload range out of bounds")
	}
	oldPayload := asset.Raw.Bytes[serialStart:serialEnd]
	newPayload := make([]byte, 0, len(oldPayload)+(len(rowPayload)-(relEnd-relStart)))
	newPayload = append(newPayload, oldPayload[:relStart]...)
	newPayload = append(newPayload, rowPayload...)
	newPayload = append(newPayload, oldPayload[relEnd:]...)
	return newPayload, nil
}

func insertDataTableRowPayload(asset *uasset.Asset, exportIndex int, layout *dataTableRowLayout, rowEntry []byte, newCount int32) ([]byte, error) {
	serialStart, serialEnd, err := dataTableSerialRange(asset, exportIndex)
	if err != nil {
		return nil, err
	}
	insertRel := layout.RowsEnd - serialStart
	if insertRel < 0 || insertRel > (serialEnd-serialStart) {
		return nil, fmt.Errorf("datatable row insertion offset out of bounds")
	}
	oldPayload := asset.Raw.Bytes[serialStart:serialEnd]
	newPayload := make([]byte, 0, len(oldPayload)+len(rowEntry))
	newPayload = append(newPayload, oldPayload[:insertRel]...)
	newPayload = append(newPayload, rowEntry...)
	newPayload = append(newPayload, oldPayload[insertRel:]...)

	countRel := layout.CountOffset - serialStart
	if err := patchInt32(newPayload, countRel, newCount, packageByteOrder(asset)); err != nil {
		return nil, err
	}
	return newPayload, nil
}

func removeDataTableRowPayload(asset *uasset.Asset, exportIndex int, layout *dataTableRowLayout, rowLoc *dataTableRowLocation, newCount int32) ([]byte, error) {
	if newCount < 0 {
		return nil, fmt.Errorf("datatable row count would become negative")
	}
	serialStart, serialEnd, err := dataTableSerialRange(asset, exportIndex)
	if err != nil {
		return nil, err
	}
	removeStart := rowLoc.NameStart - serialStart
	removeEnd := rowLoc.End - serialStart
	if removeStart < 0 || removeEnd < removeStart || removeEnd > (serialEnd-serialStart) {
		return nil, fmt.Errorf("datatable row removal range out of bounds")
	}
	oldPayload := asset.Raw.Bytes[serialStart:serialEnd]
	newPayload := make([]byte, 0, len(oldPayload)-(removeEnd-removeStart))
	newPayload = append(newPayload, oldPayload[:removeStart]...)
	newPayload = append(newPayload, oldPayload[removeEnd:]...)

	countRel := layout.CountOffset - serialStart
	if err := patchInt32(newPayload, countRel, newCount, packageByteOrder(asset)); err != nil {
		return nil, err
	}
	return newPayload, nil
}

func dataTableSerialRange(asset *uasset.Asset, exportIndex int) (int, int, error) {
	exp := asset.Exports[exportIndex]
	serialStart := int(exp.SerialOffset)
	serialEnd := int(exp.SerialOffset + exp.SerialSize)
	if serialStart < 0 || serialEnd < serialStart || serialEnd > len(asset.Raw.Bytes) {
		return 0, 0, fmt.Errorf("datatable export serial range out of bounds")
	}
	return serialStart, serialEnd, nil
}

func patchInt32(payload []byte, relOffset int, value int32, order binary.ByteOrder) error {
	if relOffset < 0 || relOffset+4 > len(payload) {
		return fmt.Errorf("datatable row count field out of bounds")
	}
	order.PutUint32(payload[relOffset:relOffset+4], uint32(value))
	return nil
}

func packageByteOrder(asset *uasset.Asset) binary.ByteOrder {
	if asset.Summary.UsesByteSwappedSerialization() {
		return binary.BigEndian
	}
	return binary.LittleEndian
}

func resolveDisplayNameRef(names []uasset.NameEntry, displayName string) (uasset.NameRef, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return uasset.NameRef{}, fmt.Errorf("row name must not be empty")
	}
	if idx := findNameIndex(names, displayName); idx >= 0 {
		return uasset.NameRef{Index: idx, Number: 0}, nil
	}

	sep := strings.LastIndex(displayName, "_")
	if sep <= 0 || sep >= len(displayName)-1 {
		return uasset.NameRef{}, fmt.Errorf("row name %q is not present in NameMap (NameMap add is not supported)", displayName)
	}
	base := displayName[:sep]
	suffix := displayName[sep+1:]
	n, err := strconv.ParseInt(suffix, 10, 32)
	if err != nil || n < 0 {
		return uasset.NameRef{}, fmt.Errorf("row name %q is not present in NameMap (NameMap add is not supported)", displayName)
	}
	idx := findNameIndex(names, base)
	if idx < 0 {
		return uasset.NameRef{}, fmt.Errorf("row name %q is not present in NameMap (NameMap add is not supported)", displayName)
	}
	return uasset.NameRef{Index: idx, Number: int32(n + 1)}, nil
}

func findNameIndex(names []uasset.NameEntry, value string) int32 {
	for i := range names {
		if names[i].Value == value {
			return int32(i)
		}
	}
	return -1
}

func encodeNameRef(ref uasset.NameRef, order binary.ByteOrder) []byte {
	buf := make([]byte, 8)
	order.PutUint32(buf[0:4], uint32(ref.Index))
	order.PutUint32(buf[4:8], uint32(ref.Number))
	return buf
}

func buildTaggedNonePayload(asset *uasset.Asset) ([]byte, error) {
	noneIdx := findNameIndex(asset.Names, "None")
	if noneIdx < 0 {
		return nil, fmt.Errorf("NameMap does not include None")
	}
	return encodeNameRef(uasset.NameRef{Index: noneIdx, Number: 0}, packageByteOrder(asset)), nil
}

func mutateTaggedPayloadProperty(baseAsset *uasset.Asset, payload []byte, path string, valueJSON string) ([]byte, *edit.PropertySetResult, error) {
	const pkgFlagUnversionedProps = uint32(0x00002000)

	buildWorking := func(forceUnversioned bool) *uasset.Asset {
		summary := baseAsset.Summary
		if forceUnversioned {
			summary.PackageFlags |= pkgFlagUnversionedProps
		} else {
			summary.PackageFlags &^= pkgFlagUnversionedProps
		}
		return &uasset.Asset{
			Raw:     uasset.RawAsset{Bytes: append([]byte(nil), payload...)},
			Summary: summary,
			Names:   baseAsset.Names,
			Exports: []uasset.ExportEntry{
				{
					SerialOffset: 0,
					SerialSize:   int64(len(payload)),
				},
			},
		}
	}

	tryMutate := func(working *uasset.Asset) ([]byte, *edit.PropertySetResult, error) {
		result, err := edit.BuildPropertySetMutation(working, 0, path, valueJSON)
		if err != nil {
			return nil, nil, err
		}
		return result.Mutation.Payload, result, nil
	}

	working := buildWorking(true)
	out, result, forceUnversionedErr := tryMutate(working)
	if forceUnversionedErr == nil {
		return out, result, nil
	}

	// Fallback for payloads that do include class serialization control bytes.
	fallback := buildWorking(false)
	out, result, withClassControlErr := tryMutate(fallback)
	if withClassControlErr == nil {
		return out, result, nil
	}
	return nil, nil, fmt.Errorf("mutate tagged payload failed (unversioned-props=%v; class-control=%v)", forceUnversionedErr, withClassControlErr)
}
