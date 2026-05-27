package cli

import (
	"fmt"
	"strings"
)

type markdownSection struct {
	Heading string
	Body    string
}

type generatedSkillSupplement struct {
	Sections []markdownSection
	Caution  string
}

func mergeGeneratedSkillWithSupplement(name, base string) string {
	base = strings.TrimRight(base, "\n")
	supplement := buildGeneratedSkillSupplement(name)
	if len(supplement.Sections) == 0 && strings.TrimSpace(supplement.Caution) == "" {
		return base + "\n"
	}

	seen := map[string]struct{}{}
	for _, heading := range sectionHeadings(base) {
		seen[normalizeHeading(heading)] = struct{}{}
	}

	var b strings.Builder
	b.WriteString(base)
	for _, section := range supplement.Sections {
		heading := strings.TrimSpace(section.Heading)
		body := strings.TrimSpace(section.Body)
		if heading == "" || body == "" {
			continue
		}
		key := normalizeHeading(heading)
		if _, exists := seen[key]; exists {
			continue
		}
		b.WriteString("\n\n## ")
		b.WriteString(heading)
		b.WriteString("\n\n")
		b.WriteString(body)
		seen[key] = struct{}{}
	}

	if strings.TrimSpace(supplement.Caution) != "" && !strings.Contains(strings.ToLower(base), "[!caution]") {
		b.WriteString("\n\n")
		b.WriteString(strings.TrimSpace(supplement.Caution))
	}

	b.WriteString("\n")
	return b.String()
}

func buildGeneratedSkillSupplement(name string) generatedSkillSupplement {
	if name == "bpx-shared" {
		return generatedSkillSupplement{
			Sections: []markdownSection{
				{
					Heading: "Global Rules",
					Body: strings.Join([]string{
						"- Prefer read commands before write commands.",
						"- Use `--dry-run` first for write-capable commands.",
						"- Use `--backup` for in-place updates unless explicitly declined.",
						"- For automation, prefer `--format toml` where available.",
					}, "\n"),
				},
				{
					Heading: "Command Selection Heuristics",
					Body: strings.Join([]string{
						"- Package shape/version checks: `info`, `dump`, `validate`.",
						"- Export/import and reference analysis: `export`, `import`, `ref`, `raw`.",
						"- Gameplay/content edits: `prop`, `var`, `datatable`, `localization`, `stringtable`, `level`.",
						"- Blueprint analysis workflow: `blueprint info` -> `blueprint disasm` -> `blueprint trace/search`.",
					}, "\n"),
				},
				{
					Heading: "Output Reading Tips",
					Body: strings.Join([]string{
						"- Treat warnings as actionable signals; they often indicate partial decode paths.",
						"- For write responses, inspect changed-byte and size-delta fields before applying.",
						"- On errors, re-run `bpx help <command>` to confirm required flags and accepted forms.",
					}, "\n"),
				},
			},
		}
	}

	topic := strings.TrimPrefix(name, "bpx-")
	if topic == "" || topic == name {
		return generatedSkillSupplement{}
	}
	usage := skillUsageLinesForTopic(topic)
	behavior := skillBehaviorLinesForTopic(topic)
	sections := make([]markdownSection, 0, 3)

	if matrix := renderCommandMatrix(topic, usage, behavior); matrix != "" {
		sections = append(sections, markdownSection{
			Heading: "Command Matrix",
			Body:    matrix,
		})
	}
	if workflow := renderSkillWorkflow(topic); workflow != "" {
		sections = append(sections, markdownSection{
			Heading: "Recommended Workflow",
			Body:    workflow,
		})
	}
	if recipe := renderSkillRecipe(topic); recipe != "" {
		sections = append(sections, markdownSection{
			Heading: "Worked Recipe",
			Body:    recipe,
		})
	}
	if caveats := renderSkillCaveats(topic); caveats != "" {
		sections = append(sections, markdownSection{
			Heading: "Code-Aligned Caveats",
			Body:    caveats,
		})
	}
	if examples := renderExampleCommands(topic, usage); examples != "" {
		sections = append(sections, markdownSection{
			Heading: "High-Signal Examples",
			Body:    examples,
		})
	}
	return generatedSkillSupplement{Sections: sections}
}

