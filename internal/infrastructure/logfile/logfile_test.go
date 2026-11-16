package logfile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pan-daniel/line-proton/internal/infrastructure/logfile"
)

func appendLine(test *testing.T, path string, limit int64, line string) string {
	test.Helper()
	file, err := logfile.Open(path, limit)
	if err != nil {
		test.Fatal(err)
	}
	if _, err := file.WriteString(line); err != nil {
		test.Fatal(err)
	}
	if err := file.Close(); err != nil {
		test.Fatal(err)
	}
	return path
}

func contentOf(test *testing.T, path string) string {
	test.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		test.Fatal(err)
	}
	return string(content)
}

func TestOpenAppendsWhileTheLogIsSmall(test *testing.T) {
	path := filepath.Join(test.TempDir(), "state", "hooks.log")
	appendLine(test, path, 1024, "first\n")
	appendLine(test, path, 1024, "second\n")
	if got := contentOf(test, path); got != "first\nsecond\n" {
		test.Errorf("hooks.log = %q", got)
	}
	if _, err := os.Stat(path + ".1"); !os.IsNotExist(err) {
		test.Errorf("rotated too early: %v", err)
	}
}

func TestOpenRotatesALogOverTheLimitReplacingTheOlderOne(test *testing.T) {
	path := filepath.Join(test.TempDir(), "hooks.log")
	if err := os.WriteFile(path+".1", []byte("ancient\n"), 0o600); err != nil {
		test.Fatal(err)
	}
	appendLine(test, path, 8, strings.Repeat("x", 9))
	appendLine(test, path, 8, "fresh\n")
	if got := contentOf(test, path+".1"); got != strings.Repeat("x", 9) {
		test.Errorf("hooks.log.1 = %q", got)
	}
	if got := contentOf(test, path); got != "fresh\n" {
		test.Errorf("hooks.log = %q", got)
	}
}
