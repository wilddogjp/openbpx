package cli

import (
	"bytes"
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"sort"
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

const (
	locResVersionLegacy                   = uint8(0)
	locResVersionCompact                  = uint8(1)
	locResVersionOptimizedCRC32           = uint8(2)
	locResVersionOptimizedCityHash64UTF16 = uint8(3)
	maxLocalizationContainerEntries       = 1_000_000
	maxLocMetadataDepth                   = 64

	locMetadataTypeNone    = int32(0)
	locMetadataTypeBoolean = int32(1)
	locMetadataTypeString  = int32(2)
	locMetadataTypeArray   = int32(3)
	locMetadataTypeObject  = int32(4)
)

var locResMagicBytes = [16]byte{
	0x0e, 0x14, 0x74, 0x75,
	0x67, 0x4a, 0x03, 0xfc,
	0x4a, 0x15, 0x90, 0x9d,
	0xc3, 0x37, 0x7f, 0x1b,
}

type locResEntry struct {
	SourceStringHash uint32
	LocalizedString  string
}

type locResResource struct {
	Version uint8
	Entries map[string]map[string]locResEntry
}

type localizationCollectContext struct {
	export     int
	objectName string
	className  string
}

type gatherableTextDataEntry struct {
	NamespaceName      string
	SourceString       string
	SourceStringMeta   map[string]any
	SourceSiteContexts []gatherableTextSourceSiteContext
}

type gatherableTextSourceSiteContext struct {
	KeyName         string
	SiteDescription string
	IsEditorOnly    bool
	IsOptional      bool
	InfoMetaData    map[string]any
	KeyMetaData     map[string]any
}

func runLocalization(args []string, stdout, stderr io.Writer) int {
	return dispatchSubcommand(
		args,
		stdout,
		stderr,
		"usage: bpx localization <read|query|resolve|set-source|set-id|set-stringtable-ref|rewrite-namespace|rekey> ...",
		"unknown localization command: %s\n",
		subcommandSpec{Name: "read", Run: runLocalizationRead},
		subcommandSpec{Name: "query", Run: runLocalizationQuery},
		subcommandSpec{Name: "resolve", Run: runLocalizationResolve},
		subcommandSpec{Name: "set-source", Run: runLocalizationSetSource},
		subcommandSpec{Name: "set-id", Run: runLocalizationSetID},
		subcommandSpec{Name: "set-stringtable-ref", Run: runLocalizationSetStringTableRef},
		subcommandSpec{Name: "rewrite-namespace", Run: runLocalizationRewriteNamespace},
		subcommandSpec{Name: "rekey", Run: runLocalizationRekey},
	)
}

func runLocalizationRead(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("localization read", stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("export", 0, "optional 1-based export index filter")
	includeHistory := fs.Bool("include-history", false, "include full history payload")
	allowOutputFormats(fs, "output format: json, toml, or csv", structuredOutputFormatJSON, structuredOutputFormatTOML, "csv")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: bpx localization read <file.uasset> [--export <n>] [--include-history] [--format json|toml|csv]")
		return 1
	}
	format := outputFormatFromFlagSet(fs)
	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	entries, warnings, err := collectLocalizationEntries(asset, *exportIndex, *includeHistory)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	sortLocalizationEntries(entries)

	switch format {
	case "json", "toml":
		return printJSON(stdout, map[string]any{
			"file":                    file,
			"exportFilter":            *exportIndex,
			"includeHistory":          *includeHistory,
			"entryCount":              len(entries),
			"entries":                 entries,
			"warnings":                warnings,
			"gatherableTextDataCount": asset.Summary.GatherableTextDataCount,
		})
	case "csv":
		return writeLocalizationEntriesCSV(stdout, entries, *includeHistory)
	default:
		fmt.Fprintf(stderr, "error: unsupported format: %s\n", format)
		return 1
	}
}

func runLocalizationQuery(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("localization query", stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("export", 0, "optional 1-based export index filter")
	namespace := fs.String("namespace", "", "exact namespace filter")
	key := fs.String("key", "", "exact key filter")
	textToken := fs.String("text", "", "substring match against text fields")
	historyType := fs.String("history-type", "", "history type filter")
	limit := fs.Int("limit", 0, "result limit")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: bpx localization query <file.uasset> [--export <n>] [--namespace <ns>] [--key <key>] [--text <token>] [--history-type <type>] [--limit <n>]")
		return 1
	}
	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	entries, warnings, err := collectLocalizationEntries(asset, *exportIndex, false)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	entries = filterLocalizationEntries(entries, *namespace, *key, *textToken, *historyType, *limit)
	sortLocalizationEntries(entries)

	return printJSON(stdout, map[string]any{
		"file":         file,
		"exportFilter": *exportIndex,
		"filters": map[string]any{
			"namespace":   *namespace,
			"key":         *key,
			"text":        *textToken,
			"historyType": *historyType,
			"limit":       *limit,
		},
		"entryCount": len(entries),
		"entries":    entries,
		"warnings":   warnings,
	})
}

func runLocalizationResolve(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("localization resolve", stderr)
	opts := registerCommonFlags(fs)
	exportIndex := fs.Int("export", 0, "optional 1-based export index filter")
	culture := fs.String("culture", "", "target culture")
	locResPath := fs.String("locres", "", "optional .locres file path")
	missingOnly := fs.Bool("missing-only", false, "show only unresolved entries")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || strings.TrimSpace(*culture) == "" {
		fmt.Fprintln(stderr, "usage: bpx localization resolve <file.uasset> [--export <n>] --culture <culture> [--locres <path>] [--missing-only]")
		return 1
	}
	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	entries, warnings, err := collectLocalizationEntries(asset, *exportIndex, false)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	var locRes *locResResource
	if strings.TrimSpace(*locResPath) != "" {
		locRes, err = loadLocResFile(*locResPath)
		if err != nil {
			fmt.Fprintf(stderr, "error: load locres: %v\n", err)
			return 1
		}
	}

	resolvedEntries := make([]map[string]any, 0, len(entries))
	missingCount := 0
	for _, entry := range entries {
		if resolveLocalizationEntry(entry, *culture, locRes) {
			missingCount++
		}
		if *missingOnly {
			if missing, _ := entry["missing"].(bool); !missing {
				continue
			}
		}
		resolvedEntries = append(resolvedEntries, entry)
	}
	sortLocalizationEntries(resolvedEntries)

	resp := map[string]any{
		"file":                    file,
		"culture":                 *culture,
		"exportFilter":            *exportIndex,
		"missingOnly":             *missingOnly,
		"entryCount":              len(resolvedEntries),
		"missingCount":            missingCount,
		"resolvedCount":           len(entries) - missingCount,
		"entries":                 resolvedEntries,
		"warnings":                warnings,
		"gatherableTextDataCount": asset.Summary.GatherableTextDataCount,
	}
	if locRes != nil {
		resp["locres"] = map[string]any{
			"path":    *locResPath,
			"version": locRes.Version,
		}
	}
	return printJSON(stdout, resp)
}

func collectLocalizationEntries(asset *uasset.Asset, exportIndex int, includeHistory bool) ([]map[string]any, []string, error) {
	targets, err := resolveLocalizationTargets(asset, exportIndex)
	if err != nil {
		return nil, nil, err
	}

	entries := make([]map[string]any, 0, 64)
	warnings := make([]string, 0, 16)
	for _, idx := range targets {
		exp := asset.Exports[idx]
		props := asset.ParseExportProperties(idx)
		for _, warning := range props.Warnings {
			warnings = append(warnings, fmt.Sprintf("export %d (%s): %s", idx+1, exp.ObjectName.Display(asset.Names), warning))
		}

		ctx := localizationCollectContext{
			export:     idx + 1,
			objectName: exp.ObjectName.Display(asset.Names),
			className:  asset.ResolveClassName(exp),
		}
		for _, p := range props.Properties {
			value, ok := asset.DecodePropertyValue(p)
			if !ok {
				continue
			}
			enrichDecodedTextValue(value)
			path := p.Name.Display(asset.Names)
			if path == "" {
				path = "<unnamed>"
			}
			collectLocalizationFromValue(ctx, path, p.TypeString(asset.Names), value, includeHistory, &entries)
		}
	}

	gatherableEntries, gatherableWarnings := collectGatherableLocalizationEntries(asset, exportIndex, includeHistory)
	entries = append(entries, gatherableEntries...)
	warnings = append(warnings, gatherableWarnings...)

	return entries, warnings, nil
}

func resolveLocalizationTargets(asset *uasset.Asset, exportIndex int) ([]int, error) {
	if exportIndex > 0 {
		idx, err := asset.ResolveExportIndex(exportIndex)
		if err != nil {
			return nil, fmt.Errorf("resolve export index: %w", err)
		}
		return []int{idx}, nil
	}
	targets := make([]int, 0, len(asset.Exports))
	for i := range asset.Exports {
		targets = append(targets, i)
	}
	return targets, nil
}

func collectLocalizationFromValue(
	ctx localizationCollectContext,
	path string,
	typeName string,
	value any,
	includeHistory bool,
	out *[]map[string]any,
) {
	rootType := propertyRootType(typeName)
	if rootType == "TextProperty" {
		if textHistory, ok := value.(map[string]any); ok && isTextHistoryValue(textHistory) {
			entry := map[string]any{
				"source":     "TextProperty",
				"export":     ctx.export,
				"objectName": ctx.objectName,
				"className":  ctx.className,
				"path":       path,
				"type":       typeName,
			}
			copyTextHistoryFields(entry, textHistory)
			if includeHistory {
				entry["history"] = textHistory
			}
			*out = append(*out, entry)
		}
		return
	}

	valueMap, ok := value.(map[string]any)
	if !ok {
		return
	}

	switch rootType {
	case "StructProperty":
		fields, ok := valueMap["value"].(map[string]any)
		if !ok {
			return
		}
		for fieldName, fieldRaw := range fields {
			fieldMap, ok := fieldRaw.(map[string]any)
			if !ok {
				continue
			}
			fieldType, _ := fieldMap["type"].(string)
			fieldValue, hasValue := fieldMap["value"]
			if !hasValue {
				continue
			}
			collectLocalizationFromValue(ctx, joinPropertyPath(path, fieldName), fieldType, fieldValue, includeHistory, out)
		}
	case "ArrayProperty", "SetProperty":
		collectLocalizationFromTypedList(ctx, path, valueMap, includeHistory, out)
	case "MapProperty":
		collectLocalizationFromMapValue(ctx, path, valueMap, includeHistory, out)
	case "OptionalProperty":
		if isSet, _ := valueMap["isSet"].(bool); !isSet {
			return
		}
		valueWrapper, ok := valueMap["value"].(map[string]any)
		if !ok {
			return
		}
		innerType, _ := valueWrapper["type"].(string)
		innerValue, hasValue := valueWrapper["value"]
		if !hasValue {
			return
		}
		collectLocalizationFromValue(ctx, path, innerType, innerValue, includeHistory, out)
	}
}

func collectLocalizationFromTypedList(
	ctx localizationCollectContext,
	path string,
	valueMap map[string]any,
	includeHistory bool,
	out *[]map[string]any,
) {
	for _, fieldName := range []string{"value", "removed", "modified", "added", "shadowed"} {
		items, ok := asAnySlice(valueMap[fieldName])
		if !ok {
			continue
		}
		for idx, itemRaw := range items {
			itemMap, ok := itemRaw.(map[string]any)
			if !ok {
				continue
			}
			itemType, _ := itemMap["type"].(string)
			itemValue, hasValue := itemMap["value"]
			if !hasValue {
				continue
			}
			itemPath := indexPropertyPath(path, idx)
			if fieldName != "value" {
				itemPath = joinPropertyPath(path, fmt.Sprintf("%s[%d]", fieldName, idx))
			}
			collectLocalizationFromValue(ctx, itemPath, itemType, itemValue, includeHistory, out)
		}
	}
}

func collectLocalizationFromMapValue(
	ctx localizationCollectContext,
	path string,
	valueMap map[string]any,
	includeHistory bool,
	out *[]map[string]any,
) {
	for _, fieldName := range []string{"value", "modified", "added", "shadowed"} {
		items, ok := asAnySlice(valueMap[fieldName])
		if !ok {
			continue
		}
		for idx, itemRaw := range items {
			itemMap, ok := itemRaw.(map[string]any)
			if !ok {
				continue
			}
			if keyMap, ok := itemMap["key"].(map[string]any); ok {
				if keyType, ok := keyMap["type"].(string); ok {
					if keyValue, hasValue := keyMap["value"]; hasValue {
						keyPath := fmt.Sprintf("%s[%d].key", path, idx)
						if fieldName != "value" {
							keyPath = joinPropertyPath(path, fmt.Sprintf("%s[%d].key", fieldName, idx))
						}
						collectLocalizationFromValue(ctx, keyPath, keyType, keyValue, includeHistory, out)
					}
				}
			}
			if valueNodeMap, ok := itemMap["value"].(map[string]any); ok {
				if valueType, ok := valueNodeMap["type"].(string); ok {
					if valueNode, hasValue := valueNodeMap["value"]; hasValue {
						valuePath := fmt.Sprintf("%s[%d].value", path, idx)
						if fieldName != "value" {
							valuePath = joinPropertyPath(path, fmt.Sprintf("%s[%d].value", fieldName, idx))
						}
						collectLocalizationFromValue(ctx, valuePath, valueType, valueNode, includeHistory, out)
					}
				}
			}
		}
	}

	removed, ok := asAnySlice(valueMap["removed"])
	if !ok {
		return
	}
	for idx, itemRaw := range removed {
		itemMap, ok := itemRaw.(map[string]any)
		if !ok {
			continue
		}
		itemType, _ := itemMap["type"].(string)
		itemValue, hasValue := itemMap["value"]
		if !hasValue {
			continue
		}
		collectLocalizationFromValue(ctx, joinPropertyPath(path, fmt.Sprintf("removed[%d]", idx)), itemType, itemValue, includeHistory, out)
	}
}

func copyTextHistoryFields(entry map[string]any, history map[string]any) {
	if historyType, ok := history["historyType"].(string); ok {
		entry["historyType"] = historyType
	}
	if historyTypeCode, ok := history["historyTypeCode"]; ok {
		entry["historyTypeCode"] = historyTypeCode
	}
	if namespace, ok := history["namespace"].(string); ok {
		entry["namespace"] = namespace
	}
	if key, ok := history["key"].(string); ok {
		entry["key"] = key
	}
	if sourceString, ok := history["sourceString"].(string); ok && sourceString != "" {
		entry["sourceString"] = sourceString
	}
	if invariant, ok := history["cultureInvariantString"].(string); ok && invariant != "" {
		entry["cultureInvariantString"] = invariant
	}
	if tableIDName, ok := history["tableIdName"].(string); ok && tableIDName != "" {
		entry["tableIdName"] = tableIDName
	}
	if display := textDisplayFromDecodedHistory(history); display != "" {
		entry["displayString"] = display
	}
}

func filterLocalizationEntries(
	entries []map[string]any,
	namespace string,
	key string,
	textToken string,
	historyType string,
	limit int,
) []map[string]any {
	filtered := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		if namespace != "" {
			entryNamespace, _ := entry["namespace"].(string)
			if entryNamespace != namespace {
				continue
			}
		}
		if key != "" {
			entryKey, _ := entry["key"].(string)
			if entryKey != key {
				continue
			}
		}
		if historyType != "" {
			entryHistoryType, _ := entry["historyType"].(string)
			if !strings.EqualFold(entryHistoryType, historyType) {
				continue
			}
		}
		if textToken != "" {
			if !localizationEntryContainsText(entry, textToken) {
				continue
			}
		}
		filtered = append(filtered, entry)
		if limit > 0 && len(filtered) >= limit {
			break
		}
	}
	return filtered
}

