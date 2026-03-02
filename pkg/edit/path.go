package edit

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type pathOpKind int

const (
	pathOpField pathOpKind = iota
	pathOpSubscript
)

type pathOp struct {
	Kind      pathOpKind
	FieldName string
	Subscript any
}

func parsePath(path string) ([]pathOp, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("path is empty")
	}

	ops := make([]pathOp, 0, 8)
	i := 0

	readIdent := func() (string, error) {
		if i >= len(path) {
			return "", fmt.Errorf("expected identifier")
		}
		start := i
		for i < len(path) {
			r := rune(path[i])
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
				i++
				continue
			}
			break
		}
		if start == i {
			return "", fmt.Errorf("expected identifier at %d", i)
		}
		return path[start:i], nil
	}

	readSubscript := func() (any, error) {
		if i >= len(path) || path[i] != '[' {
			return nil, fmt.Errorf("expected '['")
		}
		i++
		start := i
		depth := 1
		for i < len(path) {
			switch path[i] {
			case '[':
				depth++
			case ']':
				depth--
				if depth == 0 {
					raw := strings.TrimSpace(path[start:i])
					i++
					if raw == "" {
						return nil, fmt.Errorf("empty subscript")
					}
					if idx, err := strconv.Atoi(raw); err == nil {
						return idx, nil
					}
					var v any
					if err := json.Unmarshal([]byte(raw), &v); err != nil {
						return nil, fmt.Errorf("subscript is not valid int/json literal: %q", raw)
					}
					return v, nil
				}
			}
			i++
		}
		return nil, fmt.Errorf("unterminated subscript")
	}

	root, err := readIdent()
	if err != nil {
		return nil, err
	}
	ops = append(ops, pathOp{Kind: pathOpField, FieldName: root})

	for i < len(path) {
		switch path[i] {
		case '.':
			i++
			ident, err := readIdent()
			if err != nil {
				return nil, err
			}
			ops = append(ops, pathOp{Kind: pathOpField, FieldName: ident})
		case '[':
			s, err := readSubscript()
			if err != nil {
				return nil, err
			}
			ops = append(ops, pathOp{Kind: pathOpSubscript, Subscript: s})
		default:
			return nil, fmt.Errorf("unexpected token %q at %d", string(path[i]), i)
		}
	}

	return ops, nil
}
