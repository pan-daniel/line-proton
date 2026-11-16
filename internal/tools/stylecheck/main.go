package main

import (
	"errors"
	"go/parser"
	"go/token"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:], slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{ReplaceAttr: withoutTime}))))
}

func withoutTime(_ []string, attribute slog.Attr) slog.Attr {
	if attribute.Key == slog.TimeKey {
		return slog.Attr{}
	}
	return attribute
}

func run(roots []string, logger *slog.Logger) int {
	violations, err := checkRoots(rootsOrCurrentDirectory(roots))
	if err != nil {
		logger.Error("stylecheck failed", "error", err)
		return 2
	}
	for _, violation := range violations {
		logger.Warn(violation.rule, "at", violation.position.String())
	}
	return exitCodeFor(violations)
}

func rootsOrCurrentDirectory(roots []string) []string {
	if len(roots) == 0 {
		return []string{"."}
	}
	return roots
}

func exitCodeFor(violations []Violation) int {
	if len(violations) == 0 {
		return 0
	}
	return 1
}

func checkRoots(roots []string) ([]Violation, error) {
	files, err := goFilesIn(roots)
	if err != nil {
		return nil, err
	}
	return checkFiles(files)
}

func goFilesIn(roots []string) ([]string, error) {
	files := []string{}
	failures := []error{}
	for _, root := range roots {
		found, err := goFilesUnder(root)
		files = append(files, found...)
		failures = append(failures, err)
	}
	return files, errors.Join(failures...)
}

func goFilesUnder(root string) ([]string, error) {
	files := []string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && isSkipped(entry.Name(), path, root) {
			return filepath.SkipDir
		}
		files = appendIf(files, isGoSource(entry), path)
		return nil
	})
	return files, err
}

func isSkipped(name, path, root string) bool {
	return path != root && (strings.HasPrefix(name, ".") || name == "testdata" || name == "dist" || name == "docs")
}

func isGoSource(entry fs.DirEntry) bool {
	return !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go")
}

func checkFiles(paths []string) ([]Violation, error) {
	fileSet := token.NewFileSet()
	violations := []Violation{}
	failures := []error{}
	for _, path := range paths {
		found, err := checkFile(fileSet, path)
		violations = append(violations, found...)
		failures = append(failures, err)
	}
	return violations, errors.Join(failures...)
}

func checkFile(fileSet *token.FileSet, path string) ([]Violation, error) {
	file, err := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	return Check(fileSet, file), nil
}