func localizationEntryContainsText(entry map[string]any, token string) bool {
	for _, field := range []string{"displayString", "sourceString", "cultureInvariantString", "namespace", "key", "path"} {
		base, _ := entry[field].(string)
		if containsFold(base, token) {
			return true
		}
	}
	return false
}

func resolveLocalizationEntry(entry map[string]any, culture string, locRes *locResResource) bool {
	namespace, hasNamespace := entry["namespace"].(string)
	key, hasKey := entry["key"].(string)
	historyType, _ := entry["historyType"].(string)
	tableIDName, hasTableID := entry["tableIdName"].(string)

	resolved := ""
	if display, ok := entry["displayString"].(string); ok {
		resolved = display
	}
	resolvedBy := "payload"
	missing := false

	sourceHash, hasSourceHash := localizationEntrySourceStringHash(entry)
	shouldLookupLocRes := locRes != nil && hasKey && key != "" &&
		(strings.EqualFold(historyType, "Base") || strings.EqualFold(historyType, "StringTableEntry"))

	if shouldLookupLocRes {
		candidates := localizationNamespaceCandidates(namespace, hasNamespace, historyType, tableIDName, hasTableID)
		found := false
		hashMismatch := false
		for _, candidateNamespace := range candidates {
			translation, ok := locRes.lookup(candidateNamespace, key)
			if !ok || translation.LocalizedString == "" {
				continue
			}

			if hasSourceHash && translation.SourceStringHash != sourceHash {
				hashMismatch = true
				entry["sourceStringHash"] = sourceHash
				entry["locresSourceHash"] = translation.SourceStringHash
				continue
			}

			resolved = translation.LocalizedString
			resolvedBy = "locres"
			entry["locresSourceHash"] = translation.SourceStringHash
			if hasSourceHash {
				entry["sourceStringHash"] = sourceHash
			}
			if candidateNamespace != namespace {
				entry["locresNamespace"] = candidateNamespace
			}
			found = true
			break
		}
		if !found {
			missing = true
			if hashMismatch {
				resolvedBy = "sourceHashMismatch"
			} else {
				resolvedBy = "missing"
			}
		}
	} else {
		switch historyType {
		case "None":
			resolvedBy = "cultureInvariant"
		case "Base":
			resolvedBy = "sourceString"
		case "StringTableEntry":
			resolvedBy = "stringTableReference"
		}
	}

	entry["culture"] = culture
	if resolved != "" {
		entry["resolvedString"] = resolved
	}
	entry["resolvedBy"] = resolvedBy
	entry["missing"] = missing
	entry["resolved"] = !missing && resolved != ""
	return missing
}

