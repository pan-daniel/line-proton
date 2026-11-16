package application_test

import (
	"errors"
	"testing"

	"github.com/pan-daniel/line-proton/internal/application"
	"github.com/pan-daniel/line-proton/internal/domain/fonts"
	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/domain/runner"
)

type savedConfiguration struct {
	installation install.Installation
	found        bool
	failure      error
}

func (saved savedConfiguration) Load() (install.Installation, bool, error) {
	return saved.installation, saved.found, saved.failure
}

type listedRunners struct {
	runners []runner.Runner
	failure error
}

func (listed listedRunners) Installed() ([]runner.Runner, error) {
	return listed.runners, listed.failure
}

type detectedLauncher install.Launcher

func (detected detectedLauncher) Detect() install.Launcher {
	return install.Launcher(detected)
}

type fixedLocale fonts.LocaleSettings

func (locale fixedLocale) LocaleSettings() fonts.LocaleSettings {
	return fonts.LocaleSettings(locale)
}

type resolverFixture struct {
	saved      savedConfiguration
	runners    listedRunners
	launcher   install.Launcher
	locale     fonts.LocaleSettings
	executable application.RunningExecutable
}

func (fixture resolverFixture) build() application.InstallationResolver {
	executable := fixture.executable
	if executable == nil {
		executable = func() (string, error) { return "/usr/bin/line-proton", nil }
	}
	return application.NewInstallationResolver(fixture.saved, fixture.runners, detectedLauncher(fixture.launcher), fixedLocale(fixture.locale), executable, "/home/u")
}

func savedFaugusInstallation(test *testing.T) install.Installation {
	test.Helper()
	saved, err := install.New("/home/u/Faugus/line", "Proton-CachyOS Latest", fonts.Japanese, "/old/line-proton", install.Faugus)
	if err != nil {
		test.Fatal(err)
	}
	return saved
}

func runnersNamed(test *testing.T, names ...string) listedRunners {
	test.Helper()
	listed := listedRunners{}
	for _, name := range names {
		listed.runners = append(listed.runners, runnerNamed(test, name, "1 "+name))
	}
	return listed
}

func TestResolveMergesTheRequestOverTheSavedInstallation(test *testing.T) {
	fixture := resolverFixture{saved: savedConfiguration{installation: savedFaugusInstallation(test), found: true}, runners: listedRunners{failure: errors.New("not listed")}}
	resolved, err := fixture.build().Resolve(install.NewSetupRequest("", "", "kr"))
	want, _ := install.New("/home/u/Faugus/line", "Proton-CachyOS Latest", fonts.Korean, "/usr/bin/line-proton", install.Faugus)
	if err != nil || resolved != want {
		test.Errorf("Resolve = %+v, %v; want %+v", resolved, err, want)
	}
}

func TestResolveAcceptsOnlyAnInstalledSourceRunner(test *testing.T) {
	fixture := resolverFixture{saved: savedConfiguration{installation: savedFaugusInstallation(test), found: true}, runners: runnersNamed(test, "Proton-GE Latest", runner.LineRunnerName)}
	switched, err := fixture.build().Resolve(install.NewSetupRequest("", "Proton-GE Latest", ""))
	if err != nil || switched.SourceRunner() != "Proton-GE Latest" || switched.FontVariant() != fonts.Japanese {
		test.Errorf("switched = %+v, %v", switched, err)
	}
	for _, requested := range []string{"missing", runner.LineRunnerName} {
		test.Run(requested, func(test *testing.T) {
			if _, err := fixture.build().Resolve(install.NewSetupRequest("", requested, "")); !errors.Is(err, runner.ErrRunnerNotFound) {
				test.Errorf("err = %v, want ErrRunnerNotFound", err)
			}
		})
	}
}

func TestFirstResolveDetectsLauncherSourceFontAndPrefix(test *testing.T) {
	fixture := resolverFixture{
		runners:  runnersNamed(test, "Proton-CachyOS Latest", runner.LineRunnerName),
		launcher: install.Standalone,
		locale:   fonts.NewLocaleSettings("", "", "zh_TW.UTF-8"),
	}
	resolved, err := fixture.build().Resolve(install.NewSetupRequest("", "", ""))
	want, _ := install.New("/home/u/.local/share/line-proton/prefix", "Proton-CachyOS Latest", fonts.TraditionalChinese, "/usr/bin/line-proton", install.Standalone)
	if err != nil || resolved != want {
		test.Errorf("Resolve = %+v, %v; want %+v", resolved, err, want)
	}
}

func TestFirstResolveNeedsASourceRunner(test *testing.T) {
	fixture := resolverFixture{runners: runnersNamed(test, runner.LineRunnerName)}
	if _, err := fixture.build().Resolve(install.NewSetupRequest("", "", "")); !errors.Is(err, runner.ErrNoRunners) {
		test.Errorf("err = %v, want ErrNoRunners", err)
	}
}

func TestResolveStopsAtTheFirstFailure(test *testing.T) {
	unreadable, lost := errors.New("config unreadable"), errors.New("executable lost")
	examples := []struct {
		name    string
		fixture resolverFixture
		want    error
	}{
		{"unknown executable", resolverFixture{executable: func() (string, error) { return "", lost }}, lost},
		{"unreadable config", resolverFixture{saved: savedConfiguration{failure: unreadable}}, unreadable},
		{"unlisted runners", resolverFixture{runners: listedRunners{failure: unreadable}}, unreadable},
	}
	for _, example := range examples {
		test.Run(example.name, func(test *testing.T) {
			if _, err := example.fixture.build().Resolve(install.NewSetupRequest("", "", "")); !errors.Is(err, example.want) {
				test.Errorf("err = %v, want %v", err, example.want)
			}
		})
	}
}

func TestConfiguredNeedsASavedInstallation(test *testing.T) {
	if _, err := (resolverFixture{}).build().Configured(); !errors.Is(err, install.ErrNotConfigured) {
		test.Errorf("err = %v, want ErrNotConfigured", err)
	}
	saved := savedFaugusInstallation(test)
	loaded, err := resolverFixture{saved: savedConfiguration{installation: saved, found: true}}.build().Configured()
	if err != nil || loaded != saved {
		test.Errorf("Configured = %+v, %v; want %+v", loaded, err, saved)
	}
}
