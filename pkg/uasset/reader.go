package uasset

import (
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf16"
)

// ByteReader is a bounded little-endian binary reader used by parsers and CLI helpers.
type ByteReader struct {
	data     []byte
	off      int
	byteSwap bool
}

// byteReader is kept as a package-local alias for existing parser code paths.
// TODO: migrate internal callsites to exported methods and remove lowercase wrappers.
type byteReader = ByteReader

// NewByteReader creates a new reader for the given byte slice.
func NewByteReader(data []byte) *ByteReader {
	return &ByteReader{data: data}
}

// NewByteReaderWithByteSwapping creates a reader with explicit byte-swapping behavior.
func NewByteReaderWithByteSwapping(data []byte, byteSwap bool) *ByteReader {
	return &ByteReader{data: data, byteSwap: byteSwap}
}

func newByteReader(data []byte) *byteReader {
	return &byteReader{data: data}
}

func (r *ByteReader) SetByteSwapping(enabled bool) {
	r.byteSwap = enabled
}

func (r *ByteReader) IsByteSwapping() bool {
	return r.byteSwap
}

func (r *ByteReader) byteOrder() binary.ByteOrder {
	if r.byteSwap {
		return binary.BigEndian
	}
	return binary.LittleEndian
}

func (r *ByteReader) Offset() int {
	return r.off
}

func (r *ByteReader) offset() int {
	return r.Offset()
}

func (r *ByteReader) EOF() bool {
	return r.off >= len(r.data)
}

func (r *ByteReader) eof() bool {
	return r.EOF()
}

func (r *ByteReader) Seek(offset int) error {
	if offset < 0 || offset > len(r.data) {
		return fmt.Errorf("seek out of range: %d", offset)
	}
	r.off = offset
	return nil
}

func (r *ByteReader) seek(offset int) error {
	return r.Seek(offset)
}

func (r *ByteReader) Remaining() int {
	return len(r.data) - r.off
}

func (r *ByteReader) remaining() int {
	return r.Remaining()
}

func (r *ByteReader) ReadBytes(n int) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("negative length: %d", n)
	}
	if r.Remaining() < n {
		return nil, fmt.Errorf("unexpected EOF (need %d bytes, have %d)", n, r.Remaining())
	}
	start := r.off
	r.off += n
	return r.data[start:r.off], nil
}

func (r *ByteReader) readBytes(n int) ([]byte, error) {
	return r.ReadBytes(n)
}

func (r *ByteReader) Skip(n int) error {
	_, err := r.ReadBytes(n)
	return err
}

func (r *ByteReader) skip(n int) error {
	return r.Skip(n)
}

func (r *ByteReader) ReadInt32() (int32, error) {
	b, err := r.ReadBytes(4)
	if err != nil {
		return 0, err
	}
	return int32(r.byteOrder().Uint32(b)), nil
}

func (r *ByteReader) readInt32() (int32, error) {
	return r.ReadInt32()
}

func (r *ByteReader) ReadUint32() (uint32, error) {
	b, err := r.ReadBytes(4)
	if err != nil {
		return 0, err
	}
	return r.byteOrder().Uint32(b), nil
}

func (r *ByteReader) readUint32() (uint32, error) {
	return r.ReadUint32()
}

func (r *ByteReader) ReadInt64() (int64, error) {
	b, err := r.ReadBytes(8)
	if err != nil {
		return 0, err
	}
	return int64(r.byteOrder().Uint64(b)), nil
}

func (r *ByteReader) readInt64() (int64, error) {
	return r.ReadInt64()
}

func (r *ByteReader) ReadUint16() (uint16, error) {
	b, err := r.ReadBytes(2)
	if err != nil {
		return 0, err
	}
	return r.byteOrder().Uint16(b), nil
}

func (r *ByteReader) readUint16() (uint16, error) {
	return r.ReadUint16()
}

