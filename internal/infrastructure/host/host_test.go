package host_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pan-daniel/line-proton/internal/domain/fonts"
	"github.com/pan-daniel/line-proton/internal/infrastructure/host"
)

func TestLocaleReadsTheThreePosixVariables(test *testing.T) {
	variables := map[string]string{"LC_ALL": "ko_KR.UTF-8", "LC_CTYPE": "zh_CN.UTF-8", "LANG": "zh_TW.UTF-8", "LC_MESSAGES": "ja_JP.UTF-8"}
	got := host.NewLocale(func(name string) string { return variables[name] }).LocaleSettings()
	if want := fonts.NewLocaleSettings("ko_KR.UTF-8", "zh_CN.UTF-8", "zh_TW.UTF-8"); got != want {
		test.Errorf("LocaleSettings = %+v, want %+v", got, want)
	}
}

func TestExecutableResolvesSymlinks(test *testing.T) {
	directory := test.TempDir()
	binary := filepath.Join(directory, "line-proton")
	if err := os.WriteFile(binary, nil, 0o755); err != nil {
		test.Fatal(err)
	}
	link := filepath.Join(directory, "link")
	if err := os.Symlink(binary, link); err != nil {
		test.Fatal(err)
	}
	located, err := host.NewExecutable(func() (string, error) { return link, nil }).Locate()
	if err != nil || located != binary {
		test.Errorf("Locate = %q, %v; want %q", located, err, binary)
	}
}

func TestExecutableReportsWhenTheRunningBinaryIsUnknown(test *testing.T) {
	unknown := errors.New("no /proc")
	if _, err := host.NewExecutable(func() (string, error) { return "", unknown }).Locate(); !errors.Is(err, unknown) {
		test.Errorf("err = %v, want %v", err, unknown)
	}
}
