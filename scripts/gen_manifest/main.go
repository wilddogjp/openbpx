package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type manifest struct {
	Version   int            `json:"version"`
	Generated string         `json:"generated"`
	Fixtures  []manifestItem `json:"fixtures"`
}

type manifestItem struct {
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
}

func main() {
	repoRoot, err := os.Getwd()
	if err != nil {
		die("getwd", err)
	}

	patterns := []string{
		"testdata/golden/parse/*.uasset",
		"testdata/golden/parse/*.umap",
		"testdata/golden/parse/*.locres",
		"testdata/golden/operations/*/before.uasset",
		"testdata/golden/operations/*/after.uasset",
		"testdata/golden/operations/*/operation.json",
		"testdata/golden/expected_output/*.json",
		"testdata/synthetic/*.uasset",
		"testdata/synthetic/*.bin",
	}

	paths := make([]string, 0, 256)
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(repoRoot, pattern))
		if err != nil {
			die("glob fixtures", err)
		}
		for _, match := range matches {
			rel, err := filepath.Rel(filepath.Join(repoRoot, "testdata"), match)
			if err != nil {
				die("rel fixture path", err)
			}
			paths = append(paths, filepath.ToSlash(rel))
		}
	}
	sort.Strings(paths)

	items := make([]manifestItem, 0, len(paths))
	for _, rel := range paths {
		fullPath := filepath.Join(repoRoot, "testdata", filepath.FromSlash(rel))
		data, err := os.ReadFile(fullPath)
		if err != nil {
			die("read fixture", fmt.Errorf("%s: %w", rel, err))
		}
		hash := sha256.Sum256(data)
		items = append(items, manifestItem{
			Path:      rel,
			SHA256:    hex.EncodeToString(hash[:]),
			SizeBytes: int64(len(data)),
		})
	}

	payload := manifest{
		Version:   1,
		Generated: time.Now().Format("2006-01-02"),
		Fixtures:  items,
	}
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		die("marshal manifest", err)
	}
	body = append(body, '\n')

	outPath := filepath.Join(repoRoot, "testdata", "manifest.json")
	if err := os.WriteFile(outPath, body, 0o644); err != nil {
		die("write manifest", err)
	}
}

func die(context string, err error) {
	fmt.Fprintf(os.Stderr, "error: %s: %v\n", context, err)
	os.Exit(1)
}
