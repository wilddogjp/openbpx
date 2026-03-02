package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/wilddogjp/bpx/pkg/edit"
	"github.com/wilddogjp/bpx/pkg/uasset"
)

func applyPropertyMutation(asset *uasset.Asset, exportIndex int, path string, valueJSON string) ([]byte, *edit.PropertySetResult, error) {
	result, err := edit.BuildPropertySetMutation(asset, exportIndex, path, valueJSON)
	if err != nil {
		return nil, nil, err
	}
	outBytes, err := edit.RewriteAsset(asset, []edit.ExportMutation{result.Mutation})
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite asset: %w", err)
	}
	return outBytes, result, nil
}

func applyFirstPropertyMutation(asset *uasset.Asset, exportIndex int, paths []string, valueJSON string) ([]byte, *edit.PropertySetResult, string, error) {
	seen := map[string]struct{}{}
	errs := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		outBytes, result, err := applyPropertyMutation(asset, exportIndex, path, valueJSON)
		if err == nil {
			return outBytes, result, path, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", path, err))
	}
	if len(errs) == 0 {
		return nil, nil, "", fmt.Errorf("no candidate path")
	}
	return nil, nil, "", fmt.Errorf("no editable path matched (%s)", strings.Join(errs, "; "))
}

func detectStringTableEntryPaths(asset *uasset.Asset, exportIndex int, key string) []string {
	parsed := asset.ParseExportProperties(exportIndex)
	candidates := make([]string, 0, len(parsed.Properties))
	preferred := make([]string, 0, 4)
	for _, p := range parsed.Properties {
		if len(p.TypeNodes) < 3 {
			continue
		}
		if p.TypeNodes[0].Name.Display(asset.Names) != "MapProperty" {
			continue
		}
		keyType := p.TypeNodes[1].Name.Display(asset.Names)
		valueType := p.TypeNodes[2].Name.Display(asset.Names)
		if (keyType != "StrProperty" && keyType != "NameProperty") || valueType != "StrProperty" {
			continue
		}
		propName := p.Name.Display(asset.Names)
		path := fmt.Sprintf("%s[%s]", propName, strconv.Quote(key))
		lower := strings.ToLower(propName)
		if strings.Contains(lower, "entry") || strings.Contains(lower, "source") || strings.Contains(lower, "string") {
			preferred = append(preferred, path)
			continue
		}
		candidates = append(candidates, path)
	}
	return append(preferred, candidates...)
}

func parseJSONMap(raw string) (map[string]any, error) {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var out map[string]any
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, fmt.Errorf("trailing tokens are not allowed")
	}
	return out, nil
}

func marshalJSONValue(v any) (string, error) {
	buf, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func enumValueLiteralToJSON(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return `""`
	}
	if _, err := strconv.ParseInt(trimmed, 0, 64); err == nil {
		return trimmed
	}
	if _, err := strconv.ParseUint(trimmed, 0, 64); err == nil {
		return trimmed
	}
	return strconv.Quote(value)
}
