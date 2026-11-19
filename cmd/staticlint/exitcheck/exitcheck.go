// Package exitcheck defines an analyzer that forbids direct os.Exit calls
// inside the main function of the main package.
//
// Terminating a program from the middle of main bypasses deferred calls
// (file and database handles, audit emitters, background goroutines) and
// makes the control flow hard to follow. The idiomatic alternatives are
// returning an error from main, or a logging call such as log.Fatal that
// documents the intent of an abnormal termination.
//
// The analyzer checks only function main of package main; os.Exit calls
// in other functions, other packages, and test files are out of scope.
package exitcheck

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const (
	name = "exitcheck"
	doc  = "exitcheck forbids direct os.Exit calls in the main function of the main package, " +
		"because they skip deferred cleanup; return an error or use log.Fatal instead"
)

// Analyzer reports direct calls to os.Exit made from the main function
// of a main package.
var Analyzer = &analysis.Analyzer{
	Name: name,
	Doc:  doc,
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	if pass.Pkg.Name() != "main" {
		return nil, nil
	}

	for _, file := range pass.Files {
		if isGenerated(file) {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name.Name != "main" || fn.Body == nil {
				continue
			}
			inspectBody(pass, fn)
		}
	}
	return nil, nil
}

// inspectBody reports every direct os.Exit call within the body of fn,
// including calls made from nested function literals.
func inspectBody(pass *analysis.Pass, fn *ast.FuncDecl) {
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if isOSExit(pass, call.Fun) {
			pass.Reportf(call.Pos(), "direct os.Exit call in main function of main package is forbidden, "+
				"deferred functions will not run; return an error or use log.Fatal instead")
		}
		return true
	})
}

// isGenerated reports whether the file carries the "Code generated ...
// DO NOT EDIT." marker described at https://go.dev/s/generatedcode,
// for example the _testmain.go file that go test generates with an
// os.Exit call in its main function.
func isGenerated(file *ast.File) bool {
	for _, group := range file.Comments {
		if group.Pos() > file.Package {
			break
		}
		for _, comment := range group.List {
			if strings.HasPrefix(comment.Text, "// Code generated") &&
				strings.Contains(comment.Text, "DO NOT EDIT.") {
				return true
			}
		}
	}
	return false
}

// isOSExit reports whether the expression refers to the os.Exit function,
// resolved through the type information rather than by name, so that
// import aliases are handled correctly.
func isOSExit(pass *analysis.Pass, expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	pkgName, ok := pass.TypesInfo.ObjectOf(ident).(*types.PkgName)
	if !ok {
		return false
	}
	return pkgName.Imported().Path() == "os" && sel.Sel.Name == "Exit"
}
