package runners_test

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pan-daniel/line-proton/internal/domain/runner"
	"github.com/pan-daniel/line-proton/internal/infrastructure/runners"
)

type workbench struct {
	runners string
	staging string
}

func newWorkbench(test *testing.T) workbench {
	test.Helper()
	home := test.TempDir()
	return workbench{
		runners: filepath.Join(home, ".local/share/Steam/compatibilitytools.d"),
		staging: filepath.Join(home, ".local/share/Steam/.line-proton-staging"),
	}
}

func fakeRunner(test *testing.T, directory, name, version string) string {
	test.Helper()
	files := map[string]string{
		"version": version,
		"files/lib/wine/x86_64-windows/gdiplus.dll": "pe",
		"files/bin/wine":        "elf",
		runner.ToolManifestName: `"compatibilitytools" { "compat_tools" { "` + name + `" { } } }`,
	}
	for relative, content := range files {
		writeFile(test, filepath.Join(directory, name, relative), content)
	}
	return filepath.Join(directory, name)
}

func writeFile(test *testing.T, path, content string) string {
	test.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		test.Fatal(err)
	}
	return path
}

func stageFrom(test *testing.T, bench workbench, sourceName string) runner.Staging {
	test.Helper()
	return stageLoggedFrom(test, bench, sourceName, slog.New(slog.DiscardHandler))
}

func stageLoggedFrom(test *testing.T, bench workbench, sourceName string, logger *slog.Logger) runner.Staging {
	test.Helper()
	source, err := runners.NewCatalog(bench.runners).Find(sourceName)
	if err != nil {
		test.Fatal(err)
	}
	sourceRunner, present := source.Runner()
	if !present {
		test.Fatalf("source %s missing", sourceName)
	}
	staging, err := runners.NewWorkshop(bench.runners, bench.staging, logger).Stage(sourceRunner)
	if err != nil {
		test.Fatal(err)
	}
	return staging
}

func lineRunnerVersion(test *testing.T, bench workbench) string {
	test.Helper()
	line, err := runners.NewCatalog(bench.runners).Find(runner.LineRunnerName)
	published, present := line.Runner()
	if err != nil || !present {
		test.Fatalf("Proton-LINE = %v, %v", present, err)
	}
	return published.Version().String()
}

func TestCatalogListsDirectoriesWithAVersionFile(test *testing.T) {
	bench := newWorkbench(test)
	fakeRunner(test, bench.runners, "Proton-CachyOS Latest", "2 cachy\n")
	if err := os.Mkdir(filepath.Join(bench.runners, "not-a-runner"), 0o755); err != nil {
		test.Fatal(err)
	}
	installed, err := runners.NewCatalog(bench.runners).Installed()
	if err != nil || len(installed) != 1 || installed[0].Name() != "Proton-CachyOS Latest" || installed[0].Version().String() != "2 cachy" {
		test.Errorf("Installed = %v, %v", installed, err)
	}
}

func TestCatalogFindReportsAbsence(test *testing.T) {
	found, err := runners.NewCatalog(test.TempDir()).Find(runner.LineRunnerName)
	if _, present := found.Runner(); err != nil || present {
		test.Errorf("Find = %v, %v", present, err)
	}
}

func TestStagingHappensOutsideTheRunnersDirectoryUnderItsOwnName(test *testing.T) {
	bench := newWorkbench(test)
	fakeRunner(test, bench.runners, "Proton-CachyOS Latest", "2 cachy")
	staging := stageFrom(test, bench, "Proton-CachyOS Latest")
	binaries, err := staging.Binaries()
	want := []string{filepath.Join(bench.staging, runner.LineRunnerName, "files/lib/wine/x86_64-windows/gdiplus.dll")}
	if err != nil || !slices.Equal(binaries, want) {
		test.Errorf("Binaries = %v, %v", binaries, err)
	}
	manifest, err := os.ReadFile(filepath.Join(bench.staging, runner.LineRunnerName, runner.ToolManifestName))
	if err != nil || string(manifest) != runner.ToolManifest() {
		test.Errorf("manifest = %q, %v", manifest, err)
	}
	entries, _ := os.ReadDir(bench.runners)
	if len(entries) != 1 {
		test.Errorf("runners directory gained entries: %v", entries)
	}
}

