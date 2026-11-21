// Command tfnapi is a post-flip step: a typed lambda adapter given to one of the runtime's
// erased functional APIs over a typed collection is the function itself, given to the API's
// typed variant (rt_tfnapi.go):
//
//	mapComputeIfAbsent_(kc, vc, m, k, &F_tfn_K_V{f})          ->  mapComputeIfAbsentT_(vc, m, k, f)
//	Map_computeIfAbsent(M.asMap(), box(k), &F_tfn_K_V{f})      ->  M.ComputeIfAbsent(k, f)
//	Iterable_forEach(Iterable(L.asList()), &C_tfn_T{f})        ->  L.forEachT_(f)
//	Map_forEach(M.asMap(), &BiC_tfn_K_V{f})                    ->  M.forEachT_(f)
//	Map_merge(M.asMap(), box(k), box(v), &BiF_tfn_V_V_V{f})    ->  M.mergeT_(k, v, f)
//	Map_compute(M.asMap(), box(k), &BiF_tfn_K_V_V{f})          ->  M.computeT_(k, f)
//
// L is a *TList[T] (through asList() or its immutable twin asImm()), M a *TMap[K, V] (asMap()
// or asImm()). The adapter's method unboxed the API's boxed arguments and boxed f's result for
// the API to unbox; the typed variant calls f in the same order with the same values (the
// views' iterators read the typed collection in place, as the variants do; a box of null is nil
// and its unboxing the null), so the adapter, its boxes and its casts are not made. The
// adapter's function must have exactly the collection's types. The result of merge / compute
// must be unused; computeIfAbsent's is read unboxed where the code cast it, boxed elsewhere.
// Left (counted): collections that are not typed, adapters of other types, other values. A
// rerun changes nothing.
//
// usage: tfnapi [-n] [-v] <dir of package closure>
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
	done    = map[string]int{}
	left    = map[string][]string{}
)

func leave(n ast.Node, why string) {
	left[why] = append(left[why], rw.Fset.Position(n.Pos()).String())
}

func typeOf(e ast.Expr) types.Type { return rw.Info.TypeOf(e) }

func generic(t types.Type, name string) []types.Type {
	p, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return nil
	}
	n, ok := types.Unalias(p.Elem()).(*types.Named)
	if !ok || n.Origin().Obj().Name() != name || n.Obj().Pkg() != rw.Pkg {
		return nil
	}
	var out []types.Type
	for i := 0; i < n.TypeArgs().Len(); i++ {
		out = append(out, n.TypeArgs().At(i))
	}
	return out
}

// adapterFn returns f of &X{f}, X a typed lambda adapter (one func field).
func adapterFn(e ast.Expr) ast.Expr {
	u, ok := rw.Unparen(e).(*ast.UnaryExpr)
	if !ok {
		return nil
	}
	cl, ok := u.X.(*ast.CompositeLit)
	if !ok || len(cl.Elts) != 1 {
		return nil
	}
	var name string
	switch t := cl.Type.(type) {
	case *ast.Ident:
		name = t.Name
	case *ast.IndexExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			name = id.Name
		}
	case *ast.IndexListExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			name = id.Name
		}
	}
	if !strings.Contains(name, "_tfn") {
		return nil
	}
	if _, ok := cl.Elts[0].(*ast.KeyValueExpr); ok {
		return nil
	}
	return cl.Elts[0]
}

func sig(params []types.Type, res types.Type) *types.Signature {
	var ps []*types.Var
	for _, p := range params {
		ps = append(ps, types.NewParam(0, nil, "", p))
	}
	var rs *types.Tuple
	if res != nil {
		rs = types.NewTuple(types.NewParam(0, nil, "", res))
	}
	return types.NewSignatureType(nil, nil, nil, types.NewTuple(ps...), rs, false)
}

// fnOf returns the code of the typed function a stands for (reflip33: rw.TypedFn reads the adapter
// literal, the translator's function literal at the erased type and its converting closure).
func fnOf(a ast.Expr, want *types.Signature) func() string {
	f, ok := rw.TypedFn(a, want, false)
	if !ok {
		return nil
	}
	return f
}

