package desktop

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pan-daniel/line-proton/internal/domain/launcher"
)

const fileName = "line-proton.desktop"

type Applications struct {
	directory string
}

func New(directory string) Applications {
	return Applications{directory: directory}
}

func (applications Applications) Install(entry launcher.DesktopEntry) error {
	if err := os.MkdirAll(applications.directory, 0o755); err != nil {
		return err
	}
	return os.WriteFile(applications.path(), []byte(entry.Render()), 0o644)
}

func (applications Applications) Installed() bool {
	_, err := os.Stat(applications.path())
	return err == nil
}

func (applications Applications) Remove() error {
	err := os.Remove(applications.path())
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func (applications Applications) path() string {
	return filepath.Join(applications.directory, fileName)
}
