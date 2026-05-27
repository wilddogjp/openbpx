package edit

import (
	"os"
	"testing"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

func TestReplaceExportDependenciesRewritesSingleEntry(t *testing.T) {
	for _, root := range goldenFixtureRoots(t, "parse") {
		t.Run(root, func(t *testing.T) {
			fixturePath := goldenParseFixturePath(root, "BP_DependsMap.uasset")
			data, err := os.ReadFile(fixturePath)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			asset, err := uasset.ParseBytes(data, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("parse fixture: %v", err)
			}

			before, _, _, err := parseDependsMapEntries(asset)
			if err != nil {
				t.Fatalf("parse original depends map: %v", err)
			}

			out, err := ReplaceExportDependencies(asset, 0, []uasset.PackageIndex{-15, 4})
			if err != nil {
				t.Fatalf("replace export dependencies: %v", err)
			}
			rewritten, err := uasset.ParseBytes(out, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("parse rewritten asset: %v", err)
			}
			after, _, _, err := parseDependsMapEntries(rewritten)
			if err != nil {
				t.Fatalf("parse rewritten depends map: %v", err)
			}

			if got, want := after[0], []uasset.PackageIndex{-15, 4}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
				t.Fatalf("export[1] dependencies: got %v want %v", got, want)
			}
			if len(after[3]) != len(before[3]) {
				t.Fatalf("export[4] dependency count changed: got %d want %d", len(after[3]), len(before[3]))
			}
			for i := range before[3] {
				if after[3][i] != before[3][i] {
					t.Fatalf("export[4] dependency[%d]: got %v want %v", i, after[3][i], before[3][i])
				}
			}
		})
	}
}

func TestInsertImportEntriesRemapsDependsImportIndices(t *testing.T) {
	for _, root := range goldenFixtureRoots(t, "parse") {
		t.Run(root, func(t *testing.T) {
			fixturePath := goldenParseFixturePath(root, "BP_DependsMap.uasset")
			data, err := os.ReadFile(fixturePath)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			asset, err := uasset.ParseBytes(data, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("parse fixture: %v", err)
			}

			withDepends, err := ReplaceExportDependencies(asset, 0, []uasset.PackageIndex{-15})
			if err != nil {
				t.Fatalf("seed export dependency: %v", err)
			}
			seeded, err := uasset.ParseBytes(withDepends, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("parse seeded asset: %v", err)
			}

			entry := uasset.ImportEntry{
				ClassPackage: uasset.NameRef{Index: int32(findTestNameIndexByValue(seeded.Names, "/Script/CoreUObject"))},
				ClassName:    uasset.NameRef{Index: int32(findTestNameIndexByValue(seeded.Names, "Package"))},
				ObjectName:   uasset.NameRef{Index: int32(findTestNameIndexByValue(seeded.Names, "/Script/CoreUObject"))},
				PackageName:  uasset.NameRef{Index: int32(findTestNameIndexByValue(seeded.Names, "None"))},
			}
			out, err := InsertImportEntries(seeded, 0, []uasset.ImportEntry{entry})
			if err != nil {
				t.Fatalf("insert import entries: %v", err)
			}
			rewritten, err := uasset.ParseBytes(out, uasset.DefaultParseOptions())
			if err != nil {
				t.Fatalf("parse rewritten asset: %v", err)
			}
			after, _, _, err := parseDependsMapEntries(rewritten)
			if err != nil {
				t.Fatalf("parse rewritten depends map: %v", err)
			}
			if got, want := after[0], []uasset.PackageIndex{-16}; len(got) != len(want) || got[0] != want[0] {
				t.Fatalf("export[1] remapped dependencies: got %v want %v", got, want)
			}
		})
	}
}

func findTestNameIndexByValue(entries []uasset.NameEntry, needle string) int {
	for i, entry := range entries {
		if entry.Value == needle {
			return i
		}
	}
	return -1
}
