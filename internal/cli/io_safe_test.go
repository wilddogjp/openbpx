package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureOutputPathDistinctFromInputRejectsIdenticalPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "A.uasset")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := ensureOutputPathDistinctFromInput(path, path); err == nil {
		t.Fatalf("expected identical path to be rejected")
	}
}

func TestEnsureOutputPathDistinctFromInputRejectsSymlinkAlias(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.uasset")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	alias := filepath.Join(dir, "alias.uasset")
	if err := os.Symlink(target, alias); err != nil {
		t.Skipf("symlink not available: %v", err)
	}
	if err := ensureOutputPathDistinctFromInput(target, alias); err == nil {
		t.Fatalf("expected symlink alias path to be rejected")
	}
}

func TestWriteFileAtomicallyReplacesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatalf("write before: %v", err)
	}
	if err := writeFileAtomically(path, []byte("after"), 0o600); err != nil {
		t.Fatalf("writeFileAtomically: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	if string(got) != "after" {
		t.Fatalf("result body: got %q want %q", string(got), "after")
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".*.tmp-*"))
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files leaked: %s", strings.Join(matches, ", "))
	}
}
