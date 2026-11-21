// Command tryfn is a post-flip step: j2go's try statement, a function literal called in place
// that returns a completion code, is written as Go writes the same control flow.
//
// j2go emulates `try { B } catch (...) { C } finally { F }` with
//
//	code_1 := func() (code_ int) {
//		defer func() { F }()                                  // finally, if any
//		defer func() { if r_ := recover(); r_ != nil { ... } }() // the catch clauses, if any
//		B                         // `return K`: 0 completes the try, others leave it
//		return 0
//	}()
//	if code_1 == 1 { return ret_1 } ...
//
// because a recover only catches what panics while the function deferring it runs. The code
// says how the try statement completed (normally, or by a return, break or continue of the code
// around it), and ret_N carries a returned value out of the literal.
//
// 1. A try statement that ends a function (it is the last statement of the function's body,
// followed only by the code tests and panic("unreachable")) needs no literal: its defers are
// deferred by the function itself, from the place where the try statement starts, which is
// how Go code recovers in the function whose panics it handles (jsgo:
// NodeTraversal.Traverse, ParserRunner.parse). What runs before the defer statement is not
// covered by it, as it was not by the try; nothing runs after the try statement but the
// function's return. Each `return K` of the try block returns from the function: in a
// function without results `return`; in a function with one result, `ret_N = x; return 1`
// becomes `return x`, and the result is named ret_N so that a catch clause can still set it
// (`ret_N = x; code_ = 1; return` -> `ret_N = x; return`). The deferred calls run in the same
// order (catch, then finally, then the function's return), and a panic the catch clauses do
// not handle leaves the function as it left the literal.
//
// 2. Elsewhere, a try whose code nothing reads (`_ = func() (code_ int) {...}()`: the try
// block only completes normally or by a return that j2go wrote as `return 0`, and no catch
// clause sets code_) is a function literal without a result: func() { ... }().
//
// 3. A try with a finally block and no catch clause that never completes normally (its block
// ends in `return 1`, every return of it is `return 1`, and `if code_N == 1 { return ret_N }`
// follows it) leaves the function wherever it stands, so the function itself can defer the
// finally block from the place where the try starts: nothing of the function runs after the
// try, and the deferred call runs when the function returns or panics, as it did when the
// literal did. `ret_N = x; return 1` becomes `return x` (the finally block cannot change the
// value: the result is not named). jsgo: SubtypeChecker.isSubtypeCaching.
//
// Left and counted: tries whose code is read where they are not the end of the function (a
// return, break or continue from inside the try block leaves code around the literal, which
// Go cannot do from inside a function literal), and the shapes the step does not know.
//
// usage: tryfn [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"postflip/rw"
)

var count = map[string]int{}

var verbose = os.Getenv("TRYFN_VERBOSE") != ""

func note(what string, n ast.Node) {
	count[what]++
	if verbose {
		fmt.Fprintf(os.Stderr, "%s: %s\n", rw.Fset.Position(n.Pos()), what)
	}
}

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue // the handwritten runtime is read, not rewritten
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if lit, ok := n.(*ast.FuncLit); ok && isTry(lit) {
				visit(lit)
			}
			return true
		})
	}
	var keys []string
	for k := range count {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("%5d %s\n", count[k], k)
	}
	if !report {
		fmt.Printf("tryfn: %d files changed\n", rw.SaveChecked())
	}
}

// isTry reports whether lit is `func() (code_ int) {...}` called in place.
func isTry(lit *ast.FuncLit) bool {
	r := lit.Type.Results
	if lit.Type.Params.NumFields() != 0 || r == nil || len(r.List) != 1 || len(r.List[0].Names) != 1 || r.List[0].Names[0].Name != "code_" {
		return false
	}
	c, ok := rw.Parent[lit].(*ast.CallExpr)
	return ok && c.Fun == ast.Expr(lit) && len(c.Args) == 0
}

// returns are the return statements of lit's own body (not of the literals in it).
func returns(lit *ast.FuncLit) []*ast.ReturnStmt {
	var out []*ast.ReturnStmt
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			out = append(out, x)
		}
		return true
	})
	return out
}

// codeUses are the uses of the literal's code_ outside its own returns: catch clauses setting it.
func codeUses(lit *ast.FuncLit) []*ast.Ident {
	o := rw.Info.Defs[lit.Type.Results.List[0].Names[0]]
	return rw.UsesOf[o]
}

func intLit(e ast.Expr) (string, bool) {
	b, ok := e.(*ast.BasicLit)
	if !ok || b.Kind != token.INT {
		return "", false
	}
	return b.Value, true
}

