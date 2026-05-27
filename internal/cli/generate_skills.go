package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type generatedSkill struct {
	Name        string
	Description string
	Content     string
}

func runGenerateSkills(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("generate-skills", stderr)
	outputDir := fs.String("output-dir", "skills", "output directory for generated skills")
	filter := fs.String("filter", "", "optional substring filter for skill name/description")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: bpx generate-skills [--output-dir <dir>] [--filter <token>]")
		return 1
	}

	root, err := resolveSkillOutputDir(*outputDir)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	specs := buildGeneratedSkills(*filter)
	generated := make([]string, 0, len(specs))
	for _, spec := range specs {
		dir := filepath.Join(root, spec.Name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(stderr, "error: create skill directory %s: %v\n", dir, err)
			return 1
		}
		path := filepath.Join(dir, "SKILL.md")
		if err := writeFileAtomically(path, []byte(spec.Content), 0o644); err != nil {
			fmt.Fprintf(stderr, "error: write %s: %v\n", path, err)
			return 1
		}
		generated = append(generated, spec.Name)
	}

	resp := map[string]any{
		"outputDir":      root,
		"filter":         strings.TrimSpace(*filter),
		"generatedCount": len(generated),
		"skills":         generated,
	}
	return printJSON(stdout, resp)
}

func resolveSkillOutputDir(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("--output-dir must not be empty")
	}
	if trimmed == "~" || strings.HasPrefix(trimmed, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		if trimmed == "~" {
			trimmed = home
		} else {
			trimmed = filepath.Join(home, trimmed[2:])
		}
	}

	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve output directory: %w", err)
	}
	abs = filepath.Clean(abs)

	info, err := os.Stat(abs)
	switch {
	case err == nil:
		if !info.IsDir() {
			return "", fmt.Errorf("--output-dir is not a directory: %s", abs)
		}
	case os.IsNotExist(err):
		if err := os.MkdirAll(abs, 0o755); err != nil {
			return "", fmt.Errorf("create output directory: %w", err)
		}
	default:
		return "", fmt.Errorf("stat output directory: %w", err)
	}
	return abs, nil
}

func buildGeneratedSkills(filter string) []generatedSkill {
	matches := skillFilterMatcher(filter)

	skills := make([]generatedSkill, 0, 32)
	shared := generatedSharedSkill()
	shared.Content = mergeGeneratedSkillWithSupplement(shared.Name, shared.Content)
	if matches(shared.Name, shared.Description) {
		skills = append(skills, shared)
	}
	widget := generatedWidgetSkill()
	if matches(widget.Name, widget.Description) {
		skills = append(skills, widget)
	}

	for _, topic := range orderedHelpTopics() {
		if topic == "generate-skills" {
			continue
		}
		usage := skillUsageLinesForTopic(topic)
		if len(usage) == 0 {
			continue
		}
		summary := helpTopicSummary(topic)
		if summary == "" {
			summary = "BPX command skill."
		}
		name := "bpx-" + topic
		desc := fmt.Sprintf("BPX `%s` command skill. %s", topic, summary)
		if !matches(name, desc) {
			continue
		}
		skill := generatedTopicSkill(name, topic, desc, usage, skillBehaviorLinesForTopic(topic), skillHasWriteCommands(topic))
		skill.Content = mergeGeneratedSkillWithSupplement(skill.Name, skill.Content)
		skills = append(skills, skill)
	}

	sort.SliceStable(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})
	return skills
}

func skillUsageLinesForTopic(topic string) []string {
	usage := usageLinesForTopic(topic)
	if topic != "blueprint" {
		return usage
	}
	filtered := make([]string, 0, len(usage))
	for _, line := range usage {
		if strings.HasPrefix(strings.TrimSpace(line), "bpx blueprint widget-") {
			continue
		}
		filtered = append(filtered, line)
	}
	return filtered
}

func skillBehaviorLinesForTopic(topic string) []string {
	behavior := helpTopicBehaviorLines(topic)
	if topic != "blueprint" {
		return behavior
	}
	filtered := make([]string, 0, len(behavior))
	for _, line := range behavior {
		if strings.Contains(strings.ToLower(line), "widget") {
			continue
		}
		filtered = append(filtered, line)
	}
	return filtered
}

func skillHasWriteCommands(topic string) bool {
	if topic == "blueprint" {
		return false
	}
	return topicHasWriteCommands(topic)
}

func skillFilterMatcher(filter string) func(name, description string) bool {
	needle := strings.ToLower(strings.TrimSpace(filter))
	return func(name, description string) bool {
		if needle == "" {
			return true
		}
		name = strings.ToLower(name)
		description = strings.ToLower(description)
		return strings.Contains(name, needle) || strings.Contains(description, needle)
	}
}

