package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/wilddogjp/bpx/internal/cli"
)

type expectedCase struct {
	Name         string
	Argv         []string
	ExpectedCode int
}

const (
	goldenExpectedOutputDir = "testdata/golden/expected_output"
	defaultReportOutputDir  = "testdata/reports/generated_expected_output"
	generatedFixtureOracle  = "bpx-generated"
)

func main() {
	outputDirFlag := flag.String("output-dir", defaultReportOutputDir, "output directory for generated expected-output fixtures")
	allowGoldenOverwrite := flag.Bool("allow-golden-overwrite", false, "allow writing directly to testdata/golden/expected_output")
	flag.Parse()
	if flag.NArg() != 0 {
		die("arguments", fmt.Errorf("unexpected positional arguments: %v", flag.Args()))
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		die("getwd", err)
	}
	outputDir := *outputDirFlag
	if !filepath.IsAbs(outputDir) {
		outputDir = filepath.Join(repoRoot, outputDir)
	}
	outputDir = filepath.Clean(outputDir)
	goldenDir := filepath.Clean(filepath.Join(repoRoot, goldenExpectedOutputDir))
	if outputDir == goldenDir && !*allowGoldenOverwrite {
		die(
			"output-dir",
			fmt.Errorf("refusing to overwrite %s without --allow-golden-overwrite; write to %s instead", goldenExpectedOutputDir, defaultReportOutputDir),
		)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		die("mkdir expected_output", err)
	}
	generatedAt := time.Now().UTC().Format(time.RFC3339)

	cases := []expectedCase{
		{Name: "find_assets_parse_recursive", Argv: []string{"find", "assets", "testdata/golden/parse", "--pattern", "*.uasset", "--recursive"}},
		{Name: "find_summary_parse_recursive", Argv: []string{"find", "summary", "testdata/golden/parse", "--recursive", "--format", "json"}},

		{Name: "info_BP_Empty", Argv: []string{"info", "testdata/golden/parse/BP_Empty.uasset"}},
		{Name: "info_BP_SimpleVars", Argv: []string{"info", "testdata/golden/parse/BP_SimpleVars.uasset"}},
		{Name: "info_DT_Simple", Argv: []string{"info", "testdata/golden/parse/DT_Simple.uasset"}},
		{Name: "info_E_Direction", Argv: []string{"info", "testdata/golden/parse/E_Direction.uasset"}},
		{Name: "info_S_PlayerData", Argv: []string{"info", "testdata/golden/parse/S_PlayerData.uasset"}},
		{Name: "dump_BP_Empty_json", Argv: []string{"dump", "testdata/golden/parse/BP_Empty.uasset", "--format", "json"}},

		{Name: "export_list_BP_Empty", Argv: []string{"export", "list", "testdata/golden/parse/BP_Empty.uasset"}},
		{Name: "export_list_BP_SimpleVars", Argv: []string{"export", "list", "testdata/golden/parse/BP_SimpleVars.uasset"}},
		{Name: "export_list_BP_ManyImports", Argv: []string{"export", "list", "testdata/golden/parse/BP_ManyImports.uasset"}},
		{Name: "export_list_DT_Simple", Argv: []string{"export", "list", "testdata/golden/parse/DT_Simple.uasset"}},
		{Name: "export_info_BP_Empty_export2", Argv: []string{"export", "info", "testdata/golden/parse/BP_Empty.uasset", "--export", "2"}},

		{Name: "import_list_BP_ManyImports", Argv: []string{"import", "list", "testdata/golden/parse/BP_ManyImports.uasset"}},
		{Name: "import_search_BP_ManyImports_object_actor", Argv: []string{"import", "search", "testdata/golden/parse/BP_ManyImports.uasset", "--object", "Actor"}},
		{Name: "import_graph_parse_group_by_root", Argv: []string{"import", "graph", "testdata/golden/parse", "--recursive", "--group-by", "root"}},
		{Name: "import_graph_parse_group_by_object", Argv: []string{"import", "graph", "testdata/golden/parse", "--recursive", "--group-by", "object"}},

		{Name: "package_meta_BP_Empty", Argv: []string{"package", "meta", "testdata/golden/parse/BP_Empty.uasset"}},
		{Name: "package_custom_versions_BP_CustomVersions", Argv: []string{"package", "custom-versions", "testdata/golden/parse/BP_CustomVersions.uasset"}},
		{Name: "package_section_soft_object_paths_BP_SoftRefs", Argv: []string{"package", "section", "testdata/golden/parse/BP_SoftRefs.uasset", "--name", "soft-object-paths"}},
		{Name: "package_section_asset_registry_BP_Empty", Argv: []string{"package", "section", "testdata/golden/parse/BP_Empty.uasset", "--name", "asset-registry"}},
		{Name: "package_depends_BP_DependsMap", Argv: []string{"package", "depends", "testdata/golden/parse/BP_DependsMap.uasset"}},
		{Name: "package_resolve_index_BP_Empty_import6", Argv: []string{"package", "resolve-index", "testdata/golden/parse/BP_Empty.uasset", "--index", "-6"}},

		{Name: "var_list_BP_SimpleVars", Argv: []string{"var", "list", "testdata/golden/parse/BP_SimpleVars.uasset"}},
		{Name: "var_list_BP_Unicode", Argv: []string{"var", "list", "testdata/golden/parse/BP_Unicode.uasset"}},
		{Name: "name_list_BP_Empty", Argv: []string{"name", "list", "testdata/golden/parse/BP_Empty.uasset"}},
		{Name: "name_add_BP_Empty_dry_run", Argv: []string{"name", "add", "testdata/golden/parse/BP_Empty.uasset", "--value", "BPX_Golden_Add", "--dry-run"}},
		{Name: "name_add_BP_Empty_hash_override_dry_run", Argv: []string{"name", "add", "testdata/golden/parse/BP_Empty.uasset", "--value", "BPX_Golden_Add_HashOverride", "--non-case-hash", "4660", "--case-preserving-hash", "43981", "--dry-run"}},
		{Name: "name_set_BP_Empty_dry_run", Argv: []string{"name", "set", "testdata/golden/parse/BP_Empty.uasset", "--index", "1", "--value", "/Script/CoreUObject_Renamed", "--dry-run"}},
		{Name: "name_set_BP_Empty_hash_override_dry_run", Argv: []string{"name", "set", "testdata/golden/parse/BP_Empty.uasset", "--index", "2", "--value", "/Script/Engine_Renamed", "--non-case-hash", "22136", "--case-preserving-hash", "39612", "--dry-run"}},
		{Name: "name_remove_BP_Empty_non_tail_fail", Argv: []string{"name", "remove", "testdata/golden/parse/BP_Empty.uasset", "--index", "1"}, ExpectedCode: 1},
		{Name: "name_remove_ref_rewrite_single_export_data_fail", Argv: []string{"name", "remove", "testdata/golden/operations/ref_rewrite_single/before.uasset", "--index", "7"}, ExpectedCode: 1},
		{Name: "datatable_read_DT_Simple", Argv: []string{"datatable", "read", "testdata/golden/parse/DT_Simple.uasset"}},
		{Name: "datatable_read_DT_Complex", Argv: []string{"datatable", "read", "testdata/golden/parse/DT_Complex.uasset"}},
		{Name: "enum_list_E_Direction", Argv: []string{"enum", "list", "testdata/golden/parse/E_Direction.uasset"}},
		{Name: "struct_definition_S_PlayerData", Argv: []string{"struct", "definition", "testdata/golden/parse/S_PlayerData.uasset"}},
		{Name: "struct_details_S_PlayerData_export2", Argv: []string{"struct", "details", "testdata/golden/parse/S_PlayerData.uasset", "--export", "2"}},
		{Name: "stringtable_read_ST_UI", Argv: []string{"stringtable", "read", "testdata/golden/parse/ST_UI.uasset"}},
		{Name: "class_BP_Empty_export2", Argv: []string{"class", "testdata/golden/parse/BP_Empty.uasset", "--export", "2"}},
		{Name: "level_info_L_Minimal_export1", Argv: []string{"level", "info", "testdata/golden/parse/L_Minimal.umap", "--export", "1"}},
		{Name: "level_var_list_L_Minimal_LyraWorldSettings", Argv: []string{"level", "var-list", "testdata/golden/parse/L_Minimal.umap", "--actor", "LyraWorldSettings"}},
		{Name: "raw_BP_Empty_export2", Argv: []string{"raw", "testdata/golden/parse/BP_Empty.uasset", "--export", "2"}},

		{Name: "prop_list_BP_Empty_export1", Argv: []string{"prop", "list", "testdata/golden/parse/BP_Empty.uasset", "--export", "1"}},
		{Name: "prop_list_BP_AllScalarTypes_export1", Argv: []string{"prop", "list", "testdata/golden/parse/BP_AllScalarTypes.uasset", "--export", "1"}},
		{Name: "prop_list_BP_Containers_export1", Argv: []string{"prop", "list", "testdata/golden/parse/BP_Containers.uasset", "--export", "1"}},
		{Name: "prop_list_BP_MathTypes_export1", Argv: []string{"prop", "list", "testdata/golden/parse/BP_MathTypes.uasset", "--export", "1"}},
		{Name: "prop_list_BP_Nested_export1", Argv: []string{"prop", "list", "testdata/golden/parse/BP_Nested.uasset", "--export", "1"}},
		{Name: "prop_list_BP_RefTypes_export1", Argv: []string{"prop", "list", "testdata/golden/parse/BP_RefTypes.uasset", "--export", "1"}},
		{Name: "prop_list_BP_GameplayTags_export1", Argv: []string{"prop", "list", "testdata/golden/parse/BP_GameplayTags.uasset", "--export", "1"}},
		{Name: "prop_list_BP_SoftRefs_export1", Argv: []string{"prop", "list", "testdata/golden/parse/BP_SoftRefs.uasset", "--export", "1"}},
		{Name: "prop_list_BP_WithFunctions_export1", Argv: []string{"prop", "list", "testdata/golden/parse/BP_WithFunctions.uasset", "--export", "1"}},
		{Name: "prop_list_S_PlayerData_export2", Argv: []string{"prop", "list", "testdata/golden/parse/S_PlayerData.uasset", "--export", "2"}},
		{Name: "prop_list_prop_set_custom_struct_int_before_export5", Argv: []string{"prop", "list", "testdata/golden/operations/prop_set_custom_struct_int/before.uasset", "--export", "5"}},
		{Name: "prop_list_prop_set_custom_struct_int_after_export5", Argv: []string{"prop", "list", "testdata/golden/operations/prop_set_custom_struct_int/after.uasset", "--export", "5"}},
		{Name: "prop_list_prop_set_custom_struct_enum_before_export5", Argv: []string{"prop", "list", "testdata/golden/operations/prop_set_custom_struct_enum/before.uasset", "--export", "5"}},
		{Name: "prop_list_prop_set_custom_struct_enum_after_export5", Argv: []string{"prop", "list", "testdata/golden/operations/prop_set_custom_struct_enum/after.uasset", "--export", "5"}},

		{Name: "localization_read_BP_Empty_StringTableRef_export11", Argv: []string{"localization", "read", "testdata/golden/parse/BP_Empty_StringTableRef.uasset", "--export", "11", "--include-history"}},
		{Name: "localization_query_BP_Empty_StringTableRef_export11_key_UI_Start", Argv: []string{"localization", "query", "testdata/golden/parse/BP_Empty_StringTableRef.uasset", "--export", "11", "--key", "UI.Start"}},
		{Name: "localization_resolve_BP_Empty_StringTableRef_locres", Argv: []string{"localization", "resolve", "testdata/golden/parse/BP_Empty_StringTableRef.uasset", "--export", "11", "--culture", "ja", "--locres", "testdata/golden/parse/Localization_Test.locres"}},
		{Name: "localization_resolve_BP_Empty_PackageNamespace_locres", Argv: []string{"localization", "resolve", "testdata/golden/parse/BP_Empty_PackageNamespace.uasset", "--export", "11", "--culture", "ja", "--locres", "testdata/golden/parse/Localization_Test.locres"}},
		{Name: "blueprint_info_BP_WithFunctions", Argv: []string{"blueprint", "info", "testdata/golden/parse/BP_WithFunctions.uasset"}},
		{Name: "blueprint_bytecode_BP_WithFunctions_export5", Argv: []string{"blueprint", "bytecode", "testdata/golden/parse/BP_WithFunctions.uasset", "--export", "5"}},
		{Name: "blueprint_disasm_BP_WithFunctions_export5_json", Argv: []string{"blueprint", "disasm", "testdata/golden/parse/BP_WithFunctions.uasset", "--export", "5", "--format", "json"}},
		{Name: "blueprint_trace_BP_Empty_K2Node_Event_0_to_UserConstructionScript", Argv: []string{"blueprint", "trace", "testdata/golden/parse/BP_Empty.uasset", "--from", "K2Node_Event_0", "--to-function", "UserConstructionScript"}},
		{Name: "blueprint_call_args_BP_Empty_member_OpenLevelBySoftObjectPtr", Argv: []string{"blueprint", "call-args", "testdata/golden/parse/BP_Empty.uasset", "--member", "OpenLevelBySoftObjectPtr"}},
		{Name: "blueprint_refs_BP_Empty_softpath_L_TestTitle", Argv: []string{"blueprint", "refs", "testdata/golden/parse/BP_Empty.uasset", "--soft-path", "/Game/BPXFixtures/Maps/L_TestTitle"}},
		{Name: "blueprint_search_BP_Empty_class_K2Node_Event_show_NodePos_Function", Argv: []string{"blueprint", "search", "testdata/golden/parse/BP_Empty.uasset", "--class", "K2Node_Event", "--show", "NodePos,Function"}},
		{Name: "blueprint_scan_functions_parse_recursive", Argv: []string{"blueprint", "scan-functions", "testdata/golden/parse", "--recursive"}},
		{Name: "blueprint_scan_functions_parse_recursive_aggregate", Argv: []string{"blueprint", "scan-functions", "testdata/golden/parse", "--recursive", "--aggregate"}},
		{Name: "metadata_BP_WithMetadata_export1", Argv: []string{"metadata", "testdata/golden/parse/BP_WithMetadata.uasset", "--export", "1"}},
		{Name: "validate_BP_Empty", Argv: []string{"validate", "testdata/golden/parse/BP_Empty.uasset"}},
		{Name: "validate_BP_Empty_binary_equality", Argv: []string{"validate", "testdata/golden/parse/BP_Empty.uasset", "--binary-equality"}},
	}

	for _, tc := range cases {
		if err := runCase(outputDir, generatedAt, tc); err != nil {
			die(tc.Name, err)
		}
	}
}

