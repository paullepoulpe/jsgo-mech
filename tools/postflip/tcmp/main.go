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
// The translation of Arrays.sort of a typed array (a closure sorting a boxed copy and writing
// it back) is sortArrF_ with the same comparison. A natural-order sort (Collections.sort(list), List.sort(null / naturalOrder())) of strings
// sorts with compareStrNat_ (String.compareTo; a null throws NullPointerException as the JDK's
// sort does), of a class with implsplit's typed compareToT_ and no subclass with that method
// value. A typed list (L.asList() of a *TList) sorts with sortTListF_ (an immutable or nil list
// goes through List_sort as before). The comparator is typed where it is an adapter literal &X{f} of the slice's element type (f)
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
			if es, ok := n.(*ast.ExprStmt); ok && arraySort(es) {
				return false
			}
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name, _ := rw.CallName(c)
			natural := name == "Collections_sort" && len(c.Args) == 1 || sorts[name] && len(c.Args) == 2 && isNatural(c.Args[1])
			if !natural && (!sorts[name] || len(c.Args) != 2) {
				return true
			}
			view, ok := rw.Unparen(c.Args[0]).(*ast.CallExpr)
			vname := ""
			if ok {
				vname, _ = rw.CallName(view)
			}
			var elem types.Type
			var lst ast.Expr
			target := "sortSliceF_"
			switch {
			case ok && len(view.Args) == 2 && (strings.HasPrefix(vname, "sliceView_") || strings.HasPrefix(vname, "sliceViewP_")):
				st := rw.Info.TypeOf(view.Args[1])
				if strings.HasPrefix(vname, "sliceViewP_") {
					st = st.(*types.Pointer).Elem()
					target = "sortSlicePF_"
				}
				elem, lst = st.Underlying().(*types.Slice).Elem(), view.Args[1]
			case ok && len(view.Args) == 0 && tlistElem(view) != nil:
				elem, lst, target = tlistElem(view), view.Fun.(*ast.SelectorExpr).X, "sortTListF_"
			default:
				left("the list is not a view of a native slice or a typed list", c)
				return true
			}
			var fn string
			if natural {
				if fn = naturalCmp(elem); fn == "" {
					left("a natural-order sort of elements with no typed compareTo (Objects, classes with subclasses)", c)
					return true
				}
			} else if fn = typedCmp(c.Args[1], elem); fn == "" {
				left(cmpWhy(c.Args[1]), c)
				return true
			}
			if natural {
				counts["natural-order sorts of a typed list are the typed sort with the elements' typed compareTo"]++
			} else {
				counts["sorts of a typed list with a typed comparator are the typed sort"]++
			}
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

// arraySort handles the translation of Arrays.sort(a) / Arrays.sort(a, c) of a typed array,
//
//	func() { arr := A; tmp := arrmap_(arr, box); Arrays_sort(tmp); for i := range tmp { arr[i] = unbox(tmp[i]) } }()
//
// which is sortArrF_(A, cmp) with the elements' typed natural order or the typed comparator.
func arraySort(es *ast.ExprStmt) bool {
	call, ok := es.X.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	fl, ok := call.Fun.(*ast.FuncLit)
	if !ok || len(fl.Body.List) != 4 || fl.Type.Params.NumFields() != 0 {
		return false
	}
	st := fl.Body.List
	a1, ok1 := st[0].(*ast.AssignStmt)
	a2, ok2 := st[1].(*ast.AssignStmt)
	s3, ok3 := st[2].(*ast.ExprStmt)
	r4, ok4 := st[3].(*ast.RangeStmt)
	if !ok1 || !ok2 || !ok3 || !ok4 || len(a1.Lhs) != 1 || len(a1.Rhs) != 1 || len(a2.Rhs) != 1 {
		return false
	}
	if n, c := rw.CallName(a2.Rhs[0]); n != "arrmap_" || len(c.Args) != 2 || rw.Text(c.Args[0]) != rw.Text(a1.Lhs[0]) {
		return false
	}
	sn, sc := rw.CallName(s3.X)
	if sc == nil || len(sc.Args) == 0 || rw.Text(sc.Args[0]) != rw.Text(a2.Lhs[0]) || rw.Text(r4.X) != rw.Text(a2.Lhs[0]) {
		return false
	}
	var natural bool
	switch {
	case sn == "Arrays_sort" && len(sc.Args) == 1:
		natural = true
	case sn == "Arrays_sort_ObjectArr_Comparator" && len(sc.Args) == 2:
		natural = isNatural(sc.Args[1])
	default:
		return false
	}
	sl, ok := types.Unalias(rw.Info.TypeOf(a1.Rhs[0])).Underlying().(*types.Slice)
	if !ok {
		return false
	}
	var fn string
	if natural {
		fn = naturalCmp(sl.Elem())
	} else {
		fn = typedCmp(sc.Args[1], sl.Elem())
	}
	if fn == "" {
		left("an array sort without a typed comparison of its elements", es)
		return true
	}
	counts["sorts of a typed array are the typed sort of the array"]++
	arr := a1.Rhs[0]
	rw.Rewrite[es] = func() string { return "sortArrF_(" + rw.Render(arr) + ", " + fn + ")" }
	return true
}