func orderedHelpTopics() []string {
	seen := map[string]struct{}{}
	topics := make([]string, 0, 32)
	for _, category := range helpCatalog() {
		for _, line := range category.Lines {
			topic := usageTopicFromLine(line)
			if topic == "" {
				continue
			}
			if _, ok := seen[topic]; ok {
				continue
			}
			seen[topic] = struct{}{}
			topics = append(topics, topic)
		}
	}
	return topics
}

func generatedSharedSkill() generatedSkill {
	description := "Shared BPX safety and execution guidance. Use before command-specific BPX skills."
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: bpx-shared\n")
	b.WriteString("description: " + description + "\n")
	b.WriteString("---\n\n")
	b.WriteString("# bpx shared\n\n")
	b.WriteString("## Installation\n\n")
	b.WriteString("- Ensure `bpx` is available on `PATH`.\n")
	b.WriteString("- Confirm with `bpx version` and `bpx help`.\n\n")
	b.WriteString("## Safety Rules\n\n")
	b.WriteString("- Treat assets as untrusted binary input.\n")
	b.WriteString("- Prefer read commands before write commands.\n")
	b.WriteString("- Run `--dry-run` first for write-capable commands.\n")
	b.WriteString("- Use `--backup` when writing files in place.\n\n")
	b.WriteString("## Standard Workflow\n\n")
	b.WriteString("1. Inspect command shape with `bpx help <command>`.\n")
	b.WriteString("2. Identify exact targets using read commands.\n")
	b.WriteString("3. Run write command with `--dry-run`.\n")
	b.WriteString("4. Execute real write with `--backup` when approved.\n")
	b.WriteString("5. Validate with `bpx validate <file> --binary-equality` as needed.\n")
	return generatedSkill{
		Name:        "bpx-shared",
		Description: description,
		Content:     b.String(),
	}
}