func localizationEntrySourceStringHash(entry map[string]any) (uint32, bool) {
	sourceString, _ := entry["sourceString"].(string)
	if sourceString == "" {
		return 0, false
	}
	return ueTextSourceStringHash(sourceString), true
}

func ueTextSourceStringHash(source string) uint32 {
	// UE's FTextLocalizationResource::HashString uses FCrc::StrCrc32<TCHAR>.
	// For BPX we mirror the TCHAR(UTF-16 code unit) path used by asset tooling.
	crc := ^uint32(0)
	for _, unit := range utf16.Encode([]rune(source)) {
		ch := uint32(unit)
		for i := 0; i < 4; i++ {
			crc = (crc >> 8) ^ crc32.IEEETable[(crc^ch)&0xFF]
			ch >>= 8
		}
	}
	return ^crc
}

func localizationNamespaceCandidates(
	namespace string,
	hasNamespace bool,
	historyType string,
	tableIDName string,
	hasTableID bool,
) []string {
	candidates := make([]string, 0, 4)
	addCandidate := func(v string) {
		for _, existing := range candidates {
			if existing == v {
				return
			}
		}
		candidates = append(candidates, v)
	}

	if hasNamespace {
		addCandidate(namespace)
		stripped := stripPackageNamespace(namespace)
		if stripped != namespace {
			addCandidate(stripped)
		}
	}
	if strings.EqualFold(historyType, "StringTableEntry") {
		if hasTableID && tableIDName != "" {
			addCandidate(tableIDName)
			stripped := stripPackageNamespace(tableIDName)
			if stripped != tableIDName {
				addCandidate(stripped)
			}
		}
		if !hasNamespace {
			addCandidate("")
		}
	} else if !hasNamespace {
		addCandidate("")
	}

	return candidates
}

