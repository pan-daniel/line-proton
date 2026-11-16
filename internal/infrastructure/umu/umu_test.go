package umu

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLocatePrefersFaugusBundledUmuRun(test *testing.T) {
	home := test.TempDir()
	bundled := filepath.Join(home, ".local/share/faugus-launcher/umu-run")
	if err := os.MkdirAll(filepath.Dir(bundled), 0o755); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(bundled, nil, 0o755); err != nil {
		test.Fatal(err)
	}
	command, err := Locate(home, func(string) (string, error) { return "/usr/bin/umu-run", nil })
	if err != nil || command.program != "python3" || command.leading[0] != bundled {
		test.Errorf("Locate = %+v, %v", command, err)
	}
}

func TestLocateFallsBackToSystemUmuRun(test *testing.T) {
	command, err := Locate(test.TempDir(), func(string) (string, error) { return "/usr/bin/umu-run", nil })
	if err != nil || command.program != "/usr/bin/umu-run" {
		test.Errorf("Locate = %+v, %v", command, err)
	}
	_, err = Locate(test.TempDir(), func(string) (string, error) { return "", errors.New("missing") })
	if !errors.Is(err, ErrNotFound) {
		test.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestProtonRunsWithPrefixRunnerAndFixedEnvironment(test *testing.T) {
	var output bytes.Buffer
	script := `printf '%s|%s|%s|%s|%s' "$WINEPREFIX" "$PROTONPATH" "$GAMEID" "$PROTON_FSR4_UPGRADE" "$1"`
	command := Command{program: "sh", leading: []string{"-c", script, "sh"}}
	if err := NewProton(command, "/p", "/r/Proton-LINE", &output).Run(test.Context(), "LineLauncher.exe"); err != nil {
		test.Fatal(err)
	}
	if got := output.String(); got != "/p|/r/Proton-LINE|umu-default|0|LineLauncher.exe" {
		test.Errorf("environment = %q", got)
	}
}

func TestLocatorDescribesTheCommandItFinds(test *testing.T) {
	description, err := NewLocator(test.TempDir(), func(string) (string, error) { return "/usr/bin/umu-run", nil }).Describe()
	if err != nil || description != "/usr/bin/umu-run" {
		test.Errorf("Describe = %q, %v", description, err)
	}
	if _, err := NewLocator(test.TempDir(), func(string) (string, error) { return "", errors.New("missing") }).Describe(); !errors.Is(err, ErrNotFound) {
		test.Errorf("err = %v, want ErrNotFound", err)
	}
}
