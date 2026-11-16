package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
)

const noticeFile = "THIRD_PARTY_LICENSES"

var errOutdated = errors.New(noticeFile + " is out of date, run: go run ./internal/tools/licensecheck write")

type workspace struct {
	listModules func(context.Context) (string, error)
	openModule  moduleOpener
	readNotice  func() ([]byte, error)
	writeNotice func(content []byte) error
}

type command func(context.Context, workspace) error

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{ReplaceAttr: withoutTime}))
	os.Exit(run(context.Background(), os.Args[1:], currentDirectoryWorkspace(), logger))
}

func currentDirectoryWorkspace() workspace {
	return workspace{
		listModules: goListModules("."),
		openModule:  os.DirFS,
		readNotice:  func() ([]byte, error) { return os.ReadFile(noticeFile) },
		writeNotice: func(content []byte) error { return os.WriteFile(noticeFile, content, 0o644) },
	}
}

func withoutTime(_ []string, attribute slog.Attr) slog.Attr {
	if attribute.Key == slog.TimeKey {
		return slog.Attr{}
	}
	return attribute
}

func run(ctx context.Context, arguments []string, space workspace, logger *slog.Logger) int {
	selected, found := commandNamed(firstArgument(arguments))
	if !found {
		logger.Error("usage: licensecheck write|check")
		return 2
	}
	if err := selected(ctx, space); err != nil {
		logger.Error("licensecheck failed", "error", err)
		return 1
	}
	return 0
}

func firstArgument(arguments []string) string {
	if len(arguments) == 0 {
		return ""
	}
	return arguments[0]
}

func commandNamed(name string) (command, bool) {
	selected, found := map[string]command{"write": write, "check": check}[name]
	return selected, found
}

func write(ctx context.Context, space workspace) error {
	notice, err := generate(ctx, space)
	if err != nil {
		return err
	}
	return space.writeNotice([]byte(notice))
}

func check(ctx context.Context, space workspace) error {
	generated, err := generate(ctx, space)
	if err != nil {
		return err
	}
	committed, err := space.readNotice()
	if err != nil {
		return fmt.Errorf("read %s: %w", noticeFile, err)
	}
	return compare(string(committed), generated)
}

func compare(committed, generated string) error {
	if committed != generated {
		return errOutdated
	}
	return nil
}

func generate(ctx context.Context, space workspace) (string, error) {
	listing, err := space.listModules(ctx)
	if err != nil {
		return "", err
	}
	modules, err := parseModules(listing)
	if err != nil {
		return "", err
	}
	return render(modules, space.openModule)
}