// isNatural reports whether a comparator argument is the natural order (nil or
// Comparator.naturalOrder()).
func isNatural(e ast.Expr) bool {
	if id, ok := rw.Unparen(e).(*ast.Ident); ok && id.Name == "nil" {
		return true
	}
	n, c := rw.CallName(e)
	return c != nil && n == "Comparator_naturalOrder" && len(c.Args) == 0
}

// tlistElem returns T when e is L.asList() with L of type *TList[T].
func tlistElem(e *ast.CallExpr) types.Type {
	s, ok := e.Fun.(*ast.SelectorExpr)
	if !ok || s.Sel.Name != "asList" {
		return nil
	}
	p, ok := types.Unalias(rw.Info.TypeOf(s.X)).(*types.Pointer)
	if !ok {
		return nil
	}
	n, ok := types.Unalias(p.Elem()).(*types.Named)
	if !ok || n.Origin().Obj().Name() != "TList" || n.TypeArgs().Len() != 1 {
		return nil
	}
	return n.TypeArgs().At(0)
}

// naturalCmp is the typed natural order of elements of type t, or "": String.compareTo with
// Java's NullPointerException for a null string (compareStrNat_), or the typed compareTo of a
// class (implsplit's compareToT_, whose compareTo is javac's bridge to it, the method the
// runtime's natural order calls) that no type of the package embeds (a subclass could override
// compareTo).
func naturalCmp(t types.Type) string {
	if b, ok := types.Unalias(t).(*types.Basic); ok && b.Kind() == types.String {
		return "compareStrNat_"
	}
	pt, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return ""
	}
	nt, ok := types.Unalias(pt.Elem()).(*types.Named)
	if !ok || nt.Obj().Pkg() != rw.Pkg || nt.TypeArgs().Len() > 0 || embedded(nt) {
		return ""
	}
	o, _, _ := types.LookupFieldOrMethod(pt, true, rw.Pkg, "compareToT_")
	m, ok := o.(*types.Func)
	if !ok {
		return ""
	}
	sig := m.Type().(*types.Signature)
	if sig.Params().Len() != 1 || !types.Identical(sig.Params().At(0).Type(), t) || sig.Results().Len() != 1 || !types.Identical(sig.Results().At(0).Type(), types.Typ[types.Int32]) {
		return ""
	}
	return "(*" + nt.Obj().Name() + ").compareToT_"
}

// embedded reports whether a struct type of the package embeds n (Java: n has a subclass).
func embedded(n *types.Named) bool {
	for _, name := range rw.Pkg.Scope().Names() {
		tn, ok := rw.Pkg.Scope().Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		st, ok := tn.Type().Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for i := 0; i < st.NumFields(); i++ {
			f := st.Field(i)
			if !f.Embedded() {
				continue
			}
			ft := f.Type()
			if p, ok := ft.(*types.Pointer); ok {
				ft = p.Elem()
			}
			if fn, ok := types.Unalias(ft).(*types.Named); ok && fn.Origin() == n.Origin() {
				return true
			}
		}
	}
	return false
}

func cmpSig(t types.Type) *types.Signature {
	p := types.NewParam(0, nil, "", t)
	return types.NewSignatureType(nil, nil, nil, types.NewTuple(p, p), types.NewTuple(types.NewParam(0, nil, "", types.Typ[types.Int32])), false)
}

// typedCmp: the typed function of a comparator of elements of type t, or "".
func typedCmp(e ast.Expr, t types.Type) string {
	want := cmpSig(t)
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
