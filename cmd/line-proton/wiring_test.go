package main

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/pan-daniel/line-proton/internal/domain/fonts"
	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/domain/signing"
)

func writtenFile(test *testing.T, path string) string {
	test.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("kept"), 0o600); err != nil {
		test.Fatal(err)
	}
	return path
}

func standaloneInstallationIn(test *testing.T, environment environment) install.Installation {
	test.Helper()
	installation, err := install.New(install.DefaultPrefix(environment.home, install.Standalone), "Proton-CachyOS Latest", fonts.Japanese, "/usr/bin/line-proton", install.Standalone)
	if err != nil {
		test.Fatal(err)
	}
	return installation
}

func TestPlainUninstallKeepsTheStandaloneDefaultPrefix(test *testing.T) {
	environment := isolatedEnvironment(test)
	installation := standaloneInstallationIn(test, environment)
	chats := writtenFile(test, installation.LauncherExecutable())
	key := writtenFile(test, filepath.Join(environment.dataDirectory, "key.pem"))
	icon := writtenFile(test, filepath.Join(environment.dataDirectory, "line.png"))
	hookLog := writtenFile(test, filepath.Join(environment.stateDirectory, "hooks.log"))
	staged := writtenFile(test, filepath.Join(environment.stagingDirectory(), "Proton-LINE", "version"))
	if err := newAdapters(environment, installation, slog.New(slog.DiscardHandler)).uninstaller().Uninstall(false); err != nil {
		test.Fatal(err)
	}
	if _, err := os.Stat(chats); err != nil {
		test.Errorf("LINE inside the default prefix was deleted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(environment.stateDirectory, "lock")); err != nil {
		test.Errorf("the lock file other runs may hold was deleted: %v", err)
	}
	for _, removed := range []string{key, icon, hookLog, staged} {
		test.Run(filepath.Base(removed), func(test *testing.T) {
			if _, err := os.Stat(removed); !os.IsNotExist(err) {
				test.Errorf("%s survived uninstall: %v", removed, err)
			}
		})
	}
}

func TestPurgeRefusesADirectoryWithoutLine(test *testing.T) {
	environment := isolatedEnvironment(test)
	installation, err := standaloneInstallationIn(test, environment).WithPrefix(environment.home)
	if err != nil {
		test.Fatal(err)
	}
	keep := writtenFile(test, filepath.Join(environment.home, "Documents", "thesis.odt"))
	if err := newAdapters(environment, installation, slog.New(slog.DiscardHandler)).uninstaller().Uninstall(true); err == nil {
		test.Error("purging the home directory succeeded")
	}
	if _, err := os.Stat(keep); err != nil {
		test.Errorf("home content was deleted: %v", err)
	}
}

func TestUninstallWaitsForTheGlobalLock(test *testing.T) {
	environment := isolatedEnvironment(test)
	installation := standaloneInstallationIn(test, environment)
	hookLog := writtenFile(test, filepath.Join(environment.stateDirectory, "hooks.log"))
	built := newAdapters(environment, installation, slog.New(slog.DiscardHandler))
	release, err := built.lock().Acquire()
	if err != nil {
		test.Fatal(err)
	}
	finished := make(chan error)
	go func() { finished <- built.uninstaller().Uninstall(false) }()
	if !blockedOnTheLock(test, filepath.Join(environment.stateDirectory, lockFileName)) {
		test.Error("the waiter never blocked on the global lock")
	}
	if _, err := os.Stat(hookLog); err != nil {
		test.Errorf("uninstall removed state while another run held the lock: %v", err)
	}
	if err := release(); err != nil {
		test.Fatal(err)
	}
	if err := <-finished; err != nil {
		test.Fatal(err)
	}
	if _, err := os.Stat(hookLog); !os.IsNotExist(err) {
		test.Errorf("hooks.log survived uninstall: %v", err)
	}
}

func TestSigningAdaptersCreateTheCertificateUnderTheGlobalLock(test *testing.T) {
	environment := isolatedEnvironment(test)
	writtenFile(test, filepath.Join(environment.home, ".local", "share", "faugus-launcher", "umu-run"))
	installation := standaloneInstallationIn(test, environment)
	release, err := globalLock(environment).Acquire()
	if err != nil {
		test.Fatal(err)
	}
	finished := make(chan error)
	go func() {
		_, err := lockedSigningAdapters(environment, installation, slog.New(slog.DiscardHandler))
		finished <- err
	}()
	if !blockedOnTheLock(test, filepath.Join(environment.stateDirectory, lockFileName)) {
		test.Error("the waiter never blocked on the global lock")
	}
	if _, err := os.Stat(filepath.Join(environment.dataDirectory, "key.pem")); !os.IsNotExist(err) {
		test.Errorf("the certificate was created while another run held the lock: %v", err)
	}
	if err := release(); err != nil {
		test.Fatal(err)
	}
	if err := <-finished; err != nil {
		test.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(environment.dataDirectory, "key.pem")); err != nil {
		test.Errorf("no certificate after the lock was released: %v", err)
	}
}

var errWaitedForTheLock = errors.New("waited for the global lock")

func TestSigningAdaptersLoadAnExistingCertificateWithoutTheLock(test *testing.T) {
	environment := isolatedEnvironment(test)
	writtenFile(test, filepath.Join(environment.home, ".local", "share", "faugus-launcher", "umu-run"))
	installation := standaloneInstallationIn(test, environment)
	if _, err := certificateStoreFor(environment).LoadOrCreate(signing.ImpersonatedWindowsIdentity()); err != nil {
		test.Fatal(err)
	}
	release, err := globalLock(environment).Acquire()
	if err != nil {
		test.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() {
		_, err := signingAdaptersFor(environment, installation, slog.New(slog.DiscardHandler))
		finished <- err
	}()
	select {
	case err = <-finished:
	case <-time.After(10 * time.Second):
		err = errWaitedForTheLock
	}
	if err != nil {
		test.Errorf("signing adapters = %v while another run held the lock", err)
	}
	if err := release(); err != nil {
		test.Fatal(err)
	}
}

func blockedOnTheLock(test *testing.T, lockPath string) bool {
	test.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !hasBlockedWaiter(test, lockPath) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	return hasBlockedWaiter(test, lockPath)
}

func hasBlockedWaiter(test *testing.T, lockPath string) bool {
	test.Helper()
	details, err := os.Stat(lockPath)
	if err != nil {
		test.Fatal(err)
	}
	locks, err := os.ReadFile("/proc/locks")
	if err != nil {
		test.Fatal(err)
	}
	inode := ":" + strconv.FormatUint(details.Sys().(*syscall.Stat_t).Ino, 10) + " "
	return slices.ContainsFunc(strings.Split(string(locks), "\n"), func(line string) bool {
		return strings.Contains(line, "-> FLOCK") && strings.Contains(line, inode)
	})
}
