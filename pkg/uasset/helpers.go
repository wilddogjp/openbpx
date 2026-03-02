package uasset

import "fmt"

// ResolveExportIndex converts 1-based export index to zero-based index.
func (a *Asset) ResolveExportIndex(index int) (int, error) {
	if index <= 0 || index > len(a.Exports) {
		return 0, fmt.Errorf("export index out of range: %d (1..%d)", index, len(a.Exports))
	}
	return index - 1, nil
}

// ResolveImportIndex converts 1-based import index to zero-based index.
func (a *Asset) ResolveImportIndex(index int) (int, error) {
	if index <= 0 || index > len(a.Imports) {
		return 0, fmt.Errorf("import index out of range: %d (1..%d)", index, len(a.Imports))
	}
	return index - 1, nil
}

// ResolveClassName returns class object name for one export.
func (a *Asset) ResolveClassName(exp ExportEntry) string {
	if exp.ClassIndex == 0 {
		return "Class"
	}
	idx := exp.ClassIndex.ResolveIndex()
	if exp.ClassIndex > 0 {
		if idx >= 0 && idx < len(a.Exports) {
			return a.Exports[idx].ObjectName.Display(a.Names)
		}
		return "<bad-export-class-index>"
	}
	if idx >= 0 && idx < len(a.Imports) {
		return a.Imports[idx].ObjectName.Display(a.Names)
	}
	return "<bad-import-class-index>"
}
