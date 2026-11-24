// Command tcmp is a post-flip step: a sort of a typed list (a native slice seen through
// sliceView_ / sliceViewP_) with a typed comparator is the runtime's typed sort of the slice
// with the comparator's function.
//
//	Collections_sort_List_Comparator(sliceViewP_(tc_p_X, this.options_), &Comparator_tfn_X_X{func(a *X, b *X) int32 {...}})
//	  ->
//	sortSlicePF_(this.options_, func(a *X, b *X) int32 {...})
//
// List.sort(c) / Collections.sort(list, c) of a typed list goes through the erased runtime: the
// view boxes every element, the adapter's compare(Object, Object) casts both back and calls its
// typed function. sortSliceF_ / sortSlicePF_ (rt_native.go) sort a copy of the slice with
// sort.SliceStable as List_sort does on the view's boxes, so the comparator is called with the
// same elements in the same order, and write the copy back after the sort returns (an exception
// in the comparator leaves the list as it was, as before); a nil list fails as the nil view did.
// The comparator is typed where it is an adapter literal &X{f} of the slice's element type (f)
// or a never-null named comparator new_C(..) whose compare is implsplit's bridge of its typed
// compareT_ (the method value new_C(..).compareT_, evaluated where the argument was). Left,
// counted: comparators from the runtime's combinators (Comparator.comparing, reversed,
// thenComparing...: erased functions of Objects), package variables, lists that are not slice
// views. The sorted sets (TreeSet made with a Comparator) are colls': its -kind treeset report
// counts them ("made with a Comparator"). A rerun changes nothing.
//
// usage: tcmp [-n] [-v] <dir of package closure>
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

var (
	verbose bool
	counts  = map[string]int{}
	lefts   = map[string][]string{}
)

func left(why string, n ast.Node) {
	lefts[why] = append(lefts[why], rw.Fset.Position(n.Pos()).String())
}

var sorts = map[string]bool{"List_sort": true, "Collections_sort_List_Comparator": true}

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	rw.Load(os.Args[len(os.Args)-1])
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if pqueue35(c) { // reflip35
				return true
			}
			name, _ := rw.CallName(c)
			if !sorts[name] || len(c.Args) != 2 {
				return true
			}
			view, ok := rw.Unparen(c.Args[0]).(*ast.CallExpr)
			vname := ""
			if ok {
				vname, _ = rw.CallName(view)
			}
			if !ok || len(view.Args) != 2 || !(strings.HasPrefix(vname, "sliceView_") || strings.HasPrefix(vname, "sliceViewP_")) {
				left("the list is not a view of a native slice", c)
				return true
			}
			st := rw.Info.TypeOf(view.Args[1])
			if strings.HasPrefix(vname, "sliceViewP_") {
				st = st.(*types.Pointer).Elem()
			}
			elem := st.Underlying().(*types.Slice).Elem()
			fn := typedCmp(c.Args[1], elem)
			if fn == "" {
				left(cmpWhy(c.Args[1]), c)
				return true
			}
			target := "sortSliceF_"
			if strings.HasPrefix(vname, "sliceViewP_") {
				target = "sortSlicePF_"
			}
			counts["sorts of a typed slice with a typed comparator are the typed sort"]++
			lst := view.Args[1]
			rw.Rewrite[c] = func() string { return target + "(" + rw.Render(lst) + ", " + fn + ")" }
			return true
		})
	}
	keys := []string{}
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %4d %s\n", counts[k], k)
	}
	keys = keys[:0]
	for k := range lefts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", len(lefts[k]), k)
		if verbose {
			for _, p := range lefts[k] {
				fmt.Fprintf(os.Stderr, "        %s\n", p)
			}
		}
	}
	if report || len(counts) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "tcmp: files changed: %d\n", rw.SaveChecked())
}

func cmpSig(t types.Type) *types.Signature {
	p := types.NewParam(0, nil, "", t)
	return types.NewSignatureType(nil, nil, nil, types.NewTuple(p, p), types.NewTuple(types.NewParam(0, nil, "", types.Typ[types.Int32])), false)
}