// stripPackageNamespace removes trailing package namespace marker like "... [<id>]".
func stripPackageNamespace(namespace string) string {
	if len(namespace) == 0 || namespace[len(namespace)-1] != ']' {
		return namespace
	}
	start := strings.LastIndexByte(namespace, '[')
	if start < 0 {
		return namespace
	}
	return strings.TrimRightFunc(namespace[:start], unicode.IsSpace)
}

func writeLocalizationEntriesCSV(w io.Writer, entries []map[string]any, includeHistory bool) int {
	writer := csv.NewWriter(w)
	headers := []string{
		"export", "objectName", "className", "path", "type",
		"historyType", "namespace", "key", "sourceString", "cultureInvariantString", "displayString",
	}
	if includeHistory {
		headers = append(headers, "history")
	}
	if err := writer.Write(headers); err != nil {
		return 1
	}
	for _, entry := range entries {
		row := []string{
			fmt.Sprint(entry["export"]),
			toEntryString(entry["objectName"]),
			toEntryString(entry["className"]),
			toEntryString(entry["path"]),
			toEntryString(entry["type"]),
			toEntryString(entry["historyType"]),
			toEntryString(entry["namespace"]),
			toEntryString(entry["key"]),
			toEntryString(entry["sourceString"]),
			toEntryString(entry["cultureInvariantString"]),
			toEntryString(entry["displayString"]),
		}
		if includeHistory {
			row = append(row, renderAnyValue(entry["history"]))
		}
		if err := writer.Write(row); err != nil {
			return 1
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return 1
	}
	return 0
}

func toEntryString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func loadLocResFile(path string) (*locResResource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return parseLocResBytes(data)
}

func parseLocResBytes(data []byte) (*locResResource, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty locres data")
	}
	r := uasset.NewByteReader(data)
	version := locResVersionLegacy
	if len(data) >= len(locResMagicBytes) {
		header, err := r.ReadBytes(len(locResMagicBytes))
		if err != nil {
			return nil, fmt.Errorf("read magic: %w", err)
		}
		if bytes.Equal(header, locResMagicBytes[:]) {
			ver, err := r.ReadUint8()
			if err != nil {
				return nil, fmt.Errorf("read version: %w", err)
			}
			version = ver
		} else {
			if err := r.Seek(0); err != nil {
				return nil, fmt.Errorf("rewind for legacy: %w", err)
			}
		}
	}
	if version > locResVersionOptimizedCityHash64UTF16 {
		return nil, fmt.Errorf("unsupported locres version: %d", version)
	}

	localizedStrings := make([]string, 0)
	if version >= locResVersionCompact {
		arrayOffset, err := r.ReadInt64()
		if err != nil {
			return nil, fmt.Errorf("read localized string array offset: %w", err)
		}
		if arrayOffset != -1 {
			if arrayOffset < 0 || arrayOffset > int64(len(data)) {
				return nil, fmt.Errorf("localized string array offset out of range: %d", arrayOffset)
			}
			cur := r.Offset()
			if err := r.Seek(int(arrayOffset)); err != nil {
				return nil, fmt.Errorf("seek localized string array: %w", err)
			}
			localizedStrings, err = readLocResLocalizedStringArray(r, version)
			if err != nil {
				return nil, fmt.Errorf("read localized string array: %w", err)
			}
			if err := r.Seek(cur); err != nil {
				return nil, fmt.Errorf("seek back to entries: %w", err)
			}
		}
	}

	if version >= locResVersionOptimizedCRC32 {
		if _, err := r.ReadUint32(); err != nil {
			return nil, fmt.Errorf("read entry count: %w", err)
		}
	}
	namespaceCount, err := r.ReadUint32()
	if err != nil {
		return nil, fmt.Errorf("read namespace count: %w", err)
	}
	if namespaceCount > maxLocalizationContainerEntries {
		return nil, fmt.Errorf("namespace count too large: %d", namespaceCount)
	}

	out := &locResResource{
		Version: version,
		Entries: map[string]map[string]locResEntry{},
	}
	for i := uint32(0); i < namespaceCount; i++ {
		namespace, err := readLocResTextKey(r, version)
		if err != nil {
			return nil, fmt.Errorf("read namespace key[%d]: %w", i, err)
		}
		keyCount, err := r.ReadUint32()
		if err != nil {
			return nil, fmt.Errorf("read key count[%d]: %w", i, err)
		}
		if keyCount > maxLocalizationContainerEntries {
			return nil, fmt.Errorf("key count too large at namespace[%d]: %d", i, keyCount)
		}
		keys := out.Entries[namespace]
		if keys == nil {
			keys = map[string]locResEntry{}
			out.Entries[namespace] = keys
		}
		for j := uint32(0); j < keyCount; j++ {
			key, err := readLocResTextKey(r, version)
			if err != nil {
				return nil, fmt.Errorf("read key[%d][%d]: %w", i, j, err)
			}
			sourceHash, err := r.ReadUint32()
			if err != nil {
				return nil, fmt.Errorf("read source hash[%d][%d]: %w", i, j, err)
			}
			localizedString := ""
			if version >= locResVersionCompact {
				localizedStringIndex, err := r.ReadInt32()
				if err != nil {
					return nil, fmt.Errorf("read localized string index[%d][%d]: %w", i, j, err)
				}
				if localizedStringIndex >= 0 && localizedStringIndex < int32(len(localizedStrings)) {
					localizedString = localizedStrings[localizedStringIndex]
				}
			} else {
				localizedString, err = r.ReadFString()
				if err != nil {
					return nil, fmt.Errorf("read legacy localized string[%d][%d]: %w", i, j, err)
				}
			}
			keys[key] = locResEntry{
				SourceStringHash: sourceHash,
				LocalizedString:  localizedString,
			}
		}
	}
	return out, nil
}

