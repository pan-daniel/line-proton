package processes

import (
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
)

type Process struct {
	arguments   []string
	environment []string
}

func (process Process) Arguments() []string {
	return slices.Clone(process.arguments)
}

func (process Process) Environment() []string {
	return slices.Clone(process.environment)
}

type Scanner struct {
	processTable fs.FS
}

func NewScanner(processTable fs.FS) Scanner {
	return Scanner{processTable: processTable}
}

func (scanner Scanner) AnyMatching(matches func(Process) bool) (bool, error) {
	entries, err := fs.ReadDir(scanner.processTable, ".")
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(entries, func(entry fs.DirEntry) bool { return scanner.processMatches(entry.Name(), matches) }), nil
}

func (scanner Scanner) processMatches(name string, matches func(Process) bool) bool {
	if _, err := strconv.Atoi(name); err != nil {
		return false
	}
	arguments, err := fs.ReadFile(scanner.processTable, path.Join(name, "cmdline"))
	if err != nil {
		return false
	}
	environment, _ := fs.ReadFile(scanner.processTable, path.Join(name, "environ"))
	return matches(Process{arguments: nullSeparated(arguments), environment: nullSeparated(environment)})
}

func nullSeparated(raw []byte) []string {
	return strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00")
}

func Mentioning(fragment string) func(Process) bool {
	return func(process Process) bool {
		return slices.ContainsFunc(process.arguments, func(argument string) bool { return strings.Contains(argument, fragment) })
	}
}

func WithVariable(assignments ...string) func(Process) bool {
	return func(process Process) bool {
		return slices.ContainsFunc(process.environment, func(variable string) bool { return slices.Contains(assignments, variable) })
	}
}

type RunnerActivity struct {
	scanner   Scanner
	directory string
}

func NewRunnerActivity(scanner Scanner, directory string) RunnerActivity {
	return RunnerActivity{scanner: scanner, directory: strings.TrimRight(directory, "/")}
}

func (activity RunnerActivity) InUse() (bool, error) {
	startingUp := WithVariable("PROTONPATH="+activity.directory, "PROTONPATH="+activity.directory+"/")
	running := Mentioning(activity.directory + "/")
	return activity.scanner.AnyMatching(func(process Process) bool { return running(process) || startingUp(process) })
}
