// Command optional is post-flip step 7, first half: makes the code independent of whether
// java.util.Optional is a pointer (*Optional) or a value (Optional). A nil of Optional type
// becomes optNil_, and x == nil / x != nil on an Optional become optNull_(x) / !optNull_(x).
// The second half (rt_base.go, g_0aliases.go, sed) changes the type itself.
//
// With -guava, the same for Guava's com.google.common.base.Optional (Optional_base): optBaseNil_
// and optBaseNull_(x). It also reports, and refuses, a value of that type converted to an
// interface other than through obj_ (a nil pointer in an interface is not null, a null value
// boxed by obj_ is); there must be none for the value form to keep the meaning.
//
// usage: optional [-guava] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"

	"postflip/rw"
)

func main() {
	rw.Load(os.Args[len(os.Args)-1])
	guava := len(os.Args) > 2 && os.Args[1] == "-guava"
	name, nilName, nullName := "Optional", "optNil_", "optNull_"
	if guava {
		name, nilName, nullName = "Optional_base", "optBaseNil_", "optBaseNull_"
	}
	opt := types.NewPointer(rw.Pkg.Scope().Lookup(name).Type())
	isOpt := func(e ast.Expr) bool {
		tv, ok := rw.Info.Types[e]
		return ok && tv.Type != nil && types.Identical(types.Unalias(tv.Type), opt)
	}
	isNil := func(e ast.Expr) bool {
		id, ok := rw.Unparen(e).(*ast.Ident)
		return ok && id.Name == "nil"
	}
	nNil, nCmp := 0, 0
	skip := map[ast.Node]bool{}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			if b, ok := n.(*ast.BinaryExpr); ok && (b.Op == token.EQL || b.Op == token.NEQ) {
				x, y := b.X, b.Y
				if isNil(x) {
					x, y = y, x
				}
				if isNil(y) && isOpt(x) {
					not := ""
					if b.Op == token.NEQ {
						not = "!"
					}
					xx := x
					rw.Rewrite[b] = func() string { return "(" + not + nullName + "(" + rw.Render(xx) + "))" }
					skip[rw.Unparen(y)] = true
					nCmp++
				}
			}
			if id, ok := n.(*ast.Ident); ok && id.Name == "nil" && !skip[id] && isOpt(id) {
				rw.Rewrite[id] = func() string { return nilName }
				nNil++
			}
			return true
		})
	}
	if guava {
		bad := 0
		for e, tv := range rw.Info.Types {
			if tv.IsType() || tv.Type == nil || !types.Identical(types.Unalias(tv.Type), opt) {
				continue
			}
			if t := ifaceTarget(e); t != nil {
				if c, ok := rw.Parent[e].(*ast.CallExpr); ok && rw.Text(c.Fun) == "obj_" {
					continue
				}
				bad++
				fmt.Fprintf(os.Stderr, "%s: converted to %s\n", rw.Fset.Position(e.Pos()), t)
			}
		}
		if bad > 0 {
			fmt.Fprintf(os.Stderr, "%d values converted to an interface other than by obj_: nothing written\n", bad)
			os.Exit(1)
		}
	}
	fmt.Fprintf(os.Stderr, "nil Optionals: %d; comparisons with nil: %d; files changed: %d\n", nNil, nCmp, rw.Save())
}

// ifaceTarget is the interface type e is converted to where it is used (an argument, an
// assignment, a return, an element), or nil.
func ifaceTarget(e ast.Expr) types.Type {
	var t types.Type
	switch x := rw.Parent[e].(type) {
	case *ast.CallExpr:
		if tv, ok := rw.Info.Types[x.Fun]; ok && tv.IsType() {
			t = tv.Type
			break
		}
		sig, ok := types.Unalias(rw.Info.Types[x.Fun].Type).Underlying().(*types.Signature)
		if !ok {
			return nil
		}
		for i, a := range x.Args {
			if a == e {
				if sig.Variadic() && i >= sig.Params().Len()-1 {
					st := sig.Params().At(sig.Params().Len() - 1).Type()
					if !x.Ellipsis.IsValid() {
						st = st.(*types.Slice).Elem()
					}
					t = st
				} else {
					t = sig.Params().At(i).Type()
				}
			}
		}
	case *ast.AssignStmt:
		for i, r := range x.Rhs {
			if r == e && len(x.Lhs) == len(x.Rhs) {
				t = rw.Info.Types[x.Lhs[i]].Type
			}
		}
	case *ast.ReturnStmt:
		for q := rw.Parent[x]; q != nil; q = rw.Parent[q] {
			var ft *ast.FuncType
			if fd, ok := q.(*ast.FuncDecl); ok {
				ft = fd.Type
			} else if fl, ok := q.(*ast.FuncLit); ok {
				ft = fl.Type
			} else {
				continue
			}
			if len(x.Results) == 1 && x.Results[0] == e && ft.Results.NumFields() == 1 {
				t = rw.Info.Types[ft.Results.List[0].Type].Type
			}
			break
		}
	case *ast.CompositeLit:
		if tt, ok := types.Unalias(rw.Info.Types[x].Type).Underlying().(*types.Slice); ok {
			t = tt.Elem()
		}
	case *ast.KeyValueExpr:
		if cl, ok := rw.Parent[x].(*ast.CompositeLit); ok && x.Value == e {
			if tt, ok := types.Unalias(rw.Info.Types[cl].Type).Underlying().(*types.Map); ok {
				t = tt.Elem()
			}
		}
	}
	if t == nil {
		return nil
	}
	if _, ok := t.Underlying().(*types.Interface); ok {
		return t
	}
	return nil
}
