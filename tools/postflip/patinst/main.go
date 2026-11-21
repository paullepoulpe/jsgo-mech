// Command patinst is a post-flip step: Java's `x instanceof T v` (a type test that binds the
// value) is a comma-ok type assertion, not a function literal called on the spot.
//
// j2go writes the pattern as an expression, because Java uses it as one:
//
//	var v *T
//	_ = v
//	if func() bool {
//		v_ := x
//		p, ok := v_.(*T)
//		if !ok {
//			return false
//		}
//		v = p
//		return true
//	}() && v.f == 1 {
//
// Go writes the test and the binding as a statement. Where the pattern is the first thing its
// statement evaluates (the whole condition of an if, or the leftmost operand of its && / ||
// chain, under ! or not; the same for a return's value), the step writes
//
//	if v, ok := x.(*T); ok && v.f == 1 {            (all uses of v are inside the if statement)
//
//	v, ok := x.(*T)                                  (otherwise: before the statement, in its block)
//	return ok && v.f == 1
//
// and drops the declaration of v. For a class with subclasses (the assertion is to T_as and the
// value is p.as_T()), when the pattern is an if's whole condition:
//
//	if p, ok := x.(T_as); ok {
//		v := p.as_T()
//
// Java's behaviour is kept: x is evaluated once, at the same point (the pattern was the first
// operand evaluated); ok is the instanceof; v is the asserted value when it holds. When it does
// not hold, v was left as it was (null, or an earlier match's value in a loop) and is now nil:
// Java does not let a pattern variable be read where the match is not known to have succeeded
// (definite assignment), so no code reads it there; the step still requires that v is not used
// before the statement and, for the second form, that every use is in the statement's block
// after it.
//
// Left, counted: a pattern that is not the first operand evaluated (`a && x instanceof T v`: Go
// has no comma-ok expression, and evaluating x earlier is another program), a for condition, a
// variable used outside the statement's block, a T_as pattern followed by other operands.
// A rewritten pattern no longer matches, so running the step again changes nothing.
//
// usage: patinst [-n] <dir of package closure>
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

type pat struct {
	call *ast.CallExpr
	x    ast.Expr // the value tested
	typ  ast.Expr // the asserted type
	v    types.Object
	as   string // ".as_T()" or ""
}

// match recognizes the function literal of a pattern.
func match(c *ast.CallExpr) *pat {
	lit, ok := c.Fun.(*ast.FuncLit)
	if !ok || len(c.Args) != 0 || len(lit.Body.List) != 5 || lit.Type.Params.NumFields() != 0 {
		return nil
	}
	b := lit.Body.List
	a0, ok := b[0].(*ast.AssignStmt)
	if !ok || a0.Tok != token.DEFINE || len(a0.Lhs) != 1 || rw.Text(a0.Lhs[0]) != "v_" {
		return nil
	}
	a1, ok := b[1].(*ast.AssignStmt)
	if !ok || a1.Tok != token.DEFINE || len(a1.Lhs) != 2 || len(a1.Rhs) != 1 {
		return nil
	}
	ta, ok := a1.Rhs[0].(*ast.TypeAssertExpr)
	if !ok || rw.Text(ta.X) != "v_" || ta.Type == nil {
		return nil
	}
	pn, okn := rw.Text(a1.Lhs[0]), rw.Text(a1.Lhs[1])
	if rw.Text(b[2]) != "if !"+okn+" {\n"+indentOf(b[2])+"\treturn false\n"+indentOf(b[2])+"}" {
		return nil
	}
	a3, ok := b[3].(*ast.AssignStmt)
	if !ok || a3.Tok != token.ASSIGN || len(a3.Lhs) != 1 || len(a3.Rhs) != 1 {
		return nil
	}
	id, ok := a3.Lhs[0].(*ast.Ident)
	if !ok {
		return nil
	}
	rhs := rw.Text(a3.Rhs[0])
	as := ""
	if rhs != pn {
		if !strings.HasPrefix(rhs, pn+".as_") || !strings.HasSuffix(rhs, "()") {
			return nil
		}
		as = rhs[len(pn):]
	}
	if rw.Text(b[4]) != "return true" {
		return nil
	}
	return &pat{call: c, x: a0.Rhs[0], typ: ta.Type, v: rw.Info.Uses[id], as: as}
}

func indentOf(n ast.Node) string {
	f := rw.Fset.File(n.Pos())
	src := rw.Src[f]
	o := f.Offset(n.Pos())
	i := o
	for i > 0 && src[i-1] != '\n' {
		i--
	}
	return string(src[i:o])
}

