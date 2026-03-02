package uasset

// DecodeSerializedTextFromReader decodes one serialized FText payload from the current reader offset.
// The reader offset advances on success.
func (a *Asset) DecodeSerializedTextFromReader(r *ByteReader) (any, bool) {
	if a == nil || r == nil {
		return nil, false
	}
	return a.decodeTextPropertyFromReader(r)
}
