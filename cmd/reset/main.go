// Command reset generates Reset() methods for structs that are marked
// with the "// generate:reset" comment. For every package under the
// scan root it finds such structs and writes all of their Reset()
// methods into a reset.gen.go file inside the same package directory,
// so the generated methods stay private to their own packages.
//
// The command runs to a fixpoint: a struct may reference another
// marked struct (also across packages) whose Reset method only comes
// into existence during generation, so packages are reloaded until no
// generated file changes anymore. Regeneration is idempotent.
//
// Usage:
//
//	go run ./cmd/reset [root]
//
// The optional root argument defaults to the current directory.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

const (
	directive = "generate:reset"
	genFile   = "reset.gen.go"
	maxPasses = 5
)

// resetIface is the interface{ Reset() } type used to decide whether a
// field value implements Reset on its own.
var resetIface = func() *types.Interface {
	reset := types.NewFunc(token.NoPos, nil, "Reset", types.NewSignatureType(nil, nil, nil, types.NewTuple(), types.NewTuple(), false))
	return types.NewInterfaceType([]*types.Func{reset}, nil).Complete()
}()

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "reset:", err)
		os.Exit(1)
	}
}

func run() error {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	for pass := 1; pass <= maxPasses; pass++ {
		pkgs, err := packages.Load(&packages.Config{
			Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
				packages.NeedTypes | packages.NeedTypesInfo,
			Dir: root,
		}, "./...")
		if err != nil {
			return err
		}

		changed := 0
		for _, pkg := range pkgs {
			if isResetTool(pkg) {
				continue
			}
			if len(pkg.Errors) > 0 {
				fmt.Fprintf(os.Stderr, "reset: skip %s: %v\n", pkg.PkgPath, pkg.Errors[0])
				continue
			}
			did, err := generatePackage(pkg)
			if err != nil {
				return fmt.Errorf("%s: %w", pkg.PkgPath, err)
			}
			if did {
				changed++
			}
		}

		if changed == 0 {
			break
		}
		if pass == maxPasses {
			return fmt.Errorf("generation did not stabilize after %d passes", maxPasses)
		}
	}
	return nil
}

// structTarget is one marked struct that has to get a Reset method.
type structTarget struct {
	name     string
	decl     *ast.StructType
	receiver string
}

// isResetTool reports whether pkg is the reset command itself. The
// generator never rewrites its own sources.
func isResetTool(pkg *packages.Package) bool {
	if pkg.Name != "main" || len(pkg.GoFiles) == 0 {
		return false
	}
	dir := filepath.Dir(pkg.GoFiles[0])
	return filepath.Base(dir) == "reset" && filepath.Base(filepath.Dir(dir)) == "cmd"
}

// emitter carries the per-package context of a generation pass.
type emitter struct {
	pkg      *types.Package
	info     *types.Info
	marked   map[string]bool // marked struct names of this package
	importer *importer
}

// collectTargets scans the package AST for struct types carrying the
// generate:reset doc comment. Declarations in generated files are
// ignored, and structs that already declare a Reset method in real
// sources are left untouched to avoid duplicate method declarations.
// Generic structs are not supported.
func collectTargets(pkg *packages.Package) []structTarget {
	hasReset := map[string]bool{}
	var targets []structTarget

	for _, f := range pkg.Syntax {
		if filepath.Base(pkg.Fset.Position(f.Pos()).Filename) == genFile {
			// Declarations inside the generated file never drive
			// generation itself.
			continue
		}

		ast.Inspect(f, func(n ast.Node) bool {
			switch d := n.(type) {
			case *ast.FuncDecl:
				if d.Recv != nil && d.Name.Name == "Reset" {
					if name, ok := receiverTypeName(d.Recv); ok {
						hasReset[name] = true
					}
				}
			case *ast.GenDecl:
				if d.Tok != token.TYPE {
					return true
				}
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok || ts.TypeParams != nil {
						continue
					}
					st, ok := ts.Type.(*ast.StructType)
					if !ok || hasReset[ts.Name.Name] {
						continue
					}
					doc := ts.Doc
					if doc == nil && len(d.Specs) == 1 {
						doc = d.Doc
					}
					if !hasDirective(doc) {
						continue
					}
					targets = append(targets, structTarget{
						name:     ts.Name.Name,
						decl:     st,
						receiver: receiverName(ts.Name.Name, st),
					})
				}
			}
			return true
		})
	}

	sort.Slice(targets, func(i, j int) bool { return targets[i].name < targets[j].name })
	return targets
}

