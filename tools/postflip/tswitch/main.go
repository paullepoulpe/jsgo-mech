// Command tswitch is a post-flip step: a chain of if / else if whose conditions test the class
// of one value, one after the other, is a type switch.
//
//	if c, ok := t.self().(*RegExpTree_Concatenation); ok {      switch c := t.self().(type) {
//		A                                                       case *RegExpTree_Concatenation:
//	} else if _, ok2 := t.self().(*RegExpTree_Empty); ok2 {  ->      A
//		B                                                       case *RegExpTree_Empty:
//	} else {                                                         B
//		C                                                       default:
//	}                                                                C
//	                                                            }
//
// Java's instanceof chains (`if (x instanceof A) ... else if (x instanceof B) ...`) reach here as
// comma-ok assertions (castfold -instof, patinst); Go writes a chain of tests of one value's
// type as a type switch, and the hand port does (jsgo: 643 type switches and comma-ok
// assertions). Two or more consecutive links qualify when each is `if v, ok := S.(T); ok` (v
// may be _) on the same operand S, and S is pure (a variable, a field read, a node column read,
// a self() of such: what it gives does not change between the tests, and evaluating it has no
// effect). The tests run in order and a value matches at most one case the same way: a case
// clause of a type switch is taken when the value's dynamic type is T (an interface T: when it
// implements T), as the assertion succeeds, and clauses are tried in order; nil matches no
// case, as it fails every assertion. The rest of the chain (an else block, or an else if that
// is not such a test) is the default clause, unchanged. A chain that goes on with, or is the
// else of, a test of something else is left: the switch would nest that test (or itself) one
// level deeper than the chain has it. S is evaluated once instead of once per
// test, which is the same because it is pure.
//
// A link may also be `if is_[A](S) || is_B(S) ...`, a disjunction of instanceof tests of S
// alone (the generic is_[T], or a generated helper is_X whose body is the assertion to a named
// type): it is the case `case A, B:`, taken when the value's type is one of them, as the
// disjunction is true when one of the assertions succeeds. It binds no name.
//
// The switch binds one name: links that bind must bind the same name, and a link that does not,
// and the default, must not mention that name (the switch would shadow it there). The ok
// variables are not used anywhere else. Left, counted: chains that do not meet these. A rerun
// changes nothing (a switch is no if chain).
//
// usage: tswitch [-n] [-v] <dir of package closure>
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

// link is one if of a chain: its bound name ("" for _), ok name, asserted type and statement.
type link struct {
	ifs  *ast.IfStmt
	bind *ast.Ident
	ok   *ast.Ident
	typ  ast.Expr
	subj ast.Expr
	// typs: the types of a link whose condition is a disjunction of instanceof tests of one
	// value (is_[A](v) || is_B(v) ...): a case with several types. Such a link binds nothing
	// and has no ok variable.
	typs []ast.Expr
}

// isTest returns the tested value and type when e is is_[T](v), or is_X(v) of a generated
// helper without type parameters whose body is `_, ok := v.(T); return ok`.
func isTest(e ast.Expr) (ast.Expr, ast.Expr) {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) != 1 {
		return nil, nil
	}
	switch f := c.Fun.(type) {
	case *ast.IndexExpr:
		if id, ok := f.X.(*ast.Ident); ok && id.Name == "is_" {
			if fn, ok := rw.Info.Uses[id].(*types.Func); ok && fn.Pkg() == rw.Pkg && fn.Parent() == rw.Pkg.Scope() {
				return c.Args[0], f.Index
			}
		}
	case *ast.Ident:
		fn, ok := rw.Info.Uses[f].(*types.Func)
		if !ok || !strings.HasPrefix(f.Name, "is_") || fn.Pkg() != rw.Pkg || fn.Parent() != rw.Pkg.Scope() {
			return nil, nil
		}
		if t := helperType(fn); t != nil {
			return c.Args[0], t
		}
	}
	return nil, nil
}

var helperDecls map[types.Object]*ast.FuncDecl

