package uasset

import "fmt"

const (
	softObjectPathEntryMinBytes = int64(20)
	softObjectPathPreallocCap   = 4096
)

type softObjectPathEntry struct {
	PackageName NameRef
	AssetName   NameRef
	SubPath     string
}

func (a *Asset) softObjectPathAt(index int32) (softObjectPathEntry, bool) {
	var zero softObjectPathEntry
	if !a.ensureSoftObjectPathList() {
		return zero, false
	}
	if index < 0 || int(index) >= len(a.softObjectPathList) {
		return zero, false
	}
	return a.softObjectPathList[index], true
}

func (a *Asset) ensureSoftObjectPathList() bool {
	if a.softObjectPathListParsed {
		return len(a.softObjectPathList) > 0
	}
	a.softObjectPathListParsed = true

	count := a.Summary.SoftObjectPathsCount
	offset := a.Summary.SoftObjectPathsOffset
	if count <= 0 || offset <= 0 || !a.Summary.SupportsSoftObjectPathListInSummary() {
		return false
	}
	if int(offset) >= len(a.Raw.Bytes) {
		return false
	}
	available := int64(len(a.Raw.Bytes) - int(offset))
	if int64(count)*softObjectPathEntryMinBytes > available {
		return false
	}

	r := NewByteReaderWithByteSwapping(a.Raw.Bytes[offset:], a.Summary.UsesByteSwappedSerialization())
	capHint := int(count)
	if capHint > softObjectPathPreallocCap {
		capHint = softObjectPathPreallocCap
	}
	entries := make([]softObjectPathEntry, 0, capHint)
	for i := int32(0); i < count; i++ {
		pkg, err := r.ReadNameRef(len(a.Names))
		if err != nil {
			return false
		}
		assetName, err := r.ReadNameRef(len(a.Names))
		if err != nil {
			return false
		}
		subPath, err := r.ReadSoftObjectSubPath()
		if err != nil {
			return false
		}
		entries = append(entries, softObjectPathEntry{
			PackageName: pkg,
			AssetName:   assetName,
			SubPath:     subPath,
		})
	}
	a.softObjectPathList = entries
	return true
}

func (a *Asset) decodeSoftObjectPathIndex(index int32) map[string]any {
	out := map[string]any{
		"softObjectPathIndex": index,
	}
	if entry, ok := a.softObjectPathAt(index); ok {
		out["packageName"] = entry.PackageName.Display(a.Names)
		out["assetName"] = entry.AssetName.Display(a.Names)
		out["subPath"] = entry.SubPath
		return out
	}
	out["error"] = fmt.Sprintf("soft object path index %d out of range", index)
	return out
}