// generatePackage builds the reset.gen.go file for pkg. It reports
// whether the written file differs from what was on disk before, which
// drives the fixpoint loop.
func generatePackage(pkg *packages.Package) (bool, error) {
	targets := collectTargets(pkg)
	if len(targets) == 0 || len(pkg.GoFiles) == 0 {
		return false, nil
	}
	if pkg.Types == nil || pkg.TypesInfo == nil {
		fmt.Fprintf(os.Stderr, "reset: skip %s: no type information\n", pkg.PkgPath)
		return false, nil
	}

	marked := make(map[string]bool, len(targets))
	for _, t := range targets {
		marked[t.name] = true
	}

	e := &emitter{
		pkg:      pkg.Types,
		info:     pkg.TypesInfo,
		marked:   marked,
		importer: newImporter(pkg.Types),
	}

	var funcs strings.Builder
	for _, t := range targets {
		stmts, warns := e.statements(t)
		for _, w := range warns {
			fmt.Fprintf(os.Stderr, "reset: %s: %s: %s\n", pkg.PkgPath, t.name, w)
		}
		fmt.Fprintf(&funcs, "func (%s *%s) Reset() {\n\tif %s == nil {\n\t\treturn\n\t}\n", t.receiver, t.name, t.receiver)
		for _, s := range stmts {
			fmt.Fprintf(&funcs, "\t%s\n", s)
		}
		funcs.WriteString("}\n\n")
	}

	var file strings.Builder
	file.WriteString("// Code generated by cmd/reset; DO NOT EDIT.\n\n")
	fmt.Fprintf(&file, "package %s\n\n", pkg.Name)
	if block := e.importer.importBlock(); block != "" {
		file.WriteString(block)
		file.WriteString("\n")
	}
	file.WriteString(funcs.String())

	src, err := format.Source([]byte(file.String()))
	if err != nil {
		return false, fmt.Errorf("format: %w", err)
	}
	path := filepath.Join(filepath.Dir(pkg.GoFiles[0]), genFile)
	if prev, err := os.ReadFile(path); err == nil && bytes.Equal(prev, src) {
		return false, nil
	}
	if err := os.WriteFile(path, src, 0o644); err != nil {
		return false, err
	}
	fmt.Printf("reset: %s -> %s\n", pkg.PkgPath, path)
	return true, nil
}

// statements produces the resetting statements for every field of the
// marked struct, plus warnings for fields the generator left alone
// (channels, functions, interfaces, arrays and other unsupported
// shapes). Statement order follows the field declaration order.
func (e *emitter) statements(t structTarget) ([]string, []string) {
	stmts := make([]string, 0, 8)
	var warns []string

	for _, f := range t.decl.Fields.List {
		for _, name := range fieldNames(f) {
			ft := e.info.TypeOf(f.Type)
			if ft == nil {
				warns = append(warns, fmt.Sprintf("field %q: unresolved type", name))
				continue
			}
			generated, ok := e.field(t.receiver+"."+name, ft)
			if !ok {
				warns = append(warns, fmt.Sprintf("field %q (%s): unsupported type, left as is", name, ft))
				continue
			}
			stmts = append(stmts, generated...)
		}
	}
	return stmts, warns
}

// field emits the statements that reset a single value of type t named
// target. Pointers are unwrapped under a nil check, nested pointer
// levels are handled recursively, and the rules are:
//
//   - slices are truncated to [:0] without being nilled;
//   - maps are cleaned with the built-in clear;
//   - primitives (also named ones) are assigned their zero literal;
//   - values or pointees with a Reset method get it called;
//   - remaining structs get a zero literal (or an explicit zero var
//     for types that have no readable literal form).
func (e *emitter) field(target string, t types.Type) ([]string, bool) {
	if t == nil {
		return nil, false
	}

	switch tt := t.Underlying().(type) {
	case *types.Slice:
		return []string{fmt.Sprintf("%s = %s[:0]", target, target)}, true

	case *types.Map:
		return []string{fmt.Sprintf("clear(%s)", target)}, true

	case *types.Pointer:
		inner, ok := e.field("*"+target, tt.Elem())
		if !ok {
			return nil, false
		}
		indented := make([]string, 0, len(inner)+2)
		indented = append(indented, fmt.Sprintf("if %s != nil {", target))
		for _, line := range inner {
			indented = append(indented, "\t"+line)
		}
		return append(indented, "}"), true

	case *types.Basic:
		switch tt.Kind() {
		case types.Bool:
			return []string{target + " = false"}, true
		case types.String:
			return []string{target + ` = ""`}, true
		case types.UnsafePointer:
			return nil, false
		default:
			return []string{target + " = 0"}, true
		}

	case *types.Struct:
		if e.implementsReset(t) {
			return []string{methodCall(target)}, true
		}
		if named, ok := t.(*types.Named); ok {
			return []string{fmt.Sprintf("%s = %s{}", target, e.importer.typeExpr(named))}, true
		}
		// Anonymous struct with no name to build a literal from: zero
		// it through an explicit zero value.
		v := e.importer.zeroVar()
		expr := e.importer.typeExpr(t)
		return []string{fmt.Sprintf("var %s %s", v, expr), target + " = " + v}, true
	}
	return nil, false
}