func renderSkillWorkflow(topic string) string {
	switch topic {
	case "blueprint":
		return strings.Join([]string{
			"- Start with `bpx blueprint info` to identify candidate exports and confirm whether you are dealing with a Blueprint asset, a WidgetBlueprint, or a generated class payload.",
			"- Use `search` to narrow by member, node, or symbol name before moving to heavier analysis commands.",
			"- Reach for `trace` and `call-args` when you already know the node or function you care about and want a focused dependency path.",
			"- Use `disasm` or `bytecode` only after you have identified the target export and execution slice; they are best for instruction-level inspection, not broad discovery.",
			"- Prefer `refs` when you need reverse lookup from a soft object path back to the Blueprint nodes/pins that mention it.",
			"- For WidgetBlueprint construction and edit workflows, switch to [bpx-widget](../bpx-widget/SKILL.md). Keep this skill focused on Blueprint analysis rather than widget tree authoring.",
		}, "\n")
	case "widget":
		return strings.Join([]string{
			"- If the user asks to create a widget without explicitly naming an existing `.uasset` to edit, treat it as a new-widget request and start from `bpx blueprint widget-init ... --template minimum`.",
			"- Do not choose an existing WidgetBlueprint as a base asset unless the user explicitly asks to modify that specific widget.",
			"- Start a new widget with `bpx blueprint widget-init ... --template minimum` so BPX clones a validated empty WidgetBlueprint. If the widget should inherit a compiled project/plugin class, set it before adding the root widget with `--parent-class /Script/...` or `widget-parent-class --class /Script/...`.",
			"- Run widget-building commands sequentially on the same asset. Do not parallelize `widget-init`, `widget-parent-class`, repeated `widget-add` / `widget-remove`, or `widget-write` steps.",
			"- Add exactly one root container with `widget-add --parent root`. The common choices are `canvaspanel`, `overlay`, `verticalbox`, `horizontalbox`, `scrollbox`, `sizebox`, `border`, or `button` depending on layout intent.",
			"- Add child widgets under that parent with `widget-add --parent <panel>` and keep names unique. Use `--type userwidget --class /Game/.../WBP_Name` when the user wants to compose with another WidgetBlueprint.",
			"- For nested multi-child layouts, build the tree in exact order and switch to full widget paths as soon as siblings exist. A proven sequence is `root -> CanvasPanel -> Image1 -> Image2 -> Overlay -> Image3`.",
			"- After each nested `widget-add`, re-run `widget-read` and confirm the new logical path exists before continuing. This catches regressions like a missing second child under `CanvasPanel` or `Overlay` early.",
			"- Fill in appearance after structure. Common writes are `layout-data`, `slot-*`, `brush-image`, `text`, `text-color`, `text-font-family`, `text-font-size`, `text-auto-wrap-text`, `progressbar-percent`, `slider-value`, `checkbox-is-checked`, `sizebox-width`, and `scrollbox-orientation`.",
			"- Use `widget-remove` only for non-root leaf cleanup after confirming the exact path with `widget-read`.",
			"- Keep the order stable: later widget commands depend on the export/name/import state produced by earlier ones.",
			"- Re-check the tree with `widget-read`; use `validate` when you want an extra safety pass after a multi-step edit chain.",
			"- Treat `testdata/golden/ue5.6/operations/widget_add_image_nested_overlay` as the ground-truth fixture for nested `CanvasPanel -> Overlay -> Image` behavior. If a fresh asset differs or opens badly in UE, compare against that operation first.",
			"- `--package-path` is a directory like `/Game/UI`, not a full asset path like `/Game/UI/WBP_Login`. When the destination is already under a UE `Content` directory, keep `--package-path` / `--asset-name` aligned with that filesystem location.",
			"- `brush-image` expects a UE package path like `/Game/UI/T_Icon`, not a filesystem path like `C:\\Textures\\Icon.png`.",
			"- If your shell rewrites `/Game/...` arguments (for example Git Bash/MSYS), disable path conversion for those arguments first.",
		}, "\n")
	default:
		return ""
	}
}

