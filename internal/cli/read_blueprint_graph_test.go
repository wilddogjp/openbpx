package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

func TestParseBlueprintShowFields(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []string
		wantErr bool
	}{
		{name: "defaults", raw: "NodePos,Function,PinDefaults", want: []string{"NodePos", "Function", "PinDefaults"}},
		{name: "dedupe", raw: "Pins,pins,Warnings", want: []string{"Pins", "Warnings"}},
		{name: "invalid", raw: "NodePos,BadField", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseBlueprintShowFields(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseBlueprintShowFields error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("field count: got %d want %d", len(got), len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("field[%d]: got %q want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestSplitNodePinToken(t *testing.T) {
	node, pin := splitNodePinToken("K2Node_Event_0.Then")
	if node != "K2Node_Event_0" || pin != "Then" {
		t.Fatalf("split with pin: got (%q,%q)", node, pin)
	}
	node, pin = splitNodePinToken("K2Node_Event_0")
	if node != "K2Node_Event_0" || pin != "" {
		t.Fatalf("split node only: got (%q,%q)", node, pin)
	}
}

func TestBuildBlueprintGraphIndexParsesPinsFromFixture(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "golden", "parse", "BP_Empty.uasset")
	asset, err := uasset.ParseFile(fixture, uasset.ParseOptions{})
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	graph := buildBlueprintGraphIndex(asset)
	if len(graph.Nodes) == 0 {
		t.Fatalf("expected graph nodes")
	}
	pinCount := 0
	for _, node := range graph.Nodes {
		pinCount += len(node.Pins)
	}
	if pinCount == 0 {
		t.Fatalf("expected parsed pins")
	}
}

func TestRunBlueprintSearchRejectsUnsupportedShowField(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "golden", "parse", "BP_Empty.uasset")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"blueprint", "search", fixture, "--show", "NodePos,BadField"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if !containsFold(stderr.String(), "unsupported --show field") {
		t.Fatalf("expected unsupported show field error, got: %s", stderr.String())
	}
}

func TestRunBlueprintCallArgsNoMatchOnFixture(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "golden", "parse", "BP_Empty.uasset")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"blueprint", "call-args", fixture, "--member", "OpenLevelBySoftObjectPtr"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if got, ok := payload["matchCount"].(float64); !ok || got != 0 {
		t.Fatalf("matchCount: got %#v", payload["matchCount"])
	}
}