func (r *ByteReader) ReadUint8() (uint8, error) {
	b, err := r.ReadBytes(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func (r *ByteReader) readUint8() (uint8, error) {
	return r.ReadUint8()
}

func (r *ByteReader) ReadUBool() (bool, error) {
	v, err := r.ReadUint32()
	if err != nil {
		return false, err
	}
	// UE codepaths commonly treat uint32 bool as non-zero true.
	return v != 0, nil
}

func (r *ByteReader) readUBool() (bool, error) {
	return r.ReadUBool()
}

func (r *ByteReader) ReadGUID() (GUID, error) {
	var g GUID
	a, err := r.ReadUint32()
	if err != nil {
		return g, err
	}
	b, err := r.ReadUint32()
	if err != nil {
		return g, err
	}
	c, err := r.ReadUint32()
	if err != nil {
		return g, err
	}
	d, err := r.ReadUint32()
	if err != nil {
		return g, err
	}
	binary.LittleEndian.PutUint32(g[0:4], a)
	binary.LittleEndian.PutUint32(g[4:8], b)
	binary.LittleEndian.PutUint32(g[8:12], c)
	binary.LittleEndian.PutUint32(g[12:16], d)
	return g, nil
}

func (r *ByteReader) readGUID() (GUID, error) {
	return r.ReadGUID()
}

func (r *ByteReader) ReadHash20() ([20]byte, error) {
	var h [20]byte
	b, err := r.ReadBytes(20)
	if err != nil {
		return h, err
	}
	copy(h[:], b)
	return h, nil
}

func (r *ByteReader) readHash20() ([20]byte, error) {
	return r.ReadHash20()
}

func (r *ByteReader) ReadFString() (string, error) {
	lenField, err := r.ReadInt32()
	if err != nil {
		return "", err
	}
	if lenField == 0 {
		return "", nil
	}
	if lenField > 0 {
		charCount64 := int64(lenField)
		if charCount64 > maxFStringBytes {
			return "", fmt.Errorf("fstring too large: %d bytes", charCount64)
		}
		charCount := int(charCount64)
		buf, err := r.readBytes(charCount)
		if err != nil {
			return "", err
		}
		if charCount > 0 && buf[charCount-1] == 0 {
			buf = buf[:charCount-1]
		}
		return string(buf), nil
	}

	if lenField == math.MinInt32 {
		return "", fmt.Errorf("invalid wide string count: %d", lenField)
	}
	wideCount64 := -int64(lenField)
	if wideCount64 <= 0 || wideCount64 > maxFStringUTF16Units {
		return "", fmt.Errorf("invalid or too-large wide string count: %d", wideCount64)
	}
	byteCount := wideCount64 * 2
	if byteCount > maxFStringBytes {
		return "", fmt.Errorf("wide fstring too large: %d bytes", byteCount)
	}
	buf, err := r.readBytes(int(byteCount))
	if err != nil {
		return "", err
	}
	wideCount := int(wideCount64)
	vals := make([]uint16, wideCount)
	order := r.byteOrder()
	for i := 0; i < wideCount; i++ {
		vals[i] = order.Uint16(buf[i*2:])
	}
	if vals[len(vals)-1] == 0 {
		vals = vals[:len(vals)-1]
	}
	return string(utf16.Decode(vals)), nil
}

func (r *ByteReader) readFString() (string, error) {
	return r.ReadFString()
}

// ReadUTF8String decodes UE UTF-8 string serialization used by FUtf8String.
func (r *ByteReader) ReadUTF8String() (string, error) {
	lenField, err := r.ReadInt32()
	if err != nil {
		return "", err
	}
	if lenField < 0 {
		return "", fmt.Errorf("invalid utf8 string count: %d", lenField)
	}
	if int64(lenField) > maxFStringBytes {
		return "", fmt.Errorf("utf8 string too large: %d bytes", lenField)
	}
	if lenField == 0 {
		return "", nil
	}
	buf, err := r.ReadBytes(int(lenField))
	if err != nil {
		return "", err
	}
	// Keep compatibility with older SoftObjectPath writers that included trailing NULs.
	for len(buf) > 0 && buf[len(buf)-1] == 0 {
		buf = buf[:len(buf)-1]
	}
	return string(buf), nil
}

// ReadSoftObjectSubPath decodes modern FUtf8String sub-path and falls back to legacy FString variants.
func (r *ByteReader) ReadSoftObjectSubPath() (string, error) {
	start := r.Offset()
	v, err := r.ReadUTF8String()
	if err == nil {
		return v, nil
	}
	_ = r.Seek(start)
	return r.ReadFString()
}

func (r *ByteReader) readSoftObjectSubPath() (string, error) {
	return r.ReadSoftObjectSubPath()
}

func (r *ByteReader) ReadNameRef(nameCount int) (NameRef, error) {
	idx, err := r.ReadInt32()
	if err != nil {
		return NameRef{}, err
	}
	num, err := r.ReadInt32()
	if err != nil {
		return NameRef{}, err
	}
	if idx < 0 || int(idx) >= nameCount {
		return NameRef{}, fmt.Errorf("name index out of range: %d (name count %d)", idx, nameCount)
	}
	return NameRef{Index: idx, Number: num}, nil
}

func (r *ByteReader) readNameRef(nameCount int) (NameRef, error) {
	return r.ReadNameRef(nameCount)
}

func (r *ByteReader) ReadEngineVersion() (EngineVersion, error) {
	var v EngineVersion
	major, err := r.ReadUint16()
	if err != nil {
		return v, err
	}
	minor, err := r.ReadUint16()
	if err != nil {
		return v, err
	}
	patch, err := r.ReadUint16()
	if err != nil {
		return v, err
	}
	cl, err := r.ReadUint32()
	if err != nil {
		return v, err
	}
	branch, err := r.ReadFString()
	if err != nil {
		return v, err
	}
	v.Major = major
	v.Minor = minor
	v.Patch = patch
	v.Changelist = cl
	v.Branch = branch
	return v, nil
}

func (r *ByteReader) readEngineVersion() (EngineVersion, error) {
	return r.ReadEngineVersion()
}