// enclosing is the function (declaration or literal) whose body's statement list holds s
// directly, with the list.
func enclosing(s ast.Stmt) (*ast.FuncType, *ast.BlockStmt) {
	b, ok := rw.Parent[s].(*ast.BlockStmt)
	if !ok {
		return nil, nil
	}
	switch fn := rw.Parent[b].(type) {
	case *ast.FuncDecl:
		return fn.Type, b
	case *ast.FuncLit:
		// Not a deferred literal (a finally block): a recover in a call it defers would run
		// while the function deferring it runs for a panic.
		if c, ok := rw.Parent[fn].(*ast.CallExpr); ok {
			if _, ok := rw.Parent[c].(*ast.DeferStmt); ok {
				return nil, nil
			}
		}
		return fn.Type, b
	}
	return nil, nil
}

var codeTest = regexp.MustCompile(`^code_\d+ == (\d+)$`)

func visit(lit *ast.FuncLit) {
	call := rw.Parent[lit].(*ast.CallExpr)
	stmt, ok := rw.Parent[call].(*ast.AssignStmt)
	if !ok || len(stmt.Lhs) != 1 || len(stmt.Rhs) != 1 {
		note("left: try literal in an unknown statement", lit)
		return
	}
	lhs := rw.Text(stmt.Lhs[0])
	if tryEndsFunction(lit, stmt, lhs) {
		return
	}
	if tryAlwaysReturns(lit, stmt, lhs) {
		return
	}
	if lhs != "_" {
		note("left: try whose completion code is read (a return, break or continue from inside the try block leaves code around the literal)", lit)
		return
	}
	// 2. _ = func() (code_ int) { ... }()  ->  func() { ... }()
	if len(codeUses(lit)) > 0 {
		note("left: try whose code a catch clause sets, though nothing reads it", lit)
		return
	}
	rs := returns(lit)
	for _, r := range rs {
		if len(r.Results) != 1 {
			note("left: try literal with a return of an unknown shape", lit)
			return
		}
	}
	last := lit.Body.List[len(lit.Body.List)-1]
	for _, r := range rs {
		r := r
		if ast.Stmt(r) == last {
			rw.Rewrite[r] = func() string { return rw.Del }
		} else {
			rw.Rewrite[r] = func() string { return "return" }
		}
	}
	rw.Rewrite[lit.Type] = func() string { return "func()" }
	rw.Rewrite[stmt] = func() string { return rw.Render(call) }
	note("try whose code nothing reads: func() { ... }() without a completion code", lit)
}

