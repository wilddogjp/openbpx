package edit

import (
	"encoding/binary"
	"testing"
)

func TestPatchNameRefIndexAtAllowsInsertedNameIndex(t *testing.T) {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint32(data[:4], 5)

	changed, err := patchNameRefIndexAt(data, 0, map[int32]int32{5: 8}, binary.LittleEndian, 9)
	if err != nil {
		t.Fatalf("patchNameRefIndexAt returned error: %v", err)
	}
	if !changed {
		t.Fatalf("patchNameRefIndexAt reported no change")
	}
	got := int32(binary.LittleEndian.Uint32(data[:4]))
	if got != 8 {
		t.Fatalf("patched index = %d, want 8", got)
	}
}

func TestMaxRemappedNameCountExpandsForInsertedEntries(t *testing.T) {
	got := maxRemappedNameCount(4, map[int32]int32{0: 0, 1: 1, 2: 5})
	if got != 6 {
		t.Fatalf("maxRemappedNameCount = %d, want 6", got)
	}
}
