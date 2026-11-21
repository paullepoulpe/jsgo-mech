// Command instof is a post-flip step: an instanceof test that is a whole statement's value is
// written as Go writes a type test, a comma-ok type assertion, instead of a call of a helper:
//
//	return is_[*X](e)              ->  _, ok := e.(*X); return ok
//	var b bool = is_[*X](e)        ->  _, b := e.(*X)
//	if is_[*X](e) { / if !is_[*X](e) {   ->  if _, ok := e.(*X); ok { / !ok {
//
// for the generic is_[T] (rt_core.go: `_, ok := v.(T); return ok`) and for the generated
// is_X helpers whose body is that assertion (is_X(v) of a class X with subclasses asserts
// X_as). The assertion is the helper's own, of the same value, so the result is the same; the
// name ok is the first of ok, ok2, ... that the function does not use. Tests inside && / || /
// for conditions or other expressions are left (Go has no comma-ok expression; the one-line
// generic is_[T], which the compiler inlines, is what Go code writes there) and counted.
//
// usage: instof [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"sort"
	"strings"

	"postflip/rw"
)

var used = map[*ast.FuncDecl]map[string]bool{}

func okName(n ast.Node) string {
	var fd *ast.FuncDecl
	for p := n; p != nil; p = rw.Parent[p] {
		if d, ok := p.(*ast.FuncDecl); ok {
			fd = d
		}
	}
	if used[fd] == nil {
		used[fd] = map[string]bool{}
		ast.Inspect(fd, func(m ast.Node) bool {
			if id, ok := m.(*ast.Ident); ok {
				used[fd][id.Name] = true
			}
			return true
		})
	}
	for k := 1; ; k++ {
		name := "ok"
		if k > 1 {
			name = fmt.Sprintf("ok%d", k)
		}
		if !used[fd][name] {
			used[fd][name] = true
			return name
		}
	}
}

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	// The asserted type of each is_X helper: func is_X(v any) bool { _, ok := v.(T); return ok }.
	helper := map[string]string{}
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || !strings.HasPrefix(fd.Name.Name, "is_") || fd.Body == nil || len(fd.Body.List) != 2 {
				continue
			}
			as, ok := fd.Body.List[0].(*ast.AssignStmt)
			if !ok || len(as.Lhs) != 2 || len(as.Rhs) != 1 || rw.Text(as.Lhs[0]) != "_" || rw.Text(fd.Body.List[1]) != "return "+rw.Text(as.Lhs[1]) {
				continue
			}
			ta, ok := as.Rhs[0].(*ast.TypeAssertExpr)
			if !ok || ta.Type == nil || fd.Type.Params.NumFields() != 1 || rw.Text(ta.X) != fd.Type.Params.List[0].Names[0].Name {
				continue
			}
			helper[fd.Name.Name] = rw.Text(ta.Type)
		}
	}
	done := map[string]int{}
	left := map[string]int{}
	for _, f := range rw.Files {
		fn := rw.Fset.Position(f.Pos()).Filename
		if !strings.HasPrefix(fn[strings.LastIndex(fn, "/")+1:], "g_") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok || len(c.Args) != 1 {
				return true
			}
			var typ string
			switch fun := c.Fun.(type) {
			case *ast.IndexExpr:
				if id, ok := fun.X.(*ast.Ident); ok && id.Name == "is_" {
					typ = rw.Text(fun.Index)
				}
			case *ast.Ident:
				if _, isFunc := rw.Info.Uses[fun].(*types.Func); isFunc {
					typ = helper[fun.Name]
				}
			}
			if typ == "" {
				return true
			}
			// A catch clause of the try emulation: the exceptions step reads those.
			if strings.Contains(rw.Text(c.Args[0]), "ex_") {
				left["a catch clause (multi-catch; the exceptions step reads is_X(ex_) there)"]++
				return true
			}
			assert := rw.Unparen(c.Args[0])
			at := func() string {
				s := rw.Render(assert)
				if _, ok := assert.(*ast.Ident); !ok {
					if _, ok := assert.(*ast.CallExpr); !ok {
						if _, ok := assert.(*ast.SelectorExpr); !ok {
							s = "(" + s + ")"
						}
					}
				}
				return s + ".(" + typ + ")"
			}
			var neg bool
			var test ast.Expr = c
			if u, ok := rw.Up(c).(*ast.UnaryExpr); ok && u.Op == token.NOT {
				neg, test = true, u
			}
			top := rw.Up(test)
			switch s := top.(type) {
			case *ast.ReturnStmt:
				if neg || len(s.Results) != 1 || !inList(s) {
					break
				}
				name := okName(c)
				done["return"]++
				rw.Rewrite[s] = func() string { return "_, " + name + " := " + at() + "\nreturn " + name }
				return false
			case *ast.ValueSpec:
				g, ok := rw.Parent[s].(*ast.GenDecl)
				if neg || !ok || len(s.Names) != 1 || len(g.Specs) != 1 {
					break
				}
				d, ok := rw.Parent[g].(*ast.DeclStmt)
				if !ok || !inList(d) {
					break
				}
				name := s.Names[0].Name
				done["declaration"]++
				rw.Rewrite[d] = func() string { return "_, " + name + " := " + at() }
				return false
			case *ast.IfStmt:
				if s.Init != nil || rw.Unparen(s.Cond) != test {
					break
				}
				name := okName(c)
				cond := name
				if neg {
					cond = "!" + name
				}
				done["if condition"]++
				rw.Rewrite[s.Cond] = func() string { return "_, " + name + " := " + at() + "; " + cond }
				return false
			}
			left["inside an expression (&&, ||, a for condition, an argument)"]++
			return true
		})
	}
	print := func(m map[string]int, what string) {
		keys := []string{}
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(os.Stderr, "  %s: %4d %s\n", what, m[k], k)
		}
	}
	print(done, "assertion")
	print(left, "left")
	if report {
		return
	}
	fmt.Fprintf(os.Stderr, "instof: files changed: %d\n", rw.SaveChecked())
}

func inList(s ast.Stmt) bool {
	switch rw.Parent[s].(type) {
	case *ast.BlockStmt, *ast.CaseClause, *ast.CommClause:
		return true
	}
	return false
}
