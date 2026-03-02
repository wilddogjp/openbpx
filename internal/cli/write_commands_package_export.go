package cli

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

const (
	packageFileTagLE = uint32(0x9E2A83C1)
	packageFileTagBE = uint32(0xC1832A9E)

	ue5OptionalResources          = int32(1003)
	ue5RemoveObjectExportPkgGUID  = int32(1005)
	ue5TrackObjectExportInherited = int32(1006)
	ue5ScriptSerializationOffset  = int32(1010)
	ue5PackageSavedHash           = int32(1016)

	packageFlagNewlyCreated      = uint32(0x00000001)
	packageFlagCompiledIn        = uint32(0x00000010)
	packageFlagIsSaving          = uint32(0x00008000)
	packageFlagReloadForCooker   = uint32(0x40000000)
	packageFlagTransientInMemory = packageFlagNewlyCreated | packageFlagCompiledIn | packageFlagIsSaving | packageFlagReloadForCooker

	packageFlagFilterEditorOnly = uint32(0x80000000)
	packageFlagUnversionedProps = uint32(0x00002000)
	packageFlagsShapeSensitive  = packageFlagFilterEditorOnly | packageFlagUnversionedProps
)

var packageFlagNameToValue = map[string]uint32{
	// UE5.6 EPackageFlags (ref: Runtime/CoreUObject/Public/UObject/ObjectMacros.h).
	"PKG_NONE":                        0x00000000,
	"PKG_NEWLYCREATED":                0x00000001,
	"PKG_CLIENTOPTIONAL":              0x00000002,
	"PKG_SERVERSIDEONLY":              0x00000004,
	"PKG_COMPILEDIN":                  0x00000010,
	"PKG_FORDIFFING":                  0x00000020,
	"PKG_EDITORONLY":                  0x00000040,
	"PKG_DEVELOPER":                   0x00000080,
	"PKG_UNCOOKEDONLY":                0x00000100,
	"PKG_COOKED":                      0x00000200,
	"PKG_CONTAINSNOASSET":             0x00000400,
	"PKG_NOTEXTERNALLYREFERENCEABLE":  0x00000800,
	"PKG_ACCESSSPECIFIEREPICINTERNAL": 0x00001000,
	"PKG_UNVERSIONEDPROPERTIES":       0x00002000,
	"PKG_CONTAINSMAPDATA":             0x00004000,
	"PKG_ISSAVING":                    0x00008000,
	"PKG_COMPILING":                   0x00010000,
	"PKG_CONTAINSMAP":                 0x00020000,
	"PKG_REQUIRESLOCALIZATIONGATHER":  0x00040000,
	"PKG_LOADUNCOOKED":                0x00080000,
	"PKG_PLAYINEDITOR":                0x00100000,
	"PKG_CONTAINSSCRIPT":              0x00200000,
	"PKG_DISALLOWEXPORT":              0x00400000,
	"PKG_COOKGENERATED":               0x08000000,
	"PKG_DYNAMICIMPORTS":              0x10000000,
	"PKG_RUNTIMEGENERATED":            0x20000000,
	"PKG_RELOADINGFORCOOKER":          0x40000000,
	"PKG_FILTEREDITORONLY":            0x80000000,
}

type exportHeaderPositions struct {
	objectFlags                           int
	forcedExport                          int
	notForClient                          int
	notForServer                          int
	notAlwaysLoadedForEditor              int
	isAsset                               int
	isInheritedInstance                   int
	generatePublicHash                    int
	firstExportDependency                 int
	serializationBeforeSerializationDeps  int
	createBeforeSerializationDeps         int
	serializationBeforeCreateDependencies int
	createBeforeCreateDependencies        int
}