func generatedWidgetSkill() generatedSkill {
	description := "BPX widget construction and inspection skill. Use for widget-init/read/parent-class/add/remove/write workflows."
	usageLines := widgetSkillUsageLines()
	behaviorLines := []string{
		"`widget-read`: reads WidgetBlueprint / WidgetTree hierarchy as normalized JSON, plus logical widget aggregation and high-level widget/slot summaries.",
		"`widget-init`: clones a validated empty WidgetBlueprint template into a new output asset and rewrites package/object identity; this is the default starting point for unspecified new-widget requests.",
		"`widget-parent-class`: rewrites the compiled `/Script/...` parent class on a rootless WidgetBlueprint before widgets are added.",
		"`widget-add`: creates a root container/content widget or inserts a bare child widget under supported panel/content parents.",
		"`widget-remove`: removes one non-root leaf widget from the logical WidgetTree and related WidgetBlueprint metadata.",
		"`widget-write`: updates one logical widget across designer/generated trees.",
		"Do not repurpose or mutate an existing WidgetBlueprint unless the user explicitly identifies that asset as the edit target.",
		"Pair multi-step widget edits with `bpx validate <file.uasset>` when you want a structural safety pass after a write chain.",
	}

	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: bpx-widget\n")
	b.WriteString("description: " + description + "\n")
	b.WriteString("---\n\n")
	b.WriteString("# widget\n\n")
	b.WriteString("> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).\n\n")
	b.WriteString("## Usage\n\n")
	b.WriteString("```bash\n")
	for _, line := range usageLines {
		b.WriteString(line + "\n")
	}
	b.WriteString("```\n")
	b.WriteString("\n## Behavior\n\n")
	for _, line := range behaviorLines {
		b.WriteString("- " + line + "\n")
	}
	b.WriteString("\n> [!CAUTION]\n")
	b.WriteString("> This skill is write-heavy. Confirm intent and run `--dry-run` first.\n")
	b.WriteString("\n## Supported Widgets\n\n")
	b.WriteString(strings.Join([]string{
		"- Rootless start: `widget-init --template minimum`, optionally followed by `widget-parent-class` or `widget-init --parent-class` while the asset still has no root widget.",
		"- Root / parent containers: `CanvasPanel`, `Overlay`, `VerticalBox`, `HorizontalBox`, `StackBox`, `ScrollBox`, `WrapBox`, `GridPanel`, `UniformGridPanel`, `WidgetSwitcher`, `ListView`, `TileView`, `TreeView`, `Button`, `CheckBox`, `Border`, `RetainerBox`, `InvalidationBox`, `MenuAnchor`, `NamedSlot`, `SizeBox`, `ScaleBox`, `BackgroundBlur`, `SafeZone`, and `WindowTitleBarArea`.",
		"- Child widgets: `Image`, `TextBlock`, `RichTextBlock`, `ProgressBar`, `Slider`, `Spacer`, `ScrollBar`, `EditableText`, `EditableTextBox`, `MultiLineEditableTextBox`, `SpinBox`, `ComboBoxString`, `ListView`, `TileView`, `TreeView`, `CheckBox`, and `userwidget`.",
		"- `widget-add --type userwidget` expects a WidgetBlueprint asset path like `/Game/UI/WBP_Status` and instantiates the referenced `WidgetBlueprintGeneratedClass` as a child widget.",
		"- Parent-class rewrites currently accept compiled `/Script/...` classes, including project/plugin module classes such as `/Script/LyraGame.LyraActivatableWidget`.",
	}, "\n"))
	b.WriteString("\n\n## Supported Writes\n\n")
	b.WriteString(strings.Join([]string{
		"- Universal reads/writes: `text`, `visibility`, `render-opacity`, `brush-image`.",
		"- Basic widgets: `progressbar-percent`, `progressbar-fill-color`, `slider-value`, `slider-min-value`, `slider-max-value`, `slider-step-size`, `slider-orientation`, `spacer-size`, `scrollbar-thickness`, `scrollbar-orientation`, `checkbox-checked-state`, `checkbox-is-checked`, `editabletext-hint-text`, `editabletext-is-read-only`, `editabletext-is-password`, `editabletext-minimum-desired-width`, `editabletext-justification`, `editabletextbox-hint-text`, `editabletextbox-is-read-only`, `editabletextbox-is-password`, `editabletextbox-minimum-desired-width`, `editabletextbox-justification`, `multilineeditabletextbox-hint-text`, `multilineeditabletextbox-is-read-only`, `multilineeditabletextbox-justification`, `spinbox-value`, `spinbox-min-value`, `spinbox-max-value`, `spinbox-delta`, `comboboxstring-selected-option`, `comboboxstring-options`, `is-focusable`, `button-is-focusable`, `checkbox-is-focusable`, `slider-is-focusable`, `scrollbox-is-focusable`, `comboboxstring-is-focusable`, `listview-entry-widget-class`, `listview-orientation`, `listview-selection-mode`, `listview-consume-mouse-wheel`, `listview-is-focusable`, `listview-return-focus-to-selection`, `listview-clear-scroll-velocity-on-selection`, `listview-scroll-into-view-alignment`, `listview-wheel-scroll-multiplier`, `listview-enable-scroll-animation`, `listview-allow-overscroll`, `listview-enable-right-click-scrolling`, `listview-enable-touch-scrolling`, `listview-is-pointer-scrolling-enabled`, `listview-is-gamepad-scrolling-enabled`, `listview-horizontal-entry-spacing`, `listview-vertical-entry-spacing`, `listview-scrollbar-padding`, `tileview-entry-width`, `tileview-entry-height`, `tileview-scrollbar-disabled-visibility`, `tileview-entry-size-includes-entry-spacing`, `scrollbox-orientation`, `scrollbox-scrollbar-visibility`, `scrollbox-consume-mouse-wheel`, `sizebox-width-override`, `sizebox-width`, `sizebox-height-override`, `sizebox-height`, `sizebox-min-desired-width`, `sizebox-min-desired-height`, `sizebox-max-desired-width`, `sizebox-max-desired-height`, `sizebox-min-aspect-ratio`, `sizebox-max-aspect-ratio`, `scalebox-stretch`, `scalebox-stretch-direction`, `scalebox-user-specified-scale`, `scalebox-ignore-inherited-scale`, `wrapbox-wrap-size`, `wrapbox-explicit-wrap-size`, `wrapbox-inner-slot-padding`, `wrapbox-orientation`, `widgetswitcher-active-widget-index`, `retainerbox-retain-render`, `retainerbox-render-on-invalidation`, `retainerbox-render-on-phase`, `retainerbox-phase`, `retainerbox-phase-count`, `backgroundblur-strength`, `backgroundblur-apply-alpha-to-blur`, `safezone-pad-left`, `safezone-pad-right`, `safezone-pad-top`, `safezone-pad-bottom`, `invalidationbox-can-cache`, `uniformgridpanel-min-desired-slot-width`, `uniformgridpanel-min-desired-slot-height`, `uniformgridpanel-slot-padding`.",
		"- `EditableText`: generic `text` plus `editabletext-hint-text`, `editabletext-is-read-only`, `editabletext-is-password`, `editabletext-minimum-desired-width`, `editabletext-justification`.",
		"- `EditableTextBox`: generic `text` plus `editabletextbox-hint-text`, `editabletextbox-is-read-only`, `editabletextbox-is-password`, `editabletextbox-minimum-desired-width`, `editabletextbox-justification`.",
		"- `MultiLineEditableTextBox`: generic `text` plus `multilineeditabletextbox-hint-text`, `multilineeditabletextbox-is-read-only`, `multilineeditabletextbox-justification`.",
		"- `SpinBox`: `spinbox-value`, `spinbox-min-value`, `spinbox-max-value`, `spinbox-delta`.",
		"- `ComboBoxString`: `comboboxstring-selected-option`, `comboboxstring-options`, `comboboxstring-is-focusable`.",
		"- `TextBlock`: `text-color(-and-opacity)`, `text-font`, `text-font-family`, `text-typeface`, `text-font-size`, `text-justification`, `text-auto-wrap-text`, `text-wrap-text-at`, `text-line-height-percentage`, `text-shadow-offset`, `text-shadow-color-and-opacity`, `text-outline-size`, `text-outline-color`.",
		"- `Button`: per-state brush image/tint/size/draw-as writes plus `button-background-color`, `button-color-and-opacity`, and `button-is-focusable`.",
		"- `Border`: `border-padding`, `border-brush-color`, `border-content-color-and-opacity`, `border-horizontal-alignment`, `border-vertical-alignment`.",
		"- `GridPanel`: `grid-row-fill`, `grid-column-fill`.",
		"- `RichTextBlock`: `richtext-style-set`, `richtext-decorator-classes`, `richtext-override-default-style`, `richtext-default-font`, `richtext-default-font-family`, `richtext-default-typeface`, `richtext-default-font-size`, `richtext-default-color-and-opacity`, `richtext-default-shadow-offset`, `richtext-default-shadow-color-and-opacity`, `richtext-default-outline-size`, `richtext-default-outline-color`, `richtext-auto-wrap-text`, `richtext-wrap-text-at`, `richtext-line-height-percentage`, `richtext-justification`.",
		"- Slot/layout writes: `slot-padding`, `slot-size`, `slot-horizontal-alignment`, `slot-vertical-alignment`, `slot-row`, `slot-column`, `slot-row-span`, `slot-column-span`, `slot-layer`, `slot-nudge`, `layout-position`, `layout-size`, `layout-anchors`, `layout-alignment`, `layout-data`.",
	}, "\n"))
	b.WriteString("\n\n## Known Gaps\n\n")
	b.WriteString(strings.Join([]string{
		"- `widget-move` and `widget-clone` are not implemented.",
		"- `widget-remove` is intentionally narrow: non-root leaf widgets only.",
		"- Broader `RichTextBlock` styling such as strike brushes, transform policy, and material-backed font overrides is not implemented yet.",
		"- CommonUI-specific widget/property writers are not implemented yet.",
		"- `widget-parent-class` is intentionally narrow: rootless WidgetBlueprints only, and compiled `/Script/...` parent classes only.",
	}, "\n"))
	b.WriteString("\n\n## Recommended Workflow\n\n")
	b.WriteString(renderSkillWorkflow("widget"))
	b.WriteString("\n\n## Worked Recipe\n\n")
	b.WriteString(renderSkillRecipe("widget"))
	b.WriteString("\n\n## Code-Aligned Caveats\n\n")
	b.WriteString(renderSkillCaveats("widget"))
	if examples := renderExampleCommands("widget", usageLines); examples != "" {
		b.WriteString("\n\n## High-Signal Examples\n\n")
		b.WriteString(examples)
	}
	b.WriteString("\n")
	return generatedSkill{
		Name:        "bpx-widget",
		Description: description,
		Content:     b.String(),
	}
}

