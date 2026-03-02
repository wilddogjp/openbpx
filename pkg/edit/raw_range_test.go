package edit

import (
	"bytes"
	"strings"
	"testing"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

func TestRewriteRawRangeUpdatesOffsets(t *testing.T) {
	data := buildEditFixture(t, "hello")
	asset, err := uasset.ParseBytes(data, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	start := int64(asset.Summary.ExportOffset)
	replacement := []byte{0xAA, 0xBB, 0xCC}
	out, err := RewriteRawRange(asset, start, start, replacement)
	if err != nil {
		t.Fatalf("rewrite raw range: %v", err)
	}

	if got := out[start : start+int64(len(replacement))]; !bytes.Equal(got, replacement) {
		t.Fatalf("inserted bytes mismatch: got=%v want=%v", got, replacement)
	}

	rewritten, err := uasset.ParseBytes(out, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten fixture: %v", err)
	}

	delta := int32(len(replacement))
	if got, want := rewritten.Summary.ExportOffset, asset.Summary.ExportOffset+delta; got != want {
		t.Fatalf("summary export offset: got %d want %d", got, want)
	}
	for i := range asset.Exports {
		if got, want := rewritten.Exports[i].SerialOffset, asset.Exports[i].SerialOffset+int64(delta); got != want {
			t.Fatalf("export[%d] serial offset: got %d want %d", i+1, got, want)
		}
	}
}

func TestRewriteRawRangeRejectsExportPayloadOverlap(t *testing.T) {
	data := buildEditFixture(t, "hello")
	asset, err := uasset.ParseBytes(data, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	start := asset.Exports[0].SerialOffset + 1
	end := start + 2
	_, err = RewriteRawRange(asset, start, end, []byte{0x01})
	if err == nil {
		t.Fatalf("expected overlap error")
	}
	if !strings.Contains(err.Error(), "overlaps export") {
		t.Fatalf("unexpected error: %v", err)
	}
}