func runExportSetHeader(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("export set-header", stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("index", 0, "1-based export index")
	fieldsJSON := fs.String("fields", "", "JSON object of header fields to update")
	dryRun := fs.Bool("dry-run", false, "do not write output")
	backup := fs.Bool("backup", false, "create <file>.backup before overwrite")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || *exportIndex <= 0 || strings.TrimSpace(*fieldsJSON) == "" {
		fmt.Fprintln(stderr, "usage: bpx export set-header <file.uasset> --index <n> --fields '<json>' [--dry-run] [--backup]")
		return 1
	}

	fields, err := parseJSONMap(*fieldsJSON)
	if err != nil {
		fmt.Fprintf(stderr, "error: parse --fields JSON: %v\n", err)
		return 1
	}
	if len(fields) == 0 {
		fmt.Fprintln(stderr, "error: --fields must not be empty")
		return 1
	}

	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	idx, err := asset.ResolveExportIndex(*exportIndex)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	outBytes, oldValues, newValues, err := applyExportHeaderFields(asset, idx, fields)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	changed := !bytes.Equal(asset.Raw.Bytes, outBytes)

	resp := map[string]any{
		"file":        file,
		"export":      *exportIndex,
		"dryRun":      *dryRun,
		"changed":     changed,
		"oldFields":   oldValues,
		"newFields":   newValues,
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

func runPackageSetFlags(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("package set-flags", stderr)
	opts := registerCommonFlags(fs)
	flagsRaw := fs.String("flags", "", "package flags enum-or-raw value")
	dryRun := fs.Bool("dry-run", false, "do not write output")
	backup := fs.Bool("backup", false, "create <file>.backup before overwrite")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || strings.TrimSpace(*flagsRaw) == "" {
		fmt.Fprintln(stderr, "usage: bpx package set-flags <file.uasset> --flags <enum-or-raw> [--dry-run] [--backup]")
		return 1
	}

	newFlags, err := parsePackageFlagsValue(*flagsRaw)
	if err != nil {
		fmt.Fprintf(stderr, "error: parse --flags: %v\n", err)
		return 1
	}
	newFlags &= ^uint32(packageFlagTransientInMemory)

	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if changed := (asset.Summary.PackageFlags ^ newFlags) & packageFlagsShapeSensitive; changed != 0 {
		fmt.Fprintln(stderr, "error: changing PKG_FilterEditorOnly or PKG_UnversionedProperties is not supported")
		return 1
	}

	outBytes, oldFlags, err := rewritePackageFlags(asset, newFlags)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	changed := !bytes.Equal(asset.Raw.Bytes, outBytes)

	resp := map[string]any{
		"file":        file,
		"dryRun":      *dryRun,
		"changed":     changed,
		"oldFlags":    fmt.Sprintf("0x%08x", oldFlags),
		"newFlags":    fmt.Sprintf("0x%08x", newFlags),
		"oldFlagsRaw": oldFlags,
		"newFlagsRaw": newFlags,
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

func parsePackageFlagsValue(raw string) (uint32, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("empty flags value")
	}
	if n, err := strconv.ParseUint(raw, 0, 32); err == nil {
		return uint32(n), nil
	}

	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '|' || r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	if len(parts) == 0 {
		return 0, fmt.Errorf("no flags parsed")
	}

	var out uint32
	for _, part := range parts {
		name := strings.ToUpper(strings.TrimSpace(part))
		if name == "" {
			continue
		}
		if !strings.HasPrefix(name, "PKG_") {
			name = "PKG_" + name
		}
		v, ok := packageFlagNameToValue[name]
		if !ok {
			return 0, fmt.Errorf("unknown package flag: %s", part)
		}
		out |= v
	}
	return out, nil
}

func rewritePackageFlags(asset *uasset.Asset, newFlags uint32) ([]byte, uint32, error) {
	if asset == nil {
		return nil, 0, fmt.Errorf("asset is nil")
	}
	if len(asset.Raw.Bytes) < 4 {
		return nil, 0, fmt.Errorf("asset bytes are too small")
	}

	out := append([]byte(nil), asset.Raw.Bytes...)
	order, byteSwap, err := detectPackageByteOrder(out)
	if err != nil {
		return nil, 0, err
	}
	r := uasset.NewByteReaderWithByteSwapping(out, byteSwap)

	if _, err := r.ReadInt32(); err != nil { // tag
		return nil, 0, err
	}
	legacyVersion, err := r.ReadInt32()
	if err != nil {
		return nil, 0, err
	}
	if legacyVersion != -4 {
		if _, err := r.ReadInt32(); err != nil {
			return nil, 0, err
		}
	}
	fileUE4, err := r.ReadInt32()
	if err != nil {
		return nil, 0, err
	}
	fileUE5, err := r.ReadInt32()
	if err != nil {
		return nil, 0, err
	}
	fileLicensee, err := r.ReadInt32()
	if err != nil {
		return nil, 0, err
	}
	if fileUE4 == 0 && fileUE5 == 0 && fileLicensee == 0 {
		fileUE5 = asset.Summary.FileVersionUE5
	}
	if fileUE5 >= ue5PackageSavedHash {
		if err := r.Skip(20); err != nil {
			return nil, 0, err
		}
		if _, err := r.ReadInt32(); err != nil { // TotalHeaderSize
			return nil, 0, err
		}
	}
	if legacyVersion <= -2 {
		if err := skipSummaryCustomVersionsForRewrite(r, legacyVersion); err != nil {
			return nil, 0, err
		}
	}
	if fileUE5 < ue5PackageSavedHash {
		if _, err := r.ReadInt32(); err != nil { // TotalHeaderSize
			return nil, 0, err
		}
	}
	if _, err := r.ReadFString(); err != nil { // PackageName
		return nil, 0, err
	}

	flagPos := r.Offset()
	oldFlags, err := r.ReadUint32()
	if err != nil {
		return nil, 0, err
	}
	if flagPos < 0 || flagPos+4 > len(out) {
		return nil, 0, fmt.Errorf("package flags position out of range")
	}
	order.PutUint32(out[flagPos:flagPos+4], newFlags)
	return out, oldFlags, nil
}

func skipSummaryCustomVersionsForRewrite(r *uasset.ByteReader, legacy int32) error {
	count, err := r.ReadInt32()
	if err != nil {
		return err
	}
	if count < 0 {
		return fmt.Errorf("invalid custom version count: %d", count)
	}
	switch {
	case legacy == -2:
		for i := int32(0); i < count; i++ {
			if err := r.Skip(8); err != nil {
				return err
			}
		}
	case legacy >= -5:
		for i := int32(0); i < count; i++ {
			if err := r.Skip(16); err != nil {
				return err
			}
			if _, err := r.ReadInt32(); err != nil {
				return err
			}
			if _, err := r.ReadFString(); err != nil {
				return err
			}
		}
	default:
		for i := int32(0); i < count; i++ {
			if err := r.Skip(16); err != nil {
				return err
			}
			if _, err := r.ReadInt32(); err != nil {
				return err
			}
		}
	}
	return nil
}

func detectPackageByteOrder(data []byte) (binary.ByteOrder, bool, error) {
	if len(data) < 4 {
		return nil, false, fmt.Errorf("file is too small")
	}
	tag := binary.LittleEndian.Uint32(data[:4])
	switch tag {
	case packageFileTagLE:
		return binary.LittleEndian, false, nil
	case packageFileTagBE:
		return binary.BigEndian, true, nil
	default:
		return nil, false, fmt.Errorf("invalid package tag: 0x%x", tag)
	}
}

func applyExportHeaderFields(asset *uasset.Asset, exportIndex int, fields map[string]any) ([]byte, map[string]any, map[string]any, error) {
	if asset == nil {
		return nil, nil, nil, fmt.Errorf("asset is nil")
	}
	if exportIndex < 0 || exportIndex >= len(asset.Exports) {
		return nil, nil, nil, fmt.Errorf("export index out of range: %d", exportIndex+1)
	}
	positions, err := scanExportHeaderPositions(asset)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(positions) != len(asset.Exports) {
		return nil, nil, nil, fmt.Errorf("export header position mismatch")
	}
	pos := positions[exportIndex]

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}
	out := append([]byte(nil), asset.Raw.Bytes...)
	oldValues := map[string]any{}
	newValues := map[string]any{}

	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		raw := fields[key]
		switch key {
		case "objectFlags":
			v, err := parseUint32Any(raw)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("objectFlags: %w", err)
			}
			old := order.Uint32(out[pos.objectFlags : pos.objectFlags+4])
			order.PutUint32(out[pos.objectFlags:pos.objectFlags+4], v)
			oldValues[key] = old
			newValues[key] = v
		case "forcedExport":
			if err := patchBoolField(out, pos.forcedExport, order, raw, oldValues, newValues, key); err != nil {
				return nil, nil, nil, err
			}
		case "notForClient":
			if err := patchBoolField(out, pos.notForClient, order, raw, oldValues, newValues, key); err != nil {
				return nil, nil, nil, err
			}
		case "notForServer":
			if err := patchBoolField(out, pos.notForServer, order, raw, oldValues, newValues, key); err != nil {
				return nil, nil, nil, err
			}
		case "notAlwaysLoadedForEditor":
			if err := patchBoolField(out, pos.notAlwaysLoadedForEditor, order, raw, oldValues, newValues, key); err != nil {
				return nil, nil, nil, err
			}
		case "isAsset":
			if err := patchBoolField(out, pos.isAsset, order, raw, oldValues, newValues, key); err != nil {
				return nil, nil, nil, err
			}
		case "isInheritedInstance":
			if pos.isInheritedInstance < 0 {
				return nil, nil, nil, fmt.Errorf("isInheritedInstance is not available for this package version")
			}
			if err := patchBoolField(out, pos.isInheritedInstance, order, raw, oldValues, newValues, key); err != nil {
				return nil, nil, nil, err
			}
		case "generatePublicHash":
			if pos.generatePublicHash < 0 {
				return nil, nil, nil, fmt.Errorf("generatePublicHash is not available for this package version")
			}
			if err := patchBoolField(out, pos.generatePublicHash, order, raw, oldValues, newValues, key); err != nil {
				return nil, nil, nil, err
			}
		case "firstExportDependency":
			if err := patchInt32Field(out, pos.firstExportDependency, order, raw, oldValues, newValues, key); err != nil {
				return nil, nil, nil, err
			}
		case "serializationBeforeSerializationDeps":
			if err := patchInt32Field(out, pos.serializationBeforeSerializationDeps, order, raw, oldValues, newValues, key); err != nil {
				return nil, nil, nil, err
			}
		case "createBeforeSerializationDeps":
			if err := patchInt32Field(out, pos.createBeforeSerializationDeps, order, raw, oldValues, newValues, key); err != nil {
				return nil, nil, nil, err
			}
		case "serializationBeforeCreateDependencies":
			if err := patchInt32Field(out, pos.serializationBeforeCreateDependencies, order, raw, oldValues, newValues, key); err != nil {
				return nil, nil, nil, err
			}
		case "createBeforeCreateDependencies":
			if err := patchInt32Field(out, pos.createBeforeCreateDependencies, order, raw, oldValues, newValues, key); err != nil {
				return nil, nil, nil, err
			}
		default:
			return nil, nil, nil, fmt.Errorf("unsupported export header field: %s", key)
		}
	}
	return out, oldValues, newValues, nil
}

func patchBoolField(out []byte, pos int, order binary.ByteOrder, raw any, oldValues, newValues map[string]any, key string) error {
	if pos < 0 {
		return fmt.Errorf("field %s is not available", key)
	}
	v, err := parseBoolAny(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	oldRaw := order.Uint32(out[pos : pos+4])
	if v {
		order.PutUint32(out[pos:pos+4], 1)
	} else {
		order.PutUint32(out[pos:pos+4], 0)
	}
	oldValues[key] = oldRaw != 0
	newValues[key] = v
	return nil
}

func patchInt32Field(out []byte, pos int, order binary.ByteOrder, raw any, oldValues, newValues map[string]any, key string) error {
	if pos < 0 {
		return fmt.Errorf("field %s is not available", key)
	}
	v, err := parseInt32Any(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	oldRaw := int32(order.Uint32(out[pos : pos+4]))
	order.PutUint32(out[pos:pos+4], uint32(v))
	oldValues[key] = oldRaw
	newValues[key] = v
	return nil
}

func scanExportHeaderPositions(asset *uasset.Asset) ([]exportHeaderPositions, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	data := asset.Raw.Bytes
	if asset.Summary.ExportOffset < 0 || int(asset.Summary.ExportOffset) > len(data) {
		return nil, fmt.Errorf("export offset out of range: %d", asset.Summary.ExportOffset)
	}
	r := uasset.NewByteReaderWithByteSwapping(data, asset.Summary.UsesByteSwappedSerialization())
	if err := r.Seek(int(asset.Summary.ExportOffset)); err != nil {
		return nil, err
	}

	fields := make([]exportHeaderPositions, 0, len(asset.Exports))
	for i := 0; i < len(asset.Exports); i++ {
		pos := exportHeaderPositions{
			isInheritedInstance: -1,
			generatePublicHash:  -1,
		}
		if err := r.Skip(4 * 4); err != nil {
			return nil, fmt.Errorf("export[%d] read class/super/template/outer: %w", i+1, err)
		}
		if err := r.Skip(8); err != nil {
			return nil, fmt.Errorf("export[%d] read object name: %w", i+1, err)
		}
		pos.objectFlags = r.Offset()
		if _, err := r.ReadUint32(); err != nil {
			return nil, fmt.Errorf("export[%d] read object flags: %w", i+1, err)
		}
		if _, err := r.ReadInt64(); err != nil {
			return nil, fmt.Errorf("export[%d] read serial size: %w", i+1, err)
		}
		if _, err := r.ReadInt64(); err != nil {
			return nil, fmt.Errorf("export[%d] read serial offset: %w", i+1, err)
		}
		pos.forcedExport = r.Offset()
		if _, err := r.ReadUint32(); err != nil {
			return nil, fmt.Errorf("export[%d] read forcedExport: %w", i+1, err)
		}
		pos.notForClient = r.Offset()
		if _, err := r.ReadUint32(); err != nil {
			return nil, fmt.Errorf("export[%d] read notForClient: %w", i+1, err)
		}
		pos.notForServer = r.Offset()
		if _, err := r.ReadUint32(); err != nil {
			return nil, fmt.Errorf("export[%d] read notForServer: %w", i+1, err)
		}
		if asset.Summary.FileVersionUE5 < ue5RemoveObjectExportPkgGUID {
			if err := r.Skip(16); err != nil {
				return nil, fmt.Errorf("export[%d] read package guid: %w", i+1, err)
			}
		}
		if asset.Summary.FileVersionUE5 >= ue5TrackObjectExportInherited {
			pos.isInheritedInstance = r.Offset()
			if _, err := r.ReadUint32(); err != nil {
				return nil, fmt.Errorf("export[%d] read inherited flag: %w", i+1, err)
			}
		}
		if err := r.Skip(4); err != nil { // package flags
			return nil, fmt.Errorf("export[%d] read package flags: %w", i+1, err)
		}
		pos.notAlwaysLoadedForEditor = r.Offset()
		if _, err := r.ReadUint32(); err != nil {
			return nil, fmt.Errorf("export[%d] read notAlwaysLoadedForEditor: %w", i+1, err)
		}
		pos.isAsset = r.Offset()
		if _, err := r.ReadUint32(); err != nil {
			return nil, fmt.Errorf("export[%d] read isAsset: %w", i+1, err)
		}
		if asset.Summary.FileVersionUE5 >= ue5OptionalResources {
			pos.generatePublicHash = r.Offset()
			if _, err := r.ReadUint32(); err != nil {
				return nil, fmt.Errorf("export[%d] read generatePublicHash: %w", i+1, err)
			}
		}
		pos.firstExportDependency = r.Offset()
		if _, err := r.ReadInt32(); err != nil {
			return nil, fmt.Errorf("export[%d] read firstExportDependency: %w", i+1, err)
		}
		pos.serializationBeforeSerializationDeps = r.Offset()
		if _, err := r.ReadInt32(); err != nil {
			return nil, fmt.Errorf("export[%d] read serializationBeforeSerializationDeps: %w", i+1, err)
		}
		pos.createBeforeSerializationDeps = r.Offset()
		if _, err := r.ReadInt32(); err != nil {
			return nil, fmt.Errorf("export[%d] read createBeforeSerializationDeps: %w", i+1, err)
		}
		pos.serializationBeforeCreateDependencies = r.Offset()
		if _, err := r.ReadInt32(); err != nil {
			return nil, fmt.Errorf("export[%d] read serializationBeforeCreateDependencies: %w", i+1, err)
		}
		pos.createBeforeCreateDependencies = r.Offset()
		if _, err := r.ReadInt32(); err != nil {
			return nil, fmt.Errorf("export[%d] read createBeforeCreateDependencies: %w", i+1, err)
		}
		if !asset.Summary.UsesUnversionedPropertySerialization() && asset.Summary.FileVersionUE5 >= ue5ScriptSerializationOffset {
			if err := r.Skip(16); err != nil {
				return nil, fmt.Errorf("export[%d] read script serialization offsets: %w", i+1, err)
			}
		}
		fields = append(fields, pos)
	}
	return fields, nil
}

func parseUint32Any(v any) (uint32, error) {
	switch t := v.(type) {
	case uint32:
		return t, nil
	case uint64:
		if t > uint64(^uint32(0)) {
			return 0, fmt.Errorf("uint32 overflow: %d", t)
		}
		return uint32(t), nil
	case int:
		if t < 0 {
			return 0, fmt.Errorf("negative value is not allowed: %d", t)
		}
		if uint64(t) > uint64(^uint32(0)) {
			return 0, fmt.Errorf("uint32 overflow: %d", t)
		}
		return uint32(t), nil
	case int64:
		if t < 0 || uint64(t) > uint64(^uint32(0)) {
			return 0, fmt.Errorf("uint32 out of range: %d", t)
		}
		return uint32(t), nil
	case float64:
		if t < 0 || t > float64(^uint32(0)) || t != float64(uint32(t)) {
			return 0, fmt.Errorf("uint32 out of range: %v", t)
		}
		return uint32(t), nil
	case json.Number:
		u, err := strconv.ParseUint(t.String(), 0, 32)
		if err != nil {
			return 0, err
		}
		return uint32(u), nil
	case string:
		u, err := strconv.ParseUint(strings.TrimSpace(t), 0, 32)
		if err != nil {
			return 0, err
		}
		return uint32(u), nil
	default:
		return 0, fmt.Errorf("unsupported value type: %T", v)
	}
}

func parseInt32Any(v any) (int32, error) {
	switch t := v.(type) {
	case int32:
		return t, nil
	case int:
		if t < -2147483648 || t > 2147483647 {
			return 0, fmt.Errorf("int32 out of range: %d", t)
		}
		return int32(t), nil
	case int64:
		if t < -2147483648 || t > 2147483647 {
			return 0, fmt.Errorf("int32 out of range: %d", t)
		}
		return int32(t), nil
	case float64:
		if t < -2147483648 || t > 2147483647 || t != float64(int32(t)) {
			return 0, fmt.Errorf("int32 out of range: %v", t)
		}
		return int32(t), nil
	case json.Number:
		i, err := strconv.ParseInt(t.String(), 0, 32)
		if err != nil {
			return 0, err
		}
		return int32(i), nil
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(t), 0, 32)
		if err != nil {
			return 0, err
		}
		return int32(i), nil
	default:
		return 0, fmt.Errorf("unsupported value type: %T", v)
	}
}

func parseBoolAny(v any) (bool, error) {
	switch t := v.(type) {
	case bool:
		return t, nil
	case int:
		if t == 0 {
			return false, nil
		}
		if t == 1 {
			return true, nil
		}
	case int64:
		if t == 0 {
			return false, nil
		}
		if t == 1 {
			return true, nil
		}
	case float64:
		if t == 0 {
			return false, nil
		}
		if t == 1 {
			return true, nil
		}
	case json.Number:
		i, err := strconv.ParseInt(t.String(), 10, 64)
		if err == nil {
			if i == 0 {
				return false, nil
			}
			if i == 1 {
				return true, nil
			}
		}
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		if s == "true" || s == "1" {
			return true, nil
		}
		if s == "false" || s == "0" {
			return false, nil
		}
	}
	return false, fmt.Errorf("expected bool or 0/1, got %T", v)
}
