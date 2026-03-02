package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestNormalizeFlagArgsAllowsFileThenFlag(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	index := fs.Int("index", 0, "")
	_ = index
	args := normalizeFlagArgs(fs, []string{"sample.uasset", "--index", "3"})

	if got, want := strings.Join(args, " "), "--index 3 sample.uasset"; got != want {
		t.Fatalf("normalize args: got %q want %q", got, want)
	}
}

func TestNormalizeFlagArgsBoolFlagDoesNotConsumeFile(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	recursive := fs.Bool("recursive", false, "")
	_ = recursive
	args := normalizeFlagArgs(fs, []string{"sample.uasset", "--recursive"})

	if got, want := strings.Join(args, " "), "--recursive sample.uasset"; got != want {
		t.Fatalf("normalize args: got %q want %q", got, want)
	}
}

func TestNormalizeFlagArgsBoolFlagConsumesExplicitLiteral(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	recursive := fs.Bool("recursive", false, "")
	_ = recursive
	args := normalizeFlagArgs(fs, []string{"sample.uasset", "--recursive", "false"})

	if got, want := strings.Join(args, " "), "--recursive false sample.uasset"; got != want {
		t.Fatalf("normalize args: got %q want %q", got, want)
	}
}

func TestNormalizeFlagArgsUnknownFlagStaysFlagToken(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	index := fs.Int("index", 0, "")
	_ = index
	args := normalizeFlagArgs(fs, []string{"sample.uasset", "--typo", "--index", "3"})

	if got, want := strings.Join(args, " "), "--typo --index 3 sample.uasset"; got != want {
		t.Fatalf("normalize args: got %q want %q", got, want)
	}
}

func TestRegisterCommonFlagsKeepUnknownDefaultsToTrue(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	opts := registerCommonFlags(fs)
	if err := parseFlagSet(fs, []string{"sample.uasset"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if !opts.KeepUnknown {
		t.Fatalf("keep-unknown: got %v want true", opts.KeepUnknown)
	}
}

func TestRegisterCommonFlagsRejectsKeepUnknownFlag(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	_ = registerCommonFlags(fs)
	if err := parseFlagSet(fs, []string{"sample.uasset", "--keep-unknown=false"}); err == nil {
		t.Fatalf("expected parse error for removed --keep-unknown flag")
	}
}

func TestRegisterCommonFlagsRejectsStrictFlag(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	_ = registerCommonFlags(fs)
	if err := parseFlagSet(fs, []string{"sample.uasset", "--strict=false"}); err == nil {
		t.Fatalf("expected parse error for removed --strict flag")
	}
}

func TestRunExportShowIsRejected(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"export", "show", "/path/does/not/exist.uasset", "--index", "1"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if !strings.Contains(stderr.String(), "unknown export command: show") {
		t.Fatalf("expected unknown export command, got: %s", stderr.String())
	}
}

func TestRunExportInfoAcceptsExportFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"export", "info", "/path/does/not/exist.uasset", "--export", "1"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx export info") {
		t.Fatalf("unexpected usage error, export flag likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunRawAcceptsFlatForm(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"raw", "/path/does/not/exist.uasset", "--export", "1"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx raw") {
		t.Fatalf("unexpected usage error, export flag likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunRawDataAliasIsRejected(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"raw", "data", "/path/does/not/exist.uasset", "--export", "1"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: bpx raw <file.uasset> --export <n>") {
		t.Fatalf("expected canonical raw usage error, got: %s", stderr.String())
	}
}

func TestRunScanSummaryAliasIsRejected(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"scan", "summary", "/tmp"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if !strings.Contains(stderr.String(), "unknown command: scan") {
		t.Fatalf("expected unknown scan command, got: %s", stderr.String())
	}
}

func TestRunVersionPrintsSemanticVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	if got, want := strings.TrimSpace(stdout.String()), "0.1.2"; got != want {
		t.Fatalf("version: got %q want %q", got, want)
	}
}

func TestRunVersionAliasPrintsSemanticVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"--version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	if got, want := strings.TrimSpace(stdout.String()), "0.1.2"; got != want {
		t.Fatalf("version alias: got %q want %q", got, want)
	}
}

