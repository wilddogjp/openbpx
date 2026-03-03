package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/wilddogjp/openbpx/pkg/edit"
	"github.com/wilddogjp/openbpx/pkg/uasset"
)

func runName(args []string, stdout, stderr io.Writer) int {
	return dispatchSubcommand(
		args,
		stdout,
		stderr,
		"usage: bpx name <list|add|set|remove> ...",
		"unknown name command: %s\n",
		subcommandSpec{Name: "list", Run: runNameList},
		subcommandSpec{Name: "add", Run: runNameAdd},
		subcommandSpec{Name: "set", Run: runNameSet},
		subcommandSpec{Name: "remove", Run: runNameRemove},
	)
}

func runNameList(args []string, stdout, stderr io.Writer) int {
	file, asset, ok := parseSingleAssetCommand(args, "name list", "usage: bpx name list <file.uasset>", stderr)
	if !ok {
		return 1
	}
	items := make([]map[string]any, 0, len(asset.Names))
	for i, n := range asset.Names {
		items = append(items, map[string]any{
			"index":              i,
			"value":              n.Value,
			"nonCaseHash":        n.NonCaseHash,
			"casePreservingHash": n.CasePreservingHash,
		})
	}
	return printJSON(stdout, map[string]any{
		"file":  file,
		"count": len(items),
		"names": items,
	})
}

func runNameAdd(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("name add", stderr)
	opts := registerCommonFlags(fs)
	value := fs.String("value", "", "name string value")
	nonCaseHash := fs.String("non-case-hash", "", "optional uint16 override (0-65535)")
	caseHash := fs.String("case-preserving-hash", "", "optional uint16 override (0-65535)")
	dryRun := fs.Bool("dry-run", false, "do not write output")
	backup := fs.Bool("backup", false, "create <file>.backup before overwrite")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || strings.TrimSpace(*value) == "" {
		fmt.Fprintln(stderr, "usage: bpx name add <file.uasset> --value <name> [--non-case-hash <u16>] [--case-preserving-hash <u16>] [--dry-run] [--backup]")
		return 1
	}

	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	nameValue := strings.TrimSpace(*value)
	if idx := findNameIndexByValue(asset.Names, nameValue); idx >= 0 {
		fmt.Fprintf(stderr, "error: name %q already exists at index %d\n", nameValue, idx)
		return 1
	}

	nonCase, casePreserving, err := resolveNameHashes(nameValue, *nonCaseHash, *caseHash)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	updatedNames := append(make([]uasset.NameEntry, 0, len(asset.Names)+1), asset.Names...)
	updatedNames = append(updatedNames, uasset.NameEntry{
		Value:              nameValue,
		NonCaseHash:        nonCase,
		CasePreservingHash: casePreserving,
	})
	outBytes, err := edit.RewriteNameMap(asset, updatedNames)
	if err != nil {
		fmt.Fprintf(stderr, "error: rewrite name map: %v\n", err)
		return 1
	}
	changed := !bytes.Equal(asset.Raw.Bytes, outBytes)

	resp := map[string]any{
		"file":               file,
		"dryRun":             *dryRun,
		"changed":            changed,
		"oldCount":           len(asset.Names),
		"newCount":           len(updatedNames),
		"addedIndex":         len(updatedNames) - 1,
		"value":              nameValue,
		"nonCaseHash":        nonCase,
		"casePreservingHash": casePreserving,
		"outputBytes":        len(outBytes),
	}
	if *dryRun {
		return printJSON(stdout, resp)
	}
	if *backup {
		if err := createBackupFile(file); err != nil {
			fmt.Fprintf(stderr, "error: backup source file: %v\n", err)
			return 1
		}
	}
	if err := os.WriteFile(file, outBytes, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write file: %v\n", err)
		return 1
	}
	return printJSON(stdout, resp)
}

func runNameSet(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("name set", stderr)
	opts := registerCommonFlags(fs)
	index := fs.Int("index", -1, "0-based NameMap index")
	value := fs.String("value", "", "name string value")
	nonCaseHash := fs.String("non-case-hash", "", "optional uint16 override (0-65535)")
	caseHash := fs.String("case-preserving-hash", "", "optional uint16 override (0-65535)")
	dryRun := fs.Bool("dry-run", false, "do not write output")
	backup := fs.Bool("backup", false, "create <file>.backup before overwrite")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || *index < 0 || strings.TrimSpace(*value) == "" {
		fmt.Fprintln(stderr, "usage: bpx name set <file.uasset> --index <n> --value <name> [--non-case-hash <u16>] [--case-preserving-hash <u16>] [--dry-run] [--backup]")
		return 1
	}

	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if *index >= len(asset.Names) {
		fmt.Fprintf(stderr, "error: index out of range: %d (nameCount=%d)\n", *index, len(asset.Names))
		return 1
	}

	newValue := strings.TrimSpace(*value)
	nonCase, casePreserving, err := resolveNameHashes(newValue, *nonCaseHash, *caseHash)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	oldEntry := asset.Names[*index]
	updatedNames := append(make([]uasset.NameEntry, 0, len(asset.Names)), asset.Names...)
	updatedNames[*index] = uasset.NameEntry{
		Value:              newValue,
		NonCaseHash:        nonCase,
		CasePreservingHash: casePreserving,
	}
	outBytes, err := edit.RewriteNameMap(asset, updatedNames)
	if err != nil {
		fmt.Fprintf(stderr, "error: rewrite name map: %v\n", err)
		return 1
	}
	changed := !bytes.Equal(asset.Raw.Bytes, outBytes)

	resp := map[string]any{
		"file":        file,
		"dryRun":      *dryRun,
		"changed":     changed,
		"index":       *index,
		"oldEntry":    toNameEntryJSON(oldEntry),
		"newEntry":    toNameEntryJSON(updatedNames[*index]),
		"outputBytes": len(outBytes),
	}
	if *dryRun {
		return printJSON(stdout, resp)
	}
	if *backup {
		if err := createBackupFile(file); err != nil {
			fmt.Fprintf(stderr, "error: backup source file: %v\n", err)
			return 1
		}
	}
	if err := os.WriteFile(file, outBytes, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write file: %v\n", err)
		return 1
	}
	return printJSON(stdout, resp)
}

