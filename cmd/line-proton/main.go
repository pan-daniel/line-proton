package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"syscall"

	"github.com/pan-daniel/line-proton/internal/infrastructure/console"
)

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	exitCode := run(ctx, os.Args[1:], slog.New(console.NewHandler(os.Stderr, slog.LevelInfo)))
	stop()
	os.Exit(exitCode)
}

func run(ctx context.Context, arguments []string, logger *slog.Logger) int {
	selected, found := commandNamed(firstArgument(arguments))
	if !found {
		return usage(logger)
	}
	if err := selected.perform(ctx, arguments[1:], logger); err != nil {
		logger.Error(err.Error())
		return 1
	}
	return 0
}

func commandNamed(name string) (command, bool) {
	available := commands()
	index := slices.IndexFunc(available, func(candidate command) bool { return candidate.name == name })
	if index < 0 {
		return command{}, false
	}
	return available[index], true
}

func usage(logger *slog.Logger) int {
	logger.Info("usage: line-proton <command>")
	for _, available := range commands() {
		logger.Info(available.name + " — " + available.summary)
	}
	return 2
}