func readLocResLocalizedStringArray(r *uasset.ByteReader, version uint8) ([]string, error) {
	count, err := r.ReadInt32()
	if err != nil {
		return nil, err
	}
	if count < 0 || count > maxLocalizationContainerEntries {
		return nil, fmt.Errorf("localized string count out of range: %d", count)
	}
	out := make([]string, 0, count)
	for i := int32(0); i < count; i++ {
		str, err := r.ReadFString()
		if err != nil {
			return nil, fmt.Errorf("read localized string[%d]: %w", i, err)
		}
		if version >= locResVersionOptimizedCRC32 {
			if _, err := r.ReadInt32(); err != nil {
				return nil, fmt.Errorf("read localized string refcount[%d]: %w", i, err)
			}
		}
		out = append(out, str)
	}
	return out, nil
}

func readLocResTextKey(r *uasset.ByteReader, version uint8) (string, error) {
	if version >= locResVersionOptimizedCRC32 {
		if _, err := r.ReadUint32(); err != nil {
			return "", err
		}
	}
	return r.ReadFString()
}

func (r *locResResource) lookup(namespace, key string) (locResEntry, bool) {
	if r == nil {
		return locResEntry{}, false
	}
	keys := r.Entries[namespace]
	if keys == nil {
		return locResEntry{}, false
	}
	entry, ok := keys[key]
	return entry, ok
}

