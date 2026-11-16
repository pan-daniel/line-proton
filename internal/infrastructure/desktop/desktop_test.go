package desktop_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pan-daniel/line-proton/internal/domain/fonts"
	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/domain/launcher"
	"github.com/pan-daniel/line-proton/internal/infrastructure/desktop"
)

func TestInstallWritesAndRemoveDeletes(test *testing.T) {
	directory := filepath.Join(test.TempDir(), "applications")
	installation, err := install.New("/p", "Proton-GE Latest", fonts.Japanese, "/usr/bin/line-proton", install.Standalone)
	if err != nil {
		test.Fatal(err)
	}
	applications := desktop.New(directory)
	if err := applications.Install(launcher.NewDesktopEntry(installation, "/icon.png")); err != nil {
		test.Fatal(err)
	}
	content, _ := os.ReadFile(filepath.Join(directory, "line-proton.desktop"))
	if !applications.Installed() || !strings.Contains(string(content), "Name=LINE") {
		test.Errorf("installed = %v, content = %q", applications.Installed(), content)
	}
	if err := applications.Remove(); err != nil || applications.Installed() {
		test.Errorf("Remove = %v, installed = %v", err, applications.Installed())
	}
	if err := applications.Remove(); err != nil {
		test.Errorf("second Remove = %v", err)
	}
}