func TestRunHelpRootListsHelpAndWriteCommands(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "bpx help [command]") {
		t.Fatalf("expected help usage line, got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "bpx version") {
		t.Fatalf("expected version usage in root help, got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "bpx export set-header") {
		t.Fatalf("expected write command usage in root help, got: %s", stdout.String())
	}
}

func TestRunHelpTopicShowsMetadataCommands(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"help", "metadata"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "BPX help: metadata") {
		t.Fatalf("expected topic header, got: %s", out)
	}
	if !strings.Contains(out, "bpx metadata <file.uasset> --export <n>") {
		t.Fatalf("expected metadata read usage, got: %s", out)
	}
	if !strings.Contains(out, "bpx metadata set-root") || !strings.Contains(out, "bpx metadata set-object") {
		t.Fatalf("expected metadata write usage, got: %s", out)
	}
}

func TestRunHelpFindShowsPatternOnSummary(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"help", "find"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "bpx find summary <directory> [--pattern \"*.uasset\"]") {
		t.Fatalf("expected find summary pattern usage, got: %s", out)
	}
}

func TestRunHelpImportShowsPatternOnGraph(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"help", "import"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "bpx import graph <directory> [--pattern \"*.uasset\"]") {
		t.Fatalf("expected import graph pattern usage, got: %s", out)
	}
}

func TestRunHelpVersionShowsVersionCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"help", "version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "BPX help: version") {
		t.Fatalf("expected version topic header, got: %s", out)
	}
	if !strings.Contains(out, "bpx version") {
		t.Fatalf("expected version usage line, got: %s", out)
	}
}

func TestRunCommandHelpAliasShowsTopicHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"localization", "help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "BPX help: localization") {
		t.Fatalf("expected localization topic help, got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "bpx localization set-stringtable-ref") {
		t.Fatalf("expected localization write usage, got: %s", stdout.String())
	}
}

func TestRunHelpUnknownTopicFails(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"help", "no-such-topic"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if !strings.Contains(stderr.String(), "unknown help topic: no-such-topic") {
		t.Fatalf("expected unknown help topic error, got: %s", stderr.String())
	}
}

func TestRunClassInfoAliasIsRejected(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"class", "info", "/path/does/not/exist.uasset", "--export", "1"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: bpx class <file.uasset> --export <n>") {
		t.Fatalf("expected canonical class usage error, got: %s", stderr.String())
	}
}

func TestRunLevelVarListAcceptsActorFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"level", "var-list", "/tmp/nonexistent.umap", "--actor", "PlacedActor"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx level var-list") {
		t.Fatalf("unexpected usage error, actor flag likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunLevelVarSetAcceptsFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"level", "var-set", "/tmp/nonexistent.umap", "--actor", "PlacedActor", "--path", "MyInt", "--value", "1"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx level var-set") {
		t.Fatalf("unexpected usage error, flags likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunLevelInfoAcceptsFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"level", "info", "/tmp/nonexistent.umap", "--export", "1"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx level info") {
		t.Fatalf("unexpected usage error, flags likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunLevelLegacyFormsAreRejected(t *testing.T) {
	tests := []struct {
		name string
		argv []string
	}{
		{
			name: "legacy direct form",
			argv: []string{"level", "/tmp/nonexistent.umap", "--export", "1"},
		},
		{
			name: "legacy vars form",
			argv: []string{"level", "vars", "/tmp/nonexistent.umap", "--actor", "PlacedActor"},
		},
		{
			name: "legacy set-var form",
			argv: []string{"level", "set-var", "/tmp/nonexistent.umap", "--actor", "PlacedActor", "--path", "MyInt", "--value", "1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(tt.argv, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("expected legacy syntax to fail")
			}
			if !strings.Contains(stderr.String(), "unknown level command") {
				t.Fatalf("expected unknown level command error, got: %s", stderr.String())
			}
		})
	}
}

func TestRunMetadataGetAliasIsRejected(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"metadata", "get", "/path/does/not/exist.uasset", "--export", "1"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: bpx metadata <file.uasset> --export <n>") {
		t.Fatalf("expected canonical metadata usage error, got: %s", stderr.String())
	}
}

func TestRunBlueprintDisasmAcceptsExportFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"blueprint", "disasm", "/path/does/not/exist.uasset", "--export", "1"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx blueprint disasm") {
		t.Fatalf("unexpected usage error, export flag likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunBlueprintDisasmRejectsUnsupportedFormat(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"blueprint", "disasm", "/tmp/nonexistent.uasset", "--export", "1", "--format", "xml"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if !strings.Contains(stderr.String(), "unsupported format: xml") {
		t.Fatalf("expected unsupported format error, got: %s", stderr.String())
	}
}

