package config

import (
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

func find(m *yaml.Node, key string) (k, v *yaml.Node) {
	if i := keyIndex(m, key); i >= 0 {
		return m.Content[i], m.Content[i+1]
	}
	return nil, nil
}

// keyIndex is the index of key in m.Content, or -1.
func keyIndex(m *yaml.Node, key string) int {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return i
		}
	}
	return -1
}

// lastLine is the last line n covers, 1-based: the 0-based index right after it.
func lastLine(n *yaml.Node) int {
	l := n.Line
	for _, c := range n.Content {
		l = max(l, lastLine(c))
	}
	return l
}

// byteCol turns yaml's 1-based column, counted in runes, into a byte index.
func byteCol(s string, col int) int {
	i := 0
	for n := 1; n < col && i < len(s); n++ {
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
	}
	return i
}

// commentAt is the byte index of the line comment after from, or -1. A plain
// scalar cannot hold " #", so the first one after the value starts the comment.
func commentAt(s string, from int) int {
	if i := strings.Index(s[from:], " #"); i >= 0 {
		return from + i + 1
	}
	return -1
}

// align moves the comment of s back to byte column col when the padding allows,
// keeping at least one space before it.
func align(s string, from, col int) string {
	i := commentAt(s, from)
	if i < 0 || col < 0 {
		return s
	}
	code := strings.TrimRight(s[:i], " ")
	return code + strings.Repeat(" ", max(col-len(code), 1)) + s[i:]
}

// withComment replaces the comment of s, at column col when there was one.
func withComment(s string, from int, comment string, col int) string {
	if i := commentAt(s, from); i >= 0 {
		s = s[:i]
	}
	code := strings.TrimRight(s, " ")
	pad := "  "
	if col >= 0 {
		pad = strings.Repeat(" ", max(col-len(code), 1))
	}
	return code + pad + "# " + comment
}
