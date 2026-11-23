// Command nnchecks is a post-flip step: the typed checkNotNull calls that castfold makes
// (Preconditions_checkNotNullT and its message variants, runtime generics in rt_base.go) become
// inline Go checks, as tools/postflip/checks does for the Guava calls.
//
// A call is rewritten when its statement evaluates it first and nothing else: the statement is
// the call itself, an assignment or declaration of one variable whose value is the call, or a
// return of the call. The checked value must be a variable or a field of `this` (reading it can
// do nothing else), and each message argument a constant or a variable, so that evaluating them
// on the failure path only is the same as Java evaluating them before the call. The statement
// is preceded by
//
//	if v == <zero> { <the helper's panic> }
//
// with the zero value of v's type (nil for a pointer, interface, map, slice, channel or func; 0
// for a number or a node handle), which is exactly the helper's `v == zero`, and the call is
// replaced by v. The panic is the helper's: NullPointerException without a message, or
// nullFail_ of the same message. String values are left (the helper is not for them). Every
// call left is counted with its reason.
//
// usage: nnchecks [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"sort"
	"strings"

	"postflip/rw"
)

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	left := map[string]int{}
	done, optionals := 0, 0
	var at ast.Node
	leave := func(why string) {
		left[why]++
		if os.Getenv("NNCHECKS_VERBOSE") != "" {
			fmt.Fprintf(os.Stderr, "%s: %s\n", rw.Fset.Position(at.Pos()), why)
		}
	}
	markModified()

	for _, f := range rw.Files {
		name := rw.Fset.Position(f.Pos()).Filename
		if !strings.HasPrefix(name[strings.LastIndex(name, "/")+1:], "g_") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			at = c
			if why, ok := optional(c); ok {
				if why != "" {
					leave(why)
				} else {
					optionals++
				}
				return true
			}
			var fname string
			switch fn := c.Fun.(type) {
			case *ast.Ident:
				fname = fn.Name
			case *ast.IndexExpr: // explicit instantiation
				if id, ok := fn.X.(*ast.Ident); ok {
					fname = id.Name
				}
			}
			if strings.HasPrefix(fname, "Preconditions_checkNotNull") && nested(c) {
				leave("a check inside the arguments of another check")
				return true
			}
			if fname == "Preconditions_checkNotNull" || fname == "Preconditions_checkNotNull_Object_Object" {
				if why := erased(c, fname); why != "" {
					leave(why)
				} else {
					hoisted++
				}
				return true
			}
			if !strings.HasPrefix(fname, "Preconditions_checkNotNullT") {
				return true
			}
			variant := strings.TrimPrefix(fname, "Preconditions_checkNotNullT")
			if len(c.Args) == 0 {
				return true
			}
			v := rw.Unparen(c.Args[0])
			if !simple(v) {
				// The value is computed: hoisted into a temporary when the statement computes it first.
				if why := hoist(c, c.Args[0], c.Args[1:], variant); why != "" {
					leave(why)
				} else {
					hoisted++
				}
				return true
			}
			zv, ok := zero(rw.Info.Types[v].Type)
			if !ok {
				leave("checked value is a string or a struct (not the helper's nil test)")
				return true
			}
			for _, a := range c.Args[1:] {
				if !msgRead(a) {
					leave("a message argument computes something (Java computes it before the test)")
					return true
				}
			}
			throw, ok := throwOf(variant, c.Args[1:])
			if !ok {
				leave("variant " + variant)
				return true
			}
			// The statement that evaluates the call first and only it.
			var stmt ast.Stmt
			p := rw.Up(c)
			switch s := p.(type) {
			case *ast.ExprStmt:
				stmt = s
			case *ast.AssignStmt:
				if len(s.Lhs) == 1 && len(s.Rhs) == 1 && rw.Unparen(s.Rhs[0]) == ast.Expr(c) && simpleTarget(s.Lhs[0]) {
					stmt = s
				}
			case *ast.ReturnStmt:
				if len(s.Results) == 1 {
					stmt = s
				}
			case *ast.ValueSpec:
				if len(s.Names) == 1 && len(s.Values) == 1 {
					if g, ok := rw.Parent[s].(*ast.GenDecl); ok && len(g.Specs) == 1 && g.Tok == token.VAR {
						if d, ok := rw.Parent[g].(*ast.DeclStmt); ok {
							stmt = d
						}
					}
				}
			}
			if stmt == nil { // inside an expression: tested before the statement when it reads only before
				if why := hoist(c, c.Args[0], c.Args[1:], variant); why != "" {
					leave(why)
				} else {
					hoisted++
				}
				return true
			}
			switch rw.Parent[stmt].(type) {
			case *ast.BlockStmt, *ast.CaseClause, *ast.CommClause:
			default:
				leave("statement is not in a statement list")
				return true
			}
			done++
			vs := rw.Render(v)
			check := "if " + vs + " == " + zv + " {\n" + throw + "\n}\n"
			st := stmt
			if _, isExpr := st.(*ast.ExprStmt); isExpr {
				rw.Rewrite[st] = func() string { return strings.TrimSuffix(check, "\n") }
			} else {
				rw.Rewrite[c] = func() string { return vs }
				rw.Rewrite[st] = func() string { return check + rw.RenderChildren(st) }
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
	}
	msg := fmt.Sprintf("nnchecks: %d typed checkNotNull calls become inline checks (%d of them hoisted); %d Optional.ofNullable tests", done+hoisted, hoisted, optionals)
	if report {
		fmt.Fprintln(os.Stderr, msg)
		return
	}
	fmt.Fprintf(os.Stderr, "%s; files changed: %d\n", msg, rw.SaveChecked())
}

// simpleTarget: an assignment target whose evaluation does nothing (a variable or a field of a
// variable), so that checking before the assignment keeps Java's order.
func simpleTarget(e ast.Expr) bool {
	switch x := rw.Unparen(e).(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		_, ok := x.X.(*ast.Ident)
		return ok
	}
	return false
}
