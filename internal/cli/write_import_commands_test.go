package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

func TestRunImportAddTextureDryRun(t *testing.T) {
	file := filepath.Join(t.TempDir(), "BP_Test.uasset")
	if err := os.WriteFile(file, buildCLIFixture(t, "hello"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"import", "add", file,
		"--texture", "/Game/Effects/Textures/General/blurry_texture",
		"--dry-run",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if changed, _ := resp["changed"].(bool); !changed {
		t.Fatalf("changed: got false want true")
	}
	addedImports, _ := resp["addedImports"].([]any)
	if len(addedImports) != 2 {
		t.Fatalf("addedImports len: got %d want 2", len(addedImports))
	}

	asset, err := uasset.ParseFile(file, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse original file: %v", err)
	}
	if len(asset.Imports) != 1 {
		t.Fatalf("import count after dry-run: got %d want 1", len(asset.Imports))
	}
}

func TestRunImportAddTextureWritesImports(t *testing.T) {
	file := filepath.Join(t.TempDir(), "BP_Test.uasset")
	if err := os.WriteFile(file, buildCLIFixture(t, "hello"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"import", "add", file,
		"--texture", "/Game/Effects/Textures/General/blurry_texture",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(file, uasset.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse rewritten file: %v", err)
	}
	if len(asset.Imports) != 3 {
		t.Fatalf("import count: got %d want 3", len(asset.Imports))
	}
	packageSeen := false
	textureSeen := false
	for i := range asset.Imports {
		className := asset.Imports[i].ClassName.Display(asset.Names)
		target := resolveImportTargetPath(asset, asset.Imports[i])
		if className == "Package" && target == "/Game/Effects/Textures/General/blurry_texture" {
			packageSeen = true
		}
		if className == "Texture2D" && target == "/Game/Effects/Textures/General/blurry_texture" {
			textureSeen = true
		}
	}
	if !packageSeen {
		t.Fatalf("package import for blurry_texture not found")
	}
	if !textureSeen {
		t.Fatalf("texture import for blurry_texture not found")
	}
}

func TestRunImportAddTextureNoOpWhenAlreadyPresent(t *testing.T) {
	file := filepath.Join(t.TempDir(), "BP_Test.uasset")
	if err := os.WriteFile(file, buildCLIFixture(t, "hello"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"import", "add", file,
		"--texture", "/Game/Effects/Textures/General/blurry_texture",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("first add exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{
		"import", "add", file,
		"--texture", "/Game/Effects/Textures/General/blurry_texture",
		"--dry-run",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("second add exit code: got %d want 0 stderr=%s", code, stderr.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if changed, _ := resp["changed"].(bool); changed {
		t.Fatalf("changed: got true want false")
	}
	addedImports, _ := resp["addedImports"].([]any)
	if len(addedImports) != 1 {
		t.Fatalf("addedImports len: got %d want 1", len(addedImports))
	}
	firstImport, _ := addedImports[0].(map[string]any)
	if created, _ := firstImport["created"].(bool); created {
		t.Fatalf("created: got true want false")
	}
}
