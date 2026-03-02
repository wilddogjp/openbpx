package uasset

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectAssetFilesRecursive(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "Sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mustWriteTestFile(t, filepath.Join(root, "A.uasset"))
	mustWriteTestFile(t, filepath.Join(sub, "B.uasset"))
	mustWriteTestFile(t, filepath.Join(sub, "C.txt"))

	files, err := CollectAssetFiles(root, "*.uasset", true)
	if err != nil {
		t.Fatalf("CollectAssetFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("file count: got %d want 2", len(files))
	}
}

func TestCollectAssetFilesNonRecursive(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "Sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mustWriteTestFile(t, filepath.Join(root, "A.uasset"))
	mustWriteTestFile(t, filepath.Join(sub, "B.uasset"))

	files, err := CollectAssetFiles(root, "*.uasset", false)
	if err != nil {
		t.Fatalf("CollectAssetFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("file count: got %d want 1", len(files))
	}
}

func mustWriteTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}
