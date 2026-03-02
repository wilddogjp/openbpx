package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/wilddogjp/bpx/pkg/edit"
	"github.com/wilddogjp/bpx/pkg/uasset"
)

func TestRunNameAddWritesEntryAndHashes(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "BP_Empty.uasset")
	orig, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "parse", "BP_Empty.uasset"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(assetPath, orig, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"name", "add", assetPath, "--value", "BPX_CommandAdded"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(assetPath, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		t.Fatalf("parse rewritten fixture: %v", err)
	}
	last := asset.Names[len(asset.Names)-1]
	if got, want := last.Value, "BPX_CommandAdded"; got != want {
		t.Fatalf("last name value: got %q want %q", got, want)
	}
	nonCase, caseHash := edit.ComputeNameEntryHashesUE56("BPX_CommandAdded")
	if got, want := last.NonCaseHash, nonCase; got != want {
		t.Fatalf("nonCaseHash: got %d want %d", got, want)
	}
	if got, want := last.CasePreservingHash, caseHash; got != want {
		t.Fatalf("casePreservingHash: got %d want %d", got, want)
	}

	var resp map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("parse response json: %v", err)
	}
	if got, want := int(resp["newCount"].(float64))-int(resp["oldCount"].(float64)), 1; got != want {
		t.Fatalf("count delta: got %d want %d", got, want)
	}
}

func TestRunNameAddDryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "sample.uasset")
	orig := buildCLIFixture(t, "hello")
	if err := os.WriteFile(assetPath, orig, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"name", "add", assetPath, "--value", "BPX_DryRunOnly", "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"dryRun": true`) {
		t.Fatalf("dry-run response missing: %s", stdout.String())
	}
	after, err := os.ReadFile(assetPath)
	if err != nil {
		t.Fatalf("read fixture after run: %v", err)
	}
	if !bytes.Equal(orig, after) {
		t.Fatalf("dry-run modified source file")
	}
}

func TestRunNameSetWritesEntry(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "sample.uasset")
	orig := buildCLIFixture(t, "hello")
	if err := os.WriteFile(assetPath, orig, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"name", "set", assetPath, "--index", "1", "--value", "Default__BP_Renamed"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%s", code, stderr.String())
	}

	asset, err := uasset.ParseFile(assetPath, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		t.Fatalf("parse rewritten fixture: %v", err)
	}
	if got, want := asset.Names[1].Value, "Default__BP_Renamed"; got != want {
		t.Fatalf("name[1]: got %q want %q", got, want)
	}
}

func TestRunNameRemoveTailEntryWritesFile(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "sample.uasset")
	orig := buildCLIFixtureWithUnusedTailNames(t, []string{"UnusedTailA", "UnusedTailB"})
	if err := os.WriteFile(assetPath, orig, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	assetBefore, err := uasset.ParseFile(assetPath, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		t.Fatalf("parse original fixture: %v", err)
	}
	removeIndex := len(assetBefore.Names) - 1

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"name", "remove", assetPath, "--index", strconv.Itoa(removeIndex)}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%s", code, stderr.String())
	}

	assetAfter, err := uasset.ParseFile(assetPath, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		t.Fatalf("parse rewritten fixture: %v", err)
	}
	if got, want := len(assetAfter.Names), len(assetBefore.Names)-1; got != want {
		t.Fatalf("name count: got %d want %d", got, want)
	}
	if got, want := assetAfter.Names[len(assetAfter.Names)-1].Value, "UnusedTailA"; got != want {
		t.Fatalf("last name after remove: got %q want %q", got, want)
	}
}

func TestRunNameRemoveRejectsNonTailIndex(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "sample.uasset")
	orig := buildCLIFixtureWithUnusedTailNames(t, []string{"UnusedTailA", "UnusedTailB"})
	if err := os.WriteFile(assetPath, orig, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"name", "remove", assetPath, "--index", "1"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected command to fail")
	}
	if !strings.Contains(stderr.String(), "tail entry only") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRunNameRemoveRejectsExportDataRegion(t *testing.T) {
	dir := t.TempDir()
	assetPath := filepath.Join(dir, "sample.uasset")
	orig := buildCLIFixture(t, "hello")
	if err := os.WriteFile(assetPath, orig, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	asset, err := uasset.ParseFile(assetPath, uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	removeIndex := len(asset.Names) - 1

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"name", "remove", assetPath, "--index", strconv.Itoa(removeIndex)}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected command to fail")
	}
	if !strings.Contains(stderr.String(), "NamesReferencedFromExportData") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func buildCLIFixtureWithUnusedTailNames(t *testing.T, tailNames []string) []byte {
	t.Helper()

	names := []string{
		"None",
		"Default__BP_Test",
		"ObjectProperty",
		"MyProp",
		"BlueprintGeneratedClass",
		"CoreUObject",
		"StrProperty",
		"MyStr",
	}
	names = append(names, tailNames...)

	nameMap := buildCLINameMap(t, names)
	importMap := buildCLIImportMap(t)
	payload := buildCLIStringPropertyPayload(t, 7, 6, "hello")
	exports := []cliExport{{objectNameIndex: 1, payload: payload}}

	summaryTemplate := buildCLISummary(t, cliSummaryArgs{
		NameOffset:           0,
		ImportOffset:         0,
		ExportOffset:         0,
		TotalHeaderSize:      0,
		BulkDataStartOffset:  0,
		EngineMinor:          6,
		NameCount:            int32(len(names)),
		ImportCount:          1,
		ExportCount:          1,
		NamesReferencedCount: 8,
	})
	exportMapTemplate := buildCLIExportMap(t, exports, 0)

	summarySize := len(summaryTemplate)
	nameOffset := int32(summarySize)
	importOffset := int32(summarySize + len(nameMap))
	exportOffset := int32(summarySize + len(nameMap) + len(importMap))
	totalHeader := int32(summarySize + len(nameMap) + len(importMap) + len(exportMapTemplate))
	serialBase := int64(totalHeader)
	exportMap := buildCLIExportMap(t, exports, serialBase)
	bulkStart := serialBase + int64(len(payload))

	summary := buildCLISummary(t, cliSummaryArgs{
		NameOffset:           nameOffset,
		ImportOffset:         importOffset,
		ExportOffset:         exportOffset,
		TotalHeaderSize:      totalHeader,
		BulkDataStartOffset:  bulkStart,
		EngineMinor:          6,
		NameCount:            int32(len(names)),
		ImportCount:          1,
		ExportCount:          1,
		NamesReferencedCount: 8,
	})

	var out bytes.Buffer
	out.Write(summary)
	out.Write(nameMap)
	out.Write(importMap)
	out.Write(exportMap)
	out.Write(payload)
	out.Write([]byte("TAIL"))
	return out.Bytes()
}