func widgetSkillUsageLines() []string {
	lines := make([]string, 0, 6)
	for _, line := range usageLinesForTopic("blueprint") {
		if strings.HasPrefix(line, "bpx blueprint widget-") {
			lines = append(lines, line)
		}
	}
	return lines
}

func generatedTopicSkill(name, topic, description string, usageLines, behaviorLines []string, hasWrites bool) generatedSkill {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: " + name + "\n")
	b.WriteString("description: " + description + "\n")
	b.WriteString("---\n\n")
	b.WriteString("# " + topic + "\n\n")
	b.WriteString("> **PREREQUISITE:** Read [bpx-shared](../bpx-shared/SKILL.md).\n\n")
	b.WriteString("## Usage\n\n")
	b.WriteString("```bash\n")
	for _, line := range usageLines {
		b.WriteString(line + "\n")
	}
	b.WriteString("```\n")
	if len(behaviorLines) > 0 {
		b.WriteString("\n## Behavior\n\n")
		for _, line := range behaviorLines {
			b.WriteString("- " + line + "\n")
		}
	}
	if hasWrites {
		b.WriteString("\n> [!CAUTION]\n")
		b.WriteString("> This command includes write-capable operations. Confirm intent and run `--dry-run` first.\n")
	}
	return generatedSkill{
		Name:        name,
		Description: description,
		Content:     b.String(),
	}
}