// helperType is T of func is_X(v any) bool { _, ok := v.(T); return ok }, T a named type.
func helperType(fn *types.Func) ast.Expr {
	if helperDecls == nil {
		helperDecls = map[types.Object]*ast.FuncDecl{}
		for _, f := range rw.Files {
			for _, d := range f.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Body != nil {
					helperDecls[rw.Info.Defs[fd.Name]] = fd
				}
			}
		}
	}
	fd := helperDecls[fn]
	if fd == nil || fd.Type.TypeParams != nil || fd.Type.Params.NumFields() != 1 || len(fd.Type.Params.List[0].Names) != 1 || len(fd.Body.List) != 2 {
		return nil
	}
	as, ok := fd.Body.List[0].(*ast.AssignStmt)
	if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 2 || len(as.Rhs) != 1 || rw.Text(as.Lhs[0]) != "_" {
		return nil
	}
	ta, ok := as.Rhs[0].(*ast.TypeAssertExpr)
	if !ok || ta.Type == nil || rw.Text(ta.X) != fd.Type.Params.List[0].Names[0].Name {
		return nil
	}
	rs, ok := fd.Body.List[1].(*ast.ReturnStmt)
	if !ok || len(rs.Results) != 1 || rw.Text(rs.Results[0]) != rw.Text(as.Lhs[1]) {
		return nil
	}
	switch t := ta.Type.(type) {
	case *ast.Ident:
		return t
	case *ast.StarExpr:
		if _, ok := t.X.(*ast.Ident); ok {
			return t
		}
	}
	return nil
}

// orTests: the tests of a disjunction a || b || ..., each an instanceof test of the same value.
func orTests(e ast.Expr) (subj ast.Expr, typs []ast.Expr) {
	e = rw.Unparen(e)
	if b, ok := e.(*ast.BinaryExpr); ok && b.Op == token.LOR {
		s1, t1 := orTests(b.X)
		s2, t2 := orTests(b.Y)
		if s1 == nil || s2 == nil || rw.Text(s1) != rw.Text(s2) {
			return nil, nil
		}
		return s1, append(t1, t2...)
	}
	s, t := isTest(e)
	if s == nil {
		return nil, nil
	}
	return s, []ast.Expr{t}
}

