package config

import (
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Set gives key (such as budget.max_rate) value and, unless comment is empty,
// that line comment. A missing key or block is added. A value Set cannot
// replace safely is refused, with what to set by hand.
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

// replace swaps the plain scalar v for value and keeps the comment's column.
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
	line = align(line[:start]+value+line[start+len(v.Value):], start, col)
	if comment != "" {
		line = withComment(line, start, comment, col)
	}
	f.lines[i] = line
	return nil
}

// insert adds the missing keys, the last one with value, after the last line
// of block mapping m, or at the end of the file when m is the root.
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

// SetBlock replaces the top-level key and its value with text; an empty text
// removes them, with one blank line after them. A missing key is added at the end.
// Comments above the key stay, comments inside its value go.
func (f *File) SetBlock(key, text string) error {
	m, err := f.root()
	if err != nil {
		return err
	}
	var add []string
	if text != "" {
		add = split([]byte(text))
	}
	k, v := find(m, key)
	if k == nil {
		if text != "" {
			f.lines = slices.Concat(f.lines, []string{""}, add)
		}
		return nil
	}
	from, to := k.Line-1, lastLine(v)
	if text == "" && to < len(f.lines) && strings.TrimSpace(f.lines[to]) == "" {
		to++
	}
	f.lines = slices.Concat(f.lines[:from], add, f.lines[to:])
	return nil
}
