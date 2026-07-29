package uasset

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strconv"
)

// Decode status values attached to decoded property values.
const (
	// DecodeStatusUndecoded marks a payload that was preserved as raw bytes because
	// no decoder could interpret it.
	DecodeStatusUndecoded = "undecoded"
	// DecodeStatusPartial marks a payload that decoded but left bytes unconsumed.
	DecodeStatusPartial = "partial"
)

const (
	decodeStatusKey    = "decodeStatus"
	undecodedReasonKey = "undecodedReason"
	undecodedBytesKey  = "undecodedBytes"

	genericUndecodedReason = "no decoder for this payload; raw bytes preserved"
)

// UndecodedNote describes one payload that BPX could not fully decode.
type UndecodedNote struct {
	// Path is the location inside the decoded value tree, e.g. "Layers.value[0]".
	Path string `json:"path"`
	// Status is DecodeStatusUndecoded or DecodeStatusPartial.
	Status string `json:"status"`
	// Type is the declared type of the payload when known.
	Type string `json:"type,omitempty"`
	// Reason states why the payload was not decoded.
	Reason string `json:"reason"`
	// Bytes is the size of the preserved raw payload, or the unconsumed tail for
	// DecodeStatusPartial.
	Bytes int `json:"bytes,omitempty"`
}

// AnnotateUndecoded walks a decoded value tree and marks every raw-byte fallback
// and every partially consumed payload in place. Decoded values are untouched.
//
// Silence is the failure mode this guards against: without the marker a caller
// cannot tell a decoded struct from an opaque blob without inspecting keys.
func AnnotateUndecoded(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		annotateUndecodedMap(typed)
		for _, child := range typed {
			AnnotateUndecoded(child)
		}
	case []any:
		for _, child := range typed {
			AnnotateUndecoded(child)
		}
	case []map[string]any:
		for _, child := range typed {
			AnnotateUndecoded(child)
		}
	}
	return value
}

func annotateUndecodedMap(node map[string]any) {
	if _, already := node[decodeStatusKey]; already {
		return
	}

	if raw, ok := node["rawBase64"].(string); ok && raw != "" {
		if _, decoded := node["value"]; !decoded {
			node[decodeStatusKey] = DecodeStatusUndecoded
			node[undecodedReasonKey] = undecodedReasonFor(node)
			if size, err := base64.StdEncoding.DecodeString(raw); err == nil {
				node[undecodedBytesKey] = len(size)
			}
			return
		}
	}

	if trailing, ok := asInt(node["trailingBytes"]); ok && trailing > 0 {
		node[decodeStatusKey] = DecodeStatusPartial
		node[undecodedReasonKey] = fmt.Sprintf(
			"decoder left %d trailing byte(s) unconsumed; decoded fields may be incomplete",
			trailing,
		)
		node[undecodedBytesKey] = trailing
	}
}

func undecodedReasonFor(node map[string]any) string {
	if structType, ok := node["structType"].(string); ok && structType != "" {
		return fmt.Sprintf(
			"struct %s has no built-in decoder and its payload is not tagged-property data (likely a native Serialize()); raw bytes preserved",
			structType,
		)
	}
	if arrayType, ok := node["arrayType"].(string); ok && arrayType != "" {
		return fmt.Sprintf(
			"array of %s: at least one element could not be decoded; raw bytes preserved",
			arrayType,
		)
	}
	if setType, ok := node["setType"].(string); ok && setType != "" {
		return fmt.Sprintf(
			"set of %s: at least one element could not be decoded; raw bytes preserved",
			setType,
		)
	}
	if keyType, ok := node["keyType"].(string); ok && keyType != "" {
		valueType, _ := node["valueType"].(string)
		return fmt.Sprintf(
			"map<%s,%s>: at least one entry could not be decoded; raw bytes preserved",
			keyType,
			valueType,
		)
	}
	if optionalType, ok := node["optionalType"].(string); ok && optionalType != "" {
		return fmt.Sprintf(
			"optional %s: inner value could not be decoded; raw bytes preserved",
			optionalType,
		)
	}
	return genericUndecodedReason
}

// CollectUndecoded walks an annotated value tree and returns every undecoded or
// partially decoded payload found, sorted by path.
func CollectUndecoded(value any, basePath string) []UndecodedNote {
	notes := make([]UndecodedNote, 0)
	collectUndecoded(value, basePath, &notes)
	sort.Slice(notes, func(i, j int) bool { return notes[i].Path < notes[j].Path })
	return notes
}

func collectUndecoded(value any, path string, notes *[]UndecodedNote) {
	switch typed := value.(type) {
	case map[string]any:
		if status, ok := typed[decodeStatusKey].(string); ok && status != "" {
			reason, _ := typed[undecodedReasonKey].(string)
			bytes, _ := asInt(typed[undecodedBytesKey])
			*notes = append(*notes, UndecodedNote{
				Path:   path,
				Status: status,
				Type:   undecodedTypeOf(typed),
				Reason: reason,
				Bytes:  bytes,
			})
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			collectUndecoded(typed[key], joinDecodePath(path, key), notes)
		}
	case []any:
		for i, child := range typed {
			collectUndecoded(child, path+"["+strconv.Itoa(i)+"]", notes)
		}
	case []map[string]any:
		for i, child := range typed {
			collectUndecoded(child, path+"["+strconv.Itoa(i)+"]", notes)
		}
	}
}

func undecodedTypeOf(node map[string]any) string {
	for _, key := range []string{"structType", "arrayType", "setType", "optionalType", "type"} {
		if v, ok := node[key].(string); ok && v != "" {
			return v
		}
	}
	if keyType, ok := node["keyType"].(string); ok && keyType != "" {
		valueType, _ := node["valueType"].(string)
		return "map<" + keyType + "," + valueType + ">"
	}
	return ""
}

func joinDecodePath(base, key string) string {
	if base == "" {
		return key
	}
	return base + "." + key
}

func asInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	default:
		return 0, false
	}
}
