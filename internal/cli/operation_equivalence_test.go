package cli

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

type operationIgnoreRange struct {
	Offset int    `json:"offset"`
	Length int    `json:"length"`
	Reason string `json:"reason"`
}

type operationSpec struct {
	Command               string                 `json:"command"`
	Args                  map[string]any         `json:"args"`
	ActualFile            string                 `json:"actual_file,omitempty"`
	UEProcedure           string                 `json:"ue_procedure"`
	Expect                string                 `json:"expect"`
	ErrorContains         string                 `json:"error_contains"`
	Notes                 string                 `json:"notes"`
	IgnoreOffsets         []operationIgnoreRange `json:"ignore_offsets"`
	IgnorePackageSections []string               `json:"ignore_package_sections,omitempty"`
}

func TestOperationEquivalence(t *testing.T) {
	roots := goldenFixtureRoots(t, "operations")
	if len(roots) == 0 {
		t.Fatalf("no operations fixture roots found")
	}

	for _, root := range roots {
		root := root
		t.Run(filepath.Base(root), func(t *testing.T) {
			operationsDir := filepath.Join(root, "operations")
			entries, err := os.ReadDir(operationsDir)
			if err != nil {
				t.Fatalf("read operations dir: %v", err)
			}

			dirs := listOperationSpecDirs(entries, operationsDir)
			if len(dirs) == 0 {
				t.Fatalf("no operation fixture directories found")
			}

			byteEqualCount := 0
			for _, opDir := range dirs {
				opDir := opDir
				t.Run(filepath.Base(opDir), func(t *testing.T) {
					specPath := filepath.Join(opDir, "operation.json")
					beforePath, err := findOperationFixtureFile(opDir, "before")
					if err != nil {
						t.Fatalf("resolve before fixture: %v", err)
					}
					afterPath, err := findOperationFixtureFile(opDir, "after")
					if err != nil {
						t.Fatalf("resolve after fixture: %v", err)
					}

					specBytes, err := os.ReadFile(specPath)
					if err != nil {
						t.Fatalf("read operation spec: %v", err)
					}
					var spec operationSpec
					if err := json.Unmarshal(specBytes, &spec); err != nil {
						t.Fatalf("parse operation spec: %v", err)
					}
					if strings.TrimSpace(spec.Command) == "" {
						t.Fatalf("operation command must not be empty")
					}
					if len(spec.Args) == 0 {
						t.Fatalf("operation args must not be empty")
					}

					beforeBytes, err := os.ReadFile(beforePath)
					if err != nil {
						t.Fatalf("read before fixture: %v", err)
					}
					afterBytes, err := os.ReadFile(afterPath)
					if err != nil {
						t.Fatalf("read after fixture: %v", err)
					}

					tempDir := t.TempDir()
					tempFile := filepath.Join(tempDir, "work.uasset")
					if err := os.WriteFile(tempFile, beforeBytes, 0o644); err != nil {
						t.Fatalf("write temp fixture: %v", err)
					}

					argv, err := buildOperationArgv(spec, tempFile)
					if err != nil {
						t.Fatalf("build operation argv: %v", err)
					}

					var stdout bytes.Buffer
					var stderr bytes.Buffer
					code := Run(argv, &stdout, &stderr)

					expect := strings.TrimSpace(spec.Expect)
					if expect == "" {
						expect = "byte_equal"
					}

					switch expect {
					case "byte_equal":
						byteEqualCount++
						if code != 0 {
							t.Fatalf("operation command failed (code=%d): argv=%v stderr=%s", code, argv, stderr.String())
						}
					case "error_equal":
						if code == 0 {
							t.Fatalf("operation fixture was expected to fail: argv=%v stdout=%s", argv, stdout.String())
						}
						if spec.ErrorContains != "" && !strings.Contains(strings.ToLower(stderr.String()), strings.ToLower(spec.ErrorContains)) {
							t.Fatalf("stderr mismatch: want substring %q got %q", spec.ErrorContains, stderr.String())
						}
					default:
						t.Fatalf("unsupported expect value: %s", expect)
					}

					actualPath := resolveOperationPathTemplate(spec.ActualFile, tempFile)
					if actualPath == "" {
						actualPath = tempFile
					}
					actualBytes, err := os.ReadFile(actualPath)
					if err != nil {
						t.Fatalf("read command output bytes: %v", err)
					}

					match, err := equalBytesWithIgnoredOffsets(actualBytes, afterBytes, spec)
					if err != nil {
						t.Fatalf("compare output bytes: %v", err)
					}
					if !match {
						left, _ := comparableBytesWithIgnoredRegions(actualBytes, spec)
						right, _ := comparableBytesWithIgnoredRegions(afterBytes, spec)
						diffCount := 0
						for k := 0; k < len(left) && k < len(right); k++ {
							if left[k] != right[k] {
								if diffCount < 5 {
									t.Logf("  comparable byte diff at [%d]: actual=0x%02x expected=0x%02x", k, left[k], right[k])
								}
								diffCount++
							}
						}
						if len(left) != len(right) {
							t.Logf("  comparable size: actual=%d expected=%d", len(left), len(right))
						}
						t.Logf("  total comparable diffs: %d", diffCount)
						t.Fatalf("byte mismatch for operation fixture\nargv=%v", argv)
					}
				})
			}

			if byteEqualCount == 0 {
				t.Fatalf("no byte_equal operation fixtures found")
			}
		})
	}
}

func listOperationSpecDirs(entries []os.DirEntry, operationsDir string) []string {
	dirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		opDir := filepath.Join(operationsDir, entry.Name())
		if _, err := os.Stat(filepath.Join(opDir, "operation.json")); err != nil {
			continue
		}
		dirs = append(dirs, opDir)
	}
	sort.Strings(dirs)
	return dirs
}

