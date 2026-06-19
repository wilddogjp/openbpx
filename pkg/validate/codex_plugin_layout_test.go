package validate

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type codexMarketplace struct {
	Plugins []codexMarketplacePlugin `json:"plugins"`
}

type codexMarketplacePlugin struct {
	Name   string `json:"name"`
	Source struct {
		Source string `json:"source"`
		Path   string `json:"path"`
	} `json:"source"`
	Policy struct {
		Installation   string `json:"installation"`
		Authentication string `json:"authentication"`
	} `json:"policy"`
	Category string `json:"category"`
}

type codexPluginManifest struct {
	Name   string `json:"name"`
	Skills string `json:"skills"`
}

func TestCodexMarketplaceUsesStandardPluginLayout(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	marketplacePath := filepath.Join(repoRoot, ".agents", "plugins", "marketplace.json")
	payload, err := os.ReadFile(marketplacePath)
	if err != nil {
		t.Fatalf("read marketplace: %v", err)
	}

	var marketplace codexMarketplace
	if err := json.Unmarshal(payload, &marketplace); err != nil {
		t.Fatalf("parse marketplace: %v", err)
	}

	var openbpx *codexMarketplacePlugin
	for i := range marketplace.Plugins {
		if marketplace.Plugins[i].Name == "openbpx" {
			openbpx = &marketplace.Plugins[i]
			break
		}
	}
	if openbpx == nil {
		t.Fatalf("marketplace missing openbpx plugin entry")
	}

	if got, want := openbpx.Source.Source, "local"; got != want {
		t.Fatalf("source kind: got %q want %q", got, want)
	}
	if got, want := openbpx.Source.Path, "./plugins/openbpx"; got != want {
		t.Fatalf("source path: got %q want %q", got, want)
	}
	if got, want := openbpx.Policy.Installation, "AVAILABLE"; got != want {
		t.Fatalf("installation policy: got %q want %q", got, want)
	}
	if got, want := openbpx.Policy.Authentication, "ON_INSTALL"; got != want {
		t.Fatalf("authentication policy: got %q want %q", got, want)
	}
	if strings.TrimSpace(openbpx.Category) == "" {
		t.Fatalf("category is required")
	}

	pluginRoot := filepath.Join(repoRoot, filepath.FromSlash(strings.TrimPrefix(openbpx.Source.Path, "./")))
	manifestPath := filepath.Join(pluginRoot, ".codex-plugin", "plugin.json")
	manifestPayload, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read plugin manifest: %v", err)
	}

	var manifest codexPluginManifest
	if err := json.Unmarshal(manifestPayload, &manifest); err != nil {
		t.Fatalf("parse plugin manifest: %v", err)
	}
	if got, want := manifest.Name, "openbpx"; got != want {
		t.Fatalf("manifest name: got %q want %q", got, want)
	}
	if got, want := manifest.Skills, "./skills/"; got != want {
		t.Fatalf("manifest skills path: got %q want %q", got, want)
	}

	rootManifestPayload, err := os.ReadFile(filepath.Join(repoRoot, ".codex-plugin", "plugin.json"))
	if err != nil {
		t.Fatalf("read root plugin manifest: %v", err)
	}
	if !bytes.Equal(manifestPayload, rootManifestPayload) {
		t.Fatalf("plugins/openbpx manifest must match root .codex-plugin manifest")
	}

	assertRequiredFiles(t, pluginRoot, []string{
		"skills/bpx-shared/SKILL.md",
		"bin/bpx",
		"bin/bpx.cmd",
		"plugin-bin/bpx-darwin-amd64",
		"plugin-bin/bpx-darwin-arm64",
		"plugin-bin/bpx-linux-amd64",
		"plugin-bin/bpx-linux-arm64",
		"plugin-bin/bpx-windows-amd64.exe",
		"plugin-bin/bpx-windows-arm64.exe",
	})

	assertSkillNamesMatch(t, filepath.Join(repoRoot, "skills"), filepath.Join(pluginRoot, "skills"))
}

func assertRequiredFiles(t *testing.T, root string, relPaths []string) {
	t.Helper()
	for _, rel := range relPaths {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("required plugin file %s: %v", rel, err)
		}
		if info.Size() == 0 {
			t.Fatalf("required plugin file is empty: %s", rel)
		}
	}
}

func assertSkillNamesMatch(t *testing.T, rootSkillsDir, pluginSkillsDir string) {
	t.Helper()
	rootSkills := skillDirNames(t, rootSkillsDir)
	pluginSkills := skillDirNames(t, pluginSkillsDir)
	if strings.Join(rootSkills, "\n") != strings.Join(pluginSkills, "\n") {
		t.Fatalf("plugin skill directories differ from root skills\nroot=%v\nplugin=%v", rootSkills, pluginSkills)
	}
}

func skillDirNames(t *testing.T, skillsDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		t.Fatalf("read skills dir %s: %v", skillsDir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(skillsDir, entry.Name(), "SKILL.md")); err != nil {
			t.Fatalf("skill %s missing SKILL.md: %v", entry.Name(), err)
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}
