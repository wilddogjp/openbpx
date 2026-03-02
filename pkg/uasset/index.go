package uasset

import "fmt"

// PackageIndex is UE's FPackageIndex representation.
type PackageIndex int32

// Kind returns the index class: export/import/null.
func (p PackageIndex) Kind() string {
	switch {
	case p > 0:
		return "export"
	case p < 0:
		return "import"
	default:
		return "null"
	}
}

// IsNull reports whether the index is null.
func (p PackageIndex) IsNull() bool {
	return p == 0
}

// ResolveIndex returns the zero-based index for import/export or -1 for null.
func (p PackageIndex) ResolveIndex() int {
	switch {
	case p > 0:
		return int(p - 1)
	case p < 0:
		return int(-p - 1)
	default:
		return -1
	}
}

func (p PackageIndex) String() string {
	if p == 0 {
		return "null"
	}
	return fmt.Sprintf("%s:%d", p.Kind(), p.ResolveIndex())
}
