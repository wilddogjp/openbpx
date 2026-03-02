package edit

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

func TestBuildPropertyAddMutationMatchesUEFixture(t *testing.T) {
	beforePath := filepath.Join("..", "..", "testdata", "golden", "operations", "prop_add", "before.uasset")
	afterPath := filepath.Join("..", "..", "testdata", "golden", "operations", "prop_add", "after.uasset")
	beforeAsset := mustParseAsset(t, beforePath)
	wantAfter := mustReadFile(t, afterPath)

	res, err := BuildPropertyAddMutation(beforeAsset, 4, `{"name":"bCanBeDamaged","type":"BoolProperty","value":false}`)
	if err != nil {
		t.Fatalf("BuildPropertyAddMutation: %v", err)
	}
	if res.PropertyName != "bCanBeDamaged" {
		t.Fatalf("PropertyName: got %q want bCanBeDamaged", res.PropertyName)
	}

	gotAfter, err := RewriteAsset(beforeAsset, []ExportMutation{res.Mutation})
	if err != nil {
		t.Fatalf("RewriteAsset: %v", err)
	}
	if !equalBytesWithIgnoredRanges(gotAfter, wantAfter, []ignoreRange{{Offset: 24, Length: 20}}) {
		t.Fatalf("rewritten bytes do not match UE fixture")
	}
}

func TestBuildPropertyRemoveMutationMatchesUEFixture(t *testing.T) {
	beforePath := filepath.Join("..", "..", "testdata", "golden", "operations", "prop_remove", "before.uasset")
	afterPath := filepath.Join("..", "..", "testdata", "golden", "operations", "prop_remove", "after.uasset")
	beforeAsset := mustParseAsset(t, beforePath)
	wantAfter := mustReadFile(t, afterPath)

	res, err := BuildPropertyRemoveMutation(beforeAsset, 4, "bCanBeDamaged")
	if err != nil {
		t.Fatalf("BuildPropertyRemoveMutation: %v", err)
	}
	if res.PropertyName != "bCanBeDamaged" {
		t.Fatalf("PropertyName: got %q want bCanBeDamaged", res.PropertyName)
	}

	gotAfter, err := RewriteAsset(beforeAsset, []ExportMutation{res.Mutation})
	if err != nil {
		t.Fatalf("RewriteAsset: %v", err)
	}
	if !equalBytesWithIgnoredRanges(gotAfter, wantAfter, []ignoreRange{{Offset: 24, Length: 20}}) {
		t.Fatalf("rewritten bytes do not match UE fixture")
	}
}

func TestBuildPropertyAddMutationRejectsExistingProperty(t *testing.T) {
	beforePath := filepath.Join("..", "..", "testdata", "golden", "operations", "prop_remove", "before.uasset")
	beforeAsset := mustParseAsset(t, beforePath)

	_, err := BuildPropertyAddMutation(beforeAsset, 4, `{"name":"bCanBeDamaged","type":"BoolProperty","value":false}`)
	if err == nil {
		t.Fatalf("expected error when adding existing property")
	}
}

func TestBuildPropertyRemoveMutationRejectsMissingProperty(t *testing.T) {
	beforePath := filepath.Join("..", "..", "testdata", "golden", "operations", "prop_add", "before.uasset")
	beforeAsset := mustParseAsset(t, beforePath)

	_, err := BuildPropertyRemoveMutation(beforeAsset, 4, "bCanBeDamaged")
	if err == nil {
		t.Fatalf("expected error when removing missing property")
	}
}

func mustParseAsset(t *testing.T, path string) *uasset.Asset {
	t.Helper()
	asset, err := uasset.ParseFile(path, uasset.ParseOptions{
		KeepUnknown: true,
	})
	if err != nil {
		t.Fatalf("ParseFile(%s): %v", path, err)
	}
	return asset
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	return b
}

type ignoreRange struct {
	Offset int
	Length int
}

func equalBytesWithIgnoredRanges(left, right []byte, ignored []ignoreRange) bool {
	if len(left) != len(right) {
		return false
	}
	l := append([]byte(nil), left...)
	r := append([]byte(nil), right...)
	for _, item := range ignored {
		if item.Offset < 0 || item.Length < 0 || item.Offset+item.Length > len(l) {
			return false
		}
		for i := item.Offset; i < item.Offset+item.Length; i++ {
			l[i] = 0
			r[i] = 0
		}
	}
	return bytes.Equal(l, r)
}
