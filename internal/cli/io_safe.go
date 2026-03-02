package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func ensureOutputPathDistinctFromInput(inputPath, outPath string) error {
	if strings.TrimSpace(inputPath) == "" || strings.TrimSpace(outPath) == "" {
		return nil
	}
	inputCanonical, err := canonicalFilePath(inputPath)
	if err != nil {
		return fmt.Errorf("resolve input path: %w", err)
	}
	outCanonical, err := canonicalFilePath(outPath)
	if err != nil {
		return fmt.Errorf("resolve output path: %w", err)
	}
	if inputCanonical == outCanonical {
		return fmt.Errorf("refusing to overwrite input file: %s", outPath)
	}
	return nil
}

func canonicalFilePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(resolved), nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}

	dir := filepath.Dir(abs)
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		resolvedDir = dir
	}
	return filepath.Clean(filepath.Join(resolvedDir, filepath.Base(abs))), nil
}

func writeFileAtomically(path string, body []byte, mode os.FileMode) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("empty output path")
	}
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tmp, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(body); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace output file: %w", err)
	}
	committed = true

	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
