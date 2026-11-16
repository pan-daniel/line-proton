package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pan-daniel/line-proton/internal/domain/hooks"
	"github.com/pan-daniel/line-proton/internal/domain/install"
	"github.com/pan-daniel/line-proton/internal/infrastructure/logfile"
)

var errUnhealthy = errors.New("doctor found problems")

const hookLogLimit = 1 << 20

type command struct {
	name    string
	summary string
	perform func(ctx context.Context, arguments []string, logger *slog.Logger) error
}

func commands() []command {
	return []command{
		{name: "setup", summary: "install or repair LINE (--prefix, --proton, --font override the saved choice)", perform: runSetup},
		{name: "launch", summary: "start LINE without Faugus", perform: runLaunch},
		{name: "hook", summary: "pre|post, called by Faugus around each launch", perform: runHook},
		{name: "sign", summary: "re-sign the prefix", perform: runSign},
		{name: "runner", summary: "build: rebuild Proton-LINE if its source changed", perform: runRunner},
		{name: "doctor", summary: "check every part of the setup", perform: runDoctor},
		{name: "uninstall", summary: "remove everything except LINE itself (--purge removes LINE and its data)", perform: runUninstall},
		{name: "version", summary: "print the version", perform: runVersion},
	}
}

func runSetup(ctx context.Context, arguments []string, logger *slog.Logger) error {
	request, err := parseSetupFlags(arguments)
	if err != nil {
		return err
	}
	if !install.SupportedArchitecture(runtime.GOARCH) {
		return install.ErrUnsupportedArchitecture
	}
	environment, err := detectEnvironment()
	if err != nil {
		return err
	}
	installation, err := installationResolver(environment).Resolve(request)
	if err != nil {
		return err
	}
	release, err := globalLock(environment).Acquire()
	if err != nil {
		return err
	}
	return errors.Join(setUp(ctx, environment, installation, logger), release())
}

func parseSetupFlags(arguments []string) (install.SetupRequest, error) {
	flags := flag.NewFlagSet("setup", flag.ContinueOnError)
	prefix := flags.String("prefix", "", "Wine prefix directory")
	proton := flags.String("proton", "", "source Proton folder name in compatibilitytools.d")
	font := flags.String("font", "", "CJK font variant: jp, sc, tc or kr")
	if err := flags.Parse(arguments); err != nil {
		return install.SetupRequest{}, err
	}
	absolutePrefix, err := absolutePathOrEmpty(*prefix)
	return install.NewSetupRequest(absolutePrefix, *proton, *font), err
}

func absolutePathOrEmpty(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	return filepath.Abs(path)
}

func setUp(ctx context.Context, environment environment, installation install.Installation, logger *slog.Logger) error {
	built, err := newSigningAdapters(environment, installation, logger)
	if err != nil {
		return err
	}
	setup, err := built.setup()
	if err != nil {
		return err
	}
	return setup.Run(ctx)
}

func runLaunch(ctx context.Context, _ []string, logger *slog.Logger) error {
	return runMoment(ctx, hooks.StandaloneLaunch, logger)
}

func runSign(ctx context.Context, _ []string, logger *slog.Logger) error {
	return runMoment(ctx, hooks.ManualSign, logger)
}

func runHook(ctx context.Context, arguments []string, _ *slog.Logger) error {
	moment, err := hooks.ParseMoment(firstArgument(arguments))
	if err != nil {
		return err
	}
	environment, err := detectEnvironment()
	if err != nil {
		return err
	}
	logFile, err := openHookLog(environment)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelDebug})).With("hook", moment.String())
	return errors.Join(loggedFailure(logger, runMoment(ctx, moment, logger)), logFile.Close())
}

func loggedFailure(logger *slog.Logger, err error) error {
	if err != nil {
		logger.Error("hook failed", "error", err.Error())
	}
	return err
}

func runMoment(ctx context.Context, moment hooks.Moment, logger *slog.Logger) error {
	environment, err := detectEnvironment()
	if err != nil {
		return err
	}
	installation, err := installationResolver(environment).Configured()
	if err != nil {
		return err
	}
	built, err := signingAdaptersFor(environment, installation, logger)
	if err != nil {
		return err
	}
	return built.hookRunner().Run(ctx, moment)
}

func runRunner(ctx context.Context, arguments []string, logger *slog.Logger) error {
	if firstArgument(arguments) != "build" {
		return errors.New("usage: line-proton runner build")
	}
	return runMoment(ctx, hooks.AfterLaunch, logger)
}

func runDoctor(_ context.Context, _ []string, logger *slog.Logger) error {
	environment, err := detectEnvironment()
	if err != nil {
		return err
	}
	installation, err := installationResolver(environment).Configured()
	if err != nil {
		return err
	}
	report := newAdapters(environment, installation, logger).doctor().Examine()
	for _, line := range report.Lines() {
		logger.Info(line)
	}
	for _, line := range lastLines(filepath.Join(environment.stateDirectory, "hooks.log"), 5) {
		logger.Info("hooks.log: " + line)
	}
	if !report.Healthy() {
		return errUnhealthy
	}
	return nil
}

func runUninstall(_ context.Context, arguments []string, logger *slog.Logger) error {
	flags := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	purge := flags.Bool("purge", false, "also delete the Wine prefix with LINE and its data")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	environment, err := detectEnvironment()
	if err != nil {
		return err
	}
	installation, err := installationResolver(environment).Configured()
	if err != nil {
		return err
	}
	return newAdapters(environment, installation, logger).uninstaller().Uninstall(*purge)
}

func runVersion(_ context.Context, _ []string, logger *slog.Logger) error {
	logger.Info("line-proton " + version)
	return nil
}

func openHookLog(environment environment) (*os.File, error) {
	return logfile.Open(filepath.Join(environment.stateDirectory, "hooks.log"), hookLogLimit)
}

func lastLines(path string, count int) []string {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	return lines[max(0, len(lines)-count):]
}

func firstArgument(arguments []string) string {
	if len(arguments) == 0 {
		return ""
	}
	return arguments[0]
}
