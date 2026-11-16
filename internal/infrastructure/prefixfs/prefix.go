package prefixfs

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/pan-daniel/line-proton/internal/domain/prefix"
	"github.com/pan-daniel/line-proton/internal/infrastructure/filesystem"
	"github.com/pan-daniel/line-proton/internal/shared/sequence"
)

type Prefix struct {
	directory  string
	lineRunner string
}

func New(directory, lineRunner string) Prefix {
	return Prefix{directory: directory, lineRunner: filepath.Clean(lineRunner)}
}

func (winePrefix Prefix) Scan() ([]prefix.SystemFile, error) {
	root, err := os.OpenRoot(winePrefix.directory)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	var files []prefix.SystemFile
	var failures []error
	for _, directory := range prefix.SystemDirectories {
		found, err := winePrefix.scanDirectory(root, directory)
		files = append(files, found...)
		failures = append(failures, err)
	}
	return files, errors.Join(failures...)
}

func (winePrefix Prefix) Marker() (prefix.Marker, error) {
	details, err := os.Stat(winePrefix.markerPath())
	if errors.Is(err, fs.ErrNotExist) {
		return prefix.Unmarked(), nil
	}
	if err != nil {
		return prefix.Unmarked(), err
	}
	return prefix.MarkedAt(details.ModTime()), nil
}

func (winePrefix Prefix) Mark(signedAt time.Time) error {
	if err := os.WriteFile(winePrefix.markerPath(), nil, 0o644); err != nil {
		return err
	}
	return os.Chtimes(winePrefix.markerPath(), signedAt, signedAt)
}

func (winePrefix Prefix) DetachLink(link string) error {
	details, err := os.Lstat(link)
	if err != nil {
		return err
	}
	if details.Mode()&fs.ModeSymlink == 0 {
		return nil
	}
	detached := link + ".detaching"
	if err := filesystem.CloneFile(link, detached, 0o644); err != nil {
		return errors.Join(err, os.Remove(detached))
	}
	return os.Rename(detached, link)
}

func (winePrefix Prefix) AddFonts(paths []string) error {
	fontsDirectory := filepath.Join(winePrefix.directory, "drive_c", "windows", "Fonts")
	if err := os.MkdirAll(fontsDirectory, 0o755); err != nil {
		return err
	}
	return sequence.ForEach(slices.Values(paths), func(font string) error {
		return replaceWithCopy(font, filepath.Join(fontsDirectory, filepath.Base(font)))
	})
}

func (winePrefix Prefix) ReadSystemRegistry() (string, error) {
	content, err := os.ReadFile(filepath.Join(winePrefix.directory, "system.reg"))
	return string(content), err
}

func (winePrefix Prefix) Exists(relativePath string) bool {
	_, err := os.Lstat(filepath.Join(winePrefix.directory, relativePath))
	return err == nil
}

func (winePrefix Prefix) Remove() error {
	return os.RemoveAll(winePrefix.directory)
}

func (winePrefix Prefix) scanDirectory(root *os.Root, relative string) ([]prefix.SystemFile, error) {
	entries, err := fs.ReadDir(root.FS(), relative)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var files []prefix.SystemFile
	for _, entry := range entries {
		files = append(files, winePrefix.describe(root, path.Join(relative, entry.Name()))...)
	}
	return files, nil
}

func (winePrefix Prefix) describe(root *os.Root, relative string) []prefix.SystemFile {
	if !prefix.IsSystemBinary(relative) {
		return nil
	}
	details, err := root.Lstat(relative)
	if err != nil {
		return nil
	}
	return []prefix.SystemFile{prefix.NewSystemFile(filepath.Join(winePrefix.directory, relative), winePrefix.kindOf(root, relative, details), details.ModTime())}
}

func (winePrefix Prefix) kindOf(root *os.Root, relative string, details fs.FileInfo) prefix.Kind {
	if details.Mode()&fs.ModeSymlink == 0 {
		return prefix.RegularFile
	}
	if _, err := os.Stat(filepath.Join(winePrefix.directory, relative)); err != nil {
		return prefix.BrokenLink
	}
	target, err := root.Readlink(relative)
	if err != nil || !strings.HasPrefix(filepath.Clean(target), winePrefix.lineRunner+string(filepath.Separator)) {
		return prefix.LinkElsewhere
	}
	return prefix.LinkIntoLineRunner
}

func (winePrefix Prefix) markerPath() string {
	return filepath.Join(winePrefix.directory, prefix.MarkerName)
}

func replaceWithCopy(source, target string) error {
	if err := os.Remove(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return filesystem.CloneFile(source, target, 0o644)
}
