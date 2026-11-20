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
// name ok is the first of ok, ok2, ... that the function does not use.
//
// A test inside the && / || / ! structure of an if condition or of a returned value is written
// before the condition when its operand is a local variable:
//
//	if a && is_[*X](v) {             ->  if _, ok := v.(*X); a && ok {
//	return a || !is_[*X](v)          ->  _, ok := v.(*X)
//	                                     return a || !ok
//
// The assertion then runs before the operands that came before the test, and also when they
// decide the condition. That is the same program: reading a local variable and asserting its
// type has no effect and cannot fail, and nothing the condition evaluates can change the
// variable (the step requires that no function literal assigns it or takes its address).
// Other tests inside expressions (an operand that is a call or a field read, which may fail or
// change; an argument; a for condition) are left (Go has no comma-ok expression; the one-line
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
	pending := map[ast.Stmt][]hoist{}
	var pendingOrder []ast.Stmt
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
			// Inside the && / || / ! structure of an if condition or of a returned value, with a
			// local variable as the operand: the assertion is written before the condition.
			if why := hoistable(c, test, assert); why != "" {
				left[why]++
				return true
			}
			var topExpr ast.Expr = test
			for {
				switch p := rw.Parent[topExpr].(type) {
				case *ast.ParenExpr:
					topExpr = p
					continue
				case *ast.BinaryExpr:
					topExpr = p
					continue
				case *ast.UnaryExpr:
					topExpr = p
					continue
				}
				break
			}
			var st ast.Stmt
			switch s := rw.Parent[topExpr].(type) {
			case *ast.IfStmt:
				st = s
			case *ast.ReturnStmt:
				st = s
			}
			pending[st] = append(pending[st], hoist{c, okName(c), at})
			pendingOrder = append(pendingOrder, st)
			return true
		})
	}
	seenSt := map[ast.Stmt]bool{}
	for _, st := range pendingOrder {
		if seenSt[st] {
			continue
		}
		seenSt[st] = true
		hs := pending[st]
		ifs, isIf := st.(*ast.IfStmt)
		if isIf && len(hs) == 1 {
			h := hs[0]
			done["in && / || of an if condition, a local variable tested: if _, ok := x.(T); a && ok"]++
			rw.Rewrite[h.call] = func() string { return h.name }
			cond := ifs.Cond
			rw.Rewrite[cond] = func() string { return "_, " + h.name + " := " + h.at() + "; " + rw.RenderChildren(cond) }
			continue
		}
		if !inList(st) {
			left["two tests in the condition of an else-if (one statement fits before the condition)"] += len(hs)
			continue
		}
		done["in && / || of a condition or a returned value, a local variable tested: _, ok := x.(T) before the statement"] += len(hs)
		for _, h := range hs {
			h := h
			rw.Rewrite[h.call] = func() string { return h.name }
		}
		st := st
		rw.Rewrite[st] = func() string {
			f := rw.Fset.File(st.Pos())
			src := rw.Src[f]
			o := f.Offset(st.Pos())
			i := o
			for i > 0 && src[i-1] != '\n' {
				i--
			}
			ind := string(src[i:o])
			out := ""
			for _, h := range hs {
				out += "_, " + h.name + " := " + h.at() + "\n" + ind
			}
			return out + rw.RenderChildren(st)
		}
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

type hoist struct {
	call *ast.CallExpr
	name string
	at   func() string
}

// hoistable says why the test (c, or !c) cannot be written before its statement, or "".
// It can when the operand is a local variable that no function literal assigns (so nothing
// the condition evaluates before the test can change it: reading it earlier reads the same
// value, and reading a variable has no effect), and the test is reached from an if statement's
// condition (without an init statement) or a return's only value through && / || / ! and
// parentheses alone.
func hoistable(c *ast.CallExpr, test ast.Expr, operand ast.Expr) string {
	const other = "inside an expression (an argument, a for condition, an operand that is not a local variable)"
	id, ok := operand.(*ast.Ident)
	if !ok {
		return other
	}
	v, ok := rw.Info.Uses[id].(*types.Var)
	if !ok || v.IsField() || v.Parent() == rw.Pkg.Scope() || v.Parent() == nil {
		return other
	}
	for _, u := range rw.UsesOf[v] {
		inLit := false
		for p := rw.Parent[u]; p != nil; p = rw.Parent[p] {
			if _, ok := p.(*ast.FuncLit); ok {
				inLit = true
				break
			}
			if _, ok := p.(*ast.FuncDecl); ok {
				break
			}
		}
		if !inLit {
			continue
		}
		// a use in a function literal: an assignment or an address there could change the
		// variable while the condition runs; so could a literal that the variable belongs to
		if v.Pos() < u.Pos() {
			declInLit := false
			for p := rw.Parent[u]; p != nil; p = rw.Parent[p] {
				if l, ok := p.(*ast.FuncLit); ok && l.Pos() <= v.Pos() && v.Pos() < l.End() {
					declInLit = true
				}
			}
			if declInLit {
				continue
			}
		}
		switch up := rw.Parent[u].(type) {
		case *ast.AssignStmt:
			for _, l := range up.Lhs {
				if l == ast.Expr(u) {
					return other
				}
			}
		case *ast.UnaryExpr:
			if up.Op == token.AND {
				return other
			}
		case *ast.IncDecStmt:
			return other
		}
	}
	var top ast.Expr = test
	for {
		switch p := rw.Parent[top].(type) {
		case *ast.ParenExpr:
			top = p
			continue
		case *ast.BinaryExpr:
			if p.Op == token.LAND || p.Op == token.LOR {
				top = p
				continue
			}
			return other
		case *ast.UnaryExpr:
			if p.Op == token.NOT {
				top = p
				continue
			}
			return other
		case *ast.IfStmt:
			if p.Cond == top && p.Init == nil {
				return ""
			}
		case *ast.ReturnStmt:
			if len(p.Results) == 1 {
				return ""
			}
		}
		return other
	}
}

func inList(s ast.Stmt) bool {
	switch rw.Parent[s].(type) {
	case *ast.BlockStmt, *ast.CaseClause, *ast.CommClause:
		return true
	}
	return false
}