func sortLocalizationEntries(entries []map[string]any) {
	sort.Slice(entries, func(i, j int) bool {
		ei := entries[i]
		ej := entries[j]
		expI, _ := toInt64(ei["export"])
		expJ, _ := toInt64(ej["export"])
		if expI != expJ {
			return expI < expJ
		}
		pathI, _ := ei["path"].(string)
		pathJ, _ := ej["path"].(string)
		if pathI != pathJ {
			return pathI < pathJ
		}
		typeI, _ := ei["historyType"].(string)
		typeJ, _ := ej["historyType"].(string)
		return typeI < typeJ
	})
}

func toInt64(v any) (int64, bool) {
	switch t := v.(type) {
	case int:
		return int64(t), true
	case int8:
		return int64(t), true
	case int16:
		return int64(t), true
	case int32:
		return int64(t), true
	case int64:
		return t, true
	case uint8:
		return int64(t), true
	case uint16:
		return int64(t), true
	case uint32:
		return int64(t), true
	case uint64:
		if t > uint64(^uint64(0)>>1) {
			return 0, false
		}
		return int64(t), true
	default:
		return 0, false
	}
}

func propertyRootType(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if idx := strings.IndexByte(typeName, '('); idx >= 0 {
		return strings.TrimSpace(typeName[:idx])
	}
	return typeName
}

func joinPropertyPath(base, child string) string {
	if base == "" {
		return child
	}
	if child == "" {
		return base
	}
	return base + "." + child
}

