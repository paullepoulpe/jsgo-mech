// Command etaconv is a post-flip step: a func literal that only passes its parameter to a
// function is that function.
//
//	unbox: func(v any) *Var { return cast_[*Var](v) },     ->  unbox: cast_[*Var],
//	box:   func(v *Var) any { return obj_(v) },            ->  box:   obj_[*Var],
//	box:   func(v *JSType) any { return v.self() },        ->  box:   (*JSType).self,
//	unbox: func(v any) *JSType { return to_JSType(v) },    ->  unbox: to_JSType,
//
// The typed containers' conversions (tconv_: g_0casts.go, g_zconv_*.go) and some lambdas
// given to func-typed parameters are written as literals of this shape: one parameter, a body
// `return F(v)` (or `return v.m()`), F a function of the package (a generic one instantiated
// at the literal's types) or m a method, whose signature is exactly the literal's. The value
// F (the method expression (T).m) is a function of the same type that does the same thing: a
// call of it evaluates the argument and calls F on it, as the literal's call did, with the same
// results and failures; Go func values have no identity a program can compare, and the
// literal captured nothing. So the literal is F.
//
// Left, counted: literals whose body does anything else, passes the parameter converted or
// with other arguments, or calls a function whose signature differs (an implicit conversion
// of the argument or the result). A rerun changes nothing.
//
// usage: etaconv [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"postflip/rw"
)

var qual = func(p *types.Package) string {
	if p == rw.Pkg {
		return ""
	}
	return p.Name()
}

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
	}
	rw.Load(os.Args[len(os.Args)-1])
	done := map[string]int{}
	left := map[string]int{}
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			fl, ok := n.(*ast.FuncLit)
			if !ok {
				return true
			}
			sig, ok := rw.Info.TypeOf(fl).(*types.Signature)
			if !ok || sig.Params().Len() != 1 || sig.Results().Len() != 1 || len(fl.Body.List) != 1 {
				return true
			}
			ret, ok := fl.Body.List[0].(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 1 {
				return true
			}
			call, ok := ret.Results[0].(*ast.CallExpr)
			if !ok {
				return true
			}
			p := fl.Type.Params.List[0].Names
			if len(p) != 1 {
				return true
			}
			param := rw.Info.Defs[p[0]]
			isParam := func(e ast.Expr) bool {
				id, ok := e.(*ast.Ident)
				return ok && rw.Info.Uses[id] == param
			}
			if len(rw.UsesOf[param]) != 1 {
				left["the parameter is used other than once"]++
				return true
			}
			var repl string
			var fsig *types.Signature
			switch fun := call.Fun.(type) {
			case *ast.SelectorExpr:
				// v.m()
				if len(call.Args) != 0 || !isParam(fun.X) {
					left["the body is another call"]++
					return true
				}
				s := rw.Info.Selections[fun]
				if s == nil || s.Kind() != types.MethodVal || len(s.Index()) != 1 {
					left["a promoted or field method"]++
					return true
				}
				m := s.Obj().(*types.Func)
				msig := m.Type().(*types.Signature)
				if !types.Identical(msig.Recv().Type(), sig.Params().At(0).Type()) {
					left["the method's receiver is not the parameter's type"]++
					return true
				}
				fsig = types.NewSignatureType(nil, nil, nil, types.NewTuple(msig.Recv()), msig.Results(), false)
				repl = "(" + types.TypeString(msig.Recv().Type(), qual) + ")." + m.Name()
			case *ast.Ident, *ast.IndexExpr:
				if len(call.Args) != 1 || !isParam(call.Args[0]) {
					left["the body is another call"]++
					return true
				}
				var id *ast.Ident
				if ix, ok := fun.(*ast.IndexExpr); ok {
					id, _ = ix.X.(*ast.Ident)
				} else {
					id = fun.(*ast.Ident)
				}
				if id == nil {
					return true
				}
				fo, ok := rw.Info.Uses[id].(*types.Func)
				if !ok || fo.Pkg() != rw.Pkg || fo.Parent() != rw.Pkg.Scope() {
					left["not a function of the package"]++
					return true
				}
				fsig, _ = rw.Info.TypeOf(call.Fun).(*types.Signature)
				repl = rw.Text(call.Fun)
				if inst, ok := rw.Info.Instances[id]; ok && inst.TypeArgs.Len() > 0 {
					if _, explicit := fun.(*ast.IndexExpr); !explicit {
						var as []string
						for i := 0; i < inst.TypeArgs.Len(); i++ {
							as = append(as, types.TypeString(inst.TypeArgs.At(i), qual))
						}
						repl = id.Name + "[" + strings.Join(as, ", ") + "]"
					}
					fsig = inst.Type.(*types.Signature)
				}
			default:
				left["the body is another call"]++
				return true
			}
			if fsig == nil || fsig.Variadic() || fsig.Params().Len() != 1 || fsig.Results().Len() != 1 ||
				!types.Identical(fsig.Params().At(0).Type(), sig.Params().At(0).Type()) ||
				!types.Identical(fsig.Results().At(0).Type(), sig.Results().At(0).Type()) {
				left["the function's signature differs (an implicit conversion)"]++
				return true
			}
			kind := "a function"
			if _, ok := call.Fun.(*ast.SelectorExpr); ok {
				kind = "a method expression"
			} else if strings.Contains(repl, "[") {
				kind = "a generic function instantiated"
			}
			where := "elsewhere"
			if b := filepath.Base(rw.Fset.File(f.Pos()).Name()); b == "g_0casts.go" || strings.HasPrefix(b, "g_zconv") {
				where = "a typed container's conversion"
			}
			done[where+": "+kind]++
			r := repl
			rw.Rewrite[fl] = func() string { return r }
			return false
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
	fmt.Fprintf(os.Stderr, "etaconv: %d func literals are the function they call\n", total)
	if report || total == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "etaconv: files changed: %d\n", rw.SaveChecked())
}
