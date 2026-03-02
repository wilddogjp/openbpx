package validate

import (
	"bytes"
	"fmt"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

// Check is one validation check result.
type Check struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Details string `json:"details"`
}

// Report is the complete validation result.
type Report struct {
	OK     bool    `json:"ok"`
	Checks []Check `json:"checks"`
}

// Run executes validation checks for read/analysis safety.
func Run(asset *uasset.Asset, withBinaryEquality bool) Report {
	if asset == nil {
		return Report{
			OK: false,
			Checks: []Check{
				{
					Name:    "asset-present",
					Passed:  false,
					Details: "asset is nil",
				},
			},
		}
	}

	checks := make([]Check, 0, 8)
	checks = append(checks, checkCounts(asset))
	checks = append(checks, checkOffsets(asset))
	checks = append(checks, checkExportSerialRanges(asset))
	checks = append(checks, checkNameReferences(asset))
	checks = append(checks, checkUEVersion(asset))
	if withBinaryEquality {
		checks = append(checks, checkBinaryEquality(asset))
	}

	ok := true
	for _, c := range checks {
		if !c.Passed {
			ok = false
			break
		}
	}
	return Report{OK: ok, Checks: checks}
}

func checkCounts(asset *uasset.Asset) Check {
	s := asset.Summary
	match := int(s.NameCount) == len(asset.Names) && int(s.ImportCount) == len(asset.Imports) && int(s.ExportCount) == len(asset.Exports)
	if !match {
		return Check{
			Name:    "table-counts",
			Passed:  false,
			Details: fmt.Sprintf("summary(name=%d, import=%d, export=%d) parsed(name=%d, import=%d, export=%d)", s.NameCount, s.ImportCount, s.ExportCount, len(asset.Names), len(asset.Imports), len(asset.Exports)),
		}
	}
	return Check{Name: "table-counts", Passed: true, Details: "counts are consistent"}
}

func checkOffsets(asset *uasset.Asset) Check {
	s := asset.Summary
	sz := len(asset.Raw.Bytes)
	if s.NameOffset < 0 || int(s.NameOffset) > sz {
		return Check{Name: "table-offsets", Passed: false, Details: fmt.Sprintf("nameOffset out of range: %d", s.NameOffset)}
	}
	if s.ImportOffset < 0 || int(s.ImportOffset) > sz {
		return Check{Name: "table-offsets", Passed: false, Details: fmt.Sprintf("importOffset out of range: %d", s.ImportOffset)}
	}
	if s.ExportOffset < 0 || int(s.ExportOffset) > sz {
		return Check{Name: "table-offsets", Passed: false, Details: fmt.Sprintf("exportOffset out of range: %d", s.ExportOffset)}
	}
	return Check{Name: "table-offsets", Passed: true, Details: "core table offsets are within file"}
}

func checkExportSerialRanges(asset *uasset.Asset) Check {
	sz := int64(len(asset.Raw.Bytes))
	for i, exp := range asset.Exports {
		if exp.SerialOffset < 0 || exp.SerialSize < 0 {
			return Check{Name: "export-serial-ranges", Passed: false, Details: fmt.Sprintf("export %d has negative serial range", i+1)}
		}
		end := exp.SerialOffset + exp.SerialSize
		if end < exp.SerialOffset || end > sz {
			return Check{Name: "export-serial-ranges", Passed: false, Details: fmt.Sprintf("export %d serial range out of bounds (%d..%d)", i+1, exp.SerialOffset, end)}
		}
	}
	return Check{Name: "export-serial-ranges", Passed: true, Details: "all export serial ranges are valid"}
}

func checkNameReferences(asset *uasset.Asset) Check {
	max := len(asset.Names)
	for i, imp := range asset.Imports {
		for _, ref := range []uasset.NameRef{imp.ClassPackage, imp.ClassName, imp.ObjectName} {
			if ref.Index < 0 || int(ref.Index) >= max {
				return Check{Name: "name-references", Passed: false, Details: fmt.Sprintf("import %d has invalid name index %d", i+1, ref.Index)}
			}
		}
	}
	for i, exp := range asset.Exports {
		if exp.ObjectName.Index < 0 || int(exp.ObjectName.Index) >= max {
			return Check{Name: "name-references", Passed: false, Details: fmt.Sprintf("export %d has invalid object name index %d", i+1, exp.ObjectName.Index)}
		}
	}
	return Check{Name: "name-references", Passed: true, Details: "all name references are in range"}
}

func checkUEVersion(asset *uasset.Asset) Check {
	s := asset.Summary
	if s.FileVersionUE5 < 1000 || s.FileVersionUE5 > 1017 {
		return Check{Name: "ue-version-window", Passed: false, Details: fmt.Sprintf("unexpected FileVersionUE5=%d", s.FileVersionUE5)}
	}
	return Check{Name: "ue-version-window", Passed: true, Details: fmt.Sprintf("FileVersionUE5=%d", s.FileVersionUE5)}
}

func checkBinaryEquality(asset *uasset.Asset) Check {
	original := asset.Raw.Bytes
	serialized := asset.Raw.SerializeUnmodified()
	if !bytes.Equal(original, serialized) {
		return Check{Name: "binary-equality", Passed: false, Details: "no-op serialization produced different bytes"}
	}

	opts := uasset.DefaultParseOptions()
	// Reparse checks structural consistency of the serialized bytes only.
	reparsed, err := uasset.ParseBytes(serialized, opts)
	if err != nil {
		return Check{Name: "binary-equality", Passed: false, Details: fmt.Sprintf("reparse after no-op serialization failed: %v", err)}
	}
	if len(reparsed.Names) != len(asset.Names) ||
		len(reparsed.Imports) != len(asset.Imports) ||
		len(reparsed.Exports) != len(asset.Exports) {
		return Check{
			Name:   "binary-equality",
			Passed: false,
			Details: fmt.Sprintf(
				"reparse table mismatch after no-op serialization (name %d/%d import %d/%d export %d/%d)",
				len(reparsed.Names), len(asset.Names),
				len(reparsed.Imports), len(asset.Imports),
				len(reparsed.Exports), len(asset.Exports),
			),
		}
	}
	return Check{Name: "binary-equality", Passed: true, Details: "no-op bytes are equal and reparsed tables are consistent"}
}
