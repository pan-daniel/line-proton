package main

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

const apacheText = "Apache License\nVersion 2.0\n"
const bsdText = "BSD 3-Clause\n"

func fakeModules() map[string]fstest.MapFS {
	return map[string]fstest.MapFS{
		"/mod/relic": {
			"LICENSE":   {Data: []byte(apacheText)},
			"NOTICE":    {Data: []byte("relic notice\n")},
			"README.md": {Data: []byte("not a licence\n")},
		},
		"/mod/sys": {
			"LICENSE": {Data: []byte(bsdText)},
			"PATENTS": {Data: []byte("patent grant")},
		},
		"/mod/unlicensed": {
			"README.md": {Data: []byte("no licence here\n")},
			"PATENTS":   {Data: []byte("patent grant\n")},
		},
	}
}

func openFake(directory string) fs.FS {
	return fakeModules()[directory]
}

const fakeListing = "\n" +
	"golang.org/x/sys v0.50.0 /mod/sys\n" +
	"\n" +
	"github.com/sassoftware/relic/v8 v8.2.0 /mod/relic\n" +
	"golang.org/x/sys v0.50.0 /mod/sys\n"

var expectedNotice = noticeHeader +
	"\n" + rule + "\ngithub.com/sassoftware/relic/v8 v8.2.0\n" + rule + "\n" +
	"\n--- LICENSE ---\n\n" + apacheText +
	"\n--- NOTICE ---\n\nrelic notice\n" +
	"\n" + rule + "\ngolang.org/x/sys v0.50.0\n" + rule + "\n" +
	"\n--- LICENSE ---\n\n" + bsdText +
	"\n--- PATENTS ---\n\npatent grant\n"

type recordedWorkspace struct {
	written   *string
	workspace workspace
}

func fakeWorkspace(listing string, committed string) recordedWorkspace {
	written := new(string)
	return recordedWorkspace{
		written: written,
		workspace: workspace{
			listModules: func(context.Context) (string, error) { return listing, nil },
			openModule:  openFake,
			readNotice:  func() ([]byte, error) { return []byte(committed), nil },
			writeNotice: func(content []byte) error {
				*written = string(content)
				return nil
			},
		},
	}
}

func silentLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func TestParseModulesSortsAndDeduplicates(test *testing.T) {
	modules, err := parseModules(fakeListing)
	if err != nil {
		test.Fatal(err)
	}
	paths := []string{}
	for _, module := range modules {
		paths = append(paths, module.path)
	}
	want := []string{"github.com/sassoftware/relic/v8", "golang.org/x/sys"}
	if !slices.Equal(paths, want) {
		test.Errorf("paths = %q, want %q", paths, want)
	}
}

func TestParseModulesKeepsDirectoriesWithSpaces(test *testing.T) {
	modules, err := parseModules("example.com/spaced v1.0.0 /home/some one/mod\n")
	if err != nil {
		test.Fatal(err)
	}
	if modules[0].directory != "/home/some one/mod" {
		test.Errorf("directory = %q", modules[0].directory)
	}
}

func TestParseModulesRejectsMalformedLines(test *testing.T) {
	if _, err := parseModules("only-a-path\n"); err == nil {
		test.Error("malformed line was accepted")
	}
}

func TestRenderListsLicenceFilesOfEveryModule(test *testing.T) {
	modules, err := parseModules(fakeListing)
	if err != nil {
		test.Fatal(err)
	}
	notice, err := render(modules, openFake)
	if err != nil {
		test.Fatal(err)
	}
	if notice != expectedNotice {
		test.Errorf("notice =\n%s\nwant\n%s", notice, expectedNotice)
	}
}

func TestRenderFailsForModuleWithoutLicence(test *testing.T) {
	modules, err := parseModules("example.com/unlicensed v1.0.0 /mod/unlicensed\n")
	if err != nil {
		test.Fatal(err)
	}
	_, err = render(modules, openFake)
	if err == nil || !strings.Contains(err.Error(), "example.com/unlicensed") {
		test.Errorf("err = %v, want one naming the module", err)
	}
}

func TestWriteStoresGeneratedNotice(test *testing.T) {
	recorded := fakeWorkspace(fakeListing, "")
	if exitCode := run(test.Context(), []string{"write"}, recorded.workspace, silentLogger()); exitCode != 0 {
		test.Fatalf("exit code = %d, want 0", exitCode)
	}
	if *recorded.written != expectedNotice {
		test.Errorf("written =\n%s", *recorded.written)
	}
}

func TestCheckPassesWhenCommittedNoticeIsCurrent(test *testing.T) {
	recorded := fakeWorkspace(fakeListing, expectedNotice)
	if exitCode := run(test.Context(), []string{"check"}, recorded.workspace, silentLogger()); exitCode != 0 {
		test.Errorf("exit code = %d, want 0", exitCode)
	}
}

func TestCheckFailsWhenCommittedNoticeIsOutdated(test *testing.T) {
	recorded := fakeWorkspace(fakeListing, "stale")
	if exitCode := run(test.Context(), []string{"check"}, recorded.workspace, silentLogger()); exitCode != 1 {
		test.Errorf("exit code = %d, want 1", exitCode)
	}
}

func TestCheckFailsWhenCommittedNoticeIsMissing(test *testing.T) {
	recorded := fakeWorkspace(fakeListing, "")
	recorded.workspace.readNotice = func() ([]byte, error) { return nil, fs.ErrNotExist }
	if exitCode := run(test.Context(), []string{"check"}, recorded.workspace, silentLogger()); exitCode != 1 {
		test.Errorf("exit code = %d, want 1", exitCode)
	}
}

func TestUnknownCommandIsAUsageError(test *testing.T) {
	recorded := fakeWorkspace(fakeListing, "")
	if exitCode := run(test.Context(), []string{"publish"}, recorded.workspace, silentLogger()); exitCode != 2 {
		test.Errorf("exit code = %d, want 2", exitCode)
	}
}

func TestRealModulesOfThisRepositoryHaveLicences(test *testing.T) {
	listing, err := goListModules("../../..")(test.Context())
	if err != nil {
		test.Fatal(err)
	}
	modules, err := parseModules(listing)
	if err != nil {
		test.Fatal(err)
	}
	paths := []string{}
	for _, module := range modules {
		paths = append(paths, module.path)
	}
	if !slices.Contains(paths, "github.com/sassoftware/relic/v8") || slices.Contains(paths, "github.com/pan-daniel/line-proton") {
		test.Errorf("paths = %q, want relic and not the main module", paths)
	}
	notice, err := render(modules, os.DirFS)
	if err != nil {
		test.Fatal(err)
	}
	if !strings.Contains(notice, "Apache License") {
		test.Error("relic's Apache licence is missing from the notice")
	}
}
