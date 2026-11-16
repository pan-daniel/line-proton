package launcher_test

import (
	"strings"
	"testing"

	"github.com/pan-daniel/line-proton/internal/domain/fonts"
	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/domain/launcher"
)

func installationWith(test *testing.T, kind install.Launcher, executable string) install.Installation {
	test.Helper()
	built, err := install.New("/home/u/Faugus/line", "Proton-CachyOS Latest", fonts.Japanese, executable, kind)
	if err != nil {
		test.Fatal(err)
	}
	return built
}

func installation(test *testing.T, kind install.Launcher) install.Installation {
	test.Helper()
	return installationWith(test, kind, "/home/u/bin/line proton")
}

func TestFaugusGameRunsLineOnProtonLineWithHooks(test *testing.T) {
	fields := launcher.NewFaugusGame(installation(test, install.Faugus), "/home/u/.local/share/faugus-launcher/icons/line.png").Fields()
	expectations := map[string]string{
		"gameid":           "line",
		"title":            "LINE",
		"runner":           "Proton-LINE",
		"prefix":           "/home/u/Faugus/line",
		"path":             "/home/u/Faugus/line/drive_c/users/steamuser/AppData/Local/LINE/bin/LineLauncher.exe",
		"launch_arguments": "PROTON_ENABLE_WAYLAND=0 PROTON_FSR4_UPGRADE=0",
		"icon":             "/home/u/.local/share/faugus-launcher/icons/line.png",
		"pre_launch":       "'/home/u/bin/line proton' hook pre",
		"post_launch":      "'/home/u/bin/line proton' hook post",
	}
	for key, want := range expectations {
		test.Run(key, func(test *testing.T) {
			if fields[key] != want {
				test.Errorf("%s = %q, want %q", key, fields[key], want)
			}
		})
	}
}

func TestHooksPointTo(test *testing.T) {
	fields := launcher.NewFaugusGame(installation(test, install.Faugus), "/icon.png").Fields()
	if !launcher.HooksPointTo(fields, "/home/u/bin/line proton") {
		test.Error("hooks should point to the executable")
	}
	if launcher.HooksPointTo(fields, "/elsewhere/line-proton") {
		test.Error("hooks should not point to another executable")
	}
}

func TestShellQuoteEscapesSingleQuotes(test *testing.T) {
	if got := launcher.ShellQuote("it's"); got != `'it'\''s'` {
		test.Errorf("ShellQuote = %s", got)
	}
}

func TestDesktopEntryExecDependsOnLauncher(test *testing.T) {
	examples := []struct {
		name         string
		installation install.Installation
		want         string
	}{
		{"faugus", installation(test, install.Faugus), "Exec=faugus-launcher --game line"},
		{"standalone", installation(test, install.Standalone), `Exec="/home/u/bin/line proton" launch`},
		{"standalone with reserved characters", installationWith(test, install.Standalone, `/opt/a$b\c%d"e`), `Exec="/opt/a\\$b\\\\c%%d\\"e" launch`},
	}
	for _, example := range examples {
		test.Run(example.name, func(test *testing.T) {
			rendered := launcher.NewDesktopEntry(example.installation, "/icon.png").Render()
			if !strings.Contains(rendered, example.want+"\n") {
				test.Errorf("entry missing %q:\n%s", example.want, rendered)
			}
		})
	}
}
