package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// newFileMode is the mode of a config that did not exist: it holds no secrets,
// only the names of the variables that do.
const newFileMode = 0o644

// File is a configuration file being edited in place: only the changed values
// move; comments, blank lines and alignment stay byte for byte.
type File struct {
	path  string
	lines []string // without their newlines
	mode  os.FileMode
}

// OpenFile reads the configuration at path for editing.
func OpenFile(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	f := &File{path: path, lines: split(data), mode: info.Mode().Perm()}
	_, err = f.root()
	return f, err
}

// NewFile starts a configuration that does not exist yet.
func NewFile(path string, data []byte) *File {
	return &File{path: path, lines: split(data), mode: newFileMode}
}

func split(data []byte) []string { return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") }

// Bytes is the file as Save would write it.
func (f *File) Bytes() []byte { return []byte(strings.Join(f.lines, "\n") + "\n") }

// root parses the current lines and returns the top-level mapping. Every edit
// parses again, so positions are always fresh.
func (f *File) root() (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(f.Bytes(), &doc); err != nil {
		return nil, fmt.Errorf("%s: %v", f.path, err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s is not a YAML mapping", f.path)
	}
	return doc.Content[0], nil
}

// Save writes the file when check accepts the new bytes: to a temporary file
// next to it, with the same mode, renamed over the original.
func (f *File) Save(check func([]byte) error) error {
	data := f.Bytes()
	if err := check(data); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(f.path), "."+filepath.Base(f.path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // only left behind when the rename failed
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	// Without it a power cut right after the rename can leave an empty config.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), f.mode); err != nil {
		return err
	}
	// A config behind a symlink becomes a plain file; follow the link if anyone keeps theirs in one.
	return os.Rename(tmp.Name(), f.path)
}
