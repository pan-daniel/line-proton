package umu

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

var ErrNotFound = errors.New("umu-run not found; install Faugus Launcher or umu-launcher")

type Command struct {
	program string
	leading []string
}

func Locate(home string, lookPath func(string) (string, error)) (Command, error) {
	bundled := filepath.Join(home, ".local", "share", "faugus-launcher", "umu-run")
	if isFile(bundled) {
		return Command{program: "python3", leading: []string{bundled}}, nil
	}
	system, err := lookPath("umu-run")
	if err != nil {
		return Command{}, ErrNotFound
	}
	return Command{program: system}, nil
}

type Locator struct {
	home     string
	lookPath func(string) (string, error)
}

func NewLocator(home string, lookPath func(string) (string, error)) Locator {
	return Locator{home: home, lookPath: lookPath}
}

func (locator Locator) Locate() (Command, error) {
	return Locate(locator.home, locator.lookPath)
}

func (locator Locator) Describe() (string, error) {
	command, err := locator.Locate()
	return command.String(), err
}

func (command Command) String() string {
	return strings.Join(append([]string{command.program}, command.leading...), " ")
}

func isFile(path string) bool {
	details, err := os.Stat(path)
	return err == nil && details.Mode().IsRegular()
}

type Proton struct {
	command Command
	prefix  string
	runner  string
	output  io.Writer
}

func NewProton(command Command, prefix, runnerDirectory string, output io.Writer) Proton {
	return Proton{command: command, prefix: prefix, runner: runnerDirectory, output: output}
}

func (proton Proton) Run(ctx context.Context, executable string, arguments ...string) error {
	invocation := exec.CommandContext(ctx, proton.command.program, slices.Concat(proton.command.leading, []string{executable}, arguments)...)
	invocation.Env = append(os.Environ(), proton.environment()...)
	invocation.Stdout = proton.output
	invocation.Stderr = proton.output
	return invocation.Run()
}

func (proton Proton) environment() []string {
	return []string{
		"WINEPREFIX=" + proton.prefix,
		"GAMEID=umu-default",
		"PROTONPATH=" + proton.runner,
		"PROTON_ENABLE_WAYLAND=0",
		"PROTON_FSR4_UPGRADE=0",
	}
}
