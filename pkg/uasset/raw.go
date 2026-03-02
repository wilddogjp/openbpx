package uasset

// RawAsset keeps the original asset bytes so a no-op write is byte identical.
type RawAsset struct {
	Bytes []byte
}

// SerializeUnmodified returns a copy of the original bytes.
func (r RawAsset) SerializeUnmodified() []byte {
	out := make([]byte, len(r.Bytes))
	copy(out, r.Bytes)
	return out
}
