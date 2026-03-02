package uasset

import "testing"

func TestEnsureSoftObjectPathListRejectsImpossibleCountToBytesRatio(t *testing.T) {
	asset := &Asset{
		Raw:   RawAsset{Bytes: make([]byte, 64)},
		Names: []NameEntry{{Value: "None"}},
		Summary: PackageSummary{
			FileVersionUE5:        ue5AddSoftObjectPathList,
			SoftObjectPathsCount:  100,
			SoftObjectPathsOffset: 1,
		},
	}
	if asset.ensureSoftObjectPathList() {
		t.Fatalf("expected impossible soft object path count to be rejected")
	}
}
