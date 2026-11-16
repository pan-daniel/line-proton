package filesystem_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pan-daniel/line-proton/internal/infrastructure/filesystem"
)

func writeFile(test *testing.T, path, content string, mode os.FileMode) string {
	test.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		test.Fatal(err)
	}
	return path
}

func TestCloneTreeCopiesFilesModesTimesAndSymlinks(test *testing.T) {
	source, destination := test.TempDir(), filepath.Join(test.TempDir(), "copy")
	original := writeFile(test, filepath.Join(source, "files/lib/a.dll"), "payload", 0o555)
	lastYear := time.Date(2025, 11, 15, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(original, lastYear, lastYear); err != nil {
		test.Fatal(err)
	}
	if err := os.Symlink("lib/a.dll", filepath.Join(source, "files/link.dll")); err != nil {
		test.Fatal(err)
	}
	if err := filesystem.CloneTree(source, destination); err != nil {
		test.Fatal(err)
	}
	copied := filepath.Join(destination, "files/lib/a.dll")
	content, err := os.ReadFile(copied)
	if err != nil || string(content) != "payload" {
		test.Errorf("content = %q, %v", content, err)
	}
	details, err := os.Stat(copied)
	if err != nil || details.Mode().Perm() != 0o555 || !details.ModTime().Equal(lastYear) {
		test.Errorf("details = %v %v, %v", details.Mode().Perm(), details.ModTime(), err)
	}
	if target, _ := os.Readlink(filepath.Join(destination, "files/link.dll")); target != "lib/a.dll" {
		test.Errorf("symlink target = %q", target)
	}
}

func TestExchangeSwapsTwoDirectories(test *testing.T) {
	root := test.TempDir()
	writeFile(test, filepath.Join(root, "new", "version"), "new", 0o644)
	writeFile(test, filepath.Join(root, "current", "version"), "old", 0o644)
	if err := filesystem.Exchange(filepath.Join(root, "new"), filepath.Join(root, "current")); err != nil {
		test.Fatal(err)
	}
	current, _ := os.ReadFile(filepath.Join(root, "current", "version"))
	previous, _ := os.ReadFile(filepath.Join(root, "new", "version"))
	if string(current) != "new" || string(previous) != "old" {
		test.Errorf("current = %q, previous = %q", current, previous)
	}
}

func TestExchangeFailsWhenOneSideIsMissing(test *testing.T) {
	root := test.TempDir()
	writeFile(test, filepath.Join(root, "current", "version"), "old", 0o644)
	if err := filesystem.Exchange(filepath.Join(root, "missing"), filepath.Join(root, "current")); err == nil {
		test.Error("exchange with a missing directory succeeded")
	}
	if content, _ := os.ReadFile(filepath.Join(root, "current", "version")); string(content) != "old" {
		test.Errorf("current = %q", content)
	}
}

func TestLockIsExclusive(test *testing.T) {
	lock := filesystem.NewLock(filepath.Join(test.TempDir(), "state", "lock"))
	release, acquired, err := lock.TryAcquire()
	if err != nil || !acquired {
		test.Fatalf("first TryAcquire = %v, %v", acquired, err)
	}
	if _, again, err := lock.TryAcquire(); err != nil || again {
		test.Errorf("second TryAcquire = %v, %v; want not acquired", again, err)
	}
	if err := release(); err != nil {
		test.Fatal(err)
	}
	releaseAgain, acquired, err := lock.TryAcquire()
	if err != nil || !acquired {
		test.Errorf("TryAcquire after release = %v, %v", acquired, err)
	}
	if err := releaseAgain(); err != nil {
		test.Error(err)
	}
}

func TestDirectoryContentsRemovalKeepsTheNamedEntries(test *testing.T) {
	state := test.TempDir()
	lock := writeFile(test, filepath.Join(state, "lock"), "", 0o600)
	hookLog := writeFile(test, filepath.Join(state, "hooks.log"), "log", 0o600)
	download := writeFile(test, filepath.Join(state, "downloads", "LineInst.exe"), "MZ", 0o600)
	if err := filesystem.NewDirectoryContents(state, "lock").Remove(); err != nil {
		test.Fatal(err)
	}
	if _, err := os.Stat(lock); err != nil {
		test.Errorf("lock was removed: %v", err)
	}
	for _, removed := range []string{hookLog, filepath.Dir(download)} {
		test.Run(filepath.Base(removed), func(test *testing.T) {
			if _, err := os.Stat(removed); !os.IsNotExist(err) {
				test.Errorf("%s survived: %v", removed, err)
			}
		})
	}
}

func TestDirectoryContentsRemovalToleratesAMissingDirectory(test *testing.T) {
	if err := filesystem.NewDirectoryContents(filepath.Join(test.TempDir(), "missing"), "lock").Remove(); err != nil {
		test.Errorf("Remove = %v", err)
	}
}

func TestSameFilesystemComparesDevices(test *testing.T) {
	root := test.TempDir()
	if same, err := filesystem.SameFilesystem(root, writeFile(test, filepath.Join(root, "a", "b"), "", 0o644)); err != nil || !same {
		test.Errorf("same directory tree = %v, %v", same, err)
	}
	if same, err := filesystem.SameFilesystem(root, "/proc"); err != nil || same {
		test.Errorf("temporary directory and /proc = %v, %v", same, err)
	}
}
