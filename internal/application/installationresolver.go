package application

import (
	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/domain/runner"
)

type InstallationResolver struct {
	saved             SavedInstallation
	runners           InstalledRunners
	launchers         LauncherDetector
	locale            LocaleReader
	runningExecutable RunningExecutable
	home              string
}

func NewInstallationResolver(saved SavedInstallation, runners InstalledRunners, launchers LauncherDetector, locale LocaleReader, runningExecutable RunningExecutable, home string) InstallationResolver {
	return InstallationResolver{saved: saved, runners: runners, launchers: launchers, locale: locale, runningExecutable: runningExecutable, home: home}
}

func (resolver InstallationResolver) Configured() (install.Installation, error) {
	installation, found, err := resolver.saved.Load()
	if err != nil {
		return install.Installation{}, err
	}
	if !found {
		return install.Installation{}, install.ErrNotConfigured
	}
	return installation, nil
}

func (resolver InstallationResolver) Resolve(request install.SetupRequest) (install.Installation, error) {
	executable, err := resolver.runningExecutable()
	if err != nil {
		return install.Installation{}, err
	}
	base, err := resolver.baseInstallation(executable)
	if err != nil {
		return install.Installation{}, err
	}
	if err := resolver.requireRequestedSource(request); err != nil {
		return install.Installation{}, err
	}
	return request.AppliedTo(base)
}

func (resolver InstallationResolver) baseInstallation(executable string) (install.Installation, error) {
	saved, found, err := resolver.saved.Load()
	if err != nil {
		return install.Installation{}, err
	}
	if found {
		return saved.WithExecutable(executable)
	}
	return resolver.firstSetup(executable)
}

func (resolver InstallationResolver) firstSetup(executable string) (install.Installation, error) {
	source, err := resolver.selectedSource("")
	if err != nil {
		return install.Installation{}, err
	}
	return install.FirstSetup(resolver.home, source.Name(), resolver.locale.LocaleSettings().Variant(), executable, resolver.launchers.Detect())
}

func (resolver InstallationResolver) requireRequestedSource(request install.SetupRequest) error {
	if !request.ChoosesSourceRunner() {
		return nil
	}
	_, err := resolver.selectedSource(request.SourceRunner())
	return err
}

func (resolver InstallationResolver) selectedSource(requested string) (runner.Runner, error) {
	installed, err := resolver.runners.Installed()
	if err != nil {
		return runner.Runner{}, err
	}
	return runner.SelectSource(installed, requested)
}
