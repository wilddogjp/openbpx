package uasset

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzParseAsset(f *testing.F) {
	seedFuzzInputs(f)

	opts := DefaultParseOptions()
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ParseBytes(data, opts)
	})
}

func FuzzRoundTrip(f *testing.F) {
	seedFuzzInputs(f)

	opts := DefaultParseOptions()
	f.Fuzz(func(t *testing.T, data []byte) {
		asset, err := ParseBytes(data, opts)
		if err != nil {
			return
		}
		reserialized := asset.Raw.SerializeUnmodified()
		if _, err := ParseBytes(reserialized, opts); err != nil {
			t.Fatalf("reparse after no-op serialization failed: %v", err)
		}
	})
}

func seedFuzzInputs(f *testing.F) {
	baseDir := filepath.Join("..", "..", "testdata")
	patterns := []string{
		filepath.Join(baseDir, "golden", "parse", "*.uasset"),
		filepath.Join(baseDir, "golden", "parse", "*.umap"),
		filepath.Join(baseDir, "synthetic", "*.uasset"),
		filepath.Join(baseDir, "synthetic", "*.bin"),
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			data, err := os.ReadFile(match)
			if err != nil || len(data) == 0 {
				continue
			}
			f.Add(data)
		}
	}
}
