package validate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type testdataManifest struct {
	Version  int                    `json:"version"`
	Fixtures []testdataManifestItem `json:"fixtures"`
}

type testdataManifestItem struct {
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
}

func TestManifestIntegrity(t *testing.T) {
	manifestPath := filepath.Join("..", "..", "testdata", "manifest.json")
	payload, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest.json: %v", err)
	}

	var manifest testdataManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		t.Fatalf("parse manifest json: %v", err)
	}
	if manifest.Version <= 0 {
		t.Fatalf("invalid manifest version: %d", manifest.Version)
	}
	if len(manifest.Fixtures) == 0 {
		t.Fatalf("manifest has no fixtures")
	}

	seen := make(map[string]struct{}, len(manifest.Fixtures))
	baseDir := filepath.Join("..", "..", "testdata")
	for _, item := range manifest.Fixtures {
		if strings.TrimSpace(item.Path) == "" {
			t.Fatalf("manifest fixture path is empty")
		}
		if filepath.IsAbs(item.Path) || strings.Contains(item.Path, "..") {
			t.Fatalf("manifest fixture path must be relative and normalized: %s", item.Path)
		}
		if _, exists := seen[item.Path]; exists {
			t.Fatalf("duplicate manifest path: %s", item.Path)
		}
		seen[item.Path] = struct{}{}

		fullPath := filepath.Join(baseDir, filepath.FromSlash(item.Path))
		data, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("read fixture %s: %v", item.Path, err)
		}
		if got, want := int64(len(data)), item.SizeBytes; got != want {
			t.Fatalf("size mismatch for %s: got %d want %d", item.Path, got, want)
		}
		hash := sha256.Sum256(data)
		if got, want := hex.EncodeToString(hash[:]), strings.ToLower(item.SHA256); got != want {
			t.Fatalf("sha256 mismatch for %s: got %s want %s", item.Path, got, want)
		}
	}
}
