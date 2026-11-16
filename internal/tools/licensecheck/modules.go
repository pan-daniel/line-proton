package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

const moduleTemplate = "{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}} {{.Dir}}{{end}}{{end}}"

var releaseTarget = []string{"GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0"}

type module struct {
	path      string
	version   string
	directory string
}

func (module module) heading() string {
	return strings.TrimSpace(module.path + " " + module.version)
}

func goListModules(workingDirectory string) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		command := exec.CommandContext(ctx, "go", "list", "-deps", "-f", moduleTemplate, "./cmd/line-proton")
		command.Dir = workingDirectory
		command.Env = append(os.Environ(), releaseTarget...)
		output, err := command.Output()
		return string(output), describeListFailure(err)
	}
}

func describeListFailure(err error) error {
	exitError, failed := errors.AsType[*exec.ExitError](err)
	if failed {
		return fmt.Errorf("go list: %w: %s", err, exitError.Stderr)
	}
	return err
}

func parseModules(listing string) ([]module, error) {
	modules := []module{}
	failures := []error{}
	for line := range strings.Lines(listing) {
		found, err := parseLine(strings.TrimSpace(line))
		modules = append(modules, found...)
		failures = append(failures, err)
	}
	return uniqueSorted(modules), errors.Join(failures...)
}

func parseLine(line string) ([]module, error) {
	if line == "" {
		return nil, nil
	}
	fields := strings.SplitN(line, " ", 3)
	if len(fields) != 3 {
		return nil, fmt.Errorf("unexpected go list line %q", line)
	}
	return []module{{path: fields[0], version: fields[1], directory: fields[2]}}, nil
}

func uniqueSorted(modules []module) []module {
	sorted := slices.SortedFunc(slices.Values(modules), byPath)
	return slices.CompactFunc(sorted, samePath)
}

func byPath(first, second module) int {
	return cmp.Compare(first.path, second.path)
}

func samePath(first, second module) bool {
	return first.path == second.path
}