func findOperationFixtureFile(opDir, stem string) (string, error) {
	for _, ext := range []string{".uasset", ".umap"} {
		path := filepath.Join(opDir, stem+ext)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("missing %s fixture in %s", stem, opDir)
}

func buildOperationArgv(spec operationSpec, targetFile string) ([]string, error) {
	parts := strings.Fields(spec.Command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	argv := append([]string{}, parts...)
	argv = append(argv, targetFile)

	keys := make([]string, 0, len(spec.Args))
	for key := range spec.Args {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value, err := formatOperationArgValue(spec.Args[key], targetFile)
		if err != nil {
			return nil, fmt.Errorf("format --%s: %w", key, err)
		}
		argv = append(argv, "--"+key, value)
	}
	return argv, nil
}

func formatOperationArgValue(v any, targetFile string) (string, error) {
	switch x := v.(type) {
	case string:
		return resolveOperationPathTemplate(x, targetFile), nil
	case bool:
		if x {
			return "true", nil
		}
		return "false", nil
	case float64:
		if x == math.Trunc(x) {
			return strconv.FormatInt(int64(x), 10), nil
		}
		return strconv.FormatFloat(x, 'g', -1, 64), nil
	case nil:
		return "null", nil
	default:
		buf, err := json.Marshal(x)
		if err != nil {
			return "", err
		}
		return string(buf), nil
	}
}

func resolveOperationPathTemplate(raw string, targetFile string) string {
	if raw == "" {
		return ""
	}
	return strings.ReplaceAll(raw, "{TARGET}", targetFile)
}

type byteRange struct {
	start int
	end   int
}

func equalBytesWithIgnoredOffsets(actual, expected []byte, spec operationSpec) (bool, error) {
	left, err := comparableBytesWithIgnoredRegions(actual, spec)
	if err != nil {
		return false, fmt.Errorf("actual comparable bytes: %w", err)
	}
	right, err := comparableBytesWithIgnoredRegions(expected, spec)
	if err != nil {
		return false, fmt.Errorf("expected comparable bytes: %w", err)
	}
	return bytes.Equal(left, right), nil
}

func comparableBytesWithIgnoredRegions(data []byte, spec operationSpec) ([]byte, error) {
	ranges := make([]byteRange, 0, len(spec.IgnoreOffsets)+8)
	for _, item := range spec.IgnoreOffsets {
		if item.Offset < 0 || item.Length < 0 {
			return nil, fmt.Errorf("invalid ignore range: offset=%d length=%d", item.Offset, item.Length)
		}
		end := item.Offset + item.Length
		if end > len(data) {
			return nil, fmt.Errorf("ignore range out of bounds: offset=%d length=%d size=%d", item.Offset, item.Length, len(data))
		}
		ranges = append(ranges, byteRange{start: item.Offset, end: end})
	}

	for _, token := range spec.IgnorePackageSections {
		sectionRanges, err := ignoredPackageSectionRanges(data, token, spec)
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, sectionRanges...)
	}

	merged, err := mergeIgnoredRanges(ranges, len(data))
	if err != nil {
		return nil, err
	}
	if len(merged) == 0 {
		return append([]byte(nil), data...), nil
	}

	out := make([]byte, 0, len(data))
	cursor := 0
	for _, item := range merged {
		if cursor < item.start {
			out = append(out, data[cursor:item.start]...)
		}
		cursor = item.end
	}
	if cursor < len(data) {
		out = append(out, data[cursor:]...)
	}
	return out, nil
}

func mergeIgnoredRanges(ranges []byteRange, size int) ([]byteRange, error) {
	if len(ranges) == 0 {
		return nil, nil
	}
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].start == ranges[j].start {
			return ranges[i].end < ranges[j].end
		}
		return ranges[i].start < ranges[j].start
	})
	merged := make([]byteRange, 0, len(ranges))
	for _, item := range ranges {
		if item.start < 0 || item.end < item.start || item.end > size {
			return nil, fmt.Errorf("invalid ignore range: start=%d end=%d size=%d", item.start, item.end, size)
		}
		if len(merged) == 0 || item.start > merged[len(merged)-1].end {
			merged = append(merged, item)
			continue
		}
		if item.end > merged[len(merged)-1].end {
			merged[len(merged)-1].end = item.end
		}
	}
	return merged, nil
}

func ignoredPackageSectionRanges(data []byte, token string, spec operationSpec) ([]byteRange, error) {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "", "none":
		return nil, nil
	case "editor-thumbnails":
		return ignoredEditorThumbnailRanges(data)
	case "saved-hash":
		return ignoredSavedHashRanges(data)
	case "blueprint-search-tail":
		return ignoredBlueprintSearchTailRanges(data)
	case "soft-package-references":
		return ignoredSoftPackageReferenceRanges(data)
	case "soft-package-searchable-summary-offsets":
		return ignoredSoftPackageSearchableSummaryOffsetRanges(data, spec)
	case "widget-text-localization-keys":
		return ignoredWidgetTextLocalizationKeyRanges(data, spec)
	case "widget-text-key-only":
		return ignoredWidgetTextKeyOnlyRanges(data, spec)
	case "widget-variable-guid-map-values":
		return ignoredWidgetVariableGuidMapValueRanges(data, spec)
	default:
		return nil, fmt.Errorf("unsupported ignore package section: %s", token)
	}
}

