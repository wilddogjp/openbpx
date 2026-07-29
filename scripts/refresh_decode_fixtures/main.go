// Command refresh_decode_fixtures rewrites the recorded `expected` payload of
// specific expected-output fixtures after an intentional decode-coverage change,
// then refreshes their manifest entries. Run after an intentional decode-coverage change; see ZVT_RESEARCH_AND_PLAN.md.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wilddogjp/openbpx/internal/cli"
)

type manifestEntry struct {
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	SizeBytes int    `json:"size_bytes"`
}

type manifestFile struct {
	Version   int             `json:"version"`
	Generated string          `json:"generated"`
	Fixtures  []manifestEntry `json:"fixtures"`
}

func main() {
	names := []string{
		"blueprint_info_BP_WithFunctions.json",
		"metadata_BP_WithMetadata_export1.json",
		"prop_list_BP_AllScalarTypes_export1.json",
		"prop_list_BP_Containers_export1.json",
		"prop_list_BP_Empty_export1.json",
		"prop_list_BP_GameplayTags_export1.json",
		"prop_list_BP_MathTypes_export1.json",
		"prop_list_BP_Nested_export1.json",
		"prop_list_BP_RefTypes_export1.json",
		"prop_list_BP_SoftRefs_export1.json",
		"prop_list_BP_WithFunctions_export1.json",
		"prop_list_S_PlayerData_export2.json",
		"struct_definition_S_PlayerData.json",
		"struct_details_S_PlayerData_export2.json",
	}
	targets := make([]string, 0, len(names)*2)
	for _, root := range []string{"ue5.6", "ue5.7"} {
		for _, name := range names {
			targets = append(targets, "testdata/golden/"+root+"/expected_output/"+name)
		}
	}

	for _, target := range targets {
		if err := refresh(target); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", target, err)
			os.Exit(1)
		}
		fmt.Println("refreshed", target)
	}

	if err := refreshManifest("testdata/manifest.json", targets); err != nil {
		fmt.Fprintf(os.Stderr, "error: manifest: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("manifest updated")
}

const canonicalGoldenRoot = "testdata/golden"

// engineRootFor returns "testdata/golden/ue5.7" for a fixture under that root.
func engineRootFor(target string) string {
	parts := strings.Split(filepath.ToSlash(target), "/")
	if len(parts) < 3 {
		return canonicalGoldenRoot
	}
	return strings.Join(parts[:3], "/")
}

func refresh(target string) error {
	payload, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	var fixture map[string]any
	if err := json.Unmarshal(payload, &fixture); err != nil {
		return err
	}
	rawArgv, ok := fixture["argv"].([]any)
	if !ok {
		return fmt.Errorf("fixture has no argv")
	}
	// Fixtures store canonical "testdata/golden" paths; the accuracy test rebases
	// them onto the per-engine root before running. Mirror that here, then rewrite
	// the captured output back to canonical form.
	root := engineRootFor(target)
	argv := make([]string, 0, len(rawArgv))
	for _, item := range rawArgv {
		argv = append(argv, strings.ReplaceAll(fmt.Sprint(item), canonicalGoldenRoot, root))
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := cli.Run(argv, &stdout, &stderr); code != 0 {
		return fmt.Errorf("command exit %d: %s", code, stderr.String())
	}
	canonical := strings.ReplaceAll(stdout.String(), root, canonicalGoldenRoot)
	var expected any
	if err := json.Unmarshal([]byte(canonical), &expected); err != nil {
		return err
	}
	fixture["expected"] = expected

	body, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(target, body, 0o644)
}

func refreshManifest(path string, targets []string) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var manifest manifestFile
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return err
	}

	updated := map[string]bool{}
	for _, target := range targets {
		relative := strings.TrimPrefix(filepath.ToSlash(target), "testdata/")
		body, err := os.ReadFile(target)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(body)
		found := false
		for i := range manifest.Fixtures {
			if manifest.Fixtures[i].Path != relative {
				continue
			}
			manifest.Fixtures[i].SHA256 = hex.EncodeToString(sum[:])
			manifest.Fixtures[i].SizeBytes = len(body)
			found = true
			break
		}
		if !found {
			return fmt.Errorf("manifest entry not found: %s", relative)
		}
		updated[relative] = true
	}

	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o644)
}