func renderSkillRecipe(topic string) string {
	switch topic {
	case "blueprint":
		return "```bash\n" +
			"bpx blueprint info ./Content/BP_Player.uasset\n" +
			"bpx blueprint search ./Content/BP_Player.uasset --member ApplyDamage --show name,class,member\n" +
			"bpx blueprint trace ./Content/BP_Player.uasset --from K2Node_CallFunction_12 --max-depth 6\n" +
			"bpx blueprint call-args ./Content/BP_Player.uasset --member ApplyDamage --all-pins\n" +
			"bpx blueprint disasm ./Content/BP_Player.uasset --export 21 --analysis --format text\n" +
			"```\n\n" +
			strings.Join([]string{
				"- This is the preferred shape for \"find a Blueprint function, inspect the call path, then disassemble the final target export\".",
				"- Use `search` and `trace` first so `disasm` stays focused on a specific export instead of becoming a broad dump.",
				"- If your goal is WidgetBlueprint construction or widget property mutation, use [bpx-widget](../bpx-widget/SKILL.md) instead of treating this analysis recipe as an authoring workflow.",
			}, "\n")
	case "widget":
		return "```bash\n" +
			"export MSYS_NO_PATHCONV=1\n" +
			"bpx blueprint widget-init ./Content/WBP_Status.uasset --template minimum --asset-name WBP_Status --package-path /Game\n" +
			"bpx blueprint widget-add ./Content/WBP_Status.uasset --parent root --type canvaspanel --name Canvas_Root\n" +
			"bpx blueprint widget-add ./Content/WBP_Status.uasset --parent Canvas_Root --type image --name Image_Circle\n" +
			"bpx blueprint widget-add ./Content/WBP_Status.uasset --parent Canvas_Root --type image --name Image_Square\n" +
			"bpx blueprint widget-add ./Content/WBP_Status.uasset --parent Canvas_Root --type textblock --name Text_Label\n" +
			"bpx blueprint widget-write ./Content/WBP_Status.uasset --widget Canvas_Root/Image_Circle --property brush-image --value /Game/Textures/Circle\n" +
			"bpx blueprint widget-write ./Content/WBP_Status.uasset --widget Canvas_Root/Image_Square --property brush-image --value /Game/Textures/Square\n" +
			"bpx blueprint widget-write ./Content/WBP_Status.uasset --widget Canvas_Root/Text_Label --property text --value \"Hello World\"\n" +
			"bpx blueprint widget-read ./Content/WBP_Status.uasset\n" +
			"bpx validate ./Content/WBP_Status.uasset\n" +
			"```\n\n" +
			"```bash\n" +
			"export MSYS_NO_PATHCONV=1\n" +
			"bpx blueprint widget-init ./Content/WBP_MenuRoot.uasset --template minimum --asset-name WBP_MenuRoot --package-path /Game/UI --parent-class /Script/LyraGame.LyraActivatableWidget\n" +
			"bpx blueprint widget-add ./Content/WBP_MenuRoot.uasset --parent root --type overlay --name Overlay_1\n" +
			"bpx blueprint widget-add ./Content/WBP_MenuRoot.uasset --parent Overlay_1 --type userwidget --class /Game/UI/WBP_Status --name WBP_Status_1\n" +
			"bpx blueprint widget-write ./Content/WBP_MenuRoot.uasset --widget Overlay_1/WBP_Status_1 --property slot-horizontal-alignment --value fill\n" +
			"bpx blueprint widget-read ./Content/WBP_MenuRoot.uasset\n" +
			"bpx validate ./Content/WBP_MenuRoot.uasset\n" +
			"```\n\n" +
			"```bash\n" +
			"export MSYS_NO_PATHCONV=1\n" +
			"bpx blueprint widget-init ./Content/WBP_NestedStatus.uasset --template minimum --asset-name WBP_NestedStatus --package-path /Game\n" +
			"bpx blueprint widget-add ./Content/WBP_NestedStatus.uasset --parent root --type canvaspanel --name CanvasPanel_1\n" +
			"bpx blueprint widget-add ./Content/WBP_NestedStatus.uasset --parent CanvasPanel_1 --type image --name Image_1\n" +
			"bpx blueprint widget-add ./Content/WBP_NestedStatus.uasset --parent CanvasPanel_1 --type image --name Image_2\n" +
			"bpx blueprint widget-add ./Content/WBP_NestedStatus.uasset --parent CanvasPanel_1 --type overlay --name Overlay_1\n" +
			"bpx blueprint widget-add ./Content/WBP_NestedStatus.uasset --parent CanvasPanel_1/Overlay_1 --type image --name Image_3\n" +
			"bpx blueprint widget-read ./Content/WBP_NestedStatus.uasset\n" +
			"bpx validate ./Content/WBP_NestedStatus.uasset\n" +
			"```\n\n" +
			strings.Join([]string{
				"- The first recipe is the preferred happy-path for \"new widget + CanvasPanel + Image x2 + TextBlock x1\".",
				"- The second recipe shows the safe parent-class-first flow for project/plugin widget bases plus child `userwidget` composition.",
				"- When the request is simply \"make a widget\" and no existing target asset is named, use this `widget-init`-first flow instead of editing a pre-existing WidgetBlueprint.",
				"- The third recipe is the regression-sensitive nested path that should yield `CanvasPanel_1`, `CanvasPanel_1/Image_1`, `CanvasPanel_1/Image_2`, `CanvasPanel_1/Overlay_1`, and `CanvasPanel_1/Overlay_1/Image_3` in `widget-read`.",
				"- Do not insert manual `name add` steps in the normal widget workflow. `widget-add` and `widget-write --property brush-image` are expected to manage required names/imports themselves.",
				"- If a freshly built `widget-init` asset still reports missing NameMap/import-add names, treat that as a stale binary/build mismatch and update `bpx` instead of patching NameMap by hand.",
				"- Use full widget paths like `Canvas_Root/Image_Circle` once the tree has more than one child; that avoids ambiguous selectors and keeps later writes aligned with the intended widget.",
				"- In the nested recipe, use the full path `CanvasPanel_1/Overlay_1` for the final add. Do not rely on bare `Overlay_1` once siblings already exist under `CanvasPanel_1`.",
				"- If a nested asset passes BPX `validate` but still behaves differently in UE, diff it against `widget_add_image_nested_overlay` before hand-editing NameMap, imports, or GeneratedClass data.",
			}, "\n")
	default:
		return ""
	}
}