func runNameRemove(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("name remove", stderr)
	opts := registerCommonFlags(fs)
	index := fs.Int("index", -1, "0-based NameMap index")
	dryRun := fs.Bool("dry-run", false, "do not write output")
	backup := fs.Bool("backup", false, "create <file>.backup before overwrite")
	if err := parseFlagSet(fs, args); err != nil {
		return 1
	}
	if fs.NArg() != 1 || *index < 0 {
		fmt.Fprintln(stderr, "usage: bpx name remove <file.uasset> --index <n> [--dry-run] [--backup]")
		return 1
	}

	file := fs.Arg(0)
	asset, err := uasset.ParseFile(file, *opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if *index >= len(asset.Names) {
		fmt.Fprintf(stderr, "error: index out of range: %d (nameCount=%d)\n", *index, len(asset.Names))
		return 1
	}

	if *index != len(asset.Names)-1 {
		fmt.Fprintln(stderr, "error: name remove currently supports tail entry only (--index must be nameCount-1)")
		return 1
	}
	if int32(*index) < asset.Summary.NamesReferencedFromExportDataCount {
		fmt.Fprintf(stderr, "error: index %d is inside NamesReferencedFromExportData region (%d) and cannot be removed safely\n", *index, asset.Summary.NamesReferencedFromExportDataCount)
		return 1
	}
	if where, referenced := findNameReferenceInImportExportMap(asset, int32(*index)); referenced {
		fmt.Fprintf(stderr, "error: index %d is still referenced in %s\n", *index, where)
		return 1
	}

	removed := asset.Names[*index]
	updatedNames := append(make([]uasset.NameEntry, 0, len(asset.Names)-1), asset.Names[:*index]...)
	outBytes, err := edit.RewriteNameMap(asset, updatedNames)
	if err != nil {
		fmt.Fprintf(stderr, "error: rewrite name map: %v\n", err)
		return 1
	}
	changed := !bytes.Equal(asset.Raw.Bytes, outBytes)

	resp := map[string]any{
		"file":         file,
		"dryRun":       *dryRun,
		"changed":      changed,
		"oldCount":     len(asset.Names),
		"newCount":     len(updatedNames),
		"removedIndex": *index,
		"removedEntry": toNameEntryJSON(removed),
		"outputBytes":  len(outBytes),
	}
	if *dryRun {
		return printJSON(stdout, resp)
	}
	if *backup {
		if err := createBackupFile(file); err != nil {
			fmt.Fprintf(stderr, "error: backup source file: %v\n", err)
			return 1
		}
	}
	if err := os.WriteFile(file, outBytes, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write file: %v\n", err)
		return 1
	}
	return printJSON(stdout, resp)
}

func resolveNameHashes(nameValue, nonCaseRaw, caseRaw string) (uint16, uint16, error) {
	nonCaseAuto, caseAuto := edit.ComputeNameEntryHashesUE56(nameValue)
	nonCase := nonCaseAuto
	casePreserving := caseAuto
	if strings.TrimSpace(nonCaseRaw) != "" {
		v, err := parseUint16(nonCaseRaw)
		if err != nil {
			return 0, 0, fmt.Errorf("parse --non-case-hash: %w", err)
		}
		nonCase = v
	}
	if strings.TrimSpace(caseRaw) != "" {
		v, err := parseUint16(caseRaw)
		if err != nil {
			return 0, 0, fmt.Errorf("parse --case-preserving-hash: %w", err)
		}
		casePreserving = v
	}
	return nonCase, casePreserving, nil
}

func parseUint16(raw string) (uint16, error) {
	n, err := strconv.ParseUint(strings.TrimSpace(raw), 0, 16)
	if err != nil {
		return 0, err
	}
	return uint16(n), nil
}

func findNameIndexByValue(entries []uasset.NameEntry, needle string) int {
	for i, entry := range entries {
		if entry.Value == needle {
			return i
		}
	}
	return -1
}

func toNameEntryJSON(entry uasset.NameEntry) map[string]any {
	return map[string]any{
		"value":              entry.Value,
		"nonCaseHash":        entry.NonCaseHash,
		"casePreservingHash": entry.CasePreservingHash,
	}
}

func findNameReferenceInImportExportMap(asset *uasset.Asset, idx int32) (string, bool) {
	for i, imp := range asset.Imports {
		if imp.ClassPackage.Index == idx {
			return fmt.Sprintf("import[%d].ClassPackage", i+1), true
		}
		if imp.ClassName.Index == idx {
			return fmt.Sprintf("import[%d].ClassName", i+1), true
		}
		if imp.ObjectName.Index == idx {
			return fmt.Sprintf("import[%d].ObjectName", i+1), true
		}
		if imp.PackageName.Index == idx {
			return fmt.Sprintf("import[%d].PackageName", i+1), true
		}
	}
	for i, exp := range asset.Exports {
		if exp.ObjectName.Index == idx {
			return fmt.Sprintf("export[%d].ObjectName", i+1), true
		}
	}
	return "", false
}
