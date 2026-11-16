package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pan-daniel/line-proton/internal/domain/fonts"
	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/domain/runner"
	"github.com/pan-daniel/line-proton/internal/infrastructure/config"
)

func TestEnvironmentFollowsXDGVariables(test *testing.T) {
	test.Setenv("XDG_CONFIG_HOME", "/custom/config")
	test.Setenv("XDG_DATA_HOME", "")
	test.Setenv("XDG_STATE_HOME", "/custom/state")
	environment := environmentFor("/home/user")
	expectations := map[string]string{
		environment.configFile:            "/custom/config/line-proton/config.json",
		environment.dataDirectory:         "/home/user/.local/share/line-proton",
		environment.stagingDirectory():    "/home/user/.local/share/Steam/.line-proton-staging",
		environment.stateDirectory:        "/custom/state/line-proton",
		environment.runnersDirectory:      "/home/user/.local/share/Steam/compatibilitytools.d",
		environment.applicationsDirectory: "/home/user/.local/share/applications",
	}
	for got, want := range expectations {
		test.Run(want, func(test *testing.T) {
			if filepath.Clean(got) != want {
				test.Errorf("path = %q, want %q", got, want)
			}
		})
	}
}

func isolatedEnvironment(test *testing.T) environment {
	test.Helper()
	home := test.TempDir()
	test.Setenv("HOME", home)
	test.Setenv("XDG_CONFIG_HOME", "")
	test.Setenv("XDG_DATA_HOME", "")
	test.Setenv("XDG_STATE_HOME", "")
	return environmentFor(home)
}

func installedRunner(test *testing.T, environment environment, name string) string {
	test.Helper()
	directory := filepath.Join(environment.runnersDirectory, name)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "version"), []byte("1 "+name), 0o644); err != nil {
		test.Fatal(err)
	}
	return directory
}

func savedInstallation(test *testing.T, environment environment) install.Installation {
	test.Helper()
	saved, err := install.New("/home/u/Faugus/line", "Proton-CachyOS Latest", fonts.Japanese, "/old/line-proton", install.Faugus)
	if err != nil {
		test.Fatal(err)
	}
	if err := config.New(environment.configFile).Save(saved); err != nil {
		test.Fatal(err)
	}
	return saved
}

func parsedSetupFlags(test *testing.T, arguments ...string) install.SetupRequest {
	test.Helper()
	request, err := parseSetupFlags(arguments)
	if err != nil {
		test.Fatal(err)
	}
	return request
}

func TestSetupFlagsAreMergedOverTheSavedConfiguration(test *testing.T) {
	environment := isolatedEnvironment(test)
	saved := savedInstallation(test, environment)
	resolved, err := installationResolver(environment).Resolve(parsedSetupFlags(test, "--font", "kr"))
	if err != nil {
		test.Fatal(err)
	}
	executable, _ := runningExecutable().Locate()
	if resolved.FontVariant() != fonts.Korean || resolved.Prefix() != saved.Prefix() || resolved.SourceRunner() != saved.SourceRunner() || resolved.Launcher() != install.Faugus || resolved.Executable() != executable {
		test.Errorf("resolved = %+v", resolved)
	}
}

func TestRequestedProtonMustBeInstalled(test *testing.T) {
	environment := isolatedEnvironment(test)
	savedInstallation(test, environment)
	installedRunner(test, environment, "Proton-GE Latest")
	switched, err := installationResolver(environment).Resolve(parsedSetupFlags(test, "--proton", "Proton-GE Latest"))
	if err != nil || switched.SourceRunner() != "Proton-GE Latest" || switched.FontVariant() != fonts.Japanese {
		test.Errorf("switched = %+v, %v", switched, err)
	}
	if _, err := installationResolver(environment).Resolve(parsedSetupFlags(test, "--proton", "missing")); !errors.Is(err, runner.ErrRunnerNotFound) {
		test.Errorf("err = %v, want ErrRunnerNotFound", err)
	}
}

func TestRequestedPrefixBecomesAbsolute(test *testing.T) {
	environment := isolatedEnvironment(test)
	savedInstallation(test, environment)
	moved, err := installationResolver(environment).Resolve(parsedSetupFlags(test, "--prefix", "relative/prefix"))
	if err != nil || !filepath.IsAbs(moved.Prefix()) || filepath.Base(moved.Prefix()) != "prefix" {
		test.Errorf("moved = %q, %v", moved.Prefix(), err)
	}
}

func TestSetupFlagsWithoutValuesRequestNothing(test *testing.T) {
	if got := parsedSetupFlags(test); got != install.NewSetupRequest("", "", "") {
		test.Errorf("request = %+v", got)
	}
	if _, err := parseSetupFlags([]string{"--unknown"}); err == nil {
		test.Error("an unknown flag was accepted")
	}
}

func TestFreshSetupDetectsTheSourceProtonAndLocaleFont(test *testing.T) {
	environment := isolatedEnvironment(test)
	test.Setenv("LC_ALL", "")
	test.Setenv("LC_CTYPE", "")
	test.Setenv("LANG", "zh_TW.UTF-8")
	installedRunner(test, environment, "Proton-CachyOS Latest")
	resolved, err := installationResolver(environment).Resolve(parsedSetupFlags(test))
	if err != nil || resolved.SourceRunner() != "Proton-CachyOS Latest" || resolved.FontVariant() != fonts.TraditionalChinese {
		test.Errorf("resolved = %+v, %v", resolved, err)
	}
}

func TestDetectEnvironmentStartsFromTheHomeDirectory(test *testing.T) {
	environment := isolatedEnvironment(test)
	detected, err := detectEnvironment()
	if err != nil || detected != environment {
		test.Errorf("detectEnvironment = %+v, %v; want %+v", detected, err, environment)
	}
}

func TestConfiguredInstallationNeedsASavedSetup(test *testing.T) {
	environment := isolatedEnvironment(test)
	if _, err := installationResolver(environment).Configured(); !errors.Is(err, install.ErrNotConfigured) {
		test.Errorf("err = %v, want ErrNotConfigured", err)
	}
	saved := savedInstallation(test, environment)
	loaded, err := installationResolver(environment).Configured()
	if err != nil || loaded != saved {
		test.Errorf("Configured = %+v, %v; want %+v", loaded, err, saved)
	}
}
