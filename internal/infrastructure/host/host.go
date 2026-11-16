package host

import (
	"path/filepath"

	"github.com/pan-daniel/line-proton/internal/domain/fonts"
)

type Locale struct {
	variable func(string) string
}

func NewLocale(variable func(string) string) Locale {
	return Locale{variable: variable}
}

func (locale Locale) LocaleSettings() fonts.LocaleSettings {
	return fonts.NewLocaleSettings(locale.variable("LC_ALL"), locale.variable("LC_CTYPE"), locale.variable("LANG"))
}

type Executable struct {
	running func() (string, error)
}

func NewExecutable(running func() (string, error)) Executable {
	return Executable{running: running}
}

func (executable Executable) Locate() (string, error) {
	path, err := executable.running()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}
