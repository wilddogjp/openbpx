package uasset

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestParsePropertyTypeNameNodesRejectsExcessiveNodeCount(t *testing.T) {
	buf := make([]byte, 0, (maxPropertyTypeNodes+1)*12)
	appendI32 := func(v int32) {
		tmp := make([]byte, 4)
		binary.LittleEndian.PutUint32(tmp, uint32(v))
		buf = append(buf, tmp...)
	}

	for i := 0; i < maxPropertyTypeNodes+1; i++ {
		appendI32(0) // NameRef.Index
		appendI32(0) // NameRef.Number
		appendI32(1) // InnerCount (keeps parser in the loop)
	}

	r := NewByteReaderWithByteSwapping(buf, false)
	_, err := parsePropertyTypeNameNodes(r, 1)
	if err == nil {
		t.Fatalf("expected excessive node count to be rejected")
	}
	if !strings.Contains(err.Error(), "exceeds limit") {
		t.Fatalf("unexpected error: %v", err)
	}
}
