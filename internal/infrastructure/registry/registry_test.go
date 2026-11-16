package registry_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pan-daniel/line-proton/internal/domain/winreg"
	"github.com/pan-daniel/line-proton/internal/infrastructure/registry"
)

func TestRender(test *testing.T) {
	patch := winreg.NewPatch(winreg.NewKey(`HKEY_CURRENT_USER\Test`,
		winreg.StringValue(`Say "hi"`, `C:\x`),
		winreg.DWordValue("Number", 0x578),
		winreg.MultiStringValue("List", "ab"),
	))
	want := "Windows Registry Editor Version 5.00\r\n" +
		"\r\n[HKEY_CURRENT_USER\\Test]\r\n" +
		"\"Say \\\"hi\\\"\"=\"C:\\\\x\"\r\n" +
		"\"Number\"=dword:00000578\r\n" +
		"\"List\"=hex(7):61,00,62,00,00,00,00,00\r\n"
	if got := registry.Render(patch); got != want {
		test.Errorf("Render =\n%q\nwant\n%q", got, want)
	}
}

type recordingWine struct {
	calls    [][]string
	contents []string
	file     string
}

func (wine *recordingWine) Run(_ context.Context, executable string, arguments ...string) error {
	wine.calls = append(wine.calls, append([]string{executable}, arguments...))
	content, _ := os.ReadFile(wine.file)
	wine.contents = append(wine.contents, string(content))
	return nil
}

func TestImporterRunsRegeditOnARenderedFileAndCleansUp(test *testing.T) {
	prefixDirectory := test.TempDir()
	file := filepath.Join(prefixDirectory, "drive_c", "line-proton.reg")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		test.Fatal(err)
	}
	wine := &recordingWine{file: file}
	patch := winreg.NewPatch(winreg.NewKey("HKEY_CURRENT_USER\\A", winreg.StringValue("b", "c")))
	if err := registry.NewImporter(prefixDirectory, wine).Import(test.Context(), patch); err != nil {
		test.Fatal(err)
	}
	if !slices.Equal(wine.calls[0], []string{"regedit", "/S", `C:\line-proton.reg`}) {
		test.Errorf("call = %v", wine.calls[0])
	}
	if !strings.Contains(wine.contents[0], `"b"="c"`) {
		test.Errorf("file content during import = %q", wine.contents[0])
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		test.Error("reg file was not removed")
	}
}
