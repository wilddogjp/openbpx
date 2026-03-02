package cli

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

func TestSkipSummaryCustomVersionsForRewriteEnums(t *testing.T) {
	var b bytes.Buffer
	mustWrite := func(err error) {
		if err != nil {
			t.Fatalf("binary write: %v", err)
		}
	}
	w32 := func(v int32) { mustWrite(binary.Write(&b, binary.LittleEndian, v)) }
	wu32 := func(v uint32) { mustWrite(binary.Write(&b, binary.LittleEndian, v)) }

	w32(1)
	wu32(9)
	w32(4)
	w32(0x55AA)

	r := uasset.NewByteReader(b.Bytes())
	if err := skipSummaryCustomVersionsForRewrite(r, -2); err != nil {
		t.Fatalf("skipSummaryCustomVersionsForRewrite(enums): %v", err)
	}
	marker, err := r.ReadInt32()
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if marker != 0x55AA {
		t.Fatalf("marker: got 0x%x want 0x55AA", marker)
	}
}

func TestSkipSummaryCustomVersionsForRewriteGuids(t *testing.T) {
	var b bytes.Buffer
	mustWrite := func(err error) {
		if err != nil {
			t.Fatalf("binary write: %v", err)
		}
	}
	w32 := func(v int32) { mustWrite(binary.Write(&b, binary.LittleEndian, v)) }
	wstr := func(s string) {
		mustWrite(binary.Write(&b, binary.LittleEndian, int32(len(s)+1)))
		b.WriteString(s)
		b.WriteByte(0)
	}

	w32(1)
	for i := 1; i <= 16; i++ {
		b.WriteByte(byte(i))
	}
	w32(8)
	wstr("Friendly")
	w32(0xAA55)

	r := uasset.NewByteReader(b.Bytes())
	if err := skipSummaryCustomVersionsForRewrite(r, -4); err != nil {
		t.Fatalf("skipSummaryCustomVersionsForRewrite(guids): %v", err)
	}
	marker, err := r.ReadInt32()
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if marker != 0xAA55 {
		t.Fatalf("marker: got 0x%x want 0xAA55", marker)
	}
}

func TestSkipSummaryCustomVersionsForRewriteRejectsNegativeCount(t *testing.T) {
	var b bytes.Buffer
	if err := binary.Write(&b, binary.LittleEndian, int32(-1)); err != nil {
		t.Fatalf("binary write: %v", err)
	}

	r := uasset.NewByteReader(b.Bytes())
	err := skipSummaryCustomVersionsForRewrite(r, -9)
	if err == nil || !strings.Contains(err.Error(), "invalid custom version count") {
		t.Fatalf("expected invalid custom version count error, got: %v", err)
	}
}