func renderCommandMatrix(topic string, usageLines, behaviorLines []string) string {
	type row struct {
		Command string
		When    string
		Notes   string
	}
	rows := make([]row, 0, len(usageLines))
	seen := map[string]struct{}{}
	for _, usage := range usageLines {
		variant := usageVariant(topic, usage)
		if variant == "" {
			variant = topic
		}
		if _, ok := seen[variant]; ok {
			continue
		}
		seen[variant] = struct{}{}

		when := behaviorSummaryForVariant(variant, behaviorLines)
		if when == "" {
			when = "Use this command when this operation matches the target workflow."
		}
		rows = append(rows, row{
			Command: "`" + variant + "`",
			When:    when,
			Notes:   defaultNotesForVariant(variant),
		})
	}
	if len(rows) <= 1 {
		return ""
	}

	var b strings.Builder
	b.WriteString("| Command | Use when | Notable defaults |\n")
	b.WriteString("|------|------|------|\n")
	for _, r := range rows {
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", r.Command, sanitizeTableCell(r.When), sanitizeTableCell(r.Notes)))
	}
	return strings.TrimRight(b.String(), "\n")
}

func usageVariant(topic, usage string) string {
	fields := strings.Fields(usage)
	if len(fields) < 2 {
		return ""
	}
	// Expect: bpx <topic> [subcommand] ...
	if fields[0] != "bpx" || fields[1] != topic {
		return ""
	}
	if len(fields) < 3 {
		return topic
	}
	candidate := fields[2]
	if strings.HasPrefix(candidate, "<") || strings.HasPrefix(candidate, "[") || strings.HasPrefix(candidate, "--") {
		return topic
	}
	return candidate
}

