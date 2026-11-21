// Command adapters is a post-flip step for the typed translator's text: a call that is given
// a typed lambda adapter object takes the function itself, where the runtime has that form.
//
//	o.filter(&Predicate_tfn_string{func(s string) bool { ... }})      ->  o.filterFn_(func(s string) bool { ... })
//	m.computeIfAbsent(k, &Function_tfn_string_Foo{func(k string) *Foo { ... }})
//	                                                                  ->  m.ComputeIfAbsent(k, func(k string) *Foo { ... })
//	m.computeIfAbsent(k, &Function_tfn_string_any{func(k string) any { ... }})
//	                                                                  ->  m.ComputeIfAbsentObj(k, func(k string) any { ... })
//	Map_computeIfAbsent(m, k, &Function_fn{func(k any) any { ... }})   ->  Map_computeIfAbsent_f(m, k, func(k any) any { ... })
//
// The typed translator writes a Java lambda passed as a Predicate<T> or Function<K, V> as an
// adapter object (X_tfn_...: its method unboxes the arguments, calls the function and boxes
// the result; X_fn for an erased function), and the callee boxes what it passes: per call an
// adapter (and its closure, which escapes with it), boxes and unboxings. The first flip's
// steps funcparams and tnames did the same for the erased wrappers &X_fn{...}.
//   - Optional[T].filter: filterFn_ (rt_base.go) is the same filter with the predicate applied
//     to the content as it is: null check of the Optional, the predicate called once and only
//     when a value is present, the same result. The function must be a func(T) bool.
//   - (*TMap[K, V]).computeIfAbsent: ComputeIfAbsent (rt_tmap.go, tools/postflip/tnames) calls
//     the function with the key as it is and stores its result unless it is null, as
//     computeIfAbsent does through the adapter. The function must be a func(K) V.
//     A func(K) any (the lambda's result is an Object: X_tfn_K_any) goes to ComputeIfAbsentObj,
//     which unboxes the result as computeIfAbsent unboxes what the adapter returns.
//   - Map_computeIfAbsent with an erased wrapper &Function_fn{func(any) any}:
//     Map_computeIfAbsent_f (rt_coll.go) is the same function with the func value.
//
// The argument must be the address of an adapter literal holding one function literal of the
// type named; anything else is left and counted. A rerun changes nothing.
//
// usage: adapters [-n] <dir of package closure>
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

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
	}
	rw.Load(os.Args[len(os.Args)-1])
	done := map[string]int{}
	left := map[string]int{}
	// funcOf is the function literal an adapter literal &X{func...} holds (X named *_tfn* or,
	// erased, *_fn), or nil with the reason.
	funcOf := func(arg ast.Expr, erased bool) (*ast.FuncLit, string) {
		if c, ok := arg.(*ast.CallExpr); ok && len(c.Args) == 1 { // reflip30: a function literal converted to a function type
			if tv, ok := rw.Info.Types[c.Fun]; ok && tv.IsType() {
				if _, isFn := tv.Type.Underlying().(*types.Signature); isFn {
					if fl, ok := rw.Unparen(c.Args[0]).(*ast.FuncLit); ok {
						return fl, ""
					}
				}
			}
		}
		u, ok := arg.(*ast.UnaryExpr)
		if !ok || u.Op != token.AND {
			return nil, "the function is not an adapter literal"
		}
		lit, ok := u.X.(*ast.CompositeLit)
		if !ok || len(lit.Elts) != 1 {
			return nil, "the function is not an adapter literal"
		}
		name := rw.Text(lit.Type)
		if erased != strings.HasSuffix(name, "_fn") || (!erased && !strings.Contains(name, "_tfn")) {
			return nil, "the function is not an adapter literal of the kind the rule takes"
		}
		fl, ok := lit.Elts[0].(*ast.FuncLit)
		if !ok {
			return nil, "the adapter does not hold a function literal"
		}
		return fl, ""
	}
	sigIs := func(fl *ast.FuncLit, params []types.Type, result types.Type) bool {
		sig, ok := rw.Info.Types[fl].Type.(*types.Signature)
		if !ok || sig.Params().Len() != len(params) || sig.Results().Len() != 1 || !types.Identical(sig.Results().At(0).Type(), result) {
			return false
		}
		for i, p := range params {
			if !types.Identical(sig.Params().At(i).Type(), p) {
				return false
			}
		}
		return true
	}
	anyT := types.Universe.Lookup("any").Type()
	for _, file := range rw.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "Map_computeIfAbsent" && len(call.Args) == 3 {
				const rule = "Map_computeIfAbsent"
				fl, why := funcOf(call.Args[2], true)
				if fl == nil {
					left[rule+": "+why]++
					return true
				}
				if !sigIs(fl, []types.Type{anyT}, anyT) {
					left[rule+": the function is not a func(any) any"]++
					return true
				}
				done[rule+" -> Map_computeIfAbsent_f"]++
				rw.Rewrite[id] = func() string { return "Map_computeIfAbsent_f" }
				rw.Rewrite[call.Args[2]] = func() string { return rw.Render(fl) }
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			rt := rw.Info.Types[sel.X].Type
			if p, ok := rt.(*types.Pointer); ok {
				rt = p.Elem()
			}
			nt, ok := types.Unalias(rt).(*types.Named)
			if !ok {
				return true
			}
			switch {
			case sel.Sel.Name == "filter" && nt.Obj().Name() == "Optional" && nt.TypeArgs().Len() == 1 && len(call.Args) == 1:
				const rule = "Optional.filter"
				fl, why := funcOf(call.Args[0], false)
				if fl == nil {
					left[rule+": "+why]++
					return true
				}
				if !sigIs(fl, []types.Type{nt.TypeArgs().At(0)}, types.Typ[types.Bool]) {
					left[rule+": the function is not a func(T) bool of the Optional's T"]++
					return true
				}
				done[rule+" -> filterFn_"]++
				rw.Rewrite[sel.Sel] = func() string { return "filterFn_" }
				rw.Rewrite[call.Args[0]] = func() string { return rw.Render(fl) }
			case sel.Sel.Name == "computeIfAbsent" && nt.Obj().Name() == "TMap" && nt.TypeArgs().Len() == 2 && len(call.Args) == 2:
				const rule = "TMap.computeIfAbsent"
				fl, why := funcOf(call.Args[1], false)
				if fl == nil {
					left[rule+": "+why]++
					return true
				}
				if !sigIs(fl, []types.Type{nt.TypeArgs().At(0)}, nt.TypeArgs().At(1)) {
					if sigIs(fl, []types.Type{nt.TypeArgs().At(0)}, anyT) {
						done[rule+" -> ComputeIfAbsentObj"]++
						rw.Rewrite[sel.Sel] = func() string { return "ComputeIfAbsentObj" }
						rw.Rewrite[call.Args[1]] = func() string { return rw.Render(fl) }
						return true
					}
					left[rule+": the function is neither a func(K) V of the map's types nor a func(K) any"]++
					return true
				}
				done[rule+" -> ComputeIfAbsent"]++
				rw.Rewrite[sel.Sel] = func() string { return "ComputeIfAbsent" }
				rw.Rewrite[call.Args[1]] = func() string { return rw.Render(fl) }
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
	if report {
		fmt.Fprintf(os.Stderr, "adapters: %d calls take the function\n", total)
		return
	}
	fmt.Fprintf(os.Stderr, "adapters: %d calls take the function; files changed: %d\n", total, rw.SaveChecked())
}