// first reports whether c is the first operand that e evaluates: e itself, or reached through
// the left sides of && / || and through ! and parentheses.
func first(e ast.Expr, c ast.Expr) bool {
	for {
		e = rw.Unparen(e)
		if e == c {
			return true
		}
		switch x := e.(type) {
		case *ast.BinaryExpr:
			if x.Op != token.LAND && x.Op != token.LOR {
				return false
			}
			e = x.X
		case *ast.UnaryExpr:
			if x.Op != token.NOT {
				return false
			}
			e = x.X
		default:
			return false
		}
	}
}

func funcOf(n ast.Node) ast.Node {
	for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
		if _, ok := p.(*ast.FuncDecl); ok {
			return p
		}
	}
	return nil
}

func okName(n ast.Node, taken map[ast.Node]map[string]bool) string {
	fn := funcOf(n)
	if taken[fn] == nil {
		used := map[string]bool{}
		ast.Inspect(fn, func(m ast.Node) bool {
			if c, ok := m.(*ast.CallExpr); ok && match(c) != nil {
				// a pattern's own p and ok go with it; the value tested stays
				ast.Inspect(match(c).x, func(m ast.Node) bool {
					if id, ok := m.(*ast.Ident); ok {
						used[id.Name] = true
					}
					return true
				})
				return false
			}
			if id, ok := m.(*ast.Ident); ok {
				used[id.Name] = true
			}
			return true
		})
		taken[fn] = used
	}
	for i := 1; ; i++ {
		name := "ok"
		if i > 1 {
			name = fmt.Sprintf("ok%d", i)
		}
		if !taken[fn][name] {
			taken[fn][name] = true
			return name
		}
	}
}

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	done := map[string]int{}
	left := map[string]int{}
	taken := map[ast.Node]map[string]bool{}
	for _, f := range rw.Files {
		fn := rw.Fset.Position(f.Pos()).Filename
		if !strings.HasPrefix(fn[strings.LastIndex(fn, "/")+1:], "g_") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			p := match(c)
			if p == nil || p.v == nil {
				return true
			}
			// the declaration `var v T` and the `_ = v` after it
			var decl, blank ast.Stmt
			var declType ast.Expr
			var uses []*ast.Ident
			for _, id := range rw.UsesOf[p.v] {
				if as, ok := rw.Parent[id].(*ast.AssignStmt); ok && len(as.Lhs) == 1 && rw.Text(as.Lhs[0]) == "_" && len(as.Rhs) == 1 && as.Rhs[0] == ast.Expr(id) {
					blank = as
					continue
				}
				if id.Pos() >= c.Pos() && id.End() <= c.End() {
					continue // the pattern's own assignment
				}
				uses = append(uses, id)
			}
			for id, o := range rw.Info.Defs {
				if o == p.v {
					if vs, ok := rw.Parent[id].(*ast.ValueSpec); ok && len(vs.Names) == 1 && len(vs.Values) == 0 && vs.Type != nil {
						if gd, ok := rw.Parent[vs].(*ast.GenDecl); ok && len(gd.Specs) == 1 {
							if ds, ok := rw.Parent[gd].(*ast.DeclStmt); ok {
								decl, declType = ds, vs.Type
							}
						}
					}
				}
			}
			if decl == nil || blank == nil {
				left["the variable is not declared by `var v T; _ = v`"]++
				return true
			}
			// the statement the pattern is in
			var stmt ast.Stmt
			for q := rw.Parent[c]; q != nil; q = rw.Parent[q] {
				if s, ok := q.(ast.Stmt); ok {
					stmt = s
					break
				}
				if _, ok := q.(*ast.FuncLit); ok {
					break
				}
			}
			var expr ast.Expr
			ifs, isIf := stmt.(*ast.IfStmt)
			switch s := stmt.(type) {
			case *ast.IfStmt:
				if s.Init == nil {
					expr = s.Cond
				}
			case *ast.ReturnStmt:
				if len(s.Results) == 1 {
					expr = s.Results[0]
				}
			case *ast.ForStmt:
				left["a for condition"]++
				return true
			}
			if expr == nil || !first(expr, c) {
				left["not the first operand its statement evaluates (Go has no comma-ok expression)"]++
				return true
			}
			at := func() string {
				s := rw.Render(p.x)
				switch rw.Unparen(p.x).(type) {
				case *ast.Ident, *ast.CallExpr, *ast.SelectorExpr, *ast.IndexExpr:
				default:
					s = "(" + s + ")"
				}
				return s + ".(" + rw.Text(p.typ) + ")"
			}
			vname := p.v.Name()
			inside := func(a, b token.Pos) bool {
				for _, id := range uses {
					if id.Pos() < a || id.End() > b {
						return false
					}
				}
				return true
			}
			if dt := rw.Info.Types[declType].Type; p.as == "" && (dt == nil || !types.Identical(dt, rw.Info.Types[p.typ].Type)) {
				left["the variable's type is not the asserted type"]++
				return true
			}
			if p.as != "" {
				// if p, ok := x.(T_as); ok { v := p.as_T() ...
				if !isIf || rw.Unparen(ifs.Cond) != ast.Expr(c) || !inside(ifs.Body.Lbrace, ifs.Body.Rbrace) {
					left["a class with subclasses (T_as) where the pattern is not an if's whole condition, or the variable is used outside its body"]++
					return true
				}
				name := okName(c, taken)
				pn := "p"
				for i := 2; taken[funcOf(c)][pn]; i++ {
					pn = fmt.Sprintf("p%d", i)
				}
				taken[funcOf(c)][pn] = true
				done["if p, ok := x.(T_as); ok { v := p.as_T()"]++
				rw.Rewrite[ifs.Cond] = func() string { return pn + ", " + name + " := " + at() + "; " + name }
				lb := ifs.Body
				ind := indentOf(ifs) + "\t"
				first := ""
				if len(uses) > 0 {
					first = "\n" + ind + vname + " := " + pn + p.as
				} else {
					first = "\n" + ind + "_ = " + pn
				}
				rw.Rewrite[lb] = func() string {
					s := rw.RenderChildren(lb)
					return "{" + first + s[1:]
				}
				rw.Rewrite[decl] = func() string { return rw.Del }
				rw.Rewrite[blank] = func() string { return rw.Del }
				return false
			}
			if isIf && inside(ifs.Pos(), ifs.End()) {
				name := okName(c, taken)
				done["if v, ok := x.(T); ok ..."]++
				bind := vname
				if len(uses) == 0 {
					bind = "_"
				}
				rw.Rewrite[c] = func() string { return name }
				cond := ifs.Cond
				rw.Rewrite[cond] = func() string {
					rest := name
					if cond != ast.Expr(c) {
						rest = rw.RenderChildren(cond)
					}
					return bind + ", " + name + " := " + at() + "; " + rest
				}
				rw.Rewrite[decl] = func() string { return rw.Del }
				rw.Rewrite[blank] = func() string { return rw.Del }
				return false
			}
			// before the statement, in its block
			blk, ok := rw.Parent[stmt].(*ast.BlockStmt)
			if !ok {
				if cc, ok := rw.Parent[stmt].(*ast.CaseClause); ok {
					_ = cc
				}
				left["an else-if whose variable is used after the if statement"]++
				return true
			}
			if !inside(stmt.Pos(), blk.Rbrace) {
				left["the variable is used outside the statement's block, or before the statement"]++
				return true
			}
			// no other declaration of the name may be in scope at the statement: the new one
			// must not shadow or collide
			if rw.Parent[decl] == ast.Node(blk) {
				// same block: the new declaration replaces the old one
			} else if declaredIn(blk, vname) {
				left["the statement's block declares the name"]++
				return true
			}
			name := okName(c, taken)
			done["v, ok := x.(T) before the statement"]++
			bind := vname
			if len(uses) == 0 {
				bind = "_"
			}
			rw.Rewrite[c] = func() string { return name }
			ind := indentOf(stmt)
			rw.Rewrite[stmt] = func() string {
				return bind + ", " + name + " := " + at() + "\n" + ind + rw.RenderChildren(stmt)
			}
			rw.Rewrite[decl] = func() string { return rw.Del }
			rw.Rewrite[blank] = func() string { return rw.Del }
			return false
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
	print(done, "pattern")
	print(left, "left")
	if report {
		return
	}
	fmt.Fprintf(os.Stderr, "patinst: files changed: %d\n", rw.SaveChecked())
}

// declaredIn reports whether the block declares name directly.
func declaredIn(blk *ast.BlockStmt, name string) bool {
	for _, s := range blk.List {
		switch x := s.(type) {
		case *ast.DeclStmt:
			if gd, ok := x.Decl.(*ast.GenDecl); ok {
				for _, sp := range gd.Specs {
					if vs, ok := sp.(*ast.ValueSpec); ok {
						for _, n := range vs.Names {
							if n.Name == name {
								return true
							}
						}
					}
				}
			}
		case *ast.AssignStmt:
			if x.Tok == token.DEFINE {
				for _, l := range x.Lhs {
					if id, ok := l.(*ast.Ident); ok && id.Name == name {
						return true
					}
				}
			}
		}
	}
	return false
}