func runCase(outputDir, generatedAt string, tc expectedCase) error {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run(tc.Argv, &stdout, &stderr)
	if code != tc.ExpectedCode {
		return fmt.Errorf("command failed (code=%d expected=%d): stderr=%s", code, tc.ExpectedCode, stderr.String())
	}

	payload := map[string]any{
		"oracle":      generatedFixtureOracle,
		"generatedBy": "scripts/gen_expected_output",
		"generatedAt": generatedAt,
		"name":        tc.Name,
		"argv":        tc.Argv,
	}
	if tc.ExpectedCode != 0 {
		payload["expectedCode"] = tc.ExpectedCode
		payload["expectedStderr"] = strings.TrimSpace(stderr.String())
	} else {
		var expected any
		if err := json.Unmarshal(stdout.Bytes(), &expected); err != nil {
			return fmt.Errorf("decode command output: %w", err)
		}
		payload["expected"] = expected
	}
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal expected fixture: %w", err)
	}
	body = append(body, '\n')

	fileName := sanitize(tc.Name) + ".json"
	path := filepath.Join(outputDir, fileName)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("write expected fixture: %w", err)
	}
	return nil
}

var nonFileChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func sanitize(name string) string {
	out := nonFileChars.ReplaceAllString(name, "_")
	if out == "" {
		return "expected"
	}
	return out
}

func die(context string, err error) {
	fmt.Fprintf(os.Stderr, "error: %s: %v\n", context, err)
	os.Exit(1)
}