func TestPublishSwapsInTheNewRunnerAndRemovesTheOldOne(test *testing.T) {
	bench := newWorkbench(test)
	fakeRunner(test, bench.runners, "Proton-CachyOS Latest", "2 cachy")
	fakeRunner(test, bench.runners, runner.LineRunnerName, "1 old")
	if err := stageFrom(test, bench, "Proton-CachyOS Latest").Publish(); err != nil {
		test.Fatal(err)
	}
	if got := lineRunnerVersion(test, bench); got != "2 cachy" {
		test.Errorf("published version = %q", got)
	}
	if _, err := os.Stat(filepath.Join(bench.staging, runner.LineRunnerName)); !os.IsNotExist(err) {
		test.Errorf("old runner still in staging: %v", err)
	}
}

func TestPublishMovesTheFirstBuildIntoPlace(test *testing.T) {
	bench := newWorkbench(test)
	fakeRunner(test, bench.runners, "Proton-GE Latest", "1 ge")
	if err := stageFrom(test, bench, "Proton-GE Latest").Publish(); err != nil {
		test.Fatal(err)
	}
	if got := lineRunnerVersion(test, bench); got != "1 ge" {
		test.Errorf("published version = %q", got)
	}
}

func TestFailedPublishKeepsTheOldRunner(test *testing.T) {
	bench := newWorkbench(test)
	fakeRunner(test, bench.runners, "Proton-CachyOS Latest", "2 cachy")
	fakeRunner(test, bench.runners, runner.LineRunnerName, "1 old")
	staging := stageFrom(test, bench, "Proton-CachyOS Latest")
	if err := os.RemoveAll(filepath.Join(bench.staging, runner.LineRunnerName)); err != nil {
		test.Fatal(err)
	}
	if err := staging.Publish(); err == nil {
		test.Error("publishing a vanished staging copy succeeded")
	}
	if got := lineRunnerVersion(test, bench); got != "1 old" {
		test.Errorf("Proton-LINE version = %q, want the old one", got)
	}
}

func TestDiscardRemovesStaging(test *testing.T) {
	bench := newWorkbench(test)
	fakeRunner(test, bench.runners, "Proton-GE Latest", "1 ge")
	if err := stageFrom(test, bench, "Proton-GE Latest").Discard(); err != nil {
		test.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(bench.staging, runner.LineRunnerName)); !os.IsNotExist(err) {
		test.Errorf("staging still exists: %v", err)
	}
}

func TestStagingRefusesADifferentFilesystemThanTheRunners(test *testing.T) {
	staging := filepath.Join(test.TempDir(), ".line-proton-staging")
	version, err := runner.ParseVersion("1 ge")
	if err != nil {
		test.Fatal(err)
	}
	source, err := runner.New("Proton-GE Latest", version)
	if err != nil {
		test.Fatal(err)
	}
	if _, err := runners.NewWorkshop("/proc", staging, slog.New(slog.DiscardHandler)).Stage(source); !errors.Is(err, runners.ErrCrossFilesystem) {
		test.Errorf("err = %v, want ErrCrossFilesystem", err)
	}
	if _, err := os.Stat(filepath.Join(staging, runner.LineRunnerName)); !os.IsNotExist(err) {
		test.Errorf("something was staged: %v", err)
	}
}

func TestPublishOnlyWarnsWhenTheOldTreeCannotBeRemoved(test *testing.T) {
	if os.Geteuid() == 0 {
		test.Skip("root removes read-only directories anyway")
	}
	bench := newWorkbench(test)
	fakeRunner(test, bench.runners, "Proton-CachyOS Latest", "2 cachy")
	stuck := filepath.Join(fakeRunner(test, bench.runners, runner.LineRunnerName, "1 old"), "files")
	if err := os.Chmod(stuck, 0o500); err != nil {
		test.Fatal(err)
	}
	test.Cleanup(func() { _ = os.Chmod(filepath.Join(bench.staging, runner.LineRunnerName, "files"), 0o755) })
	var logs bytes.Buffer
	if err := stageLoggedFrom(test, bench, "Proton-CachyOS Latest", slog.New(slog.NewTextHandler(&logs, nil))).Publish(); err != nil {
		test.Fatalf("Publish = %v after a successful swap", err)
	}
	if got := lineRunnerVersion(test, bench); got != "2 cachy" {
		test.Errorf("published version = %q", got)
	}
	if !strings.Contains(logs.String(), "level=WARN") {
		test.Errorf("logs = %q, want a warning about the old tree", logs.String())
	}
}
