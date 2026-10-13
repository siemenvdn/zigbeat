package config

import (
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Set gives key (such as budget.max_rate) value and, unless comment is empty, that line comment.
func (f *File) Set(key, value, comment string) error {
	byHand := func(why string) error { return fmt.Errorf("set %s to %s by hand: %s", key, value, why) }
	m, err := f.root()
	if err != nil {
		return err
	}
	parts := strings.Split(key, ".")
	for i, p := range parts {
		_, v := find(m, p)
		switch {
		case v == nil && m.Style&yaml.FlowStyle != 0:
			return byHand("it is missing from a flow mapping")
		case v == nil:
			f.insert(m, i == 0, parts[i:], value, comment)
			return nil
		case i == len(parts)-1:
			return f.replace(v, value, comment, byHand)
		case v.Kind != yaml.MappingNode:
			return byHand(p + " is not a mapping")
		}
		m = v
	}
	return nil
}

func (f *File) replace(v *yaml.Node, value, comment string, byHand func(string) error) error {
	if v.Kind != yaml.ScalarNode || v.Style != 0 || v.Value == "" {
		return byHand("the value is quoted, empty or not a single value")
	}
	i := v.Line - 1
	line := f.lines[i]
	start := byteCol(line, v.Column)
	if !strings.HasPrefix(line[start:], v.Value) {
		return byHand("the value spans several lines")
	}
	col := commentAt(line, start)
	end := start + len(value)
	line = align(line[:start]+value+line[start+len(v.Value):], end, col)
	if comment != "" {
		line = withComment(line, end, comment, col)
	}
	f.lines[i] = line
	return nil
}

func (f *File) insert(m *yaml.Node, root bool, keys []string, value, comment string) {
	at, indent := len(f.lines), ""
	var add []string
	if root {
		add = append(add, "")
	} else {
		at, indent = lastLine(m), strings.Repeat(" ", m.Column-1)
	}
	for i, k := range keys {
		add = append(add, indent+strings.Repeat("  ", i)+k+":")
	}
	last := add[len(add)-1] + " " + value
	if comment != "" {
		last += "  # " + comment
	}
	add[len(add)-1] = last
	f.lines = slices.Concat(f.lines[:at], add, f.lines[at:])
}

// SetBlock replaces the top-level key and its value with text.
func (f *File) SetBlock(key, text string) error {
	m, err := f.root()
	if err != nil {
		return err
	}
	var add []string
	if text != "" {
		add = split([]byte(text))
	}
	i := keyIndex(m, key)
	if i < 0 {
		if text != "" {
			f.lines = slices.Concat(f.lines, []string{""}, add)
		}
		return nil
	}
	from, to := m.Content[i].Line-1, f.blockEnd(m, i)
	if text == "" && to < len(f.lines) && strings.TrimSpace(f.lines[to]) == "" {
		to++
	}
	f.lines = slices.Concat(f.lines[:from], add, f.lines[to:])
	return nil
}

func (f *File) blockEnd(m *yaml.Node, i int) int {
	end := len(f.lines)
	if next := i + 2; next < len(m.Content) {
		end = m.Content[next].Line - 1
	}
	for end > 0 && filler(f.lines[end-1]) {
		end--
	}
	return end
}

func filler(line string) bool {
	t := strings.TrimSpace(line)
	return t == "" || strings.HasPrefix(t, "#")
}