// typedCmp: the typed function of a comparator of elements of type t, or "".
func typedCmp(e ast.Expr, t types.Type) string {
	want := cmpSig(t)
	if g, ok := rw.TypedFn(e, want, false); ok { // reflip33
		return g()
	}
	// &X{f}: a typed lambda adapter
	if u, ok := rw.Unparen(e).(*ast.UnaryExpr); ok {
		if cl, ok := u.X.(*ast.CompositeLit); ok && len(cl.Elts) == 1 {
			if id, ok := cl.Type.(*ast.Ident); ok && strings.Contains(id.Name, "_tfn") {
				if fl, ok := cl.Elts[0].(*ast.FuncLit); ok && types.Identical(rw.Info.TypeOf(fl), want) {
					return rw.Render(fl)
				}
			}
		}
		return ""
	}
	// new_C(..) with compareT_ and its bridge
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok {
		return ""
	}
	id, ok := rw.Unparen(c.Fun).(*ast.Ident)
	if !ok {
		return ""
	}
	pt, ok := rw.Info.TypeOf(e).(*types.Pointer)
	if !ok {
		return ""
	}
	nt, ok := pt.Elem().(*types.Named)
	if !ok || nt.Obj().Pkg() != rw.Pkg || !(id.Name == "new_"+nt.Obj().Name() || strings.HasPrefix(id.Name, "new_"+nt.Obj().Name()+"_")) {
		return ""
	}
	o, _, _ := types.LookupFieldOrMethod(pt, true, rw.Pkg, "compareT_")
	m, ok := o.(*types.Func)
	if !ok {
		return ""
	}
	sig := m.Type().(*types.Signature)
	if !types.Identical(types.NewSignatureType(nil, nil, nil, sig.Params(), sig.Results(), false), want) {
		return ""
	}
	return rw.Render(e) + ".compareT_"
}

func cmpWhy(e ast.Expr) string {
	switch x := rw.Unparen(e).(type) {
	case *ast.CallExpr:
		if n, _ := rw.CallName(x); strings.HasPrefix(n, "Comparator_") {
			return "a comparator from the runtime's combinators (Comparator.comparing, reversed, thenComparing: erased functions of Objects)"
		}
	case *ast.Ident:
		if v, ok := rw.Info.Uses[x].(*types.Var); ok && v.Parent() == rw.Pkg.Scope() {
			return "a comparator held by a package variable (built from the runtime's combinators)"
		}
	}
	return "a comparator that is not a typed adapter or a named comparator with compareT_"
}

// pqueue35 (reflip35): new_PriorityQueue(C), C the converting closure of a typed comparator E with the pure cast K, is
// newPriorityQueueT_(E, K).
func pqueue35(c *ast.CallExpr) bool {
	name, _ := rw.CallName(c)
	var arg ast.Expr
	switch {
	case name == "new_PriorityQueue" && len(c.Args) == 1:
		arg = c.Args[0]
	case name == "new_PriorityQueue_int_Comparator" && len(c.Args) == 2 && rw.Info.Types[c.Args[0]].Value != nil:
		arg = c.Args[1]
	default:
		return false
	}
	if !rw.HasFunc("newPriorityQueueT_") {
		return false
	}
	if id, ok := rw.Unparen(arg).(*ast.Ident); ok && id.Name == "nil" {
		return false
	}
	pure := func(k *ast.CallExpr, t types.Type) bool {
		var id *ast.Ident
		switch f := k.Fun.(type) {
		case *ast.Ident:
			id = f
		case *ast.IndexExpr:
			id, _ = f.X.(*ast.Ident)
		case *ast.IndexListExpr:
			id, _ = f.X.(*ast.Ident)
		}
		if id == nil {
			return false
		}
		if fn, ok := rw.Info.Uses[id].(*types.Func); !ok || fn.Pkg() != rw.Pkg || fn.Parent() != rw.Pkg.Scope() {
			return false
		}
		sig, ok := rw.Info.TypeOf(k.Fun).(*types.Signature)
		if !ok || sig.Params().Len() != 1 || sig.Results().Len() != 1 || !types.Identical(sig.Results().At(0).Type(), t) {
			return false
		}
		if it, ok := sig.Params().At(0).Type().Underlying().(*types.Interface); !ok || !it.Empty() {
			return false
		}
		n := id.Name
		return n == "cast_" || n == "junboxT_" || n == "str_" || strings.HasPrefix(n, "to_") || strings.HasPrefix(n, "icast_") || strings.HasPrefix(n, "gcast_") || strings.HasSuffix(n, "_of_")
	}
	E, ks := rw.ClosureOf(arg, pure)
	if E == nil || len(ks) != 2 || rw.Text(ks[0].Fun) != rw.Text(ks[1].Fun) {
		left("a PriorityQueue whose comparator is not the converting closure of a typed comparator with a pure cast", c)
		return false
	}
	k := ks[0].Fun
	rw.Rewrite[c] = func() string { return "newPriorityQueueT_(" + rw.Render(E) + ", " + rw.Text(k) + ")" }
	counts["PriorityQueues made with the converting closure of a typed comparator keep their elements unboxed"]++
	return true
}
