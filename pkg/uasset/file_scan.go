package uasset

import (
	iofs "io/fs"
	"os"
	"path/filepath"
	"sort"
)

// CollectAssetFiles enumerates files under root matching the provided glob pattern.
// When recursive is true, subdirectories are traversed. Returned paths are sorted.
func CollectAssetFiles(root, pattern string, recursive bool) ([]string, error) {
	assets := make([]string, 0, 256)
	if recursive {
		err := filepath.WalkDir(root, func(path string, d iofs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			matched, err := filepath.Match(pattern, filepath.Base(path))
			if err != nil {
				return err
			}
			if matched {
				assets = append(assets, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		entries, err := os.ReadDir(root)
		if err != nil {
			return nil, err
		}
		for _, ent := range entries {
			if ent.IsDir() {
				continue
			}
			matched, err := filepath.Match(pattern, ent.Name())
			if err != nil {
				return nil, err
			}
			if matched {
				assets = append(assets, filepath.Join(root, ent.Name()))
			}
		}
	}
	sort.Strings(assets)
	return assets, nil
}