func indexPropertyPath(base string, idx int) string {
	if base == "" {
		return fmt.Sprintf("[%d]", idx)
	}
	return fmt.Sprintf("%s[%d]", base, idx)
}

func asAnySlice(v any) ([]any, bool) {
	switch t := v.(type) {
	case []any:
		return t, true
	case []map[string]any:
		out := make([]any, 0, len(t))
		for _, item := range t {
			out = append(out, item)
		}
		return out, true
	default:
		return nil, false
	}
}

func collectGatherableLocalizationEntries(asset *uasset.Asset, exportIndex int, includeHistory bool) ([]map[string]any, []string) {
	if asset.Summary.GatherableTextDataCount <= 0 {
		return nil, nil
	}
	if exportIndex > 0 {
		return nil, []string{
			fmt.Sprintf("gatherable text data is package-level (count=%d) and is omitted when --export is specified", asset.Summary.GatherableTextDataCount),
		}
	}

	items, warnings := parseGatherableTextDataSection(asset)
	entries := make([]map[string]any, 0, len(items))
	for i, item := range items {
		if len(item.SourceSiteContexts) == 0 {
			warnings = append(warnings, fmt.Sprintf("gatherable text data[%d] has no source site contexts", i))
			continue
		}
		for j, ctx := range item.SourceSiteContexts {
			path := ctx.SiteDescription
			if strings.TrimSpace(path) == "" {
				path = fmt.Sprintf("GatherableTextData[%d].SourceSiteContexts[%d]", i, j)
			}
			entry := map[string]any{
				"source":          "GatherableTextData",
				"export":          0,
				"objectName":      "",
				"className":       "",
				"path":            path,
				"type":            "GatherableTextData",
				"historyType":     "Base",
				"historyTypeCode": uint8(0),
				"namespace":       item.NamespaceName,
				"key":             ctx.KeyName,
				"sourceString":    item.SourceString,
				"displayString":   item.SourceString,
				"isEditorOnly":    ctx.IsEditorOnly,
				"isOptional":      ctx.IsOptional,
			}
			if len(item.SourceStringMeta) > 0 {
				entry["sourceStringMetaData"] = item.SourceStringMeta
			}
			if len(ctx.InfoMetaData) > 0 {
				entry["infoMetaData"] = ctx.InfoMetaData
			}
			if len(ctx.KeyMetaData) > 0 {
				entry["keyMetaData"] = ctx.KeyMetaData
			}
			if includeHistory {
				entry["history"] = map[string]any{
					"historyType":     "Base",
					"historyTypeCode": uint8(0),
					"namespace":       item.NamespaceName,
					"key":             ctx.KeyName,
					"sourceString":    item.SourceString,
				}
			}
			entries = append(entries, entry)
		}
	}
	return entries, warnings
}

func parseGatherableTextDataSection(asset *uasset.Asset) ([]gatherableTextDataEntry, []string) {
	data, _, _, present := sectionByOffset(asset, int64(asset.Summary.GatherableTextDataOffset))
	if !present {
		return nil, []string{
			fmt.Sprintf(
				"gatherable text data section is not present (count=%d, offset=%d)",
				asset.Summary.GatherableTextDataCount,
				asset.Summary.GatherableTextDataOffset,
			),
		}
	}
	if asset.Summary.GatherableTextDataCount <= 0 {
		return nil, nil
	}

	r := uasset.NewByteReaderWithByteSwapping(data, asset.Summary.UsesByteSwappedSerialization())
	entries := make([]gatherableTextDataEntry, 0, asset.Summary.GatherableTextDataCount)
	warnings := make([]string, 0, 4)
	for i := int32(0); i < asset.Summary.GatherableTextDataCount; i++ {
		entry, err := readGatherableTextDataEntry(r)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("gatherable text data parse stopped at %d: %v", i, err))
			break
		}
		entries = append(entries, entry)
	}
	if r.Remaining() > 0 {
		warnings = append(warnings, fmt.Sprintf("gatherable text data trailing bytes: %d", r.Remaining()))
	}
	return entries, warnings
}

func readGatherableTextDataEntry(r *uasset.ByteReader) (gatherableTextDataEntry, error) {
	var out gatherableTextDataEntry

	namespace, err := r.ReadFString()
	if err != nil {
		return out, fmt.Errorf("read namespace name: %w", err)
	}
	out.NamespaceName = namespace

	source, err := r.ReadFString()
	if err != nil {
		return out, fmt.Errorf("read source string: %w", err)
	}
	out.SourceString = source

	sourceMeta, err := readLocMetadataObject(r, 0)
	if err != nil {
		return out, fmt.Errorf("read source string metadata: %w", err)
	}
	out.SourceStringMeta = sourceMeta

	contextCount, err := r.ReadInt32()
	if err != nil {
		return out, fmt.Errorf("read source site context count: %w", err)
	}
	if contextCount < 0 || contextCount > maxLocalizationContainerEntries {
		return out, fmt.Errorf("invalid source site context count: %d", contextCount)
	}

	out.SourceSiteContexts = make([]gatherableTextSourceSiteContext, 0, contextCount)
	for i := int32(0); i < contextCount; i++ {
		ctx, err := readGatherableTextSourceSiteContext(r)
		if err != nil {
			return out, fmt.Errorf("read source site context[%d]: %w", i, err)
		}
		out.SourceSiteContexts = append(out.SourceSiteContexts, ctx)
	}

	return out, nil
}

