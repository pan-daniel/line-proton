package filesystem

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func CloneTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return cloneEntry(source, destination, path, entry)
	})
}

func cloneEntry(source, destination, path string, entry fs.DirEntry) error {
	relative, err := filepath.Rel(source, path)
	if err != nil {
		return err
	}
	details, err := entry.Info()
	if err != nil {
		return err
	}
	target := filepath.Join(destination, relative)
	switch {
	case details.IsDir():
		return os.MkdirAll(target, details.Mode().Perm()|0o700)
	case details.Mode()&fs.ModeSymlink != 0:
		return copySymlink(path, target)
	case details.Mode().IsRegular():
		return cloneRegularFile(path, target, details)
	}
	return nil
}

func copySymlink(path, target string) error {
	destination, err := os.Readlink(path)
	if err != nil {
		return err
	}
	return os.Symlink(destination, target)
}

func cloneRegularFile(path, target string, details fs.FileInfo) error {
	if err := CloneFile(path, target, details.Mode().Perm()); err != nil {
		return err
	}
	return os.Chtimes(target, details.ModTime(), details.ModTime())
}

func CloneFile(source, target string, mode fs.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	return errors.Join(fill(output, input), output.Close(), os.Chmod(target, mode))
}

func fill(output, input *os.File) error {
	if reflink(output, input) == nil {
		return nil
	}
	_, err := io.Copy(output, input)
	return err
}

func reflink(output, input *os.File) error {
	return unix.IoctlFileClone(int(output.Fd()), int(input.Fd()))
}
