package runners

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pan-daniel/line-proton/internal/domain/runner"
)

type Catalog struct {
	directory string
}

func NewCatalog(directory string) Catalog {
	return Catalog{directory: directory}
}

func (catalog Catalog) Path(name string) string {
	return filepath.Join(catalog.directory, name)
}

func (catalog Catalog) Installed() ([]runner.Runner, error) {
	entries, err := os.ReadDir(catalog.directory)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var found []runner.Runner
	for _, entry := range entries {
		found = append(found, catalog.runnerNamed(entry.Name())...)
	}
	return found, nil
}

func (catalog Catalog) Find(name string) (runner.Installed, error) {
	raw, err := os.ReadFile(filepath.Join(catalog.directory, name, "version"))
	if errors.Is(err, fs.ErrNotExist) {
		return runner.Absent(), nil
	}
	if err != nil {
		return runner.Absent(), err
	}
	return installedFrom(name, string(raw))
}

func (catalog Catalog) Remove(name string) error {
	return os.RemoveAll(catalog.Path(name))
}

func (catalog Catalog) runnerNamed(name string) []runner.Runner {
	installed, err := catalog.Find(name)
	found, present := installed.Runner()
	if err != nil || !present {
		return nil
	}
	return []runner.Runner{found}
}

func installedFrom(name, raw string) (runner.Installed, error) {
	version, err := runner.ParseVersion(raw)
	if err != nil {
		return runner.Absent(), err
	}
	found, err := runner.New(name, version)
	if err != nil {
		return runner.Absent(), err
	}
	return runner.Present(found), nil
}
