package uasset

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestAnnotateUndecodedMarksRawStructPayload(t *testing.T) {
	raw := base64.StdEncoding.EncodeToString([]byte{1, 2, 3, 4, 5})
	value := map[string]any{
		"structType": "ScalarMaterialInput",
		"rawBase64":  raw,
	}

	AnnotateUndecoded(value)

	if got := value[decodeStatusKey]; got != DecodeStatusUndecoded {
		t.Fatalf("decodeStatus: got %v want %v", got, DecodeStatusUndecoded)
	}
	reason, _ := value[undecodedReasonKey].(string)
	if !strings.Contains(reason, "ScalarMaterialInput") {
		t.Fatalf("reason does not name the struct: %q", reason)
	}
	if got := value[undecodedBytesKey]; got != 5 {
		t.Fatalf("undecodedBytes: got %v want 5", got)
	}
}

func TestAnnotateUndecodedLeavesDecodedValuesAlone(t *testing.T) {
	value := map[string]any{
		"structType": "Vector",
		"value":      map[string]any{"x": 1.0, "y": 2.0, "z": 3.0},
	}

	AnnotateUndecoded(value)

	if _, marked := value[decodeStatusKey]; marked {
		t.Fatalf("decoded value must not be marked undecoded: %v", value)
	}
}

func TestAnnotateUndecodedMarksPartialDecode(t *testing.T) {
	value := map[string]any{
		"arrayType":     "StructProperty",
		"value":         []any{},
		"trailingBytes": 12,
	}

	AnnotateUndecoded(value)

	if got := value[decodeStatusKey]; got != DecodeStatusPartial {
		t.Fatalf("decodeStatus: got %v want %v", got, DecodeStatusPartial)
	}
	if got := value[undecodedBytesKey]; got != 12 {
		t.Fatalf("undecodedBytes: got %v want 12", got)
	}
}

func TestCollectUndecodedReportsNestedPaths(t *testing.T) {
	raw := base64.StdEncoding.EncodeToString([]byte{9, 9})
	value := map[string]any{
		"arrayType": "StructProperty",
		"value": []any{
			map[string]any{
				"value": map[string]any{
					"structType": "ExpressionInput",
					"rawBase64":  raw,
				},
			},
		},
	}
	AnnotateUndecoded(value)

	notes := CollectUndecoded(value, "BaseColor")
	if len(notes) != 1 {
		t.Fatalf("expected one note, got %d (%v)", len(notes), notes)
	}
	note := notes[0]
	if note.Status != DecodeStatusUndecoded {
		t.Fatalf("status: got %q", note.Status)
	}
	if note.Type != "ExpressionInput" {
		t.Fatalf("type: got %q", note.Type)
	}
	if !strings.HasPrefix(note.Path, "BaseColor") {
		t.Fatalf("path must start at the property name: %q", note.Path)
	}
	if note.Bytes != 2 {
		t.Fatalf("bytes: got %d want 2", note.Bytes)
	}
}

func TestCollectUndecodedReturnsNothingWhenFullyDecoded(t *testing.T) {
	value := map[string]any{"structType": "Vector", "value": map[string]any{"x": 1.0}}
	AnnotateUndecoded(value)

	if notes := CollectUndecoded(value, "Location"); len(notes) != 0 {
		t.Fatalf("expected no notes, got %v", notes)
	}
}