func main() {
	report, verbose := false, false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	dir := os.Args[len(os.Args)-1]
	rw.Load(dir)
	done := map[string]int{}
	left := map[string]int{}
	leftAt := map[string][]string{}
	skip := func(why string, n ast.Node) {
		left[why]++
		leftAt[why] = append(leftAt[why], rw.Fset.Position(n.Pos()).String())
	}
	var pure func(e ast.Expr) bool
	pure = func(e ast.Expr) bool {
		if rw.Pure(e) {
			return true
		}
		call, ok := rw.Unparen(e).(*ast.CallExpr)
		if !ok || len(call.Args) != 0 {
			return false
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		return ok && sel.Sel.Name == "self" && pure(sel.X)
	}
	// asLink is ifs as a link, or nil.
	asLink := func(ifs *ast.IfStmt) *link {
		if ifs.Init == nil {
			if subj, typs := orTests(ifs.Cond); len(typs) >= 2 {
				return &link{ifs: ifs, subj: subj, typs: typs}
			}
			return nil
		}
		as, ok := ifs.Init.(*ast.AssignStmt)
		if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 2 || len(as.Rhs) != 1 {
			return nil
		}
		ta, ok := as.Rhs[0].(*ast.TypeAssertExpr)
		if !ok || ta.Type == nil {
			return nil
		}
		okId, ok1 := as.Lhs[1].(*ast.Ident)
		v, ok2 := as.Lhs[0].(*ast.Ident)
		cond, ok3 := ifs.Cond.(*ast.Ident)
		if !ok1 || !ok2 || !ok3 || okId.Name == "_" || cond.Name != okId.Name {
			return nil
		}
		l := &link{ifs: ifs, ok: okId, typ: ta.Type, subj: ta.X, typs: []ast.Expr{ta.Type}}
		if v.Name != "_" {
			l.bind = v
		}
		return l
	}
	// mentions reports whether n refers to the object of id's name as seen from inside n
	// (any identifier of that name: a conservative test).
	mentions := func(n ast.Node, name string) bool {
		found := false
		if n == nil {
			return false
		}
		ast.Inspect(n, func(c ast.Node) bool {
			if id, ok := c.(*ast.Ident); ok && id.Name == name {
				found = true
			}
			return !found
		})
		return found
	}
	// usedOnlyAsCond: the ok variable is read only by its if's condition.
	usedOnlyAsCond := func(l *link) bool {
		if l.ok == nil {
			return true
		}
		obj := rw.Info.Defs[l.ok]
		for _, u := range rw.UsesOf[obj] {
			if u != l.ifs.Cond {
				return false
			}
		}
		return true
	}
	inner := map[*ast.IfStmt]bool{}
	for _, file := range rw.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			ifs, ok := n.(*ast.IfStmt)
			if !ok || inner[ifs] {
				return true
			}
			first := asLink(ifs)
			if first == nil {
				return true
			}
			chain := []*link{first}
			subj := rw.Text(first.subj)
			var rest ast.Stmt = ifs.Else
			for rest != nil {
				ei, ok := rest.(*ast.IfStmt)
				if !ok {
					break
				}
				l := asLink(ei)
				if l == nil || rw.Text(l.subj) != subj {
					break
				}
				inner[ei] = true
				chain = append(chain, l)
				rest = ei.Else
			}
			if len(chain) < 2 {
				return true
			}
			if _, ok := rest.(*ast.IfStmt); ok {
				skip("the chain goes on with a test of something else (the switch's default would nest it)", ifs)
				return true
			}
			if p, ok := rw.Parent[ifs].(*ast.IfStmt); ok && p.Else == ast.Stmt(ifs) {
				skip("the chain is the else if of a test of something else (the switch would nest in an else block)", ifs)
				return true
			}
			if !pure(first.subj) {
				skip("the tested value is not a variable or a field read", ifs)
				return true
			}
			name := ""
			for _, l := range chain {
				if !usedOnlyAsCond(l) {
					skip("an ok variable is used beyond its test", ifs)
					return true
				}
				if l.bind != nil {
					if name != "" && name != l.bind.Name {
						skip("the links bind different names", ifs)
						return true
					}
					name = l.bind.Name
				}
			}
			if name != "" {
				for _, l := range chain {
					if l.bind == nil && mentions(l.ifs.Body, name) {
						skip("a link that does not bind mentions the bound name", ifs)
						return true
					}
				}
				if mentions(rest, name) || mentions(first.subj, name) {
					skip("the default or the tested value mentions the bound name", ifs)
					return true
				}
			}
			// The cases' types must be distinct (Go rejects a duplicate case).
			seen := map[string]bool{}
			for _, l := range chain {
				for _, typ := range l.typs {
					t := types.TypeString(rw.Info.Types[typ].Type, nil)
					if seen[t] {
						skip("two links test one type", ifs)
						return true
					}
					seen[t] = true
				}
			}
			done[fmt.Sprintf("if/else-if chain of %d type tests -> type switch", len(chain))]++
			rw.Rewrite[ifs] = func() string {
				var b strings.Builder
				b.WriteString("switch ")
				if name != "" {
					b.WriteString(name + " := ")
				}
				b.WriteString(rw.Render(first.subj) + ".(type) {\n")
				for _, l := range chain {
					var ts []string
					for _, typ := range l.typs {
						ts = append(ts, rw.Render(typ))
					}
					b.WriteString("case " + strings.Join(ts, ", ") + ":\n")
					b.WriteString(blockBody(rw.RenderChildren(l.ifs.Body)))
				}
				if rest != nil {
					b.WriteString("default:\n")
					b.WriteString(blockBody(rw.RenderChildren(rest)))
				}
				b.WriteString("}")
				return b.String()
			}
			return true
		})
	}
	keys := []string{}
	for k := range left {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", left[k], k)
		if verbose {
			for _, p := range leftAt[k] {
				fmt.Fprintf(os.Stderr, "        %s\n", p)
			}
		}
	}
	total := 0
	keys = keys[:0]
	for k, n := range done {
		keys = append(keys, k)
		total += n
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %4d %s\n", done[k], k)
	}
	if report || total == 0 {
		fmt.Fprintf(os.Stderr, "tswitch: %d chains\n", total)
		return
	}
	fmt.Fprintf(os.Stderr, "tswitch: %d chains; files changed: %d\n", total, rw.SaveChecked())
}

// blockBody is the statements of a rendered block, without its braces and the newline after "{".
func blockBody(block string) string {
	return strings.TrimLeft(strings.TrimSuffix(strings.TrimPrefix(block, "{"), "}"), "\n")
}
