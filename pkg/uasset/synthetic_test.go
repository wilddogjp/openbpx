package uasset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyntheticErrors(t *testing.T) {
	syntheticDir := filepath.Join("..", "..", "testdata", "synthetic")
	entries, err := os.ReadDir(syntheticDir)
	if err != nil {
		t.Fatalf("read synthetic dir: %v", err)
	}

	fixtures := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(entry.Name(), "gen_synthetic.go") {
			continue
		}
		if strings.HasPrefix(entry.Name(), "BP_UE") {
			continue
		}
		if entry.Name() == "BP_FutureVersion.uasset" {
			continue
		}
		fixtures = append(fixtures, filepath.Join(syntheticDir, entry.Name()))
	}
	if len(fixtures) == 0 {
		t.Fatalf("no synthetic error fixtures found")
	}

	opts := DefaultParseOptions()
	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			_, err := ParseFile(fixture, opts)
			if err == nil {
				t.Fatalf("expected parse failure for synthetic fixture")
			}
		})
	}
}

func TestVersionWindow(t *testing.T) {
	syntheticDir := filepath.Join("..", "..", "testdata", "synthetic")
	rejectedFixture := filepath.Join(syntheticDir, "BP_FutureVersion.uasset")
	if _, err := ParseFile(rejectedFixture, DefaultParseOptions()); err == nil {
		t.Fatalf("expected future-version fixture to be rejected")
	}
}