func behaviorSummaryForVariant(variant string, behaviorLines []string) string {
	prefixes := []string{
		fmt.Sprintf("`%s`:", variant),
		fmt.Sprintf("`%s` ", variant),
	}
	for _, line := range behaviorLines {
		trimmed := strings.TrimSpace(line)
		for _, prefix := range prefixes {
			if strings.HasPrefix(trimmed, prefix) {
				out := strings.TrimPrefix(trimmed, fmt.Sprintf("`%s`:", variant))
				out = strings.TrimPrefix(out, fmt.Sprintf("`%s`", variant))
				out = strings.TrimSpace(strings.TrimPrefix(out, ":"))
				out = strings.TrimSuffix(out, ".")
				if out != "" {
					return out + "."
				}
			}
		}
	}
	if len(behaviorLines) > 0 {
		return strings.TrimSuffix(strings.TrimSpace(behaviorLines[0]), ".") + "."
	}
	return ""
}

func defaultNotesForVariant(variant string) string {
	switch variant {
	case "read", "list", "info", "summary":
		return "Read-only path; safe for discovery."
	case "set", "add", "remove", "rename", "rewrite", "update-row", "add-row", "remove-row", "set-header", "set-flags", "set-source", "set-id", "set-stringtable-ref", "rekey", "rewrite-namespace", "var-set", "write-entry", "write-value":
		return "Run `--dry-run` first and use `--backup` for real writes."
	default:
		return "Check `bpx help` for exact required flags."
	}
}

func renderSkillCaveats(topic string) string {
	caveats := map[string][]string{
		"find": {
			"`find summary` continues across parse failures; inspect `parseFailures` before deciding next steps.",
			"For map-only scans, use `--pattern \"*.umap\"`.",
		},
		"import": {
			"`graph` is ImportMap-based and may not reflect K2 graph-level references.",
			"Large directory scans should use `--filter` and narrower patterns for speed.",
		},
		"prop": {
			"Write paths mutate one export only; invalid dot paths fail explicitly.",
			"Prefer `prop list` immediately before `prop set/add/remove` to avoid stale assumptions.",
		},
		"package": {
			"`set-flags` blocks unsupported/safety-critical toggles by design.",
			"`resolve-index` is the safest way to interpret signed `FPackageIndex` in automation flows.",
		},
		"localization": {
			"`resolve` is read-preview oriented; it does not mutate assets.",
			"Bulk namespace/key rewrites should always be previewed first in a narrowed scope.",
		},
		"datatable": {
			"Row mutations target DataTable exports only; non-DataTable types are rejected.",
			"When using CSV/TSV output, confirm flattened values before patching rows.",
		},
		"blueprint": {
			"Large blueprints can produce very large payloads; constrain via `--limit`/`--max-steps`.",
			"`refs --include-routes` can be expensive; disable routes when doing broad scans.",
			"`disasm --entrypoint` implies analysis-oriented output.",
			"`trace` and `call-args` are most useful after `search` or `info` has already narrowed the target export/node set.",
			"For WidgetBlueprint edit flows and UE-openability caveats, prefer [bpx-widget](../bpx-widget/SKILL.md) over embedding widget-authoring guidance here.",
		},
		"widget": {
			"On a normal `widget-init` -> `widget-add` flow, you should not need manual `name add` before inserting widgets.",
			"If `widget-add` on a fresh widget reports missing NameMap/import-add names, suspect a stale `bpx` binary rather than treating manual NameMap surgery as standard workflow.",
			"Nested multi-child panels can appear in `WidgetVariableNameToGuidMap` without appearing in `GeneratedVariables` or `PropertyGuids`. `widget_add_image_nested_overlay` is the reference case: `Overlay_1` is real and valid there even though it does not become a generated class field.",
			"`widget-parent-class` is only safe before a root widget exists. Once the WidgetBlueprint has a root/widget tree, change the structure instead of trying to reparent the class in place.",
			"`widget-add --type userwidget` expects a WidgetBlueprint asset path like `/Game/UI/WBP_Status`, not a generated-class path like `/Game/UI/WBP_Status_C`.",
			"`widget-remove` is conservative by design: it supports only non-root leaf widgets and only compacts orphan export/import/name entries when the remaining references validate cleanly.",
			"BPX structural `validate` is necessary but not sufficient for UE-openability on newly generated nested widgets. If the editor still crashes or refuses to open, compare both `widget-read` output and the binary delta against the UE-authored nested fixture before widening BPX special-cases.",
		},
		"level": {
			"`--actor` resolution supports name, `PersistentLevel.<Name>`, or export index.",
			"`var-set` uses property path semantics; use `var-list` to validate target paths first.",
		},
		"material": {
			"`material read` is the canonical entry; use flags to opt into HLSL and child scans.",
			"Directory scans should always narrow with `--parent`, `--pattern`, and `--limit`.",
		},
		"metadata": {
			"There is no `metadata read` subcommand; read form is `bpx metadata <file> --export <n>`.",
			"Object metadata updates require a valid `--import` target.",
		},
		"validate": {
			"Exit code `2` indicates a non-OK validation result (not a transport/runtime failure).",
			"`--binary-equality` is the strongest no-op round-trip safety check.",
		},
	}
	lines := caveats[topic]
	if len(lines) == 0 {
		return ""
	}
	formatted := make([]string, 0, len(lines))
	for _, line := range lines {
		formatted = append(formatted, "- "+line)
	}
	return strings.Join(formatted, "\n")
}