func readGatherableTextSourceSiteContext(r *uasset.ByteReader) (gatherableTextSourceSiteContext, error) {
	var out gatherableTextSourceSiteContext

	keyName, err := r.ReadFString()
	if err != nil {
		return out, fmt.Errorf("read key name: %w", err)
	}
	out.KeyName = keyName

	siteDescription, err := r.ReadFString()
	if err != nil {
		return out, fmt.Errorf("read site description: %w", err)
	}
	out.SiteDescription = siteDescription

	isEditorOnly, err := r.ReadUBool()
	if err != nil {
		return out, fmt.Errorf("read isEditorOnly: %w", err)
	}
	out.IsEditorOnly = isEditorOnly

	isOptional, err := r.ReadUBool()
	if err != nil {
		return out, fmt.Errorf("read isOptional: %w", err)
	}
	out.IsOptional = isOptional

	infoMetaData, err := readLocMetadataObject(r, 0)
	if err != nil {
		return out, fmt.Errorf("read info metadata: %w", err)
	}
	out.InfoMetaData = infoMetaData

	keyMetaData, err := readLocMetadataObject(r, 0)
	if err != nil {
		return out, fmt.Errorf("read key metadata: %w", err)
	}
	out.KeyMetaData = keyMetaData

	return out, nil
}

func readLocMetadataObject(r *uasset.ByteReader, depth int) (map[string]any, error) {
	if depth >= maxLocMetadataDepth {
		return nil, fmt.Errorf("loc metadata object nesting exceeds %d", maxLocMetadataDepth)
	}

	valueCount, err := r.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read value count: %w", err)
	}
	if valueCount < 0 || valueCount > maxLocalizationContainerEntries {
		return nil, fmt.Errorf("invalid metadata value count: %d", valueCount)
	}

	out := make(map[string]any, valueCount)
	for i := int32(0); i < valueCount; i++ {
		key, err := r.ReadFString()
		if err != nil {
			return nil, fmt.Errorf("read metadata key[%d]: %w", i, err)
		}
		value, err := readLocMetadataValue(r, depth+1)
		if err != nil {
			return nil, fmt.Errorf("read metadata value[%d]: %w", i, err)
		}
		out[key] = value
	}
	return out, nil
}

func readLocMetadataValue(r *uasset.ByteReader, depth int) (any, error) {
	if depth >= maxLocMetadataDepth {
		return nil, fmt.Errorf("loc metadata value nesting exceeds %d", maxLocMetadataDepth)
	}

	valueType, err := r.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read metadata value type: %w", err)
	}

	switch valueType {
	case locMetadataTypeString:
		str, err := r.ReadFString()
		if err != nil {
			return nil, fmt.Errorf("read metadata string: %w", err)
		}
		return str, nil
	case locMetadataTypeBoolean:
		b, err := r.ReadUBool()
		if err != nil {
			return nil, fmt.Errorf("read metadata bool: %w", err)
		}
		return b, nil
	case locMetadataTypeArray:
		count, err := r.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("read metadata array count: %w", err)
		}
		if count < 0 || count > maxLocalizationContainerEntries {
			return nil, fmt.Errorf("invalid metadata array count: %d", count)
		}
		out := make([]any, 0, count)
		for i := int32(0); i < count; i++ {
			value, err := readLocMetadataValue(r, depth+1)
			if err != nil {
				return nil, fmt.Errorf("read metadata array[%d]: %w", i, err)
			}
			out = append(out, value)
		}
		return out, nil
	case locMetadataTypeObject:
		obj, err := readLocMetadataObject(r, depth+1)
		if err != nil {
			return nil, fmt.Errorf("read metadata object: %w", err)
		}
		return obj, nil
	case locMetadataTypeNone:
		return nil, fmt.Errorf("unsupported metadata value type: None")
	default:
		return nil, fmt.Errorf("unsupported metadata value type: %d", valueType)
	}
}

// encodeLocResMagic is used in tests.
func encodeLocResMagic() []byte {
	buf := make([]byte, len(locResMagicBytes))
	copy(buf, locResMagicBytes[:])
	return buf
}

// patchInt64LE is used in tests.
func patchInt64LE(dst []byte, offset int, value int64) {
	binary.LittleEndian.PutUint64(dst[offset:offset+8], uint64(value))
}
