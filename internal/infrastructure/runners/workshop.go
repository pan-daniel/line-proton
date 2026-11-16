package runners

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/pan-daniel/line-proton/internal/domain/runner"
	"github.com/pan-daniel/line-proton/internal/infrastructure/filesystem"
)

var ErrCrossFilesystem = errors.New("Proton-LINE cannot be swapped in atomically because the staging directory and compatibilitytools.d are on different filesystems")

type Workshop struct {
	runnersDirectory string
	stagingDirectory string
	logger           *slog.Logger
}

func NewWorkshop(runnersDirectory, stagingDirectory string, logger *slog.Logger) Workshop {
	return Workshop{runnersDirectory: runnersDirectory, stagingDirectory: stagingDirectory, logger: logger}
}

func (workshop Workshop) Stage(source runner.Runner) (runner.Staging, error) {
	if err := workshop.ensureSameFilesystem(); err != nil {
		return nil, err
	}
	staged := filepath.Join(workshop.stagingDirectory, runner.LineRunnerName)
	if err := emptyDirectoryFor(staged); err != nil {
		return nil, err
	}
	if err := buildCopy(filepath.Join(workshop.runnersDirectory, source.Name()), staged); err != nil {
		return nil, errors.Join(err, os.RemoveAll(staged))
	}
	return Staging{path: staged, final: filepath.Join(workshop.runnersDirectory, runner.LineRunnerName), logger: workshop.logger}, nil
}

func (workshop Workshop) ensureSameFilesystem() error {
	same, err := filesystem.SameFilesystem(filepath.Dir(workshop.stagingDirectory), workshop.runnersDirectory)
	if err != nil {
		return err
	}
	if !same {
		return fmt.Errorf("%w: %s is not on the filesystem of %s", ErrCrossFilesystem, workshop.stagingDirectory, workshop.runnersDirectory)
	}
	return nil
}

func emptyDirectoryFor(staged string) error {
	if err := os.RemoveAll(staged); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Dir(staged), 0o700)
}

func buildCopy(source, staged string) error {
	if err := filesystem.CloneTree(source, staged); err != nil {
		return err
	}
	manifest := filepath.Join(staged, runner.ToolManifestName)
	if err := os.RemoveAll(manifest); err != nil {
		return err
	}
	return os.WriteFile(manifest, []byte(runner.ToolManifest()), 0o644)
}

type Staging struct {
	path   string
	final  string
	logger *slog.Logger
}

func (staging Staging) Binaries() ([]string, error) {
	var binaries []string
	err := filepath.WalkDir(staging.path, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		binaries = appendBinary(binaries, staging.path, path, entry)
		return nil
	})
	return binaries, err
}

func (staging Staging) Publish() error {
	if err := moveIntoPlace(staging.path, staging.final); err != nil {
		return err
	}
	return staging.removeReplacedTree()
}

func (staging Staging) removeReplacedTree() error {
	if err := os.RemoveAll(staging.path); err != nil {
		staging.logger.Warn("Proton-LINE is updated, but the replaced copy could not be removed", "path", staging.path, "error", err.Error())
	}
	return nil
}

func (staging Staging) Discard() error {
	return os.RemoveAll(staging.path)
}

func moveIntoPlace(staged, final string) error {
	if _, err := os.Lstat(final); errors.Is(err, fs.ErrNotExist) {
		return os.Rename(staged, final)
	}
	return filesystem.Exchange(staged, final)
}

func appendBinary(binaries []string, root, path string, entry fs.DirEntry) []string {
	relative, err := filepath.Rel(root, path)
	if err != nil || !entry.Type().IsRegular() || !runner.IsSignedBinary(relative) {
		return binaries
	}
	return append(binaries, path)
}