// tryEndsFunction rewrites case 1 when the try statement ends its function.
func tryEndsFunction(lit *ast.FuncLit, stmt *ast.AssignStmt, lhs string) bool {
	ft, body := enclosing(stmt)
	if ft == nil {
		return false
	}
	idx := -1
	for i, s := range body.List {
		if s == ast.Stmt(stmt) {
			idx = i
		}
	}
	// What follows: tests of the code, then panic("unreachable").
	var tests []*ast.IfStmt
	var tail []ast.Stmt
	for _, s := range body.List[idx+1:] {
		if is, ok := s.(*ast.IfStmt); ok && is.Init == nil && is.Else == nil && lhs != "_" && strings.HasPrefix(rw.Text(is.Cond), lhs+" == ") {
			if len(tail) > 0 {
				return false
			}
			tests = append(tests, is)
			continue
		}
		if rw.Text(s) == `panic("unreachable")` {
			tail = append(tail, s)
			continue
		}
		return false
	}
	nres := ft.Results.NumFields()
	if nres > 1 || (ft.Results != nil && len(ft.Results.List) == 1 && len(ft.Results.List[0].Names) > 0) {
		note("left: try ending a function with several or named results", lit)
		return true
	}
	rs := returns(lit)
	var retVar *types.Var
	var retDecl ast.Stmt
	if nres == 1 {
		// if code_N == 1 { return ret_N }, var ret_N T before, of the result type.
		if len(tests) != 1 || len(tests[0].Body.List) != 1 {
			note("left: try ending a function, with code tests of an unknown shape", lit)
			return true
		}
		m := codeTest.FindStringSubmatch(rw.Text(tests[0].Cond))
		r, ok := tests[0].Body.List[0].(*ast.ReturnStmt)
		if m == nil || m[1] != "1" || !ok || len(r.Results) != 1 {
			note("left: try ending a function, with code tests of an unknown shape", lit)
			return true
		}
		id, ok := r.Results[0].(*ast.Ident)
		if !ok {
			note("left: try ending a function, with code tests of an unknown shape", lit)
			return true
		}
		retVar, _ = rw.Info.Uses[id].(*types.Var)
		if retVar == nil || !types.Identical(retVar.Type(), rw.Info.Types[ft.Results.List[0].Type].Type) {
			note("left: try ending a function whose returned variable is not of the result type", lit)
			return true
		}
		for _, s := range body.List[:idx] {
			if d, ok := s.(*ast.DeclStmt); ok {
				g := d.Decl.(*ast.GenDecl)
				if vs, ok := g.Specs[0].(*ast.ValueSpec); ok && len(g.Specs) == 1 && len(vs.Names) == 1 && vs.Values == nil && rw.Info.Defs[vs.Names[0]] == retVar {
					retDecl = s
				}
			}
		}
		if retDecl == nil {
			note("left: try ending a function, its returned variable not declared before it", lit)
			return true
		}
		// The variable is only set inside the try (and returned by the test).
		for _, u := range rw.UsesOf[retVar] {
			if !(lit.Pos() <= u.Pos() && u.End() <= lit.End()) && u != id {
				note("left: try ending a function, its returned variable used elsewhere", lit)
				return true
			}
		}
		for _, r := range rs {
			v, ok := intLit(r.Results[0])
			if len(r.Results) != 1 || !ok || v != "1" {
				note("left: try ending a function with results whose block returns a code other than 1", lit)
				return true
			}
		}
	} else {
		// No result: every test of the code is `if code_N == K { return }`.
		for _, t := range tests {
			if len(t.Body.List) != 1 || rw.Text(t.Body.List[0]) != "return" {
				note("left: try ending a function, with code tests of an unknown shape", lit)
				return true
			}
		}
		for _, r := range rs {
			if _, ok := intLit(r.Results[0]); len(r.Results) != 1 || !ok {
				note("left: try ending a function with a return of an unknown shape", lit)
				return true
			}
		}
	}
	// Catch clauses: `code_ = 1` (with the result set before it) only, in a function with a result.
	var codeSets []ast.Stmt
	for _, u := range codeUses(lit) {
		as, ok := rw.Parent[u].(*ast.AssignStmt)
		if !ok || len(as.Lhs) != 1 || as.Lhs[0] != ast.Expr(u) || rw.Text(as) != "code_ = 1" || nres != 1 {
			note("left: try ending a function whose catch clause sets its code in an unknown way", lit)
			return true
		}
		codeSets = append(codeSets, as)
	}
	// Rewrite.
	if nres == 1 {
		name := retVar.Name()
		res := ft.Results.List[0].Type
		rw.Rewrite[ft.Results] = func() string { return "(" + name + " " + rw.Render(res) + ")" }
		rw.Rewrite[retDecl] = func() string { return rw.Del }
		for _, s := range codeSets {
			rw.Rewrite[s] = func() string { return rw.Del }
		}
		for _, r := range rs {
			r := r
			// ret_N = x; return 1  ->  return x
			if prev := before(r); prev != nil {
				if as, ok := prev.(*ast.AssignStmt); ok && as.Tok == token.ASSIGN && len(as.Lhs) == 1 && len(as.Rhs) == 1 {
					if id, ok := as.Lhs[0].(*ast.Ident); ok && rw.Info.Uses[id] == retVar && !mentions(as.Rhs[0], retVar) {
						x := as.Rhs[0]
						rw.Rewrite[as] = func() string { return rw.Del }
						rw.Rewrite[r] = func() string { return "return " + rw.Render(x) }
						continue
					}
				}
			}
			rw.Rewrite[r] = func() string { return "return" }
		}
		note("try ending a function with a result: the function defers its catch clauses, the result is named", lit)
	} else {
		last := lit.Body.List[len(lit.Body.List)-1]
		for _, r := range rs {
			r := r
			if ast.Stmt(r) == last {
				rw.Rewrite[r] = func() string { return rw.Del }
			} else {
				rw.Rewrite[r] = func() string { return "return" }
			}
		}
		note("try ending a function without results: the function defers its catch clauses", lit)
	}
	for _, s := range tests {
		rw.Rewrite[s] = func() string { return rw.Del }
	}
	for _, s := range tail {
		rw.Rewrite[s] = func() string { return rw.Del }
	}
	rw.Rewrite[stmt] = func() string {
		var out []string
		for _, s := range lit.Body.List {
			if txt := rw.Render(s); txt != rw.Del {
				out = append(out, txt)
			}
		}
		return strings.Join(out, "\n")
	}
	return true
}

