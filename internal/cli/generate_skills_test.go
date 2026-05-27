package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type generateSkillsResponse struct {
	OutputDir      string   `json:"outputDir"`
	Filter         string   `json:"filter"`
	GeneratedCount int      `json:"generatedCount"`
	Skills         []string `json:"skills"`
}

func TestRunGenerateSkillsWritesSkillFiles(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "skills-out")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"generate-skills", "--output-dir", outDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 (stderr=%s)", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}

	var resp generateSkillsResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("parse json output: %v\n%s", err, stdout.String())
	}
	if resp.GeneratedCount == 0 {
		t.Fatalf("generatedCount must be > 0")
	}
	if resp.OutputDir != filepath.Clean(outDir) {
		t.Fatalf("outputDir: got %q want %q", resp.OutputDir, filepath.Clean(outDir))
	}

	requiredSkills := []string{"bpx-shared", "bpx-prop", "bpx-blueprint", "bpx-widget", "bpx-validate"}
	for _, skill := range requiredSkills {
		if _, err := os.Stat(filepath.Join(outDir, skill, "SKILL.md")); err != nil {
			t.Fatalf("missing generated file for %s: %v", skill, err)
		}
	}

	if _, err := os.Stat(filepath.Join(outDir, "bpx-generate-skills", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("bpx-generate-skills must not be generated")
	}

	propSkillPath := filepath.Join(outDir, "bpx-prop", "SKILL.md")
	propBody, err := os.ReadFile(propSkillPath)
	if err != nil {
		t.Fatalf("read generated skill: %v", err)
	}
	propText := string(propBody)
	if !strings.Contains(propText, "name: bpx-prop") {
		t.Fatalf("missing frontmatter name in generated prop skill")
	}
	if !strings.Contains(propText, "bpx prop list <file.uasset> --export <n>") {
		t.Fatalf("missing usage line in generated prop skill")
	}
	if !strings.Contains(propText, "## Behavior") {
		t.Fatalf("missing behavior section in generated prop skill")
	}
	if !strings.Contains(propText, "This command includes write-capable operations.") {
		t.Fatalf("missing write caution in generated prop skill")
	}

	blueprintSkillPath := filepath.Join(outDir, "bpx-blueprint", "SKILL.md")
	blueprintBody, err := os.ReadFile(blueprintSkillPath)
	if err != nil {
		t.Fatalf("read generated blueprint skill: %v", err)
	}
	blueprintText := string(blueprintBody)
	if !strings.Contains(blueprintText, "## Command Matrix") {
		t.Fatalf("missing embedded supplement section in generated blueprint skill")
	}
	if strings.Contains(blueprintText, "bpx blueprint widget-init <out.uasset> --template <minimum>") {
		t.Fatalf("widget-init usage should be routed out of generated blueprint skill")
	}
	if strings.Contains(blueprintText, "bpx blueprint widget-write <file.uasset> --widget <path|name>") {
		t.Fatalf("widget-write usage should be routed out of generated blueprint skill")
	}
	if !strings.Contains(blueprintText, "## Recommended Workflow") {
		t.Fatalf("missing embedded workflow section in generated blueprint skill")
	}
	if !strings.Contains(blueprintText, "## Worked Recipe") {
		t.Fatalf("missing worked recipe section in generated blueprint skill")
	}
	if !strings.Contains(blueprintText, "bpx blueprint info ./Content/BP_Player.uasset") {
		t.Fatalf("missing analysis recipe in generated blueprint skill")
	}
	if !strings.Contains(blueprintText, "For WidgetBlueprint construction and edit workflows, switch to [bpx-widget](../bpx-widget/SKILL.md).") {
		t.Fatalf("missing widget skill routing guidance in generated blueprint skill")
	}
	if !strings.Contains(blueprintText, "If your goal is WidgetBlueprint construction or widget property mutation, use [bpx-widget](../bpx-widget/SKILL.md)") {
		t.Fatalf("missing widget split guidance in generated blueprint skill")
	}
	if !strings.Contains(blueprintText, "bpx blueprint search ./Content/BP_Player.uasset --member ApplyDamage --show name,class,member") {
		t.Fatalf("missing blueprint analysis example in generated blueprint skill")
	}
	if !strings.Contains(blueprintText, "For WidgetBlueprint edit flows and UE-openability caveats, prefer [bpx-widget](../bpx-widget/SKILL.md)") {
		t.Fatalf("missing blueprint caveat redirect in generated blueprint skill")
	}
	if strings.Contains(blueprintText, "This command includes write-capable operations.") {
		t.Fatalf("generated blueprint skill should not advertise write caution after widget split")
	}
	if !strings.Contains(blueprintText, "## Code-Aligned Caveats") {
		t.Fatalf("missing embedded supplement caveats in generated blueprint skill")
	}

	widgetSkillPath := filepath.Join(outDir, "bpx-widget", "SKILL.md")
	widgetBody, err := os.ReadFile(widgetSkillPath)
	if err != nil {
		t.Fatalf("read generated widget skill: %v", err)
	}
	widgetText := string(widgetBody)
	if !strings.Contains(widgetText, "name: bpx-widget") {
		t.Fatalf("missing frontmatter name in generated widget skill")
	}
	if !strings.Contains(widgetText, "bpx blueprint widget-init <out.uasset> --template <minimum>") {
		t.Fatalf("missing widget-init usage in generated widget skill")
	}
	if !strings.Contains(widgetText, "bpx blueprint widget-parent-class <file.uasset> --class </Script/Module.ClassName>") {
		t.Fatalf("missing widget-parent-class usage in generated widget skill")
	}
	if !strings.Contains(widgetText, "bpx blueprint widget-add <file.uasset> --parent <path|name|root> --type <image|textblock|richtextblock|progressbar|slider|spacer|scrollbar|editabletext|editabletextbox|multilineeditabletextbox|spinbox|comboboxstring|checkbox|userwidget|button|border") || !strings.Contains(widgetText, "|menuanchor|namedslot|sizebox|scalebox|backgroundblur|safezone|windowtitlebararea|canvaspanel|overlay|verticalbox|horizontalbox|stackbox|scrollbox|wrapbox|gridpanel|uniformgridpanel|widgetswitcher|listview|tileview|treeview>") {
		t.Fatalf("missing current widget-add type coverage in generated widget skill")
	}
	if !strings.Contains(widgetText, "bpx blueprint widget-write <file.uasset> --widget <path|name> --property <text|visibility|render-opacity|brush-image|progressbar-percent|progressbar-fill-color|slider-value") || !strings.Contains(widgetText, "editabletextbox-hint-text") || !strings.Contains(widgetText, "checkbox-checked-state") || !strings.Contains(widgetText, "listview-entry-widget-class") || !strings.Contains(widgetText, "richtext-auto-wrap-text") {
		t.Fatalf("missing current widget-write property coverage in generated widget skill")
	}
	if !strings.Contains(widgetText, "this is the default starting point for unspecified new-widget requests") {
		t.Fatalf("missing default new-widget routing guidance in generated widget skill")
	}
	if !strings.Contains(widgetText, "## Supported Widgets") {
		t.Fatalf("missing supported widgets section in generated widget skill")
	}
	if !strings.Contains(widgetText, "## Supported Writes") {
		t.Fatalf("missing supported writes section in generated widget skill")
	}
	if !strings.Contains(widgetText, "## Known Gaps") {
		t.Fatalf("missing known gaps section in generated widget skill")
	}
	if !strings.Contains(widgetText, "Parent-class rewrites currently accept compiled `/Script/...` classes, including project/plugin module classes such as `/Script/LyraGame.LyraActivatableWidget`.") {
		t.Fatalf("missing compiled parent-class scope guidance in generated widget skill")
	}
	if !strings.Contains(widgetText, "`TextBlock`: `text-color(-and-opacity)`, `text-font`, `text-font-family`, `text-typeface`, `text-font-size`, `text-justification`, `text-auto-wrap-text`, `text-wrap-text-at`, `text-line-height-percentage`, `text-shadow-offset`, `text-shadow-color-and-opacity`, `text-outline-size`, `text-outline-color`.") {
		t.Fatalf("missing TextBlock write summary in generated widget skill")
	}
	if !strings.Contains(widgetText, "`EditableTextBox`: generic `text` plus `editabletextbox-hint-text`, `editabletextbox-is-read-only`, `editabletextbox-is-password`, `editabletextbox-minimum-desired-width`, `editabletextbox-justification`.") {
		t.Fatalf("missing EditableTextBox write summary in generated widget skill")
	}
	if !strings.Contains(widgetText, "`EditableText`: generic `text` plus `editabletext-hint-text`, `editabletext-is-read-only`, `editabletext-is-password`, `editabletext-minimum-desired-width`, `editabletext-justification`.") {
		t.Fatalf("missing EditableText write summary in generated widget skill")
	}
	if !strings.Contains(widgetText, "`MultiLineEditableTextBox`: generic `text` plus `multilineeditabletextbox-hint-text`, `multilineeditabletextbox-is-read-only`, `multilineeditabletextbox-justification`.") {
		t.Fatalf("missing MultiLineEditableTextBox write summary in generated widget skill")
	}
	if !strings.Contains(widgetText, "`SpinBox`: `spinbox-value`, `spinbox-min-value`, `spinbox-max-value`, `spinbox-delta`.") {
		t.Fatalf("missing SpinBox write summary in generated widget skill")
	}
	if !strings.Contains(widgetText, "`ComboBoxString`: `comboboxstring-selected-option`, `comboboxstring-options`, `comboboxstring-is-focusable`.") {
		t.Fatalf("missing ComboBoxString write summary in generated widget skill")
	}
	if !strings.Contains(widgetText, "`RichTextBlock`: `richtext-style-set`, `richtext-decorator-classes`") || !strings.Contains(widgetText, "`richtext-auto-wrap-text`") || !strings.Contains(widgetText, "`richtext-line-height-percentage`") {
		t.Fatalf("missing RichTextBlock write summary in generated widget skill")
	}
	if !strings.Contains(widgetText, "Broader `RichTextBlock` styling such as strike brushes, transform policy, and material-backed font overrides is not implemented yet.") {
		t.Fatalf("missing updated RichTextBlock known-gap summary in generated widget skill")
	}
	if !strings.Contains(widgetText, "Do not repurpose or mutate an existing WidgetBlueprint unless the user explicitly identifies that asset as the edit target.") {
		t.Fatalf("missing existing-widget mutation guard in generated widget skill")
	}
	if !strings.Contains(widgetText, "If the user asks to create a widget without explicitly naming an existing `.uasset` to edit, treat it as a new-widget request and start from `bpx blueprint widget-init ... --template minimum`.") {
		t.Fatalf("missing widget-init-first workflow rule in generated widget skill")
	}
	if !strings.Contains(widgetText, "A proven sequence is `root -> CanvasPanel -> Image1 -> Image2 -> Overlay -> Image3`.") {
		t.Fatalf("missing nested widget-build workflow guidance in generated widget skill")
	}
	if !strings.Contains(widgetText, "If the widget should inherit a compiled project/plugin class, set it before adding the root widget with `--parent-class /Script/...` or `widget-parent-class --class /Script/...`.") {
		t.Fatalf("missing parent-class-first workflow guidance in generated widget skill")
	}
	if !strings.Contains(widgetText, "Use `--type userwidget --class /Game/.../WBP_Name` when the user wants to compose with another WidgetBlueprint.") {
		t.Fatalf("missing userwidget workflow guidance in generated widget skill")
	}
	if !strings.Contains(widgetText, "testdata/golden/ue5.6/operations/widget_add_image_nested_overlay") {
		t.Fatalf("missing nested fixture reference in generated widget skill")
	}
	if !strings.Contains(widgetText, "bpx blueprint widget-add ./Content/WBP_NestedStatus.uasset --parent CanvasPanel_1/Overlay_1 --type image --name Image_3") {
		t.Fatalf("missing nested overlay worked recipe in generated widget skill")
	}
	if !strings.Contains(widgetText, "bpx blueprint widget-init ./Content/WBP_MenuRoot.uasset --template minimum --asset-name WBP_MenuRoot --package-path /Game/UI --parent-class /Script/LyraGame.LyraActivatableWidget") {
		t.Fatalf("missing parent-class worked recipe in generated widget skill")
	}
	if !strings.Contains(widgetText, "bpx blueprint widget-add ./Content/WBP_MenuRoot.uasset --parent Overlay_1 --type userwidget --class /Game/UI/WBP_Status --name WBP_Status_1") {
		t.Fatalf("missing userwidget worked recipe in generated widget skill")
	}
	if !strings.Contains(widgetText, "When the request is simply \"make a widget\" and no existing target asset is named, use this `widget-init`-first flow instead of editing a pre-existing WidgetBlueprint.") {
		t.Fatalf("missing widget-init-first recipe guidance in generated widget skill")
	}
	if !strings.Contains(widgetText, "The second recipe shows the safe parent-class-first flow for project/plugin widget bases plus child `userwidget` composition.") {
		t.Fatalf("missing parent-class recipe guidance in generated widget skill")
	}
	if !strings.Contains(widgetText, "Do not insert manual `name add` steps in the normal widget workflow.") {
		t.Fatalf("missing name-add avoidance guidance in generated widget skill")
	}
	if !strings.Contains(widgetText, "If a freshly built `widget-init` asset still reports missing NameMap/import-add names, treat that as a stale binary/build mismatch") {
		t.Fatalf("missing stale-binary troubleshooting guidance in generated widget skill")
	}
	if !strings.Contains(widgetText, "`Overlay_1` is real and valid there even though it does not become a generated class field.") {
		t.Fatalf("missing nested overlay generated-class caveat in generated widget skill")
	}
	if !strings.Contains(widgetText, "`widget-parent-class` is only safe before a root widget exists.") {
		t.Fatalf("missing widget-parent-class safety caveat in generated widget skill")
	}
	if !strings.Contains(widgetText, "`widget-add --type userwidget` expects a WidgetBlueprint asset path like `/Game/UI/WBP_Status`, not a generated-class path like `/Game/UI/WBP_Status_C`.") {
		t.Fatalf("missing userwidget class-path caveat in generated widget skill")
	}
	if !strings.Contains(widgetText, "`widget-remove` is conservative by design: it supports only non-root leaf widgets") {
		t.Fatalf("missing widget-remove safety caveat in generated widget skill")
	}
	if !strings.Contains(widgetText, "BPX structural `validate` is necessary but not sufficient for UE-openability on newly generated nested widgets.") {
		t.Fatalf("missing validate-vs-openability caveat in generated widget skill")
	}

	sharedSkillPath := filepath.Join(outDir, "bpx-shared", "SKILL.md")
	sharedBody, err := os.ReadFile(sharedSkillPath)
	if err != nil {
		t.Fatalf("read generated shared skill: %v", err)
	}
	sharedText := string(sharedBody)
	if !strings.Contains(sharedText, "## Global Rules") {
		t.Fatalf("missing embedded supplement section in generated shared skill")
	}
}

