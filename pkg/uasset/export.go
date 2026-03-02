package uasset

// ExportEntry mirrors one serialized FObjectExport entry used by read commands.
type ExportEntry struct {
	ClassIndex    PackageIndex `json:"classIndex"`
	SuperIndex    PackageIndex `json:"superIndex"`
	TemplateIndex PackageIndex `json:"templateIndex"`
	OuterIndex    PackageIndex `json:"outerIndex"`
	ObjectName    NameRef      `json:"objectName"`
	ObjectFlags   uint32       `json:"objectFlags"`
	SerialSize    int64        `json:"serialSize"`
	SerialOffset  int64        `json:"serialOffset"`

	// Relative offsets within the export serialized payload (not file absolute offsets).
	// UE source reference: SavePackage2.cpp adjusts by SerialOffset before saving export map.
	ScriptSerializationStartOffset int64 `json:"scriptSerializationStartOffset"`
	ScriptSerializationEndOffset   int64 `json:"scriptSerializationEndOffset"`

	ForcedExport             bool   `json:"forcedExport"`
	NotForClient             bool   `json:"notForClient"`
	NotForServer             bool   `json:"notForServer"`
	NotAlwaysLoadedForEditor bool   `json:"notAlwaysLoadedForEditor"`
	IsAsset                  bool   `json:"isAsset"`
	IsInheritedInstance      bool   `json:"isInheritedInstance"`
	GeneratePublicHash       bool   `json:"generatePublicHash"`
	PackageFlags             uint32 `json:"packageFlags"`

	FirstExportDependency                 int32 `json:"firstExportDependency"`
	SerializationBeforeSerializationDeps  int32 `json:"serializationBeforeSerializationDependencies"`
	CreateBeforeSerializationDeps         int32 `json:"createBeforeSerializationDependencies"`
	SerializationBeforeCreateDependencies int32 `json:"serializationBeforeCreateDependencies"`
	CreateBeforeCreateDependencies        int32 `json:"createBeforeCreateDependencies"`
}
