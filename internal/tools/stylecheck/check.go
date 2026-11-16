package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"
)

const (
	ruleComment   = "comments are not allowed"
	ruleElse      = "else is not allowed"
	ruleContinue  = "continue is not allowed"
	ruleNesting   = "control flow nested inside control flow is not allowed"
	ruleShortName = "names must be spelled out"
	ruleVoid      = "functions must return something"
)

var abbreviations = []string{
	"cfg", "conf", "msg", "buf", "tmp", "idx", "str", "num", "val", "vals", "arg", "args",
	"src", "dst", "pkg", "cmd", "req", "res", "resp", "ptr", "cnt", "env", "opt", "opts", "dir", "dirs",
	"repo", "impl", "mgr", "svc", "util", "elem", "obj", "params", "attr", "proc", "info",
}

var voidAllowedPrefixes = []string{"Test", "Benchmark", "Fuzz", "Example"}

type Violation struct {
	position token.Position
	rule     string
}

func (violation Violation) Rule() string {
	return violation.rule
}

func (violation Violation) String() string {
	return fmt.Sprintf("%s: %s", violation.position, violation.rule)
}

type placedNode struct {
	node  ast.Node
	depth int
}

func Check(fileSet *token.FileSet, file *ast.File) []Violation {
	violations := commentViolations(fileSet, file)
	for _, placed := range placedNodes(file) {
		violations = append(violations, violationsAt(fileSet, placed)...)
	}
	return violations
}

func placedNodes(root ast.Node) []placedNode {
	placed := []placedNode{}
	openDepths := []int{}
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			openDepths = openDepths[:len(openDepths)-1]
			return true
		}
		depth := innermostDepth(openDepths)
		placed = append(placed, placedNode{node: node, depth: depth})
		openDepths = append(openDepths, depthForChildrenOf(node, depth))
		return true
	})
	return placed
}

func innermostDepth(openDepths []int) int {
	if len(openDepths) == 0 {
		return 0
	}
	return openDepths[len(openDepths)-1]
}

func depthForChildrenOf(node ast.Node, depth int) int {
	if isFunction(node) {
		return 0
	}
	if isControlFlow(node) {
		return depth + 1
	}
	return depth
}

func violationsAt(fileSet *token.FileSet, placed placedNode) []Violation {
	positions := slices.Concat(
		elsePositions(placed.node),
		continuePositions(placed.node),
		nestingPositions(placed),
	)
	violations := []Violation{}
	for _, found := range positions {
		violations = append(violations, Violation{position: fileSet.Position(found.position), rule: found.rule})
	}
	return slices.Concat(violations, nameViolations(fileSet, placed.node), voidViolations(fileSet, placed.node))
}

type finding struct {
	position token.Pos
	rule     string
}

func elsePositions(node ast.Node) []finding {
	statement, isIf := node.(*ast.IfStmt)
	if !isIf || statement.Else == nil {
		return nil
	}
	return []finding{{position: statement.Else.Pos(), rule: ruleElse}}
}

func continuePositions(node ast.Node) []finding {
	branch, isBranch := node.(*ast.BranchStmt)
	if !isBranch || branch.Tok != token.CONTINUE {
		return nil
	}
	return []finding{{position: branch.Pos(), rule: ruleContinue}}
}

func nestingPositions(placed placedNode) []finding {
	if placed.depth == 0 || !isControlFlow(placed.node) {
		return nil
	}
	return []finding{{position: placed.node.Pos(), rule: ruleNesting}}
}

func nameViolations(fileSet *token.FileSet, node ast.Node) []Violation {
	violations := []Violation{}
	for _, identifier := range declaredIdentifiers(node) {
		violations = appendIf(violations, !isSpelledOut(identifier.Name), Violation{position: fileSet.Position(identifier.Pos()), rule: ruleShortName})
	}
	return violations
}

func declaredIdentifiers(node ast.Node) []*ast.Ident {
	switch declaration := node.(type) {
	case *ast.Field:
		return declaration.Names
	case *ast.ValueSpec:
		return declaration.Names
	case *ast.TypeSpec:
		return []*ast.Ident{declaration.Name}
	case *ast.FuncDecl:
		return []*ast.Ident{declaration.Name}
	case *ast.AssignStmt:
		return definedIdentifiers(declaration.Tok, declaration.Lhs...)
	case *ast.RangeStmt:
		return definedIdentifiers(declaration.Tok, declaration.Key, declaration.Value)
	}
	return nil
}

func definedIdentifiers(operator token.Token, expressions ...ast.Expr) []*ast.Ident {
	if operator != token.DEFINE {
		return nil
	}
	identifiers := []*ast.Ident{}
	for _, expression := range expressions {
		identifiers = append(identifiers, asIdentifier(expression)...)
	}
	return identifiers
}

func asIdentifier(expression ast.Expr) []*ast.Ident {
	identifier, isIdentifier := expression.(*ast.Ident)
	if !isIdentifier {
		return nil
	}
	return []*ast.Ident{identifier}
}

func isSpelledOut(name string) bool {
	return name == "_" || (len(name) >= 3 && !slices.Contains(abbreviations, strings.ToLower(name)))
}

func voidViolations(fileSet *token.FileSet, node ast.Node) []Violation {
	function, isFunctionDeclaration := node.(*ast.FuncDecl)
	if !isFunctionDeclaration || returnsSomething(function) || mayBeVoid(function.Name.Name) {
		return nil
	}
	return []Violation{{position: fileSet.Position(function.Pos()), rule: ruleVoid}}
}

func returnsSomething(function *ast.FuncDecl) bool {
	return function.Type.Results != nil && len(function.Type.Results.List) > 0
}

func mayBeVoid(name string) bool {
	return name == "main" || name == "init" || slices.ContainsFunc(voidAllowedPrefixes, func(prefix string) bool {
		return strings.HasPrefix(name, prefix)
	})
}

func isFunction(node ast.Node) bool {
	switch node.(type) {
	case *ast.FuncDecl, *ast.FuncLit:
		return true
	}
	return false
}

func isControlFlow(node ast.Node) bool {
	switch node.(type) {
	case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		return true
	}
	return false
}

func commentViolations(fileSet *token.FileSet, file *ast.File) []Violation {
	violations := []Violation{}
	for _, comment := range allComments(file) {
		violations = appendIf(violations, !isDirective(comment), Violation{position: fileSet.Position(comment.Pos()), rule: ruleComment})
	}
	return violations
}

func allComments(file *ast.File) []*ast.Comment {
	comments := []*ast.Comment{}
	for _, group := range file.Comments {
		comments = append(comments, group.List...)
	}
	return comments
}

func isDirective(comment *ast.Comment) bool {
	return strings.HasPrefix(comment.Text, "//go:")
}

func appendIf[Item any](items []Item, condition bool, item Item) []Item {
	if !condition {
		return items
	}
	return append(items, item)
}
