package main

import (
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/pan-daniel/line-proton/internal/application"
	"github.com/pan-daniel/line-proton/internal/domain/fonts"
	"github.com/pan-daniel/line-proton/internal/domain/hooks"
	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/domain/runner"
	"github.com/pan-daniel/line-proton/internal/domain/signing"
	"github.com/pan-daniel/line-proton/internal/infrastructure/authenticode"
	"github.com/pan-daniel/line-proton/internal/infrastructure/certstore"
	"github.com/pan-daniel/line-proton/internal/infrastructure/config"
	"github.com/pan-daniel/line-proton/internal/infrastructure/console"
	"github.com/pan-daniel/line-proton/internal/infrastructure/desktop"
	"github.com/pan-daniel/line-proton/internal/infrastructure/faugus"
	"github.com/pan-daniel/line-proton/internal/infrastructure/filesystem"
	"github.com/pan-daniel/line-proton/internal/infrastructure/fontlib"
	"github.com/pan-daniel/line-proton/internal/infrastructure/host"
	"github.com/pan-daniel/line-proton/internal/infrastructure/icon"
	"github.com/pan-daniel/line-proton/internal/infrastructure/linecdn"
	"github.com/pan-daniel/line-proton/internal/infrastructure/prefixfs"
	"github.com/pan-daniel/line-proton/internal/infrastructure/processes"
	"github.com/pan-daniel/line-proton/internal/infrastructure/registry"
	"github.com/pan-daniel/line-proton/internal/infrastructure/runners"
	"github.com/pan-daniel/line-proton/internal/infrastructure/umu"
)

const lockFileName = "lock"

type adapters struct {
	environment  environment
	installation install.Installation
	logger       *slog.Logger
	signer       authenticode.Authenticode
	umuCommand   umu.Command
	umuProblem   error
}

func newAdapters(environment environment, installation install.Installation, logger *slog.Logger) adapters {
	command, problem := umuLocatorFor(environment).Locate()
	return adapters{environment: environment, installation: installation, logger: logger, umuCommand: command, umuProblem: problem}
}

func umuLocatorFor(environment environment) umu.Locator {
	return umu.NewLocator(environment.home, exec.LookPath)
}

func newSigningAdapters(environment environment, installation install.Installation, logger *slog.Logger) (adapters, error) {
	built := newAdapters(environment, installation, logger)
	if built.umuProblem != nil {
		return adapters{}, built.umuProblem
	}
	certificate, err := built.certificateStore().LoadOrCreate(signing.ImpersonatedWindowsIdentity())
	if err != nil {
		return adapters{}, err
	}
	built.signer = authenticode.New(certificate)
	return built, nil
}

func signingAdaptersFor(environment environment, installation install.Installation, logger *slog.Logger) (adapters, error) {
	if certificateStoreFor(environment).Exists() {
		return newSigningAdapters(environment, installation, logger)
	}
	return lockedSigningAdapters(environment, installation, logger)
}

func lockedSigningAdapters(environment environment, installation install.Installation, logger *slog.Logger) (adapters, error) {
	release, err := globalLock(environment).Acquire()
	if err != nil {
		return adapters{}, err
	}
	built, err := newSigningAdapters(environment, installation, logger)
	return built, errors.Join(err, release())
}

func installationResolver(environment environment) application.InstallationResolver {
	locale := host.NewLocale(os.Getenv)
	launchers := faugus.NewDetector(exec.LookPath)
	return application.NewInstallationResolver(config.New(environment.configFile), runners.NewCatalog(environment.runnersDirectory), launchers, locale, runningExecutable().Locate, environment.home)
}

func runningExecutable() host.Executable {
	return host.NewExecutable(os.Executable)
}

func globalLock(environment environment) filesystem.Lock {
	return filesystem.NewLock(filepath.Join(environment.stateDirectory, lockFileName))
}

func (built adapters) certificateStore() certstore.Store {
	return certificateStoreFor(built.environment)
}

func certificateStoreFor(environment environment) certstore.Store {
	return certstore.New(environment.dataDirectory, time.Now)
}

func (built adapters) catalog() runners.Catalog {
	return runners.NewCatalog(built.environment.runnersDirectory)
}

func (built adapters) lineRunnerDirectory() string {
	return built.catalog().Path(runner.LineRunnerName)
}

func (built adapters) wine() umu.Proton {
	return umu.NewProton(built.umuCommand, built.installation.Prefix(), built.lineRunnerDirectory(), console.NewLogWriter(built.logger))
}

func (built adapters) prefixFiles() prefixfs.Prefix {
	return prefixfs.New(built.installation.Prefix(), built.lineRunnerDirectory())
}

func (built adapters) protonPrefixLock() filesystem.Lock {
	return filesystem.NewLock(filepath.Join(built.installation.Prefix(), "pfx.lock"))
}

func (built adapters) processScanner() processes.Scanner {
	return processes.NewScanner(os.DirFS("/proc"))
}

func (built adapters) runnerActivity() processes.RunnerActivity {
	return processes.NewRunnerActivity(built.processScanner(), built.lineRunnerDirectory())
}

func (built adapters) prefixSigner() application.PrefixSigner {
	return application.NewPrefixSigner(built.prefixFiles(), built.protonPrefixLock(), built.signer, built.signer, time.Now, built.logger)
}

