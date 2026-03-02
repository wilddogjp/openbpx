package edit

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func TestSkipSummaryCustomVersionsEnums(t *testing.T) {
	var b bytes.Buffer
	w32 := func(v int32) { must(binary.Write(&b, binary.LittleEndian, v)) }
	wu32 := func(v uint32) { must(binary.Write(&b, binary.LittleEndian, v)) }

	w32(1)      // custom version count
	wu32(7)     // legacy enum tag
	w32(3)      // version
	w32(0x1234) // marker after custom versions

	r := newByteCodec(b.Bytes(), binary.LittleEndian)
	if err := skipSummaryCustomVersions(r, -2); err != nil {
		t.Fatalf("skipSummaryCustomVersions(enums): %v", err)
	}
	marker, err := r.readInt32()
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if marker != 0x1234 {
		t.Fatalf("marker: got 0x%x want 0x1234", marker)
	}
}

func TestSkipSummaryCustomVersionsGuids(t *testing.T) {
	var b bytes.Buffer
	w32 := func(v int32) { must(binary.Write(&b, binary.LittleEndian, v)) }
	wstr := func(s string) {
		must(binary.Write(&b, binary.LittleEndian, int32(len(s)+1)))
		b.WriteString(s)
		b.WriteByte(0)
	}

	w32(1) // custom version count
	for i := 1; i <= 16; i++ {
		b.WriteByte(byte(i))
	}
	w32(9)
	wstr("Friendly")
	w32(0x4321) // marker after custom versions

	r := newByteCodec(b.Bytes(), binary.LittleEndian)
	if err := skipSummaryCustomVersions(r, -4); err != nil {
		t.Fatalf("skipSummaryCustomVersions(guids): %v", err)
	}
	marker, err := r.readInt32()
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if marker != 0x4321 {
		t.Fatalf("marker: got 0x%x want 0x4321", marker)
	}
}

func TestSkipSummaryCustomVersionsOptimized(t *testing.T) {
	var b bytes.Buffer
	w32 := func(v int32) { must(binary.Write(&b, binary.LittleEndian, v)) }

	w32(1) // custom version count
	for i := 1; i <= 16; i++ {
		b.WriteByte(byte(i))
	}
	w32(11)
	w32(0x9999) // marker after custom versions

	r := newByteCodec(b.Bytes(), binary.LittleEndian)
	if err := skipSummaryCustomVersions(r, -9); err != nil {
		t.Fatalf("skipSummaryCustomVersions(optimized): %v", err)
	}
	marker, err := r.readInt32()
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if marker != 0x9999 {
		t.Fatalf("marker: got 0x%x want 0x9999", marker)
	}
}

func TestSkipSummaryCustomVersionsRejectsNegativeCount(t *testing.T) {
	var b bytes.Buffer
	must(binary.Write(&b, binary.LittleEndian, int32(-1)))

	r := newByteCodec(b.Bytes(), binary.LittleEndian)
	err := skipSummaryCustomVersions(r, -9)
	if err == nil || !strings.Contains(err.Error(), "invalid custom version count") {
		t.Fatalf("expected invalid custom version count error, got: %v", err)
	}
}