func ignoredWidgetTextLocalizationKeyRanges(data []byte, spec operationSpec) ([]byteRange, error) {
	if !strings.EqualFold(spec.Command, "blueprint widget-write") {
		return nil, nil
	}
	property := strings.TrimSpace(fmt.Sprint(spec.Args["property"]))
	propertyName := ""
	switch {
	case strings.EqualFold(property, "text"):
		propertyName = "Text"
	case strings.EqualFold(property, "editabletextbox-hint-text"),
		strings.EqualFold(property, "editabletext-hint-text"),
		strings.EqualFold(property, "multilineeditabletextbox-hint-text"):
		propertyName = "HintText"
	default:
		return nil, nil
	}
	widget := strings.TrimSpace(fmt.Sprint(spec.Args["widget"]))
	if widget == "" {
		return nil, nil
	}

	asset, err := uasset.ParseBytes(data, uasset.DefaultParseOptions())
	if err != nil {
		return nil, fmt.Errorf("parse asset for widget text key ignore: %w", err)
	}
	targets, err := collectWidgetWriteTargets(asset, -1)
	if err != nil {
		return nil, fmt.Errorf("collect widget-write targets for key ignore: %w", err)
	}
	target, err := selectWidgetWriteTarget(targets, widget)
	if err != nil {
		return nil, fmt.Errorf("resolve widget-write target for key ignore: %w", err)
	}

	ranges := make([]byteRange, 0, len(target.Exports)*2)
	reader := uasset.NewByteReaderWithByteSwapping(data, asset.Summary.UsesByteSwappedSerialization())
	for _, exportIdx := range target.Exports {
		parsed := asset.ParseExportProperties(exportIdx)
		found := false
		for _, prop := range parsed.Properties {
			if prop.Name.Display(asset.Names) != propertyName || prop.TypeString(asset.Names) != "TextProperty" {
				continue
			}
			if err := reader.Seek(prop.ValueOffset); err != nil {
				return nil, fmt.Errorf("seek text property for export %d: %w", exportIdx+1, err)
			}
			if _, err := reader.ReadInt32(); err != nil {
				return nil, fmt.Errorf("read text flags for export %d: %w", exportIdx+1, err)
			}
			if _, err := reader.ReadBytes(1); err != nil {
				return nil, fmt.Errorf("read text history type for export %d: %w", exportIdx+1, err)
			}
			namespaceStart := reader.Offset()
			if _, err := reader.ReadFString(); err != nil {
				return nil, fmt.Errorf("read text namespace for export %d: %w", exportIdx+1, err)
			}
			namespaceEnd := reader.Offset()
			keyStart := reader.Offset()
			if _, err := reader.ReadFString(); err != nil {
				return nil, fmt.Errorf("read text key for export %d: %w", exportIdx+1, err)
			}
			keyEnd := reader.Offset()
			ranges = append(ranges,
				byteRange{start: namespaceStart, end: namespaceEnd},
				byteRange{start: keyStart, end: keyEnd},
			)
			found = true
			break
		}
		if !found {
			return nil, fmt.Errorf("text property not found for widget export %d", exportIdx+1)
		}
	}
	return ranges, nil
}

func ignoredWidgetTextKeyOnlyRanges(data []byte, spec operationSpec) ([]byteRange, error) {
	if !strings.EqualFold(spec.Command, "blueprint widget-add") {
		return nil, nil
	}
	parent := strings.TrimSpace(fmt.Sprint(spec.Args["parent"]))
	if parent == "" || isWidgetAddRootSelector(parent) {
		return nil, nil
	}

	asset, err := uasset.ParseBytes(data, uasset.DefaultParseOptions())
	if err != nil {
		return nil, fmt.Errorf("parse asset for widget text key-only ignore: %w", err)
	}
	targets, err := collectWidgetWriteTargets(asset, -1)
	if err != nil {
		return nil, fmt.Errorf("collect widget targets for key-only ignore: %w", err)
	}
	parentTarget, err := selectWidgetWriteTarget(targets, parent)
	if err != nil {
		return nil, fmt.Errorf("resolve widget-add parent for key-only ignore: %w", err)
	}

	exportSet := make(map[int]struct{}, len(targets)*2)
	for _, target := range targets {
		if target.BlueprintExport != parentTarget.BlueprintExport {
			continue
		}
		for _, exportIdx := range target.Exports {
			exportSet[exportIdx] = struct{}{}
		}
	}
	if len(exportSet) == 0 {
		return nil, nil
	}

	exports := make([]int, 0, len(exportSet))
	for exportIdx := range exportSet {
		exports = append(exports, exportIdx)
	}
	sort.Ints(exports)

	reader := uasset.NewByteReaderWithByteSwapping(data, asset.Summary.UsesByteSwappedSerialization())
	ranges := make([]byteRange, 0, len(exports))
	for _, exportIdx := range exports {
		parsed := asset.ParseExportProperties(exportIdx)
		for _, prop := range parsed.Properties {
			if prop.Name.Display(asset.Names) != "Text" || prop.TypeString(asset.Names) != "TextProperty" {
				continue
			}
			if err := reader.Seek(prop.ValueOffset); err != nil {
				return nil, fmt.Errorf("seek text property for export %d: %w", exportIdx+1, err)
			}
			if _, err := reader.ReadInt32(); err != nil {
				return nil, fmt.Errorf("read text flags for export %d: %w", exportIdx+1, err)
			}
			if _, err := reader.ReadBytes(1); err != nil {
				return nil, fmt.Errorf("read text history type for export %d: %w", exportIdx+1, err)
			}
			if _, err := reader.ReadFString(); err != nil {
				return nil, fmt.Errorf("read text namespace for export %d: %w", exportIdx+1, err)
			}
			keyStart := reader.Offset()
			keyValue, err := reader.ReadFString()
			if err != nil {
				return nil, fmt.Errorf("read text key for export %d: %w", exportIdx+1, err)
			}
			if keyValue == "" || isDefaultWidgetTextLocalizationKey(keyValue) {
				break
			}
			ranges = append(ranges, byteRange{start: keyStart, end: reader.Offset()})
			break
		}
	}
	return ranges, nil
}

