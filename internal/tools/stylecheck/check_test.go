package main

import (
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func rulesBrokenBy(test *testing.T, source string) []string {
	test.Helper()
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "sample.go", source, parser.ParseComments)
	if err != nil {
		test.Fatalf("parse: %v", err)
	}
	rules := []string{}
	for _, violation := range Check(fileSet, file) {
		rules = append(rules, violation.Rule())
	}
	return rules
}

func TestCheck(test *testing.T) {
	examples := []struct {
		name   string
		source string
		want   []string
	}{
		{"clean code passes", "package sample\n\nfunc total(values []int) int {\n\tfor _, value := range values {\n\t\tuse(value)\n\t}\n\treturn 0\n}\n", []string{}},
		{"else is reported", "package sample\n\nfunc pick(flag bool) int {\n\tif flag {\n\t\treturn 1\n\t} else {\n\t\treturn 2\n\t}\n}\n", []string{ruleElse}},
		{"continue is reported", "package sample\n\nfunc skip(values []int) int {\n\tfor range values {\n\t\tcontinue\n\t}\n\treturn 0\n}\n", []string{ruleContinue}},
		{"comments are reported", "package sample\n\n// explain does things\nfunc explain() int { return 0 }\n", []string{ruleComment}},
		{"go directives are allowed", "//go:build linux\n\npackage sample\n", []string{}},
		{"if inside for is reported", "package sample\n\nfunc nested(values []int) int {\n\tfor _, value := range values {\n\t\tif value > 0 {\n\t\t\tuse(value)\n\t\t}\n\t}\n\treturn 0\n}\n", []string{ruleNesting}},
		{"function literal starts a new function", "package sample\n\nfunc spawn(values []int) int {\n\tfor _, value := range values {\n\t\tgo func() {\n\t\t\tif value > 0 {\n\t\t\t\tuse(value)\n\t\t\t}\n\t\t}()\n\t}\n\treturn 0\n}\n", []string{}},
		{"short names are reported", "package sample\n\nfunc first(x []int) int {\n\treturn x[0]\n}\n", []string{ruleShortName}},
		{"abbreviations are reported", "package sample\n\nfunc open() error {\n\tmsg := do()\n\treturn msg\n}\n", []string{ruleShortName}},
		{"void functions are reported", "package sample\n\nfunc announce(message string) {\n\tprint(message)\n}\n", []string{ruleVoid}},
		{"tests may be void", "package sample\n\nfunc TestSomething(test *T) {\n\tuse(test)\n}\n", []string{}},
	}
	for _, example := range examples {
		test.Run(example.name, func(test *testing.T) {
			got := rulesBrokenBy(test, example.source)
			if !slices.Equal(got, example.want) {
				test.Errorf("rules = %q, want %q", got, example.want)
			}
		})
	}
}

func TestRunFailsOnViolationsInTree(test *testing.T) {
	root := test.TempDir()
	source := "package sample\n\nfunc pick(flag bool) int {\n\tif flag {\n\t\treturn 1\n\t} else {\n\t\treturn 2\n\t}\n}\n"
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte(source), 0o644); err != nil {
		test.Fatal(err)
	}
	if exitCode := run([]string{root}, slog.New(slog.DiscardHandler)); exitCode != 1 {
		test.Errorf("exit code = %d, want 1", exitCode)
	}
}
