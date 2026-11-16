package prefixfs_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pan-daniel/line-proton/internal/domain/prefix"
	"github.com/pan-daniel/line-proton/internal/infrastructure/prefixfs"
)

type layout struct {
	prefix string
	runner string
}

func newLayout(test *testing.T) layout {
	test.Helper()
	root := test.TempDir()
	built := layout{prefix: filepath.Join(root, "prefix"), runner: filepath.Join(root, "Proton-LINE")}
	system32 := filepath.Join(built.prefix, "drive_c/windows/system32")
	mustMkdir(test, system32)
	mustMkdir(test, built.runner)
	mustWrite(test, filepath.Join(system32, "regular.dll"), "regular")
	mustWrite(test, filepath.Join(system32, "notepad.exe"), "program")
	mustWrite(test, filepath.Join(built.runner, "signed.dll"), "signed")
	mustWrite(test, filepath.Join(root, "foreign.dll"), "foreign")
	mustWrite(test, filepath.Join(system32, "notes.txt"), "ignored")
	mustLink(test, filepath.Join(built.runner, "signed.dll"), filepath.Join(system32, "runner.dll"))
	mustLink(test, filepath.Join(root, "foreign.dll"), filepath.Join(system32, "foreign.dll"))
	mustLink(test, filepath.Join(built.runner, "removed.dll"), filepath.Join(system32, "broken.dll"))
	return built
}

func mustMkdir(test *testing.T, path string) string {
	test.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		test.Fatal(err)
	}
	return path
}

func mustWrite(test *testing.T, path, content string) string {
	test.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		test.Fatal(err)
	}
	return path
}

func mustLink(test *testing.T, target, link string) string {
	test.Helper()
	if err := os.Symlink(target, link); err != nil {
		test.Fatal(err)
	}
	return link
}

func kinds(files []prefix.SystemFile) map[string]prefix.Kind {
	found := map[string]prefix.Kind{}
	for _, file := range files {
		found[filepath.Base(file.Path())] = file.Kind()
	}
	return found
}

func TestScanClassifiesSystemBinaries(test *testing.T) {
	built := newLayout(test)
	files, err := prefixfs.New(built.prefix, built.runner).Scan()
	if err != nil {
		test.Fatal(err)
	}
	got := kinds(files)
	want := map[string]prefix.Kind{
		"regular.dll": prefix.RegularFile,
		"notepad.exe": prefix.RegularFile,
		"runner.dll":  prefix.LinkIntoLineRunner,
		"foreign.dll": prefix.LinkElsewhere,
		"broken.dll":  prefix.BrokenLink,
	}
	if len(got) != len(want) {
		test.Errorf("kinds = %v, want %v", got, want)
	}
	for name, kind := range want {
		test.Run(name, func(test *testing.T) {
			if got[name] != kind {
				test.Errorf("kind = %v, want %v", got[name], kind)
			}
		})
	}
}

func TestDetachLinkLeavesARegularCopy(test *testing.T) {
	built := newLayout(test)
	link := filepath.Join(built.prefix, "drive_c/windows/system32/foreign.dll")
	if err := prefixfs.New(built.prefix, built.runner).DetachLink(link); err != nil {
		test.Fatal(err)
	}
	details, err := os.Lstat(link)
	content, _ := os.ReadFile(link)
	if err != nil || !details.Mode().IsRegular() || string(content) != "foreign" {
		test.Errorf("detached = %v, %q, %v", details.Mode(), content, err)
	}
}

func TestMarkerRoundTrip(test *testing.T) {
	built := newLayout(test)
	files := prefixfs.New(built.prefix, built.runner)
	before, err := files.Marker()
	if err != nil || before.CoversAll(nil) {
		test.Fatalf("fresh marker covers = %v, %v", before.CoversAll(nil), err)
	}
	signedAt := time.Now().Add(time.Hour).Truncate(time.Second)
	if err := files.Mark(signedAt); err != nil {
		test.Fatal(err)
	}
	after, _ := files.Marker()
	regular := prefix.NewSystemFile("regular.dll", prefix.RegularFile, signedAt.Add(-time.Minute))
	if !after.CoversAll([]prefix.SystemFile{regular}) || !files.Exists(prefix.MarkerName) {
		test.Error("marker should cover a file written before it")
	}
}

func TestAddFontsCopiesIntoWindowsFonts(test *testing.T) {
	built := newLayout(test)
	font := filepath.Join(test.TempDir(), "NotoSansThai-Regular.ttf")
	mustWrite(test, font, "font")
	files := prefixfs.New(built.prefix, built.runner)
	if err := files.AddFonts([]string{font}); err != nil {
		test.Fatal(err)
	}
	if !files.Exists("drive_c/windows/Fonts/NotoSansThai-Regular.ttf") {
		test.Error("font was not copied")
	}
}

func TestReadSystemRegistry(test *testing.T) {
	built := newLayout(test)
	mustWrite(test, filepath.Join(built.prefix, "system.reg"), "WINE REGISTRY Version 2\n")
	content, err := prefixfs.New(built.prefix, built.runner).ReadSystemRegistry()
	if err != nil || content != "WINE REGISTRY Version 2\n" {
		test.Errorf("ReadSystemRegistry = %q, %v", content, err)
	}
}