func ignoredWidgetVariableGuidMapValueRanges(data []byte, spec operationSpec) ([]byteRange, error) {
	if !strings.EqualFold(spec.Command, "blueprint widget-add") {
		return nil, nil
	}
	parent := strings.TrimSpace(fmt.Sprint(spec.Args["parent"]))
	childName := strings.TrimSpace(fmt.Sprint(spec.Args["name"]))
	if parent == "" || childName == "" {
		return nil, nil
	}

	exportFilter := 0
	if raw, ok := spec.Args["export"]; ok {
		switch v := raw.(type) {
		case float64:
			exportFilter = int(v)
		case string:
			if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				exportFilter = parsed
			}
		}
	}

	asset, err := uasset.ParseBytes(data, uasset.DefaultParseOptions())
	if err != nil {
		return nil, fmt.Errorf("parse asset for widget-add guid ignore: %w", err)
	}
	if isWidgetAddRootSelector(parent) {
		bpExports := findWidgetBlueprintExports(asset)
		if exportFilter > 0 {
			idx, err := asset.ResolveExportIndex(exportFilter)
			if err != nil {
				return nil, fmt.Errorf("resolve root widget-add blueprint export for guid ignore: %w", err)
			}
			bpExports = []int{idx}
		}
		if len(bpExports) != 1 {
			return nil, fmt.Errorf("root widget-add guid ignore requires exactly one WidgetBlueprint, got %d", len(bpExports))
		}
		bpIdx := bpExports[0]
		if bpIdx < 0 || bpIdx >= len(asset.Exports) {
			return nil, fmt.Errorf("root widget-add blueprint export out of range: %d", bpIdx+1)
		}
		_, err := findRootPropertyTagByName(asset, bpIdx, "WidgetVariableNameToGuidMap")
		if err != nil {
			return nil, err
		}
		return ignoredWidgetAddMapGuidRanges(data, asset, bpIdx, "WidgetVariableNameToGuidMap", childName)
	}
	targets, err := collectWidgetWriteTargets(asset, exportFilter)
	if err != nil {
		return nil, fmt.Errorf("collect widget-add targets for guid ignore: %w", err)
	}
	parentTarget, err := selectWidgetWriteTarget(targets, parent)
	if err != nil {
		return nil, fmt.Errorf("resolve widget-add parent for guid ignore: %w", err)
	}
	ctx, err := resolveWidgetAddContext(asset, *parentTarget)
	if err != nil {
		return nil, fmt.Errorf("resolve widget-add context for guid ignore: %w", err)
	}

	ranges := make([]byteRange, 0, 3)
	guidMapRanges, err := ignoredWidgetAddMapGuidRanges(data, asset, parentTarget.BlueprintExport, "WidgetVariableNameToGuidMap", childName)
	if err != nil {
		return nil, err
	}
	ranges = append(ranges, guidMapRanges...)

	generatedVarRanges, err := ignoredWidgetAddGeneratedVariableGuidRanges(data, asset, parentTarget.BlueprintExport, childName)
	if err != nil {
		lowerErr := strings.ToLower(err.Error())
		if !strings.Contains(lowerErr, "generatedvariables not found") && !strings.Contains(lowerErr, "guid entry not found in generatedvariables") {
			return nil, err
		}
		generatedVarRanges = nil
	}
	ranges = append(ranges, generatedVarRanges...)

	propertyGuidRanges, err := ignoredWidgetAddMapGuidRanges(data, asset, ctx.GeneratedClassExport, "PropertyGuids", childName)
	if err != nil {
		lowerErr := strings.ToLower(err.Error())
		if !strings.Contains(lowerErr, "propertyguids not found") && !strings.Contains(lowerErr, "guid entry not found in propertyguids") {
			return nil, err
		}
		propertyGuidRanges = nil
	}
	ranges = append(ranges, propertyGuidRanges...)

	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(spec.Args["type"])), "namedslot") {
		namedSlotRanges, err := ignoredWidgetAddNamedSlotGuidRanges(data, asset, targets, *parentTarget, ctx, childName)
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, namedSlotRanges...)
	}

	return ranges, nil
}

func ignoredWidgetAddNamedSlotGuidRanges(data []byte, asset *uasset.Asset, targets []widgetWriteTarget, parentTarget widgetWriteTarget, ctx widgetAddContext, childName string) ([]byteRange, error) {
	childTarget, err := selectWidgetWriteTarget(targets, parentTarget.Path+"/"+childName)
	if err != nil {
		return nil, fmt.Errorf("resolve NamedSlot child for guid ignore: %w", err)
	}

	ranges := make([]byteRange, 0, 3)
	for _, exportIndex := range childTarget.Exports {
		slotGuidRange, err := ignoredWidgetAddStructGuidPropertyRange(asset, exportIndex, "SlotGuid")
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, slotGuidRange)
	}

	namedSlotMapRanges, err := ignoredWidgetAddMapGuidRanges(data, asset, ctx.GeneratedClassExport, "NamedSlotsWithID", childName)
	if err != nil {
		return nil, err
	}
	ranges = append(ranges, namedSlotMapRanges...)
	return ranges, nil
}

func ignoredWidgetAddMapGuidRanges(data []byte, asset *uasset.Asset, exportIndex int, propertyName, childName string) ([]byteRange, error) {
	prop, err := findRootPropertyTagByName(asset, exportIndex, propertyName)
	if err != nil {
		return nil, err
	}

	reader := uasset.NewByteReaderWithByteSwapping(data, asset.Summary.UsesByteSwappedSerialization())
	if err := reader.Seek(prop.ValueOffset); err != nil {
		return nil, fmt.Errorf("seek %s for widget-add guid ignore: %w", propertyName, err)
	}
	removeCount, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read %s remove count: %w", propertyName, err)
	}
	switch {
	case removeCount == -1:
	case removeCount >= 0:
		for i := int32(0); i < removeCount; i++ {
			if _, err := reader.ReadNameRef(len(asset.Names)); err != nil {
				return nil, fmt.Errorf("read removed %s key %d: %w", propertyName, i, err)
			}
		}
	default:
		return nil, fmt.Errorf("invalid %s remove count: %d", propertyName, removeCount)
	}

	addCount, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read %s add count: %w", propertyName, err)
	}
	if addCount < 0 {
		return nil, fmt.Errorf("invalid %s add count: %d", propertyName, addCount)
	}

	ranges := make([]byteRange, 0, 1)
	for i := int32(0); i < addCount; i++ {
		keyRef, err := reader.ReadNameRef(len(asset.Names))
		if err != nil {
			return nil, fmt.Errorf("read %s key %d: %w", propertyName, i, err)
		}
		guidStart := reader.Offset()
		if _, err := reader.ReadGUID(); err != nil {
			return nil, fmt.Errorf("read %s guid %d: %w", propertyName, i, err)
		}
		guidEnd := reader.Offset()
		if strings.EqualFold(keyRef.Display(asset.Names), childName) {
			ranges = append(ranges, byteRange{start: guidStart, end: guidEnd})
		}
	}
	if len(ranges) == 0 {
		return nil, fmt.Errorf("child widget %q guid entry not found in %s", childName, propertyName)
	}
	return ranges, nil
}