func renderExampleCommands(topic string, usageLines []string) string {
	if len(usageLines) == 0 {
		return ""
	}
	if topic == "blueprint" {
		return strings.Join([]string{
			"```bash",
			"bpx blueprint info ./Content/BP_Player.uasset",
			"bpx blueprint search ./Content/BP_Player.uasset --member ApplyDamage --show name,class,member",
			"bpx blueprint trace ./Content/BP_Player.uasset --from K2Node_CallFunction_12 --max-depth 6",
			"bpx blueprint disasm ./Content/BP_Player.uasset --export 21 --analysis --format text",
			"```",
		}, "\n")
	}
	if topic == "widget" || strings.HasPrefix(strings.TrimSpace(usageLines[0]), "bpx blueprint widget-") {
		return strings.Join([]string{
			"```bash",
			"bpx blueprint widget-init ./Content/WBP_Login.uasset --template minimum --asset-name WBP_Login --package-path /Game/UI",
			"bpx blueprint widget-add ./Content/WBP_Login.uasset --parent root --type canvaspanel --name Canvas_Root",
			"bpx blueprint widget-add ./Content/WBP_Login.uasset --parent Canvas_Root --type image --name Image_Logo",
			"bpx blueprint widget-write ./Content/WBP_Login.uasset --widget Canvas_Root/Image_Logo --property brush-image --value /Game/UI/T_Logo",
			"```",
		}, "\n")
	}
	limit := 4
	if len(usageLines) < limit {
		limit = len(usageLines)
	}
	var b strings.Builder
	b.WriteString("```bash\n")
	for i := 0; i < limit; i++ {
		b.WriteString(exampleFromUsage(usageLines[i]))
		b.WriteByte('\n')
	}
	b.WriteString("```")
	return b.String()
}

func exampleFromUsage(usage string) string {
	replacer := strings.NewReplacer(
		"<file.uasset>", "./Sample.uasset",
		"<file.umap>", "./Sample.umap",
		"<directory>", "./Content",
		"<n>", "1",
		"<i>", "1",
		"<k>", "SampleKey",
		"<v>", "SampleValue",
		"<ns>", "Game",
		"<token>", "SampleToken",
		"<name>", "SampleName",
		"<section>", "Names",
		"<dot.path>", "MyProperty",
		"<path>", "MyProperty",
		"<culture>", "en",
		"<table-id>", "ST_Game",
		"<enum-or-raw>", "PKG_ContainsMap",
		"<new.uasset>", "./Sample.out.uasset",
		"<old>", "OldValue",
		"<new>", "NewValue",
		`'<json>'`, "'{\"value\":1}'",
		"<json>", `{"value":1}`,
		"<regex>", ".*",
		"<vm>", "0",
	)
	return replacer.Replace(usage)
}

func sanitizeTableCell(in string) string {
	s := strings.TrimSpace(in)
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}

func sectionHeadings(markdown string) []string {
	lines := strings.Split(markdown, "\n")
	headings := make([]string, 0, 8)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			headings = append(headings, strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")))
		}
	}
	return headings
}

func normalizeHeading(heading string) string {
	return strings.ToLower(strings.TrimSpace(heading))
}
