package edit

import (
	"encoding/binary"
	"path/filepath"
	"testing"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

func TestRewriteNameMapAppendEntry(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "golden", "parse", "BP_Empty.uasset")
	asset, err := uasset.ParseFile(fixturePath, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	var order binary.ByteOrder = binary.LittleEndian
	if asset.Summary.UsesByteSwappedSerialization() {
		order = binary.BigEndian
	}
	originalNameMapSize := len(encodeNameMap(asset.Names, order))

	nonCaseHash, caseHash := ComputeNameEntryHashesUE56("BPX_Test_Name")
	updatedNames := append(append([]uasset.NameEntry{}, asset.Names...), uasset.NameEntry{
		Value:              "BPX_Test_Name",
		NonCaseHash:        nonCaseHash,
		CasePreservingHash: caseHash,
	})

	outBytes, err := RewriteNameMap(asset, updatedNames)
	if err != nil {
		t.Fatalf("rewrite name map: %v", err)
	}

	reparsed, err := uasset.ParseBytes(outBytes, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		t.Fatalf("reparse rewritten bytes: %v", err)
	}
	if got, want := len(reparsed.Names), len(asset.Names)+1; got != want {
		t.Fatalf("name count: got %d want %d", got, want)
	}
	last := reparsed.Names[len(reparsed.Names)-1]
	if got := last.Value; got != "BPX_Test_Name" {
		t.Fatalf("last name value: got %q", got)
	}
	if got, want := last.NonCaseHash, nonCaseHash; got != want {
		t.Fatalf("last nonCaseHash: got %d want %d", got, want)
	}
	if got, want := last.CasePreservingHash, caseHash; got != want {
		t.Fatalf("last casePreservingHash: got %d want %d", got, want)
	}

	newNameMapSize := len(encodeNameMap(reparsed.Names, order))
	delta := int32(newNameMapSize - originalNameMapSize)
	if got, want := reparsed.Summary.ImportOffset-asset.Summary.ImportOffset, delta; got != want {
		t.Fatalf("import offset delta: got %d want %d", got, want)
	}
	if got, want := reparsed.Summary.ExportOffset-asset.Summary.ExportOffset, delta; got != want {
		t.Fatalf("export offset delta: got %d want %d", got, want)
	}
}

func TestComputeNameEntryHashesUE56MatchesFixtureASCIIEntries(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "golden", "parse", "BP_Empty.uasset")
	asset, err := uasset.ParseFile(fixturePath, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	checked := 0
	for i, entry := range asset.Names {
		if !isASCIIName(entry.Value) {
			continue
		}
		nonCaseHash, caseHash := ComputeNameEntryHashesUE56(entry.Value)
		if entry.NonCaseHash != nonCaseHash || entry.CasePreservingHash != caseHash {
			t.Fatalf("name[%d]=%q hash mismatch: got (%d,%d) want (%d,%d)",
				i,
				entry.Value,
				nonCaseHash,
				caseHash,
				entry.NonCaseHash,
				entry.CasePreservingHash,
			)
		}
		checked++
	}
	if checked == 0 {
		t.Fatalf("no ASCII names were checked")
	}
}