func ignoredWidgetAddStructGuidPropertyRange(asset *uasset.Asset, exportIndex int, propertyName string) (byteRange, error) {
	prop, err := findRootPropertyTagByName(asset, exportIndex, propertyName)
	if err != nil {
		return byteRange{}, err
	}
	if prop.ValueOffset < 0 || prop.ValueOffset+16 > len(asset.Raw.Bytes) {
		return byteRange{}, fmt.Errorf("%s value out of bounds on export[%d]", propertyName, exportIndex+1)
	}
	return byteRange{start: prop.ValueOffset, end: prop.ValueOffset + 16}, nil
}

func ignoredWidgetAddGeneratedVariableGuidRanges(data []byte, asset *uasset.Asset, blueprintExport int, childName string) ([]byteRange, error) {
	prop, err := findRootPropertyTagByName(asset, blueprintExport, "GeneratedVariables")
	if err != nil {
		return nil, err
	}

	reader := uasset.NewByteReaderWithByteSwapping(data, asset.Summary.UsesByteSwappedSerialization())
	if err := reader.Seek(prop.ValueOffset); err != nil {
		return nil, fmt.Errorf("seek GeneratedVariables for widget-add guid ignore: %w", err)
	}
	count, err := reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("read GeneratedVariables count: %w", err)
	}
	if count < 0 {
		return nil, fmt.Errorf("invalid GeneratedVariables count: %d", count)
	}

	elementEnd := prop.ValueOffset + int(prop.Size)
	ranges := make([]byteRange, 0, 1)
	for i := int32(0); i < count; i++ {
		elementStart := reader.Offset()
		fields := asset.ParseTaggedPropertiesRange(elementStart, elementEnd, false)
		if len(fields.Warnings) > 0 {
			return nil, fmt.Errorf("parse GeneratedVariables[%d]: %s", i, strings.Join(fields.Warnings, "; "))
		}
		if fields.EndOffset <= elementStart {
			return nil, fmt.Errorf("GeneratedVariables[%d] has invalid bounds", i)
		}

		varName := ""
		var varGuid *uasset.PropertyTag
		for j := range fields.Properties {
			prop := &fields.Properties[j]
			switch prop.Name.Display(asset.Names) {
			case "VarName":
				if decoded, ok := asset.DecodePropertyValue(*prop); ok {
					if nameMap, ok := decoded.(map[string]any); ok {
						if raw, ok := nameMap["name"].(string); ok {
							varName = raw
						}
					}
				}
			case "VarGuid":
				varGuid = prop
			}
		}
		if strings.EqualFold(varName, childName) {
			if varGuid == nil {
				return nil, fmt.Errorf("GeneratedVariables entry for %q is missing VarGuid", childName)
			}
			ranges = append(ranges, byteRange{start: varGuid.ValueOffset, end: varGuid.ValueOffset + 16})
		}
		if err := reader.Seek(fields.EndOffset); err != nil {
			return nil, fmt.Errorf("seek next GeneratedVariables entry: %w", err)
		}
	}
	if len(ranges) == 0 {
		return nil, fmt.Errorf("child widget %q guid entry not found in GeneratedVariables", childName)
	}
	return ranges, nil
}

func findRootPropertyTagByName(asset *uasset.Asset, exportIndex int, propertyName string) (*uasset.PropertyTag, error) {
	parsed := asset.ParseExportProperties(exportIndex)
	for i := range parsed.Properties {
		prop := &parsed.Properties[i]
		if prop.Name.Display(asset.Names) == propertyName {
			return prop, nil
		}
	}
	return nil, fmt.Errorf("%s not found for widget-add guid ignore", propertyName)
}

func ignoredEditorThumbnailRanges(data []byte) ([]byteRange, error) {
	asset, err := uasset.ParseBytes(data, uasset.DefaultParseOptions())
	if err != nil {
		return nil, fmt.Errorf("parse asset for editor thumbnail ignore: %w", err)
	}

	start := int(asset.Summary.SearchableNamesOffset)
	end := int(asset.Summary.AssetRegistryDataOffset)
	if start <= 0 || end <= start {
		start = int(asset.Summary.ThumbnailTableOffset)
	}
	if start <= 0 || end <= start || end > len(data) {
		return nil, nil
	}

	ranges := []byteRange{{start: start, end: end}}
	if asset.Summary.AssetRegistryDataOffset > 0 {
		assetRegistryStart := int(asset.Summary.AssetRegistryDataOffset)
		if assetRegistryStart+8 <= len(data) {
			// The asset registry section starts with an absolute dependency offset.
			// UE save shifts this when editor-only thumbnail/layout bytes move.
			ranges = append(ranges, byteRange{start: assetRegistryStart, end: assetRegistryStart + 8})
		}
	}
	fields, err := scanSummaryComparableFields(data, asset.Summary.FileVersionUE5)
	if err != nil {
		return nil, fmt.Errorf("scan summary fields for editor thumbnail ignore: %w", err)
	}

	ignoreNames := map[string]struct{}{
		"TotalHeaderSize":         {},
		"ThumbnailTableOffset":    {},
		"AssetRegistryDataOffset": {},
		"BulkDataStartOffset":     {},
		"WorldTileInfoDataOffset": {},
		"PreloadDependencyOffset": {},
		"PayloadTOCOffset":        {},
		"DataResourceOffset":      {},
	}
	for _, field := range fields {
		if _, ok := ignoreNames[field.name]; !ok {
			continue
		}
		ranges = append(ranges, byteRange{start: field.pos, end: field.pos + field.size})
	}
	exportFields, err := scanComparableExportOffsetFields(data, asset)
	if err != nil {
		return nil, fmt.Errorf("scan export offset fields for editor thumbnail ignore: %w", err)
	}
	for _, field := range exportFields {
		ranges = append(ranges, byteRange{start: field.serialOffsetPos, end: field.serialOffsetPos + 8})
		if field.scriptStartPos >= 0 {
			ranges = append(ranges, byteRange{start: field.scriptStartPos, end: field.scriptStartPos + 8})
		}
		if field.scriptEndPos >= 0 {
			ranges = append(ranges, byteRange{start: field.scriptEndPos, end: field.scriptEndPos + 8})
		}
	}
	return ranges, nil
}

