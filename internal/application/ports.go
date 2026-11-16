package application

import (
	"context"
	"errors"
	"time"

	"github.com/pan-daniel/line-proton/internal/domain/fonts"
	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/domain/runner"
	"github.com/pan-daniel/line-proton/internal/domain/winreg"
)

var (
	ErrSourceMissing      = errors.New("the source Proton for Proton-LINE is not installed anymore")
	ErrRunnerStillBusy    = errors.New("Proton-LINE is still in use")
	ErrUntrustedInstaller = errors.New("the downloaded LINE installer is not signed by LY Corporation")
)

type Wine interface {
	Run(ctx context.Context, executable string, arguments ...string) error
}

type WaitingLock interface {
	Acquire() (func() error, error)
}

type Lock interface {
	WaitingLock
	TryAcquire() (func() error, bool, error)
}

type RunnerActivity interface {
	InUse() (bool, error)
}

type Installer interface {
	Fetch(ctx context.Context) (string, error)
	Discard(path string) error
}

type RegistryImporter interface {
	Import(ctx context.Context, patch winreg.Patch) error
}

type IconExtractor interface {
	ExtractPNG(executable, destination string) error
}

type Removable interface {
	Remove() error
}

type Clock func() time.Time

type PathProbe func(path string) bool

type RunningExecutable func() (string, error)

type UmuLocator interface {
	Describe() (string, error)
}

type CertificateStore interface {
	Exists() bool
}

type SavedInstallation interface {
	Load() (install.Installation, bool, error)
}

type InstalledRunners interface {
	Installed() ([]runner.Runner, error)
}

type LauncherDetector interface {
	Detect() install.Launcher
}

type LocaleReader interface {
	LocaleSettings() fonts.LocaleSettings
}
