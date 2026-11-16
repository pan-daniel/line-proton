package config

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pan-daniel/line-proton/internal/domain/fonts"
	"github.com/pan-daniel/line-proton/internal/domain/install"
)

type document struct {
	Prefix       string `json:"prefix"`
	SourceRunner string `json:"source_runner"`
	FontVariant  string `json:"font_variant"`
	Executable   string `json:"executable"`
	Launcher     string `json:"launcher"`
}

type Store struct {
	file string
}

func New(file string) Store {
	return Store{file: file}
}

func (store Store) Load() (install.Installation, bool, error) {
	raw, err := os.ReadFile(store.file)
	if errors.Is(err, fs.ErrNotExist) {
		return install.Installation{}, false, nil
	}
	if err != nil {
		return install.Installation{}, false, err
	}
	var stored document
	if err := json.Unmarshal(raw, &stored); err != nil {
		return install.Installation{}, false, err
	}
	installation, err := stored.installation()
	return installation, err == nil, err
}

func (store Store) Save(installation install.Installation) error {
	raw, err := json.Marshal(documentOf(installation), jsontext.WithIndent("  "))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(store.file), 0o700); err != nil {
		return err
	}
	return os.WriteFile(store.file, raw, 0o600)
}

func (store Store) Remove() error {
	err := os.Remove(store.file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func documentOf(installation install.Installation) document {
	return document{
		Prefix:       installation.Prefix(),
		SourceRunner: installation.SourceRunner(),
		FontVariant:  installation.FontVariant().Code(),
		Executable:   installation.Executable(),
		Launcher:     installation.Launcher().String(),
	}
}

func (stored document) installation() (install.Installation, error) {
	variant, err := fonts.ParseVariant(stored.FontVariant)
	if err != nil {
		return install.Installation{}, err
	}
	kind, err := install.ParseLauncher(stored.Launcher)
	if err != nil {
		return install.Installation{}, err
	}
	return install.New(stored.Prefix, stored.SourceRunner, variant, stored.Executable, kind)
}
