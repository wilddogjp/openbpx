package cli

import (
	"encoding/base64"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

const inlineDataLimit = 64 * 1024

func parseSingleAssetCommand(args []string, command, usage string, stderr io.Writer) (string, *uasset.Asset, bool) {
	fs := newFlagSet(command, stderr)
	opts := registerCommonFlags(fs)
	if err := parseFlagSet(fs, args); err != nil {
		return "", nil, false
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, usage)
		return "", nil, false
	}
	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return "", nil, false
	}
	return file, asset, true
}

func collectExportsByClassKeyword(asset *uasset.Asset, keywords []string) []map[string]any {
	outs := make([]map[string]any, 0, 4)
	for i, exp := range asset.Exports {
		className := strings.ToLower(asset.ResolveClassName(exp))
		matched := false
		for _, keyword := range keywords {
			if strings.Contains(className, strings.ToLower(keyword)) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		outs = append(outs, exportReadInfo(asset, i))
	}
	return outs
}

func exportReadInfo(asset *uasset.Asset, idx int) map[string]any {
	exp := asset.Exports[idx]
	props := asset.ParseExportProperties(idx)
	return map[string]any{
		"index":      idx + 1,
		"objectName": exp.ObjectName.Display(asset.Names),
		"className":  asset.ResolveClassName(exp),
		"classIndex": int32(exp.ClassIndex),
		"superIndex": int32(exp.SuperIndex),
		"outerIndex": int32(exp.OuterIndex),
		"serial": map[string]any{
			"offset":                   exp.SerialOffset,
			"size":                     exp.SerialSize,
			"scriptSerializationStart": exp.ScriptSerializationStartOffset,
			"scriptSerializationEnd":   exp.ScriptSerializationEndOffset,
		},
		"properties": toPropertyOutputs(asset, props.Properties, true),
		"warnings":   props.Warnings,
	}
}

func toPropertyOutputs(asset *uasset.Asset, props []uasset.PropertyTag, withValue bool) []map[string]any {
	items := make([]map[string]any, 0, len(props))
	for _, p := range props {
		item := map[string]any{
			"name":        p.Name.Display(asset.Names),
			"type":        p.TypeString(asset.Names),
			"size":        p.Size,
			"arrayIndex":  p.ArrayIndex,
			"offset":      p.Offset,
			"valueOffset": p.ValueOffset,
			"flags":       p.Flags,
		}
		if withValue {
			if v, ok := asset.DecodePropertyValue(p); ok {
				enrichDecodedTextValue(v)
				item["value"] = v
			}
		}
		items = append(items, item)
	}
	return items
}

func enrichDecodedTextValue(v any) {
	switch t := v.(type) {
	case map[string]any:
		if isTextHistoryValue(t) {
			if display := textDisplayFromDecodedHistory(t); display != "" {
				t["displayString"] = display
			}
		}
		for _, child := range t {
			enrichDecodedTextValue(child)
		}
	case []any:
		for _, child := range t {
			enrichDecodedTextValue(child)
		}
	case []map[string]any:
		for _, child := range t {
			enrichDecodedTextValue(child)
		}
	}
}

func isTextHistoryValue(m map[string]any) bool {
	_, hasType := m["historyType"]
	_, hasTypeCode := m["historyTypeCode"]
	return hasType && hasTypeCode
}

func textDisplayFromDecodedHistory(m map[string]any) string {
	if v, ok := m["displayString"].(string); ok && v != "" {
		return v
	}
	if v, ok := m["resolvedString"].(string); ok && v != "" {
		return v
	}
	if v, ok := m["value"].(string); ok && v != "" {
		return v
	}
	historyType, _ := m["historyType"].(string)
	switch historyType {
	case "Base":
		if v, ok := m["sourceString"].(string); ok && v != "" {
			return v
		}
	case "None":
		if v, ok := m["cultureInvariantString"].(string); ok && v != "" {
			return v
		}
	case "NamedFormat":
		if formatText, ok := m["formatText"].(map[string]any); ok {
			template := textDisplayFromDecodedHistory(formatText)
			if template == "" {
				return ""
			}
			return applyTextFormatTemplate(template, namedTextFormatArguments(m["arguments"]), nil)
		}
	case "OrderedFormat":
		if formatText, ok := m["formatText"].(map[string]any); ok {
			template := textDisplayFromDecodedHistory(formatText)
			if template == "" {
				return ""
			}
			return applyTextFormatTemplate(template, nil, orderedTextFormatArguments(m["arguments"]))
		}
	case "ArgumentFormat":
		if formatText, ok := m["formatText"].(map[string]any); ok {
			template := textDisplayFromDecodedHistory(formatText)
			if template == "" {
				return ""
			}
			named, ordered := argumentTextFormatArguments(m["arguments"])
			return applyTextFormatTemplate(template, named, ordered)
		}
	case "Transform":
		sourceText, ok := m["sourceText"].(map[string]any)
		if !ok {
			return ""
		}
		base := textDisplayFromDecodedHistory(sourceText)
		switch m["transformType"] {
		case "ToUpper":
			return strings.ToUpper(base)
		case "ToLower":
			return strings.ToLower(base)
		default:
			return base
		}
	}
	if historyTypeAllowsSourceValueFallback(historyType) {
		if sourceValue, ok := m["sourceValue"].(map[string]any); ok {
			if display := textDisplayFromFormatArgumentValue(sourceValue); display != "" {
				return display
			}
		}
	}
	return ""
}

func historyTypeAllowsSourceValueFallback(historyType string) bool {
	switch historyType {
	case "AsNumber", "AsPercent", "AsCurrency", "AsDate", "AsTime", "AsDateTime":
		return false
	default:
		return true
	}
}

func namedTextFormatArguments(raw any) map[string]string {
	arguments, ok := raw.([]map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(arguments))
	for _, argument := range arguments {
		name, _ := argument["name"].(string)
		valueMap, _ := argument["value"].(map[string]any)
		if name == "" || valueMap == nil {
			continue
		}
		if display := textDisplayFromFormatArgumentValue(valueMap); display != "" {
			out[name] = display
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func orderedTextFormatArguments(raw any) []string {
	values, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, item := range values {
		valueMap, _ := item.(map[string]any)
		if valueMap == nil {
			out = append(out, "")
			continue
		}
		out = append(out, textDisplayFromFormatArgumentValue(valueMap))
	}
	return out
}

func argumentTextFormatArguments(raw any) (map[string]string, []string) {
	arguments, ok := raw.([]map[string]any)
	if !ok {
		return nil, nil
	}
	named := make(map[string]string, len(arguments))
	ordered := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		valueMap := map[string]any{
			"value": argument["value"],
		}
		display := textDisplayFromFormatArgumentValue(valueMap)
		name, _ := argument["name"].(string)
		if name != "" {
			named[name] = display
		}
		ordered = append(ordered, display)
	}
	if len(named) == 0 {
		named = nil
	}
	return named, ordered
}

func applyTextFormatTemplate(template string, named map[string]string, ordered []string) string {
	if template == "" {
		return ""
	}
	var b strings.Builder
	for i := 0; i < len(template); {
		ch := template[i]
		if ch == '{' {
			if i+1 < len(template) && template[i+1] == '{' {
				b.WriteByte('{')
				i += 2
				continue
			}
			close := strings.IndexByte(template[i+1:], '}')
			if close < 0 {
				b.WriteByte(ch)
				i++
				continue
			}
			token := template[i+1 : i+1+close]
			if replacement, ok := resolveTextFormatToken(token, named, ordered); ok {
				b.WriteString(replacement)
			} else {
				b.WriteString(template[i : i+1+close+1])
			}
			i += close + 2
			continue
		}
		if ch == '}' && i+1 < len(template) && template[i+1] == '}' {
			b.WriteByte('}')
			i += 2
			continue
		}
		b.WriteByte(ch)
		i++
	}
	return b.String()
}

func resolveTextFormatToken(token string, named map[string]string, ordered []string) (string, bool) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return "", false
	}
	if ordered != nil {
		if idx, err := strconv.Atoi(trimmed); err == nil && idx >= 0 && idx < len(ordered) {
			return ordered[idx], true
		}
	}
	if named != nil {
		if v, ok := named[trimmed]; ok {
			return v, true
		}
	}
	return "", false
}

func textDisplayFromFormatArgumentValue(v map[string]any) string {
	value, hasValue := v["value"]
	if !hasValue {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	if m, ok := value.(map[string]any); ok {
		if isTextHistoryValue(m) {
			return textDisplayFromDecodedHistory(m)
		}
	}
	switch t := value.(type) {
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}

func baseSectionResponse(file, name string, begin, end int64, data []byte, present bool) map[string]any {
	resp := map[string]any{
		"file":    file,
		"section": name,
		"present": present,
	}
	if !present {
		return resp
	}
	resp["offset"] = begin
	resp["endOffset"] = end
	resp["size"] = len(data)
	addBase64Data(resp, data, false)
	return resp
}

func addBase64Data(payload map[string]any, data []byte, full bool) {
	if full || len(data) <= inlineDataLimit {
		payload["dataBase64"] = base64.StdEncoding.EncodeToString(data)
		payload["truncated"] = false
		return
	}
	payload["truncated"] = true
	payload["previewSize"] = inlineDataLimit
	payload["dataPreviewBase64"] = base64.StdEncoding.EncodeToString(data[:inlineDataLimit])
}

func sectionByOffset(asset *uasset.Asset, start int64) ([]byte, int64, int64, bool) {
	fileSize := int64(len(asset.Raw.Bytes))
	if start <= 0 || start >= fileSize {
		return nil, 0, 0, false
	}
	end := nextKnownOffset(asset, start)
	if end <= start || end > fileSize {
		end = fileSize
	}
	return asset.Raw.Bytes[start:end], start, end, true
}

func nextKnownOffset(asset *uasset.Asset, start int64) int64 {
	candidates := make([]int64, 0, 64)
	add := func(v int64) {
		if v > start && v <= int64(len(asset.Raw.Bytes)) {
			candidates = append(candidates, v)
		}
	}
	s := asset.Summary
	add(int64(s.TotalHeaderSize))
	add(int64(s.NameOffset))
	add(int64(s.SoftObjectPathsOffset))
	add(int64(s.GatherableTextDataOffset))
	add(int64(s.ExportOffset))
	add(int64(s.ImportOffset))
	add(int64(s.CellExportOffset))
	add(int64(s.CellImportOffset))
	add(int64(s.MetaDataOffset))
	add(int64(s.DependsOffset))
	add(int64(s.SoftPackageReferencesOffset))
	add(int64(s.SearchableNamesOffset))
	add(int64(s.ThumbnailTableOffset))
	add(int64(s.AssetRegistryDataOffset))
	add(s.BulkDataStartOffset)
	add(int64(s.WorldTileInfoDataOffset))
	add(int64(s.PreloadDependencyOffset))
	add(s.PayloadTOCOffset)
	add(int64(s.DataResourceOffset))
	for _, exp := range asset.Exports {
		add(exp.SerialOffset)
		add(exp.SerialOffset + exp.SerialSize)
	}
	add(int64(len(asset.Raw.Bytes)))

	if len(candidates) == 0 {
		return int64(len(asset.Raw.Bytes))
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i] < candidates[j] })
	return candidates[0]
}
