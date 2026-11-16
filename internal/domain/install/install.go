package install

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/pan-daniel/line-proton/internal/domain/fonts"
)

var (
	ErrInvalidInstallation     = errors.New("invalid installation")
	ErrUnknownLauncher         = errors.New("unknown launcher")
	ErrNotConfigured           = errors.New("line-proton is not set up yet; run `line-proton setup`")
	ErrUnsupportedArchitecture = errors.New("line-proton needs an x86_64 machine")
	ErrPrefixNotOurs           = errors.New("refusing to delete a directory that is not a line-proton prefix")
)

type Launcher int

const (
	Faugus Launcher = iota
	Standalone
)

var launcherNames = []string{"faugus", "standalone"}

func ParseLauncher(name string) (Launcher, error) {
	index := slices.Index(launcherNames, name)
	if index < 0 {
		return Standalone, fmt.Errorf("%w: %q", ErrUnknownLauncher, name)
	}
	return Launcher(index), nil
}

func (launcher Launcher) String() string {
	return launcherNames[launcher]
}

func (launcher Launcher) UsesFaugus() bool {
	return launcher == Faugus
}

var launcherInPrefix = []string{"drive_c", "users", "steamuser", "AppData", "Local", "LINE", "bin", "LineLauncher.exe"}

type Installation struct {
	prefix       string
	sourceRunner string
	fontVariant  fonts.Variant
	executable   string
	launcher     Launcher
}

func New(prefix, sourceRunner string, fontVariant fonts.Variant, executable string, launcher Launcher) (Installation, error) {
	if !filepath.IsAbs(prefix) || !filepath.IsAbs(executable) || sourceRunner == "" {
		return Installation{}, fmt.Errorf("%w: prefix %q, executable %q, source %q", ErrInvalidInstallation, prefix, executable, sourceRunner)
	}
	return Installation{prefix: prefix, sourceRunner: sourceRunner, fontVariant: fontVariant, executable: executable, launcher: launcher}, nil
}

func (installation Installation) Prefix() string {
	return installation.prefix
}

func (installation Installation) SourceRunner() string {
	return installation.sourceRunner
}

func (installation Installation) FontVariant() fonts.Variant {
	return installation.fontVariant
}

func (installation Installation) Executable() string {
	return installation.executable
}

func (installation Installation) Launcher() Launcher {
	return installation.launcher
}

func (installation Installation) WithExecutable(executable string) (Installation, error) {
	return New(installation.prefix, installation.sourceRunner, installation.fontVariant, executable, installation.launcher)
}

func (installation Installation) WithPrefix(prefix string) (Installation, error) {
	return New(prefix, installation.sourceRunner, installation.fontVariant, installation.executable, installation.launcher)
}

func (installation Installation) WithSourceRunner(sourceRunner string) (Installation, error) {
	return New(installation.prefix, sourceRunner, installation.fontVariant, installation.executable, installation.launcher)
}

func (installation Installation) WithFontVariant(fontVariant fonts.Variant) (Installation, error) {
	return New(installation.prefix, installation.sourceRunner, fontVariant, installation.executable, installation.launcher)
}

func (installation Installation) LauncherRelativePath() string {
	return filepath.Join(launcherInPrefix...)
}

func (installation Installation) LauncherExecutable() string {
	return filepath.Join(installation.prefix, installation.LauncherRelativePath())
}

func (installation Installation) PrefixRemovable(home string, ownedByLineProton bool) error {
	prefix := filepath.Clean(installation.prefix)
	if !ownedByLineProton || prefix == filepath.Clean(home) || prefix == string(filepath.Separator) {
		return fmt.Errorf("%w: %s", ErrPrefixNotOurs, installation.prefix)
	}
	return nil
}

func FirstSetup(home, sourceRunner string, fontVariant fonts.Variant, executable string, launcher Launcher) (Installation, error) {
	return New(DefaultPrefix(home, launcher), sourceRunner, fontVariant, executable, launcher)
}

func DefaultPrefix(home string, launcher Launcher) string {
	if launcher.UsesFaugus() {
		return filepath.Join(home, "Faugus", "line")
	}
	return filepath.Join(home, ".local", "share", "line-proton", "prefix")
}

func SupportedArchitecture(goarch string) bool {
	return goarch == "amd64"
}

type Store interface {
	Load() (Installation, bool, error)
	Save(installation Installation) error
	Remove() error
}
