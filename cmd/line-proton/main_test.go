package main

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pan-daniel/line-proton/internal/infrastructure/console"
)

func consoleLogger(output *bytes.Buffer) *slog.Logger {
	return slog.New(console.NewHandler(output, slog.LevelInfo))
}

func TestUnknownCommandPrintsUsageAndExitsWithTwo(test *testing.T) {
	var output bytes.Buffer
	if exitCode := run(test.Context(), []string{"dance"}, consoleLogger(&output)); exitCode != 2 {
		test.Errorf("exit code = %d", exitCode)
	}
	if !strings.Contains(output.String(), "setup") {
		test.Errorf("usage missing commands:\n%s", output.String())
	}
}

func TestVersionCommand(test *testing.T) {
	var output bytes.Buffer
	if exitCode := run(test.Context(), []string{"version"}, consoleLogger(&output)); exitCode != 0 || !strings.Contains(output.String(), "dev") {
		test.Errorf("exit code = %d, output = %q", exitCode, output.String())
	}
}

func TestDoctorWithoutSetupFails(test *testing.T) {
	test.Setenv("HOME", test.TempDir())
	test.Setenv("XDG_CONFIG_HOME", "")
	var output bytes.Buffer
	if exitCode := run(test.Context(), []string{"doctor"}, consoleLogger(&output)); exitCode != 1 || !strings.Contains(output.String(), "line-proton setup") {
		test.Errorf("exit code = %d, output = %q", exitCode, output.String())
	}
}

func TestHookFailuresLandInTheHookLog(test *testing.T) {
	environment := isolatedEnvironment(test)
	var output bytes.Buffer
	if exitCode := run(test.Context(), []string{"hook", "post"}, consoleLogger(&output)); exitCode != 1 {
		test.Errorf("exit code = %d", exitCode)
	}
	logged, err := os.ReadFile(filepath.Join(environment.stateDirectory, "hooks.log"))
	if err != nil || !strings.Contains(string(logged), "level=ERROR") || !strings.Contains(string(logged), "line-proton setup") {
		test.Errorf("hooks.log = %q, %v", logged, err)
	}
}