// listOf returns L for Iterable(L.asList()), iup_Iterable(L.asImm()), L.asList() / L.asImm().
func collOf(e ast.Expr, name string, views ...string) (ast.Expr, []types.Type) {
	e = rw.Unparen(e)
	if c, ok := e.(*ast.CallExpr); ok && len(c.Args) == 1 {
		if tv, ok := rw.Info.Types[c.Fun]; ok && tv.IsType() {
			e = rw.Unparen(c.Args[0])
		} else if n, _ := rw.CallName(c); strings.HasPrefix(n, "iup_") {
			e = rw.Unparen(c.Args[0])
		}
	}
	c, ok := e.(*ast.CallExpr)
	if !ok || len(c.Args) != 0 {
		return nil, nil
	}
	s, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, nil
	}
	for _, v := range views {
		if s.Sel.Name == v {
			if ts := generic(typeOf(s.X), name); ts != nil {
				return s.X, ts
			}
		}
	}
	return nil, nil
}

// unboxed returns the code of a boxed argument as a value of type t (x of obj_(x) etc.), or "".
func unboxed(a ast.Expr, t types.Type) string {
	if types.Identical(typeOf(a), t) {
		return rw.Render(a)
	}
	call, ok := rw.Unparen(a).(*ast.CallExpr)
	if !ok {
		return ""
	}
	var inner ast.Expr
	if id, ok := call.Fun.(*ast.Ident); ok && len(call.Args) == 1 && (id.Name == "obj_" || id.Name == "strobj_" || id.Name == "any" || id.Name == "boxT_") {
		inner = call.Args[0]
	} else if s, ok := call.Fun.(*ast.SelectorExpr); ok && len(call.Args) == 0 && (s.Sel.Name == "self" || s.Sel.Name == "box_") {
		inner = s.X
	}
	if inner != nil && types.Identical(typeOf(inner), t) {
		return rw.Render(inner)
	}
	return ""
}

func stmtOnly(c *ast.CallExpr) bool {
	_, ok := rw.Up(c).(*ast.ExprStmt)
	return ok
}

