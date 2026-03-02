package edit

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"unicode/utf16"
)

const (
	ue5OptionalResources              = int32(1003)
	ue5RemoveObjectExportPkgGUID      = int32(1005)
	ue5TrackObjectExportInherited     = int32(1006)
	ue5AddSoftObjectPathList          = int32(1008)
	ue5DataResources                  = int32(1009)
	ue5ScriptSerializationOffset      = int32(1010)
	ue5PropertyTagExtension           = int32(1011)
	ue5PropertyTagCompleteType        = int32(1012)
	ue5MetadataSerializationOff       = int32(1014)
	ue5VerseCells                     = int32(1015)
	ue5PackageSavedHash               = int32(1016)
	ue5PayloadTOC                     = int32(1002)
	ue5NamesFromExportData            = int32(1001)
	ue5OSSubObjectShadowSerialization = int32(1017)
	ue4VersionUE56                    = int32(522)

	packageFileTag        = uint32(0x9E2A83C1)
	packageFileTagSwapped = uint32(0xC1832A9E)

	pkgFlagFilterEditorOnly = uint32(0x80000000)
	pkgFlagUnversionedProps = uint32(0x00002000)
)

type byteCodec struct {
	data  []byte
	off   int
	order binary.ByteOrder
}

func newByteCodec(data []byte, order binary.ByteOrder) *byteCodec {
	return &byteCodec{data: data, order: order}
}

func (c *byteCodec) remaining() int {
	return len(c.data) - c.off
}

func (c *byteCodec) seek(off int) error {
	if off < 0 || off > len(c.data) {
		return fmt.Errorf("seek out of range: %d", off)
	}
	c.off = off
	return nil
}

func (c *byteCodec) skip(n int) error {
	if n < 0 {
		return fmt.Errorf("negative skip: %d", n)
	}
	if c.remaining() < n {
		return errors.New("unexpected EOF")
	}
	c.off += n
	return nil
}

func (c *byteCodec) readBytes(n int) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("negative read: %d", n)
	}
	if c.remaining() < n {
		return nil, errors.New("unexpected EOF")
	}
	start := c.off
	c.off += n
	return c.data[start:c.off], nil
}

func (c *byteCodec) readInt32() (int32, error) {
	b, err := c.readBytes(4)
	if err != nil {
		return 0, err
	}
	return int32(c.order.Uint32(b)), nil
}

func (c *byteCodec) readUint32() (uint32, error) {
	b, err := c.readBytes(4)
	if err != nil {
		return 0, err
	}
	return c.order.Uint32(b), nil
}

func (c *byteCodec) readInt64() (int64, error) {
	b, err := c.readBytes(8)
	if err != nil {
		return 0, err
	}
	return int64(c.order.Uint64(b)), nil
}

func (c *byteCodec) readFString() (string, error) {
	lenField, err := c.readInt32()
	if err != nil {
		return "", err
	}
	if lenField == 0 {
		return "", nil
	}
	if lenField > 0 {
		charCount := int(lenField)
		if charCount < 0 {
			return "", fmt.Errorf("invalid string length: %d", lenField)
		}
		buf, err := c.readBytes(charCount)
		if err != nil {
			return "", err
		}
		if len(buf) > 0 && buf[len(buf)-1] == 0 {
			buf = buf[:len(buf)-1]
		}
		return string(buf), nil
	}

	if lenField == math.MinInt32 {
		return "", fmt.Errorf("invalid wide string length: %d", lenField)
	}
	wideCount := int(-lenField)
	if wideCount <= 0 {
		return "", fmt.Errorf("invalid wide string length: %d", lenField)
	}
	byteCount := wideCount * 2
	buf, err := c.readBytes(byteCount)
	if err != nil {
		return "", err
	}
	units := make([]uint16, 0, wideCount)
	for i := 0; i+1 < len(buf); i += 2 {
		units = append(units, c.order.Uint16(buf[i:i+2]))
	}
	if len(units) > 0 && units[len(units)-1] == 0 {
		units = units[:len(units)-1]
	}
	return string(utf16.Decode(units)), nil
}

type byteWriter struct {
	order binary.ByteOrder
	buf   []byte
}

func newByteWriter(order binary.ByteOrder, capHint int) *byteWriter {
	return &byteWriter{order: order, buf: make([]byte, 0, capHint)}
}

func (w *byteWriter) bytes() []byte {
	out := make([]byte, len(w.buf))
	copy(out, w.buf)
	return out
}

func (w *byteWriter) writeBytes(v []byte) {
	w.buf = append(w.buf, v...)
}

func (w *byteWriter) writeUint8(v uint8) {
	w.buf = append(w.buf, v)
}

func (w *byteWriter) writeInt32(v int32) {
	var b [4]byte
	w.order.PutUint32(b[:], uint32(v))
	w.buf = append(w.buf, b[:]...)
}

func (w *byteWriter) writeUint32(v uint32) {
	var b [4]byte
	w.order.PutUint32(b[:], v)
	w.buf = append(w.buf, b[:]...)
}

func (w *byteWriter) writeUint16(v uint16) {
	var b [2]byte
	w.order.PutUint16(b[:], v)
	w.buf = append(w.buf, b[:]...)
}

func (w *byteWriter) writeInt64(v int64) {
	var b [8]byte
	w.order.PutUint64(b[:], uint64(v))
	w.buf = append(w.buf, b[:]...)
}

func (w *byteWriter) writeNameRef(index, number int32) {
	w.writeInt32(index)
	w.writeInt32(number)
}

func (w *byteWriter) writeUBool(v bool) {
	if v {
		w.writeUint32(1)
	} else {
		w.writeUint32(0)
	}
}

func (w *byteWriter) writeFString(v string) {
	if v == "" {
		w.writeInt32(0)
		return
	}
	ascii := true
	for _, r := range v {
		if r > 0x7f {
			ascii = false
			break
		}
	}
	if ascii {
		w.writeInt32(int32(len(v) + 1))
		w.writeBytes([]byte(v))
		w.writeUint8(0)
		return
	}
	units := utf16.Encode([]rune(v))
	w.writeInt32(-int32(len(units) + 1))
	for _, u := range units {
		var b [2]byte
		w.order.PutUint16(b[:], u)
		w.writeBytes(b[:])
	}
	var z [2]byte
	w.order.PutUint16(z[:], 0)
	w.writeBytes(z[:])
}

func readInt32At(data []byte, off int, order binary.ByteOrder) (int32, error) {
	if off < 0 || off+4 > len(data) {
		return 0, fmt.Errorf("int32 read out of range: %d", off)
	}
	return int32(order.Uint32(data[off : off+4])), nil
}

func readInt64At(data []byte, off int, order binary.ByteOrder) (int64, error) {
	if off < 0 || off+8 > len(data) {
		return 0, fmt.Errorf("int64 read out of range: %d", off)
	}
	return int64(order.Uint64(data[off : off+8])), nil
}

func writeInt32At(data []byte, off int, value int32, order binary.ByteOrder) error {
	if off < 0 || off+4 > len(data) {
		return fmt.Errorf("int32 write out of range: %d", off)
	}
	order.PutUint32(data[off:off+4], uint32(value))
	return nil
}

func writeInt64At(data []byte, off int, value int64, order binary.ByteOrder) error {
	if off < 0 || off+8 > len(data) {
		return fmt.Errorf("int64 write out of range: %d", off)
	}
	order.PutUint64(data[off:off+8], uint64(value))
	return nil
}