func TestRunInfoSupportsTOMLFormat(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "golden", "parse", "BP_Empty.uasset")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"info", fixture, "--format", "toml"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 (stderr=%s)", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	var payload map[string]any
	if err := toml.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("parse toml output: %v\n%s", err, stdout.String())
	}
	if got, _ := payload["file"].(string); got != fixture {
		t.Fatalf("unexpected file in toml output: got %q want %q", got, fixture)
	}
	if _, ok := payload["assetKind"]; !ok {
		t.Fatalf("assetKind key missing in toml output")
	}
}

func TestRunBlueprintDisasmDefaultFormatIsJSON(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "golden", "parse", "BP_WithFunctions.uasset")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"blueprint", "disasm", fixture, "--export", "5"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 (stderr=%s)", code, stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("expected json output by default, decode failed: %v\n%s", err, stdout.String())
	}
	if got, _ := payload["export"].(float64); int(got) != 5 {
		t.Fatalf("unexpected export value in disasm output: %v", payload["export"])
	}
}

func TestRunBlueprintDisasmAcceptsAnalysisFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"blueprint", "disasm", "/tmp/nonexistent.uasset", "--export", "1", "--analysis", "--entrypoint", "1234", "--max-steps", "100"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx blueprint disasm") {
		t.Fatalf("unexpected usage error, analysis flags likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunBlueprintInferPackAcceptsExportFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"blueprint", "infer-pack", "/tmp/nonexistent.uasset", "--export", "1"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx blueprint infer-pack") {
		t.Fatalf("unexpected usage error, infer-pack flags likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunBlueprintTraceAcceptsFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"blueprint", "trace", "/tmp/nonexistent.uasset", "--from", "K2Node_Event_0", "--to-function", "OpenLevelBySoftObjectPtr"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx blueprint trace") {
		t.Fatalf("unexpected usage error, trace flags likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunBlueprintCallArgsAcceptsFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"blueprint", "call-args", "/tmp/nonexistent.uasset", "--member", "OpenLevelBySoftObjectPtr"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx blueprint call-args") {
		t.Fatalf("unexpected usage error, call-args flags likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunBlueprintRefsAcceptsFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"blueprint", "refs", "/tmp/nonexistent.uasset", "--soft-path", "/Game/Lyra/Maps/L_Default"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx blueprint refs") {
		t.Fatalf("unexpected usage error, refs flags likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunBlueprintSearchAcceptsFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"blueprint", "search", "/tmp/nonexistent.uasset", "--class", "K2Node_CallFunction", "--member", "OpenLevelBySoftObjectPtr", "--show", "NodePos,Function,PinDefaults"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx blueprint search") {
		t.Fatalf("unexpected usage error, search flags likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunPropListRejectsRawFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"prop", "list", "/tmp/nonexistent.uasset", "--export", "1", "--raw"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Fatalf("expected unknown flag error for --raw, got: %s", stderr.String())
	}
}

func TestRunPropAddAcceptsSpecFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"prop", "add", "/tmp/nonexistent.uasset", "--export", "1", "--spec", `{"name":"FixtureValue","type":"IntProperty","value":1}`}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx prop add") {
		t.Fatalf("unexpected usage error, spec flag likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunPropRemoveAcceptsPathFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"prop", "remove", "/tmp/nonexistent.uasset", "--export", "1", "--path", "FixtureValue"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx prop remove") {
		t.Fatalf("unexpected usage error, path flag likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunRejectsUnknownFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"info", "--typo", "/path/does/not/exist.uasset"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Fatalf("expected unknown flag error, got: %s", stderr.String())
	}
}

func TestRunLocalizationReadAcceptsExportFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"localization", "read", "/tmp/nonexistent.uasset", "--export", "1"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx localization read") {
		t.Fatalf("unexpected usage error, export flag likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunDataTableUpdateRowAcceptsFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"datatable", "update-row", "/tmp/nonexistent.uasset", "--row", "Row_A", "--values", `{"Score":999}`}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx datatable update-row") {
		t.Fatalf("unexpected usage error, flags likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunDataTableAddRowAcceptsFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"datatable", "add-row", "/tmp/nonexistent.uasset", "--row", "Row_A_1", "--values", `{"Score":999}`}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx datatable add-row") {
		t.Fatalf("unexpected usage error, flags likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunDataTableRemoveRowAcceptsFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"datatable", "remove-row", "/tmp/nonexistent.uasset", "--row", "Row_A"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx datatable remove-row") {
		t.Fatalf("unexpected usage error, flags likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunExportSetHeaderAcceptsFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"export", "set-header", "/tmp/nonexistent.uasset", "--index", "1", "--fields", `{"objectFlags":1}`}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx export set-header") {
		t.Fatalf("unexpected usage error, flags likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunPackageSetFlagsAcceptsFlagsArg(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"package", "set-flags", "/tmp/nonexistent.uasset", "--flags", "PKG_FilterEditorOnly"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx package set-flags") {
		t.Fatalf("unexpected usage error, flags likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunMetadataSetRootAcceptsFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"metadata", "set-root", "/tmp/nonexistent.uasset", "--export", "1", "--key", "ToolTip", "--value", "Updated"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx metadata set-root") {
		t.Fatalf("unexpected usage error, flags likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunMetadataSetObjectAcceptsFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"metadata", "set-object", "/tmp/nonexistent.uasset", "--export", "1", "--import", "2", "--key", "ToolTip", "--value", "Updated"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx metadata set-object") {
		t.Fatalf("unexpected usage error, flags likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunEnumWriteValueAcceptsFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"enum", "write-value", "/tmp/nonexistent.uasset", "--export", "1", "--name", "Direction", "--value", "East"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx enum write-value") {
		t.Fatalf("unexpected usage error, flags likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunStringTableWriteEntryAcceptsFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"stringtable", "write-entry", "/tmp/nonexistent.uasset", "--export", "1", "--key", "BTN_OK", "--value", "OK"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx stringtable write-entry") {
		t.Fatalf("unexpected usage error, flags likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunLocalizationSetSourceAcceptsFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"localization", "set-source", "/tmp/nonexistent.uasset", "--export", "1", "--path", "MyText", "--value", "Updated"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx localization set-source") {
		t.Fatalf("unexpected usage error, flags likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunLocalizationSetIDAcceptsFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"localization", "set-id", "/tmp/nonexistent.uasset", "--export", "1", "--path", "MyText", "--namespace", "UI", "--key", "BTN_OK"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx localization set-id") {
		t.Fatalf("unexpected usage error, flags likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunLocalizationSetStringTableRefAcceptsFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"localization", "set-stringtable-ref", "/tmp/nonexistent.uasset", "--export", "1", "--path", "MyText", "--table", "UI", "--key", "BTN_OK"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if strings.Contains(stderr.String(), "usage: bpx localization set-stringtable-ref") {
		t.Fatalf("unexpected usage error, flags likely not parsed: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "read file") {
		t.Fatalf("expected file read failure, got: %s", stderr.String())
	}
}

func TestRunLocalizationResolveRequiresCulture(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"localization", "resolve", "/tmp/nonexistent.uasset"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: bpx localization resolve") {
		t.Fatalf("expected usage error, got: %s", stderr.String())
	}
}

type failWriter struct{}

func (failWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestPrintJSONWritesEncodeError(t *testing.T) {
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	defer func() {
		_ = w.Close()
		os.Stderr = oldStderr
	}()

	code := printJSON(failWriter{}, map[string]any{"k": "v"})
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}

	_ = w.Close()
	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if !strings.Contains(out.String(), "error: encode json output:") {
		t.Fatalf("expected encode error on stderr, got: %s", out.String())
	}
}

func TestRunDumpRejectsOutPathSameAsInput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"dump", "/tmp/nonexistent.uasset", "--out", "/tmp/nonexistent.uasset"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to overwrite input file") {
		t.Fatalf("expected overwrite guard error, got: %s", stderr.String())
	}
}

func TestRunDataTableReadRejectsOutPathSameAsInput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"datatable", "read", "/tmp/nonexistent.uasset", "--out", "/tmp/nonexistent.uasset"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to overwrite input file") {
		t.Fatalf("expected overwrite guard error, got: %s", stderr.String())
	}
}