func (built adapters) refresher() application.RunnerRefresher {
	workshop := runners.NewWorkshop(built.environment.runnersDirectory, built.environment.stagingDirectory(), built.logger)
	return application.NewRunnerRefresher(built.catalog(), workshop, built.runnerActivity(), built.signer, built.signer, built.installation.SourceRunner())
}

func (built adapters) lock() filesystem.Lock {
	return globalLock(built.environment)
}

func (built adapters) fontLibrary() fontlib.Library {
	return fontlib.NewLibrary(fontlib.StandardDirectories(built.environment.home)...)
}

func (built adapters) fontPlan() fonts.Plan {
	return fonts.PlanFor(built.installation.FontVariant())
}

func (built adapters) prefixCreator() application.PrefixCreator {
	return application.NewPrefixCreator(built.wine(), built.prefixFiles())
}

func (built adapters) fontProvisioner() application.FontProvisioner {
	fontHint := fonts.InstallHint(fontlib.DistributionIdentifiers("/etc/os-release"))
	importer := registry.NewImporter(built.installation.Prefix(), built.wine())
	return application.NewFontProvisioner(built.fontLibrary(), built.prefixFiles(), importer, built.fontPlan(), fontHint)
}

func (built adapters) hookRunner() application.HookRunner {
	return application.NewHookRunner(application.HookRunnerDependencies{
		Lock:         built.lock(),
		Waiter:       application.NewIdleWaiter(built.runnerActivity(), hooks.PollInterval),
		PrefixSigner: built.prefixSigner(),
		Refresher:    built.refresher(),
		Fonts:        built.fontProvisioner(),
		Wine:         built.wine(),
		Installation: built.installation,
		Announcer:    console.NewAnnouncer(built.logger),
		Logger:       built.logger,
	})
}

func (built adapters) faugusLibrary() faugus.Library {
	return faugus.NewLibrary(faugus.GamesFile(built.environment.home), built.processScanner())
}

func (built adapters) iconPath() string {
	if built.installation.Launcher().UsesFaugus() {
		return faugus.IconFile(built.environment.home)
	}
	return filepath.Join(built.environment.dataDirectory, "line.png")
}

func (built adapters) desktopApplications() desktop.Applications {
	return desktop.New(built.environment.applicationsDirectory)
}

func (built adapters) launcherEntries() application.LauncherEntries {
	return application.NewLauncherEntries(iconExtractor{}, built.iconPath(), built.faugusLibrary(), built.desktopApplications(), built.installation, built.logger)
}

func (built adapters) installerVerifier() (linecdn.Verifier, error) {
	roots, err := linecdn.GlobalSignRoots()
	if err != nil {
		return linecdn.Verifier{}, err
	}
	return linecdn.NewVerifier(roots), nil
}

func (built adapters) setup() (application.Setup, error) {
	verifier, err := built.installerVerifier()
	if err != nil {
		return application.Setup{}, err
	}
	downloader := linecdn.NewDownloader(&http.Client{Timeout: 5 * time.Minute}, linecdn.InstallerURL, filepath.Join(built.environment.stateDirectory, "downloads"))
	lineInstaller := application.NewLineInstaller(built.wine(), built.prefixFiles(), downloader, verifier, built.installation)
	return application.NewSetup(application.SetupDependencies{
		Installation:  built.installation,
		Store:         config.New(built.environment.configFile),
		Refresher:     built.refresher(),
		Creator:       built.prefixCreator(),
		Fonts:         built.fontProvisioner(),
		PrefixSigner:  built.prefixSigner(),
		LineInstaller: lineInstaller,
		Entries:       built.launcherEntries(),
		Announcer:     console.NewAnnouncer(built.logger),
	}), nil
}

func (built adapters) doctor() application.Doctor {
	return application.NewDoctor(application.DoctorDependencies{
		Installation:      built.installation,
		Umu:               umuLocatorFor(built.environment),
		Runners:           built.catalog(),
		Prefix:            built.prefixFiles(),
		Fonts:             built.fontLibrary(),
		Certificate:       built.certificateStore(),
		Faugus:            built.faugusLibrary(),
		Desktop:           built.desktopApplications(),
		RunningExecutable: runningExecutable().Locate,
		ExecutableExists:  isExistingFile,
	})
}

func (built adapters) uninstaller() application.Uninstaller {
	removables := []application.Removable{
		built.launcherEntries(),
		application.NewLineRunnerRemoval(built.catalog()),
		built.certificateStore(),
		file{path: built.iconPath()},
		config.New(built.environment.configFile),
		filesystem.NewDirectoryContents(built.environment.stateDirectory, lockFileName),
		directory{path: built.environment.stagingDirectory()},
	}
	return application.NewUninstaller(removables, built.prefixFiles(), built.lock(), built.runnerActivity(), built.installation, built.environment.home)
}

func isExistingFile(path string) bool {
	details, err := os.Stat(path)
	return err == nil && details.Mode().IsRegular()
}

type iconExtractor struct{}

func (iconExtractor) ExtractPNG(executable, destination string) error {
	return icon.ExtractPNG(executable, destination)
}

type directory struct {
	path string
}

func (removed directory) Remove() error {
	return os.RemoveAll(removed.path)
}

type file struct {
	path string
}

func (removed file) Remove() error {
	err := os.Remove(removed.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
