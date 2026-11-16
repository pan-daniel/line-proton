package filesystem

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/pan-daniel/line-proton/internal/shared/sequence"
)

type DirectoryContents struct {
	directory string
	kept      []string
}

func NewDirectoryContents(directory string, kept ...string) DirectoryContents {
	return DirectoryContents{directory: directory, kept: slices.Clone(kept)}
}

func (contents DirectoryContents) Remove() error {
	entries, err := os.ReadDir(contents.directory)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return sequence.ForEach(slices.Values(entries), contents.removeUnlessKept)
}

func (contents DirectoryContents) removeUnlessKept(entry fs.DirEntry) error {
	if slices.Contains(contents.kept, entry.Name()) {
		return nil
	}
	return os.RemoveAll(filepath.Join(contents.directory, entry.Name()))
}
