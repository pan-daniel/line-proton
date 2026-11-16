package fontlib

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pan-daniel/line-proton/internal/domain/fonts"
)

type Library struct {
	directories []string
}

func NewLibrary(directories ...string) Library {
	return Library{directories: slices.Clone(directories)}
}

func StandardDirectories(home string) []string {
	return []string{"/usr/share/fonts", "/usr/local/share/fonts", filepath.Join(home, ".local", "share", "fonts"), filepath.Join(home, ".fonts")}
}

func (library Library) Locate(names []string) ([]string, error) {
	found := library.index(names)
	missing := slices.DeleteFunc(slices.Clone(names), func(name string) bool { return found[name] != "" })
	if len(missing) > 0 {
		return nil, fmt.Errorf("%w: %s", fonts.ErrMissingFonts, strings.Join(missing, ", "))
	}
	located := make([]string, 0, len(names))
	for _, name := range names {
		located = append(located, found[name])
	}
	return located, nil
}

func (library Library) index(names []string) map[string]string {
	found := map[string]string{}
	for _, directory := range slices.Backward(library.directories) {
		maps.Copy(found, fontsIn(directory, names))
	}
	return found
}

func fontsIn(directory string, names []string) map[string]string {
	found := map[string]string{}
	_ = filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && isWanted(entry, names) && found[entry.Name()] == "" {
			found[entry.Name()] = path
		}
		return nil
	})
	return found
}

func isWanted(entry fs.DirEntry, names []string) bool {
	return !entry.IsDir() && slices.Contains(names, entry.Name())
}

func DistributionIdentifiers(osRelease string) []string {
	raw, err := os.ReadFile(osRelease)
	if err != nil {
		return nil
	}
	var identifiers []string
	for line := range strings.Lines(string(raw)) {
		identifiers = append(identifiers, identifiersOn(line)...)
	}
	return identifiers
}

func identifiersOn(line string) []string {
	key, value, found := strings.Cut(strings.TrimSpace(line), "=")
	if !found || (key != "ID" && key != "ID_LIKE") {
		return nil
	}
	return strings.Fields(strings.Trim(value, `"'`))
}
