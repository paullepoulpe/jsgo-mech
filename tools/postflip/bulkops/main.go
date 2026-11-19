// Command bulkops is a post-flip step: a bulk operation between two typed collections of the
// same type does not go through their erased views.
//
// The translator writes `a.addAll(b)` and `m.putAll(n)` on typed collections against the erased
// interfaces when the argument's declared type is Collection or Map: a.asSet().addAll(
// Collection(b.asSet())). Where a and b are typed collections of the same Go type (go/types),
// the typed operation does the same without a view object, an array of boxed elements and a
// box per element:
//
//	a.asSet().addAll(Collection(b.asSet()))    ->  a.AddAll(b)    (*TSet[K]; addAllT before tnames)
//	a.asList().addAll(Collection(b.asList()))  ->  a.addAllT(b)   (*TList[T])
//	a.asMap().putAll(Map(b.asMap()))           ->  a.putAllT(b)   (*TMap[K, V])
//
// Java's behaviour: the elements of b are added in b's iteration order, the result of addAll is
// whether a changed, an immutable a throws UnsupportedOperationException (the typed methods
// check it before anything else, as Guava's do), a null a or b throws NullPointerException
// (a nil receiver or argument of the typed method is dereferenced). a is evaluated before b, as
// before. Left and counted: an argument that is not the erased view of a typed collection of
// the same type.
//
// forEach with a typed lambda: Map_forEach(m.asMap(), &BiConsumer_tfn_K_V{func(k K, v V) {...}})
// and Iterable_forEach(l.asList(), &Consumer_tfn_T{func(x T) {...}}), where the function's
// parameter types are the map's key and value types (the list's element type), become
// m.forEachT(func...) and l.forEachT(func...): the same calls in the same order, without the
// view, the iterator, an entry object and two boxes per mapping, and the adapter that unboxes
// them again. Left and counted: a function of other parameter types (the adapter converts).
//
// usage: bulkops [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"postflip/rw"
)

func generated(p token.Pos) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.Position(p).Filename), "g_")
}

// viewOf: e is x.<view>() of a typed collection x; returns x and the collection's type name.
func viewOf(e ast.Expr, view string) (ast.Expr, string) {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) != 0 {
		return nil, ""
	}
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != view {
		return nil, ""
	}
	p, ok := rw.Info.TypeOf(sel.X).(*types.Pointer)
	if !ok {
		return nil, ""
	}
	n, ok := p.Elem().(*types.Named)
	if !ok {
		return nil, ""
	}
	return sel.X, n.Origin().Obj().Name()
}

func main() {
	dry := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	type op struct{ view, method, typ, conv string }
	addAll := "addAllT"
	if hasMethod("TSet", "AddAll") {
		addAll = "AddAll"
	}
	ops := map[string]op{
		"asSet.addAll":  {"asSet", addAll, "TSet", "Collection"},
		"asList.addAll": {"asList", "addAllT", "TList", "Collection"},
		"asMap.putAll":  {"asMap", "putAllT", "TMap", "Map"},
	}
	done, left := map[string]int{}, map[string]int{}
	for _, f := range rw.Files {
		if !generated(f.Pos()) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if ok && len(c.Args) == 2 {
				if forEach(c, done, left) {
					return false
				}
			}
			if !ok || len(c.Args) != 1 {
				return true
			}
			sel, ok := c.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			inner, ok := rw.Unparen(sel.X).(*ast.CallExpr)
			if !ok {
				return true
			}
			isel, ok := inner.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			o, ok := ops[isel.Sel.Name+"."+sel.Sel.Name]
			if !ok {
				return true
			}
			a, tn := viewOf(sel.X, o.view)
			if a == nil || tn != o.typ {
				return true
			}
			arg := rw.Unparen(c.Args[0])
			if conv, ok := arg.(*ast.CallExpr); ok && len(conv.Args) == 1 {
				if id, ok := conv.Fun.(*ast.Ident); ok && id.Name == o.conv {
					if _, isType := rw.Info.Uses[id].(*types.TypeName); isType {
						arg = rw.Unparen(conv.Args[0])
					}
				}
			}
			b, tnb := viewOf(arg, o.view)
			if b == nil || tnb != o.typ || !types.Identical(rw.Info.TypeOf(a), rw.Info.TypeOf(b)) {
				left[o.typ+"."+sel.Sel.Name+": the argument is not the erased view of a typed collection of the same type"]++
				return true
			}
			done[o.typ+": "+o.view+"()."+sel.Sel.Name+" -> "+o.method]++
			method := o.method
			rw.Rewrite[c] = func() string { return rw.Render(a) + "." + method + "(" + rw.Render(b) + ")" }
			return false
		})
	}
	total := 0
	for _, k := range sorted(done) {
		total += done[k]
		fmt.Fprintf(os.Stderr, "  %d %s\n", done[k], k)
	}
	fmt.Fprintf(os.Stderr, "bulkops: %d bulk operations between typed collections of one type are typed\n", total)
	for _, k := range sorted(left) {
		fmt.Fprintf(os.Stderr, "left: %d %s\n", left[k], k)
	}
	if !dry {
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.SaveChecked())
	}
}

func hasMethod(typ, name string) bool {
	tn, ok := rw.Pkg.Scope().Lookup(typ).(*types.TypeName)
	if !ok {
		return false
	}
	n, ok := tn.Type().(*types.Named)
	if !ok {
		return false
	}
	for i := 0; i < n.NumMethods(); i++ {
		if n.Method(i).Name() == name {
			return true
		}
	}
	return false
}

func sorted(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// forEach rewrites Map_forEach / Iterable_forEach of a typed collection's view with a typed
// lambda adapter; reports whether it did.
func forEach(c *ast.CallExpr, done, left map[string]int) bool {
	id, ok := c.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	view, typ := "", ""
	switch id.Name {
	case "Map_forEach":
		view, typ = "asMap", "TMap"
	case "Iterable_forEach":
		view, typ = "asList", "TList"
	default:
		return false
	}
	what := typ + ": " + id.Name + " with a typed lambda"
	a, tn := viewOf(c.Args[0], view)
	if a == nil || tn != typ {
		if _, isLit := rw.Unparen(c.Args[1]).(*ast.UnaryExpr); isLit {
			left[what+": not the erased view of a post-flip typed collection"]++
		}
		return false
	}
	u, ok := rw.Unparen(c.Args[1]).(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		left[what+": the action is not a lambda adapter"]++
		return false
	}
	lit, ok := u.X.(*ast.CompositeLit)
	if !ok || len(lit.Elts) != 1 {
		left[what+": the action is not a lambda adapter"]++
		return false
	}
	fl, ok := lit.Elts[0].(*ast.FuncLit)
	if !ok {
		left[what+": the action is not a lambda adapter"]++
		return false
	}
	sig, _ := rw.Info.TypeOf(fl).(*types.Signature)
	args := rw.Info.TypeOf(a).(*types.Pointer).Elem().(*types.Named).TypeArgs()
	if sig == nil || sig.Results().Len() != 0 || sig.Params().Len() != args.Len() {
		left[what+": a function of other parameter types"]++
		return false
	}
	for i := 0; i < args.Len(); i++ {
		if !types.Identical(sig.Params().At(i).Type(), args.At(i)) {
			left[what+": a function of other parameter types"]++
			return false
		}
	}
	done[typ+": "+id.Name+"("+view+"(), lambda) -> forEachT"]++
	rw.Rewrite[c] = func() string { return rw.Render(a) + ".forEachT(" + rw.Render(fl) + ")" }
	return true
}