// tryAlwaysReturns rewrites case 3.
func tryAlwaysReturns(lit *ast.FuncLit, stmt *ast.AssignStmt, lhs string) bool {
	if lhs == "_" || len(codeUses(lit)) > 0 || strings.Contains(rw.Text(lit.Body), "recover()") {
		return false
	}
	block, ok := rw.Parent[stmt].(*ast.BlockStmt)
	if !ok {
		return false
	}
	// The function around the try: one unnamed result, not a deferred literal.
	var ft *ast.FuncType
	for n := rw.Parent[stmt]; n != nil && ft == nil; n = rw.Parent[n] {
		switch fn := n.(type) {
		case *ast.FuncDecl:
			ft = fn.Type
		case *ast.FuncLit:
			if c, ok := rw.Parent[fn].(*ast.CallExpr); ok {
				if _, ok := rw.Parent[c].(*ast.DeferStmt); ok {
					return false
				}
			}
			ft = fn.Type
		}
	}
	if ft == nil || ft.Results.NumFields() != 1 || len(ft.Results.List[0].Names) > 0 {
		return false
	}
	// The block: defers of plain literals first, a last statement `return 1`, every return `return 1`.
	n := len(lit.Body.List)
	if n < 2 {
		return false
	}
	if r, ok := lit.Body.List[n-1].(*ast.ReturnStmt); !ok || len(r.Results) != 1 || rw.Text(r.Results[0]) != "1" {
		return false
	}
	rs := returns(lit)
	for _, r := range rs {
		if len(r.Results) != 1 || rw.Text(r.Results[0]) != "1" {
			return false
		}
	}
	// var ret_N T right before, if code_N == 1 { return ret_N } right after.
	idx := -1
	for i, s := range block.List {
		if s == ast.Stmt(stmt) {
			idx = i
		}
	}
	if idx < 1 || idx+1 >= len(block.List) {
		return false
	}
	test, ok := block.List[idx+1].(*ast.IfStmt)
	if !ok || test.Init != nil || test.Else != nil || rw.Text(test.Cond) != lhs+" == 1" || len(test.Body.List) != 1 {
		return false
	}
	tr, ok := test.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(tr.Results) != 1 {
		return false
	}
	id, ok := tr.Results[0].(*ast.Ident)
	if !ok {
		return false
	}
	retVar, _ := rw.Info.Uses[id].(*types.Var)
	if retVar == nil || !types.Identical(retVar.Type(), rw.Info.Types[ft.Results.List[0].Type].Type) {
		return false
	}
	d, ok := block.List[idx-1].(*ast.DeclStmt)
	if !ok {
		return false
	}
	g := d.Decl.(*ast.GenDecl)
	vs, ok := g.Specs[0].(*ast.ValueSpec)
	if !ok || len(g.Specs) != 1 || len(vs.Names) != 1 || vs.Values != nil || rw.Info.Defs[vs.Names[0]] != retVar {
		return false
	}
	// The code is read by the test only; the variable is set before each return and read by the test only.
	if o := rw.Info.Defs[stmt.Lhs[0].(*ast.Ident)]; o == nil || len(rw.UsesOf[o]) != 1 {
		return false
	}
	type pair struct {
		as *ast.AssignStmt
		r  *ast.ReturnStmt
	}
	var pairs []pair
	for _, r := range rs {
		as, ok := before(r).(*ast.AssignStmt)
		if !ok || as.Tok != token.ASSIGN || len(as.Lhs) != 1 || len(as.Rhs) != 1 || mentions(as.Rhs[0], retVar) {
			return false
		}
		if l, ok := as.Lhs[0].(*ast.Ident); !ok || rw.Info.Uses[l] != retVar {
			return false
		}
		pairs = append(pairs, pair{as, r})
	}
	if len(rw.UsesOf[retVar]) != len(pairs)+1 {
		return false
	}
	for _, p := range pairs {
		p := p
		rw.Rewrite[p.as] = func() string { return rw.Del }
		rw.Rewrite[p.r] = func() string { return "return " + rw.Render(p.as.Rhs[0]) }
	}
	rw.Rewrite[ast.Stmt(d)] = func() string { return rw.Del }
	rw.Rewrite[ast.Stmt(test)] = func() string { return rw.Del }
	rw.Rewrite[stmt] = func() string {
		var out []string
		for _, s := range lit.Body.List {
			if txt := rw.Render(s); txt != rw.Del {
				out = append(out, txt)
			}
		}
		return strings.Join(out, "\n")
	}
	note("try with a finally block that always returns: the function defers the finally block", lit)
	return true
}

// before is the statement right before s in its list, or nil.
func before(s ast.Stmt) ast.Stmt {
	var list []ast.Stmt
	switch p := rw.Parent[s].(type) {
	case *ast.BlockStmt:
		list = p.List
	case *ast.CaseClause:
		list = p.Body
	case *ast.CommClause:
		list = p.Body
	}
	for i, x := range list {
		if x == s && i > 0 {
			return list[i-1]
		}
	}
	return nil
}

func mentions(e ast.Node, v *types.Var) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && rw.Info.Uses[id] == v {
			found = true
		}
		return !found
	})
	return found
}