func ignoredSavedHashRanges(data []byte) ([]byteRange, error) {
	asset, err := uasset.ParseBytes(data, uasset.DefaultParseOptions())
	if err != nil {
		return nil, fmt.Errorf("parse asset for saved hash ignore: %w", err)
	}
	if asset.Summary.FileVersionUE5 < 1016 {
		return nil, nil
	}

	fields, err := scanSummaryComparableFields(data, asset.Summary.FileVersionUE5)
	if err != nil {
		return nil, fmt.Errorf("scan summary fields for saved hash ignore: %w", err)
	}
	for _, field := range fields {
		if field.name != "TotalHeaderSize" {
			continue
		}
		if field.pos < 20 {
			return nil, fmt.Errorf("saved hash range out of bounds: pos=%d", field.pos)
		}
		return []byteRange{{start: field.pos - 20, end: field.pos}}, nil
	}
	return nil, fmt.Errorf("saved hash range not found")
}

func TestIgnoredBlueprintSearchTailRanges(t *testing.T) {
	path := findExistingGoldenOperationFixturePath(t, "widget_add_image_border_canvaspanel", "after.uasset")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	asset, err := uasset.ParseBytes(data, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	ranges, err := ignoredBlueprintSearchTailRanges(data)
	if err != nil {
		t.Fatalf("ignoredBlueprintSearchTailRanges: %v", err)
	}
	if len(ranges) != 1 {
		t.Fatalf("range count: got %d want 1", len(ranges))
	}

	want := byteRange{
		start: int(asset.Summary.PreloadDependencyOffset),
		end:   int(asset.Summary.BulkDataStartOffset),
	}
	if ranges[0] != want {
		t.Fatalf("range: got %+v want %+v", ranges[0], want)
	}
}

func ignoredBlueprintSearchTailRanges(data []byte) ([]byteRange, error) {
	asset, err := uasset.ParseBytes(data, uasset.DefaultParseOptions())
	if err != nil {
		return nil, fmt.Errorf("parse asset for blueprint search tail ignore: %w", err)
	}
	start := int(asset.Summary.PreloadDependencyOffset)
	end := int(asset.Summary.BulkDataStartOffset)
	if start <= 0 || end <= start || end > len(data) {
		return nil, nil
	}
	return []byteRange{{start: start, end: end}}, nil
}

func ignoredSoftPackageReferenceRanges(data []byte) ([]byteRange, error) {
	asset, err := uasset.ParseBytes(data, uasset.DefaultParseOptions())
	if err != nil {
		return nil, fmt.Errorf("parse asset for soft package reference ignore: %w", err)
	}
	start := int(asset.Summary.SoftPackageReferencesOffset)
	end := int(asset.Summary.SearchableNamesOffset)
	if start <= 0 || end <= start || end > len(data) {
		return nil, nil
	}
	return []byteRange{{start: start, end: end}}, nil
}

func ignoredSoftPackageSearchableSummaryOffsetRanges(data []byte, spec operationSpec) ([]byteRange, error) {
	asset, err := uasset.ParseBytes(data, uasset.DefaultParseOptions())
	if err != nil {
		return nil, fmt.Errorf("parse asset for soft package summary offset ignore: %w", err)
	}
	fields, err := scanSummaryComparableFields(data, asset.Summary.FileVersionUE5)
	if err != nil {
		return nil, fmt.Errorf("scan summary fields for soft package summary offset ignore: %w", err)
	}
	ignoreNames := map[string]struct{}{
		"SoftPackageReferencesOffset": {},
		"SearchableNamesOffset":       {},
	}
	ranges := make([]byteRange, 0, 2)
	for _, field := range fields {
		if _, ok := ignoreNames[field.name]; !ok {
			continue
		}
		ranges = append(ranges, byteRange{start: field.pos, end: field.pos + field.size})
	}
	return ranges, nil
}

type comparableExportOffsetField struct {
	serialOffsetPos int
	scriptStartPos  int
	scriptEndPos    int
}

type comparableSummaryField struct {
	name  string
	pos   int
	size  int
	value int64
}

func scanSummaryComparableFields(data []byte, unversionedFileUE5 int32) ([]comparableSummaryField, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("file too small")
	}
	tagLE := binary.LittleEndian.Uint32(data[:4])
	var order binary.ByteOrder = binary.LittleEndian
	switch tagLE {
	case 0x9E2A83C1:
		order = binary.LittleEndian
	case 0xC1832A9E:
		order = binary.BigEndian
	default:
		return nil, fmt.Errorf("invalid package tag: 0x%x", tagLE)
	}

	r := comparableByteCodec{data: data, order: order}
	fields := make([]comparableSummaryField, 0, 24)
	record32 := func(name string) (int32, error) {
		pos := r.off
		v, err := r.readInt32()
		if err != nil {
			return 0, err
		}
		fields = append(fields, comparableSummaryField{name: name, pos: pos, size: 4, value: int64(v)})
		return v, nil
	}
	record64 := func(name string) (int64, error) {
		pos := r.off
		v, err := r.readInt64()
		if err != nil {
			return 0, err
		}
		fields = append(fields, comparableSummaryField{name: name, pos: pos, size: 8, value: v})
		return v, nil
	}

	if _, err := r.readInt32(); err != nil {
		return nil, err
	}
	legacy, err := r.readInt32()
	if err != nil {
		return nil, err
	}
	if legacy != -4 {
		if _, err := r.readInt32(); err != nil {
			return nil, err
		}
	}
	fileUE4, err := r.readInt32()
	if err != nil {
		return nil, err
	}
	fileUE5, err := r.readInt32()
	if err != nil {
		return nil, err
	}
	fileLicensee, err := r.readInt32()
	if err != nil {
		return nil, err
	}
	if fileUE4 == 0 && fileUE5 == 0 && fileLicensee == 0 {
		fileUE4 = 522
		if unversionedFileUE5 >= 1000 {
			fileUE5 = unversionedFileUE5
		} else {
			fileUE5 = 1018
		}
	}
	if fileUE5 >= 1016 {
		if err := r.skip(20); err != nil {
			return nil, err
		}
		if _, err := record32("TotalHeaderSize"); err != nil {
			return nil, err
		}
	}
	if legacy <= -2 {
		if err := skipComparableSummaryCustomVersions(&r, legacy); err != nil {
			return nil, err
		}
	}
	if fileUE5 < 1016 {
		if _, err := record32("TotalHeaderSize"); err != nil {
			return nil, err
		}
	}
	if _, err := r.readFString(); err != nil {
		return nil, err
	}
	packageFlags, err := r.readUint32()
	if err != nil {
		return nil, err
	}
	if _, err := r.readInt32(); err != nil {
		return nil, err
	}
	if _, err := record32("NameOffset"); err != nil {
		return nil, err
	}
	if fileUE5 >= 1008 {
		if _, err := r.readInt32(); err != nil {
			return nil, err
		}
		if _, err := record32("SoftObjectPathsOffset"); err != nil {
			return nil, err
		}
	}
	if packageFlags&0x80000000 == 0 {
		if _, err := r.readFString(); err != nil {
			return nil, err
		}
	}
	if _, err := r.readInt32(); err != nil {
		return nil, err
	}
	if _, err := record32("GatherableTextDataOffset"); err != nil {
		return nil, err
	}
	if _, err := r.readInt32(); err != nil {
		return nil, err
	}
	if _, err := record32("ExportOffset"); err != nil {
		return nil, err
	}
	if _, err := r.readInt32(); err != nil {
		return nil, err
	}
	if _, err := record32("ImportOffset"); err != nil {
		return nil, err
	}
	if fileUE5 >= 1015 {
		if _, err := r.readInt32(); err != nil {
			return nil, err
		}
		if _, err := record32("CellExportOffset"); err != nil {
			return nil, err
		}
		if _, err := r.readInt32(); err != nil {
			return nil, err
		}
		if _, err := record32("CellImportOffset"); err != nil {
			return nil, err
		}
	}
	if fileUE5 >= 1014 {
		if _, err := record32("MetaDataOffset"); err != nil {
			return nil, err
		}
	}
	if _, err := record32("DependsOffset"); err != nil {
		return nil, err
	}
	if _, err := r.readInt32(); err != nil {
		return nil, err
	}
	if _, err := record32("SoftPackageReferencesOffset"); err != nil {
		return nil, err
	}
	if _, err := record32("SearchableNamesOffset"); err != nil {
		return nil, err
	}
	if _, err := record32("ThumbnailTableOffset"); err != nil {
		return nil, err
	}
	if fileUE5 >= 1018 {
		if _, err := r.readInt32(); err != nil {
			return nil, err
		}
		if _, err := record32("ImportTypeHierarchiesOffset"); err != nil {
			return nil, err
		}
	}
	if fileUE5 < 1016 {
		if err := r.skip(16); err != nil {
			return nil, err
		}
	}
	if packageFlags&0x80000000 == 0 {
		if err := r.skip(16); err != nil {
			return nil, err
		}
	}
	generationCount, err := r.readInt32()
	if err != nil {
		return nil, err
	}
	if generationCount < 0 {
		return nil, fmt.Errorf("invalid generation count: %d", generationCount)
	}
	for i := int32(0); i < generationCount; i++ {
		if err := r.skip(8); err != nil {
			return nil, err
		}
	}
	if err := skipComparableEngineVersion(&r); err != nil {
		return nil, err
	}
	if err := skipComparableEngineVersion(&r); err != nil {
		return nil, err
	}
	if _, err := r.readUint32(); err != nil {
		return nil, err
	}
	chunkCount, err := r.readInt32()
	if err != nil {
		return nil, err
	}
	if chunkCount < 0 {
		return nil, fmt.Errorf("invalid compressed chunk count: %d", chunkCount)
	}
	if err := r.skip(int(chunkCount) * 16); err != nil {
		return nil, err
	}
	if _, err := r.readUint32(); err != nil {
		return nil, err
	}
	additionalCount, err := r.readInt32()
	if err != nil {
		return nil, err
	}
	if additionalCount < 0 {
		return nil, fmt.Errorf("invalid additional packages count: %d", additionalCount)
	}
	for i := int32(0); i < additionalCount; i++ {
		if _, err := r.readFString(); err != nil {
			return nil, err
		}
	}
	if legacy > -7 {
		if _, err := r.readInt32(); err != nil {
			return nil, err
		}
	}
	if _, err := record32("AssetRegistryDataOffset"); err != nil {
		return nil, err
	}
	if _, err := record64("BulkDataStartOffset"); err != nil {
		return nil, err
	}
	if _, err := record32("WorldTileInfoDataOffset"); err != nil {
		return nil, err
	}
	chunkIDCount, err := r.readInt32()
	if err != nil {
		return nil, err
	}
	if chunkIDCount < 0 {
		return nil, fmt.Errorf("invalid chunk ID count: %d", chunkIDCount)
	}
	if err := r.skip(int(chunkIDCount) * 4); err != nil {
		return nil, err
	}
	if _, err := r.readInt32(); err != nil {
		return nil, err
	}
	if _, err := record32("PreloadDependencyOffset"); err != nil {
		return nil, err
	}
	if fileUE5 >= 1001 {
		if _, err := r.readInt32(); err != nil {
			return nil, err
		}
	}
	if fileUE5 >= 1002 {
		if _, err := record64("PayloadTOCOffset"); err != nil {
			return nil, err
		}
	}
	if fileUE5 >= 1009 {
		if _, err := record32("DataResourceOffset"); err != nil {
			return nil, err
		}
	}

	return fields, nil
}