func visit(c *ast.CallExpr) {
	name, _ := rw.CallName(c)
	switch name {
	case "mapComputeIfAbsent_":
		if len(c.Args) != 5 {
			return
		}
		mt, ok := types.Unalias(typeOf(c.Args[2])).Underlying().(*types.Map)
		if !ok {
			return
		}
		f := fnOf(c.Args[4], sig([]types.Type{mt.Key()}, mt.Elem()))
		if f == nil {
			leave(c, "computeIfAbsent: not an adapter of the map's key and value types")
			return
		}
		a := c.Args
		rw.Rewrite[c] = func() string {
			return "mapComputeIfAbsentT_(" + rw.Render(a[1]) + ", " + rw.Render(a[2]) + ", " + rw.Render(a[3]) + ", " + f() + ")"
		}
		done["mapComputeIfAbsent_ of an adapter: mapComputeIfAbsentT_"]++
	case "Map_computeIfAbsent":
		if len(c.Args) != 3 {
			return
		}
		M, kv := collOf(c.Args[0], "TMap", "asMap")
		if M == nil {
			leave(c, "computeIfAbsent: an erased map")
			return
		}
		f := fnOf(c.Args[2], sig([]types.Type{kv[0]}, kv[1]))
		k := unboxed(c.Args[1], kv[0])
		if f == nil || k == "" {
			leave(c, "computeIfAbsent: not an adapter of the map's types, or a key that is not a box")
			return
		}
		code := func() string { return rw.Render(M) + ".ComputeIfAbsent(" + k + ", " + f() + ")" }
		if stmtOnly(c) {
			rw.Rewrite[c] = code
		} else {
			e, ok := read(kv[1], c, code)
			if !ok {
				leave(c, "computeIfAbsent: the result read boxed, of a type with no boxing conversion")
				return
			}
			rw.Rewrite[e.Node] = e.Gen
		}
		done["Map.computeIfAbsent of a typed map's view: ComputeIfAbsent"]++
	case "Iterable_forEach":
		if len(c.Args) != 2 {
			return
		}
		L, ts := collOf(c.Args[0], "TList", "asList", "asImm")
		if L == nil {
			leave(c, "forEach: an erased collection")
			return
		}
		f := fnOf(c.Args[1], sig([]types.Type{ts[0]}, nil))
		if f == nil {
			leave(c, "forEach: not an adapter of the list's element type")
			return
		}
		rw.Rewrite[c] = func() string { return rw.Render(L) + ".forEachT_(" + f() + ")" }
		done["Iterable.forEach of a typed list: forEachT_"]++
	case "Map_forEach":
		if len(c.Args) != 2 {
			return
		}
		M, kv := collOf(c.Args[0], "TMap", "asMap", "asImm")
		if M == nil {
			leave(c, "forEach: an erased map")
			return
		}
		f := fnOf(c.Args[1], sig([]types.Type{kv[0], kv[1]}, nil))
		if f == nil {
			leave(c, "forEach: not an adapter of the map's types")
			return
		}
		rw.Rewrite[c] = func() string { return rw.Render(M) + ".forEachT_(" + f() + ")" }
		done["Map.forEach of a typed map: forEachT_"]++
	case "Map_merge":
		if len(c.Args) != 4 {
			return
		}
		M, kv := collOf(c.Args[0], "TMap", "asMap")
		if M == nil {
			leave(c, "merge: an erased map")
			return
		}
		f := fnOf(c.Args[3], sig([]types.Type{kv[1], kv[1]}, kv[1]))
		k, v := unboxed(c.Args[1], kv[0]), unboxed(c.Args[2], kv[1])
		if f == nil || k == "" || v == "" || !stmtOnly(c) {
			leave(c, "merge: not an adapter of the map's types, arguments that are not boxes, or the result used")
			return
		}
		rw.Rewrite[c] = func() string { return rw.Render(M) + ".mergeT_(" + k + ", " + v + ", " + f() + ")" }
		done["Map.merge of a typed map's view: mergeT_"]++
	case "Map_compute":
		if len(c.Args) != 3 {
			return
		}
		M, kv := collOf(c.Args[0], "TMap", "asMap")
		if M == nil {
			leave(c, "compute: an erased map")
			return
		}
		f := fnOf(c.Args[2], sig([]types.Type{kv[0], kv[1]}, kv[1]))
		k := unboxed(c.Args[1], kv[0])
		if f == nil || k == "" || !stmtOnly(c) {
			leave(c, "compute: not an adapter of the map's types, a key that is not a box, or the result used")
			return
		}
		rw.Rewrite[c] = func() string { return rw.Render(M) + ".computeT_(" + k + ", " + f() + ")" }
		done["Map.compute of a typed map's view: computeT_"]++
	}
}

// read: the rewrite of a read n (of type any) whose value of type t is code().
func read(t types.Type, n ast.Expr, code func() string) (rw.Edit, bool) {
	cv := rw.ConvFor(t)
	if p, ok := rw.Up(n).(*ast.CallExpr); ok && len(p.Args) == 1 && rw.Unparen(p.Args[0]) == n {
		name, _ := rw.CallName(p)
		if strings.HasPrefix(name, "cast_[") && types.Identical(typeOf(p), t) || cv.OK && name != "" && cv.IsUnbox(name) {
			return rw.Edit{Node: p, Gen: code}, true
		}
	}
	if !cv.OK {
		return rw.Edit{}, false
	}
	return rw.Edit{Node: n, Gen: func() string { return cv.Box(code()) }}, true
}

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
			if c, ok := n.(*ast.CallExpr); ok {
				visit(c)
			}
			return true
		})
	}
	var ks []string
	for k := range done {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	total := 0
	for _, k := range ks {
		fmt.Fprintf(os.Stderr, "tfnapi: %d %s\n", done[k], k)
		total += done[k]
	}
	ks = ks[:0]
	for k := range left {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Fprintf(os.Stderr, "tfnapi: left: %d %s\n", len(left[k]), k)
		if verbose {
			for _, w := range left[k] {
				fmt.Fprintf(os.Stderr, "        %s\n", w)
			}
		}
	}
	if report || total == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "tfnapi: files changed: %d\n", rw.SaveChecked())
}