func TestRunGenerateSkillsFilter(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "skills-out")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"generate-skills", "--output-dir", outDir, "--filter", "blueprint"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 (stderr=%s)", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}

	var resp generateSkillsResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("parse json output: %v\n%s", err, stdout.String())
	}
	if resp.Filter != "blueprint" {
		t.Fatalf("filter: got %q want %q", resp.Filter, "blueprint")
	}
	if resp.GeneratedCount != 1 {
		t.Fatalf("generatedCount: got %d want 1", resp.GeneratedCount)
	}
	if len(resp.Skills) != 1 || resp.Skills[0] != "bpx-blueprint" {
		t.Fatalf("skills: got %#v want [\"bpx-blueprint\"]", resp.Skills)
	}
	if _, err := os.Stat(filepath.Join(outDir, "bpx-blueprint", "SKILL.md")); err != nil {
		t.Fatalf("missing filtered skill file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "bpx-shared", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("bpx-shared should not be generated for blueprint-only filter")
	}
}

func TestRunGenerateSkillsFilterWidget(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "skills-out")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"generate-skills", "--output-dir", outDir, "--filter", "widget"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0 (stderr=%s)", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}

	var resp generateSkillsResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("parse json output: %v\n%s", err, stdout.String())
	}
	if resp.Filter != "widget" {
		t.Fatalf("filter: got %q want %q", resp.Filter, "widget")
	}
	if resp.GeneratedCount != 1 {
		t.Fatalf("generatedCount: got %d want 1", resp.GeneratedCount)
	}
	if len(resp.Skills) != 1 || resp.Skills[0] != "bpx-widget" {
		t.Fatalf("skills: got %#v want [\"bpx-widget\"]", resp.Skills)
	}
	if _, err := os.Stat(filepath.Join(outDir, "bpx-widget", "SKILL.md")); err != nil {
		t.Fatalf("missing filtered widget skill file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "bpx-blueprint", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("bpx-blueprint should not be generated for widget-only filter")
	}
}

func TestRunGenerateSkillsRejectsPositionalArgs(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"generate-skills", "extra"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: bpx generate-skills [--output-dir <dir>] [--filter <token>]") {
		t.Fatalf("expected usage error, got: %s", stderr.String())
	}
}

func TestResolveSkillOutputDirExpandsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("user home dir: %v", err)
	}
	got, err := resolveSkillOutputDir("~")
	if err != nil {
		t.Fatalf("resolve output dir: %v", err)
	}
	if got != filepath.Clean(home) {
		t.Fatalf("resolved dir: got %q want %q", got, filepath.Clean(home))
	}
}

func TestRunGenerateSkillsRejectsFileOutputPath(t *testing.T) {
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "not-a-dir")
	if err := os.WriteFile(outputFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"generate-skills", "--output-dir", outputFile}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if !strings.Contains(stderr.String(), "--output-dir is not a directory") {
		t.Fatalf("expected directory validation error, got: %s", stderr.String())
	}
}
