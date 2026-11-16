package application_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/pan-daniel/line-proton/internal/application"
	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/domain/launcher"
	"github.com/pan-daniel/line-proton/internal/domain/prefix"
	"github.com/pan-daniel/line-proton/internal/domain/runner"
)

type fixedUmu struct {
	description string
	err         error
}

func (umu fixedUmu) Describe() (string, error) {
	return umu.description, umu.err
}

type fixedCertificate bool

func (present fixedCertificate) Exists() bool {
	return bool(present)
}

func existingPaths(paths ...string) application.PathProbe {
	return func(path string) bool { return slices.Contains(paths, path) }
}

func runningAt(path string) application.RunningExecutable {
	return func() (string, error) { return path, nil }
}

func signedPrefix() fakeInventory {
	inventory := newFakeInventory(newJournal())
	inventory.marker = prefix.MarkedAt(noon)
	return inventory
}

func catalogWithLineAt(test *testing.T, lineVersion string) fakeCatalog {
	test.Helper()
	return fakeCatalog{runners: map[string]runner.Runner{
		"Proton-CachyOS Latest": runnerNamed(test, "Proton-CachyOS Latest", "2 new"),
		runner.LineRunnerName:   runnerNamed(test, runner.LineRunnerName, lineVersion),
	}, journal: newJournal()}
}

func healthyDependencies(test *testing.T, installation install.Installation) application.DoctorDependencies {
	test.Helper()
	return application.DoctorDependencies{
		Installation:      installation,
		Umu:               fixedUmu{description: "python3 umu-run"},
		Runners:           catalogWithLineAt(test, "2 new"),
		Prefix:            signedPrefix(),
		Fonts:             fixedFontLibrary{},
		Certificate:       fixedCertificate(true),
		Faugus:            fakeFaugus{fields: launcher.NewFaugusGame(installation, "/icon.png").Fields()},
		Desktop:           fakeDesktop{journal: newJournal()},
		RunningExecutable: runningAt("/usr/bin/line-proton"),
		ExecutableExists:  existingPaths("/usr/bin/line-proton"),
	}
}

func checkLine(report []string, name string) string {
	index := slices.IndexFunc(report, func(line string) bool { return strings.Contains(line, " "+name+" — ") })
	if index < 0 {
		return ""
	}
	return report[index]
}

func TestHealthyPortsGiveAHealthyReport(test *testing.T) {
	installation := installationFor(test, install.Faugus)
	if report := application.NewDoctor(healthyDependencies(test, installation)).Examine(); !report.Healthy() {
		test.Errorf("lines = %q", report.Lines())
	}
}

func TestDoctorChecksTheFaugusHooks(test *testing.T) {
	installation := installationFor(test, install.Faugus)
	current := launcher.NewFaugusGame(installation, "/icon.png").Fields()
	moved, err := installation.WithExecutable("/old/line-proton")
	if err != nil {
		test.Fatal(err)
	}
	stale := launcher.NewFaugusGame(moved, "/icon.png").Fields()
	examples := []struct {
		name    string
		fields  map[string]string
		running string
		exists  application.PathProbe
		want    string
	}{
		{"hooks into this binary", current, "/usr/bin/line-proton", existingPaths("/usr/bin/line-proton"), "✓ faugus"},
		{"hooks into another binary", stale, "/usr/bin/line-proton", existingPaths("/usr/bin/line-proton"), "✗ faugus — hooks run another binary"},
		{"hook binary deleted", current, "/home/u/bin/line-proton", existingPaths("/home/u/bin/line-proton"), "✗ faugus — the hook binary /usr/bin/line-proton is gone"},
		{"no LINE entry", nil, "/usr/bin/line-proton", existingPaths("/usr/bin/line-proton"), "✗ faugus — no LINE entry"},
	}
	for _, example := range examples {
		test.Run(example.name, func(test *testing.T) {
			dependencies := healthyDependencies(test, installation)
			dependencies.Faugus = fakeFaugus{fields: example.fields}
			dependencies.RunningExecutable = runningAt(example.running)
			dependencies.ExecutableExists = example.exists
			if got := checkLine(application.NewDoctor(dependencies).Examine().Lines(), "faugus"); !strings.HasPrefix(got, example.want) {
				test.Errorf("faugus line = %q, want prefix %q", got, example.want)
			}
		})
	}
}