type comparableByteCodec struct {
	data  []byte
	off   int
	order binary.ByteOrder
}

func (c *comparableByteCodec) remaining() int {
	return len(c.data) - c.off
}

func (c *comparableByteCodec) skip(n int) error {
	if n < 0 {
		return fmt.Errorf("negative skip: %d", n)
	}
	if c.remaining() < n {
		return errors.New("unexpected EOF")
	}
	c.off += n
	return nil
}

func (c *comparableByteCodec) readBytes(n int) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("negative read: %d", n)
	}
	if c.remaining() < n {
		return nil, errors.New("unexpected EOF")
	}
	start := c.off
	c.off += n
	return c.data[start:c.off], nil
}

func (c *comparableByteCodec) readInt32() (int32, error) {
	b, err := c.readBytes(4)
	if err != nil {
		return 0, err
	}
	return int32(c.order.Uint32(b)), nil
}

func (c *comparableByteCodec) readUint32() (uint32, error) {
	b, err := c.readBytes(4)
	if err != nil {
		return 0, err
	}
	return c.order.Uint32(b), nil
}

func (c *comparableByteCodec) readInt64() (int64, error) {
	b, err := c.readBytes(8)
	if err != nil {
		return 0, err
	}
	return int64(c.order.Uint64(b)), nil
}

