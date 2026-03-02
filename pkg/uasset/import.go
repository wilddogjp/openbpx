package uasset

// ImportEntry mirrors one serialized FObjectImport entry used by read commands.
type ImportEntry struct {
	ClassPackage   NameRef      `json:"classPackage"`
	ClassName      NameRef      `json:"className"`
	OuterIndex     PackageIndex `json:"outerIndex"`
	ObjectName     NameRef      `json:"objectName"`
	PackageName    NameRef      `json:"packageName"`
	ImportOptional bool         `json:"importOptional"`
}