func TestDoctorRedependenciesAnUnsignedPrefixAsUnhealthy(test *testing.T) {
	installation := installationFor(test, install.Standalone)
	dependencies := healthyDependencies(test, installation)
	dependencies.Prefix = newFakeInventory(newJournal())
	report := application.NewDoctor(dependencies).Examine()
	if report.Healthy() || !slices.Contains(report.Lines(), "✗ prefix — unsigned system DLLs; run `line-proton sign`") {
		test.Errorf("lines = %q", report.Lines())
	}
}

func TestDoctorShowsTheSourceAndProtonLineVersions(test *testing.T) {
	installation := installationFor(test, install.Standalone)
	examples := []struct {
		name        string
		lineVersion string
		want        string
	}{
		{"matching versions", "2 new", "✓ Proton-LINE — Proton-LINE 2 new matches Proton-CachyOS Latest 2 new"},
		{"outdated Proton-LINE", "1 old", "✗ Proton-LINE — outdated: Proton-LINE 1 old vs Proton-CachyOS Latest 2 new; run `line-proton runner build`"},
	}
	for _, example := range examples {
		test.Run(example.name, func(test *testing.T) {
			dependencies := healthyDependencies(test, installation)
			dependencies.Runners = catalogWithLineAt(test, example.lineVersion)
			if got := checkLine(application.NewDoctor(dependencies).Examine().Lines(), "Proton-LINE"); got != example.want {
				test.Errorf("runner line = %q, want %q", got, example.want)
			}
		})
	}
}

func TestDoctorRedependenciesWhatItCouldNotRead(test *testing.T) {
	denied := errors.New("permission denied")
	installation := installationFor(test, install.Faugus)
	unreadableRunners := catalogWithLineAt(test, "2 new")
	unreadableRunners.failures = map[string]error{runner.LineRunnerName: denied}
	unscannable, unmarkable := signedPrefix(), signedPrefix()
	unscannable.scanError = denied
	unmarkable.markerError = denied
	examples := []struct {
		name  string
		adapt func(dependencies application.DoctorDependencies) application.DoctorDependencies
		check string
		want  string
	}{
		{"unreadable runner versions", func(dependencies application.DoctorDependencies) application.DoctorDependencies {
			dependencies.Runners = unreadableRunners
			return dependencies
		}, "Proton-LINE", "✗ Proton-LINE — could not read the runner versions: permission denied"},
		{"unscannable prefix", func(dependencies application.DoctorDependencies) application.DoctorDependencies {
			dependencies.Prefix = unscannable
			return dependencies
		}, "prefix", "✗ prefix — could not check the system DLLs: permission denied"},
		{"unreadable signing marker", func(dependencies application.DoctorDependencies) application.DoctorDependencies {
			dependencies.Prefix = unmarkable
			return dependencies
		}, "prefix", "✗ prefix — could not check the system DLLs: permission denied"},
		{"corrupt games.json", func(dependencies application.DoctorDependencies) application.DoctorDependencies {
			dependencies.Faugus = fakeFaugus{unreadable: denied}
			return dependencies
		}, "faugus", "✗ faugus — could not read the Faugus games: permission denied"},
		{"unknown running binary", func(dependencies application.DoctorDependencies) application.DoctorDependencies {
			dependencies.RunningExecutable = func() (string, error) { return "", denied }
			return dependencies
		}, "faugus", "✗ faugus — could not find the running binary: permission denied"},
	}
	for _, example := range examples {
		test.Run(example.name, func(test *testing.T) {
			doctor := application.NewDoctor(example.adapt(healthyDependencies(test, installation)))
			if got := checkLine(doctor.Examine().Lines(), example.check); got != example.want {
				test.Errorf("%s line = %q, want %q", example.check, got, example.want)
			}
		})
	}
}

func TestDoctorAsksForUmuAndTheCertificate(test *testing.T) {
	installation := installationFor(test, install.Standalone)
	dependencies := healthyDependencies(test, installation)
	dependencies.Umu = fixedUmu{err: errors.New("umu-run not found")}
	dependencies.Certificate = fixedCertificate(false)
	lines := application.NewDoctor(dependencies).Examine().Lines()
	for _, want := range []string{"✗ umu-run — umu-run not found", "✗ certificate — missing; run `line-proton setup`"} {
		test.Run(want, func(test *testing.T) {
			if !slices.Contains(lines, want) {
				test.Errorf("lines = %q", lines)
			}
		})
	}
}