// implementsReset reports whether t or its pointer form has a Reset
// method. Types that are being marked this very run count too: their
// Reset method is being generated in the same pass.
func (e *emitter) implementsReset(t types.Type) bool {
	if t == nil {
		return false
	}
	if types.Implements(t, resetIface) || types.Implements(types.NewPointer(t), resetIface) {
		return true
	}
	named, ok := t.(*types.Named)
	return ok && named.Obj().Pkg() == e.pkg && e.marked[named.Obj().Name()]
}

// methodCall renders target.Reset(), parenthesizing dereferenced
// targets so "*x.Reset()" never appears in the output.
func methodCall(target string) string {
	if strings.HasPrefix(target, "*") {
		return fmt.Sprintf("(*%s).Reset()", strings.TrimPrefix(target, "*"))
	}
	return target + ".Reset()"
}

// importer tracks the external packages referenced by generated code
// and turns them into an import block, aliasing on name conflicts.
type importer struct {
	local  *types.Package
	byPath map[string]string    // path -> alias
	byName map[string]*imported // alias -> entry
	seq    int
}

type imported struct {
	alias string
	pkg   string
	path  string
}

func newImporter(local *types.Package) *importer {
	return &importer{
		local:  local,
		byPath: map[string]string{},
		byName: map[string]*imported{},
	}
}

func (im *importer) qualifier(p *types.Package) string {
	if p == nil {
		return ""
	}
	if p == im.local {
		return ""
	}
	if alias, ok := im.byPath[p.Path()]; ok {
		return alias
	}
	alias := p.Name()
	for _, taken := im.byName[alias]; taken; {
		im.seq++
		alias = fmt.Sprintf("%s%d", p.Name(), im.seq)
	}
	entry := &imported{alias: alias, pkg: p.Name(), path: p.Path()}
	im.byPath[p.Path()] = alias
	im.byName[alias] = entry
	return alias
}

func (im *importer) typeExpr(t types.Type) string {
	return types.TypeString(t, im.qualifier)
}

func (im *importer) zeroVar() string {
	im.seq++
	return fmt.Sprintf("z%d", im.seq)
}

func (im *importer) importBlock() string {
	if len(im.byName) == 0 {
		return ""
	}
	entries := make([]*imported, 0, len(im.byName))
	for _, e := range im.byName {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })

	var b strings.Builder
	b.WriteString("import (\n")
	for _, e := range entries {
		if e.alias == e.pkg {
			fmt.Fprintf(&b, "\t%q\n", e.path)
		} else {
			fmt.Fprintf(&b, "\t%s %q\n", e.alias, e.path)
		}
	}
	b.WriteString(")\n")
	return b.String()
}

// receiverName derives an unused receiver identifier from the type
// name, growing the lowercased prefix until it matches no field name.
func receiverName(typeName string, st *ast.StructType) string {
	used := map[string]bool{}
	for _, f := range st.Fields.List {
		for _, name := range fieldNames(f) {
			used[name] = true
		}
	}
	cand := prefix(typeName, 1)
	if used[cand] {
		cand = prefix(typeName, 2)
	}
	for used[cand] {
		cand += "i"
	}
	return cand
}

// prefix lowercases the first n runes of the type name.
func prefix(s string, n int) string {
	r := []rune(s)
	if n > len(r) {
		n = len(r)
	}
	return strings.ToLower(string(r[:n]))
}

// fieldNames lists the identifiers under which a field can be reached,
// unwrapping embedded fields to their type name.
func fieldNames(f *ast.Field) []string {
	if len(f.Names) > 0 {
		names := make([]string, 0, len(f.Names))
		for _, n := range f.Names {
			names = append(names, n.Name)
		}
		return names
	}
	if e := embeddedName(f.Type); e != "" {
		return []string{e}
	}
	return nil
}

func embeddedName(e ast.Expr) string {
	for {
		switch t := e.(type) {
		case *ast.StarExpr:
			e = t.X
		case *ast.ParenExpr:
			e = t.X
		case *ast.Ident:
			return t.Name
		case *ast.SelectorExpr:
			return t.Sel.Name
		default:
			return ""
		}
	}
}

// receiverTypeName extracts the type name from a method receiver list.
func receiverTypeName(recv *ast.FieldList) (string, bool) {
	if recv == nil || len(recv.List) == 0 {
		return "", false
	}
	switch r := recv.List[0].Type.(type) {
	case *ast.StarExpr:
		if id, ok := r.X.(*ast.Ident); ok {
			return id.Name, true
		}
	case *ast.Ident:
		return r.Name, true
	}
	return "", false
}

// hasDirective checks whether the doc comment marks the type for
// Reset generation.
func hasDirective(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}
	for _, c := range doc.List {
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(c.Text, "//"), "/"))
		if text == directive {
			return true
		}
	}
	return false
}
