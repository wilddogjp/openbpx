package uasset

import "fmt"

// NameEntry mirrors one serialized FNameEntrySerialized entry from NameMap.
type NameEntry struct {
	Value              string `json:"value"`
	NonCaseHash        uint16 `json:"nonCaseHash"`
	CasePreservingHash uint16 `json:"casePreservingHash"`
}

// NameRef is one serialized FName (index + number).
type NameRef struct {
	Index  int32 `json:"index"`
	Number int32 `json:"number"`
}

// Display resolves name text (best effort) and appends suffix if numbered.
func (n NameRef) Display(names []NameEntry) string {
	if n.Index < 0 || int(n.Index) >= len(names) {
		return fmt.Sprintf("<bad-name:%d>", n.Index)
	}
	base := names[n.Index].Value
	if n.Number <= 0 {
		return base
	}
	return fmt.Sprintf("%s_%d", base, n.Number-1)
}

// IsNone reports whether this is NAME_None with no number.
func (n NameRef) IsNone(names []NameEntry) bool {
	if n.Number != 0 || n.Index < 0 || int(n.Index) >= len(names) {
		return false
	}
	return names[n.Index].Value == "None"
}