func (c *comparableByteCodec) readFString() (string, error) {
	lenField, err := c.readInt32()
	if err != nil {
		return "", err
	}
	if lenField == 0 {
		return "", nil
	}
	if lenField > 0 {
		buf, err := c.readBytes(int(lenField))
		if err != nil {
			return "", err
		}
		if len(buf) > 0 && buf[len(buf)-1] == 0 {
			buf = buf[:len(buf)-1]
		}
		return string(buf), nil
	}
	if lenField == math.MinInt32 {
		return "", fmt.Errorf("invalid wide string length: %d", lenField)
	}
	wideCount := int(-lenField)
	buf, err := c.readBytes(wideCount * 2)
	if err != nil {
		return "", err
	}
	units := make([]uint16, 0, wideCount)
	for i := 0; i+1 < len(buf); i += 2 {
		units = append(units, c.order.Uint16(buf[i:i+2]))
	}
	if len(units) > 0 && units[len(units)-1] == 0 {
		units = units[:len(units)-1]
	}
	return string(utf16.Decode(units)), nil
}

func skipComparableEngineVersion(r *comparableByteCodec) error {
	if err := r.skip(2); err != nil {
		return err
	}
	if err := r.skip(2); err != nil {
		return err
	}
	if err := r.skip(2); err != nil {
		return err
	}
	if err := r.skip(4); err != nil {
		return err
	}
	_, err := r.readFString()
	return err
}

func skipComparableSummaryCustomVersions(r *comparableByteCodec, legacy int32) error {
	count, err := r.readInt32()
	if err != nil {
		return err
	}
	if count < 0 {
		return fmt.Errorf("invalid custom version count: %d", count)
	}
	switch {
	case legacy == -2:
		for i := int32(0); i < count; i++ {
			if err := r.skip(8); err != nil {
				return err
			}
		}
	case legacy >= -5:
		for i := int32(0); i < count; i++ {
			if err := r.skip(16); err != nil {
				return err
			}
			if _, err := r.readInt32(); err != nil {
				return err
			}
			if _, err := r.readFString(); err != nil {
				return err
			}
		}
	default:
		for i := int32(0); i < count; i++ {
			if err := r.skip(16); err != nil {
				return err
			}
			if _, err := r.readInt32(); err != nil {
				return err
			}
		}
	}
	return nil
}

func scanComparableExportOffsetFields(data []byte, asset *uasset.Asset) ([]comparableExportOffsetField, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset is nil")
	}
	if asset.Summary.ExportOffset < 0 || int(asset.Summary.ExportOffset) > len(data) {
		return nil, fmt.Errorf("export offset out of range: %d", asset.Summary.ExportOffset)
	}
	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}
	r := comparableByteCodec{data: data, order: order}
	r.off = int(asset.Summary.ExportOffset)

	fields := make([]comparableExportOffsetField, 0, len(asset.Exports))
	for i := 0; i < len(asset.Exports); i++ {
		if err := r.skip(4 * 4); err != nil {
			return nil, fmt.Errorf("export[%d] read class/super/template/outer: %w", i+1, err)
		}
		if err := r.skip(8); err != nil {
			return nil, fmt.Errorf("export[%d] read object name: %w", i+1, err)
		}
		if err := r.skip(4); err != nil {
			return nil, fmt.Errorf("export[%d] read object flags: %w", i+1, err)
		}
		if _, err := r.readInt64(); err != nil {
			return nil, fmt.Errorf("export[%d] read serial size: %w", i+1, err)
		}
		serialOffsetPos := r.off
		if _, err := r.readInt64(); err != nil {
			return nil, fmt.Errorf("export[%d] read serial offset: %w", i+1, err)
		}
		if err := r.skip(4 * 3); err != nil {
			return nil, fmt.Errorf("export[%d] read bool flags: %w", i+1, err)
		}
		if asset.Summary.FileVersionUE5 < 1005 {
			if err := r.skip(16); err != nil {
				return nil, fmt.Errorf("export[%d] read package guid: %w", i+1, err)
			}
		}
		if asset.Summary.FileVersionUE5 >= 1006 {
			if err := r.skip(4); err != nil {
				return nil, fmt.Errorf("export[%d] read inherited flag: %w", i+1, err)
			}
		}
		if err := r.skip(4); err != nil {
			return nil, fmt.Errorf("export[%d] read package flags: %w", i+1, err)
		}
		if err := r.skip(4 * 2); err != nil {
			return nil, fmt.Errorf("export[%d] read load flags: %w", i+1, err)
		}
		if asset.Summary.FileVersionUE5 >= 1003 {
			if err := r.skip(4); err != nil {
				return nil, fmt.Errorf("export[%d] read public hash flag: %w", i+1, err)
			}
		}
		if err := r.skip(4 * 5); err != nil {
			return nil, fmt.Errorf("export[%d] read dependency header: %w", i+1, err)
		}
		field := comparableExportOffsetField{
			serialOffsetPos: serialOffsetPos,
			scriptStartPos:  -1,
			scriptEndPos:    -1,
		}
		if !asset.Summary.UsesUnversionedPropertySerialization() && asset.Summary.FileVersionUE5 >= 1010 {
			field.scriptStartPos = r.off
			if _, err := r.readInt64(); err != nil {
				return nil, fmt.Errorf("export[%d] read script start: %w", i+1, err)
			}
			field.scriptEndPos = r.off
			if _, err := r.readInt64(); err != nil {
				return nil, fmt.Errorf("export[%d] read script end: %w", i+1, err)
			}
		}
		fields = append(fields, field)
	}
	return fields, nil
}
